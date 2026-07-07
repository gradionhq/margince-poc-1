---
derives-from:
  - margince-poc/docs/architecture/contract-pipeline.md
  - specs/spec/architecture/04-seam-evolution.md#0-the-one-constraint-everything-follows-from
  - specs/spec/architecture/04-seam-evolution.md#1-safe-vs-breaking-per-seam-type
  - specs/spec/architecture/04-seam-evolution.md#2-the-additive-evolution-mechanism-the-primary-tool
  - specs/spec/architecture/04-seam-evolution.md#3-classification-every-exported-type-is-seam-or-sealed-internal
  - specs/spec/architecture/04-seam-evolution.md#4-openapi--http-contract-evolution-stripe-style-oasdiff-gated
---
# Contract pipeline — one contract, generated types, gated evolution

> Contract-first (P3): the OpenAPI contract is the single source of truth for the wire.
> Go and TS types are generated from it and committed; a drift gate fails the merge if
> they disagree, and a breaking-change gate fails the merge if the contract itself
> changes in a way that breaks an existing consumer. Generated files are never
> hand-edited.

## What it's for

Two kinds of consumer depend on the product's shapes: callers of the HTTP/tool surface
(the frontend, agents, integrations), and fork authors who implement the product's
extension seams and must survive upstream upgrades. This chapter owns the machinery
that keeps both safe: the generation chain that turns the one contract into committed
Go and TS types, the two merge-blocking gates that hold the contract and the code in
lockstep, and the evolution rules that define — precisely, per seam type — what
"breaking" means and for whom.

## Principles it serves

- **P3 — contract-first.** The contract is authoritative; when the schema and the code
  disagree, the schema wins and the drift gate fails the merge. The agent tool surface
  is generated from the same document, so there is no second source of truth.
- **P2 — source customization.** Client-added fields surface additively through
  extension markers rather than contract edits, so a customized fork's wire shapes
  stay inside the same contract (CP-EXT-1, CP-EXT-2).
- **ADR-0015 — generated wiring.** The gates and the generated surfaces are wired into
  the standard check suite; regeneration is the only way generated artifacts change.
- **ADR-0017 — fork-upgrade survival.** The seam-evolution rules below are the
  compile-time/contract half of the promise that a client fork survives an upstream
  upgrade.

## How it works

The contract lives at `backend/api/crm.yaml`, a valid OpenAPI 3.1 document specifying
the V1 core (people, organizations, deals, pipelines, stages, activities, leads,
relationships, lists, tags — full CRUD plus archive) and the AI/agent surface
(cold-start, drafting and sending, approvals, search, reports, identity). It is
derived field-for-field from the data model: every component schema mirrors a table,
and names, types, nullability, and enums match the database shapes owned by the
data-model chapter. Nullability uses the OpenAPI 3.1 type-union form, which round-trips
cleanly to Go pointers and TS null-unions.

From that one document, one generation chain produces everything (pinned as
CP-STAGE-1..8). The TS side reads 3.1 directly and emits the types behind the
frontend's api-client. The Go side needs two derived-spec transforms first — a
down-conversion to 3.0, because the Go generator does not consume 3.1, and a
disambiguation pass that renames parameter components which would otherwise collide
with schema components in the flattened Go type namespace. Both transforms operate on
derived copies only; the source contract is never mutated by its own pipeline. The
generated Go and TS types are committed to the tree, sitting beside the shared kernel
as a dependency-free artifact every HTTP and MCP edge imports.

Two merge-blocking gates hold this honest. The **drift gate** regenerates into a
temporary location and diffs against the committed types: any divergence fails with an
instruction to regenerate and commit, so committed types can never go stale relative
to the contract. The **breaking-change gate** compares the base branch's contract
against the working copy and classifies every change by severity; changes classified
as errors block the merge, while additive or deprecation changes pass (CP-BREAK-20).
Both gates degrade loudly, not silently — when a tool or base reference is
unavailable, they print why they skipped.

## What breaking means, and for whom

The evolution rules rest on one verified asymmetry in Go: adding a method to an
interface a fork *implements* breaks that fork — its type silently stops satisfying
the interface and stops compiling — while adding a field to a struct, or a method to a
concrete type, is safe. That single fact splits every exported type into two worlds:

**A seam a fork implements is a frozen contract.** The fork-facing seams — the
connector, tool, datasource-provider, model-client, and workflow-handler ports, plus
the policy strategies — never grow a method, never change a signature, never remove a
member within a major version. New capability arrives as a *sibling* interface that
embeds the old one, and the call site probes for it and falls back — the language's
own extension pattern. A fork that implemented only the old seam keeps working; a fork
that wants the new behavior opts in (CP-BREAK-22). Which world a type is in is
mechanical, not folklore: every exported interface carries a seam-or-sealed doc
marker, checked in CI (CP-BREAK-23).

**A type the core merely consumes evolves additively.** Domain structs and DTO inputs
may gain fields whose zero values preserve behavior; they never lose, rename, or —
the silent-semantic-drift class — repurpose one (CP-BREAK-7..9).

**The HTTP contract follows the additive posture, gated.** Within a major version the
contract only grows: new optional fields, new operations. Removing an operation
requires deprecating it first and keeping it working at least two minor releases
(CP-BREAK-24, tagged); removal without deprecation is a gate error. Response enums a
fork might switch on carry an extensible-enum marker so new values are non-breaking
and forks are contracted to handle unknowns (CP-EXT-3). Hazards the compiler cannot
catch — a tightened precondition, a repurposed field, a new closed-enum value — are
behavior-class breaks: the gate flags them and the changelog must name them, sourced
from the detector rather than from memory (CP-BREAK-25). This whole posture is scoped
to a *released* contract: until the first external API/MCP consumer the contract has no
one to break, so the gate runs advisory and `crm.yaml` may be realigned to its spec
directly (CP-BREAK-26 / ADR-0017 Amendment 3) — the additive discipline arms when the
stance flips to `stable`.

The wire conventions themselves — list envelope, error shape, concurrency and
idempotency headers — are owned by the api-conventions chapter and never restated
here; endpoints are cited by contract operation id, never inventoried.

## The agent surface — the tool extension

The MCP tool verbs are the same REST operations, annotated with a vendor extension so
the agent tool surface is generated from the one contract (CP-MCP-1). The annotation
names the tool verb, the record type it acts on, and its risk tier: green operations
auto-execute and are reversible; yellow operations are confirm-first (CP-MCP-2). A
yellow operation invoked by an agent principal requires the single-use approval token
minted when a human approves the staged action — without it the call fails closed —
while a human's own direct call is itself the approval (CP-MCP-3, ADR-0036). The tool
tables, tier assignments, and the always-approval floor are owned by the
byo-agent-and-mcp chapter; this chapter owns only the extension mechanism.

## Contract-derived surfaces

Beyond the committed Go and TS types and the two gates, the contract drives the Go
server interfaces the handlers implement, the MCP tool list walked from the annotated
operations, and a thin typed TS fetch client over the generated types (CP-STAGE-8).
Adding an operation or schema to the contract propagates to every surface by
regeneration, never by hand.

## Adding a field

The P2 source-customization path — a real migrated column surfacing as an extra
property without changing the core contract — is generator-driven end to end: extend
the struct and the contract, regenerate, migrate, check, and assert the round-trip.
The canonical recipe is owned by the generators chapter; the contract-side markers it
relies on are pinned here (CP-EXT-1..4).

## Where it lives

The contract sits at `backend/api/` beside its codegen configuration, so codegen is
fully standalone; the generated Go types sit beside the shared kernel, the generated
TS types behind `frontend/src/lib/api-client`. Both gates run inside the standard
check suite. Read next: the api-conventions chapter for the wire conventions the
contract encodes, the architecture chapter for where generated types sit in the module
DAG, and the byo-agent-and-mcp chapter for the tool surface the annotations generate.

## Appendix

### Wire — pipeline stages
Source: margince-poc/docs/architecture/contract-pipeline.md @ a11d6c08

| ID | Stage | What happens |
|---|---|---|
| CP-STAGE-1 | Source | `backend/api/crm.yaml` — valid OpenAPI 3.1, authoritative, derived field-for-field from the data model; lives beside its codegen config (`oapi-types.cfg.yaml`) so codegen is standalone; the source path is env-overridable; the pipeline never mutates the source (derived-spec transforms only). |
| CP-STAGE-2 | TS generation | `openapi-typescript` reads 3.1 directly (skips both Go transforms) → committed TS contract types for the frontend api-client. |
| CP-STAGE-3 | Go down-convert | `openapi-down-convert` produces a derived 3.0 copy — the Go generator (`oapi-codegen`) does not consume 3.1. |
| CP-STAGE-4 | Go disambiguation | a disambiguation pass suffixes every parameter-component Go type with `Param` via `x-go-name` — `oapi-codegen` flattens component schemas and parameters into one Go type namespace, so a parameter and a schema sharing a name (e.g. `ApprovalToken`) would collide. |
| CP-STAGE-5 | Go generation | `oapi-codegen` (models only) → committed Go contract types. |
| CP-STAGE-6 | Drift gate (merge-blocking) | regenerate into a temp dir and diff against the committed Go + TS types; any divergence → DRIFT failure with "regenerate and commit." Wired into the standard check suite; the check is a file-diff. |
| CP-STAGE-7 | Breaking-change gate (merge-blocking when `stable`) | `oasdiff breaking` between the base branch's contract and the working copy; severity→policy is `--fail-on ERR` (CP-BREAK-20) when `CONTRACT_STABILITY=stable`, and advisory (detect-and-print, non-blocking) when `pre-live` — the default until first external consumer (CP-BREAK-26). Skips gracefully but loudly (prints why) when the tool or a base ref is unavailable — same degrade pattern as the drift gate. |
| CP-STAGE-8 | Derived surfaces | Go server handler interfaces from the contract; the MCP tool list generated by walking `x-mcp-tool` operations (CP-MCP-1); a thin typed TS fetch client over the generated types. All regenerated, never hand-edited. |

### Wire — x-mcp-tool
Source: margince-poc/docs/architecture/contract-pipeline.md @ a11d6c08

The extension shape, attached to a REST operation:

```yaml
x-mcp-tool:
  verb: send_email        # the tool name exposed on the agent surface
  record_type: activity   # the domain entity the tool acts on
  tier: yellow            # green = auto-approved | yellow = needs approval
```

| ID | Rule |
|---|---|
| CP-MCP-1 | Every MCP tool verb is one of the same REST operations, annotated with `x-mcp-tool`; the agent tool surface is generated by walking the annotated operations — no second source of truth beside the contract. |
| CP-MCP-2 | `tier: green` (auto-approved) = auto-execute / reversible (reads, drafts, log-activity, run-report). `tier: yellow` (needs approval) = confirm-first (sends, archive, advancing a deal to closed, enrichment that stages a proposal). |
| CP-MCP-3 | A yellow operation invoked by an *agent* principal requires the single-use approval token minted by the approve operation; without it the call fails closed as approval-required ([[api-conventions#API-CONV-10]], [[api-conventions#API-ERR-10]]). A *human's* direct call is itself the approval (ADR-0036). Tool *tables*, tier assignments, and the always-approval floor are owned by the byo-agent-and-mcp chapter — cited, never restated here. |

### Acceptance — breaking-change verdicts
Source: architecture/04-seam-evolution.md#1-safe-vs-breaking-per-seam-type; architecture/04-seam-evolution.md#4-openapi--http-contract-evolution-stripe-style-oasdiff-gated @ 5a0b29c

"Breaking" is judged *from the fork's side*, inside a major version. Interface-seam
rows apply to the fork-implemented ports (connector, tool, datasource provider, model
client, workflow handler, policy strategies); struct rows to domain structs and DTO
inputs; enum rows to enums a fork switches on; HTTP rows to the contract and its
generated types.

| ID | Seam | Change | Verdict |
|---|---|---|---|
| CP-BREAK-1 | interface | Add a **new** sibling interface (V2 embedding V1) + capability probe at the call site | **safe** — old impls still satisfy V1 |
| CP-BREAK-2 | interface | Add a method to the existing interface | **BREAKING** — the fork's type silently stops satisfying it (the canonical trap) |
| CP-BREAK-3 | interface | Change a method signature (params / returns / error set) | **BREAKING** — recompile failure for every implementer |
| CP-BREAK-4 | interface | Remove / rename a method | **BREAKING** |
| CP-BREAK-5 | interface | Tighten a documented precondition the fork relies on | **BREAKING (behavior)** — merges clean, breaks at runtime; requires a `BEHAVIOR:` changelog line |
| CP-BREAK-6 | interface | Add a method to a **sealed** interface | **safe** — the unexported method forbids external implementations |
| CP-BREAK-7 | struct | Add a field whose zero value preserves current behavior | **safe** — fork keeps compiling; a contract test asserts the round-trip |
| CP-BREAK-8 | struct | **Repurpose** an existing field (new meaning, same name/type) | **BREAKING (behavior)** — the silent-semantic-drift class, invisible to the compiler |
| CP-BREAK-9 | struct | Remove / rename a field | **BREAKING** — drops a fork's mapping (silent-field-drop) |
| CP-BREAK-10 | struct | Add a non-comparable field to a struct used as a map key | **BREAKING** — guard with a do-not-compare sentinel field |
| CP-BREAK-11 | enum | Add a value to a response enum marked `x-extensible-enum` | **safe** — forks are contracted to handle unknowns (CP-EXT-3) |
| CP-BREAK-12 | enum | Add a value to a closed enum a fork exhaustively switches on | **BREAKING (behavior)** — the fork's switch silently misroutes |
| CP-BREAK-13 | enum | Renumber / remove / repurpose a value | **BREAKING** |
| CP-BREAK-14 | HTTP | Add an optional response property; add an operation | **safe** — additive |
| CP-BREAK-15 | HTTP | Add a required request property / make a parameter required | **BREAKING** — oasdiff ERR (`new-required-request-property`, `request-parameter-became-required`) |
| CP-BREAK-16 | HTTP | Remove a required response property | **BREAKING** — oasdiff ERR (`response-required-property-removed`) |
| CP-BREAK-17 | HTTP | Remove an operation **without** prior `deprecated: true` | **BREAKING** — oasdiff ERR (`api-removed-without-deprecation`) |
| CP-BREAK-18 | HTTP | Remove an operation **after** the ≥2-minor deprecation window (CP-BREAK-24) | **safe** — the sanctioned path |
| CP-BREAK-19 | HTTP | Add a value to a response enum | **safe iff** marked `x-extensible-enum`, else WARN → `BEHAVIOR:` |

The policy those verdicts feed:

| ID | Policy |
|---|---|
| CP-BREAK-20 | **oasdiff severity→policy mapping.** ERR-class changes block the release/merge or force a major bump; WARN-class changes require a `BEHAVIOR:` changelog entry. The gate runs `--fail-on ERR`: breaking blocks, additive/deprecation passes. |
| CP-BREAK-21 | **Frozen vs consumed.** A seam a fork IMPLEMENTS is a frozen contract; a type the core merely CONSUMES evolves additively. Neither ever mutates an existing member. |
| CP-BREAK-22 | **The V2-plus-capability-probe mechanism.** A frozen seam never grows a method; new capability ships as a sibling interface embedding V1, and the call site probes for it with a fallback for forks that only implement V1. Ships as a `SEAM:` changelog entry; no major bump. |
| CP-BREAK-23 | **Seam markers, CI-checked.** Every exported interface carries `// seam: forks may implement` (frozen, additive-only) or `// internal: sealed` (unexported method; forks must not implement; may change at any time). An exported interface with neither marker fails CI. |
| CP-BREAK-24 | **Deprecate-before-remove.** Mark `deprecated: true` (and the Go deprecation comment), keep it working **≥ 2 minor releases**, remove only on a major. Removing without deprecation is an oasdiff ERR (CP-BREAK-17). |
| CP-BREAK-25 | **Changelog sourced from the detector, not memory.** Every oasdiff ERR/WARN must produce a matching `BREAKING:`/`BEHAVIOR:` changelog line, so the changelog cannot silently omit a detected break; the upgrade preflight tells a fork exactly what to re-verify. |
| CP-BREAK-26 | **Pre-1.0 contract stance (ADR-0017 Amendment 3).** The HTTP breaking-change gate honors a `CONTRACT_STABILITY` stance: `pre-live` (default, until the first external API/MCP consumer) runs the gate **advisory** — breaking changes to `crm.yaml` are detected and printed but do not block, so the contract may be realigned to its own spec directly; `stable` restores `--fail-on ERR` and CP-BREAK-24 deprecate-before-remove. Narrow scope: the **HTTP contract only** — the seam-interface freeze (CP-BREAK-21..23), the migration/expand-contract rules, and the drift gate are unaffected by the stance. |

### Acceptance — extension markers
Source: margince-poc/docs/architecture/contract-pipeline.md @ a11d6c08; architecture/04-seam-evolution.md#4-openapi--http-contract-evolution-stripe-style-oasdiff-gated @ 5a0b29c

| ID | Rule |
|---|---|
| CP-EXT-1 | Core object schemas set `additionalProperties: true` — a P2 client field arriving as an extra property is valid against the unchanged core contract. |
| CP-EXT-2 | An extension field is marked `x-extension: true`: a real migrated column surfaces additively as an extra property, distinguishable from core fields by tooling, without editing the core schema. |
| CP-EXT-3 | Every response enum a fork might switch on carries `x-extensible-enum`; adding a value is then non-breaking (CP-BREAK-11) and forks are contracted to handle unknown values gracefully. A switched response enum *without* the marker is a `BEHAVIOR:` hazard (CP-BREAK-19). |
| CP-EXT-4 | The add-a-field path is generator-driven (struct → contract → regenerate → migrate → check → round-trip assertion); the canonical recipe is owned by the generators chapter and cited, never restated here. |
