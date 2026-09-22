package repo

import (
	"context"
	"testing"
	"time"

	"github.com/mk20mm/homeagent/internal/domain/task"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// newTestExecutor 在库里加第二个成员（执行人），返回 (assignerID, executorID)。
func newTestExecutor(t *testing.T, c *ent.Client, familyID string) (string, string) {
	t.Helper()
	ctx := context.Background()
	assigner := testMemberID(t, c)
	fam, err := c.Family.Get(ctx, toUUID(familyID))
	if err != nil {
		t.Fatalf("get family: %v", err)
	}
	exe, err := c.Member.Create().
		SetName("执行人").
		SetRole(member.RoleAdult).
		SetPermissions(map[string]bool{"task.write": true}).
		SetFamily(fam).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed executor: %v", err)
	}
	return assigner, exe.ID.String()
}

func TestTaskAssignIdempotency(t *testing.T) {
	c, famID := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	assigner, _ := newTestExecutor(t, c, famID)
	ctx := context.Background()

	cmd := task.AssignTaskCmd{Title: "洗碗", AssigneeName: "执行人", Risk: "medium"}
	key := task.IdempotencyKey(assigner, cmd)

	id1, err := s.Assign(ctx, assigner, cmd, key)
	if err != nil {
		t.Fatalf("派发: %v", err)
	}
	if id1 == "" {
		t.Fatal("id 为空")
	}

	id2, err := s.Assign(ctx, assigner, cmd, key)
	if id2 != id1 {
		t.Fatalf("幂等应返回同一 id，got %q want %q", id2, id1)
	}
	if err == nil {
		t.Fatal("幂等命中应返回 CodeConflict")
	}

	n, _ := c.Task.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("库内应只有 1 条任务，got %d", n)
	}
}

func TestTaskCompleteStateAndGuard(t *testing.T) {
	c, famID := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	assigner, executor := newTestExecutor(t, c, famID)
	other := assigner // 派发人试图打卡别人指派的任务
	ctx := context.Background()

	cmd := task.AssignTaskCmd{Title: "倒垃圾", AssigneeName: "执行人"}
	id, err := s.Assign(ctx, assigner, cmd, task.IdempotencyKey(assigner, cmd))
	if err != nil {
		t.Fatalf("派发: %v", err)
	}

	// 非执行人打卡被拒
	if err := s.Complete(ctx, id, other); err == nil {
		t.Fatal("非执行人打卡应被拒绝")
	}

	// 执行人打卡：pending → in_progress
	if err := s.Complete(ctx, id, executor); err != nil {
		t.Fatalf("首次打卡: %v", err)
	}
	tk, _ := c.Task.Get(ctx, toUUID(id))
	if tk.Status != "in_progress" {
		t.Fatalf("应为 in_progress，got %q", tk.Status)
	}

	// 再次打卡：in_progress → done
	if err := s.Complete(ctx, id, executor); err != nil {
		t.Fatalf("完成打卡: %v", err)
	}
	tk, _ = c.Task.Get(ctx, toUUID(id))
	if tk.Status != "done" || tk.CompletedAt == nil {
		t.Fatalf("应为 done 且有完成时间，got %q", tk.Status)
	}

	// 重复打卡：冲突
	if err := s.Complete(ctx, id, executor); err == nil {
		t.Fatal("重复打卡应返回冲突")
	}

	// 撤销打卡：回退 pending
	if err := s.Uncomplete(ctx, id); err != nil {
		t.Fatalf("撤销打卡: %v", err)
	}
	tk, _ = c.Task.Get(ctx, toUUID(id))
	if tk.Status != "pending" || tk.CompletedAt != nil {
		t.Fatalf("撤销后应回退 pending，got %q", tk.Status)
	}
}

func TestListMyTasksIsolation(t *testing.T) {
	c, famID := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	assigner, executor := newTestExecutor(t, c, famID)
	ctx := context.Background()

	// 派给执行人 1 条，派给派发人自己 1 条
	cmd1 := task.AssignTaskCmd{Title: "扫地", AssigneeName: "执行人"}
	if _, err := s.Assign(ctx, assigner, cmd1, task.IdempotencyKey(assigner, cmd1)); err != nil {
		t.Fatalf("派发扫地: %v", err)
	}
	cmd2 := task.AssignTaskCmd{Title: "买菜", AssigneeName: "测试员"}
	if _, err := s.Assign(ctx, assigner, cmd2, task.IdempotencyKey(assigner, cmd2)); err != nil {
		t.Fatalf("派发买菜: %v", err)
	}

	// 执行人只看到自己的 1 条
	list, err := s.ListMyTasks(ctx, executor)
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if len(list) != 1 || list[0].Title != "扫地" {
		t.Fatalf("执行人应只见 1 条扫地，got %v", list)
	}

	// 派发人看到自己的 1 条
	list, err = s.ListMyTasks(ctx, assigner)
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if len(list) != 1 || list[0].Title != "买菜" {
		t.Fatalf("派发人应只见 1 条买菜，got %v", list)
	}
}

func TestTaskRemoveSoftDeletes(t *testing.T) {
	c, famID := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	assigner, _ := newTestExecutor(t, c, famID)
	ctx := context.Background()

	cmd := task.AssignTaskCmd{Title: "擦窗"}
	id, err := s.Assign(ctx, assigner, cmd, task.IdempotencyKey(assigner, cmd))
	if err != nil {
		t.Fatalf("派发: %v", err)
	}
	if err := s.Remove(ctx, id); err != nil {
		t.Fatalf("撤销派发: %v", err)
	}

	// 软删除：物理记录仍在
	n, _ := c.Task.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("软删除不应物理删除，got %d", n)
	}
	// ListMyTasks 不再返回
	list, _ := s.ListMyTasks(ctx, assigner)
	if len(list) != 0 {
		t.Fatalf("撤销后不应出现在待办里，got %v", list)
	}
	// GetTask 也不返回
	if _, err := s.GetTask(ctx, id); err == nil {
		t.Fatal("软删除后 GetTask 应返回 NotFound")
	}
}

// TestListDueSoonTimezone 时区回归：入库用 UTC（API 传 Z 后缀），查询用本地时间窗
// （调度器真实姿势 time.Now()）。修复前 from/to 未转 UTC，而 ent 的 SQLite 时间比较
// 是字符串比较，「+08:00」与「Z」混排会漏行——调度器扫不到到期任务。
func TestListDueSoonTimezone(t *testing.T) {
	c, famID := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	assigner, executor := newTestExecutor(t, c, famID)
	ctx := context.Background()

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("加载时区: %v", err)
	}

	now := time.Now()
	cmd := task.AssignTaskCmd{Title: "快到期", AssigneeName: "执行人", DueAt: now.Add(30 * time.Minute).UTC()}
	if _, err := s.Assign(ctx, assigner, cmd, task.IdempotencyKey(assigner, cmd)); err != nil {
		t.Fatalf("派发: %v", err)
	}

	from, to := now.In(loc), now.In(loc).Add(time.Hour)
	list, err := s.ListDueSoon(ctx, from, to)
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if len(list) != 1 || list[0].Title != "快到期" {
		t.Fatalf("本地时间窗应命中 1 条「快到期」，got %v", list)
	}
	if list[0].AssigneeID != executor || list[0].AssigneeName != "执行人" {
		t.Fatalf("应带出执行人，got id=%q name=%q", list[0].AssigneeID, list[0].AssigneeName)
	}
}

// TestListDueSoonWindowAndGuards 窗口半开 [from, to) 与排除规则：超窗/无执行人/已完成/软删除。
func TestListDueSoonWindowAndGuards(t *testing.T) {
	c, famID := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	assigner, executor := newTestExecutor(t, c, famID)
	ctx := context.Background()

	now := time.Now()
	mk := func(title string, due time.Time, assignee string) string {
		cmd := task.AssignTaskCmd{Title: title, DueAt: due}
		if assignee != "" {
			cmd.AssigneeName = assignee
		}
		id, err := s.Assign(ctx, assigner, cmd, task.IdempotencyKey(assigner, cmd))
		if err != nil {
			t.Fatalf("派发 %s: %v", title, err)
		}
		return id
	}

	mk("窗内", now.Add(30*time.Minute).UTC(), "执行人")
	mk("超窗", now.Add(2*time.Hour).UTC(), "执行人")
	mk("边界等于to", now.Add(time.Hour).UTC(), "执行人") // 半开区间，不含 to
	mk("待认领", now.Add(30*time.Minute).UTC(), "")     // 无执行人，不通知
	doneID := mk("已完成", now.Add(30*time.Minute).UTC(), "执行人")
	if err := s.Complete(ctx, doneID, executor); err != nil { // pending → in_progress
		t.Fatalf("首次打卡: %v", err)
	}
	if err := s.Complete(ctx, doneID, executor); err != nil { // in_progress → done
		t.Fatalf("完成打卡: %v", err)
	}
	removedID := mk("已撤回", now.Add(30*time.Minute).UTC(), "执行人")
	if err := s.Remove(ctx, removedID); err != nil {
		t.Fatalf("撤销派发: %v", err)
	}

	list, err := s.ListDueSoon(ctx, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if len(list) != 1 || list[0].Title != "窗内" {
		t.Fatalf("应只命中「窗内」1 条，got %d 条: %v", len(list), list)
	}
}

func TestGetTaskMapsFieldsAndMemberName(t *testing.T) {
	c, famID := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	assigner, executor := newTestExecutor(t, c, famID)
	ctx := context.Background()

	due := time.Now().Add(24 * time.Hour)
	cmd := task.AssignTaskCmd{Title: "洗碗", AssigneeName: "执行人", Risk: "high", DueAt: due}
	id, err := s.Assign(ctx, assigner, cmd, task.IdempotencyKey(assigner, cmd))
	if err != nil {
		t.Fatalf("派发: %v", err)
	}

	tk, err := s.GetTask(ctx, id)
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if tk.Title != "洗碗" || tk.Risk != "high" || tk.Status != "pending" {
		t.Fatalf("字段映射错误: %+v", tk)
	}
	if tk.AssigneeID != executor || tk.AssigneeName != "执行人" {
		t.Fatalf("指派人映射错误: id=%q name=%q", tk.AssigneeID, tk.AssigneeName)
	}
	if tk.DueAt == nil {
		t.Fatal("缺少截止时间")
	}

	// assignee_id → 名字（HTTP 边界用，保证与工具同一幂等键）
	name, err := s.MemberName(ctx, executor)
	if err != nil {
		t.Fatalf("MemberName: %v", err)
	}
	if name != "执行人" {
		t.Fatalf("MemberName 应为「执行人」，got %q", name)
	}
}
