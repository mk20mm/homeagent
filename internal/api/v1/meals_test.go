package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/domain/meal"
)

type stubMealService struct {
	reportID  string
	atHome    []meal.MealMember
	notAtHome []meal.MealMember
}

func (s *stubMealService) Report(_ context.Context, _ string, _ meal.ReportCmd) (string, error) {
	return s.reportID, nil
}

func (s *stubMealService) Summary(_ context.Context, date time.Time) (meal.MealSummary, error) {
	if date.IsZero() {
		date = meal.Today()
	}
	return meal.MealSummary{Date: date, AtHome: s.atHome, NotAtHome: s.notAtHome}, nil
}

func setupMealRouter(perms map[string]bool, ms MealService, uw UndoWriter) (*gin.Engine, *stubAuditLogger) {
	gin.SetMode(gin.TestMode)
	audit := &stubAuditLogger{}
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(func(c *gin.Context) {
		c.Set("member_id", "member-1")
		c.Next()
	})
	pl := &stubPermLookup{perms: perms}
	g.GET("/meals", ListMeals(ms, pl, audit))
	g.POST("/meals", ReportMeal(ms, uw, pl, audit))
	return r, audit
}

// 回归守卫：今日报饭（不传 date）的撤销数据必须记今日，
// 早先版本会把 cmd.Date 的零值格式化成 0001-01-01，撤销时扑空。
func TestReportMealUndoDateDefaultsToday(t *testing.T) {
	ms := &stubMealService{reportID: "mr-1"}
	uw := &stubUndoWriter{}
	r, _ := setupMealRouter(map[string]bool{"meal.write": true}, ms, uw)

	w := adminDo(r, http.MethodPost, "/api/v1/meals", `{"at_home":true}`)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}

	data, ok := uw.saved["report_meal"]
	if !ok {
		t.Fatal("未写 report_meal 撤销记录")
	}
	var d struct {
		Date string `json:"date"`
	}
	json.Unmarshal([]byte(data), &d)
	if d.Date != time.Now().Format("2006-01-02") {
		t.Fatalf("撤销日期应为今日，got %q", d.Date)
	}

	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if out["id"] != "mr-1" || out["undoable"] != true {
		t.Fatalf("响应应带记录 id 与撤销信息，got %+v", out)
	}
}

func TestReportMealKeepsExplicitDate(t *testing.T) {
	ms := &stubMealService{reportID: "mr-2"}
	uw := &stubUndoWriter{}
	r, _ := setupMealRouter(map[string]bool{"meal.write": true}, ms, uw)

	w := adminDo(r, http.MethodPost, "/api/v1/meals", `{"at_home":false,"date":"2026-01-02","note":"加班"}`)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}

	var d struct {
		Date string `json:"date"`
	}
	json.Unmarshal([]byte(uw.saved["report_meal"]), &d)
	if d.Date != "2026-01-02" {
		t.Fatalf("显式日期应保留，got %q", d.Date)
	}
}

func TestReportMealRejectsBadInput(t *testing.T) {
	ms := &stubMealService{}
	uw := &stubUndoWriter{}
	r, _ := setupMealRouter(map[string]bool{"meal.write": true}, ms, uw)

	// 缺 at_home
	w := adminDo(r, http.MethodPost, "/api/v1/meals", `{"note":"没状态"}`)
	if w.Code != 400 {
		t.Fatalf("缺 at_home 应 400，got %d", w.Code)
	}

	// 非法日期
	w = adminDo(r, http.MethodPost, "/api/v1/meals", `{"at_home":true,"date":"2026/01/02"}`)
	if w.Code != 400 {
		t.Fatalf("非法日期应 400，got %d", w.Code)
	}
}

func TestReportMealRequiresWritePerm(t *testing.T) {
	ms := &stubMealService{}
	uw := &stubUndoWriter{}
	r, audit := setupMealRouter(map[string]bool{}, ms, uw) // 无 meal.write

	w := adminDo(r, http.MethodPost, "/api/v1/meals", `{"at_home":true}`)
	if w.Code != 403 {
		t.Fatalf("无权限应 403，got %d", w.Code)
	}
	if len(uw.saved) != 0 {
		t.Fatal("无权限时不应写撤销记录")
	}
	// 越权尝试必须留审计（T17）
	if len(audit.entries) != 1 {
		t.Fatalf("应写 1 条越权审计，got %d", len(audit.entries))
	}
	e := audit.entries[0]
	if e.ToolName != "report_meal" || !e.PermissionDenied {
		t.Fatalf("审计条目不对: %+v", e)
	}
}

func TestParseDateUsesLocalMidnight(t *testing.T) {
	want := time.Date(2026, 9, 18, 0, 0, 0, 0, time.Local)
	d := parseDate("2026-09-18")
	if d == nil || !d.Equal(want) {
		t.Fatalf("parseDate 应为本地 0 点（与领域 today() 对齐），got %v", d)
	}
	if parseDate("") != nil {
		t.Fatal("空串应返回 nil")
	}
	if parseDate("not-a-date") != nil {
		t.Fatal("非法日期应返回 nil")
	}
}

func TestListMealsSummaryShape(t *testing.T) {
	ms := &stubMealService{
		atHome:    []meal.MealMember{{ID: "m1", Name: "爸爸"}},
		notAtHome: []meal.MealMember{{ID: "m2", Name: "妈妈"}},
	}
	r, _ := setupMealRouter(map[string]bool{"meal.write": true}, ms, nil)

	w := adminDo(r, http.MethodGet, "/api/v1/meals", "")
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var out struct {
		Date           string `json:"date"`
		AtHomeCount    int    `json:"at_home_count"`
		NotAtHomeCount int    `json:"not_at_home_count"`
		Members        []struct {
			MemberID string `json:"member_id"`
			Name     string `json:"name"`
			AtHome   bool   `json:"at_home"`
		} `json:"members"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.AtHomeCount != 1 || out.NotAtHomeCount != 1 {
		t.Fatalf("计数错误，got at=%d not=%d", out.AtHomeCount, out.NotAtHomeCount)
	}
	if len(out.Members) != 2 {
		t.Fatalf("应有 2 条成员记录，got %d", len(out.Members))
	}
	if out.Members[0].MemberID != "m1" || !out.Members[0].AtHome {
		t.Fatalf("在家成员错误：got %+v", out.Members[0])
	}
	if out.Members[1].MemberID != "m2" || out.Members[1].AtHome {
		t.Fatalf("不在家成员错误：got %+v", out.Members[1])
	}
}
