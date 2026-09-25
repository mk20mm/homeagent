// Package meal 报饭领域：申报某日是否在家用餐，按人+日期幂等（领域模型不变量）。
package meal

import (
	"context"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// MealReport 报饭视图。
type MealReport struct {
	ID       string
	MemberID string
	MemberName string
	Date     time.Time
	AtHome   bool
	Note     string
}

// ReportCmd 报饭命令。
type ReportCmd struct {
	AtHome bool
	Note   string
	Date   time.Time // 零值 = 今天
}

// MealSummary 当日用餐汇总。
type MealSummary struct {
	Date       time.Time
	AtHome     []string // 在家成员名
	NotAtHome  []string
	Total      int
}

// MealMemberReport 单条报饭+成员信息视图（handler 消费）。
type MealMemberReport struct {
	MemberID string
	Name     string
	AtHome   bool
}

// MealRepo 仓储接口（store 层实现）。
// 方法名避开 expense 的 Summary（Store 聚合上不能同名重载）。
type MealRepo interface {
	Upsert(ctx context.Context, memberID string, cmd ReportCmd) error
	RemoveReport(ctx context.Context, memberID string, date time.Time) error
	DailySummary(ctx context.Context, date time.Time) (MealSummary, error)
	ListMealReports(ctx context.Context, date time.Time) ([]MealMemberReport, error)
}

// Service 报饭领域服务。
type Service interface {
	Report(ctx context.Context, memberID string, cmd ReportCmd) error
	Cancel(ctx context.Context, memberID string, date time.Time) error
	Summary(ctx context.Context, date time.Time) (MealSummary, error)
	ListReports(ctx context.Context, date time.Time) ([]MealMemberReport, error)
}

func NewService(repo MealRepo) Service {
	return &service{repo: repo}
}

type service struct {
	repo MealRepo
}

// Report 申报：幂等（人+日），重复为更新。
func (s *service) Report(ctx context.Context, memberID string, cmd ReportCmd) error {
	if memberID == "" {
		return apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}
	if cmd.Date.IsZero() {
		cmd.Date = today()
	}
	return s.repo.Upsert(ctx, memberID, cmd)
}

// Cancel 撤销当日报饭（删除记录）。
func (s *service) Cancel(ctx context.Context, memberID string, date time.Time) error {
	if memberID == "" {
		return apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}
	if date.IsZero() {
		date = today()
	}
	return s.repo.RemoveReport(ctx, memberID, date)
}

func (s *service) Summary(ctx context.Context, date time.Time) (MealSummary, error) {
	if date.IsZero() {
		date = today()
	}
	return s.repo.DailySummary(ctx, date)
}

func (s *service) ListReports(ctx context.Context, date time.Time) ([]MealMemberReport, error) {
	if date.IsZero() {
		date = today()
	}
	return s.repo.ListMealReports(ctx, date)
}

func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
