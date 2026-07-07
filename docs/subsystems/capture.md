---
status: planned
module: backend/internal/modules/capture
derives-from:
  - specs/spec/features/02-capture-and-comms.md#the-governing-stance-read-first @ 5a0b29c
  - specs/spec/features/02-capture-and-comms.md#feature-1--email--calendar-sync--auto-logging @ 5a0b29c
  - specs/spec/features/02-capture-and-comms.md#feature-3--automatic-contact--org--activity-creation--enrichment @ 5a0b29c
  - specs/spec/features/02-capture-and-comms.md#cross-cutting-acceptance-criteria-whole-surface @ 5a0b29c
  - specs/spec/features/07-ai-native-moments.md#4-capture-activation--relationship-strength @ 5a0b29c
  - specs/spec/features/10-operational-depth.md#5-outreach-engine-promotes-d43-reply-tracking-d45-sequences-d411-bulk-activity-d48-telephony @ 5a0b29c
  - specs/spec/product/epics/E02-zero-entry-capture.md @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#21-onboarding--cold-start @ 5a0b29c
---
# Capture — the CRM fills itself, and every row says where it came from

> The subsystem that turns a rep's real communications — mail and calendar, at
> Gmail and Microsoft 365 parity — into timeline activities and self-creating
> contacts and companies, through one audited writer, idempotently, with
> provenance on every row. It is where P5 is won or lost.

<!--
Contested / flagged items (for the spec-gate reviewer):
1. Table ownership: the data-model ownership index assigns ZERO tables to this
   chapter — deliberate, not an omission. Capture is a writer, not a substrate:
   it writes into activity/activity_link (activities-and-timeline), person/
   person_email/organization/organization_domain (people-and-organizations).
   This chapter therefore pins no Schema section; the ticket-coverage gate
   should not expect DDL here.
2. activation.html primacy: the screen-to-story index lists S-E02.4 (this
   chapter's) and S-E02.5 (people-and-organizations'). The people chapter
   explicitly deferred the activation screen here and kept the strength formula
   (PO-F-3, single home). Pinned here as CAP-AC screen rows; the strength rows
   inside the stream verify against the people chapter's formula.
3. AC-activation-6 pins a voice-profile panel on this chapter's screen; the
   panel's content (corpus growth, profile quality) is the voice-profile
   chapter's domain — the row is held here only as screen acceptance.
4. EVT-SEM naming: the capture correlation chain is EVT-SEM-10 in the ratified
   event chapter (EVT-SEM-9 is the approval pair). Both are cited where they
   apply; upstream references to "the capture chain" as EVT-SEM-9 are stale.
5. Reply/engagement tracking (features/10 §5, S-E15.5 promotion of record):
   split ownership — this chapter's pipeline detects the thread-matched inbound
   reply and emits the engagement event; the sequence engine that pauses on it
   is sequences-and-deliverability's. Stated in both chapters, pinned nowhere
   twice.
-->

## What it's for

Most CRM data already exists in the inbox and the calendar; a CRM that asks reps
to retype it collects a tax and ends up empty. This subsystem removes the tax:
it watches each user's connected mailbox and calendar and writes what happens
there — emails, meetings, the people and companies behind them — into the CRM on
its own, so manual logging is the rare, measured exception rather than the
workflow ([[glossary]] `capture`). Its callers are the connectors' provider
push and delta feeds on the way in, and everything else on the way out: the
timeline and 360 screens read what it wrote, the context graph, scoring, and
briefs react to the events it emits, and the activation screen makes its first
minutes visible. The boundary is sharp: capture owns the pipeline — connect,
ingest, normalize, resolve, enrich, write-once — and the activation surface;
it owns none of the tables it fills and none of the outbound machinery.

## Principles it serves

- **P5 — auto-capture over manual entry.** The flagship. Every record this
  subsystem produces is created by a connector or an agent, not typed; the
  manual-entry rate is instrumented per workspace and per channel so
  "typed-by-hand is a smell" is a number, not a slogan.
- **P7 — own your data.** Transport is in-boundary: direct provider clients
  (with a self-hostable engine as the allowed swap), never a cloud-proxy
  aggregator in the capture path ([[scope#NEVER-9]]).
- **P12 — governance designed in.** Provenance is mandatory on every captured
  row, ambiguous merges surface for human approval instead of executing, and a
  human-edited field is never silently overwritten
  ([[acceptance-standards#GATE-AI-4]]).
- **P4 — blazing fast, always.** Ingestion is async and never sits on the
  request path; a connector outage degrades capture, never core CRM reads.
- **ADR-0022 — capture build/borrow boundary.** We borrow commodity protocol
  plumbing and build the capture semantics — normalization, dedupe, provenance,
  linking. The same decision makes capture memory-first: the raw payload is the
  durable source of truth beneath the derived rows.
- **ADR-0020 — customer-supplied inference.** Retroactive backfill over the
  captured corpus is an explicit, budgeted job on the customer's own inference,
  never an unmetered background burn.

## How it works

**Connect is per-user, consent-driven opt-in.** A rep connects their own
mailbox and calendar; team visibility of what their connection captures is
governed by RBAC, not by the connection ([[runtime-config#RC-8]]). Gmail plus
Google Calendar and Microsoft 365 plus Outlook are V1 at parity — the
regulated-DACH beachhead is Microsoft-first, so Outlook is not a fast-follow
(decision A51). Sync is incremental from provider push and delta feeds, never a
full re-scan.

**Exclusion runs before ingestion.** A bounded per-user rule set — sender or
recipient domain, or mail label; deliberately not a filtering DSL
([[runtime-config#RC-2]]) — is evaluated before anything is written. A matching
message produces zero CRM rows, and the pipeline records the skip as an event
so "personal mail is never ingested" is machine-verifiable, not asserted
([[event-bus#EVT-SEM-10]]).

**Connectors normalize; one writer writes.** A connector turns raw provider
content into normalized records and hands them to the async pipeline — it never
touches the database. A single audited writer drains the queue and, in one
transaction, rejects anything missing provenance, scopes the write to the
workspace, upserts the domain row, appends the audit entry, and emits the
domain event. Because tenancy, audit, and events live in exactly one place,
every connector inherits them for free, and the capture writer itself is a
named, separately-audited system-service exception rather than a back door
([[threat-model#D8]]).

**Idempotency is a key, not a hope.** Every captured activity carries the
originating system and that system's record id, and the pair is unique per
workspace — the capture key the activities chapter pins (ACT-DDL-1). Re-running
sync over the same window, a connector restart, or a redelivered message
resolves to the existing row and changes nothing; the bus-side consumer dedupe
mirrors the same rule ([[event-bus#EVT-DEL-2]]).

**Participants become records, deduplicated on create.** A sender or attendee
not yet in the CRM becomes exactly one person — idempotent across a whole
thread — linked to an organization derived from the email domain. Matching is
the people chapter's two-tier dedupe (PO-F-1 for people, PO-F-2 for
organizations): an exact identity match lands on the existing record, and a
fuzzy near-match is withheld as a 🟡 merge candidate for a human — never
auto-merged. The review queue for those candidates is [[data-hygiene]]'s;
capture only feeds it.

**Enrichment fills fields with evidence, and loses to humans.** From the
captured comms themselves — an email signature, meeting context — enrichment
proposes title, phone, and role onto the records capture created, each value
carrying field-level provenance so an inferred title is always distinguishable
from a typed one. A field with no evidence stays empty; nothing is invented.
And enrichment never overwrites a human-edited field without a recorded 🟡
confirm ([[acceptance-standards#GATE-AI-4]]). Company classification and the
logo-on-create ride this same enrichment-on-create discipline but are pinned
with the organization record in [[people-and-organizations]]. Third-party
firmographic providers are OUT of V1; the agent-facing enrich tool is the
agents' surface, not this pipeline.

**The raw payload is memory, not garbage.** Every captured row keeps its
re-parseable original alongside the normalized columns, off the query hot path
([[data-model#DM-CONV-11]]). That raw substrate is what lets the product
backfill retroactively — define a new field today and derive it from the whole
captured history — as an explicit, budgeted, async job on customer-supplied
inference, previewing scope before it spends.

**Trust is earned, not assumed.** Everything the firehose brings in is tagged
untrusted-by-default — data, never instructions ([[threat-model#T2]]) — and
known injection patterns are neutralized at capture time as a best-effort layer
([[threat-model#D2]]). Connector-created records default to the originating
user's visibility scope, not workspace-global, until a human promotes them, and
a suspicious auto-created record is quarantined pending review rather than
instantly trusted by scoring and routing ([[threat-model#D8]]).

**Replies are the engagement signal.** When an inbound message thread-matches a
prior outbound one, the pipeline emits a reply event — idempotent per reply,
and deliberately reply-based rather than an open-pixel
([[event-bus#EVT-SEM-14]]). Promoted to V1 by the operational-depth promotion
of record (S-E15.5); the sequence engine that pauses on it belongs to
[[sequences-and-deliverability]], and lead promotion triggered by it to
[[leads-and-qualification]].

**The first minutes are a surface, not a batch.** Right after connect, the
activation screen shows the workspace populating live — counters and a
provenance-stamped stream of people, organizations, and activities as they
land, honest about progress and degradation, never a fake-populated screen
(S-E02.4, the wow moment). The view reads from the relational core inside its
own refresh budget and never blocks on the pipeline; the relationship-strength
rows in the stream come from the people chapter's formula, cited not computed
here.

**Every step is one correlation chain on the bus.** Received, normalized, and
each resulting created or captured event share one correlation id with
causation links, so a captured email's whole journey is replayable end to end
([[event-bus#EVT-SEM-10]]); each capture event stream and the pipeline's
consumer group are pinned in the event chapter
([[event-bus#EVT-STREAM-7]], [[event-bus#EVT-CG-4]]).

## What's configurable

- **Personal-mail exclusion rules** — per-user domain and label rules that gate
  whether ingestion creates rows at all; a bounded rule set, not a config
  engine ([[runtime-config#RC-2]]).
- **Capture connection and scope** — per-user connect and disconnect of mail
  and calendar, with team visibility governed by RBAC
  ([[runtime-config#RC-8]]).
- **Provider clients** — direct in-boundary clients by default, a self-hostable
  engine as the allowed swap, cloud-proxy aggregators rejected
  ([[scope#NEVER-9]]); which binding a deployment runs is composition, not
  runtime config.
- **Degradation** — with no connector connected, capture is simply absent and
  the product runs manual-first (the smell metric then reports honestly); a
  connector outage degrades capture alone while core CRM budgets hold.

## Guarantees (enforced)

- **A captured touch is on the timeline within a minute.** From send or receipt
  to the activity appearing on the right person's (and linked deal's) timeline
  is 60 seconds p95 (CAP-PARAM-1), including auto-create and linking of new
  participants — and never at the cost of blocking the inbound render.
- **Excluded mail produces zero rows.** A message matching a personal-exclusion
  rule creates nothing anywhere, and the skip event is the machine-checkable
  proof ([[event-bus#EVT-SEM-10]]).
- **Replay is a no-op.** Re-running capture over the same window creates no
  duplicate activity (the ACT-DDL-1 capture key) and no duplicate person (the
  PO-F-1 exact tier) — double-ingest leaves counts unchanged.
- **Nothing is anonymous.** One hundred percent of rows this subsystem creates
  carry non-null source and captured-by ([[acceptance-standards#GATE-CORE-3]]);
  the manual-entry share is thereby computable per workspace and per channel.
- **Humans outrank enrichment.** No enrichment value overwrites a human-edited
  field without a recorded 🟡 approval ([[acceptance-standards#GATE-AI-4]]).
- **No wrong auto-merge, deterministically.** Ambiguous candidates surface for
  confirmation; the zero-auto-merge invariant is a hard gate even though
  candidate precision itself is a tracked eval, not a merge gate (AC3.5).
- **Capture never blocks core CRM.** All ingestion is async; killing the
  capture worker leaves record open, list, and save budgets green (ACX.5).
- **Captured content is data, never instructions.** The firehose is tagged
  untrusted, sanitized best-effort, scoped to the originating user until
  promoted, and written only through the audited single writer
  ([[threat-model#D2]], [[threat-model#D8]]).

## Acceptance

Done means a rep who connects a mailbox and then does nothing watches the CRM
become true on its own: mail and meetings land on the right timelines within a
minute, tagged with where they were read from; new people and their companies
appear as real linked records, exactly once; signature-read values sit in the
right fields, visibly distinguishable from typed ones; personal mail never
appears anywhere; and the activation screen makes the filling visible with
honest progress — degraded and empty states rendered truthfully, never a
fake-populated screen. The manual-entry rate is visible per workspace and per
channel; the under-ten-percent figure is a per-channel product KPI tied to
which connectors are live — deliberately **not** a release gate, since a team
working channels that have no connector yet is majority-manual by construction
and that is a roadmap signal, not a defect. The testable forms live in the
Acceptance appendix; the screen-state floor and performance budgets are
inherited from [[acceptance-standards]].

## Out of scope

- **The substrate it writes into.** The activity row and its capture-key
  constraint are [[activities-and-timeline]]'s; person, organization, the
  dedupe formulas (PO-F-1/2), relationship strength, classification, and the
  logo are [[people-and-organizations]]'s.
- **Cold-start from a URL.** The scrape read-back that bootstraps a workspace
  before any mailbox is connected — staging-only, evidence-or-omit — is
  [[onboarding-and-coldstart]]'s, including its contract operation and the
  stages-never-writes rule ([[event-bus#EVT-SEM-11]]).
- **Meetings, calls, and transcripts.** Transcript ingest, transcription
  bindings, and consent-gated recording (including the telephony promotion,
  S-E15.6) are [[meetings-and-transcripts]]'s; capture contributes the shared
  pipeline discipline they reuse.
- **Messaging channels.** WhatsApp and Telegram capture, and the honestly-named
  WhatsApp operator-in-path exception to the sovereign zero-egress guarantee
  (ACX.6), are [[messaging-channels]]'s.
- **Everything outbound.** Drafting, sending, sequences, and the confirm-first
  gate on every send (ACX.3) are [[drafting]],
  [[sequences-and-deliverability]], and [[approvals-and-concurrency]] — capture
  is inbound only ([[acceptance-standards#GATE-AI-7]]).
- **Merge-candidate review.** The 🟡 queue where surfaced candidates are
  confirmed or rejected is [[data-hygiene]]'s.
- **Browser-extension capture.** The inbox sidebar and the LinkedIn
  save-as-lead surface are the client-surfaces work, bound by the lead-not-
  contact gate ([[acceptance-standards#GATE-CS-2]]) and routed through
  [[leads-and-qualification]].

## Where it lives

The backend's capture module — connectors, the exclusion gate, the normalizer,
entity resolution, comms-derived enrichment, and the single audited writer —
publishing to and driven by the event bus, with the activation screen as its
one owned surface. Read next: [[activities-and-timeline]] (the rows it writes),
[[people-and-organizations]] (the records it creates and the dedupe it calls),
[[event-bus]] (the chain it emits), and [[data-model]] (the provenance
convention every row obeys).

## Appendix

### Parameters
Source: features/02-capture-and-comms.md#feature-1--email--calendar-sync--auto-logging + features/07-ai-native-moments.md#4-capture-activation--relationship-strength @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| CAP-PARAM-1 | Capture-to-timeline latency | 60 s p95 | From send/receipt/hangup to the activity visible on the matched timeline, including auto-create + link of new participants (AC1.1, shared by AC3.4); integration-tested with a clock assert, and never satisfied by blocking the inbound render. |
| CAP-PARAM-2 | Activation view refresh | < 150 ms p95 server | The activation screen reads captured counts/rows from the relational core per refresh and never blocks on the async pipeline (features/07 §4); chaos-tested — worker killed → last-known state still renders. |
| CAP-PARAM-3 | Manual-entry rate | < 10 % per live channel — **KPI, not a release gate** | The share of activities/people with human captured-by, segmented per workspace **and per channel**. The gate is only that the metric is instrumented, visible, and correct (AC1.6/ACX.2); the figure is meaningful only for channels whose connector is live, and a high rate on unconnected channels is a missing-connector signal, not a defect. |

### Wire
Source: contract/crm.yaml @ 5a0b29c

This chapter owns no contract operations — reported honestly: connectors are
not REST-surfaced. Operations are cited by operationId, never restated.

Note CAP-WIRE-N-1: mailbox/calendar connect is a per-user OAuth + consent flow
([[runtime-config#RC-8]]) and inbound sync rides provider push/delta (Gmail
watch + History; Microsoft Graph delta + change notifications, at parity per
A51) — there is no capture ingestion endpoint in the contract, by design
(capture is async and in-boundary, never a public write surface).

Note CAP-WIRE-N-2: the manual-entry fallback and agent logging ride
`logActivity` ([[activities-and-timeline]] ACT-WIRE-2 — whose replay-on-capture-
key semantics are the wire face of this chapter's idempotency), `createPerson`,
and `createOrganization` ([[people-and-organizations]] PO-WIRE rows). Capture's
only claim on them is that their human-principal writes feed the CAP-PARAM-3
metric.

Note CAP-WIRE-N-3: `coldStartReadback` is capture-adjacent (it composes the
same scrape/enrichment seam) but belongs to [[onboarding-and-coldstart]];
nothing about it is pinned here.

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c — definitions live in the central catalog ([[event-bus#events--catalog]]); cited, never redefined.

| ID | Emitted/consumed | Definition |
|---|---|---|
| `capture.received` | Emitted once per raw provider record accepted into the pipeline (connector, source system, external id, raw ref, sync cursor). | [[event-bus]] catalog (Capture) |
| `capture.normalized` | Emitted once per normalized record with the entities it produced; the context graph's capture input. | [[event-bus]] catalog (Capture) |
| `capture.failed` | Emitted on a failed capture step with error class + retryability; drives dead-letter and the ops dashboard. | [[event-bus]] catalog (Capture) |
| `capture.skipped` | Emitted when ingestion deliberately produces nothing — reason personal_exclusion, duplicate, or out_of_scope; the machine-verifiable AC1.3 proof ([[event-bus#EVT-SEM-10]]). | [[event-bus]] catalog (Capture) |
| `activity.captured` | Emitted by this module once per normalized captured activity, idempotent on the capture key (ACT-DDL-1); the P5 spine — drives the context graph, deal recency, and the CAP-PARAM-3 metric. | [[event-bus]] catalog (Activity) |
| `person.created` / `organization.created` | Emitted by the people module when this pipeline auto-creates a participant/domain org; carries source + captured-by so machine-creation is attributable. | [[event-bus]] catalog (Person/Organization) |
| `engagement.reply` | Emitted by this module when an inbound message thread-matches a prior outbound; idempotent per reply, reply-based never open-pixel ([[event-bus#EVT-SEM-14]]). | [[event-bus]] catalog (Engagement) |
| `cg:capture` | Consumed: the pipeline's own consumer group drives the capture state machine from `capture.*` ([[event-bus#EVT-CG-4]]); the capture stream is [[event-bus#EVT-STREAM-7]]. | [[event-bus]] streams/consumers |

Note CAP-EVT-N-1: the whole chain — received → normalized → per-entity created/
captured — shares one correlation id with causation links
([[event-bus#EVT-SEM-10]]); any capture performed under an approval token
carries the approval pair's correlation id ([[event-bus#EVT-SEM-9]]).
`coldstart.read_back_proposed` is emitted from the shared scrape seam but is
[[onboarding-and-coldstart]]'s ([[event-bus#EVT-SEM-11]]).

### Acceptance
Source: product/epics/E02-zero-entry-capture.md#s-e021--connect-your-mailbox-and-the-crm-logs-everything-untyped + #s-e022--contacts-and-companies-create-themselves-from-the-people-you-talk-to + #s-e023--signature-scraping-fills-the-fields-youd-otherwise-retype + #s-e024--watch-it-fill-itself-the-wow-moment + #s-e026--provenance-on-every-field-captured-by-vs-typed-by @ 5a0b29c

Owned stories, condensed to their Given/When/Then load; S-E02.5/.7/.8 are
pinned in [[people-and-organizations]].

| ID | Given/When/Then | Verification |
|---|---|---|
| S-E02.1 | V1-Must. Given a connected Gmail/Calendar (and M365/Outlook at parity, A51), when mail is sent/received with a known contact or an external meeting occurs, then the activity lands on the right person/deal/org timeline within about a minute with no "log this" click; excluded personal mail produces zero rows; re-sync creates no duplicates; manual logging works but is the rare, measured exception. | AC1.1–1.4, AC1.6–1.8 below; live-stack walkthrough ([[testing#TEST-LANE-3]]) |
| S-E02.2 | V1-Must. Given a thread with someone not in the CRM, when captured, then exactly one contact is created (not one per message) linked to a domain-derived organization that is a real record; an existing-email sender creates no duplicate; a same-name-different-domain near-match surfaces as a 🟡 merge candidate, never silently auto-merged. | AC3.1/3.2/3.5 below; dedupe tiers verify against PO-F-1/PO-F-2 ([[testing#TEST-LANE-2]]) |
| S-E02.3 | V1-Must. Given a captured email with a signature block, when enrichment runs, then title/phone fill from it where present, each value showing its source and distinguishable from typed input; a human-edited field is never silently overwritten (🟡 confirm instead); a signature with nothing useful invents nothing. | AC3.3/3.6 below; [[acceptance-standards#GATE-AI-4]] + evidence-or-omit [[acceptance-standards#GATE-AI-1]] ([[testing#TEST-LANE-2]]) |
| S-E02.4 | V1-WOW. Given a just-connected mailbox, when capture begins, then the user sees records appearing live — counts climbing, provenance visible as it lands — within minutes; on backfill completion the workspace is demonstrably non-empty and recognizably theirs with a one-line capture summary; a slow/degraded connector shows honest progress, never a fake-populated screen. | AC-activation-1..9 below ([[testing#TEST-LANE-3]]); CAP-PARAM-2 chaos/perf test; honest states per [[acceptance-standards#STATE-1]]–STATE-3 |
| S-E02.6 | V1-Must. Given any field on any record, when inspected, then its provenance shows captured (which source/connector/agent, when) or typed (which human, when) — no field origin-less; the manual-entry rate is visible; an edit preserves the other origin; every captured row has an append-only audit entry. | ACX.1/ACX.2/ACX.4 below; [[acceptance-standards#GATE-CORE-3]]; provenance display surfaces owned by [[people-and-organizations]] (PO-AC-9) |

Source: features/02-capture-and-comms.md#feature-1--email--calendar-sync--auto-logging + #feature-3--automatic-contact--org--activity-creation--enrichment @ 5a0b29c

Corpus IDs preserved verbatim (wording condensed only where the corpus text is
narrative around the criterion; the criterion itself is untouched).

| ID | Given/When/Then | Verification |
|---|---|---|
| AC1.1 | Given a connected Gmail account, when an email is sent to a known `person`, an `activity(type=email)` appears on that person's timeline within **60 s p95** of send, with `source` and `captured_by` populated. | Integration test against a seeded mailbox + clock assert ([[testing#TEST-LANE-2]]); CAP-PARAM-1 |
| AC1.2 | No activity is ever created without non-null `source` AND `captured_by`. | DB constraint + rejected-insert test ([[data-model#DM-AC-4]], [[acceptance-standards#GATE-CORE-3]]) |
| AC1.3 | Emails matching a personal-exclusion rule produce **zero** rows in `activity`. | Test: seed excluded message → assert 0 rows; `capture.skipped{personal_exclusion}` is the bus-side proof ([[event-bus#EVT-SEM-10]]) |
| AC1.4 | Re-running sync over the same history window creates **no duplicate** activities (idempotent on msg-id). | Double-run replay test → row count unchanged, against ACT-DDL-1's capture key ([[activities-and-timeline]]) |
| AC1.5 | Timeline read for a person with 200 activities renders server-side in **< 150 ms p95** (list budget). | CI benchmark ([[acceptance-standards#PERF-2]]); the read path is [[activities-and-timeline]]'s (ACT-AC-4) — cited, not re-owned |
| AC1.6 | Manual-entry rate metric is emitted: `% activities where captured_by = human:*`. **The release gate is only that the metric is instrumented and visible.** The **< 10%** figure is a **per-channel product KPI, not a build/release gate** — meaningful only for channels whose connector is live; a team dominated by not-yet-connected channels is majority-manual *by construction*, a signal of which connectors are missing, not a defect. Track segmented by live channel; do not gate code on it. | Metric-exists test per workspace and per channel ([[testing#TEST-LANE-2]]); CAP-PARAM-3 |
| AC1.7 | **User-observable:** a rep who connects their mailbox and does nothing sees the timeline populate on its own — an email to a known contact shows on that contact's (and the right deal's) timeline within a minute, tagged with where it was read from, no "log this" button pressed. | Activation walkthrough ([[testing#TEST-LANE-3]]); backed by AC1.1 |
| AC1.8 | **User-observable:** personal mail stays invisible — messages matching the exclusion rule never appear anywhere in the CRM, so the rep trusts connecting their real mailbox. | Live-stack exclusion walkthrough ([[testing#TEST-LANE-3]]); backed by AC1.3 |
| AC3.1 | A captured email from an unknown sender creates exactly one `person` (idempotent across the thread) linked to a domain-matched `organization`, with provenance. | Integration test: 5-email thread, one new sender → 1 person, 1 org ([[testing#TEST-LANE-2]]) |
| AC3.2 | Dedupe: an inbound participant whose email matches an existing `person` creates **no** new person. | Assert-no-insert test against PO-F-1's exact tier ([[people-and-organizations]]) |
| AC3.3 | An enriched field carries field-level provenance distinguishing it from human entry; querying "human-entered fields" excludes it. | Schema + query test ([[testing#TEST-LANE-2]]); [[data-model#DM-CONV-11]] |
| AC3.4 | Auto-create + link completes within the **60 s p95** capture budget (shared with AC1.1) and never blocks the inbound-mail timeline render. | Integration + perf test ([[testing#TEST-LANE-2]]); CAP-PARAM-1 |
| AC3.5 | **(ML eval, not a deterministic CI gate.)** Merge-candidate detection precision is measured as an eval against a defined, version-controlled labeled eval set; v1 KPI **≥ 95%** precision with a flaky-aware regression band. The **deterministic gate** is the safety invariant: **no wrong auto-merges — ambiguous candidates are surfaced (🟡), never auto-merged**. | Eval per [[ai-evals]]; deterministic test: ambiguous fixture → 0 auto-merges ([[testing#TEST-LANE-1]] over PO-F-1/PO-F-2) |
| AC3.6 | **User-observable:** when a rep emails or meets someone new, that person and company appear in the CRM on their own, already linked, with title/role read from the signature pre-filled — a populated contact, not a blank form (S-E02.3); every such value visibly shows it was captured and from where, so an inferred title is tellable from a typed one (S-E02.6). | Live-stack walkthrough ([[testing#TEST-LANE-3]]); display surfaces per [[people-and-organizations]] PO-AC-9 |

Source: features/02-capture-and-comms.md#cross-cutting-acceptance-criteria-whole-surface @ 5a0b29c

The whole-surface rules that bind this chapter, verbatim. ACX.3 (confirm-first
on every outbound action) binds the outbound chapters and is enforced as
[[acceptance-standards#GATE-AI-7]]; ACX.6 (local-only mode, with WhatsApp as
the named operator-in-path exception) is pinned in [[messaging-channels]] —
both cited, not re-pinned; capture's email/calendar transport is **not**
excluded from the sovereign guarantee ([[scope#NEVER-9]]).

| ID | Given/When/Then | Verification |
|---|---|---|
| ACX.1 | **Provenance universality:** 100% of records created by this surface have non-null `source` + `captured_by`. | DB constraint, enforced; CI test ([[acceptance-standards#GATE-CORE-3]]) |
| ACX.2 | **Manual-entry smell metric** is emitted per workspace **and per channel** and visible on an ops dashboard; defined as `% activities/people created with captured_by = human:*`. The **gate is that the metric exists and is correct**, not its value. The **< 10%** figure is a **per-channel KPI tied to which connectors are live**, not a release gate — see AC1.6. | Metric-exists test ([[testing#TEST-LANE-2]]); CAP-PARAM-3 |
| ACX.4 | **Full audit trail:** every send, dial, auto-create, merge, and enrichment is in `audit_log` with actor (human/agent), inputs, and outcome, replayable (P12). | Audit completeness test ([[acceptance-standards#GATE-CORE-5]]); this chapter's rows: auto-create, merge-candidate, enrichment |
| ACX.5 | **Capture never blocks core CRM:** all ingestion is async; a connector outage degrades capture but record open/list/save budgets are unaffected. | Chaos test: kill capture worker → assert core ops green ([[testing#TEST-LANE-3]]) |

Source: product/30-screen-acceptance.md#21-onboarding--cold-start @ 5a0b29c

*Activation screen — "Watch it fill itself" (implements S-E02.4 — this
chapter; S-E02.5 strength content verifies against
[[people-and-organizations]]):*

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-activation-1 | Given the mailbox was just connected, When the screen loads, Then it shows a "LIVE · capturing now" badge, a hero, and four live counter tiles (Contacts, Companies, Activities, Relationships) starting at 0. | Screen e2e lane ([[testing#TEST-LANE-3]]) |
| AC-activation-2 | Given the counter tiles, When the screen runs, Then each counter animates upward toward its target with a blinking cursor that disappears once it settles. | Screen e2e lane |
| AC-activation-3 | Given the live capture stream, When it runs, Then rows insert newest-at-top with a slide-in, each showing a typed icon, a description, a provenance tag (e.g. "connector · Gmail"), and a timestamp; the visible list is capped. | Screen e2e lane |
| AC-activation-4 | Given the stream is running, When rows are still arriving, Then the pace indicator and "N captured · streaming…" count update; when it finishes, pace shows "idle", the count shows total, and "Enter your CRM" fades in. | Screen e2e lane |
| AC-activation-5 | Given the capture stream, When rows render, Then they include all capture types — person, organization (from email domain), email/meeting links, and computed relationship-strength rows — each carrying its own provenance/basis tag. | Screen e2e lane; strength rows per [[people-and-organizations]] (PO-F-3) |
| AC-activation-6 | Given the voice profile section, When it renders, Then it shows "good → sharp" (prior "good" struck through), "+N words from your sent email" ingested, and a "self-improving" tag. | Screen e2e lane; panel content verified with the voice-profile chapter |
| AC-activation-7 | Given the "How this works" section, When it renders, Then it states three honesty guarantees: capture runs in the background and never blocks; every row is marked with its source and personal mail is never ingested; a relationship-strength baseline is computed from recency/frequency/reciprocity. | Screen e2e lane; the guarantees themselves are ACX.5, AC1.2/AC1.3, PO-F-3 |
| AC-activation-8 | Given the "Enter your CRM" button appears, When I click it, Then I navigate to home.html, with a note that capture keeps running in the background. | Screen e2e lane |
| AC-activation-9 | Given the in-app shell, When the screen loads, Then the Ledger-Green nav rail + ⌘K palette are available. | Screen e2e lane (global chrome floor) |
| CAP-AC-OPEN-1 | **Prototype gaps carried to build:** all activation counters/rows are hardcoded in the prototype — the build must drive them from real connector progress; slow / failed / empty-mailbox states are missing and must render honestly per the standard matrix ([[acceptance-standards#STATE-1]]–STATE-3); whether activation rows are clickable into records, and whether "Enter" is available before the stream settles (real capture is async), are ticket-time decisions. | Ticket-gate: the activation screen ticket must resolve these before build |
