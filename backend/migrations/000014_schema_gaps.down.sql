DROP TRIGGER IF EXISTS trg_deal_stage_pipeline ON deal;
DROP FUNCTION IF EXISTS check_deal_stage_pipeline();
DROP TRIGGER IF EXISTS trg_org_cycle ON organization;
DROP FUNCTION IF EXISTS prevent_org_cycle();
-- DEVIATION: restore uq_activity_source to its original 000003 definition
-- (without the archived_at predicate) so the rollback leaves schema as 000013.
DROP INDEX IF EXISTS uq_activity_source;
CREATE UNIQUE INDEX uq_activity_source ON activity (workspace_id, source_system, source_id)
  WHERE source_system IS NOT NULL AND source_id IS NOT NULL;
ALTER TABLE person_email DROP CONSTRAINT IF EXISTS chk_person_email_lower;
