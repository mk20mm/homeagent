package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/auditlog"
	"github.com/mk20mm/homeagent/internal/store/ent/llmusage"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// AuditQuery 审计查询参数。
type AuditQuery struct {
	PageSize         int
	Cursor           string
	ToolName         string
	PermissionDenied bool
}

// 编译期：实现 tool.AuditLogger。
var _ tool.AuditLogger = (*Store)(nil)

// Log 写审计日志（只增不改，撤销也留痕）。
func (s *Store) Log(ctx context.Context, e tool.AuditEntry) error {
	params := map[string]any{}
	if len(e.Params) > 0 {
		if err := json.Unmarshal(e.Params, &params); err != nil {
			params = map[string]any{"raw": string(e.Params)}
		}
	}
	b := s.db.AuditLog.Create().
		SetTraceID(e.TraceID).
		SetToolName(e.ToolName).
		SetParams(params).
		SetUndone(e.Undone).
		SetPermissionDenied(e.PermissionDenied).
		SetMemberID(toUUID(e.MemberID))
	if e.Result != "" {
		b.SetResult(e.Result)
	}
	// risk 是 schema 必填 enum，缺失会导致写库静默失败（撤销审计曾踩过）
	risk := e.Risk
	if risk == "" {
		risk = tool.RiskLow
	}
	b.SetRisk(auditlog.Risk(risk))
	if _, err := b.Save(ctx); err != nil {
		return apperr.New(apperr.CodeInternal, "写审计日志失败", err)
	}
	return nil
}

// ListAudit 审计列表（游标分页 + 过滤），返回下一页游标。
func (s *Store) ListAudit(ctx context.Context, memberID string, q AuditQuery) ([]tool.AuditEntry, string, error) {
	query := s.db.AuditLog.Query().
		Where(auditlog.HasMemberWith(member.IDEQ(toUUID(memberID))))
	if q.PermissionDenied {
		query = query.Where(auditlog.PermissionDeniedEQ(true))
	}
	if q.ToolName != "" {
		query = query.Where(auditlog.ToolNameEQ(q.ToolName))
	}
	if q.Cursor != "" {
		if t, err := time.Parse(time.RFC3339, q.Cursor); err == nil {
			query = query.Where(auditlog.CreatedAtLT(t))
		}
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}

	list, err := query.
		Order(ent.Desc(auditlog.FieldCreatedAt)).
		Limit(q.PageSize + 1).
		All(ctx)
	if err != nil {
		return nil, "", apperr.New(apperr.CodeInternal, "查询审计日志失败", err)
	}

	nextCursor := ""
	if len(list) > q.PageSize {
		nextCursor = list[q.PageSize-1].CreatedAt.Format(time.RFC3339)
		list = list[:q.PageSize]
	}

	out := make([]tool.AuditEntry, 0, len(list))
	for _, l := range list {
		out = append(out, tool.AuditEntry{
			TraceID:  l.TraceID,
			ToolName: l.ToolName,
			Risk:     tool.RiskLevel(l.Risk),
			Result:   l.Result,
			Undone:   l.Undone,
		})
	}
	return out, nextCursor, nil
}

// 编译期：实现 gateway.UsageRecorder。
var _ gateway.UsageRecorder = (*Store)(nil)

// Record 写 LLM 用量（网关出口埋点，ARCHITECTURE §10）。
func (s *Store) Record(ctx context.Context, modelID, model, provider string, u gateway.Usage, latencyMS int64) error {
	create := s.db.LLMUsage.Create().
		SetTraceID(tool.TraceIDFrom(ctx)).
		SetPromptTokens(u.PromptTokens).
		SetCompletionTokens(u.CompletionTokens).
		SetStatus(llmusage.StatusOk).
		SetLatencyMs(int(latencyMS))
	if modelID != "" {
		create.SetModelID(toUUID(modelID))
	}
	// cost 字段 schema 为 float（元），暂记 0；技术债 T10 统一改 int64 分后由单价计算
	create.SetCost(0)
	if _, err := create.Save(ctx); err != nil {
		return apperr.New(apperr.CodeInternal, "记录用量失败", err)
	}
	return nil
}

func toUUIDPtr(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id := toUUID(s)
	return &id
}
