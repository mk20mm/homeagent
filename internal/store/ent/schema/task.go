package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Task 家务任务。状态机：待认领→进行中→已完成；删除=软删（领域模型不变量）。
type Task struct{ ent.Schema }

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskDone       TaskStatus = "done"
)

type TaskRisk string

const (
	TaskRiskLow    TaskRisk = "low"
	TaskRiskMedium TaskRisk = "medium"
	TaskRiskHigh   TaskRisk = "high"
)

func (Task) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (Task) Fields() []ent.Field {
	return []ent.Field{
		field.String("title"),
		field.String("description").Optional(),
		field.Enum("risk").Values(
			string(TaskRiskLow), string(TaskRiskMedium), string(TaskRiskHigh),
		).Default(string(TaskRiskMedium)),
		field.Enum("status").Values(
			string(TaskPending), string(TaskInProgress), string(TaskDone),
		).Default(string(TaskPending)),
		field.Time("due_at").Optional().Nillable().Comment("到点提醒"),
		field.Time("completed_at").Optional().Nillable().Comment("打卡时间，撤销误打卡时回滚"),
		field.Int("points").Default(1).Comment("积分（三期）"),
	}
}

func (Task) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("assignee", Member.Type).Ref("tasks").Unique().Comment("指派人，可空=待认领"),
		edge.From("template", TaskTemplate.Type).Ref("tasks").Unique().Comment("来源模板，可空=自定义"),
	}
}

func (Task) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status").Edges("assignee"),
		index.Fields("status", "due_at"),
	}
}

// TaskTemplate 家务模板（如「每周三大扫除」的默认参数）。
type TaskTemplate struct{ ent.Schema }

func (TaskTemplate) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (TaskTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("title"),
		field.String("description").Optional(),
		field.Enum("risk").Values(
			string(TaskRiskLow), string(TaskRiskMedium), string(TaskRiskHigh),
		).Default(string(TaskRiskMedium)),
		field.Int("default_points").Default(1),
	}
}

func (TaskTemplate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tasks", Task.Type),
	}
}
