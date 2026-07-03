# ADR-0021 — Relational + pgvector is the context-graph substrate; a graph store is trigger-gated

**Status:** Accepted (Lars, 2026-06-17). Research parent: [`../../research/r2-pgvector-vs-graph-store.md`](../../research/runtime/r2-pgvector-vs-graph-store.md) (closes BACKLOG R2). **Amends** [ADR-0007](ADR-0007-context-graph-is-v1-substrate.md) (names the trigger its deferral left open). Sharpens, does not unwind, the §6.1 performance-budget regime ([`06-nonfunctional.md`](../narrative/06-nonfunctional.md), P4) and the egress/residency posture ([ADR-0011](ADR-0011-consent-and-retention.md), A24).

## Context

[ADR-0007](ADR-0007-context-graph-is-v1-substrate.md) made the context graph a V1 **capability** (capture→link assembly + cross-pipeline reasoning) on one PostgreSQL instance — real FKs + the typed `relationship` table + `tsvector` + `pgvector` — and **deferred** a dedicated graph *datastore* with the words "add a graph store only if multi-hop reasoning at scale proves insufficient." That deferral named **no trigger**, leaving a standing open question in `03 §3.6`, `03b` L3, and `06 §6.7` — a permanent "we'll know it when we see it" that R2 was scoped to close.

R2 establishes two things. First, the **committed workload is shallow and bounded**: the brief ranking (`formulas-and-rules.md §10`), stalled detection (§8), relationship-strength (§4), and warm-room (§9) queries are fixed-depth (1–3 hop) indexed joins with aggregation, and the `relationship` table encodes only `employment` and `deal_stakeholder` — a near-bipartite edge set. The graph-shaped patterns that favor a native engine (N-degree path-finding, community detection, centrality) are **not in scope**. Second, **published benchmarks put the crossover ~2 orders of magnitude away**: recursive CTEs run in microseconds at depth 2–3 / <10k nodes and only degrade at depth 6 / ~500k nodes / millions of edges, while `pgvector` (HNSW) serves 50M vectors at p95 60 ms / 99% recall. Adding a datastore is a one-way operational cost (P1) with no offsetting query-performance gain at our scale.

## Decision

**Relational + `pgvector` on one PostgreSQL instance is the context-graph substrate for V1 and beyond. A dedicated graph store is not roadmapped; it is adopted only if a named, measured trigger fires.**

1. **Confirm the substrate.** The context-graph capability runs on the relational core + `pgvector` (HNSW default) + `tsvector`, in the single Postgres instance — no second datastore. This is now the *confirmed* substrate, not a provisional starting point.
2. **The trigger conditions (any one opens the graph-store ADR).** Measured on reference hardware **after** index/cache/precompute tuning is exhausted:
   - **(a) Latency:** the context-graph assembly query (`PERF-7`, §6.1) p95 **> 300 ms** at the mid-market tier (250k–1M contacts).
   - **(b) Depth:** a *committed* feature requires **variable-depth traversal ≥ 3 hops** where a recursive CTE fails `PERF-7` at the mid-market tier.
   - **(c) Fan-out:** a *committed* query exhibits whole-graph combinatorial fan-out that cannot meet `PERF-7` despite tuning.
   - **(d) Volume:** a workspace crosses **~5M `relationship` edges or ~50M `activity_link` edges** *and* the harness shows recursive-CTE p95 exceeding `PERF-7`.
3. **The bar is an irreducible breach, not a forecast.** A trigger counts only after the mitigation ladder (covering indexes, Redis read-model caching, River-decoupled precompute, partitioning, `pgvectorscale`) is exhausted. Speculative "we might want graph queries" never fires it.
4. **A benchmark harness makes the trigger operational.** The `crm-search` work package ships a CI benchmark that seeds the §6.7 volume tiers, runs the canonical queries, records p50/p95/p99, and gates on `PERF-3`/`PERF-7` (P4). Its output is the evidence that confirms the substrate or justifies the graph-store ADR.
5. **Thresholds are calibration starting values.** The 300 ms / 5M-edge numbers are first-principles, to be calibrated by the harness at WP build (like the `api-rate-limits-and-abuse.md` defaults); a calibration change is an ADR-noted budget revision, not a silent bump.

## Consequences

- **Positive:** turns ADR-0007's open-ended deferral into a **monitored SLO** (`PERF-7` + R-C6), so "do we need a graph store" is answered by a dashboard, not re-litigated each planning round; keeps the single-datastore operational simplicity that the own-your-data / EU-residency posture (A24) and the small-team principle (P1) both depend on; the harness makes the decision falsifiable.
- **Negative / costs:** a future N-degree-network feature (warm-intro paths, org-hierarchy roll-up — today `[TS]`) would trip trigger (b) and carry a real datastore-adoption cost; the thresholds are unvalidated until the harness runs, so they may move; maintaining a second mental model ("when does this become a graph problem") is a small standing tax on the `crm-search` owners.
- **Open questions (→ `07-risks.md` R-C6):** final `PERF-7` threshold pending harness calibration; whether `pgvectorscale` (vs stock `pgvector`) becomes the default is a `crm-search`-WP detail, not load-bearing here.

## What this does NOT change (guardrails)

- The context-graph **capability** and its V1 scope ([ADR-0007](ADR-0007-context-graph-is-v1-substrate.md)) — unchanged; this only names the datastore trigger ADR-0007 deferred.
- The `data-model.md §5` `relationship` table and its indexes, and the `§12` `embedding`/`pgvector` table — unchanged; they are the substrate this confirms.
- The §6.1 performance-budget / CI-merge-gate regime (P4) and the §6.7 volume tiers — reinforced; `PERF-7` is one more gate in the same regime.
- The egress location ladder / EU-residency default (A8 revised, A24/ADR-0011) — reinforced by keeping inference and graph data in one governed Postgres, no third datastore to place.

## Follow-ups

- `03-architecture.md` — §3.6 open question + §3.1 candidate divergence: "separate ADR (pending)" → "resolved → ADR-0021 (trigger-gated)"; `crm-search` module comment cites the trigger.
- `03b-ai-architecture.md` L3 — add the named trigger + ADR-0021 cross-reference to the graph-store deferral sentence.
- `06-nonfunctional.md` — add `PERF-7` (context-graph assembly p95 < 300 ms at mid-market) to §6.1; rewrite the §6.7 "decision deferred" bullet to "resolved, trigger-monitored (ADR-0021)."
- `07-risks.md` — add R-C6 (graph-store deferral with measurable revisit criteria).
- `data-model.md §12`, `glossary.md` — add ADR-0021 alongside the ADR-0007 pointer.
- `DECISIONS.md` — new A29 entry; `BACKLOG.md` — close R2; `README.md` — session-pickup entry.
