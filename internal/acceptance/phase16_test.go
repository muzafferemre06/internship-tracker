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
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/httpapi"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/push"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

func TestPhase16SecondaryCatalogCoverageAndStrongMatchNotificationEndToEnd(t *testing.T) {
	ctx := context.Background()
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatalf("load production sources: %v", err)
	}
	db, repository := openRepository(t)
	registerPhase16Catalog(t, repository, configured)

	fixture, err := os.ReadFile("testdata/phase16/evreka-career.html")
	if err != nil {
		t.Fatal(err)
	}
	pageClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://evreka.co/career/" {
			t.Fatalf("unexpected career request %s", request.URL)
		}
		return phase16Response(http.StatusOK, fixture), nil
	})}
	robotsClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://evreka.co/robots.txt" {
			t.Fatalf("unexpected robots request %s", request.URL)
		}
		return phase16Response(http.StatusOK, []byte("User-agent: *\nAllow: /career/\n")), nil
	})}
	careerSource, err := scraper.NewCareerLinksSource(
		"evreka-official-career", "Evreka", "https://evreka.co/career/", "", "/career/", pageClient,
	)
	if err != nil {
		t.Fatal(err)
	}
	policy := scraper.AccessPolicy{
		Mode: "robots", Scope: "evreka.co", TargetURL: "https://evreka.co/career/",
		MinimumInterval: 24 * time.Hour, BaseCooldown: time.Hour, MaximumCooldown: 24 * time.Hour,
	}
	if _, err := repository.UpsertPushSubscription(ctx, store.PushSubscriptionInput{
		Endpoint: "https://push.example.test/phase16", P256DH: "fixture-key", Auth: "fixture-auth",
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	service := orchestrator.Service{
		Sources:  []scraper.Source{scraper.WithAccessPolicy(careerSource, policy)},
		Analyzer: analyzer.NewDeterministicAnalyzer(), Store: repository,
		Robots:  scraper.NewHTTPRobotsChecker(robotsClient, func() time.Time { return now }),
		Profile: analyzer.CandidateProfile{ClassYear: 2, FocusAreas: []string{"backend"}},
		Now:     func() time.Time { return now },
	}
	first, err := service.Run(ctx, "scheduled")
	if err != nil || len(first.Sources) != 1 || first.Sources[0].Found != 2 || first.Sources[0].New != 2 {
		t.Fatalf("first Phase 16 scan: result=%#v err=%v", first, err)
	}
	now = now.Add(25 * time.Hour)
	second, err := service.Run(ctx, "scheduled")
	if err != nil || len(second.Sources) != 1 || second.Sources[0].New != 0 {
		t.Fatalf("repeat Phase 16 scan: result=%#v err=%v", second, err)
	}

	sender := &acceptancePushSender{}
	dispatcher, err := push.NewDispatcher(repository, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.DispatchPending(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("only backend focus match should push, got %d messages", len(sender.messages))
	}
	history, err := repository.OpportunityHistory(ctx, store.OpportunityHistoryQuery{Page: 1, PageSize: 20})
	if err != nil || history.Total != 2 {
		t.Fatalf("weak secondary candidate must remain visible: history=%#v err=%v", history, err)
	}
	var listings, analyses, notifications int
	var eventType string
	if err := db.QueryRow("SELECT COUNT(*) FROM listings").Scan(&listings); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM listing_analyses").Scan(&analyses); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*), MIN(event_type) FROM notifications").Scan(&notifications, &eventType); err != nil {
		t.Fatal(err)
	}
	if listings != 2 || analyses != 2 || notifications != 1 || eventType != domain.NewSecondaryStrongMatchEvent {
		t.Fatalf("unexpected persisted Phase 16 state: listings=%d analyses=%d notifications=%d event=%q", listings, analyses, notifications, eventType)
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
	secondary := coverage.PrioritySummaries["secondary"]
	if secondary.TotalCompanies != 21 || secondary.TotalSources != 21 || secondary.AutomaticSources != 8 ||
		secondary.ManualSources != 7 || secondary.ResearchingSources != 6 || secondary.AutomaticCoveragePercent != 57.142857142857146 {
		t.Fatalf("unexpected secondary coverage: %#v", secondary)
	}
}

func registerPhase16Catalog(t *testing.T, repository *store.SQLiteRepository, configured config.SourcesConfig) {
	t.Helper()
	ctx := context.Background()
	for _, company := range configured.Companies {
		for _, source := range company.Sources {
			registration := domain.SourceRegistration{
				Key: source.ID, Company: company.Name, PriorityGroup: company.PriorityGroup,
				Type: source.Type, URL: source.URL, Adapter: source.Adapter,
				Strategy: source.EffectiveStrategy(), TrackingStatus: company.EffectiveTrackingStatus(),
				TrackingPhase: company.TrackingPhase,
				Enabled:       source.Enabled, CoverageStatus: source.EffectiveCoverageStatus(),
				CoverageReason: source.CoverageReason, CoverageReasonCode: source.CoverageReasonCode,
				LastVerifiedAt: phase15Time(t, source.LastVerifiedAt), TrustLevel: source.EffectiveTrustLevel(),
			}
			if policy, found := configured.ResolveAccessPolicy(source.URL); found {
				registration.AccessMode = policy.Mode
				registration.AccessScope = policy.Domain
				registration.MinimumInterval = time.Duration(policy.MinimumIntervalSeconds) * time.Second
				registration.BaseCooldown = time.Duration(policy.BaseCooldownSeconds) * time.Second
				registration.MaximumCooldown = time.Duration(policy.MaximumCooldownSeconds) * time.Second
			}
			if err := repository.RegisterSource(ctx, registration); err != nil {
				t.Fatalf("register source %s: %v", source.ID, err)
			}
		}
		for _, program := range company.Programs {
			if err := repository.RegisterProgramWindow(ctx, domain.ProgramWindow{
				Key: program.ID, Company: company.Name, Name: program.Name, Type: program.Type,
				URL: program.URL, Status: program.Status, LastVerifiedAt: phase15Time(t, program.LastVerifiedAt),
			}); err != nil {
				t.Fatalf("register program %s: %v", program.ID, err)
			}
		}
	}
}

func phase16Response(status int, body []byte) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}
}
