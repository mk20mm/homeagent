package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	domrun "github.com/mk20mm/homeagent/internal/domain/run"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

type recoveryMockRunService struct {
	mu              sync.Mutex
	runs            map[string]domrun.RunDetail
	recordedEvents  []domrun.RunEventItem
	statusHistory   map[string][]schema.RunStatus
	stepStatusMarks map[string]schema.StepStatus
}

func newRecoveryMockRunService() *recoveryMockRunService {
	return &recoveryMockRunService{
		runs:            make(map[string]domrun.RunDetail),
		statusHistory:   make(map[string][]schema.RunStatus),
		stepStatusMarks: make(map[string]schema.StepStatus),
	}
}

func (m *recoveryMockRunService) CreateRun(ctx context.Context, memberID string, goal string, intentKind schema.IntentKind, requestID string, conversationID string, requestedEffects []string) (domrun.RunDetail, error) {
	return domrun.RunDetail{}, nil
}

func (m *recoveryMockRunService) GetRun(ctx context.Context, memberID string, runID string) (domrun.RunDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[runID], nil
}

func (m *recoveryMockRunService) GetRunByID(ctx context.Context, runID string) (domrun.RunDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[runID], nil
}

func (m *recoveryMockRunService) ListRuns(ctx context.Context, memberID string, status string, limit int) ([]domrun.RunItem, error) {
	return nil, nil
}

func (m *recoveryMockRunService) FindInterruptedRuns(ctx context.Context, updatedBefore time.Time, limit int) ([]domrun.RunDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domrun.RunDetail
	for _, r := range m.runs {
		if r.Status == string(schema.RunStatusRunning) || r.Status == string(schema.RunStatusAwaitingInput) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *recoveryMockRunService) UpdateRunStatus(ctx context.Context, runID string, expectedVersion int, nextStatus schema.RunStatus) (domrun.RunDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusHistory[runID] = append(m.statusHistory[runID], nextStatus)
	r := m.runs[runID]
	r.Status = string(nextStatus)
	m.runs[runID] = r
	return r, nil
}

func (m *recoveryMockRunService) AddRunStep(ctx context.Context, runID string, step domrun.RunStepInput) (domrun.RunStepItem, error) {
	return domrun.RunStepItem{}, nil
}

func (m *recoveryMockRunService) UpdateRunStep(ctx context.Context, stepID string, expectedVersion int, status schema.StepStatus, resultCard string) (domrun.RunStepItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stepStatusMarks[stepID] = status
	return domrun.RunStepItem{ID: stepID, Status: string(status)}, nil
}

func (m *recoveryMockRunService) ConfirmRunStep(ctx context.Context, memberID string, runID string, stepID string, choice string, nonce string) (domrun.RunDetail, error) {
	return domrun.RunDetail{}, nil
}

func (m *recoveryMockRunService) ReplyRun(ctx context.Context, memberID string, runID string, stepID string, expectedVersion int, answers map[string]any) (domrun.RunDetail, error) {
	return domrun.RunDetail{}, nil
}

func (m *recoveryMockRunService) AmendRun(ctx context.Context, memberID string, runID string, expectedVersion int, modifications []map[string]any) (domrun.RunDetail, error) {
	return domrun.RunDetail{}, nil
}

func (m *recoveryMockRunService) CreateChildRun(ctx context.Context, memberID string, parentRunID string, goal string, intentKind schema.IntentKind) (domrun.RunDetail, error) {
	return domrun.RunDetail{ID: "mock-child", Goal: goal, ParentRunID: parentRunID}, nil
}

func (m *recoveryMockRunService) CancelRun(ctx context.Context, memberID string, runID string, reason string) (domrun.RunDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.runs[runID]
	r.Status = "cancelled"
	m.runs[runID] = r
	return r, nil
}

func (m *recoveryMockRunService) SteerRun(ctx context.Context, memberID string, runID string, instruction string) (domrun.RunDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[runID], nil
}

func (m *recoveryMockRunService) RecordRunEvent(ctx context.Context, runID string, stepID string, eventType string, payload string) (domrun.RunEventItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item := domrun.RunEventItem{
		ID:        uuid.NewString(),
		Seq:       len(m.recordedEvents) + 1,
		StepID:    stepID,
		EventType: eventType,
		Payload:   payload,
		CreatedAt: time.Now(),
	}
	m.recordedEvents = append(m.recordedEvents, item)

	// 也同步更新对应 run.Events
	if r, ok := m.runs[runID]; ok {
		r.Events = append(r.Events, item)
		m.runs[runID] = r
	}
	return item, nil
}

// 模拟场景 A：第 2 个工具（safe 查询类工具）在执行中崩溃 → 重启后自动重放该工具并继续生成。
func TestRecovery_ScenarioA_SafeToolReplayed(t *testing.T) {
	reg := tool.NewRegistry()
	var executedTools []string
	var mu sync.Mutex

	safeTool := &stubToolWithCallback{
		spec: tool.Spec{Name: "safe_query", Risk: tool.RiskLow, Replayable: true},
		onExec: func() {
			mu.Lock()
			executedTools = append(executedTools, "safe_query")
			mu.Unlock()
		},
		result: tool.Result{Summary: "查询到预算剩余 800 元"},
	}
	unsafeTool := &stubWriteToolWithCallback{
		stubToolWithCallback: stubToolWithCallback{
			spec: tool.Spec{Name: "unsafe_write", Risk: tool.RiskMedium, Idempotency: "x", Replayable: false},
			onExec: func() {
				mu.Lock()
				executedTools = append(executedTools, "unsafe_write")
				mu.Unlock()
			},
			result: tool.Result{Summary: "已记账 50 元"},
		},
	}
	_ = reg.Register(safeTool)
	_ = reg.Register(unsafeTool)

	exec := tool.NewExecutor(reg, nil, nil)
	sessions := newMemSession(map[string]map[string]bool{"m1": {}})
	mockRunSvc := newRecoveryMockRunService()

	runID := "run-scenario-a"
	convID, _ := sessions.CreateConversation(context.Background(), "m1", "test", "")

	// 构造崩溃前产生的事件：
	// Turn 1 中 LLM 要求调用 unsafe_write 和 safe_query
	// unsafe_write 已经执行完成并记录了 tool_result
	// safe_query 刚刚记录 tool_intent，随后进程崩溃！
	tc1 := gateway.ToolCall{ID: "call-1", Name: "unsafe_write", Args: json.RawMessage(`{}`)}
	tc2 := gateway.ToolCall{ID: "call-2", Name: "safe_query", Args: json.RawMessage(`{}`)}

	respPayload, _ := json.Marshal(map[string]any{
		"turn":       1,
		"content":    "我来帮您记账并查询预算",
		"tool_calls": []gateway.ToolCall{tc1, tc2},
	})
	intent1, _ := json.Marshal(map[string]any{"tool": "unsafe_write", "tool_call_id": "call-1"})
	result1, _ := json.Marshal(map[string]any{"tool": "unsafe_write", "tool_call_id": "call-1", "summary": "已记账 50 元"})
	intent2, _ := json.Marshal(map[string]any{"tool": "safe_query", "tool_call_id": "call-2"})

	mockRunSvc.runs[runID] = domrun.RunDetail{
		ID:             runID,
		MemberID:       "m1",
		ConversationID: convID,
		Goal:           "记账50并查预算",
		Status:         "running",
		Version:        1,
		Events: []domrun.RunEventItem{
			{Seq: 1, EventType: EventLLMRequestSent, Payload: `{"turn":1}`},
			{Seq: 2, EventType: EventLLMResponseReceived, Payload: string(respPayload)},
			{Seq: 3, EventType: EventToolIntent, Payload: string(intent1), StepID: "call-1"},
			{Seq: 4, EventType: EventToolResult, Payload: string(result1), StepID: "call-1"},
			{Seq: 5, EventType: EventToolIntent, Payload: string(intent2), StepID: "call-2"},
			// 崩溃点：此时 safe_query 的 EventToolResult 尚未写入
		},
	}

	// 恢复后 LLM 在 Turn 2 会收到 safe_query 的重放结果，并生成最终文字
	var turn2ReceivedMessages []gateway.Message
	scriptedProvider := gateway.NewScriptedProvider(func(round int, req gateway.ChatRequest) []gateway.StreamEvent {
		turn2ReceivedMessages = req.Messages
		return []gateway.StreamEvent{
			{Delta: "已完成记账50元，当前预算剩余800元。"},
			{Done: true},
		}
	})

	rt := New(Options{
		Provider:   scriptedProvider,
		Executor:   exec,
		Sessions:   sessions,
		RunService: mockRunSvc,
	})

	recoveredCount, err := rt.RecoverInterruptedRuns(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("恢复执行失败: %v", err)
	}
	if recoveredCount != 1 {
		t.Fatalf("应成功恢复 1 个任务，实际恢复: %d", recoveredCount)
	}

	mu.Lock()
	defer mu.Unlock()

	// 验证：safe_query 工具被自动重放执行了！
	hasSafeQuery := false
	hasUnsafeWrite := false
	for _, toolName := range executedTools {
		if toolName == "safe_query" {
			hasSafeQuery = true
		}
		if toolName == "unsafe_write" {
			hasUnsafeWrite = true
		}
	}

	if !hasSafeQuery {
		t.Errorf("安全工具 safe_query 应当在恢复时被自动重放执行")
	}
	if hasUnsafeWrite {
		t.Errorf("已执行成功的 unsafe_write 不应被重复执行")
	}

	// 验证 Run 状态最终标记为 completed
	if mockRunSvc.runs[runID].Status != string(schema.RunStatusCompleted) {
		t.Errorf("任务最终状态应为 completed，实际为: %s", mockRunSvc.runs[runID].Status)
	}

	// 验证 Turn 2 的 LLM 请求中包含了 safe_query 的重放结果
	foundSafeQueryResult := false
	for _, m := range turn2ReceivedMessages {
		if m.Role == gateway.RoleTool && strings.Contains(m.Content, "查询到预算剩余 800 元") {
			foundSafeQueryResult = true
			break
		}
	}
	if !foundSafeQueryResult {
		t.Errorf("LLM 上下文未包含 safe_query 重放得到的结果")
	}
}

// 模拟场景 B：unsafe 工具在执行中崩溃 → 重启后禁止重放，向 LLM 报告中断并提示人工确认。
func TestRecovery_ScenarioB_UnsafeToolReportedInterrupted(t *testing.T) {
	reg := tool.NewRegistry()
	var executedTools []string
	var mu sync.Mutex

	unsafeTool := &stubWriteToolWithCallback{
		stubToolWithCallback: stubToolWithCallback{
			spec: tool.Spec{Name: "record_expense", Risk: tool.RiskMedium, Idempotency: "x", Replayable: false},
			onExec: func() {
				mu.Lock()
				executedTools = append(executedTools, "record_expense")
				mu.Unlock()
			},
			result: tool.Result{Summary: "已记账"},
		},
	}
	_ = reg.Register(unsafeTool)

	exec := tool.NewExecutor(reg, nil, nil)
	sessions := newMemSession(map[string]map[string]bool{"m1": {}})
	mockRunSvc := newRecoveryMockRunService()

	runID := "run-scenario-b"
	convID, _ := sessions.CreateConversation(context.Background(), "m1", "test", "")

	tc := gateway.ToolCall{ID: "call-unsafe-1", Name: "record_expense", Args: json.RawMessage(`{}`)}
	respPayload, _ := json.Marshal(map[string]any{
		"turn":       1,
		"content":    "正在帮您记账",
		"tool_calls": []gateway.ToolCall{tc},
	})
	intent, _ := json.Marshal(map[string]any{"tool": "record_expense", "tool_call_id": "call-unsafe-1"})

	mockRunSvc.runs[runID] = domrun.RunDetail{
		ID:             runID,
		MemberID:       "m1",
		ConversationID: convID,
		Goal:           "记账100元",
		Status:         "running",
		Version:        1,
		Steps: []domrun.RunStepItem{
			{ID: "step-1", ToolName: "record_expense", Status: "executing", ExpectedVersion: 1},
		},
		Events: []domrun.RunEventItem{
			{Seq: 1, EventType: EventLLMRequestSent, Payload: `{"turn":1}`},
			{Seq: 2, EventType: EventLLMResponseReceived, Payload: string(respPayload)},
			{Seq: 3, EventType: EventToolIntent, Payload: string(intent), StepID: "call-unsafe-1"},
			// 崩溃点：unsafe 工具执行中进程退出，无 tool_result
		},
	}

	var turn2Messages []gateway.Message
	scriptedProvider := gateway.NewScriptedProvider(func(round int, req gateway.ChatRequest) []gateway.StreamEvent {
		turn2Messages = req.Messages
		return []gateway.StreamEvent{
			{Delta: "上次记账操作被意外中断，请您核对账单后告诉我是否需要重新记录。"},
			{Done: true},
		}
	})

	rt := New(Options{
		Provider:   scriptedProvider,
		Executor:   exec,
		Sessions:   sessions,
		RunService: mockRunSvc,
	})

	recoveredCount, err := rt.RecoverInterruptedRuns(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("恢复执行失败: %v", err)
	}
	if recoveredCount != 1 {
		t.Fatalf("应成功恢复 1 个任务，实际恢复: %d", recoveredCount)
	}

	mu.Lock()
	defer mu.Unlock()

	// 验证：unsafe 工具绝不能被重放！
	if len(executedTools) > 0 {
		t.Fatalf("不安全的写操作 record_expense 绝不能在恢复时被自动重放执行！实际执行: %v", executedTools)
	}

	// 验证步骤被标记为 unknown
	if mockRunSvc.stepStatusMarks["step-1"] != schema.StepStatusUnknown {
		t.Errorf("中断的写操作步骤应被标记为 unknown，实际标记: %s", mockRunSvc.stepStatusMarks["step-1"])
	}

	// 验证 LLM 收到了中断告警提示
	foundInterruptionNotice := false
	for _, m := range turn2Messages {
		if m.Role == gateway.RoleTool && strings.Contains(m.Content, "上一次执行在工具 record_expense 执行中被中断，无法确认执行结果，请决定下一步") {
			foundInterruptionNotice = true
			break
		}
	}
	if !foundInterruptionNotice {
		t.Errorf("LLM 上下文未包含指定的中断提示语")
	}

	// 验证最终状态已更新为 completed
	if mockRunSvc.runs[runID].Status != string(schema.RunStatusCompleted) {
		t.Errorf("任务最终状态应更新为 completed，实际为: %s", mockRunSvc.runs[runID].Status)
	}
}

type stubToolWithCallback struct {
	spec   tool.Spec
	onExec func()
	result tool.Result
}

func (s *stubToolWithCallback) Spec() tool.Spec { return s.spec }
func (s *stubToolWithCallback) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if s.onExec != nil {
		s.onExec()
	}
	return s.result, nil
}

type stubWriteToolWithCallback struct {
	stubToolWithCallback
}

func (w *stubWriteToolWithCallback) Undo(ctx context.Context, undoData json.RawMessage) error {
	return nil
}
