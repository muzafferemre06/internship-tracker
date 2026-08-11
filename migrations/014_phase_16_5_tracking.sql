ALTER TABLE companies ADD COLUMN tracking_phase TEXT NOT NULL DEFAULT ''
    CHECK (tracking_phase IN ('', '16.5'));

ALTER TABLE company_sources ADD COLUMN coverage_reason_code TEXT NOT NULL DEFAULT ''
    CHECK (coverage_reason_code IN ('', 'account_required', 'third_party_restricted',
        'no_public_job_source', 'client_rendered_unverified', 'periodic_program', 'source_unreachable'));
ALTER TABLE company_sources ADD COLUMN last_verified_at TEXT;

CREATE INDEX idx_companies_tracking_phase ON companies(tracking_phase, priority_group);
