-- 000005 — RBAC: team, team_membership, role, role_assignment (WP1 §2.3–§2.4)

-- team
CREATE TABLE team (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name         text NOT NULL,
  parent_team_id uuid NULL REFERENCES team(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL,
  CONSTRAINT team_name_unique UNIQUE (workspace_id, name)
);
CREATE INDEX idx_team_ws ON team (workspace_id) WHERE archived_at IS NULL;
CREATE TRIGGER trg_team_updated BEFORE UPDATE ON team
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE team ENABLE ROW LEVEL SECURITY;
ALTER TABLE team FORCE ROW LEVEL SECURITY;
CREATE POLICY team_tenant_isolation ON team
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- team_membership
CREATE TABLE team_membership (
  id      uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  team_id uuid NOT NULL REFERENCES team(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT team_membership_unique UNIQUE (team_id, user_id)
);
CREATE INDEX idx_team_membership_team ON team_membership (team_id);
CREATE INDEX idx_team_membership_user ON team_membership (workspace_id, user_id);
ALTER TABLE team_membership ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_membership FORCE ROW LEVEL SECURITY;
CREATE POLICY team_membership_tenant_isolation ON team_membership
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- role
CREATE TABLE role (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  key         text NOT NULL,
  is_system   boolean NOT NULL DEFAULT false,
  permissions jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT role_key_unique UNIQUE (workspace_id, key)
);
CREATE INDEX idx_role_ws ON role (workspace_id);
CREATE TRIGGER trg_role_updated BEFORE UPDATE ON role
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE role ENABLE ROW LEVEL SECURITY;
ALTER TABLE role FORCE ROW LEVEL SECURITY;
CREATE POLICY role_tenant_isolation ON role
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- role_assignment
CREATE TABLE role_assignment (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  role_id     uuid NOT NULL REFERENCES role(id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  team_id     uuid NULL REFERENCES team(id) ON DELETE CASCADE,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX role_assignment_unique ON role_assignment
  (role_id, user_id, COALESCE(team_id, '00000000-0000-0000-0000-000000000000'::uuid));
CREATE INDEX idx_role_assignment_user ON role_assignment (workspace_id, user_id);
ALTER TABLE role_assignment ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_assignment FORCE ROW LEVEL SECURITY;
CREATE POLICY role_assignment_tenant_isolation ON role_assignment
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
