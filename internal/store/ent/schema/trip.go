package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TripRecord 出行与用车记录（加油/充电/停车/保养/高速费，联动财务记账）。
type TripRecord struct{ ent.Schema }

func (TripRecord) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (TripRecord) Fields() []ent.Field {
	return []ent.Field{
		field.String("trip_type").Comment("gas | charging | parking | toll | maintenance | ride"),
		field.Int64("amount_cents").Default(0).Comment("费用金额（分）"),
		field.Int("mileage").Default(0).Comment("发生时里程（公里）"),
		field.String("note").Optional().Comment("备注"),
		field.Time("occurred_at").Comment("发生时间"),
		field.String("idempotency_key").Optional().Unique().Comment("sha256(vehicle+type+amount+time)"),
	}
}

func (TripRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("vehicle", Vehicle.Type).Ref("trips").Unique(),
		edge.From("member", Member.Type).Ref("trips").Unique(),
		edge.To("expense", Expense.Type).Unique().Comment("联动的财务支出记录"),
	}
}

func (TripRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("occurred_at"),
		index.Fields("trip_type"),
	}
}
