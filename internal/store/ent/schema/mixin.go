// Package schema 定义全部实体（ent）。按领域拆文件，共用两个 Mixin。
//
// 约定（docs/CONVENTIONS-backend.md §6 + ADR-004）：
//   - 主键一律 UUID（对齐领域模型 string id 与前端）
//   - TimeMixin：软删除 + 时间戳，撤销 = deleted_at 回滚
//   - AppendMixin：只增不改（消息/用量/审计/撤销记录），仅 created_at
//   - 外键一律 ent edge，不手写 xxx_id 字段
//   - 幂等用 unique 索引在库层兜底（idempotency_key / meal_report(member,date)）
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/google/uuid"
)

// UUIDMixin 主键 UUID。
type UUIDMixin struct{ mixin.Schema }

func (UUIDMixin) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
	}
}

// TimeMixin 软删除 + 时间戳。需要撤销的实体（账单/任务/报饭）使用。
type TimeMixin struct{ mixin.Schema }

func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
		field.Time("deleted_at").Optional().Nillable(),
	}
}

// AppendMixin 只增不改。消息/LLM 用量/审计/撤销记录使用，无 updated_at 与 deleted_at。
type AppendMixin struct{ mixin.Schema }

func (AppendMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}
