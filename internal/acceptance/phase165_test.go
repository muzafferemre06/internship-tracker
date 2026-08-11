package acceptance_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/config"
	"github.com/muzaffer/internship-tracker/internal/httpapi"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

func TestPhase165ResearchCohortAndOfficialInnovaIndexEndToEnd(t *testing.T) {
	ctx := context.Background()
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatal(err)
	}
	_, repository := openRepository(t)
	registerPhase16Catalog(t, repository, configured)

	fixture, err := os.ReadFile("../scraper/testdata/careerlinks/innova-careers.html")
	if err != nil {
		t.Fatal(err)
	}
	pageClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://www.innova.com.tr/is-ilanlari" {
			t.Fatalf("unexpected request %s", request.URL)
		}
		return phase16Response(http.StatusOK, fixture), nil
	})}
	robotsClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return phase16Response(http.StatusOK, []byte("User-agent: *\nAllow: /\n")), nil
	})}
	source, err := scraper.NewCareerLinksSourceWithAllowedHosts(
		"innova-official-jobs", "İnnova", "https://www.innova.com.tr/is-ilanlari", "open-positions", "/jobs/view/", []string{"www.linkedin.com"}, pageClient,
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	service := orchestrator.Service{
		Sources: []scraper.Source{scraper.WithAccessPolicy(source, scraper.AccessPolicy{
			Mode: "robots", Scope: "innova.com.tr", TargetURL: "https://www.innova.com.tr/is-ilanlari",
			MinimumInterval: 24 * time.Hour, BaseCooldown: time.Hour, MaximumCooldown: 24 * time.Hour,
		})},
		Robots:   scraper.NewHTTPRobotsChecker(robotsClient, func() time.Time { return now }),
		Analyzer: analyzer.NewDeterministicAnalyzer(), Store: repository,
		Profile: analyzer.CandidateProfile{ClassYear: 2, FocusAreas: []string{"backend"}}, Now: func() time.Time { return now },
	}
	result, err := service.Run(ctx, "manual")
	if err != nil || len(result.Sources) != 1 || result.Sources[0].Found != 2 {
		t.Fatalf("official index scan result=%#v err=%v", result, err)
	}

	handler := httpapi.NewHandler("*", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, repository, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/coverage", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("coverage status=%d body=%s", response.Code, response.Body.String())
	}
	var coverage store.CoverageReport
	if err := json.Unmarshal(response.Body.Bytes(), &coverage); err != nil {
		t.Fatal(err)
	}
	phase := coverage.SectionSummaries["phase_16_5"]
	if phase.TotalCompanies != 11 || phase.AutomaticSources != 1 || phase.ManualSources != 4 || phase.ResearchingSources != 6 ||
		phase.AutomaticCoveragePercent < 14.2 || phase.AutomaticCoveragePercent > 14.3 {
		t.Fatalf("unexpected Phase 16.5 summary: %#v", phase)
	}
	if secondary := coverage.SectionSummaries["secondary"]; secondary.TotalCompanies != 4 || secondary.AutomaticSources != 4 {
		t.Fatalf("regular secondary companies were not separated: %#v", secondary)
	}
	for _, company := range coverage.Companies {
		if company.TrackingPhase != "16.5" {
			continue
		}
		for _, item := range company.Sources {
			if item.LastVerifiedAt == nil || (item.Status != "automatic" && item.ReasonCode == "") {
				t.Fatalf("Phase 16.5 source lacks visible research metadata: %#v", item)
			}
		}
	}
}
