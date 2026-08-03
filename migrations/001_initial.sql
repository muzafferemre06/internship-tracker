PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS companies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    priority_group TEXT NOT NULL CHECK (priority_group IN ('primary', 'secondary', 'candidate')),
    tracking_status TEXT NOT NULL DEFAULT 'active' CHECK (tracking_status IN ('active', 'manual', 'paused')),
    discovery_origin TEXT,
    approved_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS company_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    company_id INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    url TEXT NOT NULL,
    adapter_type TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    scan_schedule TEXT,
    last_success_at TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (company_id, url)
);

CREATE TABLE IF NOT EXISTS listings (
    id TEXT PRIMARY KEY,
    company_id INTEGER NOT NULL REFERENCES companies(id),
    source_id INTEGER NOT NULL REFERENCES company_sources(id),
    external_id TEXT,
    title TEXT NOT NULL,
    canonical_url TEXT NOT NULL,
    raw_text TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (company_id, canonical_url)
);

CREATE TABLE IF NOT EXISTS listing_analyses (
    listing_id TEXT PRIMARY KEY REFERENCES listings(id) ON DELETE CASCADE,
    opportunity_type TEXT,
    is_application_open INTEGER CHECK (is_application_open IN (0, 1)),
    is_relevant INTEGER CHECK (is_relevant IN (0, 1)),
    matching_areas_json TEXT NOT NULL DEFAULT '[]',
    class_year_requirement INTEGER,
    gpa_requirement REAL,
    location TEXT,
    work_model TEXT,
    eligibility_status TEXT CHECK (
        eligibility_status IN ('uygun', 'kismen_uygun', 'uygun_degil', 'karar_bekliyor')
    ),
    application_deadline TEXT,
    summary TEXT,
    confidence REAL,
    needs_user_decision INTEGER NOT NULL DEFAULT 0 CHECK (needs_user_decision IN (0, 1)),
    decision_question TEXT,
    provider TEXT,
    model TEXT,
    analyzed_at TEXT,
    processing_status TEXT NOT NULL DEFAULT 'pending' CHECK (
        processing_status IN ('pending', 'processed', 'failed')
    ),
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE TABLE IF NOT EXISTS application_tracking (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id TEXT NOT NULL UNIQUE REFERENCES listings(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (
        status IN ('incelenecek', 'basvuruldu', 'sinav_mulakat', 'olumlu', 'olumsuz')
    ),
    deadline TEXT,
    interview_at TEXT,
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id TEXT REFERENCES listings(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    channel TEXT NOT NULL DEFAULT 'web_push',
    status TEXT NOT NULL CHECK (status IN ('pending', 'sent', 'failed', 'cancelled')),
    sent_at TEXT,
    dedup_key TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scan_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trigger_type TEXT NOT NULL CHECK (trigger_type IN ('manual', 'scheduled')),
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'partial', 'failed')),
    sources_succeeded INTEGER NOT NULL DEFAULT 0,
    sources_failed INTEGER NOT NULL DEFAULT 0,
    new_listings_count INTEGER NOT NULL DEFAULT 0,
    error_summary TEXT
);

CREATE INDEX IF NOT EXISTS idx_sources_enabled ON company_sources(enabled);
CREATE INDEX IF NOT EXISTS idx_listings_last_seen ON listings(last_seen_at);
CREATE INDEX IF NOT EXISTS idx_analyses_eligibility ON listing_analyses(eligibility_status);
CREATE INDEX IF NOT EXISTS idx_applications_status ON application_tracking(status);
CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
