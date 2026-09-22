package repo

import (
	"context"
	"time"

	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
	"github.com/mk20mm/homeagent/internal/store/ent/notification"
)

// NotificationRecord 通知视图（与 v1.NotificationItem 同构，repo 适配）。
type NotificationRecord = v1.NotificationItem

// CreateNotification 创建通知（事件级幂等）。
//
// 幂等键 (type, ref_id, member_id, scheduled_at) 由唯一索引在库层兜底：
// 调度器每分钟重复扫描，同一事件只入库一条，不刷屏。
// 冲突（已存在）不报错，返回 (false, nil)。
// scheduledAt 由调用方（调度器）显式传入，区分「这次扫描」与「下次扫描」。
func (s *Store) CreateNotification(ctx context.Context, memberID string, r v1.NotificationItem, scheduledAt time.Time) (created bool, err error) {
	_, err = s.db.Notification.Create().
		SetType(notification.Type(r.Type)).
		SetTitle(r.Title).
		SetBody(r.Body).
		SetNillableRefType(nilIfEmpty(r.RefType)).
		SetNillableRefID(nilIfEmpty(r.RefID)).
		SetNillableActionLabel(nilIfEmpty(r.ActionLabel)).
		SetNillableActionPath(nilIfEmpty(r.ActionPath)).
		SetScheduledAt(scheduledAt).
		SetMemberID(toUUID(memberID)).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return false, nil // 幂等：同一事件已存在
		}
		return false, apperr.New(apperr.CodeInternal, "创建通知失败", err)
	}
	return true, nil
}

// CountUnread 未读角标数（ADR-006：前端封顶 99+）。
func (s *Store) CountUnread(ctx context.Context, memberID string) (int, error) {
	n, err := s.db.Notification.Query().
		Where(
			notification.HasMemberWith(member.IDEQ(toUUID(memberID))),
			notification.ReadAtIsNil(),
		).
		Count(ctx)
	if err != nil {
		return 0, apperr.New(apperr.CodeInternal, "查询未读数失败", err)
	}
	return n, nil
}

// ListNotifications 通知列表（新的在前，分页）。
// unreadFirst=true 时未读排在已读前面（角标与列表一致）。
func (s *Store) ListNotifications(ctx context.Context, memberID string, limit int, unreadFirst bool) ([]v1.NotificationItem, error) {
	q := s.db.Notification.Query().
		Where(notification.HasMemberWith(member.IDEQ(toUUID(memberID))))
	if unreadFirst {
		q = q.Order(ent.Asc(notification.FieldReadAt), ent.Desc(notification.FieldCreatedAt))
	} else {
		q = q.Order(ent.Desc(notification.FieldCreatedAt))
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	list, err := q.All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询通知列表失败", err)
	}
	out := make([]v1.NotificationItem, 0, len(list))
	for _, n := range list {
		out = append(out, v1.NotificationItem{
			ID:          n.ID.String(),
			Type:        string(n.Type),
			Title:       n.Title,
			Body:        n.Body,
			RefType:     n.RefType,
			RefID:       n.RefID,
			ActionLabel: n.ActionLabel,
			ActionPath:  n.ActionPath,
			ReadAt:      n.ReadAt,
			CreatedAt:   n.CreatedAt,
		})
	}
	return out, nil
}

// MarkNotificationRead 标记单条已读（成员隔离：不属于该成员的不动）。
func (s *Store) MarkNotificationRead(ctx context.Context, id, memberID string) error {
	n, err := s.db.Notification.Update().
		Where(
			notification.IDEQ(toUUID(id)),
			notification.HasMemberWith(member.IDEQ(toUUID(memberID))),
		).
		SetReadAt(time.Now()).
		Save(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "标记已读失败", err)
	}
	if n == 0 {
		return apperr.New(apperr.CodeNotFound, "通知不存在或不属于该成员", nil)
	}
	return nil
}

// MarkAllNotificationsRead 全部已读（成员隔离）。
func (s *Store) MarkAllNotificationsRead(ctx context.Context, memberID string) (int, error) {
	n, err := s.db.Notification.Update().
		Where(
			notification.HasMemberWith(member.IDEQ(toUUID(memberID))),
			notification.ReadAtIsNil(),
		).
		SetReadAt(time.Now()).
		Save(ctx)
	if err != nil {
		return 0, apperr.New(apperr.CodeInternal, "全部已读失败", err)
	}
	return n, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
