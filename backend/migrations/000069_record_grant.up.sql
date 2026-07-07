-- 000069 — record_grant (GH-209 WS-B, DM-DDL-5 verbatim): flat, audited per-record sharing
-- that widens the own/team/all base scope for one record. One table for all shareable
-- types (deal/person/organization/lead); no sharing hierarchies, criteria rules, or
-- grant-of-grant delegation (bounded per data-model.md's own DM-DDL-5 note).
--
-- The record_grant_tenant_isolation policy is the standard deny-on-unset pattern (mirrors
-- consumed_approval_token, migration 000064). The widening policies on deal/person/
-- organization/lead below are additive ORs on top of each table's existing
-- <table>_tenant_isolation policy — Postgres has no ALTER POLICY ... ADD condition, so each
-- is dropped and recreated with the widened USING clause; the WITH CHECK clause is left
-- unchanged (a grant never widens WRITE eligibility for the base row itself — only record_
-- grant's own row governs whether the grantee may read/write via the app layer, which is
-- enforced in Go, not by loosening the base table's insert/update check).
--
-- DEVIATION (documented, PLAN design deviation D3): the widened policies reference a new
-- app.user_id GUC that did not exist before this migration. It is set best-effort by
-- platform/database.SetWorkspaceScope/WithWorkspaceTx from ctx's crmctx.Principal (backend
-- code, landed in GH-209 Task 1) — a tx that never carries a principal simply never matches
-- the widened OR-branch (safe no-op, not a behavior change for those call sites).

CREATE TABLE record_grant (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  record_type   text NOT NULL CHECK (record_type IN ('deal','person','organization','lead')),
  record_id     uuid NOT NULL,
  subject_type  text NOT NULL CHECK (subject_type IN ('user','team')),
  subject_id    uuid NOT NULL,
  access        text NOT NULL CHECK (access IN ('read','write')),
  granted_by    uuid NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
  reason        text NULL,
  expires_at    timestamptz NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  version       bigint NOT NULL DEFAULT 1,
  CONSTRAINT record_grant_unique UNIQUE (workspace_id, record_type, record_id, subject_type, subject_id)
);
CREATE INDEX idx_record_grant_record  ON record_grant (workspace_id, record_type, record_id);
-- Note: the plan's partial predicate `WHERE expires_at IS NULL OR expires_at > now()` is
-- rejected by Postgres ("functions in index predicate must be marked IMMUTABLE" — now() is
-- STABLE, not IMMUTABLE); indexing all rows is functionally equivalent, just without that
-- optimization.
CREATE INDEX idx_record_grant_subject ON record_grant (workspace_id, subject_type, subject_id);
CREATE INDEX idx_record_grant_workspace ON record_grant (workspace_id);
CREATE INDEX idx_record_grant_granted_by ON record_grant (granted_by);

ALTER TABLE record_grant ENABLE ROW LEVEL SECURITY;
ALTER TABLE record_grant FORCE ROW LEVEL SECURITY;

CREATE POLICY record_grant_tenant_isolation ON record_grant
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON record_grant TO margince_app;

-- Widen the RLS backstop on the four shareable object tables. Each policy is dropped and
-- recreated with the identical WITH CHECK, plus an additive OR EXISTS(record_grant ...)
-- branch on USING.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['deal','person','organization','lead'] LOOP
    EXECUTE format('DROP POLICY IF EXISTS %1$s_tenant_isolation ON %1$I;', t);
    EXECUTE format($f$
      CREATE POLICY %1$s_tenant_isolation ON %1$I
        USING (
          workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid
          AND (
            true
            OR EXISTS (
              SELECT 1 FROM record_grant rg
              WHERE rg.workspace_id = %1$I.workspace_id
                AND rg.record_type = %2$L
                AND rg.record_id = %1$I.id
                AND (
                  (rg.subject_type = 'user' AND rg.subject_id = nullif(current_setting('app.user_id', true), '')::uuid)
                  OR (rg.subject_type = 'team' AND rg.subject_id IN (
                        SELECT team_id FROM team_membership
                        WHERE user_id = nullif(current_setting('app.user_id', true), '')::uuid
                          AND workspace_id = %1$I.workspace_id))
                )
                AND (rg.expires_at IS NULL OR rg.expires_at > now())
            )
          )
        )
        WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
    $f$, t, t);
  END LOOP;
END $$;
