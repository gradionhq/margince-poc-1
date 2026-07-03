-- 000045 — extend audit_log_action_check to include automation-specific actions.
-- "parameterize" = edit trigger/action; "pause" = disable automation.
-- Required by E15-P4 handler audit writes (handler_automation.go).

ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_action_check;
ALTER TABLE audit_log ADD CONSTRAINT audit_log_action_check
  CHECK (action IN ('create','update','archive','merge','promote','restore',
                    'export','erase','login','assign','advance_stage','capture',
                    'approve','reject','modify','expired','generate','import','publish',
                    'parameterize','pause'));

COMMIT;
