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
	"time"

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
	seen           map[string]bool
	startedTrigger string
	completion     store.ScanCompletion
	succeeded      []string
	failed         []string
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

func (f *fakeStore) StartScanRun(_ context.Context, trigger string, _ time.Time) (int64, error) {
	f.startedTrigger = trigger
	return 42, nil
}

func (f *fakeStore) FinishScanRun(_ context.Context, _ int64, completion store.ScanCompletion) error {
	f.completion = completion
	return nil
}

func (f *fakeStore) RecordSourceSuccess(_ context.Context, sourceKey string, _ time.Time) error {
	f.succeeded = append(f.succeeded, sourceKey)
	return nil
}

func (f *fakeStore) RecordSourceFailure(_ context.Context, sourceKey string, _ time.Time, _ string) error {
	f.failed = append(f.failed, sourceKey)
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

	result, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("run scan: %v", err)
	}

	if len(result.Sources) != 2 {
		t.Fatalf("expected two source results, got %d", len(result.Sources))
	}
	if result.Sources[0].FetchError == nil {
		t.Fatal("expected first source to fail")
	}
	if result.Sources[1].New != 1 {
		t.Fatalf("expected second source to process one listing, got %d", result.Sources[1].New)
	}
	if result.Status != "partial" || service.Store.(*fakeStore).completion.SourcesFailed != 1 {
		t.Fatalf("expected persisted partial report, got %#v", result)
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
	first, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("run first scan: %v", err)
	}
	second, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("run second scan: %v", err)
	}

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

func TestMultiProfileFixtureScanIsolatesFailureAndPersistsPartialReport(t *testing.T) {
	fixtures := map[string][]byte{}
	for _, name := range []string{"aselsan-listings.html", "aselsannet-listings.html", "unrecognized.html"} {
		contents, err := os.ReadFile(filepath.Join("../scraper/testdata/kariyernet", name))
		if err != nil {
			t.Fatalf("read fixture %q: %v", name, err)
		}
		fixtures[name] = contents
	}
	client := &http.Client{Transport: orchestratorRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		fixtureName := "unrecognized.html"
		switch request.URL.Path {
		case "/firma-profil/aselsan":
			fixtureName = "aselsan-listings.html"
		case "/firma-profil/aselsannet":
			fixtureName = "aselsannet-listings.html"
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(fixtures[fixtureName])),
			Request:    request,
		}, nil
	})}
	newSource := func(name, company, pageName, profileURL string) scraper.Source {
		t.Helper()
		source, err := scraper.NewKariyerNetSource(name, company, pageName, profileURL, client)
		if err != nil {
			t.Fatalf("create source %q: %v", name, err)
		}
		return source
	}
	sources := []scraper.Source{
		newSource("stm-kariyer-net", "STM", "STM", "https://www.kariyer.net/firma-profil/stm"),
		newSource("aselsan-kariyer-net", "ASELSAN", "Aselsan Elektronik", "https://www.kariyer.net/firma-profil/aselsan"),
		newSource("aselsannet-kariyer-net", "ASELSAN", "Aselsannet", "https://www.kariyer.net/firma-profil/aselsannet"),
	}

	db, err := database.Open(
		context.Background(), filepath.Join(t.TempDir(), "tracker.db"), os.DirFS("../../migrations"),
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository, err := store.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	for _, registration := range []domain.SourceRegistration{
		{Key: "stm-kariyer-net", Company: "STM", PriorityGroup: "secondary", Type: "career_page", URL: "https://www.kariyer.net/firma-profil/stm", Adapter: "kariyer_net", Enabled: true},
		{Key: "aselsan-kariyer-net", Company: "ASELSAN", PriorityGroup: "primary", Type: "career_page", URL: "https://www.kariyer.net/firma-profil/aselsan", Adapter: "kariyer_net", Enabled: true},
		{Key: "aselsannet-kariyer-net", Company: "ASELSAN", PriorityGroup: "primary", Type: "career_page", URL: "https://www.kariyer.net/firma-profil/aselsannet", Adapter: "kariyer_net", Enabled: true},
	} {
		if err := repository.RegisterSource(context.Background(), registration); err != nil {
			t.Fatalf("register source %q: %v", registration.Key, err)
		}
	}

	service := Service{
		Sources: sources, Analyzer: analyzer.NewDeterministicAnalyzer(), Store: repository,
		Profile: analyzer.CandidateProfile{ClassYear: 2, FocusAreas: []string{"backend", "system_administration"}},
	}
	result, err := service.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("run scan: %v", err)
	}
	if result.Status != "partial" || len(result.Sources) != 3 {
		t.Fatalf("unexpected scan result: %#v", result)
	}
	if result.Sources[0].FetchError == nil || result.Sources[1].New != 1 || result.Sources[2].Found != 2 || result.Sources[2].New != 1 {
		t.Fatalf("failure was not isolated or cross-profile dedup failed: %#v", result.Sources)
	}
	dashboard, err := repository.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	if dashboard.LastScan == nil || dashboard.LastScan.Status != "partial" || dashboard.LastScan.SourcesSucceeded != 2 || dashboard.LastScan.SourcesFailed != 1 || dashboard.LastScan.NewListings != 2 {
		t.Fatalf("unexpected persisted scan report: %#v", dashboard.LastScan)
	}
}

type orchestratorRoundTripFunc func(*http.Request) (*http.Response, error)

func (function orchestratorRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
