---
status: planned
module: backend/internal/modules/meetings (dossier, transcript intelligence, booking, profiler); web (booking + profiler surfaces)
derives-from:
  - specs/spec/features/02-capture-and-comms.md#feature-2--call--meeting-transcription-capture
  - specs/spec/features/07-ai-native-moments.md#5-meeting--transcript-intelligence
  - specs/spec/features/07-ai-native-moments.md#5b-deal-qualification-checklist-agentic-gap-only
  - specs/spec/features/07-ai-native-moments.md#5c-smart-booking--self-scheduling-link
  - specs/spec/features/07-ai-native-moments.md#5d-deep-research-person-profiler
  - specs/spec/product/epics/E04-meetings-and-calls.md
  - specs/spec/product/30-screen-acceptance.md#bookhtml--public-meeting-booking-implements-s-e041
  - specs/spec/product/30-screen-acceptance.md#profilerhtml--deep-research-person-profiler-implements-s-e046
---
# Meetings & transcripts — the conversation is the record; the human only confirms it

> The intelligence around a single meeting: a dossier waiting before it, the call
> captured as a first-class activity during and after it, and the proposed next
> step, attendees, and deal signals inferred from what was actually said — always
> evidence-backed, always confirm-first, never live, never biometric. Plus the two
> doors into that loop: a public self-scheduling link that books graph-native
> meetings, and an on-demand, public-sources-only person profiler.

## What it's for

Reps prepare for meetings by hand, type notes from memory afterwards, and update
the deal only when nagged — so the record drifts from reality. This subsystem
makes the conversation itself the source of the record: it assembles what a rep
should know before walking in, lands the transcript as a linked, provenance-carrying
activity, and reads the deal updates, next step, and key signals out of the
conversation for the rep to confirm or fix — never to retype. Its callers are the
calendar and record surfaces that render the dossier and inferred-signal cards
(the deal 360 and pipeline board belong to
[deals-and-pipeline](deals-and-pipeline.md)), the work queue that receives its
proposed follow-ups ([tasks-and-work-queue](tasks-and-work-queue.md)), the public
booking screen a prospect uses without an account, the profiler screen a rep pulls
before a fair or first contact, and agents acting through the governed scheduling
verbs. The boundary: this chapter owns the inference and the two E04 screens; the
activity substrate the transcript lands in is
[activities-and-timeline](activities-and-timeline.md)'s, the proposal lifecycle on
the queue is [tasks-and-work-queue](tasks-and-work-queue.md)'s, and the staging
and confirm mechanics are
[approvals-and-concurrency](approvals-and-concurrency.md)'s.

## Principles it serves

- **P5 — auto-capture over manual entry.** The record is a by-product of the
  conversation: the transcript captures itself, the summary and action items are
  extracted, the booking logs itself, and the human reviews an AI-captured record
  instead of typing notes from memory.
- **P12 — governance is designed in.** Every inferred claim is evidence-or-omit
  ([[acceptance-standards#GATE-AI-1]]), every write is accept-to-persist
  ([[acceptance-standards#GATE-AI-2]]), human edits take precedence and are
  remembered ([[acceptance-standards#GATE-AI-4]]), and the RED-removed guard is
  absolute: no live-call surface, no biometric or emotion inference, ever
  ([[acceptance-standards#GATE-AI-6]], [[scope#NEVER-7]]).
- **P6 — use the model that fits.** Everything here is baseline L2 reasoning over
  a single conversation plus the deal's prior activities; it deliberately does not
  require the context graph, which would only deepen it later.
- **P7 — sovereignty is a feature.** Transcription defaults to a local
  speech-to-text model on regulated profiles, so no transcript content leaves the
  customer's infrastructure; the profiler completes with zero external egress on
  the sovereign profile.
- **P11 — clean relational core.** Everything accepted lands in real rows with
  real foreign keys — a meeting activity, a task, a deal field — never a shadow
  store.
- **ADR-0022 — capture build/borrow boundary.** Transcript ingestion rides the
  in-boundary capture transport; this chapter begins where a transcript exists.
- **ADR-0006 — the scrape/enrichment seam.** The profiler reads public sources
  only through the governed external-read seam, which an admin can disable.
- Light provenance: smart booking is decision A16; the profiler is decision A42;
  the epic's trust mechanic is founder principle 7 — reality fills the record,
  the human confirms it.

## How it works

**Before the meeting: the dossier is already waiting.** For any calendar meeting
with an external attendee whose organization we hold any activity for, the system
assembles a short dossier from records already held — what likely hurts this
customer, why the conversation is worth their time, and one or two likely paths to
close. Every claim carries the evidence it was read from as a clickable source —
an email, a prior call, an open deal, a firmographic field — and a claim with no
source in our data is omitted, never invented. An attendee we know nothing about
gets an honest no-prior-data dossier, grounding only what the email domain can
give and marking it as such. The dossier renders asynchronously and never blocks
the calendar or record view (MEET-PARAM-1); a dismissed or corrected claim is
remembered and does not silently reappear unchanged for the same account — that
correction memory rides the AI-feedback ledger owned by
[ai-runtime](ai-runtime.md).

**During and after: the transcript is a first-class activity.** A recorded
meeting or call from a connected source lands as a call or meeting activity with
the transcript attached, linked to attendees and the relevant deal, timestamped,
with no manual step — the row, its links, and its idempotent capture key are the
[activities-and-timeline](activities-and-timeline.md) substrate, and the connector
transport that delivers recordings is the [capture](capture.md) chapter's. Speaker
diarization is preserved and the recording is stored in the object store by
reference. Recording consent is a gate, not an afterthought: where consent rules
are not satisfied for a participant or region, capture is blocked or the
transcript withheld per policy, and the rep is told why. Transcription routes
through the provider-agnostic model client — a local speech-to-text model on
on-prem and data-sensitive profiles, so nothing leaves the boundary — and the
summary streams in asynchronously within the baseline AI budget (MEET-PARAM-2).
Transcript text is handled as sensitive by default: the threat model's honest
Article 9 carve-out governs it ([[threat-model#TM-DPIA-2]]), and its retention
default is seeded and owned by the data-model chapter
([[data-model#DM-SEED-3]]).

**After the call: one proposal, framed as a question.** When processing
completes, the rep sees a single proposal — the inferred next step with owner and
due date where the conversation stated one, any new attendees to create or link,
and any deal-field changes — framed "I inferred the next step — correct?".
Nothing has been written yet: before confirmation there are zero writes to the
deal, the task, or the contacts, and ignoring the proposal is treated as no,
never as silent acceptance. On accept, the records are written in one action,
each carrying provenance pointing back to the transcript lines it came from; an
edit before accept means the edit is what gets written. Where the proposal
surfaces and how accept, dismiss, and snooze behave on the queue is the
[tasks-and-work-queue](tasks-and-work-queue.md) chapter's doctrine; the token and
staging machinery is [approvals-and-concurrency](approvals-and-concurrency.md)'s;
this chapter owns producing the proposal and its evidence.

**Signals, not field writes.** From the same conversation the system infers the
deal's key signals — close-likelihood, service line, expected volume, timing —
each presented as an inferred signal with its supporting transcript quote and a
confidence indicator, never silently stamped into an editable field. An
ungrounded or low-confidence signal is shown as such or omitted: the system does
not present a guess as a fact. "Explain this" traces any inferred number to the
specific conversation evidence behind it, so a leader can trace a likelihood to
what was said rather than to a stage field someone forgot. A human override takes
precedence, is marked human-set, and is remembered as feedback
([[acceptance-standards#GATE-AI-4]]). The deal 360 and pipeline cards that render
these signals — and the qualification checklist panel — are the
[deals-and-pipeline](deals-and-pipeline.md) chapter's screens; the content is
produced here, and that chapter credits the dependency explicitly.

**The qualification checklist nags only about gaps.** Every deal carries a
background checklist auto-filled from captured signal: the same transcripts,
calls, and emails are mapped onto the qualification items, each auto-confirmed
item carrying its evidence snippet, source, and confidence. Only the items it
could not infer surface as honest gaps — the rep is nagged about missing
qualification, never about data entry the agent could have done. It re-checks on
every new call or email; a human-confirmed or overridden item is marked human-set
and never re-prompted. Which items make up the framework is bounded source
customization; the inference is not invented per deal. The checklist never
auto-advances a stage — stage moves stay confirm-first.

**Post-call coaching is the narrow, labeled exception.** Any sentiment or
coaching output is consent-based, post-call, text-only over the transcript,
draft-only, and labeled as exactly that. There is no live-call surface and no
code path that infers emotion, stress, or body language from voice or video —
the RED-removed guard is enforced as a static check, not a policy hope
([[acceptance-standards#GATE-AI-6]], [[scope#NEVER-7]]).

**The booking link is a front door, not a widget.** A public, unauthenticated
booking page computes availability from real connected-calendar free-busy plus
workspace rules — minimum notice, buffers, timezone — never a static slot list,
on the customer's own infrastructure. A known email is resolved against the
contact graph and routed to the deal owner, account manager, or territory rule —
never blind round-robin — with the rule decision visible; an unknown email
creates the person and organization by the domain rule and routes to the
configured default. The confirmed booking writes a meeting activity linked to the
resolved person, organization, and deal, stamped with booking-connector
provenance, in one audited transaction — the record is complete before the rep
touches it. A half-filled form writes nothing. The form captures consent per
purpose, persisting the grant and an append-only proof row with the exact wording
and version shown, default-deny for anything unchecked — the consent substrate is
owned by the data-model and gdpr chapters and cited, not rebuilt. Availability
and routing rules are source customization — a change is a reviewed diff, not a
settings screen — and booking is also a governed agent verb under the same
scopes, tiers, and audit path as the web page.

**The profiler researches a person from public sources only.** On demand — not
auto-run on any contact — a rep pulls a person-scoped bundle: public-info
background, suggested conversation angles, and business relevance, each line
carrying the cited public source it was read from. A claim with no public source
is omitted, not guessed; a person with little public presence gets an honest
"couldn't find" state, never a fabricated background. The profiler uses public
information only, infers no special-category data, and falls back to
company-level framing where individual data is not lawful or available — the
covert-profiling prohibition is a scope invariant ([[scope#NEVER-8]]), and the
DPIA's framing pins the posture ([[threat-model#TM-DPIA-2]],
[[threat-model#TM-DPIA-3]]). Where the person is already in the graph, the
profile surfaces our existing relationship and mutual connections with their
record references. The result is staged: nothing is written to the person record
until the rep saves it, corrections are marked typed-by-you, and dismissed lines
are excluded and remembered. On the sovereign profile it completes with zero
external egress; output carries the AI-assisted disclosure.

**Only the rep's own words feed voice learning.** Transcripts are speaker-filtered
before any voice use: only the user's own turns may feed the writing-voice corpus.
That consumer — the corpus, the profile, and the filter's enforcement — is the
[voice-profile](voice-profile.md) chapter's; this chapter's promise is that the
diarized transcript makes the filter possible and that no other consumer reads
other speakers' turns for learning.

## What's configurable

- **The qualification framework** — which checklist items exist is bounded
  runtime config / source customization per the platform boundary; per-vertical
  default frameworks are forkable templates. The inference over them is not
  configurable.
- **Booking availability and routing rules** — source customization: minimum
  notice, buffers, blocked windows, and routing precedence are versioned,
  reviewable rule code, not a runtime toggle; a rule change is testable
  (MEET-AC-16). The default slot duration is pinned (MEET-PARAM-3).
- **The speech-to-text provider** — injected per deployment: local model on
  regulated and sovereign profiles (zero transcript egress), cloud otherwise.
  With no provider, capture still lands the recording and the record degrades
  honestly to no-transcript; nothing blocks the timeline.
- **The external-read seam (ADR-0006)** — the profiler's source access is an
  admin-controlled capability; when disabled, the profiler renders an honest
  no-permission state, never a dead button.
- **Transcript retention** — the one-year transcript-erase default is seeded and
  owned by the data-model chapter ([[data-model#DM-SEED-3]]); cited here, not
  restated.
- **The dossier lead time** — the corpus promises the dossier "within N minutes
  of starting" without pinning N; the value is flagged for build to pin here
  first (note MEET-PARAM-N-1).

## Guarantees (enforced)

- **Evidence or omission, everywhere.** Every dossier claim, inferred signal,
  checklist item, and profiler line carries a non-empty evidence snippet with a
  resolvable source, or is absent; a rendered ungrounded value is a hard failure
  ([[acceptance-standards#GATE-AI-1]], MEET-AC-1/9/19).
- **Zero writes before confirm.** A transcript proposal, a half-filled booking
  form, and a staged profile each produce zero rows in real domain tables until
  the human acts; inspecting the tables mid-proposal finds nothing
  ([[acceptance-standards#GATE-AI-2]], MEET-AC-3/14/AC-profiler-6).
- **Accepted writes carry their evidence forever.** On confirm, each written
  row's provenance points to the transcript lines or booking source it came
  from, distinguishing machine-inferred-then-accepted from human-typed for the
  life of the row ([[acceptance-standards#GATE-CORE-3]], MEET-AC-4).
- **Signals never mutate fields.** Inference returns value, evidence, and
  confidence and writes no deal field; a human override wins, is marked
  human-set, and is not re-prompted ([[acceptance-standards#GATE-AI-4]],
  MEET-AC-5/10).
- **No live, no biometric — statically asserted.** No code path produces a
  live-call or biometric/emotion signal; any sentiment output is flagged
  post-call, text-only, draft ([[acceptance-standards#GATE-AI-6]], MEET-AC-7).
- **Consent gates recording and booking outreach.** An unsatisfied recording
  consent blocks or withholds the transcript with the reason shown; an unchecked
  booking purpose yields no grant, default-deny (MEET-AC-15, AC2 series).
- **Sovereignty holds.** On-prem transcription and summary complete with no
  outbound egress (AC2.4); the sovereign profiler run makes zero external calls
  (MEET-AC-22).
- **Availability is never fabricated.** A busy calendar block never appears
  bookable; slots honor notice, buffer, and timezone rules deterministically
  (MEET-AC-12).
- **Speaker-filtered voice feed.** Only the user's own transcript turns are
  readable by the voice-learning consumer; enforcement is pinned with its owner,
  [voice-profile](voice-profile.md).
- **No solely-automated decisions.** Every score is advisory and every
  consequential action is staged behind a recorded human approval — the Article
  22 posture the threat model pins ([[threat-model#TM-DPIA-3]]).

## Acceptance

Done means: a rep opens a meeting and the dossier is already there, honest when
we know nothing; after the call, the transcript sits on the timeline as a
captured record with a summary and extracted action items, and one proposal asks
"did I get this right?" — nothing written until they say yes, everything written
carrying its transcript evidence when they do; the deal shows inferred signals
with quotes and confidence instead of a stale stage field, and any number
explains itself; a prospect books a real free-busy slot and the meeting is on the
right deal, routed by rule, consent recorded, before the rep looks; a profiler
run returns only what public sources ground, stages it, and saves only on the
rep's click. The honest states — no-data dossier, thin profile, read-failed
research, no-permission seam, unrecognized booker, no-availability — render per
the standard screen-state floor inherited from
[[acceptance-standards]] (STATE-1 through STATE-5, plus the deep-research
fallback STATE-SP-3) and are not restated. The testable form of every claim
lives in the Acceptance appendix.

## Out of scope

- **The activity row and its substrate.** The polymorphic activity DDL, links,
  capture idempotency, and timeline reads —
  [activities-and-timeline](activities-and-timeline.md). This chapter owns no
  tables (see the Schema appendix and
  [[data-model#schema--ownership-index]]).
- **Connector transport.** How recordings and calendar entries physically arrive —
  the [capture](capture.md) chapter's pipeline; this chapter's ownership starts
  at the transcript's meaning. (The corpus's activity chapter phrases the
  transcript hand-off loosely; the seam honored here is transport-theirs,
  semantics-mine.)
- **The proposal queue lifecycle.** Accept, dismiss, snooze, and the
  proposals-are-not-rows doctrine —
  [tasks-and-work-queue](tasks-and-work-queue.md); approval tokens and expiry —
  [approvals-and-concurrency](approvals-and-concurrency.md).
- **The deal screens.** The deal 360, pipeline board, and their dossier,
  next-step, checklist, and signal cards are
  [deals-and-pipeline](deals-and-pipeline.md)'s screens; that chapter's render
  ACs cite this chapter as the content dependency.
- **Voice learning.** The writing-voice corpus and profile the speaker-filtered
  turns feed — [voice-profile](voice-profile.md).
- **Correction-memory storage.** The AI-feedback ledger that remembers
  dismissals and overrides — [ai-runtime](ai-runtime.md), which also owns the
  model client and routing.
- **Consent substrate and suppression.** The consent purpose, grant, and proof
  tables are the data-model chapter's DDL; enforcement is
  [gdpr-platform](gdpr-platform.md)'s.
- **Scoring formulas and forecast roll-ups.** Deal-health scoring bands and the
  forecast that consumes inferred likelihoods — the reporting and forecasting
  chapters (the partial "explain this" credit to S-E09.2 lives there).
- **The MCP tool table.** Verb, scope, and tier rows for the scheduling verbs —
  the byo-agent-and-mcp chapter; this chapter cites the operations on the wire.

## Where it lives

Planned as the meetings bounded context in the backend modules directory —
dossier assembly, transcript intelligence, booking, and the profiler as use
cases over the datasource and model-client seams — with the public booking and
profiler surfaces in the web shell. Read
[activities-and-timeline](activities-and-timeline.md) for the rows it writes,
[tasks-and-work-queue](tasks-and-work-queue.md) for where its proposals go,
[deals-and-pipeline](deals-and-pipeline.md) for the screens that render its
signals, and [voice-profile](voice-profile.md) for the speaker-filtered
consumer.

## Appendix

### Parameters
Source: features/07-ai-native-moments.md#5-meeting--transcript-intelligence @ 5a0b29c; features/02-capture-and-comms.md#feature-2--call--meeting-transcription-capture @ 5a0b29c; contract/crm.yaml (`getAvailability`) @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| MEET-PARAM-1 | Dossier first token | < 1.5 s p95 | The §5 acceptance budget for dossier rendering — the [[acceptance-standards#PERF-5]] baseline applied here; rendered async, never blocking the calendar/record view. |
| MEET-PARAM-2 | Transcript summary first token | < 1.5 s | AC2.3 — same [[acceptance-standards#PERF-5]] budget; shown async (non-blocking), model-bound. |
| MEET-PARAM-3 | Default booking slot duration | 30 minutes | `duration_minutes` default on `getAvailability`; the booking screen offers 30/60 (AC-book-4). |

Note MEET-PARAM-N-1: S-E04.1 promises the dossier "within N minutes of starting"
with N unpinned in the corpus; build must pin the lead time here before relying
on it. Booking min-notice and buffer values are source-defined rules (MEET-AC-16),
deliberately not parameters.

### Schema
Source: architecture/data-model.md (drafts) #schema--ownership-index @ 5a0b29c; contract/data-model.md §3.4, §7 @ 5a0b29c

**This chapter owns zero tables.** The ownership index assigns no table to
meetings-and-transcripts, and the 66-table corpus partition contains no
booking, availability, dossier, or profiler table. The finding, pinned:

| ID | Fact this chapter needs | Owning pin |
|---|---|---|
| MEET-SCHEMA-1 | Transcripts, calls, meetings, and bookings persist as `activity` rows (`kind` call/meeting; `meeting_status` booked/held/no_show/canceled; capture key; provenance columns) linked via `activity_link`. | [[activities-and-timeline]] ACT-DDL-1 / ACT-DDL-2 |
| MEET-SCHEMA-2 | Booking availability is computed from connected-calendar free-busy plus source-defined rules — there is deliberately no availability/slot table. | features/07 §5c (rules as code); MEET-AC-12/16 |
| MEET-SCHEMA-3 | Booking consent persists as a `person_consent` grant + append-only `consent_event` proof row (wording/version shown). | [[data-model#DM-DDL-10]] (consent cluster, contract/data-model.md §3.4) |
| MEET-SCHEMA-4 | Dossier/profiler/KPI correction memory ("does not silently reappear"; override "remembered as feedback") rides the `ai_feedback` ledger. | [[data-model#schema--ownership-index]] → ai-runtime chapter |
| MEET-SCHEMA-5 | Speaker-filtered voice-learning consumption reads `voice_corpus_source`. | [[data-model#schema--ownership-index]] → voice-profile chapter |
| MEET-SCHEMA-6 | Transcript retention/erasure default (365 days, erase — special-category free text). | [[data-model#DM-SEED-3]]; [[threat-model#TM-DPIA-2]] |

Proposals, dossiers, inferred signals, checklist states, and staged profiles are
not domain tables before acceptance by construction
([[acceptance-standards#GATE-AI-2]]); their staging home is the approvals
surface ([[approvals-and-concurrency]]).

### Wire
Source: contract/crm.yaml (Activities + AI tags) @ 5a0b29c

Operations cited by operationId, never restated. The scheduling pair is this
chapter's to specify; the activity and approval operations are cited with their
owners.

| ID | Operation (operationId) | Role in this chapter |
|---|---|---|
| MEET-WIRE-1 | `getAvailability` | Free/busy candidate slots for a host in a window — 🟢 read-only, the `check_availability` MCP verb; proposes, never books. Defaults per MEET-PARAM-3. |
| MEET-WIRE-2 | `bookMeeting` | Book a chosen slot — 🟡 confirm-first, the `book_meeting` MCP verb: agent callers supply an approval token, a human's own act is the approval. Writes one `meeting` activity linked to the supplied entities; idempotent on `Idempotency-Key`; `409 slot_taken` when the slot is gone. |
| MEET-WIRE-3 | `logActivity` | The transcript-capture write path: the call/meeting activity with transcript, capture key, and provenance rides the activities chapter's operation (its ACT-WIRE-2 pins replay-idempotency). |
| MEET-WIRE-4 | `listActivities` / `getActivity` | Timeline and detail reads of captured calls/meetings incl. raw payload (activities chapter, ACT-WIRE-1/3). |
| MEET-WIRE-5 | `updateActivity` | The rep's correction of an AI-captured record — human attribution on the delta (activities chapter, ACT-WIRE-4). |
| MEET-WIRE-6 | `listApprovals` / `approveApproval` / `rejectApproval` | The transcript proposal's staged life and its accept/dismiss — owned by the tasks/approvals chapters (TASK-WIRE-7..9); this chapter produces the staged content and evidence. |
| MEET-WIRE-7 | `recordConsent` | The consent write the booking form's purposes ride (consent cluster owner: data-model / gdpr-platform). |

Contract-extension needs (honest coverage; D-H2 docs-layer drift to reconcile,
not silent additions):

| ID | Gap | Detail |
|---|---|---|
| MEET-GAP-1 | Public booking surface | `getAvailability`/`bookMeeting` are authenticated, workspace-scoped operations; the public, unauthenticated booking page (S-E04.5, client-surface trust model) has no contract surface — routing-rule visibility, recognized-contact pre-fill, and per-purpose consent capture on the public form need specification. |
| MEET-GAP-2 | Transcript ingest | No operation ingests an uploaded/recorded transcript file (presigned upload + attach per features/02 §2); AC2.1 assumes it. Whether it is a capture-connector-only path or a wire operation must be pinned. |
| MEET-GAP-3 | Dossier read | No operation returns the pre-meeting dossier (S-E04.1) — the calendar/record surfaces need a read with per-claim evidence refs. |
| MEET-GAP-4 | Inferred signals + explain-this | No operation returns the `{value, evidence, confidence}` deal signals or the "explain this" derivation (S-E04.4, AIUC-06). |
| MEET-GAP-5 | Qualification checklist | No operation returns the per-deal checklist items/gaps (features/07 §5b, AIUC-07). |
| MEET-GAP-6 | Profiler | No operation runs, stages, or saves a deep-research profile (S-E04.6, AIUC-09); the save-to-record staging needs a wire home. |
| MEET-GAP-7 | Booking slot re-fetch | The screen's duration toggle changes the label but slots are not re-fetched (corpus screen note) — the built surface must re-query `getAvailability` on duration change. |

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c (activity §5.5, approval §5.6, capture §5.7, consent under person §5.1)

Definitions live in the central catalog ([[event-bus]]); cited, not redefined.

| ID | Event | Role in this chapter |
|---|---|---|
| MEET-EV-1 | `activity.captured` | Emitted once per landed call/meeting/booking activity — the booking's audited transaction emits exactly one (`captured_by=connector:booking`); transcript capture emits it with the capture key. |
| MEET-EV-2 | `activity.updated` | The rep's correction of a captured record (human typed-by attribution on the delta) — the visibly-kept correction AC2.5 promises. |
| MEET-EV-3 | `capture.received` / `capture.normalized` / `capture.failed` / `capture.skipped` | The transcript ingest chain (connector-actor, one correlation chain into MEET-EV-1) — emitted by the capture pipeline, owned by [capture](capture.md). |
| MEET-EV-4 | `approval.requested` / `approval.decided` | The transcript proposal staged 🟡 with proposed effect + evidence, and its confirm/dismiss; on approve the resulting domain events share the correlation id (approvals/tasks chapters own the pair). |
| MEET-EV-5 | `consent.changed` | Booking-form per-purpose consent, backed by the append-only proof row. |

Note MEET-EV-N-1 (honest report): the catalog defines no dossier, inferred-signal,
checklist, or profiler event — correct as specified, since those are advisory
read-side surfaces that write nothing until acceptance, and acceptance emits the
domain events above.

### Acceptance
Source: product/epics/E04-meetings-and-calls.md @ 5a0b29c; features/02-capture-and-comms.md#feature-2--call--meeting-transcription-capture @ 5a0b29c; features/07-ai-native-moments.md §5/§5b/§5c/§5d @ 5a0b29c; product/30-screen-acceptance.md (book, profiler sections) @ 5a0b29c

Story primacy verified against product/20-traceability.md @ 5a0b29c and the
scope chapter's epic map: E04's six stories — S-E04.1–.4/.6 (V1-WOW), S-E04.5
(V1-Must) — map to this chapter alone. Shared-credit rows honored elsewhere:
S-E03.3's inference half and the dossier/signal render ACs (AC-deal-4/5/8/9,
AC-pipeline-6) are pinned by [deals-and-pipeline](deals-and-pipeline.md) citing
this chapter; S-E16.2 (proposal lifecycle) is
[tasks-and-work-queue](tasks-and-work-queue.md)'s; S-E09.2's partial
"explain-this" credit is the forecasting/reporting chapters'. AI use-case rows:
AIUC-04 (dossier), AIUC-05 (transcript→proposal), AIUC-06 (signals/explain),
AIUC-07 (checklist), AIUC-08 (booking), AIUC-09 (profiler) — all in
[[ai-evals]].

Owned stories, condensed:

| ID | Given/When/Then | Verification |
|---|---|---|
| S-E04.1 | Given a calendar meeting with ≥1 external attendee whose org we have activity for, when I open it (or within N minutes of start), then a dossier names the likely pain, why the meeting is worth their time, and 1–2 paths to close — every claim with a clickable evidence source, sourceless claims omitted; a no-data attendee gets an honest "no prior data" dossier; a dismissed/corrected claim does not silently reappear for the same account. | Ticket-coverage gate; AIUC-04 (`RATIFY` ≥ 80% usefulness); [[acceptance-standards#GATE-AI-1]]; MEET-PARAM-1; correction memory via ai-runtime ledger. |
| S-E04.2 | Given a recorded meeting/call from a connected source, when capture runs, then a call/meeting activity exists with transcript attached, linked to attendees and deal, timestamped, no manual step; it appears in timeline context showing source and system-captured attribution; unsatisfied recording consent blocks/withholds with the reason shown; on-prem processing has zero transcript egress. | Integration lane ([[testing#TEST-LANE-2]]) over AC2.1–2.4; [[threat-model#TM-DPIA-2]]. |
| S-E04.3 | Given a captured transcript, when processing completes, then one proposal (next step with owner/due where stated + new attendees + deal-field changes) is framed as a question; zero writes before confirm; accept writes in one action with transcript-line provenance (edits win); reject/ignore changes nothing; a named date yields a task on the queue/Morning Brief. | AIUC-05 (next-step precision ≥ 85%, attendee precision ≥ 95%, [[ai-evals#AIEVAL-7]]/[[ai-evals#AIEVAL-8]]); [[acceptance-standards#GATE-AI-2]]; queue behavior owned by tasks-and-work-queue. |
| S-E04.4 | Given a captured call on an open deal, when inference runs, then close-likelihood, service line, expected volume, and timing render as evidenced signals with confidence — never stamped into fields; ungrounded → low-confidence or omitted; Riya can trace a likelihood to conversation evidence; an override wins, is marked human-set, and is remembered; any sentiment output is text-only, post-call, draft, labeled. | AIUC-06 (direction accuracy ≥ 75%, [[ai-evals#AIEVAL-19]]); [[acceptance-standards#GATE-AI-1]]/[[acceptance-standards#GATE-AI-4]]/[[acceptance-standards#GATE-AI-6]]. |
| S-E04.5 | Given a prospect opens my booking link, when slots show, then availability is real free-busy + workspace rules; a booking writes a meeting activity with booking-connector source, linked or staged; consent wording/version persists as grant + proof rows; routing follows deal-owner/AM/territory rules visibly; a BYO agent using the booking tool obeys the same Passport scopes and tiers. | AIUC-08 (routing correctness `RATIFY` ≥ 95%); integration lane ([[testing#TEST-LANE-2]]) with fixed clock; MEET-WIRE-1/2; MEET-GAP-1. |
| S-E04.6 | Given a person record (or name + org), when I request a deep-research profile, then I get public-info background + conversation angles + business relevance, every line carrying a cited public source (no source → omitted); little-public-info returns an honest thin state; public-information-only, GDPR-aware, no covert profiling; corrections/dismissals do not silently reappear. | AIUC-09 (`RATIFY` ≥ 80% usefulness); [[acceptance-standards#GATE-AI-1]]; [[scope#NEVER-8]]; [[threat-model#TM-DPIA-2]]. |

Feature 2 criteria, verbatim (features/02):

| ID | Given/When/Then | Verification |
|---|---|---|
| AC2.1 | Given an uploaded transcript linked to a calendar event, a `call\|meeting` activity is created with the transcript stored and `source`/`captured_by` set. *(integration test)* | Integration lane ([[testing#TEST-LANE-2]]); ACT-DDL-1 provenance; MEET-GAP-2. |
| AC2.2 | Generated action items become `task` rows linked to the activity; each with owner where the transcript names one. *(test: fixture transcript → assert N tasks with expected owners)* | Integration lane; **extraction here, persistence via the proposal lifecycle** — rows exist only after accept per S-E04.3/S-E16.2 ([[acceptance-standards#GATE-AI-2]]); the confirm-first resolution of this AC's tension is pinned as note MEET-NOTE-1. |
| AC2.3 | Summary generation first-token **< 1.5 s** (AI baseline budget, §3.5), shown async (non-blocking). *(perf test, model-bound)* | [[acceptance-standards#PERF-5]] (MEET-PARAM-2); non-blocking clause backed by the activities chapter's timeline-load test. |
| AC2.4 | For an on-prem profile, transcription + summary complete with **no outbound network egress** (local inference). *(network-isolation test asserting zero external calls)* | Egress/network-isolation test; sovereign profile. |
| AC2.5 | **(user-observable)** After a call/meeting, the rep opens a first-class activity on the timeline showing the transcript, a summary, and extracted action items already turned into tasks — they review and correct an AI-captured record instead of typing notes from memory. A correction the rep makes is visibly kept (the record shows it was human-edited). *(observable; backed by AC2.1/AC2.2)* | Live-stack UAT ([[testing#TEST-LANE-3]]); correction visibility via MEET-EV-2. |

Meeting & transcript intelligence criteria, verbatim (features/07 §5):

| ID | Given/When/Then | Verification |
|---|---|---|
| MEET-AC-1 | **(a)** Given a meeting with ≥1 external attendee on an org we have activity for, the dossier renders the three sections with **every claim carrying a clickable source (activity/deal/firmographic id) — claims with no source in our data are omitted, never invented**. *(deterministic test: assert evidence id on every claim; no-data-attendee fixture → honest "no prior data", 0 fabricated claims)* | Deterministic test; AIUC-04; [[acceptance-standards#GATE-AI-1]]/[[acceptance-standards#STATE-5]]. |
| MEET-AC-2 | Dossier first-token **< 1.5 s p95**, rendered async, never blocking the calendar/record view (§3.5). *(perf test)* | Perf test; MEET-PARAM-1. |
| MEET-AC-3 | **(b)** Given a captured transcript, processing yields **one** proposal (next step + attendees + deal changes); **before confirm, zero writes** occur to the deal/task/contacts. *(test: query → no changes; 🟡 gate)* | Integration lane; [[acceptance-standards#GATE-AI-2]]; AIUC-05. |
| MEET-AC-4 | On confirm, records are written in one action, each with provenance pointing to transcript line(s); a named date → a `task` with that due date + owner appearing on the queue; ignore → no change. *(test)* | Integration lane; [[acceptance-standards#GATE-CORE-3]]; queue arrival owned by tasks-and-work-queue. |
| MEET-AC-5 | **(c)** Each inferred KPI is returned as `{value, evidence_snippet, confidence}` and is **not written to a deal field**; a human override is marked `captured_by=human:*` and remembered as feedback. *(test: assert no deal-field mutation from inference; assert evidence present or signal omitted)* | Integration lane; AIUC-06; [[acceptance-standards#GATE-AI-4]]. |
| MEET-AC-6 | **(d)** "Explain this" for a likelihood returns the specific transcript quote(s)/activity ids it derived from — clickable to source, not a hallucinated rationale. *(test: derivation payload includes source ids)* | Integration lane; AIUC-06. |
| MEET-AC-7 | Any sentiment/coaching output is flagged `post-call`, `text-only`, `draft`, and there exists **no** code path producing a live-call or biometric signal. *(test: response carries the labels; static check: no live/biometric emotion endpoint)* | Static check + integration lane; [[acceptance-standards#GATE-AI-6]]; [[scope#NEVER-7]]. |

Deal-qualification checklist criteria, verbatim (features/07 §5b):

| ID | Given/When/Then | Verification |
|---|---|---|
| MEET-AC-8 | Given a deal with captured activity, each checklist item is returned as `{item, status, evidence_snippet, source_id, confidence}` and an inferred item carries a **non-empty snippet + source id or is shown as an explicit gap** — a populated item with no evidence is a hard failure. *(deterministic test: assert evidence on every auto-confirmed item; no-signal fixture → all items shown as gaps, 0 invented confirmations)* | Deterministic test; AIUC-07 (`RATIFY` ≥ 85% item precision); [[acceptance-standards#GATE-AI-1]]. |
| MEET-AC-9 | The surface lists **only gaps** as action items; auto-confirmed items are collapsed/secondary — the rep is never asked to enter what the agent already inferred. *(test: fully-inferable fixture → 0 action items)* | Integration lane; render surface: deals-and-pipeline (its AC-deal-5). |
| MEET-AC-10 | A human confirm/override stamps `captured_by=human:*`, is retained, and is **not re-prompted** on the next re-check. *(test)* | Integration lane; [[acceptance-standards#GATE-AI-4]]. |
| MEET-AC-11 | The checklist re-computes on new captured activity and never blocks the deal view (async, last-known-state on failure). *(perf + chaos test)* | Perf + chaos test; [[acceptance-standards#STATE-3]]. |

Smart-booking criteria, verbatim (features/07 §5c):

| ID | Given/When/Then | Verification |
|---|---|---|
| MEET-AC-12 | Given a connected calendar, offered slots reflect **real free-busy** (a busy block never appears as bookable), honoring min-notice + buffer + timezone. *(integration test against a seeded calendar; deterministic with a fixed clock)* | Integration lane, fixed clock; AIUC-08. |
| MEET-AC-13 | Given a booking by a **known** email, the meeting is routed to the resolved owner and a `meeting` activity is created linked to the existing person + org + open deal, `captured_by=connector:booking`, in one audit'd transaction emitting one event; an **unknown** email creates person+org via the domain rule (`features/02 §2`) and routes to the configured default. *(test: known fixture → linked to existing deal; unknown fixture → new records + default route)* | Integration lane; MEET-EV-1; [[acceptance-standards#GATE-CORE-5]]. |
| MEET-AC-14 | Before submit, **zero** CRM rows are written from a half-filled form; on confirm, exactly the records above are written. *(test)* | Integration lane; [[acceptance-standards#GATE-AI-2]] analogue on the public surface. |
| MEET-AC-15 | Per-purpose consent captured on the form is persisted as a `person_consent` grant + an append-only `consent_event` proof row (source + policy wording/version); an unchecked purpose yields no grant (default-deny). *(test)* | Integration lane; consent cluster owner data-model/gdpr-platform; MEET-EV-5. |
| MEET-AC-16 | Availability/routing **rules are source-defined** (a change is a PR/diff, not a runtime toggle) per the `04` boundary; a rule change is testable. *(test: rule fixture → expected slot/route)* | Rule-fixture test; source customization, outside the runtime-config boundary. |
| MEET-AC-17 | The booking action is reachable as a governed MCP tool with the same routing + write + audit path as the web page. *(contract test: `x-mcp-tool`)* | Contract test; MEET-WIRE-1/2; tool table owned by byo-agent-and-mcp. |
| MEET-AC-18 | **[P2]** the booking-time dossier carries evidence on every claim or omits it (inherits §5 AC); gap-aware intake asks only un-inferred items (inherits §5b AC). | P2 — not V1; inherits MEET-AC-1/8. |

Deep-research profiler criteria, verbatim (features/07 §5d):

| ID | Given/When/Then | Verification |
|---|---|---|
| MEET-AC-19 | Given a person and a research request, every returned claim (history, angle, relevance) carries a **non-empty source snippet + source id or is omitted** — an ungrounded line is a hard failure (no-guess gate). *(deterministic test: assert evidence on every line; no-public-info fixture → honest "couldn't find", 0 fabricated claims)* | Deterministic test; AIUC-09; [[acceptance-standards#GATE-AI-1]]. |
| MEET-AC-20 | The profiler **only** uses public information and creates **no** special-category inference; a request that would require non-public or special-category data returns an honest gap, not a guess. *(test: special-category probe → omitted)* | Probe test; [[threat-model#TM-DPIA-2]]; [[scope#NEVER-8]]. |
| MEET-AC-21 | Where the person/their org is already in the graph, the profile surfaces **our existing relationship + mutuals** with the contact/activity ids. *(test: in-graph fixture → relationship shown with ids)* | Integration lane. |
| MEET-AC-22 | The sovereign profile completes the profiler with **zero external egress**; the default profile secret-strips externally-fetched, model-bound payloads. *(egress test, inherits §11 gate 5)* | Egress test; [[acceptance-standards#GATE-AI-5]] lane per ai-evals. |
| MEET-AC-23 | Output renders the Art. 50 AI-assisted disclosure (§11 gate 9). | UI check; [[acceptance-standards#GATE-AI-9]] lane per ai-evals. |

Booking screen ACs, verbatim (30-screen-acceptance, book section — owned here):

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-book-1 | Given a recognized existing contact, When the page loads, Then a "Welcome back, \<name\>" banner shows their company, an "existing contact" chip, and "we recognized you from your email — nothing to re-enter." | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-book-2 | Given a recognized contact with an open deal, When the page loads, Then a routing annotation states the meeting is routed to the named account lead "because you have an open deal … Not a random round-robin." | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-book-3 | Given the host/context panel, When it renders, Then it shows host, meeting type, duration, "Google Meet — link on confirm," "Free-busy checked live · no double-booking," and a "What we already know about \<company\>" list ending with "we'll only ask you one thing." | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-book-4 | Given the duration toggle (30 / 60 min), When switched, Then the selected duration highlights and the host panel's duration label updates. | Live-stack UAT ([[testing#TEST-LANE-3]]); slot re-fetch gap pinned MEET-GAP-7. |
| AC-book-5 | Given the slot picker, When the user selects a day, Then that day highlights and the times column shows that day's open slots with a timezone note; each day shows its open-slot count. | Live-stack UAT ([[testing#TEST-LANE-3]]); slots per MEET-AC-12. |
| AC-book-6 | Given a time slot, When clicked, Then the flow advances to step 2 (intake) showing the picked slot; a "Back to times" control returns to step 1. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-book-7 | Given the intake step, When it renders, Then it asks exactly one open question ("Anything you want to make sure we cover?") noting "we already pulled the rest from your account," and states the booking "books straight onto your deal." | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-book-8 | Given the consent block, When it renders, Then it presents two separately-checkable purposes — meeting contact (checked, "always on for a booking") and product/marketing email (unchecked) — each recorded separately with its wording + timestamp, withdrawable anytime. | Live-stack UAT ([[testing#TEST-LANE-3]]); persistence per MEET-AC-15. |
| AC-book-9 | Given step 2, When "Confirm booking" is clicked, Then a confirmation step shows "You're booked, \<name\>", the date/time, and that a calendar invite + Meet link are coming and the booking "is already logged on the \<deal\> deal." | Live-stack UAT ([[testing#TEST-LANE-3]]); write per MEET-AC-13/14. |
| AC-book-10 | Given the confirmed booking, When the done step renders, Then it notes the host gets a one-page brief + a confirmation drafted "in his own voice," and the footer states availability & routing are "defined in code, reviewed like any change." A header note states it runs on your operator's / your own infrastructure (a hosting partner or self-hosted) — never a Gradion-run cloud (A35/ADR-0027). | Live-stack UAT ([[testing#TEST-LANE-3]]); voiced confirmation content: voice-profile/drafting chapters. |

Profiler screen ACs, verbatim (30-screen-acceptance, profiler section — owned here):

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-profiler-1 | Given the ready state, When the profile loads, Then the header shows the run provenance — "Read 6 public sources", a timestamp, "11 claims · 11 cited" — and a persistent "Public sources only" scope badge. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-profiler-2 | Given the ready state, When the user reads any background claim (e.g. c1 "Head of Packaging Quality"), Then the claim carries a clickable source chip naming the public source and a coloured confidence dot (high = green, medium = amber), and clicking the chip reveals the verbatim source snippet with read date and an "open source" link. | Live-stack UAT ([[testing#TEST-LANE-3]]); [[acceptance-standards#GATE-AI-1]]. |
| AC-profiler-3 | Given a background claim, When the user clicks its pencil (Correct) control and submits new text, Then the claim text is replaced, a "corrected by you · original source kept" marker appears next to the citation, and the line is treated as typed-by-you and will not silently revert. | Live-stack UAT ([[testing#TEST-LANE-3]]); [[acceptance-standards#GATE-AI-4]]. |
| AC-profiler-4 | Given a background claim, When the user clicks its dismiss (×) control, Then the line is struck through and dimmed and a toast confirms it "won't reappear on re-run unless its source changes"; clicking again restores it. | Live-stack UAT ([[testing#TEST-LANE-3]]); memory via ai-runtime ledger (MEET-SCHEMA-4). |
| AC-profiler-5 | Given the ready state, When the user reads section 2 conversation angles, Then each angle is grounded on a numbered §1 claim via a citation chip ("grounds on claim 3") that opens the derived evidence snippet, and medium-confidence angles (e.g. the Thomas Reuter mutual connection) are explicitly flagged to phrase as a question, not a claim. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-profiler-6 | Given the profile is on screen, When the user inspects the right rail, Then a "Sources read · 6" list shows each source with its claim count, a "Data boundary" panel states lawful basis (legitimate interest, public only), exclusions, and retention, and a "Save profile to record" control states nothing is written yet (staged). | Live-stack UAT ([[testing#TEST-LANE-3]]); [[acceptance-standards#GATE-AI-2]]. |
| AC-profiler-7 | Given the staged profile, When the user clicks "Save profile to record", Then a toast confirms the profile is saved to the person's record with citations intact and dismissed lines excluded. | Live-stack UAT ([[testing#TEST-LANE-3]]); wire home pending MEET-GAP-6. |
| AC-profiler-8 | Given the "Generating" state, When research runs, Then source-reading rows stream in one at a time (each flipping from a loader to a check as it grounds) above a shimmer skeleton, then auto-advance to the ready state — nothing is shown before it has a source. | Live-stack UAT ([[testing#TEST-LANE-3]]); [[acceptance-standards#STATE-2]]/[[acceptance-standards#STATE-SP-3]]. |

Pinned notes:

| ID | Note |
|---|---|
| MEET-NOTE-1 | The AC2.2 / S-E16.2 tension is resolved as: **extraction is this chapter's, persistence is the proposal lifecycle's.** AC2.2's "action items become task rows" holds *after* human accept — a transcript-extracted action item is staged as a proposal (zero rows pre-confirm per MEET-AC-3, S-E04.3, [[acceptance-standards#GATE-AI-2]]) and becomes a task row on confirm, carrying transcript-line provenance. Fixture tests for AC2.2 assert the rows post-accept, not pre-. |
| MEET-NOTE-2 | Corpus label drift (honest report): the screen→story index and the booking screen's section header both credit the booking screen to **S-E04.1**, but the public self-scheduling story is **S-E04.5** (added 2026-06-20, A16 closure); S-E04.1 is the dossier. The AC-book series is pinned here under S-E04.5, with the screen's one-page-brief line (AC-book-10) the genuine S-E04.1 touchpoint. Docs-layer reconciliation flagged. |
| MEET-NOTE-3 | Build note (pinned from the corpus screen section): the booking prototype models only the recognized-contact happy path — unrecognized/new-visitor, no-open-deal routing fallback, no-availability/all-slots-full, consent-declined, and booking-failure states are missing and must be built per the screen-state floor ([[acceptance-standards#STATE-1]]–[[acceptance-standards#STATE-5]]); whether the host's voiced confirmation needs the host's 🟡 approval before sending is an open corpus question for the drafting chapter. |
| MEET-NOTE-4 | Speaker-filter pin: transcripts are **speaker-filtered to keep only the user's own turns** before any voice-learning use (features/07 §7, verbatim); the consuming corpus and enforcement are the voice-profile chapter's — cited here because the diarized transcript (AC2.1, features/02 §2) is what makes the filter possible. |
