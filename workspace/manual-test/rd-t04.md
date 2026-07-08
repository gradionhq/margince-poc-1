# RD-T04 Live-Stack UAT — Hierarchy Roll-Up Read (RD-FORM-1)

Verification of `GET /organizations/{id}/hierarchy-rollup` against RD-FORM-1, RD-AC-1,
RD-AC-8, RD-PARAM-1/2, and PO-AC-28. Steps 1, 9, 10, 11 are `[auto]` (seed script / `go test` /
`make` invocations). Steps 2–8 are `[live]` — real `curl` calls against the booted server
(`make run` / `make uat_env`, default `http://localhost:8080`), using this app's dev-mode
`X-Workspace-ID` / `X-User-ID` header auth (`backend/cmd/api/routes.go`'s `workspaceWrap`
fallback — see `workspace/manual-test/t05.md` for the same header style). Every step has a
literal command and a literal `Expected:`.

Synthesized from: four `(verify: integration)` acceptance criteria in `workspace/specs/rd-t04.md`
+ Task 3 description in the implementation plan (see "Spec gap" note in the plan header).

Variable mapping (captured from Step 1's `RAISE NOTICE` output): `$WS`=ws, `$ROOT`=root,
`$CHILD_A`=childA, `$CHILD_B`=childB, `$VIEWER`=viewer, `$OTHER`=other. Steps 4–8 seed a few
additional fixtures (record_grant, an FX-gap org, a second workspace, a 3-level decomposition
tree, a no-permission user) immediately before the step that needs them, so the whole guide is
self-contained and runnable top-to-bottom against a fresh workspace.

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

## Step 2 [live]: scope=tree (default) — formula holds, restricted child disclosed

Replace `$ROOT`, `$VIEWER`, `$WS` with the values captured from Step 1.

```bash
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$ROOT/hierarchy-rollup" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. Body (scope omitted defaults to `"tree"`):
- `root_id = "$ROOT"`, `scope = "tree"`
- `weighted_pipeline = {"amount_minor": 1000000, "currency": "EUR"}` — root's own USD 1,000,000
  minor deal converted at the seeded `USD→EUR` rate of 1.0 and weighted at the stage's 100%
  win-probability (`1000000 × 1.0 × 100% = 1000000`); childA's 500000 is excluded because
  childA is restricted (owned by `$OTHER`, viewer's `row_scope=own` can't read it, no grant
  exists yet); childB has no deals so contributes 0.
- `closed_won = {"amount_minor": 0, "currency": "EUR"}` (no `won` deals seeded)
- `activity_count_30d = 1` (root's activity only; childA's activity is excluded along with the
  restricted node; childB has none)
- `aggregated_account_count = 2` (root + childB; childA is restricted and not counted)
- `restricted_excluded = [{"id": "$CHILD_A", "display_name": "UAT-ChildA"}]`
- `computed_at` is a non-null RFC3339 timestamp

RD-FORM-1 formula check (cross-check against Step 3's output): `tree.weighted_pipeline (1000000)
== root.self.weighted_pipeline (1000000) + childB.self.weighted_pipeline (0)` — holds.

---

## Step 3 [live]: scope=self — only root's own figures, aggregated_account_count=1

```bash
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$ROOT/hierarchy-rollup?scope=self" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. Body:
- `scope = "self"`
- `aggregated_account_count = 1`
- `weighted_pipeline = {"amount_minor": 1000000, "currency": "EUR"}` (root's own deal only)
- `closed_won = {"amount_minor": 0, "currency": "EUR"}`
- `activity_count_30d = 1`
- `restricted_excluded = []` (self-scope never exposes subtree restriction)

---

## Step 4 [live]: record_grant override — restricted child flips into the included set

First, grant `$VIEWER` read access on `$CHILD_A` (there is no `record_grant` seeded by Step 1):

```bash
psql "$TEST_DATABASE_URL" -c "
INSERT INTO record_grant (workspace_id, record_type, record_id, subject_type, subject_id, access, granted_by)
VALUES ('$WS', 'organization', '$CHILD_A', 'user', '$VIEWER', 'read', '$VIEWER');
"
```

**Expected:** `INSERT 0 1`.

Then re-request the tree-scope roll-up:

```bash
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$ROOT/hierarchy-rollup" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. Body:
- `restricted_excluded = []` (childA is no longer restricted)
- `weighted_pipeline = {"amount_minor": 1500000, "currency": "EUR"}` — root's 1000000 + childA's
  now-included 500000 (childB still contributes 0)
- `closed_won = {"amount_minor": 0, "currency": "EUR"}`
- `activity_count_30d = 2` (root's + childA's activity, now both readable)
- `aggregated_account_count = 3` (root + childA + childB)

---

## Step 5 [live]: missing FX rate → 422 fx_rate_unavailable

Step 1 only seeded a `USD→EUR` `fx_rate` row, so an open deal in a currency with no stored rate
(e.g. GBP) triggers the 422 path. Seed a standalone org + GBP deal, isolated from `$ROOT`'s tree
so it doesn't perturb Steps 2–4's totals:

```bash
psql "$TEST_DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws     uuid := '$WS';
  viewer uuid := '$VIEWER';
  p      uuid;
  s      uuid;
  fxorg  uuid;
BEGIN
  SELECT id INTO p FROM pipeline WHERE workspace_id = ws LIMIT 1;
  SELECT id INTO s FROM stage    WHERE workspace_id = ws LIMIT 1;

  INSERT INTO organization(workspace_id, name, owner_id, source, captured_by)
    VALUES (ws, 'UAT-FXGap', viewer, 'api', 'human:uat')
    RETURNING id INTO fxorg;

  -- GBP open deal — no fx_rate row exists for GBP->EUR.
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, organization_id,
                   amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws, 'UAT-FXGapDeal', p, s, fxorg, 300000, 'GBP', '1.0', 'open', 'api', 'human:uat');

  RAISE NOTICE 'fxgap_org=%', fxorg;
END$$;
EOSQL
```

**Expected:** The block exits cleanly; `RAISE NOTICE` prints `fxgap_org=<uuid>`. Record it as
`$FXGAP_ORG`.

```bash
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$FXGAP_ORG/hierarchy-rollup" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 422`. Body:
- `code = "fx_rate_unavailable"`
- `details.currency = "GBP"`
- `details.as_of` equals today's UTC date in `YYYY-MM-DD` format (i.e. `$(date -u +%F)` — the
  handler computes the FX lookup `asOf` as `time.Now().UTC()` at request time, not at seed time)

---

## Step 6 [live]: nonexistent / out-of-workspace org id → 404

Seed a second workspace with one org in it, isolated from `$WS`:

```bash
psql "$TEST_DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws2  uuid;
  org2 uuid;
BEGIN
  INSERT INTO workspace(name, slug, base_currency)
    VALUES ('UAT-RDT04-WS2', 'uat-rdt04-ws2', 'EUR')
    RETURNING id INTO ws2;

  INSERT INTO organization(workspace_id, name, source, captured_by)
    VALUES (ws2, 'UAT-WS2-Org', 'api', 'human:uat')
    RETURNING id INTO org2;

  RAISE NOTICE 'ws2_org=%', org2;
END$$;
EOSQL
```

**Expected:** The block exits cleanly; `RAISE NOTICE` prints `ws2_org=<uuid>`. Record it as
`$WS2_ORG`.

```bash
# Nonexistent UUID under workspace $WS:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/00000000-0000-0000-0000-000000000000/hierarchy-rollup" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"

# Org that exists, but in a different workspace than the one in the request header:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$WS2_ORG/hierarchy-rollup" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** Both requests return `HTTP 404` with `body.code = "not_found"`. The second request
404s because the roll-up CTE's `WHERE workspace_id = $2` filter (bound to `$WS`, the header's
workspace) excludes `$WS2_ORG`, which belongs to `ws2` — never leaking cross-workspace existence.

---

## Step 7 [live]: RD-AC-8 decomposition — tree total = self + Σ children

Seed a fresh 3-level tree (root → childA → grandchild), all owned by `$VIEWER` so every node is
readable without needing another grant, isolated from `$ROOT`'s tree so it doesn't perturb
Steps 2–4's totals:

```bash
psql "$TEST_DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws     uuid := '$WS';
  viewer uuid := '$VIEWER';
  p      uuid;
  s      uuid;
  dr     uuid;
  dca    uuid;
  dgc    uuid;
BEGIN
  SELECT id INTO p FROM pipeline WHERE workspace_id = ws LIMIT 1;
  SELECT id INTO s FROM stage    WHERE workspace_id = ws LIMIT 1;

  INSERT INTO organization(workspace_id, name, owner_id, source, captured_by)
    VALUES (ws, 'UAT-DecompRoot', viewer, 'api', 'human:uat')
    RETURNING id INTO dr;
  INSERT INTO organization(workspace_id, name, parent_org_id, owner_id, source, captured_by)
    VALUES (ws, 'UAT-DecompChildA', dr, viewer, 'api', 'human:uat')
    RETURNING id INTO dca;
  INSERT INTO organization(workspace_id, name, parent_org_id, owner_id, source, captured_by)
    VALUES (ws, 'UAT-DecompGrandchild', dca, viewer, 'api', 'human:uat')
    RETURNING id INTO dgc;

  -- win_probability=100 (same stage as Step 1) and USD->EUR fx=1.0 (seeded in Step 1), so
  -- weighted_pipeline == amount_minor exactly, mirroring hierarchy_rollup_http_test.go's
  -- TestHierarchyRollupHTTP_Decomposition fixture (3000 / 5000 / 7000).
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, organization_id,
                   amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws, 'UAT-DecompRootDeal', p, s, dr, 3000, 'USD', '1.0', 'open', 'api', 'human:uat');
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, organization_id,
                   amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws, 'UAT-DecompChildADeal', p, s, dca, 5000, 'USD', '1.0', 'open', 'api', 'human:uat');
  INSERT INTO deal(workspace_id, name, pipeline_id, stage_id, organization_id,
                   amount_minor, currency, fx_rate_to_base, status, source, captured_by)
    VALUES (ws, 'UAT-DecompGrandchildDeal', p, s, dgc, 7000, 'USD', '1.0', 'open', 'api', 'human:uat');

  INSERT INTO activity(workspace_id, kind, subject, occurred_at, source, captured_by)
    VALUES (ws, 'note', 'UAT note', now() - interval '1 day', 'api', 'human:uat') RETURNING id INTO p;
  INSERT INTO activity_link(workspace_id, activity_id, entity_type, organization_id) VALUES (ws, p, 'organization', dr);
  INSERT INTO activity(workspace_id, kind, subject, occurred_at, source, captured_by)
    VALUES (ws, 'note', 'UAT note', now() - interval '2 days', 'api', 'human:uat') RETURNING id INTO p;
  INSERT INTO activity_link(workspace_id, activity_id, entity_type, organization_id) VALUES (ws, p, 'organization', dca);
  INSERT INTO activity(workspace_id, kind, subject, occurred_at, source, captured_by)
    VALUES (ws, 'note', 'UAT note', now() - interval '3 days', 'api', 'human:uat') RETURNING id INTO p;
  INSERT INTO activity_link(workspace_id, activity_id, entity_type, organization_id) VALUES (ws, p, 'organization', dgc);

  RAISE NOTICE 'decomp_root=%   decomp_childA=%   decomp_grandchild=%', dr, dca, dgc;
END$$;
EOSQL
```

**Expected:** The block exits cleanly; `RAISE NOTICE` prints the three UUIDs. Record them as
`$DECOMP_ROOT`, `$DECOMP_CHILD_A`, `$DECOMP_GRANDCHILD`.

```bash
# root_tree (includes childA + grandchild):
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$DECOMP_ROOT/hierarchy-rollup" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"

# root_self:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$DECOMP_ROOT/hierarchy-rollup?scope=self" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"

# childA_tree (includes grandchild):
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$DECOMP_CHILD_A/hierarchy-rollup" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** All three return `HTTP 200`.
- `root_self.weighted_pipeline.amount_minor = 3000`, `root_self.activity_count_30d = 1`
- `childA_tree.weighted_pipeline.amount_minor = 12000` (5000 + 7000), `childA_tree.activity_count_30d = 2`
- `root_tree.weighted_pipeline.amount_minor = 15000` (3000 + 5000 + 7000), matching
  `root_self.weighted_pipeline (3000) + childA_tree.weighted_pipeline (12000) = 15000` —
  RD-AC-8's decomposition claim holds
- `root_tree.activity_count_30d = 3`, matching `root_self (1) + childA_tree (2) = 3`
- `root_tree.aggregated_account_count = 3` (root + childA + grandchild)

---

## Step 8 [live]: 401 / 403 auth gates fire for the hierarchy-rollup sub-path

Seed a user whose role has no `organization` permissions at all (only `deal.read`), to exercise
the 403 path:

```bash
psql "$TEST_DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws          uuid := '$WS';
  noperm_user uuid;
  noperm_role uuid;
BEGIN
  INSERT INTO app_user(workspace_id, email, display_name)
    VALUES (ws, 'noperm@uat-rdt04.test', 'NoPerm')
    RETURNING id INTO noperm_user;
  INSERT INTO role(workspace_id, key, is_system, permissions)
    VALUES (ws, 'uat-noperm', false, '{"deal":{"read":{"row_scope":"all"}}}'::jsonb)
    RETURNING id INTO noperm_role;
  INSERT INTO role_assignment(workspace_id, role_id, user_id) VALUES (ws, noperm_role, noperm_user);

  RAISE NOTICE 'noperm=%', noperm_user;
END$$;
EOSQL
```

**Expected:** The block exits cleanly; `RAISE NOTICE` prints `noperm=<uuid>`. Record it as
`$NOPERM`.

```bash
# 401: no X-Workspace-ID / X-User-ID header at all — no principal in context.
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$ROOT/hierarchy-rollup"

# 403: a real principal, but its role has no organization.read permission.
curl -s -w '\n%{http_code}\n' "http://localhost:8080/organizations/$ROOT/hierarchy-rollup" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $NOPERM"
```

**Expected:**
- First request: `HTTP 401` (`RequireAuth` fires before any permission check runs).
- Second request: `HTTP 403` (`RbacMiddleware`'s `session.AuthorizePerms(perms, "organization",
  "read")` denies — `$NOPERM`'s role only grants `deal.read`).

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
