-- 000074 — widen audit_log_action_check to admit activity_relink (AT-T01, ACT-WIRE-N-2).
-- crm.yaml's AuditLogEntry.action enum now includes activity_relink (relinkActivity,
-- ACT-WIRE-6); scripts/check-audit-action-coherence.sh (make check) requires this CHECK and
-- the contract enum to carry exactly the same token set. Mirrors 000047/000062's template:
-- union the DDL with the contract, one new token, nothing removed.

ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_action_check;
ALTER TABLE audit_log ADD CONSTRAINT audit_log_action_check
  CHECK (action IN ('create','update','archive','merge','promote','restore',
                    'export','erase','login','assign','advance_stage','capture',
                    'approve','reject','modify','expired','generate','import','publish',
                    'parameterize','pause',
                    'disqualify','anonymize','send_email','consent_grant',
                    'consent_withdraw','record_share','record_unshare',
                    'bulk_set_field','bulk_archive','bulk_reassign','score_override',
                    'activity_relink'));

COMMIT;
