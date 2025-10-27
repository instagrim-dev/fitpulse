# ADR-105: Event Backbone Selection: Kafka

- **Status:** Accepted
- **Date:** 2025-10-25
- **Owners:** Architecture; Platform; Service Teams
- **Supersedes/Amends:** N/A

## Context
We need topic‑based messaging so services can publish/subscribe and demonstrate eventual consistency across stores.

## Decision
Use **Kafka** for inter‑service messaging. Define at least one topic; publish events from services; consume to update Postgres/Dgraph as needed.

## Consequences
Decoupled services; replayable logs; supports scale if expanded beyond demo.

## Alternatives Considered
RabbitMQ (queues/AMQP), NATS (lightweight). Both fine; Kafka emphasizes log semantics and replay for data consistency demos.

## Security/Privacy Implications
Enable SASL/TLS in real deployments; restrict ACLs by topic/consumer group; avoid PII in event payloads.

## Operational/Cost Implications
Local single‑broker image is heavy but manageable; provides realistic developer experience.

## Rollout Notes
Provision Kafka/ZooKeeper via compose; include minimal admin scripts; define topics at startup.

## Revisit Criteria
If startup complexity is too high for local dev, consider Redpanda or a lighter broker for demos.

## Related Documents
- PRD: Multi‑Language Microservices Web Application (v1.0)
