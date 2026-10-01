package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Vehicle 家庭爱车台账（纯电/燃油/混动、当前里程、上次/下次保养里程）。
type Vehicle struct{ ent.Schema }

func (Vehicle) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (Vehicle) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Comment("车辆名称，如 特斯拉 Model Y"),
		field.String("plate_number").Comment("车牌号"),
		field.String("vehicle_type").Default("ev").Comment("ev | gas | hybrid"),
		field.Int("current_mileage").Default(0).Comment("当前总里程（公里）"),
		field.Int("last_maintenance_mileage").Default(0).Comment("上次保养里程"),
		field.Int("next_maintenance_mileage").Default(0).Comment("下次建议保养里程"),
		field.String("note").Optional().Comment("备注"),
	}
}

func (Vehicle) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("trips", TripRecord.Type),
	}
}
