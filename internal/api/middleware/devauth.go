// Package middleware 应用层中间件：recover / trace_id / cors / devauth。
package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
)
// MemberLookup 由 handler 层提供：memberID 存在性校验。
type MemberLookup interface {
	Exists(memberID string) bool
}

// DevAuth 开发期认证：从 X-Member-ID header 或 ?member_id 取成员身份。
//
// ⚠️ 仅用于 P0 本地联调，P1 换 JWT（c-runtime.md 决策日志：JWT 放 P1 不阻塞关键路径，
// 但权限双保险的执行层校验在 P0 已内建）。
func DevAuth(lookup MemberLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID := c.GetHeader("X-Member-ID")
		if memberID == "" {
			memberID = c.Query("member_id")
		}
		if memberID == "" {
			abortJSON(c, apperr.New(apperr.CodePermission, "缺少成员身份（X-Member-ID）", nil))
			return
		}
		if lookup != nil && !lookup.Exists(memberID) {
			slog.Warn("dev auth: unknown member", "member_id", memberID, "trace_id", c.GetString("trace_id"))
			abortJSON(c, apperr.New(apperr.CodePermission, "成员不存在", nil))
			return
		}
		c.Set("member_id", memberID)
		c.Next()
	}
}

// abortJSON 错误三段式响应（与 v1.abortWith 同语义，中间件自用）。
func abortJSON(c *gin.Context, e *apperr.Error) {
	status := 500
	switch e.Code {
	case apperr.CodeInvalidInput:
		status = 400
	case apperr.CodePermission:
		status = 403
	case apperr.CodeNotFound:
		status = 404
	case apperr.CodeConflict:
		status = 409
	}
	c.AbortWithStatusJSON(status, gin.H{
		"code":     string(e.Code),
		"message":  e.Msg,
		"trace_id": c.GetString("trace_id"),
	})
}
