// Package calendar 日程领域：事件 CRUD + 重复规则展开 + 单实例例外（C13）。
//
// 不变量（FR-CAL + 领域模型 §3）：
//   - 重复事件不预生成实例：查询时按窗口展开（周期计算是确定性逻辑，属代码不属于库）
//   - 单个实例的修改/跳过走 Override，不影响系列其余实例
//   - 可见性：private 只在创建人与家长之间；family 全家可见
//   - 事件可软删除（撤销 = 恢复），与任务/账单同一 TimeMixin 语义
package calendar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// Repeat 重复规则。
type Repeat string

const (
	RepeatOnce    Repeat = "once"
	RepeatDaily   Repeat = "daily"
	RepeatWeekly  Repeat = "weekly"
	RepeatMonthly Repeat = "monthly"
)

// Visibility 可见性。
type Visibility string

const (
	VisFamily  Visibility = "family"
	VisPrivate Visibility = "private"
)

// Event 事件视图（工具与 handler 消费）。
type Event struct {
	ID          string
	Title       string
	Description string
	Repeat      Repeat
	Interval    int
	StartAt     time.Time
	EndAt       *time.Time
	Visibility  Visibility
	IsLunar     bool
	Location    string
	OwnerID     string
	OwnerName   string
}

// Override 单实例例外。
type Override struct {
	ID          string
	EventID     string
	Occurrence  time.Time
	Action      string // modify / skip
	StartAt     *time.Time
	EndAt       *time.Time
	Title       string
	Description string
}

// Instance 展开后的实例（查询出口）。
type Instance struct {
	Event
	Occurrence  time.Time // 按规则展开的原发生时间（例外定位用）
	IsException bool      // 命中例外
	Skipped     bool      // 该实例被跳过（删单次）
}

// CreateEventCmd 建事件命令。
type CreateEventCmd struct {
	Title       string
	Description string
	StartAt     time.Time
	EndAt       *time.Time
	Repeat      Repeat
	Interval    int
	Visibility  Visibility
	IsLunar     bool
	Location    string
}

// EventRepo 仓储接口（store 层实现，依赖单向）。
// 方法名带 Calendar 前缀，避免与同 Store 上的 expense/task 方法冲突。
type EventRepo interface {
	// CalendarCreate 建事件；幂等冲突返回已存在 id + CodeConflict
	CalendarCreate(ctx context.Context, ownerID string, cmd CreateEventCmd, idempotencyKey string) (eventID string, err error)
	// CalendarDelete 软删除事件（撤销时恢复）
	CalendarDelete(ctx context.Context, eventID string) error
	// ListInRange 取窗口内的非删除事件（全家，可见性在服务层过滤）
	ListInRange(ctx context.Context, familyID string, start, end time.Time) ([]Event, error)
	// ListByOwner 取成员的事件（my 视图）
	ListByOwner(ctx context.Context, ownerID string, start, end time.Time) ([]Event, error)
	// GetEvent 单事件
	GetEvent(ctx context.Context, eventID string) (Event, error)
	// AddOverride 加单实例例外
	AddOverride(ctx context.Context, eventID string, o Override) error
	// ListOverrides 事件的所有例外
	ListOverrides(ctx context.Context, eventID string) ([]Override, error)
}

// Service 日程领域服务。
type Service interface {
	CreateEvent(ctx context.Context, ownerID string, cmd CreateEventCmd) (event Event, duplicated bool, err error)
	DeleteEvent(ctx context.Context, eventID string) error
	SkipInstance(ctx context.Context, eventID string, occurrence time.Time) error
	// ListInstances 按窗口展开实例（重复规则 + 例外合并 + 可见性过滤）
	ListInstances(ctx context.Context, memberID string, familyID string, start, end time.Time, scopeMy bool) ([]Instance, error)
	// GetEventDetail 事件详情（含可见性判定 + 可执行操作）。无权/不存在返回错误。
	GetEventDetail(ctx context.Context, eventID string, memberID string) (Detail, error)
}

func NewService(repo EventRepo) Service {
	return &service{repo: repo}
}

type service struct {
	repo EventRepo
}

// CreateEvent 建事件：校验 → 幂等键 → 入库。
func (s *service) CreateEvent(ctx context.Context, ownerID string, cmd CreateEventCmd) (Event, bool, error) {
	if cmd.Title == "" {
		return Event{}, false, apperr.New(apperr.CodeInvalidInput, "事件标题不能为空", nil)
	}
	if cmd.StartAt.IsZero() {
		return Event{}, false, apperr.New(apperr.CodeInvalidInput, "开始时间不能为空", nil)
	}
	if cmd.Repeat == "" {
		cmd.Repeat = RepeatOnce
	}
	if cmd.Interval <= 0 {
		cmd.Interval = 1
	}
	if cmd.Visibility == "" {
		cmd.Visibility = VisFamily
	}
	// 单次事件 interval 无意义
	if cmd.Repeat == RepeatOnce {
		cmd.Interval = 1
	}

	key := IdempotencyKey(ownerID, cmd)
	id, err := s.repo.CalendarCreate(ctx, ownerID, cmd, key)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) && ae.Code == apperr.CodeConflict {
			ev, _ := s.repo.GetEvent(ctx, id)
			return ev, true, nil // 幂等命中，对用户是成功
		}
		return Event{}, false, err
	}
	ev, err := s.repo.GetEvent(ctx, id)
	if err != nil {
		return Event{}, false, err
	}
	return ev, false, nil
}

func (s *service) DeleteEvent(ctx context.Context, eventID string) error {
	if eventID == "" {
		return apperr.New(apperr.CodeInvalidInput, "事件 id 不能为空", nil)
	}
	return s.repo.CalendarDelete(ctx, eventID)
}

// SkipInstance 跳过重复事件的单个实例（加 skip 例外，不影响系列）。
func (s *service) SkipInstance(ctx context.Context, eventID string, occurrence time.Time) error {
	if eventID == "" {
		return apperr.New(apperr.CodeInvalidInput, "事件 id 不能为空", nil)
	}
	if occurrence.IsZero() {
		return apperr.New(apperr.CodeInvalidInput, "实例时间不能为空", nil)
	}
	ev, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if ev.Repeat == RepeatOnce {
		return apperr.New(apperr.CodeInvalidInput, "单次事件没有实例例外，请直接删除事件", nil)
	}
	return s.repo.AddOverride(ctx, eventID, Override{
		ID:         uuid.NewString(),
		EventID:    eventID,
		Occurrence: occurrence,
		Action:     "skip",
	})
}

// ListInstances 按窗口展开实例。
//
// 对每个事件从 start_at 起按 repeat+interval 步进到窗口结束，每步一个实例；
// 再用 overrides 覆盖（skip 剔除，modify 改时间/标题）。步进有上限防超长窗口。
func (s *service) ListInstances(ctx context.Context, memberID string, familyID string, start, end time.Time, scopeMy bool) ([]Instance, error) {
	if end.IsZero() {
		end = start.AddDate(0, 1, 0)
	}
	var events []Event
	var err error
	if scopeMy {
		events, err = s.repo.ListByOwner(ctx, memberID, start, end)
	} else {
		events, err = s.repo.ListInRange(ctx, familyID, start, end)
	}
	if err != nil {
		return nil, err
	}

	out := make([]Instance, 0, len(events))
	for _, ev := range events {
		// 可见性过滤（private 只见创建人；family 全家可见）
		if ev.Visibility == VisPrivate && ev.OwnerID != memberID {
			continue
		}
		instances, err := s.expandEvent(ctx, ev, start, end)
		if err != nil {
			return nil, err
		}
		out = append(out, instances...)
	}
	return out, nil
}

// maxExpand 步进上限：daily 30 天约 31 步，weekly 约 5 步；
// 400 足够覆盖一年以上的窗口，同时防止恶意超长窗口把 CPU 打满。
const maxExpand = 400

// expandEvent 把单个事件按规则展开到 [start, end) 窗口内，合并例外。
func (s *service) expandEvent(ctx context.Context, ev Event, start, end time.Time) ([]Instance, error) {
	overrides, err := s.repo.ListOverrides(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	byOccurrence := make(map[time.Time]Override, len(overrides))
	for _, o := range overrides {
		byOccurrence[o.Occurrence] = o
	}

	// 单次事件：只在窗口包含 start_at 时出现
	if ev.Repeat == RepeatOnce {
		if ev.StartAt.Before(start) || !ev.StartAt.Before(end) {
			return nil, nil
		}
		return []Instance{toInstance(ev, ev.StartAt, byOccurrence)}, nil
	}

	interval := ev.Interval
	if interval <= 0 {
		interval = 1
	}

	out := make([]Instance, 0, 8)
	step := ev.StartAt
	for n := 0; n < maxExpand && step.Before(end); n++ {
		if !step.Before(start) { // 落在 [start, end) 内
			out = append(out, toInstance(ev, step, byOccurrence))
		}
		step = advance(step, ev.Repeat, interval)
	}
	return out, nil
}

// advance 按规则推进一个间隔。
func advance(t time.Time, r Repeat, interval int) time.Time {
	switch r {
	case RepeatDaily:
		return t.AddDate(0, 0, interval)
	case RepeatWeekly:
		return t.AddDate(0, 0, 7*interval)
	case RepeatMonthly:
		return t.AddDate(0, interval, 0)
	default:
		return t.AddDate(1, 0, 0) // 不会走到（once 已提前返回）
	}
}

// toInstance 把一次发生转成实例，合并例外（skip / modify）。
func toInstance(ev Event, occurrence time.Time, overrides map[time.Time]Override) Instance {
	inst := Instance{
		Event:      ev,
		Occurrence: occurrence,
	}
	inst.StartAt = occurrence // 默认=展开时间
	if o, ok := overrides[occurrence]; ok {
		inst.IsException = true
		if o.Action == "skip" {
			inst.Skipped = true
			return inst
		}
		if o.StartAt != nil {
			inst.StartAt = *o.StartAt
		}
		if o.EndAt != nil {
			end := *o.EndAt
			inst.EndAt = &end
		}
		if o.Title != "" {
			inst.Title = o.Title
		}
		if o.Description != "" {
			inst.Description = o.Description
		}
	}
	// 非例外：实例时长沿用事件 end_at 的偏移
	if inst.EndAt == nil && ev.EndAt != nil {
		duration := ev.EndAt.Sub(ev.StartAt)
		end := inst.StartAt.Add(duration)
		inst.EndAt = &end
	}
	return inst
}

// Detail 事件详情视图（A-02：详情页 + 深链目标）。
type Detail struct {
	Event
	CanEdit         bool     // 本期固定 false（无修改接口，前端据此隐藏入口）
	CanCancelSeries bool     // 创建人可删自己的事件
	Actions         []Action // 服务端按权限给出的可执行操作
}

// Action 详情页可执行操作。
type Action struct {
	Label string `json:"label"` // 按钮文案
	Kind  string `json:"kind"`  // cancel_series 等
}

// GetEventDetail 事件详情：取事件 → 可见性判定 → 组装可执行操作。
//
// 无权或不存在统一返回 CodeNotFound（不区分二者，防止探测他人隐私）。
// canEdit 本期固定 false：暂无修改接口，前端据此不渲染「修改」入口，
// 避免承诺不存在的能力（PRD A-02-04）。
func (s *service) GetEventDetail(ctx context.Context, eventID string, memberID string) (Detail, error) {
	ev, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return Detail{}, err
	}
	if !canSee(memberID, ev) {
		// 与不存在同码，避免泄露「该事件存在」
		return Detail{}, apperr.New(apperr.CodeNotFound, "事件不存在", nil)
	}

	canCancel := ev.OwnerID == memberID
	actions := make([]Action, 0, 1)
	if canCancel {
		actions = append(actions, Action{Label: "删除整条日程", Kind: "cancel_series"})
	}

	return Detail{
		Event:           ev,
		CanEdit:         false, // 本期无修改接口
		CanCancelSeries: canCancel,
		Actions:         actions,
	}, nil
}

// canSee 可见性判定（private 只见创建人；family 全家可见）。
// 家长权限在 handler/工具层另行校验（权限双保险第二层）。
func canSee(memberID string, ev Event) bool {
	if ev.Visibility == VisFamily {
		return true
	}
	return ev.OwnerID == memberID
}

// IdempotencyKey 建事件幂等键 sha256(owner+title+start+repeat)。
func IdempotencyKey(ownerID string, cmd CreateEventCmd) string {
	day := cmd.StartAt.UTC().Format(time.RFC3339)
	h := sha256.Sum256([]byte(ownerID + "|" + cmd.Title + "|" + day + "|" + string(cmd.Repeat)))
	return hex.EncodeToString(h[:])
}
