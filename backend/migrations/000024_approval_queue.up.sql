-- 000024 approval_queue: the approval-inbox table for staged 🟡 actions.
CREATE TABLE approval_item (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id         uuid NOT NULL REFERENCES workspace(id),
    action_type          text NOT NULL,
    payload              jsonb NOT NULL,
    dry_run_preview      jsonb,
    status               text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','approved','rejected','modified','expired')),
    requested_by         text NOT NULL,
    decided_by           text,
    decided_at           timestamptz,
    expires_at           timestamptz NOT NULL,
    trust_tiers          jsonb,
    content_egress_flags jsonb,
    created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX approval_item_workspace_id_idx ON approval_item (workspace_id);
CREATE INDEX approval_item_ws_status_idx ON approval_item (workspace_id, status);

ALTER TABLE approval_item ENABLE ROW LEVEL SECURITY;
ALTER TABLE approval_item FORCE ROW LEVEL SECURITY;

CREATE POLICY approval_item_tenant_isolation ON approval_item
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE ON approval_item TO margince_app;

-- Widen the audit action enum so approval decisions can be audited (N2).
ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_action_check;
ALTER TABLE audit_log ADD  CONSTRAINT audit_log_action_check
  CHECK (action IN ('create','update','archive','merge','promote','restore',
                    'export','erase','login','assign','advance_stage','capture',
                    'approve','reject','modify','expired'));
