BEGIN;
-- 000076 — RD-T05/RD-PARAM-5: attachment.scan_status was contract-only (crm.yaml's Attachment
-- schema already models it) until now. Lands as an on-row column, matching the contract's own
-- "either is acceptable" note and keeping the RLS/archive story on one row (no joined side-table).
-- Starts 'scanning' by default: absent an explicit verdict a row NEVER auto-transitions to
-- 'clean' — see backend/internal/modules/records for the injected Scanner seam that sets it.
ALTER TABLE attachment ADD COLUMN scan_status text NOT NULL DEFAULT 'scanning'
  CHECK (scan_status IN ('scanning', 'clean', 'blocked'));
CREATE INDEX idx_attachment_scan_status ON attachment (workspace_id, scan_status)
  WHERE archived_at IS NULL;
COMMIT;
