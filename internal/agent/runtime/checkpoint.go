package runtime

import (
	"context"
	"encoding/json"
	"log/slog"

	domrun "github.com/mk20mm/homeagent/internal/domain/run"
)

// 检查点事件类型常量（对齐 WBS T-DRC-04 及 Pi Agent 扩展）。
const (
	EventLLMRequestSent      = "llm_request_sent"
	EventLLMResponseReceived = "llm_response_received"
	EventToolIntent          = "tool_intent"
	EventToolResult          = "tool_result"
	EventTurnCompleted       = "turn_completed"
	EventHumanSteered        = "human_steered"
	EventRunCancelled        = "run_cancelled"
	EventRunPaused           = "run_paused"
	EventChildRunSpawned     = "child_run_spawned"
	EventChildRunCompleted   = "child_run_completed"
	EventChildRunFailed      = "child_run_failed"
)

// checkpoint 在调度循环关键节点写入运行检查点（T-DRC-05），并通过 Hub 实时广播给 Live Watchers（T-PI-07）。
// 若 runSvc 为空或 runID 为空，直接静默跳过。
// 即使写入失败，也仅通过 slog.Error 告警，不抛出不可恢复异常中断业务（不变量 1）。
func checkpoint(ctx context.Context, runSvc domrun.Service, hub *Hub, runID string, stepID string, eventType string, payload any) error {
	if runSvc == nil || runID == "" {
		return nil
	}

	var payloadStr string
	switch p := payload.(type) {
	case string:
		payloadStr = p
	default:
		b, err := json.Marshal(p)
		if err != nil {
			slog.Error("marshal checkpoint payload failed", "run_id", runID, "event_type", eventType, "err", err)
			payloadStr = "{}"
		} else {
			payloadStr = string(b)
		}
	}

	item, err := runSvc.RecordRunEvent(ctx, runID, stepID, eventType, payloadStr)
	if err != nil {
		slog.Error("record checkpoint failed", "run_id", runID, "step_id", stepID, "event_type", eventType, "err", err)
		return err
	}
	if hub != nil {
		hub.Broadcast(runID, item)
	}
	return nil
}
