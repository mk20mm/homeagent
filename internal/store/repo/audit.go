package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/auditlog"
	"github.com/mk20mm/homeagent/internal/store/ent/llmusage"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

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
		SetLatencyMs(e.LatencyMS).
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
func (s *Store) ListAudit(ctx context.Context, memberID string, q v1.AuditQuery) ([]v1.AuditItem, string, error) {
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

	out := make([]v1.AuditItem, 0, len(list))
	for _, l := range list {
		out = append(out, v1.AuditItem{
			ID:               l.ID.String(),
			CreatedAt:        l.CreatedAt,
			ToolName:         l.ToolName,
			Risk:             string(l.Risk),
			Result:           l.Result,
			LatencyMS:        l.LatencyMs,
			Undone:           l.Undone,
			PermissionDenied: l.PermissionDenied,
			TraceID:          l.TraceID,
		})
	}
	return out, nextCursor, nil
}

// UsageSummary 近 N 日按日汇总 LLM 用量（家庭维度，管理端仪表盘）。
// 按日聚合在应用层完成：单家庭数据量小，避免 SQLite 按日期函数分组方言差异。
func (s *Store) UsageSummary(ctx context.Context, days int) (v1.UsageSummary, error) {
	if days <= 0 {
		days = 7
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(days - 1))

	rows, err := s.db.LLMUsage.Query().
		Where(llmusage.CreatedAtGTE(start)).
		All(ctx)
	if err != nil {
		return v1.UsageSummary{}, apperr.New(apperr.CodeInternal, "查询用量统计失败", err)
	}

	buckets := make(map[string]*v1.UsageItem, days)
	for i := days - 1; i >= 0; i-- {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		buckets[d] = &v1.UsageItem{Date: d}
	}
	for _, r := range rows {
		key := r.CreatedAt.Format("2006-01-02")
		b, ok := buckets[key]
		if !ok {
			continue // 早于窗口的历史行
		}
		b.Tokens += r.PromptTokens + r.CompletionTokens
		b.Cost += r.Cost
	}

	out := v1.UsageSummary{Items: make([]v1.UsageItem, 0, len(buckets))}
	for i := days - 1; i >= 0; i-- {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		item := *buckets[d]
		out.TotalTokens += item.Tokens
		out.TotalCost += item.Cost
		out.Items = append(out.Items, item)
	}
	return out, nil
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
