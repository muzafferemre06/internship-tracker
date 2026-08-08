package acceptance_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

// bespokeExtractor is a deterministic stand-in for the LLM extraction stage: it
// turns any reduced block mentioning "staj" into a listing, citing the block's
// first LINK line as the URL. It records calls so the test can prove the
// content-hash gate keeps the model off the hot path.
type bespokeExtractor struct{ calls int }

func (e *bespokeExtractor) Name() string { return "fake-extractor" }

func (e *bespokeExtractor) Extract(_ context.Context, request scraper.ExtractionRequest) (scraper.ExtractionResult, error) {
	e.calls++
	result := scraper.ExtractionResult{}
	for _, block := range request.Blocks {
		if !strings.Contains(strings.ToLower(block.Text), "staj") {
			continue
		}
		title, link := "", ""
		for _, line := range strings.Split(block.Text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "LINK:") {
				if link == "" {
					link = strings.TrimSpace(strings.TrimPrefix(line, "LINK:"))
				}
				continue
			}
			if title == "" && line != "" {
				title = line
			}
		}
		result.Listings = append(result.Listings, scraper.ExtractedListing{
			SourceBlock: block.Index, Title: title, URL: link, Summary: block.Text, Confidence: 0.9,
		})
	}
	return result, nil
}

// TestPhase11GenericReduceThenLLMFlowsThroughIngestion exercises the Faz 11 exit
// criterion end-to-end: a bespoke HTML page is reduced and extracted (via the
// injected extractor) into the strict schema, flows through the existing
// dedup/analysis path, and a second unchanged scan produces no new listings and
// makes no further model call.
func TestPhase11GenericReduceThenLLMFlowsThroughIngestion(t *testing.T) {
	fixture, err := os.ReadFile("../scraper/testdata/llmgeneric/bespoke.html")
	if err != nil {
		t.Fatalf("read bespoke fixture: %v", err)
	}
	const pageURL = "https://kariyer.vega.example/kariyer"
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	extractor := &bespokeExtractor{}
	source, err := scraper.NewLLMGenericSource("vega-careers", "Vega Yazılım", pageURL, extractor, client)
	if err != nil {
		t.Fatalf("create llm_generic source: %v", err)
	}

	provider := &fixtureProvider{}
	modelAnalyzer, err := analyzer.NewModelAnalyzer(provider, "fixture-strict-model", analyzer.CostRates{
		InputPerMillionUSD: 0.25, OutputPerMillionUSD: 1.50,
	})
	if err != nil {
		t.Fatalf("create fixture analyzer: %v", err)
	}
	db, repository := openRepository(t)
	_ = db
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "vega-careers", Company: "Vega Yazılım", PriorityGroup: "candidate",
		Type: "career_page", URL: pageURL, Adapter: "llm_generic", Strategy: "llm_generic", Enabled: true,
	}); err != nil {
		t.Fatalf("register source: %v", err)
	}
	service := &orchestrator.Service{
		Sources: []scraper.Source{source}, Analyzer: modelAnalyzer, Store: repository,
		Profile: minimizedAcceptanceProfile(),
	}
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }

	first, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if first.Status != "completed" || first.Sources[0].Found != 2 || first.Sources[0].New != 2 || first.Sources[0].ProcessError != 0 {
		t.Fatalf("unexpected first scan: %#v", first.Sources)
	}

	now = now.Add(time.Second)
	second, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if second.Sources[0].New != 0 || second.Sources[0].ProcessError != 0 {
		t.Fatalf("unchanged rescan must dedup to zero new: %#v", second.Sources)
	}
	if extractor.calls != 1 {
		t.Fatalf("content-hash gate must keep the model off the unchanged rescan; calls=%d", extractor.calls)
	}

	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	titles := map[string]bool{}
	for _, listing := range append(append([]store.DashboardListing(nil), dashboard.NewListings...), dashboard.NeedsDecision...) {
		titles[listing.Title] = true
	}
	for _, want := range []string{"Backend Staj Programı 2026", "Veri Bilimi Yaz Stajı"} {
		if !titles[want] {
			t.Fatalf("extracted listing %q not visible in dashboard: %#v", want, dashboard)
		}
	}
}
