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
	FinishedAt time.Time `json:"finished_at"`
	Status     string    `json:"status"`
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

type DashboardRepository interface {
	Dashboard(ctx context.Context) (DashboardSnapshot, error)
}
