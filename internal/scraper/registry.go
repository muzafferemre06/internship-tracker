package scraper

import (
	"fmt"
	"strings"
)

// SourceSpec carries the configuration a factory needs to build a Source,
// independent of how that configuration arrived (file, DB row, API call).
type SourceSpec struct {
	ID       string
	Company  string
	PageName string
	URL      string
}

// SourceDeps carries shared, app-level services some adapters need but that are
// not per-source configuration (e.g. the LLM extractor the Faz 11 "llm_generic"
// adapter depends on). Deterministic adapters ignore it.
type SourceDeps struct {
	Extractor ListingExtractor
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
	// Faz 10 structured-data-first adapters (deterministic, AI-free).
	"json_ld": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		return NewJSONLDSource(spec.ID, spec.Company, spec.URL, nil)
	},
	"greenhouse": func(spec SourceSpec, _ SourceDeps) (Source, error) {
		return NewGreenhouseSource(spec.ID, spec.Company, spec.URL, nil)
	},
	// Faz 11 chaotic/bespoke adapter: reduce-then-LLM extraction.
	"llm_generic": func(spec SourceSpec, deps SourceDeps) (Source, error) {
		if deps.Extractor == nil {
			return nil, fmt.Errorf("adapter %q requires a configured listing extractor", "llm_generic")
		}
		return NewLLMGenericSource(spec.ID, spec.Company, spec.URL, deps.Extractor, nil)
	},
}

// NewSource dispatches to the registered factory for the given adapter name.
func NewSource(adapter string, spec SourceSpec, deps SourceDeps) (Source, error) {
	factory, ok := adapterFactories[adapter]
	if !ok {
		return nil, fmt.Errorf("unsupported adapter %q", adapter)
	}
	return factory(spec, deps)
}

// SupportsAdapter reports whether an adapter has a registered factory.
func SupportsAdapter(adapter string) bool {
	_, ok := adapterFactories[adapter]
	return ok
}
