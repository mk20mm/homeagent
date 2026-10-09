package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/session"
	domrun "github.com/mk20mm/homeagent/internal/domain/run"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

type llmResponsePayload struct {
	Turn      int                `json:"turn"`
	Content   string             `json:"content"`
	ToolCalls []gateway.ToolCall `json:"tool_calls"`
}

type toolResultPayload struct {
	Tool       string `json:"tool"`
	ToolCallID string `json:"tool_call_id"`
	Summary    string `json:"summary"`
	UndoID     string `json:"undo_id"`
	Card       string `json:"card"`
	Error      string `json:"error"`
}

// RecoverInterruptedRuns 扫描并恢复异常中断的 AgentRun（T-DRC-11）。
// 仅扫描更新时间早于 now - ageThreshold（默认 60s）的运行，防止抢占正常并发。
func (rt *Runtime) RecoverInterruptedRuns(ctx context.Context, ageThreshold time.Duration) (int, error) {
	if rt.runSvc == nil {
		return 0, nil
	}
	if ageThreshold <= 0 {
		ageThreshold = 60 * time.Second
	}

	cutoff := rt.now().Add(-ageThreshold)
	runs, err := rt.runSvc.FindInterruptedRuns(ctx, cutoff, 50)
	if err != nil {
		slog.Error("find interrupted runs failed", "err", err)
		return 0, err
	}

	recoveredCount := 0
	for _, r := range runs {
		if err := rt.recoverSingleRun(ctx, r); err != nil {
			slog.Error("recover run failed", "run_id", r.ID, "err", err)
		} else {
			recoveredCount++
		}
	}

	return recoveredCount, nil
}

func (rt *Runtime) recoverSingleRun(ctx context.Context, run domrun.RunDetail) error {
	slog.Info("recovering interrupted run", "run_id", run.ID, "goal", run.Goal)

	memberID := run.MemberID
	convID := run.ConversationID
	if memberID == "" {
		slog.Warn("cannot recover run without member_id", "run_id", run.ID)
		_, _ = rt.runSvc.UpdateRunStatus(ctx, run.ID, 0, schema.RunStatusFailed)
		return nil
	}

	rebuiltMsgs, nextTurn, err := rt.rebuildContextFromRun(ctx, run)
	if err != nil {
		slog.Error("rebuild context failed", "run_id", run.ID, "err", err)
		_, _ = rt.runSvc.UpdateRunStatus(ctx, run.ID, 0, schema.RunStatusFailed)
		return err
	}

	// 若 nextTurn <= 0 表示在崩溃前已完成或无需继续执行
	if nextTurn <= 0 {
		_, _ = rt.runSvc.UpdateRunStatus(ctx, run.ID, 0, schema.RunStatusCompleted)
		if convID != "" && len(rebuiltMsgs) > 0 {
			_ = rt.sessions.Append(ctx, convID, memberID, rebuiltMsgs)
		}
		return nil
	}

	return rt.RunFromCheckpoint(ctx, RunFromCheckpointRequest{
		RunID:           run.ID,
		ConversationID:  convID,
		MemberID:        memberID,
		StartTurn:       nextTurn,
		RebuiltMessages: rebuiltMsgs,
	})
}

// rebuildContextFromRun 从 RunEvent 检查点序列重建上下文与判定断点（T-DRC-12, T-DRC-15）。
func (rt *Runtime) rebuildContextFromRun(ctx context.Context, run domrun.RunDetail) ([]gateway.Message, int, error) {
	perms, err := rt.sessions.Permissions(ctx, run.MemberID)
	if err != nil {
		return nil, 0, err
	}

	userMsg := gateway.Message{Role: gateway.RoleUser, Content: run.Goal}
	rebuilt := []gateway.Message{userMsg}

	// 收集所有 tool_result 事件
	toolResults := make(map[string]toolResultPayload)
	var responseEvents []llmResponsePayload
	hasTurnCompleted := false

	for _, e := range run.Events {
		switch e.EventType {
		case EventLLMResponseReceived:
			var resp llmResponsePayload
			if err := json.Unmarshal([]byte(e.Payload), &resp); err == nil {
				responseEvents = append(responseEvents, resp)
			}
		case EventToolResult:
			var res toolResultPayload
			if err := json.Unmarshal([]byte(e.Payload), &res); err == nil {
				key := res.ToolCallID
				if key == "" {
					key = res.Tool
				}
				toolResults[key] = res
			}
		case EventTurnCompleted:
			hasTurnCompleted = true
		}
	}

	// 若没有收到过任何 LLM 响应，从第 1 轮开始
	if len(responseEvents) == 0 {
		return rebuilt, 1, nil
	}

	lastTurn := 1
	// 逐轮重建
	for i, resp := range responseEvents {
		lastTurn = resp.Turn
		isLastResponse := (i == len(responseEvents)-1)

		asstMsg := session.Message{
			Role:      gateway.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		rebuilt = append(rebuilt, asstMsg)

		if len(resp.ToolCalls) == 0 {
			if isLastResponse {
				// 最后一轮无工具调用且已收到回复，说明实际已完成
				return rebuilt, -1, nil
			}
			continue
		}

		for _, tc := range resp.ToolCalls {
			key := tc.ID
			if key == "" {
				key = tc.Name
			}

			if res, ok := toolResults[key]; ok {
				// 已完成的工具结果
				summary := res.Summary
				if summary == "" && res.Error != "" {
					summary = "执行失败: " + res.Error
				}
				rebuilt = append(rebuilt, session.Message{
					Role:       gateway.RoleTool,
					Name:       tc.Name,
					ToolCallID: tc.ID,
					Content:    summary,
				})
			} else {
				// 未完成的工具（崩溃发生在执行中或之前）
				isReplayable := rt.executor.IsReplayable(tc.Name)
				if isReplayable {
					// 场景 A：Safe 工具崩溃后自动重放（T-DRC-12）
					slog.Info("replaying safe tool", "tool", tc.Name, "run_id", run.ID)
					execRes, execErr := rt.executor.Execute(ctx, tc.Name, tc.Args, run.MemberID, perms)
					if execErr != nil {
						errMsg := execErr.Error()
						_ = checkpoint(ctx, rt.runSvc, rt.hub, run.ID, tc.ID, EventToolResult, map[string]any{
							"tool":         tc.Name,
							"tool_call_id": tc.ID,
							"error":        errMsg,
						})
						rebuilt = append(rebuilt, session.Message{
							Role:       gateway.RoleTool,
							Name:       tc.Name,
							ToolCallID: tc.ID,
							Content:    "执行失败: " + errMsg,
						})
					} else {
						_ = checkpoint(ctx, rt.runSvc, rt.hub, run.ID, tc.ID, EventToolResult, map[string]any{
							"tool":         tc.Name,
							"tool_call_id": tc.ID,
							"summary":      execRes.Summary,
							"undo_id":      execRes.UndoID,
							"card":         string(execRes.Card),
						})
						rebuilt = append(rebuilt, session.Message{
							Role:       gateway.RoleTool,
							Name:       tc.Name,
							ToolCallID: tc.ID,
							Content:    execRes.Summary,
						})
					}
				} else {
					// 场景 B：Unsafe 工具崩溃报告中断（T-DRC-15）
					slog.Warn("unsafe tool interrupted, reporting to LLM", "tool", tc.Name, "run_id", run.ID)
					// 将步骤标为 unknown
					for _, st := range run.Steps {
						if st.ToolName == tc.Name || st.ID == tc.ID {
							_, _ = rt.runSvc.UpdateRunStep(ctx, st.ID, st.ExpectedVersion, schema.StepStatusUnknown, "")
						}
					}
					notice := fmt.Sprintf("上一次执行在工具 %s 执行中被中断，无法确认执行结果，请决定下一步", tc.Name)
					_ = checkpoint(ctx, rt.runSvc, rt.hub, run.ID, tc.ID, EventToolResult, map[string]any{
						"tool":         tc.Name,
						"tool_call_id": tc.ID,
						"error":        "interrupted_unsafe",
						"notice":       notice,
					})
					rebuilt = append(rebuilt, session.Message{
						Role:       gateway.RoleTool,
						Name:       tc.Name,
						ToolCallID: tc.ID,
						Content:    notice,
					})
				}
			}
		}
	}

	if hasTurnCompleted {
		return rebuilt, -1, nil
	}

	return rebuilt, lastTurn + 1, nil
}
