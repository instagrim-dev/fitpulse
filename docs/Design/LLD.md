# Fitness & Wellness Activity Tracker — Detailed Design (LLD)

Date: 2025-10-25
Owners: Platform Eng, Data Eng, API Platform, Security, SRE
Scope: Deliver the initial three-service architecture (Identity & Account, Activity, Exercise Ontology & Search) with hardened contracts, consistency guarantees, and progressive delivery guardrails.

⸻

## 0) Executive summary

- Keep PostgreSQL as the single OLTP source of truth with strict row-level security. Identity service issues tenant-scoped tokens consumed by Activity and Ontology services.
- Activity service (Go) remains the authoritative write path for workouts; transactional outbox publishes `activity_events` and `activity_state_changed` topics.
- Exercise Ontology & Search service (Go) manages exercise taxonomy in Dgraph, subscribes to activity events for recommendation enrichment, and exposes search APIs.
- Progressive delivery gates (Argo Rollouts) and schema governance (REST SemVer + Schema Registry) prevent regressions across the three services.
- Encrypt all channels (OIDC over TLS, Kafka mTLS, OTLP over TLS) and shrink PII dwell time in logs, DLQ, and telemetry.

Targets:
- Activity create p95 ≤ 200 ms, success ≥ 99.9% per route.
- Identity token issuance p95 ≤ 150 ms; zero RLS violations/month.
- Outbox lag p95 < 300 s; consumer lag max < 60 s.
- Ontology search p95 ≤ 250 ms; Dgraph sync parity lag p95 ≤ 300 s.

⸻

## 1) Reference architecture (three services)

```mermaid
flowchart LR
  subgraph Edge
    CDN[CDN/WAF TLS 1.3]
    IdP[OIDC Provider]
  end
  Client --> CDN --> GW[API Gateway]
  GW -->|OIDC| IDN[identity-svc (FastAPI)]
  GW -->|mTLS| ACT[activity-svc (Go)]
  GW -->|mTLS| EXO[exercise-ontology-svc (Go)]

  IDN --> PG[(Postgres OLTP)]
  ACT --> PG
  ACT --> OB[(outbox table)]
  OB --> RELAY[Outbox Relay]
  RELAY --> KAFKA[(Kafka)]
  EXO --> DG[(Dgraph Ontology)]
  EXO -->|subscribe| KAFKA

  subgraph Control
    REG[Schema Registry]
    FLAGS[Feature Flags]
    OTel[OTel Collector TLS/mTLS]
  end

  GW --> OTel
  RELAY --> OTel
  EXO --> OTel
  KAFKA <--> REG
```

Notes
- Identity service manages tokens and tenant context; Activity validates tokens before writes.
- Ontology service exposes REST and optional GraphQL endpoints backed by Dgraph; consumes `ontology_updates` for taxonomy drift.
- Outbox relay may run within the Activity service pod/container for now; scale via horizontal workers when lag > threshold.

⸻

## 2) External API & compatibility

### 2.1 Identity & Account Service (FastAPI)
- `POST /v1/token` — exchanges credentials for JWT (scopes: `activities:write`, `ontology:read`). Returns 200 with `{access_token, expires_in, tenant_id}`.
- `POST /v1/accounts` — creates account (requires admin scope). Enforces idempotency via `Idempotency-Key` header + server TTL cache.
- `GET /v1/accounts/{account_id}` — scoped to tenant.
- Error matrix: RFC7807 with `validation_failed`, `unauthorized`, `rate_limited`.
- Rate limits enforced via Sliding Window counters stored in Redis (future); initial prototype uses in-memory token bucket.

### 2.2 Activity Service (Go)
- `POST /v1/activities` — idempotent create; returns `202 Accepted` with `status:"pending"` + location header.
- `GET /v1/activities/{id}` — surfaces processing state.
- `GET /v1/users/{id}/activities?cursor=` — cursor pagination (base64 of `(start_ts,id)`).
- Headers: `Authorization: Bearer <jwt>`, `Idempotency-Key`, `X-Tenant-ID` (must match token claim).
- Events: publishes `activity_events` (activity.created|updated|deleted) and `activity_state_changed` topics using Schema Registry.

### 2.3 Exercise Ontology & Search Service (Go)
- `GET /v1/exercises?query=...` — text search over Dgraph/auxiliary index.
- `GET /v1/exercises/{id}` — returns ontology node with relationships.
- `POST /v1/exercises` — admin endpoint to extend taxonomy (behind feature flag).
- Consumes `ontology_updates` for admin changes and `activity_events` for enrichment; optional GraphQL endpoint for internal tooling.

### 2.4 Compatibility policy
- REST: Semantic versioning anchored at `/v1`. Additive-only changes allowed in minor versions; breaking changes require `/v2` with 180-day Sunset.
- Events: Avro schemas validated in Schema Registry; producers guarantee backward + forward compatibility for N=2 versions. Event headers include `schema_id`, `event_version`.

⸻

## 3) Data model & DDL (expand/contract safe)

### 3.1 Identity tables
```sql
CREATE TABLE accounts (
  account_id     UUID PRIMARY KEY,
  tenant_id      UUID NOT NULL,
  email_hash     BYTEA NOT NULL,
  email_cipher   BYTEA NOT NULL,
  disabled       BOOLEAN NOT NULL DEFAULT FALSE,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX CONCURRENTLY idx_accounts_tenant_emailhash
  ON accounts(tenant_id, email_hash);

ALTER TABLE accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_accounts
  ON accounts USING (tenant_id = current_setting('app.tenant_id')::uuid)
  WITH CHECK (tenant_id = current_setting('app.tenant_id')::uuid);
```

### 3.2 Activity tables
```sql
CREATE TABLE activities (
  activity_id     UUID PRIMARY KEY,
  tenant_id       UUID NOT NULL,
  user_id         UUID NOT NULL,
  activity_type   TEXT NOT NULL,
  started_at      TIMESTAMPTZ NOT NULL,
  duration_min    INTEGER NOT NULL,
  source          TEXT NOT NULL,
  idempotency_key TEXT,
  version         TEXT NOT NULL DEFAULT 'v1',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX CONCURRENTLY idx_activities_idem
  ON activities(tenant_id, user_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

CREATE INDEX CONCURRENTLY idx_activities_user_cursor
  ON activities(user_id, started_at DESC, activity_id DESC);

ALTER TABLE activities FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_activities
  ON activities USING (tenant_id = current_setting('app.tenant_id')::uuid)
  WITH CHECK (tenant_id = current_setting('app.tenant_id')::uuid);
```

### 3.3 Outbox table
```sql
CREATE TABLE outbox (
  event_id       BIGSERIAL PRIMARY KEY,
  tenant_id      UUID NOT NULL,
  aggregate_type TEXT NOT NULL,
  aggregate_id   TEXT NOT NULL,
  event_type     TEXT NOT NULL,
  payload        JSONB NOT NULL,
  dedupe_key     TEXT,
  published_at   TIMESTAMPTZ,
  claimed_at     TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

CREATE UNIQUE INDEX CONCURRENTLY idx_outbox_dedupe
  ON outbox(tenant_id, dedupe_key)
  WHERE dedupe_key IS NOT NULL;

CREATE INDEX CONCURRENTLY idx_outbox_unpublished
  ON outbox(published_at, event_id)
  WHERE published_at IS NULL;

ALTER TABLE outbox ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_outbox
  ON outbox USING (tenant_id = current_setting('app.tenant_id')::uuid)
  WITH CHECK (tenant_id = current_setting('app.tenant_id')::uuid);
```

Monthly partitions created via scheduled task (e.g., `outbox_y2025m11`). TTL: 7 days hot storage, archive to object store.

### 3.4 Dgraph schema (ontology)
```
# Types
type Exercise {
  exercise_id
  name
  difficulty
  requires
  targets
  contraindicated_with
  complementary_to
}

type MuscleGroup {
  muscle_id
  name
}

type Equipment {
  equipment_id
  name
}

# Predicates
exercise_id: string @index(exact) .
name: string @index(term) .
difficulty: string @index(exact) .
requires: [uid] .
targets: [uid] .
contraindicated_with: [uid] .
complementary_to: [uid] .
```

⸻

## 4) Eventing & schema governance

### 4.1 Topics & schemas
- `activity_events` (key: `tenant_id:user_id`, value schema `ActivityCreated` / `ActivityUpdated` / `ActivityDeleted`).
- `activity_state_changed` (key: `activity_id`, value schema `ActivityStateChanged`).
- `ontology_updates` (key: `exercise_id`, value schema `ExerciseUpserted` / `ExerciseDeleted`).

All schemas stored in Schema Registry with compatibility `BACKWARD_TRANSITIVE`. Producers include headers:
- `schema_id`, `event_version`, `tenant_id`, `traceparent`.

### 4.2 Producer settings
- enable.idempotence=true, acks=all, linger.ms=5-15, batch.size=64-128KB, compression=zstd.
- max.in.flight=5, retries=MAX_INT.
- sasl.mechanism=PLAIN or mTLS depending on cluster.

### 4.3 Consumer idempotency
- Consumers persist `event_id`/`tenant_id` tuple in the same transaction as their side effects.
- Maintain compacted `processed_events` topic for replay auditing.

⸻

## 5) Service internals

### 5.1 Identity & Account service
- Framework: FastAPI + Pydantic; dependency injection via `FastAPI Depends` patterns.
- Token issuance: integrates with external OIDC provider for authentication; issues signed JWT using JWK private key (kid rotated quarterly).
- RLS enforcement: `SELECT set_config('app.tenant_id', :tenant_id, true)` after token verification.
- Rate limiting: per-account + per-tenant budgets; initial in-memory bucket -> Redis cluster for production.
- Observability: structured logging (JSON), Prometheus metrics via `prometheus-fastapi-instrumentator`.

### 5.2 Activity service
- Layered architecture under `internal/`: `api` (handlers), `domain` (aggregates), `persistence` (repository), `outbox` (writer), `events` (producer), `transport/http` (routing).
- Transactional flow: handler validates token claims, constructs `domain.Activity`, persists via repository (INSERT + outbox write in single transaction), returns 202.
- Outbox relay: goroutine (or separate worker binary) polling `outbox` using `SELECT ... FOR UPDATE SKIP LOCKED LIMIT N`.
- Tests: table-driven unit tests for domain logic, integration tests with ephemeral Postgres using `testcontainers-go`.

### 5.3 Exercise Ontology & Search service
- API router (chi or net/http) exposing search endpoints.
- Read path: query Dgraph using GraphQL+-; optional caching layer (Redis) for popular queries.
- Write path (admin): validates ontology mutations and publishes `ontology_updates`.
- Subscriptions: Kafka consumer group `exercise-ontology-consumer` processes `activity_events` to enrich recommendations (e.g., track popular exercises per tenant).
- Search index (future): optional integration with Meilisearch/Elastic; not required for v1.

⸻

## 6) Outbox relay algorithm (shared)

```
loop:
  rows = SELECT id, payload
           FROM outbox
          WHERE published_at IS NULL
          ORDER BY id
          FOR UPDATE SKIP LOCKED
          LIMIT batch_size;
  if rows empty:
    sleep(backoff)
    continue
  produce batch to Kafka (acks=all, zstd)
  UPDATE outbox SET published_at = now() WHERE id IN (rows)
```

Sharding: workers claim modulo on `event_id` or use range partition windows. Autoscale via KEDA on metrics `outbox_rows_unpublished` or `outbox_age_p95`.

⸻

## 7) Security & privacy
- Zero trust network: mutual TLS between services; NetworkPolicies restricting namespace egress.
- Secrets in Vault/Secrets Manager; mount via sidecar or CSI driver.
- JWT claims: `sub`, `tenant_id`, `scopes`, `iat`, `exp`, `jti`. Validate `jti` against Redis to support revocation.
- Data minimization: Activity payloads avoid raw PII; logs redact user email/PII; DLQ retention capped at 7 days.
- DSAR workflow: orchestrated deletes cascade through Postgres (logical delete + tombstone events), Dgraph, Kafka, caches.

⸻

## 8) Progressive delivery & rollback
- Argo Rollouts canary steps: 5% → 25% → 50% → 100% with 5-minute pauses.
- Analysis template checks: `http_5xx_rate`, `p95_latency`, `outbox_age_p95`, `consumer_lag_max`, `token_issue_latency_p95` (identity), `dgraph_mutation_errors` (ontology).
- Abort criteria: KPI breach for two consecutive intervals or Schema Registry compatibility failure.
- DB migrations follow expand/contract; apply via `golang-migrate` (Activity/Ontology) and Alembic (Identity).

⸻

## 9) Observability
- Tracing: OpenTelemetry SDKs (Go/Python) propagate `traceparent` via REST + Kafka headers.
- Metrics: per-service dashboards with RED metrics (rate, errors, duration) and domain KPIs (outbox lag, token issuance latency, search cache hit rate).
- Logging: JSON with correlation IDs; mask sensitive fields.
- Alerting: burn-rate policies for availability (2h>14.4, 24h>6), parity lag > 300s for >15 min, Kafka consumer lag > 60s.

⸻

## 10) Cost & capacity
- Kafka partitions: `activity_events` start at 24 (headroom ×2), `activity_state_changed` at 12, `ontology_updates` at 12.
- Postgres: PgBouncer for connection pooling, partitioned outbox to curb bloat.
- Dgraph: start with 3-node Alpha cluster; enable sharding if query p95 > 250 ms.
- Edge caching: API Gateway caches ontology list endpoints for 30 s when safe.

⸻

## 11) Rollout plan

Weeks (relative):
- **W1–W2:** Identity token issuance, Activity CRUD + outbox schema, basic Kafka pipeline.
- **W2–W3:** Dgraph schema + Ontology read APIs, Schema Registry integration, RLS enforcement.
- **W3–W4:** Outbox relay hardening, KEDA autoscaling, observability instrumentation, Argo Rollouts setup.
- **W4–W5:** DLQ policies, DSAR orchestration beta, parity dashboards, load test + resiliency drills.

RACI per stream mirrors HLD; Identity focuses on Security, Activity on Platform/API, Ontology on Data/Platform collaboration.

⸻

## 12) Prioritized backlog
1. ✅ Stand up Schema Registry checks in CI for REST + event contracts (implemented via `schemas/registry/*` and smoke validation).
2. ✅ Implement Redis-backed rate limiter in Identity service (`RATE_LIMIT_BACKEND=redis`, fakeredis tests).
3. ✅ Add parity watermark metrics and dashboards (Activity ↔ Dgraph) via Prometheus gauges on both services.
4. ✅ Implement DLQ manager with auto-retry + quarantine flows (`activity-dlq-manager` worker and retry columns).
5. ✅ Edge cache invalidation hooks for ontology responses post-update (optional HTTP invalidator in exercise service).

⸻

## 13) Review checkpoints
- Contracts: `/v1` REST endpoints documented, Schema Registry gating merges, Idempotency enforced.
- Security: RLS policies active, JWT validation + revocation hooks, TLS everywhere.
- Resilience: Outbox relay sharded, DLQ policy defined, KEDA auto-scale configured.
- Observability: Metrics + tracing pipeline validated, canary analysis passing.
- Cost: Kafka compression enabled, outbox TTL enforced, Dgraph sizing monitored.

⸻

**Outcome:** A tightly scoped, multi-language microservices platform delivering tenant-aware identity, reliable activity ingestion, and rich exercise discovery while maintaining data consistency, security, and operational excellence.
