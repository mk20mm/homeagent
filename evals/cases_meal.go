// 用餐模块金标准任务（5 条）。
//
// 覆盖：报饭（人+日幂等）、撤销报饭、晚餐建议（只读）。
// 报饭撤销数据带真实日期（服务端归一化），撤销不会扑空——这是 T-A10 修过的坑。
package evals

import (
	"github.com/mk20mm/homeagent/internal/agent/runtime"
)

// mealCases 用餐模块用例。
func mealCases() []Case {
	return []Case{
		// M1：报饭不回家（中风险写，可撤销）
		{
			ID:          "M1",
			Category:    CategoryCapability,
			Module:      "meal",
			Description: "「今晚我不回来吃」→ report_meal(at_home=false)",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "今晚我不回来吃",
					Script:   toolScript("report_meal", map[string]any{"at_home": false}, "已报饭：今晚不回家吃。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 report_meal", Metric: MetricTool, Pass: toolSucceeded(ev, "report_meal")},
					{Name: "意图识别：报饭", Metric: MetricIntent, Pass: toolSucceeded(ev, "report_meal")},
					{Name: "报饭入库 at_home=false", Metric: MetricOutcome, Pass: checkMealAtHome(s, "m-parent", false)},
					{Name: "可撤销记录", Metric: MetricOutcome, Pass: checkUndoable(s, "report_meal")},
				}
			},
		},

		// M2：报饭回家
		{
			ID:          "M2",
			Category:    CategoryTool,
			Module:      "meal",
			Description: "「今晚在家吃」→ report_meal(at_home=true)",
			Steps: []Step{
				{
					MemberID: "m-elder",
					Content:  "今晚在家吃",
					Script:   toolScript("report_meal", map[string]any{"at_home": true}, "已报饭：今晚在家吃。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 report_meal", Metric: MetricTool, Pass: toolSucceeded(ev, "report_meal")},
					{Name: "at_home=true", Metric: MetricOutcome, Pass: checkMealAtHome(s, "m-elder", true)},
				}
			},
		},

		// M3：同日重复报饭 = 更新（人+日幂等）
		{
			ID:          "M3",
			Category:    CategoryTool,
			Module:      "meal",
			Description: "同一日报两次 → 更新为最新值",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "今晚我不回来吃",
					Script:   toolScript("report_meal", map[string]any{"at_home": false}, "已报饭。"),
				},
				{
					MemberID: "m-parent",
					Content:  "等等，我回来吃",
					Script:   toolScript("report_meal", map[string]any{"at_home": true}, "已更新报饭。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "仍只有一条报饭", Metric: MetricOutcome, Pass: len(s.Meals.byID) == 1},
					{Name: "值为最新 at_home=true", Metric: MetricOutcome, Pass: checkMealAtHome(s, "m-parent", true)},
				}
			},
		},

		// M4：晚餐建议（只读，无副作用）
		{
			ID:          "M4",
			Category:    CategoryCapability,
			Module:      "meal",
			Description: "「今晚吃什么」→ suggest_dinner 只读",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "今晚吃什么",
					Script:   toolScript("suggest_dinner", map[string]any{}, "建议做「番茄炒蛋」。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 suggest_dinner", Metric: MetricTool, Pass: toolSucceeded(ev, "suggest_dinner")},
					{Name: "意图识别：问菜单", Metric: MetricIntent, Pass: toolSucceeded(ev, "suggest_dinner")},
					{Name: "只读无报饭记录", Metric: MetricOutcome, Pass: len(s.Meals.byID) == 0},
				}
			},
		},

		// M5：撤销报饭（两步）
		{
			ID:          "M5",
			Category:    CategoryTool,
			Module:      "meal",
			Description: "报饭后撤销 → 记录删除",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "今晚我不回来吃",
					Script:   toolScript("report_meal", map[string]any{"at_home": false}, "已报饭。"),
				},
				{
					MemberID: "m-parent",
					Content:  "取消刚才的报饭",
					Script:   toolScript("undo_last", map[string]any{}, "已撤销：报饭。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "撤销成功", Metric: MetricUndo, Pass: toolSucceeded(ev, "undo_last")},
					{Name: "报饭记录已删", Metric: MetricUndo, Pass: len(s.Meals.byID) == 0},
				}
			},
		},
	}
}

// checkMealAtHome 断言成员当日报饭状态。
func checkMealAtHome(s *Sandbox, memberID string, atHome bool) bool {
	for _, m := range s.Meals.byID {
		if m.memberID == memberID {
			return m.atHome == atHome
		}
	}
	return false
}
