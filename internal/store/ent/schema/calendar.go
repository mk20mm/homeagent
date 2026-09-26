package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CalendarEvent 日程事件（C13）。家庭共享日历的存储单元。
//
// 设计要点（FR-CAL-01/02/03 + 领域模型 §3）：
//   - 重复规则用 rrule 风格的枚举（daily/weekly/monthly）+ interval，存在事件行上；
//     查询时按窗口展开实例，不在库里预生成（周期计算是确定性逻辑，属代码）
//   - 单个实例的修改/删除走 EventOverride（例外表），不影响系列其余实例
//   - 可见性：family（全家可见）时老人/孩子只读可见；private 只在成员本人与家长间
//   - 农历标记 is_lunar：生日/纪念日按农历算，调度器转换成公历再提醒
//   - 颜色由成员决定（前端按 owner_id 取成员色），不在事件上存颜色
type CalendarEvent struct{ ent.Schema }

type EventRepeat string

const (
	RepeatOnce    EventRepeat = "once"    // 单次
	RepeatDaily   EventRepeat = "daily"   // 每天
	RepeatWeekly  EventRepeat = "weekly"  // 每周
	RepeatMonthly EventRepeat = "monthly" // 每月
)

type EventVisibility string

const (
	VisFamily  EventVisibility = "family"  // 全家可见（默认）
	VisPrivate EventVisibility = "private" // 仅本人与家长
)

func (CalendarEvent) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (CalendarEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("title"),
		field.String("description").Optional(),
		field.Enum("repeat").Values(
			string(RepeatOnce), string(RepeatDaily), string(RepeatWeekly), string(RepeatMonthly),
		).Default(string(RepeatOnce)).Comment("重复规则；once=单次"),
		field.Int("interval").Default(1).Comment("重复间隔，如每 2 周 interval=2"),
		field.Time("start_at").Comment("开始时间（本地，存 UTC）"),
		field.Time("end_at").Optional().Nillable().Comment("结束时间，空=全天/无结束"),
		field.Enum("visibility").Values(
			string(VisFamily), string(VisPrivate),
		).Default(string(VisFamily)).Comment("可见性：family 全家可见 / private 仅本人与家长"),
		field.Bool("is_lunar").Default(false).Comment("农历事件（生日/纪念日），提醒按农历转换"),
		field.String("location").Optional().Comment("地点"),
		field.String("idempotency_key").Optional().Unique().Comment("建事件幂等键"),
	}
}

func (CalendarEvent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", Member.Type).Ref("events").Unique().Required().Comment("创建人"),
		edge.To("overrides", EventOverride.Type).Comment("单实例例外"),
	}
}

func (CalendarEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("start_at"),
		index.Fields("repeat"),
		index.Fields("idempotency_key"),
	}
}

// EventOverride 重复事件的实例例外（FR-CAL-03：单个例外可单独修改）。
//
// 一个 (event_id, occurrence) 对应一条例外：改时间/标题或删除该次实例。
// occurrence 是该实例在系列中的逻辑发生时间（按规则展开的原值），用于定位。
type EventOverride struct{ ent.Schema }

type OverrideAction string

const (
	OverrideModify OverrideAction = "modify" // 改这次（时间/标题/描述）
	OverrideSkip   OverrideAction = "skip"   // 跳过这次（相当于删单次）
)

func (EventOverride) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (EventOverride) Fields() []ent.Field {
	return []ent.Field{
		field.Time("occurrence").Comment("该实例按规则展开的发生时间（定位用）"),
		field.Enum("action").Values(
			string(OverrideModify), string(OverrideSkip),
		).Default(string(OverrideModify)),
		field.Time("start_at").Optional().Nillable().Comment("改后的开始时间（modify）"),
		field.Time("end_at").Optional().Nillable().Comment("改后的结束时间"),
		field.String("title").Optional().Comment("改后的标题"),
		field.String("description").Optional().Comment("改后的描述"),
	}
}

func (EventOverride) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("event", CalendarEvent.Type).Ref("overrides").Unique().Required(),
	}
}

func (EventOverride) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("occurrence").Edges("event").Unique(),
	}
}
