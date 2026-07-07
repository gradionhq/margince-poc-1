---
status: planned
module: backend/internal/modules/automation (catalog registry, store, matcher/scheduler); web (automations surface)
derives-from:
  - specs/spec/features/10-operational-depth.md#1-user-centric-automation--bounded-catalog--agent-authored-standing-automations-a45adr-0035
  - specs/spec/features/03-reporting-and-scoring.md#5-workflows--automation
  - specs/spec/contract/data-model.md#automation-e15--adr-0035-bounded-catalog--agent-authored-standing-automations
  - specs/spec/contract/runtime-config-surface.md
  - specs/spec/decisions/ADR-0035-user-centric-automation.md
  - specs/spec/product/epics/E15-operational-depth.md
  - specs/spec/product/30-screen-acceptance.md#automationshtml--automations-catalog--plain-language-implements-s-e151-s-e152
  - margince-poc/docs/subsystems/automation.md @ a11d6c08
---
# Automation — a closed catalog you switch on or describe, never an engine you draw

> Everyday automation as a bounded product: a closed set of trigger-and-action
> templates a user enables and parameterizes, plus standing automations an agent
> authors from plain language — both executing the same governed action set,
> every firing tiered, approved where it must be, and audited. Its promise: no
> code, no canvas, no rules language — ever.

## What it's for

Routine operational automation — remind me when a deal goes quiet, make a task
when a stage changes, nudge me before a renewal — is user-centric, per-person
work that every competitor answers with a workflow builder. This product
refuses the builder (ADR-0035; [[scope#NEVER-2]]) but must still answer the
need without sending a sales rep to edit source. This subsystem owns that
answer end to end: the closed trigger-and-action catalog and the single
in-code registry that validates it, the standing-automation store, the runtime
that matches events and scans clocks to fire them, and the automations surface
where both authoring paths meet. Its callers are the automations screen (the
rep's enable-and-parameterize surface), the agent's governed authoring verbs
(a standing automation is a first-class object an agent can create, list,
pause, and delete), the event bus and background job queue that drive firings,
and the approval inbox that holds anything outward. The boundary: bespoke
automation *logic* beyond the catalog is a source change (ADR-0002), and
judgment-shaped standing runs execute on the loop the
[agent-runner](agent-runner.md) chapter owns — this chapter owns what an
automation is allowed to be and the deterministic path that fires it.

## Principles it serves

- **P1 — opinionated over configurable.** The catalog is a small, enumerated,
  typed set of templates, not an engine: no user-defined trigger or action
  types, no branching graphs, no expression language. It passes the
  runtime-config four-part test as its own bounded row
  ([[runtime-config#RC-11]]), and the product's scope guard names the rejected
  alternative outright ([[scope#NEVER-2]]).
- **P2 / ADR-0002 — customization is source.** Anything the catalog cannot
  express is either the agent path or a source-level typed handler — never an
  "add custom step" escape hatch. The bright line between user-centric
  operational automation and system-centric bespoke logic is the chapter's
  founding decision (ADR-0035, decision A45).
- **P12 — governance is designed in.** Every action carries a registry-derived
  autonomy tier (ADR-0026); outward or irreversible effects wait in the
  approval inbox; every firing is audited with its trigger evidence. An
  automation is just another principal acting through the one governed surface
  (ADR-0013) — no second permission model exists.
- **P5 — triggers ride captured signals.** The catalog's event triggers are
  the same domain events capture and the record surfaces already emit; the
  automation layer adds no private signal path.

## How it works

**One closed catalog, one registry.** An automation is one stored row: a name,
an origin saying whether a human enabled it from the catalog or an agent
authored it, a bounded trigger specification, one bounded action, an owner, an
enabled flag, and a tier. Trigger and action values are structured data, but
their *types* come only from the closed catalog — seven triggers and seven
actions (AUTO-PARAM-1, AUTO-PARAM-2), pinned verbatim in the appendix. A
single in-code registry is the sole authority on membership, and a
registry test asserts the enumeration in both directions: every catalog entry
exists in the registry and the registry contains nothing else. Adding a
trigger or an action is a code-and-test change, never data. The same closure
is re-asserted at the wire: a request carrying an out-of-catalog type, a
free-form rule or expression body, or a multi-step branching shape is refused
with a validation error on both the REST and agent surfaces — the anti-DSL
contract guard that keeps this a catalog rather than a rules engine.

**Path 1 — switch it on.** A user picks a template, fills its bounded
parameters — the N in "no activity for N days", the target stage, the scope —
and turns it on. Each automation is listable, editable, pausable, and
deletable, and every one of those mutations writes an audit row. This path
ships as product and needs no agent: the baseline tier gets everyday
automation deterministically.

**Path 2 — describe it.** A user tells their agent the rule in their own
words. The agent maps the request onto catalog entries and persists a named
standing automation — same table, same shapes, same enforcement; an
agent-authored automation round-trips identically to a catalog-authored one.
The authoring is unbounded *expression*; the execution is the bounded catalog.
An agent-authored or agent-suggested automation is never silently active: it
arrives as a staged 🟡 proposal the human accepts, edits, or declines, riding
the same staging machinery the
[approvals-and-concurrency](approvals-and-concurrency.md) chapter owns.

**Firing — two entries, one path.** Event triggers fire from the bus: the
matcher consumes the domain streams through the workflow consumer group
([[event-bus#EVT-CG-3]]), loads enabled automations matching the event type,
evaluates the closed filter, gates on the owner's effective permissions, and
fires. The three time-derived triggers — no activity for N days, a date field
approaching, a task overdue — consume no event; they fire from a periodic scan
on the background job queue against an injected clock, so a fixed-clock
fixture yields a deterministic firing set. Both entries converge on one shared
firing path that guarantees at-most-once firing per automation, entity, and
occurrence — replayed events and re-ticked scans dedupe rather than
double-fire, mirroring the bus-wide idempotency rule
([[event-bus#EVT-DEL-2]]).

**Tier routing at the moment of firing.** Each action's tier is read from the
registry, never set by the caller. Reversible, internal actions — create a
task, notify, draft, set a field, add to a list — run automatically (🟢).
Outward or irreversible effects — a send, a reassignment at scale, a close, an
archive — are held in the approval inbox and execute only on human approval
(🟡), consuming the single-use approval token like any other staged action
(the approvals chapter's APPR-WIRE-1 machinery). An automation can never grant
an action a tier above its authoring human's effective rights — the Passport
intersection rejects the attempt at authoring time, so the escalation cannot
exist, let alone fire.

**Every firing leaves a trail.** A firing writes one row to the run-provenance
table (fired, skipped, queued for approval, or failed), one audit entry
carrying the automation identity, trigger evidence, actor, and action result,
and emits the relevant domain event — "why did this happen?" is always
answerable from the audit alone. The automations list shows each rule's
status and most recent run from that same provenance, not a separate write
path.

**Underneath: typed handlers, not interpreted rules.** The execution machinery
is the workflow registry the reporting-era spec defined: each deterministic
automation dispatches to a typed handler registered with a declared trigger,
typed input contract, idempotency key, and risk tier, running on the durable
job queue with retries, dead-lettering, and replayable provenance. A scaffold
generator emits new handlers with registration and a test stub, so extending
the vocabulary is a consistent, test-guarded source change (P2, P3). The
user-facing catalog is exactly the subset of that registry productized as
enable-and-parameterize templates ([[runtime-config#RC-11]]); the registry's
wider handler vocabulary is source, reachable only by source-level work. What
needs judgment rather than enumeration — the standing automation that must
*reason* about a quiet deal — executes on the [agent-runner](agent-runner.md)
loop instead; that chapter deliberately left the catalog and the deterministic
path here.

**Suggestions are proposals.** The system may notice a repeated manual
sequence and propose an automation — but a suggestion is a 🟡 proposal the
user accepts or declines, never a silently applied rule, and the
pattern-based suggestion capability itself is deferred beyond V1.

## What's configurable

- **Per-automation bounded parameters only** (AUTO-PARAM-1, AUTO-PARAM-2) —
  the numbers, fields, stages, and scopes a chosen template exposes, plus
  enable, pause, and delete. This is the entirety of the runtime surface, and
  it is recorded as its own runtime-config row ([[runtime-config#RC-11]]).
  The catalog itself is closed: extending it is a deliberate code-and-test
  change, and nothing about triggers, actions, tiers, or shapes is a knob.
- **The event bus and job queue** — injected platform dependencies. Without
  the bus, event triggers do not fire (time-scans still do); without the job
  runner, time triggers do not fire. Neither degrades into polling the
  database from the request path.
- **The clock** — injected for the time-scan, so deterministic tests pin the
  firing set.
- **The agent** — Path 2 requires a connected agent; without one, Path 1 is
  fully functional and the V1-Must story stands alone.

## Guarantees (enforced)

- **Catalog-closed, both directions.** The stored trigger and action types
  exactly match the source enumeration; an out-of-set type is rejected, not
  stored, and the registry test fails if code and pinned catalog drift
  (AUTO-AC-5, AUTO-SCHEMA-1).
- **No DSL, structurally.** A contract test asserts that no API path — REST or
  agent-facing — accepts a user-defined trigger or action type, a free-form
  rule or expression body, a branching graph, or a custom code step
  ([[scope#NEVER-2]]; AUTO-WIRE-3).
- **Tier integrity.** An action's tier comes from the registry, never from the
  caller; 🟢 actions run automatically and 🟡 actions never execute without an
  approval record (AUTO-AC-1, AC-W4).
- **The author's ceiling.** An automation cannot carry an action exceeding the
  authoring human's effective rights; the Passport intersection rejects it at
  authoring time (AUTO-AC-2).
- **At-most-once firing.** One firing per automation, entity, and occurrence;
  event replay and scan re-ticks do not double-apply (AC-W3).
- **Audited and evented, every time.** Each firing writes exactly one run row,
  one audit entry with trigger evidence, and the relevant domain event
  (AUTO-AC-3).
- **Agent path has no backdoor.** Agent authoring passes the same anti-DSL
  guard, the same tier routing, and the same Passport ceiling as the human
  path; agent-authored and catalog-authored rows round-trip identically
  (AUTO-AC-4).
- **Nothing activates silently.** An agent-proposed automation is staged, not
  live, until an explicit human accept (AC-automations-4, AC-automations-5).

## Acceptance

Done means: a rep turns on "remind me if a deal I own has no activity for N
days", fills in one number, and gets the reminder — no code, no agent, no
canvas; the same rep describes a nudge rule to their agent in plain words and
a named standing automation appears in their list, its draft step running
automatically and its send waiting in the approval inbox; pausing, editing,
and deleting behave as stated; and every run is reconstructable from the audit
trail. The surface renders its honest states — a paused rule visibly paused, a
denied org-wide panel honestly locked with a route to an admin, a
deliberately-absent builder explained rather than hidden — and the standard
screen-state floor is inherited from the acceptance-standards chapter, not
restated. The testable form of every claim lives in the Acceptance appendix,
including the screen's verbatim acceptance rows and the workflow-registry
criteria the execution machinery must keep.

## Out of scope

- **A visual builder, permanently.** Not deferred — rejected
  ([[scope#NEVER-2]], ADR-0035). Unmet needs route to a product-backlog
  request or source-level work, and the surface says so on screen.
- **Bespoke automation logic** — new trigger or action types, structural
  behavior, custom routing or scoring — is source (ADR-0002), owned by the
  customization seams, not this chapter.
- **The agent execution loop** for judgment-shaped standing runs — budgets,
  suspend/resume, headless governance — belongs to
  [agent-runner](agent-runner.md); the overnight sweep content to
  [overnight-agent](overnight-agent.md).
- **Approval mechanics** — staging, tokens, TTL, re-validation — belong to
  [approvals-and-concurrency](approvals-and-concurrency.md); the inbox surface
  to [notifications-and-approval-inbox](notifications-and-approval-inbox.md).
- **Neighbouring engines actions compose with** — the add-to-list action
  writes through the list engine
  ([lists-views-segmentation](lists-views-segmentation.md)); the set-field
  action may target a governed custom field
  ([custom-fields](custom-fields.md)); outbound cadences are
  [sequences-and-deliverability](sequences-and-deliverability.md), not an
  automation template.
- **Pattern-based automation suggestions and per-automation analytics** —
  deferred beyond V1 by the feature cut line.

## Where it lives

Planned backend home: `backend/internal/modules/automation` — the catalog
registry, the standing-automation store, and the matcher/scheduler use cases —
with event dispatch riding the platform events seam
([[event-bus#EVT-CG-3]]) and time-scans on the background job queue. Planned
frontend home: the automations surface in `web`. Read
[agent-runner](agent-runner.md) for where judgment-shaped runs execute,
[approvals-and-concurrency](approvals-and-concurrency.md) for how a 🟡 firing
commits, and [byo-agent-and-mcp](byo-agent-and-mcp.md) for the governed tool
surface the authoring agent comes through.

## Appendix

### Parameters
Source: contract/runtime-config-surface.md (RC-11 row) @ 5a0b29c; features/10-operational-depth.md#1-user-centric-automation--bounded-catalog--agent-authored-standing-automations-a45adr-0035 @ 5a0b29c

The catalog **is fully enumerated in the corpus** — RC-11 and features/10 §1
carry the same verbatim list; no docs-must-complete gap exists for the
enumeration itself. The values below are pinned verbatim.

| ID | Name | Value | Meaning |
|---|---|---|---|
| AUTO-PARAM-1 | Trigger catalog (closed, 7) | record created/updated · field-reaches-value · deal enters/leaves stage · no-activity-for-N-days · date-field approaching (close/renewal) · inbound reply · task overdue | The complete trigger vocabulary ([[runtime-config#RC-11]]). The paired forms (created/updated, enters/leaves) each count as one catalog entry per RC-11's enumeration. Adding an entry is a code change asserted by the registry test (AUTO-SCHEMA-1). |
| AUTO-PARAM-2 | Action catalog (closed, 7) | create task · notify (in-app / email / Dispact channel) · assign/reassign owner · add-to-list · set field · draft (never auto-send) email · request approval | The complete action vocabulary ([[runtime-config#RC-11]]); one bounded action per automation row. No user-defined actions, no branching, no custom step. |
| AUTO-PARAM-3 | Automation shape | trigger × action × filter × parameters | Per ADR-0035: each automation is one template instantiation; the filter is the closed predicate representation embedded in the trigger spec, not an expression language. |
| AUTO-PARAM-4 | Tier assignment (registry-derived) | 🟢 auto-run: create task, notify, draft, set field, add-to-list · 🟡 held for approval: send, mass reassign, close, archive | Read from the registry, never caller-set (features/10 §1 AC; build story B-E15.1/B-E15.5). See AUTO-NOTE-1 for the reconciliation flag on the 🟡 exemplars. |
| AUTO-PARAM-5 | Seeded starter templates (6) | No-activity reminder · Renewal reminder · Stage-change notify · Route new lead to a task · Check-in cadence · Post-meeting recap draft | The automations screen's "6 templates · no code" catalog (30-screen-acceptance AC-automations-2; template names from the automations mockup). The Renewal reminder is the decision-A50 renewals-by-assembly seed: a seeded catalog template over a renewal-date custom field — renewals are a template here, not a story elsewhere. |
| AUTO-PARAM-6 | Prebuilt handler library (~6) | lead routing · idle-deal flag · SLA escalation · stage-change task creation · score recompute · welcome-sequence enroll | features/03 §5.3's starter library for the underlying workflow registry (typed source-level handlers). See AUTO-NOTE-2. |

**AUTO-NOTE-1 (docs reconciliation, pinned):** the 🟡 exemplars in the corpus
tier-routing AC — send, mass reassign, close, archive — are not literal
members of the seven-action catalog (AUTO-PARAM-2). They name the outward
effect *class* a firing can reach only via approval: a send is the
approval-gated completion of a draft-email or request-approval action, and
reassign-at-scale is the held form of assign/reassign. The docs layer must
state the mapping from the seven authoring actions to the 🟡-held effect
classes explicitly when the registry lands; until then both lists are pinned
verbatim and this note is the bridge.

**AUTO-NOTE-2 (docs reconciliation, pinned):** the corpus carries two "~6
starter" lists — the workflow registry's prebuilt handler library
(AUTO-PARAM-6, features/03 §5.3, source-level) and the automations screen's
six user-facing catalog templates (AUTO-PARAM-5). They overlap (idle-deal flag
≈ No-activity reminder; stage-change task creation ≈ Stage-change notify; lead
routing ≈ Route new lead to a task) but are not identical, and features/03 §5
predates the A45 promotion that created the user-facing catalog. The
user-reachable set is AUTO-PARAM-5 over the RC-11 vocabulary; the handler
library is registry machinery. The build must reconcile the two lists into one
seeded catalog definition — a docs-must-complete item.

### Schema
Source: contract/data-model.md#automation-e15--adr-0035-bounded-catalog--agent-authored-standing-automations @ 5a0b29c

Ownership verified against the ownership index
([[data-model#schema--ownership-index]]): `automation` and `automation_run`
are owned by this chapter; the deferred `workflow_run` stub is also assigned
here on arrival ([[data-model#schema--deferred-tables-stubs-owner-on-arrival]]
DM-DEF-2 — run records only; likely reuses audit plus a thin run table). DDL
pinned verbatim:

```sql
CREATE TABLE automation (                                 -- a named standing automation (catalog template OR agent-authored)
  -- + base columns + version
  name          text NOT NULL,
  origin        text NOT NULL CHECK (origin IN ('catalog','agent_authored')),
  trigger       jsonb NOT NULL,                            -- bounded trigger spec (event + predicate)
  action        jsonb NOT NULL,                            -- bounded action from the closed action set
  owner_id      uuid NULL REFERENCES app_user(id),
  enabled       boolean NOT NULL DEFAULT true,
  tier          text NOT NULL DEFAULT 'green' CHECK (tier IN ('green','yellow'))  -- 🟡 actions queue to approval inbox
);
CREATE TABLE automation_run (                             -- execution provenance (reuses audit for effects)
  id uuid PRIMARY KEY, workspace_id uuid NOT NULL REFERENCES workspace(id),
  automation_id uuid NOT NULL REFERENCES automation(id),
  status        text NOT NULL CHECK (status IN ('fired','skipped','queued_for_approval','failed')),
  detail        jsonb NULL,
  ran_at        timestamptz NOT NULL DEFAULT now()
);
```

| ID | Constraint / rule | Meaning |
|---|---|---|
| AUTO-SCHEMA-1 | Registry closure test | The `trigger`/`action` jsonb are validated against the single in-code closed-catalog registry; a schema/registry test asserts the enumerated sets exactly match the RC-11 / features/10 §1 list in both directions — adding a member requires a code change, not data (build story B-E15.1). |
| AUTO-SCHEMA-2 | Tier is registry-derived | `tier` reflects the action's registry-declared tier; a row claiming a tier its action does not carry is rejected (B-E15.1/B-E15.5). |
| AUTO-SCHEMA-3 | Tenancy + base columns | Both tables are workspace-scoped with RLS and carry the standard base columns + optimistic-concurrency version per the data-model conventions (corpus §1, §12.5). |
| AUTO-SCHEMA-4 | Run table is append-only provenance | `automation_run` is written by the firing path only and read through the parent resource; no standalone CRUD (corpus §12.5 contract-surface note). |

### Wire
Source: contract/crm.yaml (NET-NEW V1 RESOURCES block, data-model.md §12.5) @ 5a0b29c

Honest coverage report: the contract specifies `/automations` **by pattern,
not by expanded paths** — the NET-NEW V1 RESOURCES block declares that each
§12.5 resource follows the same shape as the expanded core resources (list
with cursor+sort, get, create, update with If-Match, archive) with schemas
drawn 1:1 from the DDL, and requires contract-codegen plus the AC-MCP-3 lint
to emit and verify the operations. No literal automation operationIds exist in
the contract file today (AUTO-GAP-1).

| ID | Surface | Role in this chapter |
|---|---|---|
| AUTO-WIRE-1 | `/automations` (+`/{id}`) — pattern-specified list/get/create/update(If-Match)/archive | Enable, parameterize (edit trigger/action params), pause (enabled=false), delete; schema 1:1 from the §12.5 DDL; every mutation writes one audit row (B-E15.4). |
| AUTO-WIRE-2 | `/automations/{id}/runs` | Run-provenance read via the parent resource; 🟡 firings queue to the approvals surface (`/approvals`), per the contract's resource note. |
| AUTO-WIRE-3 | 422 anti-DSL rejection | A user-defined trigger/action type, free-form rule or expression body, multi-step/branching graph, or "custom code step" is rejected with a reason pointing to the source path; a missing/mistyped bounded param is also 422 (B-E15.2, B-E15.4). Holds identically on REST and MCP — no privileged backdoor (ADR-0013). |
| AUTO-WIRE-4 | MCP authoring over the agents surface | A standing automation is first-class over the `crm-agents` MCP surface — create / list / pause / delete under Passport scopes (features/10 §1 AC; B-E15.6); a 🟡 firing's execution consumes the single-use approval token ([approvals-and-concurrency](approvals-and-concurrency.md) APPR-WIRE-1). |

| ID | Gap | Detail |
|---|---|---|
| AUTO-GAP-1 | OperationIds not expanded | `/automations` operations are pattern-mandated, not literal, in the contract; the codegen/lint contract requires them to exist. IDs will follow the expanded-resource naming once emitted; cite, don't invent, until then. |
| AUTO-GAP-2 | MCP tool rows not annotated | The §12.5 note promises a first-class REST+MCP surface, and B-E15.6 specifies the four agent verbs, but no MCP tool annotations for automations exist in the contract yet — same D-H2 docs-layer drift lane as AUTO-GAP-1. |

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c (definitions live in the central catalog and the event-bus chapter; cited, not redefined)

| ID | Event / channel | Role in this chapter |
|---|---|---|
| AUTO-EV-1 | `person.* / organization.* / deal.* / lead.* / activity.*` created/updated streams | Consumed: the record created/updated and field-reaches-value triggers evaluate against the entity streams [[event-bus#EVT-STREAM-1]]–[[event-bus#EVT-STREAM-5]]. |
| AUTO-EV-2 | `deal.stage_changed` | Consumed: the deal enters/leaves-stage trigger; a specific verb, never a generic update ([[event-bus#EVT-SEM-2]]). |
| AUTO-EV-3 | `engagement.reply` | Consumed: the inbound-reply trigger; reply-based, idempotent per reply, never an open-pixel ([[event-bus#EVT-SEM-14]]; corpus events.md §5.11, E15 reply-tracking). |
| AUTO-EV-4 | `activity.captured` | Consumed: activity-derived matching for the firing runtime (build story B-E15.3 traces). |
| AUTO-EV-5 | `approval.requested` / `approval.decided` | Emitted/consumed around a 🟡 firing: the held action stages with its dry-run diff and executes only on the approved decision, sharing a correlation id with the resulting domain event ([[event-bus#EVT-SEM-9]]). |
| AUTO-EV-6 | Dispatch consumer group | Firings dispatch through the workflow consumer group `cg:workflows` ([[event-bus#EVT-CG-3]]): trigger→dispatch p95 < 200 ms (AC-W2), idempotent on the event id ([[event-bus#EVT-DEL-2]], AC-W3). |
| AUTO-EV-7 | Time-derived triggers consume no event | No-activity-for-N-days, date-field approaching, and task overdue fire from the River time-scan against an injected clock, not from the bus (B-E15.3b); task-overdue explicitly has no emitted event. |
| AUTO-EV-GAP-1 | No `automation.*` event id exists | The central catalog defines no automation-lifecycle or automation-fired event; a firing's visibility rides `automation_run` + the audit entry + the fired action's own domain event. If the build needs a first-class fired event, that is a catalog addition to reconcile at the docs layer, not an assumption. |

### Acceptance
Source: product/epics/E15-operational-depth.md @ 5a0b29c; features/10-operational-depth.md#1-user-centric-automation--bounded-catalog--agent-authored-standing-automations-a45adr-0035 @ 5a0b29c; features/03-reporting-and-scoring.md#5-workflows--automation @ 5a0b29c; product/30-screen-acceptance.md#automationshtml--automations-catalog--plain-language-implements-s-e151-s-e152 @ 5a0b29c

Story primacy verified against product/20-traceability.md @ 5a0b29c: S-E15.1
(V1-Must) and S-E15.2 (V1-WOW) are owned here; both are **single ticket
atoms** per the E15 atom note (stories with no child list — S-E15.1/.2 — are
already single atoms), and the scope chapter's epic map routes E15's
automation slice to this chapter.

| ID | Given/When/Then | Verification |
|---|---|---|
| S-E15.1 | Given the automation catalog, when I pick a template (e.g. "remind me if a deal I own has no activity for N days") and set the parameter, then it runs and I get the reminder — no code, no agent; an automation whose action sends or reassigns at scale is held in my approval inbox (🟡) while a draft/task/notification (🟢) runs automatically; when any automation fires I can see in the audit trail what triggered it and what it did, and I can pause or delete it anytime. | Ticket-coverage gate; integration lane ([[testing#TEST-LANE-2]]) + live-stack UAT ([[testing#TEST-LANE-3]]). |
| S-E15.2 | Given my connected agent, when I say "when a proposal's been out 5 days with no reply, draft a nudge and remind me," then a named standing automation appears in my automations list; its draft is created (🟢) and the send waits for my approval (🟡) — the agent can't exceed what I'm allowed to do; I can read, edit, pause, or delete it, every run is in the audit trail; an automation the agent proposes unasked is a 🟡 suggestion I accept or decline, never silently active. | Ticket-coverage gate; integration lane ([[testing#TEST-LANE-2]]) + live-stack UAT ([[testing#TEST-LANE-3]]). |
| AUTO-AC-1 | Every automation's actions carry an autonomy tier (ADR-0026): 🟢 (task/notify/draft/set-field) run automatically; 🟡 (send, mass reassign, close, archive) are **held in the approval inbox** (`features/05 §1`) — asserted by a tier-routing test. *(Verbatim, features/10 §1; see AUTO-NOTE-1.)* | Tier-routing integration test ([[testing#TEST-LANE-2]]); [[acceptance-standards#GATE-CORE-7]]. |
| AUTO-AC-2 | An automation **cannot** grant an action a tier exceeding the authoring human's RBAC (Passport intersection, `features/04 §1`); attempting it is rejected. *(Verbatim, features/10 §1.)* | Privilege-escalation integration test ([[testing#TEST-LANE-2]]) at author time (B-E15.5). |
| AUTO-AC-3 | Every firing writes one `audit_log` entry (automation id, trigger evidence, actor, action result) + the relevant domain event — "why did this fire?" is answerable by looking. *(Verbatim, features/10 §1.)* | Integration lane ([[testing#TEST-LANE-2]]); [[acceptance-standards#GATE-CORE-5]]; exactly one `automation_run` row per firing (B-E15.5). |
| AUTO-AC-4 | A standing automation is a first-class object reachable over the `crm-agents` MCP surface (create/list/pause/delete) under Passport scopes; an agent-authored automation round-trips identically to a catalog-authored one. *(Verbatim, features/10 §1.)* | Round-trip equivalence test ([[testing#TEST-LANE-2]]); [[acceptance-standards#GATE-CORE-7]]; wire surface pending AUTO-GAP-2. |
| AUTO-AC-5 | The trigger set is **closed**: a contract test asserts no API path accepts a user-defined trigger/action type or free-form rule body (the anti-DSL guard). *(Verbatim, features/10 §1; scope [[scope#NEVER-2]].)* | Anti-DSL contract test on both REST and MCP surfaces (B-E15.2); [[acceptance-standards#GATE-CORE-1]]. |
| AUTO-AC-6 | **User-observable (Sam, S-E15.1):** Sam turns on "remind me if a deal I own has no activity for 7 days," fills in the number, and gets the reminder — without writing anything or opening an agent. *(Verbatim, features/10 §1.)* | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AUTO-AC-7 | **User-observable (Sam, S-E15.2):** Sam tells his agent the nudge rule in his own words; it appears in his automations list, he can see and pause it, the draft waits for his approval before sending, and every run is in the audit trail. *(Verbatim, features/10 §1.)* | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-W1 | **(registry contract):** A handler scaffolded by `crm gen workflow` compiles, registers, declares trigger + risk tier, and passes its generated test stub — verified by the worked-example test (`04` probe alignment). *(Verbatim, features/03 §5.4.)* | Generator worked-example test ([[testing#TEST-LANE-1]]). |
| AC-W2 | **(trigger latency):** Event-bus trigger → handler dispatch p95 < 200 ms from event emit on the seed dataset. *(Verbatim, features/03 §5.4.)* | Performance budget on the seeded dataset ([[acceptance-standards#GATE-CORE-6]]); dispatch group [[event-bus#EVT-CG-3]]. |
| AC-W3 | **(idempotency & reliability):** Replaying the same trigger event does not double-apply actions (idempotency-key test); failed handlers retry per policy and dead-letter; no silent loss. *(Verbatim, features/03 §5.4.)* | Integration lane ([[testing#TEST-LANE-2]]); [[event-bus#EVT-DEL-2]]; at-most-once dedup (B-E15.3b). |
| AC-W4 | **(risk gating, P12):** A 🟡 action (send email / advance to closed-won / delete) does not execute without an approval record; auto-execute is restricted to 🟢 reversible actions; asserted by test. *(Verbatim, features/03 §5.4.)* | Integration lane ([[testing#TEST-LANE-2]]); approval machinery per [approvals-and-concurrency](approvals-and-concurrency.md). |
| AC-W5 | **(bulk safety):** A bulk action shows an accurate dry-run count matching ground truth and supports undo within the window; tested. *(Verbatim, features/03 §5.4 — the bulk-operation surface itself is owned by the records-depth/bulk chapter; pinned here because the corpus binds it to the workflow criteria set.)* | Integration lane ([[testing#TEST-LANE-2]]); owning chapter per the ownership index (`bulk_operation` → records-depth). |
| AC-W6 | **(audit/replay):** Every workflow run is audit-logged with trigger/inputs/actions/approval/outcome and is replayable; the trace reconstructs the run (test). *(Verbatim, features/03 §5.4.)* | Integration lane ([[testing#TEST-LANE-2]]); [[acceptance-standards#GATE-CORE-5]]. |
| AC-W7 | **(AI-suggested workflow):** A detected repeated manual sequence produces a *proposed scaffolded handler / PR*, never a silently-applied automation (test asserts proposal state, not execution). *(Verbatim, features/03 §5.4; the runtime-catalog analogue — L2 suggestions as 🟡 proposals — is deferred beyond V1 per the features/10 cut line.)* | Proposal-state test ([[testing#TEST-LANE-2]]); [[acceptance-standards#GATE-AI-2]]. |
| AC-automations-1 | Given the Automations screen, When it loads, Then a tier legend renders explaining 🟢 "Auto-runs" (reads/drafts/tasks/reminders fire on their own) and 🟡 "Held for approval" (anything that sends or reassigns waits in the approval inbox, linked to inbox.html). | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-automations-2 | Given the catalog ("6 templates · no code"), When I click "Turn on" on a template with parameters (e.g. No-activity reminder or Stage-change notify), Then an inline parameter form reveals (N-days input / scope select / stage select) with the action's tier stated, and opening one form closes any other open form. | Live-stack UAT ([[testing#TEST-LANE-3]]); template set pinned at AUTO-PARAM-5. |
| AC-automations-3 | Given an open catalog parameter form, When I confirm "Turn on", Then the form closes and a new active automation row is appended to "Your automations" stamped provenance "set by you" with its 🟢 tier badge and an "Active · not yet fired" status; clicking "Cancel" closes the form and adds nothing. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-automations-4 | Given the agent composer, When I type (or pick a suggestion chip) a plain-language rule and press send, Then the send button shows a loading spinner, the agent stages a "Proposed standing automation — review before it goes live / not yet active" card (it is never silently activated), and the card paraphrases the When/Scope/Then/Send rows with each step tagged 🟢 or 🟡. | Live-stack UAT ([[testing#TEST-LANE-3]]); [[acceptance-standards#GATE-AI-2]]. |
| AC-automations-5 | Given a staged agent proposal, When I click "Accept & activate", Then the proposal card hides and a new active row is prepended stamped provenance "agent:claude" with the 🟡 send step still held (it links the send to the approval inbox, never auto-sent); When I click "Decline", Then nothing is activated and a "Declined — nothing was activated" confirmation shows. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-automations-6 | Given an active automation row, When I click its toggle, Then it switches between On and Paused (a paused rule "won't fire until you resume it"); When I click Delete, Then I'm told future runs stop while past runs remain in the audit log. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-automations-7 | Given an active automation (e.g. Renewal reminder), When I click "View audit trail", Then a run log reveals showing per-run who/what/when entries reconstructed from audit_log, each stamped with the tier that fired and whether approval was needed. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-automations-8 | Given any submission, When I press send on an empty agent composer, Then I'm prompted to "describe the automation in your own words first" and no proposal is staged; and the day-count in the proposal is parsed from my words when present, else shown as "a configured window" (never fabricated). | Live-stack UAT ([[testing#TEST-LANE-3]]); evidence-or-omit per [[acceptance-standards#GATE-AI-1]]. |
| AUTO-NOTE-3 | Build note (pinned): the corpus screen section flags two prototype gaps the built surface must close — a genuine run-time failure/error state for an automation that errors or is blocked (the "Blocked / skipped: 0" stat implies the concept but no error-row UI is rendered), and a loading skeleton for the active list. The no-permission state (org-wide automations locked behind an admin role with "Ask an admin") and the bounded-scope honesty panel ("no drag-and-drop canvas, no branching builder, no custom code step") are part of the screen contract, not optional polish. | Ticket-coverage gate; screen-state floor [[acceptance-standards#STATE-1]]–[[acceptance-standards#STATE-4]]. |
