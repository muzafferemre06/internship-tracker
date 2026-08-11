package domain

import "testing"

func TestNewListingNotificationUsesOpportunityIdentityForDedup(t *testing.T) {
	notification, ok := NewListingNotification(
		"opp-canonical", "listing-source-two", "Meteksan Savunma", "Backend Stajı", "primary",
		ListingAnalysis{ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable}, true,
	)
	if !ok {
		t.Fatal("expected suitable primary opportunity notification")
	}
	if notification.DedupKey != "opportunity:opp-canonical:new-primary-suitable:v1" {
		t.Fatalf("unexpected opportunity dedup key %q", notification.DedupKey)
	}
	if notification.TargetURL != "/?listing=listing-source-two" {
		t.Fatalf("notification must retain a resolvable source listing deep-link: %q", notification.TargetURL)
	}
	if len(notification.Topic) > 32 {
		t.Fatalf("Web Push Topic exceeds protocol limit: %q", notification.Topic)
	}
}

func TestNewListingNotificationAllowsStrongSecondaryMatchWithSeparateEventIdentity(t *testing.T) {
	notification, ok := NewListingNotification(
		"opp-secondary", "listing-secondary", "Evreka", "Backend Intern", "secondary",
		ListingAnalysis{
			ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable,
			MatchingAreas: []string{"backend"}, Confidence: 0.7,
		}, true,
	)
	if !ok {
		t.Fatal("expected strong secondary match notification")
	}
	if notification.EventType != NewSecondaryStrongMatchEvent {
		t.Fatalf("unexpected secondary event type %q", notification.EventType)
	}
	if notification.DedupKey != "opportunity:opp-secondary:new-secondary-strong-match:v1" {
		t.Fatalf("unexpected secondary dedup key %q", notification.DedupKey)
	}
}

func TestNewListingNotificationRejectsWeakSecondaryMatch(t *testing.T) {
	tests := []struct {
		name     string
		analysis ListingAnalysis
	}{
		{
			name: "no focus-area match",
			analysis: ListingAnalysis{ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable,
				Confidence: 0.95},
		},
		{
			name: "below fixed confidence",
			analysis: ListingAnalysis{ApplicationOpen: true, Relevant: true, Eligibility: EligibilitySuitable,
				MatchingAreas: []string{"backend"}, Confidence: 0.69},
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
