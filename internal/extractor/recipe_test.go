package extractor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/scraper"
)

func TestGeminiRecipeLearnerMapsStrictRecipeAndMinimizesInput(t *testing.T) {
	provider := &fakeProvider{response: `{
		"identity_selector":"#careers",
		"identity_text":"Vega Kariyer",
		"listing_selector":".opening",
		"title_selector":".title",
		"link_selector":"a.apply"
	}`}
	learner, err := NewGeminiRecipeLearner(provider, "gemini-3.1-flash-lite")
	if err != nil {
		t.Fatalf("create recipe learner: %v", err)
	}
	recipe, err := learner.LearnRecipe(context.Background(), scraper.RecipeLearningRequest{
		SourceKey: "vega", Company: "Vega", PageURL: "https://vega.example/jobs",
		Document:      "<main id=\"careers\"><article class=\"opening\">...</article></main>",
		Reason:        "schema_validation_failed",
		CurrentRecipe: &domain.SourceRecipe{Version: 1, ListingSelector: ".old"},
	})
	if err != nil {
		t.Fatalf("learn recipe: %v", err)
	}
	if recipe.ListingSelector != ".opening" || recipe.LinkSelector != "a.apply" {
		t.Fatalf("unexpected recipe: %#v", recipe)
	}
	encoded, _ := json.Marshal(provider.lastRequest.Input)
	if !strings.Contains(string(encoded), `"reason":"schema_validation_failed"`) ||
		!strings.Contains(string(encoded), `opening`) {
		t.Fatalf("recipe learner input is missing bounded document/reason: %s", encoded)
	}
	if provider.lastRequest.JSONSchema == nil {
		t.Fatal("recipe learning must request a strict JSON schema")
	}
}

func TestGeminiRecipeLearnerRejectsUnknownOrMissingFields(t *testing.T) {
	for _, response := range []string{
		`{"identity_selector":"#x","identity_text":"X","listing_selector":".job","title_selector":"h2","link_selector":"a","extra":true}`,
		`{"identity_selector":"#x","identity_text":"X","listing_selector":".job","title_selector":"h2"}`,
	} {
		provider := &fakeProvider{response: response}
		learner, err := NewGeminiRecipeLearner(provider, "model")
		if err != nil {
			t.Fatalf("create learner: %v", err)
		}
		if _, err := learner.LearnRecipe(context.Background(), scraper.RecipeLearningRequest{}); err == nil {
			t.Fatalf("invalid strict output must fail: %s", response)
		}
	}
}

func TestNewGeminiRecipeLearnerValidatesDependencies(t *testing.T) {
	if _, err := NewGeminiRecipeLearner(nil, "model"); err == nil {
		t.Fatal("nil provider must fail")
	}
	if _, err := NewGeminiRecipeLearner(&fakeProvider{}, ""); err == nil {
		t.Fatal("empty model must fail")
	}
}
