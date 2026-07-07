BEGIN;

-- (g reverse) consent_purpose: collapse back to one global row per key. Lossy
-- but safe (mirrors this repo's other one-way-ish backfill migrations, e.g.
-- 000022's own down.sql just drops the tables outright): per-workspace label
-- edits or archival made after the up-migration are not recoverable here — the
-- oldest surviving row per key becomes the new global row, and every consumer
-- FK is remapped onto it before the rest are deleted.
DROP POLICY IF EXISTS consent_purpose_tenant_isolation ON consent_purpose;
ALTER TABLE consent_purpose DISABLE ROW LEVEL SECURITY;
REVOKE INSERT ON consent_purpose FROM margince_app;

DROP INDEX IF EXISTS idx_consent_purpose_ws;

CREATE TEMP TABLE consent_purpose_canonical AS
SELECT DISTINCT ON (key) id, key
FROM consent_purpose
ORDER BY key, id;

ALTER TABLE consent_event DISABLE TRIGGER trg_consent_event_no_mutate;
UPDATE consent_event ce
SET purpose_id = canon.id
FROM consent_purpose cp, consent_purpose_canonical canon
WHERE ce.purpose_id = cp.id AND cp.key = canon.key;
ALTER TABLE consent_event ENABLE TRIGGER trg_consent_event_no_mutate;

UPDATE person_consent pc
SET purpose_id = canon.id
FROM consent_purpose cp, consent_purpose_canonical canon
WHERE pc.purpose_id = cp.id AND cp.key = canon.key;

DELETE FROM consent_purpose WHERE id NOT IN (SELECT id FROM consent_purpose_canonical);
DROP TABLE consent_purpose_canonical;

ALTER TABLE consent_purpose DROP CONSTRAINT IF EXISTS uq_consent_purpose_workspace_key;
ALTER TABLE consent_purpose DROP COLUMN workspace_id;
ALTER TABLE consent_purpose DROP COLUMN archived_at;
ALTER TABLE consent_purpose ALTER COLUMN label DROP NOT NULL;
ALTER TABLE consent_purpose RENAME COLUMN label TO description;
ALTER TABLE consent_purpose RENAME COLUMN key TO name;
ALTER TABLE consent_purpose ADD CONSTRAINT consent_purpose_name_key UNIQUE (name);

-- (f reverse) app_user seat_type.
ALTER TABLE app_user DROP CONSTRAINT IF EXISTS app_user_agent_is_full;
ALTER TABLE app_user DROP COLUMN IF EXISTS seat_type;

-- (e reverse) passport.
DROP INDEX IF EXISTS idx_passport_obo;
DROP INDEX IF EXISTS idx_passport_obo_fk;
ALTER TABLE passport DROP COLUMN IF EXISTS on_behalf_of;
ALTER TABLE passport DROP COLUMN IF EXISTS label;
ALTER TABLE passport
  DROP CONSTRAINT passport_granted_by_fkey,
  ADD CONSTRAINT passport_granted_by_fkey FOREIGN KEY (granted_by) REFERENCES app_user(id) ON DELETE CASCADE;
ALTER TABLE passport
  DROP CONSTRAINT passport_workspace_id_fkey,
  ADD CONSTRAINT passport_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE;

-- (d reverse) session.
DROP INDEX idx_session_user;
CREATE INDEX idx_session_user ON session (workspace_id, user_id);
ALTER TABLE session DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE session DROP COLUMN IF EXISTS ip;
ALTER TABLE session DROP COLUMN IF EXISTS user_agent;
ALTER TABLE session
  DROP CONSTRAINT session_workspace_id_fkey,
  ADD CONSTRAINT session_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE;

-- (c reverse) event_outbox RLS back to the pre-canonical (USING-only) policy.
DROP POLICY event_outbox_tenant_isolation ON event_outbox;
CREATE POLICY event_outbox_ws ON event_outbox
  USING (workspace_id::text = current_setting('app.workspace_id', true));

-- (b reverse) updated_at triggers.
DROP TRIGGER IF EXISTS trg_embedding_updated ON embedding;
DROP TRIGGER IF EXISTS trg_person_phone_updated ON person_phone;
DROP TRIGGER IF EXISTS trg_organization_domain_updated ON organization_domain;

-- (a reverse) uuidv7 defaults back to gen_random_uuid().
ALTER TABLE approval_item ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE embedding     ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE ai_metering   ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE event_outbox  ALTER COLUMN id SET DEFAULT gen_random_uuid();

COMMIT;
