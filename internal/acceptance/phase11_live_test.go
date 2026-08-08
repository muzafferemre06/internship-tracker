//go:build integration

package acceptance_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/extractor"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
)

// TestPhase11LiveGenericExtractionWithGemini drives the Faz 11 reduce-then-LLM
// path with a real Gemini model for BOTH extraction and analysis, against a
// locally served bespoke page (no live third-party site). Opt-in only.
func TestPhase11LiveGenericExtractionWithGemini(t *testing.T) {
	if os.Getenv("RUN_REAL_LISTING_ACCEPTANCE") != "1" {
		t.Skip("RUN_REAL_LISTING_ACCEPTANCE=1 is required")
	}
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Fatal("GEMINI_API_KEY is required for the opted-in acceptance test")
	}
	model := envOrDefault("GEMINI_LIVE_TEST_MODEL", "gemini-3.1-flash-lite")

	fixture, err := os.ReadFile("../scraper/testdata/llmgeneric/bespoke.html")
	if err != nil {
		t.Fatalf("read bespoke fixture: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	provider, err := analyzer.NewGoogleProvider(apiKey, "minimal", &http.Client{Timeout: 60 * time.Second})
	if err != nil {
		t.Fatalf("create live Gemini provider: %v", err)
	}
	geminiExtractor, err := extractor.NewGeminiExtractor(provider, model)
	if err != nil {
		t.Fatalf("create live extractor: %v", err)
	}
	source, err := scraper.NewLLMGenericSource(
		"vega-careers", "Vega Yazılım", server.URL+"/kariyer", geminiExtractor,
		&http.Client{Timeout: 30 * time.Second},
	)
	if err != nil {
		t.Fatalf("create live llm_generic source: %v", err)
	}
	modelAnalyzer, err := analyzer.NewModelAnalyzer(provider, model, analyzer.CostRates{
		InputPerMillionUSD: 0.25, OutputPerMillionUSD: 1.50,
	})
	if err != nil {
		t.Fatalf("create live analyzer: %v", err)
	}

	db, repository := openRepository(t)
	_ = db
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "vega-careers", Company: "Vega Yazılım", PriorityGroup: "candidate",
		Type: "career_page", URL: server.URL + "/kariyer", Adapter: "llm_generic", Strategy: "llm_generic", Enabled: true,
	}); err != nil {
		t.Fatalf("register source: %v", err)
	}
	service := &orchestrator.Service{
		Sources: []scraper.Source{source}, Analyzer: modelAnalyzer, Store: repository,
		Profile: minimizedAcceptanceProfile(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	first, err := service.Run(ctx, "manual")
	if err != nil {
		t.Fatalf("first live scan: %v", err)
	}
	if len(first.Sources) != 1 || first.Sources[0].ProcessError != 0 {
		t.Fatalf("first live scan failed: %#v (%s)", first.Sources, readAcceptanceFailure(t, db))
	}
	if first.Sources[0].Found < 1 || first.Sources[0].New < 1 {
		t.Fatalf("live extraction found no listings: %#v", first.Sources)
	}

	second, err := service.Run(ctx, "manual")
	if err != nil {
		t.Fatalf("second live scan: %v", err)
	}
	if second.Sources[0].New != 0 || second.Sources[0].ProcessError != 0 {
		t.Fatalf("second live scan did not prove dedup: %#v", second.Sources)
	}

	record := readAcceptanceRecord(t, db)
	if record.Count < 1 || record.Provider != "google" || record.Model != model || record.TotalTokens < 1 {
		t.Fatalf("live extraction+analysis metadata incomplete: %#v", record)
	}
	t.Logf(
		"PHASE11_LIVE_EVIDENCE served_page=%s provider=%s model=%s found=%d first_new=%d second_new=%d rows=%d tokens=%d",
		server.URL+"/kariyer", record.Provider, record.Model, first.Sources[0].Found,
		first.Sources[0].New, second.Sources[0].New, record.Count, record.TotalTokens,
	)
}
