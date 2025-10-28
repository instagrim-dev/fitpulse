# Developer Guide

This guide captures the workflows, tooling, and configuration needed to work productively on the three-service platform (Identity, Activity, Exercise Ontology) and its supporting infrastructure.

## 1. Prerequisites

- **Docker Engine** and the docker-compose plugin.
- **Go 1.25+** (local builds use `golang` container images, but having the toolchain installed helps for IDE integration).
- **Python 3.11+** (optional locally; the Three Musketeers pattern runs tests via containers).
- **Node.js 24.x with pnpm 10.x** (`corepack use pnpm@10.19.0`).
- Optional: **Playwright browsers** (`pnpm exec playwright install`) for running frontend E2E tests.
- Optional: set `VITE_ENABLE_TELEMETRY=true` with `VITE_TELEMETRY_URL` when validating telemetry forwarding locally; otherwise events are logged to the console in dev builds.
- **Task** (`brew install go-task/tap/go-task` or download binaries).
- Optional: **Redis CLI** for debugging the distributed rate limiter.

## 2. Repository Layout Highlights

- `services/identity-service/`: FastAPI service issuing JWTs and managing accounts.
- `services/activity-service/`: Go service for ingesting activities with transactional outbox and DLQ manager.
- `services/exercise-ontology-service/`: Go service owning the exercise catalog and cache invalidation hooks.
- `schemas/registry/`: JSON Schemas registered with Schema Registry for Kafka topics.
- `db/postgres/migrations/`: SQL migrations (`0001` base schema, `0002` DLQ retry/quarantine enrichment).
- `scripts/ci/`: Smoke test, schema registry validation, integration helpers.

## 3. Three Musketeers Workflows

Common tasks (`task --list` for the full catalog):

| Command | Purpose |
| --- | --- |
| `task prepull:images` | Pull base images (Postgres, Kafka, Schema Registry, Redis, etc.). |
| `task test:go` | Go unit tests in containers (activity + ontology services). |
| `task test:python` | Identity service pytest suite (uses fakeredis for rate limiter). |
| `task test:integration` | Go integration tests (pgx/testcontainers). |
| `task docs:lint` | Spectral lint for OpenAPI specs. |
| `task smoke:compose` | Bring up minimal stack, validate health checks, Schema Registry, Redis, and tear down. |
| `task web:test` | Frontend unit tests via Vitest (headless). |
| `task web:test:e2e` | Playwright E2E covering dashboard replay flow (requires browsers). |

Most tasks rely on Docker; ensure the daemon is running before executing.

## 4. Running the Stack Locally

1. **Pre-pull images** (first run): `task prepull:images`.
2. **Start compose stack**: `task smoke:compose` performs a build, health checks, Schema Registry validation, and shutdown. For an interactive session, run:
   ```bash
   docker compose -f infrastructure/compose/docker-compose.yml up --build
   ```
   This brings up Postgres, Redis, Kafka, Schema Registry, the three services, and the `activity-dlq-manager` worker.
3. **Access Prometheus metrics**: each Go service exposes `/metrics` (Activity, Ontology). Identity service still relies on middleware instrumentation.
4. **Swagger UI**: `task docs:serve` mounts `docs/API` and serves combined specs at <http://localhost:8088>.

## 5. Identity Service Notes

- **Rate Limiting**: Controlled by `RATE_LIMIT_BACKEND` (`memory` or `redis`). In docker-compose the service uses Redis (`redis://redis:6379/0`). Locally you can override with `docker compose ... -e RATE_LIMIT_BACKEND=memory`.
- **Configuration**: See `services/identity-service/app/config.py` for full variable list (`JWT_SECRET`, `RATE_LIMIT_REQUESTS`, `RATE_LIMIT_WINDOW_SECONDS`, etc.).
- **Testing**: `task test:python` installs `. [dev]` (pytest, fakeredis) and runs `tests/test_api.py` plus the redis rate limiter tests.

## 6. Activity Service Notes

- **Transactional Outbox**: Writers set tenant context via `set_config('app.tenant_id', ...)`. Prometheus gauge `activity_service_persistence_last_activity_persisted_timestamp_seconds` tracks persistence watermark.
- **DLQ Manager**: `activity-dlq-manager` is built from the same Dockerfile and runs as a separate compose service with exponential backoff, retry metadata (`retry_count`, `next_retry_at`, `quarantined_at`), and quarantine support. Environment knobs: `DLQ_POLL_INTERVAL`, `DLQ_MAX_RETRIES`, `DLQ_BASE_DELAY`.
- **Schema Registry**: Activity emits to `activity_events` and `activity_state_changed`; schemas maintained under `schemas/registry/`.

## 7. Exercise Ontology Service Notes

- **Dgraph Repository**: Default `DGRAPH_URL=http://dgraph-alpha:8080`; fallback is in-memory (for tests/local with Dgraph disabled).
- **Cache Invalidation**: Optional HTTP invalidator triggered after upserts (`CACHE_INVALIDATION_URL`, `CACHE_INVALIDATION_TOKEN`). If unset, a no-op invalidator is used.
- **Parity Metrics**: Gauges `exercise_ontology_service_knowledge_last_ontology_upsert_timestamp_seconds` and `_last_ontology_read_timestamp_seconds` provide parity indicators vs. Activity service.

## 8. Schema Registry Workflow

1. Edit schemas in `schemas/registry/` when events evolve.
2. Run `task docs:lint` to ensure OpenAPI is valid.
3. Execute `bash scripts/ci/smoke.sh` to spin up infra and register schemas. The script sets global compatibility to `BACKWARD_TRANSITIVE` and publishes all JSON schemas.
4. In CI, `ci.yml` runs the same smoke script ensuring contract gate is enforced before merges.

## 9. Database Migrations

- `0001_init.up.sql` sets up base tables, RLS policies, and outbox schema.
- `0002_outbox_dlq_retry.up.sql` adds DLQ retry/quarantine columns and indexes. Integration tests apply the migrations sequentially.
- Apply locally using `migrate/migrate` container or `docker compose run postgres-migrate` (already defined in compose).

## 10. Observability Cheat Sheet

| Service | Metric | Description |
| --- | --- | --- |
| Activity | `activity_service_persistence_last_activity_persisted_timestamp_seconds` | Unix timestamp of latest persisted activity. |
| Ontology | `exercise_ontology_service_knowledge_last_ontology_upsert_timestamp_seconds` | Latest Dgraph upsert watermark. |
| Ontology | `exercise_ontology_service_knowledge_last_ontology_read_timestamp_seconds` | Latest read/search watermark. |
| Outbox | `activity_service_outbox_events_delivered_total`, `activity_service_outbox_events_failed_total`, `batch_duration_seconds`, `events_dlq_total{topic}` | Health of the outbox dispatcher. |

## 11. Coding Standards & Docstrings

- **Go**: Exported types and functions require godoc comments. Run `go fmt` (`gofmt -w`) before committing.
- **Python**: Modules should include top-level docstrings. New utilities (rate limiter, cache invalidator) follow Google-style docstrings.
- **Docs**: Update `docs/API/*.yaml` and run `task docs:lint` whenever endpoints change. Update `docs/DeveloperGuide.md` and `docs/Design/LLD.md` to reflect new workflows.

## 12. Troubleshooting

| Symptom | Resolution |
| --- | --- |
| Smoke script hangs | Check `docker compose ps` for `postgres-migrate` errors (likely migration path). Ensure new migrations exist and mount paths are correct. |
| Schema Registry rejects schemas | Confirm JSON Schema format and subject naming; run `scripts/ci/schema_registry_check.sh` with `SCHEMA_REGISTRY_URL` pointing to your instance. |
| Rate limiter returns false positives | If running locally without Redis, ensure `RATE_LIMIT_BACKEND=memory` or start a local Redis instance (`docker compose up redis`). |
| DLQ manager quarantines entries immediately | Inspect `outbox_dlq.quarantine_reason` and ensure Schema Registry subject names align with event metadata. |

---

For architecture-level context, see `docs/Design/HLD.md` and `docs/Design/LLD.md`. For collaboration norms and task runner commands, reference `AGENTS.md` and the repository root `README.md`.
