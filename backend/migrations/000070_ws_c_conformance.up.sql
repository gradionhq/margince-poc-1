-- 000070 — WS-C data-model conformance sweep (AC-C1..AC-C6, D2/D3/D4).
BEGIN;

-- (a) AC-C1: restore uuidv7() PK defaults on the four surviving gen_random_uuid()
-- tables. oauth_auth_code.code_hash / consumed_approval_token.jti are natural-key
-- text PKs (never had a uuid default) and are intentionally left untouched.
ALTER TABLE event_outbox  ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE ai_metering   ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE embedding     ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE approval_item ALTER COLUMN id SET DEFAULT uuidv7();

-- (b) AC-C2: restore the three missing updated_at triggers.
CREATE TRIGGER trg_organization_domain_updated BEFORE UPDATE ON organization_domain
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_person_phone_updated BEFORE UPDATE ON person_phone
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_embedding_updated BEFORE UPDATE ON embedding
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- (c) event_outbox: canonical deny-on-unset RLS (the original policy compared
-- workspace_id::text to the raw GUC text with no WITH CHECK half; the canonical
-- form casts through nullif(...,'')::uuid both ways, matching every other
-- tenant-scoped table in this schema).
DROP POLICY event_outbox_ws ON event_outbox;
CREATE POLICY event_outbox_tenant_isolation ON event_outbox
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- (d) AC-C3 + AC-C5: session — workspace_id FK CASCADE→RESTRICT; restore
-- user_agent/ip/revoked_at (DM-DDL-6/7-adjacent session columns); idx_session_user
-- becomes a partial index over live (non-revoked) sessions, mirroring the
-- idx_passport_obo partial treatment below.
ALTER TABLE session
  DROP CONSTRAINT session_workspace_id_fkey,
  ADD CONSTRAINT session_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE RESTRICT;
ALTER TABLE session
  ADD COLUMN user_agent text NULL,
  ADD COLUMN ip         inet NULL,
  ADD COLUMN revoked_at timestamptz NULL;
DROP INDEX idx_session_user;
CREATE INDEX idx_session_user ON session (workspace_id, user_id) WHERE revoked_at IS NULL;

-- (e) AC-C3 (+D3): passport — both workspace_id AND granted_by FK CASCADE→RESTRICT
-- (D3: granted_by is a second genuine divergence beyond AC-C3's literal text);
-- restore on_behalf_of/label (DM-DDL-7), backfilling on_behalf_of from the
-- historical sole grantor before enforcing NOT NULL.
ALTER TABLE passport
  DROP CONSTRAINT passport_workspace_id_fkey,
  ADD CONSTRAINT passport_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE RESTRICT;
ALTER TABLE passport
  DROP CONSTRAINT passport_granted_by_fkey,
  ADD CONSTRAINT passport_granted_by_fkey FOREIGN KEY (granted_by) REFERENCES app_user(id) ON DELETE RESTRICT;
ALTER TABLE passport
  ADD COLUMN on_behalf_of uuid NULL REFERENCES app_user(id) ON DELETE CASCADE,
  ADD COLUMN label        text NULL;
UPDATE passport SET on_behalf_of = granted_by; -- NOSONAR: intentional full-table backfill of every existing row from the historical sole grantor, not an accidentally-missing WHERE
ALTER TABLE passport ALTER COLUMN on_behalf_of SET NOT NULL;
-- Standalone FK-column index (every FK child column needs its own — see 000010);
-- the partial composite below does not satisfy that on its own (its leading
-- column is workspace_id, not on_behalf_of).
CREATE INDEX idx_passport_obo_fk ON passport (on_behalf_of);
CREATE INDEX idx_passport_obo ON passport (workspace_id, on_behalf_of) WHERE revoked_at IS NULL;

-- (f) AC-C4: app_user seat_type + the agent-is-full-seat CHECK.
ALTER TABLE app_user
  ADD COLUMN seat_type text NOT NULL DEFAULT 'full' CHECK (seat_type IN ('read','full'));
ALTER TABLE app_user
  ADD CONSTRAINT app_user_agent_is_full CHECK (NOT is_agent OR seat_type = 'full');

-- (g) AC-C6 (D2): consent_purpose widens from a global lookup table to a
-- per-workspace one (DM-DDL-10). Backfill non-null label, rename
-- name->key/description->label, restore archived_at, add workspace_id, clone
-- the 4 global purposes into every existing workspace, remap consumer FKs
-- (append-only trigger disabled for the narrowest possible window), drop the
-- now-unreferenced global rows, then enforce NOT NULL + the per-workspace
-- UNIQUE + RLS.
UPDATE consent_purpose SET description = name WHERE description IS NULL;
ALTER TABLE consent_purpose RENAME COLUMN name TO key;
ALTER TABLE consent_purpose RENAME COLUMN description TO label;
ALTER TABLE consent_purpose ALTER COLUMN label SET NOT NULL;
ALTER TABLE consent_purpose ADD COLUMN archived_at timestamptz NULL;

ALTER TABLE consent_purpose ADD COLUMN workspace_id uuid NULL REFERENCES workspace(id) ON DELETE RESTRICT;

-- The old UNIQUE(key)-alone constraint must go before cloning below: with 2+
-- pre-existing workspaces, the cross-join clone below inserts the same key
-- more than once (once per workspace), which the old single-column constraint
-- would reject. The replacement UNIQUE(workspace_id, key) is added once the
-- clone + remap + delete steps below are done and workspace_id is NOT NULL.
ALTER TABLE consent_purpose DROP CONSTRAINT consent_purpose_name_key;

INSERT INTO consent_purpose (workspace_id, key, label)
SELECT w.id, cp.key, cp.label
FROM workspace w
CROSS JOIN consent_purpose cp
WHERE cp.workspace_id IS NULL;

-- Remap person_consent + consent_event to the new workspace-scoped rows before
-- the global rows are deleted below. consent_event is append-only
-- (trg_consent_event_no_mutate, 000022_consent.up.sql:79-81), so its remap
-- UPDATE runs with that trigger disabled — the narrowest possible window, same
-- transaction, re-enabled immediately after.
UPDATE person_consent pc
SET purpose_id = newcp.id
FROM consent_purpose oldcp, consent_purpose newcp
WHERE pc.purpose_id = oldcp.id
  AND oldcp.workspace_id IS NULL
  AND newcp.workspace_id = pc.workspace_id
  AND newcp.key = oldcp.key;

ALTER TABLE consent_event DISABLE TRIGGER trg_consent_event_no_mutate;
UPDATE consent_event ce
SET purpose_id = newcp.id
FROM consent_purpose oldcp, consent_purpose newcp
WHERE ce.purpose_id = oldcp.id
  AND oldcp.workspace_id IS NULL
  AND newcp.workspace_id = ce.workspace_id
  AND newcp.key = oldcp.key;
ALTER TABLE consent_event ENABLE TRIGGER trg_consent_event_no_mutate;

DELETE FROM consent_purpose WHERE workspace_id IS NULL;
ALTER TABLE consent_purpose ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE consent_purpose ADD CONSTRAINT uq_consent_purpose_workspace_key UNIQUE (workspace_id, key);

-- Standalone FK-column index for the new workspace_id FK (see 000010 note above).
CREATE INDEX idx_consent_purpose_ws ON consent_purpose (workspace_id);

ALTER TABLE consent_purpose ENABLE ROW LEVEL SECURITY;
ALTER TABLE consent_purpose FORCE ROW LEVEL SECURITY;
CREATE POLICY consent_purpose_tenant_isolation ON consent_purpose
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- INSERT is new (a workspace's own purposes must be writable at signup time —
-- D2 step 7); SELECT was already granted by the original 000022 migration.
GRANT SELECT, INSERT ON consent_purpose TO margince_app;

COMMIT;
