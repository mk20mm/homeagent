package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// Registry 是工具注册表，支持热插拔（新模块工具直接挂载，不改调度器核心）。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register 注册工具。写工具必须实现 Undo，否则报错（编译期约束的运行时兜底）。
func (r *Registry) Register(t Tool) error {
	spec := t.Spec()
	if spec.Name == "" {
		return apperr.New(apperr.CodeInternal, "工具名不能为空", nil)
	}
	if spec.Risk != RiskLow {
		if _, ok := t.(WriteTool); !ok {
			return apperr.Newf(apperr.CodeInternal,
				fmt.Sprintf("写操作工具 %s 必须实现 Undo（ADR-004）", spec.Name), nil)
		}
		if spec.Idempotency == "" {
			return apperr.Newf(apperr.CodeInternal,
				fmt.Sprintf("写操作工具 %s 必须声明幂等维度", spec.Name), nil)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[spec.Name]; exists {
		return apperr.Newf(apperr.CodeInternal,
			fmt.Sprintf("工具 %s 已注册", spec.Name), nil)
	}
	r.tools[spec.Name] = t
	return nil
}

// Get 按名取工具。
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// SpecsWithPermission 返回当前用户权限内的工具声明，供提示词构建注入（ADR-005 源头过滤）。
// permitted 是该用户已开启的权限键集合。
func (r *Registry) SpecsWithPermission(permitted map[string]bool) []Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Spec, 0, len(r.tools))
	for _, t := range r.tools {
		s := t.Spec()
		if permitted[s.Permission] {
			out = append(out, s)
		}
	}
	return out
}

// Executor 统一包办执行链路：权限校验→参数校验→执行→undo_log→审计。
// 工具实现不得自己跳过这条链（docs/CONVENTIONS-backend.md §3）。
type Executor struct {
	registry *Registry
	audit    AuditLogger
	undo     UndoStore
}

type AuditLogger interface {
	Log(ctx context.Context, entry AuditEntry) error
}

type AuditEntry struct {
	TraceID          string
	MemberID         string
	ToolName         string
	Risk             RiskLevel
	Params           json.RawMessage
	Result           string
	Undone           bool
	PermissionDenied bool // 越权尝试记录（安全否决项 Critical）
	LatencyMS        int
}

type UndoStore interface {
	Save(ctx context.Context, key UndoRecord) error
}

type UndoRecord struct {
	TraceID   string
	MemberID  string
	ToolName  string
	UndoData  json.RawMessage
	ExpiresIn int64 // 秒
}

func NewExecutor(reg *Registry, audit AuditLogger, undo UndoStore) *Executor {
	return &Executor{registry: reg, audit: audit, undo: undo}
}

// Execute 执行工具，链路顺序固定不可跳过。
// permitted 为 nil 表示跳过权限校（仅用于内部直调，Agent 路径必须传入）。
func (e *Executor) Execute(ctx context.Context, name string, input json.RawMessage, memberID string, permitted map[string]bool) (Result, error) {
	t, ok := e.registry.Get(name)
	if !ok {
		return Result{}, apperr.Newf(apperr.CodeNotFound, "工具不存在: "+name, nil)
	}

	spec := t.Spec()

	// 1. 权限校验（执行层兜底；提示词层已做源头过滤）
	if permitted != nil && !permitted[spec.Permission] {
		// 越权尝试留痕（安全否决项：无授权写入被执行 → Critical）
		if e.audit != nil {
			_ = e.audit.Log(ctx, AuditEntry{
				TraceID: traceIDFrom(ctx), MemberID: memberID,
				ToolName: name, Risk: spec.Risk, PermissionDenied: true,
			})
		}
		return Result{}, apperr.Newf(apperr.CodePermission,
			"无权限调用此工具: "+name, nil)
	}

	// 2. 参数校验（可信代码校验，不依赖 LLM 自查）
	if err := validateInput(spec, input); err != nil {
		return Result{}, err
	}

	// 3. 执行（注入 memberID/trace_id，供领域工具从 ctx 取用）
	ctx = WithMemberID(ctx, memberID)
	start := time.Now()
	res, err := t.Execute(ctx, input)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{}, err
	}

	// 4. 写 undo_log（写操作；失败不阻塞结果，仅记日志）
	if len(res.UndoData) > 0 && e.undo != nil {
		_ = e.undo.Save(ctx, UndoRecord{
			TraceID: traceIDFrom(ctx), MemberID: memberID,
			ToolName: name, UndoData: res.UndoData, ExpiresIn: 86400,
		})
	}

	// 5. 写审计
	if e.audit != nil {
		_ = e.audit.Log(ctx, AuditEntry{
			TraceID: traceIDFrom(ctx), MemberID: memberID,
			ToolName: name, Risk: spec.Risk, Params: input, Result: res.Summary,
			LatencyMS: int(latency),
		})
	}

	return res, nil
}

// Undo 撤销写操作。
func (e *Executor) Undo(ctx context.Context, name string, undoData json.RawMessage, memberID string) error {
	t, ok := e.registry.Get(name)
	if !ok {
		return apperr.Newf(apperr.CodeNotFound, "工具不存在: "+name, nil)
	}
	wt, ok := t.(WriteTool)
	if !ok {
		return apperr.Newf(apperr.CodeInternal, "工具 "+name+" 不支持撤销", nil)
	}
	if err := wt.Undo(ctx, undoData); err != nil {
		return err
	}
	if e.audit != nil {
		_ = e.audit.Log(ctx, AuditEntry{
			TraceID:  traceIDFrom(ctx),
			MemberID: memberID,
			ToolName: name, Risk: t.Spec().Risk, Undone: true,
		})
	}
	return nil
}

// Registry 返回执行器持有的注册表（供 API 层查询工具清单）。
func (e *Executor) Registry() *Registry { return e.registry }

type ctxKey string

const traceIDKey ctxKey = "trace_id"
const memberIDKey ctxKey = "member_id"

func traceIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// MemberIDFrom 工具从 ctx 取成员 ID（由 Executor 统一注入）。
func MemberIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(memberIDKey).(string); ok {
		return v
	}
	return ""
}

// WithTraceID 把 trace_id 塞进 ctx，供审计/日志关联。
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// TraceIDFrom 从 ctx 取 trace_id（仓储层埋点用）。
func TraceIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// WithMemberID 把成员 ID 塞进 ctx（Executor 执行工具前注入，供领域工具取用）。
func WithMemberID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, memberIDKey, id)
}
