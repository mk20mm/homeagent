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

// IncomeQuery 收入流水查询。
type IncomeQuery struct {
	PageSize  int
	Cursor    string
	Source    string
	StartDate *time.Time
	EndDate   *time.Time
	Year      int
	Month     int
}

// IncomeItem 收入展示项（对齐 openapi Income schema）。
type IncomeItem struct {
	ID          string `json:"id"`
	MemberID    string `json:"member_id"`
	Source      string `json:"source"`
	AmountCents int64  `json:"amount_cents"`
	Hint        string `json:"hint"`
	Note        string `json:"note,omitempty"`
	OccurredAt  string `json:"occurred_at"`
	Undoable    bool   `json:"undoable"`
	UndoID      string `json:"undo_id,omitempty"`
	Duplicated  bool   `json:"duplicated"`
}

// IncomeLister 收入流水查询接口。
type IncomeLister interface {
	ListIncomes(ctx context.Context, memberID string, q IncomeQuery) (items []IncomeItem, nextCursor string, err error)
}

// IncomeSummarizer 收入汇总接口。
type IncomeSummarizer interface {
	QueryIncomeSummary(ctx context.Context, memberID string, target time.Time, period string) (expense.IncomeSummary, error)
}

// FinanceSummarizer 财务综合总览接口。
type FinanceSummarizer interface {
	QueryFinanceSummary(ctx context.Context, memberID string, target time.Time, period string) (expense.FinanceSummary, error)
}

// IncomeRecorder 收入写入接口。
type IncomeRecorder interface {
	RecordIncome(ctx context.Context, cmd expense.RecordIncomeCmd, memberID string) (id string, source string, duplicated bool, err error)
	UpdateIncome(ctx context.Context, id string, memberID string, cmd expense.UpdateIncomeCmd) (next expense.IncomeRecord, prev expense.IncomeRecord, err error)
}

// IncomeUndoWriter 撤销写入。
type IncomeUndoWriter interface {
	SaveUndo(ctx context.Context, memberID string, toolName string, undoData json.RawMessage) (undoID string, err error)
}

// ListIncomes GET /incomes —— 收入流水列表。
func ListIncomes(lister IncomeLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		limit, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		q := IncomeQuery{
			PageSize: limit,
			Cursor:   c.Query("cursor"),
			Source:   c.Query("source"),
		}
		if d := parseDate(c.Query("start_date")); d != nil {
			q.StartDate = d
		}
		if d := parseDate(c.Query("end_date")); d != nil {
			q.EndDate = d
		}
		if yr, err := strconv.Atoi(c.Query("year")); err == nil && yr > 0 {
			q.Year = yr
		}
		if mo, err := strconv.Atoi(c.Query("month")); err == nil && mo > 0 && mo <= 12 {
			q.Month = mo
		}

		items, nextCursor, err := lister.ListIncomes(c.Request.Context(), memberID, q)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(200, gin.H{"items": items, "next_cursor": nextCursor})
	}
}

// CreateIncome POST /incomes —— 手动记录收入。
func CreateIncome(svc IncomeRecorder, undoWriter IncomeUndoWriter, pl PermissionLookup) gin.HandlerFunc {
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
			Source      *string `json:"source"`
			Hint        *string `json:"hint"`
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
		if body.Source == nil || *body.Source == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "收入来源不能为空", nil))
			return
		}

		occurredAt := time.Now()
		if body.OccurredAt != nil && *body.OccurredAt != "" {
			if t, err := time.Parse(time.RFC3339, *body.OccurredAt); err == nil {
				occurredAt = t
			} else if t, err := time.Parse("2006-01-02", *body.OccurredAt); err == nil {
				occurredAt = t
			}
		}

		hint := *body.Source
		if body.Hint != nil && *body.Hint != "" {
			hint = *body.Hint
		}

		cmd := expense.RecordIncomeCmd{
			AmountCents: *body.AmountCents,
			Source:      *body.Source,
			Hint:        hint,
			OccurredAt:  occurredAt,
		}
		id, source, duplicated, err := svc.RecordIncome(c.Request.Context(), cmd, memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		var undoID string
		if !duplicated {
			undoData, _ := json.Marshal(map[string]any{
				"income_id": id,
				"member_id": memberID,
			})
			uid, uerr := undoWriter.SaveUndo(c.Request.Context(), memberID, "record_income", undoData)
			if uerr != nil {
				abortWith(c, asAppErr(uerr))
				return
			}
			undoID = uid
		}

		c.JSON(201, gin.H{
			"id":           id,
			"member_id":    memberID,
			"source":       source,
			"amount_cents": *body.AmountCents,
			"hint":         hint,
			"occurred_at":  occurredAt.Format(time.RFC3339),
			"undoable":     !duplicated,
			"undo_id":      undoID,
			"duplicated":   duplicated,
		})
	}
}

// UpdateIncome PATCH /incomes/:incomeId —— 修正收入记录。
func UpdateIncome(svc IncomeRecorder, undoWriter IncomeUndoWriter, pl PermissionLookup) gin.HandlerFunc {
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

		incomeID := c.Param("incomeId")
		if incomeID == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少收入 id", nil))
			return
		}

		var body struct {
			AmountCents *int64  `json:"amount_cents"`
			Source      *string `json:"source"`
			Hint        *string `json:"hint"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数有误", err))
			return
		}

		cmd := expense.UpdateIncomeCmd{
			AmountCents: body.AmountCents,
			Source:      body.Source,
			Hint:        body.Hint,
		}
		next, prev, err := svc.UpdateIncome(c.Request.Context(), incomeID, memberID, cmd)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		undoData, _ := json.Marshal(map[string]any{
			"income_id": incomeID,
			"member_id": memberID,
			"restore": map[string]any{
				"amount_cents": prev.AmountCents,
				"source":       prev.Source,
				"hint":         prev.Hint,
			},
		})
		undoID, err := undoWriter.SaveUndo(c.Request.Context(), memberID, "update_income", undoData)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(200, gin.H{
			"id":           incomeID,
			"member_id":    memberID,
			"source":       next.Source,
			"amount_cents": next.AmountCents,
			"hint":         next.Hint,
			"undoable":     true,
			"undo_id":      undoID,
		})
	}
}

// IncomeSummary GET /incomes/summary —— 收入汇总。
func IncomeSummary(svc IncomeSummarizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		period := c.DefaultQuery("period", "month")
		target := parseYearMonth(c.Query("year"), c.Query("month"))

		s, err := svc.QueryIncomeSummary(c.Request.Context(), memberID, target, period)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		sources := make([]gin.H, 0, len(s.BySource))
		for src, cents := range s.BySource {
			sources = append(sources, gin.H{"source": src, "total_cents": cents})
		}
		c.JSON(200, gin.H{
			"total_cents": s.TotalCents,
			"sources":     sources,
		})
	}
}

// FinanceSummary GET /finance/summary —— 综合财务收支总览。
func FinanceSummaryHandler(svc FinanceSummarizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		period := c.DefaultQuery("period", "month")
		target := parseYearMonth(c.Query("year"), c.Query("month"))

		summary, err := svc.QueryFinanceSummary(c.Request.Context(), memberID, target, period)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		c.JSON(200, gin.H{
			"total_income_cents":  summary.TotalIncomeCents,
			"total_expense_cents": summary.TotalExpenseCents,
			"balance_cents":       summary.BalanceCents,
			"period":              summary.Period,
			"year":                summary.Year,
			"month":               summary.Month,
		})
	}
}

func parseYearMonth(yearStr, monthStr string) time.Time {
	now := time.Now()
	yr, err1 := strconv.Atoi(yearStr)
	mo, err2 := strconv.Atoi(monthStr)
	if err1 == nil && yr > 0 {
		if err2 == nil && mo >= 1 && mo <= 12 {
			return time.Date(yr, time.Month(mo), 1, 0, 0, 0, 0, time.Local)
		}
		return time.Date(yr, 1, 1, 0, 0, 0, 0, time.Local)
	}
	return now
}
