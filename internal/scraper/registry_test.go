package scraper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type stubExtractor struct{}

func (stubExtractor) Name() string { return "stub" }
func (stubExtractor) Extract(context.Context, ExtractionRequest) (ExtractionResult, error) {
	return ExtractionResult{}, nil
}

func TestNewSourceAppliesConfiguredAccessPolicy(t *testing.T) {
	policy := AccessPolicy{
		Mode: "robots", Scope: "careers.example.test",
		TargetURL:       "https://careers.example.test/jobs",
		MinimumInterval: 2 * time.Second, BaseCooldown: time.Minute, MaximumCooldown: time.Hour,
	}
	source, err := NewSource("json_ld", SourceSpec{
		ID: "example", Company: "Example", URL: policy.TargetURL, AccessPolicy: &policy,
	}, SourceDeps{})
	if err != nil {
		t.Fatal(err)
	}
	protected, ok := source.(ProtectedSource)
	if !ok {
		t.Fatalf("configured source does not expose its access policy: %T", source)
	}
	if got := protected.AccessPolicy(); !reflect.DeepEqual(got, policy) {
		t.Fatalf("access policy=%#v, want %#v", got, policy)
	}
}

func TestNewSourceDispatchesRegisteredAdapters(t *testing.T) {
	cases := []struct {
		adapter string
		spec    SourceSpec
		deps    SourceDeps
	}{
		{
			adapter: "kariyer_net",
			spec: SourceSpec{
				ID:      "meteksan-kariyer-net",
				Company: "Meteksan",
				URL:     "https://www.kariyer.net/firma-profil/meteksan",
			},
		},
		{
			adapter: "lever",
			spec: SourceSpec{
				ID:      "commencis-lever",
				Company: "Commencis",
				URL:     "https://jobs.lever.co/commencis/04a5cd98-ab26-4b48-bb64-3397ffe79a55",
			},
		},
		{
			adapter: "json_ld",
			spec: SourceSpec{
				ID:      "northstar-careers",
				Company: "Northstar Robotics",
				URL:     "https://careers.northstar.example/",
			},
		},
		{
			adapter: "greenhouse",
			spec: SourceSpec{
				ID:      "acme-greenhouse",
				Company: "Acme Robotics",
				URL:     "https://boards-api.greenhouse.io/v1/boards/acmerobotics/jobs",
			},
		},
		{
			adapter: "llm_generic",
			spec: SourceSpec{
				ID:      "vega-careers",
				Company: "Vega Yazılım",
				URL:     "https://kariyer.vega.example/kariyer",
			},
			deps: SourceDeps{Extractor: stubExtractor{}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.adapter, func(t *testing.T) {
			if !SupportsAdapter(testCase.adapter) {
				t.Fatalf("expected %q to be a supported adapter", testCase.adapter)
			}
			source, err := NewSource(testCase.adapter, testCase.spec, testCase.deps)
			if err != nil {
				t.Fatalf("build source: %v", err)
			}
			if source.Name() != testCase.spec.ID {
				t.Fatalf("expected source name %q, got %q", testCase.spec.ID, source.Name())
			}
		})
	}
}

func TestNewSourceLLMGenericRequiresExtractor(t *testing.T) {
	_, err := NewSource("llm_generic", SourceSpec{ID: "x", Company: "X", URL: "https://careers.example/"}, SourceDeps{})
	if err == nil {
		t.Fatal("expected error when llm_generic has no configured extractor")
	}
}

func TestNewSourceLearnedSelectorRequiresRecipeDependencies(t *testing.T) {
	spec := SourceSpec{ID: "x", Company: "X", URL: "https://careers.example/"}
	if _, err := NewSource("learned_selector", spec, SourceDeps{}); err == nil {
		t.Fatal("learned_selector without recipe store and learner must fail")
	}
	store := &memoryRecipeStore{}
	learner := &scriptedRecipeLearner{}
	if _, err := NewSource("learned_selector", spec, SourceDeps{RecipeStore: store, RecipeLearner: learner}); err != nil {
		t.Fatalf("learned_selector with dependencies must be registered: %v", err)
	}
}

func TestNewSourceRejectsUnknownAdapter(t *testing.T) {
	if SupportsAdapter("does_not_exist") {
		t.Fatalf("expected unknown adapter to be unsupported")
	}
	if _, err := NewSource("does_not_exist", SourceSpec{ID: "x", Company: "X", URL: "https://example.test"}, SourceDeps{}); err == nil {
		t.Fatalf("expected error for unknown adapter")
	}
}
