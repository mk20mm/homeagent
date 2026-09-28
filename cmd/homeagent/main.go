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
	"github.com/mk20mm/homeagent/internal/domain/task"
	"github.com/mk20mm/homeagent/internal/infra/config"
	"github.com/mk20mm/homeagent/internal/store"
	"github.com/mk20mm/homeagent/internal/store/repo"
)

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

	// 工具注册表：财务+家务+用餐+模型工具
	registry := tool.NewRegistry()
	expenseSvc := expense.NewService(storeRepo, storeRepo)
	taskSvc := task.NewService(storeRepo)
	mealSvc := meal.NewService(storeRepo)
	modelSvc := model.NewService(storeRepo, provRepo)
	for _, t := range []tool.Tool{
		expense.NewRecordExpenseTool(expenseSvc),
		expense.NewQueryBudgetTool(expenseSvc),
		expense.NewUpdateExpenseTool(expenseSvc),
		expense.NewRecordIncomeTool(expenseSvc),
		expense.NewUpdateIncomeTool(expenseSvc),
		task.NewAssignTaskTool(taskSvc),
		task.NewCompleteTaskTool(taskSvc),
		task.NewListMyTasksTool(taskSvc),
		meal.NewReportMealTool(mealSvc),
		meal.NewSuggestDinnerTool(mealSvc),
		model.NewListModelsTool(modelSvc),
		model.NewSwitchModelTool(modelSvc),
	} {
		if err := registry.Register(t); err != nil {
			slog.Error("register tool failed", "tool", t.Spec().Name, "err", err)
			os.Exit(1)
		}
	}
	slog.Info("tools registered", "count", 12)

	// 执行器：统一包办权限校验→参数校验→执行→undo_log→审计
	executor := tool.NewExecutor(registry, storeRepo, storeRepo)

	// 会话服务（member 隔离 + 历史持久化）
	sessions := session.NewService(storeRepo)

	// JWT 签发器（HS256 + JWTSecret；P1 认证）
	signer := auth.NewSigner(cfg.JWTSecret, "homeagent")

	// LLM 网关：使用 DynamicGateway，支持按模型所属供应商动态路由；
	// 数据库未配置或无密钥时，优先回退到环境变量，最后回退到脚本供应商
	var fallbackProvider gateway.Provider
	if cfg.LLMAPIKey != "" && cfg.LLMModel != "" {
		slog.Info("llm fallback provider: env", "model", cfg.LLMModel)
		fallbackProvider = gateway.NewOpenAIProvider(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMProvider)
	} else {
		slog.Info("llm fallback provider: scripted")
		fallbackProvider = gateway.NewScriptedProvider(gateway.RecordExpenseScript())
	}
	gw := gateway.NewDynamicGateway(provRepo, fallbackProvider)

	rt := runtime.New(runtime.Options{
		Provider: gw,
		Executor: executor,
		Sessions: sessions,
		Usage:    storeRepo, // 用量埋点写 llm_usage
	})

	r := gin.New()
	r.Use(gin.Logger(), middleware.Recover(), middleware.TraceID(), middleware.CORS())
	api := r.Group("/api/v1")
	v1.Register(api, executor, rt, storeRepo, storeRepo, storeRepo, storeRepo, signer, storeRepo, storeRepo, storeRepo, modelSvc, sessions, storeRepo, expenseSvc, expenseSvc, storeRepo, gw, taskSvc, mealSvc, storeRepo, storeRepo, expenseSvc, expenseSvc, storeRepo, expenseSvc)

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

