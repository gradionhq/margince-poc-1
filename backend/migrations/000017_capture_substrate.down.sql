BEGIN;
-- restore the original audit_log CHECKs
ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_actor_type_check;
ALTER TABLE audit_log ADD  CONSTRAINT audit_log_actor_type_check
  CHECK (actor_type IN ('human','agent','system'));
ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_action_check;
ALTER TABLE audit_log ADD  CONSTRAINT audit_log_action_check
  CHECK (action IN ('create','update','archive','merge','promote','restore',
                    'export','erase','login','assign','advance_stage'));
DROP INDEX IF EXISTS uq_deal_source;
DROP INDEX IF EXISTS uq_organization_source;
DROP INDEX IF EXISTS uq_person_source;
ALTER TABLE deal         DROP COLUMN IF EXISTS source_id;
ALTER TABLE deal         DROP COLUMN IF EXISTS source_system;
ALTER TABLE organization DROP COLUMN IF EXISTS source_id;
ALTER TABLE organization DROP COLUMN IF EXISTS source_system;
ALTER TABLE person       DROP COLUMN IF EXISTS source_id;
ALTER TABLE person       DROP COLUMN IF EXISTS source_system;
COMMIT;
