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

// MemberName 按 id 查成员名（HTTP 边界把 assignee_id 解析成名字，供领域幂等键）。
func (s *Store) MemberName(ctx context.Context, memberID string) (string, error) {
	m, err := s.db.Member.Query().
		Where(member.IDEQ(toUUID(memberID))).
		Only(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}
	return m.Name, nil
}

// FamilyIDByMember 成员 → 家庭 id（查全家日程/账单等聚合视图用）。
func (s *Store) FamilyIDByMember(ctx context.Context, memberID string) (string, error) {
	m, err := s.db.Member.Query().
		Where(member.IDEQ(toUUID(memberID))).
		WithFamily().
		Only(ctx)
	if err != nil {
		return "", apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}
	if m.Edges.Family == nil {
		return "", apperr.New(apperr.CodeNotFound, "成员不属于任何家庭", nil)
	}
	return m.Edges.Family.ID.String(), nil
}
