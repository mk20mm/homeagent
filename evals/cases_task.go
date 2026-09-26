// 家务模块金标准任务（6 条）。
//
// 覆盖：派发（按名解析执行人）、打卡（状态机 + 归属）、查询（只读隔离）。
// 派发工具按 assignee_name 解析成员——沙箱 members 表对齐种子数据（爸爸/奶奶/孩子）。
package evals

import (
	"github.com/mk20mm/homeagent/internal/agent/runtime"
)

// taskCases 家务模块用例。
func taskCases() []Case {
	return []Case{
		// T1：派发任务（中风险写，可撤销）
		{
			ID:          "T1",
			Category:    CategoryCapability,
			Module:      "task",
			Description: "「提醒奶奶洗碗」→ assign_task(洗碗, 奶奶)",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "提醒奶奶洗碗",
					Script:   toolScript("assign_task", map[string]any{"title": "洗碗", "assignee_name": "奶奶"}, "已派发「洗碗」给 奶奶。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 assign_task", Metric: MetricTool, Pass: toolSucceeded(ev, "assign_task")},
					{Name: "意图识别：派任务", Metric: MetricIntent, Pass: toolSucceeded(ev, "assign_task")},
					{Name: "任务入库", Metric: MetricOutcome, Pass: len(activeTasks(s)) == 1},
					{Name: "执行人=奶奶", Metric: MetricParam, Pass: checkTaskAssignee(s, "m-elder")},
					{Name: "可撤销记录", Metric: MetricOutcome, Pass: checkUndoable(s, "assign_task")},
				}
			},
		},

		// T2：派发待认领任务（无 assignee）
		{
			ID:          "T2",
			Category:    CategoryTool,
			Module:      "task",
			Description: "「发个任务：倒垃圾」→ 待认领",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "发个任务：倒垃圾",
					Script:   toolScript("assign_task", map[string]any{"title": "倒垃圾"}, "已派发「倒垃圾」，待认领。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 assign_task", Metric: MetricTool, Pass: toolSucceeded(ev, "assign_task")},
					{Name: "待认领无执行人", Metric: MetricParam, Pass: checkTaskAssignee(s, "")},
				}
			},
		},

		// T3：打卡（三步：派发 → 开始做 → 完成，状态机 pending→in_progress→done）
		{
			ID:          "T3",
			Category:    CategoryCapability,
			Module:      "task",
			Description: "派发后两次打卡 → complete_task 到 done",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "提醒奶奶洗碗",
					Script:   toolScript("assign_task", map[string]any{"title": "洗碗", "assignee_name": "奶奶"}, "已派发。"),
				},
				{
					MemberID: "m-elder",
					Content:  "我开始洗碗了",
					Script:   toolScript("complete_task", map[string]any{"title": "洗碗"}, "已开始。"),
				},
				{
					MemberID: "m-elder",
					Content:  "碗我洗完了",
					Script:   toolScript("complete_task", map[string]any{"title": "洗碗"}, "已完成「洗碗」打卡。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 complete_task", Metric: MetricTool, Pass: toolSucceeded(ev, "complete_task")},
					{Name: "意图识别：打卡", Metric: MetricIntent, Pass: toolSucceeded(ev, "complete_task")},
					{Name: "打卡终态 done", Metric: MetricOutcome, Pass: checkTaskStatus(s, "done")},
				}
			},
		},

		// T4：重复打卡幂等（已 done 再打 → 冲突）
		{
			ID:          "T4",
			Category:    CategoryTool,
			Module:      "task",
			Description: "完成后重复打卡 → 幂等冲突，只计一次",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "提醒奶奶洗碗",
					Script:   toolScript("assign_task", map[string]any{"title": "洗碗", "assignee_name": "奶奶"}, "已派发。"),
				},
				{
					MemberID: "m-elder",
					Content:  "我开始洗碗了",
					Script:   toolScript("complete_task", map[string]any{"title": "洗碗"}, "已开始。"),
				},
				{
					MemberID: "m-elder",
					Content:  "碗我洗完了",
					Script:   toolScript("complete_task", map[string]any{"title": "洗碗"}, "已完成打卡。"),
				},
				{
					MemberID: "m-elder",
					Content:  "我又洗了一遍",
					Script:   toolScript("complete_task", map[string]any{"title": "洗碗"}, "任务已完成，勿重复打卡。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "第四次打卡被拒绝", Metric: MetricOutcome, Pass: toolDenied(ev, "complete_task")},
					{Name: "仍只完成一次", Metric: MetricOutcome, Pass: checkTaskStatus(s, "done")},
				}
			},
		},

		// T5：查我的任务（只读，返回本人任务）
		{
			ID:          "T5",
			Category:    CategoryCapability,
			Module:      "task",
			Description: "「我还有啥活」→ list_my_tasks 只读",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "提醒奶奶洗碗",
					Script:   toolScript("assign_task", map[string]any{"title": "洗碗", "assignee_name": "奶奶"}, "已派发。"),
				},
				{
					MemberID: "m-elder",
					Content:  "我还有啥活",
					Script:   toolScript("list_my_tasks", map[string]any{}, "有 1 项待办：洗碗。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "工具选择 list_my_tasks", Metric: MetricTool, Pass: toolSucceeded(ev, "list_my_tasks")},
					{Name: "意图识别：查任务", Metric: MetricIntent, Pass: toolSucceeded(ev, "list_my_tasks")},
					{Name: "只读无新增任务", Metric: MetricOutcome, Pass: len(activeTasks(s)) == 1},
				}
			},
		},

		// T6：撤销派发（两步：派发 → 撤销）
		{
			ID:          "T6",
			Category:    CategoryTool,
			Module:      "task",
			Description: "派发后撤销 → 任务软删",
			Steps: []Step{
				{
					MemberID: "m-parent",
					Content:  "提醒奶奶洗碗",
					Script:   toolScript("assign_task", map[string]any{"title": "洗碗", "assignee_name": "奶奶"}, "已派发。"),
				},
				{
					MemberID: "m-parent",
					Content:  "取消刚才的任务",
					Script:   toolScript("undo_last", map[string]any{}, "已撤销：派任务。"),
				},
			},
			Check: func(s *Sandbox, ev []runtime.Event) []CheckResult {
				return []CheckResult{
					{Name: "撤销成功", Metric: MetricUndo, Pass: toolSucceeded(ev, "undo_last")},
					{Name: "任务软删", Metric: MetricUndo, Pass: len(activeTasks(s)) == 0},
				}
			},
		},
	}
}

// checkTaskAssignee 断言任务执行人。
func checkTaskAssignee(s *Sandbox, memberID string) bool {
	for _, t := range s.Tasks.byID {
		if t.assigneeID == memberID {
			return true
		}
	}
	return false
}

// checkTaskStatus 断言（首个）任务终态。
func checkTaskStatus(s *Sandbox, status string) bool {
	for _, t := range s.Tasks.byID {
		return string(t.status) == status
	}
	return false
}

// activeTasks 未删除的任务（终态断言用）。
func activeTasks(s *Sandbox) []*memTask {
	out := make([]*memTask, 0, len(s.Tasks.byID))
	for _, t := range s.Tasks.byID {
		if !t.deleted {
			out = append(out, t)
		}
	}
	return out
}
