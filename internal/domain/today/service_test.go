package today

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mk20mm/homeagent/internal/domain/calendar"
	"github.com/mk20mm/homeagent/internal/domain/meal"
	"github.com/mk20mm/homeagent/internal/domain/task"
)

// ---- mock 读接口 ----

type mockTasks struct {
	items []task.Task
	err   error
}

func (m *mockTasks) ListMyTasks(ctx context.Context, memberID string) ([]task.Task, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

type mockEvents struct {
	items []calendar.Instance
	err   error
}

func (m *mockEvents) ListInstances(ctx context.Context, memberID, familyID string, start, end time.Time, scopeMy bool) ([]calendar.Instance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

type mockMeals struct {
	summary meal.MealSummary
	err     error
}

func (m *mockMeals) Summary(ctx context.Context, date time.Time) (meal.MealSummary, error) {
	if m.err != nil {
		return meal.MealSummary{}, m.err
	}
	return m.summary, nil
}

type mockNotes struct {
	items []NotificationItem
	err   error
}

func (m *mockNotes) ListNotifications(ctx context.Context, memberID string, limit int, unreadFirst bool) ([]NotificationItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

type mockFamily struct {
	id  string
	err error
}

func (m *mockFamily) FamilyIDByMember(ctx context.Context, memberID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.id, nil
}

func fixedNow() time.Time {
	return time.Date(2026, 9, 24, 14, 0, 0, 0, time.Local)
}

func newSvc(t *testing.T, tasks TaskReader, events CalendarReader, meals MealReader, notes NotificationReader) Service {
	t.Helper()
	return &service{
		tasks:  tasks,
		events: events,
		meals:  meals,
		notes:  notes,
		family: &mockFamily{id: "fam-1"},
		now:    fixedNow,
	}
}

// TestTodayAggregation 四块聚合 + 强行动区划分正确。
func TestTodayAggregation(t *testing.T) {
	due := fixedNow().Add(3 * time.Hour)
	readAt := fixedNow().Add(-time.Hour)

	svc := newSvc(t,
		&mockTasks{items: []task.Task{
			{ID: "t1", Title: "洗碗", Status: task.StatusPending, DueAt: &due},
		}},
		&mockEvents{items: []calendar.Instance{
			{Event: calendar.Event{ID: "e1", Title: "开家长会", StartAt: fixedNow().Add(2 * time.Hour), Visibility: calendar.VisFamily, OwnerName: "爸爸"}},
			{Event: calendar.Event{ID: "e2", Title: "跳过的", StartAt: fixedNow()}, Skipped: true},
		}},
		&mockMeals{summary: meal.MealSummary{
			AtHome:     []meal.MealMember{{ID: "me", Name: "爸爸"}},
			NotAtHome:  []meal.MealMember{{ID: "other", Name: "妈妈"}},
			Unreported: []meal.MealMember{{ID: "kid", Name: "孩子"}},
		}},
		&mockNotes{items: []NotificationItem{
			{ID: "n1", Type: "task_due", Title: "该洗碗了", ActionLabel: "去打卡", ActionPath: "/chores"},
			{ID: "n2", Type: "meal_gap", Title: "孩子还没报饭", ReadAt: &readAt},
		}},
	)

	out, err := svc.Today(context.Background(), "me")
	if err != nil {
		t.Fatal(err)
	}

	// 任务进强行动区
	if len(out.NeedAction.Tasks) != 1 || out.NeedAction.Tasks[0].Title != "洗碗" {
		t.Fatalf("强行动区任务错误: %+v", out.NeedAction.Tasks)
	}
	// 我已申报（在家），不进未申报
	if out.NeedAction.MealUnreported {
		t.Fatal("我已申报，不应标记未申报")
	}
	// 未读通知进强行动区，已读不进
	if len(out.NeedAction.Notifications) != 1 || out.NeedAction.Notifications[0].ID != "n1" {
		t.Fatalf("强行动区通知错误: %+v", out.NeedAction.Notifications)
	}

	// 日程：跳过的不出现
	if len(out.Schedule) != 1 || out.Schedule[0].Title != "开家长会" {
		t.Fatalf("今日安排错误: %+v", out.Schedule)
	}

	// 用餐三态原样保留
	if len(out.Meal.AtHome) != 1 || out.Meal.AtHome[0] != "爸爸" {
		t.Fatalf("在家名单错误: %v", out.Meal.AtHome)
	}
	if len(out.Meal.Unreported) != 1 || out.Meal.Unreported[0] != "孩子" {
		t.Fatalf("未申报名单错误: %v", out.Meal.Unreported)
	}
	if out.Meal.Mine != "at_home" {
		t.Fatalf("我的用餐状态应为 at_home，got %s", out.Meal.Mine)
	}

	// 动态区含已读+未读（有限条）
	if len(out.Activity) != 2 {
		t.Fatalf("动态条数错误: %d", len(out.Activity))
	}
	if out.Activity[1].Read != true {
		t.Fatal("已读通知应标记 read=true")
	}
}

// TestTodayMealUnreported 我未申报 → 进强行动区 + mine=unreported。
func TestTodayMealUnreported(t *testing.T) {
	svc := newSvc(t,
		&mockTasks{},
		&mockEvents{},
		&mockMeals{summary: meal.MealSummary{
			AtHome:     []meal.MealMember{{ID: "other", Name: "妈妈"}},
			Unreported: []meal.MealMember{{ID: "me", Name: "爸爸"}},
		}},
		&mockNotes{},
	)

	out, err := svc.Today(context.Background(), "me")
	if err != nil {
		t.Fatal(err)
	}
	if !out.NeedAction.MealUnreported {
		t.Fatal("我未申报，应进强行动区")
	}
	if out.Meal.Mine != "unreported" {
		t.Fatalf("mine 应为 unreported，got %s", out.Meal.Mine)
	}
	// 未申报 ≠ 不在家，不能转写
	if len(out.Meal.NotAtHome) != 0 {
		t.Fatalf("未申报不应被转写成不在家: %v", out.Meal.NotAtHome)
	}
}

// TestTodayPartialFailure 局部失败策略：一处失败不拖垮整个摘要（A-01-06）。
func TestTodayPartialFailure(t *testing.T) {
	svc := newSvc(t,
		&mockTasks{err: errors.New("任务库挂了")},
		&mockEvents{err: errors.New("日程库挂了")},
		&mockMeals{summary: meal.MealSummary{
			AtHome: []meal.MealMember{{ID: "me", Name: "爸爸"}},
		}},
		&mockNotes{},
	)

	out, err := svc.Today(context.Background(), "me")
	if err != nil {
		t.Fatalf("局部失败不应让整体报错: %v", err)
	}
	// 失败模块降级为空，成功模块照常
	if len(out.NeedAction.Tasks) != 0 {
		t.Fatalf("任务失败应降级为空: %+v", out.NeedAction.Tasks)
	}
	if len(out.Schedule) != 0 {
		t.Fatalf("日程失败应降级为空: %+v", out.Schedule)
	}
	if out.Meal.Mine != "at_home" {
		t.Fatalf("用餐应正常返回: %s", out.Meal.Mine)
	}
}

// TestTodayEmpty 空态：没有任务/日程/通知时对应区块为空（不报错）。
// 用餐语义：摘要成功返回但没我的申报记录 → 我确实是「未申报」，应进强行动区。
func TestTodayEmpty(t *testing.T) {
	svc := newSvc(t, &mockTasks{}, &mockEvents{}, &mockMeals{}, &mockNotes{})
	out, err := svc.Today(context.Background(), "me")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.NeedAction.Tasks) != 0 || len(out.Schedule) != 0 || len(out.Activity) != 0 {
		t.Fatalf("空态对应区块应全空: %+v", out)
	}
	if !out.NeedAction.MealUnreported {
		t.Fatal("无申报记录时应标记我未申报")
	}
	if out.Meal.Mine != "unreported" {
		t.Fatalf("mine 应为 unreported，got %s", out.Meal.Mine)
	}
}
