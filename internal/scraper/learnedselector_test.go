package scraper

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type memoryRecipeStore struct {
	recipe domain.SourceRecipe
	has    bool
	saves  int
}

func (s *memoryRecipeStore) LoadSourceRecipe(context.Context, string) (domain.SourceRecipe, bool, error) {
	return s.recipe, s.has, nil
}

func (s *memoryRecipeStore) SaveSourceRecipe(_ context.Context, recipe domain.SourceRecipe) (domain.SourceRecipe, error) {
	s.saves++
	recipe.Version = s.saves
	s.recipe, s.has = recipe, true
	return recipe, nil
}

func (s *memoryRecipeStore) UpdateSourceRecipeSnapshot(_ context.Context, _ string, version, count int, fingerprint string) error {
	if !s.has || s.recipe.Version != version {
		return errors.New("recipe version not found")
	}
	s.recipe.GoldenListingCount = count
	s.recipe.GoldenFingerprint = fingerprint
	return nil
}

type scriptedRecipeLearner struct {
	calls   int
	recipes []domain.SourceRecipe
	reasons []string
}

func (l *scriptedRecipeLearner) LearnRecipe(_ context.Context, request RecipeLearningRequest) (domain.SourceRecipe, error) {
	l.calls++
	l.reasons = append(l.reasons, request.Reason)
	if len(l.recipes) == 0 {
		return domain.SourceRecipe{}, errors.New("no scripted recipe")
	}
	recipe := l.recipes[0]
	l.recipes = l.recipes[1:]
	return recipe, nil
}

func learnedFixtureClient(t *testing.T, current *[]byte) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(*current))}, nil
	})}
}

func TestLearnedSelectorPersistsRecipeAndRunsRestartWithoutModel(t *testing.T) {
	fixture, err := os.ReadFile("testdata/learnedselector/layout-v1.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	store := &memoryRecipeStore{}
	learner := &scriptedRecipeLearner{recipes: []domain.SourceRecipe{{
		IdentitySelector: "#careers", IdentityText: "Vega Yazılım Kariyer",
		ListingSelector: ".opening", TitleSelector: ".title", LinkSelector: "a.apply",
	}}}
	client := learnedFixtureClient(t, &fixture)

	first, err := NewLearnedSelectorSource("vega-learned", "Vega Yazılım", "https://careers.vega.example/jobs", store, learner, client)
	if err != nil {
		t.Fatalf("create first source: %v", err)
	}
	listings, err := first.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if len(listings) != 2 || learner.calls != 1 || store.saves != 1 || store.recipe.Version != 1 {
		t.Fatalf("first fetch did not learn exactly one recipe: listings=%d calls=%d recipe=%#v", len(listings), learner.calls, store.recipe)
	}

	// A new source instance simulates a process restart: the recipe must come
	// from SQLite/store and execute without another model call.
	second, err := NewLearnedSelectorSource("vega-learned", "Vega Yazılım", "https://careers.vega.example/jobs", store, learner, client)
	if err != nil {
		t.Fatalf("create restarted source: %v", err)
	}
	listings, err = second.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("restart fetch: %v", err)
	}
	if len(listings) != 2 || learner.calls != 1 {
		t.Fatalf("persisted recipe was not reused: listings=%d calls=%d", len(listings), learner.calls)
	}
}

func TestLearnedSelectorRepairsGoldenSnapshotDropInsteadOfReturningSilentZero(t *testing.T) {
	v1, err := os.ReadFile("testdata/learnedselector/layout-v1.html")
	if err != nil {
		t.Fatalf("read v1 fixture: %v", err)
	}
	v2, err := os.ReadFile("testdata/learnedselector/layout-v2.html")
	if err != nil {
		t.Fatalf("read v2 fixture: %v", err)
	}
	current := v1
	store := &memoryRecipeStore{}
	learner := &scriptedRecipeLearner{recipes: []domain.SourceRecipe{
		{IdentitySelector: "#careers", IdentityText: "Vega Yazılım Kariyer", ListingSelector: ".opening", TitleSelector: ".title", LinkSelector: "a.apply"},
		{IdentitySelector: ".jobs-board", IdentityText: "Vega Yazılım Kariyer", ListingSelector: ".position-card", TitleSelector: ".position-name", LinkSelector: "a.position-link"},
	}}
	source, err := NewLearnedSelectorSource("vega-learned", "Vega Yazılım", "https://careers.vega.example/jobs", store, learner, learnedFixtureClient(t, &current))
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := source.FetchListings(context.Background()); err != nil {
		t.Fatalf("learn v1: %v", err)
	}
	current = v2
	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("repair v2: %v", err)
	}
	if len(listings) != 2 || learner.calls != 2 || store.recipe.Version != 2 {
		t.Fatalf("layout change was not repaired: listings=%d calls=%d recipe=%#v", len(listings), learner.calls, store.recipe)
	}
	if len(learner.reasons) != 2 || learner.reasons[1] == "" {
		t.Fatalf("repair reason was not supplied to learner: %#v", learner.reasons)
	}
}

func TestLearnedSelectorFailedRepairIsSourceError(t *testing.T) {
	fixture, err := os.ReadFile("testdata/learnedselector/layout-v2.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	store := &memoryRecipeStore{has: true, saves: 1, recipe: domain.SourceRecipe{
		SourceKey: "vega-learned", Version: 1, IdentitySelector: "#careers",
		IdentityText: "Vega Yazılım Kariyer", ListingSelector: ".opening",
		TitleSelector: ".title", LinkSelector: "a.apply", GoldenListingCount: 2,
	}}
	learner := &scriptedRecipeLearner{recipes: []domain.SourceRecipe{{ListingSelector: "???"}}}
	source, err := NewLearnedSelectorSource("vega-learned", "Vega Yazılım", "https://careers.vega.example/jobs", store, learner, learnedFixtureClient(t, &fixture))
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := source.FetchListings(context.Background()); err == nil {
		t.Fatal("invalid repair must be a source error, not a silent zero result")
	}
	if store.saves != 1 {
		t.Fatalf("invalid repaired recipe must not replace active recipe: saves=%d", store.saves)
	}
}
