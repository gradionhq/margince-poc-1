# ADR-0050 — AI quality is certified per-provider via outcome-contract conformance, not guaranteed uniformly

**Status:** Proposed (2026-06-29). New decision: **DECISIONS A65**. Composes with **ADR-0012/A23** (both LLM paths tested), **ADR-0013/ADR-0009** (one governed surface, enforcement below the model), **ADR-0026/A34** (per-tool 🟢/🟡 tiers), **ADR-0020/A28** (provider-agnostic Client, no AI markup). Operationalizes **P3** (contract-first) + **P12** (governance/evidence). Canonical eval thresholds stay in [`../contract/ai-operational-spec.md §3`](../contract/ai-operational-spec.md); the user-outcome contracts + the cross-provider matrix live in [`../contract/ai-acceptance-catalog.md`](../contract/ai-acceptance-catalog.md).

## Context

Margince runs AI on a **swappable brain**. Two axes of swappability are already locked:

- **Layer-2 tier→model bindings** (`ai-operational-spec.md §1.4`): the baseline AI tasks run on the local-default (Gemma/Llama, non-Chinese) **or** the cloud-default (Haiku/Opus-class) binding — config, not code (A23/ADR-0012).
- **Surface-A BYO agents** (`03b` L1): the user points their *own* agent — Claude, Cursor, Copilot, a local OSS agent — at our governed MCP surface. We control the **tools and their descriptions**, not the model.

This creates a roadblock for acceptance testing: **a deterministic E2E assertion cannot certify a non-deterministic brain**, and "does feature X work?" has no single answer when X may run on any of several models we don't all control. The build-backlog already has the per-task eval harness (B-EP06.23/.24) and the BYO-agent *governance* tests (E10/E12/E19/E20, EP03), but nothing certifies **outcome sufficiency across providers** — "is the result on each supported AI good enough?" — and there is no artifact saying *which* AIs are supported and to what standard.

The naïve fixes are both wrong: (a) pick one model and forbid the rest — contradicts A23 and the BYO-agent value proposition; (b) promise every model works equally — unprovable, and a model regression silently breaks production.

## Decision

**AI quality is certified per provider against a model-independent outcome contract, and the certification is published as a tier. We do not claim uniform quality across models; we grade each one.**

1. **Outcome contracts, not golden outputs.** Each user-facing AI use case has a `AIUC-NN` contract (`ai-acceptance-catalog.md §2`) split into: a **deterministic end-state** (CRM/audit/approval-queue facts, model-independent), a **graded-quality** rubric+band (LLM-as-judge), and a **must-never** set (the red lines). The deterministic + must-never parts are hard gates; the graded part is a banded eval.

2. **The governance/safety layer is uniform by construction and is not part of the matrix.** Because enforcement lives below the model (ADR-0013/0009, the `crm.yaml` `x-mcp-tool` tiers), the 🟡 gate, egress-deny, scope-intersection, and audit hold identically for every AI. A provider that fails any deterministic/governance gate is simply **Not-supported** — there is no "degraded governance."

3. **Only graded quality varies by brain — and that variance is the published output.** Run the catalog as a `{AIUC} × {supported AI}` conformance matrix and emit a **certification tier** per provider: **Certified** (all gates pass + all bands met) · **Supported-degraded** (gates pass, ≥1 band below target but above a ratified floor, surfaced with an honest label) · **Not-supported** (any gate fails or a band below floor).

4. **Surface-A outcome is tested by asserting the substrate end-state, not the model's tokens.** A task-completion harness seeds a deterministic workspace, drives the candidate BYO agent toward a stated goal over the real MCP tools, then asserts the deterministic CRM/audit/approval end-state and judges the artifact. The trajectory may vary by agent; the end-state assertions are identical.

5. **Must-pass set gates the release; the rest may ship Supported-degraded.** `L2:local-default` + `L2:cloud-default` + `SA:claude` must be **Certified** before the WP3/WP4 exit; other columns may GA as Supported-degraded with the label and graduate post-dogfood. A passing certification is valid only for a **pinned model/agent version** — drift re-runs the matrix.

## Consequences

- **Cadence (unchanged from `ai-operational-spec.md §3.3`):** deterministic + must-never gates run per-PR in CI; graded bands + the cross-AI matrix run nightly/per-release. Model/agent-version bump → matrix re-run.
- **Build-backlog:** new EP06 tickets — the **cross-provider conformance matrix runner + certification tiers**, the **Surface-A task-completion harness**, and the **AIUC catalog as a consumed artifact** (B-EP06.26–.28); the WP3/WP4 exit gates extend to "the §2 outcome contracts pass on the must-pass column set." `validate_backlog.py` gains an **AI-coverage rule** (every AIUC covered by a leaf ticket; any AIUC-tracing ticket carries a deterministic-gate **and** an eval-band acceptance bullet); `99-coverage.md` gains an AIUC coverage section.
- **Trace surface:** `ai-acceptance-catalog.md` becomes a first-class trace source alongside `features/07` and `crm.yaml` — AI build stories add `Traces: AIUC-NN`.
- **GTM/trust asset:** the certification tiers can be published as a "supported agents" page — the BYO reality made legible instead of hidden (founder/GTM call whether external or internal-gate-first).
- **No new privileged paths, no AI markup change:** this is a test/certification layer over the existing governed surface; ADR-0013/0020 invariants are untouched.

## Alternatives considered

- **Single blessed model.** Rejected — contradicts A23 (both paths V1) and kills the BYO-agent wedge.
- **Uniform-quality guarantee across models.** Rejected — unprovable and silently broken by any model regression.
- **Test only governance (the existing BYO tests), skip outcome.** Rejected — leaves "does the agent actually accomplish the goal?" untested, which is precisely the user-reported gap this ADR closes.
