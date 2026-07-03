# Harvest provenance

This `skeleton/` tree is harvested from the frozen `margince-poc` reference repo. It is
the factory's input package (`factory/2026-07-02-input-package-and-spec-gate-design.md`
§3.1): a verified-running scaffold — architecture made real before any feature exists —
that later factory tickets build on top of.

- **Source repo:** `margince-poc`
- **Source commit SHA:** `a11d6c08fe3588eb5571abf3576398f4e390b22d`
- **Harvest date:** 2026-07-02
- **Harvest plan:** `factory/plans/2026-07-02-skeleton-harvest.md` (Phase 1b, branch
  `factory/skeleton-harvest`)
- **Inventory doc:** `factory/skeleton-harvest-inventory.md` — the pre-harvest sweep
  that proposed the LIFT/ADAPT/DROP split and the D-H0/D-H1/D-H2/D-H3 synthesis
  decisions. Two of those decisions were amended during execution; both amendments are
  recorded below. This file is the after-the-fact record of what was *actually* done,
  task by task — read it, not the inventory doc, for ground truth on the shipped tree.
- **Task ledger:** `.superpowers/sdd/progress.md` (`== Phase 1b skeleton harvest ==`
  section) is the authoritative sequence of adjudications; `.superpowers/sdd/task-{1..10}-report.md`
  are the individual task reports this file summarizes.

## Corrections to the inventory doc

- **Product is dropped, not kept.** The inventory doc's D-H1/D-H3 text assumed the
  `product` table landed in migration 000003 (in the 000001–000025 platform block) and
  treated Product as a possible secondary backend-only exemplar. It does not: the
  `product` table is introduced in poc migration **000034**, a *feature* migration well
  outside the platform boundary. Task 4 confirmed this and dropped all Product backend
  files (`product.go`, `product_test.go`, `product_integration_test.go`,
  `product_schema_test.go`, `product_validation_test.go`, `handler_product.go`)
  unconditionally — no table, no exemplar. The Person slice (D-H3) stands alone as the
  sample vertical slice.
- **D-H1 amended: the platform schema is not a clean 000001–000025 cut.** Task 6 found
  that several kept platform files (in-boundary packages, not feature code) reference
  columns/tables introduced by feature-numbered migrations just past the 000025 line,
  and that `make audit-coherence` requires the `audit_log.action` CHECK constraint to
  match the full action set already committed in `contract/crm.yaml`. Backporting those
  migrations verbatim (unmodified, at their original poc numbers) was ruled the correct
  fix over rewriting kept code to route around missing columns. The final, amended
  platform schema is:

  **migrations 000001–000025, plus dependency-backported migrations {000027, 000028,
  000042, 000045, 000047, 000051, 000053, 000054} (verbatim, original poc numbers, each
  a single-purpose migration with no unrelated feature chain dragged in), plus one
  skeleton-original migration, 000062.**

  | Backported migration | What it adds | Why it was needed |
  |---|---|---|
  | `000027_activity_transcript_ref` | `activity.transcript_ref` column | `crm-core/store_activity.go` (`ActivityStore.Get`) hardcodes it in its SELECT list |
  | `000028_approval_resume_window` | `approval_item.resume_window` jsonb column | `crm-approvals/approvals.go` `Create`/`Get` reference it unconditionally |
  | `000042_activity_remind_at` | `activity.remind_at` column + widened `activity_task_fields` CHECK | backs the kept `handler_next_step.go` handler |
  | `000045_audit_log_automation_actions` | widens `audit_log_action_check` (adds `parameterize`, `pause`) | intermediate step toward the CHECK version the audit-coherence gate needs |
  | `000047_audit_log_action_contract_reconcile` | widens `audit_log_action_check` to the full 28-action contract-reconciled set | makes the 10 previously-rejected contract actions insertable |
  | `000051_oauth_client_registration` | `oauth_client`, `oauth_auth_code` tables | `crm-auth/oauth_client_store.go`, `oauth_auth_code_store.go` |
  | `000053_connector_secret` | `incumbent_connection`, `connector_secret` tables | `crm-auth/incumbent_connection_store.go` |
  | `000054_connector_secret_fk_index` | standalone FK index on `connector_secret.connection_id` | `TestFKColumnsAreIndexed` gate — the composite index shipped in 000053 doesn't satisfy the standalone-FK-index requirement |

  Even after all eight backports, `audit_log_action_check` still fell 4 actions short of
  the full 32-action contract set — the remaining gap belonged to two *feature*
  migrations (poc 000048 `bulk_operation`, poc 000059) that the backport judgment guard
  correctly refused to pull in (each would drag in an unrelated multi-migration feature
  chain). Instead, Task 6 authored **`000062_audit_action_contract_sync`** — the first
  **skeleton-original** migration — which replaces the CHECK directly with the full
  32-action contract-enum set (`crm.yaml`'s canonical list at harvest time), superseding
  000047 as the authoritative version of that constraint. Its `down.sql` restores the
  CHECK to exactly the 000047 (28-value) set.

  **Skeleton-original migrations number from 000062 upward.** The next skeleton-original
  migration (if any) is 000063.

## LIFT / ADAPT / DROP — what was actually done, by area

| Area | Action | Notes |
|---|---|---|
| **Migrations** | ADAPT | 000001–000025 kept verbatim; +8 backports (table above); +000062 skeleton-original. 000026–000061 (minus the 8 backports) dropped — feature migrations, return with their features. |
| **Seed data** (`infra/seed/dev.sql`) | LIFT | Byte-for-byte port of poc's file — every INSERT target already existed unchanged in 000001–000025; the brief's "delete feature rows" instruction was a no-op (poc's seed never touched feature tables). Workspace, 3 people (Alice/Bob/Carol), 4 app users (admin/rep/readonly/manager), 5 roles + RBAC perms, 1 pipeline + 4 stages, 2 deals, 1 relationship, 3 audit_log rows. |
| **`crm-core` (handlers/services)** | ADAPT (heavy DROP) | Kept: Person (full slice, D-H3), Organization/Deal/Pipeline/Stage/Activity/Lead stores (HTTP handlers for these pruned per amendment — see cmd/server routes below), Relationship domain deleted per explicit brief instruction despite its table existing. Dropped as feature code (no backing table in-boundary, or explicit brief/amendment instruction): signals, overlay/forecast/consensus (`overlay*.go`, `forecast*.go`, `overlaymirror*.go`), custom field definitions, draft-email, invoice/offer, **product** (see correction above), automation, reports, ai_feedback, bulk_operation, conversation_link (see stub note below), custom_field, deal_room, drafting_asset, field_provenance, warm_room_joiner, resolver.go (its only non-native branch depended on deleted overlay code). |
| **`crm` platform packages** | LIFT | Tier-0 seams (`errs`, `crmctx`, `prov`, `ids`, `datasource`, `model`, `jurisdiction`, `obs`, `blobstore`, `keyvault`, `migrate`, `mcp`), `crm-auth` (sessions, passports, RBAC), `crm-audit`, `crm-approvals` (approval-token + concurrency seam only), `crm-gdpr` (full package verbatim — consent, erasure, retention worker live-registered in river.go) kept close to verbatim. |
| **`mcpserver`** | **DELETED** | `crm/mcpserver/` removed entirely (`mcpserver.go`, `mcpserver_test.go`), because its only import (`crm/crm-agents`) is itself deleted feature code. `crm/.go-arch-lint.yml`'s `mcpserver` component entry and `mayDependOn` block removed with it. `crm/mcp/` (the schema seam, distinct from `mcpserver`) is untouched and kept. |
| **`archtest`** | **DELETED** | `crm/archtest/` (the overlay-import-boundary lint package) removed — its entire subject was guarding import boundaries between `crm-agents`/`crm-ai`/`crm-capture`, all deleted; `go list` on those packages fails outright since the dirs no longer exist. Acceptable per controller ruling: external arch-lint (`go-arch-lint`, kept) still carries the DAG-enforcement gate; revisit if/when the feature packages return in Phase 1c. |
| **authz / test-helper relocations** | RELOCATED, verbatim | Two platform declarations were orphaned when their poc source files were deleted per the brief; both were moved verbatim (no logic changes) into new files rather than left broken: `reUUID` + `Authorizer` (from the deleted `handler_relationship.go`) → new `crm/crm-core/authz.go`; `seedWorkspace`/`seedAppUser`/`sqlDB` test helpers (from the deleted `handler_deal_test.go`/`relationship_test.go`) → new `crm/crm-core/helpers_shared_test.go` and `helpers_shared_external_test.go` (split by package — internal vs external test package — since a file can only declare one package), preserving the original `//go:build integration` tags. |
| **conversation-linking** | **STUBBED** | No `conversation_link` table exists in-boundary, so the feature's domain type and handlers were dropped — but the kept `datasource.Provider` interface (a Tier-0 seam) requires `LinkConversation`/`UnlinkConversation` methods that `crm-core/datasourcebinding.go` must implement to satisfy the interface. Rather than reintroduce feature code or change the Tier-0 interface, both methods were reduced to a package-level error: `errConversationLinkingUnavailable = errors.New("conversation linking is not available in this build")`, returned unconditionally by both methods. Not implemented — a placeholder that keeps the interface satisfied. |
| **`cmd/server`** | ADAPT | Kept: middleware stack (session → workspace → RBAC), workspace bootstrap, `/auth/login`, `/auth/logout`, `/me`, `/passports`, `/people` (the slice), RFC 7807 errors, cursor pagination, If-Match concurrency, outbox relay + River wiring. Feature HTTP registrars (deals surfaces, async surfaces, webhooks, HubSpot OAuth) removed from `routes.go`. |
| **Tests** | ADAPT (heavy DROP) | Deleted where the subject under test no longer exists (e.g. `handler_send_email_test.go` — its `ActivityHandler.sendEmail` was pruned; `lead_segregation_integration_test.go` — all three load-bearing dependencies gone) or asserted schema shape from a migration beyond the harvested boundary. Kept tests re-verified green after every deletion pass. |
| **`Makefile`** | ADAPT (targeted prunes) | Full 19-gate `check` composition kept. `GO_DIRS` pruned: `cli/swarm`, `cmd/crm-mcp`, `cmd/crm-mcp-http` removed (directories don't exist in the skeleton — `vet`/`lint`/`test` would `cd` into them and fail). `build-crm-mcp-http` target deleted entirely (built a binary for a deleted directory). The `swarm:` and `supervisor:` *build* targets (and their `.PHONY` entries) were also pruned — same dead-reference class (`cli/swarm`, `cli/supervisor` don't exist), though they weren't in the `check` dependency chain. |
| **CI** (`.github/workflows/ci.yml`) | LIFT, poc-verbatim | The inventory doc originally called for re-enabling `deterministic-gates` for the skeleton repo. **That call was overridden.** `ci.yml` ships byte-for-byte identical to poc — manual-only (`workflow_dispatch` trigger), the same `if: false` disabled jobs (`deterministic-gates`, `dco`, `craft-residue`). This is required, not just conservative: `cli/craft/wiring`'s own tests (`TestCIWorkflow_gateJobsExistWhileCIIsManualOnly`, `TestCIWorkflow_craftsmanshipRunsAfterDeterministicGates`) assert against poc's exact disabled-gate structure. Enabling any of those jobs for real is deferred as a reboot-time decision, not made here. `.github/PULL_REQUEST_TEMPLATE.md` and `infra/branch-protection.json` are likewise poc-verbatim copies (sha1-confirmed matching both sides). |
| **`web/` (frontend)** | ADAPT (heavy DROP) | Kept the full platform shell (per D-H0: login, AppShell, WorkspaceRail, TopBar, `railNav`, ProtectedRoute, LoginPage) plus the one real Person slice (PeoplePage/PersonList/PersonCard + stories + tests) and ShellPlaceholderPage stubs for every other rail route. Deleted feature pages (Tasks/Inbox/Members/Automations/Integrations) + their API modules, feature components/atoms (ApprovalItemContainer, PassportCard, HubSpotConnectionPanel, ApprovalGate/Item, EvidenceChip, ProvenanceTag, ConfidenceMeter, AutonomyDot, DealCard, MorningBrief, PipelineBoard, StagingCard, RecordView, DrawerPanel — all with their co-located stories/tests). `ToastContainer` wired into `AppShell`; `SessionExpiry` (unwired in poc) dropped. |
| **fe-build latent-defect repairs** | FIXED (authorized divergence) | poc's own `web/` had never had `make fe-build` run against it — the harvest was the first time. Repairing it surfaced pre-existing defects, not harvest-introduced ones (reproduced identically against the frozen poc tree before any skeleton changes): `web/package.json` gained `@types/node ^22` (missing dependency); 3 test files got fixture repairs (shape/type fixes only — no assertion changes); one type cast widened through `unknown` (`PeoplePage.test.tsx`, a partial-mock TS2352). `web/dist` added to `.gitignore` (poc never ran a build, so this entry was itself latent-missing). No test assertions were changed anywhere in this repair. |
| **`gen_manifests` template fix** | FIXED (authorized divergence) | `cli/crm-gen/gen_manifests.go`'s template unconditionally emitted `import (\n{{range}}...{{end}})`; with the skeleton's now-legitimately-empty scan-dirs (no connector/workflow/tool packages survive the harvest), this produced a syntactically valid but non-gofumpt-canonical empty `import ()` block, which fought `make fmt-check`. Fixed with a 2-line template change (`{{if .}}...{{end}}`) — verified both the empty-scan and non-empty-scan (scratch-tested with a fake connector file) branches produce gofumpt-clean output. This is a genuine latent bug the harvest exposed (poc always had ≥1 package to scan; this branch was never exercised before), not a feature change. |
| **`swarm`/`supervisor`/`crm-mcp` Makefile targets** | PRUNED | See Makefile row above — consolidated here for visibility: every Makefile target that built a binary for a directory absent from the skeleton (`cli/swarm`, `cli/supervisor`, `cmd/crm-mcp`, `cmd/crm-mcp-http`) was removed, along with their `.PHONY` entries and the dead `GO_DIRS` references. |

## Deferred to later phases (explicitly not done in this harvest)

- **Module-path rename.** The skeleton ships on the poc's existing Go module paths and
  directory layout (fidelity-first, per the Phase 1b/1c split ratified mid-harvest —
  `factory/target-structure.md`). The ratified target structure
  (`backend/internal/modules/…`, `frontend/src/features/…`) is a **Phase 1c** mechanical
  restructure, done as a dedicated pass against an approved before→after mapping doc
  once Phase 1b is fully green on the poc layout. No module-path or directory-layout
  changes were made in this task or anywhere in Phase 1b.
- **`docs/` (the spec).** Per design doc §3.2, `skeleton/docs/` — product, architecture,
  subsystems, quality, recipes, decisions, glossary — is Phase 2 work. It does not exist
  in this skeleton.
- **Feature migrations 000026–000061** (minus the 8 backports) and all feature code they
  backed return later, each with its own regenerated ticket.
- **`ci.yml` re-enablement** — see the CI row above; kept manual-only, enabling any
  disabled job is a reboot-time decision, not a harvest-time one.

## Phase 1c — backend restructure rename map

Backend module merge (Task 2, commit `ebd7fa1`) and the backend
`internal/modules/platform/shared` restructure (Task 3) are separate, sequential passes,
per `factory/1c-restructure-mapping.md`. This section records Task 3's rename map — file
content is unchanged except import blocks (gci-reordered) and the two authorized
package-clause renames (below), plus identifier exports and small documented local
copies where the extractions required them (detailed per-file in the extraction
notes); see `.superpowers/sdd/task-3-report.md` for the full move manifest, mapping
deviations, and gate outputs.

| Old (`backend/internal/`, post-Task-2) | New (`backend/internal/`) |
|---|---|
| `{ids,prov,crmctx,trust,obs}` | `shared/kernel/<name>` |
| `errs` | `shared/apperrors` (dir rename only; package clause stays `errs`) |
| `{datasource,model,connector,workflow,retrieval,migrate,mcp,wellknown}` | `shared/ports/<name>` |
| `{blobstore,keyvault}` | `platform/<name>` |
| `crm-audit` | `platform/audit` |
| `cmd/api/{httperr.go,middleware.go}` | `platform/httpserver/` — **package rename**: `main` → `httpserver` |
| `crm-auth` | `modules/identity` |
| `cmd/api/{auth_handler.go,members_handler.go}` | `modules/identity/transport/` — **package rename**: `main` → `transport` |
| `crm-approvals` | `modules/approvals` |
| `crm-gdpr` | `modules/gdpr` |
| `crm-core` (everything except handler_person.go) | `modules/directory` — includes Person's entity/store (mapping deviation: see task-3-report.md) |
| `crm-core/handler_person.go` | `modules/people/transport/` — carries local copies of shared HTTP helpers, references `modules/directory`'s exported Person API |
| `crm-core/authz.go` | `modules/directory` (mapping deviation: used only by handler_audit_history.go, not handler_person.go) |
| `pkg/jurisdiction` | unchanged (claimed by `pkg/` in Task 2) |
| `crm-contracts/go/types` | `backend/internal/contracts/types/` (relocated in Task 4 — see contract relocation below) |

`backend/.go-arch-lint.yml`, `scripts/check-audit-coverage.sh`, `scripts/check-rls-store-path.sh`
rewritten to the new component/scan-root paths.

## Phase 1c — second prune (Task 1)

Before any file moved, a second evidence-based prune ran against the still-poc-layout
tree (mapping doc §A3): `brief.go`, `brief_facts.go`, `contextgraph.go`,
`handler_next_step.go` + their tests — Morning-Brief and context-graph feature remnants
that survived Phase 1b only because they still compiled; the registrar audit found
nothing mounts them (their HTTP surfaces 404). A review-finding extension
(commit `ae176e4`) caught one more unwired remnant the same sweep had missed: the
lead-score engine (`ComputeLeadScore` and its `decay`/`meetingStatusHeld` helpers) —
same class of dead-but-compiling code. Also dropped: the orphaned
`@testing-library/user-event` frontend dependency (verified unused). Spine
stores/domain (org/deal/pipeline/stage/activity/lead, including `NewLead`) were kept —
`crm-gdpr` erasure/SAR references them in production code, their tables are platform
schema, and RLS/audit integration tests exercise them as platform guarantees. The
contract (`crm.yaml`) was left untouched (D-H2 adjudication: no corresponding action
removal). Net effect: `crm-core` (pre-merge) went from 21 prod files to ~17.

## Phase 1c — backend module merge (Task 2)

`crm` (the platform Go module) and `cmd/server` merged into one `backend/` module
(`github.com/gradionhq/margince/backend`), per mapping doc §A1 — the target layout has
one backend module with `internal/` packages, not the poc's two-module split.
`crm-de` stayed a separate sibling module (ADR-0042 compile-time jurisdiction pack);
`cli/craft` and `cli/crm-gen` stayed separate root dev-tool modules. `go.work` members
became `./backend ./crm-de ./cli/craft ./cli/crm-gen`. The one package the merge singled
out for its own top-level seam: `crm/jurisdiction` → **`backend/pkg/jurisdiction`** (a
`pkg/` root, not `internal/`, because `crm-de` imports it across the module boundary and
Go's `internal/` visibility rule would otherwise block that import). All other
`github.com/gradionhq/margince/crm/...` import paths were mechanically rewritten to
`github.com/gradionhq/margince/backend/internal/...` (scripted `sed` + `gci`, no manual
edits, no logic changes). `backend/.go-arch-lint.yml` gained a `cmd/api` exclusion
(scope-preserving) as part of the merge.

**The gitignore-swallow incident.** A pre-existing `coverage*` glob at the top of
`skeleton/.gitignore` (from the Phase 1b harvest) was broad enough to also match
`skeleton/backend/internal/crm-audit/coverage_gate_test.go` — a real, load-bearing test
file, not a coverage artifact — making it invisible to `git status`/`git add` across
every commit since the 1b harvest. Task 2 discovered and fixed this: a full repo sweep
(1,753 files) confirmed `coverage_gate_test.go` was the **only** file the overbroad
pattern had swallowed. The file was recovered (force-added, now tracked), and the
`.gitignore` pattern was narrowed to stop matching source files while still excluding
real coverage output (`*.out` already independently covers bare `coverage.out`;
the narrowed rule targets `coverage/` directories and `coverprofile`-style artifacts,
not anything named `coverage_*.go`).

## Phase 1c — backend reshape (Task 3)

Task 3 (commit `fa9b737`, 157 renames) applied the rename map in the table above,
moving the merged `backend/internal/` tree into its final
`modules/<name>/{app,adapters,transport}`, `platform/<name>`, and `shared/{kernel,ports}`
homes. Three deviations from the mapping doc's file-granularity ideal were confirmed
during execution — each judged an honest, evidence-based call rather than a mapping
violation, and recorded here so the "deviation" isn't buried in a task report:

1. **`modules/people` is transport-only.** The mapping doc's aspirational split
   (`modules/people/{domain,adapters}` alongside `modules/people/transport`) assumes
   Person's entity and store methods can be cleanly separated from the rest of
   `crm-core`. They can't yet: Person's store methods share the same RLS transaction
   manager (`store.go`'s `withWorkspaceTx`) and struct definitions
   (`crmcore.go`) as the spine stores (org/deal/pipeline/stage/activity/lead), with no
   clean seam between them today. `modules/people/transport/` (the `handler_person.go`
   descendant) is real; Person's domain/store code stayed put in `modules/directory`
   alongside the spine. See the Follow-up tickets section below.
2. **`store.go` and `crmcore.go` live in `modules/directory`, not
   `platform/database`.** The mapping doc proposed `store.go`'s RLS transaction manager
   (`withWorkspaceTx`) as a `platform/database` seam. It stayed in `modules/directory`
   instead: it's currently only consumed by directory's own stores (the Person/spine
   split above means there's no second module consumer yet to justify promoting it to a
   shared platform seam), and `crmcore.go` holds the shared struct definitions all of
   directory's stores depend on. Promoting it to `platform/database` is future work once
   a second module actually needs it.
3. **`crm-audit` landed at `platform/audit`** exactly as mapped, but transiently existed
   at `internal/crm-audit` between Task 2 (module merge) and Task 3 (reshape) — noted
   here only because the gitignore incident above references that intermediate path.

`backend/.go-arch-lint.yml` was fully rewritten: components now match the
`internal/{modules/*,platform/*,shared/*}` dirs, same dependency philosophy, test-file
exclusion kept. `check-audit-coverage.sh` and `check-rls-store-path.sh` scan roots moved
from `crm/crm-core` to the new module/platform adapter dirs.

## Phase 1c — frontend restructure rename map

Task 6 (commit follows this entry) renamed `web/` → `frontend/` and reorganized
`src/` from the flat `api/components/pages/store/styles/ui` layout into an
`app/features/shared/lib` layout, per `factory/1c-restructure-mapping.md` §C. File
content is unchanged except import-path strings (relative-path rewrites only — no
logic changes); see `.superpowers/sdd/task-6-report.md` for the full move manifest,
mapping deviations, and gate outputs.

| Old (`web/src/`) | New (`frontend/src/`) |
|---|---|
| `main.tsx`, `App.tsx`(+test), `index.css` | `app/` |
| `components/ProtectedRoute.tsx`(+test) | `app/` |
| `pages/ShellPlaceholderPage.tsx` | `app/` |
| `ui/{AppShell,WorkspaceRail,TopBar,railNav,RailIcon}` | `app/shell/` |
| `pages/LoginPage`(+test) | `features/identity/routes/` |
| `ui/LoginForm.tsx` + `components/LoginForm.test.tsx`+`.stories.tsx` | `features/identity/components/` (impl+test+story reunited — see deviation below) |
| `store/authStore.ts`(+test) | `features/identity/store/` |
| `api/auth.ts` | `features/identity/api/` |
| `pages/PeoplePage`(+test) | `features/people/routes/` |
| `components/PersonList.tsx`(+test) | `features/people/components/` |
| `ui/PersonCard.tsx` + `components/PersonCard.test.tsx`+`.stories.tsx` | `features/people/components/` (impl+test+story reunited — see deviation below) |
| `api/people.ts` | `features/people/api/` |
| `ui/{AtomCatalog,ContextMenu,DataTable,SearchField,ToastContainer,UserAvatar,useActiveNavId,forge}` | `shared/ui/` |
| `ui/FieldGuard.tsx` + `components/FieldGuard.test.tsx`+`.stories.tsx` | `shared/ui/` (impl+test+story reunited) |
| `ui/RoleBadge.tsx` + `components/RoleBadge.test.tsx`+`.stories.tsx` | `shared/ui/` (impl+test+story reunited) |
| `styles/*` (6 files incl. DS-gate tests) | `shared/styles/` |
| `api/client.ts` | `lib/api-client/client.ts` |
| `lib/api-client/generated/`, `lib/api-client/contract.test.ts` | unchanged (Task 4's territory) |
| `test/setup.ts` | unchanged |

**Mapping deviation (pre-existing split-file, not introduced by this task):** the poc
tree already had `LoginForm`, `PersonCard`, `FieldGuard`, and `RoleBadge` split across
two directories — the component implementation lived in `ui/<Name>.tsx` while its
`.test.tsx`/`.stories.tsx` lived in `components/<Name>.*`, sharing a basename. Task 6
reunites each trio (impl + test + story) into one directory per the mapping's target
(`features/identity/components/`, `features/people/components/`, `shared/ui/`) —
confirmed via import-graph inspection before moving, not assumed.

`Makefile` fe-*/ds-purity/font-lock/icon-lint targets, `pnpm-workspace.yaml`,
`biome.json`, `.gitignore`, `scripts/gen-types.sh`, `scripts/check-ds-purity.sh`,
`frontend/scripts/{check-font-lock.sh,check-icon-glyph.sh,capture-stories.mjs}`,
`frontend/.storybook/preview.tsx`, and this README rewritten to the new
`frontend/` path. `frontend/package.json`'s `name` field (`@gradion/crm-web`) is
unchanged — cosmetic rename out of scope.

## Phase 1c — contract relocation (Task 4)

`contract/crm.yaml` → **`backend/api/crm.yaml`** (the target structure's "api/ — source
of truth" home). Generated Go types moved `crm-contracts/go/types` →
**`backend/internal/contracts/types/`** (2 import-site fixes, mechanical). Generated TS
types moved to **`frontend/src/lib/api-client/generated/`**; the `@gradion/contracts`
pnpm workspace package (`skeleton/backend/internal/crm-contracts/{package.json,tsconfig.json}`)
was deleted outright — it doesn't survive as a separate package. All 4 consumer sites
(`web/src/api/people.ts` and 3 others) switched from the `@gradion/contracts` package
import to a plain relative import of `../lib/api-client/generated/index.js` — no tsconfig
path alias was introduced. `gen-types.sh`, `scripts/contract-lint.mjs`,
`scripts/check-audit-action-coherence.sh`, and the Makefile's `contract-lint` /
`test-contracts` targets were updated to the new `backend/api/crm.yaml` input path and
`frontend/src/lib/api-client/contract.test.ts` test target.

## Phase 1c — migrations + seed relocation (Task 5)

`infra/migrations/` → **`backend/migrations/`** and `infra/seed/` → **`backend/seed/`**
(pure `git mv`, 100% rename, zero content diff — all 62 numbered migration pairs plus
`dev.sql`/`reset.sql`). Every functional reference was updated to match: the Makefile
(`SEED_DIR` and 7 migrate/seed/test-db targets), `shared/ports/migrate/migrate.go`'s
default `coreDir`, `cli/crm-gen`'s `migrationsDir` const and its 4 path-building call
sites, and `scripts/check-audit-action-coherence.sh`'s migration glob. `infra/` itself
was **not** emptied or removed — `infra/docker-compose.dev.yml` and
`infra/branch-protection.json` stay put; moving them would touch the cli/craft wiring
tests and the G0 workflow for no structural benefit (mapping doc §A5). Doc-content
mentions of `infra/migrations`/`infra/seed` in `HARVEST.md`/`README.md`/this style gate
were deliberately left for this task (Task 7) to update — see below.

## Follow-up tickets (from the 1c ledger, not resolved by this restructure)

- **Decouple Person from the spine transaction** so `modules/people/{domain,adapters}`
  can complete — today `modules/people` is transport-only (see the reshape deviation
  above); Person's entity/store code stays in `modules/directory` until it can be safely
  split out.
- **Untangle the `platform/httpserver` → `modules/identity` edge.** A pre-existing
  coupling, surfaced (not introduced) by the Task 3 reshape: `platform/httpserver`
  currently reaches into `modules/identity` in a way the target's platform/module
  layering is meant to avoid. Needs its own ticket to resolve cleanly.
- **`shared/ui` → `app/shell` imports.** `shared/ui/forge.ts` re-exports `RailIcon` from
  `app/shell/RailIcon.tsx`, and `shared/ui/useActiveNavId.ts` imports `railNav` from
  `app/shell/railNav.ts` — both pre-existing same-directory co-locations in the poc's
  flat `ui/` that became cross-layer imports once `RailIcon`/`railNav` moved to
  `app/shell/` per the mapping. A future ticket should relocate the re-export
  responsibility (e.g. have `AppShell.tsx` import `RailIcon` directly instead of via the
  `shared/ui/forge` barrel) so `shared/` no longer reaches up into `app/`.
- **`crm-gen` scaffold conventions redefined with Phase 2 recipes.** The Task 3 module
  merge updated `cli/crm-gen`'s template import strings to point at
  `backend/internal/<pkg>` (mechanical string fix), but the scaffold's actual generation
  target — which module/layer a freshly generated connector/workflow/tool package should
  land in under the new `modules/platform/shared` layout — is a real design decision
  deferred to Phase 2, when the first Phase-2 recipe exercises `crm gen` for real. The
  `object`/`report` scaffold dirs and the `crm.yaml` contract-hint path were still
  pointing at the pre-harvest `crm/crm-core/...` and `spec/contract/crm.yaml` dead paths
  post-review; fixed post-review (this same 1c final-review pass) to
  `backend/internal/modules/directory/...` and `backend/api/crm.yaml` respectively —
  scaffold conventions overall are still slated for redefinition with the Phase 2
  recipes above.

## Verification

`skeleton/scripts/verify-boot.sh` is the scripted half of gate D-H0 (human-boot-check):
it logs in as the seeded admin over HTTP, confirms the session cookie authorizes a
`/people` read that includes all three seeded people (Alice, Bob, Carol), and confirms
the frontend build produces real output. See `skeleton/README.md` for the full human
boot-check loop (the other half of D-H0 — a person clicking through the app).
