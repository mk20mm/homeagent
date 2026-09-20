package v1

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/auth"
)

// MemberAuthLookup 认证查询：按名找成员（repo 实现）。
type MemberAuthLookup interface {
	FindByName(ctx context.Context, name string) (auth.MemberIdentity, error)
}

// CreateToken POST /auth/token：成员名 + auth_token 换 JWT。
//
// auth_token 为预共享密钥；比对用常量时间防时序攻击。
// 失败不区分「成员不存在」与「令牌错误」，防枚举。
func CreateToken(lookup MemberAuthLookup, signer *auth.Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name      string `json:"name" binding:"required"`
			AuthToken string `json:"auth_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "用户名或令牌不能为空", err))
			return
		}

		m, err := lookup.FindByName(c.Request.Context(), req.Name)
		if err != nil {
			// 统一错误信息防枚举；原因入日志
			abortWith(c, apperr.New(apperr.CodeInvalidCredentials, "用户名或令牌错误", err))
			return
		}
		if subtle.ConstantTimeCompare([]byte(m.Token), []byte(req.AuthToken)) != 1 {
			abortWith(c, apperr.New(apperr.CodeInvalidCredentials, "用户名或令牌错误", nil))
			return
		}

		token, exp, err := signer.Sign(m.ID, m.Role)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "签发令牌失败", err))
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":       token,
			"member_id":   m.ID,
			"role":        m.Role,
			"expires_at":  exp.Format(time.RFC3339),
		})
	}
}
