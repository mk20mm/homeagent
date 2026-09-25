package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// stubProviderService 最小实现，只测 handler 层逻辑。
type stubProviderService struct {
	providers  []dommodel.ProviderInfo
	updCalled  bool
	failUpdate bool
}

func (s *stubProviderService) ListProviders(_ context.Context) ([]dommodel.ProviderInfo, error) {
	return s.providers, nil
}

func (s *stubProviderService) UpdateProvider(_ context.Context, _ string, _ dommodel.ProviderUpdate) (dommodel.ProviderInfo, error) {
	s.updCalled = true
	if s.failUpdate {
		return dommodel.ProviderInfo{}, apperr.New(apperr.CodeInternal, "密钥加密失败", nil)
	}
	return dommodel.ProviderInfo{ID: "p1", Name: "deepseek", APIKeyMasked: "sk-a***nop", APIKeySet: true, Enabled: true}, nil
}

func (s *stubProviderService) UpdateModel(_ context.Context, _ string, u dommodel.ModelUpdate) (dommodel.ModelInfo, error) {
	s.updCalled = true
	if u.IsDefault != nil && !*u.IsDefault {
		return dommodel.ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "取消默认请直接将其他模型设为默认", nil)
	}
	return dommodel.ModelInfo{ID: "m1", ModelName: "gpt-4o", DisplayName: "GPT-4o", Provider: "openai"}, nil
}

func (s *stubProviderService) CreateModel(_ context.Context, providerID, modelName, displayName string, isDefault bool) (dommodel.ModelInfo, error) {
	return dommodel.ModelInfo{ID: "m2", ModelName: modelName, DisplayName: displayName, Provider: "deepseek", IsDefault: isDefault}, nil
}

func (s *stubProviderService) DeleteModel(_ context.Context, id string) error {
	if id == "default-model" {
		return apperr.New(apperr.CodeInvalidInput, "默认模型不可删除", nil)
	}
	return nil
}

func (s *stubProviderService) GetProviderConnection(_ context.Context, providerID string) (dommodel.ProviderConnection, error) {
	if providerID == "no-key" {
		return dommodel.ProviderConnection{}, apperr.New(apperr.CodeInvalidInput, "未配置密钥", nil)
	}
	return dommodel.ProviderConnection{
		APIKey: "sk-test", BaseURL: "https://api.test.com", Provider: "openai", Model: "gpt-4o",
	}, nil
}

func setupAdminRouter(role string, svc ProviderService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(func(c *gin.Context) {
		c.Set("member_id", "member-1")
		if role != "" {
			c.Set("role", role)
		}
		c.Next()
	})
	g.GET("/admin/providers", ListProviders(svc))
	g.PUT("/admin/providers/:id", UpdateProvider(svc, nil))
	g.POST("/admin/providers/:id/test", TestProvider(svc))
	g.POST("/admin/models", CreateModel(svc, nil))
	g.PUT("/admin/models/:id", UpdateModel(svc, nil))
	g.DELETE("/admin/models/:id", DeleteModel(svc, nil))
	return r
}

func adminDo(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestListProvidersMasksKey(t *testing.T) {
	svc := &stubProviderService{
		providers: []dommodel.ProviderInfo{
			{ID: "p1", Name: "deepseek", APIKeyMasked: "sk-a***nop", APIKeySet: true, Enabled: true},
		},
	}
	r := setupAdminRouter("parent", svc)

	w := adminDo(r, http.MethodGet, "/api/v1/admin/providers", "")
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var out struct {
		Providers []ProviderView `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Providers) != 1 || out.Providers[0].APIKeyMasked != "sk-a***nop" || !out.Providers[0].APIKeySet {
		t.Errorf("unexpected providers: %+v", out.Providers)
	}
}

func TestUpdateProviderRequiresParent(t *testing.T) {
	svc := &stubProviderService{}
	r := setupAdminRouter("child", svc) // 孩子不能改配置

	w := adminDo(r, http.MethodPut, "/api/v1/admin/providers/p1", `{"api_key":"sk-xxx"}`)
	if w.Code != 403 {
		t.Fatalf("child must be forbidden, got %d: %s", w.Code, w.Body.String())
	}
	if svc.updCalled {
		t.Error("service must not be called when forbidden")
	}
}

func TestUpdateProviderParentOK(t *testing.T) {
	svc := &stubProviderService{}
	r := setupAdminRouter("parent", svc)

	w := adminDo(r, http.MethodPut, "/api/v1/admin/providers/p1", `{"api_key":"sk-xxx"}`)
	if w.Code != 200 {
		t.Fatalf("parent update should succeed, got %d: %s", w.Code, w.Body.String())
	}
	var out ProviderView
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.APIKeyMasked != "sk-a***nop" {
		t.Errorf("masked key expected, got %q", out.APIKeyMasked)
	}
}

func TestUpdateModelRejectsUnsetDefault(t *testing.T) {
	svc := &stubProviderService{}
	r := setupAdminRouter("parent", svc)

	// 领域层禁止把 is_default 设为 false（必须通过设别的模型为默认来切换）
	w := adminDo(r, http.MethodPut, "/api/v1/admin/models/m1", `{"is_default":false}`)
	if w.Code != 400 {
		t.Fatalf("is_default:false should be rejected, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateModelAndDeleteModel(t *testing.T) {
	svc := &stubProviderService{}
	r := setupAdminRouter("parent", svc)

	// 新建模型
	w := adminDo(r, http.MethodPost, "/api/v1/admin/models", `{"provider_id":"p1","model_name":"deepseek-reasoner","display_name":"R1"}`)
	if w.Code != 201 {
		t.Fatalf("create model should succeed, got %d: %s", w.Code, w.Body.String())
	}

	// 删除模型
	w2 := adminDo(r, http.MethodDelete, "/api/v1/admin/models/m2", "")
	if w2.Code != 204 {
		t.Fatalf("delete model should return 204, got %d: %s", w2.Code, w2.Body.String())
	}

	// 尝试删除默认模型返回 400
	w3 := adminDo(r, http.MethodDelete, "/api/v1/admin/models/default-model", "")
	if w3.Code != 400 {
		t.Fatalf("delete default model should return 400, got %d: %s", w3.Code, w3.Body.String())
	}
}
