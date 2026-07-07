---
status: planned
module: modules/search (backend)
derives-from:
  - specs/spec/features/02-capture-and-comms.md#feature-6--baseline-ai-comms-summaries-draft-replies-nl-search
  - specs/spec/contract/formulas-and-rules.md#1072-retrievalconfidence-ranking-b-ep0520b
  - specs/spec/product/build-backlog/EP05.md#e-hybrid-retrieval-engine-crm-search
  - specs/spec/contract/data-model.md#12-deferred-tables-later-phases--stubs-only-no-ddl
  - margince-poc/docs/subsystems/retrieval.md @ a11d6c08
---
# Search and retrieval — find it by keyword or by meaning, and always show your sources

> The engine behind the platform's retrieval seam: a keyword arm and a semantic arm fused
> into one deterministic, workspace-scoped, citation-carrying ranking — plus the
> natural-language layer that turns a plain question into a validated query and a **cited**
> answer, or an honest "I'm not sure", never a guess. This chapter owns the engine; the
> port it fulfils is owned by [retrieval-seam](retrieval-seam.md).

## What it's for

Two ways of finding things, fused into one. Keyword search catches exact terms; semantic
search catches intent — the logistics deal in Hamburg even when those words never appear in
the record. The engine runs both arms and blends them so a query lands the right person,
organization, deal, activity or lead regardless of phrasing. Above the fused ranking sits a
conversational layer: a user or agent asks in natural language and gets an answer grounded
in real records with clickable citations, or a clarifying question when the ask is
ambiguous. Callers — the AI layer, agents over MCP, and the product's search surfaces —
never reach the engine directly; every one of them comes through the retrieval seam
([retrieval-seam](retrieval-seam.md)), whose contract (RETR-AC-1..5) this chapter's engine
is the first real implementation of.

## Principles it serves

- **P4 — blazing fast.** The keyword path is held under the pinned search budget
  ([[acceptance-standards#PERF-3]]) by a merge-blocking CI benchmark, and the engine ships
  the volume-tier benchmark harness that keeps that promise measured, not asserted.
- **P6 — embrace the LLMs.** Semantic search and grounded, cited answers — the engine is
  what lets AI features reason over the workspace without inventing facts.
- **P7 — own your data.** Embeddings are generated only on the customer's own injected
  model client (ADR-0020); under the sovereign profile a non-local client is refused before
  any call leaves the process. No external vector database, no bundled inference
  (ADR-0022).
- **P12 — governance designed in.** Every query is workspace-scoped at the database, every
  result carries its citation and trust tier, and answers cite their sources or decline.
- **ADR-0021 — relational + pgvector is the substrate.** The engine runs entirely in the
  one governed Postgres — full-text plus vector similarity, no second datastore — and
  ships the harness that makes the graph-store trigger a measured SLO instead of a debate.
- **ADR-0022 — build the engine, borrow only in-boundary transport.** Hybrid retrieval
  with rank fusion is a built differentiator, never delegated to a cloud aggregator or an
  external search service.

## How it works

- **Keyword arm.** Full-text ranking across the core objects and activities; the citation
  is the record itself. This arm works with no model bound at all, and it is the MVP cut
  line — full-text ships first, the vector arm is the fast-follow upgrade.
- **Semantic arm.** Vector similarity over the embedding store; closest-meaning records
  first. Embeddings are computed off the event stream (see Events) through the injected
  model client, deduplicated by content hash so unchanged text is never re-embedded, and
  billed to the customer's inference budget ([[operations#OPS-STOR-3]]).
- **Fusion.** Reciprocal rank fusion blends the two arm rankings into one deterministic
  list, preserving each result's citation; a golden-set test pins the fused order. An
  optional reranker stage sits after fusion, off by default (SEARCH-PARAM-5), toggled per
  query, and earns its latency before it is ever on.
- **Confidence ranking.** Where a caller needs scored results rather than a bare order —
  context assembly, evidence ranking — the engine scores candidates with the ratified
  three-weight formula over similarity, recency and source trust, tie-broken
  deterministically (SEARCH-FORM-1), so the same data always yields the same order.
- **Conversational layer.** A natural-language question is compiled to a query plan that
  is validated against the schema vocabulary before it runs; the executed plan is
  deterministic and its result is pinned against a hand-written reference. The
  utterance-to-plan step is model-bound and therefore tracked as an eval band, never
  claimed as a deterministic property (AC6.3); anything ambiguous or out-of-vocabulary
  returns a clarifying question — the no-guess rule ([[acceptance-standards#GATE-AI-1]]).
  Answers carry citations resolvable to source records. The poc's reference engine
  compiled plans without a model at all; that grammar-bound path remains the shape of the
  degraded, model-less mode.
- **One way in.** Everything above the engine reaches it only through the retrieval port;
  the AI layer never imports search internals (ADR-0007, held by the architecture lint —
  the port chapter pins this as RETR-AC-4).

## What's configurable

- **Embedding model** — injected at the composition edge (cloud, local, fake); the
  self-hostable default is pinned (SEARCH-PARAM-4) and BYOK is optional. With no model
  bound the engine degrades to keyword-only rather than failing or reaching outside.
- **Sovereignty enforcement** — under the sovereign profile a non-local model client is
  refused before any call, proven by a denied-path test; zero external egress.
- **Reranker** — optional post-fusion stage, off by default (SEARCH-PARAM-5), per-query
  toggle.
- **Ranking weights** — the three confidence weights are source constants with pinned
  defaults (SEARCH-PARAM-1..3); changing one is a code edit and redeploy, never a runtime
  knob.

## Guarantees (enforced)

- **Workspace isolation** — row-level security scopes every query on both arms and the
  fused result; a cross-tenant query returns nothing. The vector index applies workspace
  as a pre-filter, never a post-filter ([[operations#OPS-STOR-2]]).
- **Citations survive ranking** — every result, on either arm and after fusion, carries
  the identity of its source record and that record's trust tier, so captured T2 content
  arrives downstream labeled as data, never instructions ([[threat-model#T2]]; the
  tier-leak check [[threat-model#TM-VERIFY-2]] is the teeth).
- **Deterministic order** — fused rankings and confidence-scored rankings are byte-stable
  on the same data: rank fusion is deterministic and the confidence formula tie-breaks by
  identity (SEARCH-FORM-1).
- **No bundled inference** — embeddings and reranking come only through the injected model
  client (ADR-0020); no engine-operated inference endpoint exists.
- **Honest degradation** — no model means keyword-only results with a declared degraded
  state, never a fabricated semantic result and never a crash (the port pins this as
  RETR-AC-5).
- **The performance floor is a gate** — the keyword path meets
  [[acceptance-standards#PERF-3]] as a merge-blocking CI benchmark; the harness this
  chapter ships also measures the context-graph budget [[acceptance-standards#PERF-7]] and
  emits the ADR-0021 graph-store trigger evidence.

## Acceptance

Done means an operator can check the promises without reading code: a search in one
workspace never shows another workspace's records; a result can always be traced to the
record it cites; a plain-language question comes back cited or honestly unsure; pulling
the model client away leaves keyword search working and says so. The engine passes the
retrieval port's contract-compliance suite (RETR-AC-1..5 — owned by
[retrieval-seam](retrieval-seam.md), not restated here), and this feature's two ceded
criteria — the NL-plan eval band and the full-text latency budget (AC6.3, AC6.4) — are
pinned verbatim in the Acceptance appendix. The cross-cutting floor (standard screen
states, performance budgets, release gates) is inherited from the acceptance-standards
chapter and not restated. The model-bound quality bands ride the eval catalog
([[ai-evals#AIEVAL-15]], [[ai-evals#AIEVAL-16]], [[ai-evals#AIEVAL-17]]; use case AIUC-20)
— owned there, cited here.

## Out of scope

- **The port contract** — owned by [retrieval-seam](retrieval-seam.md); this chapter
  implements it.
- **Context assembly** — walking related records into evidence-carrying candidate sets is
  owned by [context-graph](context-graph.md); it reuses this chapter's embedding store and
  confidence formula but not the engine's query path.
- **The Ask-AI screen** — the screen-to-story index assigns it to the BYO-agent stories,
  so its screen ACs belong to the byo-agent-and-mcp chapter, not here; this chapter has no
  dedicated stories and claims no screens. The surfaces that render search results own
  their own screen ACs.
- **Summaries and draft replies** — the other half of this feature is owned by
  [drafting](drafting.md), which cites this chapter for grounding.
- **Model injection and routing** — owned by the ai-runtime chapter.

## Where it lives

The engine lives in the search module directory behind the retrieval port; embeddings flow
through the model port. Read [retrieval-seam](retrieval-seam.md) for the contract callers
see, [context-graph](context-graph.md) for the assembly capability beside it, and the
architecture chapter for the seam inventory and import matrix.

## Appendix

### Parameters
Source: specs/spec/contract/formulas-and-rules.md#1072-retrievalconfidence-ranking-b-ep0520b @ 5a0b29c; specs/spec/product/build-backlog/EP05.md#e-hybrid-retrieval-engine-crm-search @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| SEARCH-PARAM-1 | `W_RANK_SIM` | `0.60` | Similarity weight in the retrieval/confidence ranking (SEARCH-FORM-1). |
| SEARCH-PARAM-2 | `W_RANK_REC` | `0.30` | Recency weight in the retrieval/confidence ranking. |
| SEARCH-PARAM-3 | `W_RANK_TRUST` | `0.10` | Source-trust weight in the retrieval/confidence ranking. |
| SEARCH-PARAM-4 | Default embedding model | `bge-m3` | Self-hostable default on the injected model client; BYOK optional; no bundled embedder (ADR-0022 §6, ADR-0020). |
| SEARCH-PARAM-5 | Reranker default | off | Optional cross-encoder stage after fusion; per-query toggle; stays off until recall gains earn its latency (ADR-0022 open question, B-EP05.18). |
| SEARCH-PARAM-6 | Vector benchmark target | recall@10 ≥ 0.95, p95 ≤ 150 ms at 1M vectors | HNSW acceptance target for the embedding store; a calibration starting value, tightened if the nonfunctional budget is stricter (B-EP05.16). |

Note SEARCH-PARAM-N-1 (registry gap, honest): §10.7 declares all its weights "registered
in §0", but the `W_RANK_*` rows are absent from the §0 parameter-registry table
@ 5a0b29c. Their pinned home is §10.7.2 (mirrored here); the §0 registration is
ticket-time corpus repair, not a spec ambiguity — the values are unambiguous.

### Formulas
Source: specs/spec/contract/formulas-and-rules.md#1072-retrievalconfidence-ranking-b-ep0520b @ 5a0b29c

**SEARCH-FORM-1 — Retrieval/confidence ranking (§10.7.2, verbatim).**
Inputs: per-candidate `similarity`, `recency`, `source_trust`, each normalized 0–1.

```
score = W_RANK_SIM·similarity + W_RANK_REC·recency + W_RANK_TRUST·source_trust
```

Output: `score` in 0–1. Tie-break: deterministic, by `id` ascending. Tunables:
`W_RANK_SIM=0.60 / W_RANK_REC=0.30 / W_RANK_TRUST=0.10` (SEARCH-PARAM-1..3).
Worked example: **none in the corpus @ 5a0b29c** — §10.7.2 ships weights and tie-break
only; the golden-number fixture that would serve as the worked example is ticket-time
work (its reproducibility requirement is pinned as consumer acceptance in
[context-graph](context-graph.md) CG-AC-4/CG-AC-6). The normalization of the three
inputs is likewise not specified beyond the poc doctrine and must be fixed by the
implementing ticket's golden test.

**SEARCH-FORM-2 — Rank fusion (cited, not owned).** The fusion of record is reciprocal
rank fusion of the keyword and semantic rankings (ADR-0022 §6; B-EP05.18). The corpus
pins **no fusion constant** @ 5a0b29c; the normative pin is behavioral — the fused
ranking equals a golden reference on seeded data (SEARCH-AC-2) — so the constant chosen
at ticket time is frozen by that test, not by this chapter.

### Schema
Source: specs/spec/contract/data-model.md#12-deferred-tables-later-phases--stubs-only-no-ddl @ 5a0b29c

This chapter owns one table, currently a **deferred stub** — the ownership index carries
it as owner-on-arrival ([[data-model#DM-DEF-3]]); no DDL exists in the corpus and none is
invented here.

**SEARCH-SCHEMA-1 — `embedding` (deferred stub, pinned note).** The corpus stub
@ 5a0b29c: a pgvector store for the AI substrate / context-graph retrieval, sketched as
`(entity_type, entity_id, model, embedding vector(N), chunk, created_at)` with an HNSW
index as the default (~30× IVFFlat at equal recall, R2). The dedicated table lands with
this chapter's retrieval work, **not** the core-objects migration; the context-graph
*capability* is V1 regardless (ADR-0007/ADR-0021). Right-to-deletion must purge a
subject's embeddings (SEARCH-AC-8). Ticket-time DDL obligations the stub does not spell
out: a workspace column so the HNSW index applies tenant as a **pre-filter**, never a
post-filter ([[operations#OPS-STOR-2]]); RLS per the data-model conventions (the port's
RETR-AC-1 is the teeth); at-rest encryption inherited as customer data (B-EP05.16).

**SEARCH-SCHEMA-2 — full-text columns (sanctioned note).** The keyword arm's tsvector
columns and indexes are added by this chapter's migrations **onto tables owned by other
chapters** (core objects and activities, per the [[data-model]] ownership index); the
column mechanics ride those tables' chapters, the search behavior over them is pinned
here (B-EP05.15).

### Wire
Source: specs/spec/contract/crm.yaml (path `/search`) @ 5a0b29c

Operations are cited by contract `operationId` — request/response shapes live in the
contract, never restated here.

| ID | operationId | Operation | Tier | Errors / headers of note |
|---|---|---|---|---|
| SEARCH-WIRE-1 | `search` | Cross-object hybrid search over people, organizations, deals, activities and leads; query plus optional type restriction, cursor-paginated; results RBAC-scoped, leads a distinct result type | 🟢 `search_records` | 401; 422 |

Note SEARCH-WIRE-N-1 (contract gap, honest): the **conversational NL-search surface has
no operation** in the contract @ 5a0b29c — no ask/answer `operationId` exists, and the
NL-report plan endpoint belongs to reporting, not here. The cited-answer layer this
chapter owns (AC6.3, RETR-AC-3) must gain its contract operation at ticket time; this
chapter owns it when it lands. The context-assembly entry point is deliberately **not**
HTTP — it is reached through the retrieval port ([context-graph](context-graph.md)).

### Events
Source: specs/spec/contract/events.md#43-consumer-groups @ 5a0b29c

Event definitions live in the central catalog ([[event-bus]]) — cited here, not
redefined. The engine **emits nothing** @ 5a0b29c.

| ID | Consumer group / event | Role here |
|---|---|---|
| [[event-bus#EVT-CG-1]] | `cg:context-graph` — person, organization, deal, activity, lead streams | Consumed for pgvector **(re)embedding**: entity mutations queue embedding jobs on the injected model client. The group is shared with the AI/graph consumers per the catalog row. Re-embeds are content-hash-guarded and job-queue-batched so a redelivery is a no-op, not a re-spend ([[operations#OPS-STOR-3]], [[operations#OPS-STOR-4]]). |

### Acceptance
Source: specs/spec/features/02-capture-and-comms.md#feature-6--baseline-ai-comms-summaries-draft-replies-nl-search @ 5a0b29c; specs/spec/product/build-backlog/EP05.md#e-hybrid-retrieval-engine-crm-search @ 5a0b29c

Feature ACs verbatim from the feature spec — AC6.3/AC6.4 are the NL-search half of
Feature 6, ceded to this chapter by [drafting](drafting.md) (which pins AC6.1/6.2/6.5/6.6).
The port-contract criteria RETR-AC-1..5 are owned by [retrieval-seam](retrieval-seam.md)
and inherited via SEARCH-AC-1, not restated. No screen ACs are claimed: the Ask-AI screen
maps to S-E10.1/.3/.5 per the screen→story index (product/30-screen-acceptance.md
@ 5a0b29c) and belongs to byo-agent-and-mcp.

| ID | Given/When/Then | Verification |
|---|---|---|
| AC6.3 | **(ML eval, not a deterministic gate — NL→query is model-bound.)** NL search compiles utterances to validated query plans, measured against a **defined eval set** of labeled utterances with a flaky-aware pass threshold (KPI, tracked with a regression band; result varies with model version/prompt). NL→SQL equivalence is *not* a general deterministic property and is not claimed as one. **What is a real gate:** for any *given* compiled query, executing it returns a result equal to the hand-written reference on the seeded DB (golden-number correctness of the executed plan) *(deterministic test on a fixed compiled query)*, and an ambiguous/out-of-vocabulary utterance returns a clarifying prompt, never an unflagged wrong answer. | Eval band [[ai-evals#AIEVAL-15]] (plan-correctness ≥ 90%); deterministic gates [[ai-evals#AIEVAL-16]] (executed-plan equality = 100%) and [[ai-evals#AIEVAL-17]] (unflagged-wrong-answer = 0); use case AIUC-20 |
| AC6.4 | NL search server-side latency **< 200 ms p95** (search budget, §3.5) for full-text path. | Performance gate — [[acceptance-standards#PERF-3]] CI benchmark |
| SEARCH-AC-1 | Given the engine bound behind the retrieval port, when the port's contract-compliance suite runs against it, then it passes RETR-AC-1 through RETR-AC-5 (workspace scoping, citation + tier labels, cited-or-honest-refusal, port-only access, honest degradation) unchanged from the fake. | Port contract-compliance suite ([[retrieval-seam]]); integration lane ([[testing#TEST-LANE-2]]) |
| SEARCH-AC-2 | Given seeded data with a golden fused ranking, when the hybrid query runs both arms and fuses them, then the fused ranking equals the reference; toggling the optional reranker does not break the response contract; the fused query's server-side p95 stays within the search budget. | Golden-set integration test; PERF-3 benchmark (B-EP05.18) |
| SEARCH-AC-3 | Given the sovereign profile, when embedding generation is attempted with a non-local model client, then it is refused **before** any model call and no byte leaves the process; embeddings otherwise flow only through the injected client and both the self-hostable default and BYOK bindings pass. | Denied-path + network-isolation tests; static check for no bundled inference endpoint (B-EP05.17, ADR-0020/0022) |
| SEARCH-AC-4 | Given no model client bound, when a search runs, then keyword results are served and the semantic arm's absence is a declared degraded state — never a fabricated semantic result, never a crash. | Unit lane against the unbound composition ([[testing#TEST-LANE-1]]; RETR-AC-5 shape) |
| SEARCH-AC-5 | Given a source record whose content hash is unchanged, when its entity event is redelivered, then no re-embedding occurs (no model spend); given changed content, exactly one re-embed job runs. | Integration lane; idempotency per [[operations#OPS-STOR-3]]/[[operations#OPS-STOR-4]] |
| SEARCH-AC-6 | Given the embedding store seeded to 1M vectors, when the vector benchmark runs, then recall@10 ≥ 0.95 and p95 ≤ 150 ms (SEARCH-PARAM-6), and embeddings are verified at-rest-encrypted as customer data. | CI benchmark + encryption-inheritance test (B-EP05.16) |
| SEARCH-AC-7 | Given the volume-tier benchmark harness, when it runs the canonical queries over the seeded tiers (incl. mid-market), then it records p50/p95/p99, gates the build **red** on a [[acceptance-standards#PERF-3]] or [[acceptance-standards#PERF-7]] breach, and emits the ADR-0021 graph-store trigger evidence; a seeded breach fixture proves the gate is real. | CI harness (B-EP05.21a/b); consumed by [context-graph](context-graph.md) CG-AC-7 |
| SEARCH-AC-8 | Given a right-to-deletion request for a subject, when erasure executes, then the subject's embeddings are purged with the rest of their data. | Integration lane with [[gdpr-platform]]; [[data-model#DM-DEF-3]] stub note |
