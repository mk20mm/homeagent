package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestLiveComparisonJevVsOriginal(t *testing.T) {
	// 读取本地 .env 密钥
	apiKey := os.Getenv("TYPESAFE_API_KEY")
	if apiKey == "" {
		envBytes, err := os.ReadFile("../../../.env")
		if err == nil {
			for _, line := range strings.Split(string(envBytes), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "TYPESAFE_API_KEY=") {
					apiKey = strings.TrimPrefix(line, "TYPESAFE_API_KEY=")
					break
				}
			}
		}
	}

	if apiKey == "" {
		t.Skip("跳过在线对比：未找到 TYPESAFE_API_KEY")
	}

	query := "西门子洗碗机水龙头亮红灯排查"

	// 模拟真实搜索返回的候选池（含官方实操指南、电商导购广告、通用说明书主页、论坛闲聊灌水）
	candidates := []SearchCitationItem{
		{
			Title:      "【京东精选】2026 最新款洗碗机大促活动",
			URL:        "https://item.jd.com/dishwasher-sale",
			SourceName: "jd.com",
			Snippet:    "大容量嵌入式洗碗机限时直降 1000 元，支持多重清洗模式，免费上门安装售后无忧。",
			IsOfficial: false,
		},
		{
			Title:      "百度贴吧：家里的洗碗机红灯闪烁是坏了吗",
			URL:        "https://tieba.baidu.com/p/dishwasher-help",
			SourceName: "tieba.baidu.com",
			Snippet:    "求助各位大佬，昨天用还好好的，今天突然水龙头图标变红了，有遇到过相同情况的吗？在线等挺急的。",
			IsOfficial: false,
		},
		{
			Title:      "西门子家电官方客户支持：洗碗机进水受阻/水龙头指示灯亮起排查指引",
			URL:        "https://www.siemens-home.bsh-group.cn/service/support/dishwasher-water-tap",
			SourceName: "siemens-home.bsh-group.cn",
			Snippet:    "水龙头亮红灯表示进水流量异常。官方排查步骤：1. 确认角阀完全开启；2. 检查进水管无弯折受压；3. 拆下进水滤网清洗水垢与杂质。",
			IsOfficial: true,
		},
		{
			Title:      "西门子家电官网首页 · 探索德系精工厨房与全套智能家电",
			URL:        "https://www.siemens-home.bsh-group.cn/",
			SourceName: "siemens-home.bsh-group.cn",
			Snippet:    "欢迎访问西门子家电中国官方网站，探索冰箱、洗衣机、洗碗机、烤箱等全系高端厨房电器产品。",
			IsOfficial: true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// 1. 调用真实 Jev System One 重排，并度量耗时
	t0 := time.Now()
	jevRanked := reRankWithJev(ctx, apiKey, query, candidates)
	jevDuration := time.Since(t0)

	// 2. 调用启发式重排
	t1 := time.Now()
	heuristicRanked := reRankHeuristic(query, candidates)
	heuristicDuration := time.Since(t1)

	t.Logf("\n================ 【搜索排序能力实测对比报告】 ================")
	t.Logf("检索问题：%s", query)
	t.Logf("\n--- [1] 原始初筛自然顺序 (Raw Search Pool): ---")
	for i, c := range candidates {
		t.Logf("  #%d [%s] %s (摘要: %s...)", i+1, c.SourceName, c.Title, c.Snippet[:30])
	}

	t.Logf("\n--- [2] TypeSafe Jev System One 决策重排 (Jev Re-ranked, 耗时: %v): ---", jevDuration)
	for i, c := range jevRanked {
		t.Logf("  Top %d: [%s] %s", i+1, c.SourceName, c.Title)
	}

	t.Logf("\n--- [3] 本地启发式多因子降级重排 (Heuristic Re-ranked, 耗时: %v): ---", heuristicDuration)
	for i, c := range heuristicRanked {
		t.Logf("  Top %d: [%s] %s", i+1, c.SourceName, c.Title)
	}
	t.Logf("==============================================================\n")

	if len(jevRanked) > 0 && jevRanked[0].Title != "西门子家电官方客户支持：洗碗机进水受阻/水龙头指示灯亮起排查指引" {
		t.Errorf("Jev 应该将具体实操步骤排在第 1 名，但实际是: %s", jevRanked[0].Title)
	}
}
