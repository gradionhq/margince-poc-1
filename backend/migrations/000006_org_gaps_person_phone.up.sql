-- 000006 — org schema gaps + organization_domain + person_phone (WP1 §4.1–§4.2 + §3.3)

ALTER TABLE organization
  ADD COLUMN relevance    smallint NOT NULL DEFAULT 0 CHECK (relevance BETWEEN 0 AND 100),
  ADD COLUMN parent_org_id uuid NULL REFERENCES organization(id) ON DELETE SET NULL,
  ADD COLUMN merged_into_id uuid NULL REFERENCES organization(id) ON DELETE SET NULL,
  ADD COLUMN address jsonb NULL,
  ADD COLUMN social  jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX idx_org_owner    ON organization (workspace_id, owner_id) WHERE archived_at IS NULL;
CREATE INDEX idx_org_class    ON organization (workspace_id, classification) WHERE archived_at IS NULL;
CREATE INDEX idx_org_parent   ON organization (parent_org_id) WHERE parent_org_id IS NOT NULL;
CREATE INDEX idx_org_merged   ON organization (merged_into_id) WHERE merged_into_id IS NOT NULL;

-- Cycle prevention: org cannot be its own ancestor
CREATE OR REPLACE FUNCTION org_no_cycle() RETURNS trigger AS $$
DECLARE ancestor uuid := NEW.parent_org_id;
BEGIN
  WHILE ancestor IS NOT NULL LOOP
    IF ancestor = NEW.id THEN
      RAISE EXCEPTION 'organization cycle detected';
    END IF;
    SELECT parent_org_id INTO ancestor FROM organization WHERE id = ancestor;
  END LOOP;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_org_no_cycle BEFORE INSERT OR UPDATE ON organization
  FOR EACH ROW EXECUTE FUNCTION org_no_cycle();

-- organization_domain (data-model §4.2)
CREATE TABLE organization_domain (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  organization_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
  domain          text NOT NULL CHECK (domain = lower(domain)),
  is_primary      boolean NOT NULL DEFAULT false,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL,
  CONSTRAINT uq_org_domain UNIQUE (workspace_id, domain) DEFERRABLE INITIALLY DEFERRED
);
CREATE UNIQUE INDEX uq_org_domain_primary ON organization_domain (organization_id)
  WHERE is_primary AND archived_at IS NULL;
CREATE INDEX idx_org_domain_org ON organization_domain (organization_id);
ALTER TABLE organization_domain ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_domain FORCE ROW LEVEL SECURITY;
CREATE POLICY organization_domain_tenant_isolation ON organization_domain
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- person_phone (data-model §3.3)
CREATE TABLE person_phone (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  person_id   uuid NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  phone       text NOT NULL,
  phone_type  text NOT NULL DEFAULT 'work' CHECK (phone_type IN ('work','mobile','home','other')),
  is_primary  boolean NOT NULL DEFAULT false,
  position    integer NOT NULL DEFAULT 0,
  source      text NOT NULL,
  captured_by text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz NULL
);
CREATE UNIQUE INDEX uq_person_phone_primary ON person_phone (person_id, phone_type)
  WHERE is_primary AND archived_at IS NULL;
CREATE INDEX idx_person_phone_person ON person_phone (person_id);
ALTER TABLE person_phone ENABLE ROW LEVEL SECURITY;
ALTER TABLE person_phone FORCE ROW LEVEL SECURITY;
CREATE POLICY person_phone_tenant_isolation ON person_phone
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
