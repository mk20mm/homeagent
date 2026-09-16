package repo

import (
	"context"
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

// CreateConversation 新建会话。
func (s *Store) CreateConversation(ctx context.Context, memberID, title string) (string, error) {
	c, err := s.db.Conversation.Create().
		SetTitle(title).
		SetMemberID(toUUID(memberID)).
		Save(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeInternal, "创建会话失败", err)
	}
	return c.ID.String(), nil
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
		ID:       c.ID.String(),
		MemberID: memberID,
		Title:    c.Title,
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
	if _, err := s.LoadConversation(ctx, convID, memberID); err != nil {
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
	if err := tx.Conversation.UpdateOneID(toUUID(convID)).
		SetLastMessageAt(time.Now()).
		Exec(ctx); err != nil {
		return apperr.New(apperr.CodeInternal, "更新会话时间失败", err)
	}
	if err := tx.Commit(); err != nil {
		return apperr.New(apperr.CodeInternal, "提交消息事务失败", err)
	}
	committed = true
	return nil
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
