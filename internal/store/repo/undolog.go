package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
	"github.com/mk20mm/homeagent/internal/store/ent/undolog"
)

// 编译期：实现 tool.UndoStore（Executor 调用 Save）。
var _ tool.UndoStore = (*Store)(nil)

// Save 写撤销记录，24h 有效（schema 默认值兜底），返回撤销 id。
func (s *Store) Save(ctx context.Context, r tool.UndoRecord) (string, error) {
	data := map[string]any{}
	// undo_data 可能是任意 JSON；非 object 兜底包一层
	if err := json.Unmarshal(r.UndoData, &data); err != nil {
		data = map[string]any{"raw": string(r.UndoData)}
	}
	exp := time.Now()
	if r.ExpiresIn > 0 {
		exp = exp.Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	l, err := s.db.UndoLog.Create().
		SetTraceID(r.TraceID).
		SetToolName(r.ToolName).
		SetUndoData(data).
		SetMemberID(toUUID(r.MemberID)).
		SetExpiresAt(exp).
		Save(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeInternal, "保存撤销记录失败", err)
	}
	return l.ID.String(), nil
}

// GetUndo 取撤销记录（含 member 隔离校验），适配 v1.UndoStore 接口。
func (s *Store) GetUndo(ctx context.Context, id, memberID string) (v1.UndoRecord, error) {
	l, err := s.db.UndoLog.Query().
		Where(undolog.IDEQ(toUUID(id)), undolog.HasMemberWith(member.IDEQ(toUUID(memberID)))).
		Only(ctx)
	if err != nil {
		return v1.UndoRecord{}, apperr.New(apperr.CodeNotFound, "撤销记录不存在或不属于该成员", err)
	}
	out := v1.UndoRecord{
		ID:        l.ID.String(),
		ToolName:  l.ToolName,
		Status:    string(l.Status),
		ExpiresAt: l.ExpiresAt,
	}
	if b, err := json.Marshal(l.UndoData); err == nil {
		out.UndoData = b
	}
	return out, nil
}

// MarkUsed 标记撤销已使用（幂等：重复标记不报错）。
func (s *Store) MarkUsed(ctx context.Context, id string) error {
	_, err := s.db.UndoLog.UpdateOneID(toUUID(id)).
		SetStatus(undolog.StatusUsed).
		Save(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "标记撤销记录失败", err)
	}
	return nil
}

// SaveUndo 写撤销记录（v1.ExpenseUndoWriter 接口；undo_log 24h 有效）。
func (s *Store) SaveUndo(ctx context.Context, memberID string, toolName string, undoData json.RawMessage) (string, error) {
	data := map[string]any{}
	if err := json.Unmarshal(undoData, &data); err != nil {
		data = map[string]any{"raw": string(undoData)}
	}
	l, err := s.db.UndoLog.Create().
		SetTraceID(tool.TraceIDFrom(ctx)).
		SetToolName(toolName).
		SetUndoData(data).
		SetMemberID(toUUID(memberID)).
		SetExpiresAt(time.Now().Add(24 * time.Hour)).
		Save(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeInternal, "保存撤销记录失败", err)
	}
	return l.ID.String(), nil
}

// SweepExpired 清理过期记录（cron 调用，P1 接入）。
func (s *Store) SweepExpired(ctx context.Context) (int, error) {
	n, err := s.db.UndoLog.Update().
		Where(undolog.StatusEQ(undolog.StatusActive), undolog.ExpiresAtLT(time.Now())).
		SetStatus(undolog.StatusExpired).
		Save(ctx)
	if err != nil {
		return 0, apperr.New(apperr.CodeInternal, "清理过期撤销记录失败", err)
	}
	return n, nil
}
