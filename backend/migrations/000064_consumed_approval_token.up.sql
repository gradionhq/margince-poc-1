-- 000064 consumed_approval_token: single-use consumption ledger for the
-- X-Approval-Token verify/consume seam (T12, gate issue #59 Option 1).
-- Minting a token (an approval decision issuing one) is out of scope here;
-- this table only records first-use so a replay is rejected (APPR-PARAM-3).
CREATE TABLE consumed_approval_token (
    jti          text PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspace(id),
    consumed_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_consumed_approval_token_ws ON consumed_approval_token (workspace_id);

ALTER TABLE consumed_approval_token ENABLE ROW LEVEL SECURITY;
ALTER TABLE consumed_approval_token FORCE ROW LEVEL SECURITY;

CREATE POLICY consumed_approval_token_tenant_isolation ON consumed_approval_token
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

GRANT SELECT, INSERT ON consumed_approval_token TO margince_app;
