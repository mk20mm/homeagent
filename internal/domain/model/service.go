package model

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/sashabaranov/go-openai"
)

// ModelInfo 模型视图。
type ModelInfo struct {
	ID          string
	ModelName   string // 供应商侧名，如 gpt-4o
	DisplayName string
	Provider    string // 供应商标识
	IsDefault   bool
	Enabled     bool
}

// ProviderInfo 供应商视图（api_key 脱敏，明文绝不跨层）。
type ProviderInfo struct {
	ID           string
	Name         string // 供应商标识，如 deepseek
	BaseURL      string
	APIKeyMasked string // 脱敏值
	APIKeySet    bool   // 是否已配密钥
	Enabled      bool
}

// ProviderUpdate 更新入参（APIKey 空串 = 不修改）。
type ProviderUpdate struct {
	APIKey  *string
	BaseURL *string
	Enabled *bool
}

// ModelUpdate 模型更新入参（支持修改名称、标识、窗口、启停与设默认）。
type ModelUpdate struct {
	ModelName     *string
	DisplayName   *string
	ContextWindow *int
	Enabled       *bool
	IsDefault     *bool
}

// ProviderTestRequest 测试连接入参。
type ProviderTestRequest struct {
	ProviderID *string
	BaseURL    *string
	APIKey     *string
	Model      *string
}

// ProviderTestResult 测试连接出参。
type ProviderTestResult struct {
	OK        bool
	LatencyMs int
	Error     *string
	Models    []string
}

// ModelCreateRequest 创建模型入参。
type ModelCreateRequest struct {
	ProviderID    string
	ModelName     string
	DisplayName   string
	ContextWindow *int
	IsDefault     *bool
}

// ModelRepo 仓储接口（store 层实现）。
type ModelRepo interface {
	ListEnabled(ctx context.Context) ([]ModelInfo, error)
	ListAll(ctx context.Context) ([]ModelInfo, error)
	FindModel(ctx context.Context, modelIDOrName string) (ModelInfo, error)
	SetConversationModel(ctx context.Context, conversationID, modelID string) error
	GetConversationModel(ctx context.Context, conversationID string) (ModelInfo, error)
}

// ProviderRepo 供应商管理仓储（admin 配置用）。
type ProviderRepo interface {
	ListProviders(ctx context.Context) ([]ProviderInfo, error)
	GetProvider(ctx context.Context, id string) (ProviderInfo, error)
	GetProviderDecrypted(ctx context.Context, id string) (apiKey string, baseURL string, name string, err error)
	UpdateProvider(ctx context.Context, id string, u ProviderUpdate) (ProviderInfo, error)
	CreateModel(ctx context.Context, req ModelCreateRequest) (ModelInfo, error)
	UpdateModel(ctx context.Context, id string, u ModelUpdate) (ModelInfo, error)
	DeleteModel(ctx context.Context, id string) error

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
	ListAllModels(ctx context.Context) ([]ModelInfo, error)
	SwitchModel(ctx context.Context, conversationID, modelIDOrName string) (ModelInfo, error)
	ConversationModel(ctx context.Context, conversationID string) (ModelInfo, error)
	ListProviders(ctx context.Context) ([]ProviderInfo, error)
	UpdateProvider(ctx context.Context, id string, u ProviderUpdate) (ProviderInfo, error)
	CreateModel(ctx context.Context, req ModelCreateRequest) (ModelInfo, error)
	UpdateModel(ctx context.Context, id string, u ModelUpdate) (ModelInfo, error)
	DeleteModel(ctx context.Context, id string) error
	TestConnection(ctx context.Context, req ProviderTestRequest) (ProviderTestResult, error)
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

func (s *service) ListAllModels(ctx context.Context) ([]ModelInfo, error) {
	return s.repo.ListAll(ctx)
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

// CreateModel 添加自定义模型。
func (s *service) CreateModel(ctx context.Context, req ModelCreateRequest) (ModelInfo, error) {
	if req.ProviderID == "" {
		return ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "缺少供应商 id", nil)
	}
	if strings.TrimSpace(req.ModelName) == "" {
		return ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "模型名不能为空", nil)
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		req.DisplayName = req.ModelName
	}
	return s.provRepo.CreateModel(ctx, req)
}

// UpdateModel 更新模型配置；设默认时旧默认互斥取消。
func (s *service) UpdateModel(ctx context.Context, id string, u ModelUpdate) (ModelInfo, error) {
	if id == "" {
		return ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "缺少模型 id", nil)
	}
	if u.IsDefault != nil && !*u.IsDefault {
		return ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "取消默认请直接将其他模型设为默认", nil)
	}
	return s.provRepo.UpdateModel(ctx, id, u)
}

// DeleteModel 删除自定义模型。
func (s *service) DeleteModel(ctx context.Context, id string) error {
	if id == "" {
		return apperr.New(apperr.CodeInvalidInput, "缺少模型 id", nil)
	}
	return s.provRepo.DeleteModel(ctx, id)
}

// TestConnection 连通性测试。
func (s *service) TestConnection(ctx context.Context, req ProviderTestRequest) (ProviderTestResult, error) {
	apiKey := ""
	baseURL := ""
	model := ""

	if req.ProviderID != nil && *req.ProviderID != "" {
		decKey, decURL, _, err := s.provRepo.GetProviderDecrypted(ctx, *req.ProviderID)
		if err == nil {
			apiKey = decKey
			baseURL = decURL
		}
	}

	if req.APIKey != nil && strings.TrimSpace(*req.APIKey) != "" {
		apiKey = strings.TrimSpace(*req.APIKey)
	}
	if req.BaseURL != nil && strings.TrimSpace(*req.BaseURL) != "" {
		baseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.Model != nil && strings.TrimSpace(*req.Model) != "" {
		model = strings.TrimSpace(*req.Model)
	}

	if apiKey == "" {
		errStr := "未配置或未提供 API Key"
		return ProviderTestResult{
			OK:    false,
			Error: &errStr,
		}, nil
	}

	if model == "" {
		if strings.Contains(baseURL, "atria") {
			model = "Atria-Dawn-Preview"
		} else if strings.Contains(baseURL, "deepseek") {
			model = "deepseek-chat"
		} else {
			model = "gpt-4o-mini"
		}
	}

	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(cfg)

	start := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err := client.CreateChatCompletion(probeCtx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "ping"},
		},
		MaxTokens: 1,
	})
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		errDesc := formatProbeError(err)
		return ProviderTestResult{
			OK:        false,
			LatencyMs: latency,
			Error:     &errDesc,
		}, nil
	}

	return ProviderTestResult{
		OK:        true,
		LatencyMs: latency,
	}, nil
}

func formatProbeError(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatusCode == http.StatusUnauthorized {
			return fmt.Sprintf("认证失败：API Key 无效或过期 (HTTP 401: %s)", apiErr.Message)
		}
		if apiErr.HTTPStatusCode == http.StatusPaymentRequired {
			return fmt.Sprintf("账户欠费或余额不足 (HTTP 402: %s)", apiErr.Message)
		}
		if apiErr.HTTPStatusCode == http.StatusNotFound {
			return fmt.Sprintf("模型不存在或 Base URL 路径错误 (HTTP 404: %s)", apiErr.Message)
		}
		return fmt.Sprintf("API 请求失败 (HTTP %d: %s)", apiErr.HTTPStatusCode, apiErr.Message)
	}
	if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
		return "连接超时：请检查网络代理或 Base URL 是否正确可达"
	}
	if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host") {
		return "网络不可达：无法连接到指定的 Base URL"
	}
	return err.Error()
}

// normalizeModelName 容错：用户说 "gpt4o"/"GPT-4O" 都能匹配。
func normalizeModelName(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", ""))
}

