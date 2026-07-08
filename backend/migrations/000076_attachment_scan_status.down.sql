BEGIN;
DROP INDEX IF EXISTS idx_attachment_scan_status;
ALTER TABLE attachment DROP COLUMN IF EXISTS scan_status;
COMMIT;
