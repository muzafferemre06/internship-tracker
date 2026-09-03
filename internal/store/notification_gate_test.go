package store

import (
	"testing"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

func TestPromotableToNotification(t *testing.T) {
	tests := []struct {
		name     string
		analysis *domain.ListingAnalysis
		want     bool
	}{
		{
			name: "internship, confidence 0.9, score 85, visibility domain.VisibilityOpportunities",
			analysis: &domain.ListingAnalysis{
				OpportunityType: domain.OpportunityInternship,
				Confidence:      0.9,
				Assessment: domain.MatchAssessment{
					Score:      85,
					Visibility: domain.VisibilityOpportunities,
				},
			},
			want: true,
		},
		{
			// Regression: a Solution Architect must never be pushed
			name: "domain.OpportunityOther",
			analysis: &domain.ListingAnalysis{
				OpportunityType: domain.OpportunityOther,
				Confidence:      0.9,
				Assessment: domain.MatchAssessment{
					Score:      85,
					Visibility: domain.VisibilityOpportunities,
				},
			},
			want: false,
		},
		{
			name: "domain.OpportunityNewGraduate",
			analysis: &domain.ListingAnalysis{
				OpportunityType: domain.OpportunityNewGraduate,
				Confidence:      0.9,
				Assessment: domain.MatchAssessment{
					Score:      85,
					Visibility: domain.VisibilityOpportunities,
				},
			},
			want: false,
		},
		{
			name: "internship but confidence 0.5",
			analysis: &domain.ListingAnalysis{
				OpportunityType: domain.OpportunityInternship,
				Confidence:      0.5,
				Assessment: domain.MatchAssessment{
					Score:      85,
					Visibility: domain.VisibilityOpportunities,
				},
			},
			want: false,
		},
		{
			name: "internship but score 70",
			analysis: &domain.ListingAnalysis{
				OpportunityType: domain.OpportunityInternship,
				Confidence:      0.9,
				Assessment: domain.MatchAssessment{
					Score:      70,
					Visibility: domain.VisibilityOpportunities,
				},
			},
			want: false,
		},
		{
			name: "internship, score 85, but visibility domain.VisibilityRejected",
			analysis: &domain.ListingAnalysis{
				OpportunityType: domain.OpportunityInternship,
				Confidence:      0.9,
				Assessment: domain.MatchAssessment{
					Score:      85,
					Visibility: domain.VisibilityRejected,
				},
			},
			want: false,
		},
		{
			name:     "nil analysis",
			analysis: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := promotableToNotification(tt.analysis); got != tt.want {
				t.Errorf("promotableToNotification() = %v, want %v", got, tt.want)
			}
		})
	}
}
