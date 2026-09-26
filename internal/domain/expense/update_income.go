package expense

import (
	"context"
	"encoding/json"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// updateIncomeUndoData 撤销数据：修正收入时记录的旧值 + 归属成员（隔离用）。
type updateIncomeUndoData struct {
	IncomeID string `json:"income_id"`
	MemberID string `json:"member_id"`
	Restore  struct {
		AmountCents int64  `json:"amount_cents"`
		Hint        string `json:"hint"`
		Source      string `json:"source"`
	} `json:"restore"`
}

// UpdateIncomeTool 是隐藏工具：不暴露给 LLM，仅承载 PATCH /incomes 的撤销恢复。
type UpdateIncomeTool struct {
	svc Service
}

func NewUpdateIncomeTool(svc Service) *UpdateIncomeTool {
	return &UpdateIncomeTool{svc: svc}
}

func (t *UpdateIncomeTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "update_income",
		Description: "内部工具：修正收入的撤销恢复，不暴露给 LLM",
		Risk:        tool.RiskHigh,
		Permission:  "expense.write",
		Module:      "expense",
		Idempotency: "restore-by-income-id",
		Hidden:      true,
	}
}

func (t *UpdateIncomeTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return tool.Result{}, apperr.New(apperr.CodePermission, "内部工具，不可直接调用", nil)
}

// Undo 恢复修正前的旧值。
func (t *UpdateIncomeTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d updateIncomeUndoData
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	if d.IncomeID == "" {
		return apperr.New(apperr.CodeInvalidInput, "缺少收入 id", nil)
	}

	hint := d.Restore.Hint
	source := d.Restore.Source
	_, _, err := t.svc.UpdateIncome(ctx, d.IncomeID, d.MemberID, UpdateIncomeCmd{
		AmountCents: &d.Restore.AmountCents,
		Hint:        &hint,
		Source:      &source,
	})
	return err
}
