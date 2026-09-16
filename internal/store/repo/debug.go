package repo

import (
	"context"
	"encoding/json"
	"time"

	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/auditlog"
	"github.com/mk20mm/homeagent/internal/store/ent/expense"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
	"github.com/mk20mm/homeagent/internal/store/ent/undolog"
)

// DebugState 聚合三张表的开发期视图（P1 删除，技术债 T11）。
// 临时反向依赖 api/v1，验收完随调试端点一并移除。
func (s *Store) DebugState(ctx context.Context, memberID string) (v1.DebugState, error) {
	mid := member.IDEQ(toUUID(memberID))

	undoRows, err := s.db.UndoLog.Query().
		Where(undolog.HasMemberWith(mid)).
		Order(ent.Desc(undolog.FieldCreatedAt)).
		Limit(5).All(ctx)
	if err != nil {
		return v1.DebugState{}, err
	}
	undos := make([]v1.DebugUndo, 0, len(undoRows))
	for _, l := range undoRows {
		var data []byte
		if b, err := json.Marshal(l.UndoData); err == nil {
			data = b
		}
		undos = append(undos, v1.DebugUndo{
			ID:        l.ID.String(),
			ToolName:  l.ToolName,
			TraceID:   l.TraceID,
			Status:    string(l.Status),
			UndoData:  data,
			ExpiresAt: l.ExpiresAt,
			CreatedAt: l.CreatedAt,
		})
	}

	auditRows, err := s.db.AuditLog.Query().
		Where(auditlog.HasMemberWith(mid)).
		Order(ent.Desc(auditlog.FieldCreatedAt)).
		Limit(5).All(ctx)
	if err != nil {
		return v1.DebugState{}, err
	}
	audits := make([]v1.DebugAudit, 0, len(auditRows))
	for _, l := range auditRows {
		audits = append(audits, v1.DebugAudit{
			TraceID:          l.TraceID,
			ToolName:         l.ToolName,
			Risk:             string(l.Risk),
			Result:           l.Result,
			Undone:           l.Undone,
			PermissionDenied: l.PermissionDenied,
			CreatedAt:        l.CreatedAt,
		})
	}

	// 本月账单（含已软删除，调试需看到撤销前后的差异）
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	expRows, err := s.db.Expense.Query().
		Where(expense.HasMemberWith(mid), expense.OccurredAtGTE(start), expense.OccurredAtLT(end)).
		WithCategory().All(ctx)
	if err != nil {
		return v1.DebugState{}, err
	}
	byCat := map[string]int64{}
	var total int64
	for _, e := range expRows {
		total += e.AmountCents
		name := "其他"
		if e.Edges.Category != nil {
			name = e.Edges.Category.Name
		}
		byCat[name] += e.AmountCents
	}

	return v1.DebugState{
		UndoLogs: undos,
		Audits:   audits,
		Summary: v1.DebugSummary{
			Count:      len(expRows),
			TotalCents: total,
			ByCategory: byCat,
		},
	}, nil
}
