package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// 开发期调试端点（P1 删除，技术债 T11）：直查三张表验证「记账→撤销」闭环。

type DebugUndo struct {
	ID        string          `json:"id"`
	ToolName  string          `json:"tool_name"`
	TraceID   string          `json:"trace_id"`
	Status    string          `json:"status"`
	UndoData  json.RawMessage `json:"undo_data"`
	ExpiresAt time.Time       `json:"expires_at"`
	CreatedAt time.Time       `json:"created_at"`
}

type DebugAudit struct {
	TraceID          string    `json:"trace_id"`
	ToolName         string    `json:"tool_name"`
	Risk             string    `json:"risk"`
	Result           string    `json:"result"`
	Undone           bool      `json:"undone"`
	PermissionDenied bool      `json:"permission_denied"`
	CreatedAt        time.Time `json:"created_at"`
}

type DebugSummary struct {
	Count        int            `json:"count"`
	TotalCents   int64          `json:"total_cents"`
	ByCategory   map[string]int64 `json:"by_category_cents"`
	BudgetCents  int64          `json:"budget_cents"`
}

type DebugState struct {
	UndoLogs []DebugUndo  `json:"undo_logs"`
	Audits   []DebugAudit `json:"audits"`
	Summary  DebugSummary `json:"summary"`
}

type DebugInspector interface {
	DebugState(ctx context.Context, memberID string) (DebugState, error)
}

// Debug GET /debug/state：最近撤销记录 + 审计 + 本月汇总。
func Debug(inspector DebugInspector) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		st, err := inspector.DebugState(c.Request.Context(), memberID)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "调试查询失败", err))
			return
		}
		c.JSON(http.StatusOK, st)
	}
}
