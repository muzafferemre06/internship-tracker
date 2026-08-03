//go:build integration

package acceptance_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

func TestPhase35LiveOfficialListingWithGemini(t *testing.T) {
	if os.Getenv("RUN_REAL_LISTING_ACCEPTANCE") != "1" {
		t.Skip("RUN_REAL_LISTING_ACCEPTANCE=1 is required")
	}
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Fatal("GEMINI_API_KEY is required for the opted-in acceptance test")
	}
	postingURL := envOrDefault("REAL_LISTING_URL", commencisURL)
	company := envOrDefault("REAL_LISTING_COMPANY", "Commencis")
	expectedTitle := envOrDefault("REAL_LISTING_EXPECTED_TITLE", commencisTitle)
	model := envOrDefault("GEMINI_LIVE_TEST_MODEL", "gemini-3.1-flash-lite")

	source, err := scraper.NewLeverSource(
		"commencis-lever-spring-boot-camp-2026", company, postingURL,
		&http.Client{Timeout: 30 * time.Second},
	)
	if err != nil {
		t.Fatalf("create live Lever source: %v", err)
	}
	provider, err := analyzer.NewGoogleProvider(apiKey, "minimal", &http.Client{Timeout: 60 * time.Second})
	if err != nil {
		t.Fatalf("create live Gemini provider: %v", err)
	}
	modelAnalyzer, err := analyzer.NewModelAnalyzer(provider, model, analyzer.CostRates{
		InputPerMillionUSD: 0.25, OutputPerMillionUSD: 1.50,
	})
	if err != nil {
		t.Fatalf("create live model analyzer: %v", err)
	}
	db, repository := openRepository(t)
	registerSource(t, repository, postingURL)
	service := &orchestrator.Service{
		Sources: []scraper.Source{source}, Analyzer: modelAnalyzer, Store: repository,
		Profile: minimizedAcceptanceProfile(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	first, err := service.Run(ctx, "manual")
	if err != nil {
		t.Fatalf("first live acceptance scan: %v", err)
	}
	if len(first.Sources) != 1 {
		t.Fatalf("first live scan returned unexpected sources: %#v", first)
	}
	if first.Sources[0].ProcessError != 0 {
		t.Fatalf("first live scan analysis failed: %s", readAcceptanceFailure(t, db))
	}
	if remaining := time.Until(first.StartedAt.Add(time.Second)); remaining > 0 {
		time.Sleep(remaining)
	}
	second, err := service.Run(ctx, "manual")
	if err != nil {
		t.Fatalf("second live acceptance scan: %v", err)
	}
	if first.Sources[0].Found != 1 || first.Sources[0].New != 1 {
		t.Fatalf("first live scan did not complete: %#v", first)
	}
	if len(second.Sources) != 1 || second.Sources[0].Found != 1 || second.Sources[0].New != 0 || second.Sources[0].ProcessError != 0 {
		t.Fatalf("second live scan did not prove deduplication: %#v", second)
	}

	record := readAcceptanceRecord(t, db)
	expectedCanonicalURL, err := store.CanonicalURL(postingURL)
	if err != nil {
		t.Fatalf("canonicalize expected live URL: %v", err)
	}
	if record.Count != 1 || record.Provider != "google" || record.Model != model || record.TotalTokens < 1 ||
		record.CanonicalURL != expectedCanonicalURL || record.FirstSeenAt == "" || record.LastSeenAt == "" {
		t.Fatalf("live acceptance metadata is incomplete: %#v", record)
	}
	eligibility := domain.EligibilityStatus(record.EligibilityStatus)
	if eligibility != domain.EligibilitySuitable && eligibility != domain.EligibilityPartlySuitable && eligibility != domain.EligibilityNeedsDecision {
		t.Fatalf("unexpected live eligibility: %s", eligibility)
	}
	assertDashboardAPI(t, service, repository, expectedTitle, eligibility)

	t.Logf(
		"PHASE35_LIVE_EVIDENCE accessed_at=%s source=%s proof=%q canonical=%s provider=%s model=%s tokens=%d/%d/%d estimated_cost_usd=%.8f eligibility=%s first_new=%d second_new=%d rows=%d",
		first.StartedAt.UTC().Format(time.RFC3339), postingURL,
		expectedTitle+"; active Lever apply link present", record.CanonicalURL,
		record.Provider, record.Model, record.PromptTokens, record.CompletionTokens,
		record.TotalTokens, record.EstimatedCostUSD, eligibility,
		first.Sources[0].New, second.Sources[0].New, record.Count,
	)
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
