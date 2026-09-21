// Package undo 提供对话侧的撤销能力（T-A09，修复 T-e2e-2）。
//
// 用户在对话里说「取消刚才那笔」「撤销刚才的操作」时，由本工具撤销最近一条
// 24h 内的可撤销记录。与撤销中心（GET/POST /undo）走同一 undo_log、
// 同一 Executor.Undo 链路、同一成员隔离，只是入口从 UI 卡片换成自然语言。
package undo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// Record 工具需要的撤销记录视图（与 v1.UndoRecord 同构，由调用方适配）。
type Record struct {
	ID        string
	ToolName  string
	UndoData  json.RawMessage
	ExpiresAt time.Time
}

// Store 撤销存储最小接口（ListActive 取最近记录、MarkUsed 标记已用）。
// repo 层不依赖本包；由 main.go 用适配函数把 v1.UndoStore 转成本接口。
type Store interface {
	ListActive(ctx context.Context, memberID string) ([]Record, error)
	MarkUsed(ctx context.Context, id string) error
}

// UndoLastInput 无参数：永远撤销「最近一条」。
type UndoLastInput struct{}

// UndoLastTool 对话侧撤销工具：撤销最近一笔可撤销操作。
//
// 设计要点（ADR-004）：
//   - 走 Executor.Undo，与卡片撤销按钮完全同一链路（不绕过权限/窗口/审计）
//   - 撤销本身不再产生 undo 记录（UndoData 为空，Executor 自动跳过 undo_log）
//   - 无可撤销项时明确告知，不静默成功
type UndoLastTool struct {
	store Store
	exec  *tool.Executor
}

// NewUndoLastTool 创建工具，executor 由 SetExecutor 在 main 组装阶段注入
// （Executor 依赖 Registry，Registry 又要注册本工具，循环依赖靠 setter 打断）。
func NewUndoLastTool(store Store) *UndoLastTool {
	return &UndoLastTool{store: store}
}

// SetExecutor 注入执行器（main.go 在 executor 创建后调用）。
func (t *UndoLastTool) SetExecutor(exec *tool.Executor) {
	t.exec = exec
}

func (t *UndoLastTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        "undo_last",
		Description: "撤销最近一次操作。用户说『取消刚才那笔/撤销刚才的操作/后悔了』时使用。会撤销 24 小时内最近一笔可撤销的记账、派任务、打卡或报饭。无可撤销项时会明确告知。",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		Risk:        tool.RiskHigh,
		Permission:  "", // 撤销自己的操作是基本权利，全员可用（仍受成员隔离约束）
		Module:      "undo",
		Idempotency: "member+latest",
	}
}

func (t *UndoLastTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if _, err := tool.DecodeInput[UndoLastInput](input); err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	if memberID == "" {
		return tool.Result{}, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}

	records, err := t.store.ListActive(ctx, memberID)
	if err != nil {
		return tool.Result{}, err
	}
	if len(records) == 0 {
		// 无可撤销项：明确告知，不静默成功
		return tool.Result{
			Summary: "最近 24 小时内没有可撤销的操作。",
			Card:    mustCard("none", "没有可撤销的操作", "记账、派任务、打卡或报饭之后，24 小时内可以反悔。"),
		}, nil
	}

	latest := records[0] // ListActive 已按创建时间倒序，第一条就是最近

	// 窗口校验（与 POST /undo 同一约束）
	if latest.ExpiresAt.Before(time.Now()) {
		return tool.Result{
			Summary: "最近一笔操作已超过 24 小时撤销窗口，无法撤销。",
			Card:    mustCard("expired", "已超过撤销窗口", "操作超过 24 小时后不可自动撤销，如需修改请重新说一遍。"),
		}, nil
	}

	// 执行撤销（Executor 记审计 + 成员隔离，与卡片撤销完全同一链路）
	if t.exec == nil {
		return tool.Result{}, apperr.New(apperr.CodeInternal, "撤销执行器未初始化", nil)
	}
	if err := t.exec.Undo(ctx, latest.ToolName, latest.UndoData, memberID); err != nil {
		return tool.Result{}, err
	}
	_ = t.store.MarkUsed(ctx, latest.ID)

	summary := fmt.Sprintf("已撤销：%s。", toolLabel(latest.ToolName))
	return tool.Result{
		Summary: summary,
		Card: mustCard("undone", "已撤销",
			fmt.Sprintf("%s已撤销，操作回退完成。", toolLabel(latest.ToolName))),
	}, nil
}

// toolLabel 工具名 → 中文标签（与 undo.go 的 undoSummaryFallback 对齐）
func toolLabel(name string) string {
	labels := map[string]string{
		"record_expense": "记账",
		"update_expense": "修正账单",
		"assign_task":    "派任务",
		"complete_task":  "打卡",
		"report_meal":    "报饭",
		"suggest_dinner": "晚餐建议",
		"switch_model":   "切换模型",
	}
	if label, ok := labels[name]; ok {
		return label
	}
	return name
}

// Undo 拒绝再撤销：撤销的逆操作是「重做」，超出当前设计范围。
// 本工具的 Result.UndoData 为空，Executor 不会写 undo_log，
// 此方法仅为满足 WriteTool 编译期约束（ADR-004），实际不可达。
func (t *UndoLastTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	return apperr.New(apperr.CodeConflict, "撤销操作不可再撤销，如需恢复请重新说一遍", nil)
}

func mustCard(kind, title, body string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"type":  kind,
		"title": title,
		"body":  body,
	})
	return b
}
