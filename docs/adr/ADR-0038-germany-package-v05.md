# ADR-0038 — The Germany Package (V0.5): a DACH launch-readiness epic — compliance evidence, e-invoicing/DATEV, eIDAS e-sign, and per-fork SBOM/CRA rebuild

**Status:** Accepted (2026-06-24, founder). Recorded as **DECISIONS A51** (the umbrella competitive-gap-review decision).
Introduces a new tier label **V0.5** and a new epic **E17**.

## Context

The locked beachhead is the **regulated B2B DACH Mittelstand** (A43/ADR-0033), entered **augmentation-first**
through the **cyber-security trust channel** (`10-gtm §3.3`). A 2026-06-24 competitive-gap review (an
external AI critique, triaged against the V1 bar) confirmed that Margince's *product* feature line is
strong, but surfaced a cluster of **DACH-specific, launch-gating** capabilities that are not "nice-to-have
later" features — they are the **legal, fiscal, and trust plumbing required to sell to and operate for the
target customer at all**. Three observations forced this ADR:

1. **These are not V1-Must product features** in the usual sense (they don't change whether the core
   CRM flow works), **but they gate the first real German deal.** Tiering them V1-Must would blur them
   into the product line; leaving them Fast-follow would (wrongly) imply the beachhead can launch without
   them. Neither fits.
2. **They are coherent as a package**, not scattered: a German procurement/IT/DPO reviewer evaluates
   *compliance evidence + fiscal integration + signature + data-retention* as one "is this sellable into a
   regulated German shop?" gate. Scattering them across product epics loses that coherence.
3. **The agent-extensible-source thesis (ADR-0002) creates a unique CRA obligation:** because a customer
   (or a partner, or Gradion) **modifies the source per client**, the **SBOM and CRA conformity must be
   rebuildable for the *modified fork*** — the conformity that ships with mainline does not automatically
   hold for what is actually running. No incumbent has this problem; for Margince it is load-bearing.

Much of the substrate already exists: the **EU compliance pack** (`features/04 §6` — SBOM, DoC/CE, CVD,
SLSA provenance, sub-processor list, DPA, DPIA template, AI-Act Art.50 statement, works-council/BetrVG
qualifier), the **CRA gates + CycloneDX SBOM + reproducible builds** (EP08, ADR-0010/ADR-0025), and the
**retention engine** (`features/04 §4`). What is **missing**: German **e-invoicing (XRechnung/ZUGFeRD) +
DATEV handoff**, **eIDAS/QES e-signature** on the Angebot, a **buyer-facing assembled trust pack**, the
**per-fork SBOM/CRA rebuild**, and **GoBD-grade retention/immutability**.

## Decision

Create a dedicated **Germany Package** epic (**E17**) at a new tier **V0.5**, and route every DACH-specific
launch-readiness capability into it. It is **specified and built as the beachhead's go-live gate** —
sequenced to be ready by the V1 GA *in the DACH market*, distinct from the V1-Must/WOW product feature line.

**The new tier — V0.5 (DACH launch-readiness):**
> Capabilities required to **sell to and operate for the regulated German Mittelstand beachhead** —
> compliance evidence, fiscal/legal integration, and trust artifacts. Not product features that change the
> core CRM flow (those are V1-Must/WOW); not deferrable past the beachhead launch (those are Fast-follow).
> **V0.5 = "the German market's go-live gate."** Counted as its own line, not folded into the V1 Must+WOW
> count. Reuses V1 substrate; adds the DACH-specific edges.

**The package (E17 stories):**
1. **S-E17.1 — DACH e-invoicing (XRechnung / ZUGFeRD) + DATEV handoff.** From an accepted offer/deal,
   produce a **structured, compliant e-invoice** (XRechnung XML / ZUGFeRD hybrid PDF, EN-16931) and a
   **DATEV-format export** the customer's Steuerberater can ingest. Honors the phasing German B2B
   e-invoicing mandate. Totals derive from the offer engine (ADR-0037) — money stays exact (P11).
2. **S-E17.2 — eIDAS / QES e-signature on the Angebot.** The V1 deal-room "accept" (ADR-0037) gains a
   **legally-binding signature option** via an eIDAS-qualified trust-service provider (QES/AdES) — a
   connector + sub-processor decision deliberately deferred from ADR-0037, landed here for the DACH market.
3. **S-E17.3 — The buyer-facing compliance & certification pack ("Vertrauenspaket").** Assemble + serve a
   **per-instance, current** trust bundle a procurement/IT/DPO reviewer can consume: CRA DoC + SBOM,
   **ISO 27001** (Gradion holds it), **BSI C5 / future EUCS** (the *hosting partner's* attestation, A35),
   **TISAX** where the customer is automotive, DPA + sub-processor list, DPIA template, AI-Act Art.50
   statement, the **works-council/BetrVG** qualifier (G-RT-7), and **§393 SGB V** notes for health-near
   data. Extends the existing `features/04 §6` EU compliance pack into a buyer-facing, downloadable surface.
4. **S-E17.4 — The certificate builder: per-fork SBOM + CRA conformity rebuild.** After a client customizes
   the source (ADR-0002), **regenerate the CycloneDX SBOM and the CRA Declaration of Conformity for that
   fork** — so the conformity evidence describes what is *actually running*, not just mainline. Rides the
   reproducible-build + SBOM machinery (EP08, B-EP08.8/.9) and the fork-upgrade safety gates. This is the
   ADR-0002 ↔ CRA reconciliation: agent-extensible source stays conformant because conformity is rebuildable.
5. **S-E17.5 — GoBD-compliant retention, immutability & audit export.** German tax/accounting data
   principles (GoBD: Aufbewahrungspflicht / immutability-Unveränderbarkeit / machine-readable export) over
   the existing retention engine + append-only `audit_log` — so financially-relevant records (offers,
   accepted Angebote, invoices) meet German bookkeeping-retention law.

**Reuses, does not rebuild:** German UI/localization is already **V1 (S-E15.10a)** — referenced by the
package, not duplicated. The compliance-pack *artifacts* already exist (`features/04 §6`); E17 makes them
**buyer-facing + per-fork + DACH-complete**. Money/totals come from the offer engine (ADR-0037); audit,
retention, RBAC, and the event bus are V1 substrate.

## Consequences

- **Positive:** turns the beachhead's hardest non-product objections (e-invoicing mandate, signature,
  "show me your certificates", "is the *modified* version still CRA-conformant?") into a coherent,
  shippable package with a clear owner and tier — instead of scattered fast-follow notes. The per-fork
  SBOM/CRA rebuild (S-E17.4) turns a *risk* of the agent-extensible-source thesis into a **differentiated
  trust feature** no incumbent can match. Sharpens the cyber-security trust-channel GTM (`10-gtm §3.3`).
- **Negative / honest limits:** (a) e-invoicing + eIDAS each pull in an **external standard/connector +
  sub-processor** (XRechnung schema upkeep; an eIDAS TSP) — real integration + legal surface; (b) **V0.5 is
  a new tier** — it must not become a dumping ground; only DACH launch-gating capabilities qualify; (c) it
  adds **build scope ahead of GA** — but it is the gate for the *first paying customer*, so it is
  capacity-prioritized, not optional (re-check R-E4); (d) some items (TISAX, BSI C5/EUCS) are
  **org/partner certifications, not code** — E17 *assembles and surfaces* them, it does not manufacture them.
- **Relationship to other decisions:** composes with ADR-0037 (offer → invoice/sign), ADR-0010/ADR-0025
  (CRA/SBOM/EU posture — S-E17.3/.4 surface + per-fork them), ADR-0002 (the per-fork rebuild is the
  conformity answer to schema-as-source), ADR-0033/A43 (the beachhead this gates), ADR-0027/A35 (hosting
  partner holds cloud certs), ADR-0029 (license/seat — operates per instance). Does **not** weaken any
  locked decision.
- **Scope:** epic `product/epics/E17-germany-package.md` (stories S-E17.1–S-E17.5, tier **V0.5**); extends
  `features/04 §6` (compliance pack → buyer-facing + per-fork) and `features/06` / `features/01 §10` (offer
  → e-invoice/sign); new build over EP08 (SBOM/CRA per-fork) + new e-invoicing/eIDAS connectors. Tier
  taxonomy updated in `product/00-overview.md`. Counts canonical in `product/20-traceability.md`.
