package runtime

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/agent/tool"
)

func TestRuntime_CompactionTriggeredOnLongHistory(t *testing.T) {
	reg := tool.NewRegistry()
	exec := tool.NewExecutor(reg, nil, nil)
	sessions := newMemSession(map[string]map[string]bool{"m1": {}})

	convID, err := sessions.CreateConversation(context.Background(), "m1", "long-conv", "")
	if err != nil {
		t.Fatalf("create conv failed: %v", err)
	}

	// 注入 55 条历史消息（超过 CompactThreshold = 50）
	rawMsgs := make([]session.Message, 55)
	for i := 0; i < 55; i++ {
		rawMsgs[i] = session.Message{
			Role:    gateway.RoleUser,
			Content: fmt.Sprintf("历史消息 %d", i+1),
		}
	}
	_ = sessions.Append(context.Background(), convID, "m1", rawMsgs)

	var chatRequestMessages []gateway.Message
	summaryGenerated := "已提炼家庭重点事实：经常讨论晚餐与家务。"

	// Provider 先响应 Compaction 摘要生成，再响应实际对话
	provider := gateway.NewScriptedProvider(func(round int, req gateway.ChatRequest) []gateway.StreamEvent {
		if round == 0 {
			// Compaction 摘要生成请求
			return []gateway.StreamEvent{
				{Delta: summaryGenerated},
				{Done: true},
			}
		}
		// round == 1: 真正的用户对话请求，记录入参 Messages
		chatRequestMessages = req.Messages
		return []gateway.StreamEvent{
			{Delta: "好的，为您解答。"},
			{Done: true},
		}
	})

	rt := New(Options{
		Provider: provider,
		Executor: exec,
		Sessions: sessions,
	})

	events := collectEvents(context.Background(), rt, RunRequest{
		ConversationID: convID,
		MemberID:       "m1",
		Content:        "请问今天怎么安排",
	})

	if !hasEvent(events, "done") {
		t.Fatal("对话应正常完成并以 done 结束")
	}

	// 1. 验证摘要消息被持久化回了会话
	msgsInStore, _ := sessions.LoadMessages(context.Background(), convID, "m1")
	hasSummaryInStore := false
	for _, m := range msgsInStore {
		if m.Role == gateway.RoleSystem && strings.Contains(m.Content, summaryGenerated) {
			hasSummaryInStore = true
			break
		}
	}
	if !hasSummaryInStore {
		t.Errorf("会话存储中应当持久化保存了生成的 [会话摘要]")
	}

	// 2. 验证大模型对话请求中接收到了压缩后的历史上下文
	// 期望结构：SystemPrompt + [会话摘要] + 10条最近原文 + 当前用户提问
	foundSummaryInReq := false
	foundRecent10InReq := false
	for _, m := range chatRequestMessages {
		if m.Role == gateway.RoleSystem && strings.Contains(m.Content, summaryGenerated) {
			foundSummaryInReq = true
		}
		if m.Content == "历史消息 55" {
			foundRecent10InReq = true
		}
		// 历史消息 1 应该已经被压缩，不应直接作为完整消息出现在上下文中
		if m.Content == "历史消息 1" {
			t.Errorf("历史消息 1 应当已被摘要压缩，不应作为单独消息出现在 LLM 对话上下文中")
		}
	}

	if !foundSummaryInReq {
		t.Errorf("LLM 请求中未包含会话摘要系统消息")
	}
	if !foundRecent10InReq {
		t.Errorf("LLM 请求中未包含保留的最近原文")
	}
}
