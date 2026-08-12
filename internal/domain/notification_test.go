package domain

import "testing"

func TestNewListingNotificationUsesStrongMatchIdentityForDedup(t *testing.T) {
	notification, ok := NewListingNotification(
		"opp-canonical", "listing-source-two", "Meteksan Savunma", "Backend Stajı", "primary",
		ListingAnalysis{ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable,
			Assessment: MatchAssessment{Visibility: VisibilityNotification, PushEligible: true}}, true,
	)
	if !ok {
		t.Fatal("expected suitable primary opportunity notification")
	}
	if notification.DedupKey != "opportunity:opp-canonical:new-strong-match:v2" {
		t.Fatalf("unexpected opportunity dedup key %q", notification.DedupKey)
	}
	if notification.TargetURL != "/?listing=listing-source-two" {
		t.Fatalf("notification must retain a resolvable source listing deep-link: %q", notification.TargetURL)
	}
	if len(notification.Topic) > 32 {
		t.Fatalf("Web Push Topic exceeds protocol limit: %q", notification.Topic)
	}
}

func TestNewListingNotificationAllowsStrongMatchRegardlessOfPriority(t *testing.T) {
	notification, ok := NewListingNotification(
		"opp-secondary", "listing-secondary", "Evreka", "Backend Intern", "secondary",
		ListingAnalysis{OpportunityType: "staj", ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable,
			Assessment: MatchAssessment{Visibility: VisibilityNotification, PushEligible: true}}, true,
	)
	if !ok {
		t.Fatal("expected strong secondary match notification")
	}
	if notification.EventType != NewStrongMatchEvent {
		t.Fatalf("unexpected strong-match event type %q", notification.EventType)
	}
	if notification.DedupKey != "opportunity:opp-secondary:new-strong-match:v2" {
		t.Fatalf("unexpected secondary dedup key %q", notification.DedupKey)
	}
}

func TestNewListingNotificationRejectsAnythingOutsideNotificationLayer(t *testing.T) {
	_, ok := NewListingNotification(
		"opp-secondary-full-time", "listing-secondary-full-time", "MobileAction", "Software Engineer", "secondary",
		ListingAnalysis{OpportunityType: "diger", ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable,
			Assessment: MatchAssessment{Visibility: VisibilityOpportunities, PushEligible: false}}, true,
	)
	if ok {
		t.Fatal("a full-time secondary role must remain visible without producing an internship push")
	}
}

func TestNewListingNotificationRejectsWeakSecondaryMatch(t *testing.T) {
	tests := []struct {
		name     string
		analysis ListingAnalysis
	}{
		{
			name: "reasonable candidate has no push",
			analysis: ListingAnalysis{OpportunityType: "staj", ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable,
				Assessment: MatchAssessment{Visibility: VisibilityOpportunities}},
		},
		{
			name: "review candidate has no push",
			analysis: ListingAnalysis{OpportunityType: "staj", ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable,
				Assessment: MatchAssessment{Visibility: VisibilityReview}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := NewListingNotification("opp", "listing", "Evreka", "Intern", "secondary", test.analysis, true); ok {
				t.Fatal("weak secondary match must not notify")
			}
		})
	}
}
