package run

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

// Repo 数据仓储接口。
type Repo interface {
	CreateRun(ctx context.Context, memberID string, goal string, intentKind schema.IntentKind, requestID string, conversationID string, requestedEffects []string) (RunDetail, error)
	CreateChildRun(ctx context.Context, memberID string, parentRunID string, goal string, intentKind schema.IntentKind) (RunDetail, error)
	FindChildRuns(ctx context.Context, parentRunID string) ([]RunDetail, error)
	GetRun(ctx context.Context, memberID string, runID string) (RunDetail, error)
	GetRunByID(ctx context.Context, runID string) (RunDetail, error)
	ListRuns(ctx context.Context, memberID string, status string, limit int) ([]RunItem, error)
	FindInterruptedRuns(ctx context.Context, updatedBefore time.Time, limit int) ([]RunDetail, error)
	UpdateRunStatus(ctx context.Context, runID string, expectedVersion int, nextStatus schema.RunStatus) (RunDetail, error)
	AddRunStep(ctx context.Context, runID string, step RunStepInput) (RunStepItem, error)
	UpdateRunStep(ctx context.Context, stepID string, expectedVersion int, status schema.StepStatus, resultCard string) (RunStepItem, error)
	RecordRunEvent(ctx context.Context, runID string, stepID string, eventType string, payload string) (RunEventItem, error)
}

type service struct {
	repo Repo
}

// NewService 创建运行领域服务。
func NewService(repo Repo) Service {
	return &service{repo: repo}
}

func (s *service) CreateRun(ctx context.Context, memberID string, goal string, intentKind schema.IntentKind, requestID string, conversationID string, requestedEffects []string) (RunDetail, error) {
	return s.repo.CreateRun(ctx, memberID, goal, intentKind, requestID, conversationID, requestedEffects)
}

func (s *service) GetRun(ctx context.Context, memberID string, runID string) (RunDetail, error) {
	return s.repo.GetRun(ctx, memberID, runID)
}

func (s *service) ListRuns(ctx context.Context, memberID string, status string, limit int) ([]RunItem, error) {
	return s.repo.ListRuns(ctx, memberID, status, limit)
}

func (s *service) UpdateRunStatus(ctx context.Context, runID string, expectedVersion int, nextStatus schema.RunStatus) (RunDetail, error) {
	return s.repo.UpdateRunStatus(ctx, runID, expectedVersion, nextStatus)
}

func (s *service) AddRunStep(ctx context.Context, runID string, step RunStepInput) (RunStepItem, error) {
	return s.repo.AddRunStep(ctx, runID, step)
}

func (s *service) ConfirmRunStep(ctx context.Context, memberID string, runID string, stepID string, choice string, nonce string) (RunDetail, error) {
	_, err := s.repo.GetRun(ctx, memberID, runID)
	if err != nil {
		return RunDetail{}, err
	}

	payload, _ := json.Marshal(map[string]any{"choice": choice, "nonce": nonce})
	_, _ = s.repo.RecordRunEvent(ctx, runID, stepID, "step_confirmed", string(payload))
	_, _ = s.repo.UpdateRunStep(ctx, stepID, 0, schema.StepStatusCommitted, fmt.Sprintf(`{"choice":%q}`, choice))

	return s.repo.GetRun(ctx, memberID, runID)
}

func (s *service) ReplyRun(ctx context.Context, memberID string, runID string, stepID string, expectedVersion int, answers map[string]any) (RunDetail, error) {
	_, err := s.repo.GetRun(ctx, memberID, runID)
	if err != nil {
		return RunDetail{}, err
	}

	payload, _ := json.Marshal(answers)
	_, _ = s.repo.RecordRunEvent(ctx, runID, stepID, "user_replied", string(payload))
	_, _ = s.repo.UpdateRunStatus(ctx, runID, expectedVersion, schema.RunStatusRunning)

	return s.repo.GetRun(ctx, memberID, runID)
}

func (s *service) AmendRun(ctx context.Context, memberID string, runID string, expectedVersion int, modifications []map[string]any) (RunDetail, error) {
	r, err := s.repo.GetRun(ctx, memberID, runID)
	if err != nil {
		return RunDetail{}, err
	}

	payload, _ := json.Marshal(modifications)
	_, _ = s.repo.RecordRunEvent(ctx, runID, "", "plan_amended", string(payload))
	_, _ = s.repo.UpdateRunStatus(ctx, runID, expectedVersion, schema.RunStatus(r.Status))

	return s.repo.GetRun(ctx, memberID, runID)
}

func (s *service) RecordRunEvent(ctx context.Context, runID string, stepID string, eventType string, payload string) (RunEventItem, error) {
	return s.repo.RecordRunEvent(ctx, runID, stepID, eventType, payload)
}

func (s *service) GetRunByID(ctx context.Context, runID string) (RunDetail, error) {
	return s.repo.GetRunByID(ctx, runID)
}

func (s *service) FindInterruptedRuns(ctx context.Context, updatedBefore time.Time, limit int) ([]RunDetail, error) {
	return s.repo.FindInterruptedRuns(ctx, updatedBefore, limit)
}

func (s *service) UpdateRunStep(ctx context.Context, stepID string, expectedVersion int, status schema.StepStatus, resultCard string) (RunStepItem, error) {
	return s.repo.UpdateRunStep(ctx, stepID, expectedVersion, status, resultCard)
}

func (s *service) CancelRun(ctx context.Context, memberID string, runID string, reason string) (RunDetail, error) {
	_, err := s.repo.GetRun(ctx, memberID, runID)
	if err != nil {
		return RunDetail{}, err
	}

	payload, _ := json.Marshal(map[string]any{"reason": reason})
	_, _ = s.repo.RecordRunEvent(ctx, runID, "", "run_cancelled", string(payload))
	_, _ = s.repo.UpdateRunStatus(ctx, runID, 0, schema.RunStatusCancelled)

	// 级联取消所有活跃子智能体运行（Feature 4）
	if children, err := s.repo.FindChildRuns(ctx, runID); err == nil {
		for _, child := range children {
			if child.Status == string(schema.RunStatusRunning) || child.Status == string(schema.RunStatusAwaitingInput) {
				_, _ = s.CancelRun(ctx, child.MemberID, child.ID, "parent_run_cancelled: "+runID)
			}
		}
	}

	return s.repo.GetRun(ctx, memberID, runID)
}

func (s *service) SteerRun(ctx context.Context, memberID string, runID string, instruction string) (RunDetail, error) {
	_, err := s.repo.GetRun(ctx, memberID, runID)
	if err != nil {
		return RunDetail{}, err
	}

	payload, _ := json.Marshal(map[string]any{"instruction": instruction})
	_, _ = s.repo.RecordRunEvent(ctx, runID, "", "human_steered", string(payload))
	return s.repo.GetRun(ctx, memberID, runID)
}

func (s *service) CreateChildRun(ctx context.Context, memberID string, parentRunID string, goal string, intentKind schema.IntentKind) (RunDetail, error) {
	parent, err := s.repo.GetRun(ctx, memberID, parentRunID)
	if err != nil {
		return RunDetail{}, err
	}

	child, err := s.repo.CreateChildRun(ctx, memberID, parentRunID, goal, intentKind)
	if err != nil {
		return RunDetail{}, err
	}

	payload, _ := json.Marshal(map[string]any{
		"child_run_id": child.ID,
		"goal":         goal,
	})
	_, _ = s.repo.RecordRunEvent(ctx, parent.ID, "", "child_run_spawned", string(payload))
	return child, nil
}


