---
derives-from:
  - margince-poc/docs/architecture/architecture.md
  - architecture/01-module-dag.md#the-three-tiers
  - architecture/02-composition-and-registries.md#the-decision-one-file-per-entry--a-generated-manifest
  - architecture/11-conventions.md#2-naming-conventions
  - architecture/14-jurisdiction-packs.md#repo-layout
---
# Architecture — the modular monolith and its enforced seams

> The structural map for the product. The boundaries here are **mechanically enforced**
> (compile errors + merge-blocking lint), not conventions. ADR ids are cited as provenance.

## In plain terms — what kind of architecture this is

Margince is a **modular monolith**. The backend ships as a single server program — the
`backend/cmd/api` binary — with two small companion binaries for the async job fleet
(`backend/cmd/worker`) and schema migration (`backend/cmd/migrate`). Inside that one
program the code is divided into **modules with strict, enforced boundaries**: a module
never reaches into another module's internals; it calls the other only through a small
interface called a **port** (a "seam") — the standard **ports-and-adapters** pattern,
where the shared port packages are the ports and the owning module (or, in overlay
mode, an incumbent CRM adapter) is the adapter behind the data port.

The backend is a single Go module. Each bounded context — people, identity, deals,
search, ai, capture, agents, and so on — is a directory under
`backend/internal/modules/`, and each cohesive concern stays its own module so no
module becomes a catch-all: when a new cohesive area grows, it gets its own module
directory rather than bloating an existing one. Shared *technical* plumbing that owns
no domain knowledge — configuration, the database pool, the HTTP server chassis,
logging, the event/outbox machinery, auth middleware — lives under
`backend/internal/platform/`. Shared *domain-neutral* vocabulary — ID types, Money,
pagination, the acting-principal context, provenance — lives under
`backend/internal/shared/kernel/`, the typed error taxonomy under
`backend/internal/shared/apperrors/`, and every cross-module port interface under
`backend/internal/shared/ports/`.

Keeping modules apart behind ports is what lets many coding agents build in parallel
without colliding: they agree a port's shape up front, then work on either side of it
independently, and crossing a boundary is a **compile error or a merge-blocking lint
failure**, not a matter of discipline. The sections below give the precise version —
the layout, the boundary rules, and the port inventory.

## The repository, from the top

The repo root holds `go.work` (ties the Go modules together), the `Makefile` (the task
runner — every golden command is a make target), `docker-compose.yml` (dev
infrastructure: Postgres+pgvector, Redis), and `.github/workflows/` (the merge-blocking
gates). Under it:

- **`backend/`** — one Go module: entrypoints under `cmd/`, all application code under
  `internal/`, migrations under `backend/migrations/` (golang-migrate), and the OpenAPI
  contract at `backend/api/crm.yaml` — the source of truth every generated Go and
  TypeScript type derives from. `backend/pkg/` exists only for genuinely reusable code
  that external consumers may import; it is deliberately near-empty.
  <!-- reconcile: corpus 11-conventions.md §2 rejected a pkg/ directory outright ("no pkg/ — rejected, T1"); the ratified target layout reinstates a narrow backend/pkg/ for genuinely reusable code. The ratified layout wins here. -->
- **Jurisdiction packs** — country-specific code (the German pack first) is a
  **separate Go module beside `backend/`** (`jurisdictions/de/`), per ADR-0042, so its
  dependency graph cannot leak into the core: a pack cannot import module internals
  because the module graph makes it a compile error.
  <!-- 1c-mapping: final call pending — provisional home: a separate Go module beside backend/, per ADR-0042. -->
- **`frontend/`** — the React edge. It consumes generated TypeScript contract types
  only, never Go source.

Every module directory under `backend/internal/modules/<name>/` has the same internal
shape — `domain` (entities and rules), `app` (use cases), `ports` (the module's own
narrow interfaces), `adapters` (database and external implementations), `transport`
(HTTP/MCP handlers), and a `module.go` manifest that registers the module at the
composition root. [code-organization.md](code-organization.md) is the working guide to
that shape.

### Package naming convention

The directory name is the Go package name: `backend/internal/modules/people/domain` is
`package domain` within the people module, `backend/internal/shared/ports/datasource`
is `package datasource`, `backend/cmd/api` is `package main`. Ports and kernel packages
are named for their concept — short, lower-case, singular.

## The dependency strata (the DAG)

The poc proved a three-tier DAG; the target layout re-expresses the same discipline in
new vocabulary. Read the strata bottom-up — everything may import downward, nothing
imports upward:

- **Tier 0 — the shared leaf (`backend/internal/shared/`).** Dependency-free, no
  implementations: the cross-module **ports** (`shared/ports/` — datasource, model,
  connector, mcp, retrieval, jurisdiction), the **kernel** (`shared/kernel/` — ID
  types (UUIDv7), Money, pagination, the acting-principal context, provenance, trust
  descriptors), and the typed error taxonomy (`shared/apperrors/`). Everyone imports
  Tier 0; Tier 0 imports only the standard library and each other's types.
- **Platform (`backend/internal/platform/`)** — the technical substrate: config,
  database, httpserver, logger, events (outbox + River-backed jobs), auth middleware,
  and the purely technical seams (observability, blob storage, key vault). Platform
  sits beside Tier 0 in the DAG: it imports only `shared/`, and it **never imports a
  module** — it owns no domain knowledge.
- **Tier 1 — domain modules.** `modules/people` (the person slice of the shared domain
  — persons first, organizations/deals/activities/leads join it as the domain grows),
  `modules/identity` (identity/RBAC/JWT/Passport), `modules/search` (tsvector +
  pgvector + context-graph), `modules/ai` (the model client and routing).
- **Tier 2 — surface modules.** `modules/capture` (connectors feeding writes through
  the Sink, as async jobs) and `modules/agents` (the MCP surface, the autonomous-agent
  runner, and the approval/admission gate).
- **Tier 3 — the edge.** `backend/cmd/api` (the composition root — the only place
  implementations and jurisdiction packs are wired), `cmd/worker`, `cmd/migrate`, and
  `frontend/`.

The generated contract types (from `backend/api/crm.yaml` → Go server types + a TS
client under `frontend/src/lib/api-client`) sit beside Tier 0 as a dependency-free
generated artifact every HTTP/MCP edge imports. See the contract-pipeline chapter.

**Cardinal rule:** a module reaches another module's behavior **only through a port in
`shared/ports/`**, never by importing its internals. `agents` sits *above* `people` and
depends on the datasource port; the reverse is forbidden. This is what lets parallel
agents agree a port signature first and then code independently — reaching across is
*impossible*, not merely discouraged. The full allowed-import matrix is pinned in the
appendix (ARCH-IMPORT-1…11).

## The port layer

Cross-module contracts live in **dependency-free interface packages** under
`backend/internal/shared/ports/` that every module may import and no implementation
crosses. If two modules need to share behavior, the right place is a port — not a
direct import.

- **datasource** — the data-access port. *Implemented* by `modules/people` (native
  mode) or an incumbent adapter (overlay mode, P13). Everything above targets only the
  interface, so the same tool surface runs native or overlay unchanged.
  <!-- reconcile: corpus 01-module-dag.md names this seam `sor` (SystemOfRecordProvider); the shipped poc renamed it `datasource` and the rename is deliberate (plain naming over jargon). The poc name is kept. -->
- **model** — the provider-agnostic LLM client (local + cloud).
- **connector** — the capture/enrichment source port; writes flow through the Sink.
- **mcp** — the governed tool surface (paired with `modules/agents`); the single entry
  point (ADR-0013).
- **retrieval** — the narrow retriever port (search + context assembly) so `ai` grounds
  reasoning without importing `search` internals (ADR-0007).
- **jurisdiction** — the port country packs implement (ADR-0042).

Three purely technical seams from the poc — **observability**, **blobstore**, and
**keyvault** — carry no domain vocabulary and therefore live under
`backend/internal/platform/` rather than `shared/ports/`; the async-job seam (the poc's
workflow package, River-backed) folds into `platform/events` for the same reason. The
**audit** seam also lands in platform for now.
<!-- 1c-mapping: final call pending — the audit seam's provisional home is backend/internal/platform/. -->

The kernel sits alongside the ports: the acting-principal context (the poc's appctx —
Principal/Passport/tenant, read uniformly by every module), ID generation (UUIDv7),
provenance (source + captured-by), and trust descriptors live in `shared/kernel/`; the
typed sentinel errors live in `shared/apperrors/`.
<!-- reconcile: corpus 01/11 name the principal-context package `crmctx`; the shipped poc docs call it `appctx`. Same concept — the poc name is kept; the target package name is decided when the kernel lands. -->

## Composition root and registries

The composition root stays thin, and adding a feature never edits it. Each connector,
MCP tool, and job handler is **one new file** in its module that self-registers into
the in-process registry for its kind; the aggregating import manifest and the route
table are **generated, never hand-edited** (ADR-0015) — conflict resolution on a
generated manifest is "re-run the generator," never a hand-merge. Two agents each
adding an entry touch different files; the manifest is regenerated from the union.
`backend/cmd/api` only loads config, constructs the platform infrastructure, imports
the generated manifests so registrations run, and hands the populated registries to
the server. The one deliberate composition-root choice is implementation binding —
which datasource implementation (native vs incumbent adapter) and which jurisdiction
packs link — driven by the deployment profile, not by feature work.

## Non-negotiable structural invariants (merge-blocking gates)

Four invariants are held by gates, not review culture — pinned as ARCH-INV-1…4 in the
appendix:

1. **The module DAG** (ADR-0014): the allowed-import matrix is enforced by the
   architecture lint on every merge.
2. **No jurisdiction strings in core** (ADR-0042): country identifiers live only in
   the jurisdiction pack; three DAG edges are enforced — core never imports a pack
   (compile error, since neither is in the other's module graph), a pack never imports
   another pack, and only the entrypoints under `cmd/` import packs (the compile-time
   switch selecting which packs link into a given binary). A fitness function
   additionally fails the build on a country literal inside `backend/`.
3. **Contract-first** (P3): `backend/api/crm.yaml` is the source of truth; generated
   types are never hand-edited, and a drift gate blocks staleness.
4. **Tenancy, concurrency, provenance, audit** — enforced in the schema; see the
   data-model chapter.

## Frontend — where it sits, and its own layer scale

`frontend/` sits at Tier 3 in the Go DAG — the React edge — and has its own internal
layer model, from atom up to page: design-system atoms and cross-feature domain
components in `frontend/src/shared/`, feature components with their own hooks and
state under `frontend/src/features/<name>/` (each feature owning its `api`,
`components`, `hooks`, and `routes`), and the routing/application shell in
`frontend/src/app/`. The generated API client lives in `frontend/src/lib/api-client`
and is the only way the frontend talks to the backend — it imports generated contract
types only, never Go source.

> **Two layering scales — kept on different words.** The Go DAG uses **Tier 0–3**
> (leaf ports up to the edge); the frontend uses its own **FE layer** scale (atom up
> to page). They are unrelated schemes. Throughout the docs **"Tier" always means the
> Go DAG and "FE layer" always means the frontend scale** (pinned as ARCH-VOCAB-1);
> the frontend chapter is authoritative for the FE layers. Likewise **"module"**
> unqualified always means a bounded context under `backend/internal/modules/`; the
> compilation unit with its own go.mod is always called a **"Go module"**
> (ARCH-VOCAB-2).

All colors, spacing, and typography use design-system tokens — raw hex/px values fail
the design-system purity gate (merge-blocking). The design-system chapter owns the
token, palette, and brand detail; go there for those, not here.

## Testing and wire conventions — pointers, not restatement

Test lanes (contract compliance, Go unit, Go integration against a dedicated test
database, FE unit, headless Storybook) and their gate wiring are owned by the testing
chapter; the fast lanes all run in the standard check target, and integration tests
are deliberately excluded from it because they need live infrastructure. HTTP response
conventions — snake_case fields, create/patch/archive status shapes, problem+json
errors, integer If-Match concurrency — are owned by the api-conventions chapter and
enforced by the response-shape gate; this chapter does not restate them.

## How this serves the goals

- **Parallel agents:** cross-module contact is only through Tier-0 ports, so two
  agents owning `people` and `agents` agree the datasource signature first, then code
  independently. Generated registries (no shared manifest) keep additions from
  colliding.
- **"Wow on sight":** the whole architecture is one readable DAG; `internal/` makes
  private-vs-public obvious; every cross-module call visibly goes through a typed port.
- **Safe forks:** the port layer is the stable, versioned surface a fork codes
  against; internals can change without breaking a fork that only touched ports +
  custom code.

## Appendix

### Layer and vocabulary rules
Source: margince-poc/docs/architecture/architecture.md#web-frontend-module-structure-and-layers @ 5a0b29c

| ID | Rule |
|---|---|
| ARCH-VOCAB-1 | "Tier" always means the Go DAG scale (Tier 0 leaf ports/kernel → Tier 3 edge); "FE layer" always means the frontend's own atom-to-page scale. The two are never mixed: "Tier 3" is the Go edge, not a frontend layer. |
| ARCH-VOCAB-2 | "Module" unqualified means a bounded context under `backend/internal/modules/<name>/`. A compilation unit with its own `go.mod` is always called a "Go module" in full (the backend, each jurisdiction pack). |
| ARCH-VOCAB-3 | "Port" (synonym in older docs: "seam") means a dependency-free cross-module interface package under `backend/internal/shared/ports/`. Purely technical seams live under `backend/internal/platform/` instead. |

### Allowed-import matrix
Source: margince-poc/docs/architecture/architecture.md#allowed-import-rules @ 5a0b29c

Enforced by the architecture lint (`make arch-lint`), merge-blocking (ADR-0014).

| ID | Component | MAY import | MUST NOT import |
|---|---|---|---|
| ARCH-IMPORT-1 | `shared/ports/*`, `shared/kernel`, `shared/apperrors` (Tier 0) | stdlib + each other's *types* (no cycles) | any platform, module, or cmd package |
| ARCH-IMPORT-2 | `platform/*` | `shared/*` | any `modules/<name>`; `cmd/*` |
| ARCH-IMPORT-3 | `modules/people` | `shared/*` (kernel, apperrors; ports: datasource, connector Sink, events/jobs), `platform/*` | `modules/{identity,search,ai,capture,agents}` internals |
| ARCH-IMPORT-4 | `modules/identity` | `shared/*`, `platform/*` | every other module's internals |
| ARCH-IMPORT-5 | `modules/search` | `shared/*`, `platform/*`; embeddings via the model port | `modules/{agents,capture,people}` internals |
| ARCH-IMPORT-6 | `modules/ai` | `shared/*` (model port), search **via the retrieval port only**, `platform/*` | `modules/people` internals, `modules/agents`, `modules/search` internals |
| ARCH-IMPORT-7 | `modules/capture` | `shared/*` (connector + datasource ports; writes through Sink), `platform/*` | `modules/agents`; `modules/people` internals |
| ARCH-IMPORT-8 | `modules/agents` | `shared/*` (datasource, mcp, model ports), identity's Passport via its port, `platform/*` | `modules/people` internals (reach via datasource); incumbent SDKs |
| ARCH-IMPORT-9 | `cmd/api`, `cmd/worker`, `cmd/migrate` | every module + platform **+ enabled jurisdiction packs** (the only place impls and packs are wired) | — |
| ARCH-IMPORT-10 | `frontend/` | generated TS contract types (`frontend/src/lib/api-client`) | any Go internal |
| ARCH-IMPORT-11 | jurisdiction pack (`jurisdictions/de`, …) | `shared/ports/` (jurisdiction, connector, datasource), `shared/kernel`, `shared/apperrors` + its own `internal/` | module internals; **any other pack**; incumbent SDKs |

### Structural invariants
Source: margince-poc/docs/architecture/architecture.md#non-negotiable-structural-invariants @ 5a0b29c

All merge-blocking.

| ID | Invariant | Held by |
|---|---|---|
| ARCH-INV-1 | The module DAG (the ARCH-IMPORT matrix above) holds: modules reach each other only through `shared/ports/`; platform never imports a module; Tier 0 imports nothing above itself. ADR-0014. | architecture lint (`make arch-lint`) |
| ARCH-INV-2 | No jurisdiction strings in core: country identifiers (XRechnung / DATEV / GoBD / eIDAS / ISO-3166 literals) live only in the jurisdiction pack. Three DAG edges: core ↛ pack (compile error — separate Go modules), pack ↛ pack, only `cmd/*` → pack (the compile-time switch selecting which packs link). ADR-0042. | Go module graph + `make fitness-jurisdiction` |
| ARCH-INV-3 | Contract-first (P3): `backend/api/crm.yaml` is the source of truth; generated Go + TS types are never hand-edited. | drift gate (`make gen-types-check`) |
| ARCH-INV-4 | Tenancy, optimistic concurrency, provenance, and audit are enforced in the schema (RLS, version triggers, provenance columns), not in handler discipline. | schema + RLS conformance gates (data-model chapter) |

### Seam inventory
Source: margince-poc/docs/architecture/architecture.md#the-seam-leaf-layer @ 5a0b29c

| ID | Seam (poc name) | Target home | Role |
|---|---|---|---|
| ARCH-SEAM-1 | datasource | `shared/ports/datasource` | data-access port; implemented by `modules/people` (native) or an incumbent adapter (overlay, P13) <!-- reconcile: corpus 01 names this `sor`; shipped name is `datasource` --> |
| ARCH-SEAM-2 | model | `shared/ports/model` | provider-agnostic LLM client (local + cloud) |
| ARCH-SEAM-3 | connector | `shared/ports/connector` | capture/enrichment source port; writes flow through the Sink |
| ARCH-SEAM-4 | mcp | `shared/ports/mcp` | the governed tool surface — the single agent entry point (ADR-0013) |
| ARCH-SEAM-5 | retrieval | `shared/ports/retrieval` | narrow retriever port (search + assemble-context) so ai never imports search internals (ADR-0007) |
| ARCH-SEAM-6 | jurisdiction | `shared/ports/jurisdiction` | the port country packs implement (ADR-0042) |
| ARCH-SEAM-7 | workflow | `platform/events` | async/job seam (River-backed jobs) — purely technical, hence platform |
| ARCH-SEAM-8 | obs | `platform/` (with `platform/logger`) | observability — purely technical |
| ARCH-SEAM-9 | blobstore | `platform/` | blob storage — purely technical |
| ARCH-SEAM-10 | keyvault | `platform/` | secret/key material — purely technical |
| ARCH-SEAM-11 | audit | `platform/` | audit-trail seam <!-- 1c-mapping: final call pending — provisional home internal/platform/ --> |
| ARCH-SEAM-12 | appctx | `shared/kernel` | acting principal / Passport / tenant, read uniformly by every module <!-- reconcile: corpus names it `crmctx`; poc docs say `appctx` --> |
| ARCH-SEAM-13 | ids | `shared/kernel` | UUIDv7 ID generation and typed IDs |
| ARCH-SEAM-14 | prov | `shared/kernel` | provenance (source, captured-by) stamped on every write |
| ARCH-SEAM-15 | trust | `shared/kernel` | trust-artifact descriptors |
| ARCH-SEAM-16 | errs | `shared/apperrors` | typed sentinel error taxonomy with centralized HTTP/MCP mapping |
