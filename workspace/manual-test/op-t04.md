# OP-T04: Live UAT Guide — /products and /offer-templates

This guide exercises the live stack's `/products` and `/offer-templates` CRUD endpoints.
Each step below is runnable against a running backend API (e.g. via `make infra-up` and `make start`).

Adjust the base URL (e.g. `http://localhost:8080`) and workspace/auth headers as needed for your environment.

## Setup

Ensure you have a valid workspace and auth token. The examples below assume:
- Base URL: `http://localhost:8080`
- Authorization: Bearer token in `Authorization` header (or equivalent auth method)
- Workspace ID: in `X-Workspace-ID` header (or as part of auth context)

---

## Products CRUD

### Step 1: POST /products with fresh SKU

**[live]** Create a product with a unique SKU.

```bash
curl -X POST http://localhost:8080/products \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Consulting Day",
    "sku": "CONSULT-001",
    "unit_price_minor": 150000,
    "currency": "EUR",
    "source": "api-test",
    "captured_by": "human:test-user"
  }'
```

**Expected:** HTTP 201 Created
- `Location` header set to `/products/{id}`
- Response body contains `id`, `unit_price_minor: 150000` (exact int64), `active: true`
- Product is live in workspace.

---

### Step 2: POST /products with duplicate SKU (same workspace)

**[live]** Attempt to create another product with the same `sku` from Step 1.

```bash
curl -X POST http://localhost:8080/products \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Consulting Day (Second)",
    "sku": "CONSULT-001",
    "unit_price_minor": 200000,
    "currency": "EUR",
    "source": "api-test",
    "captured_by": "human:test-user"
  }'
```

**Expected:** HTTP 409 Conflict
- Response `code: product_sku_duplicate`
- Response `details.existing_id` equals the product `id` from Step 1
- Response `details.field: sku`
- No product created.

---

### Step 3: POST /products with null SKU (twice)

**[live]** Create two products without a `sku` field (or with `sku: null`). Both should succeed.

```bash
# First product without SKU
curl -X POST http://localhost:8080/products \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "No SKU Product A",
    "unit_price_minor": 50000,
    "currency": "EUR",
    "source": "api-test",
    "captured_by": "human:test-user"
  }'

# Second product without SKU
curl -X POST http://localhost:8080/products \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "No SKU Product B",
    "unit_price_minor": 75000,
    "currency": "EUR",
    "source": "api-test",
    "captured_by": "human:test-user"
  }'
```

**Expected:** Both requests return HTTP 201 Created
- Both products exist (no SKU collision — the unique index is partial, `WHERE sku IS NOT NULL`).
- Both have `sku: null` in the response.

---

### Step 4: GET /products on empty/fresh workspace

**[live]** List products in a fresh workspace with zero products.

```bash
curl -X GET http://localhost:8080/products \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <fresh_workspace_id>"
```

**Expected:** HTTP 200 OK
- Response `data: []` (empty array, never `null`)
- Response `page.has_more: false`
- Response `page.next_cursor: ""` or absent.

---

### Step 5: PUT /products/{id} with stale If-Match

**[live]** Attempt to update a product (from Step 1) with a stale `If-Match` version.

```bash
curl -X PUT http://localhost:8080/products/{id_from_step_1} \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -H "If-Match: 999" \
  -d '{
    "name": "Updated Name"
  }'
```

**Expected:** HTTP 409 Conflict
- Response `code: version_skew`
- Product remains unchanged (version still matches the original from Step 1).

---

### Step 6: DELETE /products/{id} and verify archived state

**[live]** Archive a product, then verify its behavior in list views.

```bash
# Archive the product from Step 1
curl -X DELETE http://localhost:8080/products/{id_from_step_1} \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>"

# List products (default: exclude archived)
curl -X GET http://localhost:8080/products \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>"

# List products including archived
curl -X GET http://localhost:8080/products?include_archived=true \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>"
```

**Expected:**
- DELETE returns HTTP 200, response body contains `archived_at` set to a non-null timestamp.
- First list (default) excludes the archived product.
- Second list (with `include_archived=true`) includes the archived product.

---

## Offer Templates CRUD

### Step 7: POST /offer-templates with is_default=true (locale de-DE)

**[live]** Create a default offer template for locale `de-DE`.

```bash
curl -X POST http://localhost:8080/offer-templates \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Standard DE",
    "locale": "de-DE",
    "is_default": true,
    "layout": {"logo_ref": "logo_de", "footer_text": "Standardangebot"},
    "source": "api-test",
    "captured_by": "human:test-user"
  }'
```

**Expected:** HTTP 201 Created
- Response includes `id`, `locale: de-DE`, `is_default: true`, `version: 1`.
- Template is the default for `de-DE` in this workspace.

---

### Step 8: POST /offer-templates with is_default=true (same locale de-DE)

**[live]** Attempt to create a second default template for the same `locale: de-DE`.

```bash
curl -X POST http://localhost:8080/offer-templates \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Premium DE",
    "locale": "de-DE",
    "is_default": true,
    "layout": {"logo_ref": "logo_premium"},
    "source": "api-test",
    "captured_by": "human:test-user"
  }'
```

**Expected:** HTTP 409 Conflict
- Response `code: offer_template_default_conflict`
- Response `details.existing_id` equals the template `id` from Step 7
- Response `details.locale: de-DE`
- No second template created.

---

### Step 9: POST /offer-templates with is_default=true (different locale en-US)

**[live]** Create a default template for `locale: en-US` (different from Step 7). This should succeed.

```bash
curl -X POST http://localhost:8080/offer-templates \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Standard EN",
    "locale": "en-US",
    "is_default": true,
    "layout": {"logo_ref": "logo_en", "footer_text": "Standard Offer"},
    "source": "api-test",
    "captured_by": "human:test-user"
  }'
```

**Expected:** HTTP 201 Created
- Response includes `locale: en-US`, `is_default: true`.
- No conflict with the `de-DE` default from Step 7 (per-locale uniqueness, not global).

---

### Step 10: POST /offer-templates with duplicate name

**[live]** Attempt to create a template with the same `name` as an existing one.

```bash
curl -X POST http://localhost:8080/offer-templates \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Standard DE",
    "locale": "fr-FR",
    "is_default": false,
    "layout": {"logo_ref": "logo_fr"},
    "source": "api-test",
    "captured_by": "human:test-user"
  }'
```

**Expected:** HTTP 409 Conflict
- Response `code: offer_template_name_duplicate`
- Response `details.existing_id` equals the template `id` from Step 7 (which has the name "Standard DE")
- No template created.

---

## Provenance (source/captured_by) Validation

### Step 11: POST /products and /offer-templates without source/captured_by

**[live]** Omit required `source` and `captured_by` fields.

```bash
# Product without provenance
curl -X POST http://localhost:8080/products \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "No Provenance Product",
    "unit_price_minor": 50000,
    "currency": "EUR"
  }'

# Offer template without provenance
curl -X POST http://localhost:8080/offer-templates \
  -H "Authorization: Bearer <token>" \
  -H "X-Workspace-ID: <workspace_id>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "No Provenance Template",
    "layout": {}
  }'
```

**Expected:** Both requests return HTTP 422 Unprocessable Entity
- Response `code: validation_error`
- Response contains two `field_errors`: one for `source`, one for `captured_by`, both with `code: required`.
- No product or template created.

---

## Summary

All steps above should pass when run against the live stack. Key acceptance criteria:
- ✅ Product CRUD works; unit_price_minor round-trips as int64.
- ✅ SKU-duplicate collision → 409 with `existing_id` detail.
- ✅ Null SKU never collides (partial index).
- ✅ Empty catalogue lists as 200 with empty `data` array.
- ✅ Offer template default-per-locale constraint enforced; `is_default` conflict → 409.
- ✅ Offer template name uniqueness enforced; duplicate name → 409.
- ✅ Missing `source`/`captured_by` → 422 validation error (both fields required).
- ✅ Archive/include_archived filtering works.
- ✅ If-Match version skew detected → 409.
