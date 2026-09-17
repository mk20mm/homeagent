// Package expense 财务领域：记账、归类、汇总、撤销（ARCHITECTURE §2.3）。
//
// 不变量（AI-PRD §3）：
//   - 金额一律 int64 分；元→分转换只在工具边界做，领域层不见 float
//   - 归类规则在代码，不由 LLM 算钱
//   - 记账幂等键 sha256(member+amount+hint+day)，库层 unique 兜底
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

// RecordExpenseCmd 领域命令：金额已是分（边界处由工具转换完毕）。
type RecordExpenseCmd struct {
	AmountCents int64
	Hint        string    // 原始表述，如 "买菜"
	Category    string    // 空则规则归类
	OccurredAt  time.Time // 零值取当前时间
}

// OccurredOrNow 返回发生时间，零值兜底当前时间。
func (cmd RecordExpenseCmd) OccurredOrNow() time.Time {
	if cmd.OccurredAt.IsZero() {
		return time.Now()
	}
	return cmd.OccurredAt
}

// ExpenseSummary 本月汇总（query_budget 用）。
type ExpenseSummary struct {
	TotalCents  int64            // 本月已花（分）
	ByCategory  map[string]int64 // 分类明细（分）
	BudgetCents int64            // 全部分类预算合计，0 表示未设
}

// ExpenseRepo 仓储接口：领域层定义，store 层实现（依赖单向，AGENTS.md §7）。
// Create 幂等冲突时返回已存在账单 id 与 CodeConflict。
type ExpenseRepo interface {
	Create(ctx context.Context, memberID string, cmd RecordExpenseCmd, idempotencyKey string) (expenseID string, err error)
	Delete(ctx context.Context, expenseID string) error
	Summary(ctx context.Context, memberID string, month time.Time) (ExpenseSummary, error)
}

// Service 财务领域服务（工具通过它操作账单，不直接碰 ent.Client）。
type Service interface {
	// RecordExpense 记账；duplicated=true 表示幂等命中（今天已记过同样的一笔，未重复入库）。
	RecordExpense(ctx context.Context, cmd RecordExpenseCmd, memberID string) (id string, category string, duplicated bool, err error)
	DeleteExpense(ctx context.Context, id string) error
	QueryBudget(ctx context.Context, memberID string) (ExpenseSummary, error)
}

func NewService(repo ExpenseRepo) Service {
	return &service{repo: repo}
}

type service struct {
	repo ExpenseRepo
}

// RecordExpense 记账：校验→归类→幂等键→入库。幂等命中返回原账单，duplicated=true。
func (s *service) RecordExpense(ctx context.Context, cmd RecordExpenseCmd, memberID string) (string, string, bool, error) {
	if cmd.AmountCents <= 0 {
		return "", "", false, apperr.New(apperr.CodeInvalidInput, "金额必须大于 0", nil)
	}
	if cmd.Hint == "" {
		return "", "", false, apperr.New(apperr.CodeInvalidInput, "缺少消费内容", nil)
	}
	if cmd.Category == "" {
		cmd.Category = ClassifyCategory(cmd.Hint)
	}

	key := IdempotencyKey(memberID, cmd)
	id, err := s.repo.Create(ctx, memberID, cmd, key)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) && ae.Code == apperr.CodeConflict {
			// 幂等命中：返回已存在账单，未重复入库（调用方应据此提示用户）
			return id, cmd.Category, true, nil
		}
		return "", "", false, err
	}
	return id, cmd.Category, false, nil
}

func (s *service) DeleteExpense(ctx context.Context, id string) error {
	if id == "" {
		return apperr.New(apperr.CodeInvalidInput, "账单 id 不能为空", nil)
	}
	return s.repo.Delete(ctx, id)
}

func (s *service) QueryBudget(ctx context.Context, memberID string) (ExpenseSummary, error) {
	return s.repo.Summary(ctx, memberID, time.Now())
}

// IdempotencyKey 记账幂等键 sha256(member+amount+hint+day)。
func IdempotencyKey(memberID string, cmd RecordExpenseCmd) string {
	day := cmd.OccurredOrNow().Format("2006-01-02")
	h := sha256.Sum256([]byte(memberID + "|" + strconv.FormatInt(cmd.AmountCents, 10) + "|" + cmd.Hint + "|" + day))
	return hex.EncodeToString(h[:])
}

// classifyRules 归类规则表（在代码，不由 LLM 算钱）。
var classifyRules = []struct {
	re   *regexp.Regexp
	name string
}{
	{regexp.MustCompile(`菜|肉|蛋|奶|水果|蔬菜|超市|市场|生鲜|粮油`), "食材"},
	{regexp.MustCompile(`外卖|美团|饿了么|配送|骑手|肯德基|麦当劳`), "外卖"},
	{regexp.MustCompile(`打车|出租|地铁|公交|加油|停车|高铁|机票`), "出行"},
	{regexp.MustCompile(`饭|餐|面|火锅|烧烤|奶茶|咖啡|饮料|零食`), "餐饮"},
	{regexp.MustCompile(`纸巾|洗衣|清洁|日用|牙膏|洗发|沐浴|垃圾袋`), "日用"},
}

// ClassifyCategory 按 hint 关键词归类，未命中返回"其他"。
func ClassifyCategory(hint string) string {
	h := strings.ToLower(hint)
	for _, r := range classifyRules {
		if r.re.MatchString(h) {
			return r.name
		}
	}
	return "其他"
}
