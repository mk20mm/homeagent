package repo

import (
	"context"
	"testing"
	"time"

	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/domain/expense"
)

func TestIncomeCreateAndIdempotency(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	cmd := expense.RecordIncomeCmd{
		AmountCents: 2000000, // 20000元
		Source:      "工资",
		Hint:        "9月工资",
	}
	key := expense.IncomeIdempotencyKey(memberID, cmd)

	id1, err := s.CreateIncome(ctx, memberID, cmd, key)
	if err != nil {
		t.Fatalf("首次记收入失败: %v", err)
	}
	if id1 == "" {
		t.Fatalf("返回的收入 id 为空")
	}

	// 幂等冲突测试
	id2, err := s.CreateIncome(ctx, memberID, cmd, key)
	if id2 != id1 {
		t.Fatalf("幂等应返回同一 id，got %q want %q", id2, id1)
	}
	if err == nil {
		t.Fatal("幂等命中应返回 CodeConflict 错误")
	}

	n, _ := c.Income.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("库内应只有 1 条收入记录，got %d", n)
	}
}

func TestIncomeUpdateAndDelete(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	cmd := expense.RecordIncomeCmd{
		AmountCents: 500000,
		Source:      "兼职",
		Hint:        "外包设计",
	}
	id, err := s.CreateIncome(ctx, memberID, cmd, expense.IncomeIdempotencyKey(memberID, cmd))
	if err != nil {
		t.Fatalf("记收入: %v", err)
	}

	newAmount := int64(600000)
	prev, err := s.UpdateIncome(ctx, memberID, id, expense.UpdateIncomeCmd{
		AmountCents: &newAmount,
	})
	if err != nil {
		t.Fatalf("修正收入: %v", err)
	}
	if prev.AmountCents != 500000 {
		t.Fatalf("旧值应为 500000，got %d", prev.AmountCents)
	}

	// 软删除
	if err := s.DeleteIncome(ctx, id); err != nil {
		t.Fatalf("撤销收入: %v", err)
	}

	// 再次查应查不到
	if _, err := s.GetIncome(ctx, memberID, id); err == nil {
		t.Fatal("已软删除的收入不应再被查出")
	}
}

func TestIncomeSummaryAndList(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	targetDate := time.Date(2026, 9, 15, 10, 0, 0, 0, time.Local)
	cmd1 := expense.RecordIncomeCmd{AmountCents: 1500000, Source: "工资", Hint: "9月基本工资", OccurredAt: targetDate}
	cmd2 := expense.RecordIncomeCmd{AmountCents: 200000, Source: "奖金", Hint: "季度绩效", OccurredAt: targetDate}

	_, err := s.CreateIncome(ctx, memberID, cmd1, expense.IncomeIdempotencyKey(memberID, cmd1))
	if err != nil {
		t.Fatalf("cmd1: %v", err)
	}
	_, err = s.CreateIncome(ctx, memberID, cmd2, expense.IncomeIdempotencyKey(memberID, cmd2))
	if err != nil {
		t.Fatalf("cmd2: %v", err)
	}

	// 汇总
	summary, err := s.IncomeSummary(ctx, memberID, targetDate, "month")
	if err != nil {
		t.Fatalf("IncomeSummary: %v", err)
	}
	if summary.TotalCents != 1700000 {
		t.Fatalf("总收入应为 1700000，got %d", summary.TotalCents)
	}
	if summary.BySource["工资"] != 1500000 || summary.BySource["奖金"] != 200000 {
		t.Fatalf("明细不符: %v", summary.BySource)
	}

	// 列表查询（按年月筛选）
	items, _, err := s.ListIncomes(ctx, memberID, v1.IncomeQuery{
		Year:  2026,
		Month: 9,
	})
	if err != nil {
		t.Fatalf("ListIncomes: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("2026年9月收入流水应为2条，got %d", len(items))
	}
}
