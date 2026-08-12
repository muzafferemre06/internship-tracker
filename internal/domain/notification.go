package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
)

const NewStrongMatchEvent = "new_strong_match_v2"

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
	if !firstSuccessfulAnalysis || !analysis.Assessment.PushEligible ||
		analysis.Assessment.Visibility != VisibilityNotification {
		return Notification{}, false
	}
	if priorityGroup != "primary" && priorityGroup != "secondary" {
		return Notification{}, false
	}
	dedupKey := "opportunity:" + opportunityID + ":new-strong-match:v2"
	topicHash := sha256.Sum256([]byte(dedupKey))
	body := truncateRunes(strings.TrimSpace(company)+" — "+strings.TrimSpace(title), 240)
	return Notification{
		EventType: NewStrongMatchEvent,
		DedupKey:  dedupKey,
		Title:     "Yeni uygun staj ilanı",
		Body:      body,
		TargetURL: "/?listing=" + url.QueryEscape(listingID),
		Topic:     "opp-" + base64.RawURLEncoding.EncodeToString(topicHash[:18]),
	}, true
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}
