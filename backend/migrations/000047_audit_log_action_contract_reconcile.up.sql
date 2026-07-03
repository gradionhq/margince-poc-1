-- 000047 — reconcile audit_log_action_check with the contract (AuditLogEntry.action).
-- The CHECK grew migration-by-migration as features needed new actions, but 7 actions
-- the contract defines (and code already emits) were never added — notably 'send_email'
-- (crm-core/handler_activity.go writes it), 'anonymize' (GDPR erasure/retention),
-- 'disqualify' (lead), consent_grant/consent_withdraw, and record_share/record_unshare.
-- Without these, those audit writes fail the CHECK at runtime (a latent integrity bug).
-- This unions the DDL with the contract so every action the system writes is accepted;
-- a new coherence gate (scripts/check-audit-action-coherence.sh, in `make check`) keeps
-- the two from drifting again.

ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_action_check;
ALTER TABLE audit_log ADD CONSTRAINT audit_log_action_check
  CHECK (action IN ('create','update','archive','merge','promote','restore',
                    'export','erase','login','assign','advance_stage','capture',
                    'approve','reject','modify','expired','generate','import','publish',
                    'parameterize','pause',
                    'disqualify','anonymize','send_email','consent_grant',
                    'consent_withdraw','record_share','record_unshare'));

COMMIT;
