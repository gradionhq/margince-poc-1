---
status: planned
module: backend/internal/modules/people (lead-scoring rules over the lead domain; no screen of its own — the score renders in the leads screen)
derives-from:
  - specs/spec/features/03-reporting-and-scoring.md#3-lead-scoring--routing
  - specs/spec/contract/formulas-and-rules.md#3-lead-scoring--transparent-weighted-signal-model
  - specs/spec/contract/formulas-and-rules.md#0-parameter-registry-all-tunables-one-place
  - specs/spec/features/01-core-objects.md#63-lead-scoring--routing-applies-the-existing-3-model-to-the-lead-object
  - specs/spec/product/epics/E13-leads-and-qualification.md#s-e136--add-a-signal-i-know-but-the-system-cant-auto-fetch-manual-scoring-input
  - specs/spec/contract/runtime-config-surface.md#1-shipped-runtime-configuration-surfaces-normative-exhaustive
---
# Lead scoring — a transparent weighted score that always explains itself

> The scoring subsystem turns each lead's firmographic fit and captured engagement into
> one 0–100 number (LEADSCORE-PARAM-1) a rep can triage by — and every point in that
> number is traceable to a named factor, a weight, and the source records behind it.
> It is a fixed, opinionated weighted-signal model, never a trained black box; only its
> weight values are runtime-tunable ([[runtime-config#RC-3]]).

## What it's for

A rep facing a machine-sourced lead list needs a fast, honest answer to "who first, and
why?" — without trusting an opaque number and without hand-entering the data that
justifies it. This subsystem computes that answer: a deterministic score built from the
lead's static fit and its decayed, auto-captured engagement, decomposable on demand into
the exact factors and source events that produced it, plus a routing decision that says
who owns the lead and why. Its callers are the leads screen owned by
[[leads-and-qualification]] (the score block, its explain popover, and sort-by-score),
the capture paths whose new signals trigger incremental recomputes, the routing and SLA
machinery acting on new leads, and reps supplying two kinds of human input: a manual
scoring signal the system cannot auto-fetch (story S-E13.6) and the Commercial Judgement
score override (ADR-0053). The lead object itself, its list, and its promotion are owned
by [[leads-and-qualification]]; this chapter owns the model.

## Principles it serves

- **P6 — a transparent weighted model, not reimplemented frontier ML.** V1 ships a
  fixed sum-of-weighted-factors model whose breakdown always equals its total; trained
  per-workspace scoring is explicitly out.
- **P5 — capture feeds the score, not manual data entry.** Behavioral points come from
  auto-captured activities; the one sanctioned manual path (S-E13.6) is a first-class,
  provenance-tagged signal, not a data-entry burden.
- **P12 — explainable and audited.** Every score decomposes to factors with source
  records ("Explain This Score"); every routing decision and every human override or
  manual input is audit-logged with its reason and provenance.
- **P1 / P2 — one opinionated model; weights are the only knob.** Teams tune weight
  values at runtime ([[runtime-config#RC-3]]); new factors or new scoring logic are a
  source-level edit of the scoring handler, never a scoring-config UI.
- **P11 — segregation holds inside scoring.** A lead's score reads lead-local signals
  only; it never reads or writes the contact graph's relationship strength
  ([[leads-and-qualification]] pins the boundary as LEADS-AC-17).
- **ADR-0053 — Commercial Judgement.** The named human override: it requires a written
  reason, is flagged in history with the prior computed value retained, and suppresses
  recompute until cleared — the same mechanism the partner-fit surface uses.

## How it works

**One formula, two halves.** The score is the clamped sum of static fit points and
decayed behavioral points (LEADSCORE-FORM-1). Fit points reward a decision-maker title,
a target company-size band, and a high-intent source, and penalize low-intent bulk
sources. Behavioral points accrue per captured engagement event and decay exponentially
with a fourteen-day half-life (LEADSCORE-PARAM-2) — the same decay primitive the
relationship-strength baseline uses, pinned once here for scoring. The result is clamped
to 0–100 (LEADSCORE-PARAM-1); a lead with no engagement is honestly a pure-fit cold
lead, never inflated.

**Honest signals only.** Which events may feed the behavioral half is constrained by
the corpus's honest-signal ruling (RT-PR-H2): V1 engagement is inbound replies and
meetings booked or held — never email opens, whose tracking the product deliberately
does not ship (LEADSCORE-FORM-2). Link-click points are a fast-follow gated on the
deferred engagement-event store this chapter will own when it lands
([[data-model#DM-DEF-4]]); until then they contribute nothing and the score degrades
gracefully rather than pretending. Deal-room views feed the broader scoring surfaces
but can never feed a raw lead's score, because a lead cannot carry a deal until it
promotes ([[leads-and-qualification]]).

**Recompute is incremental, deterministic, and idempotent.** A newly captured signal
against a lead triggers a single-record recompute inside the synchronous budget
([[acceptance-standards#PERF-R8]]); full-workspace recomputes run as an asynchronous
background job off the hot path ([[acceptance-standards#PERF-R9]]). Decay is evaluated
from each event's occurrence time at recompute, so re-running under a fixed clock always
yields the same value — the incremental and batch paths agree by construction.

**Every score explains itself.** Alongside the integer, the model returns a factor
breakdown — each factor, its points, and the source records behind it — whose sum
equals the score exactly (the golden test of AC-S1). The leads screen renders this as
the score popover with the raw-to-decayed arithmetic visible ([[leads-and-qualification]]
AC-leads-4, AC-leads-10); the same decomposition is promised over the API (AC-S7),
which is a contract gap carried honestly below (LEADSCORE-AC-OPEN-2).

**Humans can say what capture can't.** A rep who knows a qualification signal the
system cannot auto-fetch — a traffic band, an employee count, a budget hint — enters it
as a manual scoring input for a named factor the model knows (S-E13.6). It feeds the
same weighted formula as auto-captured signals, appears in the explanation as a
distinct human-provided factor with the rep as its source, and is never silently
blended with or overwritten by auto-captured data: when a factor later becomes
auto-fetchable, the auto-versus-manual precedence is explicit and shown. Every entry,
change, or clearing of a manual input is audit-logged and recomputes deterministically.
This path is what makes the score useful on a still-cold lead, where the strong
behavioral signals (a reply, a meeting) tend to promote the lead the moment they occur.

**Or overrule the model entirely.** Commercial Judgement (ADR-0053) sets the score by
hand: it demands a non-empty written reason, is flagged in score history with the prior
computed value retained, and suppresses recompute until the override is cleared — an
override is never quietly recomputed away, and an override without a reason is rejected.

**Routing follows the score's world.** New leads are assigned by enumerated rule types
— territory, segment, deal-size band, source, round-robin within a team — with capacity
caps that are never exceeded, reassignment on owner change or out-of-office, and SLA
timers that escalate when a lead sits untouched. The decision is synchronous on capture
within its budget ([[acceptance-standards#PERF-R10]]) and every decision is
audit-logged with the rule that fired. Bespoke routing logic beyond the enumerated
types is a typed workflow handler in source, not configuration.

## What's configurable

- **Weight values** — the numeric weights on the fixed factor set (fit points and
  behavioral base points, LEADSCORE-PARAM-3..11) are the model's one bounded runtime
  surface, [[runtime-config#RC-3]]: values only, on the shipped factors only. New
  factors or different logic are a source-level scoring-handler edit (P2). The corpus
  parameter registry's blanket "no runtime tuning" note predates the ratified register
  row; the register wins (see the Parameters reconciliation note).
- **Everything else is a source constant** — the half-life (LEADSCORE-PARAM-2), the
  clamp ceiling (LEADSCORE-PARAM-1), the intent-source sets and the decision-maker
  title pattern (LEADSCORE-PARAM-12..14) change by code edit and redeploy, auditable in
  one rules package.
- **Routing rules and SLA windows** — bounded runtime surfaces of their own,
  [[runtime-config#RC-4]] and [[runtime-config#RC-5]], registered against the
  leads-and-qualification surface; this chapter specifies the decision behavior those
  values parameterize.
- **The engagement-event store** — behavioral click points depend on a deferred table
  ([[data-model#DM-DEF-4]], owner-on-arrival: this chapter). Absent, those terms
  contribute zero and the score is honestly fit-plus-replies-plus-meetings; nothing
  errors and nothing pretends.

## Guarantees (enforced)

- **The breakdown always equals the score.** Computed score = sum of its weighted
  factors on a fixed fixture set under the fixed test clock; asserted by the AC-S1
  golden test.
- **Opens never enter the score.** No email-open event contributes points, ever — the
  honest-signal ruling is normative (LEADSCORE-FORM-2) and scoring copy never promises
  opens as a signal.
- **Recompute is deterministic and idempotent.** Same inputs and clock, same score, on
  both the incremental and batch paths (AC-S2/AC-S3 agree).
- **Signal arrival recomputes within budget.** Single-record recompute stays inside
  [[acceptance-standards#PERF-R8]]; batch recompute is an async job inside
  [[acceptance-standards#PERF-R9]]; neither blocks capture.
- **Human inputs are first-class and never silently lost.** A manual scoring input
  survives recompute as a distinct, provenance-tagged factor (S-E13.6); a Commercial
  Judgement override suppresses recompute until cleared and is rejected without a
  written reason (AC-S1).
- **Scoring never touches the contact graph.** Lead scores read lead-local signals
  only; the segregation assertion is pinned by [[leads-and-qualification]]
  (LEADS-AC-17) and holds inside this subsystem.
- **Routing is fair, capped, and accountable.** Round-robin distributes within ±1,
  capacity caps are never exceeded, decisions land inside
  [[acceptance-standards#PERF-R10]], and every decision is audit-logged (AC-S5).

## Acceptance

Done means: a rep sorting the lead list by score can click any score and read exactly
why it is what it is — which fit factors fired, which engagement events contributed how
many decayed points, and where each came from — with the arithmetic visibly summing to
the total. Entering a signal they know by hand updates the score and shows up labeled
as theirs; overriding the score demands a reason and visibly sticks until cleared. A
lead with no engagement shows an honest pure-fit score, and no score anywhere implies
an email-open signal the product does not capture. Routing visibly names the owner and
the rule that chose them. The score's on-screen rendering, its color thresholds, and
the popover's copy are the leads screen's acceptance rows, owned by
[[leads-and-qualification]] (AC-leads-4, AC-leads-8, AC-leads-10) — cited, not
restated; the cross-cutting screen-state floor and performance budgets are inherited
from [[acceptance-standards]].

Three gaps are carried honestly for ticket time: the corpus's own opens contradiction
(LEADSCORE-AC-OPEN-1), the missing contract surface for explanation, manual input,
override reason, and score history (LEADSCORE-AC-OPEN-2), and the absent named scoring
fixture (LEADSCORE-AC-OPEN-3).

## Out of scope

- **The lead object, its list and screen, promotion, and the segregation schema** —
  owned by [[leads-and-qualification]]; the score is one column on its table
  (LEADS-DDL-1) and one block on its screen.
- **Routing/SLA configuration values** — the bounded register rows
  [[runtime-config#RC-4]] and [[runtime-config#RC-5]].
- **Relationship strength** for real contacts — a different deterministic formula,
  owned by [[people-and-organizations]]; leads are excluded from it by construction.
- **Deal-health and signal scores** — sibling transparent weighted models with their
  own formulas: the signal score is owned by [[signals-and-warm-room]]; the deal-health
  model (AC-S8 rides it) lands with the E09 reporting/forecasting chapters.
- **The event envelope and delivery semantics** — owned by [[event-bus]].
- Trained per-workspace ML scoring, predictive lead-to-customer models, geo/territory
  routing UI, and routing simulation — out for V1 per the feature cut line.

## Where it lives

Backend: the scoring rules live with the lead domain in the shared core-objects module
(`backend/internal/modules/people`), reached through the lead operations and recompute
jobs; the tunables sit in the one auditable rules package the corpus mandates. There is
no frontend of its own — the score renders inside the leads feature owned by
[[leads-and-qualification]]. Read next: [[leads-and-qualification]] (the object and
screen this model serves), [[runtime-config]] (the RC-3 boundary), and
[[acceptance-standards]] (the PERF-R8..R10 budgets).

## Appendix

### Parameters
Source: specs/spec/contract/formulas-and-rules.md#0-parameter-registry-all-tunables-one-place @ 5a0b29c; specs/spec/contract/formulas-and-rules.md#31-formula @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| LEADSCORE-PARAM-1 | `LEADSCORE_MAX` | `100` | Score is clamped to 0–100; the clamp also floors negative-fit-only leads at 0. |
| LEADSCORE-PARAM-2 | `LEADSCORE_BEHAVIORAL_HALFLIFE_DAYS` | `14` | Exponential half-life on behavioral points: `2^(−days/14)`, the shared decay primitive. |
| LEADSCORE-PARAM-3 | `LEADSCORE_WEIGHTS` · fit: decision-maker title | `+15` | Title matches the decision-maker pattern (LEADSCORE-PARAM-14). |
| LEADSCORE-PARAM-4 | `LEADSCORE_WEIGHTS` · fit: target-size band | `+10` | Company resolves to a target-size band (heuristic: ≥51 employees if known). Inert until a lead firmographic size column exists — contributes 0 today (corpus column-readiness note). |
| LEADSCORE-PARAM-5 | `LEADSCORE_WEIGHTS` · fit: high-intent source | `+8` | `source ∈ HIGH_INTENT_SOURCES` (LEADSCORE-PARAM-12). |
| LEADSCORE-PARAM-6 | `LEADSCORE_WEIGHTS` · fit: low-intent source | `−5` | `source ∈ LOW_INTENT_SOURCES` (LEADSCORE-PARAM-13). |
| LEADSCORE-PARAM-7 | `LEADSCORE_WEIGHTS` · behavioral base: reply | `25` | Inbound reply from the lead (real `activity` direction column; live in V1). |
| LEADSCORE-PARAM-8 | `LEADSCORE_WEIGHTS` · behavioral base: meeting_held | `30` | Meeting held with the lead (live in V1). |
| LEADSCORE-PARAM-9 | `LEADSCORE_WEIGHTS` · behavioral base: meeting_booked | `20` | Meeting booked with the lead (live in V1). |
| LEADSCORE-PARAM-10 | `LEADSCORE_WEIGHTS` · behavioral base: link_click | `4` | Fast-follow: gated on the deferred engagement-event store ([[data-model#DM-DEF-4]]); contributes 0 until it ships (LEADSCORE-FORM-2). |
| LEADSCORE-PARAM-11 | `LEADSCORE_WEIGHTS` · behavioral base: email_open | `2` | **Dead weight.** Pinned verbatim from the corpus formula, but open-tracking is deliberately dropped (RT-PR-H2) — this term never fires (LEADSCORE-FORM-2; errata LEADSCORE-AC-OPEN-1). |
| LEADSCORE-PARAM-12 | `HIGH_INTENT_SOURCES` | `{'inbound','webform','referral'}` | Sources granting the high-intent fit bonus. |
| LEADSCORE-PARAM-13 | `LOW_INTENT_SOURCES` | `{'import','crawl'}` | Sources taking the low-intent fit penalty. |
| LEADSCORE-PARAM-14 | decision-maker title pattern | `/(chief|vp|head|director|founder|owner|c[a-z]o)\b/i` | The fixed title heuristic behind LEADSCORE-PARAM-3. |

**Runtime boundary (RC-3).** The weight *values* (LEADSCORE-PARAM-3..11) are the
chapter's one bounded runtime surface — [[runtime-config#RC-3]]: numeric weights on the
fixed factor set; NOT new factors, NOT new scoring logic (source-level handler edit,
P2). Everything else in this table is a source constant. Reconciliation: the corpus §0
registry's preamble says "no runtime tuning UI in v1" with a single named exception,
while the corpus runtime-config inventory ships RC-3 as a live surface
(`contract/runtime-config-surface.md` §1); the register is normative and exhaustive
([[runtime-config#RC-REG-1]]) — RC-3 wins for weight values. Flagged for corpus
registry sync, alongside: §0 does not list `HIGH_INTENT_SOURCES`,
`LOW_INTENT_SOURCES`, or the title pattern (they appear only in the §3.1 tunables
line).

### Formulas
Source: specs/spec/contract/formulas-and-rules.md#31-formula @ 5a0b29c

**LEADSCORE-FORM-1 — the lead score (transparent weighted-signal model, verbatim).**
Inputs: `lead` firmographic fields (`title`, `company_name`, `candidate_org_key`,
`source`) + behavioral signals derived from `activity`/`activity_link` rows linked to
the lead, with `occurred_at` for decay.

```
score = clamp(0, LEADSCORE_MAX,  fit_points + behavioral_points)

fit_points (static, firmographic):
   + 15  if title matches a decision-maker pattern  /(chief|vp|head|director|founder|owner|c[a-z]o)\b/i
   + 10  if company_name / candidate_org_key resolves to a target-size band (heuristic: ≥51 employees if known)
   +  8  if source ∈ HIGH_INTENT_SOURCES   (default: {'inbound','webform','referral'})
   -  5  if source ∈ LOW_INTENT_SOURCES    (default: {'import','crawl'})

behavioral_points (decayed per event):
   Σ over each behavioral activity e:
       base(e) * 2^(-days_since(e.occurred_at) / LEADSCORE_BEHAVIORAL_HALFLIFE_DAYS)
   where base(e):
       reply            = 25
       meeting_held     = 30
       meeting_booked   = 20
       link_click       =  4
       email_open       =  2
```

`clamp` keeps the score in `[0, 100]`. Decay uses the same `2^(−t/halflife)`
exponential as the relationship-strength baseline (one decay primitive across the
codebase — that formula is owned by [[people-and-organizations]]).

Output: integer `lead.score ∈ [0,100]`, plus a factor breakdown
`[{factor, points, source_activity_ids[]}]` returned by "Explain This Score" (AC-S7).
The breakdown sum equals the score (golden test, AC-S1).

Worked example (corpus-given; fixed clock `now = 2026-06-04`, which is the harness
clock [[testing#TEST-DET-1]]):
- Lead: title "VP Sales" (+15), source `webform` (+8). Activities: 1 reply 2 days ago,
  2 link-clicks 10 days ago.
- fit = 23.
- behavioral: reply `25 * 2^(-2/14) = 25*0.906 = 22.6`; clicks
  `2 * 4 * 2^(-10/14) = 8*0.610 = 4.9`. Sum = 27.5.
- score = `clamp(0,100, 23 + 27.5) = 50.5 → 51` (round half-up).
- **Derived (V1-observed variant, not corpus):** with click points inert until the
  engagement-event store lands (LEADSCORE-FORM-2), the same fixture computes
  `clamp(0,100, 23 + 22.6) = 45.6 → 46`. The golden fixture must pin whichever the
  build ships and state why (LEADSCORE-AC-OPEN-3).

Edge cases (verbatim):
- **No behavioral activity:** behavioral_points = 0; score is pure fit (cold lead).
- **Negative fit only:** `clamp` floors at 0 (no negative scores).
- **Commercial Judgement override** (A68/ADR-0053 — the canonical name for the human
  score-adjustment, shared with partner-fit): an explicit human score sets
  `lead.score`, **requires a non-empty written reason**, is flagged in history (with
  the prior computed value retained), and suppresses recompute until the override is
  cleared (AC-S1).
- **Idempotent recompute:** decay is computed from `occurred_at` at read/recompute
  time, so re-running yields the same value under a fixed clock (AC-S2 incremental +
  AC-S3 batch agree).

**LEADSCORE-FORM-2 — the honest signal-set constraint (normative).**
Source: specs/spec/features/03-reporting-and-scoring.md#32-ai-native-upgrade @ 5a0b29c (RT-PR-H2 ruling)

The ruling constrains which events may feed `behavioral_points`; where it conflicts
with the verbatim point table above, **the ruling wins**:

- **In (V1):** inbound replies (`reply`) and meetings booked/held
  (`meeting_booked`/`meeting_held`), read from real activity columns; recency/velocity
  via the decay term; plus manual scoring inputs per S-E13.6 semantics once built.
- **Never:** `email_open` — open-tracking is deliberately dropped (Apple-MPP/ePrivacy,
  corpus BACKLOG §I). The `email_open = 2` row is permanently inert, not deferred;
  scoring and brief copy must not promise opens as a signal (errata
  LEADSCORE-AC-OPEN-1).
- **Fast-follow:** `link_click` — enters only when the deferred engagement-event store
  lands ([[data-model#DM-DEF-4]], owner-on-arrival: this chapter); contributes 0 until
  then (graceful degradation, per the corpus column-readiness note).
- **Not applicable to leads:** deal-room views are part of the honest signal set for
  the contact/deal scoring surfaces, but a raw lead cannot carry a deal
  ([[leads-and-qualification]] LEADS-AC-24), so they never feed a lead score.

### Wire
Source: specs/spec/contract/crm.yaml (paths `/leads`, `/leads/{id}`) @ 5a0b29c

Scoring has **no dedicated contract operations** — carried honestly. The score rides
the lead resource, whose operations are pinned by [[leads-and-qualification]] (cited by
its WIRE ids, never restated):

| ID | Surface | Status |
|---|---|---|
| LEADSCORE-WIRE-1 | Score on the lead body (`lead.score`, integer 0–100) and the `min_score` triage filter on the lead list | Cited — [[leads-and-qualification#LEADS-WIRE-1]] (list), [[leads-and-qualification#LEADS-WIRE-3]] (read) |
| LEADSCORE-WIRE-2 | Commercial Judgement over the wire: the partial-update `score` field ("Manual human score override (formulas §3.1, AC-S1). Omit to keep the computed lead-local score.") | Cited — [[leads-and-qualification#LEADS-WIRE-4]]; **gap:** the request carries no written-reason field, so AC-S1's reason requirement is unverifiable over the current contract (LEADSCORE-AC-OPEN-2) |
| LEADSCORE-WIRE-3 | "Explain This Score" factor decomposition via API (AC-S7) | **Gap** — no operation exists in the contract (LEADSCORE-AC-OPEN-2) |
| LEADSCORE-WIRE-4 | Manual scoring input (S-E13.6) — enter/change/clear a human-provided factor value | **Gap** — no operation exists; the story names it a new build (LEADSCORE-AC-OPEN-2) |
| LEADSCORE-WIRE-5 | Score history (a table-stakes behavior of features/03 §3.1) | **Gap** — no operation and no dedicated store; history currently derivable only from the audit trail and `lead.updated` deltas (LEADSCORE-AC-OPEN-2) |

### Events
Source: specs/spec/contract/events.md#54-lead @ 5a0b29c

Event definitions live in the central catalog ([[event-bus]]) — cited here, not
redefined. What feeds the score, and what the score emits:

| ID | Event | Role for scoring | Cite |
|---|---|---|---|
| LEADSCORE-EVT-1 | `activity.captured` | **Consumed** — the recompute trigger: an inbound email (reply) or a meeting booked/held linked to the lead starts the incremental recompute inside [[acceptance-standards#PERF-R8]] | [[event-bus]] catalog row `activity.captured`; stream [[event-bus#EVT-STREAM-5]] |
| LEADSCORE-EVT-2 | `lead.created` | **Consumed** — initial fit-only score + the synchronous routing decision ([[acceptance-standards#PERF-R10]]); flows to workflows/routing and the segregated lead view only, never the contact graph | [[event-bus]] catalog row `lead.created`; [[event-bus#EVT-SEM-5]] |
| LEADSCORE-EVT-3 | `lead.updated` | **Emitted** — carries the score-recompute delta (and owner changes from routing) to the overnight agent and workflows | [[event-bus]] catalog row `lead.updated`; stream [[event-bus#EVT-STREAM-4]] |
| LEADSCORE-EVT-4 | `lead.promoted` | **Boundary** — the score value is carried onto the resulting person as part of the non-lossy promotion; promotion itself is owned by [[leads-and-qualification]] (LEADS-FORM-5) | [[event-bus]] catalog row `lead.promoted` |
| LEADSCORE-EVT-5 | `engagement.reply` | **Adjacent, cited** — the reply signal is thread-match based, never an open pixel (the bus-level echo of LEADSCORE-FORM-2); its payload is contact-scoped and feeds the warm-room surfaces | [[event-bus]] catalog row `engagement.reply` |

No open-pixel event exists anywhere in the catalog — consistent with LEADSCORE-FORM-2.

### Acceptance
Source: specs/spec/features/03-reporting-and-scoring.md#34-acceptance-criteria @ 5a0b29c; specs/spec/product/epics/E13-leads-and-qualification.md#s-e136--add-a-signal-i-know-but-the-system-cant-auto-fetch-manual-scoring-input @ 5a0b29c

**Owned story** (primacy verified: [[leads-and-qualification]] Acceptance assigns
S-E13.6 to this chapter, and the epic split in [[scope]] lists lead-scoring beside it
for E13; likewise it cedes the inherited AC-S set: "the scoring/routing AC set —
inherited AC-S1..AC-S8 applied to leads — is owned by lead-scoring").

| ID | Story | Tier | Home |
|---|---|---|---|
| S-E13.6 | Add a signal I know but the system can't auto-fetch (manual scoring input) | V1-Must (added 2026-06-24, A50) | this chapter |
| S-E13.2 | Work a scored, routed lead list, segregated from real relationships | V1-Must | [[leads-and-qualification]] — the list; the model it displays is this chapter |

**Feature acceptance criteria (verbatim from the feature spec; applied to leads per
features/01 §6.3).**

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-S1 | **(correctness):** Computed score equals the sum of its weighted factors on a fixed fixture set (golden test); a **Commercial Judgement** override is respected and flagged in history, and (A68/ADR-0053) **requires a non-empty written reason** — an override without a reason is rejected, and the prior computed value is retained. | Backend integration lane (golden test under [[testing#TEST-DET-1]]; fixture gap: LEADSCORE-AC-OPEN-3) |
| AC-S2 | **(signal-driven):** A new auto-captured activity triggers an incremental recompute; single-record recompute p95 < 150 ms. | Performance gate ([[acceptance-standards#PERF-R8]]) |
| AC-S3 | **(batch):** Full-workspace recompute of 100k records completes < 5 min as a River job, off the hot path. | Performance gate ([[acceptance-standards#PERF-R9]]) |
| AC-S4 | **(ABM):** Account score equals the defined roll-up of its contacts' scores via the `relationship` table; matches ground truth (test). | Backend integration lane. For leads the roll-up runs over the loose candidate-company key, never an org row ([[leads-and-qualification#LEADS-DDL-1]]) |
| AC-S5 | **(routing):** Round-robin distributes within ±1 across eligible owners over N assignments; rule routing assigns per the matching rule; capacity caps are never exceeded; routing decision p95 < 250 ms; every decision audit-logged (P12). | Backend integration lane + Performance gate ([[acceptance-standards#PERF-R10]]); rule values: [[runtime-config#RC-4]] |
| AC-S6 | **(SLA):** SLA timer fires escalation within the configured window (deterministic test via River scheduling). | Backend integration lane (deterministic scheduled-job test); window values: [[runtime-config#RC-5]] |
| AC-S7 | **(explainability):** Every score exposes its factor decomposition with source-record links via API and UI. | Backend integration lane + Screen e2e lane; the API operation is a contract gap (LEADSCORE-AC-OPEN-2); UI rows: [[leads-and-qualification]] AC-leads-4/AC-leads-10 |
| AC-S8 | **(user-observable — conversation-inferred deal health):** A leader looking at a deal or account sees a health/score signal built from real captured engagement (replies, meeting attendance, recency) and can click it to read the weighted factors and open the source emails/meetings behind it — the score is never a black-box number they have to take on faith (S-E09.4). | Screen e2e lane (Fast-follow; rides the deal-health model of formulas §10.5, whose home is the E09 reporting/forecasting chapters — cited, not owned here) |

**S-E13.6 — manual scoring input (condensed from the story's user-side acceptance).**

| ID | Given/When/Then | Verification |
|---|---|---|
| LEADSCORE-AC-1 | Given a lead, when I enter a manual scoring input (a typed value or banded pick for a named factor the model knows), then it feeds the same transparent weighted score as auto-captured signals and the score updates with the new input. | Backend integration lane + Screen e2e lane |
| LEADSCORE-AC-2 | Given a manually-entered signal, when I open "Explain this score", then the manual input is shown as a distinct, human-provided factor with its source = me (P5/P12) — never silently blended into or mistaken for an auto-captured signal. | Screen e2e lane |
| LEADSCORE-AC-3 | Given a factor that later becomes auto-fetchable, when the auto signal arrives, then the auto-vs-manual precedence rule is explicit and shown — the rep is never confused about which value drives the score. (The rule itself is undecided in the corpus — LEADSCORE-AC-OPEN-2.) | Screen e2e lane |
| LEADSCORE-AC-4 | Given a manual input, when I change or clear it, then the change is audit-logged and the score recomputes deterministically (re-runnable from the inputs). | Backend integration lane (audit-completeness gate) |

**Screen rows cited, not owned:** the score block, hover popover with the decay
arithmetic, and color thresholds on the leads screen are AC-leads-4, AC-leads-8, and
AC-leads-10, owned by [[leads-and-qualification]]. This chapter owns no screen; the
standard screen-state floor is inherited from [[acceptance-standards]].

**Open items (carried honestly — corpus errata + build-ticket decisions).**

| ID | Decision / errata | Verification |
|---|---|---|
| LEADSCORE-AC-OPEN-1 | **Corpus errata — opens contradiction:** formulas §3.1 pins `email_open = 2` and its column-readiness note calls open/click points "fast-follow", and features/01 §6.3 lists "opens" among the real signals — but the RT-PR-H2 ruling (features/03 §3.2) deliberately drops open-tracking (Apple-MPP/ePrivacy). **Resolution pinned here: the ruling wins — open events never enter the score** (LEADSCORE-FORM-2); the `email_open` weight is dead, `link_click` alone is the fast-follow. Flag the three corpus passages for errata. | Ticket-gate: scoring tickets must not implement an open signal; craft/copy review rejects "opens" in scoring copy |
| LEADSCORE-AC-OPEN-2 | **Contract + storage gaps:** no explain-score API operation (AC-S7); no manual-scoring-input operation or defined store (S-E13.6 names it a new build); the update-lead override field carries no written-reason field and the lead row has no recompute-suppression marker (AC-S1's mechanics); score history has no operation or store beyond audit + `lead.updated` deltas; the auto-vs-manual precedence rule is demanded but not chosen. Each must be resolved at ticket time — contract-first, and any new table must land in the data-model ownership index. | Ticket-gate + contract-drift gate |
| LEADSCORE-AC-OPEN-3 | **No named scoring fixture:** the testing chapter's fixture registry has no lead-scoring golden fixture; AC-S1 needs one pinning the worked example (LEADSCORE-FORM-1) under the fixed clock — and it must choose between the corpus arithmetic (51, includes clicks) and the V1-observed value (46, clicks inert) and say why. | Ticket-gate: the scoring ticket adds the named fixture to [[testing]] Seed |
