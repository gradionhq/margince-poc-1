# AT-T01 — relinkActivity Contract + audit_relink Token Live-UAT Guide

## Step 1: Verify git diff shows expected files

**Command:**
```bash
git diff --stat origin/main...HEAD
```

**Expected:**
The diff should show exactly these files changed:
- `backend/api/crm.yaml` (contract additions: `/activities/{id}/relink` path, `RelinkActivityRequest` schema, `activity_relink` audit token)
- `backend/internal/contracts/types/crm_gen.go` (regenerated: new `RelinkActivity` method, `RelinkActivityParams`, `RelinkActivityRequest` types, widened `action` enum)
- `frontend/src/lib/api-client/generated/crm.d.ts` (regenerated TS types, mechanically from contract)
- `backend/internal/contracts/server/activities_adapter.go` (new `RelinkActivity` 501 stub)
- `backend/internal/shared/ports/mcp/tools_gen.go` (one new MCP tool entry: `relinkActivity` with verb `update_record`)
- `backend/migrations/000074_activity_relink_audit_action.up.sql` (new migration)
- `backend/migrations/000074_activity_relink_audit_action.down.sql` (new migration)

No handler/store/frontend-component files should appear (contract-only scope).

---

## Step 2: Verify make gen-types is idempotent

**Command:**
```bash
make gen-types
git status backend/internal/contracts/types/crm_gen.go frontend/src/lib/api-client/generated/crm.d.ts
```

**Expected:**
- `make gen-types` exits 0
- `git status` shows no further changes (both generated files already committed in this branch)
- Running `make gen-types` a second time produces no diff (idempotent)

---

## Step 3: Verify migration applies and activity_relink is admitted

**Command:**
```bash
make infra-up
make migrate-up
```

**Expected:**
- `make infra-up` exits 0 (postgres, redis, minio running)
- `make migrate-up` exits 0, migration 74 applied: `74/u activity_relink_audit_action`

**Then verify activity_relink is in the constraint:**

```bash
psql 'postgres://margince:margince@localhost:5432/margince?sslmode=disable' -c \
  "SELECT check_clause FROM information_schema.check_constraints WHERE constraint_name = 'audit_log_action_check';" | grep activity_relink
```

**Expected:**
- Output contains `'activity_relink'::text` — confirming the new token is in the CHECK constraint

---

## Step 4: Verify all project gates pass

**Command:**
```bash
make check
```

**Expected:**
- All checks pass, including:
  - `make audit-coherence` (crm.yaml's `AuditLogEntry.action` enum matches `audit_log_action_check` exactly — both now have 33 tokens with `activity_relink`)
  - `make gen-types-check` (generated types are up-to-date, no drift)
  - `make gen-mcp-tools-check` (MCP tools manifest is up-to-date)
  - `make contract-breaking-check` (only additive changes: new operation, new schema, widened enum)
  - Backend tests, frontend tests, linting, type checking, all pass
  - Final output: `OK: make check passed`

---

## Step 5: Verify migration round-trip (down + up) is clean

**Command:**
```bash
make migrate-down
make migrate-up
make migrate-status
```

**Expected:**
- `make migrate-down` exits 0, rolls back migration 74: `74/d activity_relink_audit_action`
- `make migrate-up` exits 0, re-applies migration 74: `74/u activity_relink_audit_action`
- `make migrate-status` outputs `74` (non-dirty, no pending migrations, clean state)

---

## Step 6: Boot stack and verify relinkActivity is unmounted (404, not a live route)

**Command:**
```bash
make run &
```

Wait for the API to start (look for "listening on :8080" or similar in logs). Then log in
to get an authenticated session cookie (the `/activities/` subtree is behind auth
middleware, so an unauthenticated request would return 401 and prove nothing about
whether the route itself is mounted):

```bash
curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"changeme"}' \
  -c /tmp/cookies.txt
```

Then probe the relink route using an existing activity id (there is no `/v1` path
prefix on this server — use the bare path):

```bash
ACT_ID=$(PGPASSWORD=margince psql -h localhost -U margince -d margince -tAc "select id from activity limit 1;")

curl -s -i -X POST "http://localhost:8080/activities/$ACT_ID/relink" \
  -b /tmp/cookies.txt \
  -H "X-Workspace-ID: 00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{"entity_type": "person", "entity_id": "00000000-0000-0000-0000-000000000002"}'
```

**Expected:**
- HTTP response: **404 Not Found**, plain-text body `404 page not found` (not 501, not 200)
- The real live server dispatches through `routes.go`'s `crud("/activities", ...)`
  registration to the hand-written `ActivityHandler.ServeHTTP`, whose switch only
  handles `GET` (list/get), `PATCH`, and `DELETE` with a non-empty id; `POST` matches
  none of those cases and falls through to the handler's own `default: http.NotFound(w, r)`
  branch
- This confirms no route is actually mounted for `relinkActivity` yet — the 404 is the
  expected, correct response, not a bug. (The `RelinkActivity` 501 stub added to
  `activities_adapter.go` is part of the `types.ServerInterface` conformance layer only —
  per `all_operations.go`'s own doc comment, that layer is "interface-generation scope
  only" and "nothing here is wired to serve traffic," so it is never invoked for a real
  HTTP request; AT-T05 is expected to wire the real mutation and live route)

---

## Step 7: Verify make check passes (final gate)

**Command:**
```bash
make check
```

**Expected:**
- All checks pass (repeat of Step 4 — same assertion: `OK: make check passed`)
- Confirms no regressions were introduced during UAT execution
- All gates remain green after round-trip + stack boot

---

**Summary:** All seven UAT steps completed successfully. The contract is correctly declared, the migration properly widens the audit CHECK, the stack can be round-tripped cleanly, and the 404 response for `relinkActivity` confirms the operation is contract-only with no live route mounted yet (the `RelinkActivity` 501 stub exists only in the `types.ServerInterface` conformance layer and is never invoked by the live server). The project gate is fully green.
