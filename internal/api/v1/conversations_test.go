package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/session"
)

type stubConvService struct {
	conv    session.Conversation
	history []session.Message
}

func (s *stubConvService) Ensure(_ context.Context, _, _ string) (session.Conversation, []session.Message, error) {
	return s.conv, s.history, nil
}
func (s *stubConvService) Create(_ context.Context, _, _, _ string) (string, error) {
	return s.conv.ID, nil
}
func (s *stubConvService) Fork(_ context.Context, convID, memberID, forkAtMessageID, title string) (session.Conversation, error) {
	return session.Conversation{
		ID:                "forked-conv-123",
		MemberID:          memberID,
		Title:             title,
		ForkedFromID:      convID,
		ForkedAtMessageID: forkAtMessageID,
		CreatedAt:         time.Now(),
	}, nil
}
func (s *stubConvService) List(_ context.Context, _ string, _ int, _ string) ([]session.Conversation, string, error) {
	return []session.Conversation{s.conv}, "", nil
}
func (s *stubConvService) Load(_ context.Context, _, _ string) (session.Conversation, []session.Message, error) {
	return s.conv, s.history, nil
}
func (s *stubConvService) ListMessages(_ context.Context, _, _ string, _ int) ([]session.MessageView, error) {
	return nil, nil
}
func (s *stubConvService) Delete(_ context.Context, _, _ string) error {
	return nil
}
func (s *stubConvService) Append(_ context.Context, _, _ string, _ []session.Message) error {
	return nil
}
func (s *stubConvService) Permissions(_ context.Context, _ string) (map[string]bool, error) {
	return map[string]bool{"query": true}, nil
}
func (s *stubConvService) SetModel(_ context.Context, _, _, _ string) error {
	return nil
}

func setupConvRouter(svc session.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authMiddleware := func(c *gin.Context) {
		c.Set("member_id", "mem-test-1")
		c.Next()
	}
	api := r.Group("", authMiddleware)
	api.GET("/conversations", ListConversations(svc))
	api.POST("/conversations", CreateConversation(svc))
	api.POST("/conversations/:conversationId/fork", ForkConversation(svc))
	return r
}

func TestConversationsAPI_Fork(t *testing.T) {
	svc := &stubConvService{
		conv: session.Conversation{
			ID:       "orig-conv-1",
			MemberID: "mem-test-1",
			Title:    "原始对话",
		},
	}
	r := setupConvRouter(svc)

	body := `{"fork_at_message_id":"msg-456","title":"分支对话探索"}`
	req, _ := http.NewRequest(http.MethodPost, "/conversations/orig-conv-1/fork", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /conversations/:id/fork failed with code: %d, body: %s", w.Code, w.Body.String())
	}

	var resp conversationItem
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID != "forked-conv-123" {
		t.Errorf("expected forked conv id 'forked-conv-123', got '%s'", resp.ID)
	}
	if resp.ForkedFromID == nil || *resp.ForkedFromID != "orig-conv-1" {
		t.Errorf("expected ForkedFromID 'orig-conv-1', got %v", resp.ForkedFromID)
	}
	if resp.ForkedAtMessageID == nil || *resp.ForkedAtMessageID != "msg-456" {
		t.Errorf("expected ForkedAtMessageID 'msg-456', got %v", resp.ForkedAtMessageID)
	}
	if resp.Title != "分支对话探索" {
		t.Errorf("expected title '分支对话探索', got '%s'", resp.Title)
	}
}
