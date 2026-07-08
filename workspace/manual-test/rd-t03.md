# RD-T03 Live-Stack UAT — Records-Depth Schema Migration

Verification of migration 000073 (attachment, quota, bulk_operation tables) against RD-DDL-1/2/3 spec.

## Step 1: Verify migration files exist and are correctly numbered

- **Command (auto):**
  ```bash
  ls backend/migrations/ | tail -3
  ```

- **Expected:**
  ```
  000071_offers_and_products.down.sql
  000071_offers_and_products.up.sql
  000073_records_depth_schema.down.sql
  000073_records_depth_schema.up.sql
  ```
  Exactly one migration pair numbered 000073 for records-depth schema; no migration 000072.

## Step 2: Apply migration and verify table structure

- **Command (auto):**
  ```bash
  make migrate-up
  make migrate-status
  psql "$DATABASE_URL" -c "\d attachment"
  psql "$DATABASE_URL" -c "\d quota"
  psql "$DATABASE_URL" -c "\d bulk_operation"
  ```

- **Expected:**
  - `make migrate-status` returns: `73` (non-dirty)
  - **attachment table has:**
    - Columns: `id` (uuid, PK), `workspace_id` (uuid, FK), `entity_type` (text, CHECK), `entity_id` (uuid), `filename` (text), `content_type` (text, NULL), `byte_size` (bigint, NULL), `storage_key` (text), `checksum` (text, NULL), `source` (text), `captured_by` (text), `created_at` (timestamptz), `updated_at` (timestamptz), `archived_at` (timestamptz, NULL)
    - Indexes: `idx_attachment_entity` (composite, WHERE archived_at IS NULL), `idx_attachment_ws` (single-column)
    - Trigger: `trg_attachment_updated` (set_updated_at)
    - RLS: enabled, `attachment_tenant_isolation` policy
    - Grant: margince_app (SELECT, INSERT, UPDATE, DELETE)
  - **quota table has:**
    - Columns: `id` (uuid, PK), `workspace_id` (uuid, FK), `owner_id` (uuid, NULL, FK), `team_id` (uuid, NULL, FK), `period_start` (date), `period_end` (date), `target_minor` (bigint), `currency` (char(3), CHECK), `version` (bigint, default 1), `created_at` (timestamptz), `updated_at` (timestamptz), `archived_at` (timestamptz, NULL)
    - Constraints: `quota_owner_xor_team` CHECK enforcing exactly one of owner/team
    - Indexes: `idx_quota_ws`, `idx_quota_owner_fk` (partial), `idx_quota_team_fk` (partial)
    - Trigger: `trg_quota_touch` (touch_versioned)
    - RLS: enabled, `quota_tenant_isolation` policy
    - Grant: margince_app (SELECT, INSERT, UPDATE, DELETE)
  - **bulk_operation table has:**
    - Columns: `id` (uuid, PK), `workspace_id` (uuid, FK), `kind` (text), `status` (text, default 'pending', CHECK), `total` (int, default 0), `succeeded` (int, default 0), `failed` (int, default 0), `request_payload` (jsonb), `result_summary` (jsonb, NULL), `idempotency_key` (text, NULL), `requested_by` (uuid, FK), `version` (bigint, default 1), `created_at` (timestamptz), `updated_at` (timestamptz), `archived_at` (timestamptz, NULL)
    - Indexes: `idx_bulk_idem` (unique, partial on workspace_id/idempotency_key WHERE idempotency_key IS NOT NULL), `idx_bulk_status` (composite), `idx_bulk_operation_ws`, `idx_bulk_operation_requested_by_fk`
    - Trigger: `trg_bulk_operation_touch` (touch_versioned)
    - RLS: enabled, `bulk_operation_tenant_isolation` policy
    - Grant: margince_app (SELECT, INSERT, UPDATE, DELETE)

## Step 3: Verify attachment entity_type CHECK constraint

- **Command (auto):**
  ```bash
  # Create test data
  psql "$DATABASE_URL" <<'EOSQL'
  INSERT INTO workspace (id, name, slug, base_currency) VALUES 
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Test WS', 'test-ws', 'USD');
  EOSQL
  
  # Try invalid entity_type
  psql "$DATABASE_URL" -c "INSERT INTO attachment (workspace_id, entity_type, entity_id, filename, source, captured_by, storage_key) 
    VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bogus', '00000000-0000-0000-0000-000000000001', 'test.txt', 'test', 'test', 's3://key')" 2>&1
  ```

- **Expected:**
  ```
  ERROR:  new row for relation "attachment" violates check constraint "attachment_entity_type_check"
  ```
  The constraint check must reject invalid entity_type values.

## Step 4: Verify quota owner-XOR-team CHECK constraint

- **Command (auto):**
  ```bash
  psql "$DATABASE_URL" <<'EOSQL'
  INSERT INTO app_user (id, email, workspace_id, display_name) VALUES 
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'test@example.com', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Test User');
  INSERT INTO team (id, name, workspace_id) VALUES 
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', 'Test Team', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa');
  EOSQL
  
  # Try both owner and team set (should fail)
  psql "$DATABASE_URL" -c "INSERT INTO quota (workspace_id, owner_id, team_id, period_start, period_end, target_minor, currency)
    VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'cccccccc-cccc-cccc-cccc-cccccccccccc', '2026-01-01', '2026-12-31', 100000, 'USD')" 2>&1
  
  # Try both NULL (should fail)
  psql "$DATABASE_URL" -c "INSERT INTO quota (workspace_id, owner_id, team_id, period_start, period_end, target_minor, currency)
    VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', NULL, NULL, '2026-01-01', '2026-12-31', 100000, 'USD')" 2>&1
  
  # Try only owner set (should succeed)
  psql "$DATABASE_URL" -c "INSERT INTO quota (workspace_id, owner_id, team_id, period_start, period_end, target_minor, currency)
    VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', NULL, '2026-01-01', '2026-12-31', 100000, 'USD')" 2>&1
  ```

- **Expected:**
  - First insert (both set): `ERROR:  new row for relation "quota" violates check constraint "quota_owner_xor_team"`
  - Second insert (both NULL): `ERROR:  new row for relation "quota" violates check constraint "quota_owner_xor_team"`
  - Third insert (only owner set): `INSERT 0 1` (success)

## Step 5: Verify bulk_operation idempotency partial unique index

- **Command (auto):**
  ```bash
  # Two rows with same non-null idempotency_key (should fail on second)
  psql "$DATABASE_URL" -c "INSERT INTO bulk_operation (workspace_id, kind, requested_by, request_payload, idempotency_key)
    VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'edit', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '{}', 'idem-1')" 2>&1
  
  psql "$DATABASE_URL" -c "INSERT INTO bulk_operation (workspace_id, kind, requested_by, request_payload, idempotency_key)
    VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'edit', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '{}', 'idem-1')" 2>&1
  
  # Two rows with NULL idempotency_key in same workspace (should both succeed)
  psql "$DATABASE_URL" -c "INSERT INTO bulk_operation (workspace_id, kind, requested_by, request_payload, idempotency_key)
    VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'edit', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '{}', NULL)" 2>&1
  
  psql "$DATABASE_URL" -c "INSERT INTO bulk_operation (workspace_id, kind, requested_by, request_payload, idempotency_key)
    VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'edit', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '{}', NULL)" 2>&1
  ```

- **Expected:**
  - First insert with idem-1: `INSERT 0 1` (success)
  - Second insert with same idem-1: `ERROR:  duplicate key value violates unique constraint "idx_bulk_idem"`
  - First insert with NULL: `INSERT 0 1` (success)
  - Second insert with NULL: `INSERT 0 1` (success, because partial index excludes NULLs)

## Step 6: Verify migration round-trip reversibility

- **Command (auto):**
  ```bash
  # Current state
  make migrate-status
  
  # Roll back
  make migrate-down
  make migrate-status
  
  # Roll forward
  make migrate-up
  make migrate-status
  ```

- **Expected:**
  - Before rollback: `73`
  - After rollback: `71`
  - After forward: `73`
  - All migrations show non-dirty state (no `dirty` output)

## Step 7: Verify project-wide gate passes

- **Command (auto):**
  ```bash
  make check
  ```

- **Expected:**
  ```
  OK: make check-backend passed
  ...
  OK: make check passed
  ```
  All backend tests, frontend typecheck, and quality checks pass. No new failures introduced.
