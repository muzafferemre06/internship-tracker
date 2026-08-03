package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type fakeSource struct {
	name     string
	listings []domain.RawListing
	err      error
}

func (f fakeSource) Name() string { return f.name }
func (f fakeSource) FetchListings(context.Context) ([]domain.RawListing, error) {
	return f.listings, f.err
}

type fakeAnalyzer struct{}

func (fakeAnalyzer) Analyze(
	context.Context,
	domain.RawListing,
	analyzer.CandidateProfile,
) (domain.ListingAnalysis, error) {
	return domain.ListingAnalysis{Relevant: true, Eligibility: domain.EligibilitySuitable}, nil
}

type fakeStore struct {
	seen map[string]bool
}

func (f *fakeStore) UpsertRawListing(
	_ context.Context,
	listing domain.RawListing,
) (string, bool, error) {
	if f.seen[listing.URL] {
		return listing.URL, false, nil
	}
	f.seen[listing.URL] = true
	return listing.URL, true, nil
}

func (*fakeStore) SaveAnalysis(context.Context, string, domain.ListingAnalysis) error {
	return nil
}

func TestRunContinuesAfterSourceFailure(t *testing.T) {
	service := Service{
		Sources: []scraper.Source{
			fakeSource{name: "broken", err: errors.New("unavailable")},
			fakeSource{name: "working", listings: []domain.RawListing{{URL: "https://example.test/1"}}},
		},
		Analyzer: fakeAnalyzer{},
		Store:    &fakeStore{seen: map[string]bool{}},
	}

	result := service.Run(context.Background())

	if len(result.Sources) != 2 {
		t.Fatalf("expected two source results, got %d", len(result.Sources))
	}
	if result.Sources[0].FetchError == nil {
		t.Fatal("expected first source to fail")
	}
	if result.Sources[1].New != 1 {
		t.Fatalf("expected second source to process one listing, got %d", result.Sources[1].New)
	}
}

func TestMeteksanVerticalSliceDoesNotCountSecondScanAsNew(t *testing.T) {
	fixture, err := os.ReadFile("../scraper/testdata/kariyernet/meteksan-listings.html")
	if err != nil {
		t.Fatalf("read scraper fixture: %v", err)
	}
	client := &http.Client{Transport: orchestratorRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(fixture)),
			Request:    request,
		}, nil
	})}
	source, err := scraper.NewKariyerNetSource(
		"meteksan-kariyer-net",
		"Meteksan Savunma",
		"https://www.kariyer.net/firma-profil/meteksan-savunma",
		client,
	)
	if err != nil {
		t.Fatalf("create scraper: %v", err)
	}

	db, err := database.Open(
		context.Background(),
		filepath.Join(t.TempDir(), "tracker.db"),
		os.DirFS("../../migrations"),
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository, err := store.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	if err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "meteksan-kariyer-net", Company: "Meteksan Savunma", PriorityGroup: "primary",
		Type: "career_page", URL: "https://www.kariyer.net/firma-profil/meteksan-savunma",
		Adapter: "kariyer_net", Enabled: true,
	}); err != nil {
		t.Fatalf("register source: %v", err)
	}

	service := Service{
		Sources: []scraper.Source{source}, Analyzer: analyzer.NewDeterministicAnalyzer(), Store: repository,
		Profile: analyzer.CandidateProfile{ClassYear: 2, FocusAreas: []string{"backend", "system_administration"}},
	}
	first := service.Run(context.Background())
	second := service.Run(context.Background())

	if len(first.Sources) != 1 || first.Sources[0].Found != 2 || first.Sources[0].New != 2 {
		t.Fatalf("unexpected first scan: %#v", first)
	}
	if len(second.Sources) != 1 || second.Sources[0].Found != 2 || second.Sources[0].New != 0 {
		t.Fatalf("second scan counted duplicates as new: %#v", second)
	}
	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	if len(dashboard.NewListings) != 1 || dashboard.NewListings[0].Title != "Yazılım Geliştirme Stajyeri" {
		t.Fatalf("unexpected dashboard: %#v", dashboard)
	}
}

type orchestratorRoundTripFunc func(*http.Request) (*http.Response, error)

func (function orchestratorRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
