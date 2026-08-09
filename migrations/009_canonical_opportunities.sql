CREATE TABLE opportunities (
    id TEXT PRIMARY KEY,
    company_id INTEGER NOT NULL REFERENCES companies(id),
    normalized_title TEXT NOT NULL,
    normalized_location TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'merged')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE listing_opportunities (
    listing_id TEXT PRIMARY KEY REFERENCES listings(id) ON DELETE CASCADE,
    opportunity_id TEXT NOT NULL REFERENCES opportunities(id),
    match_method TEXT NOT NULL CHECK (match_method IN ('backfill', 'new', 'auto', 'split')),
    title_score REAL NOT NULL DEFAULT 1 CHECK (title_score BETWEEN 0 AND 1),
    match_reason TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE opportunity_match_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id TEXT NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    from_opportunity_id TEXT REFERENCES opportunities(id),
    candidate_opportunity_id TEXT REFERENCES opportunities(id),
    outcome TEXT NOT NULL CHECK (outcome IN ('auto_merge', 'ambiguous', 'split')),
    title_score REAL NOT NULL CHECK (title_score BETWEEN 0 AND 1),
    reason TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO opportunities(id, company_id, normalized_title, normalized_location, status, created_at, updated_at)
SELECT 'opp-' || id, company_id, '', '', 'active', created_at, updated_at
FROM listings;

INSERT INTO listing_opportunities(listing_id, opportunity_id, match_method, title_score, match_reason, created_at, updated_at)
SELECT id, 'opp-' || id, 'backfill', 1, 'migration_backfill', created_at, updated_at
FROM listings;

ALTER TABLE notifications ADD COLUMN opportunity_id TEXT REFERENCES opportunities(id);

UPDATE notifications
SET opportunity_id = (
    SELECT listing_opportunities.opportunity_id
    FROM listing_opportunities
    WHERE listing_opportunities.listing_id = notifications.listing_id
);

CREATE INDEX idx_opportunities_company_status
    ON opportunities(company_id, status);

CREATE INDEX idx_listing_opportunities_opportunity
    ON listing_opportunities(opportunity_id);

CREATE INDEX idx_opportunity_match_events_listing
    ON opportunity_match_events(listing_id, created_at);

CREATE UNIQUE INDEX idx_opportunity_match_events_decision
    ON opportunity_match_events(
        listing_id,
        IFNULL(from_opportunity_id, ''),
        IFNULL(candidate_opportunity_id, ''),
        outcome,
        reason
    );
