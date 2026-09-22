package repo

import (
	"context"
	"strings"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/conversation"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
	"github.com/mk20mm/homeagent/internal/store/ent/message"
)

var _ session.Repository = (*Store)(nil)

// CreateConversation 新建会话；modelID 空则不绑定模型（用默认）。
func (s *Store) CreateConversation(ctx context.Context, memberID, title, modelID string) (string, error) {
	b := s.db.Conversation.Create().
		SetTitle(title).
		SetMemberID(toUUID(memberID))
	if modelID != "" {
		b.SetModelID(toUUID(modelID))
	}
	c, err := b.Save(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeInternal, "创建会话失败", err)
	}
	return c.ID.String(), nil
}

// ListConversations 成员的会话列表（软删除过滤 + member 隔离，游标分页）。
// 排序：最后消息时间倒序（SQLite 下 NULL 天然在末尾，新建空会话排最后）。
func (s *Store) ListConversations(ctx context.Context, memberID string, limit int, cursor string) ([]session.Conversation, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := s.db.Conversation.Query().
		Where(
			conversation.HasMemberWith(member.IDEQ(toUUID(memberID))),
			conversation.DeletedAtIsNil(),
		)
	if cursor != "" {
		if t, err := time.Parse(time.RFC3339, cursor); err == nil {
			query = query.Where(conversation.LastMessageAtLT(t.UTC()))
		}
	}
	list, err := query.
		Order(ent.Desc(conversation.FieldLastMessageAt)).
		Limit(limit + 1).
		All(ctx)
	if err != nil {
		return nil, "", apperr.New(apperr.CodeInternal, "查询会话列表失败", err)
	}

	nextCursor := ""
	if len(list) > limit {
		if list[limit-1].LastMessageAt != nil {
			nextCursor = list[limit-1].LastMessageAt.Format(time.RFC3339)
		}
		list = list[:limit]
	}

	out := make([]session.Conversation, 0, len(list))
	for _, c := range list {
		conv := session.Conversation{
			ID:        c.ID.String(),
			MemberID:  memberID,
			Title:     c.Title,
			CreatedAt: c.CreatedAt,
		}
		if c.LastMessageAt != nil {
			conv.LastMessageAt = *c.LastMessageAt
		}
		out = append(out, conv)
	}
	return out, nextCursor, nil
}

// LoadMessageList 消息列表视图（API 层展示用，带 id/时间，按时间正序）。
func (s *Store) LoadMessageList(ctx context.Context, convID, memberID string, limit int) ([]session.MessageView, error) {
	if _, err := s.LoadConversation(ctx, convID, memberID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	list, err := s.db.Message.Query().
		Where(message.HasConversationWith(conversation.IDEQ(toUUID(convID)))).
		Order(ent.Asc(message.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询消息失败", err)
	}

	out := make([]session.MessageView, 0, len(list))
	for _, m := range list {
		content := m.Content
		if m.Role == message.RoleAssistant {
			// 存储层把 tool_calls 编码进 content，展示层只取文本
			content = session.DecodeAssistantContent(m.Content).Content
		}
		out = append(out, session.MessageView{
			ID:             m.ID.String(),
			ConversationID: convID,
			Role:           string(m.Role),
			Content:        content,
			ToolName:       m.ToolName,
			CreatedAt:      m.CreatedAt,
		})
	}
	return out, nil
}

// DeleteConversation 软删除会话（消息随会话一起对用户不可见，保留审计留痕）。
func (s *Store) DeleteConversation(ctx context.Context, convID, memberID string) error {
	if _, err := s.LoadConversation(ctx, convID, memberID); err != nil {
		return err
	}
	n, err := s.db.Conversation.Update().
		Where(conversation.IDEQ(toUUID(convID))).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "删除会话失败", err)
	}
	if n == 0 {
		return apperr.New(apperr.CodeNotFound, "会话不存在", nil)
	}
	return nil
}

// LoadConversation 取会话（含 member 隔离校验 + 软删除过滤）。
func (s *Store) LoadConversation(ctx context.Context, convID, memberID string) (session.Conversation, error) {
	c, err := s.db.Conversation.Query().
		Where(
			conversation.IDEQ(toUUID(convID)),
			conversation.DeletedAtIsNil(),
			conversation.HasMemberWith(member.IDEQ(toUUID(memberID))),
		).
		WithMember().
		WithModel().
		Only(ctx)
	if err != nil {
		return session.Conversation{}, apperr.New(apperr.CodeNotFound, "会话不存在或不属于该成员", err)
	}

	out := session.Conversation{
		ID:        c.ID.String(),
		MemberID:  memberID,
		Title:     c.Title,
		CreatedAt: c.CreatedAt,
	}
	if c.LastMessageAt != nil {
		out.LastMessageAt = *c.LastMessageAt
	}
	if c.Edges.Model != nil {
		out.ModelID = c.Edges.Model.ID.String()
	}
	return out, nil
}

// LoadMessages 按时间正序取历史消息（只增不改）。
func (s *Store) LoadMessages(ctx context.Context, convID, memberID string) ([]session.Message, error) {
	if _, err := s.LoadConversation(ctx, convID, memberID); err != nil {
		return nil, err
	}
	list, err := s.db.Message.Query().
		Where(message.HasConversationWith(conversation.IDEQ(toUUID(convID)))).
		Order(ent.Asc(message.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询消息失败", err)
	}

	out := make([]session.Message, 0, len(list))
	for _, m := range list {
		if m.Role == message.RoleAssistant {
			out = append(out, session.DecodeAssistantContent(m.Content))
			continue
		}
		out = append(out, session.Message{
			Role:       gateway.Role(m.Role),
			Content:    m.Content,
			Name:       m.ToolName,
			ToolCallID: m.TraceID,
		})
	}
	return out, nil
}

// AppendMessages 追加消息（同一事务，避免半截历史）。
func (s *Store) AppendMessages(ctx context.Context, convID, memberID string, msgs []session.Message) error {
	conv, err := s.LoadConversation(ctx, convID, memberID)
	if err != nil {
		return err
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "开启事务失败", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, m := range msgs {
		b := tx.Message.Create().
			SetConversationID(toUUID(convID))
		switch m.Role {
		case gateway.RoleAssistant:
			b.SetRole(message.RoleAssistant).
				SetContent(session.EncodeAssistantContent(m))
		case gateway.RoleTool:
			b.SetRole(message.RoleTool).
				SetContent(m.Content).
				SetToolName(m.Name).
				SetTraceID(m.ToolCallID)
		case gateway.RoleUser:
			b.SetRole(message.RoleUser).SetContent(m.Content)
		case gateway.RoleSystem:
			b.SetRole(message.RoleSystem).SetContent(m.Content)
		}
		if err := b.Exec(ctx); err != nil {
			return apperr.New(apperr.CodeInternal, "保存消息失败", err)
		}
	}

	// 标题为空时用首条用户消息截断作标题（豆包式）；同时刷新最后消息时间
	upd := tx.Conversation.UpdateOneID(toUUID(convID)).SetLastMessageAt(time.Now())
	if conv.Title == "" {
		if title := firstUserTitle(msgs); title != "" {
			upd.SetTitle(title)
		}
	}
	if err := upd.Exec(ctx); err != nil {
		return apperr.New(apperr.CodeInternal, "更新会话失败", err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.New(apperr.CodeInternal, "提交消息事务失败", err)
	}
	committed = true
	return nil
}

// firstUserTitle 取本轮首条非空用户消息，按 rune 截断 20 字，超出加省略号。
func firstUserTitle(msgs []session.Message) string {
	for _, m := range msgs {
		if m.Role == gateway.RoleUser {
			t := strings.TrimSpace(m.Content)
			if t == "" {
				continue
			}
			if len([]rune(t)) > 20 {
				return string([]rune(t)[:20]) + "…"
			}
			return t
		}
	}
	return ""
}

// Permissions 成员权限集合（权限双保险源头，ADR-005）。
func (s *Store) Permissions(ctx context.Context, memberID string) (map[string]bool, error) {
	m, err := s.db.Member.Query().Where(member.IDEQ(toUUID(memberID))).Only(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}
	if !m.Active {
		return nil, apperr.New(apperr.CodePermission, "成员已停用", nil)
	}
	perms := make(map[string]bool, len(m.Permissions))
	for k, v := range m.Permissions {
		perms[k] = v
	}
	return perms, nil
}

// Exists 成员存在且启用（开发期认证校验用）。
func (s *Store) Exists(memberID string) bool {
	if memberID == "" {
		return false
	}
	n, err := s.db.Member.Query().
		Where(member.IDEQ(toUUID(memberID)), member.ActiveEQ(true)).
		Count(context.Background())
	return err == nil && n > 0
}

func nilIf(s, empty string) *string {
	if s == empty {
		return nil
	}
	return &s
}
