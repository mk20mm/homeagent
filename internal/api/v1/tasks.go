package v1

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/task"
)

// TaskItem 对齐 openapi Task schema（handler 视图）。
type TaskItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Risk        string `json:"risk,omitempty"`
	AssigneeID  string `json:"assignee_id,omitempty"`
	Status      string `json:"status"`
	DueAt       string `json:"due_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Points      int    `json:"points,omitempty"`
}

func taskItem(t task.Task) TaskItem {
	item := TaskItem{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Risk:        t.Risk,
		AssigneeID:  t.AssigneeID,
		Status:      string(t.Status),
		Points:      t.Points,
	}
	if t.DueAt != nil {
		item.DueAt = t.DueAt.Format(time.RFC3339)
	}
	if t.CompletedAt != nil {
		item.CompletedAt = t.CompletedAt.Format(time.RFC3339)
	}
	return item
}

func taskResponse(item TaskItem, undoID string, duplicated bool) gin.H {
	return gin.H{
		"id":           item.ID,
		"title":        item.Title,
		"description":  item.Description,
		"risk":         item.Risk,
		"assignee_id":  item.AssigneeID,
		"status":       item.Status,
		"due_at":       item.DueAt,
		"completed_at": item.CompletedAt,
		"points":       item.Points,
		"undoable":     undoID != "",
		"undo_id":      undoID,
		"duplicated":   duplicated,
	}
}

// TaskService 家务领域服务（handler 只用这几个方法，与工具走同一服务）。
type TaskService interface {
	ListMyTasks(ctx context.Context, memberID string) ([]task.Task, error)
	AssignTask(ctx context.Context, assignerID string, cmd task.AssignTaskCmd) (id string, duplicated bool, err error)
	CompleteTask(ctx context.Context, taskID, memberID string) error
	GetTask(ctx context.Context, taskID string) (task.Task, error)
}

// MemberNamer 成员名查询（repo 实现；HTTP 边界把 assignee_id 解析成名字，供领域幂等键）。
type MemberNamer interface {
	MemberName(ctx context.Context, memberID string) (string, error)
}

// ListTasks GET /tasks —— 我的待办（list_my_tasks 的 HTTP 出口，member 隔离 + 软删除过滤）。
func ListTasks(svc TaskService, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "list_my_tasks", tool.RiskLow, "task.read") {
			abortWith(c, apperr.New(apperr.CodePermission, "无任务查看权限", nil))
			return
		}

		tasks, err := svc.ListMyTasks(c.Request.Context(), memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		// status 过滤（领域默认只返未完成，done 由调用方显式请求也无意义）
		status := c.Query("status")
		items := make([]TaskItem, 0, len(tasks))
		for _, t := range tasks {
			if status != "" && string(t.Status) != status {
				continue
			}
			items = append(items, taskItem(t))
		}
		c.JSON(200, gin.H{"items": items})
	}
}

// CreateTask POST /tasks —— 派任务（assign_task 的 HTTP 出口，幂等键与工具一致）。
func CreateTask(svc TaskService, namer MemberNamer, undoWriter UndoWriter, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "assign_task", tool.RiskMedium, "task.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无派任务权限", nil))
			return
		}

		var body struct {
			Title       *string `json:"title"`
			Description string  `json:"description"`
			AssigneeID  string  `json:"assignee_id"`
			DueAt       string  `json:"due_at"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数有误", err))
			return
		}
		if body.Title == nil || *body.Title == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "任务标题不能为空", nil))
			return
		}

		ctx := c.Request.Context()
		cmd := task.AssignTaskCmd{Title: *body.Title, Description: body.Description}
		if body.AssigneeID != "" {
			// 边界处把 assignee_id 解析成名字：领域幂等键按名字算，与工具同一键
			name, err := namer.MemberName(ctx, body.AssigneeID)
			if err != nil {
				abortWith(c, asAppErr(err))
				return
			}
			cmd.AssigneeName = name
		}
		if body.DueAt != "" {
			d, err := time.Parse(time.RFC3339, body.DueAt)
			if err != nil {
				abortWith(c, apperr.New(apperr.CodeInvalidInput, "截止时间格式有误", err))
				return
			}
			cmd.DueAt = d
		}

		id, duplicated, err := svc.AssignTask(ctx, memberID, cmd)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		// 幂等命中不写撤销记录（撤销会误删早先那个任务）
		var undoID string
		if !duplicated {
			undoData, _ := json.Marshal(map[string]any{"task_id": id})
			uid, err := undoWriter.SaveUndo(ctx, memberID, "assign_task", undoData)
			if err != nil {
				abortWith(c, asAppErr(err))
				return
			}
			undoID = uid
		}

		tk, err := svc.GetTask(ctx, id)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(201, taskResponse(taskItem(tk), undoID, duplicated))
	}
}

// CompleteTask POST /tasks/:taskId/complete —— 打卡（complete_task 的 HTTP 出口，防重复）。
func CompleteTask(svc TaskService, undoWriter UndoWriter, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, audit, memberID, "complete_task", tool.RiskMedium, "task.read") {
			abortWith(c, apperr.New(apperr.CodePermission, "无任务打卡权限", nil))
			return
		}

		taskID := c.Param("taskId")
		if taskID == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少任务 id", nil))
			return
		}

		ctx := c.Request.Context()
		if err := svc.CompleteTask(ctx, taskID, memberID); err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		// 撤销 = 回退未完成（Uncomplete 语义，与工具同一 undo_data）
		undoData, _ := json.Marshal(map[string]any{"task_id": taskID})
		undoID, err := undoWriter.SaveUndo(ctx, memberID, "complete_task", undoData)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		tk, err := svc.GetTask(ctx, taskID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(200, taskResponse(taskItem(tk), undoID, false))
	}
}
