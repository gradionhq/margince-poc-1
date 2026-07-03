# ADR-0019 — HTTP layer: chi + stdlib `net/http` (Mat-Ryer shape), diverging from Dispact's Gin

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md) (2026-06-11, architecture-blueprint research phase) — **awaiting Lars sign-off.** Amends ADR-0001 (which lists "Go 1.26 + Gin"). Per ADR-0001's own rule — *"every divergence from Dispact's stack requires its own ADR"* — this records the one stack-detail divergence the blueprint introduces. Built on T1 research (`foundation/research/t1-wow-codebases.md`), ADR-0016 (repo/docs conventions), and ADR-0015 (contract pipeline).

## Context

ADR-0001 adopts Dispact's stack by default for suite consistency (P9), naming **Gin** as the HTTP framework. The architecture-blueprint research phase then converged — independently, across three threads — on a different HTTP idiom for the CRM:

1. **T1 (admired Go codebases)** found the **Mat-Ryer "How I write HTTP services in Go" shape** — `NewServer(deps…) http.Handler`, a `routes()` method, handlers as closures/methods with explicit dependencies, and a thin `main()`→`run(ctx) error` — to be the readability/testability pattern that makes single-binary Go repos (pocketbase, caddy, the broader stdlib-first community) legible on sight. This is load-bearing for Goal 1 ("wow") and P3 (agent-readable).
2. **ADR-0015 (contract pipeline)** wires **`oapi-codegen`** to generate the Go server against the **chi / stdlib `http.Handler`** interface, and doc 02 generates the route-registration file — both of which assume the stdlib `http.Handler` contract, not Gin's `gin.Context`.
3. The prior `gradion-crm` reference prototype was **Go / Chi** (ADR-0001 notes it as "a reference, not a fork").

Leaving Gin in ADR-0001 while the blueprint, the codegen, and the conventions all assume chi/stdlib is a silent, unreconciled divergence — exactly what ADR-0001's divergence rule exists to prevent (surfaced as must-fix #1 in `architecture/VERIFICATION.md`).

## Decision

**For the CRM, the HTTP layer is `go-chi/chi` over the standard library `net/http` `http.Handler`, in the Mat-Ryer service shape. This diverges, deliberately and explicitly, from Dispact's Gin.**

- **Router/handler contract:** stdlib `net/http` (`http.Handler`, `http.HandlerFunc`, `*http.Request`, `http.ResponseWriter`) with `chi` for routing, middleware, and URL params. No `gin.Context`; handlers take the stdlib pair.
- **Service shape (the convention, ADR-0016 / doc 11):** `NewServer(deps…) http.Handler` constructs the handler tree with explicit dependencies; `routes()` (a **generated** file, doc 02) registers them; `main()` is a thin wrapper over `run(ctx context.Context) error`. Handlers are methods/closures over the injected seams, not globals.
- **Codegen target (ADR-0015):** `oapi-codegen` emits the server interface in its chi/std-`http` mode; the generated route table and the `imports_gen.go` manifest assume this contract.
- **Scope of the divergence:** the HTTP router/handler idiom **only**. Everything else in ADR-0001's stack is unchanged — Go 1.26, River, PostgreSQL 16 + pgvector, Redis 7 + Streams, S3/MinIO, `gw-auth`, the React 19 frontend, `@gradion/contracts`, D13/K8s/Jenkins, the quality gates. The modular-monolith architecture (ADR-0001, ADR-0014) is untouched: still one `go.mod`, one binary, `cmd/` the sole composition root.

## Rationale

- **Goal 1 / P3 (agent-readable):** the Mat-Ryer shape with explicit-deps `NewServer` is the idiom T1 found that makes the source legible and testable by humans and AI agents — handlers have no hidden globals, dependencies are visible at the constructor, and the wiring is one readable file.
- **Contract-first fit:** `oapi-codegen` → stdlib `http.Handler` is the most direct path from `crm.yaml` to typed Go handlers and to the generated route manifest (doc 02 / ADR-0015); Gin would add a framework-specific adapter layer between the contract and the handler.
- **Lower lock-in, easier testing:** stdlib `http.Handler` is trivially testable with `httptest` and carries no framework coupling; it is the substrate `chi` middleware composes over without owning the handler signature.
- **Reference-prototype continuity:** the validated `gradion-crm` spike was already Chi-based.

## Consequences

- **Negative / the cost being paid:** a real divergence from Dispact (P9 suite consistency) — two HTTP idioms across the suite, so a developer moving Dispact↔CRM learns two routing styles, and any shared HTTP middleware would need a stdlib-vs-Gin adapter. This is the price; it is bounded to the HTTP edge.
- **Low-cost and reversible:** the Mat-Ryer *shape* is router-agnostic, and `oapi-codegen` also supports a Gin target. If suite consistency is judged to outweigh the above, the blueprint can target Gin with **no structural change** — the DAG, seam layer, registries, and generated-manifest decisions (ADR-0014/0015, docs 01/02) are identical either way. Only the handler signature and the codegen mode flip.
- **Ripple if accepted:** ADR-0001's "Backend: Go 1.26 + Gin" line is superseded by this ADR for the CRM; ADR-0001 Amendment-proposal 1 is replaced by the back-reference to this ADR; docs 11/13 and ADR-0015 drop their "proposed default / fallback = Gin" hedging and read chi/std-`http` as decided.
- **Boundary:** this ADR is about the HTTP router/handler idiom only. It changes nothing about the governed MCP surface (ADR-0013), the agent runtime (ADR-0005/0009), or any data/AI decision.

## Decision required from Lars

Accept the chi / stdlib `net/http` divergence for the CRM (recommended — the blueprint, codegen, and reference prototype already assume it), **or** hold the CRM to Gin for suite consistency with Dispact (in which case the blueprints retarget Gin with no structural change). Until decided, the blueprint reads chi/std-`http` as the proposed default with Gin as the no-rework fallback.
