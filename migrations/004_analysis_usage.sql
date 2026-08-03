ALTER TABLE listing_analyses ADD COLUMN prompt_tokens INTEGER NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0);
ALTER TABLE listing_analyses ADD COLUMN completion_tokens INTEGER NOT NULL DEFAULT 0 CHECK (completion_tokens >= 0);
ALTER TABLE listing_analyses ADD COLUMN total_tokens INTEGER NOT NULL DEFAULT 0 CHECK (total_tokens >= 0);
ALTER TABLE listing_analyses ADD COLUMN estimated_cost_usd REAL NOT NULL DEFAULT 0 CHECK (estimated_cost_usd >= 0);

CREATE INDEX IF NOT EXISTS idx_analyses_processing_status
    ON listing_analyses(processing_status);
