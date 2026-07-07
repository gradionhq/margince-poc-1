---
status: planned
module: modules/agents (brief assembly + weekly report) · web (home + weekly-report surfaces)
derives-from:
  - specs/spec/features/07-ai-native-moments.md#6-morning-brief--7-deals-you-can-win-this-week-home
  - specs/spec/features/07-ai-native-moments.md#6b-weekly-bilingual-3p-report-progress--plans--problems
  - specs/spec/contract/formulas-and-rules.md#10-brief-ranking--deterministic-scoring-inputs-feeding-the-morning-brief-rank
  - specs/spec/product/epics/E05-morning-brief.md#e05--morning-brief
  - specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26
  - specs/spec/product/30-screen-acceptance.md#homehtml--morning-brief-implements-s-e0515
  - specs/spec/product/30-screen-acceptance.md#weekly-reporthtml--weekly-bilingual-3p-report-implements-s-e056
  - margince-poc/docs/subsystems/brief.md @ a11d6c08
---
# Morning Brief — the day opens as a short, evidenced action queue, never an empty pipeline

> The signature home surface: the rep opens the CRM to a ranked, finite queue of deals
> they can win this week — each with why it matters, what changed overnight, who's warm
> or blocking, and a next move drafted in their own voice — plus its weekly sibling, the
> bilingual Progress/Plans/Problems report. The formula ranks; the AI explains; nothing
> ever sends itself.

## What it's for

A table-stakes CRM opens to a full pipeline board and leaves "where do I spend this
morning" entirely to the human. This subsystem replaces that with the product bet the
founder leads with: the first thing a rep sees is a short, ranked action queue of
winnable deals, each carrying its reasons and a drafted next move, so the day starts
already triaged. Its surfaces are the home screen (the daily entry point for every rep)
and the weekly-report screen (the scheduled Progress/Plans/Problems roll-up a rep or
leader reviews and explicitly shares). Its callers beyond the UI are the forecasting
chapter, which rolls the same per-deal confidence and revenue impact into the leader's
number, and BYO agents that drive the same goal through the governed tool surface. The
scope boundary: this chapter owns the queue — candidate ranking, per-deal evidence
panels, item state, the weekly report — while the overnight reconciliation that feeds
"what changed", the approval inbox, the voice model, and the send path are sibling
chapters it composes.

## Principles it serves

- **P5 — capture-first, done-for-you.** The day opens already prioritized and drafted;
  the rep reacts to a finished queue instead of filling a board.
- **P12 — governance designed in.** Trust-or-it-dies: every claim on the surface carries
  the evidence it was read from or is omitted ([[acceptance-standards#GATE-AI-1]]); every
  drafted action is confirm-first ([[acceptance-standards#GATE-AI-7]]); people signals
  stay consent-gated and company-level, never covert per-individual profiling.
- **P11 — clean relational core.** Ranks and reasons are computed over real deal columns
  and the typed relationship graph, never over a guess or a metadata blob.
- **P6 — no frontier reinvention.** The AI layer is baseline reasoning over the context
  graph; the deterministic parts stay deterministic.
- **P4 — performance as acceptance.** Assembly is asynchronous and progressive; the home
  route never blocks on it, and the candidate-set query carries its own budget
  ([[acceptance-standards#PERF-7]]).
- **P8 — beautiful by default.** The queue is a legible, finite surface, not a data dump.
- **ADR-0007 — context graph in V1.** The brief was promoted into the V1 line because the
  capability it rides — capture-to-link assembly and cross-pipeline reasoning over the
  relational core, with no dedicated graph store — is committed V1 substrate.

## How it works

**A deterministic core ranks; the AI explains and may reorder within it.** Each of the
rep's live open deals gets a composite score in the zero-to-one range: a weighted blend
of winnability (the stage's win probability), revenue (base-currency value against the
workspace's large-deal norm), timing (bucketed distance to expected close, where overdue
and imminent both read urgent), momentum (did anything arrive on the deal overnight),
and warmth (the relationship strength of its strongest stakeholder). The weights lean
hardest on winnability and revenue, then timing, momentum, warmth (BRIEF-PARAM-3). A
deal joins the queue only if its composite clears the actionability bar
(BRIEF-PARAM-2); the queue is the top handful among those that clear it, targeting
seven items (BRIEF-PARAM-1). Ties break deterministically — larger value, then sooner
close date, then a stable identity tiebreak — so identical inputs and a fixed clock
always produce the identical queue (BRIEF-FORM-1). On top sits an optional model-bound
re-ranker whose honesty is structural, not prompted: its output must be a permutation
of the deterministic candidate set — it cannot inject a deal, drop one, or pull a
below-bar deal above the cutoff — and an item whose cited evidence does not resolve to
supplied source rows falls back to its deterministic position. A misbehaving model
degrades to the deterministic order, never past it; when the AI layer is off, that
order is used as-is. Ranking quality is an eval band, not a merge gate; the gate with
teeth is deterministic ([[ai-evals]] AIEVAL-21/22, AIUC-11).

**Honest-short is a feature, not a failure.** A quiet week yields a genuinely short
queue — padding to the target with stale deals is a defect — and a morning with nothing
actionable renders an honest "nothing needs you this morning" state, never a blank
board and never the raw full pipeline ([[acceptance-standards#STATE-SP-1]]). Stalled
deals with no overnight change and low warmth fall below the bar by construction; the
stalled input itself is [[deals-and-pipeline]]'s pure rule (its DEAL-FORM-3), consumed
here, not recomputed.

**Every panel is discrete, sourced facts — never a prose blob.** Per ranked deal the
brief answers *why it matters* (revenue, timing, strategic fit) and *what changed
overnight* (a reply arrived, a champion viewed the proposal, a renewal date moved),
each as a clickable fact that lands on its exact evidence. Nothing changed → it says so
plainly rather than recycling old context. A change the system can't confidently
interpret is shown as the raw fact flagged uncertain, never a confident wrong read.
The people picture labels each key person warm, neutral, or blocking with the evidence
for the label, and surfaces the buyer's hidden priority only when grounded in something
observed — a transcript line, a repeated theme, a viewed asset. A wrong label is
correctable in one action and the correction is remembered, so the same person is not
mislabeled on the next run; people signals stay consent-gated and company-level per
P12. Everywhere the same discipline holds: a claim carries a non-empty evidence snippet
and a resolvable source, or it is omitted ([[acceptance-standards#GATE-AI-1]]).

**The next move arrives drafted, in the rep's voice, and never sends itself.** Each
queue item carries one recommended next move and, where that move is a message, a draft
already written in the rep's register — the Voice DNA model of [[voice-profile]],
applied through [[drafting]]'s generation path. The rep's choices are approve, edit, or
send; send is an explicit human act, an edited draft is what goes out, and the logged
activity carries provenance back to the brief suggestion. Moves that touch money or an
irreversible step are flagged and gated harder. No confidence level ever relaxes this
([[acceptance-standards#GATE-AI-7]]; [[ai-evals]] AIEVAL-24).

**Confidence and revenue impact ride every item and roll into the forecast.** Each item
shows a confidence level and the revenue impact of acting, both derived from traceable
inputs: impact is the deal's value weighted by its stage probability, and confidence is
the deal-health score (BRIEF-FORM-2, citing the deal-health formula owned by the
reporting side). The leader's forecast consumes the same per-deal confidence and impact
that drive the rep's queue — "why is this deal weighted this way" traces to evidence,
not a manually set stage. The forecasting chapter owns that roll-up surface; this
chapter guarantees the values are the same ones.

**The queue remembers what you did with it.** Each generated brief is a run with a data
cutoff; each item carries acted/dismissed state. Acted items drop or move; dismissed
items do not silently reappear unchanged on the next open — a deterministic filter on
the next run's candidate set, not the AI's discretion. The overnight change detection
is keyed to the same per-rep state, so "what changed" means changed since the rep's
last brief view. The home screen also renders the overnight agent's "while you slept"
summary — reversible applied hygiene and staged corrections — as the overnight-agent
chapter's output, displayed here because this chapter owns the screen.

**The weekly bilingual 3P report is the brief's scheduled sibling.** A weekly cron
synthesizes the period's mail, calendar, chat, and CRM movement into a fixed
Progress / Plans / Problems structure — what moved, what's planned, what's blocked —
produced in both German and English from the same evidence set, so the two language
versions state the same facts. Every line traces to its source or is omitted; a quiet
week yields honestly sparse sections. The report is staged as a draft the user reviews
and edits — an edited line is marked typed-by-you, an unsupported line is removable
with counts recomputed honestly — and sharing is the user's explicit act, never an
auto-send. As a generative output it renders the AI-assisted disclosure
([[acceptance-standards#GATE-AI-9]]; [[ai-evals]] AIUC-12).

**Latency is progressive by contract.** First items render fast and the full brief
assembles within budget, shown progressively, never spinner-then-dump — the corpus pins
first items and full assembly at the values carried verbatim in the Acceptance appendix
(AIUC-11), the drafted-move first token rides the baseline AI budget
([[acceptance-standards#PERF-5]]), and the candidate-set graph query carries the
context-graph assembly budget ([[acceptance-standards#PERF-7]]).

## What's configurable

- **The ranking model** — the five weights, the actionability cutoff, the queue target,
  the timing buckets, and the revenue-norm fallback are named source constants
  (BRIEF-PARAM-1..7); changing the model is a code edit and redeploy, by design. No
  runtime tuning surface exists in V1.
- **Stage win probability** — the one runtime-tunable input to the winnability term,
  bounded config on the seeded pipeline owned by [[deals-and-pipeline]] (its
  DEAL-PARAM-3, via [[runtime-config#RC-1]]).
- **The AI re-ranker** — an injectable, optional model layer; absent or misbehaving, the
  brief degrades to the deterministic order with full function.
- **The voice model** — drafts generate through the workspace's [[voice-profile]]; a rep
  without a built profile gets baseline drafts, honestly unlabeled as voice-matched.
- **The weekly cadence** — the 3P report ships on a fixed weekly schedule in V1;
  configurable cadence and additional languages are fast-follow.
- **Brief-run retention** — generated runs are kept for a bounded window
  (BRIEF-PARAM-8), long enough for the changed-since cursor and short enough to stay a
  read model, not an archive.

## Guarantees (enforced)

- **Same inputs, same queue.** The composite is a pure function of deal columns and a
  fixed clock; golden worked examples pin the exact numbers, including the tie-break
  chain (BRIEF-FORM-1).
- **The AI cannot breach the deterministic floor.** Re-ranker output is a permutation of
  the candidate set; the set, the feature vectors, and the cutoff are computed before
  the model is consulted, and a dropped, injected, or below-bar-promoted deal is a test
  failure, not a judgment call.
- **Never an empty pipeline view.** The home route always renders the queue or the
  honest empty/short states; a blank board is unreachable
  ([[acceptance-standards#STATE-SP-1]], AIUC-11).
- **No padding.** A deal below the actionability bar is excluded, full stop; a quiet
  week is visibly quiet ([[ai-evals]] AIEVAL-22 pins stale padding as failure).
- **Evidence or omission on every line.** Rank reasons, overnight changes, people
  labels, and hidden-priority claims each carry a non-empty snippet and a resolvable
  source id or are absent ([[acceptance-standards#GATE-AI-1]]; AIEVAL-21 at zero
  violations).
- **The brief never sends.** Zero outbound and zero deal-field writes occur before an
  explicit human confirm; approve/edit/send is the only door, and money or irreversible
  moves gate harder ([[acceptance-standards#GATE-AI-7]]; AIEVAL-24 at zero).
- **The forecast sees the same numbers.** Per-deal confidence and revenue impact in the
  queue are byte-identical to the values the forecast roll-up consumes (BRIEF-FORM-2).
- **Acted and dismissed items stay handled.** Item state is persisted per run and
  filtered deterministically on the next run.
- **Tenant isolation.** Runs and items are workspace-scoped and row-level isolated; a
  rep never sees another workspace's deals in their brief.

## Acceptance

Done means: a rep opens the product in the morning and lands on a short, ranked list
framed as winnable this week — never a blank board, never the raw pipeline — where
every item states why it is there, shows what changed overnight as clickable sourced
facts, labels who is warm and who is blocking with evidence, and offers one next move
with a message already drafted in the rep's own voice, waiting on approve/edit/send.
Acting on or dismissing an item sticks. A slow week produces a visibly short queue and
an honest "nothing needs you" morning renders as a real state. On the weekly cadence
the same reasoning produces the Progress/Plans/Problems report in German and English
from one evidence set, reviewable and editable, shared only by the user's explicit act.
The leader can trace the forecast's weighting of any deal to the same evidence the rep
saw. The honest states carry the thesis: the empty-queue and honest-short-week states
are this chapter's named special-case states ([[acceptance-standards#STATE-SP-1]]), on
top of the inherited standard screen-state floor (STATE-1..5), which is not restated.
The testable form of every claim is pinned in the Acceptance appendix; the brief-ranking
eval bands and hard gates live in [[ai-evals]] (AIUC-11/12, AIEVAL-21..24).

## Out of scope

- **The overnight reconciliation run and its approval inbox** — the overnight-agent
  chapter (E06) and [[approvals-and-concurrency]] / the notifications-and-approval-inbox
  chapter; the home screen renders their staged output but does not own it.
- **The stalled rule, stage probabilities, and weighted pipeline arithmetic** —
  [[deals-and-pipeline]] (its DEAL-FORM-1..3), consumed as inputs.
- **Relationship strength** — [[people-and-organizations]] (its PO-F-3), consumed as the
  warmth input.
- **Deal-health scoring and the forecast surface** — the reporting and forecasting
  chapters; this chapter cites the health score as its confidence input and guarantees
  value identity with the roll-up.
- **The voice model and every draft/send mechanic** — [[voice-profile]] and
  [[drafting]]; the brief requests drafts and displays them.
- **The pre-meeting dossier** — [[meetings-and-transcripts]] (AIUC-04), a different
  moment on the same substrate.
- **The task work queue** — [[tasks-and-work-queue]]; brief items are not tasks.

## Where it lives

The brief assembly, ranking core, item state, and weekly-report synthesis live in the
agents module (`modules/agents`), reading the relational core and the context-graph
seams built by the search and AI modules; the web surfaces are the home screen and the
weekly-report screen. Read next: [[deals-and-pipeline]] (the inputs),
[[voice-profile]] and [[drafting]] (the drafted move), [[event-bus]] (the streams it
consumes), and [[ai-evals]] (the quality bands).

## Appendix

### Parameters
Source: specs/spec/contract/formulas-and-rules.md#0-parameter-registry-all-tunables-one-place @ 5a0b29c; specs/spec/contract/formulas-and-rules.md#10-brief-ranking--deterministic-scoring-inputs-feeding-the-morning-brief-rank @ 5a0b29c; specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| BRIEF-PARAM-1 | `BRIEF_QUEUE_TARGET` | `7` | Target action-queue length; the queue is the top 7 by composite among candidates, shorter when fewer clear the bar. §0 registry row. |
| BRIEF-PARAM-2 | `BRIEF_CANDIDATE_MIN_SCORE` | `0.15` | Actionability bar: below this composite a deal is not a candidate (the honest-short cutoff). §0 registry row. |
| BRIEF-PARAM-3 | Composite weights `W_WIN / W_REV / W_TIME / W_MOM / W_WARM` | `0.30 / 0.25 / 0.20 / 0.15 / 0.10` (sum = 1.0) | The five factor weights of the brief composite (BRIEF-FORM-1). Ratified as a locked decision (formulas §13); the AI ranker re-orders within the deterministic candidate set only. |
| BRIEF-PARAM-4 | `REVENUE_NORM` | workspace P90 deal size | The large-deal norm the revenue term is bounded against. |
| BRIEF-PARAM-5 | `REVENUE_NORM_FALLBACK` | `50_000_00` minor base (€50,000) | Fixed fallback norm when the workspace has too few deals (<10) to compute a P90, so the revenue term stays bounded. |
| BRIEF-PARAM-6 | Timing buckets | null → 0.3 · overdue → 0.9 · ≤7d → 1.0 · ≤30d → 0.7 · ≤90d → 0.4 · else → 0.2 | The `timing_score` step function over days-until-expected-close (BRIEF-FORM-1). |
| BRIEF-PARAM-7 | Momentum constants | changed overnight → `1.0` · unchanged → `0.4` | The momentum term's two values; "changed overnight" = any linked activity arrived since the rep's last brief view. |
| BRIEF-PARAM-8 | Brief-run retention | `~30` days (config) | How long generated `brief_run` rows are kept (data-model DDL comment); enough for the changed-since cursor, not an archive. |

Registry note BRIEF-PARAM-N-1 (honest): only BRIEF-PARAM-1/2 have rows in the §0
parameter registry @ 5a0b29c; the weights, `REVENUE_NORM`, its fallback, and the timing
buckets are pinned in §10's tunables line and the §13 locked-decision entry but are
missing from the §0 "all tunables, one place" table — flagged for corpus registry sync.
All are source constants; the only runtime-tunable input is `stage.win_probability`
([[deals-and-pipeline]] DEAL-PARAM-3). The cross-cutting latency budgets are
[[acceptance-standards]]'s (PERF-5, PERF-7) — cited, not owned here.

### Formulas
Source: specs/spec/contract/formulas-and-rules.md#10-brief-ranking--deterministic-scoring-inputs-feeding-the-morning-brief-rank @ 5a0b29c; specs/spec/contract/formulas-and-rules.md#1073-brief-item-revenue-impact--confidence-b-e059 @ 5a0b29c

**BRIEF-FORM-1 — the deterministic brief composite (the rank baseline).** This chapter
is the single home of the formula. Inputs (per live open deal): `base_value`
([[deals-and-pipeline]] DEAL-FORM-2 base-currency), `stage.win_probability`
([[deals-and-pipeline]] DEAL-FORM-1), `is_stalled` + idle days ([[deals-and-pipeline]]
DEAL-FORM-3), `expected_close_date`, the relationship strength of the deal's strongest
stakeholder ([[people-and-organizations]] PO-F-3), and an "overnight change" boolean
(did any linked activity arrive since the rep's last brief view — keyed to per-rep
brief-item state, BRIEF-DDL-2).

```
# all sub-scores normalized to 0..1
winnability = win_probability / 100
revenue     = min(1.0, base_value / REVENUE_NORM)            # REVENUE_NORM = workspace P90 deal size
timing      = timing_score(expected_close_date)             # see below
momentum    = 1.0 if changed_overnight else 0.4
warmth      = strongest_stakeholder_strength / 100          # PO-F-3

composite = W_WIN*winnability + W_REV*revenue + W_TIME*timing + W_MOM*momentum + W_WARM*warmth
   # default weights: W_WIN=0.30, W_REV=0.25, W_TIME=0.20, W_MOM=0.15, W_WARM=0.10  (sum=1.0)

timing_score(close_date):
   if close_date is null: 0.3
   d = days_until(close_date)
   if d < 0:        0.9      # overdue close → urgent
   elif d <= 7:     1.0
   elif d <= 30:    0.7
   elif d <= 90:    0.4
   else:            0.2
```

- **Honest-short cutoff:** a deal is a queue candidate **only if**
  `composite >= BRIEF_CANDIDATE_MIN_SCORE (0.15)`. The queue is the top
  `BRIEF_QUEUE_TARGET (7)` by composite **among candidates** — if fewer than 7 clear
  the bar, the queue is genuinely shorter (no padding with stale deals). Stalled deals
  with no overnight change and low warmth fall below the bar.
- **Output:** an ordered candidate list `[{deal_id, composite, feature_vector,
  evidence_ids[]}]` handed to the L2 ranker. The deterministic composite is the
  **fallback rank** (used directly if the AI layer is unavailable) and the **evidence
  basis** every ranked item must expose (the no-guess gate, GATE-AI-1).
- **Tie-breaks:** higher `base_value`, then sooner `expected_close_date`, then lowest
  `deal.id` (stable order).
- **WORKED EXAMPLE (verbatim):**
  - Deal A: 80% win, €60k (P90=€80k → revenue 0.75), closes in 5 days (timing 1.0),
    changed overnight (momentum 1.0), warmth 47/100=0.47.
    `composite = 0.30*0.80 + 0.25*0.75 + 0.20*1.0 + 0.15*1.0 + 0.10*0.47 = 0.24+0.188+0.20+0.15+0.047 = 0.825`. → top of queue.
  - Deal B: 25% win, no amount (revenue 0), close in 200 days (0.2), no change (0.4),
    warmth 0.1. `composite = 0.075+0+0.04+0.06+0.01 = 0.185` → just clears 0.15, low in
    queue.
  - Deal C: 10% win, stalled, no change, warmth 0 → `composite ≈ 0.03+...+0.06 < 0.15`
    → **excluded** (not padded in).
- **Edge cases:** quiet week → few deals clear the bar → short queue (correct, not
  padded); `REVENUE_NORM` undefined (new workspace, <10 deals) → `REVENUE_NORM_FALLBACK`
  keeps the revenue term bounded; acted/dismissed items are removed from the candidate
  set on the next run — a deterministic filter, not the AI's job.

**BRIEF-FORM-2 — per-item revenue impact & confidence (verbatim, B-E05.9 ratified).**

```
revenue_impact = deal.amount_minor × stage_weight   # stage_weight = the pipeline win-probability for the deal's stage
confidence     = deal_health_score                  # the deal-health composite, surfaced 0–1
```

These feed the BRIEF-FORM-1 rank inputs and carry no new weights of their own. The
deal-health formula (four weighted factors) is owned by the reporting/deal-health side
(formulas §10.5) — cited, never restated here; the forecast roll-up must consume these
exact per-deal values (the S-E05.5 identity guarantee, BRIEF-AC-5).

### Schema
Source: specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c

Ownership verified against the data-model chapter's ownership index: `brief_run` and
`brief_item` are assigned to this chapter ([[data-model]] Schema — ownership index).
Both are read models over the relational core — no domain fact lives only here.

**BRIEF-DDL-1 — the `brief_run` table (verbatim).**

```sql
CREATE TABLE brief_run (                                 -- one generated daily brief for a rep (B-E05.3b)
  user_id      uuid NOT NULL REFERENCES app_user(id),
  generated_at timestamptz NOT NULL DEFAULT now(),
  as_of        timestamptz NOT NULL                      -- the data cutoff the brief reflects; ~30-day retention (config)
);
CREATE INDEX idx_brief_run_user ON brief_run (workspace_id, user_id, generated_at DESC);
```

**BRIEF-DDL-2 — the `brief_item` table (verbatim).**

```sql
CREATE TABLE brief_item (                                -- one prioritized item within a brief (B-E05.3b/.13)
  brief_run_id uuid NOT NULL REFERENCES brief_run(id),
  kind     text NOT NULL,                                -- 'follow_up' | 'at_risk_deal' | 'signal' | …
  ref_type text NOT NULL,                                -- entity type the item points at
  ref_id   uuid NOT NULL,
  rank     int  NOT NULL,
  payload  jsonb NOT NULL,                               -- denormalized display data
  state    text NOT NULL DEFAULT 'new' CHECK (state IN ('new','acted','dismissed')),
  state_at timestamptz NULL                              -- changed-since cursor = (user via run, state_at)
);
CREATE INDEX idx_brief_item_run   ON brief_item (brief_run_id, rank);
CREATE INDEX idx_brief_item_state ON brief_item (brief_run_id, state, state_at);
```

Note BRIEF-DDL-N-1 (reconcile at ticket time): the home screen's prototype offers a
**Snooze** action (AC-home-6) but the `brief_item.state` check vocabulary is
`new/acted/dismissed` — no snoozed state exists in the pinned DDL, and the prototype's
snooze/dismiss persistence is in-memory only. The build ticket must either add a
snoozed state (a schema change) or map snooze onto dismissed-until-next-run semantics;
carried as BRIEF-AC-OPEN-3. Note BRIEF-DDL-N-2: the weekly 3P report has **no table**
in the corpus schema @ 5a0b29c — its staged draft, edits, and shared state need a home
(a new table or reuse of an existing staging shape) decided at ticket time
(BRIEF-AC-OPEN-5).

### Wire
Source: specs/spec/contract/crm.yaml @ 5a0b29c; specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c

Honest report: **no brief or weekly-report operations exist in the contract at the
pinned corpus version** — `crm.yaml` @ 5a0b29c defines 81 `operationId`s and none of
them cover the brief queue read, item act/dismiss, the drafted-move approve/edit/send
handoff, or the weekly report's read/edit/share. The data-model contract-surface note
declares `brief_run`/`brief_item` "are read through the brief endpoint," but that
endpoint is not yet in the contract — a corpus gap, not a design choice. Note
BRIEF-WIRE-N-1: the operations must be added contract-first at ticket time and this
chapter owns them when they land. Whatever lands must keep the tiering: brief read and
item act/dismiss are internal, reversible (🟢 at most); the drafted next move leaves
only through [[drafting]]'s send operation behind the approval gate (GATE-AI-7 — the
brief surface itself gets no send operation); the weekly report's share is an explicit
user act on a staged draft, never schedulable as an auto-send. The same goal driven by
a BYO agent (AIUC-11 is Surface-A scoped) must ride the governed MCP tool surface with
identical gates.

### Events
Source: specs/spec/contract/events.md#5-the-catalog @ 5a0b29c

Consumed: the brief's assembly rides the `cg:overnight-agent` consumer group
([[event-bus#EVT-CG-2]] — modules/agents; subscribes to the activity, deal, lead, and
approval streams: "what changed overnight", stalled-deal sweeps, the ranked action
queue), over the context-graph inputs maintained by [[event-bus#EVT-CG-1]]. Emitted —
honest report: the central catalog defines **no brief event types** @ 5a0b29c (46
types; none are brief-scoped) — no brief-generated, item-acted/dismissed, or
report-shared events exist. Note BRIEF-EVT-N-1: when the wire surface lands
(BRIEF-WIRE-N-1), its mutations (item act/dismiss, report edit/share) fall under the
one-mutation-one-audit-one-event rule ([[event-bus#EVT-SEM-1]]) and need catalog rows
added catalog-first with the contract work; an approved send emits through
[[drafting]]'s path, not a new brief event.

### Acceptance
Source: specs/spec/product/epics/E05-morning-brief.md#e05--morning-brief @ 5a0b29c; specs/spec/product/20-traceability.md @ 5a0b29c

**Owned stories** (primacy verified against the traceability register: E05's six
stories map to this chapter alone — the [[scope]] epic inventory assigns E05 to
morning-brief with no co-owner; not contested):

| ID | Story | Tier | Home |
|---|---|---|---|
| S-E05.1 | The home screen is a ranked queue of winnable deals | V1-WOW | this chapter |
| S-E05.2 | Why this deal matters and what changed overnight | V1-WOW | this chapter |
| S-E05.3 | Who is warm, who is blocking, and the buyer's hidden priority | V1-WOW | this chapter |
| S-E05.4 | The next move, drafted in my voice, ready to approve/edit/send | V1-WOW | this chapter (voice: [[voice-profile]]; draft/send: [[drafting]]) |
| S-E05.5 | Confidence and revenue impact on every recommendation | V1-WOW | this chapter (roll-up surface: forecasting chapter) |
| S-E05.6 | Weekly bilingual 3P report (Progress/Plans/Problems) | V1-WOW | this chapter |

**Story-level acceptance (condensed Given/When/Then from the epic; new IDs — the
source bullets are unnumbered).**

| ID | Given/When/Then | Verification |
|---|---|---|
| BRIEF-AC-1 | (S-E05.1) Given the rep opens the CRM, when the home loads, then a ranked, finite list (target 7, BRIEF-PARAM-1) framed "you can win this week" renders — never the full pipeline, never an empty board; rank reflects winnability × revenue × timing with a one-line evidenced why-this-week per item; a slow week yields an honestly short queue; acted/dismissed items drop/move and do not silently reappear unchanged. | Deterministic golden test on BRIEF-FORM-1 (fixed clock); screen e2e (AC-home-2/3); STATE-SP-1 state test; [[ai-evals]] AIUC-11 + AIEVAL-22 band |
| BRIEF-AC-2 | (S-E05.2) Given a queue item is expanded, when read, then why-it-matters and what-changed-overnight render as discrete sourced facts, each click landing on the exact evidence; nothing-changed says so plainly; an uncertain change shows the raw fact flagged uncertain, never a confident wrong read. | Deterministic evidence-id assertion (GATE-AI-1, AIEVAL-21 = 0); no-change fixture → zero fabricated changes (AIEVAL-23 band); screen e2e (AC-home-4) |
| BRIEF-AC-3 | (S-E05.3) Given the people picture, when viewed, then each key person is labeled warm/neutral/blocking with evidence; a hidden-priority claim is grounded in an observed source or omitted; a wrong label is correctable in one action and remembered (not re-mislabeled next run); signals stay consent-gated, company-level per P12 — no named-individual behavioral dossier. | Deterministic payload test (contact + activity ids present); correction-persistence test (rides the ai-feedback ledger, [[data-model]] ownership index); orphan-signal fixture → 0 person-profile rows |
| BRIEF-AC-4 | (S-E05.4) Given a queue item's action, when viewed, then one recommended next move renders with a draft in the rep's voice referencing real deal context; before confirm zero outbound is sent and no deal field mutates; approve sends with brief-suggestion provenance; edit-then-send sends the edited version; money/irreversible moves are flagged and gated harder; tone corrections feed future drafts, draft-only always. | GATE-AI-7 + AIEVAL-24 = 0 (pre-confirm sends/writes); edited-draft test; [[voice-profile]] VOICE-AC-4 (never auto-send) cited; screen e2e (AC-home-5) |
| BRIEF-AC-5 | (S-E05.5) Given any recommendation, when shown, then it carries a confidence and a revenue impact from traceable inputs (BRIEF-FORM-2); similar-revenue items make the confidence difference visible; the leader's forecast rolls up the same per-deal values, traceable to evidence, not a manual stage; low confidence is marked as verify-first, never certainty. | Golden identity test: forecast roll-up consumes byte-identical per-deal values; derivation payload includes source ids |
| BRIEF-AC-6 | (S-E05.6) Given the weekly schedule fires, when the report is produced, then it is the fixed Progress/Plans/Problems structure over the period's mail/calendar/chat/CRM, in both German and English with the same content and switchable; every line lands on its evidence or is omitted; the report is draft/reviewable and sharing is the user's explicit act, never auto-sent; a quiet week reads honestly sparse. | [[ai-evals]] AIUC-12 (0 outbound pre-share; DE/EN same source-id set); GATE-AI-9 disclosure; screen e2e (AC-weekly-report-1..8) |

**Feature-level acceptance (verbatim from the feature spec).**
Source: specs/spec/features/07-ai-native-moments.md#6-morning-brief--7-deals-you-can-win-this-week-home @ 5a0b29c

| ID | Criterion (verbatim) |
|---|---|
| BRIEF-AC-7 | Given the rep opens the CRM in the morning, the **home screen opens to a ranked action queue, never an empty pipeline and never the raw full-deal board** — a finite list (target ~7) framed "you can win this week", rendered async, **first items < 1.5 s p95**, full brief **assembly p95 < 5 s** end-to-end (graph query + rank + draft), shown progressively, never spinner-then-dump. *(perf test; UI test: empty-pipeline state is unreachable from the home route — it always renders the queue or an honest "nothing actionable this week", never a blank board)* |
| BRIEF-AC-8 | **Every ranked deal exposes the evidence behind its rank in one click** — its *why-on-the-list* reason, every *what-changed-overnight* fact, every warm/blocking label, and the hidden-priority claim each carry a **non-empty `evidence_snippet` + a clickable source id (activity/deal/signal/relationship)** or are **omitted**; a rendered claim with no source is a hard failure (the no-guess gate). *(deterministic test: assert evidence id on every claim; no-change fixture → "nothing changed overnight", 0 fabricated changes; uncertain-change fixture → raw fact + uncertain flag, never a confident assertion)* |
| BRIEF-AC-9 | Given a quiet week, the queue is **honestly short** and does not pad to a count with stale deals. *(test: low-movement fixture → queue length reflects genuine actionables, padding-with-stale = failure)* |
| BRIEF-AC-10 | Given the people picture, each key person is labeled warm/neutral/blocking with evidence, and asking *"why warm?" / "why blocking?"* returns the contact id(s) + the supporting activities — **no mystery score**; a correction is one action and is remembered (the same person is not re-mislabeled on the next run). *(deterministic test: payload includes contact ids + activity ids; correction-persistence test)* |
| BRIEF-AC-11 | The hidden-priority claim is grounded in an observed source (transcript line / repeated theme / viewed asset) or omitted; people signals are **consent-gated, company-level** — **no** named-individual behavioral dossier is created. *(test: assert source on every hidden-priority claim; orphan signal → 0 person-profile rows)* |
| BRIEF-AC-12 | Each deal shows **one** recommended next move with a drafted message referencing real deal context; **before confirm, zero outbound is sent and no deal field is mutated**; approve → the message sends and the activity logs with provenance "approved from morning brief"; edit-then-send → the **edited** version goes out; money/irreversible moves are flagged and gated harder. *(test: query → 0 sends / 0 deal-field writes pre-confirm; 🟡 gate per `03b`; edited-draft test)* |
| BRIEF-AC-13 | Each item shows a `confidence` and a `revenue_impact` from traceable inputs, and the **same per-deal confidence × impact rolls into the leader's forecast** answerable by "why is this deal weighted this way?" tracing to evidence, not a manual stage. *(test: forecast rollup uses the same per-deal values; derivation payload includes source ids)* |
| BRIEF-AC-14 | Acted items drop/move and dismissed items **do not silently reappear unchanged** on the next open. *(test: act/dismiss → next-run queue reflects it)* |

**Weekly 3P report acceptance (verbatim from the feature spec).**
Source: specs/spec/features/07-ai-native-moments.md#6b-weekly-bilingual-3p-report-progress--plans--problems @ 5a0b29c

| ID | Criterion (verbatim) |
|---|---|
| BRIEF-AC-15 | Given a week of captured signal, the cron produces a 3P report in **both DE and EN** whose Progress/Plans/Problems lines each carry a **clickable source id**; an ungrounded line is a hard failure. *(deterministic test: assert source on every line; both-language presence test; no-activity fixture → honest "nothing this week", 0 fabricated items)* |
| BRIEF-AC-16 | The two language versions are **derived from the same evidence set** (same underlying facts, not independently hallucinated). *(test: DE/EN cover the same source ids)* |
| BRIEF-AC-17 | **Before the user shares it, nothing is sent** — the report is staged for review/edit; sharing is an explicit action. *(test: pre-share → 0 outbound)* |
| BRIEF-AC-18 | Output renders the Art. 50 AI-assisted disclosure (§11 gate 9). *(GATE-AI-9)* |

**Home screen acceptance criteria (verbatim; corpus IDs preserved).** The screen→story
index tags the home screen to S-E05.1–.5; the screen is owned here. AC-home-8/9/10
render the overnight agent's staged output — the behavior behind them is the
overnight-agent chapter's (S-E06.1/.2); the screen assertion is this chapter's.
AC-home-1's weighted-pipeline tile must equal [[deals-and-pipeline]]'s roll-up (its
DEAL-FORM-2) to the cent.
Source: specs/spec/product/30-screen-acceptance.md#homehtml--morning-brief-implements-s-e0515 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-home-1 | Given I open home, When it loads, Then I see a personalized greeting with date/meeting/pipeline-delta meta, four KPI tiles (Weighted pipeline, Needs you today, Meetings today, Captured overnight), and a "Needs you today" section labeled "ranked by impact · on the context graph". | Screen e2e lane (+ golden-number vs DEAL-FORM-2 for the weighted tile) |
| AC-home-2 | Given the action queue, When it renders, Then it shows a finite ranked list numbered 1..n, each with a logo, name, a one-line deal subtitle, and an optional status flag (e.g. "Stalled 74d", "Warm signal", "Forecast"). | Screen e2e lane |
| AC-home-3 | Given each queue item, When I read it, Then it states in one line why it is on the list this week (the AI suggestion + recommended move). | Screen e2e lane |
| AC-home-4 | Given a queue item, When I click "Show the evidence", Then an evidence box expands showing the verbatim quote and its source; the toggle flips to "Hide the evidence" and the chevron rotates. | Screen e2e lane (evidence content: GATE-AI-1) |
| AC-home-5 | Given a queue item, When I click its primary action (e.g. "Draft re-engagement"), Then a toast confirms the action is "staged for your review (🟡 nothing sent yet)" — no outbound is sent from the queue. | Screen e2e lane (send gate: GATE-AI-7) |
| AC-home-6 | Given a queue item, When I click "Snooze", Then a toast confirms "Snoozed until tomorrow" and the item dims; When I click "Dismiss", Then the item animates out with a toast that the agent will learn from it. | Screen e2e lane (state persistence: BRIEF-AC-OPEN-3) |
| AC-home-7 | Given each queue item, When I view it, Then an "Open" link navigates to the relevant detail screen for the deal. | Screen e2e lane |
| AC-home-8 | Given the "While you slept" overnight section, When it renders, Then it is labeled "reversible · nothing sent" and lists what the agent did (captured/linked N activities, enriched M contacts marked 🟢 applied/reversible, flagged stalled deals, proposed close-date corrections), each with a provenance tag and a Review/open control. | Screen e2e lane (behavior: overnight-agent chapter, AIUC-15) |
| AC-home-9 | Given the overnight close-date corrections, When they render, Then they appear as an "N to review" staged flag (not auto-applied), distinct from already-applied reversible enrichments. | Screen e2e lane (behavior: overnight-agent chapter) |
| AC-home-10 | Given the screen, When it renders, Then a closing line states everything the agent did is reversible and evidenced and nothing was sent to a customer. | Screen e2e lane |
| AC-home-11 | Given the in-app shell, When home loads, Then the Ledger-Green nav rail + a top search/ask bar + a "Read a company" CTA (→ read-company.html) + ⌘K palette are available. | Screen e2e lane (shell owned by AC-shell-*) |

**Weekly-report screen acceptance criteria (verbatim; corpus IDs preserved).** The
screen→story index tags the weekly-report screen to S-E05.6; the screen is owned here.
Source: specs/spec/product/30-screen-acceptance.md#weekly-reporthtml--weekly-bilingual-3p-report-implements-s-e056 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-weekly-report-1 | Given the report state is shown, When the screen loads, Then the three fixed sections Progress / Plans / Problems each render with a live count in the section header (3 / 2 / 2) and a header showing the ISO week ("KW 25"), the date range "16–22 Jun 2026", the owner "Anna Weber", and a "Draft · reviewable" status pill. | Screen e2e lane |
| AC-weekly-report-2 | Given a synthesized line in any section, When the user clicks its source chip (e.g. "Offer · 22 Jun"), Then an evidence row expands under that line showing the grounding quote, the source meta, a "jump to source →" link, and a confidence label; opening one evidence row collapses any other open one. | Screen e2e lane (evidence content: GATE-AI-1) |
| AC-weekly-report-3 | Given the language toggle shows Deutsch active, When the user clicks "English", Then all section headers, line text, status pill, edit button, share button, coverage note, and source-chip labels re-render in English with the same content, and switching back to Deutsch restores German — same content, both languages, switchable any time. | Screen e2e lane (fact identity: BRIEF-AC-16) |
| AC-weekly-report-4 | Given the report, When the user clicks "Edit report" (or the per-line pencil), Then the line(s) become editable; on confirming an edit the line text is replaced with the typed text and an "edited by you" provenance marker is appended to that line, with a toast confirming the line is now marked as typed by the user. | Screen e2e lane |
| AC-weekly-report-5 | Given a line the user judges unsupported, When they click its remove (×) control, Then the line and its evidence row are removed and the affected section count is recomputed honestly (decremented), with a "Line removed from the report" toast. | Screen e2e lane |
| AC-weekly-report-6 | Given no share recipients are selected, When the report loads, Then the "Share KW 25 report" button is disabled and the hint reads "Pick at least one recipient."; When the user selects at least one of the three targets (team channel / lead / email to self), Then the button enables and the hint states it will share with N recipients as the user's explicit act. | Screen e2e lane |
| AC-weekly-report-7 | Given at least one recipient is selected, When the user clicks the share button, Then the status pill changes from "Draft" to "Shared · KW 25" and a toast confirms the report was shared "by you, never auto-sent" — nothing is sent until the user confirms. | Screen e2e lane (share gate: BRIEF-AC-17) |
| AC-weekly-report-8 | Given a published report, When the user clicks "Regenerate from source", Then the screen shows the generating state then returns to the report and confirms via toast that re-synthesis preserved the user's edited line(s). | Screen e2e lane |

The standard screen-state matrix (empty / loading / error / no-permission /
nothing-grounded) is inherited from [[acceptance-standards]] (STATE-1..5) and not
restated; this chapter additionally owns the named special-case states
[[acceptance-standards#STATE-SP-1]] — the empty-queue "nothing needs you this morning"
state and the honest-short-week state.

**Open build decisions (carried honestly — the build tickets must resolve them).**
Source: specs/spec/product/30-screen-acceptance.md#homehtml--morning-brief-implements-s-e0515 @ 5a0b29c; specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c

| ID | Decision needed | Verification |
|---|---|---|
| BRIEF-AC-OPEN-1 | **A discrete per-recommendation confidence indicator is missing in the prototype**: S-E05.5 requires an explicit confidence on every recommendation, but the prototype shows only revenue and status flags. The build must add it (the value is BRIEF-FORM-2's). Relatedly, rank is index-based in the prototype, not a shown winnability composite with traceable inputs. | Ticket-gate: the home-screen ticket must render confidence + expose the feature vector before build closes |
| BRIEF-AC-OPEN-2 | **The home screen's honest states are missing in the prototype**: empty-queue / honest-short ("slow week"), loading skeleton, error, and no-permission are all flagged missing. STATE-SP-1 and the STATE-1..4 floor must land as real rendered states. | Ticket-gate: state tests required by the screen-acceptance suite |
| BRIEF-AC-OPEN-3 | **Snooze has no schema home**: the prototype snoozes and dismisses in-memory only; the pinned item-state vocabulary is new/acted/dismissed (BRIEF-DDL-2, note BRIEF-DDL-N-1). Persist state, and either add a snoozed state or define snooze as dismiss-until-next-run. | Ticket-gate: persistence + vocabulary decision before build |
| BRIEF-AC-OPEN-4 | **Where the people picture renders**: the warm/blocking/hidden-priority view (S-E05.3) exists only inside the "why" copy in the prototype, not as a labeled view — confirm whether it lives on the expanded queue item or the deal detail screen. | Ticket-gate: placement decision recorded before build |
| BRIEF-AC-OPEN-5 | **The weekly 3P report has no schema or wire surface**: no table (BRIEF-DDL-N-2), no contract operations (BRIEF-WIRE-N-1), and no catalog events (BRIEF-EVT-N-1) exist for the staged report, its edits, or its share act. All three land contract-first with the report ticket. | Ticket-gate: contract + catalog rows precede build |
