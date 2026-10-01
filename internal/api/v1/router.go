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
	expUndoWriter ExpenseUndoWriter,
	inv CacheInvalidator,
	taskSvc TaskService,
	mealSvc MealService,
	mealLister MealReportLister,
	incomeLister IncomeLister,
	incomeRecorder IncomeRecorder,
	incomeSummarizer IncomeSummarizer,
	incomeUndoWriter IncomeUndoWriter,
	financeSummarizer FinanceSummarizer,
	travelSvc TravelService,
	runSvc RunService,
) {
	rg.POST("/auth/token", CreateToken(authLookup, signer))
	rg.GET("/health", health)

	// JWT 认证（P1）：令牌证明身份，权限仍走矩阵双保险
	jwtGroup := rg.Group("", middleware.JWTAuth(signer, lookup))
	jwtGroup.GET("/tools", listTools(executor, pl))
	jwtGroup.POST("/chat", Chat(rt))
	jwtGroup.POST("/undo/:id", Undo(executor, undoStore))
	jwtGroup.GET("/models", ListModels(modelLister))
	jwtGroup.GET("/audit", ListAudit(auditLister))
	jwtGroup.GET("/usage", ListUsage(usageLister))
	jwtGroup.GET("/conversations", ListConversations(convSvc))
	jwtGroup.POST("/conversations", CreateConversation(convSvc))
	jwtGroup.GET("/conversations/:conversationId/messages", ListMessages(convSvc))
	jwtGroup.DELETE("/conversations/:conversationId", DeleteConversation(convSvc))
	jwtGroup.GET("/expenses", ListExpenses(expLister))
	jwtGroup.GET("/expenses/summary", ExpenseSummary(expSummarizer))
	jwtGroup.POST("/expenses", CreateExpense(expRecorder, expUndoWriter, pl))
	jwtGroup.PATCH("/expenses/:expenseId", UpdateExpense(expRecorder, expUndoWriter, pl))
	jwtGroup.GET("/incomes", ListIncomes(incomeLister))
	jwtGroup.POST("/incomes", CreateIncome(incomeRecorder, incomeUndoWriter, pl))
	jwtGroup.PATCH("/incomes/:incomeId", UpdateIncome(incomeRecorder, incomeUndoWriter, pl))
	jwtGroup.GET("/incomes/summary", IncomeSummary(incomeSummarizer))
	jwtGroup.GET("/finance/summary", FinanceSummaryHandler(financeSummarizer))
	jwtGroup.GET("/tasks", ListTasks(taskSvc))
	jwtGroup.POST("/tasks", CreateTask(taskSvc, pl))
	jwtGroup.POST("/tasks/:taskId/complete", CompleteTask(taskSvc, pl))
	jwtGroup.GET("/meals", ListMeals(mealLister))
	jwtGroup.POST("/meals", ReportMeal(mealSvc, pl))
	jwtGroup.GET("/vehicles", ListVehicles(travelSvc))
	jwtGroup.POST("/vehicles", CreateVehicle(travelSvc, pl))
	jwtGroup.GET("/trips", ListTrips(travelSvc))
	jwtGroup.POST("/trips", CreateTrip(travelSvc, pl))
	jwtGroup.GET("/runs", ListRuns(runSvc))
	jwtGroup.GET("/runs/:runId", GetRun(runSvc))
	jwtGroup.POST("/runs/:runId/confirm", ConfirmRunStep(runSvc))
	jwtGroup.POST("/runs/:runId/reply", ReplyRun(runSvc))
	jwtGroup.POST("/runs/:runId/amend", AmendRun(runSvc))

	// 管理端配置写端点（只有 parent 能写，handler 内 requireParent 双保险）
	adminGroup := jwtGroup.Group("/admin")
	adminGroup.GET("/providers", ListProviders(provSvc))
	adminGroup.PUT("/providers/:id", UpdateProvider(provSvc, inv))
	adminGroup.POST("/providers/:id/test", TestProvider(provSvc))
	adminGroup.POST("/models", CreateModel(provSvc, inv))
	adminGroup.PUT("/models/:id", UpdateModel(provSvc, inv))
	adminGroup.DELETE("/models/:id", DeleteModel(provSvc, inv))
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
