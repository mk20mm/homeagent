package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Category 分类账本（食材/日用/外卖/出行…）。归类规则在代码，此表是分类主数据。
// monthly_budget_cents 供 query_budget 一期使用（二期可独立 budget 表）。
type Category struct{ ent.Schema }

func (Category) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (Category) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.String("icon").Optional(),
		field.Int("sort_order").Default(0),
		field.Bool("is_system").Default(true).Comment("种子数据标记"),
		field.Int64("monthly_budget_cents").Optional().Nillable().Comment("月度预算（分），可空"),
	}
}

func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("family", Family.Type).Ref("categories").Unique().Required(),
		edge.To("expenses", Expense.Type),
	}
}

func (Category) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Edges("family").Unique(),
	}
}

// Expense 支出流水。撤销 = 软删除回滚。金额存分（财务系统最小货币单位）。
type Expense struct{ ent.Schema }

func (Expense) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (Expense) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("amount_cents"),
		field.String("hint").Optional().Comment("原始表述，如 买菜"),
		field.String("note").Optional(),
		field.Time("occurred_at").Comment("消费时间，记账幂等键组成部分"),
		field.String("idempotency_key").Unique().Comment("sha256(member+amount+hint+day)"),
		field.JSON("meta", map[string]any{}).Optional(),
	}
}

func (Expense) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", Member.Type).Ref("expenses").Unique().Required(),
		edge.From("category", Category.Type).Ref("expenses").Unique(),
	}
}

func (Expense) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("occurred_at").Edges("member"),
		index.Fields("occurred_at").Edges("category"),
	}
}
