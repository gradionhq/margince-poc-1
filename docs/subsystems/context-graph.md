---
status: planned
module: backend/internal/modules/search (graph assembly — per the architecture chapter's roster, the graph capability lives inside the search module)
derives-from:
  - specs/spec/product/build-backlog/EP05.md#f-context-graph-assembly-perf-7-slo--benchmark-harness
  - specs/spec/contract/events.md#43-consumer-groups
  - specs/spec/narrative/06-nonfunctional.md#61-performance-budgets
  - margince-poc/docs/subsystems/context-graph.md @ a11d6c08
---
# Context graph — what matters in this workspace, right now, with the evidence attached

> The substrate every whole-pipeline AI feature reads from: given a workspace, assemble
> the small set of records that matter now — deals worth a nudge, stalled deals, the
> strongest stakeholders, the warm room around a deal — each carrying the evidence that
> put it there and a deterministic confidence. It is a **capability on the relational
> core, not a datastore** (ADR-0007; scope [[scope#NEVER-10]]).

## What it's for

Higher layers — the morning brief, the overnight agent, forecasting, agent intent tools —
need "what's relevant about this workspace *now*", not a raw table scan. The context
graph answers a small set of canonical questions over the existing relational core plus
the embedding store, returning ranked candidate sets those features consume directly:
**brief candidates** (open deals restricted to the per-deal inputs the brief formula
needs), **stalled deals** (linked-activity recency past the stall threshold),
**relationship strength** (the strongest stakeholders per deal, by edge-walk), and
**warm room** (people warm or blocking around a deal or organization). Every candidate
carries the discrete facts that explain its inclusion, so nothing reaches a brief or an
agent without traceable evidence. This chapter is substrate: it has **no user stories of
its own** — the morning-brief and overnight-agent chapters own the moments that ride it.

## Principles it serves

- **P4 — blazing fast.** Candidate assembly is an indexed, bounded walk, never a
  full-table scan, and it is held to the pinned assembly budget
  ([[acceptance-standards#PERF-7]]) by the CI benchmark harness.
- **P6 — embrace the LLMs.** This is the deterministic candidate layer AI rankers
  re-order *within*; the graph narrows and grounds, the model judges.
- **P12 — governance designed in.** Evidence on every candidate, workspace scoping at the
  database, and a confidence a test can reproduce byte-for-byte.
- **ADR-0007 — the context graph is V1 substrate.** Capture links records into one
  connected picture; this chapter is the cross-pipeline reasoning layer that ADR names —
  built as a capability, from day one.
- **ADR-0021 — relational + pgvector is the substrate; a graph store is trigger-gated.**
  Assembly runs as fixed-depth queries in the one governed Postgres. A dedicated graph
  datastore is not roadmapped ([[scope#NEVER-10]]); it is adopted only if the named,
  measured trigger fires — and the assembly budget is that trigger's sensor.

## How it works

- **Assembly is a bounded walk.** The candidate sets are assembled as fixed-depth
  recursive queries — at most three hops (CG-PARAM-1) — over the relational core's typed
  relationship and activity-link edges plus the embedding store, directly inside the
  query. Deliberately no graph store, no N-degree traversal, no centrality: the
  graph-shaped patterns that would justify a native engine are out of V1 scope
  (ADR-0021).
- **Two link rows make one hop.** An activity link never carries two entity references at
  once (a shape constraint the data-model chapter owns forbids it), so the deal-to-person
  hop joins **two** link rows on their shared activity. Assuming one row holds both ends
  silently returns nothing — the traversal, and its test, are written for the two-row
  shape (CG-AC-8).
- **Deterministic confidence.** Each candidate is scored with the ratified three-weight
  ranking over similarity, recency and source trust, tie-broken by identity — the formula
  is owned by [search-and-retrieval](search-and-retrieval.md) (SEARCH-FORM-1) and reused
  here unchanged, so the same seeded data yields the same order on every run. Similarity
  reads the shared embedding store via the vector index inside the query; the graph needs
  no dependency on the search engine's query path.
- **Evidence or omit.** A candidate carries the discrete facts that put it in the set —
  the linked activities, the edges walked, the recency that tripped a threshold. A
  candidate without evidence is a hard failure, the assembly-layer form of the no-guess
  gate ([[acceptance-standards#GATE-AI-1]]).
- **Trust travels as a label.** Confidence is the only ordering key; a candidate's source
  trust tier rides along as a label for downstream handling — captured T2 content stays
  data, never instructions ([[threat-model#T2]]; [trust-propagation](trust-propagation.md))
  — and contributes to ranking only through the formula's pinned trust term.
- **Freshness rides the event stream.** The graph consumes the entity streams through its
  own consumer group ([[event-bus#EVT-CG-1]]), so capture-to-link assembly and the
  reasoning inputs stay current without polling.
- **The budget is the trigger.** Assembly meets the pinned budget
  ([[acceptance-standards#PERF-7]]) at the mid-market tier, measured by the benchmark
  harness owned by [search-and-retrieval](search-and-retrieval.md). A breach that
  survives the mitigation ladder is not a slow query — it is the named ADR-0021 evidence
  that opens the graph-store decision.

## What's configurable

- **Hop depth** — capped at three (CG-PARAM-1); a deeper relationship is not pulled into
  a candidate set, ever. Raising it is a spec change, not a tuning knob.
- **Stall threshold** — the activity-recency cutoff that classifies a deal as stalled is
  owned by [deals-and-pipeline](deals-and-pipeline.md) (DEAL-PARAM-1) and read here, not
  redefined.
- **Ranking weights** — the confidence weights are the pinned constants of
  [search-and-retrieval](search-and-retrieval.md) (SEARCH-PARAM-1..3); the graph inherits
  them, it does not fork them.

## Guarantees (enforced)

- **Workspace isolation** — every assembly query runs under row-level security; a
  two-workspace fixture asserts zero cross-tenant candidates (CG-AC-1).
- **Depth-capped** — a four-hop fixture does not pull the far node in (CG-AC-2).
- **Indexed, never a sequential scan** — plan-coverage checks assert the substrate
  indexes are used on the assembly queries (CG-AC-3).
- **Byte-stable order** — identical seeded data produces an identical ranked output
  across runs: deterministic formula, deterministic tie-break (CG-AC-4).
- **Evidence-carrying** — every candidate arrives with the facts that explain it; an
  evidence-free candidate is a test failure, not a warning (CG-AC-5).
- **Budget as SLO** — assembly holds the pinned budget at mid-market or the ADR-0021
  trigger evidence is emitted; the gate is a red build, not a dashboard hope (CG-AC-7).

## Acceptance

Done means the four candidate sets exist, are correct on seeded data, and keep their
promises under measurement: an operator can seed two workspaces and see no bleed; re-run
an assembly and get the identical order; inspect any candidate and find its evidence; and
read the harness output to know whether the substrate holds the budget or the graph-store
trigger fired. The testable form of each claim is pinned in the Acceptance appendix; the
cross-cutting floor is inherited from the acceptance-standards chapter and not restated.

## Out of scope

- **The formulas the graph feeds.** The brief rank composite is owned by the
  morning-brief chapter; the warm-room signal semantics by the signals-and-warm-room
  chapter; deal-health scoring by the forecasting chapter; the scalar per-person
  relationship-strength score by [people-and-organizations](people-and-organizations.md)
  (PO-F-3); the stalled-deal flag by [deals-and-pipeline](deals-and-pipeline.md)
  (DEAL-FORM-3). The graph assembles their candidate inputs; it defines none of them.
- **The search engine and its harness.** The hybrid query path, the embedding store's
  schema, and the volume-tier benchmark harness are owned by
  [search-and-retrieval](search-and-retrieval.md).
- **HTTP and contract exposure.** None exists and none is planned here — callers reach
  assembly through the retrieval port's context-assembly entry point
  ([retrieval-seam](retrieval-seam.md)); any future wire surface belongs to the feature
  that needs it.
- **The AI ranker.** Re-ordering candidates with a model, and everything rendered to a
  user, belongs to the consuming chapters.

## Where it lives

The assembly capability lives in the graph module directory, a sibling of the search
module behind the same retrieval port — callers reach it only through that port's
context-assembly entry point. Read [retrieval-seam](retrieval-seam.md) for the contract,
[search-and-retrieval](search-and-retrieval.md) for the engine and embedding store it
shares, and the event-bus chapter for the stream it rides.

## Appendix

### Parameters
Source: specs/spec/product/build-backlog/EP05.md#f-context-graph-assembly-perf-7-slo--benchmark-harness @ 5a0b29c; margince-poc/docs/subsystems/context-graph.md @ a11d6c08

| ID | Name | Value | Meaning |
|---|---|---|---|
| CG-PARAM-1 | Hop-depth cap | `3` | Maximum traversal depth of any assembly query (fixed 1–3 hop recursive queries, ADR-0021 §Context); a deeper node is never pulled into a candidate set. |

Cited, not owned: the stall threshold is [[deals-and-pipeline]] DEAL-PARAM-1 (`60` days);
the confidence weights are [[search-and-retrieval]] SEARCH-PARAM-1..3; the assembly
budget is [[acceptance-standards#PERF-7]]; the ADR-0021 trigger's volume thresholds
(~5M relationship edges / ~50M activity-link edges, calibration starting values) are
pinned in the vendored ADR-0021, not here.

### Formulas
Source: specs/spec/contract/formulas-and-rules.md#1072-retrievalconfidence-ranking-b-ep0520b @ 5a0b29c

This chapter owns no formula. Candidate confidence is the retrieval/confidence ranking
**[[search-and-retrieval]] SEARCH-FORM-1** (sanctioned restatement, owner's tag):
`score = 0.60·similarity + 0.30·recency + 0.10·source_trust`, deterministic tie-break by
id ascending (B-EP05.20b binds assembly output to exactly this scoring). The formulas
the candidate sets feed — brief rank, warm-room semantics, deal-health, the per-person
relationship-strength score, the stalled flag — are owned by the chapters named in Out
of scope.

### Schema
Source: specs/spec/contract/data-model.md#12-deferred-tables-later-phases--stubs-only-no-ddl @ 5a0b29c

**This chapter owns no tables** — reported honestly, per the ownership index: the graph
is a capability on tables owned elsewhere ([[scope#NEVER-10]]). It reads the typed
relationship and activity-link edges (owned by their core-object chapters per the
[[data-model]] ownership index), and the `embedding` store (deferred stub
[[data-model#DM-DEF-3]], owner-on-arrival [search-and-retrieval](search-and-retrieval.md)).
It ships **no migration of its own**: the poc proved assembly query-only over the
pre-existing indexes, and the planned build inherits that shape — any index the plans
need is pinned by the owning table's chapter.

### Wire
Source: specs/spec/contract/crm.yaml @ 5a0b29c

**No operations** — honestly none, not an omission: no assembly `operationId` exists in
the contract @ 5a0b29c, and none is planned. Callers reach assembly through the
retrieval port's context-assembly entry point ([[retrieval-seam]]; seam row
[[architecture#ARCH-SEAM-5]]). A consuming feature that needs a wire surface (the brief
endpoint, a coverage view) pins it in its own chapter.

### Events
Source: specs/spec/contract/events.md#43-consumer-groups @ 5a0b29c

Event definitions live in the central catalog ([[event-bus]]) — cited here, not
redefined. The graph **emits nothing** @ 5a0b29c.

| ID | Consumer group / event | Role here |
|---|---|---|
| [[event-bus#EVT-CG-1]] | `cg:context-graph` — person, organization, deal, activity, lead streams | Consumed for capture→link assembly freshness and cross-pipeline reasoning inputs (ADR-0007). The group is shared with the search module's (re)embedding consumer per the catalog row; idempotency rides the shared consumer library ([[operations#OPS-STOR-4]]). |

### Acceptance
Source: specs/spec/product/build-backlog/EP05.md#f-context-graph-assembly-perf-7-slo--benchmark-harness @ 5a0b29c; margince-poc/docs/subsystems/context-graph.md @ a11d6c08

No screen ACs (substrate — no screen maps to this chapter in the screen→story index) and
no story-derived ACs; every criterion below is a falsifiable assembly guarantee.

| ID | Given/When/Then | Verification |
|---|---|---|
| CG-AC-1 | Given records seeded in two workspaces, when any candidate set is assembled under the first workspace's context, then zero candidates from the second workspace appear — on every one of the four sets. | Two-workspace integration fixture ([[testing#TEST-LANE-2]]); RLS per [[data-model]]; port-level twin is [[retrieval-seam]] RETR-AC-1 |
| CG-AC-2 | Given a fixture where a record is reachable only at four hops, when assembly runs, then that record is **not** in any candidate set (CG-PARAM-1 holds). | Integration fixture, depth-cap assertion |
| CG-AC-3 | Given the four assembly queries on seeded data, when their query plans are captured, then the substrate indexes are used and no sequential scan appears on the walked tables. | Plan-coverage check in the integration lane |
| CG-AC-4 | Given identical seeded data, when the same candidate set is assembled repeatedly, then the ranked output is byte-identical across runs (deterministic score, id tie-break). | Reproducibility test (B-EP05.20b) |
| CG-AC-5 | Given any assembled candidate, when it is inspected, then it carries non-empty evidence — the records and edges that produced it; a candidate with no evidence fails the suite. | Deterministic integration assertion; assembly-layer form of [[acceptance-standards#GATE-AI-1]] |
| CG-AC-6 | Given a seeded fixture with hand-computed scores, when candidates are ranked, then each confidence equals the [[search-and-retrieval]] SEARCH-FORM-1 golden number and ordering follows score then id. | Golden-number test (B-EP05.20b) |
| CG-AC-7 | Given the benchmark harness at the mid-market volume tier, when the brief-candidate assembly runs, then p95 meets [[acceptance-standards#PERF-7]]; a breach fails the build red and — if it survives the ADR-0021 mitigation ladder — is emitted as named graph-store trigger evidence. | CI harness owned by [[search-and-retrieval]] (SEARCH-AC-7, B-EP05.21) |
| CG-AC-8 | Given a deal linked to a person only through a shared activity (two link rows, one entity reference each), when the deal-to-person hop is assembled, then the person is found — the traversal joins two link rows and a single-row assumption would return empty. | Integration fixture pinning the two-row join shape |
| CG-AC-9 | Given candidates carrying captured (T2) source content, when a candidate set is returned, then each candidate's trust tier is present as a label and influences ranking only through the pinned trust term of SEARCH-FORM-1. | Tier-label assertion; [[threat-model#TM-VERIFY-2]] alignment via [[trust-propagation]] |
