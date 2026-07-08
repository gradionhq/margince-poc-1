BEGIN;
-- 000075 down — reverse of the up migration, in reverse dependency order (the view reads the
-- column, so drop the view first).
DROP VIEW IF EXISTS organization_open_pipeline_rollup;
ALTER TABLE deal DROP COLUMN IF EXISTS amount_minor_base;
COMMIT;
