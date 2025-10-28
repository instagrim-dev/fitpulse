CREATE TABLE IF NOT EXISTS activity_event_log (
    event_log_id      BIGSERIAL PRIMARY KEY,
    event_type        TEXT NOT NULL,
    tenant_id         TEXT,
    schema_id         INTEGER NOT NULL,
    schema_subject    TEXT NOT NULL,
    topic             TEXT NOT NULL,
    partition         INTEGER NOT NULL,
    record_offset     BIGINT NOT NULL,
    payload           JSONB NOT NULL,
    received_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activity_event_log_tenant ON activity_event_log (tenant_id);
CREATE INDEX IF NOT EXISTS idx_activity_event_log_topic_offset ON activity_event_log (topic, partition, record_offset);
