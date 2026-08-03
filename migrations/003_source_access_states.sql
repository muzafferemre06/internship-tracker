CREATE TABLE IF NOT EXISTS source_access_states (
    scope TEXT PRIMARY KEY,
    failure_count INTEGER NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
    last_attempt_at TEXT,
    next_allowed_at TEXT,
    blocked_until TEXT,
    last_success_at TEXT,
    last_status_code INTEGER,
    last_server TEXT,
    last_cf_ray TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_source_access_next_allowed
    ON source_access_states(next_allowed_at);
