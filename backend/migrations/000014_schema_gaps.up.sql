-- 000014 — EP02 schema gap fixes
-- person_email: lowercased CHECK
ALTER TABLE person_email ADD CONSTRAINT chk_person_email_lower
  CHECK (email = lower(email));

-- person phone E.164 CHECK (person_phone table if exists, else skip)
-- uq_activity_source: idempotent capture index.
-- DEVIATION: 000003 already created uq_activity_source WITHOUT an `archived_at IS NULL`
-- predicate. `CREATE UNIQUE INDEX IF NOT EXISTS` would silently no-op on the existing
-- name and never apply the archived-aware predicate, so we DROP and recreate it.
DROP INDEX IF EXISTS uq_activity_source;
CREATE UNIQUE INDEX IF NOT EXISTS uq_activity_source
  ON activity (workspace_id, source_system, source_id)
  WHERE source_system IS NOT NULL AND source_id IS NOT NULL AND archived_at IS NULL;

-- org cycle-prevention trigger
CREATE OR REPLACE FUNCTION prevent_org_cycle() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  cur uuid := NEW.parent_org_id;
BEGIN
  WHILE cur IS NOT NULL LOOP
    IF cur = NEW.id THEN
      RAISE EXCEPTION 'org cycle detected: % -> %', NEW.id, NEW.parent_org_id;
    END IF;
    SELECT parent_org_id INTO cur FROM organization WHERE id = cur;
  END LOOP;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_org_cycle BEFORE INSERT OR UPDATE ON organization
  FOR EACH ROW WHEN (NEW.parent_org_id IS NOT NULL)
  EXECUTE FUNCTION prevent_org_cycle();

-- deal composite FK: ensure stage belongs to the deal's pipeline
CREATE OR REPLACE FUNCTION check_deal_stage_pipeline() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM stage WHERE id = NEW.stage_id AND pipeline_id = NEW.pipeline_id
  ) THEN
    RAISE EXCEPTION 'stage % does not belong to pipeline %', NEW.stage_id, NEW.pipeline_id;
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER trg_deal_stage_pipeline BEFORE INSERT OR UPDATE ON deal
  FOR EACH ROW EXECUTE FUNCTION check_deal_stage_pipeline();
