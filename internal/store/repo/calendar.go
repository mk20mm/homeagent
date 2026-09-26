package repo

import (
	"context"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
	domcal "github.com/mk20mm/homeagent/internal/domain/calendar"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/calendarevent"
	"github.com/mk20mm/homeagent/internal/store/ent/eventoverride"
	"github.com/mk20mm/homeagent/internal/store/ent/family"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// 编译期接口实现检查。
var _ domcal.EventRepo = (*Store)(nil)

// CalendarCreate 建事件。幂等冲突（unique idempotency_key）返回已存在 id + CodeConflict。
func (s *Store) CalendarCreate(ctx context.Context, ownerID string, cmd domcal.CreateEventCmd, idempotencyKey string) (string, error) {
	owner, err := s.db.Member.Query().
		Where(member.IDEQ(toUUID(ownerID))).
		Only(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}

	repeat := string(cmd.Repeat)
	if repeat == "" {
		repeat = string(domcal.RepeatOnce) // enum 必填，空值非法
	}
	vis := string(cmd.Visibility)
	if vis == "" {
		vis = string(domcal.VisFamily) // enum 必填，空值非法
	}

	b := s.db.CalendarEvent.Create().
		SetTitle(cmd.Title).
		SetRepeat(calendarevent.Repeat(repeat)).
		SetInterval(cmd.Interval).
		SetStartAt(cmd.StartAt.UTC()).
		SetVisibility(calendarevent.Visibility(vis)).
		SetIsLunar(cmd.IsLunar).
		SetIdempotencyKey(idempotencyKey).
		SetOwner(owner)

	if cmd.Description != "" {
		b.SetDescription(cmd.Description)
	}
	if cmd.EndAt != nil {
		b.SetEndAt(cmd.EndAt.UTC())
	}
	if cmd.Location != "" {
		b.SetLocation(cmd.Location)
	}

	saved, err := b.Save(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			exist, qerr := s.db.CalendarEvent.Query().
				Where(calendarevent.IdempotencyKeyEQ(idempotencyKey), calendarevent.DeletedAtIsNil()).
				Only(ctx)
			if qerr != nil || exist == nil {
				return "", apperr.New(apperr.CodeConflict, "事件已存在但回查失败", err)
			}
			return exist.ID.String(), apperr.New(apperr.CodeConflict, "事件已存在", nil)
		}
		return "", apperr.New(apperr.CodeInternal, "建事件失败", err)
	}
	return saved.ID.String(), nil
}

// CalendarDelete 软删除事件（撤销时恢复 deleted_at）。
func (s *Store) CalendarDelete(ctx context.Context, eventID string) error {
	n, err := s.db.CalendarEvent.UpdateOneID(toUUID(eventID)).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "删除事件失败", err)
	}
	if n == nil {
		return apperr.New(apperr.CodeNotFound, "事件不存在", nil)
	}
	return nil
}

// ListInRange 取窗口内非删除事件（全家），不含已软删除的。
// 服务层再做可见性过滤（private 只见创建人）。
// start 不加过滤：跨窗口开始的重复事件也要展开进窗口（由服务层步进处理）。
func (s *Store) ListInRange(ctx context.Context, familyID string, start, end time.Time) ([]domcal.Event, error) {
	list, err := s.db.CalendarEvent.Query().
		Where(
			calendarevent.HasOwnerWith(member.HasFamilyWith(family.IDEQ(toUUID(familyID)))),
			calendarevent.DeletedAtIsNil(),
			calendarevent.StartAtLT(end.UTC()),
		).
		WithOwner().
		Order(ent.Asc(calendarevent.FieldStartAt)).
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询日程失败", err)
	}
	out := make([]domcal.Event, 0, len(list))
	for _, e := range list {
		out = append(out, mapEvent(e))
	}
	return out, nil
}

// ListByOwner 取成员的事件（my 视图）。
func (s *Store) ListByOwner(ctx context.Context, ownerID string, start, end time.Time) ([]domcal.Event, error) {
	list, err := s.db.CalendarEvent.Query().
		Where(
			calendarevent.HasOwnerWith(member.IDEQ(toUUID(ownerID))),
			calendarevent.DeletedAtIsNil(),
			calendarevent.StartAtLT(end.UTC()),
		).
		WithOwner().
		Order(ent.Asc(calendarevent.FieldStartAt)).
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询日程失败", err)
	}
	out := make([]domcal.Event, 0, len(list))
	for _, e := range list {
		out = append(out, mapEvent(e))
	}
	return out, nil
}

// GetEvent 单事件（软删除过滤）。
func (s *Store) GetEvent(ctx context.Context, eventID string) (domcal.Event, error) {
	e, err := s.db.CalendarEvent.Query().
		Where(calendarevent.IDEQ(toUUID(eventID)), calendarevent.DeletedAtIsNil()).
		WithOwner().
		Only(ctx)
	if err != nil {
		return domcal.Event{}, apperr.New(apperr.CodeNotFound, "事件不存在", err)
	}
	return mapEvent(e), nil
}

// AddOverride 加单实例例外（唯一键 event+occurrence 兜底重复跳过）。
func (s *Store) AddOverride(ctx context.Context, eventID string, o domcal.Override) error {
	b := s.db.EventOverride.Create().
		SetEventID(toUUID(eventID)).
		SetOccurrence(o.Occurrence.UTC()).
		SetAction(eventoverride.Action(o.Action))
	if o.StartAt != nil {
		b.SetStartAt(o.StartAt.UTC())
	}
	if o.EndAt != nil {
		b.SetEndAt(o.EndAt.UTC())
	}
	if o.Title != "" {
		b.SetTitle(o.Title)
	}
	if o.Description != "" {
		b.SetDescription(o.Description)
	}
	if err := b.Exec(ctx); err != nil {
		if isUniqueViolation(err) {
			// 已存在该实例例外：覆盖 action（如 skip 重复提交）
			_, uerr := s.db.EventOverride.Update().
				Where(
					eventoverride.HasEventWith(calendarevent.IDEQ(toUUID(eventID))),
					eventoverride.OccurrenceEQ(o.Occurrence.UTC()),
				).
				SetAction(eventoverride.Action(o.Action)).
				Save(ctx)
			if uerr != nil {
				return apperr.New(apperr.CodeInternal, "更新实例例外失败", uerr)
			}
			return nil
		}
		return apperr.New(apperr.CodeInternal, "加实例例外失败", err)
	}
	return nil
}

// ListOverrides 事件的所有例外。
func (s *Store) ListOverrides(ctx context.Context, eventID string) ([]domcal.Override, error) {
	list, err := s.db.EventOverride.Query().
		Where(eventoverride.HasEventWith(calendarevent.IDEQ(toUUID(eventID)))).
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询实例例外失败", err)
	}
	out := make([]domcal.Override, 0, len(list))
	for _, o := range list {
		out = append(out, mapOverride(o))
	}
	return out, nil
}

func mapEvent(e *ent.CalendarEvent) domcal.Event {
	ev := domcal.Event{
		ID:          e.ID.String(),
		Title:       e.Title,
		Description: e.Description,
		Repeat:      domcal.Repeat(string(e.Repeat)),
		Interval:    e.Interval,
		Visibility:  domcal.Visibility(string(e.Visibility)),
		IsLunar:     e.IsLunar,
		Location:    e.Location,
	}
	if !e.StartAt.IsZero() {
		ev.StartAt = e.StartAt
	}
	if e.EndAt != nil {
		end := *e.EndAt
		ev.EndAt = &end
	}
	if e.Edges.Owner != nil {
		ev.OwnerID = e.Edges.Owner.ID.String()
		ev.OwnerName = e.Edges.Owner.Name
	}
	return ev
}

func mapOverride(o *ent.EventOverride) domcal.Override {
	out := domcal.Override{
		ID:         o.ID.String(),
		Occurrence: o.Occurrence,
		Action:     string(o.Action),
		Title:      o.Title,
	}
	if o.StartAt != nil {
		t := *o.StartAt
		out.StartAt = &t
	}
	if o.EndAt != nil {
		t := *o.EndAt
		out.EndAt = &t
	}
	return out
}
