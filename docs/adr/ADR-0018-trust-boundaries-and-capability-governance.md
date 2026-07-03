# ADR-0018 — Trust boundaries: capability governance and agent-edit safety

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md) (2026-06-11, architecture-blueprint research phase). Synthesizes T6 (`foundation/research/t6-trust-boundaries.md`), verified in `verification-log.md`. Extends `05-agent-security.md` (threat model D1–D8), ADR-0013 (one governed surface), ADR-0010 (secure SDLC). Serves Goal 2 safety. Distinct from ADR-0014: this is *trust/capability* boundaries, not *coupling* boundaries.

## Context

This architecture has two agents that can subvert it: the **runtime** agent acting through MCP, and the **build-time** agent editing source on a fork. The runtime governance (`05`/`03c`: Passport, scope∩RBAC, 🟢/🟡 gates, audit) is strong *if the source is intact*. The novel surface is the build-time agent: a buggy or malicious source edit must not be able to silently weaken a control (delete an RLS policy, widen a scope, bypass the audit write).

## Decision

**1. Runtime: an object-capability admission choke-point, structurally enforced.** The MCP authorization spec (2025-11-25 — **verified**: audience-bound tokens, RFC 9728 resource metadata, PKCE, no token passthrough) governs only the transport; scope∩RBAC and 🟢/🟡 tiers are our application layer below it. We make "admission runs before Handle" a near-compile-time fact: a tool handler receives an already-admitted `Capability` value, and the only constructor that mints one lives in the **admission package** with unexported fields. **Correction F-T6:** Go encapsulation is package-scoped, so a sibling file in the same package (or `unsafe`/`reflect`) could still forge one — therefore the capability constructor lives in its *own* package, AND the real backstop is the behavioral CI gate in (3), stated as primary not secondary.

**2. The single SoR/tool seam has no other reachable mutation path** (reinforces ADR-0013's "no backdoor"). The two enumerated system-service exceptions (L3 capture writer, DB migrations) are named, owned controls with their own audit trail — tested as controls, not discovered as backdoors.

**3. Build-time: control-conformance CI gates, beyond generic SAST/DAST.** ADR-0010 mandates SAST/DAST/dep/secret scanning. This ADR adds gates that test each control's *behavior*, so a control resists silent weakening because you cannot delete it and keep the test green:
- **Tenant isolation:** every `crm`-schema table has `ENABLE` **+ `FORCE`** RLS, the app connects as a **non-superuser** role, and a behavioral ∅-query test proves cross-tenant reads return nothing. (PostgreSQL bypasses RLS for the table owner unless `FORCE`, and unconditionally for superuser/`BYPASSRLS` — **verified**; an `ENABLE`-only migration looks secure and is not.)
- **`agent ≤ human`** property test (Passport scopes can never exceed the granting human's RBAC).
- **Admission-choke import-graph invariant** (no path reaches the SoR mutation seam bypassing admission).
- **Audit-write-path invariant** and **egress default-deny** (the `sovereign` profile, A8).

**4. Policy-seam registry via the OPA PDP/PEP split** (**verified** pattern). Replaceable strategies (scoring, routing, dedupe, validation) are pure *decision* functions carrying no authority-bearing types; a frozen, non-overridable *enforcement* seam applies the decision through the same admitted capability + audit. A client-swapped strategy can change a decision but never the authority under which it is applied.

**5. Security-as-structure (D1–D8 map).** Each `05` control gets a named owning module/seam, a `CONTROL:` source marker + CODEOWNERS path (legibility), and a required CI conformance gate (enforcement) — so an agent editing that area knows it is touching a control. Note (correction F-T3b): CODEOWNERS only routes review and needs branch-protection to gate merges; it is a legibility aid, not the enforcement — the conformance gate is.

## Consequences

- **Positive:** the runtime "agent ≤ human" guarantee survives even a hostile source edit, because weakening a control breaks a merge-blocking test; supply-chain integrity targets SLSA L3 (**verified**), forks inherit L1–L2 re-attestation generators (ADR-0010 A27).
- **Negative / bound:** control-conformance tests are real engineering cost and must run against a real Postgres (testcontainers, per ADR-0015) — accepted; tenant isolation is the highest-severity bug class.
- **Open spike:** the precise compile-time guarantee around capability construction (own-package + unexported + lint against forging) needs a build-phase proof; flagged in `verification-log.md`.

## Amendment 1 (2026-06-23, deep red-team) — the guarantee is BOUNDED to the conformance-gated artifact

The "agent ≤ human survives a hostile source edit" guarantee is **not absolute** and is stated as bounded (closes RT-AR-H10): it holds for any build that passes the control-conformance CI gate. A client fork edits the source and runs **its own** CI — a fork that deletes or weakens the conformance gate runs at its own risk, and Gradion's CI never sees it. The guarantee is therefore: *for the unmodified upstream and for any fork that keeps the conformance gate green, weakening a control breaks a merge-blocking test.* Forks are contractually expected (service-contract terms, `business/15`) to keep the governance conformance suite green; a fork that removes it voids the safety claim for that deployment. **Gate-tampering detection (the "test that guards the test"):** the conformance suite includes a **manifest check** — a signed list of required control-conformance test IDs that must be present and passing; CI fails if a required test is absent/skipped, so silently deleting the gate is itself a caught failure on the upstream and on any fork that keeps the manifest check.
