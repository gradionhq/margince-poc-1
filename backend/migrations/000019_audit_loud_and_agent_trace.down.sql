-- Reverse 000019. Order: C → B → A (exact reverse of up).
-- Part C inverse.
BEGIN;
DROP TABLE IF EXISTS agent_trace;
COMMIT;

-- Part B inverse.
BEGIN;
DROP INDEX IF EXISTS uq_outbox_audit_appended;
COMMIT;

-- Part A inverse: restore the silent no-op rules (000015 behavior).
BEGIN;
CREATE OR REPLACE RULE audit_log_no_update AS ON UPDATE TO audit_log DO INSTEAD NOTHING;
CREATE OR REPLACE RULE audit_log_no_delete AS ON DELETE TO audit_log DO INSTEAD NOTHING;
COMMIT;
