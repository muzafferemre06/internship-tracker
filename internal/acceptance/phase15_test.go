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

type phase15Source struct {
	name    string
	listing domain.RawListing
}

func (s phase15Source) Name() string { return s.name }
func (s phase15Source) FetchListings(context.Context) ([]domain.RawListing, error) {
	return []domain.RawListing{s.listing}, nil
}

type phase15Analyzer struct{}

func (phase15Analyzer) Analyze(context.Context, domain.RawListing, analyzer.CandidateProfile) (domain.ListingAnalysis, error) {
	return domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		Eligibility: domain.EligibilitySuitable, Summary: "Güçlü uygun staj eşleşmesi", Confidence: 0.96,
	}, nil
}

func TestPhase15PrimaryCoverageTrustAndProgramWindowEndToEnd(t *testing.T) {
	ctx := context.Background()
	configured, err := config.LoadSources("../../configs/sources.json")
	if err != nil {
		t.Fatalf("load production sources: %v", err)
	}
	db, repository := openRepository(t)
	for _, company := range configured.Companies {
		for _, source := range company.Sources {
			registration := domain.SourceRegistration{
				Key: source.ID, Company: company.Name, PriorityGroup: company.PriorityGroup,
				Type: source.Type, URL: source.URL, Adapter: source.Adapter,
				Strategy: source.EffectiveStrategy(), TrackingStatus: company.EffectiveTrackingStatus(),
				Enabled: source.Enabled, CoverageStatus: source.EffectiveCoverageStatus(),
				CoverageReason: source.CoverageReason, TrustLevel: source.EffectiveTrustLevel(),
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

	fixture, err := os.ReadFile("../scraper/testdata/lever/commencis-posting.html")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	commencis, err := scraper.NewLeverSource("commencis-lever-spring-boot-camp-2026", "Commencis", commencisURL, client)
	if err != nil {
		t.Fatal(err)
	}
	aggregator := phase15Source{name: "meteksan-kariyer-net", listing: domain.RawListing{
		Company: "Meteksan", SourceID: "meteksan-kariyer-net", Title: "Backend Stajyeri",
		URL: "https://www.kariyer.net/is-ilani/meteksan-backend-stajyeri-15", RawText: "Backend staj programı",
	}}
	if _, err := repository.UpsertPushSubscription(ctx, store.PushSubscriptionInput{
		Endpoint: "https://push.example.test/phase15", P256DH: "fixture-key", Auth: "fixture-auth",
	}); err != nil {
		t.Fatal(err)
	}
	service := &orchestrator.Service{Sources: []scraper.Source{commencis, aggregator}, Analyzer: phase15Analyzer{}, Store: repository}
	first, err := service.Run(ctx, "manual")
	if err != nil || len(first.Sources) != 2 || first.Sources[0].New+first.Sources[1].New != 2 {
		t.Fatalf("first phase 15 scan: result=%#v err=%v", first, err)
	}
	second, err := service.Run(ctx, "manual")
	if err != nil || len(second.Sources) != 2 || second.Sources[0].New+second.Sources[1].New != 0 {
		t.Fatalf("second phase 15 scan: result=%#v err=%v", second, err)
	}

	sender := &acceptancePushSender{}
	dispatcher, err := push.NewDispatcher(repository, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.DispatchPending(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 0 {
		t.Fatalf("high-trust source must produce one push while aggregator stays silent, got %d", len(sender.messages))
	}
	var listings, notifications int
	if err := db.QueryRow("SELECT COUNT(*) FROM listings").Scan(&listings); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if listings != 2 || notifications != 0 {
		t.Fatalf("candidate visibility/trust guard mismatch: listings=%d notifications=%d", listings, notifications)
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
	primaryCoverage := coverage.PrioritySummaries["primary"]
	if primaryCoverage.TotalCompanies != 12 || primaryCoverage.AutomaticSources != 6 ||
		primaryCoverage.ManualSources != 4 || primaryCoverage.ResearchingSources != 4 {
		t.Fatalf("unexpected primary coverage: %#v", primaryCoverage)
	}
	turkcellFound := false
	for _, program := range coverage.Programs {
		if program.Company == "Turkcell" && program.Status == "closed" {
			turkcellFound = true
		}
	}
	if !turkcellFound {
		t.Fatalf("Turkcell program window missing: %#v", coverage.Programs)
	}
}

func phase15Time(t *testing.T, value string) *time.Time {
	t.Helper()
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return &parsed
}
