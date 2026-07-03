# ADR-0014 — Module boundaries are mechanically enforced, not conventional

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md) (2026-06-11, architecture-blueprint research phase). Synthesizes research tracks T2 (`foundation/research/t2-go-boundaries.md`) and T3 (`t3-agent-parallel.md`), verified in `foundation/research/verification-log.md`. Refines `03-architecture.md §3.3`. Serves Goal 3 (parallel independent agent work) and Goal 1 ("wow").

## Context

ADR-0001 locks a single modular-monolith Go binary. The module list (`crm-core`, `crm-capture`, `crm-ai`, `crm-agents`, `crm-search`, `crm-auth`, `crm-contracts`) exists in prose, but nothing *stops* hidden coupling. For a build performed by a parallel fleet of AI agents (Goal 3) and by client agents on forks (Goal 2), "remember not to import across modules" is exactly the discipline that fails silently. Boundaries must be enforced by the toolchain and CI, or they are decorative.

## Decision

**1. One Go module, seven `internal/`-bearing subtrees.** Use a single `go.mod` for the CRM. Each module's non-seam code lives under its own `internal/`, which the Go compiler *refuses* to import across module roots — an unbypassable, zero-config spine. Reserve `go.work` for the CRM↔Dispact↔`@gradion/contracts` seam, not for fencing the seven CRM modules (multi-module adds `replace`/version foot-guns for parallel agents without adding boundary strength). Promote a module to its own `go.mod` only where a fork story (T5) demands independent versioning.

**2. A dependency-free seam-interface leaf layer.** Cross-module contracts (`crmctx`, `sor`, `mcp`, `connector`, `workflow`, `model`) live in dependency-free interface packages that every module may import and no implementation crosses. Modules communicate only through these seams. `crm-agents` sits above `crm-core` and depends on `sor`; the reverse is forbidden. Intent tools reach context through a seam, never by importing `crm-search`.

**3. Three enforcement layers, two of them merge-blocking CI gates.**
- **Layer A — Go `internal/` (compiler):** the structural spine.
- **Layer B — `depguard` in `golangci-lint`:** per-module file-glob `allow`/`deny` import rules with reason strings; denied import → non-zero exit → PR fails. (Verified behavior.)
- **Layer C — `fe3dback/go-arch-lint`:** the seven-module dependency DAG declared once in `.go-arch-lint.yml` (`components` + `deps.mayDependOn`); `check` exits 1 on any forbidden edge, giving a legible, single-file picture of the architecture. CI additionally asserts zero `notMatched` packages so a new package can't silently escape the rules.

**4. Boundaries are legible in the source itself** (Goal 1): the DAG is one readable YAML file; cross-module calls visibly go through typed seams; `internal/` makes "what is private" obvious on sight.

## Consequences

- **Positive:** an agent *cannot* create cross-module coupling by accident — the compiler or a CI gate stops it. The architecture is impressive because the boundaries are mechanically true, not aspirational.
- **Known gaps (carried as named work items, not solved here):** import linters protect the *code* seam only. They do **not** catch *data*-seam coupling (two modules sharing a DB table, Redis key, or event name) — which would contradict "each module owns its schema namespace." A separate schema-namespace/event-name CI check is required (see blueprint E and ADR-0018's RLS gate). A missing depguard rule = silent allow; CI must assert rule coverage.
- **Amendment 1 (2026-06-23, deep red-team) — the data-seam gate is a concrete ownership-manifest check (closes RT-AR-M10).** Since Postgres "schema namespaces" are conceptual (one `crm` schema, `data-model §1`), the data-seam gate is an **ownership manifest**: a checked-in `module-ownership.yaml` maps every table, Redis key prefix, and `events.md` event `type` to exactly one owning module. CI (a) parses each module's migrations and asserts it only `CREATE/ALTER`s tables it owns; (b) asserts each module only publishes event `type`s and `XADD`s to stream keys it owns; (c) fails on any table/key/event with zero or >1 owners. This is the concrete form of the "separate schema-namespace/event-name CI check" named above. Owning story: build-backlog platform epic (see RT-fix sweep).
- **Boundary:** this ADR governs *coupling* boundaries. *Trust/capability* boundaries are ADR-0018. The seam *evolution* rules (how a seam changes without breaking forks) are ADR-0017.
- **Amended by [ADR-0042](ADR-0042-jurisdiction-packs.md) (2026-06-24, A57):** jurisdiction packs (`crm-de`, …) are a recognized application of §1's "own `go.mod` … where a fork story demands it" promotion — a satellite module (tied via `go.work` like the Dispact seam) that implements the Tier-0 `jurisdiction` seam and registers via `init()`. The DAG (Layer C) gains three edges: **core ↛ pack**, **pack ↛ pack**, **only `cmd/*` → pack**; the ownership manifest (Amendment 1) assigns each pack its own tables/events/keys; and a new "no jurisdiction strings in core" fitness function complements the import gates.
