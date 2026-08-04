package store

import (
	"context"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type DashboardListing struct {
	ID                string                   `json:"id"`
	Company           string                   `json:"company"`
	Title             string                   `json:"title"`
	URL               string                   `json:"url"`
	Priority          string                   `json:"priority"`
	Eligibility       domain.EligibilityStatus `json:"eligibility,omitempty"`
	Summary           string                   `json:"summary,omitempty"`
	ApplicationDueAt  *time.Time               `json:"application_deadline,omitempty"`
	ApplicationStatus domain.ApplicationStatus `json:"application_status,omitempty"`
	TrackingDeadline  *time.Time               `json:"tracking_deadline,omitempty"`
	InterviewAt       *time.Time               `json:"interview_at,omitempty"`
}

type ApplicationTracking struct {
	Status      domain.ApplicationStatus `json:"status"`
	Deadline    *time.Time               `json:"deadline,omitempty"`
	InterviewAt *time.Time               `json:"interview_at,omitempty"`
	Notes       string                   `json:"notes,omitempty"`
}

type ListingDetail struct {
	DashboardListing
	OpportunityType   string               `json:"opportunity_type,omitempty"`
	ApplicationOpen   bool                 `json:"application_open"`
	Relevant          bool                 `json:"relevant"`
	MatchingAreas     []string             `json:"matching_areas"`
	ClassRequirement  *int                 `json:"class_year_requirement,omitempty"`
	GPARequirement    *float64             `json:"gpa_requirement,omitempty"`
	Location          string               `json:"location,omitempty"`
	WorkModel         string               `json:"work_model,omitempty"`
	Confidence        float64              `json:"confidence"`
	NeedsUserDecision bool                 `json:"needs_user_decision"`
	DecisionQuestion  string               `json:"decision_question,omitempty"`
	FirstSeenAt       time.Time            `json:"first_seen_at"`
	LastSeenAt        time.Time            `json:"last_seen_at"`
	Application       *ApplicationTracking `json:"application,omitempty"`
}

type ManualCheck struct {
	SourceID      string     `json:"source_id"`
	Company       string     `json:"company"`
	URL           string     `json:"url"`
	Reason        string     `json:"reason"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
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
	ManualChecks       []ManualCheck      `json:"manual_checks"`
	LastScan           *ScanSummary       `json:"last_scan"`
}

type ListingRepository interface {
	UpsertRawListing(ctx context.Context, listing domain.RawListing) (listingID string, isNew bool, err error)
	AnalysisRequired(ctx context.Context, listingID string) (bool, error)
	SaveAnalysis(ctx context.Context, listingID string, analysis domain.ListingAnalysis) error
	SaveAnalysisFailure(ctx context.Context, listingID string, provider string, model string, reason string) error
	PendingAnalyses(ctx context.Context, limit int) ([]PendingAnalysis, error)
}

type PendingAnalysis struct {
	ListingID string
	Listing   domain.RawListing
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

type TrackingRepository interface {
	ListingDetail(ctx context.Context, listingID string) (ListingDetail, error)
	SaveApplication(ctx context.Context, listingID string, tracking ApplicationTracking) error
}
