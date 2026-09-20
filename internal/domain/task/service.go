// Package task 家务领域：派发/打卡/查询（ARCHITECTURE §2.3）。
//
// 不变量：
//   - 任务状态机 pending → in_progress → done，迁移只在代码
//   - 打卡幂等：同一任务重复完成返回冲突
//   - 派发幂等键 sha256(派给人+执行人+标题+日)
package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// TaskStatus 状态机。
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

// Task 任务视图（工具与 handler 消费）。
type Task struct {
	ID           string
	Title        string
	Description  string
	Risk         string
	Status       TaskStatus
	AssigneeID   string
	AssigneeName string
	DueAt        *time.Time
	CompletedAt  *time.Time
	Points       int
}

// AssignTaskCmd 派发命令。
type AssignTaskCmd struct {
	Title        string
	Description  string
	AssigneeName string    // 空 = 待认领
	Risk         string    // low/medium/high，空 = medium
	DueAt        time.Time // 零值 = 无截止
}

// TaskRepo 仓储接口（store 层实现，依赖单向）。
// 方法名避开 expense 的 Create/Delete（Store 聚合上不能同名重载）。
type TaskRepo interface {
	Assign(ctx context.Context, assignerID string, cmd AssignTaskCmd, idempotencyKey string) (taskID string, err error)
	Complete(ctx context.Context, taskID, memberID string) error
	Uncomplete(ctx context.Context, taskID string) error
	ListMyTasks(ctx context.Context, memberID string) ([]Task, error)
	GetTask(ctx context.Context, taskID string) (Task, error)
	Remove(ctx context.Context, taskID string) error
}

// Service 家务领域服务。
type Service interface {
	AssignTask(ctx context.Context, assignerID string, cmd AssignTaskCmd) (id string, duplicated bool, err error)
	CompleteTask(ctx context.Context, taskID, memberID string) error
	UncompleteTask(ctx context.Context, taskID string) error
	ListMyTasks(ctx context.Context, memberID string) ([]Task, error)
	GetTask(ctx context.Context, taskID string) (Task, error)
	DeleteTask(ctx context.Context, id string) error
}

func NewService(repo TaskRepo) Service {
	return &service{repo: repo}
}

type service struct {
	repo TaskRepo
}

// AssignTask 派发：校验→幂等键→入库。幂等命中返回 duplicated=true（对用户是成功）。
func (s *service) AssignTask(ctx context.Context, assignerID string, cmd AssignTaskCmd) (string, bool, error) {
	if cmd.Title == "" {
		return "", false, apperr.New(apperr.CodeInvalidInput, "任务标题不能为空", nil)
	}
	if cmd.Risk == "" {
		cmd.Risk = "medium"
	}
	key := IdempotencyKey(assignerID, cmd)
	id, err := s.repo.Assign(ctx, assignerID, cmd, key)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) && ae.Code == apperr.CodeConflict {
			return id, true, nil // 幂等命中，对用户是成功
		}
		return "", false, err
	}
	return id, false, nil
}

// CompleteTask 打卡：状态机迁移 + 幂等（已 done 返回冲突）。
func (s *service) CompleteTask(ctx context.Context, taskID, memberID string) error {
	if taskID == "" {
		return apperr.New(apperr.CodeInvalidInput, "任务 id 不能为空", nil)
	}
	return s.repo.Complete(ctx, taskID, memberID)
}

func (s *service) ListMyTasks(ctx context.Context, memberID string) ([]Task, error) {
	return s.repo.ListMyTasks(ctx, memberID)
}

// GetTask 单任务查询（家庭可见读；写路径各自校验归属）。
func (s *service) GetTask(ctx context.Context, taskID string) (Task, error) {
	if taskID == "" {
		return Task{}, apperr.New(apperr.CodeInvalidInput, "任务 id 不能为空", nil)
	}
	return s.repo.GetTask(ctx, taskID)
}

// UncompleteTask 撤销误打卡：回退 pending + 清完成时间。
func (s *service) UncompleteTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return apperr.New(apperr.CodeInvalidInput, "任务 id 不能为空", nil)
	}
	return s.repo.Uncomplete(ctx, taskID)
}

func (s *service) DeleteTask(ctx context.Context, id string) error {
	if id == "" {
		return apperr.New(apperr.CodeInvalidInput, "任务 id 不能为空", nil)
	}
	return s.repo.Remove(ctx, id)
}

// IdempotencyKey 派发幂等键 sha256(派给人+执行人+标题+日)。
func IdempotencyKey(assignerID string, cmd AssignTaskCmd) string {
	day := time.Now().Format("2006-01-02")
	h := sha256.Sum256([]byte(assignerID + "|" + cmd.AssigneeName + "|" + cmd.Title + "|" + day))
	return hex.EncodeToString(h[:])
}

// FormatDueAt 截止时间格式化（工具卡片用）。
func FormatDueAt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("01-02 15:04")
}

// RiskLabel 风险中文标签（卡片展示）。
func RiskLabel(risk string) string {
	switch risk {
	case "high":
		return "重要"
	case "low":
		return "轻松"
	default:
		return "日常"
	}
}

// PointsLabel 积分展示。
func PointsLabel(points int) string {
	return strconv.Itoa(points) + " 分"
}
