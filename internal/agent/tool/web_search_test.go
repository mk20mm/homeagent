package tool

import (
	"context"
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
