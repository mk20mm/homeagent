package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Notification 站内通知（ADR-006）。静默、事件级幂等、只增不改。
//
// 唯一键 (type, ref_id, member_id, scheduled_at) 保证同一事件只生成一条；
// read_at 为空 = 未读，角标数 = 未读计数（前端封顶 99+）。
type Notification struct{ ent.Schema }

type NotificationType string

const (
	NotifyTaskDue      NotificationType = "task_due"      // 任务到期
	NotifyChoreOverdue NotificationType = "chore_overdue" // 家务逾期（阶段 B）
	NotifyMealGap      NotificationType = "meal_gap"      // 报饭缺口
	NotifyBillDue      NotificationType = "bill_due"      // 周期账单（阶段 B）
	NotifyBudgetOver   NotificationType = "budget_over"   // 预算超支（阶段 B）
	NotifyReport       NotificationType = "report"        // 周报（阶段 B）
)

func (Notification) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, AppendMixin{}} }

func (Notification) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("type").Values(
			string(NotifyTaskDue),
			string(NotifyChoreOverdue),
			string(NotifyMealGap),
			string(NotifyBillDue),
			string(NotifyBudgetOver),
			string(NotifyReport),
		),
		field.String("title"),
		field.String("body"),
		field.String("ref_type").Optional().Comment("关联对象类型，如 task/meal_report"),
		field.String("ref_id").Optional().Comment("关联对象 id"),
		field.String("action_label").Optional().Comment("行动按钮文案，如「去打卡」"),
		field.String("action_path").Optional().Comment("行动跳转路径，如 /chores"),
		field.Time("scheduled_at").Comment("计划触发时间（幂等键的一部分）"),
		field.Time("read_at").Optional().Nillable().Comment("为空=未读"),
	}
}

func (Notification) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", Member.Type).Ref("notifications").Unique().Required(),
	}
}

func (Notification) Indexes() []ent.Index {
	return []ent.Index{
		// 幂等键：同一事件（类型+对象+成员+计划时间）只一条
		index.Fields("type", "ref_id", "scheduled_at").Edges("member").Unique(),
		// 未读角标查询：WHERE member_id=? AND read_at IS NULL
		index.Fields("read_at").Edges("member"),
	}
}
