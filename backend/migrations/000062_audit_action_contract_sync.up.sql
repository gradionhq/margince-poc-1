-- 000062 — skeleton-original migration: sync audit_log_action_check to the shipped
-- contract (crm.yaml AuditLogEntry.action). The bulk-operation/lead-scoring feature
-- migrations that originally introduced these 4 actions (poc 000048, 000059) were not
-- harvested (out-of-boundary, zero kept-code consumers); this adds only the CHECK values.

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
