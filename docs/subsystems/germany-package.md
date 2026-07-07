---
status: planned
module: jurisdictions/de/ (the German pack) + backend/internal/modules/ (invoice seam) + web/jurisdictions/de/ (DE screen bundle)
derives-from:
  - specs/spec/product/epics/E17-germany-package.md
  - specs/spec/product/build-backlog/E17.md
  - specs/spec/features/04-platform-and-compliance.md#6-eu-cra-conformity-the-product-meets-the-standard-the-practice-sells
  - specs/spec/contract/data-model.md#125-net-new-v1-objects-signals-deal-rooms-voice-agent-connections-automation-views-quota-field-mask
  - specs/spec/product/30-screen-acceptance.md#e-invoicehtml--german-e-invoice--datev-implements-s-e171
  - specs/spec/product/30-screen-acceptance.md#e-signaturehtml--eidas-e-signature-implements-s-e172
  - specs/spec/product/30-screen-acceptance.md#trust-packhtml--trust-pack-vertrauenspaket-implements-s-e173
  - specs/spec/product/30-screen-acceptance.md#gobdhtml--gobd-retention--audit-export-implements-s-e175
  - specs/spec/decisions/ADR-0038-germany-package-v05.md
  - specs/spec/decisions/ADR-0042-jurisdiction-packs.md
  - specs/spec/decisions/ADR-0049-de-external-standard-vendoring.md
---
# Germany package — the DACH go-live gate: e-invoice, e-signature, trust pack, per-fork conformity, GoBD

> The V0.5 line in one chapter: the five capabilities a regulated German buyer's
> procurement, IT, DPO, and Steuerberater evaluate as a single "is this sellable
> here?" gate — a compliant e-invoice with the DATEV handoff, a legally-binding
> eIDAS signature on the Angebot, a buyer-facing trust pack, the per-fork
> SBOM/conformity rebuild, and GoBD retention. Built as the first jurisdiction
> pack (ADR-0042): compile-time German code behind the five pack hooks, never in
> core.

## What it's for

The locked beachhead is the regulated German Mittelstand, and it does not buy on
features alone: a procurement/IT/DPO/Steuerberater review evaluates compliance
evidence, fiscal integration, signature, and data retention together, as one gate
(ADR-0038). This subsystem is that gate made shippable — the Germany Package,
tier V0.5, its own line outside the V1 Must+WOW total ([[scope#TIER-V05]]),
sequenced to be ready by V1 GA in DACH. Its callers are the offer and deal-room
flow (an accepted Angebot becomes an invoice; an acceptance can carry a
signature), the release pipeline (the trust pack and the per-fork rebuild bind to
a build), the retention and audit substrate (GoBD rides it), and the buyer-side
reviewers those surfaces exist for.

The scope boundary is drawn twice. Functionally: the package assembles and
surfaces evidence, it does not manufacture organisation or partner certificates,
and it feeds the Steuerberater's accounting system rather than becoming one — no
general ledger, no payment reconciliation, no dunning. Architecturally: every
German behaviour and every German regulatory datum lives in the German
jurisdiction pack behind the five capability hooks
([[jurisdiction#Parameters — capability hooks]], JUR-HOOK-1..5); core stays
country-neutral and gains only generic engine extensions (the invoice record, the
accept path, the export and retention machinery it already owns).

An honesty note on sources: the corpus flags e-invoicing and e-signature as
net-new build with deliberately thin feature-chapter coverage — the feature docs
carry only the seams they extend. The E17 epic and its build-lens decomposition
are therefore the primary sources this chapter distills, and the pins below say
so explicitly rather than implying feature-spec depth that does not exist.

## Principles it serves

- **P11 — money is code.** The e-invoice never free-types a total: it derives
  from the accepted offer's server-computed minor-unit money, and a document that
  does not reconcile is rejected, not emitted. The signature binds those exact
  frozen amounts.
- **P12 — governance designed in.** Issuing a fiscal document is an
  approval-gated outward action; every export is audit-logged; GoBD immutability
  is enforced in the data layer, not a settings toggle; the fork conformity gate
  fails a build rather than trusting a promise.
- **ADR-0038 — the Germany Package and the V0.5 tier.** The decision this chapter
  embodies: one owner, one tier, one coherent package for the DACH launch gate.
- **ADR-0042 — jurisdiction packs.** The package is compile-time pack work — the
  German pack implements the hooks, core cannot import it, and a non-DACH build
  ships none of it.
- **ADR-0049 — vendored standards.** Every external standard the pack must
  conform to enters the repo as a pinned, versioned artifact the tests validate
  against — never as "per the standard" prose.
- **ADR-0002 ↔ CRA, reconciled.** Agent-extensible source creates a conformity
  obligation no incumbent has; the certificate builder answers it by making the
  SBOM and Declaration of Conformity rebuildable for the modified fork
  ([[security]] SEC-CRA-5).

## How it works

**The pack seam comes first.** All five stories ride the jurisdiction-pack
framework: the port, registry, per-pack migrations, and country-isolation gates
are owned by the [[jurisdiction]] chapter and must exist before any German code
lands. The German pack contributes through all five hooks — fiscal formatter,
retention policy, conformity regime, trust-artifact set, export profiles
(JUR-HOOK-1..5) — and the eIDAS trust-service provider rides the existing
connector seam, needing no hook of its own ([[jurisdiction]] out-of-scope note).

**E-invoice (S-E17.1).** An accepted offer — and only an accepted one — generates
an invoice: a core, country-neutral record whose net, tax, and gross are copied
from the offer's server-computed totals and reconciled by test; a total that does
not reconcile to the offer is rejected rather than silently emitted. The German
pack's fiscal formatter then renders that record as a standards-conformant
XRechnung document or a ZUGFeRD hybrid (a human-readable archival PDF with the
structured data embedded), validating before returning — the refuse-to-emit gate
and the field-level bindings are pinned once in the jurisdiction chapter
(JUR-XR-1..10) and only cited here. Accounting handoff is a DATEV-format booking
export the Steuerberater ingests, mapped through the vendored chart-of-accounts
table (DEPACK-PARAM-3) and audit-logged. The mandate runs both ways: an inbound
supplier document parses into a structured staged record, and when no company
matches the supplier identity the record is left unlinked for a human to attach —
never auto-linked on a guess. Issuing is a gated outward action: it queues to the
approval inbox, a human approves before a fiscal document leaves the workspace,
and an issued invoice is immutable — a correction is a cancellation document plus
a new invoice, never an edit.

**E-signature (S-E17.2).** Acceptance in the deal room stays the V1 baseline — a
tracked in-room action whose semantics are pinned by the offers chapter and
surfaced by [[deal-rooms]] (DEALROOM-AC-2). The signature is an opt-in evidence
upgrade for deals that warrant it: the buyer signs through an eIDAS-conformant
flow at a qualified trust-service provider — advanced level by default, qualified
level where required, with the one-time identity-verification step
(DEPACK-PARAM-4). The provider is a named sub-processor honoring the operator's
residency, and the platform never receives the signer's identity document — only
the signed artifact and its validation evidence return, stored as a
signature-evidence record hash-locked to the offer's server-computed total. A
completed signature then runs the *same* accept path as the tracked click —
status flip, deal-amount sync, the accepted event, the audit trail
([[offers-and-products]] OFFER-AC-2) — an evidence upgrade, never a parallel
acceptance state machine. A provider failure leaves nothing signed and the offer
unchanged: there is no half-accepted state, and the tracked click remains the
fallback.

**Trust pack (S-E17.3).** The buyer-facing Vertrauenspaket assembles the existing
EU compliance pack ([[security]] SEC-CRA-6) into a per-instance, current,
downloadable bundle a reviewer can consume: the conformity declaration and SBOM,
Gradion's own certification, the hosting partner's cloud attestations, the data-
protection artifacts, the AI-transparency statement, and the German qualifiers —
with applicability-gated items included only where the buyer's profile flags them
(DEPACK-PARAM-6). Provenance is never ambiguous: every artifact carries its
issuer or holder (Gradion-held, partner-held, per-fork, template, qualifier) and
its validity date, a stale attestation renders as stale, and a non-applicable
artifact is shown omitted-with-reason rather than fabricated or silently dropped.
A completeness gate asserts every applicable artifact is present and bound to the
running build's signed provenance — a missing artifact fails the gate, and the
pack never assembles a partial set. Sharing the pack outside the workspace is
approval-gated, expiring, and audit-logged.

**Certificate builder (S-E17.4).** The per-fork rebuild is the ADR-0002 ↔ CRA
reconciliation: when a fork's source is modified, a release build regenerates the
software bill of materials across the modified dependency trees — reproducibly,
on the core supply-chain rails ([[security]] SEC-CRA-1, SEC-CRA-2) — and rebuilds
the Declaration of Conformity for that fork, bound to the build's signed
provenance, shaped by the pack's conformity regime (JUR-HOOK-3). The trust pack
for that instance then shows the fork's evidence, marked as a fork — what the
buyer is shown matches what is running. The discipline has teeth: a customization
that breaks a conformity gate (a flagged-vulnerable dependency, a failed scan)
fails the fork's release build before anything ships, and the failing gate names
the offending change. Manufacturer responsibility follows the fork per the graded
policy the security chapter owns (SEC-CRA-5).

**GoBD (S-E17.5).** German bookkeeping-retention law rides the existing
substrate, never a parallel archive: retention classes extend the retention
engine and policy rows the GDPR platform owns ([[gdpr-platform]],
[[data-model#DM-DDL-10]]), and the audit trail is the same append-only spine
everything else crosses ([[audit-observability]], [[data-model#DM-DDL-8]]). The
pack's retention-policy hook (JUR-HOOK-2) classifies financially-relevant records
— invoices, accepted Angebote, sent offers — into statutory classes derived from
record type, never free-set (DEPACK-PARAM-5), and within the statutory window a
classified record is protected from deletion and mutation *in the data layer*,
even against an admin, with no override toggle. Where a GDPR erasure request
collides with a GoBD obligation, the precedence is explicit and defensible: the
retention obligation wins for the statutory window with a documented legal basis,
the record is restricted rather than deleted, the split decision is written to
the audit log, and the suppressed erasure becomes actionable when the window
elapses. The auditor gets a machine-readable export of the retained records with
their audit trail, in accepted open forms (DEPACK-PARAM-7), assembled by the
pack's export profile (JUR-HOOK-5) over the core export machinery — and the
export is itself audit-logged. Retained records and their immutability survive a
backup restore, consistent with the erasure-suppression-on-restore guarantee
([[operations#OPS-DR-6]]).

## What's configurable

- **The linked pack** — whether a build carries German behaviour at all is the
  compile-time switch owned by the jurisdiction chapter; a non-DACH build omits
  the pack and everything here with it (JUR-GUAR-5).
- **Chart-of-accounts mapping** — the DATEV export maps through a configurable
  choice between the two standard German charts, from the vendored mapping table
  (DEPACK-PARAM-3).
- **Signature assurance level and opt-in** — per acceptance, the buyer signs at
  advanced or qualified level, or declines e-sign entirely and accepts with the
  tracked click; e-sign is an upgrade, never a dependency (DEPACK-PARAM-4).
- **Trust-pack applicability flags** — automotive and health-near artifacts are
  included by the instance's buyer profile, omitted-with-reason otherwise
  (DEPACK-PARAM-6).
- **Auditor-export scope** — record classes, period, and output format are chosen
  per export (DEPACK-PARAM-7).
- **What is not configurable** — retention classes derive from record type and
  are never free-set; the reconcile rule, the refuse-to-emit gate, the single
  accept path, and data-layer immutability are invariants. Vendored standard
  versions change only by a governed version bump with re-gating (ADR-0049,
  ADR-0041), never a runtime knob or a silent edit.

## Guarantees (enforced)

- **Derived, or rejected.** Invoice totals equal the accepted offer's
  server-computed totals in integer minor units; a non-reconciling document and a
  generate on a non-accepted offer are both refused — pinned by reconcile and
  state-guard tests (DEPACK-AC-1b).
- **No invalid document escapes.** The fiscal formatter validates against the
  vendored, version-pinned standard before returning and refuses to emit on any
  failure — the jurisdiction chapter's refuse-to-emit gate (JUR-XR-7), inherited
  here for the e-invoice feature.
- **One accept path.** The signed accept and the tracked click produce identical
  record and event outcomes; a signature is evidence, not a second state machine
  — a test asserts the two paths converge (DEPACK-AC-2b).
- **Nothing half-signed.** A provider failure leaves the offer unchanged, with an
  honest error and the tracked-click fallback (AC-e-signature-8, the screen's
  error state).
- **Complete, or failed.** The trust pack's completeness gate fails on any
  missing applicable artifact; the pack never fabricates, never silently drops,
  and never presents stale or partner-held evidence as current Gradion evidence
  (DEPACK-AC-3d).
- **The fork cannot ship non-conformant.** A customization that breaks a
  conformity gate fails the release build before release (DEPACK-AC-4d).
- **Immutability is data-layer, admin included.** A delete or mutation of a
  record inside its statutory window is rejected at the engine/database layer for
  every role, surfaced as the pinned locked error (DEPACK-WIRE-4), and the
  blocked attempt is itself logged (DEPACK-AC-5a).
- **Precedence is documented, never silent.** Erasure-versus-retention resolves
  explicitly with a recorded basis, and reverses when the window elapses
  (DEPACK-AC-5c).
- **Everything crosses the audit wall.** Generation, issue, exports, signatures,
  downloads, and shares each write the append-only audit trail through the one
  seam ([[audit-observability]] AUD-AC-2/AUD-AC-3).

## Acceptance

Done means a German launch survives its reviewers: the Steuerberater receives a
schema-valid e-invoice and a DATEV booking export derived from the accepted
Angebot; the buyer can sign at a qualified provider and the deal lands exactly as
a tracked click would have; procurement downloads a complete, dated,
provenance-honest trust pack that matches the running build — including a
modified fork's own evidence; and an auditor gets the retained records with their
trail while the system provably refuses to delete them early. The honest states
are part of the contract: the non-reconciling invoice rejected with a red chip,
the invalid document that fails instead of emitting, the provider timeout that
signs nothing, the incomplete pack that blocks instead of pretending, the denied
deletion with its locked error, and the empty export scope that exports nothing.
All five stories S-E17.1..S-E17.5 are owned here with primacy — the scope
roll-up's E17 row names this chapter as sole owner ([[scope]] epic inventory),
and V0.5 is its own line outside the V1 total ([[scope#TIER-V05]]). The testable
forms live in the Acceptance appendix; the cross-cutting screen-state floor and
release gates are inherited from [[acceptance-standards]] and not restated.

## Out of scope

- **The accounting system** — general ledger, payment reconciliation, dunning:
  the Steuerberater's DATEV/accounting system's job; this package feeds it
  (ADR-0038).
- **Manufacturing certificates** — ISO, C5/EUCS, TISAX are organisation or
  hosting-partner attestations; the package assembles and labels them
  (SEC-CRA-6; the partner-held rule is the security chapter's).
- **The jurisdiction port, registry, isolation gates, and the XRechnung
  field-level bindings** — owned by [[jurisdiction]] (JUR-HOOK-*, JUR-GUAR-*,
  JUR-XR-*); this chapter owns the V0.5 stories, screens, and feature behaviour.
- **The offer engine and accept semantics** — [[offers-and-products]]
  (OFFER-AC-1..3); **the deal-room surface** the buyer signs from —
  [[deal-rooms]].
- **The supply-chain rails** — reproducible builds, SBOM generation, signing,
  scan gates, and the compliance-pack contents: [[security]] (SEC-CRA-1..6);
  this chapter binds them per fork and per buyer, it does not re-own them.
- **German UI language** — locale is core and per-user, never pack content
  (JUR-GUAR-7); the pack carries only regulatory document text.
- **The GDPR consent/retention/suppression substrate** — [[gdpr-platform]]; GoBD
  extends its policy rows and precedence machinery.

## Where it lives

The German jurisdiction pack module (`jurisdictions/de/` — formatters, parsers,
retention classes, conformity regime, trust artifacts, export profiles, and the
vendored standards artifacts per ADR-0049), reached through the jurisdiction port
(`backend/internal/shared/ports/jurisdiction`); the country-neutral invoice
record and its offer-derivation seam in the invoice-owning core module under
`backend/internal/modules/`; and the conditionally-bundled German screens under
`web/jurisdictions/de/`. Read next: [jurisdiction](../architecture/jurisdiction.md)
(the seam and the XRechnung bindings), [offers-and-products](offers-and-products.md)
(the record everything derives from), [deal-rooms](deal-rooms.md) (where the buyer
acts), [security](../quality/security.md) (the CRA substrate), and
[gdpr-platform](gdpr-platform.md) (the retention substrate GoBD extends).

## Appendix

### Parameters
Source: specs/spec/product/epics/E17-germany-package.md @ 5a0b29c; specs/spec/product/build-backlog/E17.md#a-the-compliant-german-e-invoice--xrechnung--zugferd--datev-s-e171 @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#gobdhtml--gobd-retention--audit-export-implements-s-e175 @ 5a0b29c; specs/spec/decisions/ADR-0049-de-external-standard-vendoring.md @ 5a0b29c

Standards targets the corpus pins. Every external standard below enters the repo
as a vendored, version-pinned artifact with recorded provenance (ADR-0049); a
version bump is a governed re-gate (ADR-0041), and the redistribution-licence
counsel check ADR-0049 flags is open, not discharged.

| ID | Name | Value | Meaning |
|---|---|---|---|
| DEPACK-PARAM-1 | XRechnung emit target | EN-16931 / UBL 2.1, XRechnung 2.3.1 CIUS; vendored fixed release, pure-Go validation, no JVM | Sanctioned restatement of the jurisdiction chapter's pins — the version, the two-layer validator, and the refuse-to-emit gate are owned at [[jurisdiction#Wire — XRechnung bindings]] (JUR-XR-7, JUR-XR-8) |
| DEPACK-PARAM-2 | ZUGFeRD emit target | ZUGFeRD 2.2 / Factur-X, profile EN 16931 (COMFORT); container PDF/A-3u; embedded XML is UN/CEFACT CII (guideline `urn:cen.eu:en16931:2017`), validated against the shipped CII D22B XSD plus the 25 official example invoices as golden fixtures; pure-Go generation | One structured source of truth, two containers: the hybrid PDF embeds CII (not the UBL bytes — Factur-X readers reject embedded UBL); visible page, embedded XML, and XRechnung totals are cross-checked identical on build |
| DEPACK-PARAM-3 | DATEV handoff format | EXTF Buchungsstapel CSV; booking fields Umsatz / Konto / Gegenkonto / BU-Schlüssel / Belegfeld; SKR03 or SKR04 chart mapping (configurable), vendored version-pinned; German decimal-comma number formatting | The Steuerberater-ingestible booking export; the screen's worked example splits SKR03 debitor 10000 / revenue 8400 / USt 1776, reconciling to net/tax/gross |
| DEPACK-PARAM-4 | eIDAS assurance levels | AdES (default) or QES (requires a one-time identity verification: German eID / Video-Ident / Bank-Ident); TSP must be on the EU Trusted List and named in the DPA/sub-processor list; e-sign is opt-in — the tracked click remains a full accept | The two legal levels S-E17.2 offers; the platform never receives the identity document, only the signed result + validation evidence |
| DEPACK-PARAM-5 | GoBD retention classes | Invoices 10 yr (immutable); accepted offers 6 yr (immutable); sent offers 6 yr (immutable); draft offers — no statutory period (editable). Class derived from record type, never free-set. Statutory sources HGB §257 / AO §147 | Screen-pinned values (AC-gobd-3); the authoritative class → period → action taxonomy is the vendored pack artifact (ADR-0049), not a spec-authored table — the ratifying ticket validates against it |
| DEPACK-PARAM-6 | Trust-pack artifact set | CRA Declaration of Conformity + SBOM (per-fork, generated); ISO 27001 (Gradion-held); BSI C5 / EUCS (hosting-partner-held, A35); TISAX (applicability: automotive buyer); DPA + sub-processor list (incl. any eIDAS TSP); DPIA template; AI-Act Art. 50 transparency statement; works-council / BetrVG §87(1)(6) qualifier (G-RT-7); §393 SGB V notes (applicability: health-near buyer) | The named set the completeness gate asserts (the designed screen counts 9 present with TISAX omitted-by-design); extends the compliance-pack completeness check owned at [[security]] SEC-CRA-6 — assembled, not manufactured |
| DEPACK-PARAM-7 | GoBD auditor-export formats | GDPdU/IDEA (`.csv` + `index.xml`) or structured `.json`; manifest with per-class record counts (computed, never free-typed), size, and content hash | The machine-readable export forms the screen's builder offers (AC-gobd-5) |

### Schema
Source: specs/spec/contract/data-model.md#125-net-new-v1-objects-signals-deal-rooms-voice-agent-connections-automation-views-quota-field-mask @ 5a0b29c; specs/spec/product/build-backlog/E17.md#a-the-compliant-german-e-invoice--xrechnung--zugferd--datev-s-e171 @ 5a0b29c; specs/spec/product/epics/E17-germany-package.md#s-e171--a-compliant-german-e-invoice-xrechnung--zugferd--datev-handoff @ 5a0b29c

Ownership verified against the data-model chapter's ownership index: **no invoice
table exists anywhere in the corpus schema** — the offers chapter pins that
finding (OFFER-DDL-N-1) and routes the invoice/e-invoice schema here on arrival.
The build backlog is explicit that the invoice table is *net-new, added by its
first story* (a core, country-neutral entity per the ADR-0042 placement table —
the German formats are pack renderings of it). The DDL below therefore follows
the import-export-migration precedent for net-new provisional shape (IEM-DDL-1/2,
the `event_outbox` convention): **provisional until the contract extension that
ratifies the invoice wire surface** (DEPACK-WIRE-1). The ratifying ticket may
adjust columns; the pinned behaviours (accepted-offer-only generation, totals
reconcile-or-reject, issue-then-immutable, Storno-not-edit) are fixed by the
Acceptance pins regardless.

**DEPACK-DDL-1 — `invoice` (net-new; provisional).**

```sql
CREATE TABLE invoice (
  -- + base columns (id, workspace_id, timestamps per data-model conventions)
  offer_id       uuid NOT NULL REFERENCES offer(id) ON DELETE RESTRICT,
  invoice_number text NOT NULL,                     -- human-facing "Rechnung" number
  status         text NOT NULL DEFAULT 'draft'
                 CHECK (status IN ('draft','issued')),
  currency       char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  -- totals COPIED from the accepted offer's server-computed snapshot; a
  -- reconcile test ties them to the offer's net/tax/gross (never free-typed)
  net_minor      bigint NOT NULL,
  tax_minor      bigint NOT NULL,
  gross_minor    bigint NOT NULL,
  issued_at      timestamptz NULL,                  -- set on issue; issued ⇒ immutable
  source         text NOT NULL,                     -- provenance (DM-CONV-11)
  captured_by    text NOT NULL,
  CONSTRAINT invoice_number_unique UNIQUE (workspace_id, invoice_number),
  CONSTRAINT invoice_issued_at CHECK (status <> 'issued' OR issued_at IS NOT NULL)
);
CREATE INDEX idx_invoice_offer ON invoice (workspace_id, offer_id);
```

Invoice lines are not a table: the e-invoice mirrors the accepted offer's line
items read-only (through the offer reference), and the per-line document
derivation is pinned at the jurisdiction port (JUR-XR-6). Post-issue immutability
is enforced by the GoBD retention class (DEPACK-PARAM-5, DEPACK-AC-5a), not by an
extra column; a correction is a Storno plus a new document (AC-e-invoice-6) —
whether the cancellation link is a column or a convention is the ratifying
ticket's call.

**DEPACK-DDL-2 — `signature_evidence` (corpus-pinned, verbatim §12.5).** The one
E17 table the corpus schema already defines; the ownership index routes it to
this chapter.

```sql
CREATE TABLE signature_evidence (                        -- proof a document was e-signed (DE pack; B-E17.8/.8b)
  document_ref text NOT NULL,                            -- the signed artifact (offer/contract id)
  provider     text NOT NULL,                            -- e-sign provider
  envelope_id  text NOT NULL,                            -- provider's envelope/transaction id
  signer_set   jsonb NOT NULL,                           -- signers + timestamps
  status       text NOT NULL CHECK (status IN ('sent','completed','declined','voided')),
  evidence_ref text NULL,                                -- attachment id of the completion certificate
  hash         text NULL,                                -- hash of the evidence blob
  signed_at    timestamptz NULL,
  captured_by  text NOT NULL                             -- provenance (§1.6)
);
CREATE INDEX idx_sig_evidence_doc ON signature_evidence (workspace_id, document_ref);
```

Both tables run under forced row-level security with the workspace policy, and
every write crosses the append-only audit wall ([[data-model#DM-DDL-8]]).

**Schema gaps (pinned as needs, not tables — the corpus gives no shape):**

- **DEPACK-GAP-1 — pack-owned German tables.** The ownership-manifest examples
  name candidate pack tables (a DATEV-export record, an XRechnung-document
  record) as `crm-de`-owned, but no corpus source gives them columns. Any
  pack-side table ships in the pack's own migration namespace
  ([[jurisdiction]] JUR-GUAR-6) and is minted by its ticket, not here.
- **DEPACK-GAP-2 — the GoBD class taxonomy is pack data, not core schema.** The
  class → retention-period → action table is a vendored artifact in the pack
  (ADR-0049), driving the retention-policy hook; it is deliberately not a
  spec-authored DDL table.
- **DEPACK-GAP-3 — the inbound-invoice staging record.** The receive path parses
  a supplier document into a "structured inbound record" left unlinked on no
  match; no corpus source shapes it. The no-guess-link behaviour is pinned
  (DEPACK-AC-1d, AC-e-invoice-7) regardless of shape.
- **DEPACK-GAP-4 — the trust-pack assembly record.** The assembled pack is a
  per-release, build-bound artifact set with a completeness gate; whether its
  manifest is a table or a release artifact is unpinned. The gate behaviour and
  provenance labels are pinned (DEPACK-AC-3a..3d) regardless.

### Wire
Source: specs/spec/contract/crm.yaml (zero E17 operations at pin time) @ 5a0b29c; specs/spec/product/build-backlog/E17.md#b-e176--issue-invoice---outward-action--approval-inbox-then-gobd-immutable @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#gobdhtml--gobd-retention--audit-export-implements-s-e175 @ 5a0b29c

**Honest contract-coverage finding:** the contract defines **no** invoice,
e-signature, trust-pack, or GoBD operation — unlike offers, E17 has not even a
planned-resources comment block. Every row below is a promised surface pinned by
path + behaviour; operationIds must be minted by a contract extension before any
docs-cited operationId can resolve (the same discipline as the offers chapter's
OFFER-WIRE finding). The signature-evidence contract note says only that it is
read "through the DE offer/sign surface".

| ID | Element (planned) | Behavior pinned |
|---|---|---|
| DEPACK-WIRE-1 | `/invoices` (net-new resource) | Modeled on the offers resource convention (the backlog's explicit instruction). Generate-from-offer requires `offer.status=accepted` — a non-accepted offer is refused (`409`/`422`); totals are copied, reconciled, never client-set; generation is audit-logged. |
| DEPACK-WIRE-2 | `POST /invoices/{id}/issue` | 🟡 outward action modeled on the offers send convention: an agent call without a valid token → `ErrRequiresApproval` ([[api-conventions#API-ERR-10]]); queues to the approval inbox; on approve the invoice flips `draft → issued` and becomes GoBD-immutable (a correction is a Storno + new document); issue + disposition audit-logged. |
| DEPACK-WIRE-3 | Inbound receive (supplier e-invoice) | Parses XRechnung/ZUGFeRD into the staged inbound record; a malformed/non-conformant document is rejected with a clear parse error, never silently stored; no supplier match → record left unlinked, staged for manual attach (no-guess-link). No contract coverage exists. |
| DEPACK-WIRE-4 | DATEV + GoBD exports; retention-hold error | Both exports extend the core export machinery via the pack's export profile (JUR-HOOK-5) and are audit-logged. Deletion/mutation of a record inside its statutory window is rejected with the pinned sentinel **`ErrRetentionHold` · `423 Locked`** carrying `retain_until` — for every role, no admin override (AC-gobd-4). No contract coverage exists. |
| DEPACK-WIRE-5 | E-signature flow surface | Buyer-side: choose level (AdES/QES), consent, redirect to the TSP, return with evidence; the evidence record is read through the offer/sign surface (corpus contract note). TSP failure returns an honest error with the offer unchanged. No contract coverage exists. |
| DEPACK-WIRE-6 | Trust-pack download + buyer share | Downloads (single artifact or full pack) are audit-logged and name the bound build; creating a buyer-facing link is 🟡 — queued to the approval inbox, expiring, audited (AC-trust-pack-6/7). No contract coverage exists. |

### Events
Source: specs/spec/contract/events.md#5-the-catalog @ 5a0b29c

The central catalog defines **no** E17 event. The package deliberately adds none
in this spec: a completed signature emits the existing `offer.accepted` (owned by
the catalog, semantics at [[offers-and-products]] OFFER-AC-2 /
[[event-bus#EVT-SEM-4]]) — identical to the tracked click, which is the
one-accept-path guarantee. Audit visibility rides `audit.appended`
([[audit-observability]] AUD-EVT-1) for generation, issue, exports, downloads,
and shares. If the build needs an invoice-lifecycle event, that is a catalog
extension to mint with the wire surface (DEPACK-WIRE-1/2) — not assumed here.

### Acceptance

#### Acceptance — the five stories (epic G/W/T, condensed faithfully)
Source: specs/spec/product/epics/E17-germany-package.md @ 5a0b29c (all five stories tier V0.5, verified sole-owned by this chapter per the scope roll-up)

| ID | Given/When/Then | Verification |
|---|---|---|
| DEPACK-AC-1a | Given an **accepted** offer, when I generate an invoice, then I get a valid XRechnung (EN-16931) XML and/or a ZUGFeRD hybrid PDF (human-readable PDF with embedded structured XML) — schema-validated; an invalid document is a build/test failure, not a silent emit. | Generation + validation tests in the pack lane (refuse-to-emit per JUR-XR-7) |
| DEPACK-AC-1b | Given the invoice totals, when produced, then they derive from the offer's server-computed net/tax/gross (integer minor-units + ISO-4217, ADR-0037) — never free-typed; a total that doesn't reconcile to the offer is rejected. | Reconcile + rejection integration test (mirrors the screen's red recon state) |
| DEPACK-AC-1c | Given accounting needs the data, when I export, then a DATEV-format export (the booking-relevant fields) is produced for the Steuerberater to ingest, and the export is audit-logged. | EXTF fixture-parse + account-split reconcile + audit-coverage tests |
| DEPACK-AC-1d | Given the mandate phases in, when an obligation date applies, then the workspace can **receive** structured e-invoices and **issue** them — the receive path parses an inbound XRechnung/ZUGFeRD into the record. | Inbound fixture test incl. negative fixture + no-guess-link assertion |
| DEPACK-AC-2a | Given a sent offer, when acceptance requires a signature, then the buyer can sign via an eIDAS-conformant flow (AdES or, where required, QES through a qualified TSP) — the signed artifact + its validation evidence attach to the offer/deal. | Integration test against a mock/seeded TSP; evidence-row fidelity test (DEPACK-DDL-2) |
| DEPACK-AC-2b | Given a signature completes, when it lands, then it runs the **same accept path** as the V1 tracked-click accept (status → accepted, deal amount syncs, accepted event emitted, audited) — an upgrade to the evidence, not a parallel flow. | Path-equivalence test: signed and clicked accepts produce identical record/event outcomes (cites OFFER-AC-2) |
| DEPACK-AC-2c | Given the TSP is a sub-processor, when used, then it is named in the DPA/sub-processor list (S-E17.3) and the data flow honors the operator's residency — no signature data leaks outside the declared chain. | Sub-processor-list presence test (trust-pack artifact set, DEPACK-PARAM-6) + residency assertion |
| DEPACK-AC-2d | Given V1 ships tracked-click accept, when a workspace doesn't need QES, then it works without the TSP — e-sign is an opt-in upgrade, not a hard dependency of the offer flow. | No-TSP configuration test: tracked click accepts fully |
| DEPACK-AC-3a | Given the trust pack, when opened, then it assembles the **current** artifacts for this instance — the full named set (DEPACK-PARAM-6). | Completeness test enumerating the applicable artifact set |
| DEPACK-AC-3b | Given an artifact with a validity/issue date, when shown, then it carries that date and its scope — stale or org-vs-partner-vs-fork provenance is explicit (no implying Gradion holds a partner's cert). | Metadata test: no undated artifact, no mislabeled holder |
| DEPACK-AC-3c | Given the instance is a customized fork, when I view the pack, then the SBOM + CRA DoC reflect **that fork** (rebuilt per S-E17.4), not mainline — what I'm shown matches what's running. | Fork-fixture integration test through the trust-pack surface |
| DEPACK-AC-3d | Given a completeness gate, when a release/pack assembles, then a check asserts every named artifact is present (extends the compliance-pack completeness check, SEC-CRA-6) — a missing artifact fails the gate. | Gate test: missing-artifact fixture → red, never a partial pack |
| DEPACK-AC-4a | Given a fork with client modifications, when a release is built, then a CycloneDX SBOM is regenerated across the modified Go + pnpm + pinned base-image trees — reproducibly; the SBOM reflects the fork's actual dependency set. | Fork-fixture test: a fork-introduced dependency appears in the SBOM (rides SEC-CRA-1 rails) |
| DEPACK-AC-4b | Given the regenerated SBOM, when conformity is assembled, then the CRA Declaration of Conformity is rebuilt for the fork and bound to that build's signed provenance (SLSA) — describing the running artifact, not mainline. | DoC-reference test against the fork's SBOM + provenance (SEC-CRA-2 binding) |
| DEPACK-AC-4c | Given the per-fork artifacts exist, when the trust pack is viewed for that instance, then it surfaces the fork's SBOM + DoC with provenance marking it as a modified fork. | Integration test through DEPACK-AC-3c's surface |
| DEPACK-AC-4d | Given a customization breaks a CRA gate (e.g. a flagged-vulnerable dependency), when the build runs, then the gate **fails before release** — a fork cannot ship non-conformant artifacts silently; the failing gate names the offending change. | Seeded-bad-dependency test on the fork build (rides the core scan gates, SEC-CRA-3) |
| DEPACK-AC-5a | Given a financially-relevant record, when its GoBD retention class applies, then it is retained for the statutory period and protected from deletion/mutation within that window — even against an admin — enforced, not policy-only. | Delete-attempt test rejected at the engine/DB layer, every role; class-set-on-creation test (DEPACK-PARAM-5) |
| DEPACK-AC-5b | Given an audit, when requested, then a machine-readable, structured export of the retained records (with their audit trail) is produced in a GoBD-acceptable form, itself audit-logged. | Export completeness + manifest-reconcile + audit-coverage tests; attribution-fidelity test (no row reduced to a bare system event) |
| DEPACK-AC-5c | Given GDPR erasure vs GoBD retention conflict, when they collide, then the precedence is explicit and defensible — the retention obligation overrides erasure for the statutory window with a documented basis; the system does not silently resolve it the wrong way. | Precedence test: erasure suppressed-not-applied + basis logged; window-elapse test: record becomes erasable |
| DEPACK-AC-5d | Given a restore, when it runs, then GoBD-retained records and their immutability survive the restore — consistent with the erasure-suppression-on-restore guarantee ([[operations#OPS-DR-6]]). | Restore-drill assertion in the pinned cadence ([[operations#OPS-DR-3]]) |

#### Acceptance — screen: e-invoice (`e-invoice.html`, S-E17.1) — corpus ACs verbatim
Source: specs/spec/product/30-screen-acceptance.md#e-invoicehtml--german-e-invoice--datev-implements-s-e171 @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-e-invoice-1 | Given the invoice header, When the screen loads, Then a "Derived from" chain shows Offer v3 (accepted 21 Jun) → Invoice RE-2026-0087, and a "reconciles to offer gross" chip confirms the invoice gross equals the accepted offer's server-computed gross (€177.072,00). | UI component test, DE screen lane |
| AC-e-invoice-2 | Given the invoice-lines table is mirrored read-only from accepted offer v3, When totals render, Then net subtotal (€148.800,00), VAT 19% MwSt. (€28.272,00), and gross (€177.072,00) are computed from the line `data-net`/`data-tax` values in integer minor-units — not free-typed — and the gross row carries a "derived from accepted offer" badge. | UI test against a seeded invoice |
| AC-e-invoice-3 | Given the "XRechnung (EN-16931 XML)" / "ZUGFeRD (hybrid PDF/A-3)" format tabs, When the user clicks ZUGFeRD, Then the ZUGFeRD pane shows a human-readable PDF/A-3 preview with `factur-x.xml` embedded, and the visible page totals match the XRechnung XML totals (cross-checked on build). | UI tab test + build cross-check (DEPACK-PARAM-2) |
| AC-e-invoice-4 | Given each format pane, When it is shown, Then a green "Valid" banner names the validation schema (EN-16931 · XRechnung 2.3 (UBL) / ZUGFeRD 2.2), states 0 errors / 0 warnings, and asserts the generator refuses to emit a document that fails the schema. | UI state test (validator per JUR-XR-7) |
| AC-e-invoice-5 | Given the DATEV export panel (right rail), When the user clicks "Export DATEV", Then an EXTF Buchungsstapel preview (Umsatz / Konto / Gegenkonto / BU / Beleg, SKR03: debitor 10000, revenue 8400, USt 1776) is shown and a toast confirms the export is generated and audit-logged for the Steuerberater. | UI + audit-coverage test |
| AC-e-invoice-6 | Given the invoice is in Draft, When the user clicks "Queue RE-2026-0087 for issue", Then the action is queued to the approval inbox (a human approves before a fiscal document is sent), the status pill flips to "Issued"/locked, and a toast notes issuing is human-gated; once issued the document is immutable (a correction is a Storno + new document). | UI + gate test riding [[approvals-and-concurrency#APPR-AC-7]] |
| AC-e-invoice-7 | Given the inbound "Receive" drop zone, When the user simulates a parse, Then a structured record renders (supplier Spedition Vogel KG, invoice no. VG-44120, net/gross) labelled "parsed · ZUGFeRD 2.2", and a toast confirms the inbound document was parsed into a structured record. | UI test over the inbound fixture (DEPACK-AC-1d) |
| AC-e-invoice-8 | Given the "Explain this total" control under the totals, When the user clicks it, Then a calculation box expands showing net = Σ(line_net), tax = Σ(line_net × 19%), gross = net + tax, and the reconcile rule that invoice gross must equal the accepted offer gross. | UI test |

#### Acceptance — screen: e-signature (`e-signature.html`, S-E17.2) — corpus ACs verbatim
Source: specs/spec/product/30-screen-acceptance.md#e-signaturehtml--eidas-e-signature-implements-s-e172 @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-e-signature-1 | Given the Sign state with the Angebot rendered, When the signer reads the document box, Then the frozen server-computed totals are shown (Netto 148.800,00 €, 19% MwSt. 28.272,00 €, Gesamt 177.072,00 €) labelled "computed server-side" with currency stated as EUR · ISO-4217 · integer minor-units, and no amount is editable in the form. | UI component test, DE screen lane |
| AC-e-signature-2 | Given the Sign state, When the signer clicks "Explain this total", Then a breakdown expands showing the per-line minor-unit sum (14.880.000 minor → 148.800,00 €), the tax derivation, and the gross, stating the signature binds these exact frozen amounts. | UI test |
| AC-e-signature-3 | Given AdES is selected by default, When the signer selects "Qualified signature (QES)", Then the AdES card deselects, the QES card highlights, the one-time identity-verification step (German eID / Video-Ident / Bank-Ident) is revealed, and the sign-button label updates to reference QES. | UI state test (DEPACK-PARAM-4) |
| AC-e-signature-4 | Given the consent checkbox is unchecked, When the screen loads, Then the sign button is disabled; When the signer checks the consent box ("I, Dr. Bär, intend to sign and accept this Angebot…"), Then the sign button becomes enabled. | UI test |
| AC-e-signature-5 | Given QES is selected and no identity method has been chosen, When the signer clicks the sign button, Then signing does not proceed — the identity step is highlighted/scrolled into view with a prompt that QES needs one-time identity verification first. | UI guard test |
| AC-e-signature-6 | Given a valid AdES or QES selection with consent given, When the signer proceeds, Then a "Redirecting to D-Trust…" progress state is shown stating Margince never sees the ID document (only the signed result + validation evidence return), and on completion a Signed state appears with a signature-evidence card (assurance level, TSP D-Trust GmbH · EU Trusted List, signer, signed-at timestamp, SHA-256 document hash, "VALID · LTV timestamped", bound amount 177.072,00 EUR). | UI flow test against the mock TSP; evidence card renders the DEPACK-DDL-2 record |
| AC-e-signature-7 | Given the Signed state, Then a sync notice confirms the deal moves to accepted/Won, the amount syncs from the guessed €212k to the signed €177.072,00, and `offer.accepted` is emitted and audited — identical to the tracked-click path, with download options for the signed PDF + evidence. | UI + path-equivalence assertion (DEPACK-AC-2b, OFFER-AC-2) |
| AC-e-signature-8 | Given the signer does not want a legal signature, When they choose "Accept with a tracked click instead", Then the offer is accepted via the V1 baseline (evidence label "Tracked click (no e-signature)") confirming e-sign is an opt-in upgrade, not a hard dependency. | UI fallback test (DEPACK-AC-2d) |

#### Acceptance — screen: trust pack (`trust-pack.html`, S-E17.3) — corpus ACs verbatim
Source: specs/spec/product/30-screen-acceptance.md#trust-packhtml--trust-pack-vertrauenspaket-implements-s-e173 @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-trust-pack-1 | Given the pack assembles cleanly for this release, When the screen loads, Then a green "Complete" status pill and a "Completeness gate passed" banner appear, stating all 9 required artifacts are present and bound to build 8f3a1c9 with SLSA provenance verified. | UI test against a seeded pack (gate per DEPACK-AC-3d) |
| AC-trust-pack-2 | Given the assembled-artifacts list is shown, When the user reads each row, Then every artifact carries an explicit issuer/holder badge (per-fork, Gradion-held, hosting partner, template, or qualifier note) and a validity line (issue/revision date, cert number, or build), so provenance is never ambiguous. | UI + metadata test (DEPACK-AC-3b) |
| AC-trust-pack-3 | Given an artifact with a "what this fork changed" / "inspect" / "sub-processors" link (CRA DoC, SBOM, DPA), When the user clicks the linkmini control, Then the corresponding drilldown panel (fork deltas, CycloneDX component counts, or EU/EEA sub-processor list) expands inline, and toggling again collapses it. | UI drilldown test |
| AC-trust-pack-4 | Given the BSI C5 / EUCS attestation is held by the hosting partner and its Type 2 report is dated more than 12 months ago, When the user views that row, Then the validity line renders in a stale style with an alert-triangle and the text "request current copy" rather than presenting the dated attestation as current. | UI stale-state test |
| AC-trust-pack-5 | Given the buyer is pharma (BÄR Pharma), When the pack is assembled, Then the §393 SGB V health-near notes appear with an "applies — buyer is pharma" marker, while TISAX (automotive) is shown dimmed as "Omitted by design / not in scope" with a disabled n/a button and a stated reason — the pack neither fabricates nor silently drops it. | Applicability test (DEPACK-PARAM-6 flags) |
| AC-trust-pack-6 | Given the user clicks any single "Download" button or "Download full pack (.zip)", When the action fires, Then a toast confirms the download is logged to audit and (for the full pack) names the bound build 8f3a1c9. | UI + audit-coverage test (DEPACK-WIRE-6) |
| AC-trust-pack-7 | Given the user clicks "Create buyer link → approval", When the share action fires, Then a toast confirms the link is queued to the approval inbox for a human to approve before it leaves the workspace, and the share panel states the link carries an expiry and audit entry. | UI + gate test (DEPACK-WIRE-6, [[approvals-and-concurrency#APPR-AC-7]]) |
| AC-trust-pack-8 | Given the honest-scope note at the bottom, When the user reads it, Then it states E17 assembles rather than manufactures evidence — ISO 27001 is Gradion's, BSI C5/EUCS is the partner's (A35), DPIA/works-council are templates/qualifiers, and only the SBOM + CRA DoC are generated per-fork (S-E17.4). | UI presence test |

#### Acceptance — screen: GoBD (`gobd.html`, S-E17.5) — corpus ACs verbatim
Source: specs/spec/product/30-screen-acceptance.md#gobdhtml--gobd-retention--audit-export-implements-s-e175 @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-gobd-1 | Given the screen at load, When it renders, Then a law banner states financially-relevant records are retained for the statutory period and protected from deletion/mutation within that window, carrying an "Enforced, not policy-only" shield badge titled "Enforced in the data layer, not a settings toggle". | UI component test, DE screen lane |
| AC-gobd-2 | Given the three-tab segmented control (Retention classes · Audit export · Erasure vs. retention), When I click a tab, Then only that pane is shown and the clicked tab gets the active style; "Retention classes" is the default active pane on load. | UI tab test |
| AC-gobd-3 | Given the Retention classes pane, When the table renders, Then it lists Invoices (10 yr), Accepted offers (6 yr), Sent offers (6 yr) each with an "immutable" lock pill and a row count, and Draft offers (not sent) with "—" period and an "editable" pencil pill, under a note that "Class is derived from record type — not free-set". | UI test (values pinned at DEPACK-PARAM-5) |
| AC-gobd-4 | Given the "Try to delete a retained record" proof card for Rechnung RE-2026-0091, When I click "Delete record", Then a red denied panel appears reading "Deletion blocked — GoBD legal hold" stating the block applies to every role including admin with no override toggle, shows the error sentinel `ErrRetentionHold · 423 Locked · retain_until=2036-12-31`, and a toast confirms the block. | UI state test surfacing the DEPACK-WIRE-4 sentinel (engine rejection per DEPACK-AC-5a) |
| AC-gobd-5 | Given the Audit export pane, When I toggle record-class buttons (Invoices/Accepted offers/Sent offers), pick a format (GDPdU/IDEA `.csv + index.xml` vs Structured `.json`), or edit the period date inputs, Then the manifest preview rebuilds live — the filename, size, per-class row counts, and a `records (computed)` total derived by summing canonical per-class counts (412/37/128), never free-typed. | UI test (formats per DEPACK-PARAM-7) |
| AC-gobd-6 | Given the export builder, When no record class is selected, Then the manifest shows "No record classes selected — nothing to export. Pick at least one class above.", size reads 0 B, and clicking "Generate export" instead raises a toast "Pick at least one record class to export" without logging. | UI empty-scope test |
| AC-gobd-7 | Given at least one class selected, When I click "Generate export — logs to audit trail", Then a new "Audit export · N records" entry is prepended to the append-only audit-trail rail (with actor, timestamp, and a sha256 content hash) and a toast confirms the record count was logged. | UI + audit-coverage test (DEPACK-AC-5b) |
| AC-gobd-8 | Given the Erasure vs. retention pane, When it renders, Then a side-by-side scale shows "GDPR Art. 17 erasure" vs. a highlighted "GoBD / § 147 AO retention" marked as winning for the statutory window, with a documented-basis note that the record is restricted (gesperrt) not deleted, auto-purged at `retain_until`, and the split decision is written to the audit log. | UI test (precedence per DEPACK-AC-5c) |

Note DEPACK-AC-N-1 (prototype gaps, carried honestly from the corpus screen
notes): the e-invoice prototype ships no real file upload and no exercised
invalid-validation banner state; the e-signature prototype's TSP round-trip is
simulated and there is no network loading state; the trust-pack prototype lacks a
true loading skeleton and an empty-pack state; the GoBD prototype does not
validate the export date range server-side and generates no actual file. Each is
build scope for the implementing tickets, not a licence to drop the designed
states — the cross-cutting screen-state floor is [[acceptance-standards]]'s.
