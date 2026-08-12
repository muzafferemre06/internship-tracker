package matching

import (
	"testing"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

var phase19Profile = Profile{
	ClassYear:         2,
	GPA:               3.97,
	FocusAreas:        []string{"backend", "network", "system_administration", "autonomous_software", "ground_control_station"},
	PrimaryLocations:  []string{"Ankara", "remote"},
	SummerOtherCities: true,
}

func TestAssessProducesExplainableVisibilityLayers(t *testing.T) {
	tests := []struct {
		name       string
		input      Input
		wantLayer  domain.VisibilityLayer
		wantPush   bool
		minScore   int
		wantReason string
	}{
		{
			name:      "strong official Ankara backend internship notifies once",
			input:     Input{Analysis: domain.ListingAnalysis{OpportunityType: domain.OpportunityInternship, ApplicationOpen: true, Relevant: true, MatchingAreas: []string{"backend", "system_administration"}, Location: "Ankara", Eligibility: domain.EligibilitySuitable, Confidence: 0.9}, TrustLevel: "official_company"},
			wantLayer: domain.VisibilityNotification, wantPush: true, minScore: 80, wantReason: "strong_match",
		},
		{
			name:      "reasonable official bootcamp is an opportunity without push",
			input:     Input{Analysis: domain.ListingAnalysis{OpportunityType: domain.OpportunityBootcamp, ApplicationOpen: true, Relevant: true, MatchingAreas: []string{"backend"}, Location: "remote", Eligibility: domain.EligibilitySuitable, Confidence: 0.9}, TrustLevel: "official_company"},
			wantLayer: domain.VisibilityOpportunities, wantPush: false, minScore: 55, wantReason: "reasonable_match",
		},
		{
			name:      "incomplete evidence stays for review",
			input:     Input{Analysis: domain.ListingAnalysis{OpportunityType: domain.OpportunityInternship, ApplicationOpen: true, Relevant: true, MatchingAreas: []string{"backend", "network"}, Location: "Ankara", Eligibility: domain.EligibilitySuitable, Confidence: 0.7}, TrustLevel: "official_company"},
			wantLayer: domain.VisibilityReview, wantPush: false, minScore: 0, wantReason: "inference_confidence_low",
		},
		{
			name:      "closed listing is rejected with audit reason",
			input:     Input{Analysis: domain.ListingAnalysis{OpportunityType: domain.OpportunityInternship, ApplicationOpen: false, Relevant: true, MatchingAreas: []string{"backend"}, Eligibility: domain.EligibilitySuitable, Confidence: 0.9}, TrustLevel: "official_company"},
			wantLayer: domain.VisibilityRejected, wantPush: false, minScore: 0, wantReason: "application_closed",
		},
		{
			name:      "weak but open item remains review rather than rejected",
			input:     Input{Analysis: domain.ListingAnalysis{OpportunityType: domain.OpportunityTechnicalEvent, ApplicationOpen: true, Relevant: true, Location: "İzmir", Eligibility: domain.EligibilitySuitable, Confidence: 0.9}, TrustLevel: "official_company"},
			wantLayer: domain.VisibilityReview, wantPush: false, minScore: 0, wantReason: "uncertain_match",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Assess(phase19Profile, test.input)
			if got.Visibility != test.wantLayer || got.PushEligible != test.wantPush || got.Score < test.minScore || got.Reason != test.wantReason {
				t.Fatalf("Assess() = %#v, want layer=%q push=%t score>=%d reason=%q", got, test.wantLayer, test.wantPush, test.minScore, test.wantReason)
			}
			if got.Score != got.FocusScore+got.TypeScore+got.LocationScore+got.EligibilityScore+got.RequirementScore {
				t.Fatalf("score must equal persisted components: %#v", got)
			}
		})
	}
}

func TestOpportunityTypesAndVisibilityLayersAreClosedTaxonomies(t *testing.T) {
	if !domain.OpportunityBootcamp.Valid() || domain.OpportunityType("invented").Valid() {
		t.Fatal("opportunity taxonomy validation is incorrect")
	}
	if !domain.VisibilityReview.Valid() || domain.VisibilityLayer("somewhere").Valid() {
		t.Fatal("visibility taxonomy validation is incorrect")
	}
}
