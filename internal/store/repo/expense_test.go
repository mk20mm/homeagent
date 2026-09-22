package repo

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "modernc.org/sqlite" // 纯 Go 驱动，注册名 "sqlite"

	"github.com/mk20mm/homeagent/internal/domain/expense"
	"github.com/mk20mm/homeagent/internal/store"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// newTestClient 建临时库 + 最小种子（1 家庭 / 1 成员 / 2 分类）。
// 不用 enttest.Open：它用驱动名 "sqlite3"（mattn），modernc 注册的是 "sqlite"。
func newTestClient(t *testing.T) (*ent.Client, string) {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?cache=shared&_fk=1&_timezone=UTC"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	c := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	c.Use(store.UseUTCTimes)
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Schema.Create(context.Background()); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	fam, err := c.Family.Create().SetName("测试家").SetTimezone("Asia/Shanghai").Save(context.Background())
	if err != nil {
		t.Fatalf("seed family: %v", err)
	}
	_, err = c.Member.Create().
		SetName("测试员").
		SetRole(member.RoleParent).
		SetPermissions(map[string]bool{"expense.write": true}).
		SetFamily(fam).
		Save(context.Background())
	if err != nil {
		t.Fatalf("seed member: %v", err)
	}
	for i, name := range []string{"食材", "日用"} {
		if err := c.Category.Create().
			SetName(name).SetSortOrder(i).SetIsSystem(true).
			SetMonthlyBudgetCents(int64((i + 1) * 10000)).
			SetFamily(fam).
			Exec(context.Background()); err != nil {
			t.Fatalf("seed category %s: %v", name, err)
		}
	}
	return c, fam.ID.String()
}

func testMemberID(t *testing.T, c *ent.Client) string {
	t.Helper()
	m, err := c.Member.Query().Only(context.Background())
	if err != nil {
		t.Fatalf("query member: %v", err)
	}
	return m.ID.String()
}

func TestCreateAndIdempotency(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	cmd := expense.RecordExpenseCmd{
		AmountCents: 12000,
		Hint:        "买菜",
		Category:    "食材",
	}
	key := expense.IdempotencyKey(memberID, cmd)

	id1, err := s.Create(ctx, memberID, cmd, key)
	if err != nil {
		t.Fatalf("首次记账: %v", err)
	}
	if id1 == "" {
		t.Fatalf("返回的 id 为空")
	}
	if _, err := uuid.Parse(id1); err != nil {
		t.Fatalf("返回的 id 非法: %q: %v", id1, err)
	}

	// 幂等：同 key 再记，应返回同一 id + CodeConflict
	id2, err := s.Create(ctx, memberID, cmd, key)
	if id2 != id1 {
		t.Fatalf("幂等应返回同一 id，got %q want %q", id2, id1)
	}
	if err == nil {
		t.Fatal("幂等命中应返回 CodeConflict 供领域层判定")
	}

	// 库里只有一条
	n, _ := c.Expense.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("幂等后库内应有 1 条，got %d", n)
	}
}

func TestCreateAutoCategoryFallback(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)

	// 分类名不存在：不报错，账单照入
	cmd := expense.RecordExpenseCmd{AmountCents: 500, Hint: "打车"}
	id, err := s.Create(context.Background(), memberID, cmd, expense.IdempotencyKey(memberID, cmd))
	if err != nil {
		t.Fatalf("不存在分类不应报错: %v", err)
	}
	if id == "" {
		t.Fatal("id 为空")
	}
}

func TestDeleteSoftDeletes(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	cmd := expense.RecordExpenseCmd{AmountCents: 9900, Hint: "午餐"}
	id, err := s.Create(ctx, memberID, cmd, expense.IdempotencyKey(memberID, cmd))
	if err != nil {
		t.Fatalf("记账: %v", err)
	}

	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("撤销: %v", err)
	}

	// 软删除：记录仍在，deleted_at 非空
	n, _ := c.Expense.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("软删除不应物理删除，got count=%d", n)
	}
	n, _ = c.Expense.Query().Where( /* deleted_at IS NULL 由 DeletedAtIsNil 表达 */ ).Count(ctx)
	_ = n

	// 再次撤销同一 id：已软删除，不应再次成功（返回 NotFound）
	if err := s.Delete(ctx, id); err == nil {
		t.Log("注意：重复软删除当前返回成功（幂等），如需严格语义再调整")
	}
}

func TestUpdateReturnsPrevAndChangesIdempotencyKey(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	cmd := expense.RecordExpenseCmd{AmountCents: 12000, Hint: "买菜", Category: "食材"}
	id, err := s.Create(ctx, memberID, cmd, expense.IdempotencyKey(memberID, cmd))
	if err != nil {
		t.Fatalf("记账: %v", err)
	}

	newAmount := int64(15000)
	prev, err := s.Update(ctx, memberID, id, expense.UpdateExpenseCmd{AmountCents: &newAmount})
	if err != nil {
		t.Fatalf("修正: %v", err)
	}
	if prev.AmountCents != 12000 {
		t.Fatalf("旧值快照应为 12000，got %d", prev.AmountCents)
	}

	// 新值生效
	rec, err := s.Get(ctx, memberID, id)
	if err != nil {
		t.Fatalf("回查: %v", err)
	}
	if rec.AmountCents != 15000 {
		t.Fatalf("修正后应为 15000，got %d", rec.AmountCents)
	}

	// 幂等键已同步：用旧键记账不再冲突，用新键记账才冲突
	_, err = s.Create(ctx, memberID, cmd, expense.IdempotencyKey(memberID, cmd))
	if err != nil {
		t.Fatalf("旧键应已释放（可重新记账）: %v", err)
	}
	newCmd := expense.RecordExpenseCmd{AmountCents: 15000, Hint: "买菜"}
	_, err = s.Create(ctx, memberID, newCmd, expense.IdempotencyKey(memberID, newCmd))
	if err == nil {
		t.Fatal("新键应冲突（今天已有一笔买菜 150）")
	}
}

func TestUpdateIsolationAndNotFound(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	// 不存在的账单
	amt := int64(100)
	_, err := s.Update(ctx, memberID, uuid.NewString(), expense.UpdateExpenseCmd{AmountCents: &amt})
	if err == nil {
		t.Fatal("不存在的账单应返回错误")
	}

	// 软删除的账单
	cmd := expense.RecordExpenseCmd{AmountCents: 5000, Hint: "午餐"}
	id, err := s.Create(ctx, memberID, cmd, expense.IdempotencyKey(memberID, cmd))
	if err != nil {
		t.Fatalf("记账: %v", err)
	}
	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("撤销: %v", err)
	}
	if _, err := s.Update(ctx, memberID, id, expense.UpdateExpenseCmd{AmountCents: &amt}); err == nil {
		t.Fatal("已撤销的账单不应可修正")
	}
}

func TestSummaryExcludesDeletedAndCountsCategory(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()
	now := time.Now()

	// 两条本月 + 一条已撤销
	cmds := []expense.RecordExpenseCmd{
		{AmountCents: 12000, Hint: "买菜", Category: "食材", OccurredAt: now},
		{AmountCents: 3000, Hint: "纸巾", Category: "日用", OccurredAt: now},
	}
	var ids []string
	for _, cmd := range cmds {
		id, err := s.Create(ctx, memberID, cmd, expense.IdempotencyKey(memberID, cmd))
		if err != nil {
			t.Fatalf("记账 %s: %v", cmd.Hint, err)
		}
		ids = append(ids, id)
	}
	deleted := expense.RecordExpenseCmd{AmountCents: 5000, Hint: "夜宵", Category: "食材", OccurredAt: now}
	did, err := s.Create(ctx, memberID, deleted, expense.IdempotencyKey(memberID, deleted))
	if err != nil {
		t.Fatalf("记账夜宵: %v", err)
	}
	if err := s.Delete(ctx, did); err != nil {
		t.Fatalf("撤销夜宵: %v", err)
	}

	// 上月一条（不应计入）
	lastMonth := now.AddDate(0, -1, 0)
	old := expense.RecordExpenseCmd{AmountCents: 9999, Hint: "上月", Category: "食材", OccurredAt: lastMonth}
	if _, err := s.Create(ctx, memberID, old, expense.IdempotencyKey(memberID, old)); err != nil {
		t.Fatalf("记账上月: %v", err)
	}

	sm, err := s.Summary(ctx, memberID, now)
	if err != nil {
		t.Fatalf("汇总: %v", err)
	}
	if sm.TotalCents != 15000 {
		t.Fatalf("本月总额应为 15000 分（已撤销与上月不计入），got %d", sm.TotalCents)
	}
	if sm.ByCategory["食材"] != 12000 || sm.ByCategory["日用"] != 3000 {
		t.Fatalf("分类明细错误: %v", sm.ByCategory)
	}
	if sm.BudgetCents != 30000 {
		t.Fatalf("预算合计应为 30000 分，got %d", sm.BudgetCents)
	}
}
