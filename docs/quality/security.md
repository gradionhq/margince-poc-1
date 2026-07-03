---
derives-from:
  - margince-poc/docs/quality/security.md
  - margince specs/spec/narrative/06-nonfunctional.md#69-product-security--eu-cra-conformity
  - margince specs/spec/narrative/06-nonfunctional.md#62-security
---
# Security — one disclosure door, a conformity posture that ships with every release

Public vulnerability disclosure, internal triage, and EU Cyber Resilience Act
conformity are one surface: a single governed intake path for reporters, a runbook
that turns policy into consistent handling, and a release train whose every
artifact carries the evidence an auditor needs. The disclosure half is proven in
the running skeleton; the conformity half is the process layer the factory builds
around it.

## What it's for

This surface gives reporters a clear path for good-faith vulnerability disclosures
and gives the team a consistent internal path for triage, response, and
escalation. It keeps public security intake narrow, predictable, and easy to
follow — and it makes the product placeable on the EU market: the CRM is a
product with digital elements, so CRA conformity (SBOM, coordinated disclosure,
secure SDLC, patch SLAs, CE marking) is V1 scope, not a deferral (A21/ADR-0010).
Because the core is open-source, every artifact that can be public is — a trust
signal and the dogfood reference for Gradion's own CRA-readiness practice.

## Principles it serves

- **P3 — Agent-readable by construction.** The disclosure path is explicit,
  documented, and single-sourced so it can be reasoned about without guesswork.
- **P7 — Own your data.** Reporters get a clear, non-proprietary public intake
  path; customers get signed, verifiable, self-attestable releases.
- **P12 — Governance is designed in.** Triage, safe-harbor, escalation, and
  conformity evidence are part of the product surface, not an afterthought.

## The disclosure surface

The public route serves the well-known security text response with the report
contact, an expiry date, and the policy pointer (SEC-CVD-4). That contact is the
single intake path (SEC-CVD-5) and is reused as the source of truth wherever the
disclosure workflow refers to a reporter address — the public text, the policy,
and the runbook stay aligned instead of drifting through duplicated literals.

The policy defines what is in scope, what is out of scope, the safe-harbor
expectation (SEC-CVD-6), and the response timeline a reporter should expect:
acknowledgment within one business day (SEC-CVD-1) and initial triage within
three (SEC-CVD-2). In scope are defects that could create a security impact on
the service, its public routes, or the data they expose or mutate; out of scope
are routine support questions, feature requests, and reports that describe no
security effect. Good-faith research that follows the policy is treated as
authorized security work.

## Triage runbook

Internal handling starts at the same published contact. Confirm receipt, reduce
the report to the affected surface and impact, reproduce when needed, and assign
ownership for remediation or response; keep the reporter updated when new
information changes the expected timeline. If the report indicates an actively
exploited vulnerability, follow the CRA Article 14 path and make the ENISA
notification within its twenty-four-hour window (SEC-CVD-3). When an incident or
vulnerability materially affects a customer deployment, Gradion notifies affected
customers without undue delay so a NIS2-regulated customer can meet their own
reporting clock (SEC-CVD-7). Close the loop only after the issue is understood,
the response path is chosen, and any external communications have been made
through the same governed channel.

## The CRA release train

Conformity is a property of the release, produced continuously, not a binder
assembled at audit time.

Every release regenerates and publishes a machine-readable software
bill-of-materials, produced and gated in CI (SEC-CRA-1). Every artifact is
keyless-signed and carries a build-provenance attestation, and verification is
mandatory before apply — an operator's cluster refuses an unsigned artifact at
admission (SEC-CRA-2). Static analysis, dynamic analysis, dependency scanning,
and secret scanning are wired into CI as blocking gates, not advisory reports,
and the mandatory review gate that already protects agent-edited source doubles
as the security review gate (SEC-CRA-3); the standing threat model is the
threat-model chapter, and the CI half of the scanning already runs in the gate
registry ([[quality-gates#QG-3]], [[quality-gates#QG-5]]).

Vulnerabilities are remediated on severity-based clocks — the Critical, High,
Medium, and Low targets pinned below (SEC-PATCH-1 through SEC-PATCH-4) — tracked
against the published bill-of-materials, with one honest split: Gradion
**publishes** within target; the client or operator **applies** on their own
change window (SEC-PATCH-5), because air-gapped and regulated installs own their
apply cadence just as they own their disaster recovery. Actively exploited flaws
skip the queue onto the out-of-band fast path, shipped as an isolated, minimal,
cherry-pickable diff (SEC-PATCH-6). Releases follow a monthly stable cadence with
patch releases as needed (SEC-PATCH-7), plus an extended-support line for
regulated and on-prem installs with security backports across recent minors
(SEC-PATCH-8), carrying a decade-class support commitment for the core
(SEC-PATCH-9).

The product self-assesses as CRA Category 1: technical file, signed Declaration
of Conformity, CE mark — with counsel confirming the classification before the
mark is affixed, since a future feature could reclassify it (SEC-CRA-4).
Conformity attaches to the **unmodified Gradion release**: a substantial fork
modification makes the modifier the CRA manufacturer of the modified product, and
Gradion ships the attestation generators so a fork can re-attest its own delta
(SEC-CRA-5). How far a fork can drift and still be patched is graded, not vague:
in-seam additive changes are patched mechanically and guaranteed, shared-function
touches are guaranteed but supervised, and core-invariant overrides carry no
guarantee by design (SEC-PATCH-10 through SEC-PATCH-12). Everything above is
bundled per release into one downloadable EU compliance pack, so a customer's
CRA, NIS2, and GDPR auditors get a single artifact set (SEC-CRA-6).

## Auth posture (summary)

The mechanics of identity are owned by the identity subsystem chapter and are not
pinned here; the posture in one breath: opaque server-side sessions with
short-lived, rotating tokens; single sign-on and multi-factor included in the
flat tier and required for privileged admin accounts on dedicated installs; TLS
everywhere with no plaintext internal hops; encryption at rest across database,
object store, and the AI substrate — embeddings of customer data are customer
data; and no secrets in source or images, with per-workspace connector and agent
credentials stored encrypted, scoped, rotatable, and revocable. Agent-action
authorization — the part that makes this product unusual — is the threat-model
chapter's subject.

## Where it lives

The disclosure surface lives in the well-known module of the skeleton; the
conformity artifacts are produced by the release pipeline. Sibling chapters: the
threat-model chapter (the standing security analysis), quality-gates (the CI
gates that block), acceptance-standards (the cross-cutting floor).

## Appendix

### Parameters — patch policy
Source: margince specs/spec/narrative/06-nonfunctional.md#69-product-security--eu-cra-conformity @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| SEC-PATCH-1 | REMEDIATION_CRITICAL | 72h | Publish target for Critical-severity fixes (starting anchor, recalibrated on dogfood). |
| SEC-PATCH-2 | REMEDIATION_HIGH | 7d | Publish target for High-severity fixes. |
| SEC-PATCH-3 | REMEDIATION_MEDIUM | 30d | Publish target for Medium-severity fixes. |
| SEC-PATCH-4 | REMEDIATION_LOW | 90d | Publish target for Low-severity fixes. |
| SEC-PATCH-5 | PUBLISH_VS_APPLY | publish | The SLA clocks bind Gradion's **publish**; the client/operator **applies** per their change window (the air-gapped split — mirrors client-owns-DR). Tracked against the release SBOM. |
| SEC-PATCH-6 | ACTIVELY_EXPLOITED_FAST_PATH | 24h out-of-band | Actively-exploited / CISA-KEV-class flaws go on the CRA 24h fast path (the ENISA-notification path, SEC-CVD-3) and ship via the security-patch fast-lane: an isolated minimal diff, cherry-pickable onto a modified install without a full feature upgrade. |
| SEC-PATCH-7 | RELEASE_CADENCE | monthly minor | Monthly stable minor releases + scheduled/ad-hoc patch releases (A31/ADR-0023). Delivery per deployment mode: partner-hosted continuous-deploy (partner pushes); dedicated/on-prem managed-push on a schedule; air-gapped signed offline bundle into a private mirror; source-delivered client-pull of a signed tag via the upgrade preflight. |
| SEC-PATCH-8 | ESR_LINE | ~9-month line, 12-month overlap | ESR/LTS track for regulated/on-prem; security backports to current + 2 prior minors. |
| SEC-PATCH-9 | SUPPORT_HORIZON | 10-year-class | Security-update commitment for the core, carried by the ESR line; feeds release planning. |
| SEC-PATCH-10 | PATCHABILITY_GREEN | guaranteed, mechanical | In-seam/additive changes (new `x_` columns, custom migrations, new files, the policy seams): the security fast-lane applies the patch mechanically. Guaranteed. |
| SEC-PATCH-11 | PATCHABILITY_YELLOW | guaranteed, supervised | Shared-function touches: guaranteed but supervised — the upgrade preflight's union rule + the contract round-trip test + a green fixture run apply it with no silent field-drop. |
| SEC-PATCH-12 | PATCHABILITY_RED | no guarantee | Core-invariant overrides: the patch may conflict; the modifier is the CRA manufacturer (SEC-CRA-5) and resolves it themselves or buys the consulting fix (P14). The curated policy-seam set exists to keep common overrides in Green. |

### Acceptance — disclosure SLAs
Source: margince-poc/docs/quality/security.md @ a11d6c08; margince specs/spec/narrative/06-nonfunctional.md#69-product-security--eu-cra-conformity @ 5a0b29c

| ID | Requirement | Verification |
|---|---|---|
| SEC-CVD-1 | Acknowledgment of a vulnerability report within 1 business day. | Runbook SLA; disclosure-process review |
| SEC-CVD-2 | Initial triage completed within 3 business days. | Runbook SLA; disclosure-process review |
| SEC-CVD-3 | Actively exploited vulnerability → CRA Article 14 escalation with the ENISA notification made within 24 hours. | Runbook escalation step; incident-drill review |
| SEC-CVD-4 | The public route serves an RFC 9116 `security.txt` (contact, expiry, policy pointer; signing supported), and its field values come from the same single source the policy and runbook cite. | Route test on the well-known endpoint; single-source check |
| SEC-CVD-5 | One intake path: the published security contact is the sole report channel, referenced identically by the public text, the policy, and the runbook. | Consistency check across the three surfaces |
| SEC-CVD-6 | Safe-harbor: good-faith research within policy scope is treated as authorized security work. | Published policy text |
| SEC-CVD-7 | Supplier incident notification: when an incident/vulnerability materially affects a customer deployment, Gradion notifies affected customers without undue delay, aligned to the 24h fast path (SEC-PATCH-6), so a NIS2-regulated customer can meet their Art. 23 clock (24h / 72h / 1 month). | Commitment shipped in the compliance pack (SEC-CRA-6) |

### Acceptance — CRA conformity gates
Source: margince specs/spec/narrative/06-nonfunctional.md#69-product-security--eu-cra-conformity @ 5a0b29c

| ID | Gate | Verification |
|---|---|---|
| SEC-CRA-1 | SBOM: machine-readable (CycloneDX), regenerated every release across all repos/dependencies, published with the release. | Generated in CI (Syft/Trivy class), gated |
| SEC-CRA-2 | Signing + provenance: every release artifact (container images + the single binary) is cosign keyless-signed (Sigstore/Fulcio/Rekor transparency log) and carries a SLSA build-provenance attestation. Verification is mandatory before apply — Kubernetes tiers enforce at admission. Docker Content Trust is not adopted. | Release-pipeline gate + admission-controller check |
| SEC-CRA-3 | Scan gates: SAST, DAST, dependency scanning, and secret scanning run in CI as blocking gates, not advisory; the mandatory PR review gate is the security review gate. | CI configuration; overlaps [[quality-gates#QG-3]] (SAST in lint) and [[quality-gates#QG-5]] (govulncheck) |
| SEC-CRA-4 | Category-1 self-assessment: assemble the technical file (risk assessment, SBOM, secure-SDLC evidence, vulnerability management, test results), sign the EU Declaration of Conformity, affix the CE mark. Counsel confirms the Category-1 classification before the mark; a future feature could reclassify. | Counsel sign-off checkpoint before CE mark |
| SEC-CRA-5 | Fork transfers manufacturer status (A27): conformity attaches to the unmodified Gradion release; a substantial modification makes the modifier the CRA manufacturer of the modified product. Gradion ships the SBOM/threat-model generators so a fork re-attests its own delta; SaaS multi-tenant (no forking) stays wholly under Gradion's conformity. | Generators ship with the release; re-attestation is the fork's obligation |
| SEC-CRA-6 | EU compliance pack: one downloadable, per-release-current bundle — SBOM + DoC/CE + CVD policy/`security.txt` + signed-release/SLSA provenance + the incident-notification commitment (SEC-CVD-7) + sub-processor list + DPA + the DPIA template + the AI Act Art. 50 transparency statement. A modified fork re-attests its own pack delta. | Pack assembled per release; contents checklist |
