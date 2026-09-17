package repo

import (
	"context"

	"github.com/mk20mm/homeagent/internal/apperr"
	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/conversation"
	"github.com/mk20mm/homeagent/internal/store/ent/llmmodel"
)

// 编译期接口实现检查。
var _ dommodel.ModelRepo = (*Store)(nil)

// ListEnabled 已启用模型清单（含供应商）。
func (s *Store) ListEnabled(ctx context.Context) ([]dommodel.ModelInfo, error) {
	list, err := s.db.LLMModel.Query().
		Where(llmmodel.EnabledEQ(true)).
		WithProvider().
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询模型清单失败", err)
	}
	out := make([]dommodel.ModelInfo, 0, len(list))
	for _, m := range list {
		provider := ""
		if m.Edges.Provider != nil {
			provider = string(m.Edges.Provider.Name)
		}
		out = append(out, dommodel.ModelInfo{
			ID:          m.ID.String(),
			ModelName:   m.ModelName,
			DisplayName: m.DisplayName,
			Provider:    provider,
			IsDefault:   m.IsDefault,
		})
	}
	return out, nil
}

// FindModel 按 id 或模型名查找（switch 容错：id 优先，名字模糊兜底）。
func (s *Store) FindModel(ctx context.Context, modelIDOrName string) (dommodel.ModelInfo, error) {
	m, err := s.db.LLMModel.Query().
		Where(llmmodel.EnabledEQ(true), llmmodel.IDEQ(toUUID(modelIDOrName))).
		WithProvider().
		Only(ctx)
	if err != nil {
		// id 未命中 → 按模型名模糊匹配
		m, err = s.db.LLMModel.Query().
			Where(llmmodel.EnabledEQ(true), llmmodel.ModelNameContainsFold(modelIDOrName)).
			WithProvider().
			Only(ctx)
		if err != nil {
			return dommodel.ModelInfo{}, apperr.New(apperr.CodeNotFound, "模型不存在或未启用", err)
		}
	}
	return toModelInfo(m), nil
}

// SetConversationModel 更新会话当前模型。
func (s *Store) SetConversationModel(ctx context.Context, conversationID, modelID string) (err error) {
	defer func() {
		if err != nil {
			err = apperr.New(apperr.CodeInternal, "切换模型失败", err)
		}
	}()
	c, qerr := s.db.Conversation.Query().
		Where(conversation.IDEQ(toUUID(conversationID))).
		Only(ctx)
	if qerr != nil {
		return apperr.New(apperr.CodeNotFound, "会话不存在", qerr)
	}
	if _, err = c.Update().SetModelID(toUUID(modelID)).Save(ctx); err != nil {
		return err
	}
	return nil
}

// GetConversationModel 会话当前模型（未设置返回默认模型；都没有返回 NotFound）。
func (s *Store) GetConversationModel(ctx context.Context, conversationID string) (dommodel.ModelInfo, error) {
	c, err := s.db.Conversation.Query().
		Where(conversation.IDEQ(toUUID(conversationID))).
		WithModel(func(q *ent.LLMModelQuery) {
			q.Where(llmmodel.EnabledEQ(true)).WithProvider()
		}).
		Only(ctx)
	if err != nil {
		return dommodel.ModelInfo{}, apperr.New(apperr.CodeNotFound, "会话不存在", err)
	}
	if c.Edges.Model == nil {
		// 未设置 → 查全局默认
		def, err := s.db.LLMModel.Query().
			Where(llmmodel.EnabledEQ(true), llmmodel.IsDefaultEQ(true)).
			WithProvider().
			Only(ctx)
		if err != nil {
			return dommodel.ModelInfo{}, apperr.New(apperr.CodeNotFound, "未设置默认模型", err)
		}
		return toModelInfo(def), nil
	}
	return toModelInfo(c.Edges.Model), nil
}

func toModelInfo(m *ent.LLMModel) dommodel.ModelInfo {
	provider := ""
	if m.Edges.Provider != nil {
		provider = string(m.Edges.Provider.Name)
	}
	return dommodel.ModelInfo{
		ID:          m.ID.String(),
		ModelName:   m.ModelName,
		DisplayName: m.DisplayName,
		Provider:    provider,
		IsDefault:   m.IsDefault,
	}
}
