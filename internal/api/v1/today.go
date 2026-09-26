// Package v1 · today handler（PRD A-01）：今日行动摘要聚合出口。
package v1

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/today"
)

// TodayService 聚合服务（handler 只调用，不做业务）。
type TodayService interface {
	Today(ctx context.Context, memberID string) (today.Summary, error)
}

// GetToday GET /today —— 今日摘要（个人，权限服务端裁剪）。
func GetToday(svc TodayService, pl PermissionLookup, audit tool.AuditLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}
		// Today 只读聚合，走最小读权限校验（不限定工具，登录即可看自己的摘要）
		if !permissionOf(c, pl, audit, memberID, "get_today", tool.RiskLow, "today.read") {
			abortWith(c, apperr.New(apperr.CodePermission, "无今日摘要查看权限", nil))
			return
		}

		summary, err := svc.Today(c.Request.Context(), memberID)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(200, summary)
	}
}
