package scraper

import (
	"context"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type Source interface {
	Name() string
	FetchListings(ctx context.Context) ([]domain.RawListing, error)
}
