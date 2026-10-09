package session

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
)

func TestCompaction_UnderThresholdNoop(t *testing.T) {
	history := make([]Message, 20)
	for i := 0; i < 20; i++ {
		history[i] = Message{
			Role:    gateway.RoleUser,
			Content: fmt.Sprintf("消息 %d", i+1),
		}
	}

	callCount := 0
	provider := gateway.NewScriptedProvider(func(round int, req gateway.ChatRequest) []gateway.StreamEvent {
		callCount++
		return []gateway.StreamEvent{{Delta: "summary"}, {Done: true}}
	})

	compacted, err := Compact(context.Background(), provider, "test-model", history)
	if err != nil {
		t.Fatalf("Compact returned unexpected error: %v", err)
	}
	if callCount != 0 {
		t.Errorf("低于阈值不应调用 LLM 生成摘要，实际调用次数: %d", callCount)
	}
	if len(compacted) != len(history) {
		t.Errorf("低于阈值应保持原样，got %d want %d", len(compacted), len(history))
	}
}

func TestCompaction_OverThresholdGeneratesSummaryAndRetainsRecent(t *testing.T) {
	total := 55
	history := make([]Message, total)
	for i := 0; i < total; i++ {
		history[i] = Message{
			Role:    gateway.RoleUser,
			Content: fmt.Sprintf("用户历史问题 %d", i+1),
		}
	}

	callCount := 0
	var receivedPrompt string
	mockSummary := "家庭历史：成员偏好川菜，上周买了冰箱，有待办洗车。"

	provider := gateway.NewScriptedProvider(func(round int, req gateway.ChatRequest) []gateway.StreamEvent {
		callCount++
		if len(req.Messages) > 0 {
			receivedPrompt = req.Messages[0].Content
		}
		return []gateway.StreamEvent{
			{Delta: mockSummary},
			{Done: true},
		}
	})

	compacted, err := Compact(context.Background(), provider, "test-model", history)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("超过阈值应调用 1 次 LLM 生成摘要，实际: %d", callCount)
	}

	// 期望返回 1 条会话摘要 + 10 条最近原文 = 11 条
	expectedLen := 1 + RetainRecent
	if len(compacted) != expectedLen {
		t.Fatalf("压缩后应有 %d 条消息，实际: %d", expectedLen, len(compacted))
	}

	// 首条必须是会话摘要
	if compacted[0].Role != gateway.RoleSystem {
		t.Errorf("首条消息角色应为 system，实际: %s", compacted[0].Role)
	}
	if !strings.HasPrefix(compacted[0].Content, SummaryPrefix) {
		t.Errorf("首条消息内容应以前缀 %q 开头，实际: %s", SummaryPrefix, compacted[0].Content)
	}
	if !strings.Contains(compacted[0].Content, mockSummary) {
		t.Errorf("首条摘要内容应包含生成的摘要，实际: %s", compacted[0].Content)
	}

	// 后 10 条必须与原历史最后 10 条完全一致
	for i := 0; i < RetainRecent; i++ {
		origIdx := total - RetainRecent + i
		compIdx := 1 + i
		if compacted[compIdx].Content != history[origIdx].Content {
			t.Errorf("保留的第 %d 条原文不匹配: got %q, want %q",
				i+1, compacted[compIdx].Content, history[origIdx].Content)
		}
	}

	// 校验提示词中包含了前 45 条被总结的消息
	if !strings.Contains(receivedPrompt, "用户历史问题 1") {
		t.Errorf("提示词中应包含第 1 条历史问题")
	}
	if !strings.Contains(receivedPrompt, "用户历史问题 45") {
		t.Errorf("提示词中应包含第 45 条历史问题")
	}
	if strings.Contains(receivedPrompt, "用户历史问题 46") {
		t.Errorf("提示词中不应包含最近 10 条保留的消息（第 46 条）")
	}
}

func TestEffectiveHistory(t *testing.T) {
	history := []Message{
		{Role: gateway.RoleUser, Content: "古老消息 1"},
		{Role: gateway.RoleAssistant, Content: "古老消息 2"},
		{Role: gateway.RoleSystem, Content: "[会话摘要] 这是之前生成的摘要"},
		{Role: gateway.RoleUser, Content: "新消息 1"},
		{Role: gateway.RoleAssistant, Content: "新消息 2"},
	}

	effective := EffectiveHistory(history)
	if len(effective) != 3 {
		t.Fatalf("应从最近一条摘要开始截断，期望 3 条，实际 %d 条", len(effective))
	}
	if effective[0].Content != "[会话摘要] 这是之前生成的摘要" {
		t.Errorf("截断后首条应为会话摘要，实际: %s", effective[0].Content)
	}

	// 没有摘要时应完整返回
	raw := []Message{
		{Role: gateway.RoleUser, Content: "消息 1"},
	}
	if len(EffectiveHistory(raw)) != 1 {
		t.Errorf("无摘要时应返回完整列表")
	}
}
