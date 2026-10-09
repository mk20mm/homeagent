// Package runtime 是 Agent 调度器（ARCHITECTURE §2.2 组件④）。
//
// ReAct 循环：LLM 输出含 tool_call → 执行 → 结果回传 → 继续生成，直到无调用。
// 运行时约束（AI-STD-003）：步数上限（危险分级）、取消（ctx）、用量埋点、trace。
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/prompt"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	domrun "github.com/mk20mm/homeagent/internal/domain/run"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

// Event 是推给 SSE handler 的事件（对齐前端 useSSE：token/tool_call/done/error/run_started）。
type Event struct {
	Type    string          `json:"type"` // token | tool_call | done | error | run_started
	RunID   string          `json:"run_id,omitempty"` // 真实执行运行唯一标识 (AgentRun.ID)
	Content string          `json:"content,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	Card    json.RawMessage `json:"card,omitempty"`
	UndoID  string          `json:"undo_id,omitempty"` // 撤销记录 id（tool_call 携带，前端撤销按钮回指）
	Error   string          `json:"error,omitempty"`
}

// RunRequest 一次对话请求。
type RunRequest struct {
	ConversationID string
	MemberID       string
	ModelID        string // 会话内指定的模型 id（可选）
	Content        string
	TraceID        string
	RequestID      string // 客户端幂等与事务运行追踪 ID
	OnEvent        func(Event) // 流式回调，禁止阻塞（handler 直接转发 SSE）
}

// RunFromCheckpointRequest 从检查点恢复执行请求（T-DRC-13）。
type RunFromCheckpointRequest struct {
	RunID           string
	ConversationID  string
	MemberID        string
	ModelID         string
	StartTurn       int
	RebuiltMessages []gateway.Message
	OnEvent         func(Event)
}

// Options Runtime 依赖，由 main 装配。
type Options struct {
	Provider   gateway.Provider
	Executor   *tool.Executor
	Sessions   session.Service
	Usage      gateway.UsageRecorder
	RunService domrun.Service
}

// Runtime 持有全部依赖。
type Runtime struct {
	provider gateway.Provider
	executor *tool.Executor
	sessions session.Service
	usage    gateway.UsageRecorder
	runSvc   domrun.Service
	hub      *Hub
	now      func() time.Time
}

func New(o Options) *Runtime {
	rt := &Runtime{
		provider: o.Provider,
		executor: o.Executor,
		sessions: o.Sessions,
		usage:    o.Usage,
		runSvc:   o.RunService,
		hub:      NewHub(),
		now:      time.Now,
	}
	if rt.usage == nil {
		rt.usage = gateway.NoopUsageRecorder{}
	}
	return rt
}

// Hub 返回内部并发事件控制中心。
func (rt *Runtime) Hub() *Hub {
	return rt.hub
}

// Steer 向活跃中的 Run 发送实时人类干预指令。
func (rt *Runtime) Steer(runID string, instruction string) bool {
	if rt.hub == nil {
		return false
	}
	return rt.hub.Steer(runID, instruction)
}

// Cancel 紧急中断正在活跃执行中的 Run。
func (rt *Runtime) Cancel(runID string) bool {
	if rt.hub == nil {
		return false
	}
	return rt.hub.Cancel(runID)
}

// Subscribe 订阅指定 Run 的事件广播。
func (rt *Runtime) Subscribe(runID string) (<-chan domrun.RunEventItem, func()) {
	if rt.hub == nil {
		ch := make(chan domrun.RunEventItem)
		close(ch)
		return ch, func() {}
	}
	return rt.hub.Subscribe(runID)
}

type loopInput struct {
	RunID          string
	MemberID       string
	ConversationID string
	ModelID        string
	TraceID        string
	History        []session.Message
	InitialMsgs    []session.Message
	StartTurn      int
	Perms          map[string]bool
	OnEvent        func(Event)
}

// Run 执行一次完整 ReAct 循环。业务错误通过 Event(error) 推出，不中断持久化。
func (rt *Runtime) Run(ctx context.Context, req RunRequest) error {
	fail := func(msg string, err error) error {
		if req.OnEvent != nil {
			req.OnEvent(Event{Type: "error", Error: msg})
		}
		if err != nil {
			return apperr.New(apperr.CodeInternal, msg, err)
		}
		return apperr.New(apperr.CodeInternal, msg, nil)
	}

	if req.Content == "" {
		return fail("消息内容为空", nil)
	}

	conv, history, err := rt.sessions.Ensure(ctx, req.ConversationID, req.MemberID)
	if err != nil {
		return fail("会话初始化失败", err)
	}

	// 历史消息超出阈值时自动触发 Compaction 摘要压缩（T-DRC-19）
	effectiveHistory := session.EffectiveHistory(history)
	if len(effectiveHistory) > session.CompactThreshold {
		if compacted, err := session.Compact(ctx, rt.provider, conv.ModelID, effectiveHistory); err == nil && len(compacted) > 0 {
			history = compacted
			// 压缩结果持久化回 conversation（不变量 4：原始消息永久保留，仅持久化摘要供后续会话使用）
			if len(compacted) > 0 && strings.HasPrefix(compacted[0].Content, session.SummaryPrefix) {
				_ = rt.sessions.Append(ctx, conv.ID, req.MemberID, []session.Message{compacted[0]})
			}
		} else {
			history = effectiveHistory
		}
	} else {
		history = effectiveHistory
	}

	var currentRunID string
	// 幂等防重与创建 AgentRun（T-DRC-06）
	if rt.runSvc != nil {
		runDetail, err := rt.runSvc.CreateRun(ctx, req.MemberID, req.Content, schema.IntentAct, req.RequestID, conv.ID, nil)
		if err != nil {
			slog.Error("create agent run failed", "err", err, "request_id", req.RequestID)
		} else {
			currentRunID = runDetail.ID
			if runDetail.Status == string(schema.RunStatusCompleted) {
				if req.OnEvent != nil {
					req.OnEvent(Event{Type: "done", RunID: currentRunID})
				}
				return nil
			}
		}
	}

	if currentRunID != "" && req.OnEvent != nil {
		req.OnEvent(Event{Type: "run_started", RunID: currentRunID})
	}

	// 指定了模型且与会话当前不同：更新会话绑定
	if req.ModelID != "" && req.ModelID != conv.ModelID {
		_ = rt.sessions.SetModel(ctx, conv.ID, req.MemberID, req.ModelID)
		conv.ModelID = req.ModelID
	}

	perms, err := rt.sessions.Permissions(ctx, req.MemberID)
	if err != nil {
		return fail("权限加载失败", err)
	}

	return rt.executeLoop(ctx, loopInput{
		RunID:          currentRunID,
		MemberID:       req.MemberID,
		ConversationID: conv.ID,
		ModelID:        conv.ModelID,
		TraceID:        req.TraceID,
		History:        history,
		InitialMsgs:    []session.Message{{Role: gateway.RoleUser, Content: req.Content}},
		StartTurn:      1,
		Perms:          perms,
		OnEvent:        req.OnEvent,
	})
}

// RunFromCheckpoint 从已有检查点继续执行 ReAct 循环（T-DRC-13）。
func (rt *Runtime) RunFromCheckpoint(ctx context.Context, req RunFromCheckpointRequest) error {
	conv, history, err := rt.sessions.Ensure(ctx, req.ConversationID, req.MemberID)
	if err != nil {
		return err
	}
	history = session.EffectiveHistory(history)
	modelID := conv.ModelID
	if req.ModelID != "" {
		modelID = req.ModelID
	}
	perms, err := rt.sessions.Permissions(ctx, req.MemberID)
	if err != nil {
		return err
	}
	startTurn := req.StartTurn
	if startTurn <= 0 {
		startTurn = 1
	}

	return rt.executeLoop(ctx, loopInput{
		RunID:          req.RunID,
		MemberID:       req.MemberID,
		ConversationID: conv.ID,
		ModelID:        modelID,
		History:        history,
		InitialMsgs:    req.RebuiltMessages,
		StartTurn:      startTurn,
		Perms:          perms,
		OnEvent:        req.OnEvent,
	})
}

func (rt *Runtime) executeLoop(ctx context.Context, in loopInput) error {
	emit := func(e Event) {
		if in.OnEvent != nil {
			in.OnEvent(e)
		}
	}
	var isFailed bool

	updateStatus := func(status schema.RunStatus) {
		if rt.runSvc != nil && in.RunID != "" {
			if _, err := rt.runSvc.UpdateRunStatus(ctx, in.RunID, 0, status); err != nil {
				slog.Error("update run status failed", "run_id", in.RunID, "status", status, "err", err)
			}
			if rt.hub != nil {
				rt.hub.Broadcast(in.RunID, domrun.RunEventItem{
					EventType: "run_status_changed",
					Payload:   fmt.Sprintf(`{"status":%q}`, status),
				})
			}
		}
	}

	fail := func(msg string, err error) error {
		isFailed = true
		updateStatus(schema.RunStatusFailed)
		emit(Event{Type: "error", Error: msg})
		if err != nil {
			return apperr.New(apperr.CodeInternal, msg, err)
		}
		return apperr.New(apperr.CodeInternal, msg, nil)
	}

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	steerChan := make(chan string, 16)
	if rt.hub != nil && in.RunID != "" {
		rt.hub.Register(in.RunID, runCancel, steerChan)
		defer rt.hub.Unregister(in.RunID)
	}

	specs := rt.executor.Registry().SpecsWithPermission(in.Perms)
	toolDefs := gateway.ToolsFromSpecs(specs)
	toolByName := make(map[string]tool.Spec, len(specs))
	for _, s := range specs {
		toolByName[s.Name] = s
	}

	if in.TraceID != "" {
		ctx = tool.WithTraceID(ctx, in.TraceID)
	}
	ctx = tool.WithConversationID(ctx, in.ConversationID)
	ctx = tool.WithMemberID(ctx, in.MemberID)

	persist := make([]session.Message, len(in.InitialMsgs))
	copy(persist, in.InitialMsgs)

	system := prompt.Build(prompt.Options{
		MemberName: "",
		Role:       schema.RoleAdult,
		Now:        rt.now(),
	})
	reqMsgs := make([]gateway.Message, 0, 1+len(in.History)+len(in.InitialMsgs)+8)
	reqMsgs = append(reqMsgs, gateway.Message{Role: gateway.RoleSystem, Content: system})
	reqMsgs = append(reqMsgs, in.History...)
	reqMsgs = append(reqMsgs, in.InitialMsgs...)

	curRisk := tool.RiskLow
	maxTurns := tool.MaxTurns[tool.RiskLow]
	highRiskExecuted := 0

	stopped := false
	for turn := in.StartTurn; !stopped; turn++ {
		// 检查人类紧急取消（T-PI-03）
		if runCtx.Err() != nil {
			isFailed = true
			updateStatus(schema.RunStatusCancelled)
			_ = checkpoint(ctx, rt.runSvc, rt.hub, in.RunID, "", EventRunCancelled, map[string]any{
				"turn":   turn,
				"reason": "cancelled_by_user",
			})
			emit(Event{Type: "error", Error: "任务已由用户手动取消"})
			break
		}

		// 检查人类干预指令（T-PI-03 Steering）
		select {
		case instruction := <-steerChan:
			_ = checkpoint(ctx, rt.runSvc, rt.hub, in.RunID, "", EventHumanSteered, map[string]any{
				"turn":        turn,
				"instruction": instruction,
			})
			steerMsg := session.Message{
				Role:    gateway.RoleUser,
				Content: "[用户实时干预指令] " + instruction,
			}
			persist = append(persist, steerMsg)
			reqMsgs = append(reqMsgs, steerMsg)
			emit(Event{Type: "token", Content: "\n[已接收实时干预: " + instruction + "]\n"})
		default:
		}

		if turn > maxTurns {
			isFailed = true
			updateStatus(schema.RunStatusFailed)
			emit(Event{Type: "error", Error: "已达本次操作步数上限，已停止以防失控"})
			break
		}

		_ = checkpoint(ctx, rt.runSvc, rt.hub, in.RunID, "", EventLLMRequestSent, map[string]any{
			"turn":           turn,
			"messages_count": len(reqMsgs),
		})

		chatReq := gateway.ChatRequest{
			Model:    in.ModelID,
			Messages: reqMsgs,
			Tools:    toolDefs,
		}
		start := rt.now()
		ch, err := rt.provider.StreamChat(runCtx, chatReq)
		if err != nil {
			return fail("LLM 网关连接失败", err)
		}

		var text strings.Builder
		var toolCalls []gateway.ToolCall
		var usage *gateway.Usage
		var streamErr error
		for e := range ch {
			if e.Err != nil {
				streamErr = e.Err
				break
			}
			if e.Delta != "" {
				text.WriteString(e.Delta)
				emit(Event{Type: "token", Content: e.Delta})
			}
			if e.ToolCall != nil {
				toolCalls = append(toolCalls, *e.ToolCall)
			}
			if e.Done && e.Usage != nil {
				usage = e.Usage
			}
		}
		if streamErr != nil {
			return fail("大模型响应异常: "+streamErr.Error(), streamErr)
		}
		latency := rt.now().Sub(start).Milliseconds()

		if usage != nil {
			_ = rt.usage.Record(ctx, in.ModelID, chatReq.Model, rt.provider.Name(), *usage, latency)
		}

		if text.Len() == 0 && len(toolCalls) == 0 {
			fallbackMsg := "未能生成有效回复，请重试或更换模型。"
			text.WriteString(fallbackMsg)
			emit(Event{Type: "token", Content: fallbackMsg})
		}

		_ = checkpoint(ctx, rt.runSvc, rt.hub, in.RunID, "", EventLLMResponseReceived, map[string]any{
			"turn":       turn,
			"content":    text.String(),
			"tool_calls": toolCalls,
		})

		assistant := session.Message{
			Role:      gateway.RoleAssistant,
			Content:   text.String(),
			ToolCalls: toolCalls,
		}
		persist = append(persist, assistant)
		reqMsgs = append(reqMsgs, assistant)

		if len(toolCalls) == 0 {
			_ = checkpoint(ctx, rt.runSvc, rt.hub, in.RunID, "", EventTurnCompleted, map[string]any{
				"turn": turn,
			})
			break
		}

		stepIdx := 0
		for _, tc := range toolCalls {
			stepIdx++
			var stepItem domrun.RunStepItem
			if rt.runSvc != nil && in.RunID != "" {
				stepItem, _ = rt.runSvc.AddRunStep(ctx, in.RunID, domrun.RunStepInput{
					Position:        turn*10 + stepIdx,
					ToolName:        tc.Name,
					InputJSON:       string(tc.Args),
					WaitFor:         schema.WaitForNone,
					CompletionKind:  "auto",
					ExpectedVersion: 0,
				})
			}

			_ = checkpoint(ctx, rt.runSvc, rt.hub, in.RunID, tc.ID, EventToolIntent, map[string]any{
				"tool":         tc.Name,
				"args":         string(tc.Args),
				"tool_call_id": tc.ID,
				"step_id":      stepItem.ID,
			})

			res, err := rt.executor.Execute(runCtx, tc.Name, tc.Args, in.MemberID, in.Perms)
			if err != nil {
				errMsg := err.Error()
				if rt.runSvc != nil && in.RunID != "" && stepItem.ID != "" {
					_, _ = rt.runSvc.UpdateRunStep(ctx, stepItem.ID, 0, schema.StepStatusFailed, errMsg)
				}
				_ = checkpoint(ctx, rt.runSvc, rt.hub, in.RunID, tc.ID, EventToolResult, map[string]any{
					"tool":         tc.Name,
					"tool_call_id": tc.ID,
					"error":        errMsg,
					"step_id":      stepItem.ID,
				})
				emit(Event{Type: "tool_call", Tool: tc.Name, Error: errMsg, RunID: in.RunID})
				m := session.Message{
					Role:       gateway.RoleTool,
					Content:    "执行失败: " + errMsg,
					Name:       tc.Name,
					ToolCallID: tc.ID,
				}
				reqMsgs = append(reqMsgs, m)
				persist = append(persist, m)
				continue
			}

			card := res.Card
			if len(card) == 0 {
				card = []byte("{}")
			}
			if rt.runSvc != nil && in.RunID != "" && stepItem.ID != "" {
				_, _ = rt.runSvc.UpdateRunStep(ctx, stepItem.ID, 0, schema.StepStatusCommitted, string(card))
			}
			_ = checkpoint(ctx, rt.runSvc, rt.hub, in.RunID, tc.ID, EventToolResult, map[string]any{
				"tool":         tc.Name,
				"tool_call_id": tc.ID,
				"summary":      res.Summary,
				"undo_id":      res.UndoID,
				"card":         string(card),
				"step_id":      stepItem.ID,
			})
			emit(Event{Type: "tool_call", Tool: tc.Name, Card: card, UndoID: res.UndoID, RunID: in.RunID})

			m := session.Message{
				Role:       gateway.RoleTool,
				Content:    res.Summary,
				Name:       tc.Name,
				ToolCallID: tc.ID,
			}
			reqMsgs = append(reqMsgs, m)
			persist = append(persist, m)

			if spec, ok := toolByName[tc.Name]; ok {
				if spec.Risk > curRisk {
					curRisk = spec.Risk
					maxTurns = tool.MaxTurns[spec.Risk]
				}
				if spec.Risk == tool.RiskHigh {
					highRiskExecuted++
					if highRiskExecuted >= 2 {
						isFailed = true
						updateStatus(schema.RunStatusFailed)
						emit(Event{
							Type:  "error",
							Error: "涉及金额操作较多，建议人工确认，已停止本次操作",
							RunID: in.RunID,
						})
						stopped = true
					}
				}
			}
		}
	}

	if !isFailed {
		updateStatus(schema.RunStatusCompleted)
	}

	if err := rt.sessions.Append(ctx, in.ConversationID, in.MemberID, persist); err != nil {
		slog.Error("persist messages failed", "err", err, "conv", in.ConversationID)
	}
	emit(Event{Type: "done", RunID: in.RunID})
	return nil
}
