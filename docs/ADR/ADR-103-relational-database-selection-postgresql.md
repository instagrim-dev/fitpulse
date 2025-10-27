# ADR-103: Relational Database Selection: PostgreSQL

- **Status:** Accepted
- **Date:** 2025-10-25
- **Owners:** Architecture; Platform; Service Teams
- **Supersedes/Amends:** N/A

## Context
We need an OLTP store for transactional data with strong SQL, indexing, and JSON support.

## Decision
Use **PostgreSQL** for the relational data model. Manage schema via migrations and enforce data integrity with constraints and indexes.

## Consequences
Reliable transactions, rich SQL features, easy local orchestration with Docker images.

## Alternatives Considered
MySQL (mature but different JSON/CTE behaviors), SQLite (simple but limited for concurrency).

## Security/Privacy Implications
Create distinct DB users per service; use SSL in production; avoid superuser roles; apply RLS if tenancy is needed.

## Operational/Cost Implications
Lightweight to run locally; standard admin tooling; broad community support.

## Rollout Notes
Provide init scripts/migrations in container; expose ports via compose; seed minimal demo data.

## Revisit Criteria
If cloud‑managed offerings or licensing constraints change.

## Related Documents
- PRD: Multi‑Language Microservices Web Application (v1.0)
