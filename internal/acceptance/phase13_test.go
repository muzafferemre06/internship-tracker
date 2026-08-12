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

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/httpapi"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/push"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type phase13Fixture struct {
	Listings []phase13FixtureListing `json:"listings"`
}

type phase13FixtureListing struct {
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	RawText  string `json:"raw_text"`
	Location string `json:"location"`
}

type phase13Source struct {
	name    string
	listing domain.RawListing
}

func (s phase13Source) Name() string { return s.name }
func (s phase13Source) FetchListings(context.Context) ([]domain.RawListing, error) {
	return []domain.RawListing{s.listing}, nil
}

type phase13Analyzer struct {
	locations map[string]string
}

func (a phase13Analyzer) Analyze(_ context.Context, listing domain.RawListing, _ analyzer.CandidateProfile) (domain.ListingAnalysis, error) {
	return domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		Location: a.locations[listing.SourceID], Eligibility: domain.EligibilitySuitable,
		Summary: "Uygun yazılım stajı", Confidence: 0.96,
	}, nil
}

func TestPhase13CrossSourceFixtureProducesOneOpportunityAndOnePush(t *testing.T) {
	fixture := loadPhase13Fixture(t)
	db, repository := openRepository(t)
	ctx := context.Background()
	sources := registerPhase13Fixture(t, repository, fixture, "primary")
	if _, err := repository.UpsertPushSubscription(ctx, store.PushSubscriptionInput{
		Endpoint: "https://push.example.test/phase13-acceptance", P256DH: "fixture-key", Auth: "fixture-auth",
	}); err != nil {
		t.Fatal(err)
	}
	locations := make(map[string]string, len(fixture.Listings))
	for _, listing := range fixture.Listings {
		locations[listing.SourceID] = listing.Location
	}
	service := &orchestrator.Service{Sources: sources, Analyzer: phase13Analyzer{locations: locations}, Store: repository}
	result, err := service.Run(ctx, "manual")
	if err != nil {
		t.Fatalf("run cross-source fixture: %v", err)
	}
	if len(result.Sources) != 2 || result.Sources[0].New != 1 || result.Sources[1].New != 1 {
		t.Fatalf("both source observations must be retained: %#v", result.Sources)
	}

	handler := httpapi.NewHandler("*", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, repository, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard handler status=%d body=%s", response.Code, response.Body.String())
	}
	var dashboard store.DashboardSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &dashboard); err != nil {
		t.Fatal(err)
	}
	if len(dashboard.NewListings) != 1 || dashboard.NewListings[0].OpportunityID == "" {
		t.Fatalf("dashboard did not collapse duplicate sources: %#v", dashboard.NewListings)
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
		t.Fatalf("same opportunity from two sources produced %d pushes", len(sender.messages))
	}

	var listings, activeOpportunities, notifications int
	if err := db.QueryRow("SELECT COUNT(*) FROM listings").Scan(&listings); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM opportunities WHERE status = 'active'").Scan(&activeOpportunities); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if listings != 2 || activeOpportunities != 1 || notifications != 0 {
		t.Fatalf("unexpected canonical evidence: listings=%d opportunities=%d notifications=%d", listings, activeOpportunities, notifications)
	}
}

func TestPhase13MissingLocationEvidenceStaysSeparate(t *testing.T) {
	fixture := loadPhase13Fixture(t)
	_, repository := openRepository(t)
	sources := registerPhase13Fixture(t, repository, fixture, "secondary")
	locations := map[string]string{
		fixture.Listings[0].SourceID: fixture.Listings[0].Location,
		fixture.Listings[1].SourceID: "",
	}
	service := &orchestrator.Service{Sources: sources, Analyzer: phase13Analyzer{locations: locations}, Store: repository}
	if _, err := service.Run(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboard.NewListings) != 2 {
		t.Fatalf("ambiguous location evidence must stay separate: %#v", dashboard.NewListings)
	}
}

func loadPhase13Fixture(t *testing.T) phase13Fixture {
	t.Helper()
	contents, err := os.ReadFile("testdata/phase13/cross-source-listings.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture phase13Fixture
	if err := json.Unmarshal(contents, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Listings) != 2 {
		t.Fatalf("phase 13 fixture must contain two source observations: %#v", fixture)
	}
	return fixture
}

func registerPhase13Fixture(t *testing.T, repository *store.SQLiteRepository, fixture phase13Fixture, priority string) []scraper.Source {
	t.Helper()
	sources := make([]scraper.Source, 0, len(fixture.Listings))
	for _, item := range fixture.Listings {
		if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
			Key: item.SourceID, Company: "Meteksan Savunma", PriorityGroup: priority,
			Type: "fixture", URL: "https://source.example.test/" + item.SourceID,
			Adapter: "fixture", Enabled: true, TrustLevel: "official_company",
		}); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, phase13Source{name: item.SourceID, listing: domain.RawListing{
			Company: "Meteksan Savunma", SourceID: item.SourceID, Title: item.Title,
			URL: item.URL, RawText: item.RawText,
		}})
	}
	return sources
}
