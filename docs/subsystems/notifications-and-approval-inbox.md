---
status: planned
module: backend/internal/modules/agents (inbox read/decide surface + notification fan-out consumers); web (inbox + notification surfaces)
derives-from:
  - specs/spec/features/05-notifications-and-collaboration.md#1-notifications--the-agent-approval-inbox
  - specs/spec/features/05-notifications-and-collaboration.md#2-real-time-updates-presence--concurrency-control
  - specs/spec/features/05-notifications-and-collaboration.md#3-delivery-infrastructure--reuse-dispacts-bus--websocket-p9
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e112--approval-inbox-for-every-outward-action
  - specs/spec/product/30-screen-acceptance.md#inboxhtml--approval-inbox-implements-s-e1123
  - specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26
---
# Notifications & approval inbox — every 🟡 action in the product resolves here, in the open

> The trust surface of the agent product: one queue where every confirm-first
> action waits — with the full context a human needs to approve, edit, or
> reject — plus the notification path that tells the right people, and only
> the right people, that something is waiting. Its promise: nothing outward or
> irreversible happens behind the user's back, and nobody is ever notified of
> an item they could not act on.

## What it's for

The corpus's load-bearing claim is that this surface is MVP, not fast-follow:
a 🟡 confirm-first action is *defined* as "waits for a human," and that
definition is vacuous unless there is a place where the human is told, given
the context to decide, and can approve, reject, or modify. The governance
model and the security backstop cash out here or not at all. Every staged
proposal in the product converges on this one queue: the overnight agent's
reconciliation proposals, task proposals inferred from meetings, drafted
sends, offer sends, over-threshold bulk operations, and any tool call a
connected agent reaches that crosses the confirm-first line. The same chapter
owns the notification path those items ride — the in-app notification center,
mentions, assignment alerts, and the delivery classes — because approval-pending
is the highest-urgency notification the product has. The boundary is explicit:
the staging, token, re-validation, and expiry *mechanics* belong to
[approvals-and-concurrency](approvals-and-concurrency.md); the general
version-concurrency wire rules belong to the api-conventions chapter; the
event envelope and delivery semantics belong to the event-bus chapter. This
chapter owns the inbox *surface* where humans decide, the collaboration-level
rules for humans and agents working the same record, and notification
delivery.

## Principles it serves

- **P12 — governance is designed in.** The inbox is the human-in-the-loop
  gate made real: a modify records the human's delta against the agent's
  proposal, an expiry fails closed to auto-reject, and every disposition is
  attributable. The inbox is not optional configuration — it is the
  structural counterpart of every 🟡 tier and ships on by default.
- **P6 — govern the user's own agent, don't replace it.** The surface exists
  so a human can supervise their agent's outward moves, not so the product
  can act in their name.
- **P5 / P1 — opinionated defaults over a rule builder.** Alert thresholds
  and delivery preferences are a small bounded set with sane defaults; there
  is no per-event notification-rule designer.
- **P9 — shared foundations, zero new infrastructure.** Notifications, live
  updates, and approval-state transitions are *consumers* of the platform
  event bus, the shared fan-out pattern, and the shared job queue. A new
  broker is explicitly rejected (see the event-bus and operations chapters).
- **P4 — fast where it matters.** The human latency of confirm-first is
  intrinsic, not a performance regression: the agent run backgrounds and
  resumes on decision, and one event mechanism feeds both conflict detection
  and live fan-out.

## How it works

**What lands here.** Any agent tool call the contract classifies 🟡
confirm-first — a send, a delete, an advance to closed-won, anything touching
money or a customer, any external-egress tool — plus actions escalated
dynamically even when normally free: a read-volume anomaly tripping a step-up
gate, or a content-aware egress block. Sibling subsystems stage into the same
queue: the [overnight-agent](overnight-agent.md)'s proposals, proposed tasks
from [tasks-and-work-queue](tasks-and-work-queue.md), sequence enrolments and
offer sends, and over-threshold bulk operations. There is one inbox; no
surface grows a private approval queue.

**The item carries the deciding context (normative).** Every pending item
renders: *who and under what authority* — which agent, which human's
passport, which scopes; *what it wants to do* — the tool call rendered as a
dry-run diff or full preview (recipients, subject, body for a send); *why it
was gated* — the rule that triggered the 🟡, and when the content-aware
egress rule fired, a prominent flag naming exactly which fields and records
are auto-captured, untrusted, and sensitive, so the human reviews for
injection or exfiltration before approving ([[threat-model#D3]]); the *trust
tiers* of every record the action touches; and a *replayable trace handle*
into the audit log for the run that produced it. An item missing any of these
is not a rendering shortfall — it is a broken contract.

**Three decisions, and modify is the point.** Approve commits exactly the
previewed effect; reject discards it with a reason and the agent run is
informed; modify lets the human edit the payload — fix a draft, narrow
recipients, strip a field — and then approve, with the human's edited version
being what executes and both the original proposal and the human delta
audit-logged. Modify is what makes the inbox a collaboration surface rather
than a binary gate. The mechanics under these decisions — exactly-once claim
between concurrent approvers, the single-use effect-bound token, re-validation
against the live record, and the rule that an edit re-enters the admission
gate and can never escalate past its approval — are the
[approvals-and-concurrency](approvals-and-concurrency.md) chapter's
(APPR-AC-4..7); this chapter owns their honest rendering: an already-decided
item, a stale item, and an expired item are each visibly distinct states.

**Unactioned means rejected, visibly.** An undecided item expires after its
time-to-live — 72 hours by default, owned by the approvals chapter
([[approvals-and-concurrency]] APPR-PARAM-1) — and expiry is an auto-reject,
never an auto-approve. The surface renders a live countdown on every pending
item, and expired items remain visible and recoverable: the user can see that
an item expired, that nothing was sent and nothing changed, and can re-open
it into the queue ([[acceptance-standards#STATE-SP-2]]).

**The surface's honest states.** Beyond the standard screen-state floor, the
inbox owes four named special states ([[acceptance-standards#STATE-SP-2]]): a
*read-only viewer* without approve scope sees the queue with decision
controls absent, not dead; a *failed downstream execution* — an approved
action whose send or downstream call bounced — is surfaced and re-queued,
never silently lost; a *batch item* supports per-row partial approve/reject;
and the TTL countdown is live. The first two are trust-critical: an approval
that quietly failed is indistinguishable from a lie.

**Notification delivery.** Approval-pending is the high-urgency class: a live
in-app notification plus an immediate email by default (NTFY-PARAM-1). The
table-stakes set ships with it: a notification center with unread count,
mentions that resolve against access control — a mention notifies but never
grants, and never includes a field the recipient cannot read — and assignment
alerts. SLA/idle alerts with manager escalation, the batched daily digest,
and per-class delivery preferences are fast-follow (NTFY-PARAM-2/3). The
fan-out is access-filtered server-side: a user never receives an event for a
record they may not read, and never sees an inbox item they lack the
authority to act on — approver eligibility follows the same agent-never-
exceeds-human logic the approvals chapter pins.

**Humans and agents on the same record.** The novel concurrency case is an
agent writing while a human edits. The substrate is the platform version rule
owned by api-conventions (API-CC-1..5); the collaboration doctrine layered on
it is this chapter's: agents are first-class optimistic-lock citizens — an
agent whose write loses the race must re-read, re-reason, and re-propose,
and must never blind-overwrite a human's concurrent edit. Overlapping edits
are never auto-merged: the human wins the round, and a 🟡 agent write that
conflicted re-proposes into this inbox with its diff computed against the
*current* record, so the human decides against fresh data, not stale.
Field-disjoint concurrent edits may auto-merge (fast-follow) only when
neither side touched a gated or sensitive field, and the merge is audit-logged
with both contributors. Presence — showing who is viewing or editing a
record, including agent actors named with their granting human and scope —
is fast-follow, and pre-empts surprise conflicts.

**Delivery rides what exists.** Approval-state transitions and notification
signals are consumed from the platform bus — transactional outbox, relay,
per-type streams, durable consumer groups — exactly as the event-bus chapter
defines them; a consumer that was offline catches up rather than losing an
approval signal. Live inbox and record updates follow the frontend chapter's
shared pattern: server push over the event bus invalidating the matching
query keys, never ad-hoc polling or a bespoke socket per surface. The
SLA/idle scanner and the digest batcher run as scheduled jobs on the shared
queue. No new broker, no bespoke change feed: the version-bump domain event
is the change feed, and any divergence would need an ADR.

## What's configurable

- **Approval item TTL** — owned by the approvals chapter; cited here, not
  re-pinned ([[approvals-and-concurrency]] APPR-PARAM-1: 72 hours default,
  per-item override at staging).
- **Approval-pending delivery class** — live in-app plus immediate email, on
  by default (NTFY-PARAM-1). Deliberately *not* user-disableable in V1: the
  inbox is the structural counterpart of the 🟡 tier, not a preference.
- **Per-class delivery preferences** — a bounded per-class choice among
  in-app, digest, immediate email, or off, with opinionated defaults
  (NTFY-PARAM-2, fast-follow). When built, these become runtime-config
  register rows ([[runtime-config#RC-REG-1]]); there is no rule builder.
- **Digest cadence** — one batched daily morning email (NTFY-PARAM-3,
  fast-follow); no per-user batching windows in V1.
- **The event transport** — an injected platform dependency. The server
  boots and serves without the broker; staged events publish when it
  returns (event-bus chapter). Without live push, the inbox degrades
  honestly to fetch-on-open — items are never lost, only less immediate.

## Guarantees (enforced)

- **Nothing commits behind the user's back.** A 🟡 action produces zero real
  mutations and zero transport calls before a disposition; silence becomes
  rejection, never consent (mechanics: [[approvals-and-concurrency]]
  APPR-AC-2; surface: NTFY-AC-1, NTFY-AC-5).
- **One inbox.** Every confirm-first action in the product — agent send,
  overnight proposal, task proposal, over-threshold bulk op — is decidable
  from this one queue; no sibling surface owns a second approval model
  (NTFY-AC-9, AC-inbox-14).
- **The deciding context is complete.** Every pending item carries who,
  under what authority, the rendered dry-run diff, why it was gated
  (including the content-egress flag when it applies), the trust tiers of
  touched records, and a trace handle (NTFY-AC-2, NTFY-AC-3).
- **Modify commits the human's version.** The edited payload is what
  executes, and both the original proposal and the human delta are
  audit-logged (NTFY-AC-4; mechanics APPR-AC-4/5).
- **Failed execution is never silent.** An approved action that bounces
  downstream is surfaced and re-queued, not lost
  ([[acceptance-standards#STATE-SP-2]]).
- **Fan-out is access-filtered.** No notification, digest, or pushed event
  contains a field its recipient cannot read; a subscriber never receives
  events for a record it may not see; a mention notifies without granting
  (NTFY-AC-6, NTFY-AC-7, NTFY-AC-20).
- **No agent ever silently clobbers a human.** An agent write that lost the
  version race produces no mutation and must re-propose; overlapping
  human-and-agent edits resolve human-wins, with the 🟡 re-proposal landing
  here against current state (NTFY-AC-13, NTFY-AC-18; substrate
  [[api-conventions#API-CC-4]]).
- **Approval signals are durable.** An offline-then-reconnecting consumer
  receives the approval-state change via its consumer group, not a gap
  (NTFY-AC-21; delivery semantics [[event-bus#EVT-DEL-1]]).

## Acceptance

Done means: a 🟡 action proposed anywhere in the product appears in this
queue with its full deciding context and is approvable, editable-then-
approvable, or rejectable in one tap — including from a phone — and nothing
sends or commits until then; ignoring it expires it visibly to auto-reject;
deciding it leaves a human-attributable trail on the record; and the people
who should know are told immediately while the people who lack access never
see it. The honest states are part of the contract: read-only viewer, failed
downstream execution, per-row partial batch, live countdown, empty queue, and
expired-but-recoverable each render as real states, per the floor and
special-state rows inherited from the acceptance-standards chapter
(STATE-1..5, [[acceptance-standards#STATE-SP-2]]). The testable form of every
claim lives in the Acceptance appendix, alongside the two open build
decisions the corpus flags for this surface (NTFY-NOTE-1/2).

## Out of scope

- **Token, staging, re-validation, expiry mechanics** — the approvals seam:
  [approvals-and-concurrency](approvals-and-concurrency.md) (APPR-PARAM-*,
  APPR-WIRE-*, APPR-AC-*).
- **The version-concurrency wire contract** — header shape, precondition
  behavior, conflict body: api-conventions ([[api-conventions#API-CC-1..7]]).
- **Event envelope, catalog, and delivery semantics** — the event-bus
  chapter; stream hygiene and retention numbers — operations
  ([[operations#OPS-QUEUE]]).
- **Tier policy** — which tools resolve 🟡, and the non-negotiable always-🟡
  floor: [byo-agent-and-mcp](byo-agent-and-mcp.md) and
  [[threat-model#D4]].
- **The content-aware egress rule itself** — [[threat-model#D3]]; this
  chapter owns only its surfacing as a flagged decision.
- **The audit-log view and replayable-trace UI** — the audit answer "who
  signed off on what" renders on the record via
  [audit-observability](audit-observability.md); the inbox shows audit
  handles, not the trail.
- **Proposal semantics of specific producers** — what the overnight agent
  stages ([overnight-agent](overnight-agent.md)), how a task proposal is
  born ([tasks-and-work-queue](tasks-and-work-queue.md)).
- **Deferred by the corpus cut lines:** per-event notification-rule builder;
  native mobile push transport (responsive web plus email covers MVP);
  Slack/Teams delivery (a connector concern); quiet hours beyond the single
  digest cadence; approval-delegation chains; character-level co-editing
  (record-level optimistic locking is the deliberate granularity).

## Where it lives

Planned backend home: the inbox read/decide surface and the notification
fan-out consumers inside `backend/internal/modules/agents`, riding the
platform events layer and the shared job queue; planned frontend home: the
inbox and notification surfaces in `web`. Read next:
[approvals-and-concurrency](approvals-and-concurrency.md) for the mechanics
under every decision, the api-conventions chapter for the concurrency wire
contract, the event-bus chapter for how signals move, and
[overnight-agent](overnight-agent.md) and
[tasks-and-work-queue](tasks-and-work-queue.md) for what lands here.

## Appendix

### Parameters
Source: features/05-notifications-and-collaboration.md#1-notifications--the-agent-approval-inbox @ 5a0b29c; features/05-notifications-and-collaboration.md#3-delivery-infrastructure--reuse-dispacts-bus--websocket-p9 @ 5a0b29c

The approval item TTL (72h fail-closed, per-item override) and the approval
token TTL are single-homed in the approvals chapter —
[[approvals-and-concurrency]] APPR-PARAM-1 and APPR-PARAM-2 — and are cited,
not re-pinned, here.

| ID | Name | Value | Meaning |
|---|---|---|---|
| NTFY-PARAM-1 | Approval-pending delivery class | live in-app + email-immediate, on by default | The high-urgency notification class; delivered live over the shared push path and as an immediate email. Not a user preference in V1 — the inbox ships on by default (P12). |
| NTFY-PARAM-2 | Per-class delivery preference set | in-app / email-digest / email-immediate / off (fast-follow) | The whole preference vocabulary — a bounded per-class choice with opinionated defaults, explicitly not a per-event rule builder (P1/P5). Becomes a runtime-config register row when built ([[runtime-config#RC-REG-1]]). |
| NTFY-PARAM-3 | Digest cadence | daily morning batch (fast-follow) | One batched email digest; no per-user batching windows in V1. |
| NTFY-GAP-1 | Live-update latency budget | **unpinned** | features/05 §2/§3 defer to a "live-update budget … ratified in review"; the acceptance-standards PERF table ([[acceptance-standards#PERF-1]]..PERF-7, PERF-R1..R10) has no realtime-push row. A perceived-latency budget for live updates must be ratified and pinned before NTFY-AC-15/17 are verifiable. |

### Schema
Source: contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c

Ownership per the data-model chapter's index
([[data-model#schema--ownership-index]]): `approval_item` is pinned here. The
corpus DDL (base columns per [[data-model#DM-CONV-3]], RLS per
[[data-model#DM-CONV-5]]..8, and the version column per
[[data-model#DM-CONV-4]] apply and are not restated):

```sql
CREATE TABLE approval_item (                             -- the 🟡 approval-inbox item (B-EP07.6/.6a/.6b; ADR-0026/0036)
  action_type     text NOT NULL,                         -- the staged mutating/sending action
  payload         jsonb NOT NULL,                        -- the proposed action, replayable on approve
  dry_run_preview jsonb NULL,                            -- the diff/preview shown to the approver (B-E07.7)
  status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','modified','expired')),
  requested_by    uuid NULL REFERENCES app_user(id),
  passport_id     uuid NULL REFERENCES passport(id),     -- set when an agent staged the action
  decided_by      uuid NULL REFERENCES app_user(id),
  decided_at      timestamptz NULL,
  expires_at      timestamptz NOT NULL                   -- fail-closed auto-reject (B-EP07.9, 72h default)
);
CREATE INDEX idx_approval_pending ON approval_item (workspace_id, status, expires_at);
-- This is the EP07 inbox the EP11 craftsmanship CRAFT-DISPUTE queue migrates onto (see EP11 deviation notes).
```

| ID | Gap | Detail |
|---|---|---|
| NTFY-GAP-2 | No notification persistence | The corpus data model defines **no** notification, notification-read-state, or delivery-preference tables — the notification center's unread count, per-class preferences (NTFY-PARAM-2), and digest state have no schema home. Contract-extension / schema work for the build (same D-H2 docs-layer lane as the sibling chapters' gaps). |

### Wire
Source: contract/crm.yaml (Approvals tag) @ 5a0b29c

The approvals read/decide operations are owned by the approvals chapter —
cited with the owner's IDs, not re-pinned. The rows below add only this
chapter's surface role.

| ID | Operation (operationId) | Role in this chapter |
|---|---|---|
| NTFY-WIRE-1 | `listApprovals`, `getApproval` ([[approvals-and-concurrency]] APPR-WIRE-3) | The inbox read: the queue (filterable by status and proposal kind) and the single-item view with full proposed diff and evidence. This chapter's UI renders them with the normative deciding context (NTFY-AC-2). |
| NTFY-WIRE-2 | `approveApproval` ([[approvals-and-concurrency]] APPR-WIRE-4) | Approve and edit-then-approve: there is no separate modify operation — modify is approve with the optional `edited_payload`, and the edited effect is what commits (NTFY-AC-4). Already-decided answers with the conflict semantics the surface must render as a distinct state. |
| NTFY-WIRE-3 | `rejectApproval` ([[approvals-and-concurrency]] APPR-WIRE-5) | Reject with optional reason; nothing commits; the agent run is informed (NTFY-AC-4). |

Honest coverage gaps (contract extensions the build must resolve in the
docs layer — the contract ships complete, so these are drift to reconcile,
not silent additions):

| ID | Gap | Detail |
|---|---|---|
| NTFY-GAP-3 | No notification-center operations | The contract pins no list-notifications, mark-read, unread-count, or delivery-preference operations; the notification center and NTFY-PARAM-2 preferences have no wire surface (pairs with NTFY-GAP-2). |
| NTFY-GAP-4 | No push/subscription or presence surface | The live-update push path is an internal seam (server push invalidating query keys, per the frontend chapter) with no contract operation — acceptable — but presence (fast-follow, NTFY-AC-16) has no wire or ephemeral-store surface specified anywhere. |
| NTFY-GAP-5 | Status-filter enum drift | `listApprovals` filters `status` over `pending, approved, rejected` only, while the `approval_item` DDL and the `Approval` schema's expiry description include `expired` (and the DDL `modified`) — the expired-but-recoverable list (AC-inbox-12/13) is not queryable through the pinned filter. Reconcile enum vs DDL. |

### Events
Source: contract/events.md#56-approval @ 5a0b29c; central catalog in the event-bus chapter

Definitions live in the central catalog (the event-bus chapter defines only
the approval *events*; this chapter owns their user surface). Delivery
semantics are the event-bus chapter's ([[event-bus#EVT-DEL-1]]..7); stream
hygiene is operations' ([[operations#OPS-QUEUE]]).

| ID | Event | Role in this chapter |
|---|---|---|
| NTFY-EV-1 | `approval.requested` | Consumed by the read-model consumer group to light up the inbox and by the notification path to deliver the approval-pending class (NTFY-PARAM-1). Carries the staged effect, dry-run diff, and expiry the surface renders. |
| NTFY-EV-2 | `approval.decided` | Consumed to resolve the card (approved / rejected / expired), decrement the pending count, and notify the agent run; `expired` is the fail-closed auto-reject rendered by AC-inbox-12. |

| ID | Gap | Detail |
|---|---|---|
| NTFY-GAP-6 | No notification-class events | The central catalog defines no mention, assignment, SLA/idle-alert, or digest events — the table-stakes notification classes of features/05 §1 have no catalog IDs. The reminder-due gap is already owned by the tasks chapter (TASK-GAP-4); the remaining classes need catalog entries when the notification center is built. |

### Acceptance
Source: features/05-notifications-and-collaboration.md#acceptance-criteria @ 5a0b29c (§1, §2, §3 acceptance sections); product/epics/E11-access-trust-exit.md#s-e112--approval-inbox-for-every-outward-action @ 5a0b29c; product/30-screen-acceptance.md#inboxhtml--approval-inbox-implements-s-e1123 @ 5a0b29c

Story primacy verified against product/20-traceability.md @ 5a0b29c: S-E11.2
(V1-Must, features/05 §1) is owned here; the scope chapter maps epic E11
across five owning chapters with the approval-inbox surface to this one.
S-E11.3 (audit view) belongs to
[audit-observability](audit-observability.md); its two user-observable
bullets in features/05 are restated below (tagged) because they bind this
surface's behavior. The corpus §1/§2/§3 acceptance criteria are pinned
verbatim (GWT cell = the corpus text; its trailing verification sentence
moved to the Verification cell); rows whose mechanics are owned elsewhere
carry the owner's ID tag.

| ID | Given/When/Then | Verification |
|---|---|---|
| S-E11.2 | Given an agent invokes a 🟡 tool (send email, advance-to-closed, delete, merge, marketing send), when it does, then the action is held — not executed — and appears in my approval inbox with the dry-run diff, the originating agent + human authority, and the inputs that produced it; approving executes exactly that action, edit-then-approve executes *my* version, reject executes nothing and informs the agent; from a phone I can review the rendered diff and approve/reject; every disposition records who-decided-what-and-when in the audit log, and an unactioned item never silently auto-fires. | Ticket-coverage gate; integration lane ([[testing#TEST-LANE-2]]) + live-stack UAT ([[testing#TEST-LANE-3]]); mechanics [[approvals-and-concurrency]] APPR-AC-1..7. |
| NTFY-AC-1 | `[MV]` A 🟡-tier tool call (per the `crm.yaml` contract risk tier) does **not** commit until an approval item is approved; the action's side effects are absent from the DB and `audit_log` shows state `pending_approval` until decided. | Integration test in `crm-agents` ([[testing#TEST-LANE-2]]); hold engine owned by [[approvals-and-concurrency]]. |
| NTFY-AC-2 | `[MV]` An approval item's stored payload contains: agent id, granting-human/Passport id, scopes, the tool call + arguments, a rendered dry-run diff, the trigger reason, and the trust tiers of touched records. | Response-shape/contract test on the approval object ([[testing#TEST-LANE-2]]). |
| NTFY-AC-3 | `[MV]` A `send_email`/external-egress action whose body includes a **T2 + sensitivity-labeled** field read in the same session produces an approval item flagged `content_egress_review` (the D3 block surfaced). | Injection red-team probe; the gated send must appear here, never silently send ([[threat-model#D3]]; [[acceptance-standards#GATE-AI-7]] lane). |
| NTFY-AC-4 | `[MV]` **Reject** discards the action (no side effects) and emits an audit entry with `decision=reject`, approver id, and reason. **Modify** commits the *human-edited* payload and audit-logs both the original agent proposal and the human delta. **Approve** commits and logs `decision=approve` + approver id. | Integration test, all three branches ([[testing#TEST-LANE-2]]); commit mechanics [[approvals-and-concurrency]] APPR-AC-4/5. |
| NTFY-AC-5 | `[MV]` An approval item unactioned past its TTL (default 72h, [[approvals-and-concurrency]] APPR-PARAM-1) transitions to `expired` → auto-reject (never auto-approve); logged. | Time-advanced test; expiry engine owned as [[approvals-and-concurrency]] APPR-AC-2; this surface renders it (AC-inbox-12/13). |
| NTFY-AC-6 | `[MV]` An @mention of a user who lacks read access to the record notifies the user **without** including any field they cannot read, and does not grant access. | RBAC notification-shape test ([[testing#TEST-LANE-2]]). |
| NTFY-AC-7 | `[MV]` A notification/digest payload contains no field the recipient is unauthorized to read (same field-masking path as the RBAC feature). | Response-shape test ([[testing#TEST-LANE-2]]). |
| NTFY-AC-8 | Approval-pending delivers as a live in-app notification within the §2 live-update budget and as an immediate email. | Bus + push integration test; budget currently unpinned (NTFY-GAP-1). |
| NTFY-AC-9 | **User-observable (Mor/Sam, S-E11.2):** a 🟡 action waits in the approval inbox with the full context to decide — who proposed it and under whose authority, the exact rendered diff/email body and recipients, and *why* it was gated — and the human can **approve, edit-then-approve, or reject** in one tap from that screen (including on a phone). Nothing ever sends or commits behind the user's back: if they ignore it, it expires to auto-reject, never auto-approve, and the user can see that it expired. | Live-stack UAT ([[testing#TEST-LANE-3]]); mobile path per build story B-E11.6 (390 px responsive, same disposition operations as desktop). |
| NTFY-AC-10 | **User-observable (Mor, S-E11.3):** after deciding, the record's history shows the decision attributed to the human who made it — approve / reject (with reason) / the human's edit delta against what the agent proposed — so the audit answer "did a person sign off, and what exactly did they sign off on?" is visible on the record, not reconstructed. | Live-stack UAT ([[testing#TEST-LANE-3]]); the history view is owned by [audit-observability](audit-observability.md) — this row binds the inbox's write side. |
| NTFY-AC-11 | `[MV]` A `PATCH` with a stale `If-Match` returns `409`, the record is unchanged, and the response body contains the current version + state. | Concurrency integration test (two writers); owned mechanics [[api-conventions#API-CC-4]] — sanctioned restatement. |
| NTFY-AC-12 | `[MV]` A write to the UI/MCP path **without** `If-Match` is rejected (`428`), never applied unconditionally. | Contract test; owned mechanics [[api-conventions#API-CC-3]] — sanctioned restatement. |
| NTFY-AC-13 | `[MV]` An agent `update_record` with a stale version `409`s and produces no mutation; the audit trail shows the failed conditional write and the agent's subsequent re-read. | `crm-agents` test ([[testing#TEST-LANE-2]]) — the agents-are-lock-citizens rule, owned here at the collaboration level. |
| NTFY-AC-14 | `[MV]` A field-disjoint concurrent human+agent edit (when enabled) auto-merges, bumps version once, and audit-logs both contributors and the merged field set. | Merge test ([[testing#TEST-LANE-2]]); fast-follow — never on 🟡/sensitive fields. |
| NTFY-AC-15 | `[MV]` A committed mutation bumps `version` and emits exactly one domain event on `gw:events`; a subscribed client receives the live update within the live-update budget. | Bus + push integration test; emission owned by [[event-bus#EVT-DEL-5]]/EVT-SEM-1; delivery to the subscribed client owned here; budget unpinned (NTFY-GAP-1). |
| NTFY-AC-16 | `[MV]` Presence entries expire (TTL) after a client disconnects; a stale viewer does not linger. | Time-advanced presence test; fast-follow; no specified store/wire surface yet (NTFY-GAP-4). |
| NTFY-AC-17 | Live record update perceived latency p95 within the realtime budget (a UI responsiveness budget, ratified in review). | Perf test — blocked on NTFY-GAP-1 (no pinned budget). |
| NTFY-AC-18 | **User-observable (Sam/Mor, S-E11.3):** when an agent (or another person) changes a record Sam is looking at, the change appears in his open view without a manual refresh, and he can see that the agent is working there — shown as "Agent X (acting for Y) is working here" — so a conflict is never a surprise. If Sam and his agent edit at the same time, his edit is never silently overwritten by the agent: the agent is forced to re-read and re-propose against what Sam actually saved (and for a 🟡 edit, that re-proposal lands in the approval inbox against the current state). | Live-stack UAT ([[testing#TEST-LANE-3]]); presence sentence is fast-follow, the no-silent-overwrite + re-propose-into-inbox sentences are MVP (NTFY-AC-13). |
| NTFY-AC-19 | `[MV]` A core-object mutation emits a domain event on `gw:events` within the same logical unit as the write (no lost-update window between commit and emit). | Bus integration test; owned mechanics [[event-bus#EVT-DEL-5]] (transactional outbox) — sanctioned restatement. |
| NTFY-AC-20 | `[MV]` A WS client subscribed to a record it **cannot** read (RBAC) receives **no** events for it; a client that can, does. | Authorization fan-out test (reuses the RBAC matrix, [[testing#TEST-LANE-2]]) — the access-filtered fan-out is owned here. |
| NTFY-AC-21 | `[MV]` An approval-state-change event is delivered to an offline-then-reconnecting consumer via the consumer group (not dropped). | Durability test with simulated disconnect; delivery semantics [[event-bus#EVT-DEL-1]]/EVT-DEL-4 — this row binds the approval/notification classes to them. |
| NTFY-AC-22 | `[MV]` The SLA/idle scanner runs as a scheduled River job and emits alert events for records past threshold against a seeded fixture. | Job test ([[testing#TEST-LANE-2]]); fast-follow; alert events need catalog entries (NTFY-GAP-6). |
| AC-inbox-1 | Given the screen loads, When the header renders, Then it shows "Approval inbox", a count "N actions await your approval," and a banner stating nothing was sent to a customer, each Approve consumes a single-use token, every decision is logged, anything unapproved auto-rejects within 72h, and outbound is screened for injection & exfiltration before send. | Live-stack UAT ([[testing#TEST-LANE-3]]); token facts owned by [[approvals-and-concurrency]] APPR-PARAM-1/3. |
| AC-inbox-2 | Given three tabs (Needs approval / Activity / Done), When I click a tab, Then only that pane shows and the tab is active; "Needs approval" shows a live count badge. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-inbox-3 | Given a pending approval card, When it renders, Then it shows a title, "Requested by agent:<name>" with an agent glyph, a mono request/token id, a TTL flag "auto-rejects in Nh", a Scope line linking to the record(s), a payload preview, an evidence block (quote + source), and Approve / Edit / Reject controls. | Live-stack UAT ([[testing#TEST-LANE-3]]); the normative context set is NTFY-AC-2. |
| AC-inbox-4 | Given an email approval card, When the payload renders, Then it shows a "Drafted email · not sent" preview (To, subject, body) and a foot note "Cannot be un-sent · audit-logged · auto-rejects in Nh". | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-inbox-5 | Given a batch approval card, When the payload renders, Then it shows "Staged values" with one row per affected record (name, proposed value, per-row provenance source) and a staged-count flag. | Live-stack UAT ([[testing#TEST-LANE-3]]); per-row disposition is an open decision (NTFY-NOTE-2). |
| AC-inbox-6 | Given an outbound (egress) email approval, When it renders, Then it carries an "egress review · checked for injection/exfiltration" flag; reversible record-change approvals carry "Reversible · audit-logged" instead of "Cannot be un-sent." | Live-stack UAT ([[testing#TEST-LANE-3]]); flag semantics NTFY-AC-3 / [[threat-model#D3]]. |
| AC-inbox-7 | Given a pending card, When I click Approve, Then the card resolves (dimmed), controls are replaced by "Approved · token consumed · audit entry written," a toast fires, and the count decrements. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-inbox-8 | Given a pending card, When I click Reject, Then it resolves to "Rejected · agent will learn · audit entry written," a toast fires, the count decrements; nothing is executed. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-inbox-9 | Given a pending card, When I click Edit, Then a toast "Editing draft before approval — still 🟡, nothing sent" fires; the item stays 🟡 until re-approved (the edited version is what executes). | Live-stack UAT ([[testing#TEST-LANE-3]]); the real editor is an open build decision (NTFY-NOTE-1); edits-never-escalate is [[approvals-and-concurrency]] APPR-AC-5. |
| AC-inbox-10 | Given the Activity tab, When it renders, Then it lists informational captured/linked events, each with an icon, text, mono timestamp, agent/connector provenance, and a deep-link chip — none require approval. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-inbox-11 | Given the Done tab, When it renders, Then it lists 🟢 auto-executed items each with timestamp, audit id, a "reversible" marker where applicable, and "🟢 no approval needed"; previously approved 🟡 items show an "approved" badge. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-inbox-12 | Given an item exceeded its 72h TTL, When the expired section renders, Then it appears under "⌛ expired · auto-rejected at 72h TTL · recoverable · no deal changed," states the action was never sent and the deal not changed, shows an "expired · not run" tag + audit id, and offers "Handle now." | Live-stack UAT ([[testing#TEST-LANE-3]]); expiry mechanics [[approvals-and-concurrency]] APPR-AC-2; list queryability blocked by NTFY-GAP-5. |
| AC-inbox-13 | Given an expired item, When I click "Handle now," Then a toast "Re-opened — back in your approval queue" fires and it leaves the expired list. | Live-stack UAT ([[testing#TEST-LANE-3]]); re-staging semantics per [[approvals-and-concurrency]] (a fresh pending item, not a resurrected token). |
| AC-inbox-14 | Given the footer, When it renders, Then it states the two-tier rule (🟢 auto → Done; 🟡 outward/irreversible waits here for an explicit token) and that an unapproved 🟡 auto-rejecting at 72h never changes the deal and stays recoverable. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| NTFY-NOTE-1 | Build note (pinned open decision): the prototype's Edit flow is only a toast — **the editor behind Modify is undefined** (email vs batch vs stage-change editors). The corpus's cross-cutting roll-up makes this the first design obligation: "The inbox is the canonical 🟡 surface — its editor + per-row approve/reject must be designed first; other screens reference it" (30-screen-acceptance §3 gap 3 + the inbox screen's open questions, @ 5a0b29c). | Ticket-coverage gate; must land before any sibling surface that defers its 🟡 editing here (drafting, settings, voice). |
| NTFY-NOTE-2 | Build note (pinned open decision): **per-row partial-batch approve/reject is implied but has no per-row controls** in the prototype (batch cards render rows, dispositions are whole-item). [[acceptance-standards#STATE-SP-2]] makes the per-row partial-batch state mandatory; the pinned wire surface is item-granular (NTFY-WIRE-2/3), so the build must define per-row disposition semantics (e.g. edit-then-approve trimming rows vs a per-row operation) — undefined today (30-screen-acceptance §3 gap 3 + inbox open questions, @ 5a0b29c). | Ticket-coverage gate; UI + contract decision, same D-H2 docs-layer lane as NTFY-GAP-3/5. |
