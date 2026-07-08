-- 000073 — records-depth schema (RD-T03, RD-DDL-1..3 verbatim): attachment (object-store
-- references only, never blobs), quota (per-owner/team revenue target per period), and
-- bulk_operation (async bulk job, DDL single-homed here — its behavior belongs to the separate
-- bulk-operations story family, never built against here). Field history has deliberately no
-- table in this migration — it is a read-only projection over audit_log (data-model chapter).
--
-- No inter-table FK ordering constraint among the three — none references another.
--
-- Schema-drift note (RD-DDL-1 vs. 000009): attachment already exists from
-- 000009_lists_tags_attachments.up.sql with content_type/byte_size NOT NULL and no updated_at
-- column. RD-DDL-1 in the current docs pins a superset of that shape (content_type/byte_size
-- nullable, plus updated_at) — this migration reconciles the live table to that pin via ALTER,
-- not drop+recreate: attachment already has generated contract routes wired to it
-- (ListAttachments/CreateAttachment/ArchiveAttachment/GetAttachment) and may hold real rows, so
-- a drop+recreate would be needlessly destructive and not correctly reversible back to the
-- 000009 shape. The contract's crm_gen.go Attachment type comments (which assert those two
-- columns NOT NULL and no updated_at) go stale here — expected, since RD-T03 doesn't touch
-- crm.yaml/handlers; that's deferred to the separate attachments-wire ticket.

-- RD-DDL-1 — attachment (object-store references; never a bytea/blob column). No `version`
-- column — the schema note gives no "+ version" suffix for it, unlike quota/bulk_operation.
-- Reconcile the pre-existing (000009) table in place: relax content_type/byte_size to
-- nullable and add updated_at + its set_updated_at() trigger. Its index (idx_attachment_entity)
-- and RLS (enable/force/policy) already exist from 000009 and are untouched here.
ALTER TABLE attachment ALTER COLUMN content_type DROP NOT NULL;
ALTER TABLE attachment ALTER COLUMN byte_size DROP NOT NULL;
ALTER TABLE attachment ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER trg_attachment_updated BEFORE UPDATE ON attachment FOR EACH ROW EXECUTE FUNCTION set_updated_at();

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
-- match). attachment.entity_id is polymorphic (no REFERENCES) so it's exempt. attachment's own
-- workspace_id FK index (idx_attachment_ws) already exists from 000010_fk_indexes.up.sql — not
-- recreated here.
CREATE INDEX idx_quota_ws                    ON quota (workspace_id);
CREATE INDEX idx_quota_owner_fk              ON quota (owner_id) WHERE owner_id IS NOT NULL;
CREATE INDEX idx_quota_team_fk               ON quota (team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_bulk_operation_ws           ON bulk_operation (workspace_id);
CREATE INDEX idx_bulk_operation_requested_by_fk ON bulk_operation (requested_by);
