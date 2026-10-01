// Package runtime 是 Agent 调度器（ARCHITECTURE §2.2 组件④）。
//
// ReAct 循环：LLM 输出含 tool_call → 执行 → 结果回传 → 继续生成，直到无调用。
// 运行时约束（AI-STD-003）：步数上限（危险分级）、取消（ctx）、用量埋点、trace。
package runtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/prompt"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

// Event 是推给 SSE handler 的事件（对齐前端 useSSE：token/tool_call/done/error）。
type Event struct {
	Type    string          `json:"type"` // token | tool_call | done | error
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

// Options Runtime 依赖，由 main 装配。
type Options struct {
	Provider gateway.Provider
	Executor *tool.Executor
	Sessions session.Service
	Usage    gateway.UsageRecorder
}

// Runtime 持有全部依赖。
type Runtime struct {
	provider gateway.Provider
	executor *tool.Executor
	sessions session.Service
	usage    gateway.UsageRecorder
	now      func() time.Time
}

func New(o Options) *Runtime {
	rt := &Runtime{
		provider: o.Provider,
		executor: o.Executor,
		sessions: o.Sessions,
		usage:    o.Usage,
		now:      time.Now,
	}
	if rt.usage == nil {
		rt.usage = gateway.NoopUsageRecorder{}
	}
	return rt
}

// Run 执行一次完整 ReAct 循环。业务错误通过 Event(error) 推出，不中断持久化。
func (rt *Runtime) Run(ctx context.Context, req RunRequest) error {
	emit := func(e Event) {
		if req.OnEvent != nil {
			req.OnEvent(e)
		}
	}
	fail := func(msg string, err error) error {
		emit(Event{Type: "error", Error: msg})
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

	// 指定了模型且与会话当前不同：更新会话绑定
	if req.ModelID != "" && req.ModelID != conv.ModelID {
		_ = rt.sessions.SetModel(ctx, conv.ID, req.MemberID, req.ModelID)
		conv.ModelID = req.ModelID
	}

	perms, err := rt.sessions.Permissions(ctx, req.MemberID)
	if err != nil {
		return fail("权限加载失败", err)
	}

	// 权限源头过滤后的工具清单（ADR-005）
	specs := rt.executor.Registry().SpecsWithPermission(perms)
	toolDefs := gateway.ToolsFromSpecs(specs)
	toolByName := make(map[string]tool.Spec, len(specs))
	for _, s := range specs {
		toolByName[s.Name] = s
	}

	ctx = tool.WithTraceID(ctx, req.TraceID)
	// 会话上下文注入：switch_model 等工具从 ctx 取当前会话 id
	ctx = tool.WithConversationID(ctx, conv.ID)
	ctx = tool.WithMemberID(ctx, req.MemberID)

	// 待持久化的本轮消息（user + assistant + tool results）
	persist := []session.Message{{Role: gateway.RoleUser, Content: req.Content}}
	msgs := append(make([]session.Message, 0, len(history)+8), history...)
	msgs = append(msgs, gateway.Message{Role: gateway.RoleUser, Content: req.Content})

	system := prompt.Build(prompt.Options{
		MemberName: "", // P1 接成员服务后补名字
		Role:       schema.RoleAdult,
		Now:        rt.now(),
	})
	reqMsgs := append([]gateway.Message{{Role: gateway.RoleSystem, Content: system}}, msgs...)

	// 危险分级：按本循环累计最高风险降步数上限（ARCHITECTURE §8 工具表）
	curRisk := tool.RiskLow
	maxTurns := tool.MaxTurns[tool.RiskLow]
	highRiskExecuted := 0

	stopped := false
	for turn := 1; !stopped; turn++ {
		if turn > maxTurns {
			emit(Event{Type: "error", Error: "已达本次操作步数上限，已停止以防失控"})
			break
		}

		chatReq := gateway.ChatRequest{
			Model:    conv.ModelID,
			Messages: reqMsgs,
			Tools:    toolDefs,
		}
		start := rt.now()
		ch, err := rt.provider.StreamChat(ctx, chatReq)
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

		// 用量埋点（网关出口，ARCHITECTURE §10）
		if usage != nil {
			_ = rt.usage.Record(ctx, conv.ModelID, chatReq.Model, rt.provider.Name(), *usage, latency)
		}

		// 若大模型既无文本回复也无工具调用（如触发内容审查或空生成），注入友好兜底回复，防止前端挂死在思考中
		if text.Len() == 0 && len(toolCalls) == 0 {
			fallbackMsg := "未能生成有效回复，请重试或更换模型。"
			text.WriteString(fallbackMsg)
			emit(Event{Type: "token", Content: fallbackMsg})
		}

		assistant := session.Message{
			Role:      gateway.RoleAssistant,
			Content:   text.String(),
			ToolCalls: toolCalls,
		}
		persist = append(persist, assistant)
		reqMsgs = append(reqMsgs, assistant)

		// 无工具调用：循环正常终止
		if len(toolCalls) == 0 {
			break
		}

		// 执行工具（Executor 统一包办权限/参数校验/undo_log/审计）
		for _, tc := range toolCalls {
			res, err := rt.executor.Execute(ctx, tc.Name, tc.Args, req.MemberID, perms)
			if err != nil {
				// 失败回退（NFR）：错误回传 LLM 重试，不静默吞错
				errMsg := err.Error()
				emit(Event{Type: "tool_call", Tool: tc.Name, Error: errMsg})
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
		emit(Event{Type: "tool_call", Tool: tc.Name, Card: card, UndoID: res.UndoID})

			m := session.Message{
				Role:       gateway.RoleTool,
				Content:    res.Summary,
				Name:       tc.Name,
				ToolCallID: tc.ID,
			}
			reqMsgs = append(reqMsgs, m)
			persist = append(persist, m)

			// 危险分级：累计最高风险降上限；高风险连续执行 → 终止提示人工确认
			if spec, ok := toolByName[tc.Name]; ok {
				if spec.Risk > curRisk {
					curRisk = spec.Risk
					maxTurns = tool.MaxTurns[spec.Risk]
				}
				if spec.Risk == tool.RiskHigh {
					highRiskExecuted++
					if highRiskExecuted >= 2 {
						emit(Event{
							Type:  "error",
							Error: "涉及金额操作较多，建议人工确认，已停止本次操作",
						})
						stopped = true
					}
				}
			}
		}
	}

	// 先持久化，再回复（ARCHITECTURE §3：说出口即已落地）
	if err := rt.sessions.Append(ctx, conv.ID, req.MemberID, persist); err != nil {
		slog.Error("persist messages failed", "err", err, "conv", conv.ID)
	}
	emit(Event{Type: "done"})
	return nil
}
