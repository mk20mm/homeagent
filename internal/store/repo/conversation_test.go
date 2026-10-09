package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

func TestConversationRepo_Fork(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	ctx := context.Background()
	memberID := testMemberID(t, c)

	// 1. 创建原会话并写入 4 条消息
	convID, err := s.CreateConversation(ctx, memberID, "讨论周末出行", "")
	if err != nil {
		t.Fatalf("create conversation failed: %v", err)
	}

	msgs := []session.Message{
		{Role: gateway.RoleUser, Content: "周六我们去哪里玩？"},
		{Role: gateway.RoleAssistant, Content: "可以考虑去奥林匹克森林公园或者植物园。"},
		{Role: gateway.RoleUser, Content: "那去奥森吧，需要准备什么？"},
		{Role: gateway.RoleAssistant, Content: "建议带上野餐垫和风筝。"},
	}
	if err := s.AppendMessages(ctx, convID, memberID, msgs); err != nil {
		t.Fatalf("append messages failed: %v", err)
	}

	// 读取历史消息以获取第 2 条消息的 ID
	history, err := s.LoadMessageList(ctx, convID, memberID, 10)
	if err != nil {
		t.Fatalf("load message list failed: %v", err)
	}
	if len(history) != 4 {
		t.Fatalf("expected 4 messages in original, got %d", len(history))
	}
	forkPointMsgID := history[1].ID // Assistant 回答 "可以考虑去奥森或植物园"

	// 2. 从第 2 条消息处分叉时光机探索（比如用户选择植物园）
	forkedConv, err := s.ForkConversation(ctx, convID, memberID, forkPointMsgID, "改去植物园分支")
	if err != nil {
		t.Fatalf("fork conversation failed: %v", err)
	}

	// 验证分叉元数据
	if forkedConv.ForkedFromID != convID {
		t.Errorf("expected ForkedFromID %s, got %s", convID, forkedConv.ForkedFromID)
	}
	if forkedConv.ForkedAtMessageID != forkPointMsgID {
		t.Errorf("expected ForkedAtMessageID %s, got %s", forkPointMsgID, forkedConv.ForkedAtMessageID)
	}
	if forkedConv.Title != "改去植物园分支" {
		t.Errorf("expected title '改去植物园分支', got %s", forkedConv.Title)
	}

	// 验证分叉会话消息历史（只复制了前 2 条）
	forkedHistory, err := s.LoadMessageList(ctx, forkedConv.ID, memberID, 10)
	if err != nil {
		t.Fatalf("load forked message list failed: %v", err)
	}
	if len(forkedHistory) != 2 {
		t.Fatalf("expected 2 messages in forked branch, got %d", len(forkedHistory))
	}
	if forkedHistory[0].Content != "周六我们去哪里玩？" {
		t.Errorf("message 0 mismatch: %s", forkedHistory[0].Content)
	}
	if forkedHistory[1].Content != "可以考虑去奥林匹克森林公园或者植物园。" {
		t.Errorf("message 1 mismatch: %s", forkedHistory[1].Content)
	}

	// 3. 在分支中继续探索（追加新消息）
	branchMsgs := []session.Message{
		{Role: gateway.RoleUser, Content: "那改去植物园吧，门票多少钱？"},
	}
	if err := s.AppendMessages(ctx, forkedConv.ID, memberID, branchMsgs); err != nil {
		t.Fatalf("append branch messages failed: %v", err)
	}

	// 验证分支与原会话互相隔离
	origCheck, err := s.LoadMessageList(ctx, convID, memberID, 10)
	if err != nil || len(origCheck) != 4 {
		t.Fatalf("original conversation should still have exactly 4 messages, got %d", len(origCheck))
	}
	forkedCheck, err := s.LoadMessageList(ctx, forkedConv.ID, memberID, 10)
	if err != nil || len(forkedCheck) != 3 {
		t.Fatalf("forked conversation should have 3 messages, got %d", len(forkedCheck))
	}
}

func TestRunRepo_SubagentTreeLifecycle(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()

	s := New(c)
	ctx := context.Background()
	memberID := testMemberID(t, c)

	// 1. 创建根任务
	rootRun, err := s.CreateRun(ctx, memberID, "筹备家庭聚会", schema.IntentPlan, uuid.NewString(), "", []string{"cook", "clean"})
	if err != nil {
		t.Fatalf("create root run failed: %v", err)
	}

	// 2. 派生子任务（Subagent 1: 扫地）
	child1, err := s.CreateChildRun(ctx, memberID, rootRun.ID, "扫地机清扫全屋", schema.IntentAct)
	if err != nil {
		t.Fatalf("create child 1 failed: %v", err)
	}
	if child1.ParentRunID != rootRun.ID {
		t.Errorf("expected parent_run_id %s, got %s", rootRun.ID, child1.ParentRunID)
	}
	if child1.RootRunID != rootRun.ID {
		t.Errorf("expected root_run_id %s, got %s", rootRun.ID, child1.RootRunID)
	}

	// 3. 派生子任务（Subagent 2: 采购做饭食材）
	child2, err := s.CreateChildRun(ctx, memberID, rootRun.ID, "采购排骨与青菜", schema.IntentAct)
	if err != nil {
		t.Fatalf("create child 2 failed: %v", err)
	}

	// 4. 派生孙子任务（Subagent 2 派生子任务: 查询附近超市优惠）
	grandchild, err := s.CreateChildRun(ctx, memberID, child2.ID, "查询附近永辉超市优惠", schema.IntentQuery)
	if err != nil {
		t.Fatalf("create grandchild failed: %v", err)
	}
	if grandchild.ParentRunID != child2.ID {
		t.Errorf("expected grandchild parent_run_id %s, got %s", child2.ID, grandchild.ParentRunID)
	}
	if grandchild.RootRunID != rootRun.ID {
		t.Errorf("expected grandchild root_run_id to match root %s, got %s", rootRun.ID, grandchild.RootRunID)
	}

	// 5. 查找 rootRun 的直接子任务
	children, err := s.FindChildRuns(ctx, rootRun.ID)
	if err != nil {
		t.Fatalf("find child runs failed: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 direct children for root, got %d", len(children))
	}
}
