// Package model 会话内模型切换：只改 conversation 的 model 关联，工具集不变（调度器设计 §3）。
package model

import (
	"context"
	"strings"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// ModelInfo 模型视图。
type ModelInfo struct {
	ID          string
	ModelName   string // 供应商侧名，如 gpt-4o
	DisplayName string
	Provider    string // 供应商标识
	IsDefault   bool
}

// ProviderInfo 供应商视图（api_key 脱敏，明文绝不跨层）。
type ProviderInfo struct {
	ID            string
	Name          string // 供应商标识，如 deepseek
	BaseURL       string
	APIKeyMasked  string // 脱敏值
	APIKeySet     bool   // 是否已配密钥
	Enabled       bool
}

// ProviderUpdate 更新入参（APIKey 空串 = 不修改）。
type ProviderUpdate struct {
	APIKey  *string
	BaseURL *string
	Enabled *bool
}

// ModelUpdate 模型更新入参（设默认是互斥操作）。
type ModelUpdate struct {
	Enabled   *bool
	IsDefault *bool
}

// ModelRepo 仓储接口（store 层实现）。
type ModelRepo interface {
	ListEnabled(ctx context.Context) ([]ModelInfo, error)
	FindModel(ctx context.Context, modelIDOrName string) (ModelInfo, error)
	SetConversationModel(ctx context.Context, conversationID, modelID string) error
	GetConversationModel(ctx context.Context, conversationID string) (ModelInfo, error)
}

// ProviderRepo 供应商管理仓储（admin 配置用）。
type ProviderRepo interface {
	ListProviders(ctx context.Context) ([]ProviderInfo, error)
	UpdateProvider(ctx context.Context, id string, u ProviderUpdate) (ProviderInfo, error)
	UpdateModel(ctx context.Context, id string, u ModelUpdate) (ModelInfo, error)

	// DefaultEnabled 取当前默认模型的完整连接信息（含解密后的明文密钥）。
	// 仅供启动期装配网关用，绝不返回给 handler 层（明文不跨层传递）。
	DefaultEnabled(ctx context.Context) (conn ProviderConnection, ok bool, err error)
}

// ProviderConnection 网关连接信息（明文密钥，只在 store→main 之间传递）。
type ProviderConnection struct {
	APIKey   string // 明文
	BaseURL  string
	Provider string
	Model    string
}

// Service 模型领域服务。
type Service interface {
	ListModels(ctx context.Context) ([]ModelInfo, error)
	SwitchModel(ctx context.Context, conversationID, modelIDOrName string) (ModelInfo, error)
	ConversationModel(ctx context.Context, conversationID string) (ModelInfo, error)
	ListProviders(ctx context.Context) ([]ProviderInfo, error)
	UpdateProvider(ctx context.Context, id string, u ProviderUpdate) (ProviderInfo, error)
	UpdateModel(ctx context.Context, id string, u ModelUpdate) (ModelInfo, error)
}

func NewService(repo ModelRepo, provRepo ProviderRepo) Service {
	return &service{repo: repo, provRepo: provRepo}
}

type service struct {
	repo     ModelRepo
	provRepo ProviderRepo
}

func (s *service) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return s.repo.ListEnabled(ctx)
}

// SwitchModel 切换会话模型：校验存在且启用 → 更新关联。返回切换后的模型。
func (s *service) SwitchModel(ctx context.Context, conversationID, modelIDOrName string) (ModelInfo, error) {
	if conversationID == "" {
		return ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "缺少会话上下文", nil)
	}
	m, err := s.repo.FindModel(ctx, normalizeModelName(modelIDOrName))
	if err != nil {
		return ModelInfo{}, err
	}
	if err := s.repo.SetConversationModel(ctx, conversationID, m.ID); err != nil {
		return ModelInfo{}, err
	}
	return m, nil
}

func (s *service) ConversationModel(ctx context.Context, conversationID string) (ModelInfo, error) {
	return s.repo.GetConversationModel(ctx, conversationID)
}

// ListProviders 供应商清单（密钥脱敏）。
func (s *service) ListProviders(ctx context.Context) ([]ProviderInfo, error) {
	return s.provRepo.ListProviders(ctx)
}

// UpdateProvider 更新供应商配置：密钥非空才覆盖（空串 = 不改）。
func (s *service) UpdateProvider(ctx context.Context, id string, u ProviderUpdate) (ProviderInfo, error) {
	if id == "" {
		return ProviderInfo{}, apperr.New(apperr.CodeInvalidInput, "缺少供应商 id", nil)
	}
	return s.provRepo.UpdateProvider(ctx, id, u)
}

// UpdateModel 更新模型配置；设默认时旧默认互斥取消。
func (s *service) UpdateModel(ctx context.Context, id string, u ModelUpdate) (ModelInfo, error) {
	if id == "" {
		return ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "缺少模型 id", nil)
	}
	if u.IsDefault != nil && !*u.IsDefault {
		// 取消默认：必须有显式新默认，否则全家无默认模型
		return ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "取消默认请直接将其他模型设为默认", nil)
	}
	return s.provRepo.UpdateModel(ctx, id, u)
}

// normalizeModelName 容错：用户说 "gpt4o"/"GPT-4O" 都能匹配。
func normalizeModelName(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", ""))
}
