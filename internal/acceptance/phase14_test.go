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
	"github.com/muzaffer/internship-tracker/internal/config"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

func TestPhase14ManualSocialAndRobotsDomainPolicyEndToEnd(t *testing.T) {
	ctx := context.Background()
	db, repository := openRepository(t)
	registerConfiguredPhase14LinkedIn(t, repository)

	pageFixture, err := os.ReadFile("../scraper/testdata/jsonld/career-jobposting.html")
	if err != nil {
		t.Fatal(err)
	}
	robotsFixture, err := os.ReadFile("../scraper/testdata/robots/phase14.txt")
	if err != nil {
		t.Fatal(err)
	}
	pageRequests := make(map[string]int)
	pageClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		pageRequests[request.URL.String()]++
		return phase14HTTPResponse(request, http.StatusOK, pageFixture), nil
	})}
	robotsRequests := 0
	robotsClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		robotsRequests++
		if request.URL.String() != "https://careers.example.test/robots.txt" {
			t.Fatalf("unexpected robots request: %s", request.URL)
		}
		return phase14HTTPResponse(request, http.StatusOK, robotsFixture), nil
	})}

	const (
		allowedURL = "https://careers.example.test/jobs/internships/"
		blockedURL = "https://careers.example.test/jobs/private/"
	)
	policy := func(targetURL string) scraper.AccessPolicy {
		return scraper.AccessPolicy{
			Mode: "robots", Scope: "careers.example.test", TargetURL: targetURL,
			MinimumInterval: time.Hour, BaseCooldown: time.Hour, MaximumCooldown: 4 * time.Hour,
		}
	}
	newSource := func(key string, targetURL string) scraper.Source {
		t.Helper()
		if err := repository.RegisterSource(ctx, domain.SourceRegistration{
			Key: key, Company: "Northstar Robotics", PriorityGroup: "secondary", Type: "career_page",
			URL: targetURL, Adapter: "json_ld", Strategy: "structured_data", Enabled: true,
			AccessMode: "robots", AccessScope: "careers.example.test",
			MinimumInterval: time.Hour, BaseCooldown: time.Hour, MaximumCooldown: 4 * time.Hour,
		}); err != nil {
			t.Fatal(err)
		}
		source, err := scraper.NewJSONLDSource(key, "Northstar Robotics", targetURL, pageClient)
		if err != nil {
			t.Fatal(err)
		}
		return scraper.WithAccessPolicy(source, policy(targetURL))
	}
	sources := []scraper.Source{
		newSource("northstar-allowed", allowedURL),
		newSource("northstar-blocked", blockedURL),
	}
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	service := orchestrator.Service{
		Sources: sources, Analyzer: analyzer.NewDeterministicAnalyzer(), Store: repository,
		Robots: scraper.NewHTTPRobotsChecker(robotsClient, func() time.Time { return now }),
		Now:    func() time.Time { return now },
	}

	first, err := service.Run(ctx, "manual")
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if first.Status != "partial" || first.Sources[0].Found != 2 || !first.Sources[1].Skipped ||
		!strings.Contains(first.Sources[1].AccessReason, "robots.txt disallows") {
		t.Fatalf("unexpected first scan: %#v", first)
	}
	if pageRequests[allowedURL] != 1 || pageRequests[blockedURL] != 0 || robotsRequests != 1 {
		t.Fatalf("unexpected HTTP calls: pages=%#v robots=%d", pageRequests, robotsRequests)
	}

	second, err := service.Run(ctx, "manual")
	if err != nil {
		t.Fatalf("repeat scan: %v", err)
	}
	if second.Status != "failed" || !second.Sources[0].Skipped || second.Sources[0].RetryAt == nil ||
		!second.Sources[1].Skipped || second.Sources[1].RetryAt == nil {
		t.Fatalf("persistent domain interval did not block repeat: %#v", second)
	}
	if pageRequests[allowedURL] != 1 || pageRequests[blockedURL] != 0 || robotsRequests != 1 {
		t.Fatalf("repeat scan reached HTTP: pages=%#v robots=%d", pageRequests, robotsRequests)
	}
	var domainStates int
	if err := db.QueryRow("SELECT COUNT(*) FROM source_access_states WHERE scope = ?", "careers.example.test").Scan(&domainStates); err != nil {
		t.Fatal(err)
	}
	if domainStates != 1 {
		t.Fatalf("sources did not share one persistent domain state: %d", domainStates)
	}

	dashboard, err := repository.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboard.Watchlist) != 1 || dashboard.Watchlist[0].SourceID != "havelsan-linkedin" ||
		dashboard.Watchlist[0].AccessMode != "manual_only" ||
		!strings.Contains(dashboard.Watchlist[0].Reason, "otomatik erişim") {
		t.Fatalf("manual LinkedIn source is not explicit on watchlist: %#v", dashboard.Watchlist)
	}
	var blockedReason string
	if err := db.QueryRow("SELECT last_error FROM company_sources WHERE source_key = ?", "northstar-blocked").Scan(&blockedReason); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(blockedReason, "retry after") {
		t.Fatalf("repeat access reason was not persisted: %q", blockedReason)
	}
}

func registerConfiguredPhase14LinkedIn(t *testing.T, repository *store.SQLiteRepository) {
	t.Helper()
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, company := range configured.Companies {
		for _, source := range company.Sources {
			if source.ID != "havelsan-linkedin" {
				continue
			}
			policy, found := configured.ResolveAccessPolicy(source.URL)
			if !found || policy.Mode != "manual_only" || source.Enabled {
				t.Fatalf("configured LinkedIn source is not manual-only: source=%#v policy=%#v", source, policy)
			}
			if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
				Key: source.ID, Company: company.Name, PriorityGroup: company.PriorityGroup,
				Type: source.Type, URL: source.URL, Adapter: source.Adapter, Strategy: source.EffectiveStrategy(),
				TrackingStatus: company.EffectiveTrackingStatus(), Enabled: source.Enabled,
				AccessMode: policy.Mode, AccessScope: policy.Domain,
			}); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("havelsan-linkedin source is missing from configs/sources.json")
}

func phase14HTTPResponse(request *http.Request, statusCode int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode, Header: make(http.Header),
		Body: io.NopCloser(bytes.NewReader(body)), Request: request,
	}
}
