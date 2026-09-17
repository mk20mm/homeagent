package repo

import (
	"context"

	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/auth"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// FindByName 按成员名查身份（登录用）。名字家庭内唯一（schema 索引保证）。
func (s *Store) FindByName(ctx context.Context, name string) (auth.MemberIdentity, error) {
	m, err := s.db.Member.Query().
		Where(member.NameEQ(name)).
		Only(ctx)
	if err != nil {
		return auth.MemberIdentity{}, apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}
	if !m.Active {
		return auth.MemberIdentity{}, apperr.New(apperr.CodePermission, "成员已停用", nil)
	}
	return auth.MemberIdentity{
		ID:    m.ID.String(),
		Name:  m.Name,
		Role:  string(m.Role),
		Token: m.AuthToken,
	}, nil
}
