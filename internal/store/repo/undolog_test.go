package repo

import (
	"context"
	"testing"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
)

func TestUndoListActive(t *testing.T) {
	c, famID := newTestClient(t)
	s := New(c)
	memberID := testMemberID(t, c)
	ctx := tool.WithTraceID(context.Background(), "trace-1")

	save := func(toolName string, expiresInSec int64, memberID string) string {
		id, err := s.Save(ctx, tool.UndoRecord{
			TraceID:   "trace-1",
			MemberID:  memberID,
			ToolName:  toolName,
			UndoData:  []byte(`{"task_id":"t1"}`),
			ExpiresIn: expiresInSec,
		})
		if err != nil {
			t.Fatalf("save %s: %v", toolName, err)
		}
		return id
	}

	// 活跃：未过期
	save("assign_task", 3600, memberID)
	// 已过期（Save 用 ExpiresIn=负值 → 过期）
	save("complete_task", -1, memberID)

	// 另一个成员的记录（newTestExecutor 建的执行人）
	_, otherID := newTestExecutor(t, c, famID)
	save("report_meal", 3600, otherID)

	list, err := s.ListActive(ctx, memberID)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("只应返回我自己的 1 条活跃记录，got %d", len(list))
	}
	if list[0].ToolName != "assign_task" {
		t.Fatalf("应是 assign_task，got %q", list[0].ToolName)
	}
	if list[0].CreatedAt.IsZero() {
		t.Error("CreatedAt 应被填充")
	}

	// 用掉之后不再出现
	if err := s.MarkUsed(ctx, list[0].ID); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}
	list2, _ := s.ListActive(ctx, memberID)
	if len(list2) != 0 {
		t.Fatalf("已使用的记录不应出现，got %d", len(list2))
	}
}

// SaveUndo（HTTP handler 路径）固定 24h 窗口，不依赖调用者传参。
func TestUndoSaveUndoWindow(t *testing.T) {
	c, _ := newTestClient(t)
	s := New(c)
	memberID := testMemberID(t, c)
	ctx := context.Background()

	id, err := s.SaveUndo(ctx, memberID, "record_expense", []byte(`{"expense_id":"e1"}`))
	if err != nil {
		t.Fatalf("SaveUndo: %v", err)
	}
	rec, err := s.GetUndo(ctx, id, memberID)
	if err != nil {
		t.Fatalf("GetUndo: %v", err)
	}
	if !rec.ExpiresAt.After(time.Now().Add(23 * time.Hour)) {
		t.Fatalf("SaveUndo 应固定 24h 窗口，got %v", rec.ExpiresAt)
	}
	list, _ := s.ListActive(ctx, memberID)
	if len(list) != 1 {
		t.Fatalf("应在 24h 窗口内可见，got %d", len(list))
	}
}
