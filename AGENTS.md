# Project Agents Playbook

This repository is built for collaborative execution between multiple automation agents and humans. Below is a quick-start guide defining the responsibilities, hand-off boundaries, and conventions to keep the three-service architecture cohesive.

## Core Personas

### 1. Architect Agent
- **Mission:** Maintain alignment with PRD/HLD/LLD, ensure non-functional requirements stay on track.
- **Primary Files:** `docs/PRD/`, `docs/Design/HLD.md`, `docs/Design/LLD.md`.
- **Key Actions:**
  - Validate new changes against architecture decisions (languages, data stores, messaging).
  - Update diagrams/documentation when service boundaries or contracts evolve.
  - Approve introduction of new infrastructure components.

### 2. Backend Go Agent
- **Mission:** Implement and maintain `activity-service` and `exercise-ontology-service`.
- **Primary Files:** `services/activity-service/`, `services/exercise-ontology-service/`, `libs/go/`.
- **Key Actions:**
  - Own domain models, repositories, and HTTP handlers for Go services.
  - Keep Go modules tidy (`go mod tidy`) and run `go test ./...` once tests exist.
  - Coordinate with Identity/Python agent on contract shapes via shared libs.

### 3. Identity/Python Agent
- **Mission:** Implement identity/account workflows in FastAPI.
- **Primary Files:** `services/identity-service/`, `libs/python/`.
- **Key Actions:**
  - Manage token issuance, idempotent account creation, and schema alignment.
  - Maintain Pydantic models mirroring Go event structs.
  - Prepare Alembic migrations once Postgres wiring replaces in-memory storage.

### 4. DevOps/Platform Agent
- **Mission:** Own compose, Dockerfiles, CI/CD, and environment parity.
- **Primary Files:** `infrastructure/compose/`, service `Dockerfile`s, `scripts/`, `ops/`.
- **Key Actions:**
  - Keep `docker-compose.yml` aligned with service dependencies.
  - Implement health checks, seed scripts, and future K8s manifests.
  - Coordinate with Architect for new infrastructure.

## Collaboration Conventions

- **Shared DTOs:** All cross-service contracts flow through `libs/go/events` and `libs/python/schemas`. Any change requires mutual review between Go and Python owners + Architect.
- **Idempotency & State:** Activity creation must remain idempotent; state-change events (`ActivityStateChanged`) are the source of truth for optimistic UI. QA agents should validate replay scenarios.
- **Branch Hygiene:** Feature branches should encapsulate service-level work. Coordinate merge order when contracts change (e.g., update shared libs first, then consume).
- **Testing:**
  - Go services: `task test:go` (wraps Dockerized go test invocations).
  - Python service: `task lint:python` / `task test:python` (lint & pytest).
  - Integration: `task test:integration` runs Testcontainers-based Go tests (Docker required).
  - Compose smoke test: `task smoke:compose` ensures docker-compose stack boots.
- **Docs:** When APIs or infrastructure change, update PRD/HLD/LLD plus READMEs.
- **Container caching:** Use `task prepull:images` before smoke/integration runs to warm Docker caches, especially on clean runners or after opening the Dev Container (post-create runs it automatically).
- **Dev Container:** Opening `.devcontainer/devcontainer.json` in VS Code provides a preconfigured environment with Go 1.25, Python 3.11, Docker CLI, and Task pre-installed.
- **Dgraph dependency:** The Exercise Ontology service expects a Dgraph alpha node. Compose provides one; set `DGRAPH_URL` if running manually.
- **API Docs:** `task docs:serve` runs Swagger UI on http://localhost:8088 using specs in `docs/API/`.
- A GitHub Pages pipeline (`swagger-ui.yml`) publishes Swagger UI automatically; grab the environment URL from the workflow run.
- **Frontend:** Use `task web:install` followed by `task web:dev` for React development. The app expects valid JWT tokens issued by Identity service.
- **Frontend tests:** `task web:test` runs Vitest. Ensure you run `task web:install` first.
- **pnpm:** We standardize on pnpm for frontend dependencies. Run `corepack enable pnpm` on fresh environments before invoking frontend tasks.
- **Identity service:** PostgreSQL enforces row-level security; repository queries now call `SELECT set_config('app.tenant_id', :tenant, true)`. If you add new SQL, ensure the tenant is set before executing. Rate limiting is provided by an in-memory sliding window configured via `RATE_LIMIT_REQUESTS` / `RATE_LIMIT_WINDOW_SECONDS`.
- **Dgraph schema:** Compose's `dgraph-schema` service posts `db/dgraph/schema/exercise.schema` to the cluster after alpha becomes healthy. Update this file when ontology predicates change.

## Hand-off Checklist

Before handing work to another agent:
1. **Contracts Updated:** Ensure shared DTOs are merged and versioned.
2. **Builds Pass:** Run `task build:go` / `task lint:python` as relevant.
3. **Compose Ready:** Confirm services build via docker-compose with `task smoke:compose` when runtime changes.
4. **Docs Synced:** Reflect new scopes or dependencies in design docs.
5. **Open Questions:** Document unresolved decisions directly in relevant docs or commit messages.

## Escalation Paths
- Security/privacy concerns → Architect Agent + Identity Agent.
- Data consistency issues → Backend Go Agent + Architect.
- Deployment pipeline/infra blockers → DevOps/Platform Agent.

Aligning on these roles keeps the repo ready for fast, parallel iteration without stepping on each other. Update this playbook when new agents or workflows join the project.
