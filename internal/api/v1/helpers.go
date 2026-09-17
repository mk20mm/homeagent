package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// memberIDFrom 从 gin 上下文取认证中间件注入的成员 id。
func memberIDFrom(c *gin.Context) (string, bool) {
	v, ok := c.Get("member_id")
	s, _ := v.(string)
	return s, ok && s != ""
}

// asAppErr 领域层返回 apperr.Error 直接用，其余包成 internal。
func asAppErr(err error) *apperr.Error {
	if e, ok := err.(*apperr.Error); ok {
		return e
	}
	return apperr.New(apperr.CodeInternal, "系统内部错误", err)
}

// abortWith 错误三段式响应（{code, message, trace_id}，AGENTS.md 不变量 8）。
func abortWith(c *gin.Context, e *apperr.Error) {
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
