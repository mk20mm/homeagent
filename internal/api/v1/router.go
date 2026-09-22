// Package v1 注册 /api/v1 路由。接口由 OpenAPI 生成桩，P0 手写最小实现（P2 统一 strict server）。
package v1

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/runtime"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/api/middleware"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/auth"
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
	authLookup MemberAuthLookup,
	signer *auth.Signer,
	modelLister ModelLister,
	auditLister AuditLister,
	usageLister UsageLister,
	provSvc ProviderService,
	convSvc session.Service,
	expLister ExpenseLister,
	expSummarizer ExpenseSummarizer,
	expRecorder ExpenseRecorder,
	expUndoWriter UndoWriter,
	taskSvc TaskService,
	mealSvc MealService,
	memberNamer MemberNamer,
	undoSummarizer UndoSummaryProvider,
	notifStore NotificationStore,
	audit tool.AuditLogger,
) {
	rg.POST("/auth/token", CreateToken(authLookup, signer))
	rg.GET("/health", health)

	// JWT 认证（P1）：令牌证明身份，权限仍走矩阵双保险
	jwtGroup := rg.Group("", middleware.JWTAuth(signer, lookup))
	jwtGroup.GET("/tools", listTools(executor, pl))
	jwtGroup.POST("/chat", Chat(rt))
	jwtGroup.POST("/undo/:id", Undo(executor, undoStore))
	jwtGroup.GET("/undo", ListUndo(undoStore, undoSummarizer))
	jwtGroup.GET("/models", ListModels(modelLister))
	jwtGroup.GET("/audit", ListAudit(auditLister))
	jwtGroup.GET("/usage", ListUsage(usageLister))
	jwtGroup.GET("/conversations", ListConversations(convSvc))
	jwtGroup.POST("/conversations", CreateConversation(convSvc))
	jwtGroup.GET("/conversations/:conversationId/messages", ListMessages(convSvc))
	jwtGroup.DELETE("/conversations/:conversationId", DeleteConversation(convSvc))
	jwtGroup.GET("/expenses", ListExpenses(expLister))
	jwtGroup.GET("/expenses/summary", ExpenseSummary(expSummarizer))
	jwtGroup.POST("/expenses", CreateExpense(expRecorder, expUndoWriter, pl, audit))
	jwtGroup.PATCH("/expenses/:expenseId", UpdateExpense(expRecorder, expUndoWriter, pl, audit))

	// 家务（T-A01：契约已定义的 /tasks 出口，与工具同一领域服务）
	jwtGroup.GET("/tasks", ListTasks(taskSvc, pl, audit))
	jwtGroup.POST("/tasks", CreateTask(taskSvc, memberNamer, expUndoWriter, pl, audit))
	jwtGroup.POST("/tasks/:taskId/complete", CompleteTask(taskSvc, expUndoWriter, pl, audit))

	// 报饭（T-A01：契约已定义的 /meals 出口）
	jwtGroup.GET("/meals", ListMeals(mealSvc, pl, audit))
	jwtGroup.GET("/notifications", ListNotifications(notifStore))
	jwtGroup.POST("/notifications", MarkNotificationsRead(notifStore))
	jwtGroup.POST("/meals", ReportMeal(mealSvc, expUndoWriter, pl, audit))

	// 管理端配置写端点（只有 parent 能写，handler 内 requireParent 双保险）
	adminGroup := jwtGroup.Group("/admin")
	adminGroup.GET("/providers", ListProviders(provSvc))
	adminGroup.PUT("/providers/:id", UpdateProvider(provSvc))
	adminGroup.PUT("/models/:id", UpdateModel(provSvc))
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
