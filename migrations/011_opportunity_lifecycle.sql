ALTER TABLE opportunities ADD COLUMN lifecycle_status TEXT NOT NULL DEFAULT 'yeni'
    CHECK (lifecycle_status IN ('yeni', 'acik', 'incelendi', 'basvuruldu', 'suresi_doldu', 'kapatildi', 'arsivlendi'));

CREATE INDEX idx_opportunities_lifecycle_updated
    ON opportunities(lifecycle_status, updated_at DESC)
    WHERE status = 'active';
