# ADR-0045 — A code-craftsmanship gate: a learning Critic Agent that hard-blocks AI-slop, with an in-source fix-loop

**Status:** Accepted (2026-06-25, founder). Recorded as **DECISIONS A60**. Operationalizes **P3**
(code quality *is* a product feature) and **P8** (beautiful by default) for the *source*; composes with
ADR-0010 (secure SDLC / CI gates), ADR-0014/ADR-0015 (structural + contract CI gates), ADR-0016 (`AGENTS.md`
+ DO-NOT-TOUCH), ADR-0041 (mid-build re-gating), P1/P12. Introduces architecture docs
`15-code-craftsmanship.md`, `16-craftsmanship-review-agent.md`, `17-craft-gate-learning.md` (Blueprints O/P/Q)
and a new platform epic for the build. Sibling: ADR-0046 (external-contributor AI policy).

## Context

Margince is built largely by AI coding agents (the agentic build practice, A39/ADR-0002 Am.1) and is
**open-sourced for public review**. The existing quality stack — the doc-03 structural fitness functions
(imports/RLS/contract/provenance/drift), `golangci-lint`, SAST/DAST/SBOM (ADR-0010), the test architecture
(doc 05) — proves the code **works, is safe, and is consistent.** None of it proves the code is **beautiful.**

This is a real, documented gap, not a vanity one. AI-generated code carries characteristic "tells" —
over-commenting, defensive-programming noise, premature abstraction, textbook-uniform naming, type escape
hatches, surface polish over honest edge cases, dead speculative code, untrustworthy dependencies, oversized
unexplained PRs. Field data (CodeRabbit/Veracode 2025) puts AI PRs at ~1.7× the defects and ~3× the
readability problems of human PRs, *and the slop passes every automated check* — it compiles, tests pass,
lint is clean. Open-source maintainers (OCaml, QEMU, NetBSD, tldraw) are now **categorically rejecting**
unexplainable, slop-flooded AI contributions. For a product whose **source is the customization layer (P2)**
and a **product surface (P3)**, and which invites the world to read it, ugly core is a product failure and a
reputational liability.

P8 already solved the analogous problem for the UI: it separates **drift (consistency, mechanically checked)**
from **quality (distinctiveness/polish, assessed by a heuristic rubric)**, precisely because *"a consistently
mediocre UI still passes the drift check."* The same split applies to code — the doc-03 gates are the drift
check; what is missing is the **quality rubric for code** and an enforcer for it.

Two further requirements shape the decision. (1) The factory is **autonomous**: a gate that only leaves a
human-readable comment is a dead end — the local build agents need a *machine-actionable* fix signal and a way
to *prove resolution*. (2) Taste is fuzzy: a blunt blocking gate would wedge the pipeline on false positives,
and a gate this load-bearing must not silently rewrite itself (the reproducibility/auditability P12 demands).

## Decision

**Add a third quality gate — a craftsmanship Critic Agent — that reviews every PR against a normative beauty
rubric, hard-blocks (no override) on high-confidence slop, writes machine-readable fix-instructions into the
source for the local agents to resolve and strip, and improves over time through a governed (versioned,
eval-gated, human-ratified) learning flywheel.** Four parts:

**1. The standard (doc 15, Blueprint O).** A normative craftsmanship standard: binds the Google/Uber Go style
guides + "Go Code Review Comments" (and the `web/` React/TS conventions) as the idiomatic floor, then adds the
**anti-tell catalog** (the ten machine-slop tells turned into checkable rules), a **positive rubric**, and a
**severity model**. The standard is also taught upstream via a new **`## Craftsmanship`** section in every
`AGENTS.md` (extends ADR-0016 §4 / doc 11 §6) — the rules the build agent reads *before* writing.

**2. The gate (doc 16, Blueprint P).** An adversarial Critic Agent as a **required GitHub check run**, gate-2
of the verification stack (deterministic gates → this → human acceptance). Runs **only after** the doc-03/
ADR-0010 gates are green, in a **fresh session** (no authoring context). Emits **canonical JSON**
(`verdict`, `findings[]` with `category`/`severity`/`confidence`/`rationale`/`suggested_fix`). **Merge-
blocking with no human override**, made safe by a calibrated **BLOCK-eligibility rule**: it may block **only**
when a finding is `severity:BLOCKER AND confidence:high AND` objectively statable; every MAJOR/MINOR or
low-confidence finding degrades to a **non-blocking inline comment**. Default model Claude (per stack);
spec is model-neutral.

**3. The closed feedback loop (doc 16 §3).** On BLOCK, a deterministic **annotator** writes greppable, line-
anchored **`CRAFT-FIX[id]`** markers into the source (idiomatic to the existing `CONTROL:`/`JURISDICTION:`
marker family). The local build agent's `AGENTS.md` contract: **fix the code, delete the marker.** A new
deterministic, merge-blocking **residue gate** (added to doc 03) fails if any `CRAFT-FIX`/`CRAFT-DISPUTE`
marker remains in the tree — making markers **self-cleaning, leak-proof** (none ever reaches the public tree),
and forcing the loop to **converge** (the agent re-runs on every push, so removal-without-fix re-issues the
finding). A **`CRAFT-DISPUTE[id]`** marker (also residue-blocking) routes a genuine false positive to an
out-of-band **human adjudication queue** — *not* a merge override — reconciling "no override" with "not wedged
forever."

**4. The governed learning flywheel (doc 17, Blueprint Q).** The gate is a **pinned, versioned tuple**
`(prompt, rubric, exemplar-set, model)` stamped on every verdict — reproducible and auditable (P12).
**Learning signals:** dispute adjudications (gold), spot-audits of auto-PASSed PRs (catch false negatives),
post-merge defects, resolved-fix pairs. These feed a **Reflexion-style exemplar memory** and a **growing
golden set** that **eval-gates every version promotion** (BLOCK precision must stay ~100%, or auto-rollback).
Recurring rubric errors are fixed by **ADR amendment, not drift**. The most frequent blockers are promoted
**upstream into the authoring guardrails** (part 1), so the **block rate falls over time.** Explicitly **not**
autonomous self-modification — the "Zombie agent" counter-pattern is rejected.

## Consequences

- **Positive:** closes the P3/P8 gap for code with teeth, not aspiration. The codebase a customer/partner/
  agent reads — and the public sees — is held to a senior-human bar. The loop is fully autonomous-factory-
  compatible (machine-actionable, self-proving), and the residue gate guarantees no review scaffolding leaks
  into the open-source tree. Governed learning means the gate gets better *and* stays safe to leave
  merge-blocking. The upstream promotion makes craftsmanship cheaper over time (fix it at authoring, not at
  review).
- **Why hard-block / no override (founder call):** an override path on a quality gate becomes the default
  escape hatch under deadline pressure, and the gate decays to advisory (the documented rubber-stamp failure).
  The calibrated BLOCK-eligibility rule (high-confidence + objective only) + the dispute→adjudication channel
  provide the safety valve instead, **without** a per-PR override.
- **Negative / honest limits:** (a) a heuristic gate is **non-deterministic** — flagged as such in doc 03,
  distinct from the four structural gates; its trustworthiness rests entirely on the doc-17 calibration
  (golden set, ~100% BLOCK precision), which is real, ongoing platform work. (b) The agent costs tokens per
  PR (bounded: runs only after deterministic gates pass). (c) Adjudication is a **human** queue — small by
  design (only disputed high-confidence blocks), but non-zero; if it grows, that is itself the signal that
  precision has regressed. (d) The standard must be *taught* (the `## Craftsmanship` guardrails) or the block
  rate stays high — the flywheel's §6 upstream loop is the mitigation, not an afterthought.
- **Stays inside locked decisions:** the gate adds *no* runtime config (P1) and *no* privileged surface
  (ADR-0013); it is CI/SDLC tooling under ADR-0010, governed as structure (P12). It does not weaken any
  structural invariant — it is strictly additive to the gate stack.
- **Relationship to other decisions:** **extends ADR-0016** (`## Craftsmanship` in `AGENTS.md`), **ADR-0010/
  ADR-0014/ADR-0015** (a new gate alongside the existing CI gates), **doc 03** (adds two rows: the heuristic
  craftsmanship gate + the deterministic marker-residue gate); composes with ADR-0041 (rubric amendments run
  through re-gating), ADR-0036 (T7 edge cases incl. concurrency). Sibling **ADR-0046** governs the same
  standard for *external* contributors.
- **Scope:** new architecture docs `15`/`16`/`17` (Blueprints O/P/Q); two new doc-03 gate rows; a
  `## Craftsmanship` block added to the root + per-module `AGENTS.md` template; a new **platform epic** (the
  build stories: the agent + annotator + residue gate + golden-set/eval harness + adjudication queue) under
  `product/build-backlog/`, landing in the build repo. No change to `crm.yaml`, `data-model.md`, or any
  product epic.
