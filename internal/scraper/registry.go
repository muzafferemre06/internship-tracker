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

// SourceFactory builds a Source from a SourceSpec.
type SourceFactory func(spec SourceSpec) (Source, error)

// adapterFactories is the data-driven dispatch table Faz 9 introduces:
// registering a new adapter here is the only code change required to make
// it available to every strategy that references it (see
// staj-takip-spec-v2.md §16, Faz 9). It replaces a hardcoded switch in the
// orchestrator wiring.
var adapterFactories = map[string]SourceFactory{
	"kariyer_net": func(spec SourceSpec) (Source, error) {
		pageName := spec.PageName
		if strings.TrimSpace(pageName) == "" {
			pageName = spec.Company
		}
		return NewKariyerNetSource(spec.ID, spec.Company, pageName, spec.URL, nil)
	},
	"lever": func(spec SourceSpec) (Source, error) {
		return NewLeverSource(spec.ID, spec.Company, spec.URL, nil)
	},
}

// NewSource dispatches to the registered factory for the given adapter name.
func NewSource(adapter string, spec SourceSpec) (Source, error) {
	factory, ok := adapterFactories[adapter]
	if !ok {
		return nil, fmt.Errorf("unsupported adapter %q", adapter)
	}
	return factory(spec)
}

// SupportsAdapter reports whether an adapter has a registered factory.
func SupportsAdapter(adapter string) bool {
	_, ok := adapterFactories[adapter]
	return ok
}
