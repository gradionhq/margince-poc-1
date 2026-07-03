# ADR-0037 — Offers / Angebote: a bounded line-item + AI-authored quote engine (not CPQ)

**Status:** Accepted (2026-06-23, founder). Recorded as **DECISIONS A48**.
Closes the last CRITICAL parity gap from the 2026-06-23 deep red-team (**RT-PR-C1**); promotes the
`BACKLOG §C` "lightweight quote/proposal builder" from Phase-2 to **V1**.

## Context

The deep spec red-team (`reviews/feedback/2026-06-23-spec-redteam-deep.md`, RT-PR-C1) flagged that the
deal carried a **single `amount`** and the **Angebot** (the German B2B offer/quote) could only be
**attached as a document** (`/attachments`). For the locked beachhead — the **regulated B2B
Mittelstand** (A43/ADR-0033) — the Angebot is *the* central sales artifact: a deal does not progress
without one, and the line items on it are the truth of what is being sold and for how much. Leaving it
as an external Word/Excel document has three concrete costs:

1. **Auto-capture breaks** — the offer (the highest-signal sales document) lives outside the system, so
   the context graph, Morning Brief, and Deal 360 never see it.
2. **Forecast is wrong by construction** — a single `amount` can't reflect a multi-line, multi-currency,
   discounted offer; the flagship "the forecast is true" promise (E09) degrades exactly where it matters.
3. **It reads as "missing" on every buyer checklist** — the GTM flag the matrix already carried.

The original deferral was correct *for full CPQ* (taxonomy item 8: configure-price-quote depth — product
configurators, pricing-rule engines, discount-approval workflow graphs — is out of scope and would
re-introduce exactly the runtime rules engine ADR-0002/P1 reject). But "no CPQ" was over-applied to
also exclude the **bounded, everyday line-item offer** every Mittelstand rep needs. The `BACKLOG §C`
intent (Rainer R16 / Niraj N22) already named the right shape: *a lightweight line-item builder from a
rate-card, with the AI-native twist of **regenerate-from-signal***.

## Decision

Ship a **bounded Offer (Angebot) engine** in **V1**, deal-centric, source-of-truth in the relational
core, AI-authored, templated to a branded PDF, and shared/accepted through the **digital Deal Room**
(WP16, ADR-/`features/08 §5b`). It is explicitly **not** CPQ.

**1 — A bounded, typed data model (three first-class objects, `data-model §12.6`).**
- **`product`** — an optional reusable **rate-card / catalogue** entry (name, SKU, unit price + currency,
  default tax rate, unit). A workspace can sell entirely free-form (no catalogue) or maintain a rate-card
  reps pick from. *Products are data, not a configurator* — there is **no** product bundle logic, no
  option/feature configurator, no pricing-rule engine.
- **`offer`** — a **versioned** quote bound to one `deal` (`offer.version`, immutable once `sent`; a
  change creates the next version). Holds buyer/issuer snapshot, currency, validity date, status
  (`draft → sent → accepted | rejected | expired | superseded`), computed money totals, and a rendered
  PDF ref.
- **`offer_line_item`** — a typed line (`qty × unit_price`, `discount_pct`, `tax_rate`, computed
  `line_net/line_tax/line_total`), optionally referencing a `product` (a snapshot is copied onto the line
  so a later rate-card change never silently re-prices a sent offer).

**2 — Money is computed deterministically, server-side, fx-honest.** Totals (net / tax / gross, per the
P11 integer-minor-unit + ISO-4217 rule) are **derived in code from the line items**, never free-typed;
multi-currency offers convert to the workspace base currency via the same daily/frozen `fx_rate`
machinery as deals (`data-semantics §1.2`, RT-PR-C2). The deal's headline `amount_minor` is **synced from
the accepted offer's gross** (the offer becomes the deal's value source once accepted), restoring forecast
accuracy. A line-total that doesn't reconcile to its inputs is a test failure, not a stored value.

**3 — AI authoring is the differentiator, governed like every other action.**
- **Draft-from-context** — the agent assembles a first-draft offer from the deal's captured context
  (transcript line items discussed, prior emails, the org's prior offers, the rate-card), under the
  **evidence-or-omit** rule (`features/07`): every proposed line cites the source text it came from;
  unsupported lines are omitted, never invented. Prices come from the rate-card or are left blank for the
  rep — **the AI never fabricates a price**.
- **Regenerate-from-signal** — re-derive the offer from the *latest* transcript/email when scope changes
  ("they added a second site") → a new **draft version**, diffed against the prior, for the rep to accept.
- Creating/editing a draft offer is **🟢** (reversible internal write). **Sending** an offer to the buyer
  (publishing to the Deal Room / emailing the PDF) is **🟡** — it leaves the workspace, so it stops at the
  approval inbox (ADR-0026/A34, ADR-0036 token binding). The AI drafts; a human sends.

**4 — Templated, branded PDF output.** An **`offer_template`** (workspace-level, governed) defines the
branded layout (logo, header/footer, terms boilerplate, locale — DE/EN); rendering produces a PDF stored
as an attachment ref. Templates reuse the **governed asset store** (A42, `drafting_asset` boilerplate for
terms/intro blocks) — not a free CMS. Template *selection + parameters* are runtime; a structurally new
template layout is a source-level theme (the ADR-0002 boundary, same as email templates).

**5 — Acceptance via the Deal Room, no e-sign dependency in V1.** A sent offer is shared in the deal's
**Deal Room** (`deal_room`, `features/08 §5b`); buyer **view/open/accept** are tracked as
`deal_room_engagement_event`s (reusing the only V1 "view" signal, RT-PR-H2). **Accept** flips
`offer.status = accepted`, syncs the deal amount, and emits `offer.accepted`. Legally-binding
**e-signature is fast-follow** (a connector + sub-processor decision); V1 acceptance is a tracked
in-room action + the audit trail.

**The bright line (records the CPQ boundary):**
- *A bounded line-item offer from a rate-card, AI-drafted, templated, accepted in the room* → **V1, this
  ADR.** No code, no rules engine.
- *Product configurators, pricing-rule/discount-approval engines, quote-approval workflow graphs, full
  CPQ* → **OUT** (taxonomy item 8); bespoke pricing logic is **source-level (ADR-0002)**.

## Consequences

- **Positive:** closes the last CRITICAL parity gap and the beachhead's most load-bearing missing artifact;
  restores forecast accuracy (the offer, not a guessed `amount`, drives deal value); brings the
  highest-signal sales document inside auto-capture; and turns "build a quote" into the AI-native
  *regenerate-from-signal* moment no incumbent ships. Governance, money, fx, audit, Deal Room, and the
  asset store are **reused, not rebuilt** — the offer is just another governed entity on the one surface
  (ADR-0013), with its actions tiered (ADR-0026) and its sends token-bound (ADR-0036).
- **Negative / honest limits:** (a) the line-item model is deliberately flat — no bundles, no
  product options, no pricing rules (those stay source / OUT); a workspace that needs true CPQ is a
  consulting/source engagement (P14), flagged not hidden; (b) real V1 build cost — three tables + totals
  engine + PDF renderer + the AI draft/regenerate task + the Deal Room accept flow; (c) **V1 line grows
  74 → 75 stories (43 Must + 32 WOW)** — re-check lean-team capacity (R-E4) against the larger V1; (d)
  e-sign and a preference/multi-currency-rate-card UI are explicit fast-follows.
- **Relationship to other decisions:** does **not** weaken ADR-0002/P1 (no runtime engine — products are
  data, totals are code, bespoke pricing is source); composes with ADR-0026 (🟢 draft / 🟡 send),
  ADR-0036 (send token), ADR-0007 (draft-from-context rides the graph), the fx machinery (RT-PR-C2),
  A42 (asset store), and the Deal Room (WP16). Corrects the `features/01 §3.1` "products/line-items OUT
  v1" line and the parity matrix D3.6/D3.7 disposition.
- **Scope:** specified in `features/01 §10` (offers & products) + the AI authoring moment in `features/07
  §5e`; data model in `data-model §12.6`; contract surface in `crm.yaml` (net-new resources block); user
  story **S-E03.7**, build stories **B-E03.16–B-E03.23** (`build-backlog/E03.md`).
