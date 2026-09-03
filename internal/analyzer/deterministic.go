package analyzer

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

var classRequirementPattern = regexp.MustCompile(`(?i)([1-4])\s*\.\s*sınıf`)

var (
	wordRegexpCache sync.Map

	foreignLocAliases = map[string]string{
		"brasil": "Brazil", "brezilya": "Brazil", "sao paulo": "Brazil", "são paulo": "Brazil", "brazil": "Brazil",
		"deutschland": "Germany", "almanya": "Germany", "berlin": "Germany", "munich": "Germany", "münchen": "Germany", "germany": "Germany",
		"amsterdam": "Netherlands", "hollanda": "Netherlands", "netherlands": "Netherlands",
		"london": "United Kingdom", "uk": "United Kingdom", "england": "United Kingdom", "ingiltere": "United Kingdom", "united kingdom": "United Kingdom",
		"usa": "United States", "u.s.": "United States", "united states": "United States", "new york": "United States", "san francisco": "United States", "amerika": "United States",
		"warsaw": "Poland", "polonya": "Poland", "poland": "Poland",
		"bangalore": "India", "bengaluru": "India", "hindistan": "India", "india": "India",
		"dubai": "United Arab Emirates", "abu dhabi": "United Arab Emirates", "united arab emirates": "United Arab Emirates",
		"cairo": "Egypt", "mısır": "Egypt", "misir": "Egypt", "egypt": "Egypt",
		"dublin": "Ireland", "ireland": "Ireland",
		"madrid": "Spain", "barcelona": "Spain", "ispanya": "Spain", "spain": "Spain",
		"paris": "France", "fransa": "France", "france": "France",
		"bucharest": "Romania", "romanya": "Romania", "romania": "Romania",
		"lisbon": "Portugal", "portekiz": "Portugal", "portugal": "Portugal",
		"singapore":    "Singapore",
		"canada":       "Canada",
		"australia":    "Australia",
		"mexico":       "Mexico",
		"argentina":    "Argentina",
		"colombia":     "Colombia",
		"chile":        "Chile",
		"japan":        "Japan",
		"south korea":  "South Korea",
		"vietnam":      "Vietnam",
		"indonesia":    "Indonesia",
		"philippines":  "Philippines",
		"malaysia":     "Malaysia",
		"nigeria":      "Nigeria",
		"kenya":        "Kenya",
		"south africa": "South Africa",
		"sweden":       "Sweden",
		"norway":       "Norway",
		"denmark":      "Denmark",
		"finland":      "Finland",
		"switzerland":  "Switzerland",
		"austria":      "Austria",
		"belgium":      "Belgium",
		"italy":        "Italy",
		"greece":       "Greece",
		"czechia":      "Czechia",
		"hungary":      "Hungary",
		"bulgaria":     "Bulgaria",
		"ukraine":      "Ukraine",
		"serbia":       "Serbia",
		"croatia":      "Croatia",
		"israel":       "Israel",
		"saudi arabia": "Saudi Arabia",
		"qatar":        "Qatar",
		"pakistan":     "Pakistan",
		"bangladesh":   "Bangladesh",
	}

	turkishLocAliases = map[string]string{
		"türkiye": "Türkiye", "turkiye": "Türkiye", "turkey": "Türkiye",
		"istanbul": "İstanbul", "i̇stanbul": "İstanbul",
		"ankara": "Ankara",
		"izmir":  "İzmir", "i̇zmir": "İzmir",
		"bursa":     "Bursa",
		"antalya":   "Antalya",
		"kocaeli":   "Kocaeli",
		"konya":     "Konya",
		"gaziantep": "Gaziantep",
		"kayseri":   "Kayseri",
		"eskişehir": "Eskişehir", "eskisehir": "Eskişehir",
		"adana":    "Adana",
		"denizli":  "Denizli",
		"sakarya":  "Sakarya",
		"mersin":   "Mersin",
		"trabzon":  "Trabzon",
		"samsun":   "Samsun",
		"tekirdağ": "Tekirdağ", "tekirdag": "Tekirdağ",
	}

	sortedForeignKeys []string
	sortedTurkishKeys []string
)

func init() {
	for k := range foreignLocAliases {
		sortedForeignKeys = append(sortedForeignKeys, k)
	}
	sort.Strings(sortedForeignKeys)

	for k := range turkishLocAliases {
		sortedTurkishKeys = append(sortedTurkishKeys, k)
	}
	sort.Strings(sortedTurkishKeys)
}

// DeterministicAnalyzer provides a rule-based fallback for analyzing job listings.
type DeterministicAnalyzer struct{}

// NewDeterministicAnalyzer creates a new DeterministicAnalyzer.
func NewDeterministicAnalyzer() DeterministicAnalyzer {
	return DeterministicAnalyzer{}
}

// Analyze processes the listing using deterministic keyword rules.
func (DeterministicAnalyzer) Analyze(
	ctx context.Context,
	listing domain.RawListing,
	profile CandidateProfile,
) (domain.ListingAnalysis, error) {
	if err := ctx.Err(); err != nil {
		return domain.ListingAnalysis{}, fmt.Errorf("analyzer context cancelled: %w", err)
	}
	if strings.TrimSpace(listing.Title) == "" {
		return domain.ListingAnalysis{}, fmt.Errorf("listing title is required")
	}

	title := strings.ToLower(strings.TrimSpace(listing.Title))
	body := strings.ToLower(listing.RawText)
	full := title + "\n" + body

	matchingAreas := matchingFocusAreas(full, profile.FocusAreas)
	classRequirement := extractClassRequirement(full)
	applicationOpen := !containsAny(full,
		"başvurular kapanmıştır",
		"başvuru süresi sona ermiştir",
		"pozisyon kapatılmıştır",
		"ilan yayından kaldırılmıştır",
	)

	relevant := !isSeniorRole(title) && (len(matchingAreas) > 0 || containsWord(title, "staj", "stajyer", "stajyeri", "intern", "internship") || containsAny(title, "yeni mezun", "new graduate"))

	eligibility := domain.EligibilitySuitable
	if !applicationOpen || !relevant {
		eligibility = domain.EligibilityUnsuitable
	} else if classRequirement != nil && profile.ClassYear > 0 && *classRequirement > profile.ClassYear {
		eligibility = domain.EligibilityPartlySuitable
	}

	return domain.ListingAnalysis{
		OpportunityType:  opportunityType(title),
		ApplicationOpen:  applicationOpen,
		Relevant:         relevant,
		MatchingAreas:    matchingAreas,
		ClassRequirement: classRequirement,
		Location:         location(title, body),
		WorkModel:        workModel(full),
		Eligibility:      eligibility,
		Summary:          strings.TrimSpace(listing.Title),
		// Confidence is 0.6 because this analyzer is a keyword fallback whose extraction is a guess.
		// Downstream matching.MinimumConfidence is 0.80. Reporting a value below that gate
		// deliberately keeps keyword-derived analyses out of the push-notification layer and
		// routes them to manual review instead.
		Confidence: 0.6,
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

func opportunityType(title string) domain.OpportunityType {
	if isSeniorRole(title) {
		return domain.OpportunityOther
	}
	switch {
	case containsAny(title, "uzun dönem staj", "long term intern", "long-term intern"):
		return domain.OpportunityLongTermInternship
	case containsAny(title, "part-time", "yarı zamanlı") || containsWord(title, "part time"):
		return domain.OpportunityPartTimeStudent
	case containsWord(title, "staj", "stajyer", "stajyeri", "intern", "internship"):
		return domain.OpportunityInternship
	case containsAny(title, "yeni mezun", "new graduate", "graduate program"):
		return domain.OpportunityNewGraduate
	case containsWord(title, "bootcamp"):
		return domain.OpportunityBootcamp
	case containsWord(title, "hackathon"):
		return domain.OpportunityHackathon
	case containsWord(title, "yarışma", "yarisma", "competition"):
		return domain.OpportunityCompetition
	case containsWord(title, "burs", "scholarship"):
		return domain.OpportunityScholarship
	case containsAny(title, "teknik etkinlik", "technical event") || containsWord(title, "konferans", "conference"):
		return domain.OpportunityTechnicalEvent
	case containsWord(title, "eğitim", "egitim", "training"):
		return domain.OpportunityTraining
	default:
		return domain.OpportunityOther
	}
}

func location(title, body string) string {
	if loc := findLocationMatch(title, foreignLocAliases, sortedForeignKeys); loc != "" {
		return loc
	}
	if loc := findLocationMatch(title, turkishLocAliases, sortedTurkishKeys); loc != "" {
		return loc
	}
	if loc := findLocationMatch(body, foreignLocAliases, sortedForeignKeys); loc != "" {
		return loc
	}
	if loc := findLocationMatch(body, turkishLocAliases, sortedTurkishKeys); loc != "" {
		return loc
	}

	full := title + "\n" + body
	if containsAny(full, "uzaktan", "remote", "remote-first", "fully remote") {
		return "Uzaktan"
	}
	return "Belirtilmemiş"
}

func findLocationMatch(text string, aliases map[string]string, sortedKeys []string) string {
	for _, alias := range sortedKeys {
		if strings.Contains(alias, " ") || strings.Contains(alias, ".") {
			if containsAny(text, alias) {
				return aliases[alias]
			}
		} else {
			if containsWord(text, alias) {
				return aliases[alias]
			}
		}
	}
	return ""
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

// isSeniorRole reports whether a job title clearly denotes an experienced or
// managerial position, which a student opportunity never is.
func isSeniorRole(title string) bool {
	if containsWord(title, "senior", "sr", "staff", "principal", "lead", "manager", "director", "vp", "chief", "architect", "counsel", "müdür", "mudur", "yönetici", "yonetici", "kıdemli", "kidemli") {
		return true
	}
	if containsAny(title, "head of", "vice president") {
		return true
	}
	return false
}

// containsWord reports whether any of the given words appears in text as a
// whole word. Substring matching is unsafe for short tokens: "intern" occurs
// inside "internal" and "international", which previously turned ordinary
// jobs into internships.
func containsWord(text string, words ...string) bool {
	for _, word := range words {
		if word == "" {
			continue
		}
		rxIface, ok := wordRegexpCache.Load(word)
		var rx *regexp.Regexp
		if !ok {
			pattern := `(?i)(?:^|[^\p{L}\p{N}])` + regexp.QuoteMeta(word) + `(?:[^\p{L}\p{N}]|$)`
			rx = regexp.MustCompile(pattern)
			wordRegexpCache.Store(word, rx)
		} else {
			rx = rxIface.(*regexp.Regexp)
		}
		if rx.MatchString(text) {
			return true
		}
	}
	return false
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if value != "" && strings.Contains(text, value) {
			return true
		}
	}
	return false
}
