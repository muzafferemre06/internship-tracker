package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/scraper"
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
