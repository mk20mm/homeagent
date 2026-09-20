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
	if _, err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: true, Date: today}); err != nil {
		t.Fatalf("首次报饭: %v", err)
	}
	n, _ := c.MealReport.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("应有 1 条记录，got %d", n)
	}

	// 同日再报：改为不在家 → 更新而非新增
	if _, err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: false, Date: today}); err != nil {
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

	if _, err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: true, Date: today}); err != nil {
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

func TestMealUpsertReturnsIDAndSummaryCarriesIDs(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()
	today := time.Now()

	id1, err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: true, Date: today})
	if err != nil {
		t.Fatalf("报饭: %v", err)
	}
	if id1 == "" {
		t.Fatal("应返回记录 id")
	}

	// 同日更新返回同一 id
	id2, err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: false, Date: today})
	if err != nil {
		t.Fatalf("重复报饭: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("更新应返回同一 id，got %q want %q", id2, id1)
	}

	// 汇总携带成员 id 与名字（HTTP 响应需要 member_id）
	sm, err := s.DailySummary(ctx, today)
	if err != nil {
		t.Fatalf("汇总: %v", err)
	}
	if len(sm.AtHome) != 0 || len(sm.NotAtHome) != 1 {
		t.Fatalf("汇总应反映更新，got at=%v not=%v", sm.AtHome, sm.NotAtHome)
	}
	if sm.NotAtHome[0].ID == "" || sm.NotAtHome[0].Name == "" {
		t.Fatalf("汇总应携带成员 id 与名字，got %+v", sm.NotAtHome[0])
	}
}

// 回归守卫：显式日期报饭与撤销必须用同一时区语义。
// 早先 time.Parse 得 UTC 0 点、入库用本地 0 点，DateEQ 扑空 → upsert 插重复 + 撤销删不掉。
func TestMealExplicitDateRoundTrip(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	day, err := dommeal.ParseDate("2026-09-18")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	want := time.Date(2026, 9, 18, 0, 0, 0, 0, time.Local)
	if !day.Equal(want) {
		t.Fatalf("ParseDate 应为本地 0 点，got %v want %v", day, want)
	}

	id1, err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: true, Date: day})
	if err != nil {
		t.Fatalf("报饭: %v", err)
	}
	// 同日重报应 upsert（返回同一 id），而不是插入新记录
	id2, err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: false, Date: day})
	if err != nil {
		t.Fatalf("重报: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("显式日期 upsert 应返回同一 id，got %q vs %q", id1, id2)
	}
	n, _ := c.MealReport.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("同日重复应只有 1 条记录，got %d", n)
	}

	// 撤销路径：从字符串解析回来必须能删掉记录
	again, _ := dommeal.ParseDate("2026-09-18")
	if err := s.RemoveReport(ctx, memberID, again); err != nil {
		t.Fatalf("撤销: %v", err)
	}
	n, _ = c.MealReport.Query().Count(ctx)
	if n != 0 {
		t.Fatalf("撤销后应为空，got %d", n)
	}
}

func TestDailySummaryShowsUnreportedGap(t *testing.T) {
	c, famID := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	_, executorID := newTestExecutor(t, c, famID) // 第二个成员「执行人」
	ctx := context.Background()
	today := time.Now()

	// 测试员报在家，执行人未申报
	if _, err := s.Upsert(ctx, memberID, dommeal.ReportCmd{AtHome: true, Date: today}); err != nil {
		t.Fatalf("报饭: %v", err)
	}

	sm, err := s.DailySummary(ctx, today)
	if err != nil {
		t.Fatalf("汇总: %v", err)
	}
	if len(sm.AtHome) != 1 || sm.AtHome[0].Name != "测试员" {
		t.Fatalf("在家应为测试员，got %+v", sm.AtHome)
	}

	// 未申报缺口必须显式列出执行人
	var found *dommeal.MealMember
	for i := range sm.Unreported {
		if sm.Unreported[i].Name == "执行人" {
			found = &sm.Unreported[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("未申报应包含「执行人」，got %+v", sm.Unreported)
	}
	if found.ID != executorID {
		t.Fatalf("未申报成员 id 应为 %q，got %q", executorID, found.ID)
	}
	// 已申报者不能出现在未申报里
	for _, m := range sm.Unreported {
		if m.Name == "测试员" {
			t.Fatal("已申报成员不应出现在未申报缺口中")
		}
	}
}
