package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MealReport 报饭：申报某日是否在家用餐，按人+日期幂等（领域模型不变量）。
type MealReport struct{ ent.Schema }

func (MealReport) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (MealReport) Fields() []ent.Field {
	return []ent.Field{
		field.Time("date").Comment("用餐日期（零点对齐）"),
		field.Bool("at_home").Comment("是否在家用餐"),
		field.String("note").Optional(),
	}
}

func (MealReport) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", Member.Type).Ref("meal_reports").Unique().Required(),
	}
}

func (MealReport) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("date").Edges("member").Unique(),
		index.Fields("date"),
	}
}
