// 中枢/系统模块金标准任务（5 条）。
//
// 覆盖：多意图并行执行（S9 两个工具同轮）、纯对话不乱调工具、切换模型、
// 列表查询、撤销最近一笔（对话侧撤销入口）。
//
// 日程模块（FR-CAL）工具在阶段 B 才落地，本套先行覆盖跨模块联动与系统工具。
package evals

import (
	"github.com/mk20mm/homeagent/internal/agent/runtime"
)

// hubCases 中枢模块用例。
func hubCases() []Case {
	return []Case{
		// H1：多意图一句话（S9：记账 + 报饭，两个工具同轮执行）
		{
			ID:          "H1",
			Category:    CategoryCapability,
			Module:      "hub",
			Description: "「买菜120，今晚不回家吃」→ 两个工具同轮执行",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "买菜120，今晚不回家吃",
					Script: multiToolScript([]toolCallSpec{
						{name: "record_expense", args: map[string]any{"amount": 120, "hint": "买菜"}},
						{name: "report_meal", args: map[string]any{"at_home": false}},
					}, "已记账 ¥120，已报饭今晚不回家吃。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "记账执行", Metric: MetricTool, Pass: toolSucceeded(ev, "record_expense")},
					{Name: "报饭执行", Metric: MetricTool, Pass: toolSucceeded(ev, "report_meal")},
					{Name: "意图识别：多意图全中", Metric: MetricIntent, Pass: toolSucceeded(ev, "record_expense") && toolSucceeded(ev, "report_meal")},
					{Name: "记账金额 12000 分", Metric: MetricParam, Pass: checkExpenseAmount(s, 12000)},
					{Name: "报饭 at_home=false", Metric: MetricParam, Pass: checkMealAtHome(s, "m-parent", false)},
					{Name: "两笔均可撤销", Metric: MetricOutcome, Pass: checkUndoable(s, "record_expense") && checkUndoable(s, "report_meal")},
				}
			},
		},

		// H2：纯对话不调工具（意图识别负例，防过度调用）
		{
			ID:          "H2",
			Category:    CategoryCapability,
			Module:      "hub",
			Description: "「今天天气怎么样」→ 纯对话，不调工具",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "今天天气怎么样",
					Script:   chatOnlyScript("今天天气不错。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				toolCalled := false
				for _, e := range ev {
					if e.Type == "tool_call" {
						toolCalled = true
					}
				}
				return []CheckResult{
					{Name: "意图识别：闲聊不调工具", Metric: MetricIntent, Pass: !toolCalled},
					{Name: "无副作用", Metric: MetricOutcome, Pass: !toolCalled && len(s.Expenses.activeExpenses()) == 0 && len(s.Meals.byID) == 0},
				}
			},
		},

		// H3：切换会话模型（系统工具，可撤销切回）
		{
			ID:          "H3",
			Category:    CategoryTool,
			Module:      "hub",
			Description: "「换成 gpt-4o」→ switch_model",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "换成 gpt-4o",
					Script:   toolScript("switch_model", map[string]any{"model": "gpt-4o"}, "已切换到「GPT-4o」。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 switch_model", Metric: MetricTool, Pass: toolSucceeded(ev, "switch_model")},
					{Name: "会话模型已切换", Metric: MetricOutcome, Pass: checkConvModel(s, "m-gpt4o")},
				}
			},
		},

		// H4：列模型清单（只读）
		{
			ID:          "H4",
			Category:    CategoryTool,
			Module:      "hub",
			Description: "「有哪些模型」→ list_models 只读",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "有哪些模型",
					Script:   toolScript("list_models", map[string]any{}, "可用模型：DeepSeek 对话、GPT-4o。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 list_models", Metric: MetricTool, Pass: toolSucceeded(ev, "list_models")},
				}
			},
		},

		// H5：对话侧撤销（孩子撤自己的报饭——全员可撤销自己的操作）
		{
			ID:          "H5",
			Category:    CategoryCapability,
			Module:      "hub",
			Description: "孩子报饭后「取消我的报饭」→ undo_last",
			Steps: []Step{
				{
					MemberID: "m-child",
					Content:  "今晚我不回来吃",
					Script:   toolScript("report_meal", map[string]any{"at_home": false}, "已报饭。"),
				},
				{
					MemberID: "m-child",
					Content:  "取消我的报饭",
					Script:   toolScript("undo_last", map[string]any{}, "已撤销：报饭。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "孩子可报饭", Metric: MetricOutcome, Pass: toolSucceeded(ev, "report_meal")},
					{Name: "撤销成功", Metric: MetricUndo, Pass: toolSucceeded(ev, "undo_last")},
					{Name: "报饭记录已删", Metric: MetricUndo, Pass: len(s.Meals.byID) == 0},
				}
			},
		},
	}
}

// checkConvModel 断言任一会话已切到目标模型。
func checkConvModel(s *Sandbox, modelID string) bool {
	for _, id := range s.Models.conv {
		if id == modelID {
			return true
		}
	}
	return false
}
