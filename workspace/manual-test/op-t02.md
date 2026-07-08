# OP-T02 — Manual/Live UAT: offer lifecycle verbs (regenerate/render/send/accept), types regenerated

Contract-only ticket — no handler/business logic ships in this branch (spec Out-of-scope), so every
step is contract/gate inspection rather than a live HTTP round-trip. `regenerateOffer`/
`renderOffer`/`sendOffer`/`acceptOffer` have no real handler yet — `OffersAdapter`'s four new
methods are pure 501-Not-Implemented conformance stubs (mirroring its own pre-existing
`OFFER-WIRE-3..5` stubs and `ProductsAdapter`/`OfferTemplatesAdapter`'s precedent), existing only so
the generated `types.ServerInterface` compiles and `TestAllOperationsSatisfiesServerInterface`
passes.

1. **[auto]** Run `make gen-types && make gen-mcp-tools` then `git status --short`.
   Expected: exits 0; empty diff (clean regeneration — the branch's own `crm.yaml` already
   reflects the regenerated output, so a fresh regen matches byte-for-byte).

2. **[auto]** Run `grep -n "operationId: regenerateOffer\|operationId: renderOffer\|operationId: sendOffer\|operationId: acceptOffer" backend/api/crm.yaml`.
   Expected: all 4 operationIds present, under `/offers/{id}/regenerate`, `/offers/{id}/render`,
   `/offers/{id}/send`, `/offers/{id}/accept` respectively.

3. **[auto]** Run `grep -n -A3 "verb: draft_offer\|verb: render_offer\|verb: send_offer\|verb: accept_offer" backend/api/crm.yaml`.
   Expected: `draft_offer`/`render_offer` are `tier: green`; `send_offer`/`accept_offer` are
   `tier: yellow`.

4. **[auto]** Run `grep -n -B2 "operationId: sendOffer" backend/api/crm.yaml | grep -A6 "operationId: sendOffer"` and separately inspect the `sendOffer` and `acceptOffer` path blocks directly (`grep -n -A25 "operationId: sendOffer"` / `grep -n -A30 "operationId: acceptOffer"`).
   Expected: both include `$ref: '#/components/parameters/ApprovalToken'` among their `parameters`,
   and both declare a `403` response describing the missing/invalid approval-token case (mirroring
   `sendEmail`'s shape).

5. **[auto]** Run `grep -n -A5 "operationId: acceptOffer" backend/api/crm.yaml | grep -A40 "operationId: acceptOffer"`.
   Expected: a `409` response with `code: offer_not_acceptable` is present, describing the
   already-accepted/wrong-status case.

6. **[auto]** Run `grep -n -A20 "^    Offer:" backend/api/crm.yaml`.
   Expected: `fx_rate_to_base`/`fx_rate_date`/`buyer_snapshot`/`issuer_snapshot` are all present,
   each `type: [..., 'null']` and `readOnly: true`; `status`'s description no longer says "a later
   ticket"; `pdf_asset_ref`'s description no longer says "later ticket".

7. **[auto]** Run:
   ```bash
   oasdiff breaking origin/main:backend/api/crm.yaml backend/api/crm.yaml --fail-on ERR
   ```
   (the exact command `scripts/check-contract-breaking.sh` runs).
   Expected: exits 0 — confirms every OFFER-WIRE-6..9 addition + the four new `Offer` fields landed
   additive-only.

8. **[auto]** Run `make gen-mcp-tools-check`.
   Expected: exits 0 — `backend/internal/shared/ports/mcp/tools_gen.go` matches the 4 new
   `x-mcp-tool` annotations, no drift.

9. **[auto]** Run `cd backend && go build ./... && go test ./internal/contracts/... && cd ..`.
   Expected: build exits 0; `TestAllOperationsSatisfiesServerInterface` PASSES — `OffersAdapter`'s
   four new stub methods satisfy the regenerated `ServerInterface`.

10. **[auto]** Run `make check-q`.
    Expected: exits 0 — the full backend+frontend gate, including `gen-types-check`/
    `contract-breaking-check`/`gen-mcp-tools-check`/`arch-lint`, all pass with the new surface in
    place.

11. **[auto]** Run `make test-contracts`.
    Expected: all TS contract-compliance tests PASS, including the new
    "Offer FX/snapshot fields contract compliance (OFFER-WIRE-8 completion)" describe block.
