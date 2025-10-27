DROP INDEX IF EXISTS idx_outbox_unpublished;
DROP INDEX IF EXISTS idx_outbox_dedupe;
DROP TABLE IF EXISTS outbox;

DROP INDEX IF EXISTS idx_activities_user_cursor;
DROP INDEX IF EXISTS idx_activities_idem;
DROP TABLE IF EXISTS activities;

DROP TABLE IF EXISTS account_idempotency;

DROP INDEX IF EXISTS idx_accounts_tenant_emailhash;
DROP TABLE IF EXISTS accounts;
