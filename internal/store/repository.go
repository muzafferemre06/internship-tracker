package store

import (
	"context"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type ListingRepository interface {
	UpsertRawListing(ctx context.Context, listing domain.RawListing) (listingID string, isNew bool, err error)
	SaveAnalysis(ctx context.Context, listingID string, analysis domain.ListingAnalysis) error
}
