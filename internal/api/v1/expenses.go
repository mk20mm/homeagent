package v1

import (
	"context"
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
