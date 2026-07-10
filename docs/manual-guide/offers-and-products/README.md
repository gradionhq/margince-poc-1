# Offers & Products chapter — manual guide (OP-T01..T09)

Checkpoint for GitHub epic **#105 — OP (Offers & Products)**. One ordered walkthrough for a human
tester verifying everything the chapter shipped:

- **OP-T04** — rate-card (product) CRUD + offer-template admin, price-snapshot source-of-record.
- **OP-T05** — offer core: create/list under a deal, draft-only update, nested line-item CRUD, and
  **server-computed** net/tax/gross (OFFER-PARAM-4, OFFER-AC-3).
- **OP-T06** — offer render + **confirm-first (🟡) send**: branded DE/EN PDF, FX freeze +
  buyer/issuer snapshots, revision-on-resend.
- **OP-T07** — AI offer authoring: regenerate-from-signal into a new draft revision, evidence-or-omit,
  never a fabricated price.
- **OP-T08** — accept semantics: flip `accepted`, sync `deal.amount_minor` from gross, paired
  `deal.updated` on one correlation (OFFER-AC-1/2, EVT-SEM-4).
- **OP-T09** — the **offer builder screen** (`/deals/:id/offers/:offerId`): versions bar,
  live-totals line editor, staged AI lines, unpriced placeholder, explain-total, DE/EN preview, send
  card, the AC-offer-1..8 state floor.
- **OP-T01/T02/T03** — the contract (offer resources + lifecycle verbs) and the migration
  (`product` / `offer` / `offer_line_item` / `offer_template` tables) underneath, exercised by
  everything below.

> **Two things to know before you start**
> 1. **Mixed API + one screen, and there is no seed data.** Products, templates, and offers ship
>    **zero** seeded rows — you build the fixtures yourself with `curl` in Parts 1–2, then review
>    them on the builder screen in Part 3. Every write is a real, wired endpoint (not a `501`).
> 2. **A human never needs an approval token.** The 🟡 send/accept gate applies to *AI agents*. When
>    **you** (the dev `X-User-ID`) call send/accept — or click **Send** on the screen — your own
>    action *is* the approval and it goes through. The 403 "approval required" path only appears for
>    an agent caller with no token (shown as an optional negative check).

---

## Setup (do this once)

- [ ] **1. Boot the stack:**
  ```bash
  make infra-up && make migrate-up && make seed-reset && make run
  ```
  **Expected:** infra up, migrations applied (through the `product`/`offer`/`offer_line_item`/
  `offer_template` tables), dev seed loaded, API on `:8080`.
- [ ] **2. Start the frontend** in a second terminal (for Part 3):
  ```bash
  make fe-dev
  ```
  **Expected:** Vite on `http://localhost:5173`; `/api` proxies to `:8080` and injects the dev
  workspace/user headers.
- [ ] **3. Log in as `admin@example.com`** / `changeme`.

**Constants** (from `backend/seed/dev.sql`) used throughout:

| Name | Value |
|---|---|
| Workspace ID | `00000000-0000-0000-0000-000000000001` |
| Admin user ID | `00000000-0000-0000-0010-000000000001` |
| Seeded deal "Acme Expansion" (EUR, amount 5 000.00) | `00000000-0000-0000-0042-000000000001` |
| API base | `http://localhost:8080` |

Export the dev headers once (zsh/bash) — abbreviated **`$HDRS`** below:
```bash
HDRS=(-H 'Content-Type: application/json' \
  -H 'X-Workspace-ID: 00000000-0000-0000-0000-000000000001' \
  -H 'X-User-ID: 00000000-0000-0000-0010-000000000001')
DEAL=00000000-0000-0000-0042-000000000001
```

---

## Part 1 — Rate-card (products) + offer templates (OP-T04)

- [ ] **1.1 Create a product** (the price source-of-record):
  ```bash
  curl -s -X POST http://localhost:8080/products "${HDRS[@]}" -d '{
    "name":"Consulting Day","sku":"CONS-DAY","unit_price_minor":150000,
    "currency":"EUR","default_tax_rate":19.00,"unit":"day",
    "source":"ui","captured_by":"human:00000000-0000-0000-0010-000000000001"}' | jq '{id,name,unit_price_minor,currency}'
  ```
  **Expected:** `201` with the product; `unit_price_minor` is stored as **integer minor units**
  (150000 = €1 500.00). Copy the `id` as `<PROD_ID>`.
- [ ] **1.2 Duplicate-SKU guard.** Re-run the exact same `curl`.
  **Expected:** **`409`** — a SKU is unique per workspace.
- [ ] **1.3 List products:**
  ```bash
  curl -s http://localhost:8080/products "${HDRS[@]}" | jq '.data[].name'
  ```
  **Expected:** your product listed. (`?include_archived=true` also returns archived rows.)
- [ ] **1.4 Create an offer template** and prove the default-per-locale guard:
  ```bash
  curl -s -X POST http://localhost:8080/offer-templates "${HDRS[@]}" -d '{
    "name":"Standard DE","locale":"de-DE","is_default":true,"layout":{"header":"Angebot"},
    "source":"ui","captured_by":"human:00000000-0000-0000-0010-000000000001"}' | jq '{id,name,locale,is_default}'
  ```
  **Expected:** `201`. Creating a **second** `is_default:true` template for the same locale → **`409`**
  (one default per locale); a duplicate name → **`409`**.

---

## Part 2 — Offer core: create under a deal, line items, server-computed totals (OP-T05)

- [ ] **2.1 Create a draft offer under the seeded deal:**
  ```bash
  OFFER=$(curl -s -X POST http://localhost:8080/deals/$DEAL/offers "${HDRS[@]}" -d '{
    "offer_number":"ANG-2026-0001","currency":"EUR",
    "source":"ui","captured_by":"human:00000000-0000-0000-0010-000000000001"}' | jq -r .id)
  echo "OFFER=$OFFER"
  curl -s http://localhost:8080/offers/$OFFER "${HDRS[@]}" | jq '{revision,status,net_minor,tax_minor,gross_minor}'
  ```
  **Expected:** `201`; the offer starts at **revision 1, status `draft`**, with
  `net_minor/tax_minor/gross_minor` all **0** (no lines yet).
- [ ] **2.2 Add a line item — totals are computed server-side** (OFFER-PARAM-4):
  ```bash
  curl -s -X POST http://localhost:8080/offers/$OFFER/line-items "${HDRS[@]}" -d '{
    "position":1,"description":"Consulting — expansion","quantity":5,
    "unit_price_minor":150000,"discount_pct":0,"tax_rate":19,
    "source":"ui","captured_by":"human:00000000-0000-0000-0010-000000000001"}' | jq '{description,quantity,unit_price_minor,price_grounded}'
  curl -s http://localhost:8080/offers/$OFFER "${HDRS[@]}" | jq '{net_minor,tax_minor,gross_minor}'
  ```
  **Expected:** the offer now reports `net_minor: 750000` (5 × 150000), `tax_minor: 142500`
  (750000 × 19%), `gross_minor: 892500` — **the server derived every figure**; the line and offer
  totals were never sent by the client.
- [ ] **2.3 The client cannot move the totals** (OFFER-AC-3 / API-ERR-15). Try to PATCH the draft
  offer with bogus totals, then re-read:
  ```bash
  curl -s -X PATCH http://localhost:8080/offers/$OFFER "${HDRS[@]}" -d '{"gross_minor":999999999,"net_minor":999999999}' >/dev/null
  curl -s http://localhost:8080/offers/$OFFER "${HDRS[@]}" | jq '{net_minor,gross_minor}'
  ```
  **Expected:** the totals are **unchanged** (`net_minor: 750000`, `gross_minor: 892500`) — the
  client-supplied figures had **no effect**; the server value always wins (`net_minor`/`gross_minor`
  are `readOnly`, re-derived from line items).

> **⚠️ Known code gap found during testing (report this).** The contract specifies a *stronger*
> guarantee than the server currently enforces: per `crm.yaml` (createOfferLineItem, API-ERR-15,
> lines ~1316–1330), sending a `line_total` on **`POST /offers/{id}/line-items`** must be **rejected
> with `422 {field: line_total, code: field_not_writable}`**. The running build instead returns
> **`201` and silently ignores** the field (creating the line with its server-computed total). The
> value still has no effect, so the money guarantee holds *in substance* — but the specified 422
> refusal is **not implemented**, and no runtime test covers it (the contract test only asserts the
> generated type has no `line_total` field). Worth a backend ticket.
- [ ] **2.4 (optional) Product snapshot.** Add a line with `"product_id":"<PROD_ID>"` and no
  description/price. **Expected:** the line copies the product's price + description as a **snapshot** —
  later editing the product does not retro-change this offer.
- [ ] **2.4 (optional) Product snapshot.** Add a line with `"product_id":"<PROD_ID>"` and no
  description/price. **Expected:** the line copies the product's price + description as a **snapshot** —
  later editing the product does not retro-change this offer.

---

## Part 3 — The offer builder screen (OP-T09, AC-offer-1..8)

- [ ] **3.1 Reach the builder from the Deal 360.** Open `http://localhost:5173/deals/<DEAL>` (use the
  Acme deal id). In the offers section, click your `ANG-2026-0001` offer (or **New offer** to create a
  fresh draft and land straight on the builder).
  **Expected:** you land on `/deals/<DEAL>/offers/<OFFER>` — an offer builder page with the offer
  number + revision + a status pill in the header, and a **Back to deal** link.
- [ ] **3.2 Versions bar** (AC-offer-1). **Expected:** a bar listing every revision in this
  `offer_number` chain; non-draft revisions carry a **lock** icon.
- [ ] **3.3 Live-totals line editor** (AC-offer-3). Edit a line's quantity or discount.
  **Expected:** the totals **re-derive live** as you type; the footer/explain figures update.
- [ ] **3.4 Explain-total panel** (AC-offer-6). Open it. **Expected:** it breaks down net/tax/gross
  with a **"computed server-side"** caption, and **excludes** staged AI lines and unpriced
  placeholders from the total.
- [ ] **3.5 Unpriced placeholder** (AC-offer-5). **Expected:** a line with no grounded price shows as
  an unpriced placeholder and is left out of the totals — never guessed at.
- [ ] **3.6 DE/EN preview** (AC-offer-7). Toggle the preview locale. **Expected:** the rendered offer
  preview switches between German and English.
- [ ] **3.7 Send card** (AC-offer-8). **Expected:** a send card; clicking **Send offer** opens a
  confirm dialog whose copy explains *your click is the approval for this human-operated builder*.
  (Don't send yet if you want to send via curl in Part 4 — or send here and confirm the card flips to
  **Sent / locked** and starts the next revision.)
- [ ] **3.8 State floor.** Hard-reload (skeleton on first paint); as `readonly@example.com` the
  mutating affordances are gated (permission card / read-only footer); a backend outage shows a
  contained **error card with retry**, not a blank page.

---

## Part 4 — Render + confirm-first send (OP-T06)

- [ ] **4.1 Render the offer to a PDF:**
  ```bash
  curl -s -X POST http://localhost:8080/offers/$OFFER/render "${HDRS[@]}" | jq '{status,pdf_asset_ref}'
  ```
  **Expected:** `200`; `pdf_asset_ref` is now populated (the branded DE/EN PDF reference).
- [ ] **4.2 Send it — as a human, no token needed** (🟡 gate self-approved):
  ```bash
  curl -s -X POST http://localhost:8080/offers/$OFFER/send "${HDRS[@]}" | jq '{status,fx_rate_to_base,fx_rate_date}'
  ```
  **Expected:** `200`; status flips to **`sent`**, and the FX rate + snapshots **freeze** on the
  offer. (This EUR offer on an EUR workspace needs no FX row; a cross-currency offer with no stored
  rate would hard-fail **`422 fx_rate_unavailable`**.)
- [ ] **4.3 A sent offer is immutable** (OFFER-AC-1). Try to patch it:
  ```bash
  curl -s -o /dev/null -w '%{http_code}\n' -X PATCH http://localhost:8080/offers/$OFFER "${HDRS[@]}" -d '{"intro_text":"late edit"}'
  ```
  **Expected:** **`409`** (`offer_not_draft`) — a sent offer is frozen; changes only via a new
  revision (Part 6).
- [ ] **4.4 (optional) The 🟡 gate for agents.** Repeat the send call as an *agent* principal with no
  approval token (drop `X-User-ID`, use an agent `captured_by`), or read the test
  `TestOfferHandler_Send_AgentNoToken_403_ThenValidToken_200`.
  **Expected:** **`403 approval_required`** without a token; `200` once a valid `X-Approval-Token` is
  supplied — outbound never leaves without a recorded human approval.

---

## Part 5 — Accept semantics: offer → deal amount (OP-T08, OFFER-AC-1/2)

- [ ] **5.1 Note the deal's amount before accept:**
  ```bash
  curl -s http://localhost:8080/deals/$DEAL "${HDRS[@]}" | jq '{amount_minor}'
  ```
  **Expected:** the seeded `500000` (€5 000.00).
- [ ] **5.2 Accept the sent offer:**
  ```bash
  curl -s -X POST http://localhost:8080/offers/$OFFER/accept "${HDRS[@]}" | jq '{status}'
  curl -s http://localhost:8080/deals/$DEAL "${HDRS[@]}" | jq '{amount_minor}'
  ```
  **Expected:** the offer flips to **`accepted`**, and the deal's **`amount_minor` now equals the
  offer's `gross_minor` (892500)** — the accept synced the deal amount from the offer gross, emitting
  `offer.accepted` + a paired `deal.updated` on one correlation id.
- [ ] **5.3 Accept requires a sent offer** (OFFER-AC negative). Accepting a *draft* offer → **`409`**
  (`offer_not_acceptable`).

---

## Part 6 — AI authoring / regenerate (OP-T07)

> **Dev caveat:** the running dev server wires the offer handler with a **no-op retriever**, so
> `regenerate` exercises only the *revision + supersede* mechanic — it stages **zero** grounded AI
> lines and, because nothing was AI-generated, the response comes back with **`ai_generated: false`,
> `ai_disclosure: null`, and `diff_from_previous: null`**. The AI flags, disclosure text, and diff
> only populate when a real retriever generates content — those are proven by the integration tests
> in the Automated counterpart, not by the dev stack. Also note
> there is **no draft-from-context endpoint** — a fresh offer starts empty; `regenerate` (sent-only)
> is the sole AI path, so the screen's regenerate banner only appears on a **sent** offer.
>
> **Regenerate needs a `sent` offer — not an accepted one.** The `$OFFER` from Parts 1–5 is now
> `accepted`, so regenerating *it* returns `409 offer_not_acceptable`. Build a fresh offer, send it,
> and **don't accept it**, for this Part.

- [ ] **6.1 Set up a sent-but-unaccepted offer:**
  ```bash
  OFFER2=$(curl -s -X POST http://localhost:8080/deals/$DEAL/offers "${HDRS[@]}" -d '{"offer_number":"ANG-2026-0002","currency":"EUR","source":"ui","captured_by":"human:00000000-0000-0000-0010-000000000001"}' | jq -r .id)
  curl -s -X POST http://localhost:8080/offers/$OFFER2/line-items "${HDRS[@]}" -d '{"position":1,"description":"Consulting","quantity":2,"unit_price_minor":150000,"tax_rate":19,"source":"ui","captured_by":"human:00000000-0000-0000-0010-000000000001"}' >/dev/null
  curl -s -X POST http://localhost:8080/offers/$OFFER2/render "${HDRS[@]}" >/dev/null
  curl -s -X POST http://localhost:8080/offers/$OFFER2/send "${HDRS[@]}" | jq '{status}'
  ```
  **Expected:** the last line shows `status: "sent"`.
- [ ] **6.2 Regenerate it into a new draft revision:**
  ```bash
  curl -s -X POST http://localhost:8080/offers/$OFFER2/regenerate "${HDRS[@]}" | jq '{id,revision,status,ai_generated,ai_disclosure,diff_from_previous}'
  ```
  **Expected:** a **new revision (`revision: 2`), status `draft`**, and the **prior revision now
  `superseded`** (never edited in place — this is the observable dev mechanic). Per the dev caveat
  above, `ai_generated` is **`false`** and `ai_disclosure` / `diff_from_previous` are **`null`** (the
  no-op retriever produced no AI content); with a real retriever these carry the disclosure + diff.
  The response `id` is the new draft revision — use it in 6.3.
- [ ] **6.3 Regenerate refuses a draft** (revision discipline). Regenerating that **new draft**
  revision → **`409`** (`offer_not_sent`) — you regenerate from a sent revision, not an open draft.

---

## Automated counterpart

Run these to cover the same ground at the API/DB/component level (they are the merge gate):

| Command | What it proves |
|---|---|
| `make check` | Format, lint, contract-drift (OFFER-WIRE-1..9 + types), Go + FE unit tests (incl. `offerMath`, every builder component `.test.tsx`) |
| `make test-it DIR=backend/internal/modules/offers/adapters` | OP-T05/T08 store lane: create/get/list/update round-trip, **totals recompute + round-then-sum reconciliation** (OFFER-PARAM-4), product snapshot, evidence round-trip, **accept→deal.amount sync** |
| `make test-it DIR=backend/internal/modules/offers/transport` | OP-T06/T07 handler lane: render sets a real `pdf_asset_ref`; send human-no-token / agent-403-then-token / emits-once / FX-422 / patch-after-send-409; accept sent/not-sent; regenerate new-revision-supersedes-prior / draft-409 / decodes-signals |
| `make fe-test` | The offer builder screen + components (versions bar, line editor, explain-total, staged lines, regenerate banner, send card) and the Deal-360 "New offer" entry |

If a step doesn't match: the owning spec is `docs/subsystems/offers-and-products.md` (OFFER-AC-*,
OFFER-WIRE-1..9, OFFER-PARAM-1..6, AC-offer-1..8), and `docs/quality/acceptance-standards.md` is the
state floor.

> **Known gaps flagged during testing:**
> 1. **No seed fixtures** for products/offers/templates — you build them in Parts 1–2 (this guide
>    does so). A small dev-seed offer chain would make the screen walkthrough one-click.
> 2. **AI authoring is a no-op in dev** (Part 6 caveat) — grounded-line generation is only
>    observable in the integration tests, not the running app.
> 3. The subsystem doc's front-matter still says `module: backend/internal/modules/people`; the code
>    actually lives in `backend/internal/modules/offers` — cosmetic, noted if you cite the module.
