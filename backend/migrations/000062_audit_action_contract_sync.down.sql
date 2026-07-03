-- Revert 000062 — restore audit_log_action_check to the 000047 set (drop the 4
-- bulk-operation/score-override actions). NOTE: any audit_log rows written with one of
-- the dropped actions must be removed/migrated first or the re-added CHECK will fail.

ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_action_check;
ALTER TABLE audit_log ADD CONSTRAINT audit_log_action_check
  CHECK (action IN ('create','update','archive','merge','promote','restore',
                    'export','erase','login','assign','advance_stage','capture',
                    'approve','reject','modify','expired','generate','import','publish',
                    'parameterize','pause',
                    'disqualify','anonymize','send_email','consent_grant',
                    'consent_withdraw','record_share','record_unshare'));

COMMIT;
