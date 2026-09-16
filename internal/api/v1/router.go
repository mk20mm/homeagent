// Package v1 注册 /api/v1 路由。接口由 OpenAPI 生成桩，P0 手写最小实现（P2 统一 strict server）。
package v1

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/runtime"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/api/middleware"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// PermissionLookup 权限查询（repo 实现，工具清单按成员过滤）。
type PermissionLookup interface {
	Permissions(ctx context.Context, memberID string) (map[string]bool, error)
}

// Register 注册全部路由。lookup/pl/store 由 main 注入。
func Register(
	rg *gin.RouterGroup,
	executor *tool.Executor,
	rt *runtime.Runtime,
	undoStore UndoStore,
	lookup middleware.MemberLookup,
	pl PermissionLookup,
	debugger DebugInspector,
) {
	rg.GET("/health", health)

	// 开发期认证（P1 换 JWT）：X-Member-ID header 或 ?member_id
	auth := rg.Group("", middleware.DevAuth(lookup))
	auth.GET("/tools", listTools(executor, pl))
	auth.POST("/chat", Chat(rt))
	auth.POST("/undo/:id", Undo(executor, undoStore))
	auth.GET("/debug/state", Debug(debugger)) // P1 删除（技术债 T11）
}

func health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

func listTools(executor *tool.Executor, pl PermissionLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		perms, err := pl.Permissions(c.Request.Context(), memberID)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeNotFound, "成员不存在", err))
			return
		}
		specs := executor.Registry().SpecsWithPermission(perms)
		c.JSON(200, gin.H{"tools": specs})
	}
}
