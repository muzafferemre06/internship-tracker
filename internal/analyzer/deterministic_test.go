package analyzer

import (
	"context"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

func TestDeterministicAnalyzer_Analyze(t *testing.T) {
	analyzer := NewDeterministicAnalyzer()
	profile := CandidateProfile{
		FocusAreas: []string{"backend"},
		ClassYear:  3,
	}

	t.Run("empty title returns error", func(t *testing.T) {
		_, err := analyzer.Analyze(context.Background(), domain.RawListing{RawText: "body"}, profile)
		if err == nil {
			t.Error("expected error for empty title, got nil")
		}
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := analyzer.Analyze(ctx, domain.RawListing{Title: "Title"}, profile)
		if err == nil {
			t.Error("expected error for cancelled context, got nil")
		}
	})
}

func TestDeterministicAnalyzer_Regressions(t *testing.T) {
	analyzer := NewDeterministicAnalyzer()
	ctx := context.Background()
	profile := CandidateProfile{
		FocusAreas: []string{"backend", "system_administration"},
		ClassYear:  3,
	}

	tests := []struct {
		name     string
		title    string
		body     string
		wantType domain.OpportunityType
		wantRel  bool
		wantLoc  string
	}{
		{
			name:     "A senior role whose body mentions internships",
			title:    "Senior Pre-Sales Engineer",
			body:     "our international team and we run an internship program in Ankara",
			wantType: domain.OpportunityOther,
			wantRel:  false,
			wantLoc:  "Ankara",
		},
		{
			name:     "Customer Onboarding Intern - Brazil",
			title:    "Customer Onboarding Intern - Brazil",
			body:     "Remote work available",
			wantType: domain.OpportunityInternship,
			wantRel:  true,
			wantLoc:  "Brazil",
		},
		{
			name:     "Software Engineer with internal tooling",
			title:    "Software Engineer",
			body:     "we are an international company with internal tooling",
			wantType: domain.OpportunityOther,
			wantRel:  false,
			wantLoc:  "Belirtilmemiş",
		},
		{
			name:     "Genuine internship (Backend Intern)",
			title:    "Backend Intern",
			body:     "Ankara ofisimizde çalışacak",
			wantType: domain.OpportunityInternship,
			wantRel:  true,
			wantLoc:  "Ankara",
		},
		{
			name:     "Genuine internship (Yazılım Stajyeri)",
			title:    "Yazılım Stajyeri",
			body:     "backend ve sistem yönetimi için Ankara",
			wantType: domain.OpportunityInternship,
			wantRel:  true,
			wantLoc:  "Ankara",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listing := domain.RawListing{
				Title:   tt.title,
				RawText: tt.body,
			}
			got, err := analyzer.Analyze(ctx, listing, profile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.OpportunityType != tt.wantType {
				t.Errorf("OpportunityType = %v, want %v", got.OpportunityType, tt.wantType)
			}
			if got.Relevant != tt.wantRel {
				t.Errorf("Relevant = %v, want %v", got.Relevant, tt.wantRel)
			}
			if got.Location != tt.wantLoc {
				t.Errorf("Location = %q, want %q", got.Location, tt.wantLoc)
			}

			// Assert Confidence is below the downstream gate
			if got.Confidence >= 0.80 {
				t.Errorf("Confidence = %v, want < 0.80 (keeps keyword results out of notification layer)", got.Confidence)
			}
		})
	}
}
