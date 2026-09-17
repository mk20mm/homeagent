package repo

import (
	"context"
	"testing"
	"time"

	dommeal "github.com/mk20mm/homeagent/internal/domain/meal"
)

func TestMealUpsertIdempotency(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()
	today := time.Now()

	// 首次报饭：在家
	if err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: true, Date: today}); err != nil {
		t.Fatalf("首次报饭: %v", err)
	}
	n, _ := c.MealReport.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("应有 1 条记录，got %d", n)
	}

	// 同日再报：改为不在家 → 更新而非新增
	if err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: false, Date: today}); err != nil {
		t.Fatalf("重复报饭: %v", err)
	}
	n, _ = c.MealReport.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("同日重复应为更新，库内仍 1 条，got %d", n)
	}
	sm, _ := s.DailySummary(ctx, today)
	if len(sm.AtHome) != 0 || len(sm.NotAtHome) != 1 {
		t.Fatalf("汇总应反映更新后的状态，got at=%v not=%v", sm.AtHome, sm.NotAtHome)
	}
}

func TestMealRemoveAndSummary(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()
	today := time.Now()

	if err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: true, Date: today}); err != nil {
		t.Fatalf("报饭: %v", err)
	}
	if err := s.RemoveReport(ctx, memberID, today); err != nil {
		t.Fatalf("撤销报饭: %v", err)
	}
	n, _ := c.MealReport.Query().Count(ctx)
	if n != 0 {
		t.Fatalf("撤销后库内应为空，got %d", n)
	}

	// 撤销不存在的记录
	if err := s.RemoveReport(ctx, memberID, today); err == nil {
		t.Fatal("重复撤销应返回 NotFound")
	}
}
