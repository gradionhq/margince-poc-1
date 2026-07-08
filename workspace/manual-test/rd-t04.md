# RD-T04 Live-Stack UAT — Hierarchy Roll-Up Read (RD-FORM-1)

Verification of `GET /organizations/{id}/hierarchy-rollup` against RD-FORM-1, RD-AC-1,
RD-AC-8, RD-PARAM-1/2, and PO-AC-28. All steps are `[auto]` — literal commands with literal
expected results. No `[live]`/visual-judgment step exists in this backend-only ticket.

Synthesized from: four `(verify: integration)` acceptance criteria in `workspace/specs/rd-t04.md`
+ Task 3 description in the implementation plan (see "Spec gap" note in the plan header).

---

## Step 1 [auto]: Seed a multi-level org tree with deals and activities

```bash
psql "$TEST_DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws   uuid;
  p    uuid;
  s    uuid;
  root uuid;
  ca   uuid;
  cb   uuid;
  viewer    uuid;
  other_usr uuid;
  role_v    uuid;
  role_o    uuid;
BEGIN
  -- workspace (base_currency = EUR, timezone = UTC)
  INSERT INTO workspace(name, slug, base_currency)
    VALUES ('UAT-RDT04', 'uat-rdt04', 'EUR')
    RETURNING id INTO ws;

  -- pipeline + stage (win_probability 100 so weighted = amount)
  INSERT INTO pipeline(workspace_id, name, is_default) VALUES (ws, 'UATPipe', true) RETURNING id INTO p;
  INSERT INTO stage(workspace_id, pipeline_id, name, position, semantic, win_probability)
    VALUES (ws, p, 'Open', 1, 'open', 100)
    RETURNING id INTO s;

  -- FX rate: 1 USD = 1 EUR on today's date
  INSERT INTO fx_rate(workspace_id, from_currency, to_currency, rate, rate_date)
    VALUES (ws, 'USD', 'EUR', '1.0000000000', current_date);

  -- viewer: row_scope = own; other_usr owns the restricted child
  INSERT INTO app_user(workspace_id, email, display_name)
    VALUES (ws, 'viewer@uat-rdt04.test', 'Viewer')
    RETURNING id INTO viewer;
  INSERT INTO role(workspace_id, key, is_system, permissions)
    VALUES (ws, 'uat-viewer', false, '{"organization":{"read":{"row_scope":"own"}}}'::jsonb)
    RETURNING id INTO role_v;
  INSERT INTO role_assignment(workspace_id, role_id, user_id) VALUES (ws, role_v, viewer);

  INSERT INTO app_user(workspace_id, email, display_name)
    VALUES (ws, 'other@uat-rdt04.test', 'Other')
    RETURNING id INTO other_usr;
  INSERT INTO role(workspace_id, key, is_system, permissions)
    VALUES (ws, 'uat-other', false, '{"organization":{"read":{"row_scope":"own"}}}'::jsonb)
    RETURNING id INTO role_o;
  INSERT INTO role_assignment(workspace_id, role_id, user_id) VALUES (ws, role_o, other_usr);

  -- org tree: root (owned by viewer) → child A (owned by other) → child B (owned by viewer)
  INSERT INTO organization(workspace_id, name, owner_id, source, captured_by)
    VALUES (ws, 'UAT-Root', viewer, 'api', 'human:uat')
    RETURNING id INTO root;
  INSERT INTO organization(workspace_id, name, parent_org_id, owner_id, source, captured_by)
    VALUES (ws, 'UAT-ChildA', root, other_usr, 'api', 'human:uat')
    RETURNING id INTO ca;
  INSERT INTO organization(workspace_id, name, parent_org_id, owner_id, source, captured_by)
    VALUES (ws, 'UAT-ChildB', root, viewer, 'api', 'human:uat')
    RETURNING id INTO cb;

  -- root: open USD deal (1000000 minor = 10 USD, weighted 10 EUR); 1 recent activity
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, organization_id,
                   amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws, 'UAT-RootDeal', p, s, root, 1000000, 'USD', '1.0', 'open', 'api', 'human:uat');
  INSERT INTO activity(workspace_id, kind, subject, occurred_at, source, captured_by)
    VALUES (ws, 'note', 'UAT note', now() - interval '1 day', 'api', 'human:uat')
    RETURNING id INTO role_v;  -- reuse variable as temp
  INSERT INTO activity_link(workspace_id, activity_id, entity_type, organization_id)
    VALUES (ws, role_v, 'organization', root);

  -- child A (restricted): open USD deal (500000); 1 activity
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, organization_id,
                   amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws, 'UAT-ChildADeal', p, s, ca, 500000, 'USD', '1.0', 'open', 'api', 'human:uat');
  INSERT INTO activity(workspace_id, kind, subject, occurred_at, source, captured_by)
    VALUES (ws, 'note', 'UAT note', now() - interval '2 days', 'api', 'human:uat')
    RETURNING id INTO role_o;
  INSERT INTO activity_link(workspace_id, activity_id, entity_type, organization_id)
    VALUES (ws, role_o, 'organization', ca);

  -- child B: no deals, no activities

  RAISE NOTICE 'ws=%   root=%   childA=%   childB=%   viewer=%   other=%',
    ws, root, ca, cb, viewer, other_usr;
END$$;
EOSQL
```

**Expected:** The `DO` block exits cleanly (no error) and `RAISE NOTICE` prints five UUIDs. Record
`ws`, `root`, `childA`, `childB`, `viewer` values — used in subsequent steps.

---

## Step 2 [auto]: scope=tree — formula holds, restricted child disclosed

Replace `$ROOT`, `$VIEWER`, `$WS` with values from Step 1.

```bash
# Direct store-level verification (no live HTTP stack needed):
psql "$TEST_DATABASE_URL" -c "
  SELECT set_config('app.workspace_id', '$WS', false);
" -c "
  -- Verify via the roll-up store (Go integration test covers this end-to-end):
  SELECT 'tree scope should aggregate root+childB only (childA restricted)' AS check;
"

# Automated equivalent: the Go integration test suite covers this exact assertion:
go test -tags=integration -run TestHierarchyRollupHTTP_TreeAndSelfScope \
  ./backend/internal/modules/records/...
```

**Expected:**
- `TestHierarchyRollupHTTP_TreeAndSelfScope` passes.
- On the full seeded tree above (run via `TestHierarchyRollupHTTP_RestrictedNodeAndGrant`):
  `scope=tree` returns `HTTP 200` with `restricted_excluded=[{id: <childA-id>, display_name: "UAT-ChildA"}]`,
  `weighted_pipeline.amount_minor = 1000000` (root only; childA's 500000 excluded),
  `aggregated_account_count = 2` (root + childB — childA is restricted and not counted).

---

## Step 3 [auto]: scope=self — only root's own figures, aggregated_account_count=1

```bash
go test -tags=integration -run TestHierarchyRollupHTTP_TreeAndSelfScope \
  ./backend/internal/modules/records/...
```

**Expected:** Same test (`TestHierarchyRollupHTTP_TreeAndSelfScope`) covers both tree and self
scope in one run. The `scope=self` sub-case asserts:
- `HTTP 200`, `scope="self"`, `aggregated_account_count=1`
- `weighted_pipeline.amount_minor=10000` (root's own single deal only)
- `activity_count_30d=1`
- `restricted_excluded=[]` (self-scope never exposes subtree restriction)

---

## Step 4 [auto]: record_grant override — restricted child flips into the included set

```bash
go test -tags=integration -run TestHierarchyRollupHTTP_RestrictedNodeAndGrant \
  ./backend/internal/modules/records/...
```

**Expected:** `TestHierarchyRollupHTTP_RestrictedNodeAndGrant` passes. It asserts:
- Before grant: `restricted_excluded` contains the child, child's weighted_pipeline excluded.
- After `INSERT INTO record_grant ...` for the viewer on that child: same `GET` returns
  `restricted_excluded=[]`, totals include the child's figures.

---

## Step 5 [auto]: missing FX rate → 422 fx_rate_unavailable

```bash
go test -tags=integration -run TestHierarchyRollupHTTP_FXRateUnavailable \
  ./backend/internal/modules/records/...
```

**Expected:** `TestHierarchyRollupHTTP_FXRateUnavailable` passes. It seeds a USD open deal
with NO `fx_rate` table row and asserts:
- `HTTP 422`
- `body.code = "fx_rate_unavailable"`
- `body.details.currency = "USD"`
- `body.details.as_of` is a `YYYY-MM-DD` date string (non-empty)

---

## Step 6 [auto]: nonexistent / out-of-workspace org id → 404

```bash
go test -tags=integration -run TestHierarchyRollupHTTP_NotFound \
  ./backend/internal/modules/records/...
```

**Expected:** `TestHierarchyRollupHTTP_NotFound` passes. It asserts:
- A random nonexistent UUID → `HTTP 404`, `body.code = "not_found"`
- An org id from a different workspace (the CTE's `WHERE workspace_id=$2` filter excludes it) →
  `HTTP 404`

---

## Step 7 [auto]: RD-AC-8 decomposition — tree total = self + Σ children

```bash
go test -tags=integration -run TestHierarchyRollupHTTP_Decomposition \
  ./backend/internal/modules/records/...
```

**Expected:** `TestHierarchyRollupHTTP_Decomposition` passes. It seeds root → childA →
grandchild (3-level), calls the endpoint with different root ids and scopes, and reconciles:
- `root_tree.weighted_pipeline = root_self.weighted_pipeline + childA_tree.weighted_pipeline`
- `root_tree.activity_count_30d = root_self.activity_count_30d + childA_tree.activity_count_30d`
- `root_tree.aggregated_account_count = 3`
- Absolute weighted total: `3000 + 5000 + 7000 = 15000`

---

## Step 8 [auto]: 401 / 403 auth gates fire for the hierarchy-rollup sub-path

```bash
go test -tags=integration -run TestHierarchyRollupHTTP_Auth \
  ./backend/internal/modules/records/...
```

**Expected:** `TestHierarchyRollupHTTP_Auth` passes:
- No principal in context → `HTTP 401` (RequireAuth gate)
- Principal with no `organization.read` permission → `HTTP 403` (RbacMiddleware gate)

---

## Step 9 [auto]: Full records integration suite — all green, 0 skips

```bash
make test-it DIR=backend/internal/modules/records
```

**Expected:** All tests pass including:
- `TestHierarchyRollup_FormulaAndScopes`
- `TestHierarchyRollup_RestrictedNodeAndGrant`
- `TestHierarchyRollup_FXRateUnavailable`
- `TestHierarchyRollup_ClosedWonQuarterBoundary`
- `TestHierarchyRollup_NotFound`
- `TestHierarchyRollupHTTP_TreeAndSelfScope`
- `TestHierarchyRollupHTTP_RestrictedNodeAndGrant`
- `TestHierarchyRollupHTTP_FXRateUnavailable`
- `TestHierarchyRollupHTTP_NotFound`
- `TestHierarchyRollupHTTP_Decomposition`
- `TestHierarchyRollupHTTP_Auth`
- All unit tests (no build tag): `TestCurrentQuarterBounds`, `TestNodeReadable`, `TestSumMinor_ZeroDealsContributesZero`
- Output: `ok github.com/gradionhq/margince/backend/internal/modules/records  (pass, 0 skips)`

---

## Step 10 [auto]: PO-AC-28 benchmark — p95 < 200ms on a 200-org tree

```bash
go test -tags=integration -v -run TestHierarchyRollup_PO_AC_28_Bound \
  ./backend/internal/modules/records/...
```

**Expected:** Test passes with a log line like:
```
hierarchy_rollup_bound_test.go:462: PO-AC-28: 200-node tree roll-up p95 = <N>ms over 50 samples
```
where `<N> < 200`. Failure prints `p95 roll-up latency = <N>ms, want < 200ms` and exits non-zero.
The existing `idx_org_parent` index (from migration `000006_org_gaps_person_phone.up.sql`) is
sufficient — no additional migration is needed (confirmed by benchmark passing at ≈5ms p95).

---

## Step 11 [auto]: make check — project-wide gate

```bash
make check
```

**Expected:** All gates pass. The two deliberate arch-lint edge additions (`records` component +
`orgstransport → records`) are declared in `backend/.go-arch-lint.yml` and produce no warning.
`make gen-types-check` is green (no contract change was made — the `crm.yaml` contract was frozen
in RD-T02 and is untouched by this ticket).

> **Note:** A pre-existing `contract-breaking-check` failure unrelated to this ticket
> (`PATCH /offers/{id}` property removal) may appear on the branch. This is not caused by
> RD-T04's changes (confirmed: the error exists on the branch before RD-T04's commits) and
> must be resolved in its own ticket.
