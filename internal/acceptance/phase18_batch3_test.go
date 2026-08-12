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

func TestPhase18Batch3UsesOnlyVerifiedAutomaticSources(t *testing.T) {
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatalf("load production sources: %v", err)
	}
	want := map[string]struct {
		adapter, coverage, policy string
		enabled                   bool
	}{
		"Etiya":                    {adapter: "career_links", coverage: "automatic", policy: "robots", enabled: true},
		"Udemy":                    {adapter: "greenhouse", coverage: "automatic", policy: "public_api", enabled: true},
		"OBSS":                     {adapter: "manual", coverage: "manual", policy: "manual_only"},
		"T2 Software":              {adapter: "manual", coverage: "manual", policy: "manual_only"},
		"TaleWorlds Entertainment": {adapter: "manual", coverage: "manual", policy: "manual_only"},
		"LOTEC":                    {adapter: "manual", coverage: "researching", policy: "manual_only"},
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
			t.Errorf("%s policy=%#v found=%v, want %q", companyName, policy, found, expected.policy)
		}
		if source.Enabled && !scraper.SupportsAdapter(source.Adapter) {
			t.Errorf("%s adapter %q is not registered", companyName, source.Adapter)
		}
		if !source.Enabled && (source.CoverageReason == "" || source.CoverageReasonCode == "" || source.LastVerifiedAt == "") {
			t.Errorf("%s disabled source lacks honest evidence metadata: %#v", companyName, source)
		}
	}
}

func TestPhase18Batch3AutomaticSourcesDeduplicateAndNotifyOnlyStrongInternship(t *testing.T) {
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatalf("load production sources: %v", err)
	}
	etiyaFixture, err := os.ReadFile("../scraper/testdata/careerlinks/etiya-positions.html")
	if err != nil {
		t.Fatal(err)
	}
	udemyFixture, err := os.ReadFile("../scraper/testdata/greenhouse/jobs.json")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body []byte
		switch request.URL.Hostname() {
		case "www.etiya.com":
			body = etiyaFixture
		case "boards-api.greenhouse.io":
			body = udemyFixture
		default:
			t.Fatalf("unexpected request URL %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	etiya, err := scraper.NewCareerLinksSourceWithAllowedHosts(
		"etiya-official-open-positions", "Etiya", "https://www.etiya.com/en/career/all-open-positions",
		"open-positions", "/portal/open-positions/", []string{"etiya.peoplebox.biz"}, client,
	)
	if err != nil {
		t.Fatal(err)
	}
	udemy, err := scraper.NewGreenhouseSource(
		"udemy-greenhouse-board", "Udemy", "https://boards-api.greenhouse.io/v1/boards/udemy/jobs", client,
	)
	if err != nil {
		t.Fatal(err)
	}

	db, repository := openRepository(t)
	registerPhase16Catalog(t, repository, configured)
	now := time.Date(2026, 8, 11, 14, 0, 0, 0, time.UTC)
	service := orchestrator.Service{
		Sources: []scraper.Source{etiya, udemy}, Analyzer: analyzer.NewDeterministicAnalyzer(), Store: repository,
		Profile: analyzer.CandidateProfile{ClassYear: 2, FocusAreas: []string{"backend"}}, Now: func() time.Time { return now },
	}
	first, err := service.Run(context.Background(), "manual")
	if err != nil || len(first.Sources) != 2 || first.Sources[0].New != 2 || first.Sources[1].New != 2 {
		t.Fatalf("first Batch 3 scan: result=%#v err=%v", first, err)
	}
	second, err := service.Run(context.Background(), "manual")
	if err != nil || second.Sources[0].New != 0 || second.Sources[1].New != 0 {
		t.Fatalf("repeat Batch 3 scan: result=%#v err=%v", second, err)
	}
	var listings, notifications int
	if err := db.QueryRow(`SELECT COUNT(*) FROM listings
		JOIN company_sources ON company_sources.id = listings.source_id
		WHERE company_sources.source_key IN ('etiya-official-open-positions', 'udemy-greenhouse-board')`).Scan(&listings); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if listings != 4 || notifications != 0 {
		t.Fatalf("expected four visible listings and no single-area push, listings=%d notifications=%d", listings, notifications)
	}
}
