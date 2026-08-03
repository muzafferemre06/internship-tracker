package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

const defaultMaxAttempts = 3

type CostRates struct {
	InputPerMillionUSD  float64
	OutputPerMillionUSD float64
}

type ModelAnalyzer struct {
	provider    ModelProvider
	model       string
	maxAttempts int
	costRates   CostRates
	wait        func(context.Context, time.Duration) error
}

func NewModelAnalyzer(provider ModelProvider, model string, rates CostRates) (*ModelAnalyzer, error) {
	if provider == nil {
		return nil, errors.New("model provider is required")
	}
	if strings.TrimSpace(provider.Name()) == "" {
		return nil, errors.New("model provider name is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("model name is required")
	}
	if rates.InputPerMillionUSD < 0 || rates.OutputPerMillionUSD < 0 {
		return nil, errors.New("model cost rates cannot be negative")
	}
	return &ModelAnalyzer{
		provider: provider, model: model, maxAttempts: defaultMaxAttempts, costRates: rates,
		wait: waitForRetry,
	}, nil
}

func (a *ModelAnalyzer) ProviderName() string { return a.provider.Name() }

func (a *ModelAnalyzer) ModelName() string { return a.model }

type minimizedProfile struct {
	EducationField  string   `json:"education_field"`
	ClassYear       int      `json:"class_year"`
	GPA             float64  `json:"gpa"`
	FocusAreas      []string `json:"focus_areas"`
	ExperienceAreas []string `json:"experience_areas"`
	Locations       []string `json:"locations"`
}

type modelInput struct {
	Listing struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	} `json:"listing"`
	Candidate minimizedProfile `json:"candidate"`
}

func (a *ModelAnalyzer) Analyze(ctx context.Context, listing domain.RawListing, profile CandidateProfile) (domain.ListingAnalysis, error) {
	input := modelInput{Candidate: minimizeProfile(profile)}
	input.Listing.Title = strings.TrimSpace(listing.Title)
	input.Listing.Content = strings.TrimSpace(listing.RawText)
	if input.Listing.Title == "" {
		return domain.ListingAnalysis{}, errors.New("listing title is required")
	}

	request := ProviderRequest{
		Model:        a.model,
		SystemPrompt: "İlanı yalnızca verilen aday özelliklerine göre değerlendir. JSON şemasına tam uy ve bilinmeyen bilgi uydurma.",
		Input:        input,
		JSONSchema:   analysisJSONSchema(),
	}
	var lastErr error
	var usage ProviderUsage
	attempts := 0
	for attempt := 1; attempt <= a.maxAttempts; attempt++ {
		attempts = attempt
		response, err := a.provider.Complete(ctx, request)
		if err == nil {
			usage.PromptTokens += response.Usage.PromptTokens
			usage.CompletionTokens += response.Usage.CompletionTokens
			usage.TotalTokens += response.Usage.TotalTokens
			analysis, decodeErr := decodeAnalysis(response.Content)
			if decodeErr == nil {
				analysis.Provider = a.provider.Name()
				analysis.Model = a.model
				analysis.PromptTokens = usage.PromptTokens
				analysis.CompletionTokens = usage.CompletionTokens
				analysis.TotalTokens = usage.TotalTokens
				if analysis.TotalTokens < analysis.PromptTokens+analysis.CompletionTokens {
					analysis.TotalTokens = analysis.PromptTokens + analysis.CompletionTokens
				}
				analysis.EstimatedCostUSD = float64(analysis.PromptTokens)*a.costRates.InputPerMillionUSD/1_000_000 +
					float64(analysis.CompletionTokens)*a.costRates.OutputPerMillionUSD/1_000_000
				return analysis, nil
			}
			err = fmt.Errorf("validate model response: %w", decodeErr)
		}
		lastErr = err
		if attempt == a.maxAttempts || (!IsTemporary(err) && !isResponseValidationError(err)) {
			break
		}
		if err := a.wait(ctx, time.Duration(1<<(attempt-1))*100*time.Millisecond); err != nil {
			return domain.ListingAnalysis{}, err
		}
	}
	return domain.ListingAnalysis{}, fmt.Errorf("analyze listing after %d attempt(s): %w", attempts, lastErr)
}

func minimizeProfile(profile CandidateProfile) minimizedProfile {
	return minimizedProfile{
		EducationField: strings.TrimSpace(profile.EducationField), ClassYear: profile.ClassYear, GPA: profile.GPA,
		FocusAreas: cleanStrings(profile.FocusAreas), ExperienceAreas: cleanStrings(profile.ExperienceAreas),
		Locations: cleanStrings(profile.Locations),
	}
}

func cleanStrings(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			unique[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func waitForRetry(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isResponseValidationError(err error) bool {
	return strings.Contains(err.Error(), "validate model response:")
}

type analysisPayload struct {
	OpportunityType   string                   `json:"opportunity_type"`
	ApplicationOpen   bool                     `json:"application_open"`
	Relevant          bool                     `json:"relevant"`
	MatchingAreas     []string                 `json:"matching_areas"`
	ClassRequirement  *int                     `json:"class_requirement"`
	GPARequirement    *float64                 `json:"gpa_requirement"`
	Location          string                   `json:"location"`
	WorkModel         string                   `json:"work_model"`
	Eligibility       domain.EligibilityStatus `json:"eligibility"`
	ApplicationDueAt  *time.Time               `json:"application_due_at"`
	Summary           string                   `json:"summary"`
	Confidence        float64                  `json:"confidence"`
	NeedsUserDecision bool                     `json:"needs_user_decision"`
	DecisionQuestion  string                   `json:"decision_question"`
}

func decodeAnalysis(content string) (domain.ListingAnalysis, error) {
	var fields map[string]json.RawMessage
	if err := decodeSingleJSON(content, &fields); err != nil {
		return domain.ListingAnalysis{}, err
	}
	for _, name := range analysisFieldNames() {
		value, exists := fields[name]
		if !exists {
			return domain.ListingAnalysis{}, fmt.Errorf("required field %q is missing", name)
		}
		if string(value) == "null" && name != "class_requirement" && name != "gpa_requirement" && name != "application_due_at" {
			return domain.ListingAnalysis{}, fmt.Errorf("field %q cannot be null", name)
		}
	}

	var payload analysisPayload
	if err := decodeSingleJSONStrict(content, &payload); err != nil {
		return domain.ListingAnalysis{}, err
	}
	if err := validatePayload(payload); err != nil {
		return domain.ListingAnalysis{}, err
	}
	return domain.ListingAnalysis{
		OpportunityType: payload.OpportunityType, ApplicationOpen: payload.ApplicationOpen, Relevant: payload.Relevant,
		MatchingAreas: payload.MatchingAreas, ClassRequirement: payload.ClassRequirement, GPARequirement: payload.GPARequirement,
		Location: payload.Location, WorkModel: payload.WorkModel, Eligibility: payload.Eligibility,
		ApplicationDueAt: payload.ApplicationDueAt, Summary: payload.Summary, Confidence: payload.Confidence,
		NeedsUserDecision: payload.NeedsUserDecision, DecisionQuestion: payload.DecisionQuestion,
	}, nil
}

func decodeSingleJSON(content string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func decodeSingleJSONStrict(content string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func validatePayload(p analysisPayload) error {
	if !oneOf(p.OpportunityType, "staj", "uzun_donem_staj", "part_time", "yeni_mezun", "diger") {
		return fmt.Errorf("invalid opportunity_type %q", p.OpportunityType)
	}
	if !oneOf(p.WorkModel, "is_yerinde", "hibrit", "uzaktan", "belirtilmemis") {
		return fmt.Errorf("invalid work_model %q", p.WorkModel)
	}
	if !oneOf(string(p.Eligibility), string(domain.EligibilitySuitable), string(domain.EligibilityPartlySuitable), string(domain.EligibilityUnsuitable), string(domain.EligibilityNeedsDecision)) {
		return fmt.Errorf("invalid eligibility %q", p.Eligibility)
	}
	if strings.TrimSpace(p.Location) == "" || strings.TrimSpace(p.Summary) == "" {
		return errors.New("location and summary are required")
	}
	if p.Confidence < 0 || p.Confidence > 1 {
		return errors.New("confidence must be between 0 and 1")
	}
	if p.ClassRequirement != nil && (*p.ClassRequirement < 1 || *p.ClassRequirement > 6) {
		return errors.New("class_requirement must be between 1 and 6")
	}
	if p.GPARequirement != nil && (*p.GPARequirement < 0 || *p.GPARequirement > 4) {
		return errors.New("gpa_requirement must be between 0 and 4")
	}
	if p.NeedsUserDecision != (p.Eligibility == domain.EligibilityNeedsDecision) {
		return errors.New("needs_user_decision must match karar_bekliyor eligibility")
	}
	if p.NeedsUserDecision && strings.TrimSpace(p.DecisionQuestion) == "" {
		return errors.New("decision_question is required when user decision is needed")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func analysisJSONSchema() map[string]any {
	properties := map[string]any{
		"opportunity_type": map[string]any{"type": "string", "enum": []string{"staj", "uzun_donem_staj", "part_time", "yeni_mezun", "diger"}},
		"application_open": map[string]any{"type": "boolean"}, "relevant": map[string]any{"type": "boolean"},
		"matching_areas":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"class_requirement":  map[string]any{"type": []string{"integer", "null"}, "minimum": 1, "maximum": 6},
		"gpa_requirement":    map[string]any{"type": []string{"number", "null"}, "minimum": 0, "maximum": 4},
		"location":           map[string]any{"type": "string", "minLength": 1},
		"work_model":         map[string]any{"type": "string", "enum": []string{"is_yerinde", "hibrit", "uzaktan", "belirtilmemis"}},
		"eligibility":        map[string]any{"type": "string", "enum": []string{"uygun", "kismen_uygun", "uygun_degil", "karar_bekliyor"}},
		"application_due_at": map[string]any{"type": []string{"string", "null"}, "format": "date-time"},
		"summary":            map[string]any{"type": "string", "minLength": 1}, "confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		"needs_user_decision": map[string]any{"type": "boolean"}, "decision_question": map[string]any{"type": "string"},
	}
	required := analysisFieldNames()
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
}

func analysisFieldNames() []string {
	result := []string{
		"opportunity_type", "application_open", "relevant", "matching_areas",
		"class_requirement", "gpa_requirement", "location", "work_model",
		"eligibility", "application_due_at", "summary", "confidence",
		"needs_user_decision", "decision_question",
	}
	sort.Strings(result)
	return result
}
