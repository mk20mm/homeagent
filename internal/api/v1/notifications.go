package v1

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// NotificationStore 通知存储接口（repo.Store 实现，返回 v1.NotificationItem，
// 与 undolog.go 同模式：repo 依赖 v1 的视图类型，分层方向保持 store ← api）。
type NotificationStore interface {
	ListNotifications(ctx context.Context, memberID string, limit int, unreadFirst bool) ([]NotificationItem, error)
	CountUnread(ctx context.Context, memberID string) (int, error)
	MarkNotificationRead(ctx context.Context, id, memberID string) error
	MarkAllNotificationsRead(ctx context.Context, memberID string) (int, error)
}

// NotificationItem 通知视图。
type NotificationItem struct {
	ID          string
	Type        string
	Title       string
	Body        string
	RefType     string
	RefID       string
	ActionLabel string
	ActionPath  string
	ReadAt      *time.Time
	CreatedAt   time.Time
}

// ListNotifications GET /notifications —— 通知中心列表（未读优先，新的在前）。
func ListNotifications(store NotificationStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		limit := 50
		if l := c.Query("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}

		ctx := c.Request.Context()
		items, err := store.ListNotifications(ctx, memberID, limit, true)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		unread, err := store.CountUnread(ctx, memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}

		out := make([]gin.H, 0, len(items))
		for _, n := range items {
			out = append(out, gin.H{
				"id":           n.ID,
				"type":         n.Type,
				"title":        n.Title,
				"body":         n.Body,
				"ref_type":     n.RefType,
				"ref_id":       n.RefID,
				"action_label": n.ActionLabel,
				"action_path":  n.ActionPath,
				"read":         n.ReadAt != nil,
				"created_at":   n.CreatedAt.Format(time.RFC3339),
			})
		}
		c.JSON(200, gin.H{"items": out, "unread_count": unread})
	}
}

// MarkNotificationsRead POST /notifications —— 标记已读（全部或单条）。
func MarkNotificationsRead(store NotificationStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		var req struct {
			NotificationID string `json:"notification_id"`
		}
		_ = c.ShouldBindJSON(&req) // body 可空（=全部已读）

		ctx := c.Request.Context()
		if req.NotificationID != "" {
			if err := store.MarkNotificationRead(ctx, req.NotificationID, memberID); err != nil {
				abortWith(c, asAppErr(err))
				return
			}
			c.JSON(200, gin.H{"marked": 1})
			return
		}
		n, err := store.MarkAllNotificationsRead(ctx, memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(200, gin.H{"marked": n})
	}
}
