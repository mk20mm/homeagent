package v1

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// AuditQuery 审计查询参数（handler 视图，repo 适配）。
type AuditQuery struct {
	PageSize         int
	Cursor           string
	ToolName         string
	PermissionDenied bool
}

// AuditItem 审计视图（对齐 openapi AuditLog）。
type AuditItem struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	ToolName         string    `json:"tool_name"`
	Risk             string    `json:"risk"`
	Result           string    `json:"result"`
	LatencyMS        int       `json:"latency_ms"`
	Undone           bool      `json:"undone"`
	PermissionDenied bool      `json:"permission_denied"`
	TraceID          string    `json:"trace_id"`
}

// AuditLister 审计查询（repo 实现，按成员隔离）。
type AuditLister interface {
	ListAudit(ctx context.Context, memberID string, q AuditQuery) ([]AuditItem, string, error)
}

// ListAudit GET /audit：游标分页 + 越权过滤（管理端只读视图）。
func ListAudit(lister AuditLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		q := AuditQuery{Cursor: c.Query("cursor"), ToolName: c.Query("tool_name")}
		if v := c.Query("page_size"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				q.PageSize = n
			}
		}
		if v := c.Query("permission_denied"); v == "true" || v == "1" {
			q.PermissionDenied = true
		}

		items, nextCursor, err := lister.ListAudit(c.Request.Context(), memberID, q)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "查询审计日志失败", err))
			return
		}
		if items == nil {
			items = []AuditItem{}
		}
		c.JSON(http.StatusOK, gin.H{
			"items":        items,
			"next_cursor":  nextCursor,
		})
	}
}
