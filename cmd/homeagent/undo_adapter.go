package main

import (
	"context"

	v1 "github.com/mk20mm/homeagent/internal/api/v1"
	"github.com/mk20mm/homeagent/internal/domain/undo"
)

// undoStoreAdapter 把 v1.UndoStore（handler 用的接口）适配成 undo.Store（工具用的接口）。
// repo 层不依赖 domain（AGENTS.md 不变量 7），适配在 main 层完成。
type undoStoreAdapter struct {
	inner v1.UndoStore
}

var _ undo.Store = (*undoStoreAdapter)(nil)

func (a *undoStoreAdapter) ListActive(ctx context.Context, memberID string) ([]undo.Record, error) {
	records, err := a.inner.ListActive(ctx, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]undo.Record, 0, len(records))
	for _, r := range records {
		out = append(out, undo.Record{
			ID:        r.ID,
			ToolName:  r.ToolName,
			UndoData:  r.UndoData,
			ExpiresAt: r.ExpiresAt,
		})
	}
	return out, nil
}

func (a *undoStoreAdapter) MarkUsed(ctx context.Context, id string) error {
	return a.inner.MarkUsed(ctx, id)
}
