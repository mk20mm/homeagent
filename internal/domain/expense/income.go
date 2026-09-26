package expense

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// RecordIncomeCmd 记录收入命令。
type RecordIncomeCmd struct {
	AmountCents int64
	Source      string    // 工资/奖金/兼职/报销/退款/红包/利息/其他
	Hint        string    // 原始表述，如 "9月工资"
	OccurredAt  time.Time // 零值取当前时间
}

// OccurredOrNow 返回发生时间，零值兜底当前时间。
func (cmd RecordIncomeCmd) OccurredOrNow() time.Time {
	if cmd.OccurredAt.IsZero() {
		return time.Now()
	}
	return cmd.OccurredAt
}

// IncomeRecord 收入快照（撤销恢复用）。
type IncomeRecord struct {
	AmountCents int64
	Source      string
	Hint        string
}

// UpdateIncomeCmd 部分更新：nil 表示不改该字段。
type UpdateIncomeCmd struct {
	AmountCents *int64
	Source      *string
	Hint        *string
}

// HasChanges 至少一个字段被设置。
func (cmd UpdateIncomeCmd) HasChanges() bool {
	return cmd.AmountCents != nil || cmd.Source != nil || cmd.Hint != nil
}

// ApplyTo 应用变更。
func (cmd UpdateIncomeCmd) ApplyTo(prev IncomeRecord) IncomeRecord {
	next := prev
	if cmd.AmountCents != nil {
		next.AmountCents = *cmd.AmountCents
	}
	if cmd.Source != nil {
		next.Source = *cmd.Source
	}
	if cmd.Hint != nil {
		next.Hint = *cmd.Hint
	}
	if next.Source == "" {
		next.Source = ClassifyIncomeSource(next.Hint)
	}
	return next
}

// IncomeSummary 收入汇总。
type IncomeSummary struct {
	TotalCents int64            // 总收入（分）
	BySource   map[string]int64 // 来源明细（分）
}

// FinanceSummary 财务收支总览（结合总收入、总支出、结余）。
type FinanceSummary struct {
	TotalIncomeCents  int64  // 总收入（分）
	TotalExpenseCents int64  // 总支出（分）
	BalanceCents      int64  // 结余（分 = 收入 - 支出）
	Period            string // month / year
	Year              int    // 年份
	Month             int    // 月份（period=year 时为 0）
}

// IncomeRepo 仓储接口：定义收入持久化操作。
type IncomeRepo interface {
	CreateIncome(ctx context.Context, memberID string, cmd RecordIncomeCmd, idempotencyKey string) (incomeID string, err error)
	UpdateIncome(ctx context.Context, memberID string, incomeID string, cmd UpdateIncomeCmd) (prev IncomeRecord, err error)
	DeleteIncome(ctx context.Context, incomeID string) error
	IncomeSummary(ctx context.Context, memberID string, target time.Time, period string) (IncomeSummary, error)
}

// IncomeIdempotencyKey 幂等键：sha256(member+amount+source+day)。
func IncomeIdempotencyKey(memberID string, cmd RecordIncomeCmd) string {
	day := cmd.OccurredOrNow().Format("2006-01-02")
	h := sha256.Sum256([]byte(memberID + "|" + strconv.FormatInt(cmd.AmountCents, 10) + "|" + cmd.Source + "|" + cmd.Hint + "|" + day))
	return hex.EncodeToString(h[:])
}

// incomeSourceRules 收入来源归类规则。
var incomeSourceRules = []struct {
	re   *regexp.Regexp
	name string
}{
	{regexp.MustCompile(`工资|薪水|薪资|月薪|发薪|打卡工资`), "工资"},
	{regexp.MustCompile(`奖金|年终奖|绩效|季度奖|分红`), "奖金"},
	{regexp.MustCompile(`兼职|副业|外包|兼职收入|劳务`), "兼职"},
	{regexp.MustCompile(`报销|差旅报销|医疗报销|公司报销`), "报销"},
	{regexp.MustCompile(`退款|退货|退运费|差价退回`), "退款"},
	{regexp.MustCompile(`红包|转账|压岁钱|礼金`), "红包"},
	{regexp.MustCompile(`利息|理财|基金收益|股票|投资收益|分红收益`), "利息"},
}

// ClassifyIncomeSource 根据关键词自动识别收入来源，默认"其他"。
func ClassifyIncomeSource(hint string) string {
	h := strings.ToLower(hint)
	for _, r := range incomeSourceRules {
		if r.re.MatchString(h) {
			return r.name
		}
	}
	return "其他"
}

// RecordIncome 业务逻辑：校验→归类→幂等键→落库。
func (s *service) RecordIncome(ctx context.Context, cmd RecordIncomeCmd, memberID string) (string, string, bool, error) {
	if cmd.AmountCents <= 0 {
		return "", "", false, apperr.New(apperr.CodeInvalidInput, "金额必须大于 0", nil)
	}
	if cmd.Hint == "" {
		return "", "", false, apperr.New(apperr.CodeInvalidInput, "缺少收入描述", nil)
	}
	if cmd.Source == "" {
		cmd.Source = ClassifyIncomeSource(cmd.Hint)
	}

	key := IncomeIdempotencyKey(memberID, cmd)
	id, err := s.incomeRepo.CreateIncome(ctx, memberID, cmd, key)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) && ae.Code == apperr.CodeConflict {
			return id, cmd.Source, true, nil
		}
		return "", "", false, err
	}
	return id, cmd.Source, false, nil
}

// UpdateIncome 修正收入记录。
func (s *service) UpdateIncome(ctx context.Context, id string, memberID string, cmd UpdateIncomeCmd) (IncomeRecord, IncomeRecord, error) {
	if id == "" {
		return IncomeRecord{}, IncomeRecord{}, apperr.New(apperr.CodeInvalidInput, "收入 id 不能为空", nil)
	}
	if !cmd.HasChanges() {
		return IncomeRecord{}, IncomeRecord{}, apperr.New(apperr.CodeInvalidInput, "没有需要修正的字段", nil)
	}
	if cmd.AmountCents != nil && *cmd.AmountCents <= 0 {
		return IncomeRecord{}, IncomeRecord{}, apperr.New(apperr.CodeInvalidInput, "金额必须大于 0", nil)
	}
	prev, err := s.incomeRepo.UpdateIncome(ctx, memberID, id, cmd)
	if err != nil {
		return IncomeRecord{}, IncomeRecord{}, err
	}
	return cmd.ApplyTo(prev), prev, nil
}

// DeleteIncome 撤销收入（软删除）。
func (s *service) DeleteIncome(ctx context.Context, id string) error {
	if id == "" {
		return apperr.New(apperr.CodeInvalidInput, "收入 id 不能为空", nil)
	}
	return s.incomeRepo.DeleteIncome(ctx, id)
}

// QueryIncomeSummary 查询收入汇总。
func (s *service) QueryIncomeSummary(ctx context.Context, memberID string, target time.Time, period string) (IncomeSummary, error) {
	return s.incomeRepo.IncomeSummary(ctx, memberID, target, period)
}

// QueryFinanceSummary 综合财务总览（收入 - 支出 = 结余）。
func (s *service) QueryFinanceSummary(ctx context.Context, memberID string, target time.Time, period string) (FinanceSummary, error) {
	if period == "" {
		period = "month"
	}
	expSum, err := s.repo.Summary(ctx, memberID, target)
	if err != nil {
		return FinanceSummary{}, err
	}
	incSum, err := s.incomeRepo.IncomeSummary(ctx, memberID, target, period)
	if err != nil {
		return FinanceSummary{}, err
	}

	monthVal := int(target.Month())
	if period == "year" {
		monthVal = 0
	}

	return FinanceSummary{
		TotalIncomeCents:  incSum.TotalCents,
		TotalExpenseCents: expSum.TotalCents,
		BalanceCents:      incSum.TotalCents - expSum.TotalCents,
		Period:            period,
		Year:              target.Year(),
		Month:             monthVal,
	}, nil
}
