package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
)

const (
	NewPrimarySuitableEvent               = "new_primary_suitable_v1"
	NewSecondaryStrongMatchEvent          = "new_secondary_strong_match_v1"
	SecondaryStrongMatchMinimumConfidence = 0.7
)

type Notification struct {
	EventType string
	DedupKey  string
	Title     string
	Body      string
	TargetURL string
	Topic     string
}

func NewListingNotification(
	opportunityID string,
	listingID string,
	company string,
	title string,
	priorityGroup string,
	analysis ListingAnalysis,
	firstSuccessfulAnalysis bool,
) (Notification, bool) {
	if !firstSuccessfulAnalysis || !analysis.ApplicationOpen ||
		!analysis.Relevant || analysis.Eligibility != EligibilitySuitable {
		return Notification{}, false
	}

	eventType := ""
	dedupSuffix := ""
	switch priorityGroup {
	case "primary":
		eventType = NewPrimarySuitableEvent
		dedupSuffix = "new-primary-suitable:v1"
	case "secondary":
		if !secondaryNotificationOpportunity(analysis.OpportunityType) ||
			len(analysis.MatchingAreas) == 0 || analysis.Confidence < SecondaryStrongMatchMinimumConfidence {
			return Notification{}, false
		}
		eventType = NewSecondaryStrongMatchEvent
		dedupSuffix = "new-secondary-strong-match:v1"
	default:
		return Notification{}, false
	}

	dedupKey := "opportunity:" + opportunityID + ":" + dedupSuffix
	topicHash := sha256.Sum256([]byte(dedupKey))
	body := truncateRunes(strings.TrimSpace(company)+" — "+strings.TrimSpace(title), 240)
	return Notification{
		EventType: eventType,
		DedupKey:  dedupKey,
		Title:     "Yeni uygun staj ilanı",
		Body:      body,
		TargetURL: "/?listing=" + url.QueryEscape(listingID),
		Topic:     "opp-" + base64.RawURLEncoding.EncodeToString(topicHash[:18]),
	}, true
}

func secondaryNotificationOpportunity(opportunityType OpportunityType) bool {
	switch opportunityType {
	case OpportunityInternship, OpportunityLongTermInternship:
		return true
	default:
		return false
	}
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}
