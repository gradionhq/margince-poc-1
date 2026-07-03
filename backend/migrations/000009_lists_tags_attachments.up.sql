-- 000009 — list/list_member, tag/taggable, attachment metadata (WP1 §10)

-- list + list_member (data-model §10.1)
CREATE TABLE list (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name        text NOT NULL,
  entity_type text NOT NULL CHECK (entity_type IN ('person','organization','deal','lead')),
  list_type   text NOT NULL DEFAULT 'static' CHECK (list_type IN ('static','dynamic')),
  definition  jsonb NULL,
  owner_id    uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  team_id     uuid NULL REFERENCES team(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz NULL
);
CREATE INDEX idx_list_ws ON list (workspace_id) WHERE archived_at IS NULL;
CREATE TRIGGER trg_list_updated BEFORE UPDATE ON list
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE list ENABLE ROW LEVEL SECURITY;
ALTER TABLE list FORCE ROW LEVEL SECURITY;
CREATE POLICY list_tenant_isolation ON list
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

CREATE TABLE list_member (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  list_id     uuid NOT NULL REFERENCES list(id) ON DELETE CASCADE,
  entity_type text NOT NULL CHECK (entity_type IN ('person','organization','deal','lead')),
  entity_id   uuid NOT NULL,
  added_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT list_member_unique UNIQUE (list_id, entity_type, entity_id)
);
CREATE INDEX idx_list_member_list ON list_member (list_id);
CREATE INDEX idx_list_member_entity ON list_member (workspace_id, entity_type, entity_id);
ALTER TABLE list_member ENABLE ROW LEVEL SECURITY;
ALTER TABLE list_member FORCE ROW LEVEL SECURITY;
CREATE POLICY list_member_tenant_isolation ON list_member
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- tag + taggable (data-model §10.2)
CREATE TABLE tag (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_tag ON tag (workspace_id, lower(name));
CREATE INDEX idx_tag_ws ON tag (workspace_id);
ALTER TABLE tag ENABLE ROW LEVEL SECURITY;
ALTER TABLE tag FORCE ROW LEVEL SECURITY;
CREATE POLICY tag_tenant_isolation ON tag
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

CREATE TABLE taggable (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  tag_id      uuid NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
  entity_type text NOT NULL CHECK (entity_type IN ('person','organization','deal','lead','activity')),
  entity_id   uuid NOT NULL,
  tagged_at   timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT taggable_unique UNIQUE (tag_id, entity_type, entity_id)
);
CREATE INDEX idx_taggable_entity ON taggable (workspace_id, entity_type, entity_id);
CREATE INDEX idx_taggable_tag ON taggable (tag_id);
ALTER TABLE taggable ENABLE ROW LEVEL SECURITY;
ALTER TABLE taggable FORCE ROW LEVEL SECURITY;
CREATE POLICY taggable_tenant_isolation ON taggable
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- attachment (data-model §10.3 — metadata only; bytes in object store)
CREATE TABLE attachment (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  entity_type  text NOT NULL CHECK (entity_type IN ('person','organization','deal','lead','activity')),
  entity_id    uuid NOT NULL,
  filename     text NOT NULL,
  content_type text NOT NULL,
  byte_size    bigint NOT NULL,
  storage_key  text NOT NULL,
  checksum     text NULL,
  source       text NOT NULL,
  captured_by  text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL
);
CREATE INDEX idx_attachment_entity ON attachment (workspace_id, entity_type, entity_id)
  WHERE archived_at IS NULL;
ALTER TABLE attachment ENABLE ROW LEVEL SECURITY;
ALTER TABLE attachment FORCE ROW LEVEL SECURITY;
CREATE POLICY attachment_tenant_isolation ON attachment
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
