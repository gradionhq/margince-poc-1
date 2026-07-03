# ADR-0001 — Adopt the Dispact stack by default

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md)

## Context

Margince is a new product in the Gradion family alongside Dispact. P9 ("Independent products, shared foundations") requires that defaults, design language, auth, infrastructure, and data substrate align with Dispact unless an ADR records a reason to diverge (the two are independent products, not one suite — corrected 2026-06-16; they share *foundations*). We need a technical foundation that serves P4 (blazing fast), P7 (own your data: SaaS, on-prem, source-delivered, local LLM), and P3 (agent-readable, test-guarded code). Dispact already runs a proven stack with shared contracts, design system, auth, and CI/CD. Building a divergent stack would fragment the shared foundation, duplicate tooling, and forfeit reuse.

## Decision

Adopt Dispact's stack as the default for CRM. Every divergence from it requires its own ADR.

- **Backend:** Go 1.26 + Gin, single modular-monolith binary.
- **Async/jobs:** River (Postgres-backed queue) for transactional job chaining (auto-capture, agent runs).
- **Frontend:** React 19 + Vite + Zustand + TanStack Query + Tailwind 4 + Biome, reusing `gw-ui` + `gw-design-system`.
- **API:** REST, spec-first OpenAPI (`crm.yaml`) in `@gradion/contracts`; types generated for the frontend.
- **DB:** PostgreSQL 16 (dedicated `crm` schema(s)), `pgvector` for the AI substrate.
- **Cache/bus:** Redis 7; Redis Streams as the cross-product event bus.
- **Files:** S3 / MinIO, presigned uploads.
- **Auth:** Google OAuth + JWT via shared `gw-auth`; SSO (OIDC/SAML) as a commercial add-on. *(Superseded by [ADR-0043](ADR-0043-human-auth-and-session.md): email/password baseline + in-app `crm-auth` opaque server-side sessions; Google sign-in dropped; SSO/MFA included, not an add-on; `gw-auth` is a shared library, not a shared identity service.)*
- **Monorepo:** pnpm + `go.work`, reusing `@gradion/contracts`, `@gradion/core`, `gw-ui`, `gw-design-system`.
- **Deploy:** D13 / Kubernetes / Jenkins, plus a single-binary + docker-compose path for on-prem/source-delivered.
- **Quality:** mandatory TDD, coverage gates, contract-drift and design-system-drift checks.

The prior `gradion-crm` prototype (Go/Chi + River + Postgres + provider-agnostic LLM `Client`) is a **reference, not a fork**. Dispact's AI (Aria/Brief/Pulse on Gemini) is a learn-from-and-beat reference, not a blueprint (P10).

## Consequences

- **Positive:** shared infrastructure, design, auth, and tooling across the suite (P9); reuse of `gw-ui`/`gw-design-system` gets us "beautiful by default" cheaply (P8); single-binary Go suits speed (P4) and on-prem (P7); static schema + real indexes (no metadata engine) is the performance and agent-readability foundation (P4/P11); contract-first + end-to-end types + TDD are load-bearing for safe agent edits (P3).
- **Negative / costs:** we inherit Dispact's stack constraints even where CRM might prefer otherwise; divergences now carry ADR overhead; tight suite coupling raises the data-residency vs interop question (single shared Postgres cluster or separate?) — flagged in `03-architecture.md §3.6` / `07-risks.md`.
- **Deferred (each its own ADR if taken):** GraphQL for complex reporting reads (default: stay REST); a graph store beyond pgvector for the context layer (default: pgvector + relational joins first); React Native mobile now vs later (default: defer, reuse Dispact's `gw-mobile` pattern).

## Amendment 1 (2026-06-11) — HTTP layer divergence → see ADR-0019

The "Backend: Go 1.26 + **Gin**" line is amended by **ADR-0019 (HTTP layer: chi + stdlib `net/http`,
Mat-Ryer shape)**, which proposes the CRM diverge from Dispact's Gin to `go-chi/chi` over stdlib
`net/http` for the HTTP router/handler idiom only (everything else in this ADR's stack is unchanged).
That ADR carries the full rationale, consequences, and the decision required from Lars. Status:
**Proposed, pending sign-off.**
