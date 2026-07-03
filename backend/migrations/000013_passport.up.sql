-- 000013 — Agent Seat Passport (ADR-0013)
CREATE TABLE passport (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  granted_by   uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  scopes       text[] NOT NULL DEFAULT '{}',
  token_hash   text NOT NULL,
  expires_at   timestamptz NOT NULL,
  revoked_at   timestamptz NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_passport_token_hash ON passport (token_hash);
CREATE INDEX idx_passport_grantor ON passport (workspace_id, granted_by);
-- Single-column FK indexes (each FK child column needs its own index — see 000010).
CREATE INDEX idx_passport_workspace_fk ON passport (workspace_id);
CREATE INDEX idx_passport_granted_by_fk ON passport (granted_by);
ALTER TABLE passport ENABLE ROW LEVEL SECURITY;
ALTER TABLE passport FORCE ROW LEVEL SECURITY;
CREATE POLICY passport_tenant_isolation ON passport
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
