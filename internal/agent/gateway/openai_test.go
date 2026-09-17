package gateway

import (
	"encoding/json"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestBuildRequestMessages(t *testing.T) {
	p := NewOpenAIProvider("key", "https://api.example.com", "test-model", "test")

	req := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: RoleSystem, Content: "你是家事助手"},
			{Role: RoleUser, Content: "买菜花了 120"},
			{Role: RoleAssistant, Content: "好的", ToolCalls: []ToolCall{
				{ID: "call-1", Name: "record_expense", Args: json.RawMessage(`{"amount":120}`)},
			}},
			{Role: RoleTool, Content: `{"ok":true}`, ToolCallID: "call-1", Name: "record_expense"},
		},
		Tools: []ToolDef{
			{Name: "record_expense", Description: "记账", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		Temperature: 0.7,
	}

	out := p.buildRequest(req)

	if out.Model != "test-model" {
		t.Fatalf("model: %q", out.Model)
	}
	if !out.Stream {
		t.Fatal("应开启流式")
	}
	if out.StreamOptions == nil || !out.StreamOptions.IncludeUsage {
		t.Fatal("应开启 IncludeUsage")
	}
	if out.Temperature != 0.7 {
		t.Fatalf("temperature: %v", out.Temperature)
	}

	if len(out.Messages) != 4 {
		t.Fatalf("消息数 %d", len(out.Messages))
	}
	if out.Messages[0].Role != openai.ChatMessageRoleSystem || out.Messages[0].Content != "你是家事助手" {
		t.Fatalf("system 消息转换错误: %+v", out.Messages[0])
	}
	if out.Messages[1].Role != openai.ChatMessageRoleUser || out.Messages[1].Content != "买菜花了 120" {
		t.Fatalf("user 消息转换错误: %+v", out.Messages[1])
	}
	if out.Messages[2].Role != openai.ChatMessageRoleAssistant {
		t.Fatalf("assistant 消息转换错误: %+v", out.Messages[2])
	}
	if len(out.Messages[2].ToolCalls) != 1 || out.Messages[2].ToolCalls[0].Function.Name != "record_expense" {
		t.Fatalf("tool_calls 转换错误: %+v", out.Messages[2].ToolCalls)
	}
	if out.Messages[3].Role != openai.ChatMessageRoleTool || out.Messages[3].ToolCallID != "call-1" {
		t.Fatalf("tool 结果消息转换错误: %+v", out.Messages[3])
	}

	if len(out.Tools) != 1 || out.Tools[0].Function.Name != "record_expense" {
		t.Fatalf("工具转换错误: %+v", out.Tools)
	}
}

func TestBuildRequestEmptyParamsFallback(t *testing.T) {
	p := NewOpenAIProvider("key", "", "m", "")
	out := p.buildRequest(ChatRequest{
		Tools: []ToolDef{{Name: "ping", Description: "ping"}},
	})
	params, ok := out.Tools[0].Function.Parameters.(json.RawMessage)
	if !ok {
		t.Fatalf("parameters 应为 json.RawMessage，got %T", out.Tools[0].Function.Parameters)
	}
	var m map[string]any
	if err := json.Unmarshal(params, &m); err != nil {
		t.Fatalf("空 parameters 应兜底为合法 schema: %v (raw=%s)", err, string(params))
	}
	if m["type"] != "object" {
		t.Fatalf("兜底 schema type 应为 object，got %v", m["type"])
	}
}

func TestOpenAIProviderName(t *testing.T) {
	if p := NewOpenAIProvider("k", "", "m", "deepseek"); p.Name() != "deepseek" {
		t.Fatalf("自定义供应商名错误: %q", p.Name())
	}
	if p := NewOpenAIProvider("k", "", "m", ""); p.Name() != "openai" {
		t.Fatalf("默认供应商名应为 openai，got %q", p.Name())
	}
}
