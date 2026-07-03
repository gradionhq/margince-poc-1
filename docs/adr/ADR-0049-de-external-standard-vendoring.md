# ADR-0049 — The German jurisdiction pack vendors pinned copies of the external standards it must conform to

**Status:** Accepted (Lars, 2026-06-26). Recorded as **DECISIONS A64**. Composes with [ADR-0042](ADR-0042-jurisdiction-packs.md) (jurisdiction packs — the vendored material lives in `crm-de`, never core) and [ADR-0041](ADR-0041-mid-build-spec-governance-and-re-gating.md) (a standard change is a mid-build re-gate, not a silent edit); applies principle **P3** (contract-first: the vendored artifact *is* the contract DE-pack tickets build against). Does not weaken any locked decision.

## Context

The Germany Package (E17, ADR-0038/A51) must conform to external legal and standards inputs that an implementer **cannot invent**: the EN-16931 / XRechnung field model, the DATEV export format and chart-of-accounts mapping, and the GoBD retention regime. Several E17 build tickets, as written, point at these only as prose — "per EN-16931", "as required by GoBD". That is not buildable, testable, or auditable:

- An AI coding agent (ADR-0002) handed "emit a compliant XRechnung" has no deterministic source of truth for *which* business terms are mandatory — it would guess, and guesses against a tax-relevant format fail silently at the customer's revenue office, not in CI.
- "Per the standard" has no version. When EN-16931 or a DATEV release changes, nothing in the repo records *which* version the code was built against, so a drift is invisible until it breaks.
- ADR-0042 isolates German behavior in `crm-de` but says nothing about *how the regulatory inputs themselves enter the repo* — leaving the most failure-prone material (the field lists and mapping tables) ungoverned.

## Decision

**`crm-de` vendors a pinned, versioned copy of each external standard it must conform to — or a validation table mechanically derived from it — checked into the pack with its source and version/date recorded. DE-pack tickets validate against the vendored artifact, not against prose.** Specifically, three artifacts:

1. **EN-16931 / XRechnung CIUS mandatory business-term (BT) list.** The set of mandatory BT fields (and the CIUS restrictions XRechnung layers on EN-16931) as a checked-in field/validation table. `FiscalFormatter` (ADR-0042) emits and validates against this table. *(Ticket B-E17.2a.)*

2. **DATEV EXTF export field spec + SKR03/SKR04 chart-of-accounts mapping.** The EXTF export field layout and the SKR03/SKR04 account-mapping table as a versioned artifact. The DATEV `ExportProfile` builds rows against this spec. *(Ticket B-E17.4.)*

3. **GoBD record-class → retention-period → action taxonomy.** The GoBD record classification, statutory retention windows, and the per-class action (immutable / archive / purge-eligible) as a versioned table driving `RetentionPolicy`. *(Ticket B-E17.15.)*

**Form.** Each artifact is the *deterministic input* the code is tested against: tickets assert "the emitted document carries every BT in `crm-de/standards/en16931-xrechnung-bt.<ver>.…`", not "the document is EN-16931 compliant". Provenance (source, version, retrieval date) is recorded alongside each artifact. The material lives **only in `crm-de`** (ADR-0042 §2 — the pack is the home for country data); core never carries it.

**Change flow.** When an upstream standard changes (new EN-16931 release, DATEV format bump, GoBD update), the vendored copy is **bumped through the normal ADR-0041 mid-build re-gating flow** — re-gate the affected tickets, regenerate/revalidate, record the new version. A standard change is never a silent in-place edit.

**Licensing caveat (counsel check — NOT resolved here).** Some of these standards may carry redistribution or copyright restrictions on verbatim text or tables. Before vendoring each artifact verbatim, **confirm its redistribution terms**; where verbatim redistribution is not permitted, vendor a *derived validation table* (the testable field/rule set) rather than the source document, or obtain the necessary licence. This is flagged as an open **counsel check**, not asserted as cleared.

## Consequences

- **Deterministic, testable, auditable DE build.** The build target is a concrete checked-in table, so CI can verify conformance and an auditor can see exactly which standard version a release was built against. This is P3 applied to regulatory inputs: the vendored artifact is the contract.
- **Isolation holds (ADR-0042).** All regulatory source material sits inside `crm-de`; a non-DE build never carries it, and reading `crm-de/` shows *all* the German inputs in one place.
- **Drift is governed (ADR-0041).** Upstream changes flow through re-gating with a recorded version bump — no invisible drift, no untracked edits to tax-relevant logic.
- **Tickets amended:** **B-E17.2a** (EN-16931/XRechnung BT list), **B-E17.4** (DATEV EXTF + SKR03/SKR04 mapping), **B-E17.15** (GoBD taxonomy) gain a "validate against the vendored artifact at `crm-de/standards/…`" acceptance criterion and a provenance note, replacing any "per the standard" prose.
- **New `crm-de/standards/` (or equivalent) location** for the vendored artifacts + a provenance manifest (source, version, date, licence status per artifact).
- **Open counsel check carried forward:** redistribution terms per artifact must be confirmed before verbatim vendoring; derive-rather-than-copy is the fallback. This ADR records the obligation, it does not discharge it.

## Alternatives considered

- **Keep referencing standards as prose ("per EN-16931").** Rejected: not buildable by an AI agent, not testable in CI, no version anchor — the failure mode this ADR exists to remove.
- **Fetch/validate against the standard at build time from an upstream source.** Rejected: introduces a network and availability dependency in the build, breaks reproducible builds (EP08), and still leaves no in-repo record of the exact version used. Vendoring a pinned copy is the deterministic choice.
- **Hold the tables in core.** Rejected: violates ADR-0042's isolation — country regulatory data belongs in the pack, not core.
