---
derives-from:
  - margince-poc/docs/overview.md @ a11d6c08 (map format)
  - factory/docs-plan/skeleton-docs-structure.md §2–§3 (ratified chapter roster)
---
<!-- This map describes the TARGET tree. Rows for chapters not yet drafted are roster
     entries (skeleton-docs-structure.md §3): their one-liners summarize intended scope
     and must be kept true — sharpened, never contradicted — as each chapter lands.
     Entry doc: no `## Appendix`. -->
# Overview — the full chapter map

> Every chapter in the spec, grouped, with one line each: enough to find the chapter that
> owns your question. New here? [README.md](README.md) has the reading order; the table at
> the bottom answers "where do I look for X".

How to use this map: find the group your area belongs to, then open the owning chapter —
each chapter is self-contained, and every normative fact has exactly one owning chapter.
Every subsystem chapter carries a status ([README.md](README.md) has the full legend):

- **skeleton** — describes code that exists in this repository today; the consistency
  gate holds its claims to the tree.
- **planned** — specifies code the factory will build; ticket coverage holds it honest.

Both statuses are equally normative.

---

## Platform chapters

The platform the skeleton ships: seams, governance, and the machinery every feature
builds on. All **status: skeleton**.

| Chapter | What it covers |
|---|---|
| [auth-and-sessions](subsystems/auth-and-sessions.md) | Sign-in and sessions, RBAC, and the agent-passport seam that bounds what an agent may do |
| [datasource](subsystems/datasource.md) | The datasource seam and its native Postgres binding (the overlay binding belongs to the overlay feature) |
| [audit-observability](subsystems/audit-observability.md) | The append-only audit wall every mutation crosses, plus the logging/metrics/tracing infrastructure |
| [trust-propagation](subsystems/trust-propagation.md) | How trust tiers travel with data through the system (the tier definitions themselves live in the threat model) |
| [approvals-and-concurrency](subsystems/approvals-and-concurrency.md) | The approval-token seam for held actions, and optimistic concurrency via If-Match |
| [gdpr-platform](subsystems/gdpr-platform.md) | The consent and erasure machinery in the platform schema, including suppressed-row semantics |
| [intent-tools](subsystems/intent-tools.md) | The mechanism agent intent tools execute through (the governed tool set itself pins in byo-agent-and-mcp) |
| [overlay-budget](subsystems/overlay-budget.md) | The server-side meter for incumbent-API budgets in overlay mode; never fabricates headroom (the AI-spend guardrail is separate, owned by ai-runtime) |
| [retrieval-seam](subsystems/retrieval-seam.md) | The seam every retrieval caller goes through; the search engine behind it is a planned chapter |

## Feature chapters

The product the factory builds on that platform. All **status: planned**; the grouping
is navigational only — every chapter lives flat in `subsystems/`.

### Core CRM

| Chapter | What it covers |
|---|---|
| [people-and-organizations](subsystems/people-and-organizations.md) | Person and organization records — the core objects the rest of the product hangs on |
| [deals-and-pipeline](subsystems/deals-and-pipeline.md) | Deals, pipeline stages, and how deals move through them |
| [activities-and-timeline](subsystems/activities-and-timeline.md) | Activities on records and the timeline they roll up into |
| [leads-and-qualification](subsystems/leads-and-qualification.md) | Leads and the path by which they qualify into the core objects |
| [offers-and-products](subsystems/offers-and-products.md) | Offers, products, and the pricing objects behind them |
| [tasks-and-work-queue](subsystems/tasks-and-work-queue.md) | Tasks and the work queue users operate their day from |

### Capture & communication

| Chapter | What it covers |
|---|---|
| [capture](subsystems/capture.md) | Inbound capture of communication and data into records, idempotently, through the single audited writer |
| [meetings-and-transcripts](subsystems/meetings-and-transcripts.md) | Meetings and their transcripts as first-class captured records |
| [drafting](subsystems/drafting.md) | AI-assisted drafting of outbound communication |
| [voice-profile](subsystems/voice-profile.md) | The per-user writing-voice profile that drafting writes in |
| [sequences-and-deliverability](subsystems/sequences-and-deliverability.md) | Outbound sequences and the deliverability guardrails around them |
| [messaging-channels](subsystems/messaging-channels.md) | The messaging channels the product sends and receives through |

### AI-native

| Chapter | What it covers |
|---|---|
| [morning-brief](subsystems/morning-brief.md) | The daily brief that opens a user's day |
| [overnight-agent](subsystems/overnight-agent.md) | The overnight agent's background runs, including reconciliation work |
| [signals-and-warm-room](subsystems/signals-and-warm-room.md) | Buying signals and the warm-room surface built on them |
| [deal-rooms](subsystems/deal-rooms.md) | Collaborative rooms centered on a deal |
| [onboarding-and-coldstart](subsystems/onboarding-and-coldstart.md) | First-run onboarding and cold-starting a new workspace with data |
| [search-and-retrieval](subsystems/search-and-retrieval.md) | The search and retrieval engine behind the platform's retrieval seam |
| [context-graph](subsystems/context-graph.md) | The context graph that relates records for AI consumption |
| [ai-runtime](subsystems/ai-runtime.md) | The model runtime: providers, routing, and cost and safety controls |
| [agent-runner](subsystems/agent-runner.md) | How agent runs execute, are bounded, and are supervised |
| [byo-agent-and-mcp](subsystems/byo-agent-and-mcp.md) | Bring-your-own-agent access over MCP; owns the governed tool set |

### Reporting

| Chapter | What it covers |
|---|---|
| [reporting](subsystems/reporting.md) | Reports and dashboards over CRM data |
| [forecasting](subsystems/forecasting.md) | Pipeline and revenue forecasting |
| [lead-scoring](subsystems/lead-scoring.md) | The lead-scoring model and its lifecycle |

### Operational depth

| Chapter | What it covers |
|---|---|
| [automation](subsystems/automation.md) | User-defined automation: triggers and the actions they fire |
| [custom-fields](subsystems/custom-fields.md) | Customer-defined fields on core objects |
| [lists-views-segmentation](subsystems/lists-views-segmentation.md) | Lists, saved views, and segmentation |
| [data-hygiene](subsystems/data-hygiene.md) | Deduplication and ongoing data-quality upkeep |
| [import-export-migration](subsystems/import-export-migration.md) | Bulk import, export, and migration from other CRMs |
| [notifications-and-approval-inbox](subsystems/notifications-and-approval-inbox.md) | Notifications, and the inbox where held agent actions are approved or rejected |
| [records-depth](subsystems/records-depth.md) | Record hierarchy, attachments, quotas, and field history |

### Platform features

| Chapter | What it covers |
|---|---|
| [access-and-admin](subsystems/access-and-admin.md) | RBAC administration, sharing grants, SSO/MFA/SCIM, field security, and the sandbox |
| [gdpr-compliance-surfaces](subsystems/gdpr-compliance-surfaces.md) | The user-facing GDPR surfaces — SAR, erasure, consent UI, the DPA pack — on the gdpr platform |
| [client-surfaces](subsystems/client-surfaces.md) | The browser extension, sidebar, clipper, and in-assistant surfaces |
| [dispact-integration](subsystems/dispact-integration.md) | The dedicated Dispact integration |
| [germany-package](subsystems/germany-package.md) | The Germany package: country-specific compliance built on the jurisdiction seam |
| [overlay-augmentation](subsystems/overlay-augmentation.md) | Augmenting records with overlay data on top of the datasource seam |
| [mobile-and-pwa](subsystems/mobile-and-pwa.md) | The mobile / PWA surface |

---

## The rest of the tree

### `product/`

- [product.md](product/product.md) — what Margince is, for whom, and the bets it makes.
- [personas.md](product/personas.md) — the five personas, including the never-covertly-profiled guard.
- [journeys.md](product/journeys.md) — the J1–J4 end-to-end journeys and their failure rule.
- [scope.md](product/scope.md) — the scope of record: what ships in V1 (and the V0.5 launch gate), and the explicit OUT list.
- [voice-and-copy.md](product/voice-and-copy.md) — UI-copy tone rules and the banned-phrases list.
- [principles.md](product/principles.md) — the P1–P14 rubric every consequential decision cites, and its tie-breaker.

### `architecture/`

- [architecture.md](architecture/architecture.md) — the modular monolith: tiers, import rules, and the mechanically enforced seams.
- [code-organization.md](architecture/code-organization.md) — where code goes, and the end-to-end path to add a feature.
- [api-conventions.md](architecture/api-conventions.md) — the one wire contract humans and agents share: payload shapes, error semantics, concurrency and idempotency, approval gating, REST limits.
- [contract-pipeline.md](architecture/contract-pipeline.md) — how `backend/api/crm.yaml` generates the Go and TS types, and the drift gate that keeps them honest.
- [data-model.md](architecture/data-model.md) — the schema rules every table obeys (tenancy + RLS, version guards, provenance, archive-not-delete); owns the platform tables and the index assigning every other table to its owning chapter.
- [event-bus.md](architecture/event-bus.md) — the event envelope, delivery semantics, and the full event catalog.
- [frontend.md](architecture/frontend.md) — the frontend workspace: the four-layer component model, the query-cache data doctrine, and Storybook as the review surface.
- [web-design-system.md](architecture/web-design-system.md) — trust primitives, navigation order, fonts, and the design-system purity gates.
- [generators.md](architecture/generators.md) — the codegen commands and their canonical scaffolding recipes.
- [jurisdiction.md](architecture/jurisdiction.md) — the jurisdiction-pack seam and its five hooks; the core stays country-neutral.
- [runtime-config.md](architecture/runtime-config.md) — the P1 bright line: the exhaustive table of what is runtime-configurable (a surface not listed is a defect).
- [operations.md](architecture/operations.md) — deployment modes, config precedence, structured-log and metrics schemas, disaster recovery, release and patch policy.

### `quality/`

- [quality-gates.md](quality/quality-gates.md) — the gate registry: every check, what it verifies, and where it blocks.
- [craftsmanship.md](quality/craftsmanship.md) — the source-quality bar in two layers: deterministic gates plus the taste review.
- [testing.md](quality/testing.md) — the test pyramid and lanes; catch it at the cheapest layer that can prove it.
- [security.md](quality/security.md) — vulnerability disclosure, CRA conformity gates, SBOM and signing, patch SLAs.
- [acceptance-standards.md](quality/acceptance-standards.md) — the cross-cutting floor every chapter inherits: standard screen states, performance budgets, the release-gate catalogs, AC ID conventions.
- [ai-evals.md](quality/ai-evals.md) — the three-layer AI testing model, eval thresholds, and the conformance and certification tiers.
- [threat-model.md](quality/threat-model.md) — the agent-security threat model: trust tiers, attack chains, the defenses, and the accepted residual risk.

### `recipes/`

- [README.md](recipes/README.md) — the exemplar-first doctrine: copy the shape of the sample slice.
- [add-a-vertical-slice.md](recipes/add-a-vertical-slice.md) — the Person slice, layer by layer; the executable reference every other recipe points at.
- [add-a-field.md](recipes/add-a-field.md) — add a field to a core object via the generators.
- [add-an-endpoint.md](recipes/add-an-endpoint.md) — the contract-first path: contract edit → generate → handler → drift gate.
- [add-a-migration.md](recipes/add-a-migration.md) — migration conventions, the RLS and audit obligations, reversibility.
- [add-a-screen.md](recipes/add-a-screen.md) — a screen built on the design system, with the states matrix, Storybook, and tests.
- [add-an-event.md](recipes/add-an-event.md) — register in the catalog, emit through the outbox, consume idempotently.

### `adr/`

- [README.md](adr/README.md) — the canonical decision index, one line per decision; business decisions indexed but not vendored.
- `ADR-*.md` — the load-bearing engineering decisions, vendored in full so citations resolve inside the spec tree.

---

## Reference — where do I look for X?

| Looking for | Go to |
|---|---|
| A table's schema or the schema conventions | [data-model.md](architecture/data-model.md) — platform tables live there; feature tables in their owning chapter's appendix |
| Wire conventions — envelopes, error codes, limits | [api-conventions.md](architecture/api-conventions.md) |
| Performance budgets, screen states, release gates | [acceptance-standards.md](quality/acceptance-standards.md) |
| What ships — and what is explicitly OUT | [scope.md](product/scope.md) |
| What a term means (and its display form) | [glossary.md](glossary.md) |
| Why a decision was made | [adr/README.md](adr/README.md) |
| The AI quality bar and eval thresholds | [ai-evals.md](quality/ai-evals.md) |
| Agent security and trust tiers | [threat-model.md](quality/threat-model.md) |
| UI copy rules | [voice-and-copy.md](product/voice-and-copy.md) |
| How to add a field / endpoint / migration / screen / event | [recipes/](recipes/README.md) |
