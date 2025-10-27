# ADR-102: Backend Language Stack: Go and Python

- **Status:** Accepted
- **Date:** 2025-10-25
- **Owners:** Architecture; Platform; Service Teams
- **Supersedes/Amends:** N/A

## Context
We require two services in different languages that integrate cleanly and run well in containers.

## Decision
Implement **Service A** in **Go** (exposes REST API, high concurrency, small memory) and **Service B** in **Python** (data processing, Kafka consumer/producer, rich libraries).

## Consequences
Demonstrates polyglot service integration; leverages Go's performance and Python's ecosystem; clear role separation.

## Alternatives Considered
Node.js + Go; Java + Kotlin; Python + Rust. Any pair could work; chosen pair optimizes for fast local dev and small images.

## Security/Privacy Implications
Use per‑service credentials, least privilege to databases, and validate inputs on both services.

## Operational/Cost Implications
Both have mature Docker images; easy CI; modest resource usage; good observability libraries.

## Rollout Notes
Define separate Dockerfiles; standardize health endpoints; publish OpenAPI for Service A for client generation.

## Revisit Criteria
If team skills or latency constraints shift significantly.

## Related Documents
- PRD: Multi‑Language Microservices Web Application (v1.0)
