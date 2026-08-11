package domain

import "time"

// Strategy names the source-strategy tier a source is dispatched under (Faz 9).
// Values beyond "legacy_html" are introduced by later phases; "legacy_html"
// covers the pre-Faz-9 hand-written adapters (kariyer_net, lever).
//
// TrackingStatus is a company-level attribute ("active", "manual", "paused").
// "manual" means the company is deliberately watched by hand (a curated
// watchlist entry) rather than scraped, independent of whether any scrape
// attempt has ever failed.
type SourceRegistration struct {
	Key             string
	Company         string
	PriorityGroup   string
	Type            string
	URL             string
	Adapter         string
	Strategy        string
	TrackingStatus  string
	Enabled         bool
	AccessMode      string
	AccessScope     string
	MinimumInterval time.Duration
	BaseCooldown    time.Duration
	MaximumCooldown time.Duration
	CoverageStatus  string
	CoverageReason  string
	TrustLevel      string
}

// ProgramWindow models a period-based application program independently from
// job listings. Its public page may remain available while applications close.
type ProgramWindow struct {
	Key            string
	Company        string
	Name           string
	Type           string
	URL            string
	Status         string
	OpensAt        *time.Time
	ClosesAt       *time.Time
	LastVerifiedAt *time.Time
}

// SourceRecipe is a versioned deterministic extraction rule learned for one
// source. The model proposes selectors only during setup/repair; ordinary scans
// execute the stored recipe without a model call.
type SourceRecipe struct {
	SourceKey          string
	Version            int
	IdentitySelector   string
	IdentityText       string
	ListingSelector    string
	TitleSelector      string
	LinkSelector       string
	GoldenListingCount int
	GoldenFingerprint  string
}
