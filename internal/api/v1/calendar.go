package v1

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/calendar"
)

// CalendarService 日程领域服务（handler 只用这几个方法，与工具走同一服务）。
type CalendarService interface {
	CreateEvent(ctx context.Context, ownerID string, cmd calendar.CreateEventCmd) (calendar.Event, bool, error)
	DeleteEvent(ctx context.Context, eventID string) error
	SkipInstance(ctx context.Context, eventID string, occurrence time.Time) error
	ListInstances(ctx context.Context, memberID string, familyID string, start, end time.Time, scopeMy bool) ([]calendar.Instance, error)
	GetEventDetail(ctx context.Context, eventID string, memberID string) (calendar.Detail, error)
}

// FamilyLookup 成员 → 家庭 id（查全家日程用）。
type FamilyLookup interface {
	FamilyIDByMember(ctx context.Context, memberID string) (string, error)
}

// eventInstance 对齐 openapi EventInstance schema。
type eventInstance struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	StartAt     string `json:"start_at"`
	EndAt       string `json:"end_at,omitempty"`
	Repeat      string `json:"repeat"`
	Visibility  string `json:"visibility"`
	OwnerID     string `json:"owner_id"`
	OwnerName   string `json:"owner_name,omitempty"`
	IsLunar     bool   `json:"is_lunar"`
	Location    string `json:"location,omitempty"`
	Occurrence  string `json:"occurrence"`
	IsException bool   `json:"is_exception"`
	Skipped     bool   `json:"skipped"`
}

// formatPtrTime 时间指针的可空序列化（nil → 空串）。
func formatPtrTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func eventItem(inst calendar.Instance) eventInstance {
	item := eventInstance{
		ID:          inst.ID,
		Title:       inst.Title,
		Description: inst.Description,
		StartAt:     inst.StartAt.Format(time.RFC3339),
		Repeat:      string(inst.Repeat),
		Visibility:  string(inst.Visibility),
		OwnerID:     inst.OwnerID,
		OwnerName:   inst.OwnerName,
		IsLunar:     inst.IsLunar,
		Location:    inst.Location,
		Occurrence:  inst.Occurrence.Format(time.RFC3339),
		IsException: inst.IsException,
		Skipped:     inst.Skipped,
	}
	if inst.EndAt != nil {
		item.EndAt = inst.EndAt.Format(time.RFC3339)
	}
	return item
}

// GetEvent GET /events/{eventId} —— 事件详情（深链目标，无权/不存在均 404 防探测）。
func GetEvent(svc CalendarService, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "get_event", tool.RiskLow, "calendar.read") {
			abortWith(c, apperr.New(apperr.CodePermission, "无日程查看权限", nil))
			return
		}

		eventID := c.Param("eventId")
		if eventID == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少事件 id", nil))
			return
		}

		d, err := svc.GetEventDetail(c.Request.Context(), eventID, memberID)
		if err != nil {
			// 无权与不存在同码（领域层已统一），这里直接透传
			abortWith(c, asAppErr(err))
			return
		}

		actions := make([]gin.H, 0, len(d.Actions))
		for _, a := range d.Actions {
			actions = append(actions, gin.H{"label": a.Label, "kind": a.Kind})
		}
		c.JSON(200, gin.H{
			"id":                d.ID,
			"title":             d.Title,
			"description":       d.Description,
			"start_at":          d.StartAt.Format(time.RFC3339),
			"end_at":            formatPtrTime(d.EndAt),
			"repeat":            string(d.Repeat),
			"interval":          d.Interval,
			"visibility":        string(d.Visibility),
			"owner_id":          d.OwnerID,
			"owner_name":        d.OwnerName,
			"is_lunar":          d.IsLunar,
			"location":          d.Location,
			"can_edit":          d.CanEdit,
			"can_cancel_series": d.CanCancelSeries,
			"actions":           actions,
		})
	}
}

// ListEvents GET /events —— 日程列表（重复事件展开为实例，可见性过滤）。
func ListEvents(svc CalendarService, familyOf FamilyLookup, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "list_events", tool.RiskLow, "calendar.read") {
			abortWith(c, apperr.New(apperr.CodePermission, "无日程查看权限", nil))
			return
		}

		ctx := c.Request.Context()
		now := time.Now()
		start := calendar.StartOfToday(now)
		end := start.AddDate(0, 1, 0)
		if s := c.Query("start"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				start = t
			}
		}
		if e := c.Query("end"); e != "" {
			if t, err := time.Parse(time.RFC3339, e); err == nil {
				end = t
			}
		}
		scopeMy := c.Query("scope") == "my"

		var famID string
		if !scopeMy {
			fam, err := familyOf.FamilyIDByMember(ctx, memberID)
			if err != nil {
				abortWith(c, asAppErr(err))
				return
			}
			famID = fam
		}

		instances, err := svc.ListInstances(ctx, memberID, famID, start, end, scopeMy)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		items := make([]eventInstance, 0, len(instances))
		for _, inst := range instances {
			if inst.Skipped {
				continue
			}
			items = append(items, eventItem(inst))
		}
		c.JSON(200, gin.H{"items": items})
	}
}

// CreateEvent POST /events —— 建事件（create_event 的 HTTP 出口，幂等 + 可撤销）。
func CreateEvent(svc CalendarService, undoWriter UndoWriter, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "create_event", tool.RiskMedium, "calendar.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无建日程权限", nil))
			return
		}

		var body struct {
			Title       *string `json:"title"`
			Description string  `json:"description"`
			StartAt     *string `json:"start_at"`
			EndAt       string  `json:"end_at"`
			Repeat      string  `json:"repeat"`
			Interval    int     `json:"interval"`
			Visibility  string  `json:"visibility"`
			IsLunar     bool    `json:"is_lunar"`
			Location    string  `json:"location"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数有误", err))
			return
		}
		if body.Title == nil || *body.Title == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "事件标题不能为空", nil))
			return
		}
		if body.StartAt == nil || *body.StartAt == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "开始时间不能为空", nil))
			return
		}

		start, err := time.Parse(time.RFC3339, *body.StartAt)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "开始时间格式有误", err))
			return
		}
		cmd := calendar.CreateEventCmd{
			Title:       *body.Title,
			Description: body.Description,
			StartAt:     start,
			Repeat:      calendar.Repeat(body.Repeat),
			Interval:    body.Interval,
			Visibility:  calendar.Visibility(body.Visibility),
			IsLunar:     body.IsLunar,
			Location:    body.Location,
		}
		if body.EndAt != "" {
			if end, err := time.Parse(time.RFC3339, body.EndAt); err == nil {
				cmd.EndAt = &end
			}
		}

		ctx := c.Request.Context()
		ev, duplicated, err := svc.CreateEvent(ctx, memberID, cmd)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		var undoID string
		if !duplicated {
			undoData, _ := json.Marshal(map[string]any{"event_id": ev.ID})
			uid, err := undoWriter.SaveUndo(ctx, memberID, "create_event", undoData)
			if err != nil {
				abortWith(c, asAppErr(err))
				return
			}
			undoID = uid
		}

		c.JSON(201, gin.H{
			"id":         ev.ID,
			"title":      ev.Title,
			"start_at":   ev.StartAt.Format(time.RFC3339),
			"repeat":     string(ev.Repeat),
			"undoable":   undoID != "",
			"undo_id":    undoID,
			"duplicated": duplicated,
		})
	}
}

// DeleteEvent DELETE /events/{eventId} —— 删除事件（整条系列，可撤销）。
func DeleteEvent(svc CalendarService, undoWriter UndoWriter, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "delete_event", tool.RiskHigh, "calendar.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无删除日程权限", nil))
			return
		}

		eventID := c.Param("eventId")
		if eventID == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少事件 id", nil))
			return
		}

		ctx := c.Request.Context()
		if err := svc.DeleteEvent(ctx, eventID); err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		undoData, _ := json.Marshal(map[string]any{"event_id": eventID})
		undoID, err := undoWriter.SaveUndo(ctx, memberID, "create_event", undoData)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(200, gin.H{"undo_id": undoID})
	}
}

// SkipEventInstance DELETE /events/{eventId}/instances/{occurrence} —— 删重复事件的单个实例。
func SkipEventInstance(svc CalendarService, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "skip_event_instance", tool.RiskMedium, "calendar.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无修改日程权限", nil))
			return
		}

		eventID := c.Param("eventId")
		occurrenceRaw := c.Param("occurrence")
		if eventID == "" || occurrenceRaw == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少事件 id 或实例时间", nil))
			return
		}
		occurrence, err := time.Parse(time.RFC3339, occurrenceRaw)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "实例时间格式有误", err))
			return
		}

		if err := svc.SkipInstance(c.Request.Context(), eventID, occurrence); err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(200, gin.H{"skipped": true})
	}
}
