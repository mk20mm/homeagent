// Package apperr 实现错误三段式：Code（程序判断）/ Msg（面向用户中文）/ Cause（原始错误仅入日志）。
// handler 统一翻译为 HTTP 状态；未知错误不得泄漏堆栈/SQL 给前端。
package apperr

import "fmt"

type Code string

const (
	CodeInvalidInput Code = "invalid_input"
	CodeUnauthorized Code = "unauthorized"
	CodePermission   Code = "permission_denied"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeInternal     Code = "internal"
)

type Error struct {
	Code Code
	Msg  string
	Cause error
}

func New(code Code, msg string, cause error) *Error {
	return &Error{Code: code, Msg: msg, Cause: cause}
}

func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Msg, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func (e *Error) Unwrap() error { return e.Cause }
