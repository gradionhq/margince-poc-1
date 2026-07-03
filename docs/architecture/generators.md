---
module: cli/crm-gen
derives-from:
  - margince-poc/docs/architecture/generators.md @ a11d6c08
  - narrative/04-customization-paradigm.md#spike-outcome--mandatory-mitigations-2026-06-03 @ 5a0b29c
  - architecture/08-extension-and-generators.md#3-crm-gen-upgrade-preflight-adr-0017-4 @ 5a0b29c
---
# Generators — crm-gen, the source-customization command reference

> The `crm-gen` CLI implements the source-customization paradigm (ADR-0002): you add
> capabilities by **editing source and regenerating**, not by configuring a runtime
> engine. This chapter is the command reference — the generator catalog, the end-to-end
> recipe shape, and the upgrade-safety mandates every generator run must satisfy.

The `crm-gen` CLI (`cli/crm-gen/`) is the underlying binary; the user-facing entry
points are the `make gen-*` targets, which build and invoke it for you. Generated files
carry a `DO NOT EDIT` header and are never hand-edited — conflict resolution on any
generated artifact is "re-run the generator," never a hand-merge.
<!-- 1c-mapping: final call pending — the generator CLI's provisional home is cli/crm-gen/ (the poc location); the ratified backend/ tree does not yet name a home for it. -->

> **For AI agents:** when asked to add a connector, workflow, tool, custom object,
> report, or field, run the matching generator sub-command first (GEN-CMD-1..7), then
> fill in the stub. Always finish with `make gen-manifests && make check`.

## Field, two paths — pick the right one

`crm-gen field` (GEN-CMD-1) is the **deploy-time**, source-customization path for a
first-class field a developer commits. A **separate, runtime** path exists for a
*workspace* adding its own scalar custom field without a code deploy — the custom-field
catalog + AddField engine in the core data model (ADR-0002 Amendment 2), which runs a
governed schema change behind a needs-approval (Yellow) gate. That runtime surface is
owned by the runtime-config chapter's boundary table as RC-12
([[runtime-config#RC-12]], parallel draft). Don't scaffold a `crm-gen field` for a
request that's actually about workspace-level customization.

## Commands

Each command scaffolds a stub + its test and prints the next steps. Build the binary
with the `make` targets (or build it directly from `cli/crm-gen/`). The full command
catalog, with what each scaffolds and when to use it, is pinned in the appendix
(GEN-CMD-1..7).

## Full recipe — adding a capability end-to-end

1. Scaffold with the generator (the command for each kind is pinned as GEN-CMD-1..7).
2. Fill in the real logic — replace the emitted stubs.
3. Regenerate the import manifest.
4. Apply migrations, if the scaffold emitted one.
5. Run the full gate suite and verify green.

All five steps are required before committing. `make check` enforces the module DAG
([[quality-gates#QG-9]] `arch-lint`), contract-type freshness ([[quality-gates#QG-6]]
`gen-types-check`), manifest freshness ([[quality-gates#QG-8]] `gen-manifests-check`),
and the unit tests — so a forgotten step is caught, not shipped.

## Adding a field (the canonical recipe)

This is the P2 source-customization recipe; the contract-pipeline and architecture
chapters link here rather than restate it. The user-facing command is the `make`
target; it builds and invokes `crm-gen field` for you.

> **Note:** this recipe will move to `recipes/add-a-field.md` (exemplar-first, pointing
> at the sample vertical slice); this chapter stays the command reference and will link
> there once the recipe lands.

Run the field generator's make target with the table, column, and SQL type (the exact
invocation is pinned as GEN-CMD-1). That writes a sequenced migration pair under
`backend/migrations/` and prints the next steps:

1. Add the field to the domain struct in the owning module under
   `backend/internal/modules/<name>/domain/`.
2. Add the property to the schema in `backend/api/crm.yaml`.
3. `make gen-types` — regenerate the Go + TS contract types.
4. `make migrate-up && make gen-types-check && make check` — apply the migration and
   green the gates.
5. Add a round-trip assertion (the field survives create → read) — the M1 mandate; the
   generator emits the test stub.

## Upgrade safety

The customization spike proved the paradigm viable for additive and independent-logic
changes, but surfaced a real failure mode: merge conflicts land inside shared functions
(validation, DTO mapping), where a naive resolver can silently drop a field the
compiler won't catch — a dropped DTO mapping just zeroes the field. Four mitigations
are therefore **mandatory requirements**, not nice-to-haves; they are pinned verbatim
in the appendix as M1–M4, and the scaffolded upgrade preflight the M2 mandate requires
is pinned step-by-step as GEN-UPG-1..6. Nothing in an upgrade auto-merges: a green
client-fixture run is the only go signal.

## Where it lives

The generator is `cli/crm-gen/` (home pending final 1c mapping, per the note above).
What it scaffolds lands in the owning bounded contexts under
`backend/internal/modules/<name>/` (core objects and reports in their owning modules,
connectors and workflows in the capture module, tools in the agents module) and in
`backend/migrations/`. The contract it extends is `backend/api/crm.yaml`; the layout
vocabulary is the [architecture](architecture.md) and
[code-organization](code-organization.md) chapters'.

## Appendix

### Tools — generator commands
Source: margince-poc/docs/architecture/generators.md#commands @ a11d6c08

| ID | Command | Scaffolds | When to use |
|---|---|---|---|
| GEN-CMD-1 | `crm-gen field <table> <column> <sql-type> [go-type]` | a migration pair + a recipe to wire the field through struct → contract → TS → test | Add a first-class field to person / organization / deal / activity / lead. Run via `make gen-field ARGS="person nickname text string"` — the canonical recipe above. For a workspace-level runtime custom field use the RC-12 path instead, never this. |
| GEN-CMD-2 | `crm-gen object <Name>` | a new custom object: its table (UUIDv7 PK, workspace scoping, versioning, provenance, RLS pre-wired), its Go struct, and test stubs | Add a domain concept that needs its own table, not just a column (e.g. `InvoiceItem`, `SupportTicket`). |
| GEN-CMD-3 | `crm-gen workflow <name>` | a self-registering async workflow handler (River-backed) + its test | Add an event-driven async job. Fill in the event match + the job logic. |
| GEN-CMD-4 | `crm-gen connector <name>` | a self-registering capture connector + its test | Integrate a new data source (email provider, import, webhook). Implement the normalize step that maps raw input to the canonical type. |
| GEN-CMD-5 | `crm-gen tool <name>` | a self-registering governed MCP tool + its test, defaulting to the safe **Green** tier | Expose a CRM capability to the AI surface. Use **Yellow** for mutating/outward-facing actions (ADR-0026). Every tool must declare a tier. |
| GEN-CMD-6 | `crm-gen report <name>` | a compiled read-only report query + its test | Add a named, versioned read query callable from the API or scheduled reporting. |
| GEN-CMD-7 | `crm-gen manifests` | regenerates the import manifest so the binary picks up all self-registered connectors/workflows/tools | After adding any connector/workflow/tool. Run via `make gen-manifests`; `make check` fails if it's stale ([[quality-gates#QG-8]]). |

### Acceptance — upgrade-safety mandates
Source: narrative/04-customization-paradigm.md#spike-outcome--mandatory-mitigations-2026-06-03 @ 5a0b29c

The spike-mandated mitigations, IDs and substance preserved verbatim from the corpus.
<!-- reconcile: the corpus spells the command `crm gen <sub>`; the shipped poc binary is `crm-gen <sub>` (make-target wrapped). The M-IDs and requirements are unchanged; the spelling difference is cosmetic. -->

| ID | Mandate |
|---|---|
| M1 | **Per-field contract round-trip test.** Every field (core and custom) ships a DTO round-trip test, generated by `crm gen field`, so a dropped mapping fails CI. (In the spike, the custom field's test caught it; the un-tested core field would have been dropped silently.) |
| M2 | **`crm gen upgrade` preflight.** A scaffolded upgrade flow: resolve remote/branch reality, list conflicts, flag shared-function conflicts with explicit "union, don't pick" guidance, and run a contract-completeness check (every domain field appears in the DTO mapping). |
| M3 | **Behavior-change changelog discipline.** Upstream releases declare new invariants/validation in the changelog; the upgrade recipe re-runs client fixtures against them, because a conflict-free merge can still break behavior (the spike's new "stage required" rule). |
| M4 | **Agent-recipe gaps to fix:** the field recipe must state where defaults and enum-validation extensions go; the upgrade recipe must cover shared-function conflicts, the silent-field-drop risk, and the local-branch-vs-remote reality. |

### Acceptance — upgrade preflight
Source: architecture/08-extension-and-generators.md#3-crm-gen-upgrade-preflight-adr-0017-4 @ 5a0b29c

The M2 preflight, step by step (ADR-0017). **Nothing auto-merges; a green
client-fixture run is the only go signal.** The poc doc carries no preflight steps;
they are lifted from the corpus generators blueprint.

| ID | Step (in order) | Rule |
|---|---|---|
| GEN-UPG-1 | Resolve remote-vs-local | Fetch upstream and determine the real merge base against the fork's branch (not an assumed default branch) — the spike's local-branch-vs-remote-reality lesson (M4). |
| GEN-UPG-2 | Classify conflicts | Loud **UNION, DON'T PICK** on shared-function hunks (DTO mapping / validation / apply) — the preflight refuses an ours/theirs resolution and instructs the agent to union both sides. **RED, human-required** on core-override conflicts: no agent auto-resolution; a human (or a billable engagement, P14) is required. |
| GEN-UPG-3 | Run contract-completeness + contract diff | The reflective completeness check (M1/M2) over every domain↔DTO pair, plus breaking-change detection on the API contract (the contract-drift gate, [[quality-gates#QG-7]]). |
| GEN-UPG-4 | Validate migration namespacing | Assert core migrations stayed sequential/upstream-owned and custom migrations stayed timestamp-ordered and prefix-namespaced; flag any upstream column that would name-collide (ADR-0017). |
| GEN-UPG-5 | Parse the behavior-tagged changelog | Read the structured changelog tags (BREAKING / BEHAVIOR / EXPAND / CONTRACT / DEPRECATED / SEAM) to tell the fork exactly what to re-verify — closing the conflict-free-but-behavior-broke hole (M3). |
| GEN-UPG-6 | Re-run client fixtures | Against the merged tree. Green = go. Anything red, or any RED-class conflict from GEN-UPG-2, stops the upgrade for human resolution. |
