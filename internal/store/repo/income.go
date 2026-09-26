package repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/apperr"
	domexp "github.com/mk20mm/homeagent/internal/domain/expense"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/income"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// 编译期接口实现检查
var _ domexp.IncomeRepo = (*Store)(nil)
var _ v1.IncomeLister = (*Store)(nil)

// CreateIncome 创建收入记录。幂等冲突时返回已存在收入 id + CodeConflict。
func (s *Store) CreateIncome(ctx context.Context, memberID string, cmd domexp.RecordIncomeCmd, idempotencyKey string) (string, error) {
	m, err := s.db.Member.Query().
		Where(member.IDEQ(toUUID(memberID))).
		Only(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}

	occur := cmd.OccurredOrNow()
	create := s.db.Income.Create().
		SetAmountCents(cmd.AmountCents).
		SetSource(cmd.Source).
		SetHint(cmd.Hint).
		SetOccurredAt(occur).
		SetIdempotencyKey(idempotencyKey).
		SetMember(m)

	saved, err := create.Save(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			exist, qerr := s.db.Income.Query().
				Where(income.IdempotencyKeyEQ(idempotencyKey), income.DeletedAtIsNil()).
				Only(ctx)
			if qerr != nil || exist == nil {
				return "", apperr.New(apperr.CodeConflict, "收入已存在但回查失败", err)
			}
			return exist.ID.String(), apperr.New(apperr.CodeConflict, "收入已存在", nil)
		}
		return "", apperr.New(apperr.CodeInternal, "记收入失败", err)
	}
	return saved.ID.String(), nil
}

// GetIncome 取收入记录（member 隔离 + 软删除过滤）。
func (s *Store) GetIncome(ctx context.Context, memberID string, incomeID string) (domexp.IncomeRecord, error) {
	inc, err := s.db.Income.Query().
		Where(
			income.IDEQ(toUUID(incomeID)),
			income.HasMemberWith(member.IDEQ(toUUID(memberID))),
			income.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		return domexp.IncomeRecord{}, apperr.New(apperr.CodeNotFound, "收入记录不存在", err)
	}
	return domexp.IncomeRecord{
		AmountCents: inc.AmountCents,
		Source:      inc.Source,
		Hint:        inc.Hint,
	}, nil
}

// UpdateIncome 修正收入记录（member 隔离 + 软删除过滤），返回旧值快照供撤销恢复。
func (s *Store) UpdateIncome(ctx context.Context, memberID string, incomeID string, cmd domexp.UpdateIncomeCmd) (domexp.IncomeRecord, error) {
	prev, err := s.GetIncome(ctx, memberID, incomeID)
	if err != nil {
		return domexp.IncomeRecord{}, err
	}
	next := cmd.ApplyTo(prev)

	update := s.db.Income.UpdateOneID(toUUID(incomeID)).
		SetAmountCents(next.AmountCents).
		SetSource(next.Source).
		SetHint(next.Hint)

	key := domexp.IncomeIdempotencyKey(memberID, domexp.RecordIncomeCmd{
		AmountCents: next.AmountCents,
		Source:      next.Source,
		Hint:        next.Hint,
	})
	update.SetIdempotencyKey(key)

	if _, err := update.Save(ctx); err != nil {
		if isUniqueViolation(err) {
			return domexp.IncomeRecord{}, apperr.New(apperr.CodeConflict, "修改后与今天另一笔重复", err)
		}
		return domexp.IncomeRecord{}, apperr.New(apperr.CodeInternal, "修正收入失败", err)
	}
	return prev, nil
}

// DeleteIncome 软删除收入记录（撤销 = 回滚 deleted_at）。
func (s *Store) DeleteIncome(ctx context.Context, incomeID string) error {
	n, err := s.db.Income.UpdateOneID(toUUID(incomeID)).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "撤销收入失败", err)
	}
	if n == nil || n.ID == uuid.Nil {
		return apperr.New(apperr.CodeNotFound, "收入记录不存在", nil)
	}
	return nil
}

// IncomeSummary 汇总指定周期收入（月/年）。
func (s *Store) IncomeSummary(ctx context.Context, memberID string, target time.Time, period string) (domexp.IncomeSummary, error) {
	var start, end time.Time
	if period == "year" {
		start = time.Date(target.Year(), 1, 1, 0, 0, 0, 0, target.Location())
		end = start.AddDate(1, 0, 0)
	} else {
		start = time.Date(target.Year(), target.Month(), 1, 0, 0, 0, 0, target.Location())
		end = start.AddDate(0, 1, 0)
	}

	list, err := s.db.Income.Query().
		Where(
			income.HasMemberWith(member.IDEQ(toUUID(memberID))),
			income.DeletedAtIsNil(),
			income.OccurredAtGTE(start),
			income.OccurredAtLT(end),
		).
		All(ctx)
	if err != nil {
		return domexp.IncomeSummary{}, apperr.New(apperr.CodeInternal, "查询收入失败", err)
	}

	bySource := make(map[string]int64)
	var total int64
	for _, inc := range list {
		total += inc.AmountCents
		src := inc.Source
		if src == "" {
			src = "其他"
		}
		bySource[src] += inc.AmountCents
	}

	return domexp.IncomeSummary{
		TotalCents: total,
		BySource:   bySource,
	}, nil
}

// ListIncomes 查询收入流水（游标分页 + 筛选）。
func (s *Store) ListIncomes(ctx context.Context, memberID string, q v1.IncomeQuery) ([]v1.IncomeItem, string, error) {
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	query := s.db.Income.Query().
		Where(
			income.HasMemberWith(member.IDEQ(toUUID(memberID))),
			income.DeletedAtIsNil(),
		)
	if q.Source != "" {
		query = query.Where(income.SourceEQ(q.Source))
	}
	if q.StartDate != nil {
		query = query.Where(income.OccurredAtGTE(*q.StartDate))
	}
	if q.EndDate != nil {
		query = query.Where(income.OccurredAtLT(q.EndDate.AddDate(0, 0, 1)))
	}
	if q.Year > 0 {
		if q.Month > 0 {
			start := time.Date(q.Year, time.Month(q.Month), 1, 0, 0, 0, 0, time.Local)
			end := start.AddDate(0, 1, 0)
			query = query.Where(income.OccurredAtGTE(start), income.OccurredAtLT(end))
		} else {
			start := time.Date(q.Year, 1, 1, 0, 0, 0, 0, time.Local)
			end := start.AddDate(1, 0, 0)
			query = query.Where(income.OccurredAtGTE(start), income.OccurredAtLT(end))
		}
	}
	if q.Cursor != "" {
		if t, err := time.Parse(time.RFC3339, q.Cursor); err == nil {
			query = query.Where(income.OccurredAtLT(t))
		}
	}

	list, err := query.
		Order(ent.Desc(income.FieldOccurredAt)).
		Limit(q.PageSize + 1).
		All(ctx)
	if err != nil {
		return nil, "", apperr.New(apperr.CodeInternal, "查询收入流水失败", err)
	}

	nextCursor := ""
	if len(list) > q.PageSize {
		nextCursor = list[q.PageSize-1].OccurredAt.Format(time.RFC3339)
		list = list[:q.PageSize]
	}

	out := make([]v1.IncomeItem, 0, len(list))
	for _, inc := range list {
		out = append(out, v1.IncomeItem{
			ID:          inc.ID.String(),
			MemberID:    memberID,
			Source:      inc.Source,
			AmountCents: inc.AmountCents,
			Hint:        inc.Hint,
			Note:        inc.Note,
			OccurredAt:  inc.OccurredAt.Format(time.RFC3339),
			Undoable:    inc.DeletedAt == nil,
		})
	}
	return out, nextCursor, nil
}
