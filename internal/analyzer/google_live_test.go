//go:build integration

package analyzer_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
)

func TestGoogleProviderLive(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}
	model := os.Getenv("GEMINI_LIVE_TEST_MODEL")
	if model == "" {
		model = "gemini-3.1-flash-lite"
	}
	provider, err := analyzer.NewGoogleProvider(apiKey, "minimal", &http.Client{Timeout: 60 * time.Second})
	if err != nil {
		t.Fatalf("create live provider: %v", err)
	}
	modelAnalyzer, err := analyzer.NewModelAnalyzer(provider, model, analyzer.CostRates{})
	if err != nil {
		t.Fatalf("create live analyzer: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	analysis, err := modelAnalyzer.Analyze(ctx, domain.RawListing{
		Company: "Test Company", SourceID: "live-test", Title: "Backend Stajyeri",
		URL: "https://example.test/jobs/backend-intern", RawText: `Go ile API geliştirme.
Ankara hibrit çalışma. Başvurular açık. 2. sınıf veya üzeri öğrenciler başvurabilir.`,
	}, analyzer.CandidateProfile{
		EducationField: "Bilgisayar Teknolojisi", ClassYear: 2, GPA: 3.7,
		FocusAreas: []string{"backend"}, ExperienceAreas: []string{"Go", "API"},
		Locations: []string{"Ankara"},
	})
	if err != nil {
		t.Fatalf("live analysis: %v", err)
	}
	if analysis.Provider != "google" || analysis.Model != model || analysis.TotalTokens < 1 {
		t.Fatalf("live metadata is incomplete: %#v", analysis)
	}
	if analysis.Summary == "" || analysis.Confidence < 0 || analysis.Confidence > 1 {
		t.Fatalf("live schema result is invalid: %#v", analysis)
	}
	t.Logf(
		"LIVE_RESULT provider=%s model=%s eligibility=%s opportunity=%s location=%q work_model=%s confidence=%.2f tokens=%d/%d/%d summary=%q",
		analysis.Provider, analysis.Model, analysis.Eligibility, analysis.OpportunityType,
		analysis.Location, analysis.WorkModel, analysis.Confidence, analysis.PromptTokens,
		analysis.CompletionTokens, analysis.TotalTokens, analysis.Summary,
	)
}
