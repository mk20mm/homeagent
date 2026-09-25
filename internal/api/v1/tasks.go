package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
	domtask "github.com/mk20mm/homeagent/internal/domain/task"
)

type TaskService interface {
	AssignTask(ctx context.Context, assignerID string, cmd domtask.AssignTaskCmd) (string, error)
	CompleteTask(ctx context.Context, taskID, memberID string) error
	ListMyTasks(ctx context.Context, memberID string) ([]domtask.Task, error)
}

func ListTasks(svc TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "未授权", nil))
			return
		}
		list, err := svc.ListMyTasks(c.Request.Context(), memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		if list == nil {
			list = []domtask.Task{}
		}
		c.JSON(http.StatusOK, gin.H{"items": list})
	}
}

type createTaskReq struct {
	Title       string     `json:"title" binding:"required"`
	Description string     `json:"description"`
	AssigneeID  string     `json:"assignee_id"`
	DueAt       *time.Time `json:"due_at"`
}

func CreateTask(svc TaskService, pl PermissionLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "未授权", nil))
			return
		}
		if !permissionOf(c, pl, memberID, "task.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无家务权限", nil))
			return
		}

		var req createTaskReq
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "参数错误", err))
			return
		}

		cmd := domtask.AssignTaskCmd{
			Title:        req.Title,
			Description:  req.Description,
			AssigneeName: req.AssigneeID,
		}
		if req.DueAt != nil {
			cmd.DueAt = *req.DueAt
		}

		id, err := svc.AssignTask(c.Request.Context(), memberID, cmd)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

func CompleteTask(svc TaskService, pl PermissionLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "未授权", nil))
			return
		}
		if !permissionOf(c, pl, memberID, "task.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无家务权限", nil))
			return
		}
		taskID := c.Param("taskId")
		if taskID == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少任务ID", nil))
			return
		}
		if err := svc.CompleteTask(c.Request.Context(), taskID, memberID); err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
