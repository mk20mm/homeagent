package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractDDGUrl(t *testing.T) {
	rawHref := "//duckduckgo.com/l/?uddg=https%3A%2F%2Fjevai.net%2F&rut=ddc2fd60fcf5b33ac13c8a6e57fed24252615f5eae5a6fed194cd4583cc5b293"
	got := unwrapDDGRedirect(rawHref)
	expected := "https://jevai.net/"
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestFetchLiveFreeSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results := fetchLiveFreeSearch(ctx, "jev", 3)
	if len(results) == 0 {
		t.Log("Live search returned 0 results (network might be offline), testing fallback portal")
		results = fallbackDomesticSearch("jev", 3)
	}

	if len(results) == 0 {
		t.Fatalf("expected at least 1 result")
	}

	for _, r := range results {
		if !strings.HasPrefix(r.URL, "http://") && !strings.HasPrefix(r.URL, "https://") {
			t.Errorf("invalid URL: %s", r.URL)
		}
		t.Logf("Result: [%s] -> %s (%s)", r.Title, r.URL, r.SourceName)
	}
}

func TestReRankHeuristic(t *testing.T) {
	candidates := []SearchCitationItem{
		{
			Title:      "家电超市大促销：各款洗碗机低价甩卖中",
			URL:        "https://example.com/ad",
			SourceName: "example.com",
			Snippet:    "点击选购最新款智能洗碗机，满千减百，全国包邮送货上门。",
			IsOfficial: false,
		},
		{
			Title:      "西门子官方客服：洗碗机进水水龙头指示灯亮起自检排查步骤",
			URL:        "https://www.siemens-home.bsh-group.cn/faq/water",
			SourceName: "siemens-home.bsh-group.cn",
			Snippet:    "水龙头红灯亮起通常代表进水阀受阻或滤网结垢。排查步骤：首先关闭角阀，拧下进水管清洗微孔滤网水垢。",
			IsOfficial: true,
		},
		{
			Title:      "闲聊水吧：大家家里的洗碗机好用吗？",
			URL:        "https://forum.example.com/topic/1",
			SourceName: "forum.example.com",
			Snippet:    "洗碗机真的能解放双手吗，求推荐好用的品牌型号。",
			IsOfficial: false,
		},
	}

	query := "西门子洗碗机水龙头亮红灯"
	ranked := reRankHeuristic(query, candidates)

	if len(ranked) != 3 {
		t.Fatalf("expected 3 items, got %d", len(ranked))
	}

	// 官方且命中关键词的条目必须排在第 1 名
	if ranked[0].Title != "西门子官方客服：洗碗机进水水龙头指示灯亮起自检排查步骤" {
		t.Fatalf("expected official guide ranked first, got %s", ranked[0].Title)
	}
}

func TestReRankWithMockJev(t *testing.T) {
	// 启动模拟 TypeSafe System One API 服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp := typeSafeResponse{
			Answers: map[string]typeSafeAnswer{
				"c_0": {Noul: 0.12, Confidence: 0.95}, // 广告低分
				"c_1": {Noul: 0.96, Confidence: 0.98}, // 官方解决方案极高分
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	t.Setenv("TYPESAFE_ENDPOINT", ts.URL)

	candidates := []SearchCitationItem{
		{
			Title:      "洗碗机团购折扣活动",
			URL:        "https://promo.example.com",
			SourceName: "promo.example.com",
			Snippet:    "快来选购优惠洗碗机",
		},
		{
			Title:      "官方排查指南：E15 故障消除",
			URL:        "https://service.example.com/e15",
			SourceName: "service.example.com",
			Snippet:    "排查底部漏水传感器，断电倾斜排出积水即可消除",
		},
	}

	ctx := context.Background()
	ranked := reRankWithJev(ctx, "test-key", "E15 故障怎么排查", candidates)

	if len(ranked) != 2 {
		t.Fatalf("expected 2 items, got %d", len(ranked))
	}

	// Jev 打分 0.96 的 c_1 必须排在第 1 名
	if ranked[0].Title != "官方排查指南：E15 故障消除" {
		t.Fatalf("expected Jev to rank high-noul item first, got %s", ranked[0].Title)
	}
}

func TestReRankCandidatesFallbackWhenKeyFails(t *testing.T) {
	// 模拟 Jev Key 过期或服务返回 401 故障
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_api_key"}`, http.StatusUnauthorized)
	}))
	defer ts.Close()

	t.Setenv("TYPESAFE_ENDPOINT", ts.URL)
	t.Setenv("TYPESAFE_API_KEY", "expired-key-12345")

	candidates := []SearchCitationItem{
		{
			Title:      "家电超市大促销：各款洗碗机低价甩卖中",
			URL:        "https://example.com/ad",
			SourceName: "example.com",
			Snippet:    "特价抢购",
			IsOfficial: false,
		},
		{
			Title:      "西门子官方客服：洗碗机进水水龙头指示灯亮起自检排查步骤",
			URL:        "https://www.siemens-home.bsh-group.cn/faq/water",
			SourceName: "siemens-home.bsh-group.cn",
			Snippet:    "水龙头红灯排查清洗滤网",
			IsOfficial: true,
		},
	}

	ctx := context.Background()
	// 当 Key 过期时，reRankCandidates 绝不崩溃，而是无感平滑降级为启发式重排
	ranked := reRankCandidates(ctx, "西门子洗碗机水龙头亮红灯", candidates, 2)

	if len(ranked) != 2 {
		t.Fatalf("expected 2 items, got %d", len(ranked))
	}
	// 启发式重排依然能将官方结果置顶于第 1 名
	if ranked[0].Title != "西门子官方客服：洗碗机进水水龙头指示灯亮起自检排查步骤" {
		t.Fatalf("expected graceful fallback to rank official first, got %s", ranked[0].Title)
	}
}
