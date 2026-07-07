---
status: planned
module: backend/internal/modules/reporting (query-plan compiler + runner, report-key catalog, coverage views) · web (forecast-and-reports + coverage surfaces)
derives-from:
  - specs/spec/features/03-reporting-and-scoring.md#1-reporting--analytics
  - specs/spec/features/03-reporting-and-scoring.md#cross-cutting-acceptance-the-correctness-invariant
  - specs/spec/contract/data-model.md#135-sort--filter-vocabulary-the-per-resource-allow-list
  - specs/spec/contract/crm.yaml (the reports operation + its request/result schemas)
  - specs/spec/product/epics/E09-reporting-and-forecast.md
  - specs/spec/product/30-screen-acceptance.md#reportshtml--forecast--reports-implements-s-e034-s-e091245
  - specs/spec/product/30-screen-acceptance.md#coveragehtml--account-mapping--coverage-risk-implements-s-e096
  - margince-poc/docs/subsystems/reporting.md @ a11d6c08
---
# Reporting — no fast-but-wrong numbers: every figure is a compiled, inspectable query a reader can drill to its rows

> The reporting engine and the two leader surfaces built on it: a report request —
> an object, filters, a grouping, a few aggregates — compiles to a typed, validated
> query plan over real columns and typed relationship edges, never free-form SQL;
> anything out of vocabulary is refused before the database is touched; and every
> number the product shows can be explained down to the exact source rows that
> produced it. Its promise: a number Riya can show her board without re-checking it
> in a spreadsheet.

## What it's for

Incumbent reporting breaks where the data model breaks: aggregates that don't work
in computed fields, joins across associated records that silently drop or
double-count, custom-object reports that fail, snapshots reconstructed by guesswork.
This subsystem is the structural fix — because the core is a clean relational schema
(P11), every report is an honest grouped aggregate over real columns and real typed
edges, and correctness is a release gate, not an aspiration
([[acceptance-standards#AC-X1]]). It owns the report engine (the vocabulary-bound
plan compiler and runner), the prebuilt report-key catalog the data-model chapter
ceded here, the plain-language ask that compiles to a shown plan, the
explain-this-number drill-through that every aggregate in the product exposes, and
the coverage-risk views that ask the relational questions a flat schema cannot.
Its callers are the forecast-and-reports surface, the coverage surface, dashboard
widgets on other screens, sibling subsystems that render aggregates (morning brief,
deals, scoring), and BYO agents through the governed report-running verb on the MCP
surface. The boundary: this chapter owns how a number is compiled, executed, and
explained — not the stores it reads (deals, activities, relationships live with
their owning chapters) and not the forecast or score semantics whose numbers it
renders and explains.

## Principles it serves

- **P11 — the clean core is the reporting moat.** Counting works in derived fields,
  aggregation crosses associated records, custom code-defined objects report
  identically to core objects, and joins never silently drop or double-count —
  because every report compiles to honest grouped SQL over a normalized schema,
  never a metadata-engine expression evaluator.
- **P12 — governance designed in.** Every plan is workspace-scoped and inspectable;
  every agent-run report is scope-bound and audit-logged; every number decomposes to
  evidence a human can check.
- **P7 — own your data.** Any report exports with the documented query behind it;
  no number is locked in.
- **P4 — speed as credibility.** The reporting budgets are CI gates on a seeded
  benchmark ([[acceptance-standards#PERF-R1]]..PERF-R6,
  [[acceptance-standards#AS-SCALE-1]]); a slow "correct" number loses the trust a
  fast one earns.
- **P6 — baseline AI, no frontier reimplementation.** The plain-language path is a
  thin compile step onto the same deterministic plan surface; agents get the same
  surface, never a private one.
- **P1 — one opinionated way.** A bounded, typed query builder — not an open SQL
  console; power users go through a BYO agent or source-level views (P2).
- **ADR-0004 — the reporting read-path decision.** One typed query-plan
  intermediate representation over Postgres, compiled to parameterized SQL, backed
  by materialized read models and committed indexes; UI, plain-language asks, and
  BYO agents all compile to the same plan; lineage is a property of the plan, so
  explain-this-number is structural, not bolted on. A second engine was rejected
  because two engines are two correctness surfaces.

## How it works

**Vocabulary first.** Each prebuilt report key declares a closed vocabulary — the
only dimensions, measures, and filter fields it accepts, mapped to real columns and
sensible defaults. Ad-hoc reports compile against the per-object sort/filter
allow-lists the data-model chapter pins (DM-VOCAB-1..6, owned by [[data-model]]);
the key catalog itself is pinned here (REPORT-KEY-1..8), per that chapter's
explicit cession. The aggregate function set is closed to counting,
distinct-counting, summing, averaging, and taking a minimum or maximum.

**Compile, then run.** A request is validated against the vocabulary before
anything else happens: every filter key, group-by entry, aggregate field, and
aggregate function must be in vocabulary, and any miss is refused with a named
validation error **before any database call** (DM-VOCAB-ERR-1) — nothing is
silently coerced or dropped, and an unknown report key is honestly not-found. The
compiler then builds a plan from vocabulary columns only, with typed foreign-key
joins and every predicate value bound as a parameter, never interpolated into query
text. An empty request on a prebuilt key uses that key's default grouping and
aggregates, so it compiles to a real grouped query rather than a whole-table scan.

**The plan round-trips.** The executed plan is returned with the result — columns,
rows, a generated-at stamp, and the plan itself — so any number can be re-run,
saved as a normal report, and inspected. This is the single read-path abstraction
ADR-0004 decided: the UI, the plain-language path, and BYO agents all produce and
consume the same plan; none can bypass the schema or invent a join.

**Plain language compiles to a shown plan; the executed plan is deterministic.**
When a user asks in plain language — stalled enterprise deals, win rate by source
last quarter — a model compiles the utterance to a candidate plan, which is
validated like any other request and **shown to the user before it runs**. The
utterance-to-plan step is model-bound and therefore tracked as an eval band, never
claimed as deterministic ([[ai-evals#AIEVAL-15]], use case AIUC-20); the executed
plan, once compiled, is deterministic and pinned against hand-written references
([[ai-evals#AIEVAL-16]]). Anything ambiguous or out of vocabulary returns a
clarifying question, never a silent guess ([[ai-evals#AIEVAL-17]]). The sibling
[[search-and-retrieval]] chapter owns the conversational cited-answer search side
of this band (its AC6.3/AC6.4); this chapter owns the report-plan side — same
discipline, same eval rows, two surfaces. The first plan appears within the compile
budget ([[acceptance-standards#PERF-R5]]).

**History-sourced reports answer from the log, not from guesswork.** Pipeline
as-of-a-date and stage-conversion run over the append-only stage-history log the
deals chapter owns and writes ([[deals-and-pipeline]]): the as-of report takes each
deal's stage from its newest history entry on or before the requested date, with
amounts as snapshotted at the time of each change, and never reads current deal
rows; stage-conversion is a funnel of transition counts from the same history.
Conversion rates are out of scope in V1 — the engine computes counts.

**Explain this number is one mechanic, everywhere.** Every aggregate cell or KPI
carries a derivation handle. Resolving it returns the exact filter, grouping, and
aggregate in plain language plus the underlying source rows — and the drill-through
reuses the **same compiled plan** the aggregate ran, never a re-parsed filter set,
so summing the drill-through rows reproduces the displayed number exactly
([[acceptance-standards#AC-X1]]) within the derivation budget
([[acceptance-standards#PERF-R6]]). A derivation over a derived input recurses to
base rows, never an opaque intermediate. The corpus names this mechanic three ways
— explain this number, show the evidence, how is this computed — and flags them as
one interaction to unify across every surface; this chapter carries that
unification (REPORT-N-2): forecast figures, health scores, and coverage flags all
decompose through this same primitive, with the formulas owned by their chapters
(the weighted pipeline by [[deals-and-pipeline]], per-item revenue impact by
[[morning-brief#BRIEF-FORM-2]], scores by the scoring owner).

**Coverage-risk views are deterministic relational questions.** The coverage
surface asks three questions a flat schema cannot: which open deals hang on a
single engaged contact, which in-pipeline accounts have had no captured touch
inside the threshold window, and which closed-won accounts have gone silent since
close. Each is a deterministic query over typed relationship edges and captured
activity — an engaged-contact count below the threading threshold flags
single-threaded (REPORT-PARAM-1), captured-touch recency drives the no-touch views
(REPORT-PARAM-2), and post-close silence drives won-but-now-silent
(REPORT-PARAM-3). Every flag drills to its evidence — last touch, who is threaded
and who has no edge, with per-contact capture provenance — through the same explain
mechanic, and a flag clears on the next recompute when fresh multi-threaded touch
lands: proactive, never a static snapshot. Grounded evidence quotes come from
captured artifacts; where the flag is an absence, the surface says so instead of
fabricating a quote.

**Hot aggregates are served from read models.** Per ADR-0004, repeated aggregates
ride materialized read models refreshed off the event stream and the stage-history
log, with dashboard widgets cached; ad-hoc cache-miss reports hit the live planner
under the refresh budget ([[acceptance-standards#PERF-R4]]). If the cache-miss
large-scan budget cannot be met by tuning, the pre-decided escalation is a second
engine scoped to named aggregates behind the same plan surface — a follow-up
decision, with the Postgres path remaining the correctness oracle.

## What's configurable

- **The prebuilt report-key catalog** — eight keys with declared per-key
  vocabularies and defaults (REPORT-KEY-1..8); catalog and vocabularies are source
  configuration, not runtime knobs (P1). A key is never shaped like a saved-report
  identifier, so the two resolve without collision (REPORT-WIRE-2).
- **Coverage thresholds** — the threading floor and the no-touch and silence
  windows (REPORT-PARAM-1..3) are source constants with pinned defaults; no runtime
  tuning in V1.
- **Scheduled delivery** — recipients, channel, and schedule for saved-report
  delivery are the one runtime-config surface here, owned by the runtime-config
  boundary ([[runtime-config#RC-9]]); read-only over reports.
- **The model client** — the plain-language compile step rides the injected model
  client. With no model bound, the engine degrades to the structured builder and
  prebuilt keys — the grammar-bound path the poc validated — never to a worse
  guess; every deterministic guarantee below holds identically in degraded mode.

## Guarantees (enforced)

- **Parameterized queries only.** No identifier or value is ever interpolated into
  query text; allowed fields are baked into the compile-time vocabulary, not read
  from a runtime schema blob. The one user-supplied identifier — an aggregate alias
  — is validated against a strict allow-list and rejected before any query is
  generated, guarded by hostile-alias tests.
- **Out-of-vocabulary is refused pre-database.** An unknown field or function
  returns the named validation error with zero database contact (DM-VOCAB-ERR-1);
  an unknown report key is not-found — tested as behavior.
- **No fast-but-wrong numbers.** Every aggregate anywhere in the product
  reconciles by automated test to independently computed ground truth on the
  seeded benchmark and exposes a derivation ([[acceptance-standards#AC-X1]],
  [[acceptance-standards#AS-SCALE-1]]) — zero tolerance, a release gate.
- **The plan round-trips.** The plan returned with a result re-runs to the same
  result, and the drill-through reuses that exact plan, so explanation and
  aggregate are provably the same query.
- **Honest joins.** A cross-object report through a many-to-many relationship edge
  neither double-counts nor drops rows versus ground truth; both sides of every
  join are workspace-scoped, so isolation holds across the join.
- **Workspace isolation, defense-in-depth.** Every plan carries an explicit
  workspace predicate in addition to row-level security; a foreign workspace
  returns zero rows and an empty workspace short-circuits to zero.
- **Ambiguity clarifies, never guesses.** An ambiguous or out-of-vocabulary
  utterance returns a clarifying prompt — an unflagged wrong answer is a hard
  failure, deterministic on the labeled ambiguous set ([[ai-evals#AIEVAL-17]]);
  the model-bound compile quality itself is an eval band, never a merge gate
  ([[ai-evals#AIEVAL-15]]).
- **The budgets are gates.** Report, dashboard, refresh, compile, and derivation
  budgets ([[acceptance-standards#PERF-R1]]..PERF-R6) run as CI benchmarks against
  the seeded dataset.

## Acceptance

Done means Riya can build the cross-object report that silently broke in her last
CRM and trust the figure without exporting to a spreadsheet; click any number on
any surface and see, in plain language, exactly what produced it, then open the
rows themselves, with the sum of what she sees reconciling exactly to the number
she clicked; ask a question in plain words and get a shown, saveable plan or an
honest clarifying question; and open the coverage surface to see which accounts are
relationally exposed, with the evidence one click away and flags that clear
themselves when fresh touch lands. The surfaces render the standard honest states
([[acceptance-standards]] STATE-1..5) — including the coverage surface's honest
refusal to show stale data on a failed recompute and its honest-absence evidence —
and the cross-cutting floor (screen states, budgets, release gates) is inherited
from the acceptance-standards chapter, not restated. Testable forms live in the
Acceptance appendix; the model-bound band rides the eval catalog (AIUC-20, owned by
[[ai-evals]]).

## Out of scope

- **The stage-history write path** — one entry per stage move, amount snapshotted,
  same transaction as the change — is [[deals-and-pipeline]]'s; this chapter only
  reads the log.
- **Forecast semantics** — the weighted formula, commit/best-case accounting, and
  close-date hygiene (S-E09.3, S-E09.5) belong to [[deals-and-pipeline]] (the
  weighted pipeline value) and the planned forecasting chapter; this chapter runs
  and explains their numbers and pins the shared screen.
- **Score semantics** — deal-health and lead-score decomposition (S-E09.4) belong
  to the scoring owner; the explain mechanic here is what renders them.
- **Per-item revenue impact and confidence** — pinned at
  [[morning-brief#BRIEF-FORM-2]]; cited, never re-pinned.
- **Conversational cited-answer search** — [[search-and-retrieval]] (its AC6.3 and
  AC6.4); this chapter owns only the report-plan side of the plain-language band.
- **MCP tool governance** — the report-running tool row and its session caps are
  [[byo-agent-and-mcp]]'s (BYO-TOOL-9, BYO-LIM-3); the plan surface it calls is
  this chapter's.
- **Saved views, lists, and segments** — the [[data-model]] ownership index places
  them with the lists chapter; quota records live with records-depth.
- **Deferred by the cut line** — marketing/web-analytics blending, multi-touch
  attribution, pivot-table and cohort UIs, cross-workspace roll-ups, custom
  visualization plugins, and stage-conversion rates (counts ship; rates do not).

## Where it lives

The reporting module in the backend — the vocabulary-bound plan compiler, the
runner, the report-key catalog, and the coverage queries — reached over the single
reports operation on the contract and the governed report-running verb on the MCP
surface, with the forecast-and-reports and coverage surfaces in the web app. Read
next: [[deals-and-pipeline]] (the stage-history substrate and weighted value),
[[search-and-retrieval]] (the other half of the plain-language band),
[[byo-agent-and-mcp]] (the governed tool surface), and [[morning-brief]] (the
sibling surface whose numbers roll into the same forecast).

## Appendix

### Parameters
Source: specs/spec/contract/data-model.md#135-sort--filter-vocabulary-the-per-resource-allow-list @ 5a0b29c; specs/spec/features/03-reporting-and-scoring.md#12-the-clean-core-advantage-the-hubspot-fix--headline @ 5a0b29c

The prebuilt report-key catalog, ceded to this chapter by the data-model chapter
("the prebuilt report-key catalog to reporting"; its §13.5 pins the key list
verbatim and defers the vocabularies here). Key names verbatim from the corpus;
a key is never a UUID, so it never collides with a saved-report id.

| ID | Name | Value | Meaning |
|---|---|---|---|
| REPORT-KEY-1 | `open-deals-per-company` | prebuilt key | Distinct open deals counted per organization across the FK — the headline "aggregate across associated records" fix. |
| REPORT-KEY-2 | `pipeline-by-stage` | prebuilt key | Open pipeline grouped by stage; with `as_of_date` it answers from the stage-history log (pipeline as of date X), never from current rows. |
| REPORT-KEY-3 | `forecast-weighted` | prebuilt key | Weighted pipeline (stage probability × value) next to unweighted; the formula is [[deals-and-pipeline]]'s DEAL-FORM-2, and the won-at-100 / lost-at-0 accounting is the planned forecasting chapter's — this key runs and explains it. |
| REPORT-KEY-4 | `stage-conversion` | prebuilt key | Funnel of stage-transition counts from the stage-history log over all time; rates are OUT V1. |
| REPORT-KEY-5 | `rolling-coverage` | prebuilt key | Rolling pipeline-coverage ratio (open pipeline ÷ remaining quota), exact from stage history; quota rows live with records-depth. |
| REPORT-KEY-6 | `activity-volume` | prebuilt key | Activity counts by kind/owner/period over captured activity (e.g. activities per contact in 30 days). |
| REPORT-KEY-7 | `lead-funnel` | prebuilt key | Lead-status funnel counts; lead semantics are [[leads-and-qualification]]'s. |
| REPORT-KEY-8 | `win-loss` | prebuilt key | Won vs lost by period/source/owner (win rate by source last quarter is the canonical utterance). |
| REPORT-PARAM-1 | Threading floor | `2` | An open deal whose distinct engaged-contact count is below 2 is single-threaded (coverage rule verbatim in AC-coverage-6). |
| REPORT-PARAM-2 | No-touch windows | `30 / 60` days | An in-pipeline account with no captured touch for ≥30 (or ≥60) days surfaces in the matching no-touch view. |
| REPORT-PARAM-3 | Won-but-silent window | `≥ 90` days | A closed-won account with no captured activity since close beyond 90 days surfaces as won-but-now-silent. |

Note REPORT-N-1 (honest gap): each key "declares its own dimension/measure
vocabulary" per the corpus, but no corpus source enumerates the per-key
vocabularies beyond the key names and the per-object allow-lists (DM-VOCAB-1..6,
owned by [[data-model]]). The per-key vocabularies and defaults must be pinned at
ticket time before the catalog is buildable.

Note REPORT-N-3 (poc divergence, reconcile at ticket time): the poc reference
engine shipped two cross-object keys not in the corpus catalog —
stakeholders-per-deal and deals-per-person, distinct counts through the
relationship edge with both join sides workspace-scoped. AC-R3 requires exactly
such a many-to-many report; either the catalog grows two keys or the ad-hoc path
must cover the relationship join. Do not silently drop the poc's honest-join test
fixtures.

Note on thresholds: REPORT-PARAM-1..3 appear only in the story and screen ACs —
the corpus parameter registry does not carry them; registering them as named
source constants is a ticket-time task.

NL-band thresholds are cited, not pinned here: plan-correctness ≥ 90 percent is
the eval band [[ai-evals#AIEVAL-15]]; executed-plan equality = 100 percent and
unflagged-wrong-answer = 0 are the deterministic gates [[ai-evals#AIEVAL-16]] and
[[ai-evals#AIEVAL-17]] (use case AIUC-20).

### Wire
Source: specs/spec/contract/crm.yaml (reports path + RunReportRequest/ReportResult schemas) @ 5a0b29c; specs/spec/contract/README.md (MCP verb table) @ 5a0b29c

One reporting operation exists in the contract at pin time; the rest of the
V1-promised surface is an honest gap.

| ID | Element | Behavior pinned |
|---|---|---|
| REPORT-WIRE-1 | `runReport` (POST on the reports path) | Executes a validated, typed query plan — never free-form SQL. Request: `filters` / `group_by` / `aggregates` (fn ∈ count, count_distinct, sum, avg, min, max) / `as_of_date`; every field must be in the target report's vocabulary or the call returns `422 report_field_not_allowed` with no database touched. Result: `report`, the executed `plan` (round-trips inspectably), `columns`, `rows`, `total_rows`, `generated_at`, `derivation_url`. MCP verb `run_report`, tier 🟢 — the tool row and session caps are owned at [[byo-agent-and-mcp]] (BYO-TOOL-9, BYO-LIM-3); the query-cost ceiling is [[api-conventions#CAP-QUERY-COST]]. |
| REPORT-WIRE-2 | Report path parameter | Dual resolution, collision-free: a UUID-shaped value resolves to a saved report; anything else resolves against the prebuilt key catalog (REPORT-KEY-1..8) — a key is never a UUID. Unknown key → not-found. |
| REPORT-WIRE-3 | Derivation resolution (gap) | `ReportResult.derivation_url` is a declared handle, but **no operation in the contract resolves it** — the explain-this-number endpoint (plain-language definition + source rows, reusing the same compiled plan) must be minted by a contract extension before any drill-through ticket cites an operationId. |
| REPORT-WIRE-4 | Plain-language compile (gap) | No utterance-to-plan operation exists. [[search-and-retrieval]] carries the matching finding (its SEARCH-WIRE-N-1) and explicitly assigns the NL-report plan endpoint here; it must be minted, returning a shown, validated plan (or a clarifying question) without executing. |
| REPORT-WIRE-5 | Saved reports, dashboards, export, delivery (gap) | The cut line promises saved reports, ≥4 prebuilt dashboards, CSV/JSON export with the documented query retrievable, and scheduled delivery — none has a contract operation at pin time (saved-views is the lists chapter's different resource). Delivery config is runtime surface [[runtime-config#RC-9]]; export rides the async-job states ([[acceptance-standards]] STATE-SP-5). All must be minted before their tickets cut. |
| REPORT-WIRE-6 | Coverage views (gap) | No coverage-view read operation exists for S-E09.6 (three views, thresholds, evidence drawer, scope segment). Reads only; the one write-ish action on the screen (draft a re-thread intro) is 🟡 and belongs to the approvals path, never to this surface. |

Note REPORT-N-4 (carried from the corpus cross-cutting gaps): the prototype's
scope/period recompute controls are toasts — the build must define which controls
trigger a server round-trip versus a client-side filter, each with its budget.

### Events
Source: specs/spec/contract/events.md (catalog scan at pin) @ 5a0b29c

Note REPORT-EVT-N-1 (honest): the central event catalog defines **no** reporting
events — this chapter emits nothing and defines nothing. It consumes state, not
events: history-sourced reports read the stage-history table
([[deals-and-pipeline]]), and read-model refresh rides the audit/event stream per
ADR-0004 without a reporting-owned event type. Scheduled report delivery publishes
through the platform bus (P9) under [[runtime-config#RC-9]]; if delivery needs a
named event type, the event-bus chapter mints it, not this one.

### Acceptance

#### Acceptance — stories (condensed G/W/T)
Source: specs/spec/product/epics/E09-reporting-and-forecast.md#s-e091--reporting-that-just-works @ 5a0b29c; specs/spec/product/epics/E09-reporting-and-forecast.md#s-e092--explain-this-number @ 5a0b29c; specs/spec/product/epics/E09-reporting-and-forecast.md#s-e096--account-mapping--coverage-risk-views @ 5a0b29c

This chapter is primary for S-E09.1, S-E09.2, and S-E09.6 (verified: no sibling
chapter claims them; [[signals-and-warm-room]] explicitly hands the coverage view
here, and [[meetings-and-transcripts]] takes only partial explain-this credit).
S-E03.4 is [[deals-and-pipeline]]'s; S-E09.3/.5 are the planned forecasting
chapter's; S-E09.4's score semantics are the scoring owner's — their numbers
render and explain through this chapter's mechanic.

| ID | Given/When/Then | Verification |
|---|---|---|
| REPORT-AC-1 | Given captured data and deals across stages, when Riya runs the pipeline / conversion / activity / win-loss reports — including counts and sums in computed fields, cross-object joins (deal → company → owner, deal → stakeholders), reports over a code-defined custom object, and "pipeline as of last quarter-end" — then every total is correct, no row silently drops or double-counts, the custom object behaves identically to core, the as-of snapshot is exact from stage history, any report exports to CSV/JSON with the documented query retrievable, and it is fast enough to be credible. (S-E09.1, V1-Must) | Golden-number suite on the seeded benchmark ([[acceptance-standards#AC-X1]], AS-SCALE-1); CI benchmarks PERF-R1..R4 |
| REPORT-AC-2 | Given any aggregate cell, KPI, forecast figure, or health score, when Riya clicks it, then she gets the exact filter + group + aggregate definition in plain language and the underlying source rows; a derived input's lineage recurses; summing the drill-through rows reconciles exactly to the displayed number; the derivation returns sub-half-second; and the same one primitive explains every number in the product. (S-E09.2, V1-WOW) | Deterministic reconciliation test (same compiled plan reused); CI benchmark [[acceptance-standards#PERF-R6]] |
| REPORT-AC-3 | Given open deals and captured touches, when Riya (or Sam on his own book) opens the coverage surface, then single-threaded deals, in-pipeline no-touch accounts at the 30/60-day thresholds, and won-but-now-silent accounts each surface with drillable evidence (last touch, who is threaded, who has no edge), and a flagged account clears on the next recompute after fresh multi-threaded touch. (S-E09.6, V1-WOW) | Deterministic view tests on fixed fixtures + clock (REPORT-PARAM-1..3); screen/E2E suite below |

#### Acceptance — feature ACs (verbatim)
Source: specs/spec/features/03-reporting-and-scoring.md#15-acceptance-criteria @ 5a0b29c

Corpus IDs preserved verbatim. AC-R11's weighted-value formula is
[[deals-and-pipeline]]'s (DEAL-FORM-2); the reconciliation mechanic it demands is
this chapter's.

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-R1 | **(correctness, golden numbers)** For each prebuilt report and a fixed set of ≥30 parametrized custom reports, the displayed aggregate equals an independently-computed ground-truth SQL value on the seed dataset. Automated test; zero tolerance. | Golden-number suite ([[acceptance-standards#AC-X1]]) on AS-SCALE-1 |
| AC-R2 | **(the HubSpot fix)** Automated test asserts a `COUNT`/`COUNT DISTINCT` computed field returns correct values; asserts "open deals per company" aggregates correctly across the FK; asserts a report over a code-defined custom object returns the same correct result as the equivalent core-object report. All three pass. | Deterministic integration tests, reporting lane |
| AC-R3 | **(no silent join error)** A cross-object report over a many-to-many `relationship` (e.g. deal stakeholders) returns no duplicated/dropped rows vs ground truth; asserted by test. | Deterministic join test (see REPORT-N-3 for the poc fixtures) |
| AC-R4 | **(perf)** Single-object report p95 < 300 ms; cross-object < 500 ms; dashboard (≤8 widgets) < 800 ms server; saved-report cache-miss refresh < 1.5 s — all on the seed dataset, enforced in CI. | CI benchmarks [[acceptance-standards#PERF-R1]]..PERF-R4 |
| AC-R5 | **(NL reporting — ML eval, not a deterministic CI gate)** NL→query-plan is model-bound and is tracked as an **ML eval against a defined, version-controlled eval set** of ≥50 labeled NL utterances, with a **flaky-aware threshold** (KPI: ≥90% of utterances compile to a plan whose result matches the hand-authored reference; the band, not a single hard pass/fail, is the merge signal, since the result depends on model version/prompt). The eval set's construction/sizing/labeling is owned by the testing/quality spec. **The deterministic gates that remain real CI gates:** (a) for any *given* compiled plan, its executed result equals the reference golden number on the seed dataset (covered by AC-R1/AC-X1); (b) ambiguous/out-of-vocabulary utterances return a clarifying prompt and **never** an unflagged wrong answer *(deterministic test on the labeled ambiguous subset)*; (c) first plan shown < 1.5 s (perf budget). NL→SQL equivalence is not claimed as a general deterministic property. | Eval band [[ai-evals#AIEVAL-15]]; deterministic gates [[ai-evals#AIEVAL-16]]/[[ai-evals#AIEVAL-17]]; budget [[acceptance-standards#PERF-R5]] |
| AC-R6 | **(Explain This Number)** Every aggregate widget exposes drill-through; clicking returns the plain-language definition + source rows in < 400 ms p95; the summed drill-through rows reconcile exactly to the displayed aggregate (test). | Deterministic reconciliation test; [[acceptance-standards#PERF-R6]] |
| AC-R7 | **(export/own-data, P7)** Any report exports to CSV and JSON; export of a 50k-row report completes < 10 s; the documented query is retrievable via API. | Export integration test (STATE-SP-5 job states); contract op pending (REPORT-WIRE-5) |
| AC-R8 | **(BYO-agent)** A Layer-1 agent calling `run_report` is bound to its scopes, cannot read beyond the human's RBAC, and every call is audit-logged with inputs/outputs (P12). | Contract/scope test on the governed surface ([[byo-agent-and-mcp#BYO-TOOL-9]]) |
| AC-R9 | **(user-observable — reporting that just works)** A revenue leader builds a cross-object report ("open deals per company," "win rate by source last quarter") and gets a number that is *correct* — the kind of report that silently broke or returned wrong values in HubSpot now just works, with no association-resolution surprises. The leader can trust the figure without exporting to a spreadsheet to check it (S-E09.1). | Screen/E2E + golden-number suite |
| AC-R10 | **(user-observable — explain this number)** Any number on a report or dashboard is clickable; the leader sees, in plain language, the exact filter/group/aggregate behind it and can drill straight to the underlying rows — every figure traces back to records they can open, so a disputed number is settled by looking, not arguing (S-E09.2). | Screen/E2E + reconciliation test |
| AC-R11 | **(user-observable — auditable weighted pipeline)** The weighted pipeline value a leader sees decomposes on click into the deals and stage probabilities that produced it, and reconciles exactly to their sum — the leader can see *why* the number is what it is, not just the total (S-E03.4). | Reconciliation test against [[deals-and-pipeline]] DEAL-FORM-2; AC-reports-2/3 below |

Note REPORT-N-2 (unification, carried from the corpus cross-cutting gaps):
"explain this number", "show the evidence", and "how is this computed" are three
names for **one** mechanic — one interaction and one component across the reports,
home, person, company, and client surfaces, all resolving through this chapter's
derivation primitive. Ticket-gate: no surface ships a second explain
implementation.

#### Acceptance — screen: forecast & reports (verbatim)
Source: specs/spec/product/30-screen-acceptance.md#reportshtml--forecast--reports-implements-s-e034-s-e091245 @ 5a0b29c

This chapter owns the forecast-and-reports screen's pins (primary stories S-E09.1,
S-E09.2; no sibling pins it). The screen also implements S-E03.4
([[deals-and-pipeline]]), S-E09.4 (scoring owner), and S-E09.5 (planned
forecasting chapter) — their semantics live in those chapters; the rendered
behavior is pinned once, here. Corpus screen-AC IDs preserved verbatim.

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-reports-1 | Given the headline tiles, When the page loads, Then Commit and Best-case are labeled "unweighted" and Weighted pipeline "amount × prob," and a banner states the two are "different numbers on purpose." | Screen/E2E test |
| AC-reports-2 | Given the Commit tile's "Explain this number", When clicked, Then a panel expands listing each commit deal (company, deal, base value, stage prob., "rep-committed" source) with a summary row equal to the Commit headline, and a note it is a SQL SUM over real rows, not a re-estimate; rows are clickable to the deal. | Screen/E2E test (the derivation primitive, REPORT-AC-2) |
| AC-reports-3 | Given the explain breakdown, When the user sums the listed base values by hand, Then they reconcile exactly to the Commit total (no double-count). | Screen/E2E + reconciliation test ([[acceptance-standards#AC-X1]]) |
| AC-reports-4 | Given the weighted-pipeline-by-stage bar chart, When it renders, Then each stage shows a raw open-value bar and a weighted bar with both figures labeled, plus an "open weighted total" matching Σ across stages. | Screen/E2E test (values: [[deals-and-pipeline]] DEAL-FORM-2) |
| AC-reports-5 | Given the scope segmented control (Me / My team / Everyone), When changed, Then a toast confirms the recompute is "from deal owner over the same rows". | Screen/E2E test (real recompute wiring: REPORT-N-4) |
| AC-reports-6 | Given the period control (This month / quarter / FY / Custom…), When changed, Then a toast confirms "all numbers recomputed from deal_stage_history" for the exact as-of dates; Custom… displays the entered range. | Screen/E2E test (real recompute wiring: REPORT-N-4) |
| AC-reports-7 | Given the close-date hygiene section, When it renders, Then it lists each open deal whose close date is in the past, shows struck-through old → proposed new date, states these are excluded from Commit until corrected, and shows the € that correcting all would add back to Best-case. | Screen/E2E test (rule semantics: planned forecasting chapter / S-E09.5) |
| AC-reports-8 | Given a hygiene row's "Accept date", When clicked, Then the row replaces the action with "Date set <date>", dims the old date, and toasts "re-qualifies for this period · change logged & reversible" — never changed silently. | Screen/E2E test (🟡 semantics: planned forecasting chapter) |
| AC-reports-9 | Given the Forecast-vs-target box, When it renders, Then it shows best-case attainment vs quota with a fill bar, a rolling coverage ratio (open ÷ remaining quota) attributed to deal_stage_history, and a gap-to-commit-cover figure. | Screen/E2E test (REPORT-KEY-5; quota rows: records-depth) |
| AC-reports-10 | Given the Export button, When clicked, Then a toast confirms the forecast exported to CSV "with documented query included" (P7). | Screen/E2E test (contract op pending, REPORT-WIRE-5) |
| AC-reports-11 | Given the funnel + accuracy sparkline, When they render, Then the funnel shows per-stage counts and advance %s (sub-55% drops flagged red) and accuracy shows trailing-4-quarter predicted-vs-actual labeled "reproducible from history." | Screen/E2E test (REPORT-KEY-4; accuracy tracking: planned forecasting chapter) |

Screen floor: the standard state matrix ([[acceptance-standards]] STATE-1..5)
applies as real rendered states. The corpus AI-mechanics note pins:
explain-this-number on Commit as the central trust mechanic; per-row
"rep-committed" provenance; 🟡 staging on overnight close-date proposals;
raw-vs-weighted side-by-side with the formula shown; confidence dots deliberately
absent (deterministic aggregates).

#### Acceptance — screen: coverage (verbatim)
Source: specs/spec/product/30-screen-acceptance.md#coveragehtml--account-mapping--coverage-risk-implements-s-e096 @ 5a0b29c

This chapter owns the coverage screen (primary story S-E09.6;
[[signals-and-warm-room]]'s explicit hand-off, fed by that chapter's signals).
Corpus screen-AC IDs preserved verbatim.

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-coverage-1 | Given the screen loads with the "Single-threaded deals" view active, When it renders, Then it lists open accounts whose engaged-contact count is below 2 (BÄR Pharma, Brandt Automotive), each showing a thread pip meter, a "single-threaded · at risk" flag, stage, deal name, owner, last-captured-touch date with days-ago, and the deal amount. | Screen/E2E test (REPORT-PARAM-1) |
| AC-coverage-2 | Given the three view tabs ("Single-threaded deals", "In pipeline · no touch", "Won, now silent"), When the user clicks a tab, Then only that view's accounts render and each tab shows a live count badge that reflects the current book scope and any cleared flags. | Screen/E2E test |
| AC-coverage-3 | Given the "In pipeline · no touch" view is active, When the user toggles the 30-day / 60-day threshold segment, Then the row set changes accordingly — 30 days surfaces accounts with ≥30 captured no-touch days (Voss, 36d), and 60 days surfaces only the ≥60-day accounts (Kessler, 66d). | Screen/E2E test (REPORT-PARAM-2, fixed clock) |
| AC-coverage-4 | Given any account row, When the user clicks its header, Then a drawer expands showing the last captured touch, a "Who is threaded" list of engaged contacts (with name, role, date, and provenance like agent:capture vs human:logged) and unengaged contacts tagged "no edge", plus the evidence behind the flag. | Screen/E2E test |
| AC-coverage-5 | Given an account with a captured artifact (e.g. BÄR Pharma), When its drawer is open, Then a grounded quote is shown with its source citation, a "grounded" link to the deal, and a confidence dot (high/medium); Given an account with no recent captured artifact (e.g. Kessler, Voss), Then no quote is fabricated and the drawer states the flag is the absence itself. | Screen/E2E test ([[acceptance-standards#GATE-AI-1]], STATE-5) |
| AC-coverage-6 | Given an open account drawer, When the user clicks "Explain this flag", Then an explainbox reveals the exact rule and its inputs (e.g. single_threaded = distinct_engaged_contacts < 2; no_touch_60 = days_since_last_captured_touch ≥ 60; won_but_silent = closed_won AND ≥90 days), computed from captured activity only. | Screen/E2E test (the same explain primitive, REPORT-N-2) |
| AC-coverage-7 | Given a flagged account, When the user clicks "Draft a re-thread intro (🟡)", Then the action is queued to the approval inbox via a toast, the row is marked "fresh touch — clears on refresh", and the flag clears from the view (and counts decrement) on the subsequent recompute. | Screen/E2E test ([[acceptance-standards#GATE-AI-7]]; approvals path) |
| AC-coverage-8 | Given the book-scope segment ("Whole team" / "My book"), When the user switches scope, Then the scope note and open-account count update (whole team · 14 → my book · 8) and rows/counts filter to the rep's own accounts; whole-team scope is gated to the Revenue leader role. | Screen/E2E test (STATE-4 on the leader gate) |

Screen floor (corpus states, pinned): honest empty per view ("No coverage risk in
this view", framed self-clearing); refresh renders a skeleton then stamps a fresh
recompute time; the error state **declines to show stale or partial data** and
cites the last good snapshot; the no-permission state blocks whole-team scope for
a rep and offers the own-book switch; ungrounded flags render the absence
honestly. The corpus AI-mechanics note additionally pins per-contact capture
provenance, the advisory-not-autopilot footer, and that the 🟡 draft action never
reassigns, emails, or advances anything on its own.
