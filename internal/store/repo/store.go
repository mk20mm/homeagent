// Package repo 仓储层实现：ent 查询封装。领域层依赖接口，不直接碰 ent.Client
// （AGENTS.md 不变量 7：依赖单向 api → domain → store）。
package repo

import (
	"github.com/mk20mm/homeagent/internal/store/ent"
)

// Store 聚合所有仓储实现，由 main 装配后注入领域层。
type Store struct {
	db *ent.Client
}

func New(db *ent.Client) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *ent.Client { return s.db }
