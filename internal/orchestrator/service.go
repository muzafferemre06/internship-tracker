package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/matching"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

type SourceResult struct {
	Source       string
	Found        int
	New          int
	ProcessError int
	FetchError   error
	Skipped      bool
	RetryAt      *time.Time
	AccessReason string
}

type ScanResult struct {
	RunID      int64
	Trigger    string
	StartedAt  time.Time
	FinishedAt time.Time
	Status     string
	Sources    []SourceResult
}

type ReprocessResult struct {
	Found     int `json:"found"`
	Processed int `json:"processed"`
	Failed    int `json:"failed"`
}

type Service struct {
	Sources  []scraper.Source
	Analyzer analyzer.ListingAnalyzer
	Store    store.Repository
	Robots   scraper.RobotsChecker
	Profile  analyzer.CandidateProfile
	Now      func() time.Time
}

func (s Service) Run(ctx context.Context, trigger string) (ScanResult, error) {
	if s.Store == nil {
		return ScanResult{}, errors.New("scan repository is required")
	}
	if s.Analyzer == nil {
		return ScanResult{}, errors.New("listing analyzer is required")
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
	accessStates := make(map[string]*accessRunState)
	var runErrors []error

	for _, source := range s.Sources {
		sourceResult := SourceResult{Source: source.Name()}
		if protected, ok := source.(scraper.ProtectedSource); ok {
			policy := protected.AccessPolicy()
			state, exists := accessStates[policy.Scope]
			if !exists {
				decision, reserveErr := s.Store.ReserveSourceAccess(
					ctx, policy.Scope, s.now().UTC(), policy.MinimumInterval,
				)
				state = &accessRunState{decision: decision, reserved: reserveErr == nil && decision.Allowed}
				accessStates[policy.Scope] = state
				if reserveErr != nil {
					state.decision = store.AccessDecision{Allowed: false, Reason: "domain access reservation failed"}
					runErrors = append(runErrors, reserveErr)
				}
			}
			if !state.decision.Allowed {
				sourceResult.Skipped = true
				sourceResult.RetryAt = state.decision.RetryAt
				sourceResult.AccessReason = accessReason(state.decision)
				if err := s.Store.RecordSourceFailure(
					context.WithoutCancel(ctx), source.Name(), s.now().UTC(), sourceResult.AccessReason,
				); err != nil {
					sourceResult.ProcessError++
					runErrors = append(runErrors, err)
				}
				result.Sources = append(result.Sources, sourceResult)
				continue
			}
			if strings.EqualFold(policy.Mode, "robots") {
				robotsDecision := scraper.RobotsDecision{Reason: "robots.txt checker is not configured; access denied"}
				var robotsErr error
				if s.Robots != nil {
					robotsDecision, robotsErr = s.Robots.Check(ctx, policy)
				} else {
					robotsErr = errors.New("robots.txt checker is required for robots access mode")
				}
				if robotsErr != nil || !robotsDecision.Allowed {
					sourceResult.Skipped = true
					sourceResult.AccessReason = strings.TrimSpace(robotsDecision.Reason)
					if sourceResult.AccessReason == "" {
						sourceResult.AccessReason = shortError(robotsErr)
					}
					if robotsErr != nil {
						runErrors = append(runErrors, robotsErr)
					}
					if err := s.Store.RecordSourceFailure(
						context.WithoutCancel(ctx), source.Name(), s.now().UTC(), sourceResult.AccessReason,
					); err != nil {
						sourceResult.ProcessError++
						runErrors = append(runErrors, err)
					}
					result.Sources = append(result.Sources, sourceResult)
					continue
				}
			}
		}

		listings, err := source.FetchListings(ctx)
		if err != nil {
			sourceResult.FetchError = err
			var accessErr *scraper.AccessError
			if protected, ok := source.(scraper.ProtectedSource); ok &&
				errors.As(err, &accessErr) && accessErr.Protective() {
				policy := protected.AccessPolicy()
				decision, recordErr := s.Store.RecordSourceAccessFailure(
					context.WithoutCancel(ctx), policy.Scope, s.now().UTC(), store.AccessFailure{
						StatusCode: accessErr.StatusCode,
						RetryAfter: accessErr.RetryAfter,
						Server:     accessErr.Server,
						CFRay:      accessErr.CFRay,
						Reason:     shortError(err),
					}, policy.BaseCooldown, policy.MaximumCooldown,
				)
				state := accessStates[policy.Scope]
				state.protectionTriggered = true
				if recordErr != nil {
					runErrors = append(runErrors, recordErr)
					decision = store.AccessDecision{Allowed: false, Reason: "domain access protection was triggered"}
				}
				state.decision = decision
				sourceResult.RetryAt = decision.RetryAt
			}
			if recordErr := s.Store.RecordSourceFailure(
				context.WithoutCancel(ctx), source.Name(), s.now().UTC(), shortError(err),
			); recordErr != nil {
				sourceResult.ProcessError++
			}
			result.Sources = append(result.Sources, sourceResult)
			continue
		}
		if protected, ok := source.(scraper.ProtectedSource); ok {
			accessStates[protected.AccessPolicy().Scope].successfulFetch = true
		}

		sourceResult.Found = len(listings)
		for _, listing := range listings {
			listingID, isNew, err := s.Store.UpsertRawListing(ctx, listing)
			if err != nil {
				sourceResult.ProcessError++
				continue
			}
			if isNew {
				sourceResult.New++
			}
			required, err := s.Store.AnalysisRequired(ctx, listingID)
			if err != nil {
				sourceResult.ProcessError++
				continue
			}
			if !required {
				continue
			}

			analysis, err := s.Analyzer.Analyze(ctx, listing, s.Profile)
			if err != nil {
				sourceResult.ProcessError++
				if saveErr := s.Store.SaveAnalysisFailure(
					context.WithoutCancel(ctx), listingID, analyzerIdentity(s.Analyzer), analyzerModel(s.Analyzer), shortError(err),
				); saveErr != nil {
					runErrors = append(runErrors, saveErr)
				}
				continue
			}
			analysis.Assessment = matching.Assess(matchingProfile(s.Profile), matching.Input{Analysis: analysis}).Domain()
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

	for scope, state := range accessStates {
		if !state.reserved || state.protectionTriggered || !state.successfulFetch {
			continue
		}
		if err := s.Store.RecordSourceAccessSuccess(context.WithoutCancel(ctx), scope, s.now().UTC()); err != nil {
			runErrors = append(runErrors, err)
		}
	}

	result.FinishedAt = s.now().UTC()
	completion := summarize(result)
	result.Status = completion.Status
	if err := s.Store.FinishScanRun(context.WithoutCancel(ctx), result.RunID, completion); err != nil {
		return result, err
	}
	return result, errors.Join(runErrors...)
}

func (s Service) ReprocessPending(ctx context.Context, limit int) (ReprocessResult, error) {
	if s.Store == nil || s.Analyzer == nil {
		return ReprocessResult{}, errors.New("analysis repository and analyzer are required")
	}
	pending, err := s.Store.PendingAnalyses(ctx, limit)
	if err != nil {
		return ReprocessResult{}, err
	}
	result := ReprocessResult{Found: len(pending)}
	var processErrors []error
	for _, item := range pending {
		analysis, analyzeErr := s.Analyzer.Analyze(ctx, item.Listing, s.Profile)
		if analyzeErr == nil {
			analysis.Assessment = matching.Assess(matchingProfile(s.Profile), matching.Input{Analysis: analysis}).Domain()
			analyzeErr = s.Store.SaveAnalysis(ctx, item.ListingID, analysis)
		}
		if analyzeErr == nil {
			result.Processed++
			continue
		}
		result.Failed++
		if saveErr := s.Store.SaveAnalysisFailure(
			context.WithoutCancel(ctx), item.ListingID, analyzerIdentity(s.Analyzer), analyzerModel(s.Analyzer), shortError(analyzeErr),
		); saveErr != nil {
			processErrors = append(processErrors, saveErr)
		}
	}
	return result, errors.Join(processErrors...)
}

func matchingProfile(profile analyzer.CandidateProfile) matching.Profile {
	return matching.Profile{
		ClassYear: profile.ClassYear, GPA: profile.GPA,
		FocusAreas:                  append([]string(nil), profile.FocusAreas...),
		PrimaryLocations:            append([]string(nil), profile.Locations...),
		SummerOtherCities:           profile.SummerOtherCities,
		TermTimePartTimeOtherCities: profile.TermTimePartTimeOtherCities,
	}
}

type identifiedAnalyzer interface {
	ProviderName() string
	ModelName() string
}

func analyzerIdentity(value analyzer.ListingAnalyzer) string {
	if identified, ok := value.(identifiedAnalyzer); ok {
		return identified.ProviderName()
	}
	return "deterministic"
}

func analyzerModel(value analyzer.ListingAnalyzer) string {
	if identified, ok := value.(identifiedAnalyzer); ok {
		return identified.ModelName()
	}
	return ""
}

type accessRunState struct {
	decision            store.AccessDecision
	reserved            bool
	protectionTriggered bool
	successfulFetch     bool
}

func accessReason(decision store.AccessDecision) string {
	reason := strings.TrimSpace(decision.Reason)
	if reason == "" {
		reason = "domain access is not allowed yet"
	}
	if decision.RetryAt != nil {
		return fmt.Sprintf("%s; retry after %s", reason, decision.RetryAt.UTC().Format(time.RFC3339))
	}
	return reason
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
		if source.FetchError == nil && source.ProcessError == 0 && !source.Skipped {
			completion.SourcesSucceeded++
			continue
		}
		completion.SourcesFailed++
		reason := ""
		if source.Skipped {
			reason = source.AccessReason
		} else if source.FetchError != nil {
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
