-- 000019 — audit ring hardening + agent-action trace capture (EP07 Part 1).
-- Part A (B-EP07.1 fix): drop the silent DO INSTEAD NOTHING rules added by
-- migration 000015. Those rules rewrite UPDATE/DELETE to nothing, so the
-- BEFORE trigger trg_audit_no_mutate never fires and the mutation silently
-- no-ops. The spec requires a LOUD failure; dropping the rules lets the
-- trigger RAISE EXCEPTION (txn aborts, row unchanged). Append-only is now
-- enforced solely by audit_log_immutable() (migration 000003).
BEGIN;
DROP RULE IF EXISTS audit_log_no_update ON audit_log;
DROP RULE IF EXISTS audit_log_no_delete ON audit_log;
COMMIT;

-- Part B (B-EP07.4): idempotency guard for audit.appended bus emission
-- (dedup on audit_log_id, which is event_outbox.entity_id for this topic).
BEGIN;
CREATE UNIQUE INDEX IF NOT EXISTS uq_outbox_audit_appended
  ON event_outbox (entity_id) WHERE topic = 'audit.appended';
COMMIT;

-- Part C (B-EP07.5): producer-agnostic agent-action trace capture, linked to
-- audit_log by a stable trace id. RLS-scoped per data-model §1. Does not
-- depend on the Surface-B runner existing.
BEGIN;
CREATE TABLE agent_trace (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id   uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  trace_id       text NOT NULL,
  audit_log_id   uuid NULL REFERENCES audit_log(id) ON DELETE SET NULL,
  actor_id       text NOT NULL,
  inputs         jsonb NULL,
  tool_calls     jsonb NOT NULL DEFAULT '[]'::jsonb,
  outputs        jsonb NULL,
  approval_state text NULL,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_agent_trace_audit ON agent_trace (audit_log_id);
CREATE INDEX ix_agent_trace_ws_trace ON agent_trace (workspace_id, trace_id);
CREATE INDEX ix_agent_trace_ws ON agent_trace (workspace_id);
ALTER TABLE agent_trace ENABLE ROW LEVEL SECURITY;
ALTER TABLE agent_trace FORCE ROW LEVEL SECURITY;
CREATE POLICY agent_trace_tenant_isolation ON agent_trace
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
COMMIT;
