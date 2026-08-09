//go:build integration

package acceptance_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/extractor"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
)

type countingPhase12Provider struct {
	underlying  analyzer.ModelProvider
	mu          sync.Mutex
	recipeCalls int
}

func (p *countingPhase12Provider) Name() string { return p.underlying.Name() }

func (p *countingPhase12Provider) Complete(ctx context.Context, request analyzer.ProviderRequest) (analyzer.ProviderResponse, error) {
	p.mu.Lock()
	if strings.Contains(request.SystemPrompt, "çıkarım reçetesi") {
		p.recipeCalls++
	}
	p.mu.Unlock()
	return p.underlying.Complete(ctx, request)
}

func (p *countingPhase12Provider) RecipeCalls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.recipeCalls
}

func TestPhase12LiveGeminiLearnsPersistsAndRepairsRecipe(t *testing.T) {
	if os.Getenv("RUN_PHASE12_LIVE_ACCEPTANCE") != "1" {
		t.Skip("set RUN_PHASE12_LIVE_ACCEPTANCE=1 to run the live Gemini phase 12 acceptance")
	}
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" && strings.TrimSpace(os.Getenv("GEMINI_API_KEY_FILE")) != "" {
		contents, err := os.ReadFile(strings.TrimSpace(os.Getenv("GEMINI_API_KEY_FILE")))
		if err != nil {
			t.Fatalf("read GEMINI_API_KEY_FILE: %v", err)
		}
		apiKey = strings.TrimSpace(string(contents))
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY or GEMINI_API_KEY_FILE is not set")
	}
	v1, err := os.ReadFile("../scraper/testdata/learnedselector/layout-v1.html")
	if err != nil {
		t.Fatalf("read v1 fixture: %v", err)
	}
	v2, err := os.ReadFile("../scraper/testdata/learnedselector/layout-v2.html")
	if err != nil {
		t.Fatalf("read v2 fixture: %v", err)
	}
	var mu sync.RWMutex
	current := v1
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write(current)
	}))
	defer server.Close()

	google, err := analyzer.NewGoogleProvider(apiKey, "minimal", &http.Client{Timeout: 90 * time.Second})
	if err != nil {
		t.Fatalf("create Google provider: %v", err)
	}
	provider := &countingPhase12Provider{underlying: google}
	model := strings.TrimSpace(os.Getenv("PHASE12_LIVE_TEST_MODEL"))
	if model == "" {
		model = "gemini-3.1-flash-lite"
	}
	learner, err := extractor.NewGeminiRecipeLearner(provider, model)
	if err != nil {
		t.Fatalf("create recipe learner: %v", err)
	}
	listingAnalyzer, err := analyzer.NewModelAnalyzer(provider, model, analyzer.CostRates{})
	if err != nil {
		t.Fatalf("create listing analyzer: %v", err)
	}
	_, repository := openRepository(t)
	const sourceKey = "phase12-live-learned"
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: sourceKey, Company: "Vega Yazılım", PriorityGroup: "candidate", Type: "career_page",
		URL: server.URL, Adapter: "learned_selector", Strategy: "learned_selector", Enabled: true,
	}); err != nil {
		t.Fatalf("register live learned source: %v", err)
	}
	scanAt := time.Date(2026, 8, 9, 15, 0, 0, 0, time.UTC)
	run := func(label string) orchestrator.ScanResult {
		t.Helper()
		source, err := scraper.NewLearnedSelectorSource(sourceKey, "Vega Yazılım", server.URL, repository, learner, server.Client())
		if err != nil {
			t.Fatalf("%s create source: %v", label, err)
		}
		service := orchestrator.Service{Sources: []scraper.Source{source}, Analyzer: listingAnalyzer, Store: repository, Profile: minimizedAcceptanceProfile()}
		service.Now = func() time.Time { return scanAt }
		result, err := service.Run(context.Background(), "manual")
		if err != nil {
			t.Fatalf("%s scan: %v", label, err)
		}
		scanAt = scanAt.Add(3 * time.Second)
		return result
	}

	first := run("initial")
	if first.Status != "completed" || first.Sources[0].Found != 2 || first.Sources[0].New != 2 || provider.RecipeCalls() != 1 {
		t.Fatalf("unexpected initial live result: %#v recipe_calls=%d", first.Sources, provider.RecipeCalls())
	}
	second := run("restart")
	if second.Sources[0].Found != 2 || second.Sources[0].New != 0 || provider.RecipeCalls() != 1 {
		t.Fatalf("persisted live recipe was not AI-free: %#v recipe_calls=%d", second.Sources, provider.RecipeCalls())
	}
	mu.Lock()
	current = v2
	mu.Unlock()
	third := run("repair")
	if third.Status != "completed" || third.Sources[0].Found != 2 || third.Sources[0].New != 1 || provider.RecipeCalls() != 2 {
		t.Fatalf("live recipe did not self-repair: %#v recipe_calls=%d", third.Sources, provider.RecipeCalls())
	}
	recipe, ok, err := repository.LoadSourceRecipe(context.Background(), sourceKey)
	if err != nil || !ok || recipe.Version != 2 || recipe.GoldenListingCount != 2 {
		t.Fatalf("live repaired recipe missing: ok=%v recipe=%#v err=%v", ok, recipe, err)
	}
	t.Logf("phase12 live acceptance: model=%s first_new=%d second_new=%d repair_new=%d recipe_version=%d recipe_calls=%d", model, first.Sources[0].New, second.Sources[0].New, third.Sources[0].New, recipe.Version, provider.RecipeCalls())
}
