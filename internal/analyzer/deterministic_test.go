package analyzer

import (
	"context"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

var deterministicTestProfile = CandidateProfile{
	ClassYear:  2,
	FocusAreas: []string{"backend", "network", "system_administration", "autonomous_software", "ground_control_station"},
}

func TestDeterministicAnalyzerClassifiesRelevantInternship(t *testing.T) {
	analysis, err := NewDeterministicAnalyzer().Analyze(context.Background(), domain.RawListing{
		Title:   "Backend Yazılım Geliştirme Stajyeri",
		RawText: "Ankara hibrit çalışma düzeninde Go ve API geliştirme.",
	}, deterministicTestProfile)
	if err != nil {
		t.Fatalf("analyze listing: %v", err)
	}
	if analysis.OpportunityType != "staj" || !analysis.Relevant || !analysis.ApplicationOpen {
		t.Fatalf("unexpected classification: %#v", analysis)
	}
	if analysis.Eligibility != domain.EligibilitySuitable || analysis.Location != "Ankara" || analysis.WorkModel != "hibrit" {
		t.Fatalf("unexpected suitability: %#v", analysis)
	}
	if len(analysis.MatchingAreas) != 1 || analysis.MatchingAreas[0] != "backend" {
		t.Fatalf("unexpected matching areas: %#v", analysis.MatchingAreas)
	}
}

func TestDeterministicAnalyzerMarksHigherClassRequirementPartlySuitable(t *testing.T) {
	analysis, err := NewDeterministicAnalyzer().Analyze(context.Background(), domain.RawListing{
		Title:   "Network Stajyeri",
		RawText: "Adayların 3. sınıf öğrencisi olması beklenmektedir.",
	}, deterministicTestProfile)
	if err != nil {
		t.Fatalf("analyze listing: %v", err)
	}
	if analysis.Eligibility != domain.EligibilityPartlySuitable || analysis.ClassRequirement == nil || *analysis.ClassRequirement != 3 {
		t.Fatalf("unexpected class eligibility: %#v", analysis)
	}
}

func TestDeterministicAnalyzerMarksClosedListingUnsuitable(t *testing.T) {
	analysis, err := NewDeterministicAnalyzer().Analyze(context.Background(), domain.RawListing{
		Title:   "Otonom Yazılım Stajı",
		RawText: "Başvurular kapanmıştır.",
	}, deterministicTestProfile)
	if err != nil {
		t.Fatalf("analyze listing: %v", err)
	}
	if analysis.ApplicationOpen || analysis.Eligibility != domain.EligibilityUnsuitable {
		t.Fatalf("closed listing was not classified correctly: %#v", analysis)
	}
}

func TestDeterministicAnalyzerHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewDeterministicAnalyzer().Analyze(ctx, domain.RawListing{Title: "Staj"}, deterministicTestProfile)
	if err == nil {
		t.Fatal("expected canceled analysis to fail")
	}
}
