---
derives-from:
  - margince-poc/docs/architecture/code-organization.md
  - architecture/01-module-dag.md#allowed-import-rules
  - architecture/02-composition-and-registries.md#the-composition-root-stays-thin
  - architecture/11-conventions.md#2-naming-conventions
  - architecture/14-jurisdiction-packs.md#repo-layout
---
# Code organization — where code lives, and how to add a feature

This answers the two questions a doc should answer before you write any code: **"where
does my code go?"** and **"what are the steps to add a feature, end to end?"** It points
to the detail chapters for each step instead of repeating them. Read
[architecture.md](architecture.md) first for the module map and the boundary rules —
this guide is the practical "how to work inside that map."

## Where code lives

The short version, in words (the exact table is pinned as CODEORG-LOC-1…12):

- **A cross-module interface (a port)** goes in its own dependency-free package under
  `backend/internal/shared/ports/`, named for the concept it carries.
- **Domain logic and data access for an area** go in the owning module under
  `backend/internal/modules/<name>/` — entities and business rules in its `domain`
  folder, use cases in `app`, database and external implementations in `adapters`.
  The starting roster: `people` (persons — organizations, deals, activities, and leads
  join as the domain grows), `identity` (identity/RBAC), `search`, `ai`, `capture`,
  `agents`, and focused modules such as audit-adjacent GDPR, approvals, import,
  export, and voice as each area earns its own home.
- **An HTTP or tool (MCP) endpoint** is a handler in the owning module's `transport`
  folder, plus the endpoint in the contract (`backend/api/crm.yaml`). Cross-cutting
  handlers that belong to no module live with the composition root.
- **Shared technical plumbing** — config, database pool, HTTP server chassis, logger,
  events/jobs, auth middleware — lives under `backend/internal/platform/`. It is
  never a home for domain logic.
- **Shared domain-neutral vocabulary** — ID types, Money, pagination, the
  acting-principal context, provenance — lives in `backend/internal/shared/kernel/`;
  the typed error taxonomy in `backend/internal/shared/apperrors/`.
- **A database schema change** is a migration pair under `backend/migrations/`
  (golang-migrate), created via the migration make target.
- **Frontend code** goes under `frontend/src/features/<name>/` — each feature owns its
  `api`, `components`, `hooks`, and `routes`; cross-feature pieces go in
  `frontend/src/shared/`; the generated API client is `frontend/src/lib/api-client`.
- **Country-specific code** (German invoicing, GoBD retention, …) goes in the
  jurisdiction pack — a separate Go module beside `backend/` — **never** in a core
  module. <!-- 1c-mapping: final call pending — provisional home: a separate Go module beside backend/, per ADR-0042. -->
- **Generated code** (contract types, wiring manifests, route tables) is never
  hand-edited — regenerate it.
- **`backend/pkg/`** exists only for genuinely reusable code an external consumer may
  import. Treat needing it as a design smell to justify, not a default.

Most new work is **a file (or a few) inside an existing module**, not a new module or
Go module.

## Inside a module

Every module directory has the same shape, so an agent landing cold knows where the
next line of code goes: `domain` holds the entities and invariants and imports nothing
but the kernel; `app` holds the use cases that orchestrate them; `ports` holds the
module's own narrow interfaces (what *it* needs from the outside); `adapters` holds
the implementations — repositories against the database, clients against external
systems; `transport` holds the HTTP and MCP handlers, which stay thin and delegate to
`app`; and `module.go` at the module root is the manifest that wires the module into
the composition root. One concept per file, named for the concept.

## How to add a feature — the end-to-end path

The common case: a new capability with an API, data, and a screen. Do the steps in
this order (pinned as CODEORG-STEP-0…8); each links its detail chapter. If the feature
needs two modules to talk, step 0 comes first.

**0. (Only if it crosses a module boundary) Agree the port first.** If, say, the agent
surface needs new behavior from the people module, add or extend the interface under
`backend/internal/shared/ports/` and agree its shape **before** anyone codes. This is
the one step that must be done before parallel work — everything after it parallelizes
safely. See [architecture.md](architecture.md).

1. **Contract first.** Add the endpoint and any new request/response shapes to
   `backend/api/crm.yaml`, then regenerate types (`make gen-types`). The contract is
   the source of truth; the Go and TypeScript types come from it. → contract-pipeline,
   api-conventions chapters.

2. **Schema.** Add a migration under `backend/migrations/` for any new tables or
   columns. Every table carries the workspace key and row-level security. → data-model
   chapter.

3. **Domain logic + data access.** Write it in the owning module — rules in `domain`,
   the use case in `app`, the repository in `adapters`. Database access goes through
   the workspace-scoped transaction helper (the store-path gate enforces this — never
   the raw pool; pinned as CODEORG-RULE-2). Record an audit row and provenance where
   the rules require it. → data-model, audit-observability chapters.

4. **The endpoint.** Add the handler in the module's `transport` folder. Return errors
   as typed sentinels and let the centralized mapper produce the problem+json body;
   honor If-Match / version-skew per the conventions. → api-conventions chapter.

5. **Wiring.** Register the feature through the module manifest; the aggregating
   import manifest and route table are **generated** — don't hand-edit them;
   regenerate and let the drift gate confirm. Adding a feature never edits the
   composition root itself. → contract-pipeline chapter.

6. **Frontend.** Add the screens and components under
   `frontend/src/features/<name>/`. → frontend chapter.

7. **Tests.** Unit tests (no database) and integration tests (real database) go in
   their separate lanes — a unit test must not open a database, and an integration
   test fails loudly rather than skipping when its database is missing
   (CODEORG-RULE-3). → testing chapter.

8. **Green the gates.** Run the standard check target; the build then runs the
   integration tests and review. → quality-gates chapter.

## How to add a module, a port, or a Go module

These are progressively rarer. Pick the smallest one that fits:

- **A new module** — the usual case for "a new area of logic." Create
  `backend/internal/modules/<name>/` with the standard internal shape, keeping files
  to one concept each under the file-size cap (CODEORG-RULE-1); follow the
  file/declaration layout in the craftsmanship chapter (type → constructor → methods
  together, doc the exported symbols, accept interfaces / return structs). Register
  the new directory in the architecture-lint rules, listing which **ports** it may
  depend on (production cross-layer edges are checked — keep it minimal), and declare
  any new external vendor explicitly — there is no blanket vendor allow, so an
  unscoped import fails the lint. Never add an incumbent SDK to `ai` or `agents`.
  **Keep modules focused:** when a cohesive sub-area grows inside a module — a new
  noun, or a self-contained engine — give it its own module rather than piling on.
- **A new port (a Tier-0 interface)** — only when a *genuine* new boundary between two
  modules appears (module A must call module B's behavior). Put the interface under
  `backend/internal/shared/ports/`, keep it dependency-free, add it to the
  allowed-import rules, and note it in [architecture.md](architecture.md). Agree it
  before parallel work starts. A purely technical seam (no domain vocabulary) belongs
  under `backend/internal/platform/` instead.
- **A new Go module (its own go.mod)** — rare, and deliberate. Reserved for
  jurisdiction packs and, where a tool genuinely needs isolation, standalone CLIs.
  Don't split the backend into more Go modules — it is one Go module on purpose, so
  its packages share one compiler unit and the boundary rules stay compile-checked.
  See [architecture.md](architecture.md) and the jurisdiction-packs chapter.

## The one rule that keeps the structure clean

**A module reaches another module's behavior only through a port under
`shared/ports/` — never by importing its internals.** The architecture lint turns
crossing that line into a build failure, not a code-review nag. If you find yourself
wanting to import another module's internals, that is the signal to use (or add) a
port instead. This single rule is what lets many agents build in parallel without
stepping on each other.

## Generated files

Generated artifacts — contract types from `backend/api/crm.yaml`, wiring/import
manifests, route tables — carry a generated-file header and are never edited by hand
(CODEORG-RULE-4). An agent that needs one changed edits the *source of generation* and
regenerates; the drift gate fails the build if a generated file is stale or was
hand-edited. Conflict resolution on a generated manifest is "re-run the generator,"
never a hand-merge.

## Error handling

Domain errors are **typed sentinels from `shared/apperrors`** — not-found, conflict,
scope-exceeded, requires-approval, version-skew, budget-exceeded — so callers branch
on them reliably. Never return a bare string error where a caller must distinguish the
case. Wrap with context as you cross layers, preserving the sentinel so it stays
branchable at the boundary. Handlers never hand-write an error body: they return the
sentinel and the centralized mapper translates it to the problem+json (or MCP
tool-error) shape. Adding a new sentinel is rare and must be earned — the bar is that
a caller would otherwise have to string-match to branch correctly — and it lands with
its HTTP and MCP mapping in the same change.

## Context and tenancy

Every request context carries the acting principal — tenant, actor, Passport — set by
the HTTP middleware in `platform/httpserver` and read only through the typed kernel
accessors. Domain code never reads identity from headers directly, never stashes a
context on a struct, and takes the context as the first argument of anything that does
I/O or crosses a trust boundary. The workspace key read from the principal is what the
workspace-scoped transaction helper binds for row-level security.

## Common commands

The `Makefile` at the repo root is the task runner: one make target per golden command
— format, vet, lint, unit and integration test lanes, the architecture lint, the
file-length guard, contract generation and its drift check, build and run. The full
catalog is pinned below (CODEORG-CMD-1…12); the toolchain, the lint ruleset, and the
Go file/declaration-layout conventions are the code-quality operating model — see the
craftsmanship chapter.

## Appendix

### Where code lives
Source: margince-poc/docs/architecture/code-organization.md#where-code-lives @ 5a0b29c

| ID | What you're writing | Where it goes | Naming |
|---|---|---|---|
| CODEORG-LOC-1 | A cross-module interface (a port) | `backend/internal/shared/ports/<seam>/` (dependency-free — Tier 0) | the port's concept, e.g. `datasource`, `retrieval` |
| CODEORG-LOC-2 | Domain entities & rules for an area | `backend/internal/modules/<name>/domain/` | one concept per file, e.g. `forecast.go`, `deal_gate.go` |
| CODEORG-LOC-3 | Use cases / orchestration | `backend/internal/modules/<name>/app/` | verb-named use cases |
| CODEORG-LOC-4 | DB repositories / external clients | `backend/internal/modules/<name>/adapters/` | named for the system adapted |
| CODEORG-LOC-5 | An HTTP or tool (MCP) endpoint | `backend/internal/modules/<name>/transport/` + the endpoint in `backend/api/crm.yaml` | `handler_<thing>.go`, e.g. `handler_deal.go` |
| CODEORG-LOC-6 | Shared technical plumbing | `backend/internal/platform/{config,database,httpserver,logger,events,auth}` | technical concept = package name |
| CODEORG-LOC-7 | Cross-module domain-neutral types / errors | `backend/internal/shared/kernel/` · `backend/internal/shared/apperrors/` | kernel: ID types, Money, pagination, principal ctx, provenance |
| CODEORG-LOC-8 | A database schema change | a migration pair in `backend/migrations/` (`make migrate-create NAME=add_foo`) | `NNNNNN_add_foo.up.sql` / `.down.sql` |
| CODEORG-LOC-9 | Frontend (screens, components) | `frontend/src/features/<name>/{api,components,hooks,routes}`; cross-feature in `frontend/src/shared/`; generated client in `frontend/src/lib/api-client` | see the frontend chapter |
| CODEORG-LOC-10 | Country-specific code | the jurisdiction pack — a separate Go module beside `backend/` (e.g. `jurisdictions/de/`) — **never** a core module <!-- 1c-mapping: final call pending --> | see the jurisdiction-packs chapter |
| CODEORG-LOC-11 | Generated code (contract types, wiring) | **don't hand-edit** — regenerate (CODEORG-RULE-4) | `*_gen.go` + generated-file header |
| CODEORG-LOC-12 | Genuinely reusable externally-importable code | `backend/pkg/` (rare; justify it) | — |

The ratified tree, for orientation:

```
go.work · Makefile · docker-compose.yml · .github/workflows/
backend/
  cmd/{api,worker,migrate}/            # entrypoints; cmd/api = monolith composition root
  internal/
    modules/<name>/                    # bounded contexts: people, identity, deals, …
      domain/ app/ ports/ adapters/ transport/ module.go
    platform/{config,database,httpserver,logger,events,auth}
    shared/{kernel,apperrors,ports/}
  migrations/                          # golang-migrate
  api/crm.yaml                         # OpenAPI source of truth
  pkg/                                 # only genuinely reusable code
frontend/
  src/{app,features/<name>/{api,components,hooks,routes},shared,lib/api-client}
jurisdictions/de/                      # own go.mod  <!-- 1c-mapping: final call pending -->
```

### Add-a-feature step order
Source: margince-poc/docs/architecture/code-organization.md#how-to-add-a-feature @ 5a0b29c

| ID | Step | Rule |
|---|---|---|
| CODEORG-STEP-0 | Agree the port first (only if the feature crosses a module boundary) | shape agreed in `shared/ports/` before anyone codes; the one pre-parallel step |
| CODEORG-STEP-1 | Contract first | endpoint + shapes into `backend/api/crm.yaml`; `make gen-types`; generated types never hand-edited |
| CODEORG-STEP-2 | Schema | migration in `backend/migrations/`; every table carries the workspace key + RLS |
| CODEORG-STEP-3 | Domain logic + data access | in the owning module (`domain`/`app`/`adapters`); DB only via the workspace-scoped tx helper (CODEORG-RULE-2); audit + provenance where required |
| CODEORG-STEP-4 | The endpoint | handler in the module's `transport/`; sentinel errors + centralized problem+json mapping; If-Match handling per api-conventions |
| CODEORG-STEP-5 | Wiring | register via `module.go`; import manifest + routes are generated — regenerate, never hand-merge; composition root untouched |
| CODEORG-STEP-6 | Frontend | screens/components under `frontend/src/features/<name>/` |
| CODEORG-STEP-7 | Tests | unit lane (no DB) and integration lane (real DB) kept separate; integration tests fail, never skip (CODEORG-RULE-3) |
| CODEORG-STEP-8 | Green the gates | `make check`, then integration tests + review |

### Rules
Source: margince-poc/docs/architecture/code-organization.md#how-to-add-a-package-a-seam-or-a-module @ 5a0b29c

| ID | Rule |
|---|---|
| CODEORG-RULE-1 | File-size cap: no Go file over ~500 LOC; enforced by `make go-file-length` (merge-blocking god-file guard). Split by concept, not by line count alone. <!-- reconcile: corpus 11-conventions.md §3 fixes ~400–500 lines as "documented guidance, not a lint"; the shipped poc promoted it to a merge-blocking gate. The shipped gate is kept. --> |
| CODEORG-RULE-2 | Transaction rule: all database access goes through the workspace-scoped transaction helper (poc: `withWorkspaceTx`) which binds the tenant key for RLS — never the raw pool. Enforced by the store-path gate (poc: `rls-store-path`), merge-blocking. |
| CODEORG-RULE-3 | An integration test with no test-database URL fails (`t.Fatal`), never skips — a skipped test passes silently; the integration runner always provisions the database. |
| CODEORG-RULE-4 | Generated files (`*_gen.go`, routes, import manifests, contract types) carry a generated-file header and are never hand-edited; the drift gate (`make gen-types-check` and manifest checks) blocks stale or hand-edited output. |

### Commands
Source: margince-poc/docs/architecture/code-organization.md#common-commands @ 5a0b29c

| ID | Command | Does |
|---|---|---|
| CODEORG-CMD-1 | `make tidy` | go work sync + go mod tidy for all Go modules |
| CODEORG-CMD-2 | `make fmt` | gofumpt all Go files (stricter than gofmt) |
| CODEORG-CMD-3 | `make vet` | go vet all Go modules |
| CODEORG-CMD-4 | `make lint` | golangci-lint — the project ruleset (merge-blocking) |
| CODEORG-CMD-5 | `make go-file-length` | fail any Go file over the LOC cap (CODEORG-RULE-1) |
| CODEORG-CMD-6 | `make test` | unit tests only (no integration build tag) |
| CODEORG-CMD-7 | `make test-integration` | integration tests (needs dev infra + test DB up) |
| CODEORG-CMD-8 | `make arch-lint` | enforce the module DAG (merge-blocking; ARCH-IMPORT-*) |
| CODEORG-CMD-9 | `make build` / `make run` | build / run the `cmd/api` server |
| CODEORG-CMD-10 | `make gen-types` | regenerate Go + TS types from `backend/api/crm.yaml` |
| CODEORG-CMD-11 | `make gen-types-check` | drift gate — fail on stale generated types (merge-blocking) |
| CODEORG-CMD-12 | `make migrate-create NAME=<x>` | create a migration pair under `backend/migrations/` |
