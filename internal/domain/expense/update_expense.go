package expense

import (
	"context"
	"encoding/json"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// updateExpenseUndoData 撤销数据：修正账单时记录的旧值 + 归属成员（隔离用）。
type updateExpenseUndoData struct {
	ExpenseID string `json:"expense_id"`
	MemberID  string `json:"member_id"`
	Restore   struct {
		AmountCents int64  `json:"amount_cents"`
		Hint        string `json:"hint"`
		Category    string `json:"category"`
	} `json:"restore"`
}

// UpdateExpenseTool 是隐藏工具：不暴露给 LLM，仅承载 PATCH /expenses 的撤销恢复。
// 注册到 Registry 是为了让 Executor.Undo 能按名派发（ADR-004：写操作必须可撤销）。
type UpdateExpenseTool struct {
	svc Service
}

func NewUpdateExpenseTool(svc Service) *UpdateExpenseTool {
	return &UpdateExpenseTool{svc: svc}
}

func (t *UpdateExpenseTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "update_expense",
		Description: "内部工具：修正账单的撤销恢复，不暴露给 LLM",
		Risk:        tool.RiskHigh,
		Permission:  "expense.write",
		Module:      "expense",
		Idempotency: "restore-by-expense-id",
		Hidden:      true,
	}
}

func (t *UpdateExpenseTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return tool.Result{}, apperr.New(apperr.CodePermission, "内部工具，不可直接调用", nil)
}

// Undo 恢复修正前的旧值（幂等：重复撤销只是恢复成同样的旧值）。
func (t *UpdateExpenseTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d updateExpenseUndoData
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	if d.ExpenseID == "" {
		return apperr.New(apperr.CodeInvalidInput, "缺少账单 id", nil)
	}

	hint := d.Restore.Hint
	category := d.Restore.Category
	_, _, err := t.svc.UpdateExpense(ctx, d.ExpenseID, d.MemberID, UpdateExpenseCmd{
		AmountCents: &d.Restore.AmountCents,
		Hint:        &hint,
		Category:    &category,
	})
	return err
}
