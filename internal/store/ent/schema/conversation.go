package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Conversation 会话：按 member_id 严格隔离（ARCHITECTURE §6）。
type Conversation struct{ ent.Schema }

func (Conversation) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (Conversation) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").Optional().Comment("为空时用首条消息截断"),
		field.Time("last_message_at").Optional().Nillable(),
	}
}

func (Conversation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", Member.Type).Ref("conversations").Unique().Required(),
		edge.From("model", LLMModel.Type).Ref("conversations").Unique().Comment("当前模型，为空用默认"),
		edge.To("messages", Message.Type),
		edge.To("usage", LLMUsage.Type),
		edge.To("runs", AgentRun.Type),
	}
}

func (Conversation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("last_message_at").Edges("member"),
	}
}

// Message 消息，只增不改（含工具调用回执）。
type Message struct{ ent.Schema }

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
	RoleSystem    MessageRole = "system"
)

func (Message) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, AppendMixin{}} }

func (Message) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("role").Values(
			string(RoleUser), string(RoleAssistant), string(RoleTool), string(RoleSystem),
		),
		field.Text("content"),
		field.String("tool_name").Optional().Comment("工具消息专用"),
		field.String("trace_id").Optional().Comment("关联工具执行链路"),
		field.Int("tokens").Optional().Comment("本条 token 数，用量统计用"),
	}
}

func (Message) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("conversation", Conversation.Type).Ref("messages").Unique().Required(),
	}
}

func (Message) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at").Edges("conversation"),
	}
}
