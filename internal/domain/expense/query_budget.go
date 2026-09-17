package expense

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// QueryBudgetInput 记账查询参数（全部可选，留空查本月）。
type QueryBudgetInput struct {
	Month string `json:"month,omitempty"` // YYYY-MM，空=本月
}

// QueryBudgetTool 低风险只读工具：查本月预算与已花。
type QueryBudgetTool struct {
	svc Service
}

func NewQueryBudgetTool(svc Service) *QueryBudgetTool {
	return &QueryBudgetTool{svc: svc}
}

func (t *QueryBudgetTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "query_budget",
		Description: "查本月预算与已花金额。用户问『这个月花了多少/预算还有多少』时使用。只读，不产生任何记录。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "month": {"type": "string", "description": "查询月份 YYYY-MM，留空查本月"}
  }
}`),
		Risk:       tool.RiskLow,
		Permission: "expense.read",
		Module:     "expense",
	}
}

func (t *QueryBudgetTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	// 参数仅做格式校验（month 参数暂留 schema，后端固定查本月）
	if _, err := tool.DecodeInput[QueryBudgetInput](input); err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	summary, err := t.svc.QueryBudget(ctx, memberID)
	if err != nil {
		return tool.Result{}, err
	}

	// 边界处分→元（领域层不见 float，AGENTS.md 不变量 2）
	card, _ := json.Marshal(map[string]any{
		"type":        "budget",
		"budget":      centsToYuan(summary.BudgetCents),
		"spent":       centsToYuan(summary.TotalCents),
		"remaining":   centsToYuan(summary.BudgetCents - summary.TotalCents),
		"by_category": centsMapToYuan(summary.ByCategory),
		"has_budget":  summary.BudgetCents > 0,
	})

	summary_text := "本月已花 ¥" + centsToYuan(summary.TotalCents)
	if summary.BudgetCents > 0 {
		remain := summary.BudgetCents - summary.TotalCents
		if remain >= 0 {
			summary_text += "，预算剩余 ¥" + centsToYuan(remain)
		} else {
			summary_text += "，已超预算 ¥" + centsToYuan(-remain)
		}
	}

	return tool.Result{
		Summary: summary_text,
		Card:    card,
	}, nil
}

// centsToYuan 分→元展示字符串（边界处转换，只用于展示；领域层仍用 int64 分）。
func centsToYuan(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	yuan := cents / 100
	fen := cents % 100
	s := strconv.FormatInt(yuan, 10) + "."
	if fen < 10 {
		s += "0"
	}
	s += strconv.FormatInt(fen, 10)
	if neg {
		s = "-" + s
	}
	return s
}

func centsMapToYuan(m map[string]int64) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = centsToYuan(v)
	}
	return out
}
