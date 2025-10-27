# Product Requirements Document (PRD)
**Title:** Multi‑Language Microservices Web Application (Exercise PRD)  
**Version:** 1.0  
**Date:** 2025-10-25  
**Document Owner:** Platform & Application Engineering

---

## 1. Overview
This PRD formalizes the assignment brief into an enterprise‑grade specification. The goal is to design and implement a production‑grade, full‑stack system using **multi‑language microservices**, a **modern web frontend**, a **relational database** for transactional data, a **graph database** for relationship/analytics queries, and an **event backbone** for inter‑service messaging. Delivery emphasizes **clear architecture**, **well‑designed entity relationships optimized for graph queries**, and an operationally sound, containerized deployment.

> Source constraints were adapted from the assignment “Coding Assignment for Sr. Software Engineer – Multi‑Language Microservices Web Application.”

## 2. Objectives & Key Results (OKRs)
- **O1:** Ship a working system that starts with a **single `docker-compose up`** and demonstrates correct behavior end‑to‑end.
- **O2:** Demonstrate **cross‑technology integration** across two backend languages and multiple data stores.
- **O3:** Provide **reliable, consistent data** in both the relational and graph databases for published events.
- **O4:** Provide a **clear README** and **architecture documentation** suitable for onboarding and review.

**Key Results**
- KR1: Frontend reads/writes via REST APIs only; implements **optimistic UI updates** and signals pending states.
- KR2: One microservice exposes a REST API that **another microservice consumes**.
- KR3: **Event‑driven** communication publishes between at least two services via **Kafka**.
- KR4: Data published by services **persists in both** the relational and graph databases.
- KR5: All services build and run **independently** and **interoperate** in Docker.
- KR6: **Basic tests** and **error handling** exist across services.

## 3. Scope
### 3.1 Domain model (select one)
One of the following domains will be implemented (choice may be adjusted per stakeholder or demo needs):
1. Movie recommendation system  
2. E‑commerce product catalog and order tracking  
3. Ride‑sharing dispatch and matching  
4. Fitness and wellness activity tracker  
5. Event management and attendee networking

> Default baseline for examples: **Fitness and wellness activity tracker**.

### 3.2 In Scope
- Web frontend built with **React**.  
- At least **two backend microservices in different languages** (e.g., **Go** and **Python**).  
- **Relational DB** for OLTP transactional data (e.g., **PostgreSQL**).  
- **Graph DB** for relationship/analytics queries (e.g., **Dgraph**).  
- **Kafka** event topic(s) for inter‑service messaging.  
- **Docker & docker‑compose** orchestration for end‑to‑end bring‑up.  
- **Single‑command** bootstrap and **service‑level independence** for builds/runs.  
- **REST API** exposed by Service A and **consumed by Service B**.  
- **Optimistic UI** behavior with pending/processing indicators.  
- **Data consistency** checks ensuring both databases reflect published messages.  
- **Graceful failure handling** (queue retries, rollbacks, clear error responses).  
- **README.md** with architecture overview and local build/run/test instructions.

### 3.3 Out of Scope
- Advanced production controls (multi‑region HA, DR/BCP) beyond demonstration.
- Deep UI polish or design‑system fidelity (functional UI is sufficient).
- Enterprise identity integrations (SSO/OIDC) beyond simple auth, if any.

## 4. Users & Use Cases
- **End user:** interacts through the web UI to create/update domain entities and view results.  
- **System operator/developer:** builds, runs, tests locally; inspects logs, messages, and DB states.

**Representative scenarios**
- Create/update an entity (e.g., activity) → UI shows pending state → event published → data lands in both DBs → UI reflects final status.  
- Service B calls Service A’s REST endpoint to enrich or validate data before publish.  
- Intermittent broker or DB failure → retry/backoff with eventual consistency; user receives clear status feedback.

## 5. Functional Requirements
FR‑1: React frontend communicates **only** through REST APIs.  
FR‑2: At least two backend microservices written in **Go** and **Python**.  
FR‑3: **Service A** exposes REST API; **Service B** consumes it.  
FR‑4: Event‑driven publish between services using **Kafka** (topic‑based).  
FR‑5: **Published data persists** into **PostgreSQL** and **Dgraph**.  
FR‑6: System can be brought up with **one `docker-compose up`**.  
FR‑7: Frontend implements **optimistic updates** and visible “processing” states.  
FR‑8: Each service is buildable and runnable **independently**.  
FR‑9: **Basic tests** (unit/integration where relevant) and **error handling** exist.  
FR‑10: Both databases are **queryable independently** and show **consistent** results for published messages.

## 6. Non‑Functional Requirements (NFRs)
NFR‑1 **Reliability:** graceful handling of failures (queue retries, rollback or compensating behavior).  
NFR‑2 **Consistency:** published messages result in concordant rows/nodes in both DBs.  
NFR‑3 **Security basics:** containerized isolation; no secrets hard‑coded; minimal least‑privilege DB access.  
NFR‑4 **Performance:** local developer machine should perceive sub‑second UI interactions and timely event processing (< a few seconds).  
NFR‑5 **Operability:** structured logging; basic health endpoints; simple metrics or counters where feasible.  
NFR‑6 **Portability:** entire system builds and runs via Docker on common developer OS (Linux/macOS/Windows with WSL).  
NFR‑7 **Documentation:** top‑level README with architecture, local setup, and test instructions.

## 7. Architecture Constraints
- **Frontend:** React.  
- **Backend languages:** Go (Service A), Python (Service B).  
- **Relational DB:** PostgreSQL.  
- **Graph DB:** Dgraph.  
- **Messaging:** Kafka.  
- **Orchestration:** Docker + docker‑compose; single command bootstrap; “**Three Musketeers**” pattern encouraged (Makefile + Docker + Compose for consistent dev UX).

## 8. Success Metrics & Acceptance Criteria
**Acceptance Criteria**
- AC‑1: `docker-compose up` succeeds, all services healthy, frontend reachable.  
- AC‑2: Creating an entity in the UI shows an optimistic “pending” state, then resolves to “succeeded” with the final data.  
- AC‑3: Service B successfully invokes Service A’s REST endpoint as part of an end‑to‑end flow.  
- AC‑4: An event is published to Kafka and consumed by at least one other service.  
- AC‑5: The same logical record is visible and consistent in **PostgreSQL** and **Dgraph**.  
- AC‑6: Each service can be built and run locally on its own (outside compose) with documented commands.  
- AC‑7: README includes architecture diagram(s) and step‑by‑step build/run/test instructions.  
- AC‑8: Basic test suite exists and passes.

**Success Metrics (demo level)**
- 95th percentile end‑to‑end flow under 3 seconds on a developer laptop.  
- Event processing catch‑up for a burst of 100 messages in under ~10 seconds.  
- Zero critical errors in logs during happy‑path usage.

## 9. Risks & Mitigations
- **Dual‑write consistency risk** → Use idempotent writes and retries; consider outbox pattern inside a service if feasible for local demo.  
- **Local Kafka/Dgraph stability** → Pin versions, provide startup readiness checks, and include retry logic.  
- **Developer environment drift** → Use Compose with explicit image tags and a Makefile wrapper.

## 10. Deliverables
- Source code repositories for frontend and services.  
- `docker-compose.yml` orchestrating all services and infrastructure (PostgreSQL, Dgraph, Kafka/ZooKeeper).  
- Top‑level `README.md` with architecture overview and setup/test steps.  
- Basic test suites and example payloads.

## 11. Timeline & Milestones (suggested for exercise)
- **Day 1:** Skeleton repos, Compose topology, health endpoints.  
- **Day 2:** REST integration A→B; Kafka publish/consume; Postgres schema & Dgraph types.  
- **Day 3:** Optimistic UI, basic tests, consistency checks, README polish.

## 12. Compliance & Privacy (exercise scope)
- No PII required. If modeled, redact logs and avoid storing secrets in code.  
- Follow container image and dependency hygiene.

## 13. Glossary
- **Three Musketeers pattern:** unify build/run/test via Make + Docker + Compose for consistent local execution.
