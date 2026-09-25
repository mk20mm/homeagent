package gateway

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	numRe  = regexp.MustCompile(`(\d+(?:\.\d+)?)`)
	hintRe = regexp.MustCompile(`(?:买|购)([^花了\s\d]+)`)
)

func parseExpenseFromMessages(msgs []Message) (float64, string) {
	amount := 120.0
	hint := "买菜"
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleUser {
			content := strings.TrimSpace(msgs[i].Content)
			if idx := strings.Index(content, "花了"); idx >= 0 {
				after := content[idx+len("花了"):]
				if m := numRe.FindStringSubmatch(after); len(m) > 1 {
					if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > 0 {
						amount = v
					}
				}
				h := strings.TrimSpace(content[:idx])
				for _, prefix := range []string{"今天", "昨天", "刚才", "前天"} {
					h = strings.TrimPrefix(h, prefix)
				}
				if h != "" {
					hint = h
				}
			} else {
				matches := numRe.FindAllStringSubmatch(content, -1)
				if len(matches) > 0 {
					last := matches[len(matches)-1]
					if v, err := strconv.ParseFloat(last[1], 64); err == nil && v > 0 {
						amount = v
					}
				}
				if m := hintRe.FindStringSubmatch(content); len(m) > 1 && m[1] != "" {
					hint = "买" + strings.TrimSpace(m[1])
				} else if content != "" {
					hint = content
				}
			}
			break
		}
	}
	return amount, hint
}

// RecordExpenseScript 是「今天买菜花了 120」的预置两轮脚本：
//
//	round 0：流式前缀 + tool_call(record_expense)
//	round 1：工具结果回传后的收尾文本（无 tool_call，循环终止）
//
// 供 runtime/evals 复用，保持「对话→执行→撤销」演示确定可复现。
func isExpenseContent(msgs []Message) bool {
	if len(msgs) == 0 {
		return true // 单测无消息时保持兼容
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleUser {
			content := msgs[i].Content
			return strings.Contains(content, "花了") ||
				strings.Contains(content, "买") ||
				strings.Contains(content, "购") ||
				strings.Contains(content, "记账") ||
				strings.Contains(content, "元") ||
				strings.Contains(content, "块") ||
				strings.Contains(content, "支出")
		}
	}
	return true
}

func RecordExpenseScript() func(int, ChatRequest) []StreamEvent {
	var lastAmount float64 = 120.0
	return func(round int, req ChatRequest) []StreamEvent {
		if !isExpenseContent(req.Messages) {
			return []StreamEvent{
				{Delta: "好的，收到。"},
				{Done: true, Usage: &Usage{PromptTokens: 20, CompletionTokens: 5, TotalTokens: 25}},
			}
		}

		isToolResponse := (round%2 == 1) || (len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == RoleTool)
		if isToolResponse {
			delta := "已记账 ¥120，食材类。"
			if lastAmount != 120.0 {
				delta = fmt.Sprintf("已记账 ¥%.2f，食材类。", lastAmount)
			}
			return []StreamEvent{
				{Delta: delta},
				{Done: true, Usage: &Usage{PromptTokens: 120, CompletionTokens: 25, TotalTokens: 145}},
			}
		}

		amount, hint := parseExpenseFromMessages(req.Messages)
		lastAmount = amount
		args, _ := json.Marshal(map[string]any{
			"amount": amount,
			"hint":   hint,
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
