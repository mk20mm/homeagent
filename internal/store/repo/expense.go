package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	sqlite "modernc.org/sqlite"

	"github.com/mk20mm/homeagent/internal/apperr"
	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	domexp "github.com/mk20mm/homeagent/internal/domain/expense"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/category"
	"github.com/mk20mm/homeagent/internal/store/ent/expense"
	"github.com/mk20mm/homeagent/internal/store/ent/family"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// 编译期接口实现检查（缺失编译不过）。
var _ domexp.ExpenseRepo = (*Store)(nil)

// SQLITE_CONSTRAINT_UNIQUE modernc 未导出该常量，硬编码错误码。
const SQLITE_CONSTRAINT_UNIQUE = 2067

// Create 创建账单。幂等冲突（unique idempotency_key）时返回已存在账单 id + CodeConflict。
func (s *Store) Create(ctx context.Context, memberID string, cmd domexp.RecordExpenseCmd, idempotencyKey string) (string, error) {
	m, err := s.db.Member.Query().
		Where(member.IDEQ(toUUID(memberID))).
		WithFamily().
		Only(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}

	occur := cmd.OccurredOrNow()
	create := s.db.Expense.Create().
		SetAmountCents(cmd.AmountCents).
		SetHint(cmd.Hint).
		SetOccurredAt(occur).
		SetIdempotencyKey(idempotencyKey).
		SetMember(m)

	// 分类：关联到本家庭已有分类；不存在不报错（归类降级为“其他”）
	if cmd.Category != "" && m.Edges.Family != nil {
		cat, err := s.db.Category.Query().Where(
			category.HasFamilyWith(family.IDEQ(m.Edges.Family.ID)),
			category.NameEQ(cmd.Category),
		).Only(ctx)
		if err == nil {
			create.SetCategory(cat)
		}
	}

	saved, err := create.Save(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			exist, qerr := s.db.Expense.Query().
				Where(expense.IdempotencyKeyEQ(idempotencyKey), expense.DeletedAtIsNil()).
				Only(ctx)
			if qerr != nil || exist == nil {
				return "", apperr.New(apperr.CodeConflict, "账单已存在但回查失败", err)
			}
			// 幂等命中：返回已存在 id，配 CodeConflict 让领域层判定
			return exist.ID.String(), apperr.New(apperr.CodeConflict, "账单已存在", nil)
		}
		return "", apperr.New(apperr.CodeInternal, "记账失败", err)
	}
	return saved.ID.String(), nil
}

// Delete 软删除账单（撤销 = 回滚 deleted_at）。
func (s *Store) Delete(ctx context.Context, expenseID string) error {
	n, err := s.db.Expense.UpdateOneID(toUUID(expenseID)).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "撤销账单失败", err)
	}
	if n == nil || n.ID == uuid.Nil {
		return apperr.New(apperr.CodeNotFound, "账单不存在", nil)
	}
	return nil
}

// Summary 本月汇总：总额 + 分类明细 + 预算合计。
func (s *Store) Summary(ctx context.Context, memberID string, month time.Time) (domexp.ExpenseSummary, error) {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	end := start.AddDate(0, 1, 0)

	list, err := s.db.Expense.Query().
		Where(
			expense.HasMemberWith(member.IDEQ(toUUID(memberID))),
			expense.DeletedAtIsNil(),
			expense.OccurredAtGTE(start),
			expense.OccurredAtLT(end),
		).
		WithCategory().
		All(ctx)
	if err != nil {
		return domexp.ExpenseSummary{}, apperr.New(apperr.CodeInternal, "查询账单失败", err)
	}

	byCat := make(map[string]int64)
	var total int64
	for _, e := range list {
		total += e.AmountCents
		name := "其他"
		if e.Edges.Category != nil {
			name = e.Edges.Category.Name
		}
		byCat[name] += e.AmountCents
	}

	// 预算合计（本家庭全部分类）
	var budget int64
	cats, err := s.db.Category.Query().
		Where(category.HasFamilyWith(family.HasMembersWith(member.IDEQ(toUUID(memberID))))).
		All(ctx)
	if err == nil {
		for _, c := range cats {
			if c.MonthlyBudgetCents != nil {
				budget += *c.MonthlyBudgetCents
			}
		}
	}

	return domexp.ExpenseSummary{
		TotalCents:  total,
		ByCategory:  byCat,
		BudgetCents: budget,
	}, nil
}

// ListExpenses 记账流水（member 隔离 + 软删除过滤，occurred_at 倒序，游标分页）。
func (s *Store) ListExpenses(ctx context.Context, memberID string, q v1.ExpenseQuery) ([]v1.ExpenseItem, string, error) {
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	query := s.db.Expense.Query().
		Where(
			expense.HasMemberWith(member.IDEQ(toUUID(memberID))),
			expense.DeletedAtIsNil(),
		)
	if q.Category != "" {
		query = query.Where(expense.HasCategoryWith(category.NameEQ(q.Category)))
	}
	if q.StartDate != nil {
		query = query.Where(expense.OccurredAtGTE(*q.StartDate))
	}
	if q.EndDate != nil {
		// end_date 含端点：查到次日 0 点
		query = query.Where(expense.OccurredAtLT(q.EndDate.AddDate(0, 0, 1)))
	}
	if q.Cursor != "" {
		if t, err := time.Parse(time.RFC3339, q.Cursor); err == nil {
			query = query.Where(expense.OccurredAtLT(t))
		}
	}
	list, err := query.
		Order(ent.Desc(expense.FieldOccurredAt)).
		WithCategory().
		Limit(q.PageSize + 1).
		All(ctx)
	if err != nil {
		return nil, "", apperr.New(apperr.CodeInternal, "查询记账流水失败", err)
	}

	nextCursor := ""
	if len(list) > q.PageSize {
		nextCursor = list[q.PageSize-1].OccurredAt.Format(time.RFC3339)
		list = list[:q.PageSize]
	}

	out := make([]v1.ExpenseItem, 0, len(list))
	for _, e := range list {
		cat := "其他"
		if e.Edges.Category != nil {
			cat = e.Edges.Category.Name
		}
		out = append(out, v1.ExpenseItem{
			ID:          e.ID.String(),
			MemberID:    memberID,
			Category:    cat,
			AmountCents: e.AmountCents,
			Hint:        e.Hint,
			OccurredAt:  e.OccurredAt.Format(time.RFC3339),
		})
	}
	return out, nextCursor, nil
}

// isUniqueViolation 判定 SQLite 唯一约束冲突（幂等命中）。
// modernc 驱动：*sqlite.Error{code: 2067}；包装后兜底字符串匹配。
func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == SQLITE_CONSTRAINT_UNIQUE
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func toUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}
