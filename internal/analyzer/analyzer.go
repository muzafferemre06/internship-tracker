package analyzer

import (
	"context"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type CandidateProfile struct {
	Education       string
	ClassYear       int
	GPA             float64
	FocusAreas      []string
	ExperienceNotes []string
}

type ListingAnalyzer interface {
	Analyze(
		ctx context.Context,
		listing domain.RawListing,
		profile CandidateProfile,
	) (domain.ListingAnalysis, error)
}
