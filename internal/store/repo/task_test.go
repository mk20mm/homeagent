package repo

import (
	"context"
	"testing"

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
}
