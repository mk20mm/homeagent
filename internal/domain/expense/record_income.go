// Package expense 财务领域：记收入工具（高风险 A3 级，必须可撤销）。
package expense

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// RecordIncomeInput 记收入参数（inputSchema 由 Spec 返回给 LLM）。
type RecordIncomeInput struct {
	Amount     float64 `json:"amount"`             // 金额（元）
	Hint       string  `json:"hint"`               // 原始表述提示，如 "9月工资"
	Source     string  `json:"source,omitempty"`   // 可选；空则由规则自动归类
	OccurredAt string  `json:"occurred_at,omitempty"`
}

// RecordIncomeTool 是高风险写工具：实现 tool.WriteTool（编译期强制 Undo）。
type RecordIncomeTool struct {
	svc Service
}

func NewRecordIncomeTool(svc Service) *RecordIncomeTool {
	return &RecordIncomeTool{svc: svc}
}

func (t *RecordIncomeTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "record_income",
		Description: "记录一笔家庭收入。当用户说『发了工资X元/收到奖金X元/报销到账X元/兼职收入X元』等入账信息时使用。何时不该用：支出消费场景、金额未知时先追问。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["amount", "hint"],
  "properties": {
    "amount": {"type": "number", "minimum": 0.01, "description": "收入金额（元）"},
    "hint": {"type": "string", "description": "收入内容提示，如 9月工资/兼职外包/医疗报销"},
    "source": {"type": "string", "description": "可选预设来源：工资/奖金/兼职/报销/退款/红包/利息/其他"}
  }
}`),
		Risk:        tool.RiskHigh,
		Permission:  "expense.write",
		Module:      "expense",
		Idempotency: "user+amount+source+hint+day",
	}
}

func (t *RecordIncomeTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[RecordIncomeInput](input)
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
		if pt, err := time.Parse(time.RFC3339, in.OccurredAt); err == nil {
			occurredAt = pt
		}
	}

	id, source, duplicated, err := t.svc.RecordIncome(ctx, RecordIncomeCmd{
		AmountCents: cents,
		Hint:        in.Hint,
		Source:      in.Source,
		OccurredAt:  occurredAt,
	}, memberID)
	if err != nil {
		return tool.Result{}, err
	}

	summary := "已记收入"
	if duplicated {
		summary = "今天已记过这笔收入，未重复记入"
	}

	var undo json.RawMessage
	if !duplicated {
		undo, _ = json.Marshal(map[string]any{"income_id": id})
	}
	card, _ := json.Marshal(map[string]any{
		"type":       "income",
		"income_id":  id,
		"amount":     in.Amount,
		"source":     source,
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

// Undo 软删除收入记录（ADR-004：写操作必须可撤销）。
func (t *RecordIncomeTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		IncomeID string `json:"income_id"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	return t.svc.DeleteIncome(ctx, d.IncomeID)
}
