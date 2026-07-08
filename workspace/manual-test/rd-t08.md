# RD-T08 Live-Stack UAT — Formula-Field GENERATED Columns (RD-AC-6/RD-AC-7/RD-AC-N-1)

Verification of migration 000075 (`deal.amount_minor_base` GENERATED column +
`organization_open_pipeline_rollup` view) against RD-AC-6/RD-AC-7/RD-AC-N-1.

All steps are `[auto]` — literal commands with literal expected results. No `[manual]` step
exists in this backend/schema-boundary ticket.

---

## Step 1 [auto]: Apply migration; verify `deal.amount_minor_base` is a GENERATED column

```bash
make migrate-up
make migrate-status
psql "$DATABASE_URL" -c "\d deal" | grep amount_minor_base
psql "$DATABASE_URL" -c "SELECT column_name, is_generated, generation_expression FROM information_schema.columns WHERE table_name='deal' AND column_name='amount_minor_base';"
```

**Expected:**

- `make migrate-status` returns `75` (non-dirty).
- `\d deal | grep amount_minor_base` shows a line like:
  ```
   amount_minor_base   | bigint | ... generated always as (...) stored
  ```
- The `information_schema` query returns exactly one row:
  ```
   column_name       | is_generated | generation_expression
  -------------------+--------------+--------------------------------------------------------------
   amount_minor_base | ALWAYS       | (round(((amount_minor)::numeric * fx_rate_to_base)))::bigint
  ```
  `is_generated = ALWAYS` and `generation_expression` references both `amount_minor` and
  `fx_rate_to_base` — proof that this is a real database-GENERATED column (RD-AC-6), not an
  app-side interpreted expression.

---

## Step 2 [auto]: Verify computed value and NULL-propagation behavior

```bash
psql "$DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws_id uuid;
  pipe_id uuid;
  stage_id uuid;
  deal1_id uuid;
  deal2_id uuid;
  v bigint;
BEGIN
  INSERT INTO workspace(name, slug, base_currency) VALUES ('UAT-RDT08', 'uat-rdt08', 'USD') RETURNING id INTO ws_id;
  INSERT INTO pipeline(workspace_id, name, is_default) VALUES (ws_id, 'UATPipe', true) RETURNING id INTO pipe_id;
  INSERT INTO stage(workspace_id, pipeline_id, name, position, semantic) VALUES (ws_id, pipe_id, 'Open', 1, 'open') RETURNING id INTO stage_id;

  -- Both inputs present: amount_minor_base should be round(100000 * 1.1) = 110000.
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws_id, 'UAT-Deal1', pipe_id, stage_id, 100000, 'EUR', 1.1, 'open', 'api', 'human:uat') RETURNING id INTO deal1_id;
  SELECT amount_minor_base FROM deal WHERE id = deal1_id INTO v;
  RAISE NOTICE 'Deal1 amount_minor_base: % (expect 110000)', v;
  ASSERT v = 110000, 'FAIL: expected 110000, got ' || v::text;

  -- Missing fx_rate_to_base: amount_minor_base should be NULL.
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws_id, 'UAT-Deal2', pipe_id, stage_id, 50000, 'EUR', NULL, 'open', 'api', 'human:uat') RETURNING id INTO deal2_id;
  SELECT amount_minor_base FROM deal WHERE id = deal2_id INTO v;
  RAISE NOTICE 'Deal2 amount_minor_base (fx NULL): % (expect NULL)', v;
  ASSERT v IS NULL, 'FAIL: expected NULL, got ' || v::text;

  RAISE NOTICE 'Step 2 passed';
  RAISE EXCEPTION 'rollback' USING ERRCODE = '40001';
END;
$$;
EOSQL
```

**Expected:**

```
NOTICE:  Deal1 amount_minor_base: 110000 (expect 110000)
NOTICE:  Deal2 amount_minor_base (fx NULL): <NULL> (expect NULL)
NOTICE:  Step 2 passed
ERROR:  rollback
```

- Deal with both inputs: `amount_minor_base = 110000` (`round(100000 × 1.1)`).
- Deal with `fx_rate_to_base = NULL`: `amount_minor_base IS NULL` — the honest "not computable
  yet" state, not a fabricated default.
- The `ERROR: rollback` is the intentional transaction rollback to keep the DB clean.

---

## Step 3 [auto]: Verify `organization_open_pipeline_rollup` view behavior

```bash
psql "$DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws_id uuid;
  pipe_id uuid;
  stage_id uuid;
  org_a uuid;
  org_b uuid;
  org_c uuid;
  v_sum bigint;
  v_count bigint;
BEGIN
  INSERT INTO workspace(name, slug, base_currency) VALUES ('UAT-RDT08-View', 'uat-rdt08-view', 'USD') RETURNING id INTO ws_id;
  SET LOCAL app.workspace_id = '';  -- bypass RLS for UAT seeding
  INSERT INTO pipeline(workspace_id, name, is_default) VALUES (ws_id, 'UATPipe', true) RETURNING id INTO pipe_id;
  INSERT INTO stage(workspace_id, pipeline_id, name, position, semantic) VALUES (ws_id, pipe_id, 'Open', 1, 'open') RETURNING id INTO stage_id;

  -- Org A: one open deal, both inputs present -> correct aggregate.
  INSERT INTO organization(workspace_id, name, source, captured_by) VALUES (ws_id, 'UAT-OrgA', 'api', 'human:uat') RETURNING id INTO org_a;
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, organization_id, amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws_id, 'UAT-D1', pipe_id, stage_id, org_a, 200000, 'EUR', 1.0, 'open', 'api', 'human:uat');
  SELECT open_pipeline_minor_base, open_deal_count FROM organization_open_pipeline_rollup WHERE organization_id = org_a INTO v_sum, v_count;
  RAISE NOTICE 'Org A: sum=%, count=% (expect sum=200000, count=1)', v_sum, v_count;
  ASSERT v_sum = 200000 AND v_count = 1, 'FAIL org A';

  -- Org B: no deals at all -> no row (never a fabricated zero).
  INSERT INTO organization(workspace_id, name, source, captured_by) VALUES (ws_id, 'UAT-OrgB', 'api', 'human:uat') RETURNING id INTO org_b;
  SELECT count(*) FROM organization_open_pipeline_rollup WHERE organization_id = org_b INTO v_count;
  RAISE NOTICE 'Org B: row count=% (expect 0 — no row at all)', v_count;
  ASSERT v_count = 0, 'FAIL org B: expected no row';

  -- Org C: open deal with fx_rate_to_base NULL -> row exists (open_deal_count=1), sum NULL.
  INSERT INTO organization(workspace_id, name, source, captured_by) VALUES (ws_id, 'UAT-OrgC', 'api', 'human:uat') RETURNING id INTO org_c;
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, organization_id, amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws_id, 'UAT-D2', pipe_id, stage_id, org_c, 50000, 'EUR', NULL, 'open', 'api', 'human:uat');
  SELECT open_pipeline_minor_base, open_deal_count FROM organization_open_pipeline_rollup WHERE organization_id = org_c INTO v_sum, v_count;
  RAISE NOTICE 'Org C: sum=%, count=% (expect sum=NULL, count=1)', v_sum, v_count;
  ASSERT v_sum IS NULL AND v_count = 1, 'FAIL org C';

  RAISE NOTICE 'Step 3 passed';
  RAISE EXCEPTION 'rollback' USING ERRCODE = '40001';
END;
$$;
EOSQL
```

**Expected:**

```
NOTICE:  Org A: sum=200000, count=1 (expect sum=200000, count=1)
NOTICE:  Org B: row count=0 (expect 0 — no row at all)
NOTICE:  Org C: sum=<NULL>, count=1 (expect sum=NULL, count=1)
NOTICE:  Step 3 passed
ERROR:  rollback
```

- Org A: correct aggregate (`open_pipeline_minor_base = 200000`, `open_deal_count = 1`).
- Org B: no row returned — not a zero-valued row. The view omits orgs with no open deals.
- Org C: row present (`open_deal_count = 1`), but `open_pipeline_minor_base IS NULL` —
  the "not computable yet" state, distinct from Org B's genuine no-row state.

---

## Step 4 [auto]: Run automated tests

```bash
make test-it DIR=backend/internal/modules/records
(cd backend && go test ./internal/modules/records/...)
```

**Expected:**

- `make test-it DIR=backend/internal/modules/records` output ends with:
  ```
  --- PASS: TestDealAmountMinorBaseIsGenerated (...)
  --- PASS: TestOrgOpenPipelineRollupIsARealSQLView (...)
  --- PASS: TestDealAmountMinorBase_Values (...)
  --- PASS: TestOrgOpenPipelineRollup_Bound (...)
  PASS
  ok  github.com/gradionhq/margince/backend/internal/modules/records
  ```
- `(cd backend && go test ./internal/modules/records/...)` output ends with:
  ```
  --- PASS: TestRDAC7_NoFormulaEvalDependencyInGoMod (...)
  --- PASS: TestRDAC7_NoFormulaEvalImportAnywhereInBackend (...)
  --- PASS: TestRDAC7_NoFormulaAuthoringContractOperation (...)
  PASS
  ok  github.com/gradionhq/margince/backend/internal/modules/records
  ```

All 7 tests pass: 4 integration-tagged (schema + bound) and 3 unit/static (negative-scope).

---

## Step 5 [auto]: Round-trip reversibility

```bash
make migrate-down
make migrate-status
make migrate-up
make migrate-status
```

**Expected:**

- First `make migrate-status` returns `74` (non-dirty) — migration 000075 rolled back cleanly;
  `organization_open_pipeline_rollup` dropped, `deal.amount_minor_base` column removed.
- Second `make migrate-status` returns `75` (non-dirty) — migration re-applied cleanly.
- No dirty state at any point. Verifies the down migration reverses the up migration completely
  (RD-AC-6's migration round-trip acceptance criterion).

---

## Step 6 [auto]: Full project gate

```bash
make check-q
```

**Expected:**

```
OK: check-q passed
```

All linters, vet, format checks, generated-types checks, contract-lint, module-shape tests
(`modules_shape_test.go` accepts the test-only `records` package), and static unit tests pass.
No frontend changes, no `crm.yaml` changes — `be-only` scope confirmed.
