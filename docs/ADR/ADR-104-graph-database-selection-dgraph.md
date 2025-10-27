# ADR-104: Graph Database Selection: Dgraph

- **Status:** Accepted
- **Date:** 2025-10-25
- **Owners:** Architecture; Platform; Service Teams
- **Supersedes/Amends:** N/A

## Context
We require a graph engine for relationship and traversal queries optimized for the chosen domain model.

## Decision
Use **Dgraph** to model relationships and run traversal queries. Keep the graph **off the OLTP write path** and update it from events to avoid tight coupling.

## Consequences
Expressive traversals; eventual consistency with OLTP; independent scaling for graph workloads.

## Alternatives Considered
Neo4j (popular, strong tooling), ArangoDB (multi‑model). Both viable; Dgraph chosen for native distributed design in a container‑friendly setup.

## Security/Privacy Implications
Restrict access to graph endpoints; enforce auth at service layer; use TLS in production; sanitize query inputs.

## Operational/Cost Implications
Single‑node local brings simplicity; cluster modes add complexity if scaled later.

## Rollout Notes
Run Dgraph via compose; define schema/types; provide a loader to project events into the graph.

## Revisit Criteria
If traversal needs or ops burden favor Neo4j or ArangoDB, or if write‑path coupling is required (not recommended).

## Related Documents
- PRD: Multi‑Language Microservices Web Application (v1.0)
