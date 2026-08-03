package analyzer

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

var classRequirementPattern = regexp.MustCompile(`(?i)([1-4])\s*\.\s*sınıf`)

type DeterministicAnalyzer struct{}

func NewDeterministicAnalyzer() DeterministicAnalyzer {
	return DeterministicAnalyzer{}
}

func (DeterministicAnalyzer) Analyze(
	ctx context.Context,
	listing domain.RawListing,
	profile CandidateProfile,
) (domain.ListingAnalysis, error) {
	if err := ctx.Err(); err != nil {
		return domain.ListingAnalysis{}, err
	}
	if strings.TrimSpace(listing.Title) == "" {
		return domain.ListingAnalysis{}, fmt.Errorf("listing title is required")
	}

	text := strings.ToLower(listing.Title + "\n" + listing.RawText)
	matchingAreas := matchingFocusAreas(text, profile.FocusAreas)
	classRequirement := extractClassRequirement(text)
	applicationOpen := !containsAny(text,
		"başvurular kapanmıştır",
		"başvuru süresi sona ermiştir",
		"pozisyon kapatılmıştır",
		"ilan yayından kaldırılmıştır",
	)
	relevant := len(matchingAreas) > 0 || containsAny(text, "staj", "intern", "yeni mezun")

	eligibility := domain.EligibilitySuitable
	if !applicationOpen || !relevant {
		eligibility = domain.EligibilityUnsuitable
	} else if classRequirement != nil && profile.ClassYear > 0 && *classRequirement > profile.ClassYear {
		eligibility = domain.EligibilityPartlySuitable
	}

	return domain.ListingAnalysis{
		OpportunityType:  opportunityType(text),
		ApplicationOpen:  applicationOpen,
		Relevant:         relevant,
		MatchingAreas:    matchingAreas,
		ClassRequirement: classRequirement,
		Location:         location(text),
		WorkModel:        workModel(text),
		Eligibility:      eligibility,
		Summary:          strings.TrimSpace(listing.Title),
		Confidence:       0.7,
	}, nil
}

func matchingFocusAreas(text string, configured []string) []string {
	keywords := map[string][]string{
		"backend":                {"backend", "back-end", "api geliştirme", "sunucu tarafı", "golang", " go ", "java"},
		"network":                {"network", "networking", "ağ teknoloj", "haberleşme ağı"},
		"system_administration":  {"sistem yönet", "system administration", "linux", "devops", "kubernetes", "bulut altyap"},
		"autonomous_software":    {"otonom", "autonomous", "computer vision", "görüntü işleme", "yapay zeka", "machine learning"},
		"ground_control_station": {"yer kontrol", "ground control", "gcs", "komuta kontrol"},
	}

	matches := make([]string, 0)
	for _, area := range configured {
		if containsAny(text, keywords[area]...) {
			matches = append(matches, area)
		}
	}
	sort.Strings(matches)
	return matches
}

func extractClassRequirement(text string) *int {
	matches := classRequirementPattern.FindStringSubmatch(text)
	if len(matches) != 2 {
		return nil
	}
	value := int(matches[1][0] - '0')
	return &value
}

func opportunityType(text string) string {
	switch {
	case containsAny(text, "uzun dönem staj", "long term intern", "long-term intern"):
		return "uzun_donem_staj"
	case containsAny(text, "staj", "intern"):
		return "staj"
	case containsAny(text, "part-time", "part time", "yarı zamanlı"):
		return "part_time"
	case containsAny(text, "yeni mezun", "new graduate", "graduate program"):
		return "yeni_mezun"
	default:
		return "diger"
	}
}

func location(text string) string {
	switch {
	case containsAny(text, "ankara"):
		return "Ankara"
	case containsAny(text, "uzaktan", "remote"):
		return "Uzaktan"
	default:
		return "Belirtilmemiş"
	}
}

func workModel(text string) string {
	switch {
	case containsAny(text, "hibrit", "hybrid"):
		return "hibrit"
	case containsAny(text, "uzaktan", "remote"):
		return "uzaktan"
	case containsAny(text, "iş yerinde", "ofiste", "on-site", "onsite"):
		return "is_yerinde"
	default:
		return "belirtilmemis"
	}
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if value != "" && strings.Contains(text, value) {
			return true
		}
	}
	return false
}
