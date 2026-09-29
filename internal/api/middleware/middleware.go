// Package middleware 应用层中间件：recover / trace_id / cors / jwtauth。
package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// MemberLookup 由 handler 层提供：memberID 存在且启用校验（JWT 中间件用）。
type MemberLookup interface {
	Exists(memberID string) bool
}

// TraceID 为每个请求生成 trace_id，供审计/日志关联（AI-STD-008 最小 Trace 字段）。
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Trace-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("trace_id", id)
		c.Writer.Header().Set("X-Trace-ID", id)
		c.Next()
	}
}

// Recover 捕获 panic，记录结构化日志，返回 500（不泄漏堆栈给前端）。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"trace_id", c.GetString("trace_id"),
					"path", c.Request.URL.Path,
					"recover", rec)
				c.AbortWithStatusJSON(500, gin.H{
					"code":    "internal",
					"message": "系统内部错误",
				})
			}
		}()
		c.Next()
	}
}

// CORS 允许前端与原生应用跨域（单家庭私有中枢，支持 Capacitor 原生壳与局域网移动端）。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		reqHeaders := c.GetHeader("Access-Control-Request-Headers")
		reqMethod := c.GetHeader("Access-Control-Request-Method")

		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if reqHeaders != "" {
			c.Writer.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS,HEAD")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			slog.Info("CORS preflight 204", "origin", origin, "req_headers", reqHeaders, "req_method", reqMethod)
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// abortJSON 错误三段式响应（与 v1.abortWith 同语义，中间件自用）。
func abortJSON(c *gin.Context, e *apperr.Error) {
	status := 500
	switch e.Code {
	case apperr.CodeInvalidInput:
		status = 400
	case apperr.CodeUnauthorized:
		status = 401
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
