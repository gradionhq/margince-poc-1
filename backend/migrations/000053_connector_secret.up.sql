-- 000053 — connector_secret: per-tenant OAuth token vault for incumbent CRM
-- connectors (ADR-0048). incumbent_connection tracks the logical connection
-- (one per workspace+connector); connector_secret holds the encrypted token
-- material, one row per rotation (append + latest-by-rotated_at — a superseded
-- row stays queryable for audit, it is not overwritten in place).

CREATE TABLE incumbent_connection (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  connector     text NOT NULL CHECK (connector IN ('hubspot')),
  status        text NOT NULL CHECK (status IN ('active','revoked')) DEFAULT 'active',
  scopes        text[] NOT NULL DEFAULT '{}',
  connected_at  timestamptz NOT NULL DEFAULT now(),
  revoked_at    timestamptz
);

CREATE UNIQUE INDEX uq_incumbent_connection_workspace_connector
  ON incumbent_connection (workspace_id, connector);
CREATE INDEX idx_incumbent_connection_workspace_id ON incumbent_connection (workspace_id);

ALTER TABLE incumbent_connection ENABLE ROW LEVEL SECURITY;
ALTER TABLE incumbent_connection FORCE ROW LEVEL SECURITY;
CREATE POLICY incumbent_connection_tenant_isolation ON incumbent_connection
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

CREATE TABLE connector_secret (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  connection_id uuid NOT NULL REFERENCES incumbent_connection(id) ON DELETE CASCADE,
  ciphertext    bytea NOT NULL,
  kms_key_id    text NOT NULL,
  rotated_at    timestamptz NOT NULL DEFAULT now(),
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_connector_secret_connection_id ON connector_secret (connection_id, rotated_at DESC);
CREATE INDEX idx_connector_secret_workspace_id ON connector_secret (workspace_id);

ALTER TABLE connector_secret ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector_secret FORCE ROW LEVEL SECURITY;
CREATE POLICY connector_secret_tenant_isolation ON connector_secret
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
