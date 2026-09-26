package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Member 家庭成员。会话与数据访问必带 member_id（领域模型不变量）。
type Member struct{ ent.Schema }

type Role string

const (
	RoleParent   Role = "parent"
	RoleAdult    Role = "adult"
	RoleElder    Role = "elder"
	RoleChild    Role = "child"
	RoleRoommate Role = "roommate"
)

// PermissionTemplate 权限模板（ADR-005 权限矩阵的预设）。
type PermissionTemplate string

const (
	PermAdmin   PermissionTemplate = "admin"
	PermFull    PermissionTemplate = "full"
	PermLimited PermissionTemplate = "limited"
	PermChild   PermissionTemplate = "child"
)

func (Member) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (Member) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.Enum("role").Values(
			string(RoleParent), string(RoleAdult), string(RoleElder),
			string(RoleChild), string(RoleRoommate),
		).Default(string(RoleAdult)),
		field.Enum("permission_template").Values(
			string(PermAdmin), string(PermFull), string(PermLimited), string(PermChild),
		).Default(string(PermFull)).Comment("权限矩阵模板"),
		field.JSON("permissions", map[string]bool{}).Comment("权限矩阵：模块开关，如 expense.write"),
		field.String("auth_token").Optional().Sensitive().Comment("加密存储"),
		field.Bool("active").Default(true).Comment("停用=隐藏不删"),
	}
}

func (Member) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("family", Family.Type).Ref("members").Unique().Required(),
		edge.To("conversations", Conversation.Type),
		edge.To("expenses", Expense.Type),
		edge.To("tasks", Task.Type),
		edge.To("assigned_tasks", Task.Type),
		edge.To("meal_reports", MealReport.Type),
		edge.To("notifications", Notification.Type),
		edge.To("undo_logs", UndoLog.Type),
		edge.To("audit_logs", AuditLog.Type),
		edge.To("usages", LLMUsage.Type),
		edge.To("events", CalendarEvent.Type),
	}
}

func (Member) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Edges("family").Unique(),
	}
}
