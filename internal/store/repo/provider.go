package repo

import (
	"context"

	"github.com/mk20mm/homeagent/internal/apperr"
	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
	"github.com/mk20mm/homeagent/internal/infra/crypto"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/llmmodel"
	"github.com/mk20mm/homeagent/internal/store/ent/llmprovider"
)
// 编译期：实现 dommodel.ProviderRepo。
var _ dommodel.ProviderRepo = (*providerStore)(nil)

// encryptionKey 由 main 注入（来自 ENCRYPTION_KEY）。
type providerStore struct {
	*Store
	key string
}

// NewProviderStore 包装聚合 Store，附加加密能力。
func NewProviderStore(s *Store, encryptionKey string) dommodel.ProviderRepo {
	return &providerStore{Store: s, key: encryptionKey}
}

// ListProviders 供应商清单（api_key 脱敏）。
func (s *providerStore) ListProviders(ctx context.Context) ([]dommodel.ProviderInfo, error) {
	list, err := s.db.LLMProvider.Query().All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询供应商失败", err)
	}
	out := make([]dommodel.ProviderInfo, 0, len(list))
	for _, p := range list {
		out = append(out, toProviderInfo(p, s.key))
	}
	return out, nil
}

// UpdateProvider 更新供应商；api_key 空串/nil = 不修改。
func (s *providerStore) UpdateProvider(ctx context.Context, id string, u dommodel.ProviderUpdate) (dommodel.ProviderInfo, error) {
	p, err := s.db.LLMProvider.Query().Where(llmprovider.IDEQ(toUUID(id))).Only(ctx)
	if err != nil {
		return dommodel.ProviderInfo{}, apperr.New(apperr.CodeNotFound, "供应商不存在", err)
	}

	upd := p.Update()
	if u.APIKey != nil && *u.APIKey != "" {
		enc, err := crypto.Encrypt(*u.APIKey, s.key)
		if err != nil {
			return dommodel.ProviderInfo{}, apperr.New(apperr.CodeInternal, "密钥加密失败", err)
		}
		upd.SetAPIKey(enc)
	}
	if u.BaseURL != nil {
		upd.SetBaseURL(*u.BaseURL)
	}
	if u.Enabled != nil {
		upd.SetEnabled(*u.Enabled)
	}

	saved, err := upd.Save(ctx)
	if err != nil {
		return dommodel.ProviderInfo{}, apperr.New(apperr.CodeInternal, "更新供应商失败", err)
	}
	return toProviderInfo(saved, s.key), nil
}

// UpdateModel 更新模型；设默认时旧默认互斥取消（单事务内完成）。
func (s *providerStore) UpdateModel(ctx context.Context, id string, u dommodel.ModelUpdate) (dommodel.ModelInfo, error) {
	m, err := s.db.LLMModel.Query().Where(llmmodel.IDEQ(toUUID(id))).Only(ctx)
	if err != nil {
		return dommodel.ModelInfo{}, apperr.New(apperr.CodeNotFound, "模型不存在", err)
	}

	upd := m.Update()
	if u.ModelName != nil && *u.ModelName != "" {
		upd.SetModelName(*u.ModelName)
	}
	if u.DisplayName != nil && *u.DisplayName != "" {
		upd.SetDisplayName(*u.DisplayName)
	}
	if u.ContextWindow != nil && *u.ContextWindow > 0 {
		upd.SetContextWindow(*u.ContextWindow)
	}
	if u.Enabled != nil {
		upd.SetEnabled(*u.Enabled)
	}
	if u.IsDefault != nil && *u.IsDefault {
		// 互斥：先取消其他默认，再设本模型
		if _, err := s.db.LLMModel.Update().
			Where(llmmodel.IsDefaultEQ(true), llmmodel.Not(llmmodel.ID(m.ID))).
			SetIsDefault(false).
			Save(ctx); err != nil {
			return dommodel.ModelInfo{}, apperr.New(apperr.CodeInternal, "取消旧默认模型失败", err)
		}
		upd.SetIsDefault(true)
	}

	saved, err := upd.Save(ctx)
	if err != nil {
		return dommodel.ModelInfo{}, apperr.New(apperr.CodeInternal, "更新模型失败", err)
	}
	// 更新后的实体没有 edge，重查带 provider 的完整视图
	loaded, err := s.db.LLMModel.Query().
		Where(llmmodel.IDEQ(saved.ID)).
		WithProvider().
		Only(ctx)
	if err != nil {
		return toModelInfo(saved), nil // provider 名加载失败不致命
	}
	return toModelInfo(loaded), nil
}

// DefaultEnabled 取默认模型的连接信息（解密明文，仅供启动期装配网关）。
// 条件：模型启用 + is_default + 供应商启用 + 供应商已配密钥。
func (s *providerStore) DefaultEnabled(ctx context.Context) (conn dommodel.ProviderConnection, ok bool, err error) {
	m, err := s.db.LLMModel.Query().
		Where(llmmodel.EnabledEQ(true), llmmodel.IsDefaultEQ(true)).
		WithProvider().
		Only(ctx)
	if err != nil {
		// 没有默认模型（NotFound）是正常情况
		return dommodel.ProviderConnection{}, false, nil
	}
	if m.Edges.Provider == nil || !m.Edges.Provider.Enabled {
		return dommodel.ProviderConnection{}, false, nil
	}

	plain, decErr := crypto.Decrypt(m.Edges.Provider.APIKey, s.key)
	if decErr != nil || plain == "" {
		return dommodel.ProviderConnection{}, false, nil
	}

	return dommodel.ProviderConnection{
		APIKey:   plain,
		BaseURL:  m.Edges.Provider.BaseURL,
		Provider: string(m.Edges.Provider.Name),
		Model:    m.ModelName,
	}, true, nil
}

// GetProvider 查询单个供应商信息（脱敏）。
func (s *providerStore) GetProvider(ctx context.Context, id string) (dommodel.ProviderInfo, error) {
	p, err := s.db.LLMProvider.Query().Where(llmprovider.IDEQ(toUUID(id))).Only(ctx)
	if err != nil {
		return dommodel.ProviderInfo{}, apperr.New(apperr.CodeNotFound, "供应商不存在", err)
	}
	return toProviderInfo(p, s.key), nil
}

// GetProviderDecrypted 查询解密后的供应商明文凭据（仅供测试连接/内部网关使用）。
func (s *providerStore) GetProviderDecrypted(ctx context.Context, id string) (apiKey string, baseURL string, name string, err error) {
	p, qerr := s.db.LLMProvider.Query().Where(llmprovider.IDEQ(toUUID(id))).Only(ctx)
	if qerr != nil {
		return "", "", "", apperr.New(apperr.CodeNotFound, "供应商不存在", qerr)
	}
	plain, decErr := crypto.Decrypt(p.APIKey, s.key)
	if decErr != nil {
		plain = ""
	}
	return plain, p.BaseURL, string(p.Name), nil
}

// CreateModel 添加自定义模型。
func (s *providerStore) CreateModel(ctx context.Context, req dommodel.ModelCreateRequest) (dommodel.ModelInfo, error) {
	prov, err := s.db.LLMProvider.Query().Where(llmprovider.IDEQ(toUUID(req.ProviderID))).Only(ctx)
	if err != nil {
		return dommodel.ModelInfo{}, apperr.New(apperr.CodeNotFound, "所属供应商不存在", err)
	}

	builder := s.db.LLMModel.Create().
		SetProviderID(prov.ID).
		SetModelName(req.ModelName).
		SetDisplayName(req.DisplayName).
		SetEnabled(true)

	if req.ContextWindow != nil && *req.ContextWindow > 0 {
		builder.SetContextWindow(*req.ContextWindow)
	}

	if req.IsDefault != nil && *req.IsDefault {
		if _, err := s.db.LLMModel.Update().
			Where(llmmodel.IsDefaultEQ(true)).
			SetIsDefault(false).
			Save(ctx); err != nil {
			return dommodel.ModelInfo{}, apperr.New(apperr.CodeInternal, "取消旧默认模型失败", err)
		}
		builder.SetIsDefault(true)
	}

	saved, err := builder.Save(ctx)
	if err != nil {
		return dommodel.ModelInfo{}, apperr.New(apperr.CodeInternal, "创建模型失败", err)
	}

	loaded, err := s.db.LLMModel.Query().
		Where(llmmodel.IDEQ(saved.ID)).
		WithProvider().
		Only(ctx)
	if err != nil {
		return toModelInfo(saved), nil
	}
	return toModelInfo(loaded), nil
}

// DeleteModel 删除自定义模型（不能删除当前默认模型）。
func (s *providerStore) DeleteModel(ctx context.Context, id string) error {
	m, err := s.db.LLMModel.Query().Where(llmmodel.IDEQ(toUUID(id))).Only(ctx)
	if err != nil {
		return apperr.New(apperr.CodeNotFound, "模型不存在", err)
	}
	if m.IsDefault {
		return apperr.New(apperr.CodeInvalidInput, "不能删除当前默认模型，请先将其他模型设为默认", nil)
	}

	if err := s.db.LLMModel.DeleteOneID(m.ID).Exec(ctx); err != nil {
		return apperr.New(apperr.CodeInternal, "删除模型失败", err)
	}
	return nil
}

func toProviderInfo(p *ent.LLMProvider, key string) dommodel.ProviderInfo {
	plain, err := crypto.Decrypt(p.APIKey, key)
	if err != nil {
		// 解密失败（旧明文或 key 变更）：标记为已配置但不回显
		return dommodel.ProviderInfo{
			ID: p.ID.String(), Name: string(p.Name), BaseURL: p.BaseURL,
			APIKeyMasked: crypto.Mask(p.APIKey), APIKeySet: p.APIKey != "",
			Enabled: p.Enabled,
		}
	}
	return dommodel.ProviderInfo{
		ID: p.ID.String(), Name: string(p.Name), BaseURL: p.BaseURL,
		APIKeyMasked: crypto.Mask(plain), APIKeySet: plain != "",
		Enabled: p.Enabled,
	}
}
