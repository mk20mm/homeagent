package meal

import (
	"context"
	"encoding/json"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// ReportMealInput 报饭参数。
type ReportMealInput struct {
	AtHome bool   `json:"at_home"`
	Note   string `json:"note,omitempty"`
	Date   string `json:"date,omitempty"` // YYYY-MM-DD，空=今天
}

// ReportMealTool 中风险写工具：报饭，按人+日幂等，可撤销。
type ReportMealTool struct {
	svc Service
}

func NewReportMealTool(svc Service) *ReportMealTool {
	return &ReportMealTool{svc: svc}
}

func (t *ReportMealTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "report_meal",
		Description: "报饭：申报今晚（或指定日期）是否在家吃饭。家人说『今晚不回来吃/在家吃』时使用。同一天重复上报为更新。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["at_home"],
  "properties": {
    "at_home": {"type": "boolean", "description": "是否在家用餐"},
    "note": {"type": "string", "description": "备注，如 加班/有客"},
    "date": {"type": "string", "format": "date", "description": "日期 YYYY-MM-DD，留空=今天"}
  }
}`),
		Risk:        tool.RiskMedium,
		Permission:  "meal.write",
		Module:      "meal",
		Idempotency: "member+date",
	}
}

func (t *ReportMealTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[ReportMealInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	cmd := ReportCmd{AtHome: in.AtHome, Note: in.Note}
	if in.Date != "" {
		if d, err := ParseDate(in.Date); err == nil {
			cmd.Date = d
		}
	}

	if _, err := t.svc.Report(ctx, memberID, cmd); err != nil {
		return tool.Result{}, err
	}

	// 服务端归一化日期（空 → 今日），撤销数据必须用真实日期，否则撤销会扑空
	day := cmd.Date
	if day.IsZero() {
		day = today()
	}

	dateLabel := "今晚"
	if !cmd.Date.IsZero() {
		dateLabel = cmd.Date.Format("01-02")
	}
	choice := "在家吃"
	if !in.AtHome {
		choice = "不回家吃"
	}
	undo, _ := json.Marshal(map[string]any{"date": day.Format("2006-01-02")})
	card, _ := json.Marshal(map[string]any{
		"type":    "meal",
		"at_home": in.AtHome,
		"note":    in.Note,
		"date":    dateLabel,
	})

	return tool.Result{
		Summary:  "已报饭：" + dateLabel + choice,
		Card:     card,
		UndoData: undo,
	}, nil
}

// Undo 撤销当日报饭。
func (t *ReportMealTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		Date string `json:"date"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	date, err := ParseDate(d.Date)
	if err != nil {
		return apperr.New(apperr.CodeInvalidInput, "日期格式错误", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	return t.svc.Cancel(ctx, memberID, date)
}
