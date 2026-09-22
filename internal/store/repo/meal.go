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

// Upsert 报饭：同日重复为更新（member+date 唯一索引兜底）。返回记录 id。
func (s *Store) Upsert(ctx context.Context, memberID string, cmd dommeal.ReportCmd) (string, error) {
	// 与入库口径一致用 UTC 比较（见 store.UseUTCTimes）
	day := cmd.Date.UTC()
	// 先查当日记录：存在则更新（幂等），不存在则插入
	exist, err := s.db.MealReport.Query().
		Where(mealreport.HasMemberWith(member.IDEQ(toUUID(memberID))), mealreport.DateEQ(day)).
		Only(ctx)
	if err == nil && exist != nil {
		if err := s.db.MealReport.UpdateOneID(exist.ID).
			SetAtHome(cmd.AtHome).
			SetNote(cmd.Note).
			Exec(ctx); err != nil {
			return "", apperr.New(apperr.CodeInternal, "更新报饭失败", err)
		}
		return exist.ID.String(), nil
	}
	// ent NotFound → 插入；其他错误返回
	if !ent.IsNotFound(err) {
		return "", apperr.New(apperr.CodeInternal, "查询报饭记录失败", err)
	}

	created, err := s.db.MealReport.Create().
		SetDate(day).
		SetAtHome(cmd.AtHome).
		SetNote(cmd.Note).
		SetMemberID(toUUID(memberID)).
		Save(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeInternal, "报饭失败", err)
	}
	return created.ID.String(), nil
}

// RemoveReport 删除当日报饭（撤销）。
func (s *Store) RemoveReport(ctx context.Context, memberID string, date time.Time) error {
	n, err := s.db.MealReport.Delete().
		Where(mealreport.HasMemberWith(member.IDEQ(toUUID(memberID))), mealreport.DateEQ(date.UTC())).
		Exec(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "撤销报饭失败", err)
	}
	if n == 0 {
		return apperr.New(apperr.CodeNotFound, "当日报饭记录不存在", nil)
	}
	return nil
}

// DailySummary 当日用餐汇总：在家/不在家名单 + 未申报缺口（全体活跃成员差集）。
func (s *Store) DailySummary(ctx context.Context, date time.Time) (dommeal.MealSummary, error) {
	list, err := s.db.MealReport.Query().
		Where(mealreport.DateEQ(date.UTC())).
		WithMember().
		All(ctx)
	if err != nil {
		return dommeal.MealSummary{}, apperr.New(apperr.CodeInternal, "查询报饭汇总失败", err)
	}
	reported := make(map[string]dommeal.MealMember, len(list))
	out := dommeal.MealSummary{Date: date, Total: len(list)}
	for _, r := range list {
		if r.Edges.Member == nil {
			continue
		}
		m := dommeal.MealMember{ID: r.Edges.Member.ID.String(), Name: r.Edges.Member.Name}
		reported[m.ID] = m
		if r.AtHome {
			out.AtHome = append(out.AtHome, m)
		} else {
			out.NotAtHome = append(out.NotAtHome, m)
		}
	}

	// 未申报 = 全体活跃成员 - 已申报（做饭人要知道还缺谁）
	all, err := s.db.Member.Query().Where(member.ActiveEQ(true)).All(ctx)
	if err != nil {
		return dommeal.MealSummary{}, apperr.New(apperr.CodeInternal, "查询家庭成员失败", err)
	}
	for _, m := range all {
		if _, ok := reported[m.ID.String()]; ok {
			continue
		}
		out.Unreported = append(out.Unreported, dommeal.MealMember{ID: m.ID.String(), Name: m.Name})
	}
	return out, nil
}
