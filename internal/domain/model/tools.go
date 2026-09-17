package model

import (
	"context"
	"encoding/json"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// ListModelsTool 低风险只读：已启用模型清单，全员可用。
type ListModelsTool struct {
	svc Service
}

func NewListModelsTool(svc Service) *ListModelsTool {
	return &ListModelsTool{svc: svc}
}

func (t *ListModelsTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "list_models",
		Description: "查可用的 AI 模型清单。用户想换模型或问有哪些模型时使用。只读。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {}
}`),
		Risk:       tool.RiskLow,
		Permission: "", // 全员可用
		Module:     "model",
	}
}

func (t *ListModelsTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	models, err := t.svc.ListModels(ctx)
	if err != nil {
		return tool.Result{}, err
	}
	items := make([]map[string]any, 0, len(models))
	names := ""
	for i, m := range models {
		items = append(items, map[string]any{
			"id":           m.ID,
			"model":        m.ModelName,
			"display_name": m.DisplayName,
			"provider":     m.Provider,
			"is_default":   m.IsDefault,
		})
		if i > 0 {
			names += "、"
		}
		names += m.DisplayName
	}
	card, _ := json.Marshal(map[string]any{
		"type":  "model_list",
		"items": items,
		"count": len(items),
	})
	summary := "暂无可用模型"
	if len(models) > 0 {
		summary = "可用模型：" + names
	}
	return tool.Result{Summary: summary, Card: card}, nil
}

// SwitchModelTool 低风险写：会话内切模型，工具集不变。
type SwitchModelTool struct {
	svc Service
}

func NewSwitchModelTool(svc Service) *SwitchModelTool {
	return &SwitchModelTool{svc: svc}
}

func (t *SwitchModelTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "switch_model",
		Description: "切换当前会话使用的 AI 模型。用户说『换成 GPT-4O/用 deepseek』时使用。切换后工具集不变。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["model"],
  "properties": {
    "model": {"type": "string", "description": "模型名或 id，如 gpt-4o / deepseek-chat"}
  }
}`),
		Risk:        tool.RiskLow,
		Permission:  "", // 全员可用
		Module:      "model",
		Idempotency: "conversation+model",
	}
}

func (t *SwitchModelTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[struct {
		Model string `json:"model"`
	}](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	// required 由 JSON Schema 校验（validateInput），这里防御性兜底
	if in.Model == "" {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "缺少 model 参数", nil)
	}

	convID := tool.ConversationIDFrom(ctx)
	if convID == "" {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "切换模型需要会话上下文", nil)
	}

	// 记旧模型供撤销
	prev, _ := t.svc.ConversationModel(ctx, convID)

	m, err := t.svc.SwitchModel(ctx, convID, in.Model)
	if err != nil {
		return tool.Result{}, err
	}

	undo, _ := json.Marshal(map[string]any{
		"conversation_id":  convID,
		"previous_model_id": prev.ID,
	})
	card, _ := json.Marshal(map[string]any{
		"type":    "model_switched",
		"model":   m.ModelName,
		"display": m.DisplayName,
	})

	return tool.Result{
		Summary:  "已切换到「" + m.DisplayName + "」",
		Card:     card,
		UndoData: undo,
	}, nil
}

// Undo 切回旧模型。
func (t *SwitchModelTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		ConversationID  string `json:"conversation_id"`
		PreviousModelID string `json:"previous_model_id"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	if d.PreviousModelID == "" {
		return nil // 原本无模型，无需回退（撤销=保持现状）
	}
	_, err := t.svc.SwitchModel(ctx, d.ConversationID, d.PreviousModelID)
	return err
}
