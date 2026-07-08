# OP-T05 — Manual/Live UAT: offer core CRUD (create/list, get/update draft-only, line-item nested CRUD, server-computed money)

This guide exercises every acceptance criterion introduced by OP-T05. Steps marked `[auto]` are
fully covered by the unit/integration tests in Tasks 1–4 and run without a live stack. Steps
marked `[live]` require `make infra-up` and `make start` (or equivalent). No step is `[manual]`
— this is a pure API ticket with no visual surface.

All `curl` examples assume:

```
BASE=http://localhost:8080
TOKEN=<your-bearer-token>
WS=<workspace_id>
DEAL=<deal_id>       # a live deal in $WS (seed one if needed)
```

Substitute real values before running.

---

## Task 1: Domain types + store ports

### Step 1.1 — Build gate: domain/ports layer compiles [auto]

```bash
go build ./backend/internal/modules/offers/...
```

**Expected:** exits 0 — `domain.Offer`, `domain.OfferLineItem`, `domain.NewOffer`,
`domain.NewOfferLineItem`, `domain.OfferStatusDraft`, `ports.OfferStore`,
`ports.OfferLineItemStore` all compile without error or unused-import warnings.

---

## Task 2: OfferStore + OfferLineItemStore SQL adapters

### Step 2.1 — Offer create→list→get→update round-trip [auto]

```bash
make test-it DIR=backend/internal/modules/offers
```

**Expected:** `TestOfferStore_CreateGetListUpdate_RoundTrip` passes —
- Created offer has `status: "draft"`, `revision: 1`, `net_minor: 0`, `tax_minor: 0`, `gross_minor: 0`.
- `Get` returns the same `offer_number`.
- `List` returns exactly one item for the deal.
- `Update` with `intro_text: "Hello"` returns the updated row with `version` bumped by 1.

### Step 2.2 — offer_number+revision collision → 409 [auto]

**Expected:** `TestOfferStore_Create_DuplicateOfferNumberRevision_Rejected` passes —
second `Create` with the same `offer_number` returns `adapters.ErrDuplicateOfferNumber`.

### Step 2.3 — Unknown deal → 404 [auto]

**Expected:** `TestOfferStore_Create_UnknownDeal_NotFound` passes —
`Create` with a random `deal_id` that doesn't exist returns `errs.ErrNotFound`.

### Step 2.4 — Missing provenance → 422 [auto]

**Expected:** `TestOfferStore_Create_MissingProvenance_Rejected` passes —
`Create` with blank `source`/`captured_by` returns `errs.ErrNullProvenance`.

### Step 2.5 — Update rejected on non-draft offer → 409 [auto]

**Expected:** `TestOfferStore_Update_NonDraft_Rejected` passes —
after force-setting `status='sent'` in the DB, `Update` returns `adapters.ErrOfferNotDraft`.

### Step 2.6 — Version skew → 409 [auto]

**Expected:** `TestOfferStore_Update_VersionSkew_Rejected` passes —
`Update` with `ifMatch = version+99` returns `errs.ErrVersionSkew`.

### Step 2.7 — UUID FK columns (buyer_org_id / template_id) no type mismatch [auto]

**Expected:** `TestOfferStore_Update_UuidFKColumns_NoTypeMismatch` passes —
`Update` setting `buyer_org_id` and `template_id` to real UUIDs succeeds without a
`"types uuid and text cannot be matched"` Postgres error; returned row reflects both FKs.

### Step 2.8 — Product snapshot-on-pick (OFFER-AC-9b) [auto]

**Expected:** `TestOfferLineItemStore_Create_ProductSnapshot` passes —
- Line is created with `product_id` set; `description`, `unit_price_minor`, `tax_rate` are
  copied from the product (not the placeholder values on the request body).
- After `PATCH /products/{id}` changes `unit_price_minor`, a `List` for the same line still
  shows the original snapshotted price (the line is never re-read from the product).

### Step 2.9 — Round-then-sum reconciliation diverges from sum-then-round (OFFER-AC-3, OFFER-PARAM-4) [auto]

**Expected:** `TestOfferLineItemStore_Reconciliation_RoundThenSum_DivergesFromSumThenRound`
passes with the exact discriminating numbers (2 lines, `quantity=1`, `unit_price_minor=201`,
`discount_pct=50`, `tax_rate=50`):

- `round-then-sum` result: `net_minor=202`, `tax_minor=102`, `gross_minor=304`.
- `sum-then-round` (wrong) would yield `net_minor=201`, `tax_minor=101` — a one-unit difference
  in both; a regression to sum-then-round would **fail** this test.

### Step 2.10 — Position collision → typed 409 error [auto]

**Expected:** `TestOfferLineItemStore_Create_PositionConflict_Rejected` passes —
second `Create` at `position=1` returns `*adapters.ErrDuplicatePosition` (typed error, not a
raw constraint error).

### Step 2.11 — Line-item mutations rejected when offer not draft [auto]

**Expected:** `TestOfferLineItemStore_Mutations_RejectedWhenOfferNotDraft` passes —
after force-setting `status='sent'`, both `Update` and `Delete` on a line item return
`adapters.ErrOfferNotDraft`.

### Step 2.12 — Hard delete resets totals to zero [auto]

**Expected:** `TestOfferLineItemStore_Delete_HardDeletes_And_RecomputesTotals` passes —
- The `offer_line_item` row is gone from the DB (count = 0; not soft-archived).
- After deletion, the parent offer's `net_minor`, `tax_minor`, `gross_minor` are all 0.

### Step 2.13 — Missing provenance on line create → 422 [auto]

**Expected:** `TestOfferLineItemStore_Create_MissingProvenance_Rejected` passes —
`Create` with blank `source`/`captured_by` returns `errs.ErrNullProvenance`.

---

## Task 3: OfferHandler HTTP transport

### Step 3.1 — Handler unit tests [auto]

```bash
go test ./backend/internal/modules/offers/...
```

**Expected:** all handler tests pass — no integration build tag needed for this group:
- `TestOfferHandler_CreateDealOffer_Created` — `POST /deals/{id}/offers` returns 201;
  response body has `status: "draft"`, `revision: 1`, `net_minor: 0`.
- `TestOfferHandler_CreateDealOffer_MissingProvenance_422` — omitting `source`/`captured_by`
  returns 422 with `field_errors` for both fields, `code: required`.
- `TestOfferHandler_CreateDealOffer_DuplicateOfferNumber_409` — fake store returning
  `ErrDuplicateOfferNumber` maps to `409 offer_number_duplicate`.
- `TestOfferHandler_ListDealOffers_Empty_OK` — empty store returns 200 `data: []`.
- `TestOfferHandler_GetOffer_NotFound_404` — missing id returns 404.
- `TestOfferHandler_UpdateOffer_NotDraft_409` — fake store returning `ErrOfferNotDraft`
  maps to `409 offer_not_draft`.
- `TestOfferHandler_CreateOfferLineItem_Created` — `POST /offers/{id}/line-items` returns 201.
- `TestOfferHandler_CreateOfferLineItem_PositionConflict_409` — `*ErrDuplicatePosition`
  maps to `409 offer_line_item_position_duplicate`.
- `TestOfferHandler_UpdateOfferLineItem_Updated` — `PATCH .../line-items/{id}` returns 200.
- `TestOfferHandler_DeleteOfferLineItem_NoContent` — `DELETE .../line-items/{id}` returns 204.
- `TestOfferHandler_RoutingDispatch_UnknownSuffix_404` — `POST /offers/{id}/regenerate`
  (out-of-scope verb) returns 404, proving the dispatch tree doesn't swallow it.

### Step 3.2 — Build gate after transport layer [auto]

```bash
go build ./backend/internal/modules/offers/...
```

**Expected:** exits 0.

---

## Task 4: Route wiring, RBAC, OffersAdapter re-wiring

### Step 4.1 — Workspace-wide build gate [auto]

```bash
go build ./backend/...
```

**Expected:** exits 0 — `OffersAdapter{H: ...}` compiles, `NewAllOperations` parameter
signature matches, no unused imports.

### Step 4.2 — Architecture lint [auto]

```bash
make arch-lint
```

**Expected:** "OK - No warnings found" — `OfferStore`/`OfferLineItemStore`/`OfferHandler`
fit the existing module DAG; no new component/dep entries needed.

### Step 4.3 — Generated-types drift check (no crm.yaml change) [auto]

```bash
make gen-types-check
```

**Expected:** exits 0 — `crm.yaml` and `crm_gen.go` are untouched by this ticket; no drift.

### Step 4.4 — Route-mount test (every contract op resolves) [auto]

```bash
go test ./backend/cmd/api/...
```

**Expected:** `TestEveryServedContractOpIsRouted` passes — `"offers": true` in
`servedResources` makes every `/offers/{id}/...` contract operation (including the still-501
`regenerate`/`render`/`send`/`accept` paths) resolve to a mounted pattern.

### Step 4.5 — ServerInterface conformance [auto]

```bash
go test ./backend/internal/contracts/server/...
```

**Expected:** `TestAllOperationsSatisfiesServerInterface` passes (compile-time check) —
`*OffersAdapter` (pointer receiver) satisfies the full `types.ServerInterface`.

### Step 4.6 — Full quality gate [auto]

```bash
make check-q
```

**Expected:** exits 0 — all sub-gates pass: `gen-types-check`, `contract-breaking-check`,
`gen-mcp-tools-check`, `arch-lint`, unit + integration tests.

---

## Live-stack UAT

Start the stack before running these steps:

```bash
make infra-up
make start     # or equivalent; backend listens on $BASE
```

Seed a deal if you don't already have one (the offer endpoint requires a real deal):

```bash
DEAL=$(curl -s -X POST $BASE/deals \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "UAT Deal",
    "pipeline_id": "<pipeline_id>",
    "stage_id": "<stage_id>",
    "source": "api-test",
    "captured_by": "human:uat-runner"
  }' | jq -r '.id')
echo "DEAL=$DEAL"
```

### Step 5: POST /deals/{id}/offers — create a draft offer [live]

```bash
OFFER=$(curl -s -X POST $BASE/deals/$DEAL/offers \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "offer_number": "ANG-2026-UAT-001",
    "currency": "EUR",
    "source": "api-test",
    "captured_by": "human:uat-runner"
  }')
echo $OFFER | jq .
OFFER_ID=$(echo $OFFER | jq -r '.id')
OFFER_VERSION=$(echo $OFFER | jq -r '.version')
echo "OFFER_ID=$OFFER_ID  OFFER_VERSION=$OFFER_VERSION"
```

**Expected:** HTTP 201 Created
- `Location` header set to `/offers/{id}`.
- Response body: `status: "draft"`, `revision: 1`, `net_minor: 0`, `tax_minor: 0`,
  `gross_minor: 0`, `version: 1`.

---

### Step 6: GET /deals/{id}/offers — list under deal [live]

```bash
curl -s -X GET "$BASE/deals/$DEAL/offers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq .
```

**Expected:** HTTP 200 OK
- `data` array contains the offer from Step 5 (`id` matches `$OFFER_ID`).
- `page.has_more: false` (only one offer).
- Offers are ordered most-recent-first (descending `id` keyset).

---

### Step 7: GET /offers/{id} — get by id [live]

```bash
curl -s -X GET $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq .
```

**Expected:** HTTP 200 OK
- Response `offer_number: "ANG-2026-UAT-001"`, `deal_id` equals `$DEAL`, `status: "draft"`.

---

### Step 8: PATCH /offers/{id} — update while draft, with If-Match [live]

```bash
UPDATED=$(curl -s -X PATCH $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -H "If-Match: $OFFER_VERSION" \
  -d '{"intro_text": "Thank you for your interest."}')
echo $UPDATED | jq .
OFFER_VERSION=$(echo $UPDATED | jq -r '.version')
echo "new OFFER_VERSION=$OFFER_VERSION"
```

**Expected:** HTTP 200 OK
- Response `intro_text: "Thank you for your interest."`.
- `version` incremented by 1 compared to Step 5's `version`.

---

### Step 9: PATCH /offers/{id} — stale If-Match → 409 version_skew [live]

```bash
curl -s -X PATCH $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -H "If-Match: 9999" \
  -d '{"intro_text": "Should not apply"}' | jq .
```

**Expected:** HTTP 409 Conflict
- Response `code: version_skew`.
- `intro_text` on the offer unchanged (verify with a follow-up GET).

---

### Step 10: POST /deals/{id}/offers — offer_number collision → 409 [live]

```bash
curl -s -X POST $BASE/deals/$DEAL/offers \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "offer_number": "ANG-2026-UAT-001",
    "currency": "EUR",
    "source": "api-test",
    "captured_by": "human:uat-runner"
  }' | jq .
```

**Expected:** HTTP 409 Conflict
- Response `code: offer_number_duplicate`.
- No new offer created (the `offer_number+revision` pair is already taken).

---

### Step 11: POST /deals/{id}/offers — missing provenance → 422 [live]

```bash
curl -s -X POST $BASE/deals/$DEAL/offers \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "offer_number": "ANG-2026-UAT-002",
    "currency": "EUR"
  }' | jq .
```

**Expected:** HTTP 422 Unprocessable Entity
- Response `code: validation_error`.
- `field_errors` contains entries for `source` (`code: required`) and `captured_by`
  (`code: required`).

---

### Step 12: POST /offers/{id}/line-items — add a line with discount+tax (empty offer → zero totals first) [live]

Verify zero totals on the offer before adding lines:

```bash
curl -s $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq '{net_minor, tax_minor, gross_minor}'
```

**Expected:** `{"net_minor": 0, "tax_minor": 0, "gross_minor": 0}` — empty offer has zero totals.

Now add the first line (quantity=5, unit_price_minor=200000, discount_pct=10, tax_rate=19):

```bash
LINE1=$(curl -s -X POST $BASE/offers/$OFFER_ID/line-items \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "position": 1,
    "description": "Consulting — Platform expansion",
    "quantity": 5,
    "unit_price_minor": 200000,
    "discount_pct": 10,
    "tax_rate": 19,
    "source": "api-test",
    "captured_by": "human:uat-runner"
  }')
echo $LINE1 | jq .
LINE1_ID=$(echo $LINE1 | jq -r '.id')
echo "LINE1_ID=$LINE1_ID"
```

**Expected:** HTTP 201 Created
- `Location` header set to `/offers/{id}/line-items/{lineId}`.
- Response: `position: 1`, `quantity: 5`, `unit_price_minor: 200000`, `discount_pct: 10`,
  `tax_rate: 19`.

Verify offer totals updated:
- `line_net = round(5 × 200000 × (1 − 10/100)) = round(900000.0) = 900000`
- `line_tax = round(900000 × 19/100) = round(171000.0) = 171000`
- `gross = 900000 + 171000 = 1071000`

```bash
curl -s $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq '{net_minor, tax_minor, gross_minor}'
```

**Expected:** `{"net_minor": 900000, "tax_minor": 171000, "gross_minor": 1071000}`.

---

### Step 13: Add a second line — round-then-sum reconciliation (OFFER-PARAM-4, OFFER-AC-3) [live]

Add a second line with different values to verify totals accumulate correctly using round-then-sum:

```bash
LINE2=$(curl -s -X POST $BASE/offers/$OFFER_ID/line-items \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "position": 2,
    "description": "Support retainer",
    "quantity": 1,
    "unit_price_minor": 201,
    "discount_pct": 50,
    "tax_rate": 50,
    "source": "api-test",
    "captured_by": "human:uat-runner"
  }')
echo $LINE2 | jq .
LINE2_ID=$(echo $LINE2 | jq -r '.id')
echo "LINE2_ID=$LINE2_ID"
```

**Expected:** HTTP 201 Created with `position: 2`.

Verify round-then-sum for the second line and the combined offer totals:
- Line 2: `line_net = round(1 × 201 × 0.50) = round(100.5) = 101` (Go rounds 0.5 away from
  zero); `line_tax = round(101 × 0.50) = round(50.5) = 51`.
- Combined: `net = 900000 + 101 = 900101`, `tax = 171000 + 51 = 171051`,
  `gross = 900101 + 171051 = 1071152`.

```bash
curl -s $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq '{net_minor, tax_minor, gross_minor}'
```

**Expected:** `{"net_minor": 900101, "tax_minor": 171051, "gross_minor": 1071152}`.

---

### Step 14: GET /offers/{id}/line-items — list in position order [live]

```bash
curl -s $BASE/offers/$OFFER_ID/line-items \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq .
```

**Expected:** HTTP 200 OK
- `data` array contains two items, ordered `position: 1` then `position: 2`.
- No cursor/pagination params — this endpoint returns the complete flat list.

---

### Step 15: PATCH /offers/{id}/line-items/{lineId} — update a line item [live]

```bash
curl -s -X PATCH $BASE/offers/$OFFER_ID/line-items/$LINE1_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{"quantity": 3}' | jq .
```

**Expected:** HTTP 200 OK
- Response `quantity: 3`.
- Verify offer totals are recomputed:
  - Line 1 (updated): `line_net = round(3 × 200000 × 0.90) = 540000`;
    `line_tax = round(540000 × 0.19) = 102600`.
  - Line 2 (unchanged): `net=101`, `tax=51`.
  - Combined: `net = 540000 + 101 = 540101`, `tax = 102600 + 51 = 102651`,
    `gross = 540101 + 102651 = 642752`.

```bash
curl -s $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq '{net_minor, tax_minor, gross_minor}'
```

**Expected:** `{"net_minor": 540101, "tax_minor": 102651, "gross_minor": 642752}`.

---

### Step 16: position collision → 409 offer_line_item_position_duplicate [live]

```bash
curl -s -X POST $BASE/offers/$OFFER_ID/line-items \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "position": 1,
    "description": "Duplicate position",
    "quantity": 1,
    "unit_price_minor": 100,
    "source": "api-test",
    "captured_by": "human:uat-runner"
  }' | jq .
```

**Expected:** HTTP 409 Conflict
- Response `code: offer_line_item_position_duplicate`.
- `details.existing_id` matches `$LINE1_ID`, `details.field: "position"`.
- No new line item created; offer totals unchanged.

---

### Step 17: Snapshot-on-pick — mutate product price, re-GET line unchanged (OFFER-AC-9b) [live]

First, create a product:

```bash
PRODUCT=$(curl -s -X POST $BASE/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Premium License",
    "unit_price_minor": 500000,
    "currency": "EUR",
    "default_tax_rate": 19.0,
    "source": "api-test",
    "captured_by": "human:uat-runner"
  }')
PRODUCT_ID=$(echo $PRODUCT | jq -r '.id')
PRODUCT_VERSION=$(echo $PRODUCT | jq -r '.version')
echo "PRODUCT_ID=$PRODUCT_ID"
```

**Expected:** HTTP 201 Created — `unit_price_minor: 500000`, `default_tax_rate: 19.0`.

Create a line item referencing this product (with a placeholder `description` and `unit_price_minor`
— the store overwrites them from the product snapshot):

```bash
LINE3=$(curl -s -X POST $BASE/offers/$OFFER_ID/line-items \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d "{
    \"position\": 3,
    \"product_id\": \"$PRODUCT_ID\",
    \"description\": \"placeholder\",
    \"quantity\": 2,
    \"unit_price_minor\": 1,
    \"source\": \"api-test\",
    \"captured_by\": \"human:uat-runner\"
  }")
LINE3_ID=$(echo $LINE3 | jq -r '.id')
echo $LINE3 | jq '{description, unit_price_minor, tax_rate}'
```

**Expected:** HTTP 201 Created — `description: "Premium License"` (copied from product),
`unit_price_minor: 500000` (not `1`), `tax_rate: 19`.

Now mutate the product's price:

```bash
curl -s -X PUT $BASE/products/$PRODUCT_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -H "If-Match: $PRODUCT_VERSION" \
  -d '{"unit_price_minor": 999999, "name": "Premium License"}' | jq '{unit_price_minor}'
```

**Expected:** HTTP 200 OK — `unit_price_minor: 999999`.

Re-GET the line item and verify the snapshot is unchanged:

```bash
curl -s $BASE/offers/$OFFER_ID/line-items \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq --arg id "$LINE3_ID" '.data[] | select(.id==$id) | {unit_price_minor, description}'
```

**Expected:** `unit_price_minor: 500000` (the original snapshot price, not 999999).
`description` still `"Premium License"`. Product price change does not alter the written line.

---

### Step 18: DELETE /offers/{id}/line-items/{lineId} — hard delete, totals recomputed [live]

```bash
curl -s -o /dev/null -w "%{http_code}" -X DELETE \
  $BASE/offers/$OFFER_ID/line-items/$LINE2_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS"
```

**Expected:** HTTP 204 No Content.

Verify the line is gone (not soft-archived) and totals are recomputed without it:

```bash
curl -s $BASE/offers/$OFFER_ID/line-items \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq '[.data[].position]'
```

**Expected:** positions `[1, 3]` only — `position: 2` (`$LINE2_ID`) is gone entirely (hard
delete, not archived).

```bash
curl -s $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" | jq '{net_minor, tax_minor, gross_minor}'
```

**Expected:** totals reflect only line 1 (updated quantity=3) and line 3 (snapshot price=500000,
quantity=2, tax_rate=19, discount_pct=0):
- Line 1: `net=540000`, `tax=102600`.
- Line 3: `net = round(2 × 500000 × 1.0) = 1000000`; `tax = round(1000000 × 0.19) = 190000`.
- Combined: `net=1540000`, `tax=292600`, `gross=1832600`.

---

### Step 19: Draft-only guard — PATCH rejected on non-draft offer [live]

Force the offer to `status='sent'` directly in the database (the transition verbs are out of
scope for this ticket):

```bash
# Run inside psql or your DB console:
# UPDATE offer SET status='sent' WHERE id='<OFFER_ID>';
```

Attempt to update the now-sent offer:

```bash
curl -s -X PATCH $BASE/offers/$OFFER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{"intro_text": "Should be rejected"}' | jq .
```

**Expected:** HTTP 409 Conflict
- Response `code: offer_not_draft`.
- Offer `intro_text` unchanged.

Attempt to add a line item:

```bash
curl -s -X POST $BASE/offers/$OFFER_ID/line-items \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{
    "position": 10,
    "description": "Blocked",
    "quantity": 1,
    "unit_price_minor": 100,
    "source": "api-test",
    "captured_by": "human:uat-runner"
  }' | jq .
```

**Expected:** HTTP 409 Conflict — `code: offer_not_draft`.

Attempt to update an existing line item:

```bash
curl -s -X PATCH $BASE/offers/$OFFER_ID/line-items/$LINE1_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{"quantity": 99}' | jq .
```

**Expected:** HTTP 409 Conflict — `code: offer_not_draft`.

Attempt to delete a line item:

```bash
curl -s -o /dev/null -w "%{http_code}" -X DELETE \
  $BASE/offers/$OFFER_ID/line-items/$LINE1_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS"
```

**Expected:** HTTP 409 — all four line-item mutations are rejected when the parent offer is not
draft.

---

### Step 20: Out-of-scope verbs still return 501 [live]

```bash
curl -s -o /dev/null -w "%{http_code}" -X POST \
  $BASE/offers/$OFFER_ID/regenerate \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS"

curl -s -o /dev/null -w "%{http_code}" -X POST \
  $BASE/offers/$OFFER_ID/send \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS"
```

**Expected:** both return HTTP 501 Not Implemented — `regenerate`/`render`/`send`/`accept` are
a separate ticket and remain untouched stubs.

---

## Summary

All steps above must pass before OP-T05 is considered complete. Key acceptance criteria:

- ✅ `POST /deals/{id}/offers` creates a draft offer; `offer_number+revision` collision → 409.
- ✅ `GET /deals/{id}/offers` lists most-recent-first, paginated, includes archived when asked.
- ✅ `GET /offers/{id}` returns the full offer record.
- ✅ `PATCH /offers/{id}` applies bounded partial updates; `If-Match` enforced; rejected 409 on
  non-draft or stale version.
- ✅ Missing `source`/`captured_by` on any POST → 422 with per-field errors.
- ✅ Empty offer (zero lines) has `net_minor=0`, `tax_minor=0`, `gross_minor=0`.
- ✅ Line-item POST/PATCH/DELETE recompute totals in the same tx using round-then-sum
  (OFFER-PARAM-4).
- ✅ `position` collision within an offer → 409 `offer_line_item_position_duplicate` with
  `existing_id` detail.
- ✅ Snapshot-on-pick: product description/price/tax copied at create time; subsequent product
  price mutation does not alter the line.
- ✅ All four line-item mutations rejected 409 when parent offer is not draft.
- ✅ Line-item DELETE is a hard delete (row gone, not archived).
- ✅ `RegenerateOffer`/`RenderOffer`/`SendOffer`/`AcceptOffer` remain 501 stubs.
- ✅ `make check-q` (incl. `ServerInterface` conformance) green.
