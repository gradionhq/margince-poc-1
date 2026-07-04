---
derives-from:
  - margince-poc/docs/architecture/event-bus.md
  - margince specs/spec/contract/events.md#0-why-an-event-catalog-the-load-bearing-decisions
  - margince specs/spec/contract/events.md#2-the-standard-envelope
  - margince specs/spec/contract/events.md#3-delivery-semantics
  - margince specs/spec/contract/events.md#4-redis-streams-layout
  - margince specs/spec/contract/events.md#5-the-catalog
---
# Event bus — one committed fact, one reliable event

> The "do it later, reliably" half of the system: when a record changes, the work that
> follows — downstream reactions, caches, agents, audit — happens **off** the
> synchronous request, yet is never lost and never fired for a change that didn't
> actually commit. Every domain event on the bus is listed in this chapter's catalog;
> no other chapter defines one.

## What it's for

Keep the HTTP request path fast and honest. A capture or mutation should return as
soon as the intent is safely recorded; the heavy or downstream work — the real domain
write, fan-out to anyone listening — runs afterward on a worker, not while the caller
waits. The hard part is doing this *without* the two classic failure modes: publishing
an event for a change that later rolled back, or losing an event because the process
crashed between the write and the publish. The design makes side-effects asynchronous
while keeping them exactly as trustworthy as the database transaction that triggered
them.

The bus is also the product's nervous system: the context graph, the overnight agent,
the workflow registry, the read-model caches, the security anomaly detector, and the
Dispact bridge all react to domain events rather than polling tables. That is why the
catalog below is central — consumers genuinely span modules, so event definitions live
once, here, and every other chapter cites them.

## Principles it serves

- **P4 — blazing fast.** Side-effects leave the request path entirely; the event is
  emitted application-side after commit, never as a database trigger inside the write
  path, so the synchronous response is not held hostage to downstream work.
- **P5 — auto-capture over manual entry.** The async capture path is what lets
  ingestion accept a record and reliably write it later; the captured-activity event is
  the spine that lets everything downstream react to what capture brought in.
- **P12 — governance designed in.** Every domain mutation writes exactly one audit row
  and emits exactly one domain event, transactionally staged and trace-linked, so the
  trail is reliable rather than best-effort.

## How it works

- **Capture returns immediately.** The async capture entry validates an incoming
  record and enqueues a job, returning to the caller at once; the real domain write
  runs later on a worker, off the request path.
- **River is the worker.** A durable Postgres-backed job queue owns the "run it later"
  jobs and dequeues them with a pool of workers. River *runs* jobs; the bus
  *broadcasts* facts — two different tools, deliberately not conflated.
- **The outbox bridges database to bus.** A producer writes its event into an outbox
  row *in the same transaction* as the mutation it describes. If the transaction rolls
  back, the event vanishes with it; if it commits, the event is durably staged. The
  outbox table shape is owned by the data-model chapter ([[data-model#DM-DDL-9]]).
- **A relay publishes.** A background relay job polls unpublished outbox rows in
  creation order, publishes each to the bus, then marks it published — crash-safe, so
  a relay that dies mid-batch re-publishes and consumers dedupe (EVT-DEL-5).
- **Redis Streams is the bus.** Published events land on one stream per entity type
  on the shared cross-product bus that Dispact also rides — nine streams in V1
  (EVT-STREAM-1..9). Workspace is a field in the envelope, not a stream: consumers
  filter tenant in-process, the bus analogue of row-level security.
- **Consumer groups fan out.** Each consuming module owns one consumer group, so each
  module sees every event once per group and scales horizontally inside the group —
  seven groups in V1 (EVT-CG-1..7): context graph, overnight agent, workflow registry,
  capture pipeline, Dispact bridge, read-model caches, and the security audit stream.
- **One envelope for everything.** Every event carries the standard envelope
  (EVT-ENV-1): a time-ordered unique event id (the dedupe key), the type and its
  payload schema version, the workspace, the structured actor (who did this, under
  whose authority), the entity reference, the per-type payload, and a trace block
  linking correlation, causation, and the audit row written in the same operation.

## The semantic rules

The catalog is not just a list; it encodes promises consumers build against. Each rule
here is pinned in the appendix (EVT-SEM-1..14).

**One mutation, one audit row, one event.** The audit row is the durable
system-of-record of what happened; the event is the notification that lets other
modules react. They are written in the same logical operation, but the publish happens
only after commit, via the outbox.

**Specific verbs replace the generic update — never double-fire.** A stage transition
emits a stage-changed event *instead of* a generic updated event, never in addition;
the same holds for owner changes, merges, promotions, and captures. Consumers
subscribe by meaning, and the catalog stays statically enumerable for contract and
codegen. Exactly one co-fire is sanctioned: a single write that changes both stage and
amount emits both the stage-changed and the updated event, sharing one correlation id
— distinct concerns for distinct consumers, never a folded-in substitute. The
offer-acceptance flow uses the same paired-emission discipline: accepting an offer
syncs the deal's amount server-side and emits a paired deal-updated event under the
same correlation id, carrying the server-computed total, never a client value.

**Lead events never enter the contact graph.** A created lead is a segregated pool
entry for routing, scoring, and list freshness only; it is excluded from person
dedupe, relationship strength, and "people we know" until a promotion event
materializes a real person, carrying the conversion lineage and the triggering
evidence. A cold outbound touch with no reply never fires a promotion.

**Events carry references, not bodies.** An event names the entity and a small typed
payload of changed or relevant fields, never the full row. A consumer that needs the
whole record reads it back through the datasource port under its own scopes — the bus
is not a field-mask bypass, and role-based masking stays honest. Minimal denormalized
routing keys are allowed; sensitive or maskable values are not.

**Ordering is per-entity only.** All events for a given entity land on one stream in
commit order; across entities, order is best-effort. Redelivery can reorder within an
entity, so order-sensitive consumers apply last-writer-wins by the entity's version —
an update older than what is already applied is ignored, closing the
redelivery-reorder hazard.

## What's configurable

- **The workflow seam.** A dependency-free event model — the envelope carrier plus a
  handler registry — lets deterministic automations register against topics without
  coupling to the queue, the broker, or the database. Judgment-call reactions run on a
  separate reaction runner, the autonomous agent path, consuming the same events.
- **Topics.** Event types follow the entity-dot-verb convention with past-tense verbs
  (a fact that already happened); the relay derives each stream from the type's entity
  segment.
- **Sinks.** The async capture sink is wired with the real domain writer injected, so
  what happens when a job runs is a seam, not hard-coded.
- **The broker is optional.** If no Redis endpoint is configured, the relay disables
  itself and the server boots in degraded mode — bus off, outbox intact — which also
  keeps the standard check suite infra-free.
- **Stream hygiene.** Retention caps, trim policy, and the sizing of the redelivery
  and offline-consumer window are operational tunables owned by the operations chapter
  ([[operations#OPS-QUEUE]]); this chapter owns only the invariants that reference
  them (EVT-DEL-4, EVT-DEL-6).

## Guarantees (enforced)

- **Side-effects never block the HTTP path** — capture and downstream work ride the
  job queue, dequeued by workers.
- **The outbox is transactional** — an event becomes publishable iff its mutation
  committed: no ghost events, no lost events, even when the broker is briefly down
  (EVT-DEL-5).
- **At-least-once, idempotent by contract** — a consumer may see an event more than
  once; every consumer dedupes on the event id and makes effects idempotent
  (EVT-DEL-1, EVT-DEL-2). Exactly-once across a database and an external bus is a
  distributed-transaction trap deliberately not attempted.
- **Per-entity ordering, version-guarded** — order holds within an entity's stream;
  redelivery reorder is closed by last-writer-wins on the entity version (EVT-DEL-3,
  EVT-DEL-7).
- **Schema versions migrate without gaps** — additive payload changes never bump a
  type's version; breaking ones do, with a dual-publish window and dedupe-set lifetime
  pinned to the retention horizon so an offline consumer cannot silently miss or
  double-apply (EVT-DEL-6). A consumer offline longer than retention is a fail-loud
  alarm that re-bootstraps from the read model and the audit log, never a silent gap.
- **The server boots without the broker** — the bus is optional and degrades
  gracefully.
- **Tenant isolation** — the outbox table is row-level-security-forced; the relay's
  cross-workspace read is the single deliberate privileged exception, because the
  relay is infrastructure, not a tenant query. On the bus itself, every consumer
  filters on the workspace field in the envelope.

## Acceptance

Done means: a committed mutation is observable on the bus exactly as the catalog
describes it — right type, right envelope, right payload keys, one audit row linked in
the trace — and a rolled-back mutation is observable nowhere. A consumer replaying a
redelivered event produces no double effect. With the broker absent, the server still
boots and serves, and staged events publish once it returns. The catalog appendix is
the testable form: every one of its 46 event types (per the catalog appendix total) is
a contract a fixture can assert against.

## Out of scope

Queue and stream hygiene numbers — retention, trim caps, alert thresholds — belong to
the operations chapter (cite OPS-QUEUE). The outbox table DDL belongs to the
data-model chapter ([[data-model#DM-DDL-9]]). The approval flow's user surface and
tool tiers belong to the agent chapters; only the approval *events* are defined here.

## Where it lives

The outbox, relay, River-backed jobs, and broker wiring live in the platform events
layer under `backend/internal/platform/events`; emitting modules live under
`backend/internal/modules/` and stage events through the outbox in their own
transactions. Read next: the data-model chapter for the outbox and audit-log shapes,
the operations chapter for stream hygiene, and the architecture chapter for the module
and port layout the consumer groups map onto.

## Appendix

### Events — envelope
Source: contract/events.md#2-the-standard-envelope; contract/events.md#3-delivery-semantics @ 5a0b29c; margince-poc/docs/architecture/event-bus.md @ a11d6c08

**EVT-ENV-1 — the standard envelope.** Every event on the bus is this shape; `payload`
is the only per-type-varying field. Typed in the generated Go and TS contract types.

```jsonc
{
  "event_id":       "uuidv7",            // unique per event; UUIDv7 so it is time-ordered (dedupe key)
  "type":           "deal.stage_changed",// <entity>.<verb> from the catalog
  "version":        1,                    // schema version of THIS type's payload; bumped on breaking change
  "workspace_id":   "uuid",              // tenant key; every consumer filters on it (RLS analogue on the bus)
  "occurred_at":    "2026-06-04T10:15:30.123Z", // UTC instant the fact happened (= audit_log.occurred_at)
  "actor": {                              // who caused it (mirrors audit_log actor columns)
    "type":        "human|agent|connector|system",
    "id":          "human:<user-uuid> | agent:<agent-id> | connector:<name> | system",
    "passport_id": "uuid|null",          // Agent Seat Passport that authorized an agent action
    "on_behalf_of":"uuid|null"           // the human authority for an agent/connector action
  },
  "entity": {                             // the entity ref — NOT the body
    "type": "person|organization|deal|lead|activity|approval|...",
    "id":   "uuid"
  },
  "payload":        { /* per-type, see catalog — changed/relevant fields only */ },
  "trace": {
    "correlation_id": "uuid",            // groups all events from one originating request/agent-run/capture batch
    "causation_id":   "uuid|null",       // the event_id that caused THIS one (chains: capture → created → stage_changed)
    "audit_log_id":   "uuid"             // the audit_log row written in the same operation
  }
}
```

Additive payload fields do **not** bump `version`; removing/renaming/retyping a field
does, and the old version stays published until consumers migrate (EVT-DEL-6).

The delivery contract:

| ID | Rule |
|---|---|
| EVT-DEL-1 | **At-least-once delivery.** A consumer may see an event more than once (retry, reconnect, redelivery of a pending entry). Mechanism: stream consumer-group reads with acknowledgement; un-acked entries are redelivered via claim. |
| EVT-DEL-2 | **Consumers dedupe on `event_id`.** Effects must be idempotent (upsert by natural key, never blind insert). Each consumer keeps a processed-`event_id` set with TTL ≥ the redelivery window; mirrors the DB-level capture idempotency keys owned by the data-model chapter. |
| EVT-DEL-3 | **Per-entity ordering only; no global order.** All events for a given `entity_id` append to the same per-type stream in commit order; across entities order is best-effort (`occurred_at` + time-ordered `event_id` break ties). |
| EVT-DEL-4 | **Durability window.** Events survive a consumer being offline; pending entries are retained until acked or claimed. Streams are a transient delivery buffer — the audit log is the permanent record; trim caps and retention values are owned by [[operations#OPS-QUEUE]]. |
| EVT-DEL-5 | **Transactional outbox + relay.** Domain write + audit row commit in one transaction; the event publish is post-commit via an outbox row written inside that transaction, relayed to the stream by a background job and stamped published. No event for a rolled-back write; no write lost if the broker is briefly down. Table shape: [[data-model#DM-DDL-9]]. |
| EVT-DEL-6 | **Migration windows pin to the retention horizon.** Consumer dedupe-set TTL and the dual-publish window for a bumped payload version are both ≥ the stream retention horizon (values: [[operations#OPS-QUEUE]]); dual-publish runs until one full retention horizon after the last consumer group migrates off the old version (monitored). A consumer offline longer than retention is a fail-loud alarm that must re-bootstrap from the read model / audit log — never a silent gap. |
| EVT-DEL-7 | **Redelivery reorder is closed by version.** A claim-redelivered entry can arrive after later ones; order-sensitive consumers key on the entity's version/updated-at and ignore anything older than the value already applied (last-writer-wins by version, not arrival). |

### Events — streams & consumer groups
Source: contract/events.md#4-redis-streams-layout @ 5a0b29c

One stream per entity type on the shared `gw:events` bus; workspace is an envelope
field, not a stream (per-tenant streams would explode key count at multi-tenant
scale). Each stream entry is a single field holding the JSON envelope; the
time-ordered `event_id` lives inside, the stream's own auto-id outside.

| ID | Stream key | Entity segment |
|---|---|---|
| EVT-STREAM-1 | `gw:events:crm:person` | person |
| EVT-STREAM-2 | `gw:events:crm:organization` | organization |
| EVT-STREAM-3 | `gw:events:crm:deal` | deal |
| EVT-STREAM-4 | `gw:events:crm:lead` | lead |
| EVT-STREAM-5 | `gw:events:crm:activity` | activity |
| EVT-STREAM-6 | `gw:events:crm:approval` | approval |
| EVT-STREAM-7 | `gw:events:crm:capture` | capture |
| EVT-STREAM-8 | `gw:events:crm:coldstart` | coldstart |
| EVT-STREAM-9 | `gw:events:crm:audit` | audit |

<!-- reconcile: the corpus pins 9 streams, but its catalog later grew entity segments
(consent, retention, offer, mirror, engagement, signal, forecast) whose stream homes
are not pinned. The per-entity-type rule implies additional streams; default until
adjudicated: consent/retention ride the person stream (person-scoped facts, same
emitting module), offer rides the deal stream, and engagement/signal/forecast plus the
deferred mirror trio get their own streams when their WPs land. SPEC-DISPUTE candidate. -->

One consumer group per consuming module — each module gets every event once per group
and scales horizontally inside the group. Corpus module names map to the target layout
as: crm-core → `modules/people` (the shared domain module — deals, leads, offers, and
organizations are slices of it under the ratified layout), crm-capture →
`modules/capture`, crm-agents → `modules/agents`, crm-ai → `modules/ai`, crm-search →
`modules/search`; bus plumbing and the workflow dispatch seam are `platform/events`.

| ID | Group | Module (target layout) | Subscribes to | Purpose |
|---|---|---|---|---|
| EVT-CG-1 | `cg:context-graph` | modules/search + modules/ai | person, organization, deal, activity, lead | ADR-0007 context-graph assembly — capture→link assembly, relationship strength, pgvector (re)embedding, cross-pipeline reasoning inputs. |
| EVT-CG-2 | `cg:overnight-agent` | modules/agents | activity, deal, lead, approval | Overnight agent / Morning Brief — "what changed overnight", stalled-deal + field-hygiene sweeps, ranked action queue. |
| EVT-CG-3 | `cg:workflows` | platform/events (handler registry) | all (filtered per handler's declared trigger) | Dispatch typed workflow handlers; trigger→dispatch p95 < 200ms (AC-W2); idempotent on `event_id` (AC-W3). |
| EVT-CG-4 | `cg:capture` | modules/capture | capture.* | Drive the capture pipeline state machine; emit downstream `activity.captured` / `person.created`. |
| EVT-CG-5 | `cg:flow-bridge` | Dispact interop | person, deal, activity | Cross-link CRM ↔ Dispact conversations; mirror to Dispact's view of the shared bus. |
| EVT-CG-6 | `cg:read-model` | modules/people | all | Maintain Redis read-model caches — invalidate/refresh on mutation (version-guarded, EVT-DEL-7). |
| EVT-CG-7 | `cg:audit-stream` | modules/agents (security) | audit, approval, all `actor.type=agent` | Anomaly detection / D6 egress monitoring. |

### Events — catalog
Source: contract/events.md#5-the-catalog @ 5a0b29c

**46 event types total — 43 active, 3 deferred** (the overlay-mirror trio, marked
below). ID is the event type string; `Δ` denotes a before/after diff sub-object
`{ field: {from, to} }` mirroring the audit log's before/after. Payload shows key
fields only; every event also carries the full EVT-ENV-1 envelope. Emit is the owning
module in target-layout vocabulary (mapping above). Consumers: CG = `cg:context-graph`,
ON = `cg:overnight-agent`, WF = `cg:workflows`, RM = `cg:read-model`, FB =
`cg:flow-bridge`, AS = `cg:audit-stream`, CAP = `cg:capture`; bold = the load-bearing
consumer.

**Person (7)**

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `person.created` | 1 | `full_name, primary_email?, owner_id?, source, captured_by, converted_from_lead_id?` | modules/people | **CG**, RM, WF, FB |
| `person.updated` | 1 | `delta: Δ` (e.g. title, owner_id, legal_hold) | modules/people | **CG**, RM, WF |
| `person.archived` | 1 | `reason?` | modules/people | CG, RM |
| `person.merged` | 1 | `merged_from_id, merged_into_id, relinked: {emails, phones, relationships, activity_links}` — own verb, not two updates: the context graph collapses two nodes and relinks edges atomically | modules/people | **CG**, RM, WF |
| `person.restored` | 1 | `{}` | modules/people | CG, RM |
| `consent.changed` | 1 | `person_id, purpose, new_state, lawful_basis, source, policy_version` (per-purpose; backed by the append-only consent event log) | modules/people | RM, WF (outbound suppression) |
| `retention.applied` | 1 | `object_type, object_id, action: archive\|anonymize\|erase, policy_id` (nightly evaluator; skips legal hold) | modules/people | RM, AS |

**Organization (5)**

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `organization.created` | 1 | `display_name, primary_domain?, parent_org_id?, owner_id?, source, captured_by` | modules/people | **CG**, RM, WF |
| `organization.updated` | 1 | `delta: Δ` (incl. enriched firmographics, custom columns) | modules/people | **CG**, RM, WF |
| `organization.archived` | 1 | `reason?` | modules/people | CG, RM |
| `organization.merged` | 1 | `merged_from_id, merged_into_id` | modules/people | **CG**, RM |
| `organization.restored` | 1 | `{}` | modules/people | CG, RM |

**Deal (6)**

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `deal.created` | 1 | `name, pipeline_id, stage_id, amount_minor?, currency?, organization_id?, owner_id?, source, captured_by` | modules/deals | **CG**, **ON**, RM, WF |
| `deal.updated` | 1 | `delta: Δ` (amount, expected_close_date, custom fields; **not** stage, **not** owner — each has its own event) | modules/deals | **CG**, **ON**, RM, WF |
| `deal.stage_changed` | 1 | `from_stage_id, to_stage_id, from_status, to_status, amount_minor_at_change?, currency_at_change?, win_probability` — the load-bearing deal event; carries the amount snapshot so as-of-date reports and the stalled/forecast sweep react without a read-back | modules/deals | **CG**, **ON**, RM, WF |
| `deal.owner_changed` | 1 | `from_owner_id?, to_owner_id, reason?` | modules/deals | **CG**, **ON**, RM, WF |
| `deal.archived` | 1 | `reason?` | modules/deals | CG, ON, RM |
| `deal.restored` | 1 | `{}` | modules/deals | CG, RM |

**Offer / Angebot (5)** — A48/ADR-0037

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `offer.created` | 1 | `offer_id, deal_id, revision, currency, source, captured_by` | modules/people | CG, RM |
| `offer.sent` | 1 | `offer_id, deal_id, revision, gross_minor, fx_rate_to_base, valid_until` (🟡 — leaves the workspace; rides the approval gate + effect-bound token like `send_email`) | modules/people | CG, ON, RM, WF |
| `offer.accepted` | 1 | `offer_id, deal_id, revision, gross_minor` (server-computed; pairs a `deal.updated`, EVT-SEM-4) | modules/people | **CG**, **ON**, RM, WF |
| `offer.rejected` | 1 | `offer_id, deal_id, revision, reason?` | modules/people | CG, ON, RM |
| `offer.superseded` | 1 | `offer_id, deal_id, from_revision, to_revision` | modules/people | CG, RM |

**Lead (4)**

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `lead.created` | 1 | `full_name?, email?, company_name?, status, score, source, captured_by, source_system?, source_id?` | modules/people / modules/capture | **CG** (lead-segregated view only, EVT-SEM-5), **ON**, WF (routing) |
| `lead.updated` | 1 | `delta: Δ` (status new→working, score recompute, owner) | modules/people | ON, WF |
| `lead.promoted` | 1 | `promoted_person_id, dedupe_outcome: "merged"\|"created", trigger: "inbound_reply"\|"meeting"\|"human_qualify", evidence_ref` | modules/people | **CG**, **ON**, RM, WF |
| `lead.disqualified` | 1 | `reason?` | modules/people | ON, WF |

**Activity (3)**

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `activity.captured` | 1 | `kind, occurred_at, links: [{entity_type, entity_id}], source_system?, source_id?, source, captured_by` — the spine of P5 and the context graph; emitted once per normalized activity (idempotent on source system + id); drives deal last-activity maintenance + the stalled-deal sweep; `captured_by` is the manual-entry-smell metric input (ACX.2) | modules/capture | **CG**, **ON**, RM, WF, FB |
| `activity.updated` | 1 | `delta: Δ` (e.g. task is_done, human correction of a captured field — emitted with `captured_by=human:*`, the typed-by flag) | modules/people | CG, ON, WF |
| `activity.archived` | 1 | `reason?` | modules/people | CG, RM |

**Approval (2)** — the 🟡 confirm-first gate

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `approval.requested` | 1 | `approval_id, tool, risk_tier: "yellow", requested_by_agent, on_behalf_of, target: {entity_type, entity_id}, proposed_effect, dry_run_diff, expires_at` | modules/agents | RM (approval inbox UI), **ON**, AS |
| `approval.decided` | 1 | `approval_id, decision: "approved"\|"rejected"\|"expired", decided_by, edited_effect?, approval_token?` | modules/agents | modules/agents (execute on approve), RM, WF, AS |

**Capture pipeline (4)** — connector-actor events (`actor.type=connector`)

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `capture.received` | 1 | `connector, source_system, external_id, raw_ref, sync_cursor?` | modules/capture | CAP, AS |
| `capture.normalized` | 1 | `connector, source_system, external_id, produced: [{entity_type, entity_id, op}]` | modules/capture | **CG**, CAP |
| `capture.failed` | 1 | `connector, source_system, external_id, error_class, retryable` | modules/capture | CAP (dead-letter), RM (ops dashboard) |
| `capture.skipped` | 1 | `connector, source_system, external_id, reason: "personal_exclusion"\|"duplicate"\|"out_of_scope"` (the AC1.3 exclusion proof, EVT-SEM-10) | modules/capture | AS |

**Cold-start (3)**

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `coldstart.read_back_proposed` | 1 | `source_url, fields: [{name, value, evidence_snippet, source_url, confidence}], degraded?` (staging only — writes nothing, EVT-SEM-11) | modules/capture (scrape seam, ADR-0006) | RM (staging card UI) |
| `coldstart.accepted` | 1 | `source_url, accepted_fields, produced: [{entity_type, entity_id}]` | modules/people | **CG**, RM, AS |
| `coldstart.rejected` | 1 | `source_url, reason?` | modules/people | AS |

**Audit (1)** — cross-cutting

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `audit.appended` | 1 | `audit_log_id, action, entity_type, entity_id?, authorization_rule?` (thin pointer, EVT-SEM-12; never emitted for non-mutating reads) | modules/people | AS (anomaly/D6), RM (audit view) |

**Overlay / augmentation mirror (3) — DEFERRED** to the overlay WP (A53). Exist only
in overlay mode against an incumbent system of record; absent in native mode.
Registered here so the contract names them (P3) even though the mirror schema and
adapter build are decomposed at WP entry. Emitted by the incumbent adapter (the
overlay datasource adapter), never above the datasource seam (AC-OV-1).

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `mirror.conflict` — *deferred* | 1 | `incumbent, object_type, mirror_id, incumbent_external_id, resolution: "incumbent_wins", diverged_fields: [name]` | overlay adapter | RM (overlay observability), AS |
| `mirror.write_rejected` — *deferred* | 1 | `incumbent, object_type, mirror_id, reason: "version_skew"\|"precondition_failed"\|"scope_denied", baseline_ref` | overlay adapter | RM (caller "record changed, review" / 🟡 re-resolution), AS |
| `mirror.budget_degraded` — *deferred* | 1 | `incumbent, budget: "rate"\|"daily"\|"24h_allocation"\|"service_protection", action: "force_fresh_degraded_to_mirror", staleness_ms` | overlay adapter | RM (staleness warning), AS |

**Engagement & signals (4)** — E08 warm-room, E15 reply-tracking

| ID | v | Payload (key fields) | Emit | Consumers |
|---|---|---|---|---|
| `engagement.reply` | 1 | `matched_outbound_activity_id, contact_id, organization_id?, channel: "email"\|"deal_room", occurred_at, idempotency_key` (thread-match, EVT-SEM-14; a duplicate inbound for the same reply does **not** re-emit) | modules/capture | **CG**, RM (warm-room), WF, AS |
| `signal.detected` | 1 | `signal_id, kind, source_channel, entity_type, entity_id, resolution_state, resolution_confidence?, severity` | modules/ai / modules/capture | **CG**, RM (warm-room), **ON**, AS |
| `signal.resolved` | 1 | `signal_id, resolution_state, resolved_org_id?, resolved_person_id?, matched_on?, match_confidence?` | modules/ai | RM, AS |
| `forecast.period_closed` | 1 | `scope: "owner"\|"team", owner_id?, team_id?, pipeline_id?, period_start, period_end, predicted_minor, actual_minor, currency` (one per closed period per scope; captures a forecast snapshot) | modules/people (scheduled period-close job) | RM (forecast accuracy), AS |

### Events — semantic rules
Source: contract/events.md#0-why-an-event-catalog-the-load-bearing-decisions; contract/events.md#5-the-catalog; contract/events.md#7-open-questions @ 5a0b29c

| ID | Rule |
|---|---|
| EVT-SEM-1 | **One mutation → one audit row + one domain event.** Emitted application-side, post-commit (not a DB trigger); the audit row is the durable record of *what happened*, the event is the *notification*. Same logical operation, publish staged via the outbox (EVT-DEL-5). |
| EVT-SEM-2 | **Specific verbs replace generic `updated` — never double-fire.** `deal.stage_changed`, `deal.owner_changed`, `person.merged`, `lead.promoted`, `activity.captured` are emitted *instead of* a generic `*.updated` for that transition, never in addition. Consumers subscribe by meaning; the catalog stays statically enumerable for contract/codegen. |
| EVT-SEM-3 | **The one sanctioned co-fire.** A single write changing both stage and amount emits BOTH `deal.stage_changed` and `deal.updated` (distinct concerns, distinct consumers), sharing one `correlation_id`. A stage transition is never folded into a generic `updated`. |
| EVT-SEM-4 | **Offer-accept pairs a deal update.** On `offer.accepted` the domain module syncs the deal's amount/currency from the accepted offer's server-computed gross and emits a paired `deal.updated` under the same `correlation_id`. Money totals are server-computed (P11); the event never carries a client value. |
| EVT-SEM-5 | **Lead events never enter the contact graph (ADR-0008).** `lead.created` is recorded for routing/scoring/lead-list freshness only — excluded from person dedupe, relationship strength, and "people we know" until `lead.promoted` materializes a person, carrying the conversion lineage + triggering evidence. A cold outbound touch with no reply never fires `lead.promoted`. |
| EVT-SEM-6 | **Refs, not bodies — no field-mask bypass.** Events name `(entity_type, entity_id)` + a small typed payload of changed/relevant fields, never the full row. Consumers needing full or masked data read back through the datasource port under their own scopes. Minimal denormalized routing keys (e.g. `primary_email`) are allowed; sensitive or field-masked values never ride the bus. Any consumer forwarding event content externally is itself a gated tool/connector. |
| EVT-SEM-7 | **Per-entity ordering only** — the consumer-facing form of EVT-DEL-3: key on `entity.id` for order; never assume cross-entity order. |
| EVT-SEM-8 | **Last-writer-wins by version on redelivery reorder** — the consumer obligation form of EVT-DEL-7: ignore an update older than the entity version already applied. |
| EVT-SEM-9 | **Every 🟡 action rides the approval pair.** The tool returns requires-approval + emits `approval.requested` carrying the dry-run diff; on `approval.decided{approved}` the agent executes with the approval token, and the resulting domain event (e.g. `activity.captured`, `deal.stage_changed`) carries the same `correlation_id` (the ACX.3 confirm-first invariant on the bus). |
| EVT-SEM-10 | **Capture is one correlation chain.** `capture.received` → `capture.normalized` → the per-entity `*.created`/`activity.captured` events share one `correlation_id`; `causation_id` links each step. `capture.skipped{personal_exclusion}` is the machine-verifiable AC1.3 proof that excluded mail produced zero rows. |
| EVT-SEM-11 | **Cold-start stages, never writes.** `coldstart.read_back_proposed` writes nothing to real tables (zero rows before accept); only `coldstart.accepted` triggers the actual writes (stamped `source=coldstart`, `captured_by=agent:coldstart`), which then emit their own `*.created` events under one `correlation_id`. Every proposed field carries its evidence snippet or is absent (the no-guess gate). |
| EVT-SEM-12 | **`audit.appended` is a thin pointer.** For consumers that only need "an auditable thing happened" without subscribing to every domain stream; carries `audit_log_id` so the consumer reads the full before/after diff from the durable table. Never emitted for non-mutating reads. |
| EVT-SEM-13 | **Mirror events are overlay-only and incumbent-wins.** `mirror.*` exist only in overlay mode, emitted by the incumbent adapter below the datasource seam (AC-OV-1); `mirror.conflict` fires when reconciliation overwrites a diverged mirror row from a fresh incumbent read — never the reverse (AC-OV-8); `mirror.write_rejected` carries the lost-update protection (AC-OV-4). Observability/UX signals only — they never mutate canonical incumbent data. Deferred to the overlay WP. |
| EVT-SEM-14 | **Engagement is reply-based, never an open-pixel.** `engagement.reply` fires only when an inbound message thread-matches a prior outbound, idempotent per reply (a duplicate inbound for the same reply does not re-emit). |
