package domain

// Strategy names the source-strategy tier a source is dispatched under (Faz 9).
// Values beyond "legacy_html" are introduced by later phases; "legacy_html"
// covers the pre-Faz-9 hand-written adapters (kariyer_net, lever).
//
// TrackingStatus is a company-level attribute ("active", "manual", "paused").
// "manual" means the company is deliberately watched by hand (a curated
// watchlist entry) rather than scraped, independent of whether any scrape
// attempt has ever failed.
type SourceRegistration struct {
	Key            string
	Company        string
	PriorityGroup  string
	Type           string
	URL            string
	Adapter        string
	Strategy       string
	TrackingStatus string
	Enabled        bool
}
