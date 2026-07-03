DROP TABLE IF EXISTS person_phone, organization_domain CASCADE;
DROP TRIGGER IF EXISTS trg_org_no_cycle ON organization;
DROP FUNCTION IF EXISTS org_no_cycle();
DROP INDEX IF EXISTS idx_org_parent;
DROP INDEX IF EXISTS idx_org_merged;
DROP INDEX IF EXISTS idx_org_class;
DROP INDEX IF EXISTS idx_org_owner;
ALTER TABLE organization
  DROP COLUMN IF EXISTS relevance,
  DROP COLUMN IF EXISTS parent_org_id,
  DROP COLUMN IF EXISTS merged_into_id,
  DROP COLUMN IF EXISTS address,
  DROP COLUMN IF EXISTS social;
