---
status: skeleton
module: backend/internal/shared/ports/retrieval
derives-from:
  - margince-poc/docs/subsystems/retrieval.md @ a11d6c08
---
# Retrieval seam — one narrow door to search, whatever engine sits behind it

> The port every retrieval caller goes through: ask for records by keyword **or** meaning and get one
> ranked, workspace-scoped, citation-carrying result list — or ask in plain language and get a
> **cited** answer or an honest "I'm not sure", never a guess. This chapter owns the port's contract;
> the search engine behind it is planned work owned by
> [search-and-retrieval](search-and-retrieval.md).

## What it's for

The AI layer, agents, and the product's search surfaces all need to find the right person, org, deal,
activity or lead without knowing how search works. The port gives them one narrow vocabulary — a
ranked search over the workspace's records, and a context-assembly entry point for gathering related
evidence — and keeps everything behind it swappable: the AI layer never touches search internals
(ADR-0007; the port is ARCH-SEAM-5 in the architecture chapter's seam inventory). The skeleton ships
the port and its contract; the engine that fulfils it returns as a feature.

## Principles it serves

- **P4 — blazing fast.** The port's keyword path inherits the pinned search budget
  ([[acceptance-standards#PERF-3]]) — a budget on whatever implementation is bound, not on one engine.
- **P6 — embrace the LLMs.** The port is shaped for grounded, cited answers, not bare row lists — its
  contract is what keeps an AI answer anchored to real records.
- **P7 — own your data.** Nothing in the port's contract requires external inference; an
  implementation degrades to keyword-only rather than reaching outside.
- **P12 — governance designed in.** Every query is workspace-scoped at the database; every result
  carries its source's identity and trust tier; answers cite their sources.

## How it works

The port defines what any implementation must guarantee, not how it ranks:

- **Two entry points.** A ranked search — one query in, one deterministic ordered result list out —
  and a context-assembly call that gathers the related records an AI caller needs as evidence.
- **Every result is a citation.** A result carries the identity of the record it came from, so a
  caller can always show its user where a claim came from; results also carry their source's trust
  tier, so captured, untrusted content arrives labeled as data, never as instructions
  ([[threat-model#T2]]).
- **Workspace scoping is the database's job.** Whatever the implementation, queries run inside the
  tenant-isolating path; a cross-tenant query returns nothing.
- **Cited-or-honest-refusal.** An answer built over the port either cites resolvable source records or
  comes back as a clarifying question or a declared "not sure" — the no-guess rule
  ([[acceptance-standards#GATE-AI-1]]).
- **Honest absence.** When no engine is bound, or the bound engine is degraded, callers receive a
  declared honest state — empty-with-a-reason, never a fabricated success and never a crash.

The proven reference implementation — the poc's hybrid engine, with a full-text keyword arm, a
vector-similarity semantic arm, deterministic reciprocal-rank fusion of the two rankings, and a
natural-language layer that compiles questions to a validated query plan deterministically rather
than via an LLM — is the documented shape the planned engine re-ships. That behaviour, its indexing
and embedding machinery, and its tunables are owned by [search-and-retrieval](search-and-retrieval.md)
and are not pinned here.

## What's configurable

- **The bound implementation** — injected at the composition edge: the real engine when its feature
  lands, a fake in tests, nothing at all in the bare skeleton. Callers observe the difference only as
  the declared honest states above.
- **Engine knobs** — embedding model choice, sovereignty enforcement, reranking, embedding dedup —
  belong to the engine, and are specified by [search-and-retrieval](search-and-retrieval.md).

## Guarantees (enforced)

- **Workspace isolation** — row-level security scopes every query regardless of implementation; a
  cross-tenant query returns nothing (the data-model chapter owns the mechanics).
- **Trust tiers survive retrieval** — a result's trust tier travels with it, so downstream AI treats
  captured content as data, never instructions ([[threat-model#T2]]; the tier-leak check
  [[threat-model#TM-VERIFY-2]] is the teeth).
- **No-guess** — ambiguous input yields a clarifier, never an unflagged wrong answer; answers cite
  sources or are absent ([[acceptance-standards#GATE-AI-1]]).
- **One way in** — everything above the engine reaches it only through this port; the AI layer never
  imports search internals (ADR-0007) — held by the architecture lint over the allowed-import matrix
  ([[quality-gates#QG-9]]; the row is [[architecture#ARCH-IMPORT-6]]).

## Acceptance

Done, for this chapter, means the port and its contract exist and any implementation is held to them:
a caller can be written today against the port and tested against a fake; that fake and every future
engine pass the same contract-compliance suite — workspace scoping, tier labels on results,
cited-or-honest-refusal, honest absence. The surface truth is checkable by an operator: a search in
one workspace never shows another workspace's records, and an AI answer either shows its sources or
says it is not sure. The testable form of each claim is pinned in the Acceptance appendix; the
cross-cutting floor (standard screen states, performance budgets, release gates) is inherited from the
acceptance-standards chapter and not restated.

## Out of scope

The engine itself — indexing, embeddings, rank fusion, the conversational answer layer, reranking,
sovereignty enforcement on model calls, and the volume-tier benchmark harness — is planned work owned
by [search-and-retrieval](search-and-retrieval.md). The context-graph walking behind the
context-assembly entry point is owned by [context-graph](context-graph.md); model injection is owned
by [ai-runtime](ai-runtime.md).

## Where it lives

The port at `backend/internal/shared/ports/retrieval` (Tier 0); the planned engine behind it lives in
`backend/internal/modules/search`. Read the architecture chapter for the seam inventory and import
matrix, and [search-and-retrieval](search-and-retrieval.md) for the engine.

## Appendix

### Acceptance
Source: margince-poc/docs/subsystems/retrieval.md#guarantees-enforced-not-aspirational @ a11d6c08

| ID | Given/When/Then | Verification |
|---|---|---|
| RETR-AC-1 | Given records in two workspaces, when any implementation of the port runs a query under the first workspace's context, then only that workspace's rows return and a cross-tenant query returns nothing. | Port contract-compliance suite against every binding; integration lane RLS assertions ([[testing#TEST-LANE-2]]); store-path gate ([[quality-gates#QG-13]]). |
| RETR-AC-2 | Given any result returned through the port, when a caller inspects it, then it carries the identity of its source record (the citation) and that record's trust tier; captured T2 content is never unlabeled. | Port contract-compliance suite, unit lane ([[testing#TEST-LANE-1]]); tier-leak check [[threat-model#TM-VERIFY-2]]. |
| RETR-AC-3 | Given a natural-language question over the port, when an answer is produced, then it cites resolvable source records — and given an ambiguous or out-of-vocabulary question, the return is a clarifying question or declared uncertainty, never an uncited claim. | Contract-compliance suite, unit lane ([[testing#TEST-LANE-1]]); the evidence-or-omit gate [[acceptance-standards#GATE-AI-1]] when AI surfaces land. |
| RETR-AC-4 | Given any caller above the engine, when it retrieves, then it imports only the retrieval port — the AI layer holds no import of search internals (ADR-0007). | Architecture lint over the allowed-import matrix, merge-blocking ([[quality-gates#QG-9]], row [[architecture#ARCH-IMPORT-6]]). |
| RETR-AC-5 | Given no engine bound (or a degraded one), when a caller queries the port, then it receives a declared honest state — empty with a stated reason — never a fabricated result and never a crash. | Unit lane against the fake and the unbound composition ([[testing#TEST-LANE-1]]). |
