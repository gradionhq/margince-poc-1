BEGIN;

CREATE TABLE erasure_suppression (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  email_hash   text NOT NULL,
  reason       text NOT NULL DEFAULT 'gdpr_erasure',
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, email_hash)
);
CREATE INDEX idx_erasure_suppression_ws ON erasure_suppression (workspace_id);

ALTER TABLE erasure_suppression ENABLE ROW LEVEL SECURITY;
ALTER TABLE erasure_suppression FORCE ROW LEVEL SECURITY;
CREATE POLICY erasure_suppression_tenant_isolation ON erasure_suppression
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

GRANT SELECT, INSERT ON erasure_suppression TO margince_app;

-- Retention policy: per-workspace, per object type + optional category.
CREATE TABLE retention_policy (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  object_type  text NOT NULL,
  category     text NULL,
  retain_days  integer NOT NULL CHECK (retain_days > 0),
  action       text NOT NULL CHECK (action IN ('archive','anonymize','erase')),
  lawful_basis text NULL,
  enabled      boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- NULL-safe unique index: two rows with NULL category and same (ws, object_type) collide.
CREATE UNIQUE INDEX uq_retention_policy
  ON retention_policy (workspace_id, object_type, COALESCE(category,''));

CREATE INDEX idx_retention_policy_ws ON retention_policy (workspace_id);

ALTER TABLE retention_policy ENABLE ROW LEVEL SECURITY;
ALTER TABLE retention_policy FORCE ROW LEVEL SECURITY;
CREATE POLICY retention_policy_tenant_isolation ON retention_policy
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

CREATE TRIGGER trg_retention_policy_updated
  BEFORE UPDATE ON retention_policy
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON retention_policy TO margince_app;

-- Seed the 5 spec §3.4 defaults for every existing workspace.
INSERT INTO retention_policy (workspace_id, object_type, category, retain_days, action)
SELECT id, 'lead',     'unconverted',       365,  'anonymize' FROM workspace
UNION ALL
SELECT id, 'activity', NULL,               1095,  'archive'   FROM workspace
UNION ALL
SELECT id, 'activity', 'transcript',        365,  'erase'     FROM workspace
UNION ALL
SELECT id, 'person',   'no_consent_no_deal', 730, 'anonymize' FROM workspace
UNION ALL
SELECT id, 'deal',     'lost',             1825,  'archive'   FROM workspace
ON CONFLICT DO NOTHING;

-- Legal-hold flag on the four mutable core objects.
ALTER TABLE person       ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;
ALTER TABLE organization ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;
ALTER TABLE deal         ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;
ALTER TABLE lead         ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;

COMMIT;
