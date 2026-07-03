# ADR-0011 — Per-purpose consent with proof, and a V1 data-retention engine

**Status:** Accepted (2026-06-10, with Lars) — DECISIONS A22. Built into `data-model.md §3.4` (consent model rebuilt), `features/04 §4` (cut-lines flipped to MVP), build-plan WP6+. Raised by the security-practice cross-assessment (`foundation/spec/feedback/security-practice-crossref.md`): marketing consent was a single coarse flag with no capture flow, enforcement was fast-follow not MVP, and data retention was OUT of v1 — awkward for a CRM whose own consulting practice sells GDPR readiness.

## Context

The prior model (DECISIONS pre-A22, `data-model.md §3.4`) put one `consent_state` (`unknown|granted|withdrawn`) plus `consent_basis/at/source` on `person`. Three problems, all GDPR-relevant for a CRM that does outbound:

- **No proof of consent (Art 7(1)).** The controller must be able to *demonstrate* consent. A bare state flag with a timestamp does not capture the wording shown, the policy version, or a double-opt-in confirmation. The burden of proof is on us / the customer-as-controller.
- **No per-purpose granularity (Art 6/7).** Consent is purpose-specific. A single flag cannot express "agreed to the newsletter, not to behavioural profiling." Bundled consent is not valid consent.
- **Enforcement was fast-follow, and retention was OUT entirely.** A v1 instance would happily send to `unknown` contacts, and nothing enforced storage-limitation (Art 5(1)(e)) — data would accrue forever by default.

## Decision

**Rebuild consent as per-purpose with proof, enforce it in MVP, and ship a retention engine in V1.**

### Consent
- **Purposes are first-class.** A workspace-scoped `consent_purpose` reference set (seeded: `marketing_email`, `marketing_phone`, `profiling`, `product_updates`; extensible per the `04` boundary).
- **Current state per person × purpose** in `person_consent` (`state`, `lawful_basis`, `captured_at`, `source`, `policy_version`).
- **Proof is append-only.** Every grant/withdrawal writes a `consent_event` row: timestamp, purpose, source/channel, lawful basis, the **exact policy wording + version** presented, and the **double-opt-in confirmation** event reference where required. `consent_event` is append-only/tamper-evident like `audit_log`.
- **Enforcement is MVP, default-deny.** An outbound action for a purpose is blocked/`409`-suppressed unless an **active, proven `granted`** record exists for *that* purpose. `unknown` is treated as no-consent.

### Retention
- **`retention_policy`** (workspace-scoped): per object-type/category, a retention period and an action ladder (archive → anonymize → erase), with a lawful-basis label.
- **Nightly River evaluator** applies due policies, fully audited; erasure reuses the A13 suppression-list + PII-free tombstone path.
- **`legal_hold`** suspends deletion for records under investigation/litigation; a held record is never auto-acted, and the hold is audited.

## Consequences

- **Positive:** GDPR Art 5(1)(e), 6, 7 are actually satisfied, not gestured at; the CRM matches what the consulting practice sells; proof-of-consent and retention become demonstrable in an audit (and exportable, P7).
- **Ripple:** `data-model.md §3.4` replaces the single-flag schema; `features/04 §4` flips "consent-aware suppression", "per-purpose consent", and "retention engine" from fast-follow/OUT to **MVP**, with new `[MV]` acceptance criteria (per-purpose suppression; proof-record completeness; retention-evaluator action + legal-hold); WP6 gains the consent+retention work and exit gates.
- **Negative / to bound:** more schema and UI surface (purpose management, consent timeline on the record, retention-policy admin). Bounded by seeding sensible default purposes and a default retention policy so a workspace is compliant out-of-the-box without configuration. Per-purpose consent *capture* still depends on the surfaces that collect it (booking link A16, import mapping, forms) declaring the purpose + wording — those surfaces must pass the purpose through, not invent a blanket grant.
- **Boundary:** the DPA and sub-processor list (the US LLM vendors as Art 28 processors) are legal/commercial artifacts, not schema — tracked in `features/04 §4` as required documents, produced alongside, not in the data model.
