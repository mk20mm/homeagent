package run

import (
	"context"
	"time"

	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

// RunStepInput 创建步骤入参。
type RunStepInput struct {
	Position          int
	ToolName          string
	InputJSON         string
	OperationKey      string
	DependsOnStepIDs  []string
	WaitFor           schema.StepWaitFor
	CompletionKind    string
	ExpectedVersion   int
	ConfirmationNonce string
}

// RunItem 简要信息。
type RunItem struct {
	ID         string    `json:"id"`
	Goal       string    `json:"goal"`
	IntentKind string    `json:"intent_kind"`
	Status     string    `json:"status"`
	Version    int       `json:"version"`
	RequestID  string    `json:"request_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// RunStepItem 步骤详情。
type RunStepItem struct {
	ID              string `json:"id"`
	Position        int    `json:"position"`
	ToolName        string `json:"tool_name"`
	Status          string `json:"status"`
	WaitFor         string `json:"wait_for"`
	CompletionKind  string `json:"completion_kind"`
	OperationKey    string `json:"operation_key,omitempty"`
	ResultCard      string `json:"result_card,omitempty"`
	ExpectedVersion int    `json:"expected_version"`
}

// RunEventItem 事件详情。
type RunEventItem struct {
	ID        string    `json:"id"`
	Seq       int       `json:"seq"`
	StepID    string    `json:"step_id,omitempty"`
	EventType string    `json:"event_type"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// RunDetail 编排完整详情。
type RunDetail struct {
	ID         string         `json:"id"`
	Goal       string         `json:"goal"`
	IntentKind string         `json:"intent_kind"`
	Status     string         `json:"status"`
	Version    int            `json:"version"`
	RequestID  string         `json:"request_id,omitempty"`
	Steps      []RunStepItem  `json:"steps"`
	Events     []RunEventItem `json:"events"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Service 调度中心运行领域服务接口。
type Service interface {
	CreateRun(ctx context.Context, memberID string, goal string, intentKind schema.IntentKind, requestID string, conversationID string, requestedEffects []string) (RunDetail, error)
	GetRun(ctx context.Context, memberID string, runID string) (RunDetail, error)
	ListRuns(ctx context.Context, memberID string, status string, limit int) ([]RunItem, error)
	UpdateRunStatus(ctx context.Context, runID string, expectedVersion int, nextStatus schema.RunStatus) (RunDetail, error)
	AddRunStep(ctx context.Context, runID string, step RunStepInput) (RunStepItem, error)
	ConfirmRunStep(ctx context.Context, memberID string, runID string, stepID string, choice string, nonce string) (RunDetail, error)
	ReplyRun(ctx context.Context, memberID string, runID string, stepID string, expectedVersion int, answers map[string]any) (RunDetail, error)
	AmendRun(ctx context.Context, memberID string, runID string, expectedVersion int, modifications []map[string]any) (RunDetail, error)
}
