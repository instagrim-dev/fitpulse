# ADR-106: Development Workflow & Orchestration: Three Musketeers (Make + Docker + Compose)

- **Status:** Accepted
- **Date:** 2025-10-25
- **Owners:** Architecture; Platform; Service Teams
- **Supersedes/Amends:** N/A

## Context
We need a consistent local developer experience across languages/services and a single‑command bootstrap.

## Decision
Adopt the **Three Musketeers** pattern: Makefile targets wrap Docker and docker‑compose for build/run/test. The top‑level `make up` runs the full stack; `make web`, `make svc-a`, `make svc-b` run components independently.

## Consequences
Predictable, repeatable workflows; easy CI mirroring; reduced “works on my machine” failures.

## Alternatives Considered
Ad hoc scripts; language‑specific CLIs. These fragment workflows and increase onboarding time.

## Security/Privacy Implications
Ensure no secrets in Makefiles or Compose; use env files or a dev secrets manager.

## Operational/Cost Implications
Minimal; improves consistency and reduces support overhead.

## Rollout Notes
Ship standard Make targets and a top‑level README with common commands; pin image versions; add healthchecks.

## Revisit Criteria
If the team moves to devcontainers or Bazel‑style builds across the org.

## Related Documents
- PRD: Multi‑Language Microservices Web Application (v1.0)
