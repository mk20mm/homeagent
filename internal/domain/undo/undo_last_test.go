package undo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// --- 测试替身 ---

type stubStore struct {
	records  []Record
	markedID string
	failList bool
}

func (s *stubStore) ListActive(_ context.Context, _ string) ([]Record, error) {
	if s.failList {
		return nil, apperr.New(apperr.CodeInternal, "查询失败", nil)
	}
	return s.records, nil
}

func (s *stubStore) MarkUsed(_ context.Context, id string) error {
	s.markedID = id
	return nil
}

type stubWriteTool struct {
	spec    tool.Spec
	undone  bool
	failErr error
}

func (t *stubWriteTool) Spec() tool.Spec { return t.spec }

func (t *stubWriteTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return tool.Result{}, nil
}

func (t *stubWriteTool) Undo(_ context.Context, _ json.RawMessage) error {
	if t.failErr != nil {
		return t.failErr
	}
	t.undone = true
	return nil
}

// 构造一个挂了 stubWriteTool 的 Executor（只测 Undo 路径）。
func newExecutorWithStub() (*tool.Executor, *stubWriteTool) {
	reg := tool.NewRegistry()
	st := &stubWriteTool{spec: tool.Spec{
		Name: "record_expense", Risk: tool.RiskHigh, Idempotency: "x",
	}}
	if err := reg.Register(st); err != nil {
		panic(err)
	}
	return tool.NewExecutor(reg, nil, nil), st
}

// --- 测试 ---

func TestUndoLastUndoesMostRecent(t *testing.T) {
	// ListActive 按 createdAt 倒序返回，records[0] 就是最近一笔
	store := &stubStore{records: []Record{
		{ID: "new", ToolName: "record_expense", UndoData: []byte(`{"expense_id":"new"}`), ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "old", ToolName: "record_expense", UndoData: []byte(`{"expense_id":"old"}`), ExpiresAt: time.Now().Add(time.Hour)},
	}}
	exec, st := newExecutorWithStub()
	ut := NewUndoLastTool(store)
	ut.SetExecutor(exec)

	ctx := tool.WithMemberID(context.Background(), "member-1")
	res, err := ut.Execute(ctx, []byte(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !st.undone {
		t.Error("期望 Executor.Undo 被调用")
	}
	if store.markedID != "new" {
		t.Errorf("期望标记最近记录 new，实际 %s", store.markedID)
	}
	if res.Summary == "" {
		t.Error("撤销成功应返回说明文案")
	}
}

func TestUndoLastEmptyListTellsUser(t *testing.T) {
	store := &stubStore{records: nil}
	exec, _ := newExecutorWithStub()
	ut := NewUndoLastTool(store)
	ut.SetExecutor(exec)

	ctx := tool.WithMemberID(context.Background(), "member-1")
	res, err := ut.Execute(ctx, []byte(`{}`))
	if err != nil {
		t.Fatalf("空清单不应报错: %v", err)
	}
	if res.Summary == "" {
		t.Error("空清单应返回明确告知文案")
	}
	var card map[string]any
	if err := json.Unmarshal(res.Card, &card); err != nil {
		t.Fatalf("卡片解析失败: %v", err)
	}
	if card["type"] != "none" {
		t.Errorf("期望卡片 type=none，实际 %v", card["type"])
	}
}

func TestUndoLastExpiredWindow(t *testing.T) {
	store := &stubStore{records: []Record{
		{ID: "expired", ToolName: "record_expense", ExpiresAt: time.Now().Add(-time.Hour)},
	}}
	exec, st := newExecutorWithStub()
	ut := NewUndoLastTool(store)
	ut.SetExecutor(exec)

	ctx := tool.WithMemberID(context.Background(), "member-1")
	res, err := ut.Execute(ctx, []byte(`{}`))
	if err != nil {
		t.Fatalf("过期不应报错: %v", err)
	}
	if st.undone {
		t.Error("过期记录不应执行撤销")
	}
	if store.markedID != "" {
		t.Error("过期记录不应标记已用")
	}
	var card map[string]any
	if err := json.Unmarshal(res.Card, &card); err != nil {
		t.Fatalf("卡片解析失败: %v", err)
	}
	if card["type"] != "expired" {
		t.Errorf("期望卡片 type=expired，实际 %v", card["type"])
	}
}

func TestUndoLastPropagatesUndoFailure(t *testing.T) {
	store := &stubStore{records: []Record{
		{ID: "new", ToolName: "record_expense", ExpiresAt: time.Now().Add(time.Hour)},
	}}
	// 手工造一个执行器，其 record_expense 撤销必失败
	reg := tool.NewRegistry()
	st := &stubWriteTool{
		spec: tool.Spec{
			Name: "record_expense", Risk: tool.RiskHigh, Idempotency: "x",
		},
		failErr: errors.New("db down"),
	}
	if err := reg.Register(st); err != nil {
		t.Fatal(err)
	}
	exec := tool.NewExecutor(reg, nil, nil)
	ut := NewUndoLastTool(store)
	ut.SetExecutor(exec)

	ctx := tool.WithMemberID(context.Background(), "member-1")
	if _, err := ut.Execute(ctx, []byte(`{}`)); err == nil {
		t.Error("撤销失败时应向上传播错误，不静默成功")
	}
	if store.markedID != "" {
		t.Error("撤销失败时不应标记已用")
	}
}

func TestUndoLastRequiresMemberID(t *testing.T) {
	store := &stubStore{records: []Record{
		{ID: "new", ToolName: "record_expense", ExpiresAt: time.Now().Add(time.Hour)},
	}}
	exec, _ := newExecutorWithStub()
	ut := NewUndoLastTool(store)
	ut.SetExecutor(exec)

	// 无 memberID（ctx 未注入）
	if _, err := ut.Execute(context.Background(), []byte(`{}`)); err == nil {
		t.Error("缺少成员身份应报错")
	}
}
