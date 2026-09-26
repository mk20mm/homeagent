// Package today 是「今日行动摘要」聚合服务（PRD A-01）。
//
// 设计原则：
//   - 不建表、不缓存，只读各模块既有领域接口，单一事实源仍在各模块。
//   - 权限在服务端一次性裁剪：任务只看我的、日程按可见性、通知只看我的。
//   - 「未申报」原样保留，不转写成「不在家」；通知入库 ≠ 用户已看到。
package today

import (
	"context"
	"time"

	"github.com/mk20mm/homeagent/internal/domain/calendar"
	"github.com/mk20mm/homeagent/internal/domain/meal"
	"github.com/mk20mm/homeagent/internal/domain/task"
)

// 各模块读接口（只取 Today 需要的最小方法集）。
type TaskReader interface {
	ListMyTasks(ctx context.Context, memberID string) ([]task.Task, error)
}

type CalendarReader interface {
	ListInstances(ctx context.Context, memberID string, familyID string, start, end time.Time, scopeMy bool) ([]calendar.Instance, error)
}

type MealReader interface {
	Summary(ctx context.Context, date time.Time) (meal.MealSummary, error)
}

// NotificationReader 通知读接口（与 v1.NotificationStore 同形）。
type NotificationReader interface {
	ListNotifications(ctx context.Context, memberID string, limit int, unreadFirst bool) ([]NotificationItem, error)
}

// NotificationItem 通知视图（与 v1 同形，避免 today 反向依赖 api 层）。
type NotificationItem struct {
	ID          string
	Type        string
	Title       string
	Body        string
	RefType     string
	RefID       string
	ActionLabel string
	ActionPath  string
	ReadAt      *time.Time
	CreatedAt   time.Time
}

// FamilyLookup 由 member 查 family（与 v1 handler 同接口）。
type FamilyLookup interface {
	FamilyIDByMember(ctx context.Context, memberID string) (string, error)
}

// Service 今日摘要聚合服务。
type Service interface {
	Today(ctx context.Context, memberID string) (Summary, error)
}

type service struct {
	tasks  TaskReader
	events CalendarReader
	meals  MealReader
	notes  NotificationReader
	family FamilyLookup
	now    func() time.Time
}

func NewService(tasks TaskReader, events CalendarReader, meals MealReader, notes NotificationReader, family FamilyLookup) Service {
	return &service{tasks: tasks, events: events, meals: meals, notes: notes, family: family, now: time.Now}
}

// Summary 今日摘要（对齐 PRD A-01-01~04 的四块）。
type Summary struct {
	NeedAction NeedAction      `json:"need_action"`   // 需要我处理
	Schedule   []ScheduleItem  `json:"schedule"`      // 今日安排
	Meal       MealOverview    `json:"meal"`          // 家庭用餐概览 + 我的状态
	Activity   []ActivityItem  `json:"activity"`      // 代理人/通知动态（有限条）
}

// NeedAction 强行动区：只放真正需要我处理的事项（A-01-01）。
type NeedAction struct {
	Tasks          []TaskItem        `json:"tasks"`           // 我的未完成任务
	MealUnreported bool              `json:"meal_unreported"` // 我今天尚未申报用餐
	Notifications  []ActivityItem    `json:"notifications"`   // 未读且需回应的通知
}

type TaskItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	DueAt  string `json:"due_at"` // 空串=无截止
}

type ScheduleItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	StartAt    string `json:"start_at"`
	EndAt      string `json:"end_at"`
	OwnerName  string `json:"owner_name"`
	Visibility string `json:"visibility"`
}

// MealOverview 用餐概览（A-01-03）：三态原样保留。
type MealOverview struct {
	Date       string   `json:"date"`
	AtHome     []string `json:"at_home"`
	NotAtHome  []string `json:"not_at_home"`
	Unreported []string `json:"unreported"`
	// Mine: at_home / not_at_home / unreported（我自己的状态）
	Mine string `json:"mine"`
}

type ActivityItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	ActionLabel string `json:"action_label"`
	ActionPath  string `json:"action_path"`
	Read        bool   `json:"read"`
	CreatedAt   string `json:"created_at"`
}

const (
	maxTasks     = 10
	maxNotes     = 5  // 动态区有限条
	maxUnread    = 10
)

// Today 聚合今日摘要。
//
// 局部失败策略（A-01-06）：某个模块失败时，其余模块照常返回，
// 失败模块以空值 + 前端可识别的降级呈现，不让一处错误拖垮整个首页。
func (s *service) Today(ctx context.Context, memberID string) (Summary, error) {
	now := s.now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)

	out := Summary{
		Schedule: []ScheduleItem{},
		Activity: []ActivityItem{},
		NeedAction: NeedAction{
			Tasks:         []TaskItem{},
			Notifications: []ActivityItem{},
		},
	}

	// 1. 我的未完成任务（只看自己的，member 隔离由 task 领域保证）
	if tasks, err := s.tasks.ListMyTasks(ctx, memberID); err == nil {
		for i, t := range tasks {
			if i >= maxTasks {
				break
			}
			due := ""
			if t.DueAt != nil {
				due = t.DueAt.Format(time.RFC3339)
			}
			out.NeedAction.Tasks = append(out.NeedAction.Tasks, TaskItem{
				ID:     t.ID,
				Title:  t.Title,
				Status: string(t.Status),
				DueAt:  due,
			})
		}
	}

	// 2. 今日日程（可见性由 calendar 领域裁剪）
	famID := ""
	if fam, err := s.family.FamilyIDByMember(ctx, memberID); err == nil {
		famID = fam
	}
	if insts, err := s.events.ListInstances(ctx, memberID, famID, start, end, false); err == nil {
		for _, inst := range insts {
			if inst.Skipped {
				continue
			}
			out.Schedule = append(out.Schedule, ScheduleItem{
				ID:         inst.ID,
				Title:      inst.Title,
				StartAt:    inst.StartAt.Format(time.RFC3339),
				EndAt:      formatPtrTime(inst.EndAt),
				OwnerName:  inst.OwnerName,
				Visibility: string(inst.Visibility),
			})
		}
	}

	// 3. 用餐概览 + 我的状态
	if ms, err := s.meals.Summary(ctx, start); err == nil {
		out.Meal = MealOverview{
			Date:       start.Format("2006-01-02"),
			AtHome:     names(ms.AtHome),
			NotAtHome:  names(ms.NotAtHome),
			Unreported: names(ms.Unreported),
		}
		switch {
		case containsID(ms.AtHome, memberID):
			out.Meal.Mine = "at_home"
		case containsID(ms.NotAtHome, memberID):
			out.Meal.Mine = "not_at_home"
		default:
			out.Meal.Mine = "unreported"
			out.NeedAction.MealUnreported = true
		}
	}

	// 4. 通知动态（有限条，新的在前）+ 未读进强行动区
	if notes, err := s.notes.ListNotifications(ctx, memberID, maxNotes, true); err == nil {
		for _, n := range notes {
			item := ActivityItem{
				ID:          n.ID,
				Type:        n.Type,
				Title:       n.Title,
				Body:        n.Body,
				ActionLabel: n.ActionLabel,
				ActionPath:  n.ActionPath,
				Read:        n.ReadAt != nil,
				CreatedAt:   n.CreatedAt.Format(time.RFC3339),
			}
			out.Activity = append(out.Activity, item)
			if n.ReadAt == nil && len(out.NeedAction.Notifications) < maxUnread {
				out.NeedAction.Notifications = append(out.NeedAction.Notifications, item)
			}
		}
	}

	return out, nil
}

func names(ms []meal.MealMember) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return out
}

func containsID(ms []meal.MealMember, id string) bool {
	for _, m := range ms {
		if m.ID == id {
			return true
		}
	}
	return false
}

func formatPtrTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
