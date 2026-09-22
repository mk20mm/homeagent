package scheduler

import (
	"context"
	"testing"
	"time"

	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/domain/meal"
	domtask "github.com/mk20mm/homeagent/internal/domain/task"
)

// --- 替身 ---

type stubNotifier struct {
	created map[string]int // memberID -> 计数
	fail    bool
}

func (n *stubNotifier) CreateNotification(_ context.Context, memberID string, _ v1.NotificationItem, _ time.Time) (bool, error) {
	if n.fail {
		return false, errBoom
	}
	if n.created == nil {
		n.created = map[string]int{}
	}
	n.created[memberID]++
	return true, nil
}

var errBoom = context.DeadlineExceeded

type stubTasks struct {
	list []domtask.Task
	err  error
}

func (s *stubTasks) ListDueSoon(_ context.Context, _, _ time.Time) ([]domtask.Task, error) {
	return s.list, s.err
}

type stubMeals struct {
	summary meal.MealSummary
	err     error
}

func (s *stubMeals) Summary(_ context.Context, _ time.Time) (meal.MealSummary, error) {
	return s.summary, s.err
}

// --- 任务到期源 ---

func TestTaskDueSourceNotifiesAssignee(t *testing.T) {
	due := time.Now().Add(30 * time.Minute)
	src := NewTaskDueSource(&stubTasks{list: []domtask.Task{
		{ID: "t1", Title: "洗碗", AssigneeID: "m1", DueAt: &due},
	}})
	items, memberIDs, _, err := src.Scan(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 条通知，实际 %d", len(items))
	}
	if memberIDs[0] != "m1" {
		t.Errorf("期望通知 m1，实际 %s", memberIDs[0])
	}
	if items[0].Type != "task_due" || items[0].RefID != "t1" {
		t.Errorf("通知内容错误: %+v", items[0])
	}
}

func TestTaskDueSourceSkipsUnassigned(t *testing.T) {
	due := time.Now().Add(30 * time.Minute)
	src := NewTaskDueSource(&stubTasks{list: []domtask.Task{
		{ID: "t1", Title: "待认领", DueAt: &due}, // 无 assignee
	}})
	items, _, _, err := src.Scan(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("待认领任务不应通知，实际 %d 条", len(items))
	}
}

// --- 报饭缺口源 ---

func TestMealGapSourceBeforeTriggerHour(t *testing.T) {
	src := NewMealGapSource(&stubMeals{summary: meal.MealSummary{
		Unreported: []meal.MealMember{{ID: "m1", Name: "爸爸"}},
	}})
	// 15:00，未到触发时间
	now := time.Date(2026, 9, 21, 15, 30, 0, 0, time.Local)
	items, _, _, err := src.Scan(context.Background(), now)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("16:00 前不应生成，实际 %d 条", len(items))
	}
}

func TestMealGapSourceNotifiesUnreported(t *testing.T) {
	src := NewMealGapSource(&stubMeals{summary: meal.MealSummary{
		Unreported: []meal.MealMember{
			{ID: "m1", Name: "爸爸"},
			{ID: "m2", Name: "妈妈"},
		},
	}})
	now := time.Date(2026, 9, 21, 16, 30, 0, 0, time.Local)
	items, memberIDs, _, err := src.Scan(context.Background(), now)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条通知，实际 %d", len(items))
	}
	if memberIDs[0] != "m1" || memberIDs[1] != "m2" {
		t.Errorf("目标成员错误: %v", memberIDs)
	}
	if items[0].Type != "meal_gap" {
		t.Errorf("期望 type=meal_gap，实际 %s", items[0].Type)
	}
}

func TestMealGapSourceAllReported(t *testing.T) {
	src := NewMealGapSource(&stubMeals{summary: meal.MealSummary{}})
	now := time.Date(2026, 9, 21, 17, 0, 0, 0, time.Local)
	items, _, _, err := src.Scan(context.Background(), now)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("全员已申报不应生成通知，实际 %d 条", len(items))
	}
}

// --- 调度器幂等 ---

func TestTickIsIdempotent(t *testing.T) {
	due := time.Now().Add(30 * time.Minute)
	taskSrc := NewTaskDueSource(&stubTasks{list: []domtask.Task{
		{ID: "t1", Title: "洗碗", AssigneeID: "m1", DueAt: &due},
	}})
	notif := &stubNotifier{}
	s := New(notif)
	s.AddSource(taskSrc)

	// 连续 tick 两次：CreateNotification 会被调两次（幂等由库层唯一键兜底），
	// 但调度器本身不因重复扫描跳过——库层冲突时 created=false。
	s.tick(context.Background())
	s.tick(context.Background())
	if notif.created["m1"] != 2 {
		t.Errorf("调度器应把每次扫描的结果都交给库层判幂等，实际调用了 %d 次", notif.created["m1"])
	}
}

func TestTickSkipsOnSourceError(t *testing.T) {
	src := NewTaskDueSource(&stubTasks{err: errBoom})
	notif := &stubNotifier{}
	s := New(notif)
	s.AddSource(src)

	s.tick(context.Background())
	if len(notif.created) != 0 {
		t.Errorf("源出错时不应创建任何通知")
	}
}
