// 账单模块金标准任务（6 条，对齐 §10.14「每模块 5–10 条真实任务」）。
//
// 脚本模拟 LLM 对真实输入的工具调用决策；断言走 Code/Rule Grader（§6.3）：
//   - intent/tool/param 维度看事件序列与卡片参数
//   - outcome 维度看内存仓储终态（账单入库金额/分类，边界处元→分）
package evals

import (
	"github.com/mk20mm/homeagent/internal/agent/runtime"
)

// expenseCases 账单模块用例。
func expenseCases() []Case {
	return []Case{
		// S1：记账+归类（意图识别 top 场景）
		{
			ID:          "S1",
			Category:    CategoryCapability,
			Module:      "expense",
			Description: "「今天买菜花了 120」→ record_expense(120, 食材)",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "今天买菜花了 120",
					Script:   toolScript("record_expense", map[string]any{"amount": 120, "hint": "买菜"}, "已记账 ¥120，食材类。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 record_expense", Metric: MetricTool, Pass: toolSucceeded(ev, "record_expense")},
					{Name: "意图识别：记账", Metric: MetricIntent, Pass: toolSucceeded(ev, "record_expense")},
					{Name: "金额抽取 120 元", Metric: MetricParam, Pass: checkExpenseAmount(s, 12000)},
					{Name: "归类食材", Metric: MetricParam, Pass: checkExpenseCategory(s, "食材")},
					{Name: "入库终态 + 可撤销记录", Metric: MetricOutcome, Pass: checkUndoable(s, "record_expense")},
				}
			},
		},

		// S2：记账+自动归类（打车 → 出行）
		{
			ID:          "S2",
			Category:    CategoryTool,
			Module:      "expense",
			Description: "「打车 35」→ record_expense(35, 出行)",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "打车 35",
					Script:   toolScript("record_expense", map[string]any{"amount": 35, "hint": "打车"}, "已记账 ¥35，出行类。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 record_expense", Metric: MetricTool, Pass: toolSucceeded(ev, "record_expense")},
					{Name: "金额 35 元 → 3500 分", Metric: MetricParam, Pass: checkExpenseAmount(s, 3500)},
					{Name: "自动归类出行", Metric: MetricParam, Pass: checkExpenseCategory(s, "出行")},
				}
			},
		},

		// S3：明确指定分类（LLM 传 category，绕过规则归类）
		{
			ID:          "S3",
			Category:    CategoryTool,
			Module:      "expense",
			Description: "「买日用品 88，算日用」→ 显式 category=日用",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "买日用品 88，算日用",
					Script:   toolScript("record_expense", map[string]any{"amount": 88, "hint": "日用品", "category": "日用"}, "已记账 ¥88，日用类。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 record_expense", Metric: MetricTool, Pass: toolSucceeded(ev, "record_expense")},
					{Name: "显式分类日用", Metric: MetricParam, Pass: checkExpenseCategory(s, "日用")},
				}
			},
		},

		// S4：查询预算（只读工具，意图识别）
		{
			ID:          "S4",
			Category:    CategoryCapability,
			Module:      "expense",
			Description: "「这个月花了多少」→ query_budget 只读",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "这个月花了多少",
					Script:   toolScript("query_budget", map[string]any{}, "本月已花 ¥120。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 query_budget", Metric: MetricTool, Pass: toolSucceeded(ev, "query_budget")},
					{Name: "意图识别：查预算", Metric: MetricIntent, Pass: toolSucceeded(ev, "query_budget")},
					{Name: "只读无副作用", Metric: MetricOutcome, Pass: len(s.Expenses.activeExpenses()) == 0 && len(s.UndoLog.activeRecords()) == 0},
				}
			},
		},

		// S5：撤销记账（两步：记账 → 对话撤销）
		{
			ID:          "S5",
			Category:    CategoryCapability,
			Module:      "expense",
			Description: "记账后「取消刚才那笔」→ undo_last 软删",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "买菜花了 120",
					Script:   toolScript("record_expense", map[string]any{"amount": 120, "hint": "买菜"}, "已记账。"),
				},
				{
					MemberID: "m-parent",
					Content:  "取消刚才那笔",
					Script:   toolScript("undo_last", map[string]any{}, "已撤销：记账。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "记账成功", Metric: MetricOutcome, Pass: toolSucceeded(ev, "record_expense")},
					{Name: "撤销成功", Metric: MetricUndo, Pass: toolSucceeded(ev, "undo_last")},
					{Name: "撤销后账单软删", Metric: MetricUndo, Pass: len(s.Expenses.activeExpenses()) == 0},
					{Name: "撤销记录标记已用", Metric: MetricUndo, Pass: len(s.UndoLog.activeRecords()) == 0},
				}
			},
		},

		// S6：幂等——同一天同一笔记账不重复入库
		{
			ID:          "S6",
			Category:    CategoryTool,
			Module:      "expense",
			Description: "同一笔记两次 → 幂等命中，只入库一条",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "买菜花了 120",
					Script:   toolScript("record_expense", map[string]any{"amount": 120, "hint": "买菜"}, "已记账。"),
				},
				{
					MemberID: "m-parent",
					Content:  "买菜花了 120",
					Script:   toolScript("record_expense", map[string]any{"amount": 120, "hint": "买菜"}, "今天已记过这笔。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				list := s.Expenses.activeExpenses()
				return []CheckResult{
					{Name: "只入库一条", Metric: MetricOutcome, Pass: len(list) == 1},
					{Name: "第二次幂等提示", Metric: MetricOutcome, Pass: cardHas(s, ev, "record_expense", "duplicated", true)},
				}
			},
		},
	}
}

// checkExpenseAmount 断言账单终态金额（分）。
func checkExpenseAmount(s *Sandbox, cents int64) bool {
	for _, e := range s.Expenses.activeExpenses() {
		if e.cents == cents {
			return true
		}
	}
	return false
}

// checkExpenseCategory 断言账单终态分类。
func checkExpenseCategory(s *Sandbox, category string) bool {
	for _, e := range s.Expenses.activeExpenses() {
		if e.category == category {
			return true
		}
	}
	return false
}

// checkUndoable 断言写操作留下了可撤销记录（ADR-004）。
func checkUndoable(s *Sandbox, toolName string) bool {
	for _, r := range s.UndoLog.activeRecords() {
		if r.toolName == toolName {
			return true
		}
	}
	return false
}

// cardHas 断言卡片字段值（duplicated 标记等）。
func cardHas(_ *Sandbox, ev []runtime.Event, toolName, key string, want any) bool {
	card, ok := cardOf(ev, toolName)
	if !ok {
		return false
	}
	return card[key] == want
}
