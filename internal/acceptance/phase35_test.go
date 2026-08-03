package acceptance_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/httpapi"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

const (
	commencisURL   = "https://jobs.lever.co/commencis/04a5cd98-ab26-4b48-bb64-3397ffe79a55"
	commencisTitle = "Spring Boot Development Camp 2026"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type fixtureProvider struct {
	calls int
	input []byte
}

func (p *fixtureProvider) Name() string { return "fake-google" }

func (p *fixtureProvider) Complete(_ context.Context, request analyzer.ProviderRequest) (analyzer.ProviderResponse, error) {
	p.calls++
	p.input, _ = json.Marshal(request.Input)
	return analyzer.ProviderResponse{
		Content: `{"opportunity_type":"staj","application_open":true,"relevant":true,"matching_areas":["backend"],"class_requirement":3,"gpa_requirement":null,"location":"Istanbul, Turkey","work_model":"uzaktan","eligibility":"kismen_uygun","application_due_at":"2026-08-14T23:59:59Z","summary":"Backend odaklı program, ancak üçüncü sınıf şartı kullanıcı kararını gerektiriyor.","confidence":0.94,"needs_user_decision":false,"decision_question":""}`,
		Usage:   analyzer.ProviderUsage{PromptTokens: 211, CompletionTokens: 97, TotalTokens: 308},
	}, nil
}

func TestPhase35FixturePathPersistsAnalysisAndDeduplicates(t *testing.T) {
	fixture, err := os.ReadFile("../scraper/testdata/lever/commencis-posting.html")
	if err != nil {
		t.Fatalf("read Lever fixture: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(bytes.NewReader(fixture)),
		}, nil
	})}
	source, err := scraper.NewLeverSource("commencis-lever-spring-boot-camp-2026", "Commencis", commencisURL, client)
	if err != nil {
		t.Fatalf("create fixture source: %v", err)
	}
	provider := &fixtureProvider{}
	modelAnalyzer, err := analyzer.NewModelAnalyzer(provider, "fixture-strict-model", analyzer.CostRates{
		InputPerMillionUSD: 0.25, OutputPerMillionUSD: 1.50,
	})
	if err != nil {
		t.Fatalf("create fixture analyzer: %v", err)
	}
	db, repository := openRepository(t)
	registerSource(t, repository, commencisURL)
	service := &orchestrator.Service{
		Sources: []scraper.Source{source}, Analyzer: modelAnalyzer, Store: repository,
		Profile: minimizedAcceptanceProfile(),
	}
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }

	first, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("first acceptance scan: %v", err)
	}
	now = now.Add(time.Second)
	second, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("second acceptance scan: %v", err)
	}
	if first.Status != "completed" || first.Sources[0].New != 1 || second.Sources[0].New != 0 {
		t.Fatalf("unexpected dedup results: first=%#v second=%#v", first, second)
	}
	if provider.calls != 1 {
		t.Fatalf("processed duplicate must not be analyzed again, calls=%d", provider.calls)
	}
	input := string(provider.input)
	if strings.Contains(input, "Bilkent") || strings.Contains(input, "ODTU") || !strings.Contains(input, `"education_field":"CTIS"`) {
		t.Fatalf("model input was not minimized: %s", input)
	}

	record := readAcceptanceRecord(t, db)
	if record.Count != 1 || record.Provider != "fake-google" || record.Model != "fixture-strict-model" ||
		record.PromptTokens != 211 || record.CompletionTokens != 97 || record.TotalTokens != 308 || record.EstimatedCostUSD <= 0 {
		t.Fatalf("analysis metadata was not persisted: %#v", record)
	}
	assertDashboardAPI(t, service, repository, commencisTitle, domain.EligibilityPartlySuitable)
}

type acceptanceRecord struct {
	Count             int
	Provider          string
	Model             string
	PromptTokens      int
	CompletionTokens  int
	TotalTokens       int
	EstimatedCostUSD  float64
	CanonicalURL      string
	FirstSeenAt       string
	LastSeenAt        string
	EligibilityStatus string
	Summary           string
	DecisionQuestion  string
}

func openRepository(t *testing.T) (*sql.DB, *store.SQLiteRepository) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "phase35.db"), os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("open acceptance database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository, err := store.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("create acceptance repository: %v", err)
	}
	return db, repository
}

func registerSource(t *testing.T, repository *store.SQLiteRepository, sourceURL string) {
	t.Helper()
	err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "commencis-lever-spring-boot-camp-2026", Company: "Commencis", PriorityGroup: "candidate",
		Type: "official_ats_posting", URL: sourceURL, Adapter: "lever", Enabled: true,
	})
	if err != nil {
		t.Fatalf("register acceptance source: %v", err)
	}
}

func minimizedAcceptanceProfile() analyzer.CandidateProfile {
	return analyzer.CandidateProfile{
		EducationField: "CTIS", ClassYear: 2, GPA: 3.97,
		FocusAreas:      []string{"backend", "network", "system_administration"},
		ExperienceAreas: []string{"autonomous_software", "ground_control_station"},
		Locations:       []string{"Ankara", "remote"},
	}
}

func readAcceptanceRecord(t *testing.T, db *sql.DB) acceptanceRecord {
	t.Helper()
	var result acceptanceRecord
	err := db.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(listing_analyses.provider), ''),
			COALESCE(MAX(listing_analyses.model), ''),
			COALESCE(MAX(listing_analyses.prompt_tokens), 0),
			COALESCE(MAX(listing_analyses.completion_tokens), 0),
			COALESCE(MAX(listing_analyses.total_tokens), 0),
			COALESCE(MAX(listing_analyses.estimated_cost_usd), 0),
			COALESCE(MAX(listings.canonical_url), ''),
			COALESCE(MAX(listings.first_seen_at), ''), COALESCE(MAX(listings.last_seen_at), ''),
			COALESCE(MAX(listing_analyses.eligibility_status), ''),
			COALESCE(MAX(listing_analyses.summary), ''),
			COALESCE(MAX(listing_analyses.decision_question), '')
		FROM listings
		JOIN listing_analyses ON listing_analyses.listing_id = listings.id
	`).Scan(&result.Count, &result.Provider, &result.Model, &result.PromptTokens,
		&result.CompletionTokens, &result.TotalTokens, &result.EstimatedCostUSD,
		&result.CanonicalURL, &result.FirstSeenAt, &result.LastSeenAt, &result.EligibilityStatus,
		&result.Summary, &result.DecisionQuestion)
	if err != nil {
		t.Fatalf("read acceptance record: %v", err)
	}
	return result
}

func readAcceptanceFailure(t *testing.T, db *sql.DB) string {
	t.Helper()
	var reason string
	if err := db.QueryRow(`
		SELECT COALESCE(MAX(last_error), '') FROM listing_analyses
	`).Scan(&reason); err != nil {
		t.Fatalf("read acceptance failure: %v", err)
	}
	return reason
}

func assertDashboardAPI(
	t *testing.T,
	service *orchestrator.Service,
	repository *store.SQLiteRepository,
	expectedTitle string,
	expectedEligibility domain.EligibilityStatus,
) {
	t.Helper()
	handler := httpapi.NewHandler("*", slog.New(slog.NewTextHandler(io.Discard, nil)), service, repository)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard status=%d body=%s", response.Code, response.Body.String())
	}
	var dashboard store.DashboardSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &dashboard); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	listings := dashboard.NewListings
	if expectedEligibility == domain.EligibilityNeedsDecision {
		listings = dashboard.NeedsDecision
	}
	if len(listings) != 1 || listings[0].Title != expectedTitle ||
		listings[0].Eligibility != expectedEligibility {
		t.Fatalf("listing is not visible in dashboard API: %#v", dashboard)
	}
}
