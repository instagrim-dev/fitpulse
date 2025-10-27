CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS accounts (
    account_id     UUID PRIMARY KEY,
    tenant_id      UUID NOT NULL,
    email_hash     BYTEA NOT NULL,
    email_cipher   TEXT NOT NULL,
    disabled       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_tenant_emailhash
    ON accounts(tenant_id, email_hash);

ALTER TABLE accounts FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_accounts ON accounts;
CREATE POLICY tenant_isolation_accounts
  ON accounts USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE IF NOT EXISTS account_idempotency (
    tenant_id       UUID NOT NULL,
    idempotency_key TEXT NOT NULL,
    account_id      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, idempotency_key),
    FOREIGN KEY (account_id) REFERENCES accounts(account_id)
);

ALTER TABLE account_idempotency FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_account_idem ON account_idempotency;
CREATE POLICY tenant_isolation_account_idem
  ON account_idempotency USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE IF NOT EXISTS activities (
    activity_id     UUID PRIMARY KEY,
    tenant_id       UUID NOT NULL,
    user_id         UUID NOT NULL,
    activity_type   TEXT NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL,
    duration_min    INTEGER NOT NULL,
    source          TEXT NOT NULL,
    idempotency_key TEXT,
    version         TEXT NOT NULL DEFAULT 'v1',
    processing_state TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_activities_idem
    ON activities(tenant_id, user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_activities_user_cursor
    ON activities(user_id, started_at DESC, activity_id DESC);

ALTER TABLE activities FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_activities ON activities;
CREATE POLICY tenant_isolation_activities
  ON activities USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE IF NOT EXISTS outbox (
    event_id        BIGSERIAL PRIMARY KEY,
    tenant_id       UUID NOT NULL,
    aggregate_type  TEXT NOT NULL,
    aggregate_id    TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    topic           TEXT NOT NULL,
    schema_subject  TEXT NOT NULL,
    partition_key   TEXT NOT NULL,
    payload         JSONB NOT NULL,
    dedupe_key      TEXT,
    published_at    TIMESTAMPTZ,
    claimed_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_outbox_dedupe
    ON outbox(tenant_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_outbox_unpublished
    ON outbox(published_at, event_id)
    WHERE published_at IS NULL;

ALTER TABLE outbox FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_outbox ON outbox;
CREATE POLICY tenant_isolation_outbox
  ON outbox USING (
    current_setting('app.tenant_id', true) IS NULL
    OR tenant_id = current_setting('app.tenant_id', true)::uuid
  )
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE IF NOT EXISTS outbox_dlq (
    dlq_id        BIGSERIAL PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    event_id      BIGINT,
    event_type    TEXT NOT NULL,
    topic         TEXT,
    payload       JSONB NOT NULL,
    reason        TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_dlq_tenant
    ON outbox_dlq(tenant_id, created_at DESC);

ALTER TABLE outbox_dlq FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_outbox_dlq ON outbox_dlq;
CREATE POLICY tenant_isolation_outbox_dlq
  ON outbox_dlq USING (
    current_setting('app.tenant_id', true) IS NULL
    OR tenant_id = current_setting('app.tenant_id', true)::uuid
  )
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);
