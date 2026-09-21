// Package main 是唯一入口，只做依赖装配与启动。
// 分层依赖在 cmd 装配，业务逻辑全在 internal/（docs/CONVENTIONS-backend.md §1）。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/runtime"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/api/middleware"
	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/auth"
	"github.com/mk20mm/homeagent/internal/domain/expense"
	"github.com/mk20mm/homeagent/internal/domain/meal"
	"github.com/mk20mm/homeagent/internal/domain/model"
	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
	"github.com/mk20mm/homeagent/internal/domain/task"
	"github.com/mk20mm/homeagent/internal/domain/undo"
	"github.com/mk20mm/homeagent/internal/infra/config"
	"github.com/mk20mm/homeagent/internal/store"
	"github.com/mk20mm/homeagent/internal/store/repo"
)

// undoSummaryProvider 给撤销中心提供「对象摘要」：按工具类型查不同的领域对象。
// 查不到时 handler 会降级成工具标签，这里只管尽力查。
type undoSummaryProvider struct {
	repo  *repo.Store
	tasks task.Service
}

func (p undoSummaryProvider) ExpenseBrief(ctx context.Context, memberID, expenseID string) (int64, string, error) {
	e, err := p.repo.Get(ctx, memberID, expenseID)
	if err != nil {
		return 0, "", err
	}
	return e.AmountCents, e.Category, nil
}

func (p undoSummaryProvider) TaskTitle(ctx context.Context, taskID string) (string, error) {
	t, err := p.tasks.GetTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	return t.Title, nil
}

func main() {
	migrate := flag.Bool("migrate", false, "建表并写入种子数据后退出")
	flag.Parse()

	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(cfg.LogLevel),
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// 数据层：SQLite（modernc 纯 Go 驱动）+ ent 自动建表 + 幂等种子。
	client, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		slog.Error("open store failed", "err", err, "path", cfg.DBPath)
		os.Exit(1)
	}
	defer func() { _ = client.Close() }()

	if *migrate {
		if err := store.MustSeed(ctx, client); err != nil {
			slog.Error("seed failed", "err", err)
			os.Exit(1)
		}
		// 开发期：打印成员登录凭据（name + auth_token → POST /auth/token 换 JWT）
		if members, err := client.Member.Query().All(ctx); err == nil {
			for _, m := range members {
				slog.Info("seed member", "id", m.ID, "name", m.Name, "role", m.Role,
					"auth_token", m.AuthToken)
			}
		}
		slog.Info("migrate + seed done", "path", cfg.DBPath)
		return
	}

	// 仓储层（聚合：expense/conversation/undolog/audit/usage）
	storeRepo := repo.New(client)
	// 供应商仓储（带加密能力，api_key 落库加密）
	provRepo := repo.NewProviderStore(storeRepo, cfg.EncryptionKey)

	// 工具注册表：一期 10 工具（9 可见 + update_expense 隐藏 + undo_last 对话侧撤销）
	registry := tool.NewRegistry()
	expenseSvc := expense.NewService(storeRepo)
	taskSvc := task.NewService(storeRepo)
	mealSvc := meal.NewService(storeRepo)
	modelSvc := model.NewService(storeRepo, provRepo)
	undoLastTool := undo.NewUndoLastTool(&undoStoreAdapter{inner: storeRepo})
	for _, t := range []tool.Tool{
		expense.NewRecordExpenseTool(expenseSvc),
		expense.NewQueryBudgetTool(expenseSvc),
		expense.NewUpdateExpenseTool(expenseSvc),
		task.NewAssignTaskTool(taskSvc),
		task.NewCompleteTaskTool(taskSvc),
		task.NewListMyTasksTool(taskSvc),
		meal.NewReportMealTool(mealSvc),
		meal.NewSuggestDinnerTool(mealSvc),
		model.NewListModelsTool(modelSvc),
		model.NewSwitchModelTool(modelSvc),
		undoLastTool,
	} {
		if err := registry.Register(t); err != nil {
			slog.Error("register tool failed", "tool", t.Spec().Name, "err", err)
			os.Exit(1)
		}
	}
	slog.Info("tools registered", "count", len(registry.List()))

	// 执行器：统一包办权限校验→参数校验→执行→undo_log→审计
	executor := tool.NewExecutor(registry, storeRepo, storeRepo)
	// 打断循环依赖：executor 创建后注入给 undo_last
	undoLastTool.SetExecutor(executor)

	// 会话服务（member 隔离 + 历史持久化）
	sessions := session.NewService(storeRepo)

	// JWT 签发器（HS256 + JWTSecret；P1 认证）
	signer := auth.NewSigner(cfg.JWTSecret, "homeagent")

	// LLM 网关：优先用数据库配置的默认模型 + 供应商密钥；
	// 数据库没配或密钥缺失时，回落环境变量；都没有用脚本供应商（本地联调不依赖外网）
	provider := buildProvider(ctx, cfg, provRepo)
	rt := runtime.New(runtime.Options{
		Provider: provider,
		Executor: executor,
		Sessions: sessions,
		Usage:    storeRepo, // 用量埋点写 llm_usage
	})

	r := gin.New()
	r.Use(middleware.Recover(), middleware.TraceID(), middleware.CORS())
	api := r.Group("/api/v1")
	v1.Register(api, executor, rt, storeRepo, storeRepo, storeRepo, storeRepo, signer, storeRepo, storeRepo, storeRepo, modelSvc, sessions, storeRepo, expenseSvc, expenseSvc, storeRepo, taskSvc, mealSvc, storeRepo, undoSummaryProvider{repo: storeRepo, tasks: taskSvc}, storeRepo)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		slog.Info("homeagent listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// buildProvider 选 LLM 供应商：数据库默认模型 → 环境变量 → 脚本供应商。
//
// 优先级设计（配置单一真相源是数据库，环境变量只做开发期覆盖/兜底）：
//  1. 数据库默认模型 + 其供应商已配密钥 → 真实供应商
//  2. 环境变量 LLM_API_KEY + LLM_MODEL → 真实供应商（开发期便利）
//  3. 都没有 → 脚本供应商（本地联调不花一分钱）
func buildProvider(ctx context.Context, cfg config.Config, provRepo dommodel.ProviderRepo) gateway.Provider {
	// 1. 数据库默认模型（明文密钥只在 store→main 这一段传递）
	if conn, ok, err := provRepo.DefaultEnabled(ctx); err == nil && ok {
		slog.Info("llm provider: db default", "provider", conn.Provider, "model", conn.Model)
		return gateway.NewOpenAIProvider(conn.APIKey, conn.BaseURL, conn.Model, conn.Provider)
	}

	// 2. 环境变量兜底
	if cfg.LLMAPIKey != "" && cfg.LLMModel != "" {
		slog.Info("llm provider: env", "model", cfg.LLMModel)
		return gateway.NewOpenAIProvider(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMProvider)
	}

	// 3. 脚本供应商
	slog.Info("llm provider: scripted (no api_key configured in db or env)")
	return gateway.NewScriptedProvider(gateway.RecordExpenseScript())
}
