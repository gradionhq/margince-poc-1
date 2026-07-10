# Overnight Agent chapter — manual guide (ONA-T01..T05)

Checkpoint for GitHub epic **#104 — ONA (Overnight Agent)**. One ordered walkthrough for a human
tester verifying everything the Overnight Agent chapter shipped:

- **ONA-T01** — the reconciliation-pass spine: the context-assembler seam, the no-guess gate, the
  tier router (D4 always-🟡 floor), the stager, the approval-decided executor, `agent:overnight`
  attribution + audit, and the honest normal / quiet / degraded run states.
- **ONA-T02** — close-date hygiene: deterministic flags + replacement-date compute, per-deal tier
  from the ratified risk policy, the 🟢 auto-apply lane, the 🟡 provisional-confirm lane, and the
  **never-a-past-close-date** invariant across every tier.
- **ONA-T03** — reconciliation proposals: field changes (stage / next-step / amount), new contacts
  to create, and drafted follow-ups — staged 🟡 with proposed-change + evidence + confidence.
- **ONA-T04** — data-vs-claims integrity: deterministic cross-checks (untraced call, proposal-sent
  without mail, meeting without recap, stage unsupported by signal) — each an evidenced 🟡 flag; a
  supported record produces **no** flag.
- **ONA-T05** — stalled-deal recovery: the DEAL-FORM-3 boolean joined to a specific reason +
  clickable evidence + an in-voice recovery draft staged 🟡 **unsent**; asked-to-wait is never
  falsely flagged; approve → send with provenance; edit-then-approve sends the edited version.

> **⚠️ Read this first — this chapter has no screen and no endpoint.**
> The overnight pass is **background work a not-yet-built agent-runner will call as a plain Go
> function** (`RunPass`). It is deliberately *not* wired into the running server: there is **no nav
> link, no page, no `curl` you can run, and no dev-seed fixture** that triggers it. Its human-facing
> output surfaces (the approval inbox, the morning brief) are owned by *other* chapters and are not
> live yet — the `/approvals` HTTP handlers currently return **`501 Not Implemented`**.
>
> So, unlike the other manual guides, **this one is not a click-through or a `curl` walkthrough — it
> is a test-driven walkthrough.** The observation surface for everything ONA shipped is the Go
> test lane, where the pass runs end-to-end against a real Postgres and a fixture-backed
> context-assembler. Each step below is a **Do** (a test command) → **Expected** (a green run, plus
> — in plain words — *what that green proves*), so you finish knowing the behavior was verified, not
> just that a test passed.
>
> This is expected and correct for the epic's scope: ONA-T01..T05 built the pass and its guarantees;
> the scheduler, the live assembler, and the inbox UI are explicitly downstream work (see
> **Known gaps** at the end).

---

## Setup (do this once)

The integration tests need a live, migrated Postgres. You do **not** need the app server or the
frontend for any step in this guide.

- [ ] **1. Bring infra up** (Postgres + Redis):
  ```bash
  make infra-up
  ```
  **Expected:** containers healthy. (`make test-it` below provisions its own throwaway migrated
  database clone per run, so you do **not** need `migrate-up`, `seed-reset`, or `make run`.)

- [ ] **2. Confirm the ONA package builds and its unit tests pass** (no DB needed — this is the fast
  smoke test):
  ```bash
  cd backend && go test ./internal/modules/agents/... && cd ..
  ```
  **Expected:** `ok` for `.../agents` and `.../agents/app` (and any sub-packages). This runs every
  **unit** test — the gate, the tier router, the batch/run-state builder, the close-date-hygiene
  math, and the per-producer no-guess decoders — with no database. If this is red, stop here; the
  integration steps won't be meaningful.

**How the test runner works** (used by every Part below):
`make test-it DIR=<pkg> [RUN=<regex>]` clones + migrates a fresh test DB, then runs
`go test -tags=integration -v -count=1 -run <regex> <pkg>`. The ONA integration package is
**`backend/internal/modules/agents/app`**. There is no ONA-specific make target; `test-it` scoped to
that package is the tool. Run the **whole** ONA integration package in one shot with:

```bash
make test-it DIR=backend/internal/modules/agents/app
```

**Expected:** all integration tests in the package pass. The Parts below break the same package into
per-ticket runs (with a `RUN=` filter) so you can read each guarantee individually and know which
ticket each green line belongs to.

---

## Part 0 — The gate, the tier router, and the honest run states (ONA-T01, unit)

These run without a DB — they are the pure-logic core the pass is built on.

- [ ] **0.1 The no-guess gate drops any proposal missing a resolvable source, evidence, or
  confidence** (OVN-AC-1):
  ```bash
  cd backend && go test -v ./internal/modules/agents/app/ -run 'TestGateProposals' && cd ..
  ```
  **Expected:** green. Proves a complete proposal passes and that dropping **each** required field
  individually removes it — the pass would rather stay silent than guess.

- [ ] **0.2 The tier router is caller-proof** (OVN-AC-2, threat-model D4):
  ```bash
  cd backend && go test -v ./internal/modules/agents/app/ -run 'TestRouteTier' && cd ..
  ```
  **Expected:** green. Proves the D4 floor names (`send, outbound, archive, merge, disqualify,
  close-deal, enrich`) are **always 🟡**, an unknown action type **defaults 🟡** (default-deny), only
  the three declared internal-reversible actions resolve 🟢, and the tier is derived from the action
  — **never settable by the caller**.

- [ ] **0.3 The three honest run states** (quiet / degraded / normal, OVN-AC-3):
  ```bash
  cd backend && go test -v ./internal/modules/agents/app/ -run 'TestBuildBatch' && cd ..
  ```
  **Expected:** green. Proves a zero-survivor run is `RunQuiet` (an honest "nothing needed", never
  padded), a producer failure is `RunDegraded` carrying its reason, and a noisy run is `RunNormal`
  **grouped by action type and ranked by confidence** — triageable, not a dump.

---

## Part 1 — The pass spine, end-to-end (ONA-T01)

- [ ] **1.1 The full pass over a noisy fixture, through the real assembler + real stager:**
  ```bash
  make test-it DIR=backend/internal/modules/agents/app RUN='TestRunPass_NoisyFixtureEndToEndThroughTheRealAssembler|TestRunPass_AssemblerErrorDegradesTheRun'
  ```
  **Expected:** both green. The noisy-fixture test feeds three proposals — a 🟢 `log_link`, a 🟡
  `send`, and a `send` with **empty evidence** — and proves: the empty-evidence one is **dropped by
  the gate**, the 🟢 one is **applied immediately** via the effector, the 🟡 one is **staged pending**
  as `overnight.send` in the approvals repo, and the result is `RunNormal` with the survivors grouped.
  The second test proves an **assembler failure degrades the run** (`RunDegraded` + reason) and
  **never blocks core CRM** — the pass returns whatever survived and `Produce` is never even reached.

- [ ] **1.2 The 🟡 lane writes zero domain rows before a decision, and the 🟢 lane audits exactly
  once** (OVN-AC-2, OVN-AC-8):
  ```bash
  make test-it DIR=backend/internal/modules/agents/app RUN='TestStageProposal_ZeroDomainWritesBeforeDecision|TestApplyGreen_WritesAuditAndEvent'
  ```
  **Expected:** both green. The first stages a `close-deal` (a D4 name → forced 🟡) against a **real
  seeded deal** and proves the deal's status and version are **byte-identical before and after** —
  staging touches no record field (GATE-AI-2) — and that the staged item is attributed
  `RequestedBy = agent:overnight`. The second proves a 🟢 apply emits **exactly one**
  `overnight.applied` event, returns a **rollback handle**, and writes **exactly one** `audit_log`
  row for `agent:overnight` (one audit row + one event per commit).

- [ ] **1.3 The approval-decided executor** (OVN-AC-7, OVN-AC-8):
  ```bash
  make test-it DIR=backend/internal/modules/agents/app RUN='TestHandleDecided_'
  ```
  **Expected:** green. Proves that when a staged `overnight.*` item is **approved**, the effector
  runs **once** and writes exactly one `audit_log action='update'` row for `agent:overnight`; when
  **rejected**, the effector is **never called** (inaction commits nothing outward); and items
  **outside** the `overnight.*` namespace are ignored.

---

## Part 2 — Close-date hygiene, every tier, invariant held (ONA-T02)

- [ ] **2.1 The full close-date sweep across every tier:**
  ```bash
  make test-it DIR=backend/internal/modules/agents/app RUN=TestRunPass_CloseDateHygiene_FullSweep_InvariantHoldsAcrossEveryTier
  ```
  **Expected:** green. Against a fixed clock (`2026-07-09`) and one deal per case, it proves the
  whole policy at once:
  - **The invariant (OVN-AC-1):** after the run, `count(open deals whose expected_close_date < today)`
    is **`0`** — no open deal is ever left claiming a past close date, regardless of tier.
  - **🟢 auto-apply lane:** a clear-overdue date on an active, early-stage, low-stakes deal is
    **auto-applied** to a non-past date (and the rep-`commit`-override deal is forced 🟡 instead —
    the rep override is load-bearing).
  - **🟡 provisional-confirm lane:** a forecast-bearing / late-stage / missing-date deal gets a
    provisional replacement plus a `overnight.close-date-confirm-request` (there are exactly **4**).
  - **Downgrade-and-review:** a deal that has **gone quiet** is downgraded with a
    `overnight.close-date-downgrade-review` whose payload reason is `quiet` / "gone quiet" — *not*
    optimistically re-dated. (The DOWNGRADE-vs-PROVISIONAL framing is asserted explicitly.)
  - **Untouched cases:** won/lost (closed) deals and a healthy unflagged deal are left exactly as
    seeded — a supported record produces no change.

---

## Part 3 — Reconciliation proposals (ONA-T03)

- [ ] **3.1 Field changes + new contacts + drafted follow-ups, end-to-end:**
  ```bash
  make test-it DIR=backend/internal/modules/agents/app RUN=TestRunPass_ReconciliationProduceEndToEnd
  ```
  **Expected:** green. The fixture carries one **valid** and one **deliberately malformed** fact for
  each of the three producers. It proves: exactly **3** surviving groups
  (`overnight.field_change`, `overnight.create_contact`, `overnight.draft_followup`), one item each;
  every malformed fact **dropped by the no-guess gate** (a `close_date` field change is rejected here
  because close dates belong to ONA-T02; a contact with no email *and* no phone is dropped; an
  empty-body follow-up is dropped); every survivor carries non-empty evidence + a confidence; and
  **all three are 🟡** — the effector is **never called**, nothing is auto-applied.

---

## Part 4 — Data-vs-claims integrity check (ONA-T04)

- [ ] **4.1 Supported records stay silent; contradictions flag (never auto-apply):**
  ```bash
  make test-it DIR=backend/internal/modules/agents/app RUN='TestIntegrityCheckProduce_'
  ```
  **Expected:** both green. `...SupportedFixtureAloneYieldsRunQuiet` proves that a day where every
  claim is corroborated (call has a trace, "proposal sent" has an outbound email, meeting has a
  recap, stage matches its signal) produces **zero flags** → `RunQuiet` — a supported record is
  never nagged. `...MixedFixtureFlagsContradictionsNeverAutoApplies` proves that adding one genuine
  contradiction per check yields exactly **4 `integrity_flag`s + 1 `stage_correction`** (5 items),
  each carrying the claim + the missing/contradicting evidence + a confidence, that malformed claims
  are dropped, and that the effector is **never called** — a stage correction is a **🟡 proposal**,
  never a silent move.

- [ ] **4.2 (optional) The per-check unit corroboration** (no DB):
  ```bash
  cd backend && go test -v ./internal/modules/agents/app/ -run 'TestProduce.*Flags' && cd ..
  ```
  **Expected:** green. Proves each check in isolation — e.g. a call with a matching trace → 0 flags;
  a call with no trace → 1 `integrity_flag` (`check="untraced_call"`, confidence `0.75`); a malformed
  claim (not-JSON / missing confidence / missing description) → 0 flags (the no-guess decode gate).

---

## Part 5 — Stalled-deal recovery (ONA-T05)

- [ ] **5.1 Only evidence-supported stalls stage; asked-to-wait is suppressed; drafts are never
  fabricated:**
  ```bash
  make test-it DIR=backend/internal/modules/agents/app RUN='TestRunPass_StalledRecoveryProduceStagesOnlySupportedDeal|TestStalledRecoveryProduce_'
  ```
  **Expected:** all green. Proves: a stalled deal with an evidence signal stages exactly **1**
  `overnight.stalled_recovery` (🟡), attributed `agent:overnight`, with the **specific reason**
  (`champion_quiet`) and a drafted recovery — while a deal marked **asked-to-wait**
  (`wait_until_active`) produces **nothing** (OVN-AC-6, no false stall); a suppressed-only fixture is
  `RunQuiet`; and a genuinely-stalled deal with **no draft** still stages (1 item) but with a
  **null** draft — never a fabricated one (OVN-AC-5 draft degradation).

- [ ] **5.2 On approve, the recovery sends the *persisted* (possibly edited) draft with provenance;
  a draft-less item is a safe no-op** (OVN-AC-7):
  ```bash
  make test-it DIR=backend/internal/modules/agents/app RUN='TestHandleDecided_ApprovedRecoverySendUsesFetchedPayloadAndProvenance|TestHandleDecided_StalledRecoveryDraftlessPayloadNoOps'
  ```
  **Expected:** both green. The first proves an approved recovery logs the follow-up using the
  **fetched** payload (so **edit-then-approve sends the edited version**, in order), ties the
  resulting activity's provenance back to the overnight suggestion, emits one `overnight.applied`
  per send, and records the logged-activity id as the audit **rollback handle**. The second proves an
  approved item whose payload has a **null draft** logs nothing — a safe no-op, never a blank send.

---

## Part 6 — The negative guarantee (ONA-T01, OVN-AC-9)

- [ ] **6.1 No unattended multi-channel auto-send path ships:**
  ```bash
  cd backend && go test -v ./internal/modules/agents/ -run TestOVNAC9_NoDirectEgressImport && cd ..
  ```
  **Expected:** green. This is a **static import check**: it fails the build if any file in the
  agents module imports an outbound-egress package directly. It proves that **all** outbound must
  route through the approvals 🟡 gate — there is no code path by which the pass could send on its own.

---

## Automated counterpart

The single command that covers everything above (and is the merge gate) is the integration lane; the
unit half runs with no DB.

| Command | What it proves |
|---|---|
| `cd backend && go test ./internal/modules/agents/...` | The **unit** half with no DB: gate (no-guess), tier router (D4 floor + default-deny + caller-proof), batch/run-states, close-date-hygiene math, per-producer decoders, and the OVN-AC-9 static import check |
| `make test-it DIR=backend/internal/modules/agents/app` | The **whole ONA integration package** on a throwaway migrated DB: T01 spine end-to-end + zero-writes + ApplyGreen audit + executor, T02 close-date full sweep + invariant, T03 reconciliation, T04 integrity, T05 stalled recovery + send-on-approve |
| `make test-integration` | The full repo integration lane (parallel, zero-skip) — includes the ONA package alongside every other chapter (needs `make infra-up` first) |

If a step doesn't match what you see:

1. Open the owning subsystem chapter `docs/subsystems/overnight-agent.md` for the full acceptance
   criteria and gate IDs (OVN-AC-1..9, OVN-PARAM-*, OVN-EVT-*, OVN-WIRE-*).
2. `docs/quality/acceptance-standards.md` is the GATE-AI-1/2/3/7 and STATE-SP-1/2 floor; the D4
   always-🟡 names live in `docs/quality/threat-model.md`.

> **Known gaps flagged during testing** (all expected for this epic's scope — the pass shipped, its
> drivers and surfaces are downstream work):
> 1. **No runner / scheduler / trigger.** `RunPass` is a plain function with no caller in production
>    yet — a human cannot trigger the pass outside the test lane (`docs/subsystems/overnight-agent.md`
>    "Out of scope"; the agent-runner chapter owns this).
> 2. **No live context-assembler.** Only a fixture-backed assembler exists; the real event-bus-backed
>    one (`cg:overnight-agent`) is future T02-T05 runner work.
> 3. **The approval inbox is not live (OVN-GAP-2 companion).** The `/approvals` HTTP handlers return
>    `501`, so the pass's staged output cannot yet be observed or approved over the wire or in the UI.
> 4. **OVN-GAP-1: no rollback wire operation.** 🟢 applied items carry a rollback handle in the audit
>    trail, but the contract defines no undo/rollback endpoint yet.
>
> Until #1–#3 land, a live end-to-end demo (a real day of capture → a real morning inbox) is not
> possible, and this test-driven guide is the honest way to verify what the chapter delivered.
