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
