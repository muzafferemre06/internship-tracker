package analyzer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

const validAnalysisJSON = `{
  "opportunity_type":"staj",
  "application_open":true,
  "relevant":true,
  "matching_areas":["backend"],
  "class_requirement":2,
  "gpa_requirement":3.0,
  "location":"Ankara",
  "work_model":"hibrit",
  "eligibility":"uygun",
  "application_due_at":null,
  "summary":"Backend stajı",
  "confidence":0.94,
  "needs_user_decision":false,
  "decision_question":""
}`

type fakeProvider struct {
	responses []ProviderResponse
	errors    []error
	requests  []ProviderRequest
}

func (*fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) Complete(_ context.Context, request ProviderRequest) (ProviderResponse, error) {
	p.requests = append(p.requests, request)
	index := len(p.requests) - 1
	if index < len(p.errors) && p.errors[index] != nil {
		return ProviderResponse{}, p.errors[index]
	}
	if index < len(p.responses) {
		return p.responses[index], nil
	}
	return ProviderResponse{}, errors.New("unexpected fake provider call")
}

func newTestModelAnalyzer(t *testing.T, provider *fakeProvider) *ModelAnalyzer {
	t.Helper()
	analyzer, err := NewModelAnalyzer(provider, "test/model", CostRates{InputPerMillionUSD: 1, OutputPerMillionUSD: 2})
	if err != nil {
		t.Fatalf("create analyzer: %v", err)
	}
	analyzer.wait = func(context.Context, time.Duration) error { return nil }
	return analyzer
}

func TestModelAnalyzerReturnsStrictValidatedAnalysisAndUsage(t *testing.T) {
	provider := &fakeProvider{responses: []ProviderResponse{{
		Content: validAnalysisJSON,
		Usage:   ProviderUsage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500},
	}}}
	modelAnalyzer := newTestModelAnalyzer(t, provider)

	analysis, err := modelAnalyzer.Analyze(context.Background(), domain.RawListing{
		Title: "Backend Stajyeri", RawText: "Go ve API geliştirme",
	}, CandidateProfile{
		EducationField: "Bilgisayar Teknolojisi", ClassYear: 2, GPA: 3.5,
		FocusAreas: []string{"backend"}, ExperienceAreas: []string{"Go", "API"}, Locations: []string{"Ankara"},
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if analysis.Eligibility != domain.EligibilitySuitable || analysis.Provider != "fake" || analysis.Model != "test/model" {
		t.Fatalf("unexpected analysis: %#v", analysis)
	}
	if analysis.TotalTokens != 1500 || analysis.EstimatedCostUSD != 0.002 {
		t.Fatalf("unexpected usage: %#v", analysis)
	}
	if len(provider.requests) != 1 || provider.requests[0].Model != "test/model" {
		t.Fatalf("model was not forwarded: %#v", provider.requests)
	}
	if !strings.Contains(provider.requests[0].SystemPrompt, `eligibility tam olarak "karar_bekliyor"`) {
		t.Fatalf("decision invariant is missing from system prompt: %q", provider.requests[0].SystemPrompt)
	}
}

func TestModelAnalyzerMinimizesCandidateProfile(t *testing.T) {
	provider := &fakeProvider{responses: []ProviderResponse{{Content: validAnalysisJSON}}}
	modelAnalyzer := newTestModelAnalyzer(t, provider)

	_, err := modelAnalyzer.Analyze(context.Background(), domain.RawListing{Title: "Staj", RawText: "İlan"}, CandidateProfile{
		EducationField: "CTIS", ExperienceAreas: []string{"backend"},
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	encoded := provider.requests[0].Input.(modelInput)
	if encoded.Candidate.EducationField != "CTIS" || len(encoded.Candidate.ExperienceAreas) != 1 {
		t.Fatalf("minimized profile lost required fields: %#v", encoded.Candidate)
	}
}

func TestModelAnalyzerRetriesInvalidJSONSchemaAndTemporaryErrors(t *testing.T) {
	tests := []struct {
		name      string
		responses []ProviderResponse
		errors    []error
	}{
		{name: "broken JSON", responses: []ProviderResponse{{Content: `{`}, {Content: validAnalysisJSON}}},
		{name: "schema error", responses: []ProviderResponse{{Content: strings.Replace(validAnalysisJSON, `"confidence":0.94`, `"confidence":2`, 1)}, {Content: validAnalysisJSON}}},
		{name: "rate limit", errors: []error{&ProviderError{StatusCode: 429, Temporary: true, Err: errors.New("limited")}, nil}, responses: []ProviderResponse{{}, {Content: validAnalysisJSON}}},
		{name: "server error", errors: []error{&ProviderError{StatusCode: 503, Temporary: true, Err: errors.New("unavailable")}, nil}, responses: []ProviderResponse{{}, {Content: validAnalysisJSON}}},
		{name: "timeout", errors: []error{&ProviderError{Temporary: true, Err: context.DeadlineExceeded}, nil}, responses: []ProviderResponse{{}, {Content: validAnalysisJSON}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &fakeProvider{responses: test.responses, errors: test.errors}
			_, err := newTestModelAnalyzer(t, provider).Analyze(context.Background(), domain.RawListing{Title: "Staj"}, CandidateProfile{})
			if err != nil {
				t.Fatalf("analyze after retry: %v", err)
			}
			if len(provider.requests) != 2 {
				t.Fatalf("expected two attempts, got %d", len(provider.requests))
			}
		})
	}
}

func TestModelAnalyzerRequiresEverySchemaField(t *testing.T) {
	invalid := strings.Replace(validAnalysisJSON, `"application_open":true,`, "", 1)
	provider := &fakeProvider{responses: []ProviderResponse{{Content: invalid}, {Content: invalid}, {Content: invalid}}}
	_, err := newTestModelAnalyzer(t, provider).Analyze(context.Background(), domain.RawListing{Title: "Staj"}, CandidateProfile{})
	if err == nil || !strings.Contains(err.Error(), "required field") {
		t.Fatalf("expected missing field error, got %v", err)
	}
}

func TestModelAnalyzerAddsValidationFeedbackToRetry(t *testing.T) {
	invalid := strings.Replace(validAnalysisJSON, `"needs_user_decision":false`, `"needs_user_decision":true`, 1)
	provider := &fakeProvider{responses: []ProviderResponse{{Content: invalid}, {Content: validAnalysisJSON}}}
	_, err := newTestModelAnalyzer(t, provider).Analyze(context.Background(), domain.RawListing{Title: "Staj"}, CandidateProfile{})
	if err != nil {
		t.Fatalf("analyze corrected response: %v", err)
	}
	if len(provider.requests) != 2 || !strings.Contains(provider.requests[1].SystemPrompt, "needs_user_decision must match") {
		t.Fatalf("validation feedback was not included in retry: %#v", provider.requests)
	}
}

func TestModelAnalyzerRejectsUnknownFieldsAndStopsAfterLimit(t *testing.T) {
	invalid := strings.Replace(validAnalysisJSON, `"summary":"Backend stajı"`, `"summary":"Backend stajı","invented":true`, 1)
	provider := &fakeProvider{responses: []ProviderResponse{{Content: invalid}, {Content: invalid}, {Content: invalid}}}
	_, err := newTestModelAnalyzer(t, provider).Analyze(context.Background(), domain.RawListing{Title: "Staj"}, CandidateProfile{})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict schema error, got %v", err)
	}
	if len(provider.requests) != defaultMaxAttempts {
		t.Fatalf("expected bounded retry count, got %d", len(provider.requests))
	}
}

func TestModelAnalyzerDoesNotRetryPermanentProviderError(t *testing.T) {
	provider := &fakeProvider{errors: []error{&ProviderError{StatusCode: 401, Err: errors.New("unauthorized")}}}
	_, err := newTestModelAnalyzer(t, provider).Analyze(context.Background(), domain.RawListing{Title: "Staj"}, CandidateProfile{})
	if err == nil || len(provider.requests) != 1 {
		t.Fatalf("permanent error should not be retried: err=%v calls=%d", err, len(provider.requests))
	}
}

func TestModelAnalyzerAcceptsUnsuitableAndNeedsDecisionOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		eligibility domain.EligibilityStatus
	}{
		{
			name:        "unsuitable",
			content:     strings.Replace(validAnalysisJSON, `"eligibility":"uygun"`, `"eligibility":"uygun_degil"`, 1),
			eligibility: domain.EligibilityUnsuitable,
		},
		{
			name: "needs decision",
			content: strings.NewReplacer(
				`"eligibility":"uygun"`, `"eligibility":"karar_bekliyor"`,
				`"needs_user_decision":false`, `"needs_user_decision":true`,
				`"decision_question":""`, `"decision_question":"Çalışma dönemi uygun mu?"`,
			).Replace(validAnalysisJSON),
			eligibility: domain.EligibilityNeedsDecision,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &fakeProvider{responses: []ProviderResponse{{Content: test.content}}}
			analysis, err := newTestModelAnalyzer(t, provider).Analyze(
				context.Background(), domain.RawListing{Title: "Staj"}, CandidateProfile{},
			)
			if err != nil || analysis.Eligibility != test.eligibility {
				t.Fatalf("unexpected outcome: analysis=%#v err=%v", analysis, err)
			}
		})
	}
}
