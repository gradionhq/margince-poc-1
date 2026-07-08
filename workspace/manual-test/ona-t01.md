# ONA-T01 — Live-UAT Guide: overnight-agent spine

Backend-only, no-migration, leaf-module ticket. The `agents` package is compiled in but not wired
into `routes.go` — there is no UI surface to click and no HTTP endpoint to probe. Every step below
is `[auto]` (a command that passes deterministically against the repo tree) or `[live]` (commands
run against the real booted stack). No `[manual]` step exists.

Run all `[auto]` steps from the repo root unless noted otherwise. `[live]` steps require
`make infra-up` to have completed first.

---

## Step 1: Unit test lane [auto]

**Command:**
```bash
cd backend && go test ./internal/modules/agents/... -v 2>&1 | tail -40
```

**Expected:**

All tests pass and exit 0. Confirm the following named tests appear in the output:

- `TestFixtureAssembler_ImplementsContextAssembler` — proves `FixtureAssembler` satisfies the `ContextAssembler` seam (OVN-EVT-1)
- `TestFixtureAssembler_ReturnsItsCannedView` — canned view round-trips correctly
- `TestFixtureAssembler_PropagatesAssemblerError` — error path propagated
- `TestGateProposals_CompleteFixturePasses` — a complete proposal survives the gate (GATE-AI-1/OVN-AC-1)
- `TestGateProposals_DropsEachMissingFieldIndividually` — missing source, evidence, or confidence each drop the proposal
- `TestRouteTier_D4FloorNamesAlwaysYellow` — all seven D4-floor names (`send`, `outbound`, `archive`, `merge`, `disqualify`, `close-deal`, `enrich`) resolve `TierYellow` under every adversarial payload (OVN-AC-2/GATE-AI-7)
- `TestRouteTier_UnknownActionTypeDefaultsYellow` — default-deny floor holds for unrecognised action types
- `TestRouteTier_DeclaredGreenActionResolvesGreen` — `log_link` resolves `TierGreen`
- `TestRouteTier_NeverSetByCaller` — tier is only ever derived, never hand-set
- `TestBuildBatch_QuietFixtureYieldsHonestEmpty` — zero survivors, no producer error → `RunQuiet` (P4)
- `TestBuildBatch_DegradedFixtureCarriesReason` — producer error → `RunDegraded` with non-empty `DegradedReason`
- `TestBuildBatch_NoisyFixtureIsGroupedAndRanked` — proposals grouped by `ActionType`, ranked by `Confidence` descending (OVN-AC-3)
- `TestOVNAC9_NoDirectEgressImport` — static `go/parser` scan confirms no production `.go` file under `modules/agents` imports `net/smtp` or `net/http` directly (OVN-AC-9)

Final line of output: `ok  github.com/gradionhq/margince/backend/internal/modules/agents/...`

---

## Step 2: Integration test lane [live]

**Command:**
```bash
make infra-up
make test-it DIR=backend/internal/modules/agents/app
```

**Expected:**

All five integration tests pass and exit 0. Confirm these named tests appear:

- `TestStageProposal_ZeroDomainWritesBeforeDecision` — a real `deal` row's `(status, version)` is byte-identical before and after staging a `close-deal` 🟡 proposal; the staged `approval_item` carries `action_type = "overnight.close-deal"` and `requested_by = "agent:overnight"` (GATE-AI-2/OVN-AC-2)
- `TestApplyGreen_WritesAuditAndEvent` — the spy effector is called; exactly one `overnight.applied` emission; exactly one `audit_log` row for `agent:overnight` (OVN-AC-8/GATE-CORE-5)
- `TestHandleDecided_ApprovedExecutesAndAudits` — approved `overnight.log_link` item: effector called, exactly one `audit_log` row for `agent:overnight action=update`, separate from the approvals module's own human-attributed `approve` row
- `TestHandleDecided_RejectedExecutesNothing` — rejected item: effector not called, no extra audit row
- `TestHandleDecided_IgnoresItemsOutsideItsNamespace` — `some_other_module.action` item: effector not called
- `TestRunPass_NoisyFixtureEndToEndThroughTheRealAssembler` — a noisy batch through every seam: one no-guess drop (empty evidence), one `log_link` 🟢 applied (effector called), one `send` 🟡 staged (one pending `approval_item` with `action_type = "overnight.send"`); `RunState = RunNormal`; two groups in result
- `TestRunPass_AssemblerErrorDegradesTheRun` — assembler error alone degrades the run; `Produce` is never called; `RunState = RunDegraded` with matching `DegradedReason`

---

## Step 3: Migration-status check [auto]

**Command:**
```bash
make migrate-status
```

**Expected:**

Output shows the current migration version with no dirty flag — identical to the version on
`origin/main`. No new migration file exists anywhere under `backend/migrations/` for this branch:

```bash
git diff --name-only origin/main -- backend/migrations/
```

**Expected:** empty output — this ticket ships no migration (the `agents` module owns no domain
table).

---

## Step 4: Real-stack boot [live]

**Command:**
```bash
make infra-up
make migrate-up
make seed-reset
```

Then in a separate terminal, start the server:

```bash
make run
```

Back in the first terminal, confirm the `agents` package compiles cleanly in isolation:

```bash
cd backend && go build ./internal/modules/agents/...
```

**Expected:**

- `go build` exits 0 with no output — the leaf package compiles.
- The server starts without any new startup error. Because `modules/agents` is a leaf not wired
  into `cmd/api/routes.go`, there is no new HTTP route, no new startup log line, and nothing to
  click in the UI. Confirm the server log shows no `agents`-related panic or fatal error between
  startup and the first request.

---

## Step 5: Project gate [auto]

**Command:**
```bash
make check-q
```

**Expected:**

Exits 0. All checks pass — lint, type-check, Go unit tests, frontend tests, and every
project-gate script. No existing `approvals`-module test regressed (this ticket is purely
additive from that module's perspective; confirm by grepping the summary output for
`crmapprovals`/`approvals` failures — there should be none).
