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
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/google/uuid"
)

// sqliteTextType 时间字段在 SQLite 下的列类型声明。
//
// ent 默认把 field.Time 映射成 datetime，而 SQLite 没有真正的日期类型——
// 「datetime」按类型推导规则落到 NUMERIC 亲和性，比较时会把「2026-09-21T09:39:42Z」
// 这样的字符串截成数字 2026，于是同年的所有时间互相「相等」，排序与范围查询全错
// （账本倒序错位、游标分页跳页）。声明成 TEXT 后比较按字符串语义，配合入库统一
// UTC（store.UseUTCTimes）即正确。
func sqliteTextType() map[string]string {
	return map[string]string{dialect.SQLite: "TEXT"}
}

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
