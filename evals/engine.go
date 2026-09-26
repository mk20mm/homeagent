// evals 引擎：用例模型、评分维度与运行器（AI-PRD §6）。
//
// 评分维度对齐 §6.1：
//   - intent    意图识别准确率（top 场景：记账/报饭/打卡）
//   - tool      工具选择准确率（选对工具）
//   - param     参数抽取准确率（金额/类别/人）
//   - outcome   对话→执行成功率（数据库终态断言）
//   - undo      撤销链路成功率
//
// §6.2 安全否决项：CategorySafety 用例失败 = 整套 0 分，不可被平均分抵消。
package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/runtime"
)

// Category 用例类别（§6：capability / tool / safety）。
type Category string

const (
	CategoryCapability Category = "capability" // 意图识别：top 场景被正确理解
	CategoryTool       Category = "tool"       // 工具选择/参数抽取
	CategorySafety     Category = "safety"     // 安全否决项（§6.2）
)

// Metric 评分维度（§6.1 五项可测量指标）。
type Metric string

const (
	MetricIntent  Metric = "intent"  // 意图识别准确率
	MetricTool    Metric = "tool"    // 工具选择准确率
	MetricParam   Metric = "param"   // 参数抽取准确率
	MetricOutcome Metric = "outcome" // 对话→执行成功率
	MetricUndo    Metric = "undo"    // 撤销链路成功率
)

// thresholds §6.1 阈值（v0）。低于阈值 = 整套不通过。
var thresholds = map[Metric]float64{
	MetricIntent:  0.90,
	MetricTool:    0.95,
	MetricParam:   0.90,
	MetricOutcome: 0.85,
	MetricUndo:    0.99,
}

// CheckResult 一条断言结果。每条断言归属一个评分维度。
type CheckResult struct {
	Name   string
	Metric Metric
	Pass   bool
	Detail string // 失败原因（失败时填）
}

// Step 一个对话回合：脚本模拟 LLM 对用户输入的响应。
type Step struct {
	// MemberID 说话成员（权限矩阵锚点）
	MemberID string
	// Content 用户输入（展示与 prompt 一致性，实际不参与断言）
	Content string
	// Script LLM 行为脚本：round n 应产出什么（token/工具调用/收尾）。
	// 脚本即「LLM 应当给出的工具调用」——evals 断言的是调度链路能否正确执行它。
	Script func(round int, req gateway.ChatRequest) []gateway.StreamEvent
}

// Case 一个金标准用例。
type Case struct {
	ID          string
	Category    Category
	Module      string // expense / task / meal / system
	Description string
	Steps       []Step
	// Check 终态断言（Code/Rule Grader，§6.3）：数据库终态/权限/工具调用序列。
	Check func(s *Sandbox, events []runtime.Event) []CheckResult
}

// RunResult 单用例结果。
type RunResult struct {
	Case   Case
	Pass   bool
	Checks []CheckResult
}

// SuiteResult 整套结果（含按维度聚合的指标）。
type SuiteResult struct {
	Results []RunResult
	Metrics map[Metric]MetricStat
	// SafetyHit §6.2 否决项命中（任一 safety 用例失败）
	SafetyHit bool
	// Pass 整套是否通过：无否决命中 + 五项指标全部达标
	Pass bool
}

// MetricStat 单维度统计。
type MetricStat struct {
	Pass  int
	Total int
}

// Rate 通过率。
func (m MetricStat) Rate() float64 {
	if m.Total == 0 {
		return 1 // 无用例的维度视为达标（不因模块未覆盖而判败）
	}
	return float64(m.Pass) / float64(m.Total)
}

// runCase 跑一个用例：沙箱隔离，按步驱动 Runtime，收事件交给断言。
func runCase(ctx context.Context, c Case) RunResult {
	sb := newSandbox()
	var allEvents []runtime.Event
	var mu sync.Mutex

	// 每个成员一个会话：跨成员用例（派发→打卡）模拟各自视角的对话，
	// 与生产一致（会话按 member 隔离，Ensure 会复用已有会话）。
	convs := map[string]string{}
	for _, st := range c.Steps {
		if _, ok := convs[st.MemberID]; !ok {
			id, err := sb.Sessions.CreateConversation(ctx, st.MemberID, "", "")
			if err != nil {
				return RunResult{Case: c, Checks: []CheckResult{{
					Name: "会话初始化", Pass: false, Detail: err.Error(),
				}}}
			}
			convs[st.MemberID] = id
		}
		rt := sb.newRuntime(st.Script)
		req := runtime.RunRequest{
			ConversationID: convs[st.MemberID],
			MemberID:       st.MemberID,
			Content:        st.Content,
			OnEvent: func(e runtime.Event) {
				mu.Lock()
				defer mu.Unlock()
				allEvents = append(allEvents, e)
			},
		}
		if err := rt.Run(ctx, req); err != nil {
			mu.Lock()
			allEvents = append(allEvents, runtime.Event{Type: "error", Error: err.Error()})
			mu.Unlock()
		}
	}

	checks := c.Check(sb, allEvents)
	pass := true
	for _, ch := range checks {
		if !ch.Pass {
			pass = false
			break
		}
	}
	return RunResult{Case: c, Pass: pass, Checks: checks}
}

// runSuite 跑完全部用例并聚合指标（§6.1 + §6.2）。
func runSuite(ctx context.Context, cases []Case) SuiteResult {
	res := SuiteResult{Metrics: map[Metric]MetricStat{}}
	for _, c := range cases {
		r := runCase(ctx, c)
		res.Results = append(res.Results, r)
		if c.Category == CategorySafety && !r.Pass {
			res.SafetyHit = true // §6.2：否决项命中即 0 分
		}
		for _, ch := range r.Checks {
			if ch.Metric == "" {
				continue // 否决项只看 SafetyHit，不进平均分
			}
			st := res.Metrics[ch.Metric]
			st.Total++
			if ch.Pass {
				st.Pass++
			}
			res.Metrics[ch.Metric] = st
		}
	}

	res.Pass = !res.SafetyHit
	if res.Pass {
		for m, th := range thresholds {
			if res.Metrics[m].Rate() < th {
				res.Pass = false
				break
			}
		}
	}
	return res
}

// ---- 断言助手（Code/Rule Grader，§6.3）----

// toolEvents 取某工具的成功调用事件（参数/终态断言的输入）。
func toolEvents(events []runtime.Event, toolName string) []runtime.Event {
	out := make([]runtime.Event, 0)
	for _, e := range events {
		if e.Type == "tool_call" && e.Tool == toolName {
			out = append(out, e)
		}
	}
	return out
}

// toolSucceeded 工具被成功执行（无错误）。
func toolSucceeded(events []runtime.Event, toolName string) bool {
	for _, e := range toolEvents(events, toolName) {
		if e.Error == "" {
			return true
		}
	}
	return false
}

// toolDenied 工具被拒绝（执行层权限兜底）。
func toolDenied(events []runtime.Event, toolName string) bool {
	for _, e := range toolEvents(events, toolName) {
		if e.Error != "" {
			return true
		}
	}
	return false
}

// cardOf 取工具卡片（同工具多次调用时取最后一次：幂等提示在第二次卡片上）。
func cardOf(events []runtime.Event, toolName string) (map[string]any, bool) {
	var last map[string]any
	found := false
	for _, e := range toolEvents(events, toolName) {
		if len(e.Card) > 0 && e.Error == "" {
			var m map[string]any
			if err := json.Unmarshal(e.Card, &m); err == nil {
				last, found = m, true
			}
		}
	}
	return last, found
}

// cardNum 取卡片数值字段（金额断言用，兼容 int/float64）。
func cardNum(card map[string]any, key string) (float64, bool) {
	v, ok := card[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// ---- 脚本构造助手 ----
//
// 脚本即「LLM 应当给出的响应」。常用模式抽成函数，用例只写参数。

// toolScript 两轮脚本：round 0 发起工具调用，round 1 收尾文本（模拟工具结果回传后的总结）。
func toolScript(name string, args map[string]any, reply string) func(int, gateway.ChatRequest) []gateway.StreamEvent {
	return func(round int, _ gateway.ChatRequest) []gateway.StreamEvent {
		if round > 0 {
			return []gateway.StreamEvent{{Delta: reply}, doneEvent()}
		}
		return []gateway.StreamEvent{toolCallEvent(name, args), doneEvent()}
	}
}

// multiToolScript 一轮发起多个工具调用（多意图句子，S9）。
type toolCallSpec struct {
	name string
	args map[string]any
}

func multiToolScript(calls []toolCallSpec, reply string) func(int, gateway.ChatRequest) []gateway.StreamEvent {
	return func(round int, _ gateway.ChatRequest) []gateway.StreamEvent {
		if round > 0 {
			return []gateway.StreamEvent{{Delta: reply}, doneEvent()}
		}
		out := make([]gateway.StreamEvent, 0, len(calls)+1)
		for _, c := range calls {
			out = append(out, toolCallEvent(c.name, c.args))
		}
		out = append(out, doneEvent())
		return out
	}
}

// chatOnlyScript 纯对话脚本（无工具调用，意图识别负例/追问）。
func chatOnlyScript(reply string) func(int, gateway.ChatRequest) []gateway.StreamEvent {
	return func(int, gateway.ChatRequest) []gateway.StreamEvent {
		return []gateway.StreamEvent{{Delta: reply}, doneEvent()}
	}
}

func toolCallEvent(name string, args map[string]any) gateway.StreamEvent {
	return gateway.StreamEvent{ToolCall: &gateway.ToolCall{ID: "call_eval", Name: name, Args: mustJSON(args)}}
}

func doneEvent() gateway.StreamEvent {
	return gateway.StreamEvent{Done: true, Usage: &gateway.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal: %v", err))
	}
	return b
}

// summarize 打印量化结果（make eval 的可读输出）。
func summarize(res SuiteResult) string {
	var b strings.Builder
	b.WriteString("\n========== evals 金标准评测 ==========\n")

	byCat := map[Category][]RunResult{}
	for _, r := range res.Results {
		byCat[r.Case.Category] = append(byCat[r.Case.Category], r)
	}
	for _, cat := range []Category{CategoryCapability, CategoryTool, CategorySafety} {
		list := byCat[cat]
		if len(list) == 0 {
			continue
		}
		passN := 0
		for _, r := range list {
			if r.Pass {
				passN++
			}
		}
		b.WriteString(fmt.Sprintf("[%s] %d/%d\n", cat, passN, len(list)))
		for _, r := range list {
			mark := "PASS"
			if !r.Pass {
				mark = "FAIL"
			}
			b.WriteString(fmt.Sprintf("  %s %s.%s %s\n", mark, r.Case.Module, r.Case.ID, r.Case.Description))
			for _, ch := range r.Checks {
				if !ch.Pass {
					b.WriteString(fmt.Sprintf("      x [%s] %s: %s\n", ch.Metric, ch.Name, ch.Detail))
				}
			}
		}
	}

	b.WriteString("\n--- 指标（§6.1 阈值）---\n")
	for _, m := range []Metric{MetricIntent, MetricTool, MetricParam, MetricOutcome, MetricUndo} {
		st := res.Metrics[m]
		th := thresholds[m]
		rate := st.Rate()
		status := "OK"
		if rate < th {
			status = "BELOW"
		}
		b.WriteString(fmt.Sprintf("  %-8s %d/%d = %.0f%% (阈值 %.0f%%) %s\n",
			m, st.Pass, st.Total, rate*100, th*100, status))
	}

	b.WriteString("\n")
	if res.SafetyHit {
		b.WriteString("!! 安全否决项命中：整套 0 分（§6.2，不可被平均分抵消）\n")
	}
	if res.Pass {
		b.WriteString("RESULT: PASS（阶段 A evals 出口达标）\n")
	} else {
		b.WriteString("RESULT: FAIL\n")
	}
	return b.String()
}

// timeoutCtx 防止单个用例挂死（脚本供应商不会挂，保险起见）。
func timeoutCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
