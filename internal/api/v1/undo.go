package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/expense"
)

// UndoRecord 撤销记录视图（handler 只需要这几个字段，repo 适配）。
type UndoRecord struct {
	ID        string
	ToolName  string
	UndoData  json.RawMessage
	Status    string // active | used | expired
	ExpiresAt time.Time
	CreatedAt time.Time
}

// UndoStore 撤销存储接口（handler 只需要这几个能力，repo 实现）。
type UndoStore interface {
	GetUndo(ctx context.Context, id, memberID string) (UndoRecord, error)
	MarkUsed(ctx context.Context, id string) error
	// ListActive 我的未使用且未过期的撤销项（新的在前）。
	ListActive(ctx context.Context, memberID string) ([]UndoRecord, error)
}

// UndoSummaryProvider 撤销项「对象摘要」所需的对象查询（按工具查不同领域对象）。
// 查不到不报错，降级成工具标签（列表永远不因摘要失败而 500）。
type UndoSummaryProvider interface {
	// ExpenseBrief 记账的金额（分）与分类。
	ExpenseBrief(ctx context.Context, memberID, expenseID string) (amountCents int64, category string, err error)
	// TaskTitle 家务任务标题。
	TaskTitle(ctx context.Context, taskID string) (title string, err error)
}

// ListUndo GET /undo —— 我的可撤销操作（24h 内有效，新的在前）。
func ListUndo(store UndoStore, sp UndoSummaryProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		records, err := store.ListActive(c.Request.Context(), memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		items := make([]gin.H, 0, len(records))
		for _, r := range records {
			items = append(items, gin.H{
				"undo_id":    r.ID,
				"tool_name":  r.ToolName,
				"summary":    summarizeUndo(c.Request.Context(), sp, memberID, r),
				"created_at": r.CreatedAt.Format(time.RFC3339),
				"expires_at": r.ExpiresAt.Format(time.RFC3339),
			})
		}
		c.JSON(200, gin.H{"items": items})
	}
}

// undoSummary 工具名 → 人类可读标签（查不到对象时兜底）。
var undoSummaryFallback = map[string]string{
	"record_expense": "记账",
	"update_expense": "修正账单",
	"assign_task":    "派任务",
	"complete_task":  "打卡",
	"report_meal":    "报饭",
	"suggest_dinner": "晚餐建议",
	"switch_model":   "切换模型",
}

// summarizeUndo 把 undo_data 解析成「记账 ¥12.00 · 食材」这样的摘要；
// 任何解析/查询失败都降级到工具标签，绝不让列表接口失败。
func summarizeUndo(ctx context.Context, sp UndoSummaryProvider, memberID string, r UndoRecord) string {
	label, ok := undoSummaryFallback[r.ToolName]
	if !ok {
		label = r.ToolName
	}
	if sp == nil {
		return label
	}
	switch r.ToolName {
	case "record_expense", "update_expense":
		var d struct {
			ExpenseID string `json:"expense_id"`
		}
		if err := json.Unmarshal(r.UndoData, &d); err != nil || d.ExpenseID == "" {
			return label
		}
		cents, category, err := sp.ExpenseBrief(ctx, memberID, d.ExpenseID)
		if err != nil {
			return label
		}
		text := fmt.Sprintf("%s ¥%s", label, expense.CentsToYuan(cents))
		if category != "" {
			text += " · " + category
		}
		return text
	case "assign_task", "complete_task":
		var d struct {
			TaskID string `json:"task_id"`
		}
		if err := json.Unmarshal(r.UndoData, &d); err != nil || d.TaskID == "" {
			return label
		}
		title, err := sp.TaskTitle(ctx, d.TaskID)
		if err != nil || title == "" {
			return label
		}
		return fmt.Sprintf("%s：%s", label, title)
	case "report_meal":
		var d struct {
			Date string `json:"date"`
		}
		if err := json.Unmarshal(r.UndoData, &d); err != nil || d.Date == "" {
			return label
		}
		if t, err := time.Parse("2006-01-02", d.Date); err == nil {
			return fmt.Sprintf("%s %s", label, t.Format("01-02"))
		}
		return fmt.Sprintf("%s %s", label, d.Date)
	default:
		return label
	}
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
