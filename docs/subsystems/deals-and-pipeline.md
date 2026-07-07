---
status: planned
module: backend/internal/modules/deals
derives-from:
  - specs/spec/features/01-core-objects.md#3-deals @ 5a0b29c
  - specs/spec/features/01-core-objects.md#4-pipelines--stages @ 5a0b29c
  - specs/spec/contract/formulas-and-rules.md#5-stage-win-probability-defaults-seeded-default-pipeline @ 5a0b29c
  - specs/spec/contract/formulas-and-rules.md#6-weighted-pipeline-value--amount--stage-probability-roll-up @ 5a0b29c
  - specs/spec/contract/formulas-and-rules.md#8-stalled-deal-rule--last_activity_at-threshold--asked-to-wait-suppression @ 5a0b29c
  - specs/spec/contract/data-model.md#6-deals-pipelines-stages @ 5a0b29c
  - specs/spec/product/epics/E03-pipeline-and-deals.md @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#23-deals-forecast-tasks @ 5a0b29c
---
# Deals & pipeline — parity executed excellently, on numbers a leader can check by hand

> The core selling surface: deals moving through ordered stages, viewed as a board or a
> table, with the stage trajectory recorded for the rep and a weighted pipeline value
> that reconciles exactly to the deals behind it. Reps live here between inbox visits;
> the forecast, the Morning Brief, and every agent that touches a deal build on it.

## What it's for

Every CRM has a pipeline, so nothing here is novel — the need this subsystem answers is
a pipeline a rep actually enjoys working and a leader actually trusts. That means three
things the incumbents get wrong: interactions inside the enforced speed budgets so the
board feels instant rather than a click-maze; a deal history that is a real, append-only
record rather than notes a rep must remember to write; and a weighted value that is pure
arithmetic over visible inputs, verifiable by hand on a small set. Its callers are the
pipeline and deal-360 screens, the forecasting and reporting chapters (which consume its
amounts, probabilities, and stage history), the Morning Brief and overnight agent (which
consume its stalled flags and stage-change events), and any BYO agent driving deals
through the governed tool surface. The boundary: this chapter owns the deal, the
pipeline, the stage, the stage history, and the three deterministic rules over them —
not the offer engine, not forecast categorization, and not the AI that infers next
steps.

## Principles it serves

- **P11 — clean relational core.** Deals, pipelines, and stages are real rows with real
  foreign keys; stakeholders ride the typed relationship table, never a comma field or
  a directional association; the weighted number "just sums" because the model is
  normalized.
- **P4 — blazing fast, always.** The board, the table, the 360, and the drag-save all
  carry pinned p95 budgets ([[acceptance-standards#PERF-1]],
  [[acceptance-standards#PERF-2]], [[acceptance-standards#PERF-4]]) enforced in CI —
  speed is this epic's differentiator, not polish.
- **P5 — auto-capture over manual entry.** The 360 is assembled from captured activity;
  stage history is recorded by the move itself; a deal is created *from context* with
  people and activity pre-attached — the rep confirms, the rep does not transcribe.
- **P12 — governance designed in.** Every stage move is one audit row plus one domain
  event; advancing into a terminal stage is a governed, confirm-first transition
  (ADR-0026), and the history it leaves is append-only.
- **P1 / ADR-0002 — opinionated over configurable.** Pipeline shape is code and seed.
  The only runtime edits are the bounded pipeline-edit surface ([[runtime-config#RC-1]])
  and bounded stage gating ([[runtime-config#RC-13]]); a structurally new pipeline is a
  source-level change, never a settings screen.
- **ADR-0032 — partner attribution is structural.** A partner-sourced deal links a
  first-class partner organization, so partner pipeline is a reportable slice, not a
  free-text tag.
- **ADR-0008 — leads stay segregated.** A deal attaches to a person or organization,
  never to a raw lead.

## How it works

**One set of deals, two honest projections.** A workspace ships with exactly one seeded
default sales pipeline of ordered stages; each stage carries its position, an
open/won/lost semantic, and a default win probability (seeded values pinned as
DEAL-FORM-1). The rep sees the same deals as a kanban board grouped by stage and as a
sortable, filterable table, and switches between them without a reload or filter loss.
Each card shows its time-in-stage and its stalled state at a glance; each column shows
its count, raw sum, and weighted sum, and the totals strip recomputes as deals move.
Reading a column and paging beyond a screenful is cursor-paginated list reading under
the standard list conventions (api-conventions chapter).

**Drag is the primary mutation, and it writes the history for you.** Dropping a card on
a new stage issues one save inside the mutation budget; the move is optimistic — the
card shifts immediately and snaps back with an honest toast when the save is refused
(DEAL-AC-B1) — and won and lost are not standing columns but drop zones that appear
during the drag (DEAL-AC-B3). That single committed write
produces exactly one audit entry, appends exactly one row to the stage history — prior
stage, new stage, who moved it, when, and a snapshot of the amount at that moment — and
emits exactly one stage-changed event ([[acceptance-standards#GATE-CORE-5]]). The
history is append-only and is what later answers "how did this deal move", stage
conversion, and "pipeline as of date X" — the reporting over it belongs to the
forecasting chapter; the record itself is made here, at write time, not reconstructed.
Creation writes the first history row itself — no prior stage, into the initial stage —
so reconstructing the pipeline as of any date needs no creation special case
(DEAL-AC-H1).

**Probability lives on the stage row, and only there.** A deal's win probability is
read live from its current stage — never cached on the deal, never hard-coded in a
roll-up. Terminal stages are honest by construction: a won stage is always 100, a lost
stage always 0, held by a database check. When a workspace retunes a stage through the
bounded pipeline-edit surface, every number downstream follows automatically.

**Closing is governed, not incidental.** A deal cannot drift shut: closing requires a
terminal won-or-lost status, a lost deal requires a stated reason, and a closed deal
freezes the exchange rate that valued it (the FX rules are the data-model chapter's,
[[data-model#DM-FX-3]]). The deal-advance operation is the tool surface's one
*dynamically tiered* action — the tier is resolved from the transition's stage
semantics, not declared statically. A transition is confirm-first (🟡) when **either
endpoint** is a terminal (won or lost) stage — closing and reopening are both governed;
open→open moves, in either direction, are auto-approved (🟢). For a human in the UI the
confirm dialog is itself the approval; for an agent the same transition fails closed
without a valid single-use approval token, verified and consumed exactly as the
approvals-and-concurrency chapter pins it ([[approvals-and-concurrency#APPR-WIRE-1]],
[[approvals-and-concurrency#APPR-AC-7]]). The tool-table row that declares this tier
belongs to the byo-agent-and-mcp chapter; this chapter owns the behavioral rule the
resolver enforces. The write itself is server-derived: the deal's status follows the
target stage's semantic — a request stating a mismatching status is refused — and
advancing into a terminal stage stamps the close time and freezes the valuing rate in
the same committed write (DEAL-WIRE-9). Reopening reverses it under the same gate: a
terminal→open move is 🟡 like a close, clears the close timestamp, the lost reason, and
the frozen rate — the deal reverts to daily-rate conversion and re-freezes at its next
close — and lands in the history like any other transition (DEAL-AC-R1..R3).

**The stalled flag is deterministic, and it respects a customer's "wait."** A deal with
no activity past the idle threshold (DEAL-PARAM-1) flags as stalled — an
absolute-duration comparison on UTC instants, stable under a fixed test clock,
identical in every timezone. The one suppression: when capture has recorded that the
customer explicitly asked to wait, a wait-until date on the deal suppresses the flag
until that date passes (a dateless deferral falls back to the window in DEAL-PARAM-2).
Extracting the commitment is AI work owned by the capture pipeline; the rule consuming
it here is pure and deterministic (DEAL-FORM-3). The last-activity timestamp the rule
reads is maintained by the activities-and-timeline chapter's write path.

**The weighted number reconciles, or it refuses.** Weighted pipeline value is the sum,
over live open deals, of base-currency value times stage probability, rounded per deal
— half away from zero — so the displayed total equals the sum of the displayed parts
exactly (DEAL-FORM-2). The totals are computed server-side and read whole, never
client-summed (DEAL-EXT-1). A
mixed-currency workspace converts to the base currency under the data-model chapter's
FX rules ([[data-model#DM-FX-4]], [[data-model#DM-FX-5]]); a missing rate is a hard,
surfaced failure — never a silent rate of one, because a wrong number is worse than a
missing one. The output always carries its per-deal breakdown, so "explain this number"
is a decomposition, not a story.

**Deals begin from context, not from a blank form.** Created from a contact, a company,
or a captured thread, a new deal opens with the organization linked, the relevant
people attached as stakeholders with roles through the typed relationship table (owned
by the people-and-organizations chapter), and recent activity already on its timeline;
it lands in the default pipeline's first stage, and a likely duplicate for the same
organization is warned about rather than silently doubled. The 360 then shows the
buying committee as captured — including the honest single-threaded warning when there
is only one contact — and an *inferred* next step presented as a confirm-or-dismiss
suggestion, never a silent write and never a blank field demanding typing. Producing
that inference is the meetings-and-transcripts chapter's intelligence; this chapter
owns the surface that stages it and the task it becomes on confirm.

**Partner-sourced deals are attributed structurally.** A deal records the partner
organization that sourced or co-sells it as a link to a first-class, classified partner
record. Partner-sourced pipeline is then an indexed, reportable slice — weighted value
and counts per partner — with the attribution audited like any other mutation. V1 is
internal attribution only; the partner *program* that will run on this substrate is
explicitly later.

## What's configurable

- **Bounded pipeline edit** — reorder stages, rename a stage, set a per-stage win
  probability, rename the seeded pipeline; nothing structural. This is the runtime
  surface pinned as [[runtime-config#RC-1]] (boundary pending ratification), and the
  per-stage probability is the parameter registry's single runtime-tunable exception
  (DEAL-PARAM-3). Structurally new pipelines or stage-change automation are out —
  source-level work per ADR-0002.
- **Per-stage required-field gating & format validation** — "cannot move to Proposal
  without an amount and a decision-maker", enforced at write time with field-level
  validation errors; the bounded surface is [[runtime-config#RC-13]]. The register
  entry stands (the contract wins over the feature spec's out-of-scope line), but the
  per-stage required-field mechanics have no schema, contract operation, or evaluation
  rule specified yet — an unbuilt extension whose schema, operation, and evaluation
  rule arrive by their own docs change; PILOT-EXCLUDED (DEAL-N-PILOT). Not a rules DSL.
- **Stalled thresholds** — the idle threshold (DEAL-PARAM-1, sixty days) and the
  dateless-deferral suppression window (DEAL-PARAM-2, ninety days) are named source
  constants with defaults, deliberately *not* runtime config in V1.

## Guarantees (enforced)

- **A deal's stage always belongs to its pipeline.** Held in the database via the
  composite-key pattern (DEAL-DDL-N-1), not just handler validation; a mismatched write
  is refused with a field-level validation error.
- **One gesture, one fact.** A stage move commits one save, one audit row, one
  append-only history row, and one stage-changed event per deal-advance write — never
  zero, never two ([[acceptance-standards#GATE-CORE-5]]). The sanctioned stage+amount
  co-fire ([[event-bus#EVT-SEM-3]]) applies to combined edits made through the general
  update operation, not to the advance verb.
- **Stage history is append-only and complete.** Every transition since creation is
  reconstructable with who/when and the amount snapshot at change; no update path
  rewrites it.
- **Terminal stages cannot lie.** Won is 100 and lost is 0 by database check; a closed
  deal has a terminal status, a lost deal has a non-null reason, and a closed deal with
  an amount carries its frozen FX rate ([[data-model#DM-FX-3]]).
- **Terminal transitions are governed.** An agent advancing a deal into — or out of — a
  won or lost stage without a valid approval token fails closed as approval-required
  ([[api-conventions#API-ERR-10]]); the tier resolves from the transition's stage
  semantics (either endpoint terminal → 🟡), so renaming a stage cannot dodge the gate.
- **Probability is read live from the stage row.** Never cached per deal, never
  hard-coded in a roll-up; retuning a stage instantly reprices its column
  (DEAL-FORM-1).
- **The weighted total reconciles exactly.** Per-deal rounding makes the displayed
  total equal the sum of displayed parts (DEAL-FORM-2); an amountless deal contributes
  zero *with a marker*, and a missing FX rate hard-fails as an FX-unavailable error
  rather than substituting a value.
- **The stalled flag is a pure function.** Same inputs, same boolean, under a fixed
  clock, in every timezone (DEAL-FORM-3); a recorded customer wait suppresses it until
  the wait expires.
- **Exactly one default pipeline per workspace.** The partial unique index (DEAL-DDL-1)
  enforces at-most-one; exactly-one is established by the workspace seed and pinned by
  DEAL-AC-13 — deal creation always has a home.
- **Deals never attach to raw leads** (ADR-0008); stakeholders resolve through the
  typed relationship table ([[acceptance-standards#GATE-CORE-4]]).

## Acceptance

Done means a rep can live on the board: open it and see cards grouped by stage with
aging visible, flip to the table without losing filters, drag a card and have it stick
in one fast save with the history recorded, and open a deal to a 360 that is already
assembled — stakeholders with roles, captured timeline, linked company, current stage,
and a next-step suggestion to confirm or dismiss. A leader sees raw and weighted totals
that match hand arithmetic and decompose to the deals behind them; won counts at 100%,
lost at 0%, and a missing exchange rate refuses loudly rather than fudging. Closing a
deal always asks for the outcome, and a lost close asks for the reason. Empty stages
still render as drop targets, an empty board is an honest empty state, and every screen
inherits the standard state floor — empty, loading, error, no-permission, and
nothing-grounded for the inferred panels ([[acceptance-standards#STATE-1]]..5) — plus
the cross-cutting core gates and performance budgets of the acceptance-standards
chapter, none of which are restated here. The testable form of every claim lives in the
Acceptance appendix: the six owned stories, the corpus feature criteria, and the two
owned screens' criteria verbatim.

## Out of scope

- **Offers / Angebote and products** — the versioned line-item quote on a deal, its
  totals, PDF, and acceptance flow: the offers-and-products chapter (S-E03.7 lives
  there; an accepted offer syncs the deal amount via its own paired events).
- **Forecast categories, deal-health scoring, close-date hygiene** — forecasting
  chapter; this chapter supplies the inputs (amounts, probabilities, stage history) and
  the rep-set category column travels on the deal row it owns.
- **Next-step and stage-hint inference** — the meetings-and-transcripts chapter's
  intelligence; this chapter stages, confirms, and records the result.
- **The MCP tool table** (verbs, scopes, tiers — including the dynamic tier row for the
  deal-advance verb) — byo-agent-and-mcp chapter.
- **Approval staging, tokens, and expiry** — approvals-and-concurrency chapter.
- **FX rate storage and conversion rules** — data-model chapter (DM-FX-1..7,
  [[data-model#DM-DDL-12]]).
- **The activity timeline and last-activity maintenance** — activities-and-timeline
  chapter. **The relationship table** — people-and-organizations chapter.

## Where it lives

Planned as the deals bounded context in the backend modules directory — its own module
per the target structure's bounded-context doctrine (DEAL-N-MODULE). The skeleton
(45aa8a7) carries the deal substrate — the deal and pipeline stores and the terminal
gate — inside the shared spine module; creating the deals module migrates that
substrate out (single home, never two). It is exposed through
the contract like every other write, with its board, table, and 360 in the frontend's
deals feature directory. The contract gaps this chapter closes (roll-up read, board
card fields, composite 360 read, restore, stakeholder-role enum, composite stage FK)
are pinned as contract-first extensions (DEAL-EXT-1..6). Read next: forecasting (what the numbers feed),
offers-and-products (the quote on the deal), approvals-and-concurrency (the gate the
terminal advance rides), and byo-agent-and-mcp (how agents reach all of it).

## Appendix

### Parameters
Source: contract/formulas-and-rules.md#0-parameter-registry-all-tunables-one-place @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| DEAL-PARAM-1 | `STALLED_THRESHOLD_DAYS` | `60` | Idle threshold for the stalled flag (DEAL-FORM-3); absolute duration over UTC instants, not calendar days. Source constant; code edit + redeploy to change. |
| DEAL-PARAM-2 | `STALLED_ASKED_TO_WAIT_DAYS` | `90` | Suppression window applied when a captured deferral has no explicit date: capture may set the deal's wait-until to occurrence + this window. Source constant. |
| DEAL-PARAM-3 | `stage.win_probability` | per-stage `0–100` integer; seeded defaults in DEAL-FORM-1 | The parameter registry's **only** runtime-tunable exception: retunable per stage on the seeded pipeline via the bounded pipeline-edit surface ([[runtime-config#RC-1]]). Terminal stages pinned: won=100, lost=0 (DEAL-DDL-2 check). |

### Formulas — stage win-probability defaults (seeded default pipeline)
Source: contract/formulas-and-rules.md#5-stage-win-probability-defaults-seeded-default-pipeline @ 5a0b29c

**DEAL-FORM-1.** This chapter is the single home of the seeded values; the seed catalog
cites this pin. The seeded default sales pipeline (one per workspace, `is_default=true`)
ships these stages:

| ID (`position`) | `name` | `semantic` | `win_probability` |
|---|---|---|---|
| 0 | New | open | 10 |
| 1 | Qualified | open | 25 |
| 2 | Discovery | open | 40 |
| 3 | Proposal | open | 60 |
| 4 | Negotiation | open | 80 |
| 5 | Closed Won | won | 100 |
| 6 | Closed Lost | lost | 0 |

- **Output:** seed rows in `stage` (DEAL-DDL-2).
- **Worked example:** a deal in "Proposal" carries stage probability `0.60` for weighted
  value (DEAL-FORM-2).
- **Edge case:** a workspace retunes "Proposal" to 55 → the weighted roll-up and the
  forecasting chapter's defaults use 55; probabilities are read live from
  `stage.win_probability`, never hard-coded in a roll-up.
- **Tunable:** the seven values, per stage, via [[runtime-config#RC-1]] (DEAL-PARAM-3).

### Formulas — weighted pipeline value
Source: contract/formulas-and-rules.md#6-weighted-pipeline-value--amount--stage-probability-roll-up @ 5a0b29c

**DEAL-FORM-2.** Weighted pipeline = Σ over live open deals of `base_value ×
stage_probability`, decomposable to its constituents and reconciling exactly to their
sum. FX behavior is the data-model chapter's rule set ([[data-model#DM-FX-2]]..7),
applied here, not redefined.

- **Inputs:** `deal.amount_minor`, `deal.currency`, `stage.win_probability`,
  `deal.fx_rate_to_base`/`fx_rate_date` (closed) or the daily stored rate (open),
  `workspace.base_currency`.

```
# round() everywhere below = half away from zero ("half-up"), per deal — see tie-break
base_value(deal) =                                  # minor units in base currency
   deal.amount_minor                                if deal.currency = workspace.base_currency
 else round( deal.amount_minor * rate )             # rate = native→base
   where rate = deal.fx_rate_to_base                if deal.status != 'open'   (frozen at close)
              = fx_rate(deal.currency → base, as_of) if open                   (daily stored rate)

weighted_value(deal)   = round( base_value(deal) * stage.win_probability / 100 )
unweighted_pipeline    = Σ base_value(d)      over live open deals in scope
weighted_pipeline      = Σ weighted_value(d)  over live open deals in scope
```

- **Tie-break / rounding:** the rounding mode is **half away from zero ("half-up")**,
  applied **per deal** — in `base_value` and `weighted_value` alike — and the totals
  are the sums of the rounded parts, so the displayed total equals the sum of displayed
  per-deal weighted values exactly (reconciliation, AC-R11 — forecasting chapter).
  Boundary: a computed weighted value of `12,345.5` minor rounds away from zero to
  `12,346`.
- **Output:** `{ unweighted_minor, weighted_minor, base_currency, as_of_date }` + a
  per-deal breakdown `[{deal_id, base_value, win_probability, weighted_value}]` for
  "Explain This Number".
- **WORKED EXAMPLE** (base = EUR; as-of 2026-06-04):
  - Deal A: €100,000 (10,000,000 minor), stage Proposal (60%) → weighted = 6,000,000
    (€60,000).
  - Deal B: $50,000, open; daily rate USD→EUR = 0.92 → base = round(5,000,000 × 0.92) =
    4,600,000 (€46,000), stage Negotiation (80%) → weighted = 3,680,000 (€36,800).
  - unweighted = €146,000; weighted = €96,800. Breakdown sums exactly.
  - Rounding boundary micro-line: a deal whose weighted value computes to `12,345.5`
    minor rounds to `12,346` (half away from zero).
- **Edge cases:**
  - `amount_minor IS NULL` → contributes 0 to both totals; flagged in the breakdown as
    "no amount", never silently summed as 0 without a marker.
  - Mixed currencies: never `SUM` native minor units ([[data-model#DM-FX-4]]); always
    base. Caught by test.
  - Closed deals: excluded from *pipeline* (`status='open'` only); their frozen rate
    serves the forecasting chapter's won-value reports.
  - **FX-unavailable failure:** a roll-up needing a rate it does not have returns `422
    code: fx_rate_unavailable` (RFC-7807 `Problem`, offending `currency` + `as_of` in
    `details`) — never a silent rate=1 (DEAL-WIRE-8). Rate population and its ops
    alarming ride the data-model chapter's stored-rates rule
    ([[data-model#DM-FX-5]]); single-currency workspaces make the mechanism inert
    ([[data-model#DM-FX-1]]).
- **Tunables:** none — pure arithmetic over DEAL-FORM-1 probabilities and stored FX.

Note DEAL-FORM-N-1 (source: contract/formulas-and-rules.md §6.1 @ 5a0b29c; review
2026-07-03): the corpus named two rounding modes ("banker's/half-up"); **half away from
zero** is the decided default — one mode, applied per deal, everywhere this formula
runs.

### Formulas — stalled-deal rule with "asked to wait" suppression
Source: contract/formulas-and-rules.md#8-stalled-deal-rule--last_activity_at-threshold--asked-to-wait-suppression @ 5a0b29c

**DEAL-FORM-3.** A deterministic, fixed-clock-stable boolean over the deal's
last-activity instant, suppressed by a recorded customer deferral.

- **Inputs:** `deal.last_activity_at`, `deal.status`, `deal.created_at`,
  `deal.wait_until`.

```
is_stalled(deal, now_utc):
    if deal.status != 'open':            return false      # closed deals never stall
    if deal.last_activity_at IS NULL:    base = deal.created_at
    else:                                base = deal.last_activity_at
    idle = now_utc - base                                  # absolute duration (DST-immune)
    if idle <= STALLED_THRESHOLD_DAYS * 24h:  return false  # 60 days (DEAL-PARAM-1)

    # --- suppression: customer asked us to wait ---
    if asked_to_wait_until(deal) is not null
       and date(now_utc) <= asked_to_wait_until(deal):     # holds through the end of the wait day, UTC
           return false                                    # not stalled — we're respecting a wait
    return true

asked_to_wait_until(deal):
   # read the deal.wait_until column directly (no raw scan)
   if deal.wait_until IS NOT NULL: return deal.wait_until   # an explicit future date
   return null
```

- **Output:** boolean `is_stalled`; when stalled, a reason (default
  `no_activity_60_days` from this rule; richer reasons are the overnight agent's
  evidence join, morning-brief chapter).
- **WORKED EXAMPLE** (`now = 2026-06-04`, threshold 60d):
  - Last activity 2026-03-01 → idle ≈ 95 days > 60, no wait signal → **stalled**,
    reason `no_activity_60_days`.
  - Same deal, but a 2026-05-20 captured note "customer asked to revisit in Q3" sets
    `wait_until = 2026-09-01` → `date(now) ≤ wait_until` → **not stalled** (suppressed);
    after 2026-09-01 with still no activity → stalled again.
  - Last activity 2026-05-25 → idle ≈ 10 days → not stalled.
- **Edge cases:** never-any-activity falls back to `created_at` (an abandoned deal does
  flag); the wait boundary is pinned: suppression holds while `date(now_utc) ≤
  wait_until` — through the end of the wait day, UTC (decided 2026-07-03); a past
  `wait_until` means the suppression expired and the deal stalls normally; `last_activity_at` is maintained by the activities-and-timeline chapter's
  write path, so a back-dated captured activity correctly un-stalls. The commitment
  *extraction* that populates `wait_until` is L2 capture work (🟡, capture chapter); a
  dateless defer may set `wait_until = occurred_at + STALLED_ASKED_TO_WAIT_DAYS`
  (DEAL-PARAM-2). The wait suppresses the stalled flag only — not the forecasting
  chapter's overdue-close-date hygiene.
- **Tunables:** DEAL-PARAM-1, DEAL-PARAM-2.

### Schema
Source: contract/data-model.md#6-deals-pipelines-stages @ 5a0b29c

All DM-CONV universal conventions apply (base columns, RLS, provenance, money,
indexing — data-model chapter). Four tables are owned here, per the ownership index
([[data-model#schema--ownership-index]]). `fx_rate` shares the corpus section but is
owned by [[data-model#DM-DDL-12]]; `relationship` (stakeholders) by the
people-and-organizations chapter; `activity`/`activity_link` by
activities-and-timeline.

**DEAL-DDL-1 — `pipeline`.** At-most-one live default per workspace via partial unique
index; exactly-one is established by the workspace seed (DEAL-AC-13).

```sql
CREATE TABLE pipeline (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name         text NOT NULL,
  is_default   boolean NOT NULL DEFAULT false, -- exactly one seeded default per workspace
  position     integer NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL,
  CONSTRAINT pipeline_name_unique UNIQUE (workspace_id, name)
);
CREATE UNIQUE INDEX uq_pipeline_default ON pipeline (workspace_id) WHERE is_default AND archived_at IS NULL;
```

**DEAL-DDL-2 — `stage`.** Position unique per pipeline; terminal probability pinned by
check.

```sql
CREATE TABLE stage (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  pipeline_id   uuid NOT NULL REFERENCES pipeline(id) ON DELETE CASCADE,
  name          text NOT NULL,
  position      integer NOT NULL,            -- "stage.order" — unique within pipeline
  semantic      text NOT NULL DEFAULT 'open' CHECK (semantic IN ('open','won','lost')),
  win_probability smallint NOT NULL DEFAULT 0 CHECK (win_probability BETWEEN 0 AND 100),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,

  -- terminal-stage probability rule (features/01 §4.1): won=100, lost=0
  CONSTRAINT stage_terminal_prob CHECK (
    (semantic = 'won'  AND win_probability = 100) OR
    (semantic = 'lost' AND win_probability = 0)   OR
    (semantic = 'open')
  )
);
-- stage.order unique per pipeline; (pipeline_id, position) indexed (features/01 §4.1)
CREATE UNIQUE INDEX uq_stage_position ON stage (pipeline_id, position) WHERE archived_at IS NULL;
CREATE INDEX idx_stage_pipeline ON stage (pipeline_id) WHERE archived_at IS NULL;
```

Reorder is a single transaction that rewrites `position` (unique index checked at
COMMIT, or via a temporary offset); `(pipeline_id, position)` is both the uniqueness
key and the ordered-read index.

**DEAL-DDL-3 — `deal`.** Native money, FX freeze at close, stalled/wait columns,
rep-settable forecast category, partner attribution.

```sql
CREATE TABLE deal (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name            text NOT NULL,

  -- money (native; §1.4)
  amount_minor    bigint NULL,
  currency        char(3) NULL CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),

  -- FX freeze for base-currency roll-ups (data-semantics §1.2)
  fx_rate_to_base numeric(20,10) NULL,  -- native→base, frozen at close (NULL while open → use daily fx_rate)
  fx_rate_date    date NULL,

  pipeline_id     uuid NOT NULL REFERENCES pipeline(id) ON DELETE RESTRICT,
  stage_id        uuid NOT NULL REFERENCES stage(id)    ON DELETE RESTRICT,
  organization_id uuid NULL REFERENCES organization(id) ON DELETE SET NULL, -- primary org; never a raw lead (ADR-0008 §5)
  owner_id        uuid NULL REFERENCES app_user(id)     ON DELETE SET NULL,
  partner_org_id  uuid NULL REFERENCES organization(id) ON DELETE SET NULL, -- partner who sourced/co-sells this deal (A41/ADR-0032); deal registration + referral/margin attribution (A38)

  status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open','won','lost')),
  lost_reason     text NULL,
  expected_close_date date NULL,
  closed_at       timestamptz NULL,

  -- rep-settable forecast category (formulas-and-rules §7; rep override wins over probability-derived default)
  forecast_category text NULL CHECK (forecast_category IS NULL OR forecast_category IN ('commit','best_case','pipeline','omitted')),

  wait_until      date NULL,             -- 'customer asked us to wait until' date; suppresses the stalled flag (§8) but NOT the overdue close-date flag (§11); set from a captured commitment (🟡) or manually
  last_activity_at timestamptz NULL,      -- drives the deterministic stalled/idle flag (features/01 §4.1; data-semantics §2 r3)

  source          text NOT NULL,
  captured_by     text NOT NULL,
  raw             jsonb NULL,

  search_tsv      tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(name,''))) STORED,

  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL,

  -- close requires status terminal + lost_reason when lost (features/01 §3.1)
  CONSTRAINT deal_lost_reason CHECK (status <> 'lost' OR lost_reason IS NOT NULL),
  CONSTRAINT deal_closed_at   CHECK (status = 'open' OR closed_at IS NOT NULL),
  -- closed deals must have a frozen FX rate so base-currency roll-ups are reproducible (data-semantics §1.2)
  CONSTRAINT deal_closed_fx   CHECK (status = 'open' OR amount_minor IS NULL OR fx_rate_to_base IS NOT NULL)
);

CREATE INDEX idx_deal_ws_live    ON deal (workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_deal_stage      ON deal (stage_id) WHERE archived_at IS NULL;        -- Kanban column read
CREATE INDEX idx_deal_pipeline   ON deal (pipeline_id, stage_id) WHERE archived_at IS NULL;
CREATE INDEX idx_deal_owner      ON deal (workspace_id, owner_id) WHERE archived_at IS NULL;
CREATE INDEX idx_deal_org        ON deal (organization_id) WHERE organization_id IS NOT NULL AND archived_at IS NULL; -- "open deals per company"
CREATE INDEX idx_deal_partner    ON deal (workspace_id, partner_org_id) WHERE partner_org_id IS NOT NULL AND archived_at IS NULL; -- "deals sourced/co-sold by partner X" (A41)
CREATE INDEX idx_deal_stalled    ON deal (workspace_id, last_activity_at) WHERE status = 'open' AND archived_at IS NULL;
CREATE INDEX idx_deal_close      ON deal (workspace_id, expected_close_date) WHERE status = 'open' AND archived_at IS NULL; -- forecast by period
CREATE INDEX idx_deal_search     ON deal USING gin (search_tsv);
```

**DEAL-DDL-4 — `deal_stage_history`.** Append-only transition snapshots, sourced from
the stage-change write; powers as-of-date pipeline, stage conversion, and rolling
coverage (consumed by the forecasting chapter).

```sql
CREATE TABLE deal_stage_history (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  deal_id       uuid NOT NULL REFERENCES deal(id) ON DELETE CASCADE,
  from_stage_id uuid NULL REFERENCES stage(id) ON DELETE SET NULL,
  to_stage_id   uuid NOT NULL REFERENCES stage(id) ON DELETE RESTRICT,
  changed_by    text NOT NULL,                 -- principal string (human:/agent:)
  changed_at    timestamptz NOT NULL DEFAULT now(),
  amount_minor_at_change bigint NULL,          -- snapshot for as-of-date value
  currency_at_change     char(3) NULL
);
CREATE INDEX idx_dsh_deal ON deal_stage_history (deal_id, changed_at);
CREATE INDEX idx_dsh_ws_time ON deal_stage_history (workspace_id, changed_at); -- as-of-date scans
```

Note DEAL-DDL-N-1 (source: contract/data-model.md#6-deals-pipelines-stages, OQ-5 @
5a0b29c): the "stage belongs to its pipeline" rule cannot be a clean single-table CHECK
across two FKs; the corpus prefers the **composite-FK pattern** — `UNIQUE (id,
pipeline_id)` on `stage` plus FK `deal (stage_id, pipeline_id) → stage (id,
pipeline_id)` — over a trigger (DB-guaranteed). The build must implement one of the
two; DEAL-AC-1 pins the observable rule either way. Pinned concretely as the migration
ticket's spec in DEAL-EXT-6.

### Wire
Source: contract/crm.yaml (Deals, Pipelines tags) @ 5a0b29c

Operations cited by `operationId`, never restated; list envelope, cursor pagination,
idempotency, `If-Match`/version, and the error catalog are the api-conventions
chapter's.

| ID | Operations | Notes |
|---|---|---|
| DEAL-WIRE-1 | `listDeals` | Live-by-default, cursor-paginated; filters: `pipeline_id`, `stage_id` (one Kanban column read), `owner_id`, `organization_id`, `status`, `stalled` (the DEAL-FORM-3 boolean). Serves board and table (PERF-2 budget). |
| DEAL-WIRE-2 | `createDeal` | 201 + Location; idempotency-key replay per [[api-conventions#API-CC-6]]. Stage must belong to the pipeline → else `422 validation_error` with field error `stage_not_in_pipeline` (DEAL-WIRE-8). Attaches to person/org, never a raw lead (ADR-0008 §5). |
| DEAL-WIRE-3 | `getDeal` · `updateDeal` · `archiveDeal` | The 360 read (PERF-1); partial merge update (closing requires terminal status + `lost_reason` if lost, else 422); archive is soft-delete ([[api-conventions#API-ERR-4]]). MCP-exposed archive is 🟡 (tool table: byo-agent-and-mcp chapter). |
| DEAL-WIRE-4 | `advanceDeal` | The deal-advance verb; writes one `deal_stage_history` row + one audit row (prior + next stage) and emits `deal.stage_changed` (DEAL-EVT-3). **Dynamic tier:** resolver reads BOTH endpoints' `semantic` — a transition is 🟡 when either endpoint is terminal (won/lost): close and reopen alike; open→open is 🟢 in either direction. Agent callers on a 🟡 transition need `X-Approval-Token` ([[approvals-and-concurrency#APPR-WIRE-1]]) else `403 approval_required` ([[api-conventions#API-ERR-10]]). Carries the deal's version, If-Match semantics per [[api-conventions#API-CC-2]] (DEAL-AC-B2). Accepts an idempotency key. |
| DEAL-WIRE-5 | `listDealStakeholders` | The deal↔person relationship reverse-lookup (rows owned by people-and-organizations; indexed both directions, DEAL-AC-10). |
| DEAL-WIRE-6 | `listPipelines` · `createPipeline` · `getPipeline` · `updatePipeline` | `getPipeline` returns ordered stages; `updatePipeline` is the bounded rename/reorder/default surface ([[runtime-config#RC-1]]); a second default → 409. |
| DEAL-WIRE-7 | `listStages` · `createStage` · `getStage` · `updateStage` | Stages ordered by position within pipeline; `position` unique per pipeline (409 on collision); terminal probability enforced (won=100, lost=0 → else 422); `updateStage` carries the RC-1 probability retune. |
| DEAL-WIRE-8 | Error surface | `422 validation_error` with `details.errors[{field, code}]` incl. `stage_not_in_pipeline` ([[api-conventions#API-ERR-15]]); `422 fx_rate_unavailable` on a roll-up missing a stored rate (DEAL-FORM-2 — RFC-7807 `Problem` with `currency` + `as_of` in `details`); `403 approval_required` / `approval_token_invalid` on the governed advance ([[api-conventions#API-ERR-10]], [[api-conventions#API-ERR-11]]); `409 version_skew` on stale `If-Match` ([[api-conventions#API-ERR-8]]). |
| DEAL-WIRE-9 | Terminal advance write semantics | The server derives `status` from the target stage's `semantic`; an explicit `status` in the request that mismatches the target semantic → `422 validation_error`. Advance-to-terminal sets `closed_at = now` and freezes `fx_rate_to_base`/`fx_rate_date` using the stored daily rate for the close date per [[data-model#DM-FX-3]]; the `deal_closed_at`/`deal_closed_fx` CHECKs (DEAL-DDL-3) are satisfied by the same write. |

### Wire — contract extensions (D-H2)
Source: chapter review D-H2, 2026-07-03 (extends contract/crm.yaml @ 5a0b29c)

Rule: these are **contract-first extension tickets** — `crm.yaml` grows before any
handler.

| ID | Extension | Spec |
|---|---|---|
| DEAL-EXT-1 | Pipeline roll-up read | A read returning DEAL-FORM-2's output object: per-column and per-pipeline unweighted/weighted totals + the per-deal breakdown + the fx-unavailable error surface (DEAL-WIRE-8). The totals are never client-computed. |
| DEAL-EXT-2 | `listDeals` row fields + filter | Each row gains `stage_entered_at` and `stakeholder_count` (the board card model); a `person_id` filter serves the person-360 Deals tab / reverse lookup per DEAL-AC-10. |
| DEAL-EXT-3 | Composite deal-360 read | One composite read per the one-composite-read doctrine (architecture/frontend.md): record + stakeholders + timeline refs in one round trip. |
| DEAL-EXT-4 | Restore (un-archive) operation | `deal.restored` (DEAL-EVT-6) currently has no trigger operation; the restore op supplies it. |
| DEAL-EXT-5 | Stakeholder role enum | The stakeholder `role` becomes a CHECK-constrained enum, NULL disallowed for the stakeholder kind (today: free text, NULL allowed, NULLs-distinct unique — duplicate NULL-role stakeholders possible). |
| DEAL-EXT-6 | Composite stage-in-pipeline FK | The DEAL-DDL-N-1 delta pinned concretely: `UNIQUE (id, pipeline_id)` on `stage` + composite FK `(stage_id, pipeline_id)` on `deal` — the migration ticket's exact spec. |

### Events
Source: contract/events.md#53-deal @ 5a0b29c

Definitions live in the central catalog ([[event-bus#events--catalog]], stream
[[event-bus#EVT-STREAM-3]]); cited here, not redefined. All ride the standard envelope
and outbox ([[event-bus#EVT-ENV-1]], [[event-bus#EVT-DEL-5]]).

| ID | Event | Role here |
|---|---|---|
| DEAL-EVT-1 | `deal.created` | Emitted on create (incl. create-from-context). |
| DEAL-EVT-2 | `deal.updated` | Emitted on non-stage, non-owner field changes (delta payload). |
| DEAL-EVT-3 | `deal.stage_changed` | **The load-bearing deal event** — emitted instead of `deal.updated` on a stage transition, from the same write that appends DEAL-DDL-4; carries `from/to_stage_id`, `from/to_status`, the amount snapshot, and `win_probability`. Closed-won is this event with `to_status = won` (see note DEAL-EVT-N-1). |
| DEAL-EVT-4 | `deal.owner_changed` | Owner reassignment has its own event, not a delta. |
| DEAL-EVT-5 | `deal.archived` | Soft-delete emission. |
| DEAL-EVT-6 | `deal.restored` | Un-archive emission. |

Note DEAL-EVT-N-1 (source: contract/events.md#53-deal; contract/crm.yaml `advanceDeal`
@ 5a0b29c): `features/01 §3.1` says "closed-won emits `deal.won`"; the ratified event
catalog contains **no** `deal.won` — the closed-won fact is `deal.stage_changed` with
`to_status=won`, as the contract's advance operation states. The catalog is the
promotion-of-record here; DEAL-AC-6 is pinned verbatim but verified against
DEAL-EVT-3.

Note DEAL-N-MODULE (source: architecture chapter Tier-1 roster; review 2026-07-03;
reconciled against skeleton@45aa8a7, 2026-07-03: the shipped spine holds the deal and
pipeline stores and a terminal gate implementing the superseded target-semantic-only
tier rule — the deals-module tickets migrate that substrate here and replace the gate
logic with the either-endpoint rule pinned in this chapter): the
architecture chapter's Tier-1 roster names modules/people as the first bounded context
and states the roster grows; deals lands as its own module
(`backend/internal/modules/deals`, this chapter's front-matter) per the target
structure's bounded-context doctrine. The event catalog's emitter column for the
`deal.*` rows is corrected accordingly ([[event-bus#events--catalog]]).

### Acceptance — owned stories
Source: product/epics/E03-pipeline-and-deals.md @ 5a0b29c

Cross-cutting floor (screen states, budgets, core gates) inherited from
[[acceptance-standards#STATE-1]]..5 and [[acceptance-standards#GATE-CORE-1]]..8, not
restated. S-E03.7 (Angebot) is owned by the offers-and-products chapter.

| ID | Tier | Condensed Given/When/Then | Verification |
|---|---|---|---|
| S-E03.1 | V1-Must | Given a pipeline with deals, when the rep opens the board or switches to the table, then the same deals render as stage-grouped cards (count + aging per card) or sortable/filterable rows, switching without reload or filter loss, cursor-paged, within the fast-list budget ([[acceptance-standards#PERF-2]]); stalled/aging state is visible at a glance. | Screen ACs AC-pipeline-1/2/5/7/8; CI perf benchmark; frontend e2e |
| S-E03.2 | V1-Must | Given a card dragged to a new stage, when it drops, then one save persists it (PERF-4 budget) with one audit row, one history row, one `deal.stage_changed` (DEAL-EVT-3); a terminal target asks for the outcome (+ lost-reason if lost) as a 🟡 confirm; the dated stage-by-stage history answers "how did this deal move". | Integration test (one-write/one-audit/one-event); AC-pipeline-3/4; AC-deal-6/10 |
| S-E03.3 | V1-Must | Given a deal, when opened, then one fast view (PERF-1) shows stakeholders with roles, the auto-captured timeline, the linked org, and the current stage; a single-stakeholder deal shows its single-threaded risk; an inferred next step is shown confirm-or-dismiss, never a blank required field. *The inference is produced by the meetings-and-transcripts chapter's intelligence — owned dependency, cited honestly.* | AC-deal-1/4/7/10; integration test over the 360 read; e2e confirm/dismiss |
| S-E03.4 | V1-Must | Given deals with amounts and stage probabilities, when pipeline value renders, then raw and weighted totals match DEAL-FORM-2 exactly (hand-verifiable), decompose per stage/deal, handle currencies honestly (base conversion or hard `fx_rate_unavailable` failure); won counts at 100% and lost at 0% in forecast roll-ups — closed deals never distort the open-pipeline totals (DEAL-FORM-2 sums open deals only; the 100/0 accounting is the forecasting chapter's). | Unit tests on DEAL-FORM-2 (worked example as fixture); reconciliation property test; AC-pipeline-2, AC-deal-2 |
| S-E03.5 | V1-Must | Given a contact, company, or captured thread, when the rep creates a deal from there, then it opens with org linked, people attached as stakeholders, recent activity on the timeline, landing in the default pipeline's first stage — creation from context pre-fills everything, requiring the rep to confirm/adjust only name, amount, and close date; a likely duplicate for that org warns before creating. | AC-pipeline-9/10; integration test on `createDeal` from context; duplicate-warning e2e |
| S-E03.6 | V1-Must | Given a partner-sourced deal, when registered, then the partner **organization** (first-class, classified partner) is linked — not free text; partner-sourced pipeline filters as its own reportable slice (indexed); the partner org's record shows the deals it sourced and its contributed pipeline (rendering rides the org-360; the filter slice is pinned here); attribution changes are audited without losing the prior value; V1 has no partner portal or program operations. | Integration test on `partner_org_id` + `idx_deal_partner` path; audit-trail assertion; reporting filter test |

### Acceptance — feature criteria (verbatim)
Source: features/01-core-objects.md#3-deals, #4-pipelines--stages @ 5a0b29c

| ID | Given/When/Then (corpus text, verbatim) | Verification |
|---|---|---|
| DEAL-AC-1 | `deal` has FK to exactly one `pipeline` and one `stage`, and the stage **must** belong to that pipeline (constraint or validated write). | Schema test on DEAL-DDL-N-1 pattern; negative write test |
| DEAL-AC-2 | Amount stored as integer minor-units + ISO-4217 currency (no float money). | Schema assertion ([[data-model#DM-CONV-9]]) |
| DEAL-AC-3 | Closing a deal requires `status ∈ {won,lost}` and, if lost, a non-null `lost_reason`. | DB check `deal_lost_reason` + API 422 test |
| DEAL-AC-4 | Stakeholders modeled via typed `relationship` (deal↔person, role) — not a comma field. | Schema/integration test ([[acceptance-standards#GATE-CORE-4]]) |
| DEAL-AC-5 | Save **p95 < 150 ms**; open **p95 < 100 ms** server. | CI benchmark (restates [[acceptance-standards#PERF-4]], [[acceptance-standards#PERF-1]]) |
| DEAL-AC-6 | `advance_deal` / close transitions are audit-logged with prior+next stage; closed-won emits `deal.won`. | Integration test; event assertion — see note DEAL-EVT-N-1 (catalog verb is `deal.stage_changed` with `to_status=won`) |
| DEAL-AC-7 | **User-observable:** opening a deal 360, the rep sees the inferred next step pre-filled from recent captured activity (e.g. "send the contract you promised Friday") and can accept it as a task in one click or dismiss it — they do not start from a blank next-action field (S-E03.3). | e2e (AC-deal-4); inference dependency: meetings-and-transcripts |
| DEAL-AC-8 | **User-observable:** the rep can create a deal directly from a contact, email thread, or meeting and the org, stakeholders, and recent activity are already linked — they do not re-key context that the CRM already holds (S-E03.5). | e2e (AC-pipeline-9/10) |
| DEAL-AC-9 | ≥0 stakeholders per deal, each with a role from a defined enum, via `relationship` table. | Integration test (rows owned by people-and-organizations) |
| DEAL-AC-10 | "Deals where contact = X" and "stakeholders of deal Y" are both indexed reverse-lookups (**p95 < 150 ms**). | CI benchmark (restates [[acceptance-standards#PERF-2]]) |
| DEAL-AC-11 | **User-observable:** on the deal 360 the rep sees every stakeholder with their role (champion, economic buyer, blocker) drawn from captured email/meeting participants — they did not type the buying committee in by hand (S-E03.3). | e2e (AC-deal-7) |
| DEAL-AC-12 | `stage.order` unique within a pipeline; `(pipeline_id, order)` indexed; reorder is transactional. | Schema test on `uq_stage_position`; concurrent-reorder integration test |
| DEAL-AC-13 | Exactly one default pipeline seeded on workspace init; deals require a pipeline. | Seed test on `uq_pipeline_default`; NOT NULL FK |
| DEAL-AC-14 | Probability is 0–100 integer; won=100, lost=0 enforced for terminal stages. | DB check `stage_terminal_prob` + API 422 test |
| DEAL-AC-15 | Kanban/list view of a stage **p95 < 150 ms** server for 50 cards (cursor pagination beyond). | CI benchmark (restates [[acceptance-standards#PERF-2]]) |
| DEAL-AC-16 | Drag-to-advance issues one save (**p95 < 150 ms**), one audit row, one `deal.stage_changed` event. | Integration test ([[acceptance-standards#GATE-CORE-5]]); CI benchmark |
| DEAL-AC-17 | Stalled flag is a deterministic, testable rule over `last_activity_at` (fixed clock test → stable boolean). | Fixed-clock unit tests on DEAL-FORM-3 (worked example as fixture) |
| DEAL-AC-18 | **User-observable:** the rep can switch between a Kanban board and a table of the same deals and drag a card to the next stage; the move sticks immediately and the deal's aging/stalled indicator updates without a manual refresh (S-E03.1, S-E03.2). | e2e (AC-pipeline-3/7) |
| DEAL-AC-19 | **User-observable:** every stage move is visible later as dated history on the deal (who/what moved it, when), so a rep or leader can see how a deal progressed rather than only its current stage (S-E03.2). | Integration + e2e (AC-deal-10; DEAL-DDL-4) |

### Acceptance — chapter pins (reopen, history, board mechanics)
Source: chapter review D-H2, 2026-07-03 (extends features/01-core-objects.md#3-deals, product/30-screen-acceptance.md pipeline.html @ 5a0b29c)

| ID | Given/When/Then | Verification |
|---|---|---|
| DEAL-AC-R1 | Given a closed deal, when it is moved terminal→open (reopen), then the transition is 🟡 and requires the approval token exactly like close (either-endpoint rule, DEAL-WIRE-4). | Integration test |
| DEAL-AC-R2 | Given a reopen, when the write commits, then `closed_at` and `lost_reason` are cleared and the frozen `fx_rate_to_base`/`fx_rate_date` are cleared — the deal reverts to daily-rate conversion; values re-freeze at the next close. | Integration test |
| DEAL-AC-R3 | Given a reopen, when the write commits, then the stage-history row records the reopen transition (semantic terminal→open) like any other move. | Integration test |
| DEAL-AC-H1 | Given deal creation, when the deal row is written, then an initial `deal_stage_history` row is written with `from_stage` NULL → the initial stage, so as-of-date reconstruction needs no special case. | Integration test |
| DEAL-AC-B1 | Given a board drag, when the card drops, then the move is optimistic — the card moves immediately; on `409 version_skew`, `403` approval-required-not-satisfied, or `422` it snaps back with an honest toast naming the cause, and the column refetches. | Screen e2e |
| DEAL-AC-B2 | Given a board drag or Advance click, when the save issues, then `advanceDeal` carries the deal's version (If-Match semantics per [[api-conventions#API-CC-2]]). | Screen e2e |
| DEAL-AC-B3 | Given the board, when it renders, then terminal stages are not standing columns — Won/Lost drop zones appear during drag (and are reachable via the Advance button's dialog); the five open-stage columns stay AC-pipeline-1's truth. | Screen e2e |
| DEAL-AC-B4 | Given the Advance button, when clicked, then it targets the next stage in order: open→open advances directly (🟢); a terminal target opens the outcome dialog (per AC-deal-6). In-column card order defaults to amount desc (same as the table default). | Screen e2e |

### Acceptance — screen: pipeline board & table
Source: product/30-screen-acceptance.md#23-deals-forecast-tasks (pipeline.html) @ 5a0b29c

Implements S-E03.1/.2; new-deal modal S-E03.5. Standard state floor
[[acceptance-standards#STATE-1]]..4 applies (the prototype's noted missing
loading/error/empty-board/no-permission states are build requirements, not exemptions).

| ID | Given/When/Then (corpus text, verbatim) | Verification |
|---|---|---|
| AC-pipeline-1 | Given the board view is active, When the page loads, Then deals render as cards grouped into five stage columns (New 10%, Qualified 25%, Discovery 40%, Proposal 60%, Negotiation 80%); each column header shows stage name, default probability %, and deal count, and a sub-line shows the raw sum and `weighted sum×prob%`. | Frontend e2e; probabilities from DEAL-FORM-1 |
| AC-pipeline-2 | Given the toolbar totals strip, When the board renders or any deal moves, Then "Weighted" (Σ amount×stage-prob, accent orange), "Raw pipeline" (Σ amount), and "Open deals" (count) update to match the board. | Frontend e2e; totals reconcile per DEAL-FORM-2 |
| AC-pipeline-3 | Given a deal card, When dragged from one column and dropped on a different stage, Then the drop target highlights during hover, the card persists in the new column, its in-stage age resets to 0d, and a toast confirms "<Co> moved <from> → <to> · stage history recorded". | Frontend e2e over DEAL-WIRE-4 |
| AC-pipeline-4 | Given a deal card is clicked without a drag, When the click resolves, Then it navigates to deal.html; a click that is the tail end of a drag does NOT navigate. | Frontend e2e |
| AC-pipeline-5 | Given a card with risk/aging signals, When it renders, Then a stalled deal shows a left orange border + "Stalled Nd" amber flag, and a single-stakeholder deal shows a red "Single-threaded" flag, each with a hover title. | Frontend e2e; stalled from DEAL-FORM-3 |
| AC-pipeline-6 | Given a deal carrying an inferred next step, When the card renders, Then a dashed-accent "Next:" panel shows the suggested step + `inferred from <source>` + ✓/✗ confirm pair; ✓ toasts "Next step confirmed · task created", ✗ toasts "Suggestion dismissed", either removes the panel. | Frontend e2e; inference: meetings-and-transcripts |
| AC-pipeline-7 | Given the Board/Table segmented control, When the user selects Table, Then the same deals appear as rows (Company, Deal, Stage pill, Prob., Amount, Weighted, Age) sorted by amount desc, no reload; switching back restores the board. | Frontend e2e |
| AC-pipeline-8 | Given the table view, When a stalled deal renders, Then its Age cell is amber + ⚠. | Frontend e2e |
| AC-pipeline-9 | Given the "New deal" button, When clicked, Then a modal opens pre-filled from context (company linked, deal name suggested from thread, stakeholders attached, default pipeline/stage, recent activity count), with a duplicate-deal warning when an open deal already exists for that org. | Frontend e2e over DEAL-WIRE-2 |
| AC-pipeline-10 | Given the new-deal modal, When "Confirm & create" is clicked, Then a toast confirms "Deal created from context · org + people + recent activity pre-attached" and the modal closes; "Cancel" closes with no change. | Frontend e2e |

Note DEAL-AC-N-1 (source: product/30-screen-acceptance.md pipeline.html "Open
questions" @ 5a0b29c; rewritten per review 2026-07-03): the corpus flags
terminal/backward board moves for build confirmation. This chapter's rule: a stage
transition is 🟡 when **either endpoint** is a terminal stage — close and reopen both
governed, and DEAL-WIRE-4 governs board and 360 alike; open→open transitions in **any
direction** are 🟢, so a backward open→open drag shows no confirm — the corpus flag's
terminal concern is covered by the either-endpoint rule. Multi-currency totals render
per DEAL-FORM-2 (not hard-coded EUR); the "task created" toast must create a real task.

### Acceptance — screen: deal 360
Source: product/30-screen-acceptance.md#23-deals-forecast-tasks (deal.html) @ 5a0b29c

Implements S-E03.3/.5 (this chapter) and S-E04.1/.3/.4 (meetings-and-transcripts —
dossier and inferred-signal content is theirs; the screen's ACs pin here as the screen
owner). Standard state floor applies.

| ID | Given/When/Then (corpus text, verbatim) | Verification |
|---|---|---|
| AC-deal-1 | Given a deal, When it opens, Then the header shows deal name, linked company (→ company.html), deal value, a stage stepper with prior stages done, the current stage highlighted, and terminal Won (100%)/Lost (0%) nodes muted. | Frontend e2e |
| AC-deal-2 | Given the KPI strip, When the user clicks "Explain this number" under Weighted value, Then a popover expands showing `value × stage-prob = weighted` plus the rule "Won = 100%, Lost = 0%"; collapse toggles. | Frontend e2e; arithmetic per DEAL-FORM-2 |
| AC-deal-3 | Given the win-probability KPI, When it renders, Then it is labeled "stage default (deterministic) · <stage>" — distinguishing it from the inferred close-likelihood signal in the right rail. | Frontend e2e |
| AC-deal-4 | Given the inferred next-step card, When it renders, Then it shows the proposed step + due date, "🟡 confirm to create", and an evidence quote with a clickable source ("open transcript"); ✓ toasts task-created and removes the card, ✗ toasts dismissed/"agent will learn" and removes it. | Frontend e2e; inference: meetings-and-transcripts |
| AC-deal-5 | Given the qualification checklist, When it renders, Then each confirmed item shows a green check with its source citation, and each gap shows an amber help icon, an italic "Couldn't infer …", and an accent dashed CTA; a footer states "N of 5 confirmed from captured signal." | Frontend e2e; content: meetings-and-transcripts |
| AC-deal-6 | Given "Advance stage", When clicked, Then a confirm dialog frames it as a high-value 🟡 transition; OK records Closed Won (toast: weighted 100%), Cancel prompts for an optional lost-reason (non-blank → Closed Lost recorded with reason, weighted 0%; blank → kept open, nothing changed). | Frontend e2e over DEAL-WIRE-4 |
| AC-deal-7 | Given the stakeholders rail, When it renders, Then each contact shows name, title, and a role flag (Champion/Stakeholder/Blocker), plus a "Multi-threaded" indicator and an explicit "No economic buyer identified yet" gap notice. | Frontend e2e over DEAL-WIRE-5 |
| AC-deal-8 | Given the pre-meeting dossier, When it renders, Then it lists what-hurts-them / why-worth-their-time / paths-to-close, each with an evidence quote, a clickable source, and a confidence chip (High/Medium), labeled "advisory, not written to the record." | Frontend e2e; content: meetings-and-transcripts |
| AC-deal-9 | Given the inferred deal signals card, When it renders, Then close-likelihood, expected volume, and timing each show a value, a confidence dot, and an evidence quote+source, AND any ungrounded signal renders as "Not grounded — omitted (no guess)" with no value. | Frontend e2e; [[acceptance-standards#STATE-5]]; content: meetings-and-transcripts |
| AC-deal-10 | Given the activity timeline, When it renders, Then it shows auto-captured emails/calls/meetings each with timestamp + source connector provenance and a footer "You logged none of this," and the stage-history card lists each transition with a date. | Frontend e2e; history from DEAL-DDL-4; timeline: activities-and-timeline |
| AC-deal-11 | Given the Tasks card, When the user clicks a task's checkbox, Then it marks done (toast); each task shows assignee, an explicit due date (distinct from the "Stalled" signal), and "created by" provenance. | Frontend e2e; tasks: activities-and-timeline |

Note DEAL-N-PILOT (source: review 2026-07-03): the pilot excludes RC-13 mechanics and
the AI-fed surfaces — AC-pipeline-6's inferred-next-step panel content and AC-deal-4's
inference content (the ✓/✗ write path is the tasks/meetings chapters'); everything else
in this chapter is pilot scope.
