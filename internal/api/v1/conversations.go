package v1

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// conversationItem 会话列表项（对齐 openapi Conversation schema）。
type conversationItem struct {
	ID            string  `json:"id"`
	MemberID      string  `json:"member_id"`
	Title         string  `json:"title"`
	ModelID       *string `json:"model_id"`
	LastMessageAt *string `json:"last_message_at"`
	CreatedAt     string  `json:"created_at"`
}

func toConversationItem(c session.Conversation) conversationItem {
	out := conversationItem{
		ID:       c.ID,
		MemberID: c.MemberID,
		Title:    c.Title,
	}
	if c.ModelID != "" {
		m := c.ModelID
		out.ModelID = &m
	}
	if !c.LastMessageAt.IsZero() {
		t := c.LastMessageAt.Format(time.RFC3339)
		out.LastMessageAt = &t
	}
	if !c.CreatedAt.IsZero() {
		out.CreatedAt = c.CreatedAt.Format(time.RFC3339)
	}
	return out
}

// ListConversations GET /conversations —— 会话列表（游标分页，member 隔离）。
func ListConversations(svc session.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		cursor := c.Query("page_cursor")
		items, nextCursor, err := svc.List(c.Request.Context(), memberID, pageSize, cursor)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		out := make([]conversationItem, 0, len(items))
		for _, it := range items {
			out = append(out, toConversationItem(it))
		}
		c.JSON(200, gin.H{"items": out, "next_cursor": nextCursor})
	}
}

// CreateConversation POST /conversations —— 新建会话（可指定标题与模型）。
func CreateConversation(svc session.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		var req struct {
			Title   string `json:"title"`
			ModelID string `json:"model_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求格式错误", err))
			return
		}
		convID, err := svc.Create(c.Request.Context(), memberID, req.Title, req.ModelID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		conv, _, err := svc.Load(c.Request.Context(), convID, memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(201, toConversationItem(conv))
	}
}

// ListMessages GET /conversations/:id/messages —— 会话历史消息（时间正序）。
func ListMessages(svc session.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		convID := c.Param("conversationId")
		if convID == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "会话 id 不能为空", nil))
			return
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))
		items, err := svc.ListMessages(c.Request.Context(), convID, memberID, pageSize)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		out := make([]gin.H, 0, len(items))
		for _, m := range items {
			item := gin.H{
				"id":              m.ID,
				"conversation_id": m.ConversationID,
				"role":            m.Role,
				"content":         m.Content,
				"created_at":      m.CreatedAt.Format(time.RFC3339),
			}
			if m.ToolName != "" {
				item["tool_name"] = m.ToolName
			}
			out = append(out, item)
		}
		c.JSON(200, gin.H{"items": out})
	}
}

// DeleteConversation DELETE /conversations/:id —— 软删除会话。
func DeleteConversation(svc session.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		convID := c.Param("conversationId")
		if convID == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "会话 id 不能为空", nil))
			return
		}
		if err := svc.Delete(c.Request.Context(), convID, memberID); err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.Status(204)
	}
}
