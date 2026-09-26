package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/calendar"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/calendarevent"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// newCalendarTestClient 临时库 + 2 成员（同一家庭，用于可见性测试）。
func newCalendarTestClient(t *testing.T) (*ent.Client, string, string) {
	t.Helper()
	c, famID := newTestClient(t)
	ctx := context.Background()

	fam, err := c.Family.Get(ctx, toUUID(famID))
	if err != nil {
		t.Fatalf("get family: %v", err)
	}
	b, err := c.Member.Create().
		SetName("成员乙").SetRole(member.RoleChild).
		SetPermissions(map[string]bool{"calendar.read": true}).
		SetFamily(fam).Save(ctx)
	if err != nil {
		t.Fatalf("seed member b: %v", err)
	}
	a, err := c.Member.Query().Where(member.NameEQ("测试员")).Only(ctx)
	if err != nil {
		t.Fatalf("query member a: %v", err)
	}
	return c, a.ID.String(), b.ID.String()
}

func TestEventCreateIdempotency(t *testing.T) {
	c, ownerID, _ := newCalendarTestClient(t)
	s := New(c)
	ctx := context.Background()

	start := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	cmd := calendar.CreateEventCmd{Title: "开家长会", StartAt: start, Repeat: calendar.RepeatWeekly}
	key := calendar.IdempotencyKey(ownerID, cmd)

	id1, err := s.CalendarCreate(ctx, ownerID, cmd, key)
	if err != nil {
		t.Fatalf("建事件: %v", err)
	}
	id2, err := s.CalendarCreate(ctx, ownerID, cmd, key)
	if err == nil {
		t.Fatal("幂等命中应返回错误")
	}
	if id2 != id1 {
		t.Fatalf("幂等应返回同一 id，got %q want %q", id2, id1)
	}
	n, _ := c.CalendarEvent.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("库内应只有 1 条事件，got %d", n)
	}
}

// TestExpandWeekly 验证重复展开 + 例外跳过 + 窗口边界。
func TestExpandWeekly(t *testing.T) {
	c, ownerID, _ := newCalendarTestClient(t)
	s := New(c)
	ctx := context.Background()

	start := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC) // 周四
	cmd := calendar.CreateEventCmd{Title: "周会", StartAt: start, Repeat: calendar.RepeatWeekly}
	_, err := s.CalendarCreate(ctx, ownerID, cmd, calendar.IdempotencyKey(ownerID, cmd))
	if err != nil {
		t.Fatalf("建事件: %v", err)
	}

	svc := calendar.NewService(s)
	winStart := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	winEnd := winStart.AddDate(0, 0, 21) // 三周窗口

	// 跳过第二周
	second := start.AddDate(0, 0, 7)
	if err := svc.SkipInstance(ctx, mustEventID(t, c, "周会"), second); err != nil {
		t.Fatalf("跳过实例: %v", err)
	}

	instances, err := svc.ListInstances(ctx, ownerID, "", winStart, winEnd, true)
	if err != nil {
		t.Fatalf("展开: %v", err)
	}
	if len(instances) != 3 { // 第1/3周，跳过第2周
		t.Fatalf("应展开 3 个实例（跳过 1 个），got %d", len(instances))
	}
	// 跳过的实例也在列表里（标记 skipped），供前端打删除线
	skipped := 0
	for _, inst := range instances {
		if inst.Skipped {
			skipped++
		}
	}
	if skipped != 1 {
		t.Fatalf("应有 1 个被跳过的实例，got %d", skipped)
	}
}

// TestVisibilityPrivate 验证 private 事件只有创建人可见。
func TestVisibilityPrivate(t *testing.T) {
	c, ownerA, ownerB := newCalendarTestClient(t)
	s := New(c)
	ctx := context.Background()

	start := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	cmd := calendar.CreateEventCmd{
		Title:      "私事",
		StartAt:    start,
		Visibility: calendar.VisPrivate,
	}
	if _, err := s.CalendarCreate(ctx, ownerA, cmd, calendar.IdempotencyKey(ownerA, cmd)); err != nil {
		t.Fatalf("建事件: %v", err)
	}
	cmd2 := calendar.CreateEventCmd{
		Title:      "家事",
		StartAt:    start,
		Visibility: calendar.VisFamily,
	}
	if _, err := s.CalendarCreate(ctx, ownerA, cmd2, calendar.IdempotencyKey(ownerA, cmd2)); err != nil {
		t.Fatalf("建事件: %v", err)
	}

	famID, _ := s.FamilyIDByMember(ctx, ownerA)
	svc := calendar.NewService(s)
	winStart := start.AddDate(0, 0, -1)
	winEnd := start.AddDate(0, 0, 1)

	// 成员乙（family 模式）看不见甲的 private 事件
	instances, err := svc.ListInstances(ctx, ownerB, famID, winStart, winEnd, false)
	if err != nil {
		t.Fatalf("展开: %v", err)
	}
	for _, inst := range instances {
		if inst.Title == "私事" {
			t.Fatal("private 事件不应被其他成员看到")
		}
	}
	if len(instances) != 1 {
		t.Fatalf("成员乙应只看到 1 个家庭事件，got %d", len(instances))
	}

	// 甲自己（my 模式）两个都能看到
	mine, err := svc.ListInstances(ctx, ownerA, "", winStart, winEnd, true)
	if err != nil {
		t.Fatalf("展开: %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("甲应看到自己的 2 个事件，got %d", len(mine))
	}
}

// TestExpandMonthly 验证每月重复跨月边界正确。
func TestExpandMonthly(t *testing.T) {
	c, ownerID, _ := newCalendarTestClient(t)
	s := New(c)
	ctx := context.Background()

	start := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC) // 1/31
	cmd := calendar.CreateEventCmd{Title: "月度对账", StartAt: start, Repeat: calendar.RepeatMonthly}
	_, err := s.CalendarCreate(ctx, ownerID, cmd, calendar.IdempotencyKey(ownerID, cmd))
	if err != nil {
		t.Fatalf("建事件: %v", err)
	}

	svc := calendar.NewService(s)
	winStart := start
	winEnd := start.AddDate(0, 6, 0) // 半年
	instances, err := svc.ListInstances(ctx, ownerID, "", winStart, winEnd, true)
	if err != nil {
		t.Fatalf("展开: %v", err)
	}
	if len(instances) == 0 {
		t.Fatal("每月重复半年内应至少有实例")
	}
	// 验证每个实例都在窗口内
	for _, inst := range instances {
		if inst.StartAt.Before(winStart) || !inst.StartAt.Before(winEnd) {
			t.Fatalf("实例 %s 超出窗口", inst.StartAt)
		}
	}
}

// TestDeleteEventSoftDelete 验证软删除后查询不可见。
func TestDeleteEventSoftDelete(t *testing.T) {
	c, ownerID, _ := newCalendarTestClient(t)
	s := New(c)
	ctx := context.Background()

	start := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	cmd := calendar.CreateEventCmd{Title: "一次性", StartAt: start}
	id, err := s.CalendarCreate(ctx, ownerID, cmd, calendar.IdempotencyKey(ownerID, cmd))
	if err != nil {
		t.Fatalf("建事件: %v", err)
	}
	if err := s.CalendarDelete(ctx, id); err != nil {
		t.Fatalf("删除: %v", err)
	}
	if _, err := s.GetEvent(ctx, id); err == nil {
		t.Fatal("软删除后 GetEvent 应失败")
	}
}

// TestGetEventDetailPermission 验证详情接口的可见性闸门（A-02-06 / A-05）。
// 无权成员查他人 private 事件 → 与不存在同码（防探测）。
func TestGetEventDetailPermission(t *testing.T) {
	c, ownerA, ownerB := newCalendarTestClient(t)
	s := New(c)
	ctx := context.Background()

	start := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	cmd := calendar.CreateEventCmd{
		Title:      "私事",
		StartAt:    start,
		Visibility: calendar.VisPrivate,
	}
	id, err := s.CalendarCreate(ctx, ownerA, cmd, calendar.IdempotencyKey(ownerA, cmd))
	if err != nil {
		t.Fatalf("建事件: %v", err)
	}

	svc := calendar.NewService(s)

	// 创建人可见
	if _, err := svc.GetEventDetail(ctx, id, ownerA); err != nil {
		t.Fatalf("创建人应能查看自己的 private 事件: %v", err)
	}
	// 他人不可见：返回 CodeNotFound（不是 Permission，避免泄露存在性）
	_, err = svc.GetEventDetail(ctx, id, ownerB)
	if err == nil {
		t.Fatal("他人查看 private 事件应失败")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFound {
		t.Fatalf("应返回 CodeNotFound（防探测），got %v", err)
	}
	// 不存在的事件也返回 NotFound（与无权同码）
	if _, err := svc.GetEventDetail(ctx, "00000000-0000-0000-0000-000000000000", ownerA); err == nil {
		t.Fatal("不存在的事件应失败")
	}
}

func mustEventID(t *testing.T, c *ent.Client, title string) string {
	t.Helper()
	e, err := c.CalendarEvent.Query().Where(calendarevent.TitleEQ(title)).Only(context.Background())
	if err != nil {
		t.Fatalf("查事件「%s」: %v", title, err)
	}
	return e.ID.String()
}
