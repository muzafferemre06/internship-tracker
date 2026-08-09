package acceptance_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
)

type phase12Learner struct {
	calls int
}

func (l *phase12Learner) LearnRecipe(_ context.Context, request scraper.RecipeLearningRequest) (domain.SourceRecipe, error) {
	l.calls++
	if request.Reason == "initial_setup" {
		return domain.SourceRecipe{IdentitySelector: "#careers", IdentityText: "Vega Yazılım Kariyer", ListingSelector: ".opening", TitleSelector: ".title", LinkSelector: "a.apply"}, nil
	}
	return domain.SourceRecipe{IdentitySelector: ".jobs-board", IdentityText: "Vega Yazılım Kariyer", ListingSelector: ".position-card", TitleSelector: ".position-name", LinkSelector: "a.position-link"}, nil
}

func TestPhase12LearnedRecipePersistsRunsWithoutAIAndSelfRepairs(t *testing.T) {
	v1, err := os.ReadFile("../scraper/testdata/learnedselector/layout-v1.html")
	if err != nil {
		t.Fatalf("read v1 fixture: %v", err)
	}
	v2, err := os.ReadFile("../scraper/testdata/learnedselector/layout-v2.html")
	if err != nil {
		t.Fatalf("read v2 fixture: %v", err)
	}
	current := v1
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(current))}, nil
	})}
	_, repository := openRepository(t)
	const sourceKey = "vega-learned"
	const pageURL = "https://careers.vega.example/jobs"
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: sourceKey, Company: "Vega Yazılım", PriorityGroup: "candidate", Type: "career_page",
		URL: pageURL, Adapter: "learned_selector", Strategy: "learned_selector", Enabled: true,
	}); err != nil {
		t.Fatalf("register learned source: %v", err)
	}
	learner := &phase12Learner{}
	provider := &fixtureProvider{}
	modelAnalyzer, err := analyzer.NewModelAnalyzer(provider, "fixture-strict-model", analyzer.CostRates{})
	if err != nil {
		t.Fatalf("create analyzer: %v", err)
	}
	scanAt := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	run := func(label string) orchestrator.ScanResult {
		t.Helper()
		// Recreate the source every time to prove persistence crosses process-like restarts.
		source, err := scraper.NewLearnedSelectorSource(sourceKey, "Vega Yazılım", pageURL, repository, learner, client)
		if err != nil {
			t.Fatalf("%s create source: %v", label, err)
		}
		service := orchestrator.Service{Sources: []scraper.Source{source}, Analyzer: modelAnalyzer, Store: repository, Profile: minimizedAcceptanceProfile()}
		service.Now = func() time.Time { return scanAt }
		result, err := service.Run(context.Background(), "manual")
		if err != nil {
			t.Fatalf("%s scan: %v", label, err)
		}
		scanAt = scanAt.Add(3 * time.Second)
		return result
	}

	first := run("initial")
	if first.Status != "completed" || first.Sources[0].Found != 2 || first.Sources[0].New != 2 || learner.calls != 1 {
		t.Fatalf("unexpected initial result: %#v learner_calls=%d", first.Sources, learner.calls)
	}
	second := run("restart")
	if second.Sources[0].Found != 2 || second.Sources[0].New != 0 || learner.calls != 1 {
		t.Fatalf("persisted recipe did not run AI-free: %#v learner_calls=%d", second.Sources, learner.calls)
	}
	current = v2
	third := run("repair")
	if third.Sources[0].Found != 2 || third.Sources[0].New != 1 || learner.calls != 2 {
		t.Fatalf("changed layout did not self-repair: %#v learner_calls=%d", third.Sources, learner.calls)
	}
	recipe, ok, err := repository.LoadSourceRecipe(context.Background(), sourceKey)
	if err != nil || !ok || recipe.Version != 2 || recipe.GoldenListingCount != 2 || recipe.GoldenFingerprint == "" {
		t.Fatalf("repaired golden recipe not persisted: ok=%v recipe=%#v err=%v", ok, recipe, err)
	}
}
