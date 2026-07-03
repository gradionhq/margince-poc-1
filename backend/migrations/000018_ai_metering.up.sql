BEGIN;
CREATE TABLE ai_metering (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  uuid NOT NULL REFERENCES workspace(id),
    task          text NOT NULL,
    tier          text NOT NULL,
    tokens_in     integer NOT NULL DEFAULT 0,
    tokens_out    integer NOT NULL DEFAULT 0,
    cached        boolean NOT NULL DEFAULT false,
    cost_est      double precision NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE ai_metering ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_metering FORCE ROW LEVEL SECURITY;
CREATE POLICY ai_metering_tenant_isolation ON ai_metering
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
-- Standalone FK index (every FK column must be individually indexed —
-- TestFKColumnsAreIndexed; a composite's leading column does not satisfy it).
CREATE INDEX ix_ai_metering_workspace_id ON ai_metering (workspace_id);
-- Query index for per-workspace time-ordered reads.
CREATE INDEX ix_ai_metering_ws_created ON ai_metering (workspace_id, created_at DESC);
COMMIT;
