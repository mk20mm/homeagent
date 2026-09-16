// Package middleware 应用层中间件：recover / trace_id / cors。
package middleware

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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

// CORS 允许前端跨域（开发期）。
func CORS() gin.HandlerFunc {
	allow := []string{"http://localhost:5173", "http://localhost:3001"}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		for _, a := range allow {
			if strings.EqualFold(origin, a) {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Trace-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
