# CF-T02 — Manual/Live UAT: `custom_field` catalog table (CUSTOM-FIELDS-SCHEMA-1)

Migration-only ticket — no handler/business logic ships in this branch (spec Out-of-scope), so
every step exercises the schema directly against a live Postgres rather than an HTTP round-trip.
All steps are `[auto]` or `[live]` (need a real database).

1. **[auto]** Run `ls backend/migrations/000071_custom_field_catalog.*.sql`.
   Expected: both `.up.sql` and `.down.sql` present (substitute the actual migration number if
   `000071` was already taken at build time — see the plan's Task 1 Step 1).

2. **[live]** Run `make infra-up` then `make migrate-up`.
   Expected: exits 0 — the migration applies cleanly against a clean database.

3. **[live]** Run `make psql` and inside psql run `\d custom_field`.
   Expected: all base columns present — `id uuid`, `workspace_id uuid NOT NULL`,
   `created_at timestamptz NOT NULL`, `updated_at timestamptz NOT NULL`,
   `archived_at timestamptz` (nullable) — plus `object`, `slug`, `label`, `type`, `status`,
   `column_name`, `currency`, `options`, `created_by`, `version bigint NOT NULL`. Both
   `idx_custom_field_slug` and `idx_custom_field_col` listed as unique indexes.
   `Triggers: trg_custom_field_touch` listed. "Force row security is enabled" in the output.

4. **[live]** Inside psql, run `SELECT policyname FROM pg_policies WHERE tablename = 'custom_field';`.
   Expected: exactly one row, `custom_field_tenant_isolation`.

5. **[live]** Inside psql, run:
   ```sql
   SELECT conname, contype FROM pg_constraint WHERE conrelid = 'custom_field'::regclass ORDER BY conname;
   ```
   Expected: CHECK constraints on `object`, `type`, `status`, `currency` present (four `c`-type
   constraints beyond the primary key and the `workspace_id` foreign key); no CHECK referencing
   both `currency` and `type` together (the spec's explicit "no cross-column CHECK" decision).

6. **[live]** Inside psql, run:
   ```sql
   \dt *_custom
   \dt custom_field_options
   \dt field_metadata
   ```
   Expected: no relations found for any of the three — confirms CUSTOM-FIELDS-AC-9 (no sidecar
   per-object `<object>_custom` table, no options sidecar table, no `field_metadata`/EAV table).

7. **[live]** Run `make migrate-down`, then inside `make psql` run `\d custom_field`.
   Expected: `Did not find any relation named "custom_field"` — the down migration removed the
   table (and its policy/trigger/indexes) cleanly.

8. **[live]** Run `make migrate-up` again.
   Expected: exits 0 — clean re-apply, proving the down migration left no residue (no orphaned
   policy, trigger, or index blocking re-creation).

9. **[auto]** Run `go build ./backend/internal/shared/ports/migrate/...`.
   Expected: exits 0.

10. **[live]** Run `make test-integration` (0 skips).
    Expected: passes, including
    `backend/internal/shared/ports/migrate/migrate_integration_test.go`'s
    `TestRunWithAggregatesEnabledSet`, which applies the whole `backend/migrations` directory
    (including this migration) end to end.

11. **[auto]** Run `make check-q`.
    Expected: exits 0.
