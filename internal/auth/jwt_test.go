package auth

import (
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	s := NewSigner("test-secret", "homeagent")
	token, exp, err := s.Sign("member-1", "parent")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if !exp.After(time.Now()) {
		t.Fatalf("expiry in past: %v", exp)
	}

	claims, err := s.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.MemberID != "member-1" || claims.Role != "parent" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.Issuer != "homeagent" {
		t.Fatalf("unexpected issuer: %q", claims.Issuer)
	}
}

func TestVerifyRejectsBadTokens(t *testing.T) {
	s := NewSigner("test-secret", "homeagent")
	forged := signOrFail(t, NewSigner("other-secret", "homeagent"), "member-1", "parent")

	cases := map[string]string{
		"empty":        "",
		"garbage":      "not.a.jwt",
		"wrong-secret": forged,
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Verify(tok); err == nil {
				t.Fatalf("expected error for %q", name)
			}
		})
	}
}

// 篡改令牌内容后验签必须失败（防 CLAUmmy 篡改 member_id）。
func TestVerifyRejectsTampered(t *testing.T) {
	s := NewSigner("test-secret", "homeagent")
	token, _, err := s.Sign("member-1", "parent")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	// 截断签名段
	tampered := token[:len(token)-3]
	if _, err := s.Verify(tampered); err == nil {
		t.Fatal("expected error for truncated signature")
	}
}

func TestSignFailsWithoutSecret(t *testing.T) {
	s := NewSigner("", "homeagent")
	if _, _, err := s.Sign("member-1", "parent"); err == nil {
		t.Fatal("expected error signing with empty secret")
	}
}

func TestSignFailsWithoutMember(t *testing.T) {
	s := NewSigner("test-secret", "homeagent")
	if _, _, err := s.Sign("", "parent"); err == nil {
		t.Fatal("expected error signing empty member_id")
	}
}

func signOrFail(t *testing.T, s *Signer, memberID, role string) string {
	t.Helper()
	token, _, err := s.Sign(memberID, role)
	if err != nil {
		t.Fatalf("sign helper: %v", err)
	}
	return token
}
