package v1

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// ProviderView 供应商视图（对齐 openapi Provider，api_key 只回脱敏值）。
type ProviderView struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	BaseURL       string `json:"base_url"`
	APIKeyMasked  string `json:"api_key_masked"`
	APIKeySet     bool   `json:"api_key_set"`
	Enabled       bool   `json:"enabled"`
}

// ModelView 模型视图（对齐 openapi Model）。
type ModelView struct {
	ID          string `json:"id"`
	ModelName   string `json:"model_name"`
	DisplayName string `json:"display_name"`
	Provider    string `json:"provider"`
	IsDefault   bool   `json:"is_default"`
	Enabled     bool   `json:"enabled"`
}

// ProviderService 供应商/模型管理（领域服务实现）。
type ProviderService interface {
	ListProviders(ctx context.Context) ([]dommodel.ProviderInfo, error)
	ListAllModels(ctx context.Context) ([]dommodel.ModelInfo, error)
	UpdateProvider(ctx context.Context, id string, u dommodel.ProviderUpdate) (dommodel.ProviderInfo, error)
	CreateModel(ctx context.Context, req dommodel.ModelCreateRequest) (dommodel.ModelInfo, error)
	UpdateModel(ctx context.Context, id string, u dommodel.ModelUpdate) (dommodel.ModelInfo, error)
	DeleteModel(ctx context.Context, id string) error
	TestConnection(ctx context.Context, req dommodel.ProviderTestRequest) (dommodel.ProviderTestResult, error)
}

func toProviderView(p dommodel.ProviderInfo) ProviderView {
	return ProviderView{
		ID: p.ID, Name: p.Name, BaseURL: p.BaseURL,
		APIKeyMasked: p.APIKeyMasked, APIKeySet: p.APIKeySet, Enabled: p.Enabled,
	}
}

func toModelView(m dommodel.ModelInfo) ModelView {
	return ModelView{
		ID: m.ID, ModelName: m.ModelName, DisplayName: m.DisplayName,
		Provider: m.Provider, IsDefault: m.IsDefault, Enabled: m.Enabled,
	}
}

// ListProviders GET /admin/providers：供应商清单（脱敏密钥）。
func ListProviders(svc ProviderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		list, err := svc.ListProviders(c.Request.Context())
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "查询供应商失败", err))
			return
		}
		out := make([]ProviderView, 0, len(list))
		for _, p := range list {
			out = append(out, toProviderView(p))
		}
		c.JSON(http.StatusOK, gin.H{"providers": out})
	}
}

// TestProviderConnection POST /admin/providers/test。
func TestProviderConnection(svc ProviderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		var req struct {
			ProviderID *string `json:"provider_id"`
			BaseURL    *string `json:"base_url"`
			APIKey     *string `json:"api_key"`
			Model      *string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求格式错误", err))
			return
		}

		res, err := svc.TestConnection(c.Request.Context(), dommodel.ProviderTestRequest{
			ProviderID: req.ProviderID,
			BaseURL:    req.BaseURL,
			APIKey:     req.APIKey,
			Model:      req.Model,
		})
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":         res.OK,
			"latency_ms": res.LatencyMs,
			"error":      res.Error,
			"models":     res.Models,
		})
	}
}

// UpdateProvider PUT /admin/providers/{id}。
func UpdateProvider(svc ProviderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		id := c.Param("id")
		if id == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少供应商 id", nil))
			return
		}

		var req struct {
			APIKey  *string `json:"api_key"`
			BaseURL *string `json:"base_url"`
			Enabled *bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求格式错误", err))
			return
		}

		p, err := svc.UpdateProvider(c.Request.Context(), id, dommodel.ProviderUpdate{
			APIKey: req.APIKey, BaseURL: req.BaseURL, Enabled: req.Enabled,
		})
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusOK, toProviderView(p))
	}
}

// CreateModel POST /admin/models。
func CreateModel(svc ProviderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		var req struct {
			ProviderID    string `json:"provider_id" binding:"required"`
			ModelName     string `json:"model_name" binding:"required"`
			DisplayName   string `json:"display_name" binding:"required"`
			ContextWindow *int   `json:"context_window"`
			IsDefault     *bool  `json:"is_default"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求格式错误或缺少必填字段", err))
			return
		}

		m, err := svc.CreateModel(c.Request.Context(), dommodel.ModelCreateRequest{
			ProviderID:    req.ProviderID,
			ModelName:     req.ModelName,
			DisplayName:   req.DisplayName,
			ContextWindow: req.ContextWindow,
			IsDefault:     req.IsDefault,
		})
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusCreated, toModelView(m))
	}
}

// ListAllModels GET /admin/models。
func ListAllModels(svc ProviderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		list, err := svc.ListAllModels(c.Request.Context())
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "查询模型清单失败", err))
			return
		}
		out := make([]ModelView, 0, len(list))
		for _, m := range list {
			out = append(out, toModelView(m))
		}
		c.JSON(http.StatusOK, gin.H{"models": out})
	}
}

// UpdateModel PUT /admin/models/{id}。
func UpdateModel(svc ProviderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		id := c.Param("id")
		if id == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少模型 id", nil))
			return
		}

		var req struct {
			ModelName     *string `json:"model_name"`
			DisplayName   *string `json:"display_name"`
			ContextWindow *int    `json:"context_window"`
			Enabled       *bool   `json:"enabled"`
			IsDefault     *bool   `json:"is_default"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求格式错误", err))
			return
		}

		m, err := svc.UpdateModel(c.Request.Context(), id, dommodel.ModelUpdate{
			ModelName:     req.ModelName,
			DisplayName:   req.DisplayName,
			ContextWindow: req.ContextWindow,
			Enabled:       req.Enabled,
			IsDefault:     req.IsDefault,
		})
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusOK, toModelView(m))
	}
}

// DeleteModel DELETE /admin/models/{id}。
func DeleteModel(svc ProviderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		id := c.Param("id")
		if id == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少模型 id", nil))
			return
		}

		if err := svc.DeleteModel(c.Request.Context(), id); err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": true})
	}
}

// requireParent 只有 parent 角色能改配置（权限双保险：JWT claims 里的 role）。
func requireParent(c *gin.Context) bool {
	role, ok := c.Get("role")
	if !ok || role != "parent" {
		abortWith(c, apperr.New(apperr.CodePermission, "只有家长可以管理供应商配置", nil))
		return false
	}
	return true
}
