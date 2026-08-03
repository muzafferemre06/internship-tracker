package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
	RunID      int64
	Trigger    string
	StartedAt  time.Time
	FinishedAt time.Time
	Status     string
	Sources    []SourceResult
}

type Service struct {
	Sources  []scraper.Source
	Analyzer analyzer.ListingAnalyzer
	Store    store.Repository
	Profile  analyzer.CandidateProfile
	Now      func() time.Time
}

func (s Service) Run(ctx context.Context, trigger string) (ScanResult, error) {
	if s.Store == nil {
		return ScanResult{}, errors.New("scan repository is required")
	}
	startedAt := s.now().UTC()
	runID, err := s.Store.StartScanRun(ctx, trigger, startedAt)
	if err != nil {
		return ScanResult{}, err
	}
	result := ScanResult{
		RunID: runID, Trigger: trigger, StartedAt: startedAt,
		Sources: make([]SourceResult, 0, len(s.Sources)),
	}

	for _, source := range s.Sources {
		sourceResult := SourceResult{Source: source.Name()}
		listings, err := source.FetchListings(ctx)
		if err != nil {
			sourceResult.FetchError = err
			if recordErr := s.Store.RecordSourceFailure(
				context.WithoutCancel(ctx), source.Name(), s.now().UTC(), shortError(err),
			); recordErr != nil {
				sourceResult.ProcessError++
			}
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
		finishedAt := s.now().UTC()
		if sourceResult.ProcessError > 0 {
			reason := fmt.Sprintf("%d listing processing error(s)", sourceResult.ProcessError)
			if err := s.Store.RecordSourceFailure(context.WithoutCancel(ctx), source.Name(), finishedAt, reason); err != nil {
				sourceResult.ProcessError++
			}
		} else if err := s.Store.RecordSourceSuccess(context.WithoutCancel(ctx), source.Name(), finishedAt); err != nil {
			sourceResult.ProcessError++
		}

		result.Sources = append(result.Sources, sourceResult)
	}

	result.FinishedAt = s.now().UTC()
	completion := summarize(result)
	result.Status = completion.Status
	if err := s.Store.FinishScanRun(context.WithoutCancel(ctx), result.RunID, completion); err != nil {
		return result, err
	}
	return result, nil
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

type sourceError struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

func summarize(result ScanResult) store.ScanCompletion {
	completion := store.ScanCompletion{FinishedAt: result.FinishedAt}
	errorsBySource := make([]sourceError, 0)
	for _, source := range result.Sources {
		completion.NewListings += source.New
		if source.FetchError == nil && source.ProcessError == 0 {
			completion.SourcesSucceeded++
			continue
		}
		completion.SourcesFailed++
		reason := ""
		if source.FetchError != nil {
			reason = shortError(source.FetchError)
		} else {
			reason = fmt.Sprintf("%d listing processing error(s)", source.ProcessError)
		}
		errorsBySource = append(errorsBySource, sourceError{Source: source.Source, Error: reason})
	}
	switch {
	case completion.SourcesFailed == 0:
		completion.Status = "completed"
	case completion.SourcesSucceeded == 0:
		completion.Status = "failed"
	default:
		completion.Status = "partial"
	}
	if len(errorsBySource) > 0 {
		encoded, _ := json.Marshal(errorsBySource)
		completion.ErrorSummary = string(encoded)
	}
	return completion
}

func shortError(err error) string {
	message := strings.TrimSpace(err.Error())
	const maxErrorBytes = 500
	if len(message) > maxErrorBytes {
		return message[:maxErrorBytes]
	}
	return message
}
