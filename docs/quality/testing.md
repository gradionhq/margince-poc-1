---
derives-from:
  - margince-poc/docs/quality/testing.md
  - margince specs/spec/architecture/05-test-architecture.md#4-determinism--why-green-is-trustworthy-the-agent-edit-safety-contract
  - margince specs/spec/contract/seed-and-fixtures.md#3-test-fixtures-back-the-features01-acceptance-criteria
---
# Testing — catch it at the cheapest layer that can prove it

The suite is not a coverage number; it is the guardrail that makes a green run
trustworthy enough to be the merge contract. One rule orders everything: **catch
issues at the cheapest layer that can actually prove the behaviour**, and only climb
when the layer below genuinely cannot express the assertion. The gate registry
(the quality-gates chapter) says *where* the suite blocks; this chapter says *what*
runs, *on what infrastructure*, and *why its green can be trusted*.

## The pyramid

Five layers, ordered by cost, cheapest at the bottom:

1. **Backend unit + contract compliance** — pure logic with no database and no
   network, plus type-level assertions that the generated types still satisfy the API
   contract's structural invariants. Fastest and most granular; the default home for
   any assertion that fits.
2. **Backend integration** — real HTTP handlers over a real database, cache, and
   object store: row-level security, the RBAC matrices, migrations, cross-module
   behaviour. The workhorse layer — it catches and pins most issues.
3. **Frontend unit** — component logic, hooks, and rendering in a simulated DOM with a
   testing library; far cheaper than booting a real browser.
4. **Component stories** — every exported story doubles as a visual regression test,
   rendered in a headless real browser; reserved for behaviour that is meaningless in
   a simulated DOM.
5. **Manual end-to-end** — the **final human gate**, run on the real UI in a browser
   against the live seeded stack. It is not a substitute for automated tests: if a
   scenario can be proved by an integration test, it belongs there, not in the manual
   guide. The manual pass verifies observable behaviour after every automated layer is
   green.

## The three backend lanes

Backend tests run in exactly three lanes, pinned below as the TEST-LANE rows and held
apart by a dedicated gate ([[quality-gates#QG-15]]) so the boundary cannot erode:

- **Unit** — hermetic. A unit test never opens a real database or cache; it uses
  fakes and an in-memory cache stand-in. This lane runs in the aggregate check target
  on every commit.
- **Integration** — build-tagged. The runner provisions every backing service and
  runs test packages concurrently, each isolated on its own **throwaway per-package
  infrastructure**: a private database clone, a private cache database, and a private
  object-store bucket (within a package, tests still run serially). Two rules are
  absolute. **Never skip**: a test that cannot reach its service fails the run — a
  skip is a failure, not a pass, because a silently skipped guardrail is a deleted
  guardrail. **Own your data**: every test creates and scopes its assertions to its
  own workspace, so parallel runs cannot collide.
- **Live-stack UAT** — build-tagged, never tagged as integration. It drives the
  seeded development stack and exists only for harnesses a hermetic test cannot
  cover (stubbed external services, whole-stack flows). It runs as its own required
  CI check.

The aggregate check target runs the unit, frontend, and contract-compliance layers
only; the integration and live-stack lanes each run as their own required CI check on
every pull request, on infrastructure CI provisions. A skip or failure in any lane
blocks the merge.

## RBAC matrix testing

Authorization is tested as **tables, not test functions**: one test function per
dimension, iterated over a shared case matrix. Extending coverage means adding a row,
never writing a new test function for a new role–object–action combination. The tests
re-implement no decision logic — they seed fixture users and roles, drive the real
handler or the real authorization seam, and assert the outcome, so the matrix pins the
production decision path and nothing else.

Five matrix areas exist, pinned below as TEST-RBAC-1..5, each holding one invariant:

1. **Object-level** — the role decides whether an actor may act on an object at all;
   a read-only role can never create.
2. **Row-scope** — an "own" scope narrows visibility to records the actor owns; an
   "all" scope does not narrow it.
3. **Field-mask** — within a readable object, the role decides which fields are
   present and writable; a role with no entry for an object is fully denied, not
   merely masked.
4. **Agent-below-human** — an agent passport's effective scope can never exceed the
   granting human's authority, and tools that need approval still require it.
5. **Auth-state** — each session state (none, valid, expired) yields the correct
   authentication outcome per endpoint.

These matrices are the automated counterpart of the manual identity checks: a new RBAC
combination is proved by a matrix row first; the manual pass is the final human gate,
never the place a combination is proved.

## Frontend test placement

The decision is a short ladder, and the default is always the cheapest rung:

- **Pure logic** — formatting, reducers, hook state, derivations — is a unit test in
  the simulated DOM. Always. Test hooks directly as pure logic; do not re-test a hook
  inside every component that consumes it.
- **Rendering and callbacks** — a component renders correctly for given props,
  callbacks fire on events — is a unit test with the testing library. Feature
  components mock their hooks and assert the wiring (composition, conditional
  rendering, panel glue) rather than re-rendering the whole tree.
- **Real-browser behaviour** — focus crossing a portal, keyboard activation
  semantics, layout measurement, drag with real coordinates — is a story with an
  interaction test, carrying a one-line comment naming the browser-only reason. If
  that comment cannot be written honestly, the test belongs in the unit lane. The
  default for interaction tests is: don't.
- **Visual states a human reviews** — loading, empty, error, dark mode, breakpoints —
  are render-only stories with no interaction test.

Frontend tests live beside the components they pin, under the frontend source tree.

## Contract-compliance testing

A dedicated compliance suite asserts that the types generated from the API contract
satisfy the contract's structural invariants. These assertions are **type-level**:
they fail at compile time, not at runtime, and they run in the aggregate check target.
Everything downstream trusts them — a component test never re-tests contract types,
and a backend handler test asserts contract-typed responses rather than hand-built
shapes. The contract itself is guarded by its own gates in the registry
([[quality-gates#QG-6]], [[quality-gates#QG-7]]).

## Determinism — why green is trustworthy

A guardrail you cannot trust is worse than none, so the suite pins every source of
nondeterminism; the pins live below as the TEST-DET rows.

- **The clock is fixed.** The test harness runs on a fixed clock
  (2026-06-04T12:00:00Z, TEST-DET-1), so every duration rule — the stalled-deal flag,
  retention expiry — is a stable boolean, not a flake.
- **Infrastructure images are pinned by digest** (TEST-DET-2), never by floating
  tags: identical containers on every run, with container reuse per suite for speed
  and data isolation per test for correctness.
- **Every test owns its workspace** (TEST-DET-3): a stable, unique identifier per
  test, written with a conflict-tolerant insert, so parallel runs never collide and a
  database reset predictably removes exactly the rows a test created.
- **Fixture rows declare their provenance** (TEST-DET-4), so seeded test data is
  distinguishable from human and agent capture and never contaminates the capture
  metrics.
- **The schema is static**: custom fields are real columns behind real migrations, so
  the test database is reproducible from the versioned migrations under the backend
  migrations directory — no metadata interpreter to drift.

## Seed doctrine

Seeding is **conflict-tolerant and self-contained**. Test inserts tolerate re-runs (a
duplicate insert is a no-op, not an error), and each test or fixture creates its own
workspace and minimal users in a single transaction — no test ever depends on the
demo seed or on another test's rows. The named fixtures pinned below are the canonical
expression of this doctrine: each one is the **minimal row set that makes exactly one
acceptance criterion machine-verifiable**, loadable independently, deterministic under
the fixed clock, and honest about provenance. When a new acceptance criterion needs
data, the answer is a new named fixture, not a fatter shared seed.

## Where tests live

Backend tests live with the modules they pin, under the backend internal tree;
frontend tests and stories live beside their components under the frontend source
tree; migrations live in the backend migrations directory. The gate registry
(quality-gates chapter) owns where each lane blocks; the acceptance-standards chapter
owns the performance budgets and screen-state floor the suites assert against.

## Appendix

### Parameters — determinism pins
Source: margince specs/spec/architecture/05-test-architecture.md#4-determinism--why-green-is-trustworthy-the-agent-edit-safety-contract @ 5a0b29c; margince specs/spec/contract/seed-and-fixtures.md#3-test-fixtures @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| TEST-DET-1 | FIXED_TEST_CLOCK | `2026-06-04T12:00:00Z` | The harness clock every fixture and duration rule (stalled flag, retention expiry) is evaluated against; lives in the test harness, not the seed. |
| TEST-DET-2 | IMAGE_DIGEST_PIN | Postgres 16 + Redis 7 at pinned image digests | Test containers are pinned by digest, never floating version tags; container reuse per suite, data isolation per test. |
| TEST-DET-3 | PER_TEST_WORKSPACE | one stable, unique workspace UUID per test | Each test creates and owns its workspace via a conflict-tolerant insert; a stable constant (not a random UUID) so parallel runs never collide and a test-database reset predictably removes it. |
| TEST-DET-4 | FIXTURE_PROVENANCE | `source='fixture:<name>'` | Fixture rows carry fixture provenance, with `captured_by` set per the scenario, so seeded test data is excluded from capture metrics and capture/agent paths are exercised honestly. |

### Acceptance — test lanes
Source: margince-poc/docs/quality/testing.md @ a11d6c08

| ID | Lane | Build tag | What runs | Infrastructure | When |
|---|---|---|---|---|---|
| TEST-LANE-1 | Unit | none | Hermetic Go unit tests + contract-compliance assertions; pure logic, no real DB/cache (fakes + in-memory cache stand-in only) | None | Aggregate check target, every commit; merge-blocking ([[quality-gates#QG-16]]); the lane boundary itself is gated ([[quality-gates#QG-15]]) |
| TEST-LANE-2 | Integration | `integration` | Real HTTP handlers over a real database: RLS, the RBAC matrices (TEST-RBAC-1..5), migrations, cross-module behaviour | Runner-provisioned Postgres + Redis + object store (digest-pinned, TEST-DET-2); per-package throwaway database clone + private cache db + private bucket; serial within a package | Own required CI check on every PR ([[quality-gates#QG-23]]); **never skips** — a skip fails the run |
| TEST-LANE-3 | Live-stack UAT | `liveuat` | Harnesses a hermetic test cannot cover: seeded-stack flows, stubbed external services | The seeded development stack, provisioned by CI | Own required CI check on every PR; never tagged `integration` |

### Acceptance — RBAC matrices
Source: margince-poc/docs/quality/testing.md#rbac-matrix-tests @ a11d6c08

Table-driven, one matrix per dimension, in the integration lane (TEST-LANE-2); each
row pins one invariant. Coverage grows by adding matrix rows, never new test
functions; no decision logic is re-implemented in a test.

| ID | Matrix | Invariant pinned |
|---|---|---|
| TEST-RBAC-1 | Object-level | Role × object × action decides whether the actor may act at all — a read-only role can never create. |
| TEST-RBAC-2 | Row-scope | An "own" row scope narrows visibility to records the actor owns; an "all" scope does not narrow visibility. |
| TEST-RBAC-3 | Field-mask | Within a readable object, the role decides which fields are present in the response and which are writable; a role with no entry for an object is fully denied, not merely masked. |
| TEST-RBAC-4 | Agent-below-human | An agent passport's effective scope never exceeds the granting human's RBAC — an over-scoped bind is rejected as such — and needs-approval tools still require a recorded approval. |
| TEST-RBAC-5 | Auth-state | Each session state (none / valid / expired) yields the correct authentication outcome per endpoint — denied versus served. |

### Seed — named fixtures
Source: margince specs/spec/contract/seed-and-fixtures.md#3-test-fixtures @ 5a0b29c

The canonical named fixtures the acceptance-criteria tests load. IDs are the fixture
names, preserved verbatim from the corpus. Each fixture is loadable independently in
one transaction, is self-contained (creates its own workspace and minimal users — see
TEST-DET-3), runs under the fixed clock (TEST-DET-1), and carries fixture provenance
(TEST-DET-4).

| ID | Shape | AC it makes verifiable |
|---|---|---|
| `seeded-mailbox` | A connector-fed inbox stub with 3 inbound emails (known sender, unknown sender, reply-to-existing-thread) + 2 calendar events, each with a stable provider `source_id` | Capture creates exactly one `activity` per message, idempotent on re-sync; auto-create person/org from a new domain (`features/01 §5.1`, `§1.2/§2.1`; `features/04 §3`) |
| `duplicate-email-pair` | One live `person` with `person_email = jane@acme.com`; a create-person request reusing that email | Dedupe-on-create returns 409 + the existing id, no silent duplicate (`features/01 §1.3`) — violates `uq_person_email_dedupe` |
| `merge-pair` | Two live persons A and B (different emails) each owning ≥1 activity, ≥1 employment relationship, and B on a deal as stakeholder | Merge A→B relinks all activities/relationships/stakeholders with zero orphaned FKs; A archived with `merged_into_id` (`features/01 §1.3`) |
| `asked-to-wait-deal` | An open deal whose latest activity is a meeting note dated **65 days** before the fixed clock, and a second deal with activity **5 days** ago | The deterministic stalled flag (`now − last_activity_at > 60d`) is true for the first, false for the second — a stable boolean (`features/01 §4.1`) |
| `engaging-lead` | A `lead` (status `working`, `captured_by='agent:sdr'`) plus a captured **inbound reply** activity from that lead's email | Promotion-on-engagement creates/merges a `person`, sets `converted_from_lead_id`, marks the lead `promoted`, carries history (`features/01 §6.4`; ADR-0008) |
| `cold-send-lead` | A `lead` plus a captured **outbound** email we sent, with **no** inbound reply | Cold-send-no-reply leaves it a `lead`, **no** `person` created — the load-bearing anti-pollution test (`features/01 §6.4`) |
| `promote-into-existing` | A `lead` whose email matches an existing live `person`, plus an inbound reply | Promotion **merges** into the existing person (no duplicate); all lead activities relink; zero orphaned FKs (`features/01 §6.4` merge-not-duplicate) |
| `import-batch-N` | N=50 prospect records from a connector/import path (stable `source_id`s) | Anti-pollution: 0 `person` rows, N `lead` rows; re-run idempotent (`uq_lead_source`); segregation — leads absent from contact search/dedupe/relationship-strength/reporting (`features/01 §6.2`) |
| `stage-in-wrong-pipeline` | A deal-create request with a `stage_id` whose `pipeline_id` ≠ the requested `pipeline_id` | The composite-FK guard rejects it (`features/01 §3.1`; `data-model §6.3`) |
| `multi-currency-rollup` | 3 closed deals in EUR/USD/GBP each with a frozen `fx_rate_to_base`, plus matching `fx_rate` rows | Base-currency roll-up sums frozen rates; summing native `amount_minor` across currencies is forbidden (`data-semantics §1`, AC-DS-FX1) |
| `rbac-matrix` | One record per object owned by a team-A Rep; a team-B Rep, a Manager over team A, a Read-only, an Ops identity | The role × object × action × ownership authorization matrix: team-B Rep gets 403/filtered; amount masked for a non-owner Rep (`features/04 §1`) — the seed behind TEST-RBAC-1..3 |
| `passport-overscope` | A human with Rep RBAC plus an attempt to bind an Agent Seat Passport with `act-with-approval` on `deal` | Bind rejected `422 scope_exceeds_grantor`; agent effective scope = intersection (`features/04 §1`) — the seed behind TEST-RBAC-4 |
| `consent-withdrawn` | A `person` with a `withdrawn` `person_consent` for `marketing_email` (plus its `consent_event` proof row) | Outbound for that purpose is blocked/suppressed (default-deny per purpose); a grant for a *different* purpose does not authorize it (`data-model §3.4`; `features/04 §4`) |
| `consent-default-deny` | A `person` with **no** `person_consent` row for `marketing_email` (state `unknown`) | Outbound marketing for that purpose is suppressed — un-granted ≠ allowed (`features/04 §4`) |
| `retention-expiry` | An over-age `lead` matching a `retention_policy` (action `erase`), plus one identical lead under `legal_hold` | The nightly evaluator erases the first (tombstone + suppression-list, audited) and **skips** the held one (`data-model §3.4`; `features/04 §4`) |
| `archived-record` | One archived `person` (`archived_at` set) | Absent from default lists, still fetchable by id, retained in audit (`features/01 §1.1`) |
| `asked-to-wait-suppressed` | An open deal **65 days** idle under the fixed clock with `wait_until` **30 days in the future** (pilot addition, 2026-07-03) | The wait suppresses the stalled flag while it holds — flag is false (DEAL-FORM-3; DEAL-AC-17) |
| `wait-expired` | An open deal **65 days** idle with `wait_until` = **yesterday** under the fixed clock (pilot addition, 2026-07-03) | An expired wait no longer suppresses — the deal flags stalled (DEAL-FORM-3; DEAL-AC-17) |
| `open-deal-fx` | An open **USD** deal plus a stored daily `fx_rate` row for the fixed clock date (pilot addition, 2026-07-03) | An open deal converts at the daily stored rate ([[data-model#DM-FX-4]]; DEAL-FORM-2) |
| `fx-rate-missing` | An open deal in a currency with **no** stored `fx_rate` row (pilot addition, 2026-07-03) | The roll-up surfaces `fx_rate_unavailable` — never a silent rate of 1 (DEAL-FORM-2; DEAL-WIRE-8) |
