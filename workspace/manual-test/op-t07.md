# OP-T07 Manual UAT Guide

Run this only after the stack is up with `make infra-up && make migrate-up && make seed-reset && make run`.
The commands below assume:

- the API is listening on `http://localhost:8080`
- the database is reachable through `DATABASE_URL`
- you can act as a tenant by sending `X-Workspace-ID` + `X-User-ID`
- `jq` is available for extracting ids from JSON responses

Use one shell session so the variables exported below stay available for later steps.

## Bootstrap

Set these from `make seed-reset`'s output before running the guide for real:

```bash
export API_BASE='http://localhost:8080'
export WS_ID='<seeded workspace id>'
export USER_ID='<seeded user id>'
export DEAL_ID='<seeded deal id>'
```

If your seed output does not print a deal id, pick any live deal in the workspace and set
`DEAL_ID` to that id before you start.

## Step 1 [live]: Seed a draft offer with zero line items

```bash
export OFFER_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"offer_number":"OP-T07-UAT-001","currency":"EUR","source":"manual-test","captured_by":"human:uat"}' \
  "${API_BASE}/deals/${DEAL_ID}/offers" | jq -r '.id')"

curl -sS \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | jq '{id, status, revision, net_minor, tax_minor, gross_minor}'

curl -sS \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}/line-items" | jq '.data | length'
```

Expected: `POST /deals/{id}/offers` returns `201 Created` with `status: "draft"` and
`revision: 1`. The follow-up `GET /offers/{id}` shows `net_minor: 0`, `tax_minor: 0`,
`gross_minor: 0`. `GET /offers/{id}/line-items` returns `0` rows.

## Step 2 [live]: Regenerate from an empty context

```bash
export NEW_OFFER_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}/regenerate" | jq -r '.id')"

curl -sS \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${NEW_OFFER_ID}" | jq '{id, status, revision, ai_generated, ai_disclosure, diff_from_previous, net_minor, tax_minor, gross_minor}'

curl -sS \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${NEW_OFFER_ID}/line-items" | jq '.data | length'
```

Expected: `POST /offers/{id}/regenerate` returns `200 OK`, a new offer id, `status: "draft"`,
`revision: 2`, `ai_generated: true`, a non-empty `ai_disclosure`, `diff_from_previous` equal to
`{"added":[],"removed":[],"changed":[]}`, and all money totals equal to `0`. The new offer has
`0` line items.

## Step 3 [auto]: Grounded vs ungrounded signals stay evidence-only

Covered by `TestOfferStore_Regenerate_GroundedAndUngroundedSignals`
(`backend/internal/modules/offers/adapters/store_offer_regenerate_test.go`).

```bash
go test ./backend/internal/modules/offers/adapters -run TestOfferStore_Regenerate_GroundedAndUngroundedSignals -v
```

Expected: exits `0`. A context with 2 groundable line signals and 1 ungroundable one persists
exactly 2 evidenced lines, and the ungroundable signal is omitted.

## Step 4 [live]: The prior revision is superseded

```bash
curl -sS \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | jq '{id, status, revision}'
```

Expected: `200 OK`, `status: "superseded"` on the original offer, with `revision: 1`.

## Step 5 [live]: Regenerate the new draft again

```bash
export THIRD_OFFER_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${NEW_OFFER_ID}/regenerate" | jq -r '.id')"

curl -sS \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${NEW_OFFER_ID}" | jq '{id, status, revision}'

curl -sS \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${THIRD_OFFER_ID}" | jq '{id, status, revision}'
```

Expected: `POST /offers/{id}/regenerate` returns `200 OK`, a third offer id, and `revision: 3`
on the new row. The second offer (`${NEW_OFFER_ID}`) is now `status: "superseded"`.

## Step 6 [live]: Regenerate an already-superseded offer is refused

```bash
curl -sS -i -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}/regenerate"
```

Expected: `409 Conflict` with problem code `offer_not_draft`.

## Step 7 [live]: PATCH on a superseded offer still fails with offer_not_draft

```bash
curl -sS -i -X PATCH \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"intro_text":"should be rejected"}' \
  "${API_BASE}/offers/${OFFER_ID}"
```

Expected: `409 Conflict` with problem code `offer_not_draft`.

## Step 8 [auto]: Rate-card fallback grounds an unpriced signal

Covered by `TestOfferStore_Regenerate_RateCardFallback_WhenSignalHasNoPrice`
(`backend/internal/modules/offers/adapters/store_offer_regenerate_test.go`).

```bash
go test ./backend/internal/modules/offers/adapters -run TestOfferStore_Regenerate_RateCardFallback_WhenSignalHasNoPrice -v
```

Expected: exits `0`. A signal that names a real `product_id` but carries no conversation price
persists the product's rate-card price, never a fabricated one.

## Step 9 [auto]: Totals stay on the shared server-computed path

Covered by the Task 2 and Task 6 offer-store tests. The AI lane uses `computeLineTotals` and
`recomputeOfferTotals` only; there is no second totals algorithm in the diff.

```bash
go test ./backend/internal/modules/offers/... -run TestOfferStore_Regenerate -v
```

Expected: exits `0`. The AI lane exercises the shared totals path only, and the regenerated
offer's money totals stay server-computed from the written line items.
