---
derives-from: specs/spec/decisions/ (ADR-0001…ADR-0052 + DECISIONS.md); margince-poc docs/decisions/README.md (index format) @ 5a0b29c
---
# Decisions (ADRs)

The docs cite decisions by id, like "(ADR-0014)". This page is the canonical index of
those decisions — each row is what was decided, in one line. The full text of every
load-bearing engineering decision is vendored as a sibling file in this directory
(`ADR-XXXX-*.md`), so a chapter's `derives-from` citation of an ADR id always resolves
inside the spec tree. Business and go-to-market decisions (pricing, licensing, partner
program, positioning) are indexed here but **not** vendored — their full text stays in
the foundation corpus, because they bind the company, not the build. Two of them leave
code hooks in the product (seat enforcement, the seat-type ceiling); those hooks are
pinned in the access-and-admin chapter and annotated on their rows below.

The numbering is continuous today (0001–0052). If an id is ever retired, the gap stays
and is annotated; superseded and amended decisions keep their rows with a pointer to
the successor — annotated, never deleted. Statuses are normalized on vendoring: the
architecture-blueprint ADRs whose corpus front-matter still reads *Proposed* were
ratified wholesale by the corpus decision log and are marked accordingly.

This page is an index, not a chapter: the table below is the pinned record itself.

| ID | What it decided | Status | Vendored? |
|---|---|---|---|
| ADR-0001 | The default stack is one Go modular-monolith binary, River (Postgres) background jobs, and a React / Vite / Zustand / TanStack Query / Tailwind / Biome front end; any divergence needs its own ADR. | Accepted (ratified 2026-06-04) · auth model superseded by ADR-0043; HTTP layer superseded by ADR-0019 | yes |
| ADR-0002 | Per-client customization is done by editing the source code, not through a runtime config engine. | Accepted (ratified 2026-06-04) | yes |
| ADR-0003 | Three-layer AI: orchestrate the user's own agent through governed tools, ship a built-in baseline AI tier, both over a shared foundation (capture, retrieval, provider-agnostic model client). | Accepted (ratified 2026-06-04) · funding clause (bundled Layer-2 inference) superseded by ADR-0020 | yes |
| ADR-0004 | Reporting runs through one typed query-plan builder that compiles to parameterized SQL over the relational core — no second analytics engine, no free-form SQL; NL and agent reporting target the same query-plan. | Accepted (ratified 2026-06-04) | yes |
| ADR-0005 | Build our own agent loop; the customer brings their own API key or runs a local model. | Accepted (ratified 2026-06-04) | yes |
| ADR-0006 | Web-scrape / enrichment is a defined connector seam. | Accepted (2026-06-04) | yes |
| ADR-0007 | The context graph is V1 substrate, built on the relational database + pgvector — no separate graph store. | Accepted (2026-06-04) · amended by ADR-0021 (names the graph-store trigger) | yes |
| ADR-0008 | A first-class Lead object, with a defined lead→contact promotion. | Accepted (2026-06-04) | yes |
| ADR-0009 | Agent-first surface: intent tools, an MCP tool surface, a hosted connector, and a governed autonomous loop. | Accepted (2026-06-10) | yes |
| ADR-0010 | EU CRA conformity and a documented secure development process are V1 scope. | Accepted (2026-06-10) | yes |
| ADR-0011 | Consent is tracked per purpose, with proof — seeded purposes, current state per person and purpose with lawful basis and policy version; a V1 retention engine ships (retention ladder, legal hold, erasure). | Accepted (2026-06-10) | yes |
| ADR-0012 | Both a fully-local LLM path and a hosted-cloud path are first-class, tested, and shipped in V1; default local models are non-Chinese open weights; the zero-egress path is gated by a no-external-calls test. | Accepted (2026-06-10) | yes |
| ADR-0013 | One core CRM, one governed surface (the MCP/REST API), one network-auth model — no privileged back doors. | Accepted (2026-06-10) | yes |
| ADR-0014 | Module boundaries are enforced by the compiler + lint, not by convention. | Accepted (ratified 2026-06-04) | yes |
| ADR-0015 | Contract-first codegen: Go and TypeScript types, the agent tool list, and the self-registration manifests are generated from `crm.yaml` behind a merge-blocking drift gate; central registries generate one-file-per-entry. | Accepted (ratified 2026-06-04) | yes |
| ADR-0016 | Repository layout + doc conventions: a domain-legible root, a single golden-path bring-up command, in-repo docs travelling with the source, no generic `pkg/` layout. | Accepted (ratified 2026-06-04) | yes |
| ADR-0017 | Fork-upgrade survival: seam versioning and migration architecture so a customer fork can still take upstream updates. | Accepted (ratified 2026-06-04) | yes |
| ADR-0018 | Trust boundaries: how capabilities are governed so an AI agent's edits stay safe. | Accepted (ratified 2026-06-04) | yes |
| ADR-0019 | The HTTP layer is `chi` over stdlib `net/http` in the service-struct shape (`NewServer(deps) http.Handler`) — deliberately not Gin. | Accepted (ratified 2026-06-04) | yes |
| ADR-0020 | AI inference is customer-supplied (their key or self-hosted); the product ships none. | Accepted (2026-06-16) | yes |
| ADR-0021 | Relational + pgvector is the context-graph substrate; a dedicated graph store is deferred behind a measurable trigger. | Accepted (2026-06-17) | yes |
| ADR-0022 | Build our own capture semantics + retrieval engine; borrow only in-boundary transport; capture is memory-first. | Accepted (2026-06-17) | yes |
| ADR-0023 | Releases are graded by how far a fork stays inside the seams: in-seam/additive changes patch mechanically, shared-function edits get a supervised patch, and a security fix ships as an isolated diff that cherry-picks onto a modified install. | Accepted (2026-06-17) | yes |
| ADR-0024 | The outward GTM frame is DACH-primary (sovereign, European, controllable AI); adoption messaging is augmentation-first — an AI layer over the incumbent CRM, migrate later. | Accepted (2026-06-17) · canonical positioning line amended by ADR-0052; beachhead half refined by ADR-0033 | no — indexed only |
| ADR-0025 | EU compliance posture: the AI features are limited/minimal-risk under the AI Act (only Art. 50 transparency binds), GDPR profiling is the load-bearing regime, NIS2 is a supplier obligation — all bundled into one supplier-conformity pack. | Accepted (2026-06-17) | yes |
| ADR-0026 | Every tool has an autonomy tier — runs freely (read/draft) or asks first (send/change) — and a tier can only be tightened, never loosened, without review. | Accepted (2026-06-17) · amended by ADR-0036 (approval-token binding + re-validation) | yes |
| ADR-0027 | The product is never vendor-hosted; every running instance is partner-operated or customer-self-hosted; data residency follows the tier. | Accepted (2026-06-17) | yes |
| ADR-0028 | Flat per-seat pricing — €25/seat/month, free under 10 seats, same price self-host or partner-hosted; no feature tiers, no hosting tiers, no AI markup, no contact billing. | Accepted (2026-06-17) · seat structure superseded by ADR-0047; partner economics superseded by ADR-0030 | no — indexed only |
| ADR-0029 | The license is BUSL 1.1 — free production use up to the seat grant, 2-year Apache-2.0 Change Date; per-seat entitlement is enforced server-side via the patch tool. | Accepted (2026-06-18) · seat definition revised by ADR-0047 (the grant counts full seats only) | no — indexed only; its seat-enforcement code hook is pinned in the access-and-admin chapter |
| ADR-0030 | Partner program (Scenario C): tiered buy-low license margin (15/20/25%) gated on certified staff, volume, and retention — never on recruiting; no recruiter override. | Accepted (2026-06-19) | no — indexed only |
| ADR-0031 | Service contracts are four metal tiers (Bronze/Silver/Gold/Platinum) on one SLA engine, with the managed fork-upgrade guarantee as the centerpiece. | Accepted (2026-06-20) | no — indexed only |
| ADR-0032 | `organization.classification` is a V1 core field, and partner is a first-class object — a 1:1 extension table of `organization` plus typed `relationship` edges, built over the relational core. | Accepted (2026-06-22) | yes |
| ADR-0033 | Beachhead reframe: regulated B2B Mittelstand is the first external customer; agencies collapse into the Gradion dogfood instance as the internal proving ground. | Accepted (2026-06-23) | no — indexed only |
| ADR-0034 | Partner functional roles are Hosting / Business Consulting / Strategic; implementation and custom development stay Gradion's turf, not a recruited tier. | Accepted (2026-06-23) | no — indexed only |
| ADR-0035 | User-facing automation is a bounded catalog plus agent-authored standing automations — not an open workflow builder. | Accepted (2026-06-23) | yes |
| ADR-0036 | How approval tokens are bound, staged effects re-validated, and native optimistic concurrency works (the `version` / `If-Match` mechanism). | Accepted (2026-06-23) | yes |
| ADR-0037 | Offers (Angebote): a bounded line-item + AI-authored quote engine — deliberately not a full CPQ configurator. | Accepted (2026-06-23) | yes |
| ADR-0038 | The Germany Package (V0.5): DACH launch readiness — e-invoicing/DATEV, eIDAS e-sign, compliance evidence, per-fork SBOM/CRA rebuild. | Accepted (2026-06-24) | yes |
| ADR-0039 | Per-record sharing via one generic, audited `record_grant` (read or write, optional expiry) that widens both the application visibility query and the row-level-security policy; an agent-initiated grant needs approval and never exceeds the granting human's access. | Accepted (2026-06-24) | yes |
| ADR-0040 | The visual identity: the "Ledger Green" palette and the Margin-rule M logo. | Accepted (2026-06-24) | no — indexed only |
| ADR-0041 | Mid-build spec governance: every spec or backlog change is classified into a change-class and re-gated; the backlog validator and contract-drift check are the mechanical gates that must stay green. | Accepted (2026-06-24) | yes |
| ADR-0042 | Country-specific code lives in its own compile-time module (a jurisdiction pack) behind a seam, never in core. | Accepted (2026-06-24) | yes |
| ADR-0043 | Human auth & session model: an in-app auth module, email/password baseline, server-side sessions. | Accepted (2026-06-25) | yes |
| ADR-0044 | In overlay mode, per-user visibility is re-enforced from a batched snapshot materialized into the mirror, governed by a per-object freshness limit and failing closed when the snapshot is stale. | Accepted (2026-06-25) | yes |
| ADR-0045 | A code-craftsmanship gate: a Critic agent that hard-blocks low-quality AI code, with an in-source fix loop. | Accepted (2026-06-25) | yes |
| ADR-0046 | The AI-assisted contribution policy for the open-source project: human accountability, DCO sign-off, disclosure. | Accepted (2026-06-25) | yes |
| ADR-0047 | Two seat classes: unlimited free read seats (view + read-only AI, never counted, never billed) and paid full seats (free ≤10 full) — only seats that act are billed; a read seat's BYO agent inherits the read-only ceiling. | Accepted (2026-06-25) · revises ADR-0028 §1 and ADR-0029's seat definition | no — indexed only; its `seat_type` ceiling code hook is pinned in the access-and-admin chapter |
| ADR-0048 | Connector secrets are stored encrypted in the application database, sealed by a pluggable key provider (cloud KMS or self-contained on-prem key), never returned through any API, and rotatable. | Accepted (2026-06-26) | yes |
| ADR-0049 | The German jurisdiction pack vendors pinned, versioned copies of the external standards it must conform to. | Accepted (2026-06-26) | yes |
| ADR-0050 | AI quality is certified per provider against a model-independent outcome contract: the deterministic end-state and red lines are hard gates identical for every model; only the graded-quality band varies, published as a per-provider tier. | Proposed (2026-06-29) | yes |
| ADR-0051 | Blob/object storage goes through one pluggable `blobstore` seam whose default is a sovereign, self-hosted S3-API store bound to the deployment tier; a public-cloud endpoint is opt-in configuration, never the default. | Accepted (2026-06-29) | yes |
| ADR-0052 | Positioning elevates to "the sovereign AI business platform, starting with CRM"; straight GmbH is the first named DACH GTM + design partner; the roadmap is sovereignty-led. | Accepted (2026-07-02) · amends ADR-0024's canonical line | no — indexed only |
