package v1

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/expense"
)

// ExpenseQuery 流水查询（游标分页 + 分类/日期过滤）。
type ExpenseQuery struct {
	PageSize  int
	Cursor    string
	Category  string
	StartDate *time.Time
	EndDate   *time.Time
}

// ExpenseItem 流水展示项（对齐 openapi Expense schema）。
type ExpenseItem struct {
	ID          string `json:"id"`
	MemberID    string `json:"member_id"`
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
	Hint        string `json:"hint"`
	OccurredAt  string `json:"occurred_at"`
}

// ExpenseLister 记账流水查询（repo 实现，member 隔离 + 软删除过滤）。
type ExpenseLister interface {
	ListExpenses(ctx context.Context, memberID string, q ExpenseQuery) (items []ExpenseItem, nextCursor string, err error)
}

// ExpenseSummarizer 记账汇总（expense.Service 实现）。
type ExpenseSummarizer interface {
	QueryBudget(ctx context.Context, memberID string) (expense.ExpenseSummary, error)
}

// ExpenseRecorder 记账写入（expense.Service 实现）。
type ExpenseRecorder interface {
	RecordExpense(ctx context.Context, cmd expense.RecordExpenseCmd, memberID string) (id string, category string, duplicated bool, err error)
	UpdateExpense(ctx context.Context, id string, memberID string, cmd expense.UpdateExpenseCmd) (next expense.ExpenseRecord, prev expense.ExpenseRecord, err error)
}

// ExpenseUndoWriter 撤销记录写入（repo 实现，handler 直调写 undo_log）。
type ExpenseUndoWriter interface {
	SaveUndo(ctx context.Context, memberID string, toolName string, undoData json.RawMessage) (undoID string, err error)
}

// permissionOf 查成员权限矩阵（ADR-005 运行时再校验）。
func permissionOf(c *gin.Context, pl PermissionLookup, memberID string, perm string) bool {
	perms, err := pl.Permissions(c.Request.Context(), memberID)
	if err != nil {
		return false
	}
	return perms[perm]
}

// ListExpenses GET /expenses —— 记账流水（按成员隔离，occurred_at 倒序）。
func ListExpenses(lister ExpenseLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		q := ExpenseQuery{
			PageSize: pageSize,
			Cursor:   c.Query("page_cursor"),
			Category: c.Query("category"),
		}
		if d := parseDate(c.Query("start_date")); d != nil {
			q.StartDate = d
		}
		if d := parseDate(c.Query("end_date")); d != nil {
			q.EndDate = d
		}
		items, nextCursor, err := lister.ListExpenses(c.Request.Context(), memberID, q)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(200, gin.H{"items": items, "next_cursor": nextCursor})
	}
}

// ExpenseSummary GET /expenses/summary —— 记账汇总（本月总额 + 分类明细）。
func ExpenseSummary(svc ExpenseSummarizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		s, err := svc.QueryBudget(c.Request.Context(), memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		cats := make([]gin.H, 0, len(s.ByCategory))
		for name, cents := range s.ByCategory {
			cats = append(cats, gin.H{"category": name, "total_cents": cents})
		}
		c.JSON(200, gin.H{
			"total_cents": s.TotalCents,
			"categories":  cats,
		})
	}
}

// CreateExpense POST /expenses —— 手动记一笔（FAB 快捷记账），同样幂等可撤销。
func CreateExpense(svc ExpenseRecorder, undoWriter ExpenseUndoWriter, pl PermissionLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, memberID, "expense.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无记账权限", nil))
			return
		}

		var body struct {
			AmountCents *int64  `json:"amount_cents"`
			Hint        *string `json:"hint"`
			Category    *string `json:"category"`
			OccurredAt  *string `json:"occurred_at"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数有误", err))
			return
		}
		if body.AmountCents == nil || *body.AmountCents <= 0 {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "金额必须大于 0", nil))
			return
		}
		if body.Hint == nil || *body.Hint == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少消费内容", nil))
			return
		}

		occurredAt := time.Now()
		if body.OccurredAt != nil {
			if t, err := time.Parse(time.RFC3339, *body.OccurredAt); err == nil {
				occurredAt = t
			}
		}

		category := ""
		if body.Category != nil {
			category = *body.Category
		}
		id, category, duplicated, err := svc.RecordExpense(c.Request.Context(), expense.RecordExpenseCmd{
			AmountCents: *body.AmountCents,
			Hint:        *body.Hint,
			Category:    category,
			OccurredAt:  occurredAt,
		}, memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		// 幂等命中不写撤销记录（撤销会误删早先那笔）
		var undoID string
		if !duplicated {
			undoData, _ := json.Marshal(map[string]any{"expense_id": id})
			uid, err := undoWriter.SaveUndo(c.Request.Context(), memberID, "record_expense", undoData)
			if err != nil {
				abortWith(c, asAppErr(err))
				return
			}
			undoID = uid
		}

		c.JSON(201, gin.H{
			"id":           id,
			"member_id":    memberID,
			"category":     category,
			"amount_cents": *body.AmountCents,
			"hint":         *body.Hint,
			"occurred_at":  occurredAt.Format(time.RFC3339),
			"undoable":     !duplicated,
			"undo_id":      undoID,
			"duplicated":   duplicated,
		})
	}
}

// UpdateExpense PATCH /expenses/:expenseId —— 修正账单（金额/类目/备注），旧值进 undo_log 可恢复。
func UpdateExpense(svc ExpenseRecorder, undoWriter ExpenseUndoWriter, pl PermissionLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		if !permissionOf(c, pl, memberID, "expense.write") {
			abortWith(c, apperr.New(apperr.CodePermission, "无记账权限", nil))
			return
		}

		expenseID := c.Param("expenseId")
		if expenseID == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少账单 id", nil))
			return
		}

		var body struct {
			AmountCents *int64  `json:"amount_cents"`
			Hint        *string `json:"hint"`
			Category    *string `json:"category"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数有误", err))
			return
		}

		cmd := expense.UpdateExpenseCmd{
			AmountCents: body.AmountCents,
			Hint:        body.Hint,
			Category:    body.Category,
		}
		next, prev, err := svc.UpdateExpense(c.Request.Context(), expenseID, memberID, cmd)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		// 撤销 = 恢复旧值（不是删除）
		undoData, _ := json.Marshal(map[string]any{
			"expense_id": expenseID,
			"member_id":  memberID,
			"restore": map[string]any{
				"amount_cents": prev.AmountCents,
				"hint":         prev.Hint,
				"category":     prev.Category,
			},
		})
		undoID, err := undoWriter.SaveUndo(c.Request.Context(), memberID, "update_expense", undoData)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(200, gin.H{
			"id":           expenseID,
			"member_id":    memberID,
			"category":     next.Category,
			"amount_cents": next.AmountCents,
			"hint":         next.Hint,
			"undoable":     true,
			"undo_id":      undoID,
		})
	}
}

// parseDate 解析 YYYY-MM-DD 查询参数。
func parseDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t
	}
	return nil
}
