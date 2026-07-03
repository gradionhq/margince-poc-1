---
derives-from:
  - margince-poc/docs/quality/quality-gates.md
  - margince specs/spec/architecture/05-test-architecture.md#2-the-five-mandated-guardrail-tests
---
# Quality gates — every check, what it verifies, where it blocks

This chapter is the gate registry: the one place that answers what checks guard the
code, what each one verifies, and where it stops bad work from getting in. If you read
one quality chapter to understand how quality is enforced, read this one; the testing
chapter is its sibling and owns the test lanes and determinism doctrine the gates
depend on.

## What a gate is

A gate is an automated check that fails loudly and blocks the work until it is fixed.
A check that can be skipped, ignored, or manually overridden is not a gate — it is a
suggestion. Every gate in the registry pinned below is therefore a **required check
that blocks a merge to the default branch**, and one rule binds them all: **no gate
may silently skip.** A gate that cannot run — a missing tool, a missing dependency, an
unreachable service — is a failure, not a pass. Every gate runs for real, every time.

## The short version

One aggregate check target runs the full set of fast, deterministic gates in a fixed
order — formatting, then lint, then the architectural and data invariants, then the
hermetic tests, backend before frontend — with narrower inner-loop subsets for the
backend-only and frontend-only halves. Contributors run it before pushing; if it is
red, the work is not done.

Two gates cannot live inside that aggregate target because they need live services or
a judged review: the **integration suite**, which provisions a real database, cache,
and object store, and the **craftsmanship review**, which judges whether the change
reads as a careful human's work against a rubric rather than a mechanical rule. Each
runs as its own required check and is marked as a live-service or judged gate in the
registry.

Beyond the registry, the test-architecture decision mandates **five merge-blocking
guardrail tests, G-a through G-e**, pinned verbatim below. Their logic: a green run is
the safety contract that permits an agent to ship an edit, so each guardrail either
resists silent weakening (deleting the control turns the test red) or is
generator-emitted (the author cannot forget to write it). They are pinned in this
registry so no drift in the gate table can quietly drop one; the testing chapter owns
the determinism doctrine that makes their green trustworthy.

## The running-scaffold gate

This repository is guarded from its **first commit** by the running-scaffold gate: the
stack comes up under compose, migrations apply cleanly to a fresh database, the sample
vertical slice's tests pass end-to-end, and the contract pipeline is drift-free. There
is no window in the repository's history where the scaffold does not run — every later
gate assumes a working, migratable, contract-honest baseline because this gate
established one on day one. It is pinned as QG-25.

## How gates are enforced

There are three enforcement points, from cheapest to authoritative:

- **The developer loop.** The aggregate check target is the local truth: run it, and a
  red result means the change is not ready. It exists so failures are found where they
  are cheapest to fix.
- **The pre-push filter.** A local hook runs the cheapest deterministic subset —
  residue checks, the craft module's own tests, formatting, and vetting — so the most
  common mistakes cannot even be pushed.
- **CI is the authority.** Every gate in the registry — plus the live-service and
  judged gates — runs as a required check on each pull request to the default branch.
  A red gate blocks the merge, with **no manual override**. The local layers are
  conveniences; CI is the contract.

## Adding a new gate

1. Write the check as a deterministic script that exits non-zero on failure, or extend
   an existing lint configuration.
2. Wire it into the aggregate check target so the developer loop runs it.
3. Add its row to the gate-registry appendix below — what it verifies and where it
   blocks. A gate that is not in this registry does not exist.
4. Make it a required CI check.

A check that is missing any of these steps is not a gate. In particular, a check that
runs only when someone remembers to invoke it violates the no-silent-skip rule by
construction.

## Appendix

### Acceptance — gate registry
Source: margince-poc/docs/quality/quality-gates.md @ a11d6c08

Every row is a required, merge-blocking check. "Aggregate + CI" means the gate runs in
the aggregate check target locally and as a required CI check on every pull request.
The two gates marked **live-service** and **judged** run as their own required CI
checks (they need provisioned services or a judged review and cannot run in the
aggregate target).

| ID | Gate (target) | What it checks | Where it blocks |
|---|---|---|---|
| QG-1 | `fmt-check` | Every Go file is formatted to the strict standard (gofumpt) | Aggregate + CI |
| QG-2 | `vet` | No `go vet` findings in any module | Aggregate + CI |
| QG-3 | `lint` | The full Go lint ruleset, all hard-blocking: function-length and complexity caps, unchecked errors, SQL-injection and other security (SAST) checks, an import allow/deny + slopsquatting guard, dead/speculative code, and ~25 more (golangci-lint) | Aggregate + CI |
| QG-4 | `go-file-length` | No Go file exceeds the file-length cap (the "god file" guard) | Aggregate + CI |
| QG-5 | `govulncheck` | No known security vulnerability (CVE) in a dependency or in our own code — adds the public CVE database on top of QG-3's SAST/import guards | Aggregate + CI |
| QG-6 | `gen-types-check` | The generated Go + TypeScript types still match the API contract | Aggregate + CI |
| QG-7 | `contract-breaking-check` | No breaking change to the API contract since the default branch — severity-classified: breaking blocks; additive fields and deprecations pass | Aggregate + CI |
| QG-8 | `gen-manifests-check` | The generated wiring matches the connectors/workflows/tools on disk | Aggregate + CI |
| QG-9 | `arch-lint` | Code only imports across module boundaries through an allowed seam — the module map is respected | Aggregate + CI |
| QG-10 | `fitness-jurisdiction` | No country-specific code or strings leak into the shared core | Aggregate + CI |
| QG-11 | `audit-coverage` | No data change skips the audit trail — every mutation goes through the audit seam | Aggregate + CI |
| QG-12 | `audit-coherence` | The audit-log action/actor vocabulary in the database matches the contract | Aggregate + CI |
| QG-13 | `rls-store-path` | Database access goes through the tenant-isolating transaction path, never the raw superuser pool | Aggregate + CI |
| QG-14 | `check-craft-doc` | The code-quality standard is present in the agent instructions | Aggregate + CI |
| QG-15 | `test-lanes` | A unit test never opens a real database or cache — the lane boundary (see the testing chapter, [[testing#TEST-LANE-1]]) | Aggregate + CI |
| QG-16 | `test` | Go unit tests pass — the fast, hermetic, no-database lane | Aggregate + CI |
| QG-17 | `fe-lint` | Front-end code passes the linter (Biome) | Aggregate + CI |
| QG-18 | `fe-typecheck` | Front-end TypeScript type-checks | Aggregate + CI |
| QG-19 | `ds-purity` | UI uses design-system tokens, not raw colours/pixels | Aggregate + CI |
| QG-20 | `font-lock` | Only the approved fonts are used | Aggregate + CI |
| QG-21 | `icon-lint` | UI icons are from the approved set | Aggregate + CI |
| QG-22 | `fe-test` | Front-end unit + component (Storybook) tests pass | Aggregate + CI |
| QG-23 | `test-integration` — **live-service** | The database, RLS, and cross-module behaviour work against a real Postgres/Redis/storage ([[testing#TEST-LANE-2]]) | Own required CI check on every PR (provisions the live services) |
| QG-24 | Craftsmanship review — **judged** | The code reads like a careful human wrote it — a judged rubric, not a mechanical check | Own required CI check on every PR |
| QG-25 | Running scaffold | The stack comes up under compose, migrations apply cleanly, the sample vertical slice's tests pass end-to-end, and the contract pipeline is drift-free | Required from this repository's first commit onward (bootstrap gate; not in the poc registry) |
| QG-26 | `check-image-pins` | Every CI workflow action (`.github/workflows/*.yml` `uses:`) is pinned to a commit SHA, never a floating tag — supply-chain hardening against a moved tag | Aggregate + CI |

Notes carried from the source registry:

- One fast pre-flight — a check that every internal reference in the API contract
  resolves (`contract-lint`) — is a standalone convenience, deliberately **not** part
  of the aggregate target and not a merge gate.
- Reconciliation: the testing chapter's live-stack UAT lane ([[testing#TEST-LANE-3]])
  also runs as its own required CI check per the source testing handbook, but the
  source gate registry lists only QG-23 and QG-24 as extra-CI gates — flagged here
  rather than invented as a registry row.

### Acceptance — guardrail tests
Source: margince specs/spec/architecture/05-test-architecture.md#2-the-five-mandated-guardrail-tests @ 5a0b29c

The five mandated, merge-blocking guardrail tests. IDs are corpus IDs, preserved
verbatim.

| ID | Test | What it proves | Generated / located | Merge-blocking? |
|---|---|---|---|---|
| G-a | **Per-field DTO round-trip** (M1) | A field survives `domain → DTO → domain` as identity; a dropped mapping in a merged `toDTO` hunk fails CI instead of silently zeroing the field | Emitted by `crm gen field` next to the type; the generator output contract makes emitting it mandatory | **Yes** |
| G-b | **Contract-completeness** (M2) | *Every* domain field appears in `toDTO` (and reverse) — reflection-walked, covers core **and** future custom fields with no per-field authoring; the structural answer to the silent-field-drop conflict surface (C1) | One reflective test per domain↔DTO pair; checked by `crm gen upgrade` preflight | **Yes** |
| G-c | **The three security gates** | (i) **injection red-team** — seeded auto-captured T2 payloads + a real BYO agent reach no egress without a 🟡 gate / volume limit; (ii) **tier-leak** — T2 content is always labeled in tool output and egress of T2+sensitive fields is gated; (iii) **capture-isolation** — connector-created records default to originating-user scope, not workspace-global | Agents + capture integration suite; injection corpus under the evals injection set | **Yes** (injection = GA-blocking for any BYO-agent seat) |
| G-d | **Brain-swap conformance probe** | Swapping the model client (Claude API ↔ local Gemma ↔ user's Codex via A2) causes **zero change** to tools, scopes, 🟡 gates, or audit. No brain can exceed agent ≤ human or write past a 🟡 gate | Parameterized over the registered model factory bindings; asserts the same tool/scope/gate/audit trace per brain | **Yes** |
| G-e | **Dual-binding A7 evals** (A23 / ADR-0012) | Extraction / summary / NL→query / schema-validity (plus no-guess = 0, injection-egress = 0) pass on **both** the local-default and the cloud-default binding; sovereign zero-egress conformance passes on the local path | Per-task golden sets run twice — once per binding — as the WP3 exit gate | **Yes** (WP3 exit; deterministic gates per-PR, quality bands per-release) |

Reconciliation: none of G-a..G-e has a corresponding QG row yet — the source poc
registry predates the guardrail mandate. They are pinned here as merge-blocking
requirements the registry must grow to run as the generator, agent surface, and eval
harness land; a registry that ships those subsystems without these rows is a spec
defect, not a judgment call.
