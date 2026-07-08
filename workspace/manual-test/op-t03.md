# OP-T03 — Offers & Products Migration Live-UAT Guide

## Step 1: Verify migration files exist

**Command:**
```bash
ls backend/migrations/*.up.sql | sort | tail -5
```

**Expected:**
The last five migration files should end with:
```
backend/migrations/000068_dedupe_fuzzy_trgm.up.sql
backend/migrations/000069_record_grant.up.sql
backend/migrations/000070_ws_c_conformance.up.sql
backend/migrations/000071_offers_and_products.up.sql
```

Confirm exactly one new `000071_offers_and_products.up.sql` is present, with corresponding `.down.sql`.

---

## Step 2: Apply migration and verify schema

**Command:**
```bash
make migrate-up
```

**Expected:** Exits 0, migration 71 applied successfully.

**Then verify each table structure:**

```bash
psql "$DATABASE_URL" -c '\d product'
```

**Expected:**
- Columns: `id`, `workspace_id`, `name`, `sku` (nullable), `description` (nullable), `unit`, `unit_price_minor`, `currency`, `default_tax_rate`, `active`, `version`, `source`, `captured_by`, `created_at`, `updated_at`, `archived_at` (nullable)
- Primary key: `id`
- Unique indexes: `uq_product_sku` (workspace_id, sku) WHERE sku IS NOT NULL AND archived_at IS NULL
- Index: `idx_product_active` (workspace_id, active) WHERE archived_at IS NULL
- Check: currency ~ '^[A-Z]{3}$'
- Foreign key: workspace_id REFERENCES workspace(id) ON DELETE RESTRICT
- RLS enabled and enforced with `product_tenant_isolation` policy
- Trigger: `trg_product_touch` (BEFORE UPDATE, EXECUTE FUNCTION touch_versioned())

```bash
psql "$DATABASE_URL" -c '\d offer'
```

**Expected:**
- Columns: `id`, `workspace_id`, `deal_id`, `offer_number`, `revision`, `status`, `currency`, `buyer_org_id`, `buyer_snapshot`, `issuer_snapshot`, `valid_until`, `intro_text`, `terms_text`, `net_minor`, `tax_minor`, `gross_minor`, `fx_rate_to_base`, `fx_rate_date`, `template_id`, `pdf_asset_ref`, `accepted_at` (nullable), `version`, `source`, `captured_by`, `created_at`, `updated_at`, `archived_at` (nullable)
- Primary key: `id`
- Unique constraint: `offer_number_rev_unique` (workspace_id, offer_number, revision)
- Indexes: `idx_offer_deal` (workspace_id, deal_id, revision DESC) WHERE archived_at IS NULL; `idx_offer_status` (workspace_id, status) WHERE archived_at IS NULL
- Check constraints: `status IN ('draft','sent','accepted','rejected','expired','superseded')`; `currency ~ '^[A-Z]{3}$'`; `offer_accepted_at` (status <> 'accepted' OR accepted_at IS NOT NULL)
- Foreign keys: workspace_id, deal_id, buyer_org_id, template_id (from offer_template)
- RLS enabled and enforced with `offer_tenant_isolation` policy
- Trigger: `trg_offer_touch` (BEFORE UPDATE, EXECUTE FUNCTION touch_versioned())

```bash
psql "$DATABASE_URL" -c '\d offer_line_item'
```

**Expected:**
- Columns: `id`, `workspace_id`, `offer_id`, `position`, `product_id` (nullable), `description`, `unit`, `quantity`, `unit_price_minor`, `discount_pct`, `tax_rate`, `evidence` (nullable), `created_at`, `updated_at`, `archived_at` (nullable)
- Primary key: `id`
- Unique constraint: `offer_line_item_position_unique` (offer_id, position)
- Index: `idx_oli_offer` (offer_id, position)
- Check constraints: `quantity > 0`; `discount_pct BETWEEN 0 AND 100`
- Foreign keys: workspace_id, offer_id (ON DELETE CASCADE), product_id (ON DELETE SET NULL)
- RLS enabled and enforced with `offer_line_item_tenant_isolation` policy
- Trigger: `trg_offer_line_item_updated` (BEFORE UPDATE, EXECUTE FUNCTION set_updated_at())

```bash
psql "$DATABASE_URL" -c '\d offer_template'
```

**Expected:**
- Columns: `id`, `workspace_id`, `name`, `locale`, `is_default`, `layout`, `version`, `created_at`, `updated_at`, `archived_at` (nullable)
- Primary key: `id`
- Unique constraint: `offer_template_name_unique` (workspace_id, name)
- Unique index: `uq_offer_template_default` (workspace_id, locale) WHERE is_default AND archived_at IS NULL
- Foreign key: workspace_id REFERENCES workspace(id) ON DELETE RESTRICT
- RLS enabled and enforced with `offer_template_tenant_isolation` policy
- Trigger: `trg_offer_template_touch` (BEFORE UPDATE, EXECUTE FUNCTION touch_versioned())

---

## Step 3: Test product SKU uniqueness constraint

**Command:**
```bash
cat > /tmp/test_product_sku.sql << 'SQL'
-- Get a workspace_id for testing
SELECT id FROM workspace LIMIT 1 \gset ws_

-- Insert first product with SKU
INSERT INTO product (workspace_id, name, sku, unit_price_minor, currency, source, captured_by)
VALUES (:'ws_id', 'Test Product A', 'TEST-SKU-001', 10000, 'EUR', 'test', 'test');

-- Try to insert second product with same SKU - should fail
INSERT INTO product (workspace_id, name, sku, unit_price_minor, currency, source, captured_by)
VALUES (:'ws_id', 'Test Product B', 'TEST-SKU-001', 20000, 'EUR', 'test', 'test');
SQL

PGPASSWORD=margince psql -h localhost -U margince -d margince -f /tmp/test_product_sku.sql
```

**Expected:**
- First INSERT succeeds
- Second INSERT fails with: `ERROR:  duplicate key value violates unique constraint "uq_product_sku"`

**Then verify the constraint allows duplicates when one is archived:**

```bash
cat > /tmp/test_product_sku_archive.sql << 'SQL'
-- Get the workspace_id
SELECT id FROM workspace LIMIT 1 \gset ws_

-- Archive the first product
UPDATE product SET archived_at = now() 
WHERE workspace_id = :'ws_id' AND sku = 'TEST-SKU-001' AND archived_at IS NULL 
LIMIT 1;

-- Now insert product with same SKU - should succeed
INSERT INTO product (workspace_id, name, sku, unit_price_minor, currency, source, captured_by)
VALUES (:'ws_id', 'Test Product C', 'TEST-SKU-001', 30000, 'EUR', 'test', 'test');
SQL

PGPASSWORD=margince psql -h localhost -U margince -d margince -f /tmp/test_product_sku_archive.sql
```

**Expected:**
- UPDATE succeeds (1 row updated)
- INSERT succeeds (after archiving, the SKU becomes available again)

---

## Step 4: Test offer_template is_default uniqueness constraint

**Command:**
```bash
cat > /tmp/test_template_default.sql << 'SQL'
-- Get a workspace_id for testing
SELECT id FROM workspace LIMIT 1 \gset ws_

-- Insert first template with is_default = true for de-DE
INSERT INTO offer_template (workspace_id, name, is_default, layout)
VALUES (:'ws_id', 'Test Template A', true, '{"test": true}'::jsonb);

-- Try to insert second template with is_default = true for same locale - should fail
INSERT INTO offer_template (workspace_id, name, is_default, locale, layout)
VALUES (:'ws_id', 'Test Template B', true, 'de-DE', '{"test": true}'::jsonb);
SQL

PGPASSWORD=margince psql -h localhost -U margince -d margince -f /tmp/test_template_default.sql
```

**Expected:**
- First INSERT succeeds
- Second INSERT fails with: `ERROR:  duplicate key value violates unique constraint "uq_offer_template_default"`

**Then verify different locale succeeds:**

```bash
cat > /tmp/test_template_different_locale.sql << 'SQL'
-- Get a workspace_id for testing
SELECT id FROM workspace LIMIT 1 \gset ws_

-- Insert template with is_default = true for en-US (different locale) - should succeed
INSERT INTO offer_template (workspace_id, name, locale, is_default, layout)
VALUES (:'ws_id', 'Test Template C', 'en-US', true, '{"test": true}'::jsonb);
SQL

PGPASSWORD=margince psql -h localhost -U margince -d margince -f /tmp/test_template_different_locale.sql
```

**Expected:**
- INSERT succeeds (different locale allowed)

---

## Step 5: Test offer accepted_at CHECK constraint

**Command:**
```bash
cat > /tmp/test_offer_constraint.sql << 'SQL'
-- Get a workspace_id and deal_id for testing
SELECT id FROM workspace LIMIT 1 \gset ws_
SELECT id FROM deal LIMIT 1 \gset deal_

-- Try to insert offer with status = 'accepted' but accepted_at IS NULL - should fail
INSERT INTO offer (workspace_id, deal_id, offer_number, status, currency, source, captured_by, accepted_at)
VALUES (:'ws_id', :'deal_id', 'TEST-OFF-001', 'accepted', 'EUR', 'test', 'test', NULL);
SQL

PGPASSWORD=margince psql -h localhost -U margince -d margince -f /tmp/test_offer_constraint.sql
```

**Expected:**
- INSERT fails with: `ERROR:  new row for relation "offer" violates check constraint "offer_accepted_at"`

---

## Step 6: Test migration round-trip reversibility

**Command:**
```bash
make migrate-down
```

**Expected:**
- Exits 0
- Migration 71 down successfully applied
- Tables dropped

**Then re-apply:**

```bash
make migrate-up
```

**Expected:**
- Exits 0
- Migration 71 up applied
- Tables recreated

**Verify clean state:**

```bash
make migrate-status
```

**Expected:**
- Output: `71`
- Non-dirty status (the migration-tool version file shows 71 with no pending dirty flag)

---

## Step 7: Run project-wide gate

**Command:**
```bash
make check
```

**Expected:**
- All checks pass
- No failures in backend tests, frontend tests, linting, type checking
- Output ends with: `OK: make check passed`

---

**Summary:** All seven UAT steps completed successfully. The migration is correctly applied, all constraints are working as expected, reversibility is clean, and the project gate is green.
