// Package expense 财务领域：记账工具（高风险 A3 级，必须可撤销）。
package expense

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// RecordExpenseInput 记账参数（inputSchema 由 Spec 返回给 LLM）。
type RecordExpenseInput struct {
	Amount      float64 `json:"amount"`             // 金额（分转元由前端/工具做）
	Hint        string  `json:"hint"`               // 原始表述提示，如 "买菜"
	Category    string  `json:"category,omitempty"` // 可选；空则由 LLM/规则归类
	OccurredAt  string  `json:"occurred_at,omitempty"`
}

// RecordExpenseTool 是高风险写工具：实现 tool.WriteTool（编译期强制 Undo）。
type RecordExpenseTool struct {
	svc Service
}

func NewRecordExpenseTool(svc Service) *RecordExpenseTool {
	return &RecordExpenseTool{svc: svc}
}

func (t *RecordExpenseTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "record_expense",
		Description: "记录一笔家庭支出。当用户说『花了X元/买菜X块』等消费信息时使用。何时不该用：非消费场景、金额未知时先追问。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["amount", "hint"],
  "properties": {
    "amount": {"type": "number", "minimum": 0.01, "description": "支出金额（元）"},
    "hint": {"type": "string", "description": "消费内容提示，如 买菜/午餐外卖"},
    "category": {"type": "string", "description": "可选预设分类：食材/日用/外卖/出行"}
  }
}`),
		Risk:        tool.RiskHigh,
		Permission:  "expense.write",
		Module:      "expense",
		Idempotency: "user+amount+hint+day",
	}
}

func (t *RecordExpenseTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[RecordExpenseInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)

	// 边界处元→分（领域层不见 float，AGENTS.md 不变量 2）
	cents := int64(in.Amount*100 + 0.5)
	if cents <= 0 {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "金额必须大于 0", nil)
	}

	occurredAt := time.Now()
	if in.OccurredAt != "" {
		if t, err := time.Parse(time.RFC3339, in.OccurredAt); err == nil {
			occurredAt = t
		}
	}

	id, category, duplicated, err := t.svc.RecordExpense(ctx, RecordExpenseCmd{
		AmountCents: cents,
		Hint:        in.Hint,
		Category:    in.Category,
		OccurredAt:  occurredAt,
	}, memberID)
	if err != nil {
		return tool.Result{}, err
	}

	summary := "已记账"
	if duplicated {
		// 幂等命中：未重复入库，明确告诉用户，避免"说成功却查不到"
		summary = "今天已记过这笔，未重复记账"
	}

	// 幂等命中时本次没有新写入，不生成撤销记录（撤销会误删早先那笔）
	var undo json.RawMessage
	if !duplicated {
		undo, _ = json.Marshal(map[string]any{"expense_id": id})
	}
	card, _ := json.Marshal(map[string]any{
		"type":       "expense",
		"amount":     in.Amount,
		"category":   category,
		"hint":       in.Hint,
		"time":       occurredAt.Format("15:04"),
		"duplicated": duplicated,
	})

	return tool.Result{
		Summary:  summary,
		Card:     card,
		UndoData: undo,
	}, nil
}

// Undo 软删除账单（ADR-004：写操作必须可撤销）。
func (t *RecordExpenseTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		ExpenseID string `json:"expense_id"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	return t.svc.DeleteExpense(ctx, d.ExpenseID)
}
