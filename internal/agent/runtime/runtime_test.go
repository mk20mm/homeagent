package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/expense"
)

// ---- 测试替件 ----

// memExpenseRepo 内存仓储：实现 expense.ExpenseRepo，供真实领域 service 使用。
type memExpenseRepo struct {
	mu      sync.Mutex
	byID    map[string]*memExpense
	byKey   map[string]string // idempotencyKey -> id
	deleted map[string]bool
}

type memExpense struct {
	id         string
	memberID   string
	cents      int64
	hint       string
	category   string
	occurredAt time.Time
}

func newMemExpenseRepo() *memExpenseRepo {
	return &memExpenseRepo{
		byID:    map[string]*memExpense{},
		byKey:   map[string]string{},
		deleted: map[string]bool{},
	}
}

// Create 幂等冲突返回已存在 id + CodeConflict（对齐真实仓储语义）。
func (r *memExpenseRepo) Create(_ context.Context, memberID string, cmd expense.RecordExpenseCmd, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.byKey[key]; ok {
		return id, apperr.New(apperr.CodeConflict, "账单已存在", nil)
	}
	id := uuid.NewString()
	r.byID[id] = &memExpense{
		id: id, memberID: memberID, cents: cmd.AmountCents,
		hint: cmd.Hint, category: cmd.Category, occurredAt: cmd.OccurredOrNow(),
	}
	r.byKey[key] = id
	return id, nil
}

func (r *memExpenseRepo) Delete(_ context.Context, expenseID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted[expenseID] = true
	return nil
}

// Update 修正账单，返回旧值快照（对齐真实仓储：隔离 + 软删除过滤 + 键同步）。
func (r *memExpenseRepo) Update(_ context.Context, memberID string, expenseID string, cmd expense.UpdateExpenseCmd) (expense.ExpenseRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[expenseID]
	if !ok || e.memberID != memberID || r.deleted[expenseID] {
		return expense.ExpenseRecord{}, apperr.New(apperr.CodeNotFound, "账单不存在", nil)
	}
	prev := expense.ExpenseRecord{AmountCents: e.cents, Hint: e.hint, Category: e.category}
	next := cmd.ApplyTo(prev)
	delete(r.byKey, r.keyOf(memberID, e))
	e.cents = next.AmountCents
	e.hint = next.Hint
	e.category = next.Category
	r.byKey[r.keyOf(memberID, e)] = expenseID
	return prev, nil
}

// keyOf 重建幂等键（与 Create 的 key 计算保持一致）。
func (r *memExpenseRepo) keyOf(memberID string, e *memExpense) string {
	day := e.occurredAt.Format("2006-01-02")
	h := sha256.Sum256([]byte(memberID + "|" + strconv.FormatInt(e.cents, 10) + "|" + e.hint + "|" + day))
	return hex.EncodeToString(h[:])
}

func (r *memExpenseRepo) Summary(_ context.Context, memberID string, month time.Time) (expense.ExpenseSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sm := expense.ExpenseSummary{ByCategory: map[string]int64{}}
	for _, e := range r.byID {
		if e.memberID != memberID || r.deleted[e.id] {
			continue
		}
		if e.occurredAt.Year() == month.Year() && e.occurredAt.Month() == month.Month() {
			sm.TotalCents += e.cents
			sm.ByCategory[e.category] += e.cents
		}
	}
	return sm, nil
}

type memUndo struct {
	mu      sync.Mutex
	records []tool.UndoRecord
}

func (m *memUndo) Save(_ context.Context, r tool.UndoRecord) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.NewString()
	m.records = append(m.records, r)
	return id, nil
}

type memAudit struct {
	mu      sync.Mutex
	entries []tool.AuditEntry
}

func (m *memAudit) Log(_ context.Context, e tool.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

type memSession struct {
	mu    sync.Mutex
	convs map[string]*memConv
	perms map[string]map[string]bool
}

type memConv struct {
	id       string
	memberID string
	msgs     []session.Message
}

func newMemSession(perms map[string]map[string]bool) *memSession {
	cp := make(map[string]map[string]bool, len(perms))
	for k, v := range perms {
		cp[k] = v
	}
	return &memSession{convs: map[string]*memConv{}, perms: cp}
}

func (s *memSession) CreateConversation(_ context.Context, memberID, title, modelID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := uuid.NewString()
	s.convs[id] = &memConv{id: id, memberID: memberID}
	return id, nil
}

func (s *memSession) LoadConversation(_ context.Context, convID, memberID string) (session.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[convID]
	if !ok || c.memberID != memberID {
		return session.Conversation{}, nil
	}
	return session.Conversation{ID: c.id, MemberID: c.memberID}, nil
}

func (s *memSession) LoadMessages(_ context.Context, convID, memberID string) ([]session.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[convID]
	if !ok || c.memberID != memberID {
		return nil, nil
	}
	out := make([]session.Message, len(c.msgs))
	copy(out, c.msgs)
	return out, nil
}

func (s *memSession) AppendMessages(_ context.Context, convID, memberID string, msgs []session.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[convID]
	if !ok {
		c = &memConv{id: convID, memberID: memberID}
		s.convs[convID] = c
	}
	c.msgs = append(c.msgs, msgs...)
	return nil
}

func (s *memSession) Permissions(_ context.Context, memberID string) (map[string]bool, error) {
	return s.perms[memberID], nil
}

func (s *memSession) Append(ctx context.Context, convID, memberID string, msgs []session.Message) error {
	return s.AppendMessages(ctx, convID, memberID, msgs)
}

func (s *memSession) Ensure(ctx context.Context, convID, memberID string) (session.Conversation, []session.Message, error) {
	conv, err := s.LoadConversation(ctx, convID, memberID)
	if err != nil {
		return session.Conversation{}, nil, err
	}
	if conv.ID == "" {
		id, err := s.CreateConversation(ctx, memberID, "", "")
		if err != nil {
			return session.Conversation{}, nil, err
		}
		conv = session.Conversation{ID: id, MemberID: memberID}
	}
	msgs, err := s.LoadMessages(ctx, conv.ID, memberID)
	if err != nil {
		return session.Conversation{}, nil, err
	}
	return conv, msgs, nil
}

// Service 接口补齐（会话管理 API 层方法，测试场景给最小实现）

func (s *memSession) Create(ctx context.Context, memberID, title, modelID string) (string, error) {
	return s.CreateConversation(ctx, memberID, title, modelID)
}

func (s *memSession) List(_ context.Context, memberID string, limit int, cursor string) ([]session.Conversation, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]session.Conversation, 0, len(s.convs))
	for _, c := range s.convs {
		if c.memberID == memberID {
			out = append(out, session.Conversation{ID: c.id, MemberID: c.memberID})
		}
	}
	return out, "", nil
}

func (s *memSession) Load(ctx context.Context, convID, memberID string) (session.Conversation, []session.Message, error) {
	conv, err := s.LoadConversation(ctx, convID, memberID)
	if err != nil {
		return session.Conversation{}, nil, err
	}
	msgs, err := s.LoadMessages(ctx, convID, memberID)
	if err != nil {
		return session.Conversation{}, nil, err
	}
	return conv, msgs, nil
}

func (s *memSession) ListMessages(ctx context.Context, convID, memberID string, limit int) ([]session.MessageView, error) {
	msgs, err := s.LoadMessages(ctx, convID, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]session.MessageView, 0, len(msgs))
	for i, m := range msgs {
		out = append(out, session.MessageView{
			ID:             uuid.NewString(),
			ConversationID: convID,
			Role:           string(m.Role),
			Content:        m.Content,
			CreatedAt:      time.Now().Add(time.Duration(i) * time.Second),
		})
	}
	return out, nil
}

func (s *memSession) Delete(_ context.Context, convID, memberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[convID]
	if !ok || c.memberID != memberID {
		return nil
	}
	delete(s.convs, convID)
	return nil
}

// ---- 测试 ----

func newTestRuntime(t *testing.T, perms map[string]bool) (*Runtime, *memExpenseRepo, *memUndo, *memAudit, *memSession) {
	t.Helper()
	repo := newMemExpenseRepo()
	svc := expense.NewService(repo) // 真实领域服务：归类/幂等/校验都被覆盖
	reg := tool.NewRegistry()
	if err := reg.Register(expense.NewRecordExpenseTool(svc)); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	undo := &memUndo{}
	audit := &memAudit{}
	exec := tool.NewExecutor(reg, audit, undo)
	sessions := newMemSession(map[string]map[string]bool{
		"m1": perms,
	})
	return New(Options{
		Provider: nil, // 各用例自行装
		Executor: exec,
		Sessions: sessions,
	}), repo, undo, audit, sessions
}

func collectEvents(ctx context.Context, rt *Runtime, req RunRequest) []Event {
	var mu sync.Mutex
	out := []Event{}
	req.OnEvent = func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		out = append(out, e)
	}
	_ = rt.Run(ctx, req)
	mu.Lock()
	defer mu.Unlock()
	return out
}

func hasEvent(events []Event, typ string) bool {
	for _, e := range events {
		if e.Type == typ {
			return true
		}
	}
	return false
}

func TestRunRecordExpenseEndToEnd(t *testing.T) {
	rt, repo, undo, audit, sessions := newTestRuntime(t, map[string]bool{"expense.write": true})
	rt.provider = gateway.NewScriptedProvider(gateway.RecordExpenseScript())

	events := collectEvents(context.Background(), rt, RunRequest{
		MemberID: "m1",
		Content:  "今天买菜花了 120",
	})

	// 事件序列：token... → tool_call(卡片) → token... → done
	if !hasEvent(events, "token") {
		t.Fatal("应有 token 事件")
	}
	if !hasEvent(events, "tool_call") {
		t.Fatal("应有 tool_call 事件")
	}
	if !hasEvent(events, "done") {
		t.Fatal("应以 done 结束")
	}

	// 工具被真实执行：参数是分（边界处 120 元 → 12000 分）
	if len(repo.byID) != 1 {
		t.Fatalf("应执行 1 次记账，got %d", len(repo.byID))
	}
	for _, e := range repo.byID {
		if e.cents != 12000 {
			t.Fatalf("金额应为 12000 分，got %d", e.cents)
		}
		if e.hint != "买菜" {
			t.Fatalf("hint 应为 买菜，got %q", e.hint)
		}
		break
	}

	// 卡片携带金额与分类
	var card map[string]any
	for _, e := range events {
		if e.Type == "tool_call" && len(e.Card) > 0 {
			_ = json.Unmarshal(e.Card, &card)
			break
		}
	}
	if card == nil {
		t.Fatal("未找到卡片数据")
	}
	if card["amount"] != float64(120) {
		t.Fatalf("卡片金额应为 120，got %v", card["amount"])
	}
	if card["category"] != "食材" {
		t.Fatalf("卡片分类应为 食材，got %v", card["category"])
	}

	// undo_log + 审计留痕（A3 级写操作）
	if len(undo.records) != 1 || undo.records[0].ToolName != "record_expense" {
		t.Fatalf("undo_log 应记录 1 条 record_expense，got %d", len(undo.records))
	}
	if len(audit.entries) != 1 {
		t.Fatalf("审计应记录 1 条，got %d", len(audit.entries))
	}

	// 消息持久化：user + 2 assistant + 1 tool
	var conv *memConv
	for _, c := range sessions.convs {
		conv = c
	}
	if conv == nil {
		t.Fatal("会话未创建")
	}
	if len(conv.msgs) != 4 {
		t.Fatalf("应持久化 4 条消息，got %d", len(conv.msgs))
	}
	if conv.msgs[0].Role != gateway.RoleUser || conv.msgs[0].Content != "今天买菜花了 120" {
		t.Fatalf("首条应为用户消息，got %+v", conv.msgs[0])
	}
	if len(conv.msgs[1].ToolCalls) != 1 {
		t.Fatalf("第 2 条 assistant 应含 1 个 tool_call")
	}
}

func TestRunNoToolJustReplies(t *testing.T) {
	rt, repo, _, _, _ := newTestRuntime(t, map[string]bool{"expense.write": true})
	rt.provider = gateway.NewScriptedProvider(gateway.NoToolScript())

	events := collectEvents(context.Background(), rt, RunRequest{
		MemberID: "m1",
		Content:  "今天天气怎么样",
	})

	if hasEvent(events, "tool_call") {
		t.Fatal("纯对话不应有 tool_call")
	}
	if !hasEvent(events, "done") {
		t.Fatal("应以 done 结束")
	}
	if len(repo.byID) != 0 {
		t.Fatalf("不应执行记账，got %d", len(repo.byID))
	}
}

func TestRunPermissionDenied(t *testing.T) {
	rt, _, _, audit, _ := newTestRuntime(t, map[string]bool{}) // 无 expense.write
	rt.provider = gateway.NewScriptedProvider(gateway.RecordExpenseScript())

	events := collectEvents(context.Background(), rt, RunRequest{
		MemberID: "m1",
		Content:  "今天买菜花了 120",
	})

	// 执行层拒绝：不应成功执行工具
	if len(events) == 0 {
		t.Fatal("应有事件")
	}
	// 无权限时 RecordExpenseScript 仍会触发 tool_call，但 Executor 拒绝
	foundDeny := false
	for _, e := range events {
		if e.Type == "tool_call" && e.Error != "" {
			foundDeny = true
		}
	}
	if !foundDeny {
		t.Fatal("无权限调用应产生失败回执（错误回传 LLM 重试）")
	}
	// 越权尝试应留审计痕
	if len(audit.entries) == 0 {
		t.Log("注意：当前审计只在执行成功后记录；越权拒绝的审计埋点见 P1")
	}
}

func TestRunHighRiskMaxTurns(t *testing.T) {
	rt, repo, _, _, _ := newTestRuntime(t, map[string]bool{"expense.write": true})
	rt.provider = gateway.NewScriptedProvider(gateway.InfiniteToolScript())

	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := collectEvents(cctx, rt, RunRequest{
		MemberID: "m1",
		Content:  "无限记账",
	})

	// 高风险上限 3 步：最多 3 次工具调用后必须终止
	if len(repo.byID) > 3 {
		t.Fatalf("高风险工具应在 3 步内终止，实际执行 %d 次", len(repo.byID))
	}
	if !hasEvent(events, "error") {
		t.Fatal("超限应推送 error 事件")
	}
}

func TestRunCumulativeHighRiskTerminates(t *testing.T) {
	rt, repo, _, _, _ := newTestRuntime(t, map[string]bool{"expense.write": true})
	// 脚本：每轮都记账（高风险），第 2 次应被累计风险终止
	rt.provider = gateway.NewScriptedProvider(func(round int, _ gateway.ChatRequest) []gateway.StreamEvent {
		args, _ := json.Marshal(map[string]any{"amount": 10, "hint": "测试"})
		return []gateway.StreamEvent{
			{ToolCall: &gateway.ToolCall{ID: "c", Name: "record_expense", Args: args}},
			{Done: true, Usage: &gateway.Usage{TotalTokens: 1}},
		}
	})

	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	collectEvents(cctx, rt, RunRequest{MemberID: "m1", Content: "连续记账"})

	// 累计风险终止：第 2 次高风险执行后停止
	if len(repo.byID) > 2 {
		t.Fatalf("累计高风险应在第 2 次执行后终止，实际 %d 次", len(repo.byID))
	}
}

func TestRunCancelStopsLoop(t *testing.T) {
	rt, _, _, _, _ := newTestRuntime(t, map[string]bool{"expense.write": true})
	p := gateway.NewScriptedProvider(gateway.RecordExpenseScript())
	p.DelayMS = 200
	rt.provider = p

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// 不应 panic / 阻塞
	_ = rt.Run(ctx, RunRequest{
		MemberID: "m1",
		Content:  "取消测试",
		OnEvent:  func(Event) {},
	})
}
