// Package gateway 是 LLM 网关：统一供应商抽象（ARCHITECTURE §2.1）。
//
// 设计原则：
//   - 供应商无关消息/工具/事件模型，屏蔽 OpenAI/Anthropic/Ollama 差异
//   - 流式输出通过 channel；context 取消即终止生成（runtime 约束）
//   - tool_call 在网关内部聚合为完整调用，runtime 不处理分片
//   - 用量埋点在网关出口（ARCHITECTURE §10），自动覆盖所有供应商
package gateway

import (
	"context"
	"encoding/json"

	"github.com/mk20mm/homeagent/internal/agent/tool"
)

// Role 消息角色（与 openapi Message.role 对齐）。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall 是一次完整的工具调用（LLM 产出，参数待校验）。
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// Message 是供应商无关的对话消息。
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`  // assistant 发起的调用
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool 结果消息的关联 id
	Name       string     `json:"name,omitempty"`         // 工具名（部分供应商需要）
}

// ToolDef 是下发给 LLM 的工具定义，由 tool.Spec 转换而来。
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// ToolsFromSpecs 把工具声明转成下发给 LLM 的定义（权限过滤后调用）。
func ToolsFromSpecs(specs []tool.Spec) []ToolDef {
	out := make([]ToolDef, 0, len(specs))
	for _, s := range specs {
		out = append(out, ToolDef{
			Name:        s.Name,
			Description: s.Description,
			Parameters:  s.InputSchema,
		})
	}
	return out
}

// Usage 是一次调用的 token 计数（用于成本埋点）。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// StreamEvent 是流式事件。ToolCall 为完整聚合结果；Done 携带 Usage。
type StreamEvent struct {
	Delta    string
	ToolCall *ToolCall
	Usage    *Usage
	Done     bool
}

// ChatRequest 是一次聊天请求。
type ChatRequest struct {
	Model       string
	Messages    []Message
	Tools       []ToolDef
	Temperature float32
}

// Provider 是供应商抽象。StreamChat 流式推送事件，ctx 取消时终止生成。
type Provider interface {
	Name() string
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}

// UsageRecorder 记录 LLM 用量（写 llm_usage 表），由 runtime 在每次调用后触发。
type UsageRecorder interface {
	Record(ctx context.Context, modelID string, model string, provider string, u Usage, latencyMS int64) error
}

// NoopUsageRecorder 不记录用量（仅用于不需要计量的场景）。
type NoopUsageRecorder struct{}

func (NoopUsageRecorder) Record(context.Context, string, string, string, Usage, int64) error {
	return nil
}
