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
	providers   []dommodel.ProviderInfo
	updCalled   bool
	failUpdate  bool
	testOK      bool
	testLatency int
	testErr     string
}

func (s *stubProviderService) ListProviders(_ context.Context) ([]dommodel.ProviderInfo, error) {
	return s.providers, nil
}

func (s *stubProviderService) ListAllModels(_ context.Context) ([]dommodel.ModelInfo, error) {
	return []dommodel.ModelInfo{
		{ID: "m1", ModelName: "gpt-4o", DisplayName: "GPT-4o", Provider: "openai", IsDefault: true, Enabled: true},
	}, nil
}

func (s *stubProviderService) UpdateProvider(_ context.Context, _ string, _ dommodel.ProviderUpdate) (dommodel.ProviderInfo, error) {
	s.updCalled = true
	if s.failUpdate {
		return dommodel.ProviderInfo{}, apperr.New(apperr.CodeInternal, "密钥加密失败", nil)
	}
	return dommodel.ProviderInfo{ID: "p1", Name: "deepseek", APIKeyMasked: "sk-a***nop", APIKeySet: true, Enabled: true}, nil
}

func (s *stubProviderService) CreateModel(_ context.Context, req dommodel.ModelCreateRequest) (dommodel.ModelInfo, error) {
	s.updCalled = true
	if req.ModelName == "" {
		return dommodel.ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "模型名不能为空", nil)
	}
	return dommodel.ModelInfo{ID: "m2", ModelName: req.ModelName, DisplayName: req.DisplayName, Provider: "atria", Enabled: true}, nil
}

func (s *stubProviderService) UpdateModel(_ context.Context, _ string, u dommodel.ModelUpdate) (dommodel.ModelInfo, error) {
	s.updCalled = true
	if u.IsDefault != nil && !*u.IsDefault {
		return dommodel.ModelInfo{}, apperr.New(apperr.CodeInvalidInput, "取消默认请直接将其他模型设为默认", nil)
	}
	modelName := "gpt-4o"
	if u.ModelName != nil {
		modelName = *u.ModelName
	}
	displayName := "GPT-4o"
	if u.DisplayName != nil {
		displayName = *u.DisplayName
	}
	return dommodel.ModelInfo{ID: "m1", ModelName: modelName, DisplayName: displayName, Provider: "openai", Enabled: true}, nil
}

func (s *stubProviderService) DeleteModel(_ context.Context, id string) error {
	s.updCalled = true
	if id == "default-model" {
		return apperr.New(apperr.CodeInvalidInput, "不能删除当前默认模型", nil)
	}
	return nil
}

func (s *stubProviderService) TestConnection(_ context.Context, _ dommodel.ProviderTestRequest) (dommodel.ProviderTestResult, error) {
	s.updCalled = true
	var errPtr *string
	if s.testErr != "" {
		errPtr = &s.testErr
	}
	return dommodel.ProviderTestResult{
		OK:        s.testOK,
		LatencyMs: s.testLatency,
		Error:     errPtr,
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
	g.POST("/admin/providers/test", TestProviderConnection(svc))
	g.PUT("/admin/providers/:id", UpdateProvider(svc))
	g.GET("/admin/models", ListAllModels(svc))
	g.POST("/admin/models", CreateModel(svc))
	g.PUT("/admin/models/:id", UpdateModel(svc))
	g.DELETE("/admin/models/:id", DeleteModel(svc))
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

func TestTestProviderConnectionRequiresParent(t *testing.T) {
	svc := &stubProviderService{}
	r := setupAdminRouter("child", svc)

	w := adminDo(r, http.MethodPost, "/api/v1/admin/providers/test", `{"api_key":"sk-xxx"}`)
	if w.Code != 403 {
		t.Fatalf("child must be forbidden, got %d", w.Code)
	}
}

func TestTestProviderConnectionParentOK(t *testing.T) {
	svc := &stubProviderService{
		testOK:      true,
		testLatency: 120,
	}
	r := setupAdminRouter("parent", svc)

	w := adminDo(r, http.MethodPost, "/api/v1/admin/providers/test", `{"api_key":"sk-test","base_url":"https://api.atria-asi.ai/v1"}`)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		OK        bool `json:"ok"`
		LatencyMs int  `json:"latency_ms"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.LatencyMs != 120 {
		t.Errorf("unexpected test result: %+v", out)
	}
}

func TestCreateModelParentOK(t *testing.T) {
	svc := &stubProviderService{}
	r := setupAdminRouter("parent", svc)

	w := adminDo(r, http.MethodPost, "/api/v1/admin/models", `{"provider_id":"p1","model_name":"Atria-Dawn-Preview","display_name":"Atria Dawn"}`)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var out ModelView
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.ModelName != "Atria-Dawn-Preview" || out.DisplayName != "Atria Dawn" {
		t.Errorf("unexpected model view: %+v", out)
	}
}

func TestDeleteModelParentOK(t *testing.T) {
	svc := &stubProviderService{}
	r := setupAdminRouter("parent", svc)

	w := adminDo(r, http.MethodDelete, "/api/v1/admin/models/m1", "")
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
