package run

import (
	"context"
	"testing"
	"time"

	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

type mockRunRepo struct {
	runs      map[string]RunDetail
	children  map[string][]RunDetail
	events    []RunEventItem
	statuses  map[string]schema.RunStatus
}

func newMockRunRepo() *mockRunRepo {
	return &mockRunRepo{
		runs:     make(map[string]RunDetail),
		children: make(map[string][]RunDetail),
		statuses: make(map[string]schema.RunStatus),
	}
}

func (m *mockRunRepo) CreateRun(_ context.Context, memberID string, goal string, intentKind schema.IntentKind, requestID string, conversationID string, requestedEffects []string) (RunDetail, error) {
	r := RunDetail{ID: "run-root", MemberID: memberID, Goal: goal, Status: string(schema.RunStatusRunning)}
	m.runs[r.ID] = r
	return r, nil
}

func (m *mockRunRepo) CreateChildRun(_ context.Context, memberID string, parentRunID string, goal string, intentKind schema.IntentKind) (RunDetail, error) {
	r := RunDetail{ID: "run-child-" + goal, MemberID: memberID, ParentRunID: parentRunID, Goal: goal, Status: string(schema.RunStatusRunning)}
	m.runs[r.ID] = r
	m.children[parentRunID] = append(m.children[parentRunID], r)
	return r, nil
}

func (m *mockRunRepo) FindChildRuns(ctx context.Context, parentRunID string) ([]RunDetail, error) {
	var out []RunDetail
	for _, c := range m.children[parentRunID] {
		cur, _ := m.GetRun(ctx, c.MemberID, c.ID)
		out = append(out, cur)
	}
	return out, nil
}

func (m *mockRunRepo) GetRun(_ context.Context, _ string, runID string) (RunDetail, error) {
	r := m.runs[runID]
	if st, ok := m.statuses[runID]; ok {
		r.Status = string(st)
	}
	return r, nil
}

func (m *mockRunRepo) GetRunByID(_ context.Context, runID string) (RunDetail, error) {
	r := m.runs[runID]
	if st, ok := m.statuses[runID]; ok {
		r.Status = string(st)
	}
	return r, nil
}

func (m *mockRunRepo) ListRuns(_ context.Context, _ string, _ string, _ int) ([]RunItem, error) {
	return nil, nil
}

func (m *mockRunRepo) FindInterruptedRuns(_ context.Context, _ time.Time, _ int) ([]RunDetail, error) {
	return nil, nil
}

func (m *mockRunRepo) UpdateRunStatus(_ context.Context, runID string, _ int, nextStatus schema.RunStatus) (RunDetail, error) {
	m.statuses[runID] = nextStatus
	r := m.runs[runID]
	r.Status = string(nextStatus)
	m.runs[runID] = r
	return r, nil
}

func (m *mockRunRepo) AddRunStep(_ context.Context, _ string, _ RunStepInput) (RunStepItem, error) {
	return RunStepItem{}, nil
}

func (m *mockRunRepo) UpdateRunStep(_ context.Context, _ string, _ int, _ schema.StepStatus, _ string) (RunStepItem, error) {
	return RunStepItem{}, nil
}

func (m *mockRunRepo) RecordRunEvent(_ context.Context, runID string, stepID string, eventType string, payload string) (RunEventItem, error) {
	ev := RunEventItem{ID: "ev-1", StepID: stepID, EventType: eventType, Payload: payload}
	m.events = append(m.events, ev)
	return ev, nil
}

func TestService_CascadeCancel(t *testing.T) {
	repo := newMockRunRepo()
	svc := NewService(repo)
	ctx := context.Background()

	// 1. 创建父任务与子任务
	parent, _ := repo.CreateRun(ctx, "mem-1", "大扫除", schema.IntentPlan, "req-1", "", nil)
	child1, _ := repo.CreateChildRun(ctx, "mem-1", parent.ID, "扫地", schema.IntentAct)
	child2, _ := repo.CreateChildRun(ctx, "mem-1", parent.ID, "擦窗户", schema.IntentAct)
	// child2 已经完成
	repo.statuses[child2.ID] = schema.RunStatusCompleted
	c2 := repo.runs[child2.ID]
	c2.Status = string(schema.RunStatusCompleted)
	repo.runs[child2.ID] = c2

	// 2. 取消父任务
	cancelledParent, err := svc.CancelRun(ctx, "mem-1", parent.ID, "用户外出")
	if err != nil {
		t.Fatalf("cancel parent run failed: %v", err)
	}
	if cancelledParent.Status != string(schema.RunStatusCancelled) {
		t.Fatalf("expected parent cancelled, got %s", cancelledParent.Status)
	}

	// 3. 验证 child1 被级联取消，child2 维持 completed
	child1Updated, _ := repo.GetRun(ctx, "mem-1", child1.ID)
	if child1Updated.Status != string(schema.RunStatusCancelled) {
		t.Fatalf("expected running child1 to be cascade cancelled, got %s", child1Updated.Status)
	}

	child2Updated, _ := repo.GetRun(ctx, "mem-1", child2.ID)
	if child2Updated.Status != string(schema.RunStatusCompleted) {
		t.Fatalf("expected completed child2 to stay completed, got %s", child2Updated.Status)
	}
}

func TestService_SteerRun(t *testing.T) {
	repo := newMockRunRepo()
	svc := NewService(repo)
	ctx := context.Background()

	run, _ := repo.CreateRun(ctx, "mem-1", "做晚饭", schema.IntentPlan, "req-1", "", nil)

	_, err := svc.SteerRun(ctx, "mem-1", run.ID, "不要放辣")
	if err != nil {
		t.Fatalf("steer run failed: %v", err)
	}

	foundSteeredEvent := false
	for _, ev := range repo.events {
		if ev.EventType == "human_steered" {
			foundSteeredEvent = true
			break
		}
	}
	if !foundSteeredEvent {
		t.Fatal("expected human_steered event to be recorded")
	}
}

func TestService_CreateChildRun(t *testing.T) {
	repo := newMockRunRepo()
	svc := NewService(repo)
	ctx := context.Background()

	parent, _ := repo.CreateRun(ctx, "mem-1", "家庭大采购", schema.IntentPlan, "req-1", "", nil)

	child, err := svc.CreateChildRun(ctx, "mem-1", parent.ID, "采购水果", schema.IntentAct)
	if err != nil {
		t.Fatalf("create child run failed: %v", err)
	}
	if child.ParentRunID != parent.ID {
		t.Fatalf("expected parent %s, got %s", parent.ID, child.ParentRunID)
	}

	foundSpawnEvent := false
	for _, ev := range repo.events {
		if ev.EventType == "child_run_spawned" {
			foundSpawnEvent = true
			break
		}
	}
	if !foundSpawnEvent {
		t.Fatal("expected child_run_spawned event to be recorded on parent")
	}
}
