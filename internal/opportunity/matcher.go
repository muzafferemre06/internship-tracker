package opportunity

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	AutoMergeThreshold = 0.92
	AmbiguousThreshold = 0.80
)

type Outcome string

const (
	AutoMerge Outcome = "auto_merge"
	Ambiguous Outcome = "ambiguous"
	Separate  Outcome = "separate"
)

type LocationRelation string

const (
	LocationCompatible LocationRelation = "compatible"
	LocationIncomplete LocationRelation = "incomplete"
	LocationConflict   LocationRelation = "conflict"
	LocationAbsent     LocationRelation = "absent"
)

type Identity struct {
	Title    string
	Location string
}

type Decision struct {
	Outcome            Outcome
	Score              float64
	Reason             string
	NormalizedTitle    string
	NormalizedLocation string
}

func Evaluate(incoming, existing Identity) Decision {
	incomingTitle := NormalizeTitle(incoming.Title)
	existingTitle := NormalizeTitle(existing.Title)
	exact := incomingTitle != "" && incomingTitle == existingTitle
	score := titleSimilarity(incomingTitle, existingTitle)
	relation, normalizedLocation := compareLocations(incoming.Location, existing.Location)
	outcome, reason := classify(score, exact, relation)
	return Decision{
		Outcome: outcome, Score: score, Reason: reason,
		NormalizedTitle: incomingTitle, NormalizedLocation: normalizedLocation,
	}
}

func classify(score float64, exact bool, location LocationRelation) (Outcome, string) {
	if location == LocationConflict {
		return Separate, "location_conflict"
	}
	if exact {
		switch location {
		case LocationCompatible:
			return AutoMerge, "exact_title_location_match"
		case LocationAbsent:
			return AutoMerge, "exact_title_locations_absent"
		default:
			return Ambiguous, "location_missing"
		}
	}
	if score >= AutoMergeThreshold {
		if location == LocationCompatible {
			return AutoMerge, "fuzzy_title_location_match"
		}
		return Ambiguous, "location_missing"
	}
	if score >= AmbiguousThreshold {
		return Ambiguous, "title_similarity_uncertain"
	}
	return Separate, "title_similarity_low"
}

var asciiFold = strings.NewReplacer(
	"ı", "i", "İ", "i", "ş", "s", "Ş", "s", "ğ", "g", "Ğ", "g",
	"ü", "u", "Ü", "u", "ö", "o", "Ö", "o", "ç", "c", "Ç", "c",
)

func NormalizeTitle(value string) string {
	return strings.Join(normalizeTokens(value, true), " ")
}

func normalizeLocation(value string) string {
	tokens := normalizeTokens(value, false)
	kept := tokens[:0]
	for _, token := range tokens {
		switch token {
		case "turkey", "turkiye", "turkiye'", "tr":
			continue
		default:
			kept = append(kept, token)
		}
	}
	sort.Strings(kept)
	return strings.Join(kept, " ")
}

func normalizeTokens(value string, title bool) []string {
	value = asciiFold.Replace(strings.ToLower(strings.TrimSpace(value)))
	tokens := strings.FieldsFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for index, token := range tokens {
		if !title {
			continue
		}
		switch token {
		case "stajyeri", "stajyer", "staji", "intern", "internship":
			tokens[index] = "staj"
		}
	}
	return tokens
}

func compareLocations(left, right string) (LocationRelation, string) {
	left = normalizeLocation(left)
	right = normalizeLocation(right)
	switch {
	case left == "" && right == "":
		return LocationAbsent, ""
	case left == "" || right == "":
		if left != "" {
			return LocationIncomplete, left
		}
		return LocationIncomplete, right
	case left == right:
		return LocationCompatible, left
	default:
		return LocationConflict, left
	}
}

func titleSimilarity(left, right string) float64 {
	if left == "" || right == "" {
		return 0
	}
	if left == right {
		return 1
	}
	return math.Max(characterSimilarity(left, right), tokenDice(left, right))
}

func characterSimilarity(left, right string) float64 {
	a, b := []rune(left), []rune(right)
	maximum := len(a)
	if len(b) > maximum {
		maximum = len(b)
	}
	if maximum == 0 {
		return 1
	}
	return 1 - float64(levenshtein(a, b))/float64(maximum)
}

func levenshtein(left, right []rune) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for leftIndex, leftRune := range left {
		current := make([]int, len(right)+1)
		current[0] = leftIndex + 1
		for rightIndex, rightRune := range right {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current[rightIndex+1] = minimum(
				current[rightIndex]+1,
				previous[rightIndex+1]+1,
				previous[rightIndex]+cost,
			)
		}
		previous = current
	}
	return previous[len(right)]
}

func tokenDice(left, right string) float64 {
	leftSet := tokenSet(left)
	rightSet := tokenSet(right)
	intersection := 0
	for token := range leftSet {
		if _, found := rightSet[token]; found {
			intersection++
		}
	}
	return 2 * float64(intersection) / float64(len(leftSet)+len(rightSet))
}

func tokenSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, token := range strings.Fields(value) {
		result[token] = struct{}{}
	}
	return result
}

func minimum(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}
