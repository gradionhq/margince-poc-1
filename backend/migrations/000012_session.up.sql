-- 000012 — Session table (opaque DB token, Approach B)
CREATE TABLE session (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id     uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  user_id          uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  token_hash       text NOT NULL,
  expires_at       timestamptz NOT NULL,
  idle_expires_at  timestamptz NOT NULL,
  last_seen_at     timestamptz NOT NULL DEFAULT now(),
  created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_session_token_hash ON session (token_hash);
CREATE INDEX idx_session_user ON session (workspace_id, user_id);
CREATE INDEX idx_session_expires ON session (expires_at);
-- Single-column FK indexes (each FK child column needs its own index — see 000010).
CREATE INDEX idx_session_workspace_fk ON session (workspace_id);
CREATE INDEX idx_session_user_fk ON session (user_id);
ALTER TABLE session ENABLE ROW LEVEL SECURITY;
ALTER TABLE session FORCE ROW LEVEL SECURITY;
CREATE POLICY session_tenant_isolation ON session
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
