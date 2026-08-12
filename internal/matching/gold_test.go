package matching

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type goldFixture struct {
	Name        string                   `json:"name"`
	Type        domain.OpportunityType   `json:"type"`
	Open        bool                     `json:"open"`
	Areas       []string                 `json:"areas"`
	Location    string                   `json:"location"`
	Eligibility domain.EligibilityStatus `json:"eligibility"`
	Confidence  float64                  `json:"confidence"`
	Trust       string                   `json:"trust"`
	ScoreMin    int                      `json:"score_min"`
	Visibility  domain.VisibilityLayer   `json:"visibility"`
	Push        bool                     `json:"push"`
}

func TestPhase19GoldSet(t *testing.T) {
	contents, err := os.ReadFile("testdata/phase19-gold.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []goldFixture
	if err := json.Unmarshal(contents, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) < 32 {
		t.Fatalf("gold set has %d fixtures, want at least 32", len(fixtures))
	}
	profile := phase19Profile
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			got := Assess(profile, Input{Analysis: domain.ListingAnalysis{OpportunityType: fixture.Type, ApplicationOpen: fixture.Open, Relevant: fixture.Eligibility != domain.EligibilityUnsuitable, MatchingAreas: fixture.Areas, Location: fixture.Location, Eligibility: fixture.Eligibility, Confidence: fixture.Confidence}, TrustLevel: fixture.Trust})
			if got.Visibility != fixture.Visibility || got.PushEligible != fixture.Push || got.Score < fixture.ScoreMin {
				t.Fatalf("got score=%d layer=%q push=%t; want score >=%d layer=%q push=%t", got.Score, got.Visibility, got.PushEligible, fixture.ScoreMin, fixture.Visibility, fixture.Push)
			}
		})
	}
}
