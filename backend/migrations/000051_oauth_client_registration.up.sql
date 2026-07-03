-- 000051 — OAuth 2.1 DCR client registry + PKCE authorization codes (ADR-0013 A2 transport).
-- Public clients only (no client_secret column — PKCE S256 replaces it per OAuth 2.1).
CREATE TABLE oauth_client (
  client_id     uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  redirect_uris text[] NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_oauth_client_workspace_fk ON oauth_client (workspace_id);
ALTER TABLE oauth_client ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_client FORCE ROW LEVEL SECURITY;
CREATE POLICY oauth_client_tenant_isolation ON oauth_client
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- One-time-use, short-lived (<=10 min) authorization code, hashed at rest like
-- passport/session token_hash. Consume (Task 2) is the only writer of used_at.
CREATE TABLE oauth_auth_code (
  code_hash      text PRIMARY KEY,
  client_id      uuid NOT NULL REFERENCES oauth_client(client_id) ON DELETE CASCADE,
  workspace_id   uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  code_challenge text NOT NULL,
  redirect_uri   text NOT NULL,
  scopes         text[] NOT NULL DEFAULT '{}',
  granted_by     uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  expires_at     timestamptz NOT NULL,
  used_at        timestamptz NULL,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_oauth_auth_code_client_fk ON oauth_auth_code (client_id);
CREATE INDEX idx_oauth_auth_code_workspace_fk ON oauth_auth_code (workspace_id);
CREATE INDEX idx_oauth_auth_code_granted_by_fk ON oauth_auth_code (granted_by);
CREATE INDEX idx_oauth_auth_code_expires ON oauth_auth_code (expires_at);
ALTER TABLE oauth_auth_code ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_auth_code FORCE ROW LEVEL SECURITY;
CREATE POLICY oauth_auth_code_tenant_isolation ON oauth_auth_code
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE ON oauth_client, oauth_auth_code TO margince_app;

COMMIT;
