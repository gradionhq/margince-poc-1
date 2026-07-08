# OP-T01 — Manual/Live UAT: offer data resources into crm.yaml, types regenerated

Contract-only ticket — no handler/business logic ships in this branch (spec Out-of-scope), so
every step is contract/gate inspection rather than a live HTTP round-trip (no
`listOfferTemplates`/`createDealOffer`/etc. Go handler exists yet in this repo — confirmed via
`grep -rl "OfferTemplate\|createDealOffer\|OfferLineItem" backend/internal --include='*.go'`
returning nothing besides the generated `crm_gen.go`/`tools_gen.go`).

1. **[auto]** Run `make gen-types && make gen-mcp-tools` then `git status --short`.
   Expected: exits 0; empty diff (clean regeneration — the branch's own crm.yaml already
   reflects the regenerated output, so a fresh regen matches byte-for-byte).

2. **[auto]** Run `grep -n "description:" backend/api/crm.yaml | grep -A0 -B0 "" | sed -n '1,1p'`
   — more directly: `grep -n -A9 "^    Product:" backend/api/crm.yaml`.
   Expected: `Product` now carries a `description` property (nullable string), alongside its
   existing `name`/`sku`/`unit_price_minor`/`currency`/`default_tax_rate`/`unit`/`active`
   fields (OFFER-WIRE-1 completion).

3. **[auto]** Run `grep -n "operationId: listOfferTemplates\|operationId: getOfferTemplate\|operationId: createOfferTemplate\|operationId: updateOfferTemplate\|operationId: archiveOfferTemplate" backend/api/crm.yaml`.
   Expected: all 5 operationIds present, under `/offer-templates` and `/offer-templates/{id}`.

4. **[auto]** Run `grep -n -A8 "^    OfferTemplate:" backend/api/crm.yaml`.
   Expected: `name`/`locale`/`is_default`/`layout` properties present; `locale` defaults
   `de-DE`.

5. **[auto]** Run `grep -n "operationId: listDealOffers\|operationId: createDealOffer" backend/api/crm.yaml`.
   Expected: both present under `/deals/{id}/offers`.

6. **[auto]** Run `grep -n -A15 "^    Offer:" backend/api/crm.yaml`.
   Expected: `deal_id`/`offer_number`/`revision`/`status`/`currency`/`net_minor`/
   `tax_minor`/`gross_minor` present; `net_minor`/`tax_minor`/`gross_minor` each carry
   `readOnly: true`.

7. **[auto]** Run `grep -n "net_minor\|tax_minor\|gross_minor\|line_net\|line_tax\|line_total" backend/api/crm.yaml | grep -i "CreateOfferRequest\|CreateOfferLineItemRequest\|UpdateOfferRequest\|UpdateOfferLineItemRequest"`.
   Expected: **zero matches** — confirms no money-total field exists on any offer/line-item
   request body (API-ERR-15).

8. **[auto]** Run `grep -n "operationId: getOffer\|operationId: updateOffer" backend/api/crm.yaml`.
   Expected: both present under `/offers/{id}`; `updateOffer`'s description documents the
   draft-only / `offer_not_draft` 409 semantic.

9. **[auto]** Run `grep -n "operationId: listOfferLineItems\|operationId: createOfferLineItem\|operationId: updateOfferLineItem\|operationId: deleteOfferLineItem" backend/api/crm.yaml`.
   Expected: all 4 present under `/offers/{id}/line-items` and
   `/offers/{id}/line-items/{lineId}`; `deleteOfferLineItem` responds `204` (hard delete).

10. **[auto]** Run:
    ```bash
    oasdiff breaking origin/main:backend/api/crm.yaml backend/api/crm.yaml --fail-on ERR
    ```
    (the exact command `scripts/check-contract-breaking.sh` runs).
    Expected: exits 0 — confirms every OFFER-WIRE-2..5 change + the `Product.description`
    addition landed additive-only.

11. **[auto]** Run `make gen-mcp-tools-check`.
    Expected: exits 0 — `backend/internal/shared/ports/mcp/tools_gen.go` matches the 11 new
    `x-mcp-tool` annotations, no drift.

12. **[auto]** Run `make check`.
    Expected: exits 0; `gen-types-check` / `contract-breaking-check` / `gen-mcp-tools-check`
    all pass as part of the full backend gate.

13. **[auto]** Run `make test-contracts`.
    Expected: all TS contract-compliance tests `PASS`, including the new `Product.description`,
    `OfferTemplate`, `OfferTemplateListResponse`, `Offer`, `UpdateOfferRequest`,
    `OfferLineItem`, `OfferLineItemListResponse` describe blocks.
