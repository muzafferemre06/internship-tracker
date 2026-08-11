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
	"github.com/muzaffer/internship-tracker/internal/config"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
)

func TestPhase18Batch2OfficialATSConfiguration(t *testing.T) {
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatalf("load production sources: %v", err)
	}
	want := map[string]struct {
		adapter string
		host    string
	}{
		"Binalyze": {adapter: "ashby_board", host: "api.ashbyhq.com"},
		"Insider":  {adapter: "lever_board", host: "jobs.lever.co"},
	}
	for companyName, expected := range want {
		company := phase18Company(t, configured, companyName)
		if company.PriorityGroup != "secondary" || company.EffectiveTrackingStatus() != "active" || len(company.Sources) != 1 {
			t.Fatalf("%s must be one active secondary source: %#v", companyName, company)
		}
		source := company.Sources[0]
		if source.Adapter != expected.adapter || source.EffectiveStrategy() == "" || source.EffectiveCoverageStatus() != "automatic" ||
			!source.Enabled || source.EffectiveTrustLevel() != "official_ats" {
			t.Errorf("%s source mismatch: %#v", companyName, source)
		}
		policy, found := configured.ResolveAccessPolicy(source.URL)
		if !found || policy.Domain != expected.host || (policy.Mode != "public_api" && policy.Mode != "robots") {
			t.Errorf("%s policy=%#v found=%v", companyName, policy, found)
		}
		if !scraper.SupportsAdapter(source.Adapter) {
			t.Errorf("%s adapter %q is not registered", companyName, source.Adapter)
		}
	}
}

func TestPhase18Batch2BoardsDeduplicateAndKeepFullTimeRolesSilent(t *testing.T) {
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatalf("load production sources: %v", err)
	}
	ashbyFixture, err := os.ReadFile("../scraper/testdata/ashby/binalyze-board.json")
	if err != nil {
		t.Fatal(err)
	}
	leverFixture, err := os.ReadFile("../scraper/testdata/lever/insiderone-board.html")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body []byte
		switch request.URL.String() {
		case "https://api.ashbyhq.com/posting-api/job-board/binalyze":
			body = ashbyFixture
		case "https://jobs.lever.co/insiderone":
			body = leverFixture
		default:
			t.Fatalf("unexpected request URL %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	ashby, err := scraper.NewAshbyBoardSource("binalyze-ashby-board", "Binalyze", "https://api.ashbyhq.com/posting-api/job-board/binalyze", client)
	if err != nil {
		t.Fatal(err)
	}
	lever, err := scraper.NewLeverBoardSource("insiderone-lever-board", "Insider", "https://jobs.lever.co/insiderone", client)
	if err != nil {
		t.Fatal(err)
	}

	db, repository := openRepository(t)
	registerPhase16Catalog(t, repository, configured)
	now := time.Date(2026, 8, 11, 13, 0, 0, 0, time.UTC)
	service := orchestrator.Service{
		Sources: []scraper.Source{ashby, lever}, Analyzer: analyzer.NewDeterministicAnalyzer(), Store: repository,
		Profile: analyzer.CandidateProfile{ClassYear: 2, FocusAreas: []string{"backend"}}, Now: func() time.Time { return now },
	}
	first, err := service.Run(context.Background(), "manual")
	if err != nil || len(first.Sources) != 2 || first.Sources[0].New != 2 || first.Sources[1].New != 2 {
		t.Fatalf("first ATS scan: result=%#v err=%v", first, err)
	}
	second, err := service.Run(context.Background(), "manual")
	if err != nil || second.Sources[0].New != 0 || second.Sources[1].New != 0 {
		t.Fatalf("repeat ATS scan: result=%#v err=%v", second, err)
	}
	var listings, notifications int
	if err := db.QueryRow(`SELECT COUNT(*) FROM listings
		JOIN company_sources ON company_sources.id = listings.source_id
		WHERE company_sources.source_key IN ('binalyze-ashby-board', 'insiderone-lever-board')`).Scan(&listings); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if listings != 4 || notifications != 0 {
		t.Fatalf("full-time ATS roles must remain visible and silent: listings=%d notifications=%d", listings, notifications)
	}
}
