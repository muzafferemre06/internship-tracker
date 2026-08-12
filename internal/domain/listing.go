package domain

import "time"

// OpportunityType is the stable Faz 19 taxonomy shared by listings, program
// windows and future RSS/email evidence. It deliberately describes the
// opportunity, not the candidate's application state.
type OpportunityType string

const (
	OpportunityInternship               OpportunityType = "staj"
	OpportunityLongTermInternship       OpportunityType = "uzun_donem_staj"
	OpportunityPartTimeStudent          OpportunityType = "part_time_ogrenci"
	OpportunityNewGraduate              OpportunityType = "yeni_mezun"
	OpportunityBootcamp                 OpportunityType = "bootcamp"
	OpportunityHackathon                OpportunityType = "hackathon"
	OpportunityCompetition              OpportunityType = "yarisma"
	OpportunityScholarship              OpportunityType = "burs"
	OpportunityUniversityCompanyProgram OpportunityType = "universite_sirket_programi"
	OpportunityTechnicalEvent           OpportunityType = "teknik_etkinlik"
	OpportunityTraining                 OpportunityType = "egitim"
	OpportunityOther                    OpportunityType = "diger"
)

func (t OpportunityType) Valid() bool {
	switch t {
	case OpportunityInternship, OpportunityLongTermInternship, OpportunityPartTimeStudent,
		OpportunityNewGraduate, OpportunityBootcamp, OpportunityHackathon,
		OpportunityCompetition, OpportunityScholarship, OpportunityUniversityCompanyProgram,
		OpportunityTechnicalEvent, OpportunityTraining, OpportunityOther:
		return true
	default:
		return false
	}
}

// VisibilityLayer is independent from lifecycle and application status. A
// candidate may still archive or apply to an item in any layer.
type VisibilityLayer string

const (
	VisibilityNotification  VisibilityLayer = "bildirim"
	VisibilityOpportunities VisibilityLayer = "firsatlar"
	VisibilityReview        VisibilityLayer = "incelenecek"
	VisibilityRejected      VisibilityLayer = "elenen"
)

func (l VisibilityLayer) Valid() bool {
	switch l {
	case VisibilityNotification, VisibilityOpportunities, VisibilityReview, VisibilityRejected:
		return true
	default:
		return false
	}
}

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
	OpportunityType   OpportunityType
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
