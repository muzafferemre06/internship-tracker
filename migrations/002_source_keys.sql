ALTER TABLE company_sources ADD COLUMN source_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sources_key ON company_sources(source_key);
