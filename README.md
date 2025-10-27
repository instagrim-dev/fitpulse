# Fitness & Wellness Activity Tracker

A three-service platform for tenant-aware identity, reliable activity ingestion, and exercise discovery. The stack pairs FastAPI (Identity) and Go (Activity, Exercise Ontology) with Postgres, Kafka, and Dgraph, wrapped in a Three Musketeers workflow (Docker + docker-compose + Taskfile) for reproducible local and CI builds.

## Repository Guide

| Directory | Purpose |
| --- | --- |
| `docs/` | Product/HLD/LLD, API specs, rollout notes |
| `services/activity-service/` | Go service handling workout ingestion + transactional outbox |
| `services/identity-service/` | FastAPI identity service issuing tenant-scoped JWTs |
| `services/exercise-ontology-service/` | Go service reading/writing Dgraph ontology + search APIs |
| `libs/` | Shared event contracts and helper libraries (Go/Python) |
| `db/` | Database migrations and Dgraph schema |
| `frontend/` | React SPA (pnpm + Vite + Vitest) |
| `infrastructure/compose/` | docker-compose stack + supporting configs |
| `.github/workflows/` | CI/CD pipelines and distribution builds |

## Architecture Snapshot

- **Identity Service (FastAPI)** — issues signed JWTs with tenant scopes, manages account creation with Postgres row-level security (RLS) and idempotency.
- **Activity Service (Go)** — authoritative activity store; writes to Postgres and publishes Kafka events through a transactional outbox with DLQ handling and Prometheus metrics.
- **Exercise Ontology & Search (Go)** — synchronises exercise taxonomy in Dgraph, consumes activity events, exposes REST/GraphQL search.
- **Shared Infrastructure** — Postgres 16, Kafka + Schema Registry, Dgraph Alpha, Prometheus scraping endpoints, Swagger UI pipeline.

```mermaid
flowchart LR
  Web[React Frontend] -->|REST / HTTPS| Gateway[API Gateway]
  Gateway --> Identity[Identity Service<br/>(FastAPI)]
  Gateway --> Activity[Activity Service<br/>(Go)]
  Gateway --> Ontology[Exercise Ontology Service<br/>(Go)]

  Identity --> Postgres[(PostgreSQL<br/>RLS)]
  Activity --> Postgres
  Activity --> Outbox[(Transactional Outbox)]
  Outbox --> Kafka[(Kafka + Schema Registry)]
  Kafka --> Ontology
  Ontology --> Dgraph[(Dgraph Ontology)]
  Kafka --> Analytics[(Future Analytics / OLAP)]
```

Design decisions, contracts, and rollout plans live in `docs/Design/HLD.md` and `docs/Design/LLD.md`.

## Prerequisites

- Docker Engine + docker-compose plugin
- Task (installed automatically in CI; locally via `brew install go-task/tap/go-task` or manual download)
- `corepack enable pnpm` (Node.js ≥20 recommended)
- Optional: VS Code Dev Containers for a prebuilt toolchain (`.devcontainer/devcontainer.json`)

### Editor IntelliSense

- VS Code picks up workspace TypeScript, Go, and Python settings from `.vscode/settings.json` (including `go.work`, pyright paths, and the frontend TypeScript SDK).  
- TypeScript module resolution now supports the `@/` alias shared between Vite and Vitest; Vitest pulls the main Vite config to keep IntelliSense aligned.  
- Pyright is configured via `pyrightconfig.json` so identity-service and shared `libs/python` schemas resolve across services without extra editor tweaks.

## Three Musketeers Quickstart

```bash
# Warm container cache (avoids slow CI pulls, also run inside Dev Container post-create)
task prepull:images

# Launch full stack & run smoke probes
task smoke:compose

# Run service test matrices
task test:go
task test:python     # installs .[dev] in container before pytest
task test:integration # Go integration (Testcontainers)
task docs:lint        # Spectral lint for OpenAPI definitions

# Frontend workflows
task web:install
task web:test
pnpm run lint      # ESLint with TSDoc validation
pnpm run docs      # Generate HTML docs (output in frontend/web/typedoc)
```

To iterate on the frontend:
```bash
cd frontend/web
pnpm dev
```
The dev server proxies to docker-compose APIs (override via `.env`).

## Service Development Loops

### Identity (FastAPI)
- Config via environment variables (see `services/identity-service/app/config.py`).
- Migrations: `db/postgres/migrations/*.sql` (managed with `migrate/migrate` container in compose).
- Tests: `task test:python` (3 Musketeers), or `python -m pytest` after `pip install .[dev]` locally.
- Distributed rate limiting: defaults to in-memory; set `RATE_LIMIT_BACKEND=redis` with `REDIS_URL` to enable shared sliding windows (compose wiring includes a Redis container).

### Activity (Go)
- Uses `pgxpool` with tenant context set via `set_config('app.tenant_id', ...)`.
- Outbox dispatcher metrics exported at `/metrics` (Prometheus).
- Integration tests (`go test -tags integration ./...`) spin up Postgres with Testcontainers; requires Docker.
- DLQ manager runs as a separate worker (`activity-dlq-manager`) with exponential backoff and quarantine (see compose service and Prometheus gauge `activity_service_persistence_last_activity_persisted_timestamp_seconds`).

### Exercise Ontology (Go)
- Interacts with Dgraph; schema auto-applied from `db/dgraph/schema/exercise.schema` by compose helper.
- TODOs and roadmap tracked in `docs/Design/LLD.md` backlog.
- Emits parity watermarks via Prometheus (`exercise_ontology_service_knowledge_last_ontology_upsert_timestamp_seconds` and `_last_ontology_read_timestamp_seconds`) and calls an optional edge cache invalidation webhook when exercises mutate (`CACHE_INVALIDATION_URL`).

For a complete walkthrough covering Three Musketeers workflows, migrations, Schema Registry validation, Redis limiter configuration, DLQ manager tuning, and observability tips, see `docs/DeveloperGuide.md`.

## Pre-commit Hooks

We recommend installing [pre-commit](https://pre-commit.com/) and enabling the hooks defined in `.pre-commit-config.yaml`:

```bash
pip install pre-commit
pre-commit install
```

Hooks run formatters/lints (gofmt/goimports, golangci-lint, black, ruff, prettier, Spectral) to keep the repo consistent.

## Observability & Diagnostics

- Prometheus endpoints mounted at `/metrics` for Go services; FastAPI uses `prometheus-fastapi-instrumentator` (configure via settings).
- Activity outbox metrics (`batch_duration_seconds`, `events_delivered_total`, `events_failed_total`, `events_dlq_total{topic="..."}`) surface replay health.
- Compose deploys Grafana/Prometheus stubs (TODO) — see `infrastructure/compose/docker-compose.yml` for ports.

## Testing Matrix

| Task | What's Covered |
| --- | --- |
| `task test:go` | Go unit tests for activity & ontology services |
| `task test:python` | FastAPI JWT issuance, scopes, rate limiting (pytest) |
| `task test:integration` | Go integration suite (Postgres via Testcontainers) covering outbox dispatcher success, DLQ failures, schema caching, and unknown event handling |
| `task web:test` | Vitest frontend unit tests |
| `task docs:lint` | Lints OpenAPI specs with Spectral |
| `task smoke:compose` | Starts a minimal stack (Postgres, Kafka, identity/activity services) and verifies Schema Registry subjects |

## Continuous Integration & Distribution

- **`ci.yml`** — runs the full Three Musketeers flow on pushes/PRs: builds, lint/tests, integration suite, frontend build/tests, compose smoke test.
- **`swagger-ui.yml`** — publishes static Swagger UI to GitHub Pages from `docs/API` artifacts.
- **`build-dist.yml`** — idiomatic release workflow building distributable artifacts on tags: Go binaries (activity/ontology), Python wheel/sdist (`identity-service`), and frontend production bundle zipped for deployment.

Artifacts are uploaded to the workflow run for downstream packaging or release automation.

## Releases

1. Tag the repository (e.g., `git tag v0.3.0 && git push origin v0.3.0`).
2. `build-dist` workflow compiles services, builds the identity wheel, runs `pnpm build`, and publishes artifacts under the GitHub Actions run.
3. Optional: Promote artifacts to package registries or container builds via follow-up workflows.

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| `docker: command not found` in Task runs | Install Docker Engine / Desktop and ensure `docker` is on PATH |
| `pytest` missing modules in CI | Confirm `task test:python` ran; it installs `. [dev]` dependencies in container |
| Go integration tests hang | Ensure Docker daemon is running; tests require Testcontainers |
| Frontend `pnpm` command fails | Run `corepack enable pnpm` or install pnpm globally |
| Swagger UI empty | Run `task docs:serve` locally or check `swagger-ui` workflow logs for schema errors |

## Further Reading

- `docs/Design/HLD.md` & `docs/Design/LLD.md`: Architecture decisions, contracts, backlog.
- `AGENTS.md`: Collaboration playbook for multi-agent development.
- `ops/` & `scripts/ci/`: Deployment tooling, smoke tests, and helper scripts.
