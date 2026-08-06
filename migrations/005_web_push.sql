ALTER TABLE listing_analyses ADD COLUMN first_processed_at TEXT;

UPDATE listing_analyses
SET first_processed_at = analyzed_at
WHERE processing_status = 'processed' AND first_processed_at IS NULL;

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint TEXT NOT NULL UNIQUE CHECK (length(endpoint) BETWEEN 1 AND 4096),
    endpoint_hash TEXT NOT NULL UNIQUE CHECK (length(endpoint_hash) = 64),
    p256dh TEXT NOT NULL CHECK (length(p256dh) BETWEEN 1 AND 256),
    auth TEXT NOT NULL CHECK (length(auth) BETWEEN 1 AND 128),
    expiration_at TEXT,
    failure_count INTEGER NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
    last_success_at TEXT,
    last_failure_at TEXT,
    last_status_code INTEGER,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS notification_payloads (
    notification_id INTEGER PRIMARY KEY REFERENCES notifications(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    body TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 240),
    target_url TEXT NOT NULL CHECK (target_url LIKE '/%' AND length(target_url) <= 1024),
    topic TEXT NOT NULL CHECK (length(topic) BETWEEN 1 AND 32)
);

CREATE TABLE IF NOT EXISTS notification_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    notification_id INTEGER NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    subscription_id INTEGER REFERENCES push_subscriptions(id) ON DELETE SET NULL,
    subscription_endpoint_hash TEXT NOT NULL CHECK (length(subscription_endpoint_hash) = 64),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'sending', 'sent', 'failed', 'cancelled')
    ),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TEXT,
    lease_until TEXT,
    last_status_code INTEGER,
    last_error TEXT,
    sent_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (notification_id, subscription_endpoint_hash)
);

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_expiration
    ON push_subscriptions(expiration_at);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_due
    ON notification_deliveries(status, next_attempt_at, lease_until);
