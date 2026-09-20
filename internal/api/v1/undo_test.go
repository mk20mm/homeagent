package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// stubUndoStore 撤销存储桩（ListUndo 用）。
type stubUndoStore struct {
	records []UndoRecord
}

func (s *stubUndoStore) GetUndo(_ context.Context, _, _ string) (UndoRecord, error) {
	return UndoRecord{}, nil
}

func (s *stubUndoStore) MarkUsed(_ context.Context, _ string) error { return nil }

func (s *stubUndoStore) ListActive(_ context.Context, _ string) ([]UndoRecord, error) {
	// 与 repo 一致：新的在前
	out := make([]UndoRecord, len(s.records))
	copy(out, s.records)
	for i := len(out) - 1; i > 0; i-- {
		for j := 0; j < i; j++ {
			if out[j].CreatedAt.Before(out[j+1].CreatedAt) {
				out[j], out[j+1] = out[j+1], out[j]
			}
		}
	}
	return out, nil
}

// stubSummaryProvider 摘要查询桩。
type stubSummaryProvider struct {
	cents    int64
	category string
	title    string
	fail     bool
}

func (s *stubSummaryProvider) ExpenseBrief(_ context.Context, _, _ string) (int64, string, error) {
	if s.fail {
		return 0, "", apperr.New(apperr.CodeNotFound, "找不到", nil)
	}
	return s.cents, s.category, nil
}

func (s *stubSummaryProvider) TaskTitle(_ context.Context, _ string) (string, error) {
	if s.fail {
		return "", apperr.New(apperr.CodeNotFound, "找不到", nil)
	}
	return s.title, nil
}

func setupUndoRouter(store UndoStore, sp UndoSummaryProvider) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(func(c *gin.Context) {
		c.Set("member_id", "member-1")
		c.Next()
	})
	g.GET("/undo", ListUndo(store, sp))
	return r
}

func TestListUndoSummaries(t *testing.T) {
	now := time.Now()
	store := &stubUndoStore{records: []UndoRecord{
		{
			ID: "u1", ToolName: "record_expense",
			UndoData:  json.RawMessage(`{"expense_id":"e1"}`),
			ExpiresAt: now.Add(20 * time.Hour), CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			ID: "u2", ToolName: "assign_task",
			UndoData:  json.RawMessage(`{"task_id":"t1"}`),
			ExpiresAt: now.Add(10 * time.Hour), CreatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID: "u3", ToolName: "report_meal",
			UndoData:  json.RawMessage(`{"date":"2026-09-18"}`),
			ExpiresAt: now.Add(5 * time.Hour), CreatedAt: now,
		},
	}}
	sp := &stubSummaryProvider{cents: 1280, category: "食材", title: "洗碗"}
	r := setupUndoRouter(store, sp)

	w := adminDo(r, http.MethodGet, "/api/v1/undo", "")
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			UndoID   string `json:"undo_id"`
			ToolName string `json:"tool_name"`
			Summary  string `json:"summary"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out.Items) != 3 {
		t.Fatalf("应返回 3 项，got %d", len(out.Items))
	}
	// 最新的在前（report_meal 创建时间最晚）
	if out.Items[0].UndoID != "u3" {
		t.Fatalf("应按创建时间倒序，首项 %s", out.Items[0].UndoID)
	}
	expect := map[string]string{
		"u1": "记账 ¥12.80 · 食材",
		"u2": "派任务：洗碗",
		"u3": "报饭 09-18",
	}
	for _, it := range out.Items {
		if it.Summary != expect[it.UndoID] {
			t.Errorf("summary %s = %q, want %q", it.UndoID, it.Summary, expect[it.UndoID])
		}
	}
}

func TestListUndoSummaryFallback(t *testing.T) {
	now := time.Now()
	store := &stubUndoStore{records: []UndoRecord{
		{ID: "u1", ToolName: "record_expense", UndoData: json.RawMessage(`{"expense_id":"gone"}`), CreatedAt: now},
		{ID: "u2", ToolName: "switch_model", UndoData: json.RawMessage(`{}`), CreatedAt: now.Add(-time.Hour)},
	}}
	sp := &stubSummaryProvider{fail: true} // 查询全失败
	r := setupUndoRouter(store, sp)

	w := adminDo(r, http.MethodGet, "/api/v1/undo", "")
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			UndoID  string `json:"undo_id"`
			Summary string `json:"summary"`
		} `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Items) != 2 {
		t.Fatalf("应返回 2 项，got %d", len(out.Items))
	}
	for _, it := range out.Items {
		want := "记账"
		if it.UndoID == "u2" {
			want = "切换模型"
		}
		if it.Summary != want {
			t.Errorf("降级摘要 %s = %q, want %q", it.UndoID, it.Summary, want)
		}
	}
}

func TestListUndoRequiresMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1")
	g.GET("/undo", ListUndo(&stubUndoStore{}, nil)) // 无 member_id 中间件

	w := adminDo(r, http.MethodGet, "/api/v1/undo", "")
	if w.Code != 403 {
		t.Fatalf("缺成员身份应 403，got %d", w.Code)
	}
}
