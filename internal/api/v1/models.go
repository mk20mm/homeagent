package v1

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/apperr"
	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
)

// ModelInfo 模型视图（对齐 openapi Model）。
type ModelInfo struct {
	ID          string `json:"id"`
	ModelName   string `json:"model_name"`
	DisplayName string `json:"display_name"`
	Provider    string `json:"provider"`
	IsDefault   bool   `json:"is_default"`
}

// ModelLister 已启用模型清单（repo 实现，领域层不感知 HTTP）。
type ModelLister interface {
	ListEnabled(ctx context.Context) ([]dommodel.ModelInfo, error)
}

// ListModels GET /models：会话内模型切换的清单来源（全员可用）。
func ListModels(lister ModelLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := lister.ListEnabled(c.Request.Context())
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "查询模型清单失败", err))
			return
		}
		out := make([]ModelInfo, 0, len(list))
		for _, m := range list {
			out = append(out, ModelInfo{
				ID:          m.ID,
				ModelName:   m.ModelName,
				DisplayName: m.DisplayName,
				Provider:    m.Provider,
				IsDefault:   m.IsDefault,
			})
		}
		c.JSON(http.StatusOK, gin.H{"models": out})
	}
}
