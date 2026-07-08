# RD-T07 Live-Stack UAT — Field-History Projection (RD-WIRE-5)

Verification of `GET /field-history` against RD-WIRE-5, RD-AC-5, RD-AC-11, and RD-PARAM-6.
Step 1 is `[auto]` (psql seed with `RAISE NOTICE` variable capture). Steps 2–7 are `[live]` —
real `curl` calls against the booted server (`make run` / `make uat_env`, default
`http://localhost:8080`), using dev-mode `X-Workspace-ID` / `X-User-ID` header auth
(`backend/cmd/api/routes.go`'s `workspaceWrap` fallback — same pattern as `rd-t04.md`).
Step 8 is `[auto]` (automated test suite). Every step has a literal command and a literal
`Expected:`.

Variable mapping (captured from Step 1's `RAISE NOTICE` output):
`$WS`=ws, `$ORG`=org, `$OTHER_ORG`=other_org, `$VIEWER`=viewer, `$NOPERM`=noperm.

---

## Step 1 [auto]: Seed workspace, organization, and audit_log rows

```bash
psql "$TEST_DATABASE_URL" <<'EOSQL'
DO $$
DECLARE
  ws          uuid;
  org         uuid;
  other_org   uuid;
  viewer      uuid;
  noperm      uuid;
  role_v      uuid;
  role_n      uuid;
  passport_val uuid := 'aaaabbbb-0000-0000-0000-ccccddddeeee';
BEGIN
  -- workspace
  INSERT INTO workspace(name, slug, base_currency)
    VALUES ('UAT-RDT07', 'uat-rdt07', 'EUR')
    RETURNING id INTO ws;

  -- viewer: full organization read
  INSERT INTO app_user(workspace_id, email, display_name)
    VALUES (ws, 'viewer@uat-rdt07.test', 'Viewer')
    RETURNING id INTO viewer;
  INSERT INTO role(workspace_id, key, is_system, permissions)
    VALUES (ws, 'uat-viewer', false, '{"organization":{"read":{"row_scope":"all"}}}'::jsonb)
    RETURNING id INTO role_v;
  INSERT INTO role_assignment(workspace_id, role_id, user_id) VALUES (ws, role_v, viewer);

  -- noperm: deal read only (no organization permission at all)
  INSERT INTO app_user(workspace_id, email, display_name)
    VALUES (ws, 'noperm@uat-rdt07.test', 'NoPerm')
    RETURNING id INTO noperm;
  INSERT INTO role(workspace_id, key, is_system, permissions)
    VALUES (ws, 'uat-noperm', false, '{"deal":{"read":{"row_scope":"all"}}}'::jsonb)
    RETURNING id INTO role_n;
  INSERT INTO role_assignment(workspace_id, role_id, user_id) VALUES (ws, role_n, noperm);

  -- primary organization (entity we'll query)
  INSERT INTO organization(workspace_id, name, source, captured_by)
    VALUES (ws, 'Acme Corp', 'api', 'human:uat')
    RETURNING id INTO org;

  -- second organization (entity scoping: its rows must never appear in org's results)
  INSERT INTO organization(workspace_id, name, source, captured_by)
    VALUES (ws, 'Other Corp', 'api', 'human:uat')
    RETURNING id INTO other_org;

  -- Row A: create (before=NULL) — oldest, occurred 3 hours ago.
  -- 2 after keys → 2 entries (industry, name — alphabetical order within one row).
  INSERT INTO audit_log(workspace_id, actor_type, actor_id, action,
                        entity_type, entity_id, before, after, occurred_at)
    VALUES (ws, 'human', viewer::text, 'create',
            'organization', org,
            NULL,
            '{"industry": "Technology", "name": "Acme Corp"}'::jsonb,
            now() - interval '3 hours');

  -- Row B: update — field removal (before has "website", after drops it).
  -- 1 entry emitted: website ("http://old.example.com" → null).
  INSERT INTO audit_log(workspace_id, actor_type, actor_id, action,
                        entity_type, entity_id, before, after, occurred_at)
    VALUES (ws, 'human', viewer::text, 'update',
            'organization', org,
            '{"name": "Acme Corp", "website": "http://old.example.com"}'::jsonb,
            '{"name": "Acme Corp"}'::jsonb,
            now() - interval '2 hours');

  -- Row C: multi-field update — 3 keys, 2 changed (name, phone), 1 unchanged (website).
  -- 2 entries emitted (name, phone — alphabetical); website absent from output.
  INSERT INTO audit_log(workspace_id, actor_type, actor_id, action,
                        entity_type, entity_id, before, after, occurred_at)
    VALUES (ws, 'human', viewer::text, 'update',
            'organization', org,
            '{"name": "Acme Corp", "phone": "555-1234", "website": "https://acme.com"}'::jsonb,
            '{"name": "Acme Ltd", "phone": "555-9999", "website": "https://acme.com"}'::jsonb,
            now() - interval '1 hour');

  -- Row D: agent actor — most recent. passport_id + evidence set; 1 field changed (employees).
  INSERT INTO audit_log(workspace_id, actor_type, actor_id, passport_id, action,
                        entity_type, entity_id, before, after, evidence, occurred_at)
    VALUES (ws, 'agent', 'agent:system-enricher', passport_val, 'update',
            'organization', org,
            '{"employees": 100}'::jsonb,
            '{"employees": 250}'::jsonb,
            '{"source": "linkedin", "confidence": 0.9}'::jsonb,
            now());

  -- Row E: different entity_id — must NEVER appear in queries for $ORG.
  INSERT INTO audit_log(workspace_id, actor_type, actor_id, action,
                        entity_type, entity_id, before, after, occurred_at)
    VALUES (ws, 'human', viewer::text, 'create',
            'organization', other_org,
            NULL,
            '{"name": "Other Corp"}'::jsonb,
            now() - interval '30 minutes');

  RAISE NOTICE 'ws=%   org=%   other_org=%   viewer=%   noperm=%',
    ws, org, other_org, viewer, noperm;
END$$;
EOSQL
```

**Expected:** The `DO` block exits cleanly (no error) and `RAISE NOTICE` prints five UUIDs.
Record `ws`, `org`, `other_org`, `viewer`, `noperm` — used in subsequent steps as
`$WS`, `$ORG`, `$OTHER_ORG`, `$VIEWER`, `$NOPERM` respectively.

---

## Step 2 [live]: Full listing — newest-first, multi-field row emits 2 entries, unchanged field absent

Replace `$ORG`, `$VIEWER`, `$WS` with the values captured from Step 1.

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. Body `data` array has 6 entries (newest-first across all rows):

1. Row D (agent, `occurred_at ≈ now()`): 1 entry — `field="employees"`, `old_value="100"`,
   `new_value="250"`, `actor_type="agent"`, `actor_id="agent:system-enricher"`,
   `passport_id="aaaabbbb-0000-0000-0000-ccccddddeeee"`,
   `evidence={"source":"linkedin","confidence":0.9}`.
2. Row C (human, `occurred_at ≈ now()-1h`): 2 entries sharing the same `id` and `changed_at`:
   - `field="name"`, `old_value="Acme Corp"`, `new_value="Acme Ltd"`, `passport_id=null`,
     `evidence=null`
   - `field="phone"`, `old_value="555-1234"`, `new_value="555-9999"`, `passport_id=null`,
     `evidence=null`
   - **`field="website"` is absent** — `before.website == after.website` (`reflect.DeepEqual`);
     no fabricated entry (RD-AC-5).
3. Row B (human, `occurred_at ≈ now()-2h`): 1 entry — `field="website"`,
   `old_value="http://old.example.com"`, `new_value=null` (field removed).
4. Row A (human, `occurred_at ≈ now()-3h`): 2 entries (create, `before=null`) sharing the same
   `id` and `changed_at` — alphabetical field order within the row:
   - `field="industry"`, `old_value=null`, `new_value="Technology"`
   - `field="name"`, `old_value=null`, `new_value="Acme Corp"`

`page.has_more=false`, `page.next_cursor=null` (all rows fit).

Row E (`$OTHER_ORG`) entries are absent — entity scoping is enforced.

---

## Step 3 [live]: `?field=name` — only the "name" field's entries, server-side

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG&field=name" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. `data` has exactly 2 entries (only rows that changed "name",
newest-first):

1. Row C entry: `field="name"`, `old_value="Acme Corp"`, `new_value="Acme Ltd"`
2. Row A entry: `field="name"`, `old_value=null`, `new_value="Acme Corp"`

Row D's `employees` entry, Row B's `website` entry, and Row A's `industry` entry are absent.
This is a server-side filter applied before pagination — not a client-side hint that gets
ignored (RD-AC-5).

---

## Step 4 [live]: `?actor_type=agent` — only agent rows, carrying `passport_id`+`evidence`; human rows carry neither

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG&actor_type=agent" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. `data` has exactly 1 entry (Row D):
- `field="employees"`, `old_value="100"`, `new_value="250"`
- `actor_type="agent"`, `actor_id="agent:system-enricher"`
- `passport_id="aaaabbbb-0000-0000-0000-ccccddddeeee"` (non-null — agent actor)
- `evidence={"source":"linkedin","confidence":0.9}` (non-null — agent actor)

Rows A, B, C (all `actor_type='human'`) are entirely absent.

Now confirm human rows carry neither field — request the human rows:

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG&actor_type=human" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. All returned entries have `passport_id=null` and `evidence=null`
regardless of what the underlying `audit_log` columns contain — the spec restricts these two
fields to `actor_type='agent'` only (RD-AC-5).

---

## Step 5 [live]: RBAC deny — viewer without `organization.read` gets 403

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $NOPERM"
```

**Expected:** `HTTP 403`. Body has `code` set to a forbidden/access-denied value (matches the
sibling `/records/{entity_type}/{id}/history` route's `403` shape). The store is never called —
the RBAC gate fires in the handler before any DB work.

---

## Step 6 [live]: Nonexistent entity_id → honest `200 {data: []}`, never 404 (RD-AC-5)

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=00000000-0000-0000-0000-000000000000" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. Body:
```json
{"data": [], "page": {"has_more": false, "next_cursor": null}}
```

Never a `404` or `500`. An `entity_id` with no `audit_log` rows is a valid, honest empty
result — this is the "no fabricated timeline" requirement (RD-AC-5).

---

## Step 7 [live]: Cursor pagination — `?limit=1` yields `has_more=true`; follow-up cursor page continues with no overlap or gap

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG&limit=1" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. `page.has_more=true`, `page.next_cursor` is a non-null opaque string
(base64-encoded keyset). `data` contains Row D's entry (employees, the most-recent row) —
row-boundary preservation means a single row's entries are never split across pages, so the
single-entry Row D fills this page exactly.

Record the cursor value as `$CURSOR`.

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG&limit=1&cursor=$CURSOR" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. `data` contains Row C's 2 entries (name and phone — both siblings from
the same `audit_log` row, emitted together because a single row's entries are never split).
`page.has_more=true`, `page.next_cursor` is a new non-null cursor. No entry from Row D appears
(no overlap); no Row C entry is missing (no gap).

Record the new cursor as `$CURSOR2`.

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG&limit=1&cursor=$CURSOR2" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. `data` contains Row B's 1 entry (website removal). `page.has_more=true`.

Record the new cursor as `$CURSOR3`.

```bash
curl -s -w '\n%{http_code}\n' \
  "http://localhost:8080/field-history?entity_type=organization&entity_id=$ORG&limit=1&cursor=$CURSOR3" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $VIEWER"
```

**Expected:** `HTTP 200`. `data` contains Row A's 2 entries (industry and name, create row).
`page.has_more=false`, `page.next_cursor=null` — genuine exhaustion, no more rows.

Across all four pages: 1 + 2 + 1 + 2 = 6 entries, matching the Step 2 total. No duplicates,
no gaps.

---

## Step 8 [auto]: Automated test suite — unit lane and integration lane both green

```bash
(cd backend && go test ./internal/modules/records/... -run .)
```

**Expected:** All unit tests pass. Output includes tests for `diffRowFields`,
`encodeCursor`/`decodeCursor`, and `FieldHistoryHandler` (parameter validation, RBAC deny,
happy path, honest-empty).

```bash
make test-it DIR=backend/internal/modules/records
```

**Expected:** All integration tests pass including:
- `TestFieldHistory_DiffProjection` (RD-AC-11/RD-WIRE-5: multi-field row, unchanged field absent)
- `TestFieldHistory_Attribution` (RD-AC-5: agent attribution + actor_type/field filters)
- `TestFieldHistory_Masking` (RD-AC-5/RD-PARAM-6: masked field withheld, erasure tombstone → zero entries)
- `TestFieldHistory_Empty` (RD-AC-5: nonexistent entity_id → honest empty, no error)
- `TestFieldHistory_Pagination` (cursor pagination, no overlap/gap)
- `TestFieldHistory_NewestFirst` (ordering)

Output: `ok github.com/gradionhq/margince/backend/internal/modules/records (pass, 0 skips)`
