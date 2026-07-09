# RD-T06 Live-Stack UAT — Sales Quota CRUD + Attainment

Verification of `/quotas` and `/quotas/{id}/attainment` against RD-DDL-2, RD-FORM-2,
RD-AC-2/3/4/5/6, and the RBAC bootstrap-reality check. Steps 1, 9, 12, 13, 14 are `[auto]`
(seed script / `go test` / `make` invocations). Steps 2–8 and 10–11 are `[live]` — real `curl`
calls against the booted server (`make run` / `make uat_env`, default `http://localhost:8080`),
using dev-mode `X-Workspace-ID` / `X-User-ID` header auth (`backend/cmd/api/routes.go`'s
`workspaceWrap` fallback — same header style as `workspace/manual-test/rd-t04.md`). Every step
has a literal command and a literal `Expected:`.

Variable mapping (captured from Step 1's `RAISE NOTICE` output): `$WS`=ws, `$USER`=usr,
`$TEAM`=team, `$PIPE`=pipe, `$STAGE`=stage. Additional variables captured per-step are noted
inline.

---

## Step 1 [auto]: Seed workspace, user, team, pipeline + stage

```bash
psql "$TEST_DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws      uuid;
  usr     uuid;
  team    uuid;
  pipe    uuid;
  stg     uuid;
  role_id uuid;
BEGIN
  INSERT INTO workspace(name, slug, base_currency)
    VALUES ('UAT-RDT06', 'uat-rdt06', 'EUR')
    RETURNING id INTO ws;

  INSERT INTO app_user(workspace_id, email, display_name)
    VALUES (ws, 'admin@uat-rdt06.test', 'UAT Admin')
    RETURNING id INTO usr;

  -- Full quota CRUA role so dev-mode header auth passes RbacMiddleware on /quotas.
  INSERT INTO role(workspace_id, key, is_system, permissions)
    VALUES (ws, 'uat-admin', false,
      '{"quota":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},
         "update":{"row_scope":"all"},"archive":{"row_scope":"all"}}}'::jsonb)
    RETURNING id INTO role_id;
  INSERT INTO role_assignment(workspace_id, role_id, user_id)
    VALUES (ws, role_id, usr);

  INSERT INTO team(workspace_id, name)
    VALUES (ws, 'UAT-Team')
    RETURNING id INTO team;

  -- semantic='open' avoids the stage_terminal_prob check constraint.
  INSERT INTO pipeline(workspace_id, name, is_default)
    VALUES (ws, 'UAT-Pipe', true)
    RETURNING id INTO pipe;
  INSERT INTO stage(workspace_id, pipeline_id, name, position, semantic)
    VALUES (ws, pipe, 'Open', 1, 'open')
    RETURNING id INTO stg;

  RAISE NOTICE 'ws=%   usr=%   team=%   pipe=%   stage=%',
    ws, usr, team, pipe, stg;
END$$;
EOSQL
```

**Expected:** The `DO` block exits cleanly (no error) and `RAISE NOTICE` prints five UUIDs. Record
the values — they are referenced as `$WS`, `$USER`, `$TEAM`, `$PIPE`, `$STAGE` in all subsequent
steps.

---

## Step 2 [live]: POST /quotas owner-only → 201

Replace all `$VAR` placeholders with the values from Step 1.

```bash
curl -s -w '\n%{http_code}\n' -X POST "http://localhost:8080/quotas" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -H "Content-Type: application/json" \
  -d "{\"owner_id\":\"$USER\",\"period_start\":\"2025-01-01\",\"period_end\":\"2025-12-31\",\"target_minor\":28000000,\"currency\":\"EUR\"}"
```

**Expected:** `HTTP 201`. Body:
- `id` is a non-empty UUID — capture it as `$QID` for Steps 5–8.
- `workspace_id = "$WS"`, `owner_id = "$USER"`, `team_id` absent or null.
- `target_minor = 28000000`, `currency = "EUR"`, `version = 1`, `archived_at` null.
- `Location` response header: `/quotas/$QID`.

---

## Step 3 [live]: POST /quotas team-only → 201

```bash
curl -s -w '\n%{http_code}\n' -X POST "http://localhost:8080/quotas" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -H "Content-Type: application/json" \
  -d "{\"team_id\":\"$TEAM\",\"period_start\":\"2025-01-01\",\"period_end\":\"2025-12-31\",\"target_minor\":5000000,\"currency\":\"USD\"}"
```

**Expected:** `HTTP 201`. Body: `team_id = "$TEAM"`, `owner_id` absent or null, `target_minor =
5000000`. Capture `id` as `$QID_TEAM`.

---

## Step 4 [live]: POST /quotas validation — both-set and neither-set → 422

```bash
# both owner_id and team_id set:
curl -s -w '\n%{http_code}\n' -X POST "http://localhost:8080/quotas" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -H "Content-Type: application/json" \
  -d "{\"owner_id\":\"$USER\",\"team_id\":\"$TEAM\",\"period_start\":\"2025-01-01\",\"period_end\":\"2025-12-31\",\"target_minor\":1,\"currency\":\"EUR\"}"

# neither owner_id nor team_id:
curl -s -w '\n%{http_code}\n' -X POST "http://localhost:8080/quotas" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -H "Content-Type: application/json" \
  -d '{"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":1,"currency":"EUR"}'
```

**Expected:** Both return `HTTP 422`. Body (RD-AC-5 field-error shape):
- `details.errors[0].field = "owner_id"`, `details.errors[0].code = "owner_xor_team_required"`.

---

## Step 5 [live]: GET /quotas — list and owner/team filters

```bash
# default list (includes $QID and $QID_TEAM):
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"

# filter by owner_id:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas?owner_id=$USER" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"

# filter by team_id:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas?team_id=$TEAM" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"
```

**Expected:**
- Default list: `HTTP 200`; `data` has 2 items; `page.has_more = false`.
- `?owner_id=$USER`: `HTTP 200`; `data` has 1 item (`$QID`); `team_id` null.
- `?team_id=$TEAM`: `HTTP 200`; `data` has 1 item (`$QID_TEAM`); `owner_id` null.

---

## Step 6 [live]: GET /quotas/{id} — 200 round-trip and 404

```bash
# 200 round-trip:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas/$QID" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"

# 404 for nonexistent id:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas/00000000-0000-0000-0000-000000000000" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"
```

**Expected:**
- First: `HTTP 200`; `id = "$QID"`, `target_minor = 28000000`, `currency = "EUR"`.
- Second: `HTTP 404`; `body.code = "not_found"`.

---

## Step 7 [live]: PATCH /quotas/{id} — valid If-Match (200) and stale If-Match (409)

Step 2 returned `version = 1`; use `1` as the `If-Match` value.

```bash
# valid If-Match — bumps target_minor and increments version:
curl -s -w '\n%{http_code}\n' -X PATCH "http://localhost:8080/quotas/$QID" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -H "Content-Type: application/json" \
  -H "If-Match: 1" \
  -d '{"target_minor":30000000}'

# stale If-Match (version far ahead of actual row):
curl -s -w '\n%{http_code}\n' -X PATCH "http://localhost:8080/quotas/$QID" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -H "Content-Type: application/json" \
  -H "If-Match: 9999" \
  -d '{"target_minor":99}'
```

**Expected:**
- First: `HTTP 200`; `target_minor = 30000000`; `version > 1`.
- Second: `HTTP 409`; `body.code = "version_skew"`.

---

## Step 8 [live]: DELETE /quotas/{id} → 200 archived_at set; subsequent GET → 404

```bash
# Archive the team quota:
curl -s -w '\n%{http_code}\n' -X DELETE "http://localhost:8080/quotas/$QID_TEAM" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"

# Re-GET the archived quota:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas/$QID_TEAM" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"

# include_archived=true makes it visible again:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas?include_archived=true" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"
```

**Expected:**
- DELETE: `HTTP 200`; `archived_at` is a non-null RFC3339 timestamp (never 204 — the handler
  always returns the archived entity, RD-AC-4).
- Re-GET: `HTTP 404` (archived rows excluded from default Get, RD-AC-6).
- `include_archived=true` list: `HTTP 200`; `data` includes both `$QID` (live) and `$QID_TEAM`
  (archived).

---

## Step 9 [auto]: Seed won deals then verify attainment golden number (RD-AC-3)

```bash
psql "$TEST_DATABASE_URL" -c "
INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, owner_id,
                  amount_minor, currency, fx_rate_to_base, status, closed_at, source, captured_by)
VALUES
  ('$WS','UAT-WonDeal-1','$PIPE','$STAGE','$USER',
   18000000,'EUR','1.0000000000','won','2025-06-15T12:00:00Z','api','human:uat'),
  ('$WS','UAT-WonDeal-2','$PIPE','$STAGE','$USER',
   13387200,'EUR','1.0000000000','won','2025-06-15T12:00:00Z','api','human:uat');
"
```

**Expected:** `INSERT 0 2`.

```bash
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas/$QID/attainment" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"
```

**Expected:** `HTTP 200`. Body (RD-AC-3 golden-number reconciliation; note `target_minor` is
30,000,000 after Step 7's patch):
- `quota_id = "$QID"`
- `closed_won_minor = 31387200` (18,000,000 + 13,387,200)
- `target_minor = 30000000`
- `attainment_pct ≈ 104.6` (31387200 / 30000000 × 100)
- `gap_minor = 1387200` (positive = quota exceeded)
- `band = "met"` (attainment ≥ 100%)
- `contributing_deals` has 2 items; the sum of their `base_value_minor` fields equals
  `closed_won_minor = 31387200`

---

## Step 10 [live]: Attainment 422 — missing FX rate and nonexistent quota

Seed a USD quota (no USD→EUR `fx_rate` row exists in this workspace):

```bash
USD_QID=$(curl -s -X POST "http://localhost:8080/quotas" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -H "Content-Type: application/json" \
  -d "{\"owner_id\":\"$USER\",\"period_start\":\"2025-01-01\",\"period_end\":\"2025-12-31\",\"target_minor\":10000000,\"currency\":\"USD\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "USD_QID=$USD_QID"

curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas/$USD_QID/attainment" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"

# Nonexistent quota:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas/00000000-0000-0000-0000-000000000000/attainment" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"
```

**Expected:**
- USD quota attainment: `HTTP 422`; `body.code = "attainment_computation_failed"` (no USD→EUR
  fx_rate row exists for this workspace).
- Nonexistent quota: `HTTP 404`; `body.code = "not_found"`.

---

## Step 11 [live]: Auth gates — 401 (no principal) and 403 (no quota.read permission)

```bash
psql "$TEST_DATABASE_URL" <<EOSQL
DO \$\$
DECLARE
  noperm_user uuid;
  noperm_role uuid;
BEGIN
  INSERT INTO app_user(workspace_id, email, display_name)
    VALUES ('$WS', 'noperm@uat-rdt06.test', 'NoPerm')
    RETURNING id INTO noperm_user;
  INSERT INTO role(workspace_id, key, is_system, permissions)
    VALUES ('$WS', 'uat-noperm', false, '{"deal":{"read":{"row_scope":"all"}}}'::jsonb)
    RETURNING id INTO noperm_role;
  INSERT INTO role_assignment(workspace_id, role_id, user_id)
    VALUES ('$WS', noperm_role, noperm_user);
  RAISE NOTICE 'noperm=%', noperm_user;
END\$\$;
EOSQL
```

**Expected:** Block exits cleanly; `RAISE NOTICE` prints a UUID. Record it as `$NOPERM`.

```bash
# 401: no headers → no principal in context:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas"

# 403: real principal but role has no quota.read:
curl -s -w '\n%{http_code}\n' "http://localhost:8080/quotas" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $NOPERM"
```

**Expected:**
- First: `HTTP 401` (`RequireAuth` fires before any permission check).
- Second: `HTTP 403` (`RbacMiddleware`'s `session.AuthorizePerms(perms, "quota", "read")` denies —
  `$NOPERM`'s role only grants `deal.read`; `"quota"` is absent from its permissions).

---

## Step 12 [auto]: Transport integration tests — all green, 0 skips

```bash
make test-it DIR=backend/internal/modules/records/transport
```

**Expected:** All tests pass including:
- `TestQuotaHTTP_Create` (4 sub-tests)
- `TestQuotaHTTP_GetAndList` (7 sub-tests)
- `TestQuotaHTTP_Update` (3 sub-tests)
- `TestQuotaHTTP_Archive`
- `TestQuotaHTTP_Attainment` (4 sub-tests: golden-number 200, target_zero 422, missing-FX 422, 404)
- `TestQuotaHTTP_Auth` (401 no-session, 403 no-quota-perm)
- `TestQuotaHTTP_RBACBootstrap` (POST /workspaces → bootstrap admin → GET /quotas must not 403)
- All unit tests (no build tag): `TestQuotaHandler_Create_201`, etc.
- Output: `ok github.com/gradionhq/margince/backend/internal/modules/records/transport`

---

## Step 13 [auto]: Full records integration suite — all green, 0 skips

```bash
make test-it DIR=backend/internal/modules/records
```

**Expected:** All tests pass including all `TestQuotaStore_*` (CRUD, attainment, scoping) and all
hierarchy-rollup tests. Output:
`ok github.com/gradionhq/margince/backend/internal/modules/records`

---

## Step 14 [auto]: make check — project-wide gate

```bash
make check
```

**Expected:** All gates pass. The `"quota"` entry added to `knownObjects` in
`backend/internal/shared/ports/session/session.go` ensures `session.ValidatePermissions` accepts
`adminPermissionsJSON`'s quota permissions; the RBAC bootstrap gap confirmed by
`TestQuotaHTTP_RBACBootstrap` is now closed.

> **Note:** Any pre-existing `contract-breaking-check` failures unrelated to this ticket are not
> caused by RD-T06's changes and must be resolved in their own tickets.
