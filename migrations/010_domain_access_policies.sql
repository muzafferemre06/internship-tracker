ALTER TABLE company_sources ADD COLUMN access_mode TEXT NOT NULL DEFAULT 'legacy'
    CHECK (access_mode IN ('legacy', 'robots', 'public_api', 'manual_only'));
ALTER TABLE company_sources ADD COLUMN access_scope TEXT NOT NULL DEFAULT '';
ALTER TABLE company_sources ADD COLUMN minimum_interval_seconds INTEGER NOT NULL DEFAULT 0
    CHECK (minimum_interval_seconds >= 0);
ALTER TABLE company_sources ADD COLUMN base_cooldown_seconds INTEGER NOT NULL DEFAULT 0
    CHECK (base_cooldown_seconds >= 0);
ALTER TABLE company_sources ADD COLUMN maximum_cooldown_seconds INTEGER NOT NULL DEFAULT 0
    CHECK (maximum_cooldown_seconds >= 0);

CREATE INDEX idx_company_sources_access_scope
    ON company_sources(access_scope, access_mode);
