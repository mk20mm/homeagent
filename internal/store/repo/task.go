package repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/apperr"
	domtask "github.com/mk20mm/homeagent/internal/domain/task"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
	"github.com/mk20mm/homeagent/internal/store/ent/task"
)

// 编译期接口实现检查。
var _ domtask.TaskRepo = (*Store)(nil)

// Assign 派发任务。幂等冲突（unique idempotency_key）返回已存在 id + CodeConflict。
func (s *Store) Assign(ctx context.Context, assignerID string, cmd domtask.AssignTaskCmd, idempotencyKey string) (string, error) {
	assigner := toUUID(assignerID)
	risk := cmd.Risk
	if risk == "" {
		risk = "medium" // schema enum 必填，空值非法
	}
	b := s.db.Task.Create().
		SetTitle(cmd.Title).
		SetRisk(task.Risk(risk)).
		SetIdempotencyKey(idempotencyKey).
		SetAssignerID(assigner)

	if cmd.Description != "" {
		b.SetDescription(cmd.Description)
	}
	if !cmd.DueAt.IsZero() {
		b.SetDueAt(cmd.DueAt)
	}
	if cmd.AssigneeName != "" {
		executor, err := s.db.Member.Query().
			Where(member.NameEQ(cmd.AssigneeName)).
			Only(ctx)
		if err != nil {
			return "", apperr.New(apperr.CodeNotFound, "执行人「"+cmd.AssigneeName+"」不存在", err)
		}
		b.SetAssignee(executor)
	}

	saved, err := b.Save(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			exist, qerr := s.db.Task.Query().
				Where(task.IdempotencyKeyEQ(idempotencyKey), task.DeletedAtIsNil()).
				Only(ctx)
			if qerr != nil || exist == nil {
				return "", apperr.New(apperr.CodeConflict, "任务已存在但回查失败", err)
			}
			return exist.ID.String(), apperr.New(apperr.CodeConflict, "任务已存在", nil)
		}
		return "", apperr.New(apperr.CodeInternal, "派发任务失败", err)
	}
	return saved.ID.String(), nil
}

// Complete 打卡：状态机迁移 + 幂等（已 done 返回冲突）。memberID 须为执行人。
func (s *Store) Complete(ctx context.Context, taskID, memberID string) error {
	t, err := s.db.Task.Query().
		Where(task.IDEQ(toUUID(taskID))).
		WithAssignee().
		Only(ctx)
	if err != nil {
		return apperr.New(apperr.CodeNotFound, "任务不存在", err)
	}
	// 待认领任务允许任何人打卡；已指派只能执行人打卡
	if t.Edges.Assignee != nil && t.Edges.Assignee.ID != toUUID(memberID) {
		return apperr.New(apperr.CodePermission, "这不是指派给你的任务", nil)
	}
	if t.Status == task.StatusDone {
		return apperr.New(apperr.CodeConflict, "任务已完成，勿重复打卡", nil)
	}

	status := task.StatusInProgress
	if t.Status == task.StatusInProgress {
		status = task.StatusDone
	}
	update := s.db.Task.UpdateOneID(t.ID).SetStatus(status)
	if status == task.StatusDone {
		update.SetCompletedAt(time.Now())
	}
	if err := update.Exec(ctx); err != nil {
		return apperr.New(apperr.CodeInternal, "打卡失败", err)
	}
	return nil
}

// Uncomplete 撤销误打卡：回退 pending + 清完成时间。
func (s *Store) Uncomplete(ctx context.Context, taskID string) error {
	if err := s.db.Task.UpdateOneID(toUUID(taskID)).
		SetStatus(task.StatusPending).
		ClearCompletedAt().
		Exec(ctx); err != nil {
		return apperr.New(apperr.CodeInternal, "撤销打卡失败", err)
	}
	return nil
}

// ListMyTasks 我的待办：执行人是我且未完成（待认领的不返回）。
func (s *Store) ListMyTasks(ctx context.Context, memberID string) ([]domtask.Task, error) {
	list, err := s.db.Task.Query().
		Where(
			task.HasAssigneeWith(member.IDEQ(toUUID(memberID))),
			task.StatusNEQ(task.StatusDone),
			task.DeletedAtIsNil(),
		).
		WithAssignee().
		Order(ent.Asc(task.FieldDueAt), ent.Desc(task.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询任务失败", err)
	}
	out := make([]domtask.Task, 0, len(list))
	for _, t := range list {
		out = append(out, mapTask(t))
	}
	return out, nil
}

// ListDueSoon 未完成且在时间窗内到期的任务（调度器扫描用）。
// 窗口 [from, to)：from 通常为当前时间，to 为提醒提前量（如 1 小时后）。
// 待认领任务（无 assignee）不通知——没有具体责任人。
//
// 注意：ent 的 SQLite 时间比较是按 time.Time 的字符串格式做的，
// from/to 与 due_at 必须同一时区。统一转 UTC（存入时 curl 传 Z 后缀）。
func (s *Store) ListDueSoon(ctx context.Context, from, to time.Time) ([]domtask.Task, error) {
	list, err := s.db.Task.Query().
		Where(
			task.StatusNEQ(task.StatusDone),
			task.DeletedAtIsNil(),
			task.HasAssignee(),
		task.DueAtGTE(from.UTC()),
		task.DueAtLT(to.UTC()),
		).
		WithAssignee().
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询到期任务失败", err)
	}
	out := make([]domtask.Task, 0, len(list))
	for _, t := range list {
		out = append(out, mapTask(t))
	}
	return out, nil
}

// GetTask 单任务查询（软删除过滤；家庭可见读）。
func (s *Store) GetTask(ctx context.Context, taskID string) (domtask.Task, error) {
	t, err := s.db.Task.Query().
		Where(task.IDEQ(toUUID(taskID)), task.DeletedAtIsNil()).
		WithAssignee().
		Only(ctx)
	if err != nil {
		return domtask.Task{}, apperr.New(apperr.CodeNotFound, "任务不存在", err)
	}
	return mapTask(t), nil
}

// mapTask ent 行 → 领域视图（assignee 边须已加载）。
func mapTask(t *ent.Task) domtask.Task {
	tk := domtask.Task{
		ID:          t.ID.String(),
		Title:       t.Title,
		Description: t.Description,
		Risk:        string(t.Risk),
		Status:      domtask.TaskStatus(t.Status),
		Points:      t.Points,
	}
	if t.Edges.Assignee != nil {
		tk.AssigneeID = t.Edges.Assignee.ID.String()
		tk.AssigneeName = t.Edges.Assignee.Name
	}
	if t.DueAt != nil {
		d := *t.DueAt
		tk.DueAt = &d
	}
	if t.CompletedAt != nil {
		c := *t.CompletedAt
		tk.CompletedAt = &c
	}
	return tk
}

// Remove 软删除任务（撤销派发）。
func (s *Store) Remove(ctx context.Context, taskID string) error {
	n, err := s.db.Task.UpdateOneID(toUUID(taskID)).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "撤销任务失败", err)
	}
	if n == nil || n.ID == uuid.Nil {
		return apperr.New(apperr.CodeNotFound, "任务不存在", nil)
	}
	return nil
}
