package orchestrator

import (
	"context"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type SourceResult struct {
	Source       string
	Found        int
	New          int
	ProcessError int
	FetchError   error
}

type ScanResult struct {
	Sources []SourceResult
}

type Service struct {
	Sources  []scraper.Source
	Analyzer analyzer.ListingAnalyzer
	Store    store.ListingRepository
	Profile  analyzer.CandidateProfile
}

func (s Service) Run(ctx context.Context) ScanResult {
	result := ScanResult{Sources: make([]SourceResult, 0, len(s.Sources))}

	for _, source := range s.Sources {
		sourceResult := SourceResult{Source: source.Name()}
		listings, err := source.FetchListings(ctx)
		if err != nil {
			sourceResult.FetchError = err
			result.Sources = append(result.Sources, sourceResult)
			continue
		}

		sourceResult.Found = len(listings)
		for _, listing := range listings {
			listingID, isNew, err := s.Store.UpsertRawListing(ctx, listing)
			if err != nil {
				sourceResult.ProcessError++
				continue
			}
			if !isNew {
				continue
			}

			sourceResult.New++
			analysis, err := s.Analyzer.Analyze(ctx, listing, s.Profile)
			if err != nil {
				sourceResult.ProcessError++
				continue
			}
			if err := s.Store.SaveAnalysis(ctx, listingID, analysis); err != nil {
				sourceResult.ProcessError++
			}
		}

		result.Sources = append(result.Sources, sourceResult)
	}

	return result
}
