package repo

import (
	"context"
	"testing"

	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/llmmodel"
	"github.com/mk20mm/homeagent/internal/store/ent/llmprovider"
	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
)

// seedTestModels 建两个模型供切换测试。
func seedTestModels(t *testing.T, c *ent.Client) (defaultID, otherID string) {
	t.Helper()
	ctx := context.Background()
	prov, err := c.LLMProvider.Create().
		SetName(llmprovider.NameDeepseek).
		SetEnabled(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	def, err := c.LLMModel.Create().
		SetModelName("deepseek-chat").
		SetDisplayName("DeepSeek 对话").
		SetEnabled(true).
		SetIsDefault(true).
		SetProvider(prov).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed default model: %v", err)
	}
	other, err := c.LLMModel.Create().
		SetModelName("gpt-4o").
		SetDisplayName("GPT-4o").
		SetEnabled(true).
		SetProvider(prov).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed other model: %v", err)
	}
	return def.ID.String(), other.ID.String()
}

func TestListEnabledAndFindModel(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()
	defID, _ := seedTestModels(t, c)
	s := New(c)
	ctx := context.Background()

	list, err := s.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("查询清单: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 个启用模型，got %d", len(list))
	}

	// 按 id 精确查
	m, err := s.FindModel(ctx, defID)
	if err != nil {
		t.Fatalf("按 id 查: %v", err)
	}
	if m.ModelName != "deepseek-chat" {
		t.Fatalf("模型名不匹配: %q", m.ModelName)
	}
	// 按名字模糊查
	m, err = s.FindModel(ctx, "gpt-4o")
	if err != nil || m.ModelName != "gpt-4o" {
		t.Fatalf("按名查 gpt-4o: err=%v m=%+v", err, m)
	}
	// 不存在的模型
	if _, err := s.FindModel(ctx, "no-such-model"); err == nil {
		t.Fatal("不存在模型应返回错误")
	}

	// 未启用的查不到
	if err := c.LLMModel.Update().Where(llmmodel.ModelNameEQ("gpt-4o")).SetEnabled(false).Exec(ctx); err != nil {
		t.Fatalf("禁用模型: %v", err)
	}
	if _, err := s.FindModel(ctx, "gpt-4o"); err == nil {
		t.Fatal("已禁用模型不应被查到")
	}
}

func TestSwitchConversationModel(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()
	defID, otherID := seedTestModels(t, c)
	s := New(c)
	ctx := context.Background()

	memberID := testMemberID(t, c)
	conv, err := c.Conversation.Create().
		SetTitle("测试会话").
		SetMemberID(toUUID(memberID)).
		Save(ctx)
	if err != nil {
		t.Fatalf("建会话: %v", err)
	}

	// 未设置模型 → 返回默认
	m, err := s.GetConversationModel(ctx, conv.ID.String())
	if err != nil || m.ID != defID {
		t.Fatalf("默认模型应为 deepseek，got err=%v m=%+v", err, m)
	}

	// 切换
	if err := s.SetConversationModel(ctx, conv.ID.String(), otherID); err != nil {
		t.Fatalf("切换模型: %v", err)
	}
	m, err = s.GetConversationModel(ctx, conv.ID.String())
	if err != nil || m.ID != otherID {
		t.Fatalf("切换后应为 gpt-4o，got err=%v m=%+v", err, m)
	}
}

func TestResolveConnectionAndModelCRUD(t *testing.T) {
	c, _ := newTestClient(t)
	defer func() { _ = c.Close() }()
	ctx := context.Background()

	encKey := "01234567890123456789012345678901"
	s := New(c)
	ps := NewProviderStore(s, encKey)

	// 建 provider 并配加密 key
	plainKey := "sk-test-secret-12345"
	prov, err := ps.UpdateProvider(ctx, "", dommodel.ProviderUpdate{})
	// 取出任意已有 provider 或新建
	providers, err := ps.ListProviders(ctx)
	if err != nil || len(providers) == 0 {
		p, err := c.LLMProvider.Create().
			SetName(llmprovider.NameDeepseek).
			SetEnabled(true).
			SetBaseURL("https://api.deepseek.com").
			Save(ctx)
		if err != nil {
			t.Fatalf("create provider: %v", err)
		}
		prov.ID = p.ID.String()
	} else {
		prov = providers[0]
	}

	// 更新 API Key
	if _, err := ps.UpdateProvider(ctx, prov.ID, dommodel.ProviderUpdate{
		APIKey: &plainKey,
	}); err != nil {
		t.Fatalf("update provider key: %v", err)
	}

	// 创建模型
	modelInfo, err := ps.CreateModel(ctx, prov.ID, "deepseek-reasoner", "DeepSeek R1", true)
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	if modelInfo.ModelName != "deepseek-reasoner" || !modelInfo.IsDefault {
		t.Fatalf("model mismatch: %+v", modelInfo)
	}

	// 测试 ResolveConnection (默认模型)
	conn, ok, err := ps.ResolveConnection(ctx, "")
	if err != nil || !ok {
		t.Fatalf("resolve default conn: ok=%v err=%v", ok, err)
	}
	if conn.APIKey != plainKey || conn.Model != "deepseek-reasoner" {
		t.Fatalf("conn mismatch: %+v", conn)
	}

	// 按模型名解析
	conn2, ok, err := ps.ResolveConnection(ctx, "deepseek-reasoner")
	if err != nil || !ok || conn2.Model != "deepseek-reasoner" {
		t.Fatalf("resolve by name: ok=%v err=%v conn=%+v", ok, err, conn2)
	}

	// GetProviderConnection
	pconn, err := ps.GetProviderConnection(ctx, prov.ID)
	if err != nil || pconn.APIKey != plainKey {
		t.Fatalf("get provider connection: err=%v pconn=%+v", err, pconn)
	}

	// 默认模型不可删除
	if err := ps.DeleteModel(ctx, modelInfo.ID); err == nil {
		t.Fatal("默认模型应不可删除")
	}

	// 建非默认模型并删除
	m2, err := ps.CreateModel(ctx, prov.ID, "temp-model", "Temp", false)
	if err != nil {
		t.Fatalf("create temp model: %v", err)
	}
	if err := ps.DeleteModel(ctx, m2.ID); err != nil {
		t.Fatalf("delete non-default model: %v", err)
	}
}

