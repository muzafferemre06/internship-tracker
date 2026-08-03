package analyzer

import (
	"context"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type CandidateProfile struct {
	EducationField  string
	ClassYear       int
	GPA             float64
	FocusAreas      []string
	ExperienceAreas []string
	Locations       []string
}

type ListingAnalyzer interface {
	Analyze(
		ctx context.Context,
		listing domain.RawListing,
		profile CandidateProfile,
	) (domain.ListingAnalysis, error)
}
