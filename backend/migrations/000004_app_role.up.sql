-- 000004 — non-superuser application role (data-model §1.3).
-- RLS is bypassed by superusers and (without FORCE) by the table owner, so the
-- app/agent connection MUST run as a non-superuser, non-BYPASSRLS role. The Go
-- pool connects as (or SET ROLEs to) margince_app; the migration/admin role
-- (the owner) stays separate.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'margince_app') THEN
    CREATE ROLE margince_app NOLOGIN;  -- NOLOGIN: reached via SET ROLE (grant LOGIN + a secret for a dedicated connection in real deploys)
  END IF;
END $$;

GRANT USAGE ON SCHEMA public TO margince_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO margince_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO margince_app;

-- Cover tables/sequences created by later migrations (and `crm gen`), since
-- those run as the same owner role that executes this statement.
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO margince_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO margince_app;
