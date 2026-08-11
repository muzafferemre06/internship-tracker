ALTER TABLE company_sources ADD COLUMN coverage_status TEXT NOT NULL DEFAULT 'automatic'
    CHECK (coverage_status IN ('automatic', 'feed', 'manual', 'researching', 'broken'));
ALTER TABLE company_sources ADD COLUMN coverage_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE company_sources ADD COLUMN trust_level TEXT NOT NULL DEFAULT 'aggregator'
    CHECK (trust_level IN ('official_company', 'official_ats', 'verified_newsletter', 'aggregator'));

CREATE INDEX idx_company_sources_coverage
    ON company_sources(coverage_status, trust_level);
