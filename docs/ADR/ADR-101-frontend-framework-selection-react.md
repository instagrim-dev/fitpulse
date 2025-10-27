# ADR-101: Frontend Framework Selection: React

- **Status:** Accepted
- **Date:** 2025-10-25
- **Owners:** Architecture; Platform; Service Teams
- **Supersedes/Amends:** N/A

## Context
We need a modern web framework with strong ecosystem, TypeScript support, robust dev tooling, and easy state management for optimistic UI.

## Decision
Use **React** for the frontend with TypeScript. Adopt a simple state manager (React Query or minimal Context + SWR) to support optimistic updates and server state synchronization.

## Consequences
Fast developer onboarding; rich component ecosystem; straightforward optimistic UI patterns; portable to SPA or SSR in future iterations.

## Alternatives Considered
Angular (heavier initial footprint), Vue/Svelte (excellent but smaller enterprise adoption in some orgs), plain web components (more boilerplate).

## Security/Privacy Implications
Follow Content Security Policy, avoid dangerouslySetInnerHTML, and keep secrets out of client. Use HTTPS and same‑site cookies or tokens.

## Operational/Cost Implications
Well-known toolchain; easy CI; no vendor lock‑in; CDN-friendly builds.

## Rollout Notes
Create a React app scaffold with Dockerfile and compose service; document `make web` workflow.

## Revisit Criteria
If the team standardizes on a different enterprise UI stack or SSR becomes mandatory for SEO.

## Related Documents
- PRD: Multi‑Language Microservices Web Application (v1.0)
