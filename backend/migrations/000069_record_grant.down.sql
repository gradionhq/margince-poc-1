-- 000069 down — restore the four tables' original tenant-isolation-only policy, then drop
-- record_grant.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['deal','person','organization','lead'] LOOP
    EXECUTE format('DROP POLICY IF EXISTS %1$s_tenant_isolation ON %1$I;', t);
    EXECUTE format($f$
      CREATE POLICY %1$s_tenant_isolation ON %1$I
        USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
        WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
    $f$, t);
  END LOOP;
END $$;

DROP TABLE IF EXISTS record_grant;
