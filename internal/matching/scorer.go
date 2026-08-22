// Package matching provides the deterministic, explainable Faz 19 candidate
// match assessment. It is intentionally separate from analyzer confidence:
// confidence measures extraction certainty; score measures candidate fit.
package matching

import (
	"strings"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

const (
	NotificationThreshold = 80
	OpportunityThreshold  = 55
	RejectedThreshold     = 35
	MinimumConfidence     = 0.80
)

type Profile struct {
	ClassYear                   int
	GPA                         float64
	FocusAreas                  []string
	PrimaryLocations            []string
	SummerOtherCities           bool
	TermTimePartTimeOtherCities bool
}

type Input struct {
	Analysis   domain.ListingAnalysis
	TrustLevel string
}

// Assessment stores a scalar score and its components so every layer decision
// can be audited and re-evaluated when candidate preferences change.
type Assessment struct {
	Score            int                    `json:"score"`
	FocusScore       int                    `json:"focus_score"`
	TypeScore        int                    `json:"type_score"`
	LocationScore    int                    `json:"location_score"`
	EligibilityScore int                    `json:"eligibility_score"`
	RequirementScore int                    `json:"requirement_score"`
	Visibility       domain.VisibilityLayer `json:"visibility"`
	PushEligible     bool                   `json:"push_eligible"`
	Reason           string                 `json:"reason"`
}

func (a Assessment) Domain() domain.MatchAssessment {
	return domain.MatchAssessment{
		Score: a.Score, FocusScore: a.FocusScore, TypeScore: a.TypeScore,
		LocationScore: a.LocationScore, EligibilityScore: a.EligibilityScore,
		RequirementScore: a.RequirementScore, Visibility: a.Visibility,
		PushEligible: a.PushEligible, Reason: a.Reason,
	}
}

func Assess(profile Profile, input Input) Assessment {
	a := input.Analysis
	result := Assessment{
		FocusScore:       focusScore(profile.FocusAreas, a.MatchingAreas),
		TypeScore:        typeScore(a.OpportunityType),
		LocationScore:    locationScore(profile, a.Location),
		EligibilityScore: eligibilityScore(a.Eligibility),
		RequirementScore: requirementScore(profile, a),
	}
	result.Score = result.FocusScore + result.TypeScore + result.LocationScore + result.EligibilityScore + result.RequirementScore

	if !a.ApplicationOpen {
		result.Visibility, result.Reason = domain.VisibilityRejected, "application_closed"
		return result
	}
	if !strings.EqualFold(a.WorkModel, "uzaktan") && !isRemoteLocation(a.Location) && !isDomesticLocation(a.Location) {
		result.Visibility, result.Reason = domain.VisibilityRejected, "foreign_non_remote_location"
		return result
	}
	if !a.Relevant || a.Eligibility == domain.EligibilityUnsuitable {
		result.Visibility, result.Reason = domain.VisibilityRejected, "explicitly_irrelevant"
		return result
	}
	if a.Confidence < MinimumConfidence {
		result.Visibility, result.Reason = domain.VisibilityReview, "inference_confidence_low"
		return result
	}
	if a.Eligibility == domain.EligibilityPartlySuitable {
		result.Visibility, result.Reason = domain.VisibilityReview, "eligibility_uncertain"
		return result
	}
	if result.Score >= NotificationThreshold && notificationOpportunity(a.OpportunityType) && trustedForPush(input.TrustLevel) {
		result.Visibility, result.PushEligible, result.Reason = domain.VisibilityNotification, true, "strong_match"
		return result
	}
	if result.Score >= OpportunityThreshold {
		result.Visibility, result.Reason = domain.VisibilityOpportunities, "reasonable_match"
		return result
	}
	if result.Score < RejectedThreshold {
		result.Visibility, result.Reason = domain.VisibilityReview, "insufficient_match_evidence"
		return result
	}
	result.Visibility, result.Reason = domain.VisibilityReview, "uncertain_match"
	return result
}

func notificationOpportunity(kind domain.OpportunityType) bool {
	return kind == domain.OpportunityInternship || kind == domain.OpportunityLongTermInternship || kind == domain.OpportunityPartTimeStudent
}

func focusScore(profile, matches []string) int {
	configured := make(map[string]struct{}, len(profile))
	for _, area := range profile {
		configured[normalize(area)] = struct{}{}
	}
	count := 0
	seen := map[string]struct{}{}
	for _, area := range matches {
		area = normalize(area)
		if _, ok := configured[area]; ok {
			if _, duplicate := seen[area]; !duplicate {
				count++
				seen[area] = struct{}{}
			}
		}
	}
	if count > 2 {
		count = 2
	}
	return count * 20
}

func typeScore(kind domain.OpportunityType) int {
	switch kind {
	case domain.OpportunityInternship, domain.OpportunityLongTermInternship:
		return 25
	case domain.OpportunityPartTimeStudent:
		return 22
	case domain.OpportunityBootcamp, domain.OpportunityHackathon, domain.OpportunityCompetition, domain.OpportunityUniversityCompanyProgram:
		return 15
	case domain.OpportunityTechnicalEvent, domain.OpportunityTraining:
		return 10
	case domain.OpportunityScholarship:
		return 8
	case domain.OpportunityNewGraduate:
		return 5
	default:
		return 5
	}
}

func locationScore(profile Profile, location string) int {
	location = normalize(location)
	if location == "" || location == "belirtilmemis" {
		return 5
	}
	for _, primary := range profile.PrimaryLocations {
		if normalize(primary) == location || (normalize(primary) == "remote" && (location == "uzaktan" || location == "remote")) {
			return 20
		}
	}
	if profile.SummerOtherCities {
		return 10
	}
	return 0
}

func eligibilityScore(status domain.EligibilityStatus) int {
	switch status {
	case domain.EligibilitySuitable:
		return 10
	case domain.EligibilityPartlySuitable:
		return 4
	default:
		return 0
	}
}

func requirementScore(profile Profile, a domain.ListingAnalysis) int {
	score := 0
	if a.ClassRequirement == nil || profile.ClassYear >= *a.ClassRequirement {
		score += 3
	}
	if a.GPARequirement == nil || profile.GPA >= *a.GPARequirement {
		score += 2
	}
	return score
}

func trustedForPush(level string) bool {
	switch strings.TrimSpace(level) {
	case "official_company", "official_ats", "verified_newsletter":
		return true
	default:
		return false
	}
}
func normalize(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

var turkishFoldReplacer = strings.NewReplacer("İ", "i", "I", "i", "ı", "i", "Ğ", "g", "ğ", "g", "Ü", "u", "ü", "u", "Ş", "s", "ş", "s", "Ö", "o", "ö", "o", "Ç", "c", "ç", "c")

// turkishProvinces lists all 81 il names plus country name variants, used to
// recognize domestic (Turkey-based) listings from free-form location text.
var turkishProvinces = []string{
	"adana", "adiyaman", "afyonkarahisar", "agri", "amasya", "ankara", "antalya", "artvin",
	"aydin", "balikesir", "bilecik", "bingol", "bitlis", "bolu", "burdur", "bursa",
	"canakkale", "cankiri", "corum", "denizli", "diyarbakir", "edirne", "elazig", "erzincan",
	"erzurum", "eskisehir", "gaziantep", "giresun", "gumushane", "hakkari", "hatay", "isparta",
	"mersin", "istanbul", "izmir", "kars", "kastamonu", "kayseri", "kirklareli", "kirsehir",
	"kocaeli", "konya", "kutahya", "malatya", "manisa", "kahramanmaras", "mardin", "mugla",
	"mus", "nevsehir", "nigde", "ordu", "rize", "sakarya", "samsun", "siirt", "sinop", "sivas",
	"tekirdag", "tokat", "trabzon", "tunceli", "sanliurfa", "usak", "van", "yozgat", "zonguldak",
	"aksaray", "bayburt", "karaman", "kirikkale", "batman", "sirnak", "bartin", "ardahan",
	"igdir", "yalova", "karabuk", "kilis", "osmaniye", "duzce", "turkiye", "turkey",
}

// isRemoteLocation reports whether the location text itself denotes a remote
// position (some sources put "remote"/"uzaktan" in Location instead of
// WorkModel).
func isRemoteLocation(location string) bool {
	loc := strings.ToLower(strings.TrimSpace(location))
	return loc == "remote" || loc == "uzaktan"
}

// isDomesticLocation reports whether a free-form location string refers to
// Turkey or is unspecified. Unspecified locations are treated as domestic so
// unknown-location listings are not silently dropped.
func isDomesticLocation(location string) bool {
	loc := strings.ToLower(strings.TrimSpace(turkishFoldReplacer.Replace(location)))
	if loc == "" || loc == "belirtilmemis" {
		return true
	}
	for _, province := range turkishProvinces {
		if strings.Contains(loc, province) {
			return true
		}
	}
	return false
}
