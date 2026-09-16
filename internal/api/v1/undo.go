package v1

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// UndoRecord 撤销记录视图（handler 只需要这几个字段，repo 适配）。
type UndoRecord struct {
	ID        string
	ToolName  string
	UndoData  json.RawMessage
	Status    string // active | used | expired
	ExpiresAt time.Time
}

// UndoStore 撤销存储接口（handler 只需要这两个能力，repo 实现）。
type UndoStore interface {
	GetUndo(ctx context.Context, id, memberID string) (UndoRecord, error)
	MarkUsed(ctx context.Context, id string) error
}

// Undo POST /undo/{id}：24h 窗口内撤销写操作（ADR-004）。
func Undo(exec *tool.Executor, store UndoStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		id := c.Param("id")
		if id == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少撤销 id", nil))
			return
		}

		ctx := tool.WithTraceID(c.Request.Context(), c.GetString("trace_id"))
		rec, err := store.GetUndo(ctx, id, memberID)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeNotFound, "撤销记录不存在或不属于该成员", err))
			return
		}

		// 24h 窗口校验（代码控制，AI-PRD §3 确定性边界）
		if string(rec.Status) != "active" {
			abortWith(c, apperr.New(apperr.CodeConflict, "该操作已撤销或已失效", nil))
			return
		}
		if rec.ExpiresAt.Before(time.Now()) {
			abortWith(c, apperr.New(apperr.CodeConflict, "撤销窗口已过期（24 小时）", nil))
			return
		}

		// 执行撤销（Executor 记审计，撤销也留痕）
		if err := exec.Undo(ctx, rec.ToolName, rec.UndoData, memberID); err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "撤销失败", err))
			return
		}
		_ = store.MarkUsed(ctx, id)

		c.JSON(200, gin.H{
			"undone":   true,
			"trace_id": c.GetString("trace_id"),
		})
	}
}
