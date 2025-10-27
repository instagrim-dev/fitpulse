ALTER TABLE outbox_dlq
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS last_attempt_at,
    DROP COLUMN IF EXISTS next_retry_at,
    DROP COLUMN IF EXISTS quarantined_at,
    DROP COLUMN IF EXISTS quarantine_reason,
    DROP COLUMN IF EXISTS aggregate_type,
    DROP COLUMN IF EXISTS aggregate_id,
    DROP COLUMN IF EXISTS schema_subject,
    DROP COLUMN IF EXISTS partition_key;

DROP INDEX IF EXISTS idx_outbox_dlq_next_retry;
