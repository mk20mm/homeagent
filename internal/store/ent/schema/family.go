package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Family 家庭（单家庭自用，保留多家庭结构以备扩展）。
type Family struct{ ent.Schema }

func (Family) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (Family) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Default("我的家"),
		field.String("timezone").Default("Asia/Shanghai"),
		field.Bool("lunar").Default(true).Comment("农历支持"),
	}
}

func (Family) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("members", Member.Type),
		edge.To("categories", Category.Type),
		edge.To("runs", AgentRun.Type),
	}
}
