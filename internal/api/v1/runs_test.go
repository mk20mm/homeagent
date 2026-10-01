package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	domrun "github.com/mk20mm/homeagent/internal/domain/run"
)

type stubRunService struct {
	runs   []domrun.RunItem
	detail domrun.RunDetail
}

func (s *stubRunService) GetRun(_ context.Context, _ string, runID string) (domrun.RunDetail, error) {
	d := s.detail
	d.ID = runID
	return d, nil
}

func (s *stubRunService) ListRuns(_ context.Context, _ string, _ string, _ int) ([]domrun.RunItem, error) {
	return s.runs, nil
}

func (s *stubRunService) ConfirmRunStep(_ context.Context, _ string, runID string, stepID string, choice string, _ string) (domrun.RunDetail, error) {
	d := s.detail
	d.ID = runID
	d.Steps = []domrun.RunStepItem{
		{ID: stepID, Status: "committed", ResultCard: choice},
	}
	return d, nil
}

func (s *stubRunService) ReplyRun(_ context.Context, _ string, runID string, stepID string, _ int, _ map[string]any) (domrun.RunDetail, error) {
	d := s.detail
	d.ID = runID
	d.Status = "running"
	return d, nil
}

func (s *stubRunService) AmendRun(_ context.Context, _ string, runID string, _ int, _ []map[string]any) (domrun.RunDetail, error) {
	d := s.detail
	d.ID = runID
	d.Version = d.Version + 1
	return d, nil
}

func setupRunRouter(svc RunService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 模拟已认证中间件注入 member_id
	authMiddleware := func(c *gin.Context) {
		c.Set("member_id", "mem-test-1")
		c.Next()
	}

	api := r.Group("", authMiddleware)
	api.GET("/runs", ListRuns(svc))
	api.GET("/runs/:runId", GetRun(svc))
	api.POST("/runs/:runId/confirm", ConfirmRunStep(svc))
	api.POST("/runs/:runId/reply", ReplyRun(svc))
	api.POST("/runs/:runId/amend", AmendRun(svc))
	return r
}

func TestRunsAPI_Endpoints(t *testing.T) {
	svc := &stubRunService{
		runs: []domrun.RunItem{
			{ID: "r1", Goal: "做饭并扫地", IntentKind: "plan", Status: "running", Version: 1, CreatedAt: time.Now()},
		},
		detail: domrun.RunDetail{
			ID:         "r1",
			Goal:       "做饭并扫地",
			IntentKind: "plan",
			Status:     "running",
			Version:    1,
			Steps: []domrun.RunStepItem{
				{ID: "s1", Position: 1, ToolName: "start_cooking", Status: "committed", WaitFor: "none"},
				{ID: "s2", Position: 2, ToolName: "start_cleaning", Status: "pending", WaitFor: "entity_completed"},
			},
			Events: []domrun.RunEventItem{
				{ID: "e1", Seq: 1, EventType: "run_created", CreatedAt: time.Now()},
			},
			CreatedAt: time.Now(),
		},
	}
	r := setupRunRouter(svc)

	// 1. GET /runs
	req, _ := http.NewRequest(http.MethodGet, "/runs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /runs failed with code: %d", w.Code)
	}
	var listResp struct {
		Runs []domrun.RunItem `json:"runs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil || len(listResp.Runs) != 1 {
		t.Fatalf("unexpected list response: %s", w.Body.String())
	}

	// 2. GET /runs/:runId
	req, _ = http.NewRequest(http.MethodGet, "/runs/r1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /runs/r1 failed with code: %d", w.Code)
	}
	var getResp domrun.RunDetail
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil || getResp.ID != "r1" || len(getResp.Steps) != 2 {
		t.Fatalf("unexpected detail response: %s", w.Body.String())
	}

	// 3. POST /runs/:runId/confirm
	bodyConfirm := `{"step_id":"s2","choice":"proceed"}`
	req, _ = http.NewRequest(http.MethodPost, "/runs/r1/confirm", strings.NewReader(bodyConfirm))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /runs/r1/confirm failed with code: %d", w.Code)
	}

	// 4. POST /runs/:runId/reply
	bodyReply := `{"request_id":"req-1","step_id":"s2","expected_version":1,"answers":{"note":"yes"}}`
	req, _ = http.NewRequest(http.MethodPost, "/runs/r1/reply", strings.NewReader(bodyReply))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /runs/r1/reply failed with code: %d", w.Code)
	}

	// 5. POST /runs/:runId/amend
	bodyAmend := `{"expected_version":1,"modifications":[{"field":"time"}]}`
	req, _ = http.NewRequest(http.MethodPost, "/runs/r1/amend", strings.NewReader(bodyAmend))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /runs/r1/amend failed with code: %d", w.Code)
	}
}
