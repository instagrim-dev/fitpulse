ALTER TABLE outbox_dlq
    ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS quarantined_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS quarantine_reason TEXT,
    ADD COLUMN IF NOT EXISTS aggregate_type TEXT,
    ADD COLUMN IF NOT EXISTS aggregate_id TEXT,
    ADD COLUMN IF NOT EXISTS schema_subject TEXT,
    ADD COLUMN IF NOT EXISTS partition_key TEXT;

CREATE INDEX IF NOT EXISTS idx_outbox_dlq_next_retry
    ON outbox_dlq(next_retry_at)
    WHERE quarantined_at IS NULL;
