---
status: planned
module: backend/internal/modules/reporting (forecast roll-ups, deal-health scoring, close-date hygiene rules — rides the reporting read path) · web (the forecast surface inside the reports feature)
derives-from:
  - specs/spec/features/03-reporting-and-scoring.md#2-forecasting @ 5a0b29c
  - specs/spec/features/03-reporting-and-scoring.md#3-lead-scoring--routing @ 5a0b29c
  - specs/spec/contract/formulas-and-rules.md#7-forecast-categories--commit--best-case--pipeline--roll-up @ 5a0b29c
  - specs/spec/contract/formulas-and-rules.md#105-deal-health-score--transparent-weighted-factor-model-e09-features03-32 @ 5a0b29c
  - specs/spec/contract/formulas-and-rules.md#11-close-date-hygiene--realism-ratified--decisions-a6 @ 5a0b29c
  - specs/spec/contract/data-model.md#saved-views-quota-field-mask @ 5a0b29c
  - specs/spec/product/epics/E09-reporting-and-forecast.md @ 5a0b29c
  - margince-poc/docs/subsystems/deals.md @ a11d6c08
---
# Forecasting — a forecast the leader can defend, on dates nobody knows are wrong

> The leader's number: open pipeline rolled up by forecast category, coverage against
> quota, accuracy against what actually closed, a deal-health read inferred from the
> captured conversation, and a hard rule that no open deal ever claims a close date in
> the past. Its single promise: every forecast figure reconciles exactly to the deals
> behind it, and no human-set number is ever silently moved.

## What it's for

A forecast is only useful if the leader is willing to commit to it upward, and that
willingness dies the first time a number cannot be explained or turns out to rest on a
close date everyone knew was wrong. This subsystem exists so Riya can answer "why is
this number what it is?" on the spot: it owns the forecast-category ladder and its
roll-up, the coverage and accuracy metrics over the stage history, the deterministic
deal-health score with its evidence, and the close-date hygiene rule that keeps the
dates under the forecast honest. Its callers are the leader's forecast surface (a
screen owned by the [reporting](reporting.md) chapter), the [morning-brief](morning-brief.md)
(whose per-deal confidence and revenue impact must be the same values this roll-up
consumes), the [overnight-agent](overnight-agent.md) (which executes the close-date
correction policy this chapter defines), and any BYO agent running what-if analysis
through the governed report tool. The boundary: this chapter owns the forecast
*semantics* — categories, roll-up, coverage, accuracy, health, hygiene — not the
weighted-pipeline arithmetic (deals-and-pipeline's), not the report engine or the
explain-this-number primitive (reporting's), and not the approval mechanics
(approvals-and-concurrency's).

## Principles it serves

- **P11 — clean relational core.** Every forecast total is a real aggregate over real
  deal rows with real category and date columns; commit decomposes to the deals a
  leader can open, and the accuracy report replays from append-only stage history
  rather than a reconstructed guess.
- **P12 — governance designed in.** The rep's forecast category is never silently
  overwritten by an inference; the advisory health read sits next to the human's call;
  a forecast-bearing close date is never moved without a human confirm; the one
  sanctioned automatic category write — the quiet-deal downgrade — is flagged, audited,
  and surfaced for review.
- **P6 — embrace the LLMs, without black boxes.** The deal-health score itself is a
  deterministic weighted composite; the model layer may explain it and never computes
  it, so the number is reproducible and the explanation is decomposition, not story.
- **P1 / ADR-0002 — opinionated over configurable.** The category cutoffs, health
  weights, and hygiene thresholds are named source constants with one ratified default
  each — no runtime tuning surface.
- **ADR-0004 — the reporting read path.** The forecast is a parameterized read over
  the same typed query-plan engine as every other report — deliberately not a separate
  subsystem — so reconciliation, lineage, and RBAC are inherited structurally.

## How it works

**Every open deal sits in exactly one forecast category.** The ladder is commit,
best-case, pipeline, omitted — commit is what the rep will stake the number on,
best-case is upside, pipeline is everything else open in the period, omitted is an
explicit human exclusion. The category is rep-settable and the rep's setting always
wins; the system only supplies a *default* for unset deals, derived deterministically
from the stage's win probability — at or above ninety it defaults to commit, at or
above fifty to best-case, otherwise pipeline (FCAST-PARAM-1, FCAST-PARAM-2). Nothing
in the product — not the health score, not the overnight run, not an agent — silently
rewrites a rep-set category; the single exception is the flagged, reviewed downgrade
described below (FCAST-FORM-3).

**The roll-up reconciles, by construction.** For an owner or team and a period
(bucketed in the workspace zone), the roll-up sums base-currency deal values per
category: commit is the commit deals, best-case *includes* commit plus the best-case
deals, pipeline includes both plus the rest (FCAST-FORM-1). Each deal lands in exactly
one category and is counted once regardless of how many stakeholders it carries. Closed
deals follow the hundred-zero rule this chapter owns per the deals chapter's story
row: a won deal counts at its full value inside commit — real revenue stays in the
number — while a lost deal is excluded from every category; and because the
open-pipeline totals ([[deals-and-pipeline#DEAL-FORM-2]]) sum open deals only, closed
deals never distort them. The per-deal arithmetic underneath — base-currency
conversion, frozen rates at close, per-deal rounding — is the deals chapter's formula
and the data-model chapter's FX rules ([[data-model#DM-FX-3]],
[[data-model#DM-FX-4]]), consumed here, never re-derived. A deal with no close date is
never silently bucketed into the current period; it is excluded from period roll-ups
and surfaced as needing a date.

**Coverage and accuracy come from history, not memory.** The rolling
pipeline-coverage ratio — open pipeline divided by remaining quota — is computed
against the quota object (owned by the records-depth chapter) with the open-pipeline
side reproducible as of any date from the append-only stage history the deals chapter
records (FCAST-FORM-4, derived). When a period closes, a scheduled job captures one
predicted-versus-actual snapshot per scope — the prediction persisted is the number
the leader actually saw, not a later re-derivation — so the accuracy report is
reproducible and honest about how good past forecasts were (FCAST-DDL-1,
FCAST-EVT-1).

**Deal health is inferred from the conversation and shown with its evidence.** Each
open deal carries a health score built from what capture actually observed — activity
recency, stage velocity against the workspace's own pace, breadth of two-way
stakeholder engagement, and kept-versus-overdue commitments — as a deterministic
weighted composite (FCAST-FORM-2), so a deal that looks fine on stage but has gone
quiet reads as at-risk (below the at-risk floor, FCAST-PARAM-6). The score is a health
lens, deliberately distinct from the morning brief's priority composite. The AI layer
may *explain* the score — joining each factor to the source emails, meetings, and
overdue commitments behind it, through the same explain primitive as every other
number — but never computes it, never writes it to a deal field, and never rewrites
the rep's stage or category: the read is advisory, rendered next to the human's call,
and framed as a rep-copilot coaching signal transparent to the rep, not a covert
manager metric ([[acceptance-standards#GATE-AI-1]],
[[acceptance-standards#GATE-AI-2]]). In V1 this deterministic advisory is also what
stands in for the predictive close-probability the feature spec sketches — a trained
per-workspace model is consciously deferred.

**The brief and the forecast see the same numbers.** The morning brief's per-item
confidence *is* the deal-health score and its revenue impact is the stage-weighted
value ([[morning-brief#BRIEF-FORM-2]]); this chapter's roll-up must consume those
exact per-deal values, byte-identical, so "why is this deal weighted this way?" has
one answer everywhere.

**No open deal ever claims a close date in the past.** That state is invalid — the
deal asserts it closed, yet it hasn't — and it is a hard invariant (INV-CLOSE-PAST),
enforced at the write layer (saving a past close date on an open deal is rejected at
source) and swept nightly so nothing that slips through survives a run. Four
deterministic flags mark trouble: overdue, missing, unrealistically soon for an
early stage, and unrealistically dated on a stalled deal (FCAST-FORM-3). A flagged
deal is excluded from the current period's commit and best-case until a human confirms
a real date, and surfaced under "slipped / needs a date" — the forecast is never built
on a stale or guessed date. The replacement date is proposed from experience — the
workspace's observed median days-per-stage over its won deals, falling back to an
opinionated default when history is thin — times the stages remaining.

**How the fix lands is risk-tiered, ratified, and never silent.** An *active*,
low-stakes, clear-overdue deal — not in this period's commit or best-case, not
late-stage — is auto-corrected (🟢) with full provenance, a rollback handle, and a
"here's what I changed" record: a slipped date never waits on a human nudge. A
forecast-bearing, late-stage, missing, or unrealistic date gets a provisional
replacement now (so the invariant holds instantly) plus a 🟡 confirm, and the deal
stays out of commit until a human sets the real date — a number a leader is staking on
is never moved by a guess. A deal that has gone quiet (the deals chapter's stalled
rule, [[deals-and-pipeline#DEAL-FORM-3]]) is *not* optimistically re-dated: its
category is downgraded one notch, it is flagged at-risk, and it surfaces for an
alive-or-lost decision — the explicit guard against zombie deals wearing an
ever-fresh future date. The policy is this chapter's formula; its overnight execution,
staging, and morning "while you slept" record belong to the
[overnight-agent](overnight-agent.md), and the confirm mechanics to
[approvals-and-concurrency](approvals-and-concurrency.md).

## What's configurable

- **Category cutoffs** — the default commit and best-case probability floors
  (FCAST-PARAM-1, FCAST-PARAM-2); the rep's explicit category always beats the
  default. Source constants.
- **Health weights and floors** — the four factor weights (FCAST-PARAM-3), the
  engagement saturation count (FCAST-PARAM-4), the velocity fallback for thin history
  (FCAST-PARAM-5), and the at-risk threshold (FCAST-PARAM-6). Source constants.
- **Close-date hygiene tunables** — the unrealistic-soon window (FCAST-PARAM-7), the
  days-per-stage velocity fallback (FCAST-PARAM-8), the won-deal history floor before
  observed velocity is trusted (FCAST-PARAM-9), and the master enable for the 🟢
  auto-apply lane (FCAST-PARAM-10) — switching it off routes every correction 🟡. All
  source constants; the overnight-agent chapter consumes these pins.
- **Stage win probability** — the one runtime-tunable input to the category default,
  owned by [[deals-and-pipeline]] (its DEAL-PARAM-3); this chapter reads it live,
  never caches it.
- **The quota target** — the denominator of coverage is the quota object owned by the
  records-depth chapter; absent or attained quota degrades coverage to a defined,
  honest state, never a divide-by-zero.

## Guarantees (enforced)

- **Totals reconcile exactly.** Weighted and unweighted forecast totals equal the sum
  of their constituent deals, category roll-ups equal the ground-truth grouping with
  no double-count across stakeholders or categories, and every figure decomposes to
  rows a leader can open (AC-F1, AC-F2, [[acceptance-standards#AC-X1]]).
- **The human number is never silently overwritten.** The rep-set category and any
  advisory read are independently present fields; the advisory is visually distinct;
  the only automatic category write is the quiet-deal downgrade, which is flagged,
  audited, and surfaced for review (AC-F6, AC-F7).
- **Won is one hundred, lost is zero, and closed deals never distort open pipeline.**
  Won value stays in commit at full value, lost value is excluded everywhere, and the
  open-pipeline totals sum open deals only (FCAST-FORM-1; the terminal probabilities
  are held by the deals chapter's database check).
- **No open deal survives with a past close date.** Rejected at write, swept nightly;
  after every run the invariant holds regardless of which risk tier applied
  (INV-CLOSE-PAST, AC-F9).
- **No forecast on a guessed date.** An overdue, missing, provisional, or
  quiet-downgraded deal is excluded from the current period's commit and best-case
  until a human confirms a real date (FCAST-FORM-3, AC-F9).
- **Deal health never writes.** The score is a pure, fixed-clock-reproducible function
  of deal columns; inference produces zero deal-field mutations, and every surfaced
  factor carries resolvable evidence or is absent ([[acceptance-standards#GATE-AI-1]],
  [[acceptance-standards#GATE-AI-2]]; [[ai-evals]] AIEVAL-18 at one hundred percent,
  AIEVAL-20 at zero).
- **Value identity with the brief.** The per-deal confidence and revenue impact the
  forecast consumes are byte-identical to the brief's ([[morning-brief#BRIEF-FORM-2]]).
- **Accuracy is reproducible.** The persisted predicted-versus-actual row per closed
  period equals what recomputing from stage history yields, and the predicted side is
  the number that was shown (AC-F4).

## Acceptance

Done means: Riya opens the forecast and sees commit, best-case, and weighted pipeline
as deliberately distinct numbers that each decompose, on click, to the deals and
probabilities behind them and reconcile exactly; coverage against quota and a
predicted-versus-actual accuracy trend render from history; every open deal with a
past, missing, or unrealistic close date is listed as slipped-and-excluded with its
proposed correction waiting on a confirm, and none of those dates was moved silently;
each deal's health read shows its weighted factors with clickable source evidence,
sitting next to — never replacing — the rep's own call. Honest states are part of the
contract: no quota renders coverage as honestly unavailable, an empty pipeline renders
the standard empty state, and an ungrounded health factor is omitted rather than
guessed. The testable form of every claim is pinned in the Acceptance appendix; the
cross-cutting floor (standard screen states, the reporting performance budgets
[[acceptance-standards#PERF-R3]], release gates) is inherited from the
acceptance-standards chapter and not restated.

## Out of scope

- **The weighted + scenario predictive forecast (S-E09.3)** — Fast-follow by design;
  its single home is the scope chapter's deferred list ([[scope#S-E09.3]]). The V1
  power case — what-if via a BYO agent re-parameterizing the governed report tool —
  ships without a scenario UI; trained per-workspace forecast models,
  territory/multi-currency roll-ups, and pipeline-generation time series are OUT V1.
- **The report engine, prebuilt dashboards, and the explain-this-number primitive** —
  the [reporting](reporting.md) chapter; the forecast is a parameterized read over its
  engine, and the leader's forecast-and-reports screen (with its AC series) is that
  chapter's screen.
- **Weighted-pipeline arithmetic, stage probabilities, and the stalled rule** —
  [[deals-and-pipeline]] (its DEAL-FORM-1..3), consumed as inputs.
- **Overnight execution of the close-date policy** — the
  [overnight-agent](overnight-agent.md) chapter stages and applies it; the approval
  inbox mechanics are [approvals-and-concurrency](approvals-and-concurrency.md)'s.
- **Quota administration** — the quota object and its screens belong to the
  records-depth chapter; coverage only reads it.
- **Lead scoring and routing** — the lead-scoring chapter; deal health shares the
  transparent weighted-model discipline but is a different formula on a different
  object.
- **Coverage-risk views (S-E09.6)** — the account-mapping screen and its
  single-threaded / no-touch / won-but-silent views are the reporting chapter's, per
  the screen→story index.

## Where it lives

Forecast semantics live with the reporting module in the backend — the roll-up,
coverage, accuracy, health, and hygiene rules ride the typed query-plan read path —
with the hygiene sweep executed by the agents module's overnight pass and the forecast
surface rendered inside the frontend's reports feature. Read next:
[deals-and-pipeline](deals-and-pipeline.md) (the numbers underneath),
[reporting](reporting.md) (the engine and the screen), [morning-brief](morning-brief.md)
(the value-identity partner), and [overnight-agent](overnight-agent.md) (the hands
that apply the hygiene policy).

## Appendix

### Parameters
Source: contract/formulas-and-rules.md#0-parameter-registry-all-tunables-one-place @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| FCAST-PARAM-1 | `FORECAST_COMMIT_MIN_PROB` | `90` | Default-category floor: an unset open deal whose stage win-probability is ≥ this defaults to `commit`. The rep's explicit category always wins. Source constant. |
| FCAST-PARAM-2 | `FORECAST_BESTCASE_MIN_PROB` | `50` | Default-category floor for `best_case`; below it the default is `pipeline`. Also the `late_stage` threshold in the close-date risk policy (FCAST-FORM-3). Source constant. |
| FCAST-PARAM-3 | `W_HEALTH_ACT / VEL / ENG / COM` | `0.30 / 0.25 / 0.20 / 0.25` | Deal-health factor weights (sum = 1.0) for activity recency, stage velocity, engagement breadth, commitments (FCAST-FORM-2). Source constants. |
| FCAST-PARAM-4 | `ENGAGE_NORM` | `3` | Distinct engaged stakeholders at which the deal-health engagement factor saturates at 1.0. Source constant. |
| FCAST-PARAM-5 | `STAGE_VELOCITY_FALLBACK_DAYS` | `14` | Expected days-in-stage for the health velocity factor when the pipeline has fewer than 10 won deals of history. Source constant. |
| FCAST-PARAM-6 | `DEAL_HEALTH_AT_RISK` | `0.35` | Health below this flags the deal at-risk (the deterministic basis for the risk surfacing and the §11 downgrade's at-risk mark). Source constant. |
| FCAST-PARAM-7 | `CLOSE_DATE_UNREALISTIC_SOON_DAYS` | `7` | An open deal dated to close within this window while stage win-probability is under 40 flags `unrealistic_soon`. Source constant. |
| FCAST-PARAM-8 | `CLOSE_DATE_STAGE_DAYS` | `14` | Default days-per-remaining-stage for the proposed replacement date when won-deal history is thin. Source constant. |
| FCAST-PARAM-9 | `CLOSE_DATE_MIN_HISTORY` | `20` | Won deals required in a pipeline before the workspace-observed median stage velocity is trusted over the FCAST-PARAM-8 fallback. Source constant. |
| FCAST-PARAM-10 | `CLOSE_DATE_AUTOAPPLY` | `true` | Master enable for the 🟢 auto-apply tier of close-date correction; OFF routes every correction 🟡 provisional-confirm. Source constant. |

Note FCAST-PARAM-N-1 (ownership handover): FCAST-PARAM-7..10 are the close-date
tunables the [overnight-agent](overnight-agent.md) chapter pinned *provisionally* as
OVN-PARAM-1..4 because no owning chapter existed when it landed. This chapter is the
single home of the §11 formula and its tunables; the overnight-agent chapter's
OVN-PARAM-1..4 rows convert to citations of FCAST-PARAM-7..10 (maintainer edit to
that file — its values are identical, so nothing renumbers).

Note FCAST-PARAM-N-2: the corpus registry (§0) also lists the retired
`CLOSE_DATE_AUTOROLL` binary switch as superseded by the A6 hybrid policy; it is not
pinned here. The stalled threshold and wait window are
[[deals-and-pipeline#DEAL-PARAM-1]] / [[deals-and-pipeline#DEAL-PARAM-2]], and
`stage.win_probability` is [[deals-and-pipeline#DEAL-PARAM-3]] — cited, not owned
here.

### Formulas — forecast categories & roll-up
Source: contract/formulas-and-rules.md#7-forecast-categories--commit--best-case--pipeline--roll-up @ 5a0b29c

**FCAST-FORM-1.** Category ladder + per-owner/team roll-up. The category is rep-set
(never silently overwritten, AC-F6/AC-F7); the deterministic default below fills only
the unset. Inputs: `deal.forecast_category` (rep-set enum
`commit/best_case/pipeline/omitted` — column DDL owned by
[[deals-and-pipeline#DEAL-DDL-3]]), `stage.win_probability`
([[deals-and-pipeline#DEAL-FORM-1]], read live), `deal.status`, `base_value`
([[deals-and-pipeline#DEAL-FORM-2]]).

```
category ∈ { commit, best_case, pipeline, omitted }

# Default (when rep has not set one), by stage probability:
default_category(deal):
    if deal.status = 'won'                          -> commit (closed-won counts in commit)
    elif win_probability >= FORECAST_COMMIT_MIN_PROB    (90) -> commit
    elif win_probability >= FORECAST_BESTCASE_MIN_PROB  (50) -> best_case
    else                                            -> pipeline
# 'omitted' is only ever set by a human (a deal the rep explicitly excludes).
```

```
forecast(owner, period):                            # period bucketed in workspace zone (data-semantics §2 r4)
   scope = open deals where owner_id=owner AND expected_close_date in period AND status in (open, won)
   commit_total    = Σ base_value(d)  where category(d) = commit
   best_case_total = commit_total + Σ base_value(d) where category(d) = best_case   # best-case INCLUDES commit
   pipeline_total  = Σ base_value(d)  where category(d) in (commit, best_case, pipeline)
```

- Semantics (standard forecast ladder): **commit** — deals the rep will stake the
  number on (high confidence + won), the conservative forecast; **best_case** — upside
  that could land this period; **pipeline** — everything else open in the period;
  **omitted** — explicitly excluded from the forecast (human-only; see note
  FCAST-FORM-N-1).
- Each deal belongs to **exactly one** category (no double-count across categories);
  team roll-up sums member commit/best/pipeline; a deal with multiple stakeholders is
  still one deal → counted once (AC-F2).
- **Output:** `{commit_total, best_case_total, pipeline_total, base_currency, period}`
  per owner/team, reconciling to the constituent deals (AC-F1); shown alongside the
  advisory figure, never overwriting the human category (AC-F7).
- **WORKED EXAMPLE (verbatim)** (owner Mor, Q2 2026, base EUR):
  - Deal A €60k, Negotiation 80% → default `best_case`. Deal B €100k, rep set
    `commit`. Deal C €30k, Discovery 40% → `pipeline`.
  - commit = €100k; best_case = €100k + €60k = €160k; pipeline = €100k + €60k + €30k
    = €190k.
- **Edge cases:**
  - **Rep override beats default:** if the rep set a category, use it; default only
    fills the unset.
  - **Closed-won mid-period:** stays in commit (it's real revenue) — the "won counts
    100%" half of the closed-deal accounting this chapter owns (deals' S-E03.4 row).
  - **Closed-lost:** excluded from all forecast categories (status filter) — the
    "lost counts 0%" half. Open-pipeline totals sum open deals only
    ([[deals-and-pipeline#DEAL-FORM-2]]), so closed deals never distort them.
  - **`expected_close_date IS NULL`:** excluded from period roll-ups (not silently
    bucketed into the current period); surfaced as "no close date" (and flagged
    `missing`, FCAST-FORM-3).
- **Tunables:** FCAST-PARAM-1, FCAST-PARAM-2.

Note FCAST-FORM-N-1 (source: contract/formulas-and-rules.md §7.1 vs §11 @ 5a0b29c;
review 2026-07-03): §7 states `omitted` is "only ever set by a human", while §11's
ratified downgrade ladder (Commit→Best-case→Pipeline→Omitted) lets the quiet-deal
🔻 path reach Omitted mechanically. Reconciliation pinned here: the human-only clause
governs the *default-category* path (no default and no inference ever sets omitted);
the §11 downgrade is the one sanctioned system write, and it is flagged, audited, and
surfaced for review — never silent. Flagged for maintainer ratification.

### Formulas — deal-health score (transparent weighted-factor model)
Source: contract/formulas-and-rules.md#105-deal-health-score--transparent-weighted-factor-model-e09-features03-32 @ 5a0b29c

**FCAST-FORM-2.** One 0..1 deal-health score per open deal — the deterministic
weighted sum of four normalized factors, so a golden test can assert
`health == Σ wᵢ·factorᵢ`. A *health* lens (higher = healthier), distinct from the
brief's priority composite ([[morning-brief#BRIEF-FORM-1]]). The AI layer may explain
the score but does not compute it. Inputs: `deal.last_activity_at`, days-in-stage vs
the pipeline's observed velocity, distinct engaged stakeholders (two-way engagement,
reciprocity > 0 per the people chapter's strength model), open overdue
tasks/follow-ups, the stalled flag ([[deals-and-pipeline#DEAL-FORM-3]]).

```
# all factors normalized to 0..1 (1.0 = healthiest)
activity_recency   = recency_score(deal.last_activity_at, now_utc)      # fresh contact = healthy
stage_velocity     = velocity_score(deal, pipeline)                     # moving at/above expected pace = healthy
engagement         = min(1.0, distinct_engaged_stakeholders / ENGAGE_NORM)   # breadth of two-way engagement
commitments        = commitment_score(deal)                            # kept vs stalled/overdue commitments

health = W_HEALTH_ACT*activity_recency
       + W_HEALTH_VEL*stage_velocity
       + W_HEALTH_ENG*engagement
       + W_HEALTH_COM*commitments
   # default weights: W_HEALTH_ACT=0.30, W_HEALTH_VEL=0.25, W_HEALTH_ENG=0.20, W_HEALTH_COM=0.25  (sum=1.0)

recency_score(last_activity_at, now):
   if last_activity_at is null: 0.0
   d = days_since(last_activity_at)
   if d <= 3:    1.0
   elif d <= 7:  0.8
   elif d <= 14: 0.6
   elif d <= 30: 0.4
   elif d <= 60: 0.2
   else:         0.0                       # ≥ STALLED_THRESHOLD_DAYS (§8) → floor

velocity_score(deal, pipeline):
   expected = workspace median days-in-current-stage for won deals in this pipeline (§11 stage velocity);
              fall back to STAGE_VELOCITY_FALLBACK_DAYS when <10 won deals of history
   age = days_in_current_stage(deal)
   r   = age / expected
   if r <= 1.0:  1.0                        # at or ahead of pace
   elif r <= 1.5: 0.6
   elif r <= 2.0: 0.3
   else:          0.0                       # ≥2× expected → stuck

commitment_score(deal):
   open_overdue = count(open tasks/follow-ups on deal with due_at < now)
   if deal.is_stalled (§8):          return 0.0
   if open_overdue == 0:             return 1.0
   if open_overdue == 1:             return 0.5
   return 0.2                                # 2+ overdue commitments
```

- **Output:** `{deal_id, health (0..1), factors: {activity_recency, stage_velocity,
  engagement, commitments}, weights}` — the per-factor breakdown is exposed (no black
  box). A deal is **at-risk** when `health < DEAL_HEALTH_AT_RISK (0.35)` — the
  deterministic basis for the at-risk flag, the risk surfacing, and FCAST-FORM-3's
  downgrade mark.
- **WORKED EXAMPLE (verbatim):** last activity 5 days ago (recency 0.8), in-stage
  1.2× expected (velocity 0.6), 2 engaged stakeholders / ENGAGE_NORM=3 (engagement
  0.67), no overdue commitments & not stalled (commitments 1.0):
  `health = 0.30*0.8 + 0.25*0.6 + 0.20*0.67 + 0.25*1.0 = 0.24+0.15+0.134+0.25 = 0.774`
  → healthy.
- **Governance:** the score is read-only inference — zero deal-field mutations
  ([[acceptance-standards#GATE-AI-2]]; [[ai-evals]] AIEVAL-20 = 0); the explain layer
  cites source records for every factor ([[ai-evals]] AIEVAL-18 = 100%, AIUC-06); it
  never rewrites stage or forecast category (AC-F7, B-E09.17's culture guard).
- **Tunables:** FCAST-PARAM-3..6.

### Formulas — close-date hygiene & realism (INV-CLOSE-PAST + risk-tiered fix)
Source: contract/formulas-and-rules.md#11-close-date-hygiene--realism-ratified--decisions-a6 @ 5a0b29c

**FCAST-FORM-3.** **Hard invariant (`INV-CLOSE-PAST`): an open deal's
`expected_close_date` is never in the past.** A deal still open past its own claimed
close date is an *invalid state*, not soft hygiene: the write layer rejects saving a
past close date on an open deal at source, and the state never persists past the
nightly run (corpus §12 invariant 5). The risk tier governs how the replacement date
is chosen and whether it is final or provisional — never whether a past date may stay.
Inputs: `deal.status`, `deal.expected_close_date`, `stage.position` /
`stage.win_probability`, `deal.last_activity_at`, `now_utc`, workspace timezone, the
stalled flag ([[deals-and-pipeline#DEAL-FORM-3]]).

Deterministic flags (only on `status='open'`):

```
today   = (now_utc at workspace tz)::date
overdue        = expected_close_date < today
missing        = expected_close_date IS NULL
unrealistic_soon  = expected_close_date <= today + CLOSE_DATE_UNREALISTIC_SOON_DAYS
                    AND stage.win_probability < 40          # early stage can't close that fast
unrealistic_stale = is_stalled(deal)                         # §8
                    AND expected_close_date <= today + STALLED_THRESHOLD_DAYS
flagged = overdue OR missing OR unrealistic_soon OR unrealistic_stale
```

Computed correction (activity- and experience-informed, not a flat stage count):

```
remaining_open_stages = count of open stages at/after the deal's current stage

# stage velocity from EXPERIENCE: workspace-observed median days-per-stage for won deals
# in this pipeline; falls back to the opinionated default when there isn't enough history.
stage_velocity = workspace_median_days_per_stage(pipeline)         # if >= CLOSE_DATE_MIN_HISTORY won deals
                 else CLOSE_DATE_STAGE_DAYS                          # default 14

proposed_close_date = today + max(1, remaining_open_stages) * stage_velocity
```

Activity gate and application policy (DECISIONS A6 hybrid):

```
quiet  = is_stalled(deal)                    # §8: no real activity for STALLED_THRESHOLD_DAYS (60), wait_until honored
active = NOT quiet

in_forecast_commit = deal counts in THIS period's Commit or Best-case (FCAST-FORM-1)  # the number a leader is staking on
late_stage         = stage.win_probability >= FORECAST_BESTCASE_MIN_PROB              # 50
clear_overdue      = overdue AND NOT missing AND NOT unrealistic_soon                  # a plain past date, not a judgment call

if quiet:                          # — gone dark → do NOT optimistically push the date forward
    action = DOWNGRADE_AND_REVIEW
elif CLOSE_DATE_AUTOAPPLY AND clear_overdue AND active
     AND NOT in_forecast_commit AND NOT late_stage:
    action = AUTO_APPLY            # 🟢 final
else:                              # forecast-bearing / late-stage / missing / unrealistic / switch OFF
    action = PROVISIONAL_CONFIRM   # 🟡 provisional date set now, real date confirmed by human
```

- **🟢 `AUTO_APPLY`** (active, low-stakes, clear-overdue): the agent writes
  `expected_close_date = proposed_close_date` overnight with full provenance + audit,
  lands a rollback-able "here's what I changed and why" record in the Morning Brief,
  reason = the overdue date + the activity/velocity basis for the new one. Reversible
  in one action; the rep is informed, not asked.
- **🟡 `PROVISIONAL_CONFIRM`** (forecast-bearing / late-stage / missing /
  unrealistic): the past date is replaced **now** with `proposed_close_date` flagged
  `provisional=true` so `INV-CLOSE-PAST` always holds, but the deal stays **excluded
  from Commit/Best-case** and surfaces 🟡 "confirm the real close date" until a human
  sets it. The forecast number is never moved by a guess; the invalid state is still
  gone instantly.
- **🔻 `DOWNGRADE_AND_REVIEW`** (quiet): the deal is **not** re-dated optimistically.
  Its forecast category is downgraded one notch (Commit→Best-case→Pipeline→Omitted,
  never below Omitted — see note FCAST-FORM-N-1) and deal health flagged at-risk; the
  date is set provisional only to satisfy the invariant; it surfaces 🟡 "this deal has
  gone quiet — is it still alive? set a real date or mark lost." The explicit guard
  against zombie deals wearing a perpetually-fresh future date.
- **Forecast impact (ties to FCAST-FORM-1):** an `overdue`, `missing`, `provisional`,
  or quiet-downgraded open deal is excluded from this period's Commit/Best-case and
  surfaced under "slipped / needs a date" (or "gone quiet / needs review") until a
  human confirms a real date.
- **Output:** per-deal `{ flags: [...], proposed_close_date | null,
  action: AUTO_APPLY|PROVISIONAL_CONFIRM|DOWNGRADE_AND_REVIEW, provisional: bool,
  downgrade: bool }`; consumed by the roll-up (FCAST-FORM-1), the Morning Brief, and
  the overnight agent (which executes the policy — [[overnight-agent]]).
- **WORKED EXAMPLES (verbatim):**
  - *🟢 auto-apply (the Slack-nudge case):* an early-stage (20%) deal with
    `expected_close_date = today − 12d`, **active** (replied 3 days ago), **not** in
    Commit/Best-case → `clear_overdue ∧ active ∧ ¬in_forecast_commit ∧ ¬late_stage` →
    the agent rolls the date to `proposed_close_date` overnight and tells the rep
    ("close date was 12 days past; moved to <date> based on 2 stages remaining and
    your ~18-day stage velocity — undo?"). No human chases it.
  - *🟡 provisional-confirm:* a Negotiation-stage (60%) deal with the same overdue
    date is `late_stage` + `in_forecast_commit` → date set provisional now (invariant
    holds), excluded from Commit, surfaced 🟡 "confirm the real close date." Never
    silently moves a forecast-bearing date.
  - *🔻 downgrade-and-review:* an overdue deal with **no activity for 75 days** →
    `quiet` → not re-dated forward; forecast category dropped one notch,
    `deal_health=at-risk`, surfaced 🟡 "gone quiet — still alive? set a date or mark
    lost." The dead deal stops zombie-walking.
- **Edge cases:** won/lost deals are never flagged (closed); a deal legitimately
  closing today is fine (`overdue` is strict `<`); a non-null `deal.wait_until`
  suppresses the `quiet` branch **and** `unrealistic_stale`, but **not** `overdue` —
  a paused deal still must not claim a *past* close date, so it takes the 🟡
  provisional path.
- **Tunables:** FCAST-PARAM-7..10.

Note FCAST-FORM-N-2 (source: product/epics/E09-reporting-and-forecast.md S-E09.5 @
5a0b29c vs contract/formulas-and-rules.md §11 @ 5a0b29c; review 2026-07-03): the epic's
S-E09.5 bullet still carries the pre-ratification parenthetical "auto-roll is off by
default", written against the retired binary `CLOSE_DATE_AUTOROLL` switch. The
ratified A6 hybrid (§11, and the E09 build atoms) supersedes it:
`CLOSE_DATE_AUTOAPPLY=true` by default, with the 🟢 lane bounded to active,
non-forecast-bearing, non-late-stage, clear-overdue deals. This chapter pins the
ratified default (FCAST-PARAM-10); the epic's stale phrasing is flagged for
maintainer cleanup.

### Formulas — rolling pipeline-coverage ratio (derived)
Source: features/03-reporting-and-scoring.md#2-forecasting @ 5a0b29c; product/build-backlog/E09.md (B-E09.11) @ 5a0b29c

**FCAST-FORM-4 (DERIVED — the corpus defines this metric in words and acceptance
criteria but pins no formula block; the formalization below is this chapter's, marked
for ratification).** Rolling coverage = open pipeline ÷ remaining quota, exact from
stage history.

```
coverage(scope, period, as_of):
   open_pipeline   = Σ base_value(d) over live open deals in scope with expected_close_date in period,
                     reconstructed as of `as_of` from deal_stage_history           # same per-deal values as FCAST-FORM-1
   closed_won      = Σ base_value(d) over deals in scope won within period up to `as_of`
   remaining_quota = quota.target_minor − closed_won                               # quota: records-depth chapter's object
   if no quota row:            coverage = undefined  → surfaced "no target set" (honest state, never a guess)
   if remaining_quota <= 0:    coverage = defined "target met" state               # never NaN/Inf (B-E09.11 edge)
   else:                        coverage = open_pipeline / remaining_quota
```

- **Output:** `{coverage_ratio | state, open_pipeline_minor, remaining_quota_minor,
  base_currency, as_of}`; recomputing at the same `as_of` yields the identical number
  (reproducible from history, AC-F3 / [[acceptance-standards#AC-X1]] no-drift).
- **Worked example (DERIVED):** quota €200k for the quarter, €50k already closed-won
  → remaining €150k; open pipeline dated in-quarter €450k → coverage = 3.0×.
- **Tie-break/edge:** the numerator reconciles to the same per-deal values the
  FCAST-FORM-1 roll-up uses — no independent re-summation that could disagree
  (B-E09.11).
- **Tunables:** none — arithmetic over the quota object and stage history.

### Schema
Source: contract/data-model.md#saved-views-quota-field-mask @ 5a0b29c

Ownership verified against the data-model chapter's ownership index
([[data-model]] Schema — ownership index): `forecast_snapshot` is assigned to this
chapter. `quota` (the coverage denominator) is the records-depth chapter's;
`deal`, `deal_stage_history`, and the `forecast_category` column are
[[deals-and-pipeline]]'s (its DEAL-DDL series) — cited, not restated.

**FCAST-DDL-1 — the `forecast_snapshot` table (verbatim).**

```sql
CREATE TABLE forecast_snapshot (                          -- predicted-vs-actual captured once per closed period per scope (E09 AC-F4 forecast accuracy)
  -- + base columns
  owner_id      uuid NULL REFERENCES app_user(id),
  team_id       uuid NULL REFERENCES team(id),
  pipeline_id   uuid NULL REFERENCES pipeline(id),         -- null = all pipelines for the scope
  period_start  date NOT NULL,
  period_end    date NOT NULL,
  predicted_minor bigint NOT NULL,                         -- the forecast total as it stood at period close
  actual_minor  bigint NOT NULL,                           -- realized closed-won at period close
  currency      char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  closed_at     timestamptz NOT NULL DEFAULT now(),        -- when the period-close capture ran
  CHECK ((owner_id IS NOT NULL) <> (team_id IS NOT NULL)),  -- exactly one of owner/team
  UNIQUE (workspace_id, owner_id, team_id, pipeline_id, period_start, period_end)
);
```

Note FCAST-DDL-N-1: the persisted `predicted_minor` must be the roll-up the leader was
shown at period close, and recomputing from `deal_stage_history` must reproduce it
(AC-F4, B-E09.12) — the snapshot is a captured fact, not a re-derivation.

### Wire
Source: contract/crm.yaml (Reports tag, `runReport`) @ 5a0b29c; contract/data-model.md#135-sort--filter-vocabulary @ 5a0b29c

By ratified design there is **no dedicated forecast path**: the forecast is a
parameterized report over the reporting chapter's engine (B-E09.10 — "forecast rides
the report engine — no dedicated `/forecast` path"). Operations cited, never restated:

| ID | Operation | Notes |
|---|---|---|
| FCAST-WIRE-1 | `runReport` | The reporting chapter's operation (🟢 `run_report` MCP verb, RBAC-bound, audit-logged, per-cell derivation handles). The forecast surface and BYO-agent what-ifs ride it; the prebuilt report keys serving this chapter are `forecast-weighted`, `rolling-coverage`, `stage-conversion`, `win-loss` (data-model §13.5), and the deals vocabulary filters on `forecast_category` and sorts on `expected_close_date`. |
| FCAST-WIRE-2 | `updateDeal` / `advanceDeal` | Category set/override and any confirmed close-date write are ordinary governed deal writes ([[deals-and-pipeline#DEAL-WIRE-3]], [[deals-and-pipeline#DEAL-WIRE-4]]). |
| FCAST-WIRE-3 | `listApprovals` / `approveApproval` / `rejectApproval` | The 🟡 close-date confirms ride the approvals contract via the overnight batch ([[overnight-agent#OVN-WIRE-1]], [[overnight-agent#OVN-WIRE-2]]). |

Honest gaps:

| ID | Gap | Notes |
|---|---|---|
| FCAST-GAP-1 | No forecast roll-up response shape | FCAST-FORM-1's output object (`commit_total`/`best_case_total`/`pipeline_total` + constituent breakdown) has no named schema in the contract; it must land as a prebuilt-report result contract (or a contract-first extension) so AC-F1/AC-F2 are testable against a stable shape. The E09 build atoms (B-E09.10) own landing it. |
| FCAST-GAP-2 | No deal-health read on the wire | §10.5's output (`health`, `factors`, `weights` + evidence links) has no endpoint and no Deal field — deliberately *not* a writable deal column (GATE-AI-2 / AIEVAL-20). B-E09.15/.16 must land the read contract-first. |
| FCAST-GAP-3 | No forecast-accuracy read | `forecast_snapshot` has no list/read operation and no prebuilt report key; AC-F4's "queryable" clause is unimplementable until B-E09.12 lands one. |
| FCAST-GAP-4 | No hygiene-list read | FCAST-FORM-3's per-deal output (flags, proposed date, action, provisional) powering the "slipped / needs a date" section has no wire shape; the 🟡 items ride approvals, but the read-side list needs a contract home (B-E09.18..20). |

### Events
Source: contract/events.md#53-deal @ 5a0b29c; contract/events.md#53a-offer--angebot-a48adr-0037 @ 5a0b29c; contract/events.md#511-engagement--signals-e08-warm-room-e15-reply-tracking @ 5a0b29c

Event definitions live in the central catalog ([[event-bus#events--catalog]]);
cited, never redefined.

| ID | Event(s) | Role here |
|---|---|---|
| FCAST-EVT-1 | `forecast.period_closed` | Emitted once per closed period per scope by the scheduled period-close job; carries `predicted_minor`/`actual_minor` and captures one `forecast_snapshot` row (FCAST-DDL-1). Consumed by the accuracy read model and the audit stream. |
| FCAST-EVT-2 | `deal.stage_changed` | Consumed: carries the amount + probability snapshot so as-of-date forecast reads and the nightly stalled/close-date sweep react without a read-back ([[deals-and-pipeline]] emits). |
| FCAST-EVT-3 | `offer.accepted` (+ paired `deal.updated`) | Consumed: load-bearing for forecast value freshness — acceptance updates the deal's value source and the paired event (shared correlation id) lets the forecast read model follow (catalog-owned semantics). |
| FCAST-EVT-4 | `deal.updated` + `audit.appended` | The 🟢 auto-applied close-date correction commits as a normal audited domain mutation with agent provenance — pinned at [[overnight-agent#OVN-EVT-4]]; cited here as the hygiene policy's commit path. |

Note FCAST-EVT-N-1 (review 2026-07-03): the event-bus catalog lists the
`forecast.period_closed` emitter as the people module's scheduled job — the
pre-roster-growth default. The period-close computation is this chapter's semantics;
when the module roster grows (as it did for deals), the emitter column should follow
the owning module. Flagged for the event-bus chapter's maintainer; not restated as a
pin here.

### Acceptance — owned stories
Source: product/epics/E09-reporting-and-forecast.md @ 5a0b29c

Story primacy verified against product/20-traceability.md @ 5a0b29c: **S-E09.4**
(V1-WOW) and **S-E09.5** (V1-Must) are owned here and by no other chapter. S-E09.1/.2
are the reporting chapter's; S-E09.6 is the reporting chapter's (per the screen→story
index, confirmed by the signals-and-warm-room chapter's same finding); S-E09.3
(weighted + scenario predictive forecast) is Fast-follow with its single home at
[[scope#S-E09.3]] — the V1 forecast this chapter specifies is the deterministic slice
features/03 §2.3 marks IN. S-E03.4's weighted-pipeline arithmetic is
[[deals-and-pipeline]]'s; its 100/0 closed-deal accounting clause is assigned to this
chapter by that story's own row and pinned in FCAST-FORM-1.

The screen finding, recorded per [[acceptance-standards]] ACID-4 (every screen's AC
series is owned by exactly one chapter): the leader's forecast surface is the
forecast-and-reports screen, whose AC series (implementing S-E03.4 and
S-E09.1/.2/.4/.5 together) belongs to the **reporting** chapter as screen owner. This
chapter therefore pins **no screen AC series**; the hygiene and reconciliation screen
criteria are cited below by ID as the verification lane for this chapter's stories.

Condensed story acceptance (full Given/When/Then in the epic):

| ID | Given/When/Then | Verification |
|---|---|---|
| S-E09.4 | Given a deal with captured activity, when Riya looks at deal health, then the score is driven by real signals (activity recency, stage velocity, engagement, stalled commitments), not a manually-set field — a quiet deal on a green stage reads at-risk; clicking it shows the weighted factors and the source records behind each (the shared explain primitive); the inference is advisory, shown next to the rep's stage/category and never silently rewriting either; it is framed as a transparent rep-copilot coaching signal, not covert surveillance. | Golden fixed-clock test on FCAST-FORM-2; AC-S7/AC-S8; zero-mutation test ([[acceptance-standards#GATE-AI-2]], [[ai-evals]] AIEVAL-20); [[ai-evals]] AIUC-06 + AIEVAL-18/19 |
| S-E09.5 | Given an open deal whose close date is past, missing, or unrealistic for its stage, when the forecast is viewed, then the deal is flagged ("slipped / needs a date" with the reason) and not counted in this period's Commit/Best-case until corrected; a proposed corrected date lands per the FCAST-FORM-3 risk tiers — 🟢 auto-applied only on active, low-stakes, clear-overdue deals (reversible, logged), 🟡 provisional+confirm on forecast-bearing/late-stage/missing/unrealistic, 🔻 downgrade-and-review on quiet deals — and no date is ever changed silently. | Fixed-clock tests on FCAST-FORM-3 (worked examples as fixtures); AC-F9; screen criteria AC-reports-7/8 (reporting chapter's series, cited) |

### Acceptance — feature criteria (verbatim)
Source: features/03-reporting-and-scoring.md#24-acceptance-criteria @ 5a0b29c; features/03-reporting-and-scoring.md#34-acceptance-criteria (AC-S8) @ 5a0b29c

Cross-cutting floor inherited, not restated: the forecast dashboard budget is
[[acceptance-standards#PERF-R3]] (AC-F5 restates it with the owner's tag), golden-number
zero-tolerance is [[acceptance-standards#AC-X1]], and the AI gates are
[[acceptance-standards#GATE-AI-1]]/[[acceptance-standards#GATE-AI-2]].

| ID | Given/When/Then (corpus text, verbatim) | Verification |
|---|---|---|
| AC-F1 | Weighted + unweighted forecast totals reconcile exactly to the sum of constituent deals (golden-number test). | Golden test on FCAST-FORM-1 over [[deals-and-pipeline#DEAL-FORM-2]] values (worked examples as fixtures) |
| AC-F2 | Forecast category roll-up by owner/team equals the ground-truth grouping; no double-count across stakeholders. | Golden grouping test; many-to-many stakeholder fixture |
| AC-F3 | Rolling pipeline-coverage ratio computed from `deal_stage_history` matches an as-of-date ground truth on the seed dataset. | Fixed-clock golden test on FCAST-FORM-4; no-drift recompute test |
| AC-F4 (accuracy tracking) | Predicted-vs-actual is persisted per closed period and queryable; the accuracy report is reproducible from history. | Persistence test (`forecast.period_closed` → one FCAST-DDL-1 row); reconciliation vs history; wire gap FCAST-GAP-3 must close first |
| AC-F5 (perf) | Forecast dashboard load p95 < 800 ms server on the seed dataset. | CI benchmark (restates [[acceptance-standards#PERF-R3]]) |
| AC-F6 (explainability) | Every predictive figure exposes its driver decomposition; the advisory probability is visually distinct from the rep-set category (design review + test that both fields are independently present in the API response). | API two-field test; design review; decomposition via the reporting chapter's explain primitive |
| AC-F7 (user-observable — forecast a leader can trust) | A leader sees weighted and unweighted forecast totals plus a rolling coverage ratio, and can see their own rep-set commit category sitting *next to* the AI's advisory probability — the human number is never silently overwritten, and the leader can click the AI figure to see what drove it (e.g. "47 days no activity") (S-E09.3, S-E05.5). | e2e on the reporting chapter's screen; no-overwrite mutation test |
| AC-F8 (user-observable — confidence and revenue impact roll up) | Each deal's advisory confidence carries through to a revenue-impact view the leader reads at the forecast level, so they can see which deals move the number and by how much — not just a single opaque total (S-E05.5). | Golden identity test: roll-up consumes byte-identical per-deal values ([[morning-brief#BRIEF-FORM-2]], BRIEF-AC-5) |
| AC-F9 (close-date hygiene — `formulas-and-rules.md §11`) | an open deal with `expected_close_date < today` (overdue), null, or unrealistic for its stage is flagged and **omitted from the current-period Commit/Best-case** until corrected. Per **`INV-CLOSE-PAST`** an open deal's close date is never left in the past; the correction follows the **risk tier (DECISIONS A6 hybrid):** an *active* clear-overdue deal not in Commit/Best-case and not late-stage is 🟢 auto-corrected to an activity/velocity-informed date (reversible, audited, "what I changed" record); in-forecast / late-stage / `null` / unrealistic cases get a **provisional** date now + 🟡 confirm of the real one; a `is_stalled` (quiet) deal is 🔻 downgraded a forecast notch + flagged at-risk rather than re-dated forward. Fixed-clock tests assert: the flag fires; the deal drops out of this period's commit total until a human-confirmed valid date; **no `open` deal survives a run with `expected_close_date < today`**; the 🟢 path writes-with-rollback only on active non-forecast clear-overdue deals; the quiet path downgrades + does not push the date forward (S-E09.5). | Fixed-clock test suite on FCAST-FORM-3; write-layer rejection test; nightly-sweep invariant test |
| AC-S8 (user-observable — conversation-inferred deal health) | A leader looking at a deal or account sees a health/score signal built from real captured engagement (replies, meeting attendance, recency) and can click it to read the weighted factors and open the source emails/meetings behind it — the score is never a black-box number they have to take on faith (S-E09.4). | e2e over the FCAST-FORM-2 decomposition + evidence links (AC-S7's decomposition mechanics); [[ai-evals]] AIUC-06 |

Note FCAST-AC-N-1: AC-S8 is the one criterion this chapter owns from the corpus's
scoring series — it traces S-E09.4; the remainder of the AC-S series (AC-S1..S7,
scoring and routing) is the lead-scoring chapter's. AC-S1's golden
`score == Σ weighted factors` discipline applies to FCAST-FORM-2 via B-E09.15's
acceptance, cited not duplicated.

Note FCAST-AC-N-2 (eval floors): [[ai-evals]] pins the deal-health lane — AIEVAL-18
(signal grounding = 100%, hard), AIEVAL-19 (direction accuracy ≥ 75%, band),
AIEVAL-20 (deal-field mutations from inference = 0, hard) — and AIUC-06 as the
catalog entry. The deterministic score itself is a golden test, not an eval; the
bands govern only the explain layer.

Note FCAST-AC-OPEN-1 (source: product/30-screen-acceptance.md reports.html "Open
questions" @ 5a0b29c): the prototype renders no deterministic deal-health surface —
the deal-360 screen shows the meetings chapter's inferred close-likelihood signal
(a different, model-produced number) and the forecast screen has no health column;
the corpus itself asks "confirm whether a health column belongs." The build must land
a rendered surface for FCAST-FORM-2's score + decomposition (S-E09.4 is V1-WOW), with
its screen AC landing in the owning screen's series (reporting's screen or the deals
chapter's deal-360). Ticket-gate: B-E09.13/.16/.17 must resolve the placement before
the S-E09.4 tickets close.
