// Package auth JWT 签发与校验（ADR-005 信任边界：成员身份由令牌声明，权限仍由矩阵校验）。
//
// 设计：
//   - HS256 + JWTSecret（infra/config，密钥不进 git）
//   - claims 只放 member_id + role；权限每次请求从矩阵实时查（不缓存令牌内权限，
//     权限变更立即生效）
//   - 有效期 7 天（家事场景，老人小孩不频繁登录）；撤销靠过期，jti 黑名单留 P2
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenDuration 令牌有效期。
const TokenDuration = 7 * 24 * time.Hour

// MemberIdentity 认证通过后的成员身份（repo 查询、handler 消费，auth 层定义避免反向依赖）。
type MemberIdentity struct {
	ID    string
	Name  string
	Role  string
	Token string // 预共享密钥（开发期明文，生产应加密存储）
}

// Claims JWT 声明：member_id + role + 标准声明。
type Claims struct {
	MemberID string `json:"member_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Signer 签发/校验令牌。
type Signer struct {
	secret []byte
	issuer string
}

// NewSigner secret 为空时签发必失败（fail-closed，不默认明文）。
func NewSigner(secret, issuer string) *Signer {
	return &Signer{secret: []byte(secret), issuer: issuer}
}

// Sign 签发令牌。
func (s *Signer) Sign(memberID, role string) (token string, expiresAt time.Time, err error) {
	if memberID == "" {
		return "", time.Time{}, errors.New("member_id required")
	}
	if len(s.secret) == 0 {
		return "", time.Time{}, errors.New("jwt secret not configured")
	}
	now := time.Now()
	exp := now.Add(TokenDuration)
	claims := Claims{
		MemberID: memberID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   memberID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// Verify 校验令牌，返回声明。签名错误/过期/格式错误统一返回 error（不区分，防探测）。
func (s *Signer) Verify(token string) (Claims, error) {
	if token == "" {
		return Claims{}, errors.New("empty token")
	}
	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return Claims{}, fmt.Errorf("verify token: %w", err)
	}
	if claims.MemberID == "" {
		return Claims{}, errors.New("token missing member_id")
	}
	return claims, nil
}
