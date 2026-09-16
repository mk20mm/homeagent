package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// LLMProvider LLM 供应商配置。api_key 加密存储，绝不落明文（ARCHITECTURE §6）。
type LLMProvider struct{ ent.Schema }

type ProviderName string

const (
	ProviderOpenAI    ProviderName = "openai"
	ProviderAnthropic ProviderName = "anthropic"
	ProviderDeepSeek  ProviderName = "deepseek"
	ProviderZhipu     ProviderName = "zhipu"
	ProviderOllama    ProviderName = "ollama"
)

func (LLMProvider) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (LLMProvider) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("name").Values(
			string(ProviderOpenAI), string(ProviderAnthropic), string(ProviderDeepSeek),
			string(ProviderZhipu), string(ProviderOllama),
		),
		field.String("base_url").Optional().Comment("为空用供应商默认"),
		field.String("api_key").Optional().Sensitive().Comment("应用层加密"),
		field.Bool("enabled").Default(true),
	}
}

func (LLMProvider) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("models", LLMModel.Type),
	}
}

// LLMModel 模型清单。会话内切换只改 model_id，工具集不变（调度器设计 §3）。
type LLMModel struct{ ent.Schema }

func (LLMModel) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (LLMModel) Fields() []ent.Field {
	return []ent.Field{
		field.String("model_name").Comment("供应商侧模型名，如 gpt-4o"),
		field.String("display_name"),
		field.Int("context_window").Optional(),
		field.Int("max_output").Optional(),
		field.Float("cost_in_per_1m").Optional().Comment("每百万输入 token 成本"),
		field.Float("cost_out_per_1m").Optional().Comment("每百万输出 token 成本"),
		field.JSON("capabilities", []string{}).Optional().Comment("vision/tools/reasoning"),
		field.Bool("enabled").Default(true),
		field.Bool("is_default").Default(false).Comment("全局默认模型（唯一）"),
	}
}

func (LLMModel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("provider", LLMProvider.Type).Ref("models").Unique().Required(),
		edge.To("conversations", Conversation.Type),
		edge.To("usages", LLMUsage.Type),
	}
}

func (LLMModel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("model_name").Edges("provider").Unique(),
	}
}

// LLMUsage 用量埋点，只增不改。网关统一写入（调度器设计 §3）。
type LLMUsage struct{ ent.Schema }

type UsageStatus string

const (
	UsageOK      UsageStatus = "ok"
	UsageError   UsageStatus = "error"
	UsageTimeout UsageStatus = "timeout"
)

func (LLMUsage) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, AppendMixin{}} }

func (LLMUsage) Fields() []ent.Field {
	return []ent.Field{
		field.String("trace_id"),
		field.Int("prompt_tokens").Default(0),
		field.Int("completion_tokens").Default(0),
		field.Float("cost").Default(0),
		field.Int("latency_ms").Default(0),
		field.Enum("status").Values(
			string(UsageOK), string(UsageError), string(UsageTimeout),
		).Default(string(UsageOK)),
		field.String("error").Optional(),
	}
}

func (LLMUsage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("model", LLMModel.Type).Ref("usages").Unique(),
		edge.From("conversation", Conversation.Type).Ref("usage").Unique(),
		edge.From("member", Member.Type).Ref("usages").Unique(),
	}
}

func (LLMUsage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("trace_id"),
		index.Fields("created_at").Edges("model"),
	}
}
