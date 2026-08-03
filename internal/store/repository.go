package store

import (
	"context"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type DashboardListing struct {
	ID          string                   `json:"id"`
	Company     string                   `json:"company"`
	Title       string                   `json:"title"`
	URL         string                   `json:"url"`
	Priority    string                   `json:"priority"`
	Eligibility domain.EligibilityStatus `json:"eligibility,omitempty"`
}

type ScanSummary struct {
	ID               int64     `json:"id"`
	FinishedAt       time.Time `json:"finished_at"`
	Status           string    `json:"status"`
	SourcesSucceeded int       `json:"sources_succeeded"`
	SourcesFailed    int       `json:"sources_failed"`
	NewListings      int       `json:"new_listings_count"`
	ErrorSummary     string    `json:"error_summary,omitempty"`
}

type DashboardSnapshot struct {
	NewListings        []DashboardListing `json:"new_listings"`
	NeedsDecision      []DashboardListing `json:"needs_decision"`
	ActiveApplications []DashboardListing `json:"active_applications"`
	LastScan           *ScanSummary       `json:"last_scan"`
}

type ListingRepository interface {
	UpsertRawListing(ctx context.Context, listing domain.RawListing) (listingID string, isNew bool, err error)
	SaveAnalysis(ctx context.Context, listingID string, analysis domain.ListingAnalysis) error
}

type ScanCompletion struct {
	FinishedAt       time.Time
	Status           string
	SourcesSucceeded int
	SourcesFailed    int
	NewListings      int
	ErrorSummary     string
}

type ScanRepository interface {
	StartScanRun(ctx context.Context, trigger string, startedAt time.Time) (int64, error)
	FinishScanRun(ctx context.Context, runID int64, completion ScanCompletion) error
	RecordSourceSuccess(ctx context.Context, sourceKey string, finishedAt time.Time) error
	RecordSourceFailure(ctx context.Context, sourceKey string, finishedAt time.Time, reason string) error
}

type AccessDecision struct {
	Allowed   bool
	RetryAt   *time.Time
	Reason    string
	FailCount int
}

type AccessFailure struct {
	StatusCode int
	RetryAfter *time.Time
	Server     string
	CFRay      string
	Reason     string
}

type AccessRepository interface {
	ReserveSourceAccess(
		ctx context.Context,
		scope string,
		attemptedAt time.Time,
		minimumInterval time.Duration,
	) (AccessDecision, error)
	RecordSourceAccessFailure(
		ctx context.Context,
		scope string,
		failedAt time.Time,
		failure AccessFailure,
		baseCooldown time.Duration,
		maximumCooldown time.Duration,
	) (AccessDecision, error)
	RecordSourceAccessSuccess(ctx context.Context, scope string, succeededAt time.Time) error
}

type Repository interface {
	ListingRepository
	ScanRepository
	AccessRepository
}

type DashboardRepository interface {
	Dashboard(ctx context.Context) (DashboardSnapshot, error)
}
