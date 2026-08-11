package acceptance_test

import (
	"bytes"
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

func TestPhase18ApprovedBatchCoverageAndFalseNotificationGuardEndToEnd(t *testing.T) {
	ctx := context.Background()
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatalf("load production sources: %v", err)
	}
	want := map[string]struct {
		adapter, coverage, policy string
		enabled                   bool
	}{
		"MobileAction": {adapter: "lever_board", coverage: "automatic", policy: "robots", enabled: true},
		"SİMSOFT":      {adapter: "manual", coverage: "manual", policy: "manual_only"},
		"Netaş":        {adapter: "manual", coverage: "manual", policy: "manual_only"},
		"Bilişim AŞ":   {adapter: "manual", coverage: "manual", policy: "manual_only"},
	}
	for companyName, expected := range want {
		company := phase18Company(t, configured, companyName)
		if company.PriorityGroup != "secondary" || len(company.Sources) != 1 {
			t.Fatalf("%s must be one secondary source: %#v", companyName, company)
		}
		source := company.Sources[0]
		if source.Adapter != expected.adapter || source.EffectiveCoverageStatus() != expected.coverage || source.Enabled != expected.enabled {
			t.Errorf("%s source mismatch: %#v", companyName, source)
		}
		policy, found := configured.ResolveAccessPolicy(source.URL)
		if !found || policy.Mode != expected.policy {
			t.Errorf("%s access policy=%#v found=%v, want %q", companyName, policy, found, expected.policy)
		}
	}
	netas := phase18Company(t, configured, "Netaş")
	if len(netas.Programs) != 1 || netas.Programs[0].Status != "unknown" || netas.Programs[0].Type != "internship" {
		t.Fatalf("Netaş COOP window must be honest and unknown: %#v", netas.Programs)
	}

	db, repository := openRepository(t)
	registerPhase16Catalog(t, repository, configured)
	fixture, err := os.ReadFile("../scraper/testdata/lever/mobileaction-board.html")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://jobs.lever.co/mobile-action" {
			t.Fatalf("unexpected board request %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	board, err := scraper.NewLeverBoardSource("mobileaction-lever-board", "MobileAction", "https://jobs.lever.co/mobile-action", client)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	service := orchestrator.Service{
		Sources: []scraper.Source{board}, Analyzer: analyzer.NewDeterministicAnalyzer(), Store: repository,
		Profile: analyzer.CandidateProfile{ClassYear: 2, FocusAreas: []string{"backend"}}, Now: func() time.Time { return now },
	}
	first, err := service.Run(ctx, "manual")
	if err != nil || len(first.Sources) != 1 || first.Sources[0].Found != 2 || first.Sources[0].New != 2 {
		t.Fatalf("first MobileAction board scan: result=%#v err=%v", first, err)
	}
	second, err := service.Run(ctx, "manual")
	if err != nil || second.Sources[0].New != 0 {
		t.Fatalf("repeat MobileAction board scan: result=%#v err=%v", second, err)
	}
	var listings, notifications int
	if err := db.QueryRow(`SELECT COUNT(*) FROM listings
		JOIN company_sources ON company_sources.id = listings.source_id
		WHERE company_sources.source_key = 'mobileaction-lever-board'`).Scan(&listings); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if listings != 2 || notifications != 0 {
		t.Fatalf("full-time roles must remain visible and silent: listings=%d notifications=%d", listings, notifications)
	}

	handler := httpapi.NewHandler("*", discardLogger(), nil, repository, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/coverage", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("coverage status=%d body=%s", response.Code, response.Body.String())
	}
	var coverage store.CoverageReport
	if err := json.Unmarshal(response.Body.Bytes(), &coverage); err != nil {
		t.Fatal(err)
	}
	secondary := coverage.PrioritySummaries["secondary"]
	if secondary.TotalCompanies != 19 || secondary.TotalSources != 19 || secondary.AutomaticSources != 6 ||
		secondary.ManualSources != 7 || secondary.ResearchingSources != 6 || secondary.AutomaticCoveragePercent != 50 {
		t.Fatalf("unexpected Phase 18 secondary coverage: %#v", secondary)
	}
	foundProgram := false
	for _, program := range coverage.Programs {
		if program.Company == "Netaş" && program.Status == "unknown" {
			foundProgram = true
		}
	}
	if !foundProgram {
		t.Fatalf("Netaş COOP program missing from coverage: %#v", coverage.Programs)
	}
}

func phase18Company(t *testing.T, configured config.SourcesConfig, name string) config.CompanyConfig {
	t.Helper()
	for _, company := range configured.Companies {
		if company.Name == name {
			return company
		}
	}
	t.Fatalf("company %q is missing", name)
	return config.CompanyConfig{}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
