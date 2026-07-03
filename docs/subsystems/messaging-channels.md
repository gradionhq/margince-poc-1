---
status: planned
module: backend/internal/modules/capture
derives-from:
  - margince specs/spec/features/02-capture-and-comms.md#feature-7--messaging-capture-whatsapp--telegram-adr-0022
  - margince specs/spec/features/02-capture-and-comms.md#cross-cutting-acceptance-criteria-whole-surface
  - margince specs/spec/decisions/ADR-0022-capture-build-borrow-boundary.md
---
# Messaging channels — WhatsApp & Telegram on the record, honest about who is in the path

> The channel layer that puts WhatsApp and Telegram conversations on the CRM
> record: business-API capture and reply on the customer's own credentials, no
> second vendor in the path — and the one place the sovereign story names its
> exception out loud instead of papering over it.

## What it's for

For the DACH mid-market, deals move over WhatsApp and Telegram as much as over
email — and those conversations vanish the moment they happen anywhere but the
rep's phone. This chapter gives the record the channel: messages on a connected
business account land on the right person and deal timeline automatically, and
a reply can be sent from the record. Its callers are the capture pipeline
(writes the messages in), the timeline surfaces (render them), and agents
through the governed tools (may propose a reply). The boundary is sharp: this
chapter owns the channel model and its sovereignty posture — which networks
connect, on whose credentials, and what a sovereign deployment may honestly
claim. The connector seam and single-writer doctrine belong to [[capture]];
the rows the messages become belong to [[activities-and-timeline]].

## Principles it serves

- **P5 — auto-capture over manual entry.** A messaging thread is exactly the
  history reps never type up; capture extends the zero-typing timeline to the
  channels where DACH deals actually happen.
- **P7 — own your data.** Transport stays inside the customer's trust boundary
  on the customer's own credentials, the system-of-record copy in the
  customer's own database; no aggregator ever proxies the messages
  ([[scope#NEVER-9]]).
- **P12 — governance is designed in.** Captured messages carry provenance and
  a trust label; an outbound reply is a gated action.
- **ADR-0022 — capture build/borrow boundary.** Build the capture semantics,
  borrow only in-boundary transport, reject cloud-proxy aggregators — and
  state plainly that WhatsApp cannot be made zero-egress.

## How it works

**Business-API on the customer's credentials — the in-boundary rule restated
for messaging.** Messaging differs from email in one intrinsic way: the
network operator is unavoidably in the path — the network's nature, not a
Margince choice. So the transport rule becomes: customer's own channel
credentials, the receiving client or webhook inside the customer's boundary,
the system-of-record copy in the customer's database, and **no second vendor**
stacked on top of the operator (ADR-0022; [[scope#NEVER-9]]). Telegram
satisfies it fully — a self-hosted client inside the boundary speaks to the
network directly. WhatsApp cannot: its on-premises option is sunset, the cloud
API runs on the operator's infrastructure, and the best achievable posture is
the customer's own business account with in-region residency at rest and
delivery to a customer-hosted endpoint.

**The sovereign exception is named, never hidden.** The sovereign profile's
zero-egress guarantee excludes WhatsApp by name (ACX.6): the operator is
always in the path, so the channel is never marketed as in-boundary. A
sovereign workspace may still enable it — but as an explicit, disclosed
acceptance of a named egress at the moment of enablement, never a silent
default (MSG-AC-1). Telegram and email/calendar keep the full guarantee.

**Captured messages become timeline activities.** A message rides the same
capture pipeline as email: normalized into one activity row of a messaging
kind — the two kinds ADR-0022 admitted alongside the five core kinds, pinned
in the timeline chapter's schema ([[activities-and-timeline]] ACT-DDL-1) —
linked to the matching person and deal, provenance-stamped, and held
idempotent by the capture key. New participants are auto-created and linked
under the same dedupe and never-auto-merge safety email uses; the mechanics
are [[capture]]'s. Captured content is external-origin: it enters
trust-labeled as captured tier and defaults to the connecting user's
visibility scope, under the capture-governance control the threat model owns
([[threat-model#D8]] — the auto-capture writer is a named, audited
system-service exception; a poisoned auto-created record is quarantined,
never instantly trusted).

**Consent travels with the channel.** A channel message is captured personal
communication, so the consent substrate applies unchanged: checks are
per-purpose and default-deny ([[gdpr-platform]] GDPR-AC-1, GDPR-AC-2), and the
suppression guard means an erased identity cannot be resurrected by a new
message from that sender (enforced in [[capture]]).

**Replying from the record is a gated send.** An outbound message proposed
through the agent or tool path resolves to the confirm-first tier and
dispatches nothing without a recorded human approval
([[acceptance-standards#GATE-AI-7]]; staging and token mechanics belong to
[[approvals-and-concurrency]]). Capturing inbound is free; sending is gated —
the same class as sending email.

**The cut line.** V1 ships Telegram capture on a self-hosted client. WhatsApp
capture on the customer's business account, and outbound send on either
channel, are the named table-stakes fast-follow; signal extraction over
message threads belongs to the AI-native surface. The feature ships on this
cut line, not on a dedicated story — the traceability matrix carries no
messaging story id — so the feature acceptance series below is the backbone.

## What's configurable

- **Channel enablement per workspace** — connecting or disconnecting an
  account is admin configuration; channel credentials live in the connector
  secret store the data-model chapter owns, never in code or model context.
- **Sovereign-profile interaction** — Telegram connects normally; WhatsApp
  requires the disclosed opt-in (MSG-AC-1). No knob makes WhatsApp zero-egress.

## Guarantees (enforced)

- **No second vendor in the path.** A dedicated or sovereign deployment's
  messaging capture shows zero third-party egress beyond the network operator
  itself (AC7.4; [[scope#NEVER-9]]).
- **No duplicate history.** Re-running capture creates no duplicate activities
  — held by the capture key the timeline chapter owns (AC7.2;
  [[activities-and-timeline]] ACT-DDL-1).
- **Nothing anonymous.** Every captured message row carries non-null,
  connector-attributed provenance (AC7.1; [[data-model#DM-CONV-11]]).
- **No unconfirmed send.** An outbound message via the agent path without an
  approval token dispatches nothing (AC7.3;
  [[acceptance-standards#GATE-AI-7]]).
- **The sovereign claim stays honest.** WhatsApp is excluded from the
  zero-egress guarantee by name; enabling it under the sovereign profile is a
  disclosed, recorded choice (ACX.6; MSG-AC-1).

## Acceptance

Done means a rep on a connected channel watches the conversation appear on the
right person and deal within the capture latency budget, tagged with where it
came from; replaying capture changes nothing; an unapproved reply from the
record sends nothing; and a sovereign operator can verify — by test, not
prose — that the only messaging egress is the network operator itself, with
WhatsApp's exception disclosed wherever it is enabled. Testable forms are in
the Acceptance appendix; the floor is inherited from [[acceptance-standards]].

## Out of scope

- **The connector seam and single capture writer** — [[capture]]'s doctrine;
  this chapter states only what the channel must obey.
- **The activity substrate and timeline read** — [[activities-and-timeline]]'s.
- **Rendering** — messages appear in timeline and composer surfaces owned by
  the record-screen chapters; this chapter owns no screens.
- **Signal extraction from threads** (commitments, sentiment) — the AI-native
  chapters, sharing the capture extraction pipeline.
- **Outbound cadences** — [[sequences-and-deliverability]].

## Where it lives

The messaging connectors of the backend's capture module, behind the connector
seam [[capture]] owns, writing activity rows into the people module's timeline
slice; no tables, screens, or dedicated stories of its own. Read next:
[[capture]], [[activities-and-timeline]], [[approvals-and-concurrency]].

## Appendix

### Wire
Source: contract/crm.yaml (Activities tag); features/02-capture-and-comms.md#feature-7--messaging-capture-whatsapp--telegram-adr-0022 @ 5a0b29c

**Contract gap, reported honestly:** the corpus contract defines **no
messaging-specific operations** — no channel connect/disconnect, no outbound
send. The activity kind vocabulary already admits `whatsapp` and `telegram`;
captured messages surface through the activity operations owned by
[[activities-and-timeline]] (ACT-WIRE-1 `listActivities`, ACT-WIRE-3
`getActivity`) — cited, nothing re-pinned here.

Note MSG-WIRE-N-1 (gap): channel connect and outbound send are the Feature 7
fast-follow (MVP cut line); shipping them extends the contract first — the
send operation must land as a 🟡 tool under [[acceptance-standards#GATE-AI-7]]
with the `X-Approval-Token` admission rule ([[approvals-and-concurrency]]
APPR-WIRE-1). Until then this chapter owns no wire surface.

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c

No messaging-specific event types exist; captured messages ride the capture
correlation chain and the activity spine — cited, not redefined.

| ID | Event | Definition |
|---|---|---|
| MSG-EVT-1 | `activity.captured` | Emitted once per normalized captured message (kind `whatsapp`/`telegram`), idempotent on the capture key; payload and consumers per the [[event-bus]] catalog (Activity). |
| MSG-EVT-2 | `capture.received` → `capture.normalized` → `activity.captured` | One correlation chain per captured message, [[event-bus#EVT-SEM-10]]. |

### Acceptance
Source: features/02-capture-and-comms.md#feature-7--messaging-capture-whatsapp--telegram-adr-0022; #cross-cutting-acceptance-criteria-whole-surface @ 5a0b29c

AC7.1–AC7.4 and ACX.6 are the corpus criteria, wording verbatim. MSG-AC-1 pins
the enablement-disclosure rule the prose states, which no corpus AC carries.

| ID | Given/When/Then | Verification |
|---|---|---|
| AC7.1 | An inbound Telegram message from a known `person` produces an `activity(type=telegram)` on that person's timeline within **60 s p95**, with non-null `source` + `captured_by`. | integration test against a self-hosted client |
| AC7.2 | Re-running capture over the same window creates **no duplicate** activities (idempotent on message-id, shares AC1.4's invariant). | test (capture-replay against [[activities-and-timeline]] ACT-DDL-1) |
| AC7.3 | Outbound messaging via the agent/MCP path is **🟡 confirm-first** — an unconfirmed send dispatches nothing (shares ACX.3). | contract test: tool tier = 🟡 ([[acceptance-standards#GATE-AI-7]]) |
| AC7.4 | **No cloud-proxy aggregator is in the capture path** for any non-SaaS tier: a dedicated/sovereign deployment's messaging capture shows zero third-party-vendor egress beyond the network operator (Telegram/Meta). | deployment conformance test; WhatsApp's Meta egress is the documented exception |
| ACX.6 | **Local-only mode:** for the on-prem/regulated profile, the entire surface (capture, transcription, summaries, drafts, search) runs with zero external egress (P7). *(network-isolation test)* **Named exception (ADR-0022):** **WhatsApp** capture is operator-in-path — the WhatsApp Cloud API runs on Meta's infrastructure (On-Premises sunset 2025-10-23), so WhatsApp is **not** a zero-egress channel and is excluded from the sovereign guarantee; Telegram (self-hosted client) and email/calendar (direct in-boundary clients) are not excluded. | network-isolation test |
| MSG-AC-1 | Given a workspace on the sovereign profile, when an admin enables the WhatsApp channel, then the enablement surface discloses the named operator egress (the ACX.6 carve-out) and records the acknowledgment in the audit log; without the acknowledgment the channel does not connect. Telegram enablement carries no such disclosure. | enablement integration test + the AC7.4 conformance run with the channel enabled |
