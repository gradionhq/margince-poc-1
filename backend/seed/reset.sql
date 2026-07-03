-- Reset the dev workspace to a clean slate, then `make seed-dev` re-applies dev.sql.
-- Deletes every row scoped to the dev workspace across all tenant tables (those with a
-- workspace_id column), discovered dynamically so new tables are covered automatically.
--
-- session_replication_role = replica disables FK/triggers for the duration, so the
-- deletes are order-independent (no need to hand-maintain a dependency order). Requires
-- a superuser connection (the `margince` role used by `make psql`).
--
-- Scoped to the dev workspace only — workspaces bootstrapped via POST /workspaces are
-- left untouched.

BEGIN;

SET session_replication_role = replica;

DO $$
DECLARE
  t text;
  ws constant uuid := '00000000-0000-0000-0000-000000000001';
BEGIN
  FOR t IN
    SELECT c.table_name
    FROM information_schema.columns c
    JOIN information_schema.tables tb
      ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name
    WHERE c.table_schema = 'public'
      AND c.column_name = 'workspace_id'
      AND tb.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('DELETE FROM %I WHERE workspace_id = %L', t, ws);
  END LOOP;
END $$;

DELETE FROM workspace WHERE id = '00000000-0000-0000-0000-000000000001';

SET session_replication_role = origin;

COMMIT;
