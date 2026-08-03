package domain

import "time"

type EligibilityStatus string

const (
	EligibilitySuitable       EligibilityStatus = "uygun"
	EligibilityPartlySuitable EligibilityStatus = "kismen_uygun"
	EligibilityUnsuitable     EligibilityStatus = "uygun_degil"
	EligibilityNeedsDecision  EligibilityStatus = "karar_bekliyor"
)

type RawListing struct {
	Company   string
	SourceID  string
	Title     string
	URL       string
	RawText   string
	FetchedAt time.Time
}

type ListingAnalysis struct {
	OpportunityType   string
	ApplicationOpen   bool
	Relevant          bool
	MatchingAreas     []string
	ClassRequirement  *int
	GPARequirement    *float64
	Location          string
	WorkModel         string
	Eligibility       EligibilityStatus
	ApplicationDueAt  *time.Time
	Summary           string
	Confidence        float64
	NeedsUserDecision bool
	DecisionQuestion  string
	Provider          string
	Model             string
	PromptTokens      int
	CompletionTokens  int
	TotalTokens       int
	EstimatedCostUSD  float64
}
