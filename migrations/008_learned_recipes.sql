ALTER TABLE company_sources ADD COLUMN strategy_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE company_sources ADD COLUMN last_listing_count INTEGER;
ALTER TABLE company_sources ADD COLUMN last_listing_fingerprint TEXT;

CREATE TABLE source_extraction_recipes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id INTEGER NOT NULL REFERENCES company_sources(id) ON DELETE CASCADE,
    version INTEGER NOT NULL CHECK (version > 0),
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    identity_selector TEXT NOT NULL,
    identity_text TEXT NOT NULL,
    listing_selector TEXT NOT NULL,
    title_selector TEXT NOT NULL,
    link_selector TEXT NOT NULL,
    golden_listing_count INTEGER NOT NULL CHECK (golden_listing_count >= 0),
    golden_fingerprint TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source_id, version)
);

CREATE UNIQUE INDEX idx_source_extraction_recipes_active
    ON source_extraction_recipes(source_id) WHERE active = 1;

CREATE TABLE source_extraction_block_cache (
    source_id INTEGER NOT NULL REFERENCES company_sources(id) ON DELETE CASCADE,
    block_hash TEXT NOT NULL,
    listings_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source_id, block_hash)
);
