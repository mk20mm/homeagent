package v1

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/gateway"
	"github.com/mk20mm/homeagent/internal/apperr"
	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
)

// ProviderView 供应商视图（对齐 openapi Provider，api_key 只回脱敏值）。
type ProviderView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	APIKeyMasked string `json:"api_key_masked"`
	APIKeySet    bool   `json:"api_key_set"`
	Enabled      bool   `json:"enabled"`
}

// ModelView 模型视图（对齐 openapi Model）。
type ModelView struct {
	ID          string `json:"id"`
	ModelName   string `json:"model_name"`
	DisplayName string `json:"display_name"`
	Provider    string `json:"provider"`
	IsDefault   bool   `json:"is_default"`
}

// ProviderTestResult 供应商连通性测试结果（对齐 openapi ProviderTestResult）。
type ProviderTestResult struct {
	Success   bool   `json:"success"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// CreateModelRequest 创建模型入参。
type CreateModelRequest struct {
	ProviderID  string `json:"provider_id" binding:"required"`
	ModelName   string `json:"model_name" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	IsDefault   bool   `json:"is_default"`
}

// CacheInvalidator 网关缓存失效接口。
type CacheInvalidator interface {
	Invalidate()
}

// ProviderService 供应商/模型管理（领域服务实现）。
type ProviderService interface {
	ListProviders(ctx context.Context) ([]dommodel.ProviderInfo, error)
	UpdateProvider(ctx context.Context, id string, u dommodel.ProviderUpdate) (dommodel.ProviderInfo, error)
	UpdateModel(ctx context.Context, id string, u dommodel.ModelUpdate) (dommodel.ModelInfo, error)
	CreateModel(ctx context.Context, providerID, modelName, displayName string, isDefault bool) (dommodel.ModelInfo, error)
	DeleteModel(ctx context.Context, id string) error
	GetProviderConnection(ctx context.Context, providerID string) (dommodel.ProviderConnection, error)
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
		Provider: m.Provider, IsDefault: m.IsDefault,
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

// UpdateProvider PUT /admin/providers/{id}。
func UpdateProvider(svc ProviderService, inv CacheInvalidator) gin.HandlerFunc {
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
		if inv != nil {
			inv.Invalidate()
		}
		c.JSON(http.StatusOK, toProviderView(p))
	}
}

// TestProvider POST /admin/providers/{id}/test：测试供应商连通性。
func TestProvider(svc ProviderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		id := c.Param("id")
		if id == "" {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "缺少供应商 id", nil))
			return
		}

		conn, err := svc.GetProviderConnection(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusOK, ProviderTestResult{
				Success:   false,
				LatencyMS: 0,
				Error:     err.Error(),
			})
			return
		}

		latency, probeErr := gateway.Probe(c.Request.Context(), conn)
		if probeErr != nil {
			c.JSON(http.StatusOK, ProviderTestResult{
				Success:   false,
				LatencyMS: latency,
				Error:     probeErr.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, ProviderTestResult{
			Success:   true,
			LatencyMS: latency,
		})
	}
}

// CreateModel POST /admin/models。
func CreateModel(svc ProviderService, inv CacheInvalidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireParent(c) {
			return
		}
		var req CreateModelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求格式错误", err))
			return
		}
		m, err := svc.CreateModel(c.Request.Context(), req.ProviderID, req.ModelName, req.DisplayName, req.IsDefault)
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		if inv != nil {
			inv.Invalidate()
		}
		c.JSON(http.StatusCreated, toModelView(m))
	}
}

// UpdateModel PUT /admin/models/{id}。
func UpdateModel(svc ProviderService, inv CacheInvalidator) gin.HandlerFunc {
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
			Enabled   *bool `json:"enabled"`
			IsDefault *bool `json:"is_default"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "请求格式错误", err))
			return
		}

		m, err := svc.UpdateModel(c.Request.Context(), id, dommodel.ModelUpdate{
			Enabled: req.Enabled, IsDefault: req.IsDefault,
		})
		if err != nil {
			abortWith(c, asAppErr(err))
			return
		}
		if inv != nil {
			inv.Invalidate()
		}
		c.JSON(http.StatusOK, toModelView(m))
	}
}

// DeleteModel DELETE /admin/models/{id}。
func DeleteModel(svc ProviderService, inv CacheInvalidator) gin.HandlerFunc {
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
		if inv != nil {
			inv.Invalidate()
		}
		c.Status(http.StatusNoContent)
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
