ALTER TABLE opportunities ADD COLUMN opportunity_type TEXT NOT NULL DEFAULT 'diger';
ALTER TABLE opportunities ADD COLUMN visibility_layer TEXT NOT NULL DEFAULT 'incelenecek'
    CHECK (visibility_layer IN ('bildirim', 'firsatlar', 'incelenecek', 'elenen'));
ALTER TABLE opportunities ADD COLUMN match_score INTEGER NOT NULL DEFAULT 0 CHECK (match_score BETWEEN 0 AND 100);
ALTER TABLE opportunities ADD COLUMN focus_score INTEGER NOT NULL DEFAULT 0 CHECK (focus_score BETWEEN 0 AND 40);
ALTER TABLE opportunities ADD COLUMN type_score INTEGER NOT NULL DEFAULT 0 CHECK (type_score BETWEEN 0 AND 25);
ALTER TABLE opportunities ADD COLUMN location_score INTEGER NOT NULL DEFAULT 0 CHECK (location_score BETWEEN 0 AND 20);
ALTER TABLE opportunities ADD COLUMN eligibility_score INTEGER NOT NULL DEFAULT 0 CHECK (eligibility_score BETWEEN 0 AND 10);
ALTER TABLE opportunities ADD COLUMN requirement_score INTEGER NOT NULL DEFAULT 0 CHECK (requirement_score BETWEEN 0 AND 5);
ALTER TABLE opportunities ADD COLUMN assessment_reason TEXT NOT NULL DEFAULT 'not_assessed';
ALTER TABLE opportunities ADD COLUMN assessed_at TEXT;

CREATE TABLE opportunity_evidence (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    opportunity_id TEXT NOT NULL REFERENCES opportunities(id) ON DELETE CASCADE,
    listing_id TEXT REFERENCES listings(id) ON DELETE CASCADE,
    program_window_id INTEGER REFERENCES program_windows(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL CHECK (source_type IN ('web', 'program_window', 'rss', 'email')),
    source_url TEXT NOT NULL,
    first_observed_at TEXT NOT NULL,
    last_observed_at TEXT NOT NULL,
    freshness_at TEXT,
    CHECK ((listing_id IS NOT NULL) != (program_window_id IS NOT NULL)),
    UNIQUE(listing_id),
    UNIQUE(program_window_id)
);

INSERT INTO opportunity_evidence(opportunity_id, listing_id, source_type, source_url, first_observed_at, last_observed_at, freshness_at)
SELECT listing_opportunities.opportunity_id, listings.id, 'web', listings.canonical_url,
    listings.first_seen_at, listings.last_seen_at, listings.last_seen_at
FROM listings JOIN listing_opportunities ON listing_opportunities.listing_id = listings.id;

CREATE INDEX idx_opportunities_visibility ON opportunities(visibility_layer, status, match_score DESC);
CREATE INDEX idx_opportunity_evidence_opportunity ON opportunity_evidence(opportunity_id, last_observed_at DESC);
