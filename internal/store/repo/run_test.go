package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	domrun "github.com/mk20mm/homeagent/internal/domain/run"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

func TestRunRepo_Lifecycle(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	ctx := context.Background()
	memberID := testMemberID(t, c)

	reqID := uuid.NewString()
	goal := "做两道菜，做完后扫厨房"

	// 1. 创建 Run
	run, err := s.CreateRun(ctx, memberID, goal, schema.IntentPlan, reqID, "", []string{"cooking", "cleaning"})
	if err != nil {
		t.Fatalf("create run failed: %v", err)
	}
	if run.Goal != goal {
		t.Fatalf("expected goal %q, got %q", goal, run.Goal)
	}
	if run.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", run.Version)
	}

	// 2. 幂等创建：相同 request_id 返回原记录
	run2, err := s.CreateRun(ctx, memberID, goal, schema.IntentPlan, reqID, "", nil)
	if err != nil {
		t.Fatalf("idempotent create run failed: %v", err)
	}
	if run2.ID != run.ID {
		t.Fatalf("expected same run ID %s, got %s", run.ID, run2.ID)
	}

	// 3. 添加步骤
	step1, err := s.AddRunStep(ctx, run.ID, domrun.RunStepInput{
		Position:       1,
		ToolName:       "start_cooking",
		InputJSON:      `{"recipe_ids":["r1","r2"]}`,
		OperationKey:   "op-cook-1",
		WaitFor:        schema.WaitForNone,
		CompletionKind: "local_committed",
	})
	if err != nil {
		t.Fatalf("add step 1 failed: %v", err)
	}
	if step1.Position != 1 || step1.ToolName != "start_cooking" {
		t.Fatalf("unexpected step 1: %+v", step1)
	}

	step2, err := s.AddRunStep(ctx, run.ID, domrun.RunStepInput{
		Position:         2,
		ToolName:         "start_cleaning",
		InputJSON:        `{"room":"kitchen"}`,
		OperationKey:     "op-clean-1",
		DependsOnStepIDs: []string{step1.ID},
		WaitFor:          schema.WaitForEntityCompleted,
		CompletionKind:   "observed_device_success",
	})
	if err != nil {
		t.Fatalf("add step 2 failed: %v", err)
	}
	if step2.Position != 2 {
		t.Fatalf("expected position 2, got %d", step2.Position)
	}

	// 4. 更新步骤
	card := `{"type":"action","summary":"已启动做饭指导"}`
	updStep, err := s.UpdateRunStep(ctx, step1.ID, 1, schema.StepStatusCommitted, card)
	if err != nil {
		t.Fatalf("update step 1 failed: %v", err)
	}
	if updStep.Status != "committed" || updStep.ResultCard != card {
		t.Fatalf("unexpected updated step: %+v", updStep)
	}

	// 5. 更新 Run 状态
	updRun, err := s.UpdateRunStatus(ctx, run.ID, 1, schema.RunStatusWaitingExternal)
	if err != nil {
		t.Fatalf("update run status failed: %v", err)
	}
	if updRun.Status != "waiting_external" || updRun.Version != 2 {
		t.Fatalf("unexpected updated run status/version: %+v", updRun)
	}

	// 6. 版本冲突测试
	_, err = s.UpdateRunStatus(ctx, run.ID, 1, schema.RunStatusCompleted)
	if err == nil {
		t.Fatal("expected conflict error on mismatched expected version, got nil")
	}

	// 7. 查询详情与验证事件链
	detail, err := s.GetRun(ctx, memberID, run.ID)
	if err != nil {
		t.Fatalf("get run detail failed: %v", err)
	}
	if len(detail.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(detail.Steps))
	}
	if len(detail.Events) < 3 {
		t.Fatalf("expected >=3 events, got %d", len(detail.Events))
	}

	// 8. 列表查询
	list, err := s.ListRuns(ctx, memberID, "", 10)
	if err != nil {
		t.Fatalf("list runs failed: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least 1 run in list")
	}
}
