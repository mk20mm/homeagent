package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Income 收入流水。撤销 = 软删除回滚。金额存分（财务系统最小货币单位）。
type Income struct{ ent.Schema }

func (Income) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (Income) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("amount_cents"),
		field.String("source").Comment("收入来源：工资/奖金/兼职/报销/退款/红包/利息/其他"),
		field.String("hint").Optional().Comment("原始表述，如 9月工资入账"),
		field.String("note").Optional(),
		field.Time("occurred_at").Comment("发生时间，记账幂等键组成部分"),
		field.String("idempotency_key").Unique().Comment("sha256(member+amount+source+day)"),
		field.JSON("meta", map[string]any{}).Optional(),
	}
}

func (Income) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", Member.Type).Ref("incomes").Unique().Required(),
	}
}

func (Income) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("occurred_at").Edges("member"),
		index.Fields("source").Edges("member"),
	}
}
