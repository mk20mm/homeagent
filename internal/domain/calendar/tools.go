package calendar

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// CreateEventInput 建事件参数（LLM 可调）。
type CreateEventInput struct {
	Title       string `json:"title"`
	StartAt     string `json:"start_at"`
	EndAt       string `json:"end_at,omitempty"`
	Repeat      string `json:"repeat,omitempty"`
	Interval    int    `json:"interval,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	IsLunar     bool   `json:"is_lunar,omitempty"`
}

// CreateEventTool 中风险写工具：建日程事件，可撤销。
type CreateEventTool struct {
	svc Service
}

func NewCreateEventTool(svc Service) *CreateEventTool {
	return &CreateEventTool{svc: svc}
}

func (t *CreateEventTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "create_event",
		Description: "在家庭日历建一个事件。『下周三下午三点开家长会/奶奶农历生日』时使用。repeat 留空=单次。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["title", "start_at"],
  "properties": {
    "title": {"type": "string", "description": "事件标题，如 开家长会"},
    "start_at": {"type": "string", "format": "date-time", "description": "开始时间，如 2026-09-30T15:00:00+08:00"},
    "end_at": {"type": "string", "format": "date-time", "description": "结束时间，可选"},
    "repeat": {"type": "string", "enum": ["once", "daily", "weekly", "monthly"], "description": "重复规则，默认 once"},
    "interval": {"type": "integer", "minimum": 1, "description": "重复间隔，默认 1（如每 2 周=weekly+2）"},
    "visibility": {"type": "string", "enum": ["family", "private"], "description": "可见性，默认 family 全家可见"},
    "description": {"type": "string", "description": "补充说明"},
    "location": {"type": "string", "description": "地点"},
    "is_lunar": {"type": "boolean", "description": "农历事件（生日/纪念日），默认 false"}
  }
}`),
		Risk:        tool.RiskMedium,
		Permission:  "calendar.write",
		Module:      "calendar",
		Idempotency: "member+title+start+repeat",
	}
}

func (t *CreateEventTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[CreateEventInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	start, err := parseEventTime(in.StartAt)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "开始时间格式有误", err)
	}
	cmd := CreateEventCmd{
		Title:       in.Title,
		Description: in.Description,
		StartAt:     start,
		Repeat:      Repeat(in.Repeat),
		Interval:    in.Interval,
		Visibility:  Visibility(in.Visibility),
		IsLunar:     in.IsLunar,
		Location:    in.Location,
	}
	if in.EndAt != "" {
		if end, err := parseEventTime(in.EndAt); err == nil {
			cmd.EndAt = &end
		}
	}

	ev, duplicated, err := t.svc.CreateEvent(ctx, memberID, cmd)
	if err != nil {
		return tool.Result{}, err
	}

	var undo json.RawMessage
	if !duplicated {
		undo, _ = json.Marshal(map[string]any{"event_id": ev.ID})
	}
	summary := "已建事件「" + ev.Title + "」" + FormatEventWhen(ev)
	if duplicated {
		summary = "「" + ev.Title + "」已存在，未重复创建"
	}
	card, _ := json.Marshal(map[string]any{
		"type":       "event",
		"event_id":   ev.ID,
		"title":      ev.Title,
		"start_at":   ev.StartAt.Format(time.RFC3339),
		"repeat":     string(ev.Repeat),
		"visibility": string(ev.Visibility),
		"owner_name": ev.OwnerName,
		"duplicated": duplicated,
	})

	return tool.Result{
		Summary:  summary,
		Card:     card,
		UndoData: undo,
	}, nil
}

// Undo 撤销建事件（软删除事件）。
func (t *CreateEventTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	return t.svc.DeleteEvent(ctx, d.EventID)
}

// ListEventsInput 查询日程参数。
type ListEventsInput struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
	Scope string `json:"scope,omitempty"`
}

// ListEventsTool 低风险读工具：查日程（展开重复事件 + 可见性过滤）。
type ListEventsTool struct {
	svc      Service
	familyOf FamilyLookup
}

// FamilyLookup 成员 → 家庭 id（工具层把 memberID 换成 familyID 供仓储查全家）。
type FamilyLookup interface {
	FamilyIDByMember(ctx context.Context, memberID string) (string, error)
}

func NewListEventsTool(svc Service, familyOf FamilyLookup) *ListEventsTool {
	return &ListEventsTool{svc: svc, familyOf: familyOf}
}

func (t *ListEventsTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "list_events",
		Description: "查家庭日程（按时间窗口）。『这周有什么安排/明天有什么事』时使用。重复事件自动展开。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "start": {"type": "string", "format": "date-time", "description": "窗口起点，默认今天 0 点"},
    "end": {"type": "string", "format": "date-time", "description": "窗口终点，默认今天 +30 天"},
    "scope": {"type": "string", "enum": ["my", "family"], "description": "my=只看我相关的，family=全家可见的"}
  }
}`),
		Risk:       tool.RiskLow,
		Permission: "calendar.read",
		Module:     "calendar",
	}
}

func (t *ListEventsTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[ListEventsInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	now := time.Now()
	start := StartOfToday(now)
	end := start.AddDate(0, 1, 0)
	if in.Start != "" {
		if t, err := parseEventTime(in.Start); err == nil {
			start = t
		}
	}
	if in.End != "" {
		if t, err := parseEventTime(in.End); err == nil {
			end = t
		}
	}

	scopeMy := in.Scope == "my"
	instances, err := t.svc.ListInstances(ctx, memberID, "", start, end, scopeMy)
	if err != nil {
		return tool.Result{}, err
	}
	// scope=family 时仓储需要 familyID；scope=my 用 memberID 直接查
	if !scopeMy {
		famID, err := t.familyOf.FamilyIDByMember(ctx, memberID)
		if err != nil {
			return tool.Result{}, err
		}
		instances, err = t.svc.ListInstances(ctx, memberID, famID, start, end, false)
		if err != nil {
			return tool.Result{}, err
		}
	}

	if len(instances) == 0 {
		return tool.Result{
			Summary: "该时间窗口内没有日程安排",
			Card:    mustMarshal(map[string]any{"type": "events", "items": []any{}, "empty": true}),
		}, nil
	}

	items := make([]map[string]any, 0, len(instances))
	for _, inst := range instances {
		if inst.Skipped {
			continue // 跳过的实例不展示
		}
		items = append(items, map[string]any{
			"event_id":     inst.ID,
			"title":        inst.Title,
			"start_at":     inst.StartAt.Format(time.RFC3339),
			"end_at":       FormatPtrTime(inst.EndAt),
			"repeat":       string(inst.Repeat),
			"owner_name":   inst.OwnerName,
			"is_lunar":     inst.IsLunar,
			"occurrence":   inst.Occurrence.Format(time.RFC3339),
			"is_exception": inst.IsException,
		})
	}
	summary := FormatInstanceSummary(items)
	return tool.Result{
		Summary: summary,
		Card:    mustMarshal(map[string]any{"type": "events", "items": items}),
	}, nil
}

// parseEventTime 解析 RFC3339 时间（带时区），失败返回错误。
func parseEventTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// StartOfToday 本地今天 0 点（调度与默认窗口用）。
func StartOfToday(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func FormatPtrTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// FormatEventWhen 事件时间的中文表述。
func FormatEventWhen(ev Event) string {
	return FormatTime(ev.StartAt)
}

// FormatTime 友好的中文时间。
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("01月02日 15:04")
}

// FormatInstanceSummary 列表的 LLM 摘要。
func FormatInstanceSummary(items []map[string]any) string {
	if len(items) == 0 {
		return "该时间窗口内没有日程安排"
	}
	if len(items) == 1 {
		return "有 1 项安排：" + firstTitle(items)
	}
	return "有 " + intToStr(len(items)) + " 项安排，最近的是「" + firstTitle(items) + "」"
}

func firstTitle(items []map[string]any) string {
	if len(items) == 0 {
		return ""
	}
	if t, ok := items[0]["title"].(string); ok {
		return t
	}
	return ""
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
