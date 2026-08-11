CREATE TABLE IF NOT EXISTS program_windows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    company_id INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    program_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    program_type TEXT NOT NULL,
    url TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('open', 'closed', 'unknown')),
    opens_at TEXT,
    closes_at TEXT,
    last_verified_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (opens_at IS NULL OR closes_at IS NULL OR opens_at <= closes_at)
);

CREATE INDEX IF NOT EXISTS idx_program_windows_company_status
    ON program_windows(company_id, status);
