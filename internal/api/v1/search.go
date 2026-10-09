package v1

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

type rerankSearchRequest struct {
	Query      string                    `json:"query" binding:"required"`
	Candidates []tool.SearchCitationItem `json:"candidates" binding:"required"`
}

type rerankSearchResponse struct {
	Results []tool.SearchCitationItem `json:"results"`
}

// RerankSearch 处理 POST /search/rerank 请求：按需调用 TypeSafe Jev System One 对信源候选集执行智能决策重排。
func RerankSearch() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		var req rerankSearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求参数格式错误", err))
			return
		}

		if len(req.Candidates) == 0 {
			c.JSON(http.StatusOK, rerankSearchResponse{Results: []tool.SearchCitationItem{}})
			return
		}

		apiKey := os.Getenv("TYPESAFE_API_KEY")
		var results []tool.SearchCitationItem

		if apiKey != "" {
			results = tool.ReRankWithJev(c.Request.Context(), apiKey, req.Query, req.Candidates)
		}

		// 若未配置 Jev Key 或网络异常降级，回退到多因子启发式重排
		if len(results) == 0 {
			results = tool.ReRankHeuristic(req.Query, req.Candidates)
		}

		c.JSON(http.StatusOK, rerankSearchResponse{
			Results: results,
		})
	}
}
