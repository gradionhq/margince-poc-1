# ADR-0004 — Reporting read-path (the analytics-in-Go question)

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md)

> Closes review finding **F-6** (`REVIEW-stage6.md` §2.5, §5): reporting integrity is a top-3 differentiator (`02` §2 #3; `features/03`), but ADR-0001 adopted Dispact's Go stack for suite-consistency (P9) reasons and **never weighed Go's analytics/query-layer ecosystem gap** against the most demanding workload in the product. This ADR makes that weighing explicit and decides the read-path. It amends, but does not overturn, ADR-0001.

## Context

`features/03` commits to a reporting surface that is, in data-engineering terms, demanding: cross-object aggregates over real FKs and the `relationship` table, funnels, `deal_stage_history` "as-of-date" snapshots, period-over-period, NL→validated-query-plan, materialized read models, "Explain This Number" lineage/drill-through, and `<300–800 ms` p95 on a seed dataset of **1M activities / 100k people / 20k orgs / 5k deals** (`features/03` budgets). Correctness is itself a budget (`AC-X1`): fast-but-wrong is a failure.

**The ecosystem reality this ADR must face honestly:**

- Go has **no mature analytical-query / semantic-layer / query-builder ecosystem** comparable to the JVM (Apache Calcite for query planning/optimization/SQL-dialect translation), Python (the pandas/Arrow/Ibis/dbt world), or even Node. There is no Go Calcite. There is no Go semantic layer (Cube/LookML-equivalent) we can adopt.
- The implication ADR-0001 left unstated: **we hand-build, in Go, a typed query-plan compiler, lineage tracking, and a safe NL→query layer from scratch.** That is real, sized work, not a library import. It was listed nowhere in ADR-0001's rationale or its "Negative" section. F-6 is correct.
- Two further constraints bound the solution space:
  - **P11 (clean relational core)** and ADR-0001's "no metadata engine on the hot path" must survive. The reporting path must not reintroduce a dynamic-schema interpreter.
  - **P7 (own your data; on-prem; single-binary; fully local inference for regulated clients)** means any engine we adopt must run inside the single Go binary / docker-compose / air-gapped footprint, or it is a non-starter for the dedicated/on-prem and source-delivered modes (`06` §6.3).
  - **P2 (custom objects are code → real tables)** means client forks add columns/tables/joins the reporting path must handle generically without per-client engine work (ties to F-14: the customization perf-linter).

### Options considered

**Option A — Typed query-plan builder over Postgres + materialized read models + careful indexing.**
A Go-native, contract-bound query-plan IR (select / filter / group / aggregate / join over the *enumerated* core + code-defined objects), compiled to parameterized Postgres SQL. Heavy/repeated aggregates are served from materialized read models (the Redis-cached aggregates already in `features/03` budgets, plus Postgres materialized views / summary tables refreshed off the audit/event stream and `deal_stage_history`). Lineage is a property of the plan (every aggregate carries its filter+group+source-row definition → "Explain This Number" for free).

- **For:** stays entirely inside Postgres 16 + the existing stack — zero new runtime dependency, preserves P7 single-binary/on-prem/air-gap, preserves P11 (honest SQL over real indexes), reuses ADR-0001 infra. The plan IR is the *same* surface NL and BYO-agents compile to (`features/03` §1.3) — one schema-bound abstraction, not three. Lineage is structural. Custom objects (P2) are just more real tables the planner already handles.
- **Against:** we build the query-plan compiler, the materialization/refresh orchestration, and lineage tracking ourselves in Go. Postgres is a capable analytical engine for this dataset size with proper indexing + materialized views, but it is not a columnar OLAP engine; very wide ad-hoc scans on the largest tables (1M+ activities, multi-join, cache-miss) are where it is weakest and where the `<1.5 s` cache-miss budget is at most risk.

**Option B — Embed an analytical engine (DuckDB) for heavy aggregates.**
Keep Postgres as system-of-record; run heavy/columnar aggregates through an embedded DuckDB (in-process, can query Postgres via the `postgres` scanner or read Parquet snapshots).

- **For:** DuckDB is genuinely excellent at exactly the large-scan columnar aggregates Postgres is weakest at; embeddable (preserves single-binary/on-prem/P7); could make the cache-miss budget comfortable.
- **Against:** **two query engines = two correctness surfaces** — `AC-X1` ("no fast-but-wrong numbers") now has to hold across both, and a number that disagrees between the Postgres path and the DuckDB path is the exact failure mode this product is built to eliminate. Data freshness/sync between SoR and the columnar copy is a new, permanent consistency problem. Go's DuckDB bindings are CGO (complicates the cross-platform single-binary story). It is a second thing to operate, secure (RBAC must be enforced identically on both paths), and reason about. This is real complexity bought against a budget Option A can likely meet.

**Option C — Separate read service (CQRS / dedicated reporting service, possibly non-Go).**
Split reporting into its own deployable, optionally on a stack with a mature analytics ecosystem (JVM+Calcite, or a warehouse).

- **For:** access to the mature ecosystem the review notes Go lacks; independent scaling.
- **Against:** **breaks the modular-monolith / single-binary / on-prem / air-gapped story (P7) head-on** — a separate service (especially a non-Go or warehouse one) is the opposite of "the whole thing runs on the client's infrastructure." It also fragments the suite (anti-P9) and reintroduces eventual-consistency and a second RBAC enforcement point. Disproportionate for the v1 dataset scale.

## Decision

**Adopt Option A: a typed, contract-bound query-plan builder over Postgres 16, backed by materialized read models and disciplined indexing. Reject an embedded second engine (B) and a separate read service (C) for v1.** Treat the cache-miss large-scan budget as the explicit risk to monitor, with a pre-decided escalation path to Option B *scoped to specific heavy aggregates only*.

Concretely:

1. **Query-plan IR is the single read-path abstraction.** One typed Go IR (`reporting/queryplan`) over the enumerated core objects + code-defined custom objects (P2). It compiles to parameterized SQL. **NL reporting and BYO-agents target this same IR — never free-form SQL** (this is also the F-1 / NL-safety answer, below). There is no open SQL console and no dynamic-schema interpreter; the planner only emits joins/aggregates the contract declares (preserves P1/P11).
2. **Materialized read models for hot/repeated aggregates.** Postgres materialized views + summary/rollup tables (e.g. per-stage pipeline rollups, per-owner activity counts) refreshed off the append-only audit/event stream and `deal_stage_history`; Redis caches the dashboard-widget results (`features/03` dashboard budget). Ad-hoc cache-miss reports hit the live planner under the `<1.5 s` refresh budget.
3. **Lineage is a property of the plan, not a bolt-on.** Every aggregate carries its filter+group+aggregate definition and a drill-through query to source rows → `AC-R6` "Explain This Number" and `AC-X1` reconciliation are structural, not separately built.
4. **Indexing is a committed, tested artifact.** The index set required to meet the `features/03` p95 budgets on the seed dataset is part of the schema and asserted in the benchmark harness (depends on G11 harness existing). Custom-object reporting (P2) inherits the same index-coverage discipline — this is the seam the F-14 customization perf-linter must guard.
5. **Pre-decided escalation, not a rebuild.** If, against the seed-dataset benchmark, cache-miss large-scan aggregates cannot meet budget with Postgres tuning + materialization, escalate to Option B **for those named aggregates only**, behind the *same* query-plan IR (the IR makes the engine swappable per-aggregate without changing the NL/agent/UI surface). This requires its own follow-up ADR and must carry a dual-path golden-number test (the Postgres path remains the correctness oracle).

### NL→query safety / determinism (addresses the F-1 / `AC-R5` concern)

The NL path is **not NL→SQL.** The model compiles an utterance to the typed query-plan IR, which is validated against the contract before any execution and **shown to the user before it runs** (`features/03` §1.3). Consequences that make this safe and bounded:

- The model **cannot emit SQL, invent joins, reach tables outside the contract, or bypass RBAC** — it can only produce a plan the validator accepts, and the validator only accepts contract-declared objects/fields/joins. NL→SQL-injection and "wrong silent answer" are structurally excluded.
- Determinism is honest: a *validated plan* is deterministic and re-runnable/saveable as a normal report; the *NL→plan compilation step* is a model call and is therefore an **eval, not a deterministic gate** (this is the F-10 re-labeling — `AC-R5` is a ≥90% eval against a reference set, not a build gate). Ambiguous utterances return a clarifying prompt, never a silent plan.
- RBAC is enforced **at plan execution on the Postgres path**, identically for human, NL, and BYO-agent callers (`AC-R8`) — one enforcement point, which is also why Option C (a second engine with its own RBAC) was rejected.

## Consequences

- **Positive:** No new runtime dependency; P7 single-binary/on-prem/air-gap and P9 suite-consistency intact; P11/P1 honest-SQL-no-metadata-engine intact; one IR serves UI + NL + BYO-agent (`features/03`); lineage and `AC-X1` reconciliation are structural; custom objects report identically to core (P2). The Go-ecosystem gap is addressed not by importing a missing library but by **scoping the build to a typed plan-compiler over the relational core we already control** — which is tractable precisely because the schema is static (P11).
- **Negative / honest costs (the ones ADR-0001 omitted):** we build and own a query-plan compiler, materialization/refresh orchestration, and lineage tracking in Go — real engineering, sized below. Postgres is not columnar; the cache-miss large-scan budget is the genuine risk and may force the Option-B escalation (a second correctness surface) for specific aggregates. Materialized-view refresh adds operational surface (staleness windows, refresh cost) that the dashboard freshness story must specify.
- **Honest build sizing (v1, order-of-magnitude, not a project plan — feeds G10/G20):**
  - Query-plan IR + validator + SQL compiler over core objects: **substantial** — the core net-new component; the single biggest reporting-specific build.
  - Materialized read models + refresh orchestration (off audit/event stream + `deal_stage_history`) + Redis widget cache: **moderate**, but the operational/freshness design is non-trivial.
  - Lineage / drill-through ("Explain This Number"): **low-moderate** *if* built into the IR from day one; expensive if retrofitted — so it is a day-one requirement, not a fast-follow.
  - NL→plan compiler + eval harness + clarifying-prompt path: **moderate**, and gated on the G11 eval-set/harness existing.
  - Benchmark harness + seed dataset + committed index set (shared with G11): **moderate**, and a prerequisite for calling any `features/03` p95 a CI gate.
- **Dependencies / unblocks:** requires the G11 seed dataset + benchmark harness to exist before any reporting p95 is a real gate; the Option-B escalation requires a follow-up ADR; the custom-fork perf story is F-14 (promote the perf-linter from backlog).
- **Relation to ADR-0001:** amends its "Negative"/"Deferred" sections — the reporting/analytics-ecosystem gap is now a named, sized, decided cost rather than an undisclosed one. ADR-0001's "stay REST, defer GraphQL for reporting reads" stands: the typed query-plan IR over REST is the reporting read-path; GraphQL is not needed and not adopted.
