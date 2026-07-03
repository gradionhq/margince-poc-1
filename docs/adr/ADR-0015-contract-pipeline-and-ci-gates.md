# ADR-0015 — Contract-first codegen pipeline, generated registries, and CI gates

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md) (2026-06-11, architecture-blueprint research phase). Synthesizes T4 (`foundation/research/t4-contract-codegen.md`) and the T3 manifest correction (F-T3a in `verification-log.md`). Refines `03-architecture.md §3.1/§3.5` and `contracts/README.md`. Serves Goals 1 and 3.

## Context

The contract (`crm.yaml`) is the source of truth for the Go server and TS client. The pipeline that turns it into code — and the CI gates that keep generated code, the contract, and performance honest — *is* architecture: it is what makes "the tests are green" trustworthy for an agent-built, fork-customized system. Two non-obvious facts shaped the decision (both verified against primary sources).

## Decision

**1. Author OpenAPI 3.1; overlay down to 3.0 for the Go generator only.** `oapi-codegen`/`kin-openapi` and `ogen` cannot parse 3.1 `type: [T, null]` (kin-openapi issue #230 "v3.1 Soon!"; ogen #1410) — **verified**. So `crm.yaml` stays authoritative 3.1, and a generate-time OpenAPI Overlay rewrites `type:[T,null]` → `nullable:true` feeding **`oapi-codegen`** (chi/std-interface mode) for Go. The TS client uses **`openapi-typescript` v7 + `openapi-fetch`** directly (3.1-native since PR #968 — **verified**, no overlay). Both publish through `@gradion/contracts`. A bespoke generator walks `x-mcp-tool` to emit the `crm-agents` MCP tool list from the same contract. **Contract stays hand-written 3.1 YAML — TypeSpec rejected for V1** (added toolchain without a V1 payoff).

**2. All central lists are generated, never hand-edited (the F-T3a correction).** Go `init()`-based self-registration only runs if the package is blank-imported into the binary, so the import list is itself a shared, merge-prone file (Caddy ships exactly such a file, `modules/standard/imports.go` — **verified counter-evidence**). Therefore: connectors, MCP tools, workflow handlers, and per-module route registration are **scaffolded one-file-per-entry and the aggregating manifest (`imports_gen.go`, route tables) is code-generated** (`crm gen` scans `seams/*`, `connectors/*`, …). Conflict resolution on a generated manifest is "re-run gen," never a hand-merge. This is what actually delivers the no-shared-manifest property for Goal 3; the hand-maintained `routes.go` that T1 admired is generated instead (resolves contradiction F-X1).

**3. Drift is a merge-blocking gate.** `go generate ./...` then `git diff --exit-code` over all Go/TS/MCP/manifest artifacts fails the PR on any divergence between committed code and the contract. A response-conformance contract test catches code-vs-contract *behavioral* drift. Trustworthy only because every generator is version-pinned.

**4. Contract linting:** `redocly lint` (3.1) + a custom Spectral ruleset encoding `contracts/README.md` conventions (plural resources, RFC-7807 `code`, provenance-on-create, cursor envelope, well-formed `x-mcp-tool`).

**5. Performance budgets (`§3.5`) are gated, but correctly scoped.** k6 percentile thresholds for the HTTP rows + Go `benchstat` for cold-start, made deterministic by: fixed runner class, `--with-demo` fixed-clock seed at defined volume tiers, testcontainers PG16+Redis7 at pinned digests, warmup-excluded steady state, and interleaved ≥10-iteration baseline-vs-PR comparison with a tolerance band. **The hard gate runs nightly / on an isolated runner** (per-commit p95 in shared CI is too noisy — correction F-T4); per-commit perf is advisory. The AI "first token < 1.5s" row is an **observability SLO, not a hard gate** (model-bound, non-hermetic).

## Consequences

- **Positive:** one contract drives server + client + MCP tools + docs; drift is impossible to merge; adding a connector/tool/workflow touches only new files; perf regressions are caught without flaky red builds.
- **Negative / bound:** the 3.1→3.0 overlay is real complexity living in the build — accepted because the source contract stays modern and the cost is one generate-time transform. Revisit when oapi-codegen ships native 3.1.
- **Boundary:** the breaking-change *policy* over the contract (what `oasdiff` severity gates a release) belongs to ADR-0017; this ADR only wires `oasdiff` as the detection mechanism.
