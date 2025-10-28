CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_account ON refresh_tokens (account_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON refresh_tokens (expires_at);

CREATE TABLE IF NOT EXISTS identity_audit_log (
    audit_id BIGSERIAL PRIMARY KEY,
    account_id UUID,
    tenant_id UUID,
    event_type TEXT NOT NULL,
    actor TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_identity_audit_account ON identity_audit_log (account_id);
CREATE INDEX IF NOT EXISTS idx_identity_audit_tenant ON identity_audit_log (tenant_id);
CREATE INDEX IF NOT EXISTS idx_identity_audit_event ON identity_audit_log (event_type);
