-- 000073 — records-depth schema (RD-T03, RD-DDL-1..3 verbatim): attachment (object-store
-- references only, never blobs), quota (per-owner/team revenue target per period), and
-- bulk_operation (async bulk job, DDL single-homed here — its behavior belongs to the separate
-- bulk-operations story family, never built against here). Field history has deliberately no
-- table in this migration — it is a read-only projection over audit_log (data-model chapter).
--
-- No inter-table FK ordering constraint among the three — none references another.
--
-- Note: attachment table exists from 000009 with different schema (content_type/byte_size NOT NULL,
-- no updated_at). This migration drops and recreates it to match RD-DDL-1 spec.

-- RD-DDL-1 — attachment (object-store references; never a bytea/blob column). No `version`
-- column — the schema note gives no "+ version" suffix for it, unlike quota/bulk_operation.
-- Drop existing attachment table first to recreate with correct spec schema.
DROP TABLE IF EXISTS attachment CASCADE;

CREATE TABLE attachment (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  entity_type  text NOT NULL CHECK (entity_type IN ('person','organization','deal','activity','lead')),
  entity_id    uuid NOT NULL,
  filename     text NOT NULL,
  content_type text NULL,
  byte_size    bigint NULL,
  storage_key  text NOT NULL,      -- S3/MinIO object key
  checksum     text NULL,          -- sha256 for dedupe/integrity
  source       text NOT NULL,
  captured_by  text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL
);
CREATE INDEX idx_attachment_entity ON attachment (workspace_id, entity_type, entity_id) WHERE archived_at IS NULL;
CREATE TRIGGER trg_attachment_updated BEFORE UPDATE ON attachment FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE attachment ENABLE ROW LEVEL SECURITY;
ALTER TABLE attachment FORCE ROW LEVEL SECURITY;
CREATE POLICY attachment_tenant_isolation ON attachment
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON attachment TO margince_app;

-- RD-DDL-2 — quota (per-owner/team revenue target per period, E09 forecast attainment)
CREATE TABLE quota (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  owner_id      uuid NULL REFERENCES app_user(id),
  team_id       uuid NULL REFERENCES team(id),
  period_start  date NOT NULL,
  period_end    date NOT NULL,
  target_minor  bigint NOT NULL,
  currency      char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  version       bigint NOT NULL DEFAULT 1,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,
  CONSTRAINT quota_owner_xor_team CHECK ((owner_id IS NOT NULL) <> (team_id IS NOT NULL))
);
CREATE TRIGGER trg_quota_touch BEFORE UPDATE ON quota FOR EACH ROW EXECUTE FUNCTION touch_versioned();

ALTER TABLE quota ENABLE ROW LEVEL SECURITY;
ALTER TABLE quota FORCE ROW LEVEL SECURITY;
CREATE POLICY quota_tenant_isolation ON quota
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON quota TO margince_app;

-- RD-DDL-3 — bulk_operation (async bulk job over many records; DDL single-homed here, its
-- behavior belongs to the bulk-operations story family — no handler/service logic here)
CREATE TABLE bulk_operation (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  kind            text NOT NULL,
  status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','done','failed')),
  total           int  NOT NULL DEFAULT 0,
  succeeded       int  NOT NULL DEFAULT 0,
  failed          int  NOT NULL DEFAULT 0,
  request_payload jsonb NOT NULL,
  result_summary  jsonb NULL,
  idempotency_key text  NULL,
  requested_by    uuid NOT NULL REFERENCES app_user(id),
  version         bigint NOT NULL DEFAULT 1,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL
);
CREATE UNIQUE INDEX idx_bulk_idem ON bulk_operation (workspace_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_bulk_status ON bulk_operation (workspace_id, status, created_at DESC);
CREATE TRIGGER trg_bulk_operation_touch BEFORE UPDATE ON bulk_operation FOR EACH ROW EXECUTE FUNCTION touch_versioned();

ALTER TABLE bulk_operation ENABLE ROW LEVEL SECURITY;
ALTER TABLE bulk_operation FORCE ROW LEVEL SECURITY;
CREATE POLICY bulk_operation_tenant_isolation ON bulk_operation
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON bulk_operation TO margince_app;

-- Dedicated single-column FK indexes (DM-CONV-14 / TestFKColumnsAreIndexed): the pinned
-- composite/partial indexes above don't satisfy the repo-wide invariant's exact
-- indexdef LIKE '%(col)%' check for these FK columns (a leading-composite-column doesn't
-- match). attachment.entity_id is polymorphic (no REFERENCES) so it's exempt.
CREATE INDEX idx_attachment_ws               ON attachment (workspace_id);
CREATE INDEX idx_quota_ws                    ON quota (workspace_id);
CREATE INDEX idx_quota_owner_fk              ON quota (owner_id) WHERE owner_id IS NOT NULL;
CREATE INDEX idx_quota_team_fk               ON quota (team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_bulk_operation_ws           ON bulk_operation (workspace_id);
CREATE INDEX idx_bulk_operation_requested_by_fk ON bulk_operation (requested_by);
