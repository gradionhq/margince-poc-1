BEGIN;
-- 000075 — formula-field boundary proof (RD-T08, RD-AC-6/RD-AC-7/RD-AC-N-1): a formula field is
-- a database-GENERATED column, never a runtime-authored expression. Two concrete examples:
--
-- 1. Same-row GENERATED column (RD-AC-6): deal.amount_minor_base, a base-currency-converted
--    amount computed from the deal's own amount_minor x fx_rate_to_base (DM-FX-4: roll-ups must
--    aggregate the base-currency value, never raw native amount_minor across currencies).
--    GENERATED ALWAYS AS ... STORED — mirrors deal.search_tsv's existing GENERATED column
--    (000003_core_objects.up.sql) in style. A NULL input (amount_minor or fx_rate_to_base)
--    yields a NULL result: Postgres GENERATED columns evaluate the expression per row, and
--    ordinary NULL-propagating arithmetic already gives the "not-computable-yet" honest-null
--    state for free, no CASE needed. Open deals commonly have fx_rate_to_base still NULL (the
--    deal_closed_fx CHECK only requires it once a deal with an amount transitions off 'open'),
--    so this is a real, not a contrived, missing-input case.
--
-- 2. Cross-record aggregate (RD-AC-N-1's flagged reconciliation): a same-row GENERATED column
--    structurally cannot read other tables (Postgres limitation), so the per-organization open-
--    pipeline roll-up is served as a SQL view (security_invoker, inherits the base tables' RLS —
--    mirrors 000020_fts_search_coverage.up.sql's search_corpus view), reading only existing
--    columns (deal.amount_minor_base, deal.status, deal.organization_id, deal.archived_at) — no
--    new tables, never a runtime interpreter. An organization with no open deals produces no row
--    at all (never a fabricated zero); an organization with open deals whose amount_minor_base is
--    itself not yet computable (missing FX input) still produces a row, with the aggregate
--    column NULL (SUM ignores NULLs) — both are the honest "not computable yet" state, distinct
--    from a genuine zero-value roll-up.
ALTER TABLE deal ADD COLUMN amount_minor_base bigint
  GENERATED ALWAYS AS (round(amount_minor * fx_rate_to_base)::bigint) STORED;

CREATE OR REPLACE VIEW organization_open_pipeline_rollup
  WITH (security_invoker = true) AS
    SELECT
      d.organization_id,
      sum(d.amount_minor_base) AS open_pipeline_minor_base,
      count(*)                 AS open_deal_count
    FROM deal d
    WHERE d.status = 'open'
      AND d.organization_id IS NOT NULL
      AND d.archived_at IS NULL
    GROUP BY d.organization_id;
COMMIT;
