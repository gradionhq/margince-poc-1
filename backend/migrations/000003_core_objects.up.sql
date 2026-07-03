-- 000003 — core domain objects (data-model §3/§4/§6/§7/§8/§11), WP1 spine.
-- Tenant tables carry workspace_id + RLS (deny-on-unset); mutable domain tables
-- carry version (§1.3a); captured/entered tables carry provenance (§1.6).
-- Created in FK order: lead -> person -> organization -> pipeline/stage/deal -> activity -> audit.

-- 8. lead (thin, segregated; no org FK — ADR-0008)
CREATE TABLE lead (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  full_name     text NULL,
  email         text NULL,
  title         text NULL,
  company_name  text NULL,           -- FREE TEXT, not an org FK (ADR-0008 §1)
  candidate_org_key text NULL,
  status        text NOT NULL DEFAULT 'new' CHECK (status IN ('new','working','promoted','disqualified')),
  score         integer NOT NULL DEFAULT 0,
  owner_id      uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  version       bigint NOT NULL DEFAULT 1,
  source        text NOT NULL,
  captured_by   text NOT NULL,
  raw           jsonb NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL
);
CREATE INDEX idx_lead_ws_live ON lead (workspace_id) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX uq_lead_email ON lead (workspace_id, lower(email)) WHERE email IS NOT NULL AND archived_at IS NULL;
CREATE TRIGGER trg_lead_touch BEFORE UPDATE ON lead FOR EACH ROW EXECUTE FUNCTION touch_versioned();

-- 3.1 person
CREATE TABLE person (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  first_name    text NULL,
  last_name     text NULL,
  full_name     text NOT NULL,
  title         text NULL,
  owner_id      uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  social        jsonb NOT NULL DEFAULT '{}'::jsonb,
  address       jsonb NULL,
  merged_into_id uuid NULL REFERENCES person(id) ON DELETE SET NULL,
  converted_from_lead_id uuid NULL REFERENCES lead(id) ON DELETE SET NULL,
  version       bigint NOT NULL DEFAULT 1,
  source        text NOT NULL,
  captured_by   text NOT NULL,
  raw           jsonb NULL,
  search_tsv    tsvector GENERATED ALWAYS AS (
                  to_tsvector('simple', coalesce(full_name,'') || ' ' || coalesce(title,''))) STORED,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL
);
CREATE INDEX idx_person_ws_live     ON person (workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_person_owner       ON person (workspace_id, owner_id) WHERE archived_at IS NULL;
CREATE INDEX idx_person_search      ON person USING gin (search_tsv);
CREATE INDEX idx_person_merged_into ON person (merged_into_id) WHERE merged_into_id IS NOT NULL;
CREATE INDEX idx_person_from_lead   ON person (converted_from_lead_id) WHERE converted_from_lead_id IS NOT NULL;
CREATE TRIGGER trg_person_touch BEFORE UPDATE ON person FOR EACH ROW EXECUTE FUNCTION touch_versioned();

-- 3.2 person_email (exact-email dedupe key)
CREATE TABLE person_email (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  person_id    uuid NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  email        text NOT NULL,
  email_type   text NOT NULL DEFAULT 'work' CHECK (email_type IN ('work','personal','other')),
  is_primary   boolean NOT NULL DEFAULT false,
  position     integer NOT NULL DEFAULT 0,
  source       text NOT NULL,
  captured_by  text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL
);
CREATE UNIQUE INDEX uq_person_email ON person_email (workspace_id, lower(email)) WHERE archived_at IS NULL;
CREATE INDEX idx_person_email_person ON person_email (person_id);
CREATE TRIGGER trg_person_email_updated BEFORE UPDATE ON person_email FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 4.1 organization (core slice)
CREATE TABLE organization (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name          text NOT NULL,
  website       text NULL,
  classification text NULL,
  owner_id      uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  version       bigint NOT NULL DEFAULT 1,
  source        text NOT NULL,
  captured_by   text NOT NULL,
  raw           jsonb NULL,
  search_tsv    tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(name,''))) STORED,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL
);
CREATE INDEX idx_org_ws_live ON organization (workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_org_search  ON organization USING gin (search_tsv);
CREATE TRIGGER trg_org_touch BEFORE UPDATE ON organization FOR EACH ROW EXECUTE FUNCTION touch_versioned();

-- 6.1 pipeline
CREATE TABLE pipeline (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name         text NOT NULL,
  is_default   boolean NOT NULL DEFAULT false,
  position     integer NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL,
  CONSTRAINT pipeline_name_unique UNIQUE (workspace_id, name)
);
CREATE UNIQUE INDEX uq_pipeline_default ON pipeline (workspace_id) WHERE is_default AND archived_at IS NULL;
CREATE TRIGGER trg_pipeline_updated BEFORE UPDATE ON pipeline FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 6.2 stage (composite-unique on (id,pipeline_id) backs the deal stage-in-pipeline FK)
CREATE TABLE stage (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  pipeline_id     uuid NOT NULL REFERENCES pipeline(id) ON DELETE CASCADE,
  name            text NOT NULL,
  position        integer NOT NULL,
  semantic        text NOT NULL DEFAULT 'open' CHECK (semantic IN ('open','won','lost')),
  win_probability smallint NOT NULL DEFAULT 0 CHECK (win_probability BETWEEN 0 AND 100),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL,
  CONSTRAINT stage_terminal_prob CHECK (
    (semantic = 'won'  AND win_probability = 100) OR
    (semantic = 'lost' AND win_probability = 0)   OR
    (semantic = 'open')),
  CONSTRAINT stage_id_pipeline_unique UNIQUE (id, pipeline_id)
);
CREATE UNIQUE INDEX uq_stage_position ON stage (pipeline_id, position) WHERE archived_at IS NULL;
CREATE INDEX idx_stage_pipeline ON stage (pipeline_id) WHERE archived_at IS NULL;
CREATE TRIGGER trg_stage_updated BEFORE UPDATE ON stage FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 6.3 deal
CREATE TABLE deal (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name            text NOT NULL,
  amount_minor    bigint NULL,
  currency        char(3) NULL CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),
  fx_rate_to_base numeric(20,10) NULL,
  fx_rate_date    date NULL,
  pipeline_id     uuid NOT NULL REFERENCES pipeline(id) ON DELETE RESTRICT,
  stage_id        uuid NOT NULL,
  organization_id uuid NULL REFERENCES organization(id) ON DELETE SET NULL,
  owner_id        uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  partner_org_id  uuid NULL REFERENCES organization(id) ON DELETE SET NULL,
  status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open','won','lost')),
  lost_reason     text NULL,
  expected_close_date date NULL,
  closed_at       timestamptz NULL,
  forecast_category text NULL CHECK (forecast_category IS NULL OR forecast_category IN ('commit','best_case','pipeline','omitted')),
  wait_until      date NULL,
  last_activity_at timestamptz NULL,
  version         bigint NOT NULL DEFAULT 1,
  source          text NOT NULL,
  captured_by     text NOT NULL,
  raw             jsonb NULL,
  search_tsv      tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(name,''))) STORED,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL,
  CONSTRAINT deal_lost_reason CHECK (status <> 'lost' OR lost_reason IS NOT NULL),
  CONSTRAINT deal_closed_at   CHECK (status = 'open' OR closed_at IS NOT NULL),
  CONSTRAINT deal_closed_fx   CHECK (status = 'open' OR amount_minor IS NULL OR fx_rate_to_base IS NOT NULL),
  -- stage must belong to the deal's pipeline (DB-guaranteed composite FK, §6.3)
  CONSTRAINT deal_stage_in_pipeline FOREIGN KEY (stage_id, pipeline_id) REFERENCES stage (id, pipeline_id) ON DELETE RESTRICT
);
CREATE INDEX idx_deal_ws_live  ON deal (workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_deal_stage    ON deal (stage_id) WHERE archived_at IS NULL;
CREATE INDEX idx_deal_pipeline ON deal (pipeline_id, stage_id) WHERE archived_at IS NULL;
CREATE INDEX idx_deal_owner    ON deal (workspace_id, owner_id) WHERE archived_at IS NULL;
CREATE INDEX idx_deal_org      ON deal (organization_id) WHERE organization_id IS NOT NULL AND archived_at IS NULL;
CREATE INDEX idx_deal_stalled  ON deal (workspace_id, last_activity_at) WHERE status = 'open' AND archived_at IS NULL;
CREATE INDEX idx_deal_search   ON deal USING gin (search_tsv);
CREATE TRIGGER trg_deal_touch BEFORE UPDATE ON deal FOR EACH ROW EXECUTE FUNCTION touch_versioned();

-- 7. activity + activity_link
CREATE TABLE activity (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  kind          text NOT NULL CHECK (kind IN ('email','call','meeting','note','task','whatsapp','telegram')),
  subject       text NULL,
  body          text NULL,
  occurred_at   timestamptz NOT NULL DEFAULT now(),
  due_at        timestamptz NULL,
  assignee_id   uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  is_done       boolean NOT NULL DEFAULT false,
  done_at       timestamptz NULL,
  duration_seconds integer NULL,
  direction     text NULL CHECK (direction IS NULL OR direction IN ('inbound','outbound')),
  meeting_status text NULL CHECK (meeting_status IS NULL OR meeting_status IN ('booked','held','no_show','canceled')),
  source_system text NULL,
  source_id     text NULL,
  version       bigint NOT NULL DEFAULT 1,
  source        text NOT NULL,
  captured_by   text NOT NULL,
  raw           jsonb NULL,
  search_tsv    tsvector GENERATED ALWAYS AS (
                  to_tsvector('simple', coalesce(subject,'') || ' ' || coalesce(body,''))) STORED,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,
  CONSTRAINT activity_task_fields CHECK (kind = 'task' OR (due_at IS NULL AND assignee_id IS NULL AND is_done = false)),
  CONSTRAINT activity_done_at CHECK (is_done = false OR done_at IS NOT NULL)
);
CREATE UNIQUE INDEX uq_activity_source ON activity (workspace_id, source_system, source_id)
  WHERE source_system IS NOT NULL AND source_id IS NOT NULL;
CREATE INDEX idx_activity_ws_time ON activity (workspace_id, occurred_at DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_activity_kind    ON activity (workspace_id, kind, occurred_at DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_activity_tasks   ON activity (workspace_id, assignee_id, due_at) WHERE kind = 'task' AND is_done = false AND archived_at IS NULL;
CREATE INDEX idx_activity_search  ON activity USING gin (search_tsv);
CREATE TRIGGER trg_activity_touch BEFORE UPDATE ON activity FOR EACH ROW EXECUTE FUNCTION touch_versioned();

CREATE TABLE activity_link (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  activity_id   uuid NOT NULL REFERENCES activity(id) ON DELETE CASCADE,
  entity_type   text NOT NULL CHECK (entity_type IN ('person','organization','deal')),
  person_id       uuid NULL REFERENCES person(id) ON DELETE CASCADE,
  organization_id uuid NULL REFERENCES organization(id) ON DELETE CASCADE,
  deal_id         uuid NULL REFERENCES deal(id) ON DELETE CASCADE,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT activity_link_shape CHECK (
    (entity_type='person'       AND person_id IS NOT NULL AND organization_id IS NULL AND deal_id IS NULL) OR
    (entity_type='organization' AND organization_id IS NOT NULL AND person_id IS NULL AND deal_id IS NULL) OR
    (entity_type='deal'         AND deal_id IS NOT NULL AND person_id IS NULL AND organization_id IS NULL))
);
CREATE UNIQUE INDEX uq_activity_link ON activity_link (activity_id, entity_type, coalesce(person_id,organization_id,deal_id));
CREATE INDEX idx_alink_person ON activity_link (person_id) WHERE person_id IS NOT NULL;
CREATE INDEX idx_alink_org    ON activity_link (organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_alink_deal   ON activity_link (deal_id) WHERE deal_id IS NOT NULL;

-- 11. audit_log (append-only spine, P12)
CREATE TABLE audit_log (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  actor_type    text NOT NULL CHECK (actor_type IN ('human','agent','system')),
  actor_id      text NOT NULL,
  passport_id   uuid NULL,
  on_behalf_of  uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  action        text NOT NULL CHECK (action IN ('create','update','archive','merge','promote','restore','export','erase','login','assign','advance_stage')),
  entity_type   text NOT NULL,
  entity_id     uuid NULL,
  before        jsonb NULL,
  after         jsonb NULL,
  authorization_rule text NULL,
  evidence      jsonb NULL,
  occurred_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_entity ON audit_log (workspace_id, entity_type, entity_id, occurred_at DESC);
CREATE INDEX idx_audit_actor  ON audit_log (workspace_id, actor_id, occurred_at DESC);
CREATE INDEX idx_audit_time   ON audit_log (workspace_id, occurred_at DESC);

CREATE OR REPLACE FUNCTION audit_log_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'audit_log is append-only (attempted % on row %)', TG_OP, OLD.id
    USING ERRCODE = 'check_violation';
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_audit_no_mutate BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

-- Tenant isolation (RLS, deny-on-unset) on every tenant table created above.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'lead','person','person_email','organization','pipeline','stage',
    'deal','activity','activity_link','audit_log'
  ] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY;', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY;', t);
    EXECUTE format($f$
      CREATE POLICY %1$s_tenant_isolation ON %1$I
        USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
        WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
    $f$, t);
  END LOOP;
END $$;
