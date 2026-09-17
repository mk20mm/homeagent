package v1

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// UsageItem 按日汇总的用量（对齐 openapi /usage items）。
type UsageItem struct {
	Date   string  `json:"date"`
	Tokens int     `json:"tokens"`
	Cost   float64 `json:"cost"`
}

// UsageSummary 用量汇总视图（对齐 openapi /usage）。
type UsageSummary struct {
	TotalCost   float64     `json:"total_cost"`
	TotalTokens int         `json:"total_tokens"`
	Items       []UsageItem `json:"items"`
}

// UsageLister 用量汇总（repo 实现）。
type UsageLister interface {
	UsageSummary(ctx context.Context, days int) (UsageSummary, error)
}

// ListUsage GET /usage：近 N 日 LLM 用量（管理端仪表盘）。
func ListUsage(lister UsageLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		days := 7
		if v := c.Query("days"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				days = n
			}
		}

		summary, err := lister.UsageSummary(c.Request.Context(), days)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "查询用量统计失败", err))
			return
		}
		if summary.Items == nil {
			summary.Items = []UsageItem{}
		}
		c.JSON(http.StatusOK, summary)
	}
}
