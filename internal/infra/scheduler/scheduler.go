// Package scheduler 主动服务调度器（ADR-006）。
//
// robfig/cron 每 1 分钟 tick → 扫描到期项 → 查幂等键 → 静默生成通知。
// 阶段 A 两个源：任务到期（due_at 前 1 小时）与报饭缺口（16:00 后未申报）。
// 阶段 B 新增源（周期账单/日程提醒/周报）只需加一个 Source，不动核心。
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/domain/meal"
	domtask "github.com/mk20mm/homeagent/internal/domain/task"
)

// Notifier 通知写入能力（repo 实现）。
type Notifier interface {
	CreateNotification(ctx context.Context, memberID string, n v1.NotificationItem, scheduledAt time.Time) (created bool, err error)
}

// TaskScanner 任务扫描（repo 实现）。
type TaskScanner interface {
	ListDueSoon(ctx context.Context, from, to time.Time) ([]domtask.Task, error)
}

// MealSummaryProvider 报饭汇总（领域服务）。
type MealSummaryProvider interface {
	Summary(ctx context.Context, date time.Time) (meal.MealSummary, error)
}

// Source 通知源：每次 tick 决定要生成哪些通知（阶段 B 扩展点）。
// 返回通知项与其对应的目标成员 id（按位置对齐）。
type Source interface {
	Name() string
	Scan(ctx context.Context, now time.Time) (items []v1.NotificationItem, memberIDs []string, schedules []time.Time, err error)
}

// Scheduler cron 调度器。
type Scheduler struct {
	cron    *cron.Cron
	sources []Source
	notifier Notifier
}

func New(notifier Notifier) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds(), cron.WithLocation(time.Local)),
		notifier: notifier,
	}
}

// AddSource 注册通知源（热插拔，阶段 B 直接加）。
func (s *Scheduler) AddSource(src Source) {
	s.sources = append(s.sources, src)
}

// Start 启动调度器（每分钟 tick）。
func (s *Scheduler) Start() {
	s.cron.AddFunc("* * * * * *", func() {
		s.tick(context.Background())
	})
	s.cron.Start()
	slog.Info("scheduler started", "sources", len(s.sources))
}

// Stop 优雅停止。
func (s *Scheduler) Stop() {
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()
}

// tick 单次扫描（导出供测试与手动触发）。
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()
	for _, src := range s.sources {
		items, memberIDs, schedules, err := src.Scan(ctx, now)
		if err != nil {
			slog.Error("scheduler source failed", "source", src.Name(), "err", err)
			continue
		}
		for i, item := range items {
			sched := now
			if i < len(schedules) {
				sched = schedules[i]
			}
			memberID := ""
			if i < len(memberIDs) {
				memberID = memberIDs[i]
			}
			if memberID == "" {
				continue
			}
			created, err := s.notifier.CreateNotification(ctx, memberID, item, sched)
			if err != nil {
				slog.Error("create notification failed", "source", src.Name(), "err", err)
				continue
			}
			if created {
				slog.Info("notification created", "source", src.Name(), "type", item.Type, "title", item.Title)
			}
		}
	}
}

// --- 内置源：任务到期 ---

// TaskDueSource 扫描「未来 1 小时内到期且未完成」的任务，通知执行人。
// 提前量 1 小时：太早提醒会被忘，太晚来不及安排。
type TaskDueSource struct {
	tasks TaskScanner
}

func NewTaskDueSource(tasks TaskScanner) *TaskDueSource {
	return &TaskDueSource{tasks: tasks}
}

func (s *TaskDueSource) Name() string { return "task_due" }

func (s *TaskDueSource) Scan(ctx context.Context, now time.Time) ([]v1.NotificationItem, []string, []time.Time, error) {
	from := now
	to := now.Add(time.Hour)
	list, err := s.tasks.ListDueSoon(ctx, from, to)
	if err != nil {
		return nil, nil, nil, err
	}
	items := make([]v1.NotificationItem, 0, len(list))
	memberIDs := make([]string, 0, len(list))
	schedules := make([]time.Time, 0, len(list))
	for _, t := range list {
		if t.AssigneeID == "" {
			continue // 待认领任务无责任人，不通知
		}
		due := ""
		if t.DueAt != nil {
			due = t.DueAt.Format("15:04")
		}
		items = append(items, v1.NotificationItem{
			Type:        "task_due",
			Title:       fmt.Sprintf("任务快到期：%s", t.Title),
			Body:        fmt.Sprintf("「%s」将于 %s 到期，记得完成。", t.Title, due),
			RefType:     "task",
			RefID:       t.ID,
			ActionLabel: "去打卡",
			ActionPath:  "/chores",
		})
		memberIDs = append(memberIDs, t.AssigneeID)
		// 幂等键的 scheduled_at 用任务到期时间（同一任务这一小时内只一条）
		sched := now
		if t.DueAt != nil {
			sched = *t.DueAt
		}
		schedules = append(schedules, sched)
	}
	return items, memberIDs, schedules, nil
}

// --- 内置源：报饭缺口 ---

// MealGapSource 每天 16:00 后扫描当日未申报报饭的成员，逐人提醒。
//
// 通知对象是「未申报者本人」：做饭人需要的是人数，而人数由各人的申报决定——
// 催未申报者「今晚还报饭吗」比通知做饭人「有人没报」更接近问题根（ADR-006 §决策 5）。
// 做饭人在 /meal 页能直接看到缺口，不需要额外通知。
type MealGapSource struct {
	meals MealSummaryProvider
	// 16:00 后才生成（早了报饭还没开始；调度器每分钟 tick，16:00 后的第一次 tick 生效）
	TriggerHour int
}

func NewMealGapSource(meals MealSummaryProvider) *MealGapSource {
	return &MealGapSource{meals: meals, TriggerHour: 16}
}

func (s *MealGapSource) Name() string { return "meal_gap" }

func (s *MealGapSource) Scan(ctx context.Context, now time.Time) ([]v1.NotificationItem, []string, []time.Time, error) {
	if now.Hour() < s.TriggerHour {
		return nil, nil, nil, nil // 16:00 前不生成
	}
	summary, err := s.meals.Summary(ctx, now)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(summary.Unreported) == 0 {
		return nil, nil, nil, nil // 全员已申报，无缺口
	}
	// 幂等键的 scheduled_at 用当日 16:00（同一天只一条）
	sched := time.Date(now.Year(), now.Month(), now.Day(), s.TriggerHour, 0, 0, 0, now.Location())
	items := make([]v1.NotificationItem, 0, len(summary.Unreported))
	memberIDs := make([]string, 0, len(summary.Unreported))
	schedules := make([]time.Time, 0, len(summary.Unreported))
	for _, m := range summary.Unreported {
		items = append(items, v1.NotificationItem{
			Type:        "meal_gap",
			Title:       "今晚报饭了吗？",
			Body:        fmt.Sprintf("今晚（%s）还没报饭，做饭人需要确认人数。", now.Format("01-02")),
			RefType:     "meal_report",
			RefID:       now.Format("2006-01-02"),
			ActionLabel: "去报饭",
			ActionPath:  "/meal",
		})
		memberIDs = append(memberIDs, m.ID)
		schedules = append(schedules, sched)
	}
	return items, memberIDs, schedules, nil
}
