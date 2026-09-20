// Package meal 报饭领域：申报某日是否在家用餐，按人+日期幂等（领域模型不变量）。
package meal

import (
	"context"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// MealReport 报饭视图。
type MealReport struct {
	ID         string
	MemberID   string
	MemberName string
	Date       time.Time
	AtHome     bool
	Note       string
}

// ReportCmd 报饭命令。
type ReportCmd struct {
	AtHome bool
	Note   string
	Date   time.Time // 零值 = 今天
}

// MealMember 汇总中的成员（携带 id，供 REST 响应）。
type MealMember struct {
	ID   string
	Name string
}

// MealSummary 当日用餐汇总。
type MealSummary struct {
	Date       time.Time
	AtHome     []MealMember // 在家成员
	NotAtHome  []MealMember
	Unreported []MealMember // 未申报成员（缺口显式，P6）
	Total      int
}

// MealRepo 仓储接口（store 层实现）。
// 方法名避开 expense 的 Summary（Store 聚合上不能同名重载）。
type MealRepo interface {
	Upsert(ctx context.Context, memberID string, cmd ReportCmd) (id string, err error)
	RemoveReport(ctx context.Context, memberID string, date time.Time) error
	DailySummary(ctx context.Context, date time.Time) (MealSummary, error)
}

// Service 报饭领域服务。
type Service interface {
	Report(ctx context.Context, memberID string, cmd ReportCmd) (id string, err error)
	Cancel(ctx context.Context, memberID string, date time.Time) error
	Summary(ctx context.Context, date time.Time) (MealSummary, error)
}

func NewService(repo MealRepo) Service {
	return &service{repo: repo}
}

type service struct {
	repo MealRepo
}

// Report 申报：幂等（人+日），重复为更新。返回记录 id。
func (s *service) Report(ctx context.Context, memberID string, cmd ReportCmd) (string, error) {
	if memberID == "" {
		return "", apperr.New(apperr.CodePermission, "缺少成员身份", nil)
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

// Today 今日 0 点（本地时区）：领域默认日期，HTTP 边界与工具共用。
func Today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func today() time.Time {
	return Today()
}

// ParseDate 把 YYYY-MM-DD 解析为本地 0 点。time.Parse 得到 UTC 0 点，
// 与入库的 today()（本地 0 点）相差一个时区，会让 DateEQ 查询/撤销扑空。
func ParseDate(s string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local), nil
}
