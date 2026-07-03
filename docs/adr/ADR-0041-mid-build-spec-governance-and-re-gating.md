# ADR-0041 — Mid-build spec governance: change-classes, re-gating, and the backlog-DAG gate

- **Status:** Accepted (2026-06-24)
- **Decision log:** A56
- **Relates to:** P3 (contract-first), P11 (static schema), P12 (governance designed-in); ADR-0015 (contract pipeline + CI gates), ADR-0017 (fork-upgrade survival); `spec/product/build-backlog/README.md` (coverage gates + the umbrella/leaf convention), `spec/product/build-backlog/validate_backlog.py` (the machine gate), `spec/product/20-traceability.md` (the tier matrix).

## Context

The spec is decision-complete and the build-backlog is now an **agentic-delivery-ready artifact**: 777 build stories = **676 PR-sized leaf tickets + 101 rollup umbrellas**, machine-verified by [`validate_backlog.py`](../product/build-backlog/validate_backlog.py) as an acyclic dependency graph (0 cycles, 0 dangling refs, 0 duplicate IDs, 0 missing-WP, 0 oversized leaves) with a complete topological build order exported to `backlog-graph.json` for the pipeline to consume.

That guarantee is only worth something **if it survives the build**. During WP0→WP17 the spec *will* change — an ADR gets amended, a capability moves between tiers, a new requirement lands, a leaf turns out bigger than thought, a contract field is added. Until now there was **no written process for re-gating those changes** (it was implied to be "edit the doc + hope"). Without one, edits silently drift the three lenses (product / features / build-backlog) and the contract out of sync, and the pipeline's invariants (traceability, acyclic DAG, PR-sized leaves, tier-agreement) rot. This ADR closes that gap.

## Decision

**Every change to the spec or backlog during the build is classified and re-gated; the backlog validator + contract-drift check are the mechanical CI gates that must stay green.**

### Change-classes and the required re-gating steps

- **Class A — contract change** (`contract/crm.yaml`, `data-model.md`, `events.md`, `interfaces.md`). Contract-first (P3): edit the contract, regenerate types/registries, then update the `Traces` line of every build story that cites the changed path/entity/event. A **load-bearing** contract change also needs an ADR + a `DECISIONS.md` pointer. Gate: contract-drift green; codegen compiles.
- **Class B — scope / tier change** (a capability moves V1-Must ↔ V1-WOW ↔ Fast-follow ↔ Backlog, or a brand-new capability). The product tier (`spec/product/` + `20-traceability.md`) and the `features/` cut-line tag (`[MVP]`/`[TS]`/`[DIFF]`/`[Backlog]`) **must be edited together and must agree** — a disagreement is a defect (the tier-agreement gate). Add or retire the corresponding build stories. Any change to the **V1 scope line** needs an ADR + `DECISIONS.md` pointer. Gate: coverage gate (every `[MVP]` capability / contract endpoint / entity / event / NFR gate → ≥1 build story) + tier-agreement.
- **Class C — decomposition change** (a leaf is found too big mid-build, or a new dependency is discovered). Use the **umbrella/leaf convention** (`build-backlog/README.md`): keep the existing story ID as a rollup umbrella and add lettered/digit children — **never re-ID an existing story** (inbound `Depends on` references must keep resolving). New dependencies are added to `Depends on`; the graph must stay acyclic. Gate: `validate_backlog.py` (0 cycles, 0 dangling, 0 oversized leaves, topo-complete).
- **Class D — pure ADR amendment** (no contract/scope/decomposition delta). Update the ADR body + status, the `DECISIONS.md` pointer, and re-check the `Traces` of stories citing that ADR. Gate: no orphaned ADR references.

### The mechanical gates (run in CI on every spec/backlog PR)

1. **`validate_backlog.py`** — acyclic DAG · no dangling/duplicate IDs · no missing-WP · **no oversized (L/XL) leaf tickets** · topological order complete. The dependency graph (not the WP label) is the authoritative build order.
2. **Contract-drift** (ADR-0015) — generated types match `crm.yaml`; no hand-edited `*_gen` files.
3. **Coverage gate** (`build-backlog/99-coverage.md`) — every `[MVP]` capability, contract endpoint, data-model entity, event, seam, and NFR gate maps to ≥1 build story.
4. **Tier-agreement** — every user story's tier matches its `features/` cut-line tag (no product-vs-features contradiction).

### Authority

- **Founder** signs off Class-B changes that move the **V1 scope line** (and any new/amended load-bearing ADR).
- **Build lead** signs off Class-A additive contract changes, Class-C decompositions, and Class-D amendments, provided all four gates stay green.

## Consequences

- The spec stays **executable** throughout the build: the agentic pipeline can keep trusting `backlog-graph.json` because every change is re-gated against the same invariants that made it trustworthy.
- Changes are **auditable** — each carries its class, its gate results, and (for scope/contract) an ADR pointer.
- **No silent drift** between the contract, the two functional lenses, and the build-backlog: the gates fail the PR instead.
- This is a **process** ADR; it adds no runtime surface and no contract change. It formalizes how ADR-0015's CI gates and the build-backlog coverage gates are applied to *ongoing* edits, not just the initial authoring.
- Cost: every spec/backlog PR runs the validator + drift + coverage checks. These are fast (seconds) and already exist; the only new requirement is discipline in picking the right change-class and editing the paired lens.

## Re-gating log

A dated record of applied changes, each with its class + gate results (the audit trail this ADR mandates).

| Date | Change | Class | Gate result |
|---|---|---|---|
| 2026-06-24 PM | **Build-readiness reconciliation.** (a) Reconciled stale gating docs to the validator (`99-coverage.md`/`20-traceability.md`: 466→**787**; closed the A53 "Gate #3 formally open / zero `B-E18.*`" caveat; repointed every A49/A51/A53 "decompose at WP-entry" item to its landed `B-` stories; closed the L-sizing importability gate). (b) **Authored two missing contract artifacts:** the `/overlay/*` management surface in `crm.yaml` (+ 5 overlay sentinels in `interfaces.md §0`), bound 1:1 to `data-model §12`, with the AC-OV-2/ADR-0018 bounded-equivalence invariant made explicit; and `webhook_subscription`/`webhook_delivery` (S-E10.6) in `data-model §12.5` + the `crm.yaml` NET-NEW block + `api-rate-limits §2.3`. (c) Fixed the M365/Graph connector tier drift (`B-EP05.5a/b` `[TS]`/fast-follow → V1-Must parity, matching its A51 feature cut-line). | **A** (contract: `crm.yaml`/`data-model.md`/`interfaces.md` — additive, no breaking change; codegen must re-emit) + **B** (tier-agreement: M365 build story ↔ `features/02 §1` cut-line) | `validate_backlog.py` **787/787, 0 cycles / 0 dangling / 0 missing-WP / 0 oversized leaves**; `crm.yaml` parses as OpenAPI 3.1; contract-drift = additive (new paths/schemas, no removals). Recorded in `spec/README.md` pickup. |
