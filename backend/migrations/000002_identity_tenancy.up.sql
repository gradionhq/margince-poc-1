-- 000002 — identity & tenancy (data-model §2). workspace is the tenant root
-- (no RLS); every other table is tenant-scoped + RLS-enforced.

CREATE TABLE workspace (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  name          text NOT NULL,
  slug          text NOT NULL,
  base_currency char(3) NOT NULL,
  timezone      text NOT NULL DEFAULT 'UTC',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,
  CONSTRAINT workspace_slug_unique UNIQUE (slug),
  CONSTRAINT workspace_base_currency_iso CHECK (base_currency ~ '^[A-Z]{3}$')
);
CREATE TRIGGER trg_workspace_updated BEFORE UPDATE ON workspace
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE app_user (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  email         text NOT NULL,
  display_name  text NOT NULL,
  timezone      text NOT NULL DEFAULT 'UTC',
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deactivated')),
  is_agent      boolean NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL
);
-- expression unique key must be an index, not a table constraint
CREATE UNIQUE INDEX app_user_email_unique ON app_user (workspace_id, lower(email));
CREATE INDEX idx_app_user_ws ON app_user (workspace_id) WHERE archived_at IS NULL;
CREATE TRIGGER trg_app_user_updated BEFORE UPDATE ON app_user
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Tenant isolation (data-model §1.3): deny-on-unset GUC; a connection with no
-- app.workspace_id sees zero rows and writes nothing.
ALTER TABLE app_user ENABLE ROW LEVEL SECURITY;
ALTER TABLE app_user FORCE ROW LEVEL SECURITY;
CREATE POLICY app_user_tenant_isolation ON app_user
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
