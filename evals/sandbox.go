// Package evals 是金标准评测套件（AI-PRD §6 / AI-STD-005）。
//
// 设计（AI-PRD §6.3「评测方式」）：
//   - 优先 Code/Rule Grader：参数 JSON Schema 校验、数据库终态断言、权限断言、Tool Presence
//   - 有副作用的用例不靠重复真实执行：走沙箱——真实工具层 + 真实领域服务 + 内存仓储，
//     LLM 由 ScriptedProvider 模拟（脚本即「LLM 应当给出的工具调用」），不真调供应商
//   - LLM-as-Judge 与人工抽检不在本套件；开放性回复质量留待真实供应商接入后另行评测
//
// 沙箱即「评测专用依赖装配」：和 cmd/homeagent 的差别只有仓储换成了内存实现、
// Provider 换成了脚本。被测对象（工具层/执行器链路/领域服务/调度器）一行没改。
package evals

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/runtime"
	"github.com/mk20mm/homeagent/internal/agent/session"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/expense"
	"github.com/mk20mm/homeagent/internal/domain/meal"
	"github.com/mk20mm/homeagent/internal/domain/model"
	"github.com/mk20mm/homeagent/internal/domain/task"
	"github.com/mk20mm/homeagent/internal/domain/undo"
)

// ---- 内存仓储 ----
//
// 语义与 internal/store/repo 对齐（幂等冲突码、成员隔离、软删除），
// 但不依赖 SQLite——evals 要在无库、无网络、无 gcc 的环境确定可跑。
// 若仓储语义与真实实现漂移，单测会先暴露（repo 层有独立单测覆盖）。

// memExpenseRepo 账单仓储（对齐 repo/expense.go 语义）。
type memExpenseRepo struct {
	mu      sync.Mutex
	byID    map[string]*memExpense
	byKey   map[string]string // 幂等键 -> id
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

func (r *memExpenseRepo) Create(_ context.Context, memberID string, cmd expense.RecordExpenseCmd, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.byKey[key]; ok {
		return id, apperr.New(apperr.CodeConflict, "账单已存在", nil)
	}
	id := uuid.NewString()
	r.byID[id] = &memExpense{
		id:         id,
		memberID:   memberID,
		cents:      cmd.AmountCents,
		hint:       cmd.Hint,
		category:   cmd.Category,
		occurredAt: cmd.OccurredOrNow(),
	}
	r.byKey[key] = id
	return id, nil
}

func (r *memExpenseRepo) Update(_ context.Context, memberID, expenseID string, cmd expense.UpdateExpenseCmd) (expense.ExpenseRecord, error) {
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

func (r *memExpenseRepo) keyOf(memberID string, e *memExpense) string {
	day := e.occurredAt.Format("2006-01-02")
	h := sha256.Sum256([]byte(memberID + "|" + strconv.FormatInt(e.cents, 10) + "|" + e.hint + "|" + day))
	return hex.EncodeToString(h[:])
}

func (r *memExpenseRepo) Delete(_ context.Context, expenseID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted[expenseID] = true
	return nil
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

// activeExpenses 未删除账单（终态断言用）。
func (r *memExpenseRepo) activeExpenses() []memExpense {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]memExpense, 0, len(r.byID))
	for _, e := range r.byID {
		if !r.deleted[e.id] {
			out = append(out, *e)
		}
	}
	return out
}

// memTaskRepo 家务仓储（对齐 repo/task.go 语义：状态机、归属、幂等）。
type memTaskRepo struct {
	mu      sync.Mutex
	byID    map[string]*memTask
	byKey   map[string]string // 幂等键 -> id
	members map[string]string // name -> id（派发按名解析执行人）
}

type memTask struct {
	id          string
	title       string
	description string
	risk        string
	status      task.TaskStatus
	assignerID  string
	assigneeID  string
	dueAt       *time.Time
	completedAt *time.Time
	deleted     bool
}

func newMemTaskRepo(members map[string]string) *memTaskRepo {
	cp := make(map[string]string, len(members))
	for k, v := range members {
		cp[k] = v
	}
	return &memTaskRepo{
		byID:    map[string]*memTask{},
		byKey:   map[string]string{},
		members: cp,
	}
}

func (r *memTaskRepo) Assign(_ context.Context, assignerID string, cmd task.AssignTaskCmd, idempotencyKey string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.byKey[idempotencyKey]; ok {
		return id, apperr.New(apperr.CodeConflict, "任务已存在", nil)
	}
	if cmd.AssigneeName != "" {
		if _, ok := r.members[cmd.AssigneeName]; !ok {
			return "", apperr.New(apperr.CodeNotFound, "执行人「"+cmd.AssigneeName+"」不存在", nil)
		}
	}
	id := uuid.NewString()
	t := &memTask{
		id:          id,
		title:       cmd.Title,
		description: cmd.Description,
		risk:        cmd.Risk,
		status:      task.StatusPending,
		assignerID:  assignerID,
	}
	if cmd.Risk == "" {
		t.risk = "medium"
	}
	if cmd.AssigneeName != "" {
		t.assigneeID = r.members[cmd.AssigneeName]
	}
	if !cmd.DueAt.IsZero() {
		d := cmd.DueAt
		t.dueAt = &d
	}
	r.byID[id] = t
	r.byKey[idempotencyKey] = id
	return id, nil
}

func (r *memTaskRepo) Complete(_ context.Context, taskID, memberID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[taskID]
	if !ok || t.deleted {
		return apperr.New(apperr.CodeNotFound, "任务不存在", nil)
	}
	if t.assigneeID != "" && t.assigneeID != memberID {
		return apperr.New(apperr.CodePermission, "这不是指派给你的任务", nil)
	}
	if t.status == task.StatusDone {
		return apperr.New(apperr.CodeConflict, "任务已完成，勿重复打卡", nil)
	}
	switch t.status {
	case task.StatusPending:
		t.status = task.StatusInProgress
	case task.StatusInProgress:
		t.status = task.StatusDone
		now := time.Now()
		t.completedAt = &now
	}
	return nil
}

func (r *memTaskRepo) Uncomplete(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[taskID]
	if !ok {
		return apperr.New(apperr.CodeNotFound, "任务不存在", nil)
	}
	t.status = task.StatusPending
	t.completedAt = nil
	return nil
}

func (r *memTaskRepo) ListMyTasks(_ context.Context, memberID string) ([]task.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]task.Task, 0, len(r.byID))
	for _, t := range r.byID {
		if t.deleted || t.assigneeID != memberID || t.status == task.StatusDone {
			continue
		}
		out = append(out, memToTask(t))
	}
	return out, nil
}

func (r *memTaskRepo) GetTask(_ context.Context, taskID string) (task.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[taskID]
	if !ok || t.deleted {
		return task.Task{}, apperr.New(apperr.CodeNotFound, "任务不存在", nil)
	}
	return memToTask(t), nil
}

func (r *memTaskRepo) Remove(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[taskID]
	if !ok {
		return apperr.New(apperr.CodeNotFound, "任务不存在", nil)
	}
	t.deleted = true
	return nil
}

func memToTask(t *memTask) task.Task {
	return task.Task{
		ID:          t.id,
		Title:       t.title,
		Description: t.description,
		Risk:        t.risk,
		Status:      t.status,
		AssigneeID:  t.assigneeID,
		DueAt:       t.dueAt,
		CompletedAt: t.completedAt,
	}
}

// memMealRepo 报饭仓储（对齐 repo/meal.go：人+日 upsert）。
type memMealRepo struct {
	mu      sync.Mutex
	byID    map[string]*memMeal
	members map[string]string // id -> name（汇总要名字）
}

type memMeal struct {
	id       string
	memberID string
	date     time.Time
	atHome   bool
	note     string
}

func newMemMealRepo(members map[string]string) *memMealRepo {
	cp := make(map[string]string, len(members))
	for k, v := range members {
		cp[k] = v
	}
	return &memMealRepo{byID: map[string]*memMeal{}, members: cp}
}

func (r *memMealRepo) Upsert(_ context.Context, memberID string, cmd meal.ReportCmd) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	day := cmd.Date
	if day.IsZero() {
		day = meal.Today()
	}
	for _, m := range r.byID {
		if m.memberID == memberID && sameDay(m.date, day) {
			m.atHome = cmd.AtHome
			m.note = cmd.Note
			return m.id, nil
		}
	}
	id := uuid.NewString()
	r.byID[id] = &memMeal{id: id, memberID: memberID, date: day, atHome: cmd.AtHome, note: cmd.Note}
	return id, nil
}

func (r *memMealRepo) RemoveReport(_ context.Context, memberID string, date time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.byID {
		if m.memberID == memberID && sameDay(m.date, date) {
			delete(r.byID, m.id)
			return nil
		}
	}
	return apperr.New(apperr.CodeNotFound, "当日报饭记录不存在", nil)
}

func (r *memMealRepo) DailySummary(_ context.Context, date time.Time) (meal.MealSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := meal.MealSummary{Date: date}
	reported := map[string]bool{}
	for _, m := range r.byID {
		if !sameDay(m.date, date) {
			continue
		}
		name := r.members[m.memberID]
		mm := meal.MealMember{ID: m.memberID, Name: name}
		reported[m.memberID] = true
		if m.atHome {
			out.AtHome = append(out.AtHome, mm)
		} else {
			out.NotAtHome = append(out.NotAtHome, mm)
		}
	}
	for id, name := range r.members {
		if reported[id] {
			continue
		}
		out.Unreported = append(out.Unreported, meal.MealMember{ID: id, Name: name})
	}
	out.Total = len(out.AtHome) + len(out.NotAtHome)
	return out, nil
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// memModelRepo 模型仓储（对齐 repo/model.go：切换/列表）。
type memModelRepo struct {
	mu     sync.Mutex
	models []model.ModelInfo
	conv   map[string]string // convID -> modelID
}

func newMemModelRepo() *memModelRepo {
	return &memModelRepo{
		models: []model.ModelInfo{
			{ID: "m-deepseek", ModelName: "deepseek-chat", DisplayName: "DeepSeek 对话", Provider: "deepseek", IsDefault: true, Enabled: true},
			{ID: "m-gpt4o", ModelName: "gpt-4o", DisplayName: "GPT-4o", Provider: "openai", Enabled: true},
		},
		conv: map[string]string{},
	}
}

func (r *memModelRepo) ListEnabled(_ context.Context) ([]model.ModelInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.ModelInfo, 0, len(r.models))
	for _, m := range r.models {
		if m.Enabled {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *memModelRepo) ListAll(_ context.Context) ([]model.ModelInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.ModelInfo, 0, len(r.models))
	for _, m := range r.models {
		out = append(out, m)
	}
	return out, nil
}

func (r *memModelRepo) FindModel(_ context.Context, name string) (model.ModelInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	want := normalizeModelName(name)
	for _, m := range r.models {
		if m.ID == name || normalizeModelName(m.ModelName) == want || normalizeModelName(m.DisplayName) == want {
			return m, nil
		}
	}
	return model.ModelInfo{}, apperr.New(apperr.CodeNotFound, "模型不存在或未启用", nil)
}

func (r *memModelRepo) SetConversationModel(_ context.Context, convID, modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conv[convID] = modelID
	return nil
}

func (r *memModelRepo) GetConversationModel(_ context.Context, convID string) (model.ModelInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.conv[convID]; ok {
		for _, m := range r.models {
			if m.ID == id {
				return m, nil
			}
		}
	}
	for _, m := range r.models {
		if m.IsDefault {
			return m, nil
		}
	}
	return model.ModelInfo{}, apperr.New(apperr.CodeNotFound, "模型不存在或未启用", nil)
}

func normalizeModelName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// memUndoLog 撤销记录仓储（对齐 repo/undolog.go：24h 窗口、已用不返回）。
type memUndoLog struct {
	mu      sync.Mutex
	records []*memUndoRecord
}

type memUndoRecord struct {
	id        string
	memberID  string
	toolName  string
	undoData  json.RawMessage
	expiresAt time.Time
	used      bool
	createdAt time.Time
}

func newMemUndoLog() *memUndoLog {
	return &memUndoLog{}
}

func (m *memUndoLog) Save(_ context.Context, r tool.UndoRecord) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp := time.Now()
	if r.ExpiresIn > 0 {
		exp = exp.Add(time.Duration(r.ExpiresIn) * time.Second)
	} else {
		exp = exp.Add(24 * time.Hour)
	}
	rec := &memUndoRecord{
		id:        uuid.NewString(),
		memberID:  r.MemberID,
		toolName:  r.ToolName,
		undoData:  r.UndoData,
		expiresAt: exp,
		createdAt: time.Now(),
	}
	m.records = append(m.records, rec)
	return rec.id, nil
}

// ListActive 对话撤销用：未使用且未过期，新的在前（对齐 repo.ListActive 排序）。
func (m *memUndoLog) ListActive(_ context.Context, memberID string) ([]undo.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]undo.Record, 0, len(m.records))
	for _, r := range m.records {
		if r.memberID != memberID || r.used || r.expiresAt.Before(time.Now()) {
			continue
		}
		out = append(out, undo.Record{
			ID:        r.id,
			ToolName:  r.toolName,
			UndoData:  r.undoData,
			ExpiresAt: r.expiresAt,
		})
	}
	// 倒序：最近在前
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (m *memUndoLog) MarkUsed(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.records {
		if r.id == id {
			r.used = true
			return nil
		}
	}
	return apperr.New(apperr.CodeNotFound, "撤销记录不存在", nil)
}

// activeRecords 未使用的撤销记录（终态断言用）。
func (m *memUndoLog) activeRecords() []memUndoRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]memUndoRecord, 0, len(m.records))
	for _, r := range m.records {
		if !r.used {
			out = append(out, *r)
		}
	}
	return out
}

// memAudit 审计日志（对齐 repo/audit.go：越权尝试留 PermissionDenied 痕）。
type memAudit struct {
	mu      sync.Mutex
	entries []tool.AuditEntry
}

func newMemAudit() *memAudit {
	return &memAudit{}
}

func (a *memAudit) Log(_ context.Context, e tool.AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
	return nil
}

func (a *memAudit) denied() []tool.AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]tool.AuditEntry, 0, len(a.entries))
	for _, e := range a.entries {
		if e.PermissionDenied {
			out = append(out, e)
		}
	}
	return out
}

// memSession 会话服务（对齐 repo/conversation.go：member 隔离 + 只增消息）。
type memSession struct {
	mu    sync.Mutex
	convs map[string]*memConv
	perms map[string]map[string]bool // memberID -> 权限集合
}

type memConv struct {
	id       string
	memberID string
	msgs     []session.Message
}

func newMemSession(perms map[string]map[string]bool) *memSession {
	cp := make(map[string]map[string]bool, len(perms))
	for k, v := range perms {
		inner := make(map[string]bool, len(v))
		for pk, pv := range v {
			inner[pk] = pv
		}
		cp[k] = inner
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

func (s *memSession) ListConversations(_ context.Context, memberID string, _ int, _ string) ([]session.Conversation, string, error) {
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

func (s *memSession) DeleteConversation(_ context.Context, convID, memberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[convID]
	if !ok || c.memberID != memberID {
		return nil
	}
	delete(s.convs, convID)
	return nil
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

func (s *memSession) LoadMessageList(_ context.Context, convID, memberID string, _ int) ([]session.MessageView, error) {
	msgs, err := s.LoadMessages(nil, convID, memberID)
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

// Service 接口方法（handler 层用，沙箱给最小实现）

func (s *memSession) Create(ctx context.Context, memberID, title, modelID string) (string, error) {
	return s.CreateConversation(ctx, memberID, title, modelID)
}

func (s *memSession) List(ctx context.Context, memberID string, limit int, cursor string) ([]session.Conversation, string, error) {
	return s.ListConversations(ctx, memberID, limit, cursor)
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
	return s.LoadMessageList(ctx, convID, memberID, limit)
}

func (s *memSession) Delete(ctx context.Context, convID, memberID string) error {
	return s.DeleteConversation(ctx, convID, memberID)
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

// 编译期接口检查：仓储实现必须满足领域接口，否则沙箱装配编译失败。
var (
	_ expense.ExpenseRepo = (*memExpenseRepo)(nil)
	_ task.TaskRepo       = (*memTaskRepo)(nil)
	_ meal.MealRepo       = (*memMealRepo)(nil)
	_ model.ModelRepo     = (*memModelRepo)(nil)
	_ tool.UndoStore      = (*memUndoLog)(nil)
	_ undo.Store          = (*memUndoLog)(nil)
	_ tool.AuditLogger    = (*memAudit)(nil)
	_ session.Repository  = (*memSession)(nil)
	_ session.Service     = (*memSession)(nil)
)

// Sandbox 评测沙箱：持有全部被测依赖的内存实现。
//
// 每个用例 newSandbox() 一份，用例间状态隔离；Runtime 与真实服务装配一致。
type Sandbox struct {
	Expenses *memExpenseRepo
	Tasks    *memTaskRepo
	Meals    *memMealRepo
	Models   *memModelRepo
	UndoLog  *memUndoLog
	Audit    *memAudit
	Sessions *memSession
	Exec     *tool.Executor
	Registry *tool.Registry
}

// sandboxMembers 沙箱成员（对齐种子数据：家长/老人/小孩三种权限模板）。
type sandboxMember struct {
	id    string
	name  string
	perms map[string]bool
}

// sandboxMembers 返回 evals 用成员：m-parent / m-elder / m-child。
// 权限矩阵对齐 store.MustSeed（ADR-005 权限双保险的测试锚点）。
// 返回 (成员列表, id->名)。
func sandboxMembers() ([]sandboxMember, map[string]string) {
	list := []sandboxMember{
		{
			id:   "m-parent",
			name: "爸爸",
			perms: map[string]bool{
				"expense.write": true, "expense.read": true,
				"task.write": true, "task.read": true,
				"meal.write": true, "system.admin": true,
			},
		},
		{
			id:   "m-elder",
			name: "奶奶",
			perms: map[string]bool{
				"expense.write": true, "expense.read": true,
				"task.read": true, "meal.write": true,
			},
		},
		{
			id:   "m-child",
			name: "孩子",
			perms: map[string]bool{
				"task.read": true, "meal.write": true,
			},
		},
	}
	idByName := make(map[string]string, len(list))
	for _, m := range list {
		idByName[m.id] = m.name
	}
	return list, idByName
}

func mustMembers() []sandboxMember {
	list, _ := sandboxMembers()
	return list
}

// memberIDByName 成员名 -> id（用例按名字引用成员更可读）。
func memberIDByName(name string) string {
	for _, m := range mustMembers() {
		if m.name == name {
			return m.id
		}
	}
	panic("unknown member: " + name)
}

// newSandbox 装配一个全内存评测环境（工具层与领域服务均为生产行为）。
func newSandbox() *Sandbox {
	_, idByName := sandboxMembers()
	byName := make(map[string]string, len(idByName))
	for id, name := range idByName {
		byName[name] = id
	}
	exp := newMemExpenseRepo()
	tasks := newMemTaskRepo(byName)
	meals := newMemMealRepo(idByName)
	models := newMemModelRepo()
	undoLog := newMemUndoLog()
	audit := newMemAudit()

	expenseSvc := expense.NewService(exp)
	taskSvc := task.NewService(tasks)
	mealSvc := meal.NewService(meals)
	modelSvc := model.NewService(models, nil) // ProviderRepo 仅 admin 配置用，evals 不测

	registry := tool.NewRegistry()
	undoLast := undo.NewUndoLastTool(undoLog)
	for _, t := range []tool.Tool{
		expense.NewRecordExpenseTool(expenseSvc),
		expense.NewQueryBudgetTool(expenseSvc),
		expense.NewUpdateExpenseTool(expenseSvc),
		task.NewAssignTaskTool(taskSvc),
		task.NewCompleteTaskTool(taskSvc),
		task.NewListMyTasksTool(taskSvc),
		meal.NewReportMealTool(mealSvc),
		meal.NewSuggestDinnerTool(mealSvc),
		model.NewListModelsTool(modelSvc),
		model.NewSwitchModelTool(modelSvc),
		undoLast,
	} {
		if err := registry.Register(t); err != nil {
			panic(fmt.Sprintf("register tool %s: %v", t.Spec().Name, err))
		}
	}
	exec := tool.NewExecutor(registry, audit, undoLog)
	undoLast.SetExecutor(exec)

	perms := map[string]map[string]bool{}
	for _, m := range mustMembers() {
		perms[m.id] = m.perms
	}

	return &Sandbox{
		Expenses: exp,
		Tasks:    tasks,
		Meals:    meals,
		Models:   models,
		UndoLog:  undoLog,
		Audit:    audit,
		Sessions: newMemSession(perms),
		Exec:     exec,
		Registry: registry,
	}
}

// newRuntime 用脚本供应商装配 Runtime（脚本即「LLM 应当给出的工具调用」）。
func (s *Sandbox) newRuntime(script func(int, gateway.ChatRequest) []gateway.StreamEvent) *runtime.Runtime {
	return runtime.New(runtime.Options{
		Provider: gateway.NewScriptedProvider(script),
		Executor: s.Exec,
		Sessions: s.Sessions,
	})
}
