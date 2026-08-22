package scraper

import (
	"context"
	"fmt"
	"strings"
)

// SourceSpec carries the configuration a factory needs to build a Source,
// independent of how that configuration arrived (file, DB row, API call).
type SourceSpec struct {
	ID                  string
	Company             string
	PageName            string
	URL                 string
	ListingContainerID  string
	ListingPathPrefix   string
	ListingAllowedHosts []string
	AccessPolicy        *AccessPolicy
}

// SourceDeps carries shared, app-level services some adapters need but that are
// not per-source configuration (e.g. the LLM extractor the Faz 11 "llm_generic"
// adapter depends on). Deterministic adapters ignore it.
type SourceDeps struct {
	Extractor       ListingExtractor
	RecipeStore     RecipeStore
	RecipeLearner   RecipeLearner
	BlockCache      ExtractionBlockStore
	FeedCheckpoints FeedCheckpointStore
}

// SourceFactory builds a Source from a SourceSpec and shared dependencies.
type SourceFactory func(spec SourceSpec, deps SourceDeps) (Source, error)

// adapterFactories is the data-driven dispatch table Faz 9 introduces:
// registering a new adapter here is the only code change required to make
// it available to every strategy that references it (see
// staj-takip-spec-v2.md §16, Faz 9). It replaces a hardcoded switch in the
// orchestrator wiring.
var adapterFactories = map[string]SourceFactory{
	"kariyer_net": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		pageName := spec.PageName
		if strings.TrimSpace(pageName) == "" {
			pageName = spec.Company
		}
		return NewKariyerNetSource(spec.ID, spec.Company, pageName, spec.URL, nil)
	},
	"lever": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		return NewLeverSource(spec.ID, spec.Company, spec.URL, nil)
	},
	"lever_board": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		return NewLeverBoardSource(spec.ID, spec.Company, spec.URL, nil)
	},
	// Faz 10 structured-data-first adapters (deterministic, AI-free).
	"json_ld": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		return NewJSONLDSource(spec.ID, spec.Company, spec.URL, nil)
	},
	"career_links": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		return NewCareerLinksSourceWithAllowedHosts(spec.ID, spec.Company, spec.URL, spec.ListingContainerID, spec.ListingPathPrefix, spec.ListingAllowedHosts, nil)
	},
	"greenhouse": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		return NewGreenhouseSource(spec.ID, spec.Company, spec.URL, nil)
	},
	"ashby_board": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		return NewAshbyBoardSource(spec.ID, spec.Company, spec.URL, nil)
	},
	// Faz 11 chaotic/bespoke adapter: reduce-then-LLM extraction.
	"llm_generic": func(spec SourceSpec, deps SourceDeps) (Source, error) {
		if deps.Extractor == nil {
			return nil, fmt.Errorf("adapter %q requires a configured listing extractor", "llm_generic")
		}
		return NewLLMGenericSource(spec.ID, spec.Company, spec.URL, deps.Extractor, nil, deps.BlockCache)
	},
	"learned_selector": func(spec SourceSpec, deps SourceDeps) (Source, error) {
		if deps.RecipeStore == nil || deps.RecipeLearner == nil {
			return nil, fmt.Errorf("adapter %q requires a recipe store and learner", "learned_selector")
		}
		return NewLearnedSelectorSource(spec.ID, spec.Company, spec.URL, deps.RecipeStore, deps.RecipeLearner, nil)
	},
	// Faz 20 open-feed adapter: RSS 2.0 / Atom with a persistent checkpoint.
	"rss_feed": func(spec SourceSpec, deps SourceDeps) (Source, error) {
		if deps.FeedCheckpoints == nil {
			return nil, fmt.Errorf("adapter %q requires a feed checkpoint store", "rss_feed")
		}
		return NewRSSFeedSource(spec.ID, spec.Company, spec.URL, deps.FeedCheckpoints, nil)
	},
	// Faz 20 same-domain feed discovery: spec.URL is a career/news page, not a
	// feed URL directly; the feed link is discovered from it at setup time.
	"rss_discover": func(spec SourceSpec, deps SourceDeps) (Source, error) {
		if deps.FeedCheckpoints == nil {
			return nil, fmt.Errorf("adapter %q requires a feed checkpoint store", "rss_discover")
		}
		return NewRSSFeedSourceFromPage(context.Background(), spec.ID, spec.Company, spec.URL, deps.FeedCheckpoints, nil)
	},
}

// NewSource dispatches to the registered factory for the given adapter name.
func NewSource(adapter string, spec SourceSpec, deps SourceDeps) (Source, error) {
	factory, ok := adapterFactories[adapter]
	if !ok {
		return nil, fmt.Errorf("unsupported adapter %q", adapter)
	}
	source, err := factory(spec, deps)
	if err != nil {
		return nil, err
	}
	if spec.AccessPolicy != nil {
		return WithAccessPolicy(source, *spec.AccessPolicy), nil
	}
	return source, nil
}

// WithAccessPolicy makes the resolved, configuration-owned access policy the
// authoritative policy for a source, regardless of adapter defaults.
func WithAccessPolicy(source Source, policy AccessPolicy) ProtectedSource {
	return configuredAccessSource{Source: source, policy: policy}
}

type configuredAccessSource struct {
	Source
	policy AccessPolicy
}

func (s configuredAccessSource) AccessPolicy() AccessPolicy { return s.policy }

// SupportsAdapter reports whether an adapter has a registered factory.
func SupportsAdapter(adapter string) bool {
	_, ok := adapterFactories[adapter]
	return ok
}
