package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UndoLog 撤销记录，24h 有效（ADR-004）。只增不改，cron 清理过期。
type UndoLog struct{ ent.Schema }

type UndoStatus string

const (
	UndoActive  UndoStatus = "active"
	UndoUsed    UndoStatus = "used"
	UndoExpired UndoStatus = "expired"
)

func (UndoLog) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, AppendMixin{}} }

func (UndoLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("trace_id"),
		field.String("tool_name"),
		field.JSON("undo_data", map[string]any{}).Comment("逆向数据，如账单ID"),
		field.Enum("status").Values(
			string(UndoActive), string(UndoUsed), string(UndoExpired),
		).Default(string(UndoActive)),
		field.Time("expires_at").Default(func() time.Time {
			return time.Now().Add(24 * time.Hour)
		}),
	}
}

func (UndoLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", Member.Type).Ref("undo_logs").Unique().Required(),
	}
}

func (UndoLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "expires_at").Edges("member"),
	}
}

// AuditLog 审计日志，只增不改，撤销也留痕（ARCHITECTURE §5/§10）。
type AuditLog struct{ ent.Schema }

func (AuditLog) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, AppendMixin{}} }

func (AuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("trace_id"),
		field.String("tool_name"),
		field.Enum("risk").Values("low", "medium", "high"),
		field.JSON("params", map[string]any{}).Optional(),
		field.String("result").Optional(),
		field.Bool("undone").Default(false),
		field.Bool("permission_denied").Default(false).Comment("越权尝试记录"),
		field.Int("latency_ms").Default(0),
	}
}

func (AuditLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", Member.Type).Ref("audit_logs").Unique().Required(),
	}
}

func (AuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("trace_id"),
		index.Fields("created_at").Edges("member"),
		index.Fields("permission_denied", "created_at"),
	}
}
