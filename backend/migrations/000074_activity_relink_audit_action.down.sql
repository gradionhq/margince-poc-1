-- Revert 000074 — restore audit_log_action_check to the 000062 set (drop activity_relink).
-- NOTE: any audit_log rows written with action='activity_relink' must be removed/migrated
-- first or the re-added CHECK will fail.

ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_action_check;
ALTER TABLE audit_log ADD CONSTRAINT audit_log_action_check
  CHECK (action IN ('create','update','archive','merge','promote','restore',
                    'export','erase','login','assign','advance_stage','capture',
                    'approve','reject','modify','expired','generate','import','publish',
                    'parameterize','pause',
                    'disqualify','anonymize','send_email','consent_grant',
                    'consent_withdraw','record_share','record_unshare',
                    'bulk_set_field','bulk_archive','bulk_reassign','score_override'));

COMMIT;
