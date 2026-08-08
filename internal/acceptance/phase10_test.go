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
	"github.com/muzaffer/internship-tracker/internal/store"
)

// TestPhase10JSONLDStructuredDataFlowsThroughIngestion exercises the Faz 10
// exit criterion: a JSON-LD career page is normalized to the strict schema with
// zero AI calls in the adapter, enters the existing dedup/analysis path, and a
// second unchanged scan produces no new listings and no repeat analysis.
func TestPhase10JSONLDStructuredDataFlowsThroughIngestion(t *testing.T) {
	fixture, err := os.ReadFile("../scraper/testdata/jsonld/career-jobposting.html")
	if err != nil {
		t.Fatalf("read JSON-LD fixture: %v", err)
	}
	const pageURL = "https://careers.northstar.example/"
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	source, err := scraper.NewJSONLDSource("northstar-careers", "Northstar Robotics", pageURL, client)
	if err != nil {
		t.Fatalf("create JSON-LD source: %v", err)
	}

	provider := &fixtureProvider{}
	modelAnalyzer, err := analyzer.NewModelAnalyzer(provider, "fixture-strict-model", analyzer.CostRates{
		InputPerMillionUSD: 0.25, OutputPerMillionUSD: 1.50,
	})
	if err != nil {
		t.Fatalf("create fixture analyzer: %v", err)
	}
	db, repository := openRepository(t)
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "northstar-careers", Company: "Northstar Robotics", PriorityGroup: "candidate",
		Type: "career_page", URL: pageURL, Adapter: "json_ld", Strategy: "json_ld", Enabled: true,
	}); err != nil {
		t.Fatalf("register JSON-LD source: %v", err)
	}
	service := &orchestrator.Service{
		Sources: []scraper.Source{source}, Analyzer: modelAnalyzer, Store: repository,
		Profile: minimizedAcceptanceProfile(),
	}
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }

	first, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("first JSON-LD scan: %v", err)
	}
	if first.Status != "completed" || first.Sources[0].Found != 2 || first.Sources[0].New != 2 || first.Sources[0].ProcessError != 0 {
		t.Fatalf("unexpected first scan result: %#v", first.Sources)
	}

	now = now.Add(time.Second)
	second, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("second JSON-LD scan: %v", err)
	}
	if second.Sources[0].Found != 2 || second.Sources[0].New != 0 || second.Sources[0].ProcessError != 0 {
		t.Fatalf("unchanged rescan must dedup to zero new: %#v", second.Sources)
	}
	if provider.calls != 2 {
		t.Fatalf("each distinct posting analyzed once, no re-analysis on rescan; calls=%d", provider.calls)
	}

	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	titles := map[string]bool{}
	for _, listing := range append(append([]store.DashboardListing(nil), dashboard.NewListings...), dashboard.NeedsDecision...) {
		titles[listing.Title] = true
	}
	for _, want := range []string{"Software Engineering Intern", "Data Engineering Intern"} {
		if !titles[want] {
			t.Fatalf("JSON-LD posting %q not visible in dashboard: %#v", want, dashboard)
		}
	}
	_ = db
}
