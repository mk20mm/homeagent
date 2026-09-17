package middleware

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/auth"
)

// JWTAuth 校验 Authorization: Bearer <token>，把 member_id/role 注入 ctx。
//
// 权限不在此校验：令牌只证明「你是谁」，权限矩阵每次请求实时查（ADR-005 双保险）。
// DevAuth（明文 X-Member-ID）随 P1 JWT 上线已下线。
func JWTAuth(signer *auth.Signer, lookup MemberLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			abortJSON(c, apperr.New(apperr.CodeUnauthorized, "缺少认证令牌", nil))
			return
		}
		// Bearer <token>（大小写不敏感，容错无空格）
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abortJSON(c, apperr.New(apperr.CodeUnauthorized, "认证令牌格式错误", nil))
			return
		}
		claims, err := signer.Verify(strings.TrimSpace(parts[1]))
		if err != nil {
			slog.Warn("jwt verify failed", "trace_id", c.GetString("trace_id"), "err", err)
			abortJSON(c, apperr.New(apperr.CodeUnauthorized, "登录已过期或令牌无效", nil))
			return
		}
		// 成员可能被停用/删除，令牌仍在有效期内 → 拒绝
		if lookup != nil && !lookup.Exists(claims.MemberID) {
			slog.Warn("jwt member inactive", "member_id", claims.MemberID, "trace_id", c.GetString("trace_id"))
			abortJSON(c, apperr.New(apperr.CodeUnauthorized, "成员不存在或已停用", nil))
			return
		}
		c.Set("member_id", claims.MemberID)
		c.Set("role", claims.Role)
		c.Next()
	}
}
