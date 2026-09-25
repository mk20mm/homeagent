package repo

import (
	"context"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
	dommeal "github.com/mk20mm/homeagent/internal/domain/meal"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/mealreport"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// 编译期接口实现检查。
var _ dommeal.MealRepo = (*Store)(nil)

// Upsert 报饭：同日重复为更新（member+date 唯一索引兜底）。
func (s *Store) Upsert(ctx context.Context, memberID string, cmd dommeal.ReportCmd) error {
	day := cmd.Date
	// 先查当日记录：存在则更新（幂等），不存在则插入
	exist, err := s.db.MealReport.Query().
		Where(mealreport.HasMemberWith(member.IDEQ(toUUID(memberID))), mealreport.DateEQ(day)).
		Only(ctx)
	if err == nil && exist != nil {
		if err := s.db.MealReport.UpdateOneID(exist.ID).
			SetAtHome(cmd.AtHome).
			SetNote(cmd.Note).
			Exec(ctx); err != nil {
			return apperr.New(apperr.CodeInternal, "更新报饭失败", err)
		}
		return nil
	}
	// ent NotFound → 插入；其他错误返回
	if !ent.IsNotFound(err) {
		return apperr.New(apperr.CodeInternal, "查询报饭记录失败", err)
	}

	if err := s.db.MealReport.Create().
		SetDate(day).
		SetAtHome(cmd.AtHome).
		SetNote(cmd.Note).
		SetMemberID(toUUID(memberID)).
		Exec(ctx); err != nil {
		return apperr.New(apperr.CodeInternal, "报饭失败", err)
	}
	return nil
}

// RemoveReport 删除当日报饭（撤销）。
func (s *Store) RemoveReport(ctx context.Context, memberID string, date time.Time) error {
	n, err := s.db.MealReport.Delete().
		Where(mealreport.HasMemberWith(member.IDEQ(toUUID(memberID))), mealreport.DateEQ(date)).
		Exec(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "撤销报饭失败", err)
	}
	if n == 0 {
		return apperr.New(apperr.CodeNotFound, "当日报饭记录不存在", nil)
	}
	return nil
}

// DailySummary 当日用餐汇总：在家/不在家成员名单。
func (s *Store) DailySummary(ctx context.Context, date time.Time) (dommeal.MealSummary, error) {
	list, err := s.db.MealReport.Query().
		Where(mealreport.DateEQ(date)).
		WithMember().
		All(ctx)
	if err != nil {
		return dommeal.MealSummary{}, apperr.New(apperr.CodeInternal, "查询报饭汇总失败", err)
	}
	out := dommeal.MealSummary{Date: date, Total: len(list)}
	for _, r := range list {
		if r.Edges.Member == nil {
			continue
		}
		name := r.Edges.Member.Name
		if r.AtHome {
			out.AtHome = append(out.AtHome, name)
		} else {
			out.NotAtHome = append(out.NotAtHome, name)
		}
	}
	return out, nil
}

// ListMealReports 查询某日的所有报饭记录并携带成员信息。
func (s *Store) ListMealReports(ctx context.Context, date time.Time) ([]dommeal.MealMemberReport, error) {
	list, err := s.db.MealReport.Query().
		Where(mealreport.DateEQ(date)).
		WithMember().
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询报饭列表失败", err)
	}
	var out []dommeal.MealMemberReport
	for _, r := range list {
		if r.Edges.Member == nil {
			continue
		}
		out = append(out, dommeal.MealMemberReport{
			MemberID: r.Edges.Member.ID.String(),
			Name:     r.Edges.Member.Name,
			AtHome:   r.AtHome,
		})
	}
	return out, nil
}
