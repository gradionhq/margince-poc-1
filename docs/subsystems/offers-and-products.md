---
status: planned
module: backend/internal/modules/people
derives-from:
  - margince specs/spec/features/01-core-objects.md#10-offers--angebote--the-bounded-line-item-quote-engine-a48adr-0037
  - margince specs/spec/features/07-ai-native-moments.md#5e-offerangebot-authoring--draft-from-context--regenerate-from-signal
  - margince specs/spec/contract/data-model.md#126-offers--angebote-the-bounded-line-item-quote-engine--a48adr-0037
  - margince specs/spec/decisions/ADR-0037-offers-angebote-engine.md
  - margince specs/spec/product/epics/E03-pipeline-and-deals.md#s-e037--build-send--close-on-a-real-offer-angebot-ai-drafted-from-the-conversation
  - margince specs/spec/product/30-screen-acceptance.md#offerhtml--offer--angebote-builder-implements-s-e037
---
# Offers & products — the Angebot as a first-class sales record: AI-drafted, server-priced, immutable once sent

> The bounded line-item quote engine (ADR-0037): a versioned offer bound to one
> deal, built from an optional rate-card or free-form, drafted by the agent from
> the deal's captured context under evidence-or-omit, priced only by the server,
> sent through the confirm-first gate, and accepted in the deal room. Its promise:
> the offer the buyer accepted is exactly the record the forecast reads — no
> free-typed total, no fabricated price, no silently re-priced history.

## What it's for

For the regulated Mittelstand beachhead (ADR-0033) the Angebot is the central
sales artifact — a deal does not progress without one — yet before this subsystem
it could only be attached as an opaque document, so auto-capture, the forecast,
and the buyer checklist all lost the highest-signal record in the sale
(ADR-0037). This subsystem makes the offer a first-class, versioned object on the
deal: typed line items, deterministic server-computed money, a branded PDF, and a
tracked acceptance that becomes the deal's value source. Its callers are the
offer-builder surface on the deal, the AI authoring moment (draft-from-context
and regenerate-from-signal), the approval inbox (sending is confirm-first), the
deal room (where the buyer accepts), and the forecast (which reads the accepted
gross). The boundary is deliberate: this is a sales record, not CPQ — no product
configurator, no pricing-rule engine, no quote-approval workflow graph; bespoke
pricing logic is a source-level engagement (ADR-0002), in the same spirit as the
scope's rejection of runtime rules engines ([[scope#NEVER-2]]).

## Principles it serves

- **P11 — money is code.** Totals are derived server-side from the line items in
  integer minor units ([[data-model#DM-CONV-9]]); a client-supplied total is
  rejected, and multi-currency offers roll up to base through the same stored-rate
  FX machinery as deals ([[data-model#DM-FX-1..7]]).
- **P5 — assembled-for-you.** The first draft is built from what was actually
  discussed — transcript lines, prior emails, the org's prior accepted offers,
  the rate-card — instead of being typed from a blank form.
- **P12 — evidence, audit, governed action.** Every AI-drafted line cites its
  source or is omitted ([[acceptance-standards#GATE-AI-1]]); every write is
  audited and emits exactly one domain event; sending rides the approval gate.
- **P1 / ADR-0002 — no runtime engine.** Products are data, totals are code, and
  a structurally new template layout is a source-level theme, not a CMS.
- **ADR-0037 — the bounded Angebote engine.** The decision this chapter embodies:
  promote the everyday line-item offer to V1 while keeping full CPQ explicitly
  out — the bright line between a rate-card quote and a configurator.
- **ADR-0026 / ADR-0036 — tiered and token-bound.** Drafting and regenerating are
  auto-execute (reversible internal writes); sending leaves the workspace and is
  confirm-first, bound by the approval token
  ([[approvals-and-concurrency#APPR-WIRE-1]]).

## How it works

**The rate-card is optional data, not a configurator.** A workspace may maintain
a product catalogue — name, optional SKU, unit price with its currency, a default
tax rate (OFFER-PARAM-1) — or quote entirely free-form; a workspace with zero
products can still build and send a complete offer. When a line references a
product, the price and description are *copied onto the line as a snapshot*, so a
later rate-card change never silently re-prices an offer that already went out.

**An offer is born as a draft on one deal.** It carries a human-facing offer
number, a revision counter, a currency, an optional validity date
(OFFER-PARAM-3), and a status that only ever moves draft → sent → accepted,
rejected, expired, or superseded. While in draft it is freely editable — lines
added, quantities changed, discounts applied — and every edit re-derives the
money: line net is quantity times unit price less the discount percentage,
rounded in integer minor units, with tax applied per line rate and the net, tax,
and gross totals summed from the lines (OFFER-PARAM-4). The client never states a
total; a request that tries is rejected as a validation failure
([[api-conventions#API-ERR-15]]).

**The AI drafts; it never prices from thin air.** On demand, draft-from-context
assembles a first-draft offer from the deal's captured context under the
evidence-or-omit gate: every proposed line carries the snippet and source it came
from, and a line that cannot be grounded is omitted, never invented. Prices come
from the rate-card or are left blank for the rep — the agent never fabricates a
price ([[acceptance-standards#GATE-AI-1]]). When scope changes ("they added a
second site"), regenerate-from-signal re-derives the offer from the latest signal
into a *new draft revision*, presented as a diff — added, removed, changed lines
— against the prior, for the rep to accept line by line, never a silent
overwrite. Proposed lines are staged: they persist and count toward totals only
on the rep's accept ([[acceptance-standards#GATE-AI-2]]), and every generative
output renders the AI-assisted disclosure ([[acceptance-standards#GATE-AI-9]]).
Both moments are auto-execute-tier internal writes; neither can cause a send.

**Sending is confirm-first and freezes the record.** Rendering produces a
branded PDF from a governed, workspace-level template (DE/EN locale,
OFFER-PARAM-2), whose rendered totals must equal the server-computed totals.
Sending leaves the workspace, so for an agent it stops at the approval inbox
with the rendered PDF for one-tap approve/edit/reject, token-bound like every
confirm-first action ([[approvals-and-concurrency#APPR-WIRE-1]],
[[api-conventions#API-ERR-10]]). At send the buyer and issuer blocks are
snapshotted and, in a multi-currency workspace, the FX rate to base is frozen
(OFFER-PARAM-5) — a missing rate hard-fails rather than defaulting to parity.
From that moment the offer is an immutable record: any change happens by
regenerating into the next revision as a fresh draft, with the prior marked
superseded.

**Acceptance happens in the deal room; its meaning is pinned here.** The sent
offer is shared in the deal's deal room, where buyer view, open, and accept are
recorded as company-level engagement events — the deal-rooms chapter owns that
surface, its tables, and the buyer's story. What *this* chapter owns is the
accept semantics: accept flips the offer to accepted, syncs the deal's amount
from the accepted offer's server-computed gross — the offer becomes the deal's
value source, restoring forecast honesty — and emits the accepted event paired
with a deal update under one correlation ([[event-bus#EVT-SEM-4]]), fully
audited. V1 acceptance is a tracked in-room action plus the audit trail;
legally-binding e-signature is a fast-follow owned elsewhere.

## What's configurable

- **The rate-card itself** — per-workspace data: present or absent, each product
  active or inactive, with unit, currency, and a default tax rate the line may
  override (OFFER-PARAM-1). Data only — no bundles, options, or pricing rules.
- **Offer templates** — selection and parameters (logo, header/footer, terms
  boilerplate, locale) are runtime, with one default per locale (OFFER-PARAM-2);
  a structurally new layout is a source-level theme (ADR-0002). Terms and intro
  blocks may reuse approved assets from the governed asset store.
- **Validity date** — per offer, optional; drives the expired status
  (OFFER-PARAM-3).
- **What is not configurable** — the totals derivation, the send tier, and the
  immutability of sent offers are invariants, not knobs.

## Guarantees (enforced)

- **Server-priced, always.** Net, tax, and gross are derived from the lines in
  integer minor units; a client-supplied total is rejected, and a reconciliation
  test ties every offer to its lines (OFFER-AC-3).
- **No fabricated price.** An AI-drafted line carries grounded evidence or is
  omitted; a price the agent cannot ground in the rate-card or the conversation
  renders blank for the rep (OFFER-AC-4, [[acceptance-standards#GATE-AI-1]]).
- **Sent means sealed.** An offer is editable only while draft; regeneration of a
  sent offer creates the next revision as a fresh draft and marks the prior
  superseded — history is never rewritten in place (OFFER-AC-1).
- **Accept syncs the forecast.** Acceptance flips the status and syncs the deal's
  amount from the accepted gross, with a paired deal update on the same
  correlation (OFFER-AC-2, [[event-bus#EVT-SEM-4]]).
- **FX honesty.** Multi-currency offers roll up to the workspace base via the
  stored rate frozen at send; an unavailable rate hard-fails, never silently
  rate-one (OFFER-AC-8).
- **Snapshot pricing.** A rate-card change never re-prices an already-sent offer
  — the line copied its price (OFFER-AC-6).
- **Governed send.** Sending is confirm-first for agents — approval-required
  without a valid token, token-bound to the exact staged effect
  (OFFER-AC-10, [[approvals-and-concurrency#APPR-AC-7]]).
- **Audited like everything else.** Every offer write carries provenance and
  emits exactly one audit row and one domain event (OFFER-AC-12,
  [[data-model#DM-CONV-11]]).

## Acceptance

Done means: a rep can build an offer from the rate-card or free-form and watch
exact, server-computed totals; the agent can draft it from the deal's captured
context with evidence on every line and blanks where it cannot ground a price; a
scope change regenerates a diffed new revision instead of overwriting; sending
queues to the approval inbox with the rendered branded PDF and seals the sent
revision; and the buyer's accept in the deal room flips the status, syncs the
deal amount, and leaves a complete audit trail. The honest states are part of the
contract: an empty draft from a context-less deal, a blank unpriced line that is
excluded from totals until priced, locked prior revisions, and the not-yet-
accepted room state. The ownership split is explicit: S-E03.7 (build, send,
close on a real Angebot) is this chapter's story, including the offer-builder
screen; S-E08.6 (the buyer accepts in the deal room) is the deal-rooms chapter's
story and screen — but the accept *semantics* (sent offers immutable, accept
syncs the deal amount, totals server-computed) are pinned here
(OFFER-AC-1..3) and deal-rooms cites them. Testable forms live in the Acceptance
appendix; the cross-cutting screen-state floor is inherited from the
acceptance-standards chapter.

## Out of scope

- **CPQ, in every form** — product configurators, bundles and option trees,
  pricing-rule and discount-approval engines, quote-approval workflow graphs:
  OUT, per the ADR-0037 bright line and the scope's no-runtime-rules-engine
  stance ([[scope#NEVER-2]]); bespoke pricing is a source-level engagement.
- **The deal room surface and engagement tracking** — the buyer-facing room, its
  engagement events (including the offer-accept event type on that stream), and
  S-E08.6 are owned by deal-rooms; this chapter pins only what accept *means*.
- **E-signature** — a fast-follow connector and sub-processor decision; the
  eIDAS e-sign surface and its evidence table belong to the germany-package
  chapter.
- **Invoicing** — an accepted offer is not an invoice. No invoice table exists in
  the V1 schema (none appears in the schema ownership index,
  [[data-model#Schema — ownership index]]); the German e-invoice feature is the
  germany-package chapter's.
- **The deal screen and deal money model** — the deal 360 where offers surface,
  and the deal's own amount/FX columns, are deals-and-pipeline's.
- **Approval token mechanics** — mint, binding, single-use, expiry:
  approvals-and-concurrency; this chapter's send simply rides that gate.

## Where it lives

The offers seam inside the backend's shared crm-core domain module (the same
module directory where deals and their satellites live and emit), plus the
offer-builder surface in the web app's deal area. Read next: deals-and-pipeline
(the deal whose value this record becomes), deal-rooms (where the buyer
accepts), approvals-and-concurrency (the send gate), data-model (money and FX
conventions), and event-bus (the offer event family).

## Appendix

### Parameters
Source: features/01-core-objects.md#101-products--rate-card-optional @ 5a0b29c; features/01-core-objects.md#102-the-offer-angebot-record--line-items @ 5a0b29c; contract/data-model.md#126-offers--angebote-the-bounded-line-item-quote-engine--a48adr-0037 @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| OFFER-PARAM-1 | Product default tax rate | `default_tax_rate numeric(5,2)`, default `0`; percent (e.g. `19.00` DE USt.) | Copied to a line on pick; per-line override allowed. Data, not a rule engine. |
| OFFER-PARAM-2 | Template locale + default | `locale` default `de-DE`; DE/EN at launch; at most one default template per locale (partial unique index, OFFER-DDL-4) | Template selection + parameters are runtime; a structurally new layout is a source-level theme (ADR-0002). |
| OFFER-PARAM-3 | Offer validity | `valid_until date NULL`, per offer | Drives the `expired` status; no default. |
| OFFER-PARAM-4 | Totals derivation | line net = round(`quantity` × `unit_price_minor` × (1 − `discount_pct`/100)) in integer minor units; line tax = line net × `tax_rate`/100; offer `net/tax/gross_minor` = Σ lines | Derived in code, stored only for the rendered record and re-checked by test; line totals are never stored as free values (P11; screen proof AC-offer-3/-6). |
| OFFER-PARAM-5 | FX freeze moment | `fx_rate_to_base` + `fx_rate_date` frozen at **send** | Base-currency roll-up exactly like deals ([[data-model#DM-FX-3..5]], RT-PR-C2); missing rate → `422 fx_rate_unavailable`, never rate=1 (OFFER-AC-8). |
| OFFER-PARAM-6 | Revision rule | `revision integer`, starts 1; regenerating a `sent` offer creates revision n+1 as a fresh `draft`, prior → `superseded` | The versioning invariant behind OFFER-AC-1; unique per (workspace, offer number, revision) (OFFER-DDL-2). |

### Schema
Source: contract/data-model.md#126-offers--angebote-the-bounded-line-item-quote-engine--a48adr-0037 @ 5a0b29c

Four tables, owned here per the schema ownership index ([[data-model#Schema — ownership index]]). Each carries the universal base columns; user-mutable ones carry `version` ([[data-model#DM-CONV-3..4]]); money follows [[data-model#DM-CONV-9]].

OFFER-DDL-1 — `product` (optional rate-card / catalogue entry):

```sql
CREATE TABLE product (                                    -- a sellable item on the rate-card; OPTIONAL (a workspace may quote fully free-form)
  -- + base columns + version
  name          text NOT NULL,
  sku           text NULL,
  description   text NULL,
  unit          text NOT NULL DEFAULT 'unit',             -- 'unit' | 'hour' | 'day' | 'licence' | 'month' (free text; display only)
  unit_price_minor bigint NOT NULL,
  currency      char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  default_tax_rate numeric(5,2) NOT NULL DEFAULT 0,        -- percent, e.g. 19.00 (DE USt.); per-line override allowed
  active        boolean NOT NULL DEFAULT true,
  source        text NOT NULL,
  captured_by   text NOT NULL,
  UNIQUE (workspace_id, sku) -- sku optional; uniqueness only enforced when present (partial index below)
);
CREATE UNIQUE INDEX uq_product_sku ON product (workspace_id, sku) WHERE sku IS NOT NULL AND archived_at IS NULL;
CREATE INDEX idx_product_active ON product (workspace_id, active) WHERE archived_at IS NULL;
```

OFFER-DDL-2 — `offer` (a versioned Angebot bound to one deal):

```sql
CREATE TABLE offer (
  -- + base columns + version
  deal_id       uuid NOT NULL REFERENCES deal(id) ON DELETE RESTRICT,
  offer_number  text NOT NULL,                             -- human-facing "Angebot" number, unique per workspace
  revision      integer NOT NULL DEFAULT 1,                -- bumped when a SENT offer is regenerated → new revision (prior → superseded)
  status        text NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft','sent','accepted','rejected','expired','superseded')),
  currency      char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),

  -- snapshots (so a sent offer is a fixed record even if the deal/org later change)
  buyer_org_id  uuid NULL REFERENCES organization(id) ON DELETE SET NULL,
  buyer_snapshot jsonb NULL,                               -- name/address/VAT captured at send time
  issuer_snapshot jsonb NULL,                              -- seller legal block at send time

  valid_until   date NULL,                                 -- offer expiry (drives 'expired')
  intro_text    text NULL,                                 -- AI-drafted / templated cover blurb
  terms_text    text NULL,                                 -- boilerplate (may ref a drafting_asset, A42)

  -- computed money totals (DERIVED from line items in code; stored for the rendered record, re-checked by test)
  net_minor     bigint NOT NULL DEFAULT 0,
  tax_minor     bigint NOT NULL DEFAULT 0,
  gross_minor   bigint NOT NULL DEFAULT 0,
  fx_rate_to_base numeric(20,10) NULL,                     -- frozen at send (base-currency roll-up, RT-PR-C2)
  fx_rate_date  date NULL,

  template_id   uuid NULL REFERENCES offer_template(id) ON DELETE SET NULL,
  pdf_asset_ref text NULL,                                 -- rendered PDF (attachment/blob ref)
  accepted_at   timestamptz NULL,
  source        text NOT NULL,
  captured_by   text NOT NULL,                             -- 'agent:draft' on AI-authored drafts; 'human:*' on edits/send
  CONSTRAINT offer_number_rev_unique UNIQUE (workspace_id, offer_number, revision),
  CONSTRAINT offer_accepted_at CHECK (status <> 'accepted' OR accepted_at IS NOT NULL)
);
CREATE INDEX idx_offer_deal ON offer (workspace_id, deal_id, revision DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_offer_status ON offer (workspace_id, status) WHERE archived_at IS NULL;
```

OFFER-DDL-3 — `offer_line_item` (a typed line; price snapshot copied from `product`):

```sql
CREATE TABLE offer_line_item (
  -- + base columns
  offer_id      uuid NOT NULL REFERENCES offer(id) ON DELETE CASCADE,
  position      integer NOT NULL,                          -- display order, unique per offer
  product_id    uuid NULL REFERENCES product(id) ON DELETE SET NULL,  -- optional rate-card ref
  description   text NOT NULL,                              -- snapshot (free-typed or copied from product)
  unit          text NOT NULL DEFAULT 'unit',
  quantity      numeric(14,3) NOT NULL CHECK (quantity > 0),
  unit_price_minor bigint NOT NULL,                         -- snapshot — never re-read from product after send
  discount_pct  numeric(5,2) NOT NULL DEFAULT 0 CHECK (discount_pct BETWEEN 0 AND 100),
  tax_rate      numeric(5,2) NOT NULL DEFAULT 0,
  -- line_net/line_tax/line_total are DERIVED in code (qty × unit_price × (1-discount), + tax); not stored as free values
  evidence      jsonb NULL,                                 -- {snippet, source_id} when AI-drafted (evidence-or-omit, features/07)
  UNIQUE (offer_id, position)
);
CREATE INDEX idx_oli_offer ON offer_line_item (offer_id, position);
```

OFFER-DDL-4 — `offer_template` (branded, governed PDF layout, DE/EN):

```sql
CREATE TABLE offer_template (
  -- + base columns + version
  name          text NOT NULL,
  locale        text NOT NULL DEFAULT 'de-DE',             -- DE/EN at launch
  is_default    boolean NOT NULL DEFAULT false,
  layout        jsonb NOT NULL,                             -- logo/header/footer/terms-block refs (bounded params, not a CMS)
  UNIQUE (workspace_id, name)
);
CREATE UNIQUE INDEX uq_offer_template_default ON offer_template (workspace_id, locale) WHERE is_default AND archived_at IS NULL;
```

Note OFFER-DDL-N-1: acceptance tracking reuses `deal_room` / `deal_room_engagement_event` (owned by deal-rooms; that table's `event_type` enum gains `offer_accept`). No invoice table exists anywhere in the corpus schema — the e-invoice feature (E17) is germany-package's, and its detail is a named corpus thin spot, not a table this chapter can pin.

### Wire
Source: contract/crm.yaml (OFFERS / ANGEBOTE planned-resources block, comments only) @ 5a0b29c

**Honest contract-coverage finding (D-H2 contract-extension item):** at pin time
`crm.yaml` defines 81 operations across its paths, and **none** is an offer,
product, or offer-template operation — the offers surface exists in the contract
only as the net-new-resources *comment block* quoted below. The chapter
therefore pins the promised surface by path + behavior; operationIds do not yet
exist and must be minted by a contract extension before any docs-cited
operationId can resolve (the D-H2 "ships complete / every cited operationId
resolves" gate). Until then, no prose or ticket may cite an offer operationId as
if it existed.

| ID | Element (planned path) | Behavior pinned |
|---|---|---|
| OFFER-WIRE-1 | `/products` | Rate-card CRUD (list/get/create/update If-Match/archive); MCP `search_records`/`create_record`, tier 🟢. OPTIONAL — a workspace may quote free-form. |
| OFFER-WIRE-2 | `/offer-templates` | Branded DE/EN PDF layouts; admin surface; at most one default per locale (OFFER-DDL-4). |
| OFFER-WIRE-3 | `/deals/{id}/offers` | List/create an offer under a deal; `offer.revision` versioning (OFFER-PARAM-6). |
| OFFER-WIRE-4 | `/offers/{id}` | Get/update — update allowed **only while `status=draft`**; If-Match concurrency ([[api-conventions#API-CC-2]]). |
| OFFER-WIRE-5 | `/offers/{id}/line-items` | Nested CRUD; line totals are SERVER-COMPUTED, not client-set; a client-sent `line_total` → 422 ([[api-conventions#API-ERR-15]]). |
| OFFER-WIRE-6 | `POST /offers/{id}/regenerate` | 🟢 AI: regenerate-from-signal → new draft revision + diff; `x-mcp-tool` verb `draft_offer`, tier green. |
| OFFER-WIRE-7 | `POST /offers/{id}/render` | Produce the branded PDF → `pdf_asset_ref`; rendered totals must equal server-computed totals (OFFER-AC-9). |
| OFFER-WIRE-8 | `POST /offers/{id}/send` | 🟡 — leaves the workspace (publish to Deal Room / email PDF); agent call without a valid token → `ErrRequiresApproval` ([[api-conventions#API-ERR-10]]); token binding per [[approvals-and-concurrency#APPR-WIRE-1]]. Freezes FX + snapshots (OFFER-PARAM-5). |
| OFFER-WIRE-9 | `POST /offers/{id}/accept` | Flip `status=accepted`, sync `deal.amount_minor` from `gross_minor`, emit `offer.accepted` (+ paired deal update, [[event-bus#EVT-SEM-4]]); in-room action — no e-sign in V1. |

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c

Five offer events, defined and owned by the central event catalog
([[event-bus]]); cited here, never redefined: `offer.created`, `offer.sent`,
`offer.accepted`, `offer.rejected`, `offer.superseded`. Semantics this chapter
leans on: `offer.sent` is 🟡 — it rides the approval gate and, for webhook
consumers, delivers only after the gate clears; `offer.accepted` pairs a
`deal.updated` under the same correlation id with server-computed money only
([[event-bus#EVT-SEM-4]]); every offer write emits exactly one audit row + one
domain event ([[event-bus#EVT-SEM-1]]).

### Acceptance

#### Acceptance — accept semantics (pinned here; deal-rooms cites)
Source: features/01-core-objects.md#102-the-offer-angebot-record--line-items @ 5a0b29c; features/01-core-objects.md#104-templated-pdf--deal-room-acceptance @ 5a0b29c; decisions/ADR-0037-offers-angebote-engine.md @ 5a0b29c

S-E08.6 (the buyer accepts in the deal room) is the deal-rooms chapter's story and screen; the semantics below are this chapter's single-home pins.

| ID | Given/When/Then | Verification |
|---|---|---|
| OFFER-AC-1 | Given a `sent` offer, when any mutation is attempted, then it is rejected — a sent offer is an immutable record; regeneration creates the next `revision` as a fresh `draft` and marks the prior `superseded`, never editing in place. | Integration test, offers lane |
| OFFER-AC-2 | Given a `sent` offer, when the buyer accepts, then `offer.status=accepted` (+ `accepted_at`), the deal's `amount_minor` syncs from the accepted offer's `gross_minor` (the offer becomes the deal's value source), and `offer.accepted` is emitted with a paired `deal.updated` on one correlation id ([[event-bus#EVT-SEM-4]]), fully audited. | Integration test, offers lane + event assertion |
| OFFER-AC-3 | Given any offer, when totals are read or written, then net/tax/gross are derived server-side from the line items (OFFER-PARAM-4); a client-supplied `line_total`/`gross` is rejected (`422`), and a reconciliation test ties every offer's stored totals to its lines. | Integration + reconciliation test |

#### Acceptance — S-E03.7 (condensed G/W/T)
Source: product/epics/E03-pipeline-and-deals.md#s-e037--build-send--close-on-a-real-offer-angebot-ai-drafted-from-the-conversation @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| OFFER-AC-4 | Given a deal, when I create an offer, then I build it from an optional rate-card or free-form with typed line items (qty × unit price, discount %, tax), totals computed server-side in integer minor units + ISO-4217, multi-currency rolling up to base via the same FX machinery as deals. | Integration test (OFFER-AC-3/-8 pin the hard edges) |
| OFFER-AC-5 | Given the deal's captured context, when I ask the agent to draft, then every proposed line cites its source text under evidence-or-omit and no price is fabricated (ungrounded prices blank); on scope change, regenerate-from-signal produces a new draft revision shown as a diff against the prior. | Deterministic AI-lane test (OFFER-AC-13..18) |
| OFFER-AC-6 | Given a ready draft, when it is sent, then sending is 🟡 (queues to the approval inbox with the rendered branded DE/EN PDF for approve/edit/reject); a sent offer is immutable and a change creates the next revision. | Integration test riding [[approvals-and-concurrency#APPR-AC-7]] |
| OFFER-AC-7 | Given buyer engagement, when they view/open/accept in the Deal Room, then engagement is tracked and Accept runs the OFFER-AC-2 semantics. | Deal-rooms E2E cites OFFER-AC-2 |
| OFFER-AC-8 | Given a multi-currency workspace, when an offer converts to base, then it uses the frozen-at-send `fx_rate_to_base`; a missing rate hard-fails `422 fx_rate_unavailable`, never rate=1. Given V1, when I look for configurators, pricing-rule/discount-approval engines, or full CPQ, then none exist — products are data, totals are code, bespoke pricing is source (ADR-0002; [[scope#NEVER-2]] spirit). | Integration test + static scope assertion |

#### Acceptance — features/01 §10 capability ACs (verbatim)
Source: features/01-core-objects.md#101-products--rate-card-optional; #102-the-offer-angebot-record--line-items; #103-ai-authoring--draft-from-context--regenerate-from-signal-the-differentiator; #104-templated-pdf--deal-room-acceptance @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| OFFER-AC-9a | `product.unit_price_minor` is integer minor-units + ISO-4217 (no float). | Schema + unit test |
| OFFER-AC-9b | A rate-card price change **never** re-prices an already-`sent` offer (the line copies a price snapshot, §10.2). | Integration test |
| OFFER-AC-9c | A workspace with **zero** products can still build and send a fully free-typed offer (catalogue is optional). | Integration test |
| OFFER-AC-10a | Money totals (net/tax/gross) are **derived server-side** from the line items — a client-supplied `line_total`/`gross` is rejected (`422`); a totals test reconciles every offer to its lines (no free-typed total, P11). | Same lane as OFFER-AC-3 |
| OFFER-AC-10b | A multi-currency-workspace offer rolls up to the **workspace base currency** via `fx_rate` (frozen at send), exactly like deals (RT-PR-C2); a missing rate hard-fails (`422 fx_rate_unavailable`), never rate=1. | Integration test |
| OFFER-AC-10c | An offer is editable **only while `status=draft`**; **sending is 🟡** (`ErrRequiresApproval` for agents — it leaves the workspace) and bound by the ADR-0036 approval token. | Integration test riding APPR-* |
| OFFER-AC-10d | Regenerating a `sent` offer creates the **next `revision` as a fresh `draft`** and marks the prior `superseded` — a sent offer is an immutable record. | Integration test |
| OFFER-AC-10e | On **accept**, `offer.status=accepted`, the **deal's `amount_minor` syncs from the accepted offer's `gross_minor`** (the offer becomes the deal's value source — forecast honesty restored), and `offer.accepted` is emitted + audited. | Same lane as OFFER-AC-2 |
| OFFER-AC-10f | Every offer write carries `source`+`captured_by` and emits exactly one audit row + one domain event (P12). | Integration test + audit assertion |
| OFFER-AC-11a | Each AI-drafted line carries `{description, qty, unit_price?, evidence:{snippet, source_id}}`; a line with no grounding is omitted (test: assert evidence on every AI line, 0 fabricated lines). | Deterministic AI-lane test ([[acceptance-standards#GATE-AI-1]]) |
| OFFER-AC-11b | The AI **never writes a price it cannot ground** in the rate-card or the conversation — un-priced lines render blank for the rep (test: no fabricated `unit_price`). | Deterministic AI-lane test ([[acceptance-standards#GATE-AI-1]]) |
| OFFER-AC-11c | Draft + regenerate are **🟢** (reversible internal writes); the resulting offer cannot be sent without the **🟡** send gate. | Integration test |
| OFFER-AC-11d | Regenerate produces a **diff** (added/removed/changed lines) against the prior revision, not a silent overwrite. | Deterministic test |
| OFFER-AC-12a | Rendering produces a PDF stored as an attachment ref (`offer.pdf_asset_ref`); the rendered totals equal the server-computed totals (no drift). | Render integration test |
| OFFER-AC-12b | A sent offer is shared in the deal's **Deal Room**; buyer **view/open/accept** are recorded as `deal_room_engagement_event`s (the only V1 "view" signal, RT-PR-H2) — these feed the honest engagement signal set, not "email opens". | Deal-rooms lane (table owned there) |
| OFFER-AC-12c | **Accept** in the room flips `offer.status=accepted` and runs the §10.2 deal-sync + event; acceptance is recorded with its audit trail (V1 = tracked action; e-sign fast-follow). | Same lane as OFFER-AC-2 |

#### Acceptance — features/07 §5e ACs (verbatim)
Source: features/07-ai-native-moments.md#5e-offerangebot-authoring--draft-from-context--regenerate-from-signal @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| OFFER-AC-13 | Given a deal with captured context, draft-from-context produces an offer whose every AI line carries a **non-empty `evidence_snippet` + source id or is omitted** — an ungrounded line is a hard failure. *(deterministic test: assert evidence on every AI line; no-context fixture → honest empty draft, 0 fabricated lines)* | Deterministic AI-lane test ([[acceptance-standards#GATE-AI-1]]) |
| OFFER-AC-14 | The AI **never emits a `unit_price` it cannot ground** in the rate-card or the conversation; un-priced lines render blank for the rep. *(test: no fabricated price; rate-card fixture → catalogue price; no-match fixture → blank)* | Deterministic AI-lane test ([[acceptance-standards#GATE-AI-1]]) |
| OFFER-AC-15 | Before the rep accepts, the draft is a `status=draft` offer that **cannot be sent** without the §5.6 🟡 gate; draft + regenerate write **zero** sends. *(test)* | Integration test |
| OFFER-AC-16 | Regenerate produces a **diff** vs the prior revision (added/removed/changed lines) and creates the **next `revision` as a fresh draft**, marking the prior `superseded`; a `sent` offer is never mutated in place. *(test: regenerate fixture → diff + new revision + prior superseded)* | Deterministic test |
| OFFER-AC-17 | Money totals on the draft are **server-computed** (`features/01 §10.2`); the AI proposes lines, not totals. *(test)* | Same lane as OFFER-AC-3 |
| OFFER-AC-18 | Output renders the Art. 50 AI-assisted disclosure (§11 gate 9). | UI assertion ([[acceptance-standards#GATE-AI-9]]) |

#### Acceptance — screen (offer builder, `offer.html`, primary story S-E03.7)
Source: product/30-screen-acceptance.md#offerhtml--offer--angebote-builder-implements-s-e037 @ 5a0b29c

This chapter owns one screen: the offer builder (`offer.html`), whose primary
story is S-E03.7. (Corpus drift note: the screen→story index row still says
"not yet prototyped" for S-E03.7, while §6.1 of the same doc ships the built
screen with derived ACs — treat §6 as the live coverage, per that doc's own
note.) The buyer-facing room screen (`deal-room.html`, S-E08.4/S-E08.6 with
AC-deal-room-1..8) is deal-rooms'. Corpus screen-AC IDs preserved; text
condensed — the source carries the full prototype detail.

| ID | Given/When/Then (condensed) | Verification |
|---|---|---|
| AC-offer-1 | Given the offer header, when the screen loads, then it shows the offer title, a link to the parent deal, a Draft status pill, and a versions bar with locked prior revisions (superseded/sent immutable) and the active current draft. | Screen/E2E test |
| AC-offer-2 | Given a regenerate banner (v2 → v3), when it renders, then it quotes the captured scope-change line, summarizes the delta, and links the full diff. | Screen/E2E test |
| AC-offer-3 | Given a line-item row, when qty, unit price, or discount % changes, then the line net re-derives as round(qty × unit_minor × (1 − disc%)) in integer minor units and net/VAT/gross totals — and the PDF preview — update in step (OFFER-PARAM-4). | Screen/E2E test |
| AC-offer-4 | Given a staged AI-proposed line, when I Accept, then its AI-proposed flag becomes human provenance, it enters the PDF preview and the recomputed totals; Edit converts it to a typed-by-you line; Dismiss removes it and recomputes. | Screen/E2E test ([[acceptance-standards#GATE-AI-2]]) |
| AC-offer-5 | Given a line whose price is ungrounded, when the screen renders, then it shows a set-price placeholder with a "we won't guess a number" banner and the line is excluded from totals until a price is typed. | Screen/E2E test ([[acceptance-standards#GATE-AI-1]]) |
| AC-offer-6 | Given the totals block, when I click "Explain this total", then a panel shows the per-line net formula and net/tax/gross figures, notes staged & unpriced lines are excluded, and the gross carries a "computed server-side" pill with the ISO-4217 minor-units caption. | Screen/E2E test |
| AC-offer-7 | Given the DE/EN preview toggle, when I switch language, then PDF title, meta, line labels, totals labels, legal text, and locale currency formatting all swap. | Screen/E2E test |
| AC-offer-8 | Given the send card, when I queue the draft to the approval inbox, then the status flips to Sent (locked), copy states sending is 🟡-gated and a sent revision is immutable (a change starts the next revision). | Screen/E2E test |

Note OFFER-AC-N-1: the prototype lacks loading-skeleton, server-error, and
no-permission states — the planned build must add them per the standard
screen-state floor ([[acceptance-standards#Acceptance — standard screen-state matrix]]).
