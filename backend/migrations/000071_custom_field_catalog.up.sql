-- 000071 — custom_field catalog (CUSTOM-FIELDS-SCHEMA-1): system-of-record for every
-- runtime-added column. Base columns (DM-CONV-3) + version (DM-CONV-4) + RLS (DM-CONV-5-8),
-- mirroring 000069_record_grant's tenant_isolation shape (that migration itself has no
-- updated_at/archived_at/version-trigger, so the trigger here instead mirrors
-- 000003_core_objects's trg_lead_touch). No cf_-prefixed columns land here — that engine
-- work is a later ticket (CF-T03). No options sidecar table — picklist values live in the
-- `options` jsonb column (custom-fields.md, "no separate options table in V1").
--
-- DEVIATION (documented): no CHECK ties `currency` to `type = 'currency'` — the spec asks
-- for that conditional-required validation to stay an application-layer (handler) concern
-- rather than a non-trivial cross-column DDL expression.

CREATE TABLE custom_field (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  object        text NOT NULL CHECK (object IN ('person','organization','deal','lead','activity')),
  slug          text NOT NULL,
  label         text NOT NULL,
  type          text NOT NULL CHECK (type IN ('text','number','date','currency','picklist','boolean')),
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  column_name   text NOT NULL,
  currency      char(3) NULL CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),
  options       jsonb NULL,
  created_by    uuid NOT NULL REFERENCES app_user(id),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,
  version       bigint NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX idx_custom_field_slug ON custom_field (workspace_id, object, slug);
CREATE UNIQUE INDEX idx_custom_field_col  ON custom_field (workspace_id, object, column_name);

CREATE TRIGGER trg_custom_field_touch BEFORE UPDATE ON custom_field
  FOR EACH ROW EXECUTE FUNCTION touch_versioned();

ALTER TABLE custom_field ENABLE ROW LEVEL SECURITY;
ALTER TABLE custom_field FORCE ROW LEVEL SECURITY;

CREATE POLICY custom_field_tenant_isolation ON custom_field
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON custom_field TO margince_app;
