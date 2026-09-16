package gateway

import "encoding/json"

// RecordExpenseScript 是「今天买菜花了 120」的预置两轮脚本：
//
//	round 0：流式前缀 + tool_call(record_expense)
//	round 1：工具结果回传后的收尾文本（无 tool_call，循环终止）
//
// 供 runtime/evals 复用，保持「对话→执行→撤销」演示确定可复现。
func RecordExpenseScript() func(int, ChatRequest) []StreamEvent {
	return func(round int, _ ChatRequest) []StreamEvent {
		switch round {
		case 0:
			args, _ := json.Marshal(map[string]any{
				"amount": 120,
				"hint":   "买菜",
			})
			return []StreamEvent{
				{Delta: "好"},
				{Delta: "的"},
				{Delta: "，"},
				{Delta: "已记"},
				{Delta: "上"},
				{ToolCall: &ToolCall{ID: "call_1", Name: "record_expense", Args: args}},
				{Done: true, Usage: &Usage{PromptTokens: 50, CompletionTokens: 30, TotalTokens: 80}},
			}
		default:
			return []StreamEvent{
				{Delta: "已记账 ¥120，食材类。"},
				{Done: true, Usage: &Usage{PromptTokens: 120, CompletionTokens: 25, TotalTokens: 145}},
			}
		}
	}
}

// NoToolScript 是纯对话脚本（不触发工具），用于验证循环正常终止。
func NoToolScript() func(int, ChatRequest) []StreamEvent {
	return func(int, ChatRequest) []StreamEvent {
		return []StreamEvent{
			{Delta: "今天"},
			{Delta: "天气"},
			{Delta: "不错"},
			{Done: true, Usage: &Usage{PromptTokens: 10, CompletionTokens: 6, TotalTokens: 16}},
		}
	}
}

// InfiniteToolScript 每轮都返回 tool_call，用于验证危险分级步数上限。
func InfiniteToolScript() func(int, ChatRequest) []StreamEvent {
	return func(int, ChatRequest) []StreamEvent {
		args, _ := json.Marshal(map[string]any{"amount": 1, "hint": "测试"})
		return []StreamEvent{
			{ToolCall: &ToolCall{ID: "call_loop", Name: "record_expense", Args: args}},
			{Done: true, Usage: &Usage{TotalTokens: 5}},
		}
	}
}
