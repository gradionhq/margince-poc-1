---
status: planned
module: backend/internal/modules/agents (overnight reconciliation pass; scheduled and executed by the agent runner)
derives-from:
  - specs/spec/features/07-ai-native-moments.md#8-overnight-agent
  - specs/spec/product/epics/E06-overnight-agent.md
  - specs/spec/contract/formulas-and-rules.md#11-close-date-hygiene--realism-ratified--decisions-a6
  - specs/spec/contract/ai-acceptance-catalog.md
  - margince-poc/docs/subsystems/reconciliation.md
---
# Overnight agent — it cleans your mess while you slept, and stages every change for your morning review

> The scheduled after-hours pass that turns a day of captured calls, mail, and
> meetings into evidence-backed, tier-routed, staged proposals: field corrections,
> new contacts to create, integrity flags where the record contradicts captured
> reality, and stalled-deal recoveries drafted but unsent. Its promise: the rep's
> morning is reviewing a staged batch, never discovering surprise changes — nothing
> outward moves without approval.

## What it's for

Captured activity piles up faster than anyone reconciles it against the records it
touches: a call that should have moved a close date, a number mentioned in a
meeting, a promised follow-up nobody drafted, a deal quietly going dark. Left to
hand-work, that reconciliation is the evening admin reps never do — so the record
rots. This subsystem runs overnight over the day's captured activity and reasons
about what changed, emitting concrete proposals a rep can approve, edit, or reject
first thing in the morning (stories S-E06.1 and S-E06.2). It owns the proposal
shapes, the record-versus-reality integrity check, the stalled-deal recovery
drafting, and the no-guess and tier-routing gates that decide what may be shown and
who must approve it.

Its callers: the [agent-runner](agent-runner.md) schedules and drives the run as
background work; its outputs surface to humans through the approval inbox (owned by
notifications-and-approval-inbox on the [approvals-and-concurrency](approvals-and-concurrency.md)
seam) and the "while you slept" section of the morning brief (morning-brief
chapter). The boundary: this chapter stages; it never commits, sends, or renders.
The deterministic stalled *detector* belongs to
[deals-and-pipeline](deals-and-pipeline.md) ([[deals-and-pipeline#DEAL-FORM-3]]);
this chapter consumes that boolean and owns the reason join, the evidence, and the
drafted recovery.

## Principles it serves

- **P5 — auto-capture over manual entry.** "The record reconciles itself
  overnight": a day of automatically captured activity becomes ready-to-review
  proposals, so the rep confirms instead of typing.
- **P12 — governance designed in.** Every proposal is staged with evidence and a
  confidence, routed by a tier the caller cannot set, and fully audited; approval
  is structural, not bolted on. The morning inbox is the resolution of the P5/P12
  tension — do my admin for me, but never surprise me and never auto-send.
- **P6 — embrace the LLMs.** The reconciliation reasoning is a model reading an
  assembled context view; deterministic rules (the stalled boolean, the close-date
  hygiene flags) and the gates frame and constrain it rather than compete with it.
- **P4 — fast core, async intelligence.** The run is background work that never
  blocks core CRM; its failure degrades to an honest, smaller morning batch.
- **ADR-0007 — the context graph is V1 substrate.** The decision that promoted
  reconciliation (S-E06.1) and stalled-deal recovery (S-E06.2) into V1: both ride
  the capture-to-link graph and the scheduled-autonomy substrate. Only the
  fill-the-calendar SDR stays deferred, and for channel-terms risk, not the graph
  ([[scope#S-E06.3]]).

## How it works

**Reading the day.** The pass reads a day of captured calls, mail, and meetings —
never raw rows. It reaches them through an injected context-assembler seam that
returns a provenance-stamped assembled view, so every fact the reasoning sees is
already attributed to its source. On the event side it is a registered consumer of
the capture and domain streams; the day's window is what arrived since the last
run.

**Reconciliation proposals (S-E06.1).** From that view the reasoning stages field
changes — stage, next step, amount, and a corrected expected close date —
newly-detected contacts to create, and follow-ups to draft. Close-date correction
is not model judgment: the flags (overdue, missing, unrealistic-soon,
unrealistic-stale) and the replacement-date computation are the deterministic
close-date hygiene rule, and its ratified risk policy decides the tier per deal. A
clear-overdue date on an active, low-stakes deal is auto-applied (🟢) with full
provenance, a rollback handle, and a "here's what I changed overnight" record; a
forecast-bearing, late-stage, missing, or unrealistic date gets a provisional
replacement plus a 🟡 confirm; a deal that has gone quiet is downgraded and marked
for review rather than optimistically re-dated. The open-deal
never-a-past-close-date invariant holds after every run regardless of tier
(OVN-PARAM-1..4 cite the forecasting-owned tunables; the policy itself is the cited formula).

**The integrity check — data versus claims (S-E06.1, a2).** Because capture
records mail, calendar, and calls independently of what the rep typed, the run can
cross-check the *record* against *captured reality* and flag contradictions: a
logged call with no matching calendar entry or transcript trace; a deal claiming
"proposal sent" with no outbound mail carrying an attachment; a "meeting" with no
recap; a stage the captured signal does not support. Each flag carries the claim,
the missing or contradicting evidence, and a confidence — and it is a *proposal*,
never an automatic edit: a stage correction is 🟡, never a silent move. A record
the evidence supports produces no flag. This is pure read-and-flag work — no
outbound, low risk — and it is the half of capture only an independent capture
pipeline can do: it turns capture into a data-integrity check.

**Stalled-deal recovery (S-E06.2).** Whether a deal has gone dark is decided by
the deterministic rule owned by deals-and-pipeline — open deals only, idleness on
UTC instants against the sixty-day threshold, suppressed while a recorded customer
"asked to wait" holds ([[deals-and-pipeline#DEAL-FORM-3]],
[[deals-and-pipeline#DEAL-PARAM-1]]). This chapter takes that boolean and adds
what the rule cannot: the *specific* reason (no reply in N days, a missed promised
follow-up, a champion gone quiet) joined to clickable activity evidence, and a
recovery follow-up already drafted — in the rep's voice via
[voice-profile](voice-profile.md), referencing the actual last exchange — staged
unsent (🟡). A deal that merely looks stalled while a wait is recorded is not
falsely flagged; if judgment still surfaces it, the caveat is shown. On approval
the recovery sends and the resulting activity logs with provenance back to the
overnight suggestion; on edit-then-approve, the edited version is what sends. The
same reasoning surfaced on demand, rep-initiated, is deal coaching — owned by the
signals-and-warm-room chapter, not here.

**The no-guess gate.** Every proposal that would be shown must carry a resolvable
source, a non-empty evidence snippet, and a confidence
([[acceptance-standards#GATE-AI-1]]). A proposal failing any of these is dropped,
not shown — the pass would rather stay silent than guess.

**Tier routing.** Each proposal is stamped as agent-sourced and routed by a tier
derived from the action it would take, never set by the caller. Only reversible,
internal, rollback-carrying updates — logging, linking a contact, the clear-overdue
close-date roll — may resolve 🟢 staged-and-applied. Anything outbound or
high-value — a send, an advance toward closed, a money field — is strictly 🟡 and
remains unexecuted until approved; the always-🟡 floor is a server-side tool
property no configuration can lower ([[threat-model#D4]]).

**Staged, never committed.** The only sink is the approvals seam: 🟡 proposals are
staged as approval-requested items and commit nothing until a human decides
([[acceptance-standards#GATE-AI-2]]); an unactioned item expires to auto-reject,
never auto-approve ([[approvals-and-concurrency#APPR-PARAM-1]]), so inaction
commits nothing outward. The morning artifact is a single ranked, grouped batch —
triageable, not a dump — presented in the approval inbox with the "while you
slept" summary and staged-flag counts on the brief; the inbox and brief screen
contracts are [[acceptance-standards#STATE-SP-2]] and
[[acceptance-standards#STATE-SP-1]]. Accepted follow-up proposals become owned
work through the proposal lifecycle described in
[tasks-and-work-queue](tasks-and-work-queue.md).

## What's configurable

- **Close-date correction tunables** — the unrealistic-soon window (OVN-PARAM-1),
  the default stage-velocity fallback (OVN-PARAM-2), the won-deal history floor
  before observed velocity is trusted (OVN-PARAM-3), and the master enable for the
  🟢 auto-apply lane (OVN-PARAM-4) — switching it off reverts every close-date
  correction to 🟡 provisional-confirm. All are source constants, no runtime
  tuning surface (P1).
- **The stalled threshold and wait window** — owned by deals-and-pipeline
  ([[deals-and-pipeline#DEAL-PARAM-1]], [[deals-and-pipeline#DEAL-PARAM-2]]);
  this chapter consumes the rule, never re-derives it.
- **The context-assembler seam** — injected, so the day's activity can come from
  the real assembler or a fixture; the pass only ever sees the provenance-stamped
  view.
- **The stager** — injected; staged proposals land on the approvals seam, and
  where that lives is a deployment concern.
- **The drafting model and voice profile** — recovery drafting is baseline-AI at
  the cut line, upgraded by the voice profile when one is built; with no model
  available the deterministic sweeps (close-date hygiene, stalled detection,
  integrity checks over structural evidence) still stage their flags, and
  generative drafts are omitted rather than guessed.

## Guarantees (enforced)

- **Zero domain writes before accept (🟡 lane).** Before a human decision, 🟡
  proposals produce zero rows in real domain tables and zero writes to any record
  field — no send occurs, no deal-advance or money field moves
  ([[acceptance-standards#GATE-AI-2]], OVN-AC-2).
- **The 🟢 lane is narrow and reversible.** Only reversible internal updates may
  apply overnight, and only carrying a rollback handle plus a "done overnight,
  here's what I changed" record; the D4 floor names (send, outbound, archive,
  merge, disqualify, close-deal, enrich) can never resolve 🟢
  ([[threat-model#D4]]).
- **No outbound without an approval token.** Any send executes only through the
  🟡 gate with a recorded human approval, enforced at the tool-contract tier
  ([[acceptance-standards#GATE-AI-7]], [[approvals-and-concurrency#APPR-AC-7]]).
- **Evidence or omission.** Every staged item carries the proposed change, a
  non-empty evidence snippet, and a confidence, or it is dropped
  ([[acceptance-standards#GATE-AI-1]], OVN-AC-1).
- **Integrity flags propose, never apply.** A data-versus-claims contradiction
  becomes an evidenced flag and, where relevant, a 🟡 stage-correction proposal —
  never a silent move; a supported record produces no flag (OVN-AC-4).
- **A recorded wait suppresses a stall, with the asymmetry preserved.** An
  asked-to-wait deal is not falsely flagged stalled (or is flagged with the caveat
  shown), while a past close date on the same deal still takes the 🟡 provisional
  path — a paused deal may wait, but may not claim a past close date (OVN-AC-6).
- **Everything is attributed and audited.** Every proposal and every 🟢 applied
  change is attributed to the overnight agent, distinguishable from human entry,
  with one append-only audit row and one domain event per accept, commit, or
  override ([[acceptance-standards#GATE-AI-3]], OVN-AC-8).

## Acceptance

Done means: after a day of captured activity, the run completes and the rep's
morning inbox holds a ranked, grouped batch — field changes, new contacts,
integrity flags, stalled-deal recoveries with drafts — each item showing proposed
change, evidence, and confidence, with one-tap approve, edit, or reject; nothing
outward has happened yet, and what the agent did apply overnight is reversible and
plainly listed. Honest states are part of the contract: a quiet day renders an
honest "nothing needed" morning rather than padding; a failed or degraded run
surfaces smaller and says so; the inbox's read-only, expiry, and
failed-downstream-execution states are inherited from
[[acceptance-standards#STATE-SP-2]] and the brief's empty states from
[[acceptance-standards#STATE-SP-1]]. Testable forms are pinned in the Acceptance
appendix (OVN-AC-1..9); the cross-cutting floors inherit from the
acceptance-standards chapter and are not restated.

## Out of scope

- **The fill-the-calendar virtual SDR (S-E06.3)** — deferred to Fast-follow for
  outreach-channel terms-of-service risk, not for the graph; its single home is
  the scope chapter's deferred list ([[scope#S-E06.3]]). V1's only obligation is
  negative: no unattended multi-channel auto-send path ships (OVN-AC-9).
- **On-demand deal coaching** — the rep-initiated companion to the stalled-deal
  recovery (story S-E08.5) belongs to the signals-and-warm-room chapter; it shares
  this chapter's reasoning but is a different surface and story.
- **The scheduler and run mechanics** — triggers, retries, run provenance, and the
  autonomy substrate are the [agent-runner](agent-runner.md)'s; this chapter is a
  pass the runner executes.
- **The approval inbox surface and token mechanics** — the queue UI is
  notifications-and-approval-inbox; staging, decision, expiry, and token binding
  are [approvals-and-concurrency](approvals-and-concurrency.md).
- **The stalled detector and its thresholds** —
  [[deals-and-pipeline#DEAL-FORM-3]]; **brief ranking and the morning surface** —
  the morning-brief chapter.

## Where it lives

A reconciliation pass inside the backend's agents module — a leaf that imports no
relational core: the day's activity arrives through the injected context-assembler
seam and every output leaves through the injected stager onto the approvals seam.
Read next: [agent-runner](agent-runner.md) (what executes it),
[approvals-and-concurrency](approvals-and-concurrency.md) (what guards its
output), [deals-and-pipeline](deals-and-pipeline.md) (the stalled rule it
consumes), and [tasks-and-work-queue](tasks-and-work-queue.md) (where accepted
follow-ups become work).

## Appendix

### Parameters
Source: contract/formulas-and-rules.md#11-close-date-hygiene--realism-ratified--decisions-a6 @ 5a0b29c

The close-date tunables are owned by the forecasting chapter — cited, not pinned here:

| ID | Owned pin | Name |
|---|---|---|
| OVN-PARAM-1 | [[forecasting#FCAST-PARAM-7]] | `CLOSE_DATE_UNREALISTIC_SOON_DAYS` |
| OVN-PARAM-2 | [[forecasting#FCAST-PARAM-8]] | `CLOSE_DATE_STAGE_DAYS` |
| OVN-PARAM-3 | [[forecasting#FCAST-PARAM-9]] | `CLOSE_DATE_MIN_HISTORY` |
| OVN-PARAM-4 | [[forecasting#FCAST-PARAM-10]] | `CLOSE_DATE_AUTOAPPLY` |

Note OVN-PARAM-N-1: the stalled threshold (60 days) and the dateless-deferral wait
window (90 days) are pinned as [[deals-and-pipeline#DEAL-PARAM-1]] and
[[deals-and-pipeline#DEAL-PARAM-2]] — cited, not owned here. Note OVN-PARAM-N-2:
this chapter owns no tables; per the ownership index
([[data-model#schema--ownership-index]]) its outputs land in `approval_item`
(notifications-and-approval-inbox), `brief_run`/`brief_item` (morning-brief), and
domain tables owned by their feature chapters — commits happen only through the
approvals seam.

### Wire
Source: contract/crm.yaml (Approvals tag) @ 5a0b29c

The run itself has no wire surface — it is scheduled background work driven by the
agent runner, not an endpoint. Its human-facing output rides the approvals
contract (operations owned by approvals-and-concurrency; cited, not restated):

| ID | Operation | Notes |
|---|---|---|
| OVN-WIRE-1 | `listApprovals` / `getApproval` | The morning-batch read: staged items with proposed change + evidence + confidence; `overnight` is among the contract's enumerated `kind` filter examples ([[approvals-and-concurrency#APPR-WIRE-3]]). |
| OVN-WIRE-2 | `approveApproval` / `rejectApproval` | Approve (optionally edited) commits with token mint; reject discards ([[approvals-and-concurrency#APPR-WIRE-4]], [[approvals-and-concurrency#APPR-WIRE-5]]). |

Honest gaps:

| ID | Gap | Notes |
|---|---|---|
| OVN-GAP-1 | No rollback operation for 🟢 applied items | The feature spec promises every 🟢 overnight change a rollback handle and a one-action undo, but the contract defines no undo/rollback operation; the build atoms for this epic (product/20-traceability.md B-E06.* rows @ 5a0b29c) must land the contract shape or pin reversal to the standard entity-update path with audited provenance. |
| OVN-GAP-2 | No run-status read | Nothing on the wire exposes overnight-run state (last run, window covered, failure); the morning surfaces render from staged output only. Acceptable for V1 (the artifact is the batch), noted for the agent-runner chapter's run-provenance work. |

### Events
Source: contract/events.md#6-consumption-summary--who-reacts-to-what @ 5a0b29c; contract/events.md#55-activity @ 5a0b29c; contract/events.md#56-approval-the--confirm-first-gate-03b-l1 @ 5a0b29c

Event definitions live in the central event catalog; cited, never redefined.

| ID | Event(s) | Role here |
|---|---|---|
| OVN-EVT-1 | `activity.captured`, `activity.updated`, `deal.*`, `lead.*`, `approval.*` | Consumed via the catalog's overnight consumer group (`cg:overnight-agent`) to compute what changed, the stalled/field-hygiene sweep, and the ranked batch. |
| OVN-EVT-2 | `approval.requested` | Emitted for every 🟡 proposal, carrying the proposed effect and dry-run diff; this is the staging event the inbox renders. |
| OVN-EVT-3 | `approval.decided` | Consumed by the agents module to execute an approved (possibly edited) effect with the minted token; definition and semantics owned by the approval pair in the catalog. |
| OVN-EVT-4 | `deal.updated` + `audit.appended` | The 🟢 auto-applied close-date correction commits as a normal audited domain mutation with agent provenance — one audit row + one domain event per commit ([[acceptance-standards#GATE-CORE-5]]). |

### Acceptance
Source: features/07-ai-native-moments.md#8-overnight-agent @ 5a0b29c; product/epics/E06-overnight-agent.md @ 5a0b29c; contract/ai-acceptance-catalog.md @ 5a0b29c

Story primacy verified against product/20-traceability.md @ 5a0b29c: S-E06.1 and
S-E06.2 (both V1-WOW) are owned here and by no other chapter; S-E06.3 is deferred
with its single home at [[scope#S-E06.3]]. No screen in
product/30-screen-acceptance.md @ 5a0b29c names an S-E06 story — the output
surfaces belong to the inbox ([[acceptance-standards#STATE-SP-2]]) and brief
([[acceptance-standards#STATE-SP-1]]) owners. The on-demand coaching criteria
(feature §8b, story S-E08.5, catalog row AIUC-18) belong to the
signals-and-warm-room chapter and are deliberately absent here.

Condensed story acceptance (full Given/When/Then in the epic):

| ID | Given/When/Then | Verification |
|---|---|---|
| S-E06.1 | Given a day of captured calls/mail/meetings, when the overnight run completes, then field changes, new contacts, and follow-ups exist staged in an approval inbox — none committed — each with proposed change + evidence + confidence and approve/edit/reject; low-risk reversible internal updates may be 🟢 applied with a rollback-able "done overnight" record; everything is ranked/grouped and fully audited. | OVN-AC-1..4, OVN-AC-8 |
| S-E06.2 | Given a deal past its expected cadence, when the run completes, then it is flagged stalled with the specific reason and activity evidence, a recovery is pre-drafted in the rep's voice and unsent (🟡); an asked-to-wait deal is not falsely flagged; approval sends with provenance to the suggestion. | OVN-AC-5..7 |

Feature §8 acceptance criteria, verbatim:

| ID | Given/When/Then | Verification |
|---|---|---|
| OVN-AC-1 | Given a day of captured calls/mail/meetings, when the overnight run completes, then proposed field changes (stage/next-step/amount), new contacts to create, and drafted follow-ups exist staged in an approval inbox — and every overnight change appears in the morning approval inbox attributed to the agent, each with proposed-change + evidence + confidence, one-tap approve / edit / reject. | Integration test: post-run → items present, attributed `captured_by=agent:overnight`, each carrying non-empty evidence + confidence; inaction commits nothing outward |
| OVN-AC-2 | No outward / irreversible action executes without a 🟡 confirm — before approval, zero sends occur and zero writes hit a deal advance / money field; reversible internal updates (logging, linking) may be 🟢 applied only if they carry a rollback handle and a "done overnight" record. | Integration test: query → 0 outbound / 0 high-value writes pre-approval; 🟢 items are reversible + logged; 🟡 gate per the tool-contract tier ([[acceptance-standards#GATE-AI-7]]) |
| OVN-AC-3 | The inbox is ranked/grouped, not an undifferentiated dump. | Noisy-run fixture → items grouped/ranked |
| OVN-AC-4 | (a2) Data-vs-Claims: given a captured day that contradicts the record (a logged call with no calendar/transcript trace; a "proposal sent" stage with no matching outbound email; a stage unsupported by signal), the run surfaces an integrity flag carrying the claim + the missing/contradicting evidence + confidence, and any stage correction is a 🟡 proposal, never an automatic move; a record that is supported produces no flag. | Deterministic test: untraced-call fixture → flag with evidence; supported-record fixture → 0 flags; stage correction requires approval token (catalog floor AIUC-16) |
| OVN-AC-5 | Given a deal past its expected cadence, it is flagged stalled with the specific reason (no-reply-N-days / missed-follow-up / champion-quiet) backed by clickable activity evidence, and a recovery follow-up is pre-drafted, unsent (🟡), referencing the real last exchange. | Deterministic test: flag carries reason + evidence id; draft exists + is unsent (stalled boolean from [[deals-and-pipeline#DEAL-FORM-3]]; catalog floor AIUC-17) |
| OVN-AC-6 | Given a deal that looks stalled but the record shows "customer asked us to wait", it is not falsely flagged (or flagged with the caveat). | Asked-to-wait fixture → no false stall, or caveat present |
| OVN-AC-7 | On approve of a recovery/follow-up, it sends and the activity logs with provenance tying it to the overnight suggestion; on edit-then-approve the edited version sends. | Integration test through the approvals seam ([[approvals-and-concurrency#APPR-AC-4]]) |
| OVN-AC-8 | Every action taken or proposed has an audit trail: which agent, under whose authority, the inputs, and the approval state — one append-only audit row + one domain event per accept/commit/override. | Audit-completeness test per P12 ([[acceptance-standards#GATE-CORE-5]]) |
| OVN-AC-9 | The fill-the-calendar workout is explicitly marked Fast-follow on this surface and there is no unattended multi-channel auto-send path in V1. | Static check: no autonomous outbound-SDR send path ships; the deferred story's remaining criteria travel with [[scope#S-E06.3]] |

Note OVN-AC-N-1: the catalog pins eval floors for this chapter — AIUC-15
(reconciliation precision ≥ 85%, 0 outbound/high-value writes pre-approval, 🟢
items carry rollback handles), AIUC-16 (integrity-flag precision ≥ 85%, 0 silent
stage moves, 0 false flags on supported records), AIUC-17 (stall-reason accuracy ≥
80%, 0 false stalls on asked-to-wait, 0 auto-sent recoveries). Note OVN-AC-N-2:
the accept-to-persist floor ([[acceptance-standards#GATE-AI-2]]) governs the 🟡
lane absolutely; the 🟢 lane is the corpus-sanctioned exception for
reversible-internal-with-rollback only, and the always-🟡 floor names can never
enter it ([[threat-model#D4]]).
