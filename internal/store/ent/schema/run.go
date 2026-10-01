package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AgentRun 事务编排运行记录（ARCHITECTURE §5.5）。
type AgentRun struct{ ent.Schema }

type RunStatus string

const (
	RunStatusRunning              RunStatus = "running"
	RunStatusAwaitingInput        RunStatus = "awaiting_input"
	RunStatusAwaitingConfirmation RunStatus = "awaiting_confirmation"
	RunStatusWaitingExternal      RunStatus = "waiting_external"
	RunStatusCompleted            RunStatus = "completed"
	RunStatusPartial              RunStatus = "partial"
	RunStatusFailed               RunStatus = "failed"
	RunStatusCancelled            RunStatus = "cancelled"
)

type IntentKind string

const (
	IntentQuery   IntentKind = "query"
	IntentAct     IntentKind = "act"
	IntentPlan    IntentKind = "plan"
	IntentAmend   IntentKind = "amend"
	IntentControl IntentKind = "control"
)

func (AgentRun) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (AgentRun) Fields() []ent.Field {
	return []ent.Field{
		field.String("goal").Comment("用户期望得到的成果目标"),
		field.Enum("intent_kind").Values(
			string(IntentQuery), string(IntentAct), string(IntentPlan),
			string(IntentAmend), string(IntentControl),
		).Default(string(IntentAct)),
		field.Enum("status").Values(
			string(RunStatusRunning), string(RunStatusAwaitingInput),
			string(RunStatusAwaitingConfirmation), string(RunStatusWaitingExternal),
			string(RunStatusCompleted), string(RunStatusPartial),
			string(RunStatusFailed), string(RunStatusCancelled),
		).Default(string(RunStatusRunning)),
		field.Int("version").Default(1).Comment("乐观并发版本"),
		field.String("request_id").Optional().Comment("客户端幂等请求 ID"),
		field.JSON("requested_effects", []string{}).Optional().Comment("用户明确授权的作用域边界"),
	}
}

func (AgentRun) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("family", Family.Type).Ref("runs").Unique().Required(),
		edge.From("member", Member.Type).Ref("runs").Unique().Required(),
		edge.From("conversation", Conversation.Type).Ref("runs").Unique(),
		edge.To("steps", RunStep.Type),
		edge.To("events", RunEvent.Type),
	}
}

func (AgentRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status").Edges("member"),
		index.Fields("request_id"),
	}
}

// RunStep 运行步骤。
type RunStep struct{ ent.Schema }

type StepStatus string

const (
	StepStatusPending         StepStatus = "pending"
	StepStatusExecuting       StepStatus = "executing"
	StepStatusCommitted       StepStatus = "committed"
	StepStatusWaitingExternal StepStatus = "waiting_external"
	StepStatusFailed          StepStatus = "failed"
	StepStatusUnknown         StepStatus = "unknown"
	StepStatusSkipped         StepStatus = "skipped"
)

type StepWaitFor string

const (
	WaitForNone            StepWaitFor = "none"
	WaitForEntityCompleted StepWaitFor = "entity_completed"
	WaitForScheduledAt     StepWaitFor = "scheduled_at"
	WaitForUserConfirm     StepWaitFor = "user_confirm"
)

func (RunStep) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (RunStep) Fields() []ent.Field {
	return []ent.Field{
		field.Int("position").Default(0).Comment("稳定排序序号"),
		field.String("tool_name").Comment("调用的工具名"),
		field.Text("input_json").Default("{}").Comment("校验后的入参快照"),
		field.Enum("status").Values(
			string(StepStatusPending), string(StepStatusExecuting),
			string(StepStatusCommitted), string(StepStatusWaitingExternal),
			string(StepStatusFailed), string(StepStatusUnknown), string(StepStatusSkipped),
		).Default(string(StepStatusPending)),
		field.String("operation_key").Optional().Comment("步骤原子幂等操作键"),
		field.JSON("depends_on_step_ids", []string{}).Optional().Comment("前置步骤 UUID 列表"),
		field.Enum("wait_for").Values(
			string(WaitForNone), string(WaitForEntityCompleted),
			string(WaitForScheduledAt), string(WaitForUserConfirm),
		).Default(string(WaitForNone)),
		field.String("completion_kind").Default("local_committed").Comment("local_committed / observed_device_success / member_confirmed / plan_saved"),
		field.Text("entity_ref").Optional().Comment("绑定的实体引用快照"),
		field.Text("result_card").Optional().Comment("步骤产出的结果卡片"),
		field.Int("expected_version").Default(1).Comment("预期版本"),
		field.String("confirmation_nonce").Optional().Comment("二次确认随机串"),
	}
}

func (RunStep) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", AgentRun.Type).Ref("steps").Unique().Required(),
	}
}

func (RunStep) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("position").Edges("run"),
		index.Fields("operation_key"),
	}
}

// RunEvent 调度事件，只增不改。
type RunEvent struct{ ent.Schema }

func (RunEvent) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, AppendMixin{}} }

func (RunEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int("seq").Default(1).Comment("单 Run 内递增序列号"),
		field.String("step_id").Optional().Comment("关联的步骤 ID"),
		field.String("event_type").Comment("事件类型，如 step_started, step_completed"),
		field.Text("payload").Default("{}").Comment("事件详情 JSON"),
	}
}

func (RunEvent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", AgentRun.Type).Ref("events").Unique().Required(),
	}
}

func (RunEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("seq").Edges("run"),
	}
}
