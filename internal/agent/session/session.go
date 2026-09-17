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
	CreatedAt      time.Time
}

// MessageView 消息列表视图（API 层用，比 Message 多 id/时间；runtime 仍用 Message 发 LLM）。
type MessageView struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	ToolName       string
	CreatedAt      time.Time
}

// Repository 会话仓储：领域层定义接口，store 层实现。
type Repository interface {
	// CreateConversation 新建会话，返回 id；modelID 空则不绑定模型
	CreateConversation(ctx context.Context, memberID, title, modelID string) (convID string, err error)
	// LoadConversation 取会话（含 member 隔离校验）
	LoadConversation(ctx context.Context, convID, memberID string) (Conversation, error)
	// ListConversations 成员的会话列表（游标分页，按最后消息时间倒序）
	ListConversations(ctx context.Context, memberID string, limit int, cursor string) (convs []Conversation, nextCursor string, err error)
	// DeleteConversation 软删除会话（含 member 隔离校验）
	DeleteConversation(ctx context.Context, convID, memberID string) error
	// LoadMessages 按时间正序取历史消息
	LoadMessages(ctx context.Context, convID, memberID string) ([]Message, error)
	// LoadMessageList 消息列表视图（API 层展示用，带 id/时间）
	LoadMessageList(ctx context.Context, convID, memberID string, limit int) ([]MessageView, error)
	// AppendMessages 追加消息（同一事务，避免半截历史）
	AppendMessages(ctx context.Context, convID, memberID string, msgs []Message) error
	// Permissions 取成员权限集合（权限双保险源头，ADR-005）
	Permissions(ctx context.Context, memberID string) (map[string]bool, error)
}

// Service 会话服务：负责装配请求上下文（历史 + 权限 + 模型）。
type Service interface {
	// Ensure 会话存在（convID 空则新建），返回会话与历史
	Ensure(ctx context.Context, convID, memberID string) (conv Conversation, history []Message, err error)
	// Create 显式新建会话（可指定标题与模型）
	Create(ctx context.Context, memberID, title, modelID string) (convID string, err error)
	// List 成员的会话列表（游标分页）
	List(ctx context.Context, memberID string, limit int, cursor string) (convs []Conversation, nextCursor string, err error)
	// Load 取会话详情 + 历史消息（切换会话时用）
	Load(ctx context.Context, convID, memberID string) (conv Conversation, history []Message, err error)
	// ListMessages 消息列表（切换会话时前端加载，带 id/时间）
	ListMessages(ctx context.Context, convID, memberID string, limit int) (items []MessageView, err error)
	// Delete 软删除会话
	Delete(ctx context.Context, convID, memberID string) error
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
		convID, err = s.repo.CreateConversation(ctx, memberID, "", "")
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

// Create 显式新建会话（前端「新建对话」按钮调用）。
func (s *service) Create(ctx context.Context, memberID, title, modelID string) (string, error) {
	if memberID == "" {
		return "", apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}
	return s.repo.CreateConversation(ctx, memberID, title, modelID)
}

// List 成员的会话列表。
func (s *service) List(ctx context.Context, memberID string, limit int, cursor string) ([]Conversation, string, error) {
	if memberID == "" {
		return nil, "", apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}
	return s.repo.ListConversations(ctx, memberID, limit, cursor)
}

// Load 取会话详情与历史消息（切换会话时前端加载用）。
func (s *service) Load(ctx context.Context, convID, memberID string) (Conversation, []Message, error) {
	conv, err := s.repo.LoadConversation(ctx, convID, memberID)
	if err != nil {
		return Conversation{}, nil, err
	}
	history, err := s.repo.LoadMessages(ctx, convID, memberID)
	if err != nil {
		return Conversation{}, nil, err
	}
	return conv, history, nil
}

// ListMessages 消息列表（展示视图）。
func (s *service) ListMessages(ctx context.Context, convID, memberID string, limit int) ([]MessageView, error) {
	if convID == "" {
		return nil, apperr.New(apperr.CodeInvalidInput, "会话 id 不能为空", nil)
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.repo.LoadMessageList(ctx, convID, memberID, limit)
}

// Delete 软删除会话。
func (s *service) Delete(ctx context.Context, convID, memberID string) error {
	if memberID == "" {
		return apperr.New(apperr.CodePermission, "缺少成员身份", nil)
	}
	if convID == "" {
		return apperr.New(apperr.CodeInvalidInput, "会话 id 不能为空", nil)
	}
	return s.repo.DeleteConversation(ctx, convID, memberID)
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
