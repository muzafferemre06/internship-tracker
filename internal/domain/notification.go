package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
)

const NewPrimarySuitableEvent = "new_primary_suitable_v1"

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
	if !firstSuccessfulAnalysis || priorityGroup != "primary" || !analysis.ApplicationOpen ||
		!analysis.Relevant || analysis.Eligibility != EligibilitySuitable {
		return Notification{}, false
	}

	dedupKey := "opportunity:" + opportunityID + ":new-primary-suitable:v1"
	topicHash := sha256.Sum256([]byte(dedupKey))
	body := truncateRunes(strings.TrimSpace(company)+" — "+strings.TrimSpace(title), 240)
	return Notification{
		EventType: NewPrimarySuitableEvent,
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
