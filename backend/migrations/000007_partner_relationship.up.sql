-- 000007 — partner (1:1 org extension) + relationship typed-edge (WP1 §4.3 + §5)

-- partner (1:1 extension of organization)
CREATE TABLE partner (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  organization_id uuid NOT NULL UNIQUE REFERENCES organization(id) ON DELETE CASCADE,
  cert_status     text NOT NULL DEFAULT 'pending' CHECK (cert_status IN ('pending','active','suspended','expired')),
  partner_role    text NULL CHECK (partner_role IS NULL OR partner_role IN ('hosting','consulting','strategic')),
  margin_tier     text NULL CHECK (margin_tier IS NULL OR margin_tier IN ('standard','silver','gold','platinum')),
  certified_staff integer NOT NULL DEFAULT 0,
  version         bigint NOT NULL DEFAULT 1,
  retention_rate  numeric(5,2) NULL,
  source          text NOT NULL,
  captured_by     text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL
);
CREATE INDEX idx_partner_ws ON partner (workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_partner_tier ON partner (workspace_id, margin_tier) WHERE archived_at IS NULL;
CREATE TRIGGER trg_partner_touch BEFORE UPDATE ON partner
  FOR EACH ROW EXECUTE FUNCTION touch_versioned();
ALTER TABLE partner ENABLE ROW LEVEL SECURITY;
ALTER TABLE partner FORCE ROW LEVEL SECURITY;
CREATE POLICY partner_tenant_isolation ON partner
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- relationship (typed edge: employment, stakeholder, partner kinds)
CREATE TABLE relationship (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  kind            text NOT NULL CHECK (kind IN (
                    'employment','deal_stakeholder',
                    'partner_of','referred_by','co_sell_with')),
  person_id           uuid NULL REFERENCES person(id) ON DELETE CASCADE,
  organization_id     uuid NULL REFERENCES organization(id) ON DELETE CASCADE,
  deal_id             uuid NULL REFERENCES deal(id) ON DELETE CASCADE,
  counterparty_org_id uuid NULL REFERENCES organization(id) ON DELETE SET NULL,
  role            text NULL,
  is_primary      boolean NOT NULL DEFAULT false,
  started_at      date NULL,
  ended_at        date NULL,
  source          text NOT NULL,
  captured_by     text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL,
  CONSTRAINT rel_employment_shape CHECK (
    kind <> 'employment' OR (person_id IS NOT NULL AND organization_id IS NOT NULL AND deal_id IS NULL)),
  CONSTRAINT rel_stakeholder_shape CHECK (
    kind <> 'deal_stakeholder' OR (person_id IS NOT NULL AND deal_id IS NOT NULL AND organization_id IS NULL)),
  CONSTRAINT rel_partner_shape CHECK (
    kind NOT IN ('partner_of','referred_by','co_sell_with') OR
    (counterparty_org_id IS NOT NULL AND person_id IS NULL AND deal_id IS NULL))
);

CREATE UNIQUE INDEX uq_rel_current_primary_employer ON relationship
  (workspace_id, person_id) WHERE kind = 'employment' AND is_primary AND ended_at IS NULL AND archived_at IS NULL;
CREATE UNIQUE INDEX uq_rel_deal_person_role ON relationship
  (deal_id, person_id, role) WHERE kind = 'deal_stakeholder' AND archived_at IS NULL;
CREATE INDEX idx_rel_org_people ON relationship (organization_id, person_id)
  WHERE kind = 'employment' AND archived_at IS NULL;
CREATE INDEX idx_rel_person_orgs ON relationship (person_id, organization_id)
  WHERE kind = 'employment' AND archived_at IS NULL;
CREATE INDEX idx_rel_deal_stakeholders ON relationship (deal_id)
  WHERE kind = 'deal_stakeholder' AND archived_at IS NULL;
CREATE INDEX idx_rel_partner_dir ON relationship (organization_id, counterparty_org_id)
  WHERE kind IN ('partner_of','referred_by','co_sell_with') AND archived_at IS NULL;

ALTER TABLE relationship ENABLE ROW LEVEL SECURITY;
ALTER TABLE relationship FORCE ROW LEVEL SECURITY;
CREATE POLICY relationship_tenant_isolation ON relationship
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
