// Package session 会话管理：会话隔离、历史消息持久化（ARCHITECTURE §2.2 组件①）。
//
// 不变量：
//   - 会话按 member_id 严格隔离，任何查询必带 memberID（AI-PRD §3）
//   - 消息只增不改；assistant 消息含工具调用时 content 存 JSON（P1 接真实供应商时加字段）
package session

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// Message 直接复用网关消息模型（供应商无关）。
type Message = gateway.Message

// Conversation 会话视图。
type Conversation struct {
	ID             string
	MemberID       string
	Title          string
	ModelID        string
	LastMessageAt  time.Time
}

// Repository 会话仓储：领域层定义接口，store 层实现。
type Repository interface {
	// CreateConversation 新建会话，返回 id
	CreateConversation(ctx context.Context, memberID, title string) (convID string, err error)
	// LoadConversation 取会话（含 member 隔离校验）
	LoadConversation(ctx context.Context, convID, memberID string) (Conversation, error)
	// LoadMessages 按时间正序取历史消息
	LoadMessages(ctx context.Context, convID, memberID string) ([]Message, error)
	// AppendMessages 追加消息（同一事务，避免半截历史）
	AppendMessages(ctx context.Context, convID, memberID string, msgs []Message) error
	// Permissions 取成员权限集合（权限双保险源头，ADR-005）
	Permissions(ctx context.Context, memberID string) (map[string]bool, error)
}

// Service 会话服务：负责装配请求上下文（历史 + 权限 + 模型）。
type Service interface {
	// Ensure 会话存在（convID 空则新建），返回会话与历史
	Ensure(ctx context.Context, convID, memberID string) (conv Conversation, history []Message, err error)
	// Append 追加一轮消息
	Append(ctx context.Context, convID, memberID string, msgs []Message) error
	// Permissions 成员权限
	Permissions(ctx context.Context, memberID string) (map[string]bool, error)
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

type service struct {
	repo Repository
}

func (s *service) Ensure(ctx context.Context, convID, memberID string) (Conversation, []Message, error) {
	if memberID == "" {
		return Conversation{}, nil, apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}
	var conv Conversation
	if convID == "" {
		var err error
		convID, err = s.repo.CreateConversation(ctx, memberID, "")
		if err != nil {
			return Conversation{}, nil, err
		}
		conv = Conversation{ID: convID, MemberID: memberID}
	} else {
		var err error
		conv, err = s.repo.LoadConversation(ctx, convID, memberID)
		if err != nil {
			return Conversation{}, nil, err
		}
	}
	history, err := s.repo.LoadMessages(ctx, convID, memberID)
	if err != nil {
		return Conversation{}, nil, err
	}
	return conv, history, nil
}

func (s *service) Append(ctx context.Context, convID, memberID string, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	return s.repo.AppendMessages(ctx, convID, memberID, msgs)
}

func (s *service) Permissions(ctx context.Context, memberID string) (map[string]bool, error) {
	return s.repo.Permissions(ctx, memberID)
}

// EncodeAssistantContent 把 assistant 消息（可能含 tool_calls）序列化为可存储的 content。
// 纯文本直接返回；含工具调用时存 JSON，读取端用 DecodeAssistantContent 还原。
func EncodeAssistantContent(m Message) string {
	if len(m.ToolCalls) == 0 {
		return m.Content
	}
	b, err := json.Marshal(map[string]any{
		"text":       m.Content,
		"tool_calls": m.ToolCalls,
	})
	if err != nil {
		return m.Content
	}
	return string(b)
}

// DecodeAssistantContent 从存储的 content 还原 assistant 消息。
func DecodeAssistantContent(raw string) Message {
	if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return Message{Role: gateway.RoleAssistant, Content: raw}
	}
	var parsed struct {
		Text      string             `json:"text"`
		ToolCalls []gateway.ToolCall `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return Message{Role: gateway.RoleAssistant, Content: raw}
	}
	return Message{
		Role:      gateway.RoleAssistant,
		Content:   parsed.Text,
		ToolCalls: parsed.ToolCalls,
	}
}
