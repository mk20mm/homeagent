package run

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

type mockRunService struct {
	createdRun   RunDetail
	steps        []RunStepItem
	statusUpdate map[string]schema.RunStatus
}

func (m *mockRunService) CreateRun(ctx context.Context, memberID string, goal string, intentKind schema.IntentKind, requestID string, conversationID string, requestedEffects []string) (RunDetail, error) {
	m.createdRun = RunDetail{
		ID:         "run-test-123",
		Goal:       goal,
		IntentKind: string(intentKind),
		Status:     "running",
		Version:    1,
		RequestID:  requestID,
	}
	return m.createdRun, nil
}

func (m *mockRunService) GetRun(ctx context.Context, memberID string, runID string) (RunDetail, error) {
	return m.createdRun, nil
}

func (m *mockRunService) ListRuns(ctx context.Context, memberID string, status string, limit int) ([]RunItem, error) {
	return nil, nil
}

func (m *mockRunService) UpdateRunStatus(ctx context.Context, runID string, expectedVersion int, nextStatus schema.RunStatus) (RunDetail, error) {
	if m.statusUpdate == nil {
		m.statusUpdate = make(map[string]schema.RunStatus)
	}
	m.statusUpdate[runID] = nextStatus
	m.createdRun.Status = string(nextStatus)
	return m.createdRun, nil
}

func (m *mockRunService) AddRunStep(ctx context.Context, runID string, step RunStepInput) (RunStepItem, error) {
	item := RunStepItem{
		ID:             step.OperationKey,
		Position:       step.Position,
		ToolName:       step.ToolName,
		Status:         "pending",
		WaitFor:        string(step.WaitFor),
		CompletionKind: step.CompletionKind,
	}
	if item.ID == "" {
		item.ID = "step-id"
	}
	m.steps = append(m.steps, item)
	return item, nil
}

func (m *mockRunService) ConfirmRunStep(ctx context.Context, memberID string, runID string, stepID string, choice string, nonce string) (RunDetail, error) {
	return m.createdRun, nil
}

func (m *mockRunService) ReplyRun(ctx context.Context, memberID string, runID string, stepID string, expectedVersion int, answers map[string]any) (RunDetail, error) {
	return m.createdRun, nil
}

func (m *mockRunService) AmendRun(ctx context.Context, memberID string, runID string, expectedVersion int, modifications []map[string]any) (RunDetail, error) {
	return m.createdRun, nil
}

func TestSubmitTaskPlanTool_ExecuteAndDAG(t *testing.T) {
	mockSvc := &mockRunService{}
	planTool := NewSubmitTaskPlanTool(mockSvc)

	ctx := tool.WithMemberID(context.Background(), "mem-1")
	ctx = tool.WithConversationID(ctx, "conv-1")

	// 1. 成功场景：2步无环计划
	validInput := `{
		"goal": "做完两道菜后扫厨房",
		"requested_effects": ["cooking", "cleaning"],
		"steps": [
			{"position": 1, "tool_name": "start_cooking", "input": {"recipes": ["r1", "r2"]}},
			{"position": 2, "tool_name": "start_cleaning", "input": {"room": "kitchen"}, "depends_on_positions": [1], "wait_for": "entity_completed"}
		]
	}`

	res, err := planTool.Execute(ctx, json.RawMessage(validInput))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !strings.Contains(res.Summary, "已创建复合计划") {
		t.Fatalf("unexpected summary: %s", res.Summary)
	}
	if len(mockSvc.steps) != 2 {
		t.Fatalf("expected 2 steps added, got %d", len(mockSvc.steps))
	}

	// 2. 撤销场景 (Undo)
	err = planTool.Undo(ctx, res.UndoData)
	if err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if mockSvc.statusUpdate["run-test-123"] != schema.RunStatusCancelled {
		t.Fatalf("expected status cancelled, got %v", mockSvc.statusUpdate["run-test-123"])
	}

	// 3. 循环依赖检测：step 1 depends on 2
	cycleInput := `{
		"goal": "循环依赖",
		"steps": [
			{"position": 1, "tool_name": "t1", "input": {}, "depends_on_positions": [2]},
			{"position": 2, "tool_name": "t2", "input": {}}
		]
	}`
	_, err = planTool.Execute(ctx, json.RawMessage(cycleInput))
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if appErr, ok := err.(*apperr.Error); ok {
		if appErr.Code != apperr.CodeInvalidInput {
			t.Fatalf("expected CodeInvalidInput, got %v", appErr.Code)
		}
	}

	// 4. 自依赖检测：step 1 depends on 1
	selfDepInput := `{
		"goal": "自依赖",
		"steps": [
			{"position": 1, "tool_name": "t1", "input": {}, "depends_on_positions": [1]}
		]
	}`
	_, err = planTool.Execute(ctx, json.RawMessage(selfDepInput))
	if err == nil {
		t.Fatal("expected self dep error, got nil")
	}

	// 5. 步骤超限 (超过 10 步)
	steps11 := make([]PlanStepInput, 11)
	for i := 0; i < 11; i++ {
		steps11[i] = PlanStepInput{Position: i + 1, ToolName: "t", Input: map[string]any{}}
	}
	payload11, _ := json.Marshal(SubmitTaskPlanInput{Goal: "超限", Steps: steps11})
	_, err = planTool.Execute(ctx, payload11)
	if err == nil {
		t.Fatal("expected over-limit error, got nil")
	}
}
