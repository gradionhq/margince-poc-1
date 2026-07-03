# ADR-0010 — EU CRA conformity and a documented secure SDLC are V1 scope

**Status:** Accepted (2026-06-10, with Lars) — DECISIONS A21. Built into `06 §6.9` (product security & CRA), `features/04 §6` (CRA conformity capability), build-plan WP13. Raised by the security-practice cross-assessment (`foundation/spec/feedback/security-practice-crossref.md`): the CRM had strong *architectural* security primitives but none of the CRA *process* artifacts — and those are exactly what Gradion's own cyber-security consulting practice sells.

## Context

The EU Cyber Resilience Act regulates "products with digital elements" placed on the EU market. CE marking against it is enforceable from December 2027; the active-exploitation reporting duty to ENISA lands earlier — the **CRA Art.14 manufacturer reporting duty commences 11 Sep 2026** (the 24h/72h notification obligations for actively-exploited vulns and severe incidents), ahead of the Dec 2027 CE date (cross-ref ADR-0025 Amendment 1 / `eu-certificates.md` §32). The CRM is such a product, sold and/or hosted in the EU. The spec (`05`, `06`, `api-rate-limits-and-abuse.md`) already covered security-by-design *mechanisms* — agent governance, audit, RLS isolation, encryption, rate-limiting — but said **nothing** about the CRA's documentary obligations: SBOM, coordinated vulnerability disclosure, vulnerability-management with patch SLAs, a documented secure SDLC, and the technical documentation + Declaration of Conformity behind a CE mark.

Two further facts make this load-bearing rather than box-ticking:
- **Dogfood credibility.** Gradion is standing up a consulting practice that sells CRA/NIS2/ISO-27001 readiness (the `softwarehersteller.md` playbook *is* the CRA program). A flagship product that cannot show its own SBOM undercuts the pitch. The CRM must meet the standard the practice sells, first.
- **Open-source leverage.** The core is open-source (A24). The SBOM, CVD policy, and DoC can therefore be *public* — simultaneously a compliance artifact, a trust signal, an AI-SEO asset, and a reference deliverable for the practice.

Deferring to "nearer 2027" was rejected: SBOM hygiene and a CVD process are cheap to start and expensive to retrofit onto a grown product, and the practice needs the reference now.

## Decision

Treat full CRA conformity as **V1 scope**. Concretely, V1 ships and the CI/release gates enforce:

1. **SBOM** — a machine-readable Software Bill of Materials (CycloneDX) covering every component and dependency across all repos, **regenerated on every release** and published with the release.
2. **Coordinated Vulnerability Disclosure** — a public CVD/VDP policy, a `security.txt` (RFC 9116) at the well-known path, and an intake + triage process. Includes the CRA **24h ENISA notification** path for actively-exploited vulnerabilities.
3. **Documented secure SDLC** — the `05` threat model as the standing document; SAST, DAST, dependency scanning, and secret scanning wired into CI as gates (not advisory); the mandatory PR review gate (already load-bearing for the agent-editable-source paradigm, `04`) doubles as the security review gate.
4. **Vulnerability management + patch SLAs** — a written process with severity-based remediation windows (e.g. critical within N days), tracked against the SBOM.
5. **Security-update commitment** — a support policy stating the supported-update window (CRA expects a support period appropriate to the product; we commit a 10-year-class horizon for the core), feeding the product/release planning.
6. **Technical documentation + EU Declaration of Conformity + CE mark** — the CRM is a **Category-1** product (self-assessment), not a CRA "important/critical product" (it is not a VPN, password manager, browser, or OS), so conformity is via self-assessment: assemble the technical file (risk assessment, SBOM, secure-SDLC evidence, vuln-management process, test results), sign the DoC, affix the CE mark.

The artifacts that can be public (SBOM, CVD policy, DoC) are published, and are explicitly reusable as the **reference deliverable** for the consulting practice.

## Consequences

- **Positive:** the product meets the standard the practice sells; public SBOM/CVD/DoC are trust + AI-SEO + sales assets; CRA risk is retired early; the secure-SDLC gates raise baseline code quality (which the agent-editable-source paradigm already depends on).
- **Ripple (must be reflected):** `06` gains §6.9 (product security & CRA); `features/04` gains §6 (CRA conformity capability with MVP cut-line + acceptance criteria); the build plan gains **WP13** (SBOM + CVD + secure-SDLC gates) with CI exit gates; `07-risks` notes the CRA obligation and the NIS2-for-Gradion scoping question (A24).
- **Negative / to bound:** CRA classification (Category-1 self-assessment vs important-product third-party audit) is a legal determination — recorded as Category-1 on the current feature set, **to confirm with counsel** before the CE mark is affixed; if a future feature (e.g. a shipped password/secret manager) reclassifies the product, third-party assessment is triggered.
- **Boundary:** this ADR is about the **CRM product's** conformity. Whether Gradion-the-company is an NIS2 entity is a separate determination (A24).

## Amendment 1 — modification transfers conformity (2026-06-10, Jan Moser review; DECISIONS A27)

Jan's point 4: if the codebase can be modified by third parties (the whole P2 paradigm — clients fork and modify the core), that **invalidates our internal SBOM, threat model, and DoC**, because they describe the Gradion release, not the fork.

Resolution — and it is how the CRA actually allocates responsibility, not a workaround:
- **Conformity attaches to the unmodified Gradion-released artifact.** Our SBOM/threat-model/DoC/CE cover what *we* ship.
- **A substantial modification makes the modifier the "manufacturer"** of the modified product under the CRA, transferring the conformity obligation (SBOM/threat-model/DoC for the delta) to them. This parallels the source-available "you own your fork" model (BUSL-1.1, `12-license.md`) already in `customization-portability.md` — modification transfers responsibility along with control.
- **We ship the SBOM + threat-model generators** (already V1 per the decision above) so a fork can **re-attest its own delta**; helping a client re-attest is a billable consulting engagement (P14 — the practice's CRA service applied to the client's fork).
- **By deployment mode:** SaaS multi-tenant (no client forking, `06 §6.3`) stays wholly under Gradion's conformity; **dedicated / source-delivered** forks carry the modifier's obligation. This is stated plainly in the exit/ownership runbook (`customization-portability.md`) alongside the BUSL license entitlements (held-version run/fork under the BUSL grant + the irrevocable two-year Apache-2.0 conversion).

Ripple: `06 §6.9` and `features/04 §6` note the released-artifact scope + the re-attestation tooling; `customization-portability.md` exit runbook adds the CRA-obligation-transfer line.
