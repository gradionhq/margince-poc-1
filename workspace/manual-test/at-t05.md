# AT-T05 — Live-UAT Guide: `POST /activities/{id}/relink`

Runnable top-to-bottom against a fresh workspace. Use the stack’s auth mode for your environment:
`X-Workspace-ID`/`X-User-ID` headers for dev-mode header auth, or a session cookie if that is
how the server is booted. Each step is tagged `[auto]` when it is seed/setup or a verification
command, and `[live]` when it is a real request against the running server.

## Prereqs

- `make infra-up migrate-up seed-reset run` or `make uat_env UAT_SLUG=at-t05`
- A seeded human principal that can access `/activities`
- `psql "$DATABASE_URL"` available for the audit-row assertions

## Step 1 — [auto] Seed one workspace, two people, and one bare note activity

Create or seed a workspace, then create two distinct people, `A` and `B`, plus one `note`
activity with provenance set and no links yet.

```bash
BASE="http://localhost:8080"
WS="<seeded-workspace-id>"
USER="<seeded-user-id>"

A_ID=$(curl -s -X POST "$BASE/people" \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER" \
  -d '{"full_name":"AT-T05 Person A","source":"manual-test","captured_by":"human:uat"}' \
  | python3 -c "import sys, json; print(json.load(sys.stdin)['id'])")

B_ID=$(curl -s -X POST "$BASE/people" \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER" \
  -d '{"full_name":"AT-T05 Person B","source":"manual-test","captured_by":"human:uat"}' \
  | python3 -c "import sys, json; print(json.load(sys.stdin)['id'])")

ACTIVITY_ID=$(curl -s -X POST "$BASE/activities" \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER" \
  -d '{"kind":"note","subject":"AT-T05 relink target","source":"manual-test","captured_by":"human:uat"}' \
  | python3 -c "import sys, json; print(json.load(sys.stdin)['id'])")

echo "A_ID=$A_ID"
echo "B_ID=$B_ID"
echo "ACTIVITY_ID=$ACTIVITY_ID"
```

**Expected:** all three variables are non-empty UUID strings. The activity has no links yet.

## Step 2 — [live] Add the first person link

```bash
curl -s -X POST "$BASE/activities/$ACTIVITY_ID/relink" \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER" \
  -d "{\"entity_type\":\"person\",\"entity_id\":\"$A_ID\"}"
```

**Expected:** `200 OK`. The response body `links` has exactly one entry:

```json
{"entity_type":"person","entity_id":"<A_ID>"}
```

## Step 3 — [live] Repeat the exact same relink request

```bash
curl -s -X POST "$BASE/activities/$ACTIVITY_ID/relink" \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER" \
  -d "{\"entity_type\":\"person\",\"entity_id\":\"$A_ID\"}"
```

**Expected:** `200 OK`. `links` is unchanged and still has exactly one entry for `<A_ID>`.
Verify the audit row count does not increase:

```bash
psql "$DATABASE_URL" -c "
  SELECT count(*)
  FROM audit_log
  WHERE entity_type='activity'
    AND entity_id='$ACTIVITY_ID'::uuid
    AND action='activity_relink';"
```

**Expected:** `count = 1`.

## Step 4 — [live] Move the typed link from A to B

```bash
curl -s -X POST "$BASE/activities/$ACTIVITY_ID/relink" \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER" \
  -d "{\"entity_type\":\"person\",\"entity_id\":\"$B_ID\"}"
```

**Expected:** `200 OK`. The response body `links` has exactly one `person`-type entry and it now
points to `<B_ID>` instead of `<A_ID>`.

Verify the audit row count increased by one:

```bash
psql "$DATABASE_URL" -c "
  SELECT count(*)
  FROM audit_log
  WHERE entity_type='activity'
    AND entity_id='$ACTIVITY_ID'::uuid
    AND action='activity_relink';"
```

**Expected:** `count = 2`.

## Step 5 — [auto] Check provenance stayed byte-identical across the relinks

Capture the activity before Step 2 and after Step 4, then diff `source` and `captured_by`.

```bash
curl -s -X GET "$BASE/activities/$ACTIVITY_ID" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER"
```

**Expected:** `source` and `captured_by` are unchanged across all relink calls. Only `links`
changes.

## Step 6 — [live] Invalid entity type is rejected with 422

```bash
curl -s -i -X POST "$BASE/activities/$ACTIVITY_ID/relink" \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER" \
  -d "{\"entity_type\":\"lead\",\"entity_id\":\"$(python3 - <<'PY'
import uuid
print(uuid.uuid4())
PY
)\"}"
```

**Expected:** `422 Unprocessable Entity`, `code: validation_error`, and `details.errors` contains
`{"field":"entity_type","code":"invalid_link_entity_type"}`.

## Step 7 — [live] Missing activity returns 404

```bash
curl -s -i -X POST "$BASE/activities/00000000-0000-0000-0000-000000000000/relink" \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" \
  -H "X-User-ID: $USER" \
  -d "{\"entity_type\":\"person\",\"entity_id\":\"$A_ID\"}"
```

**Expected:** `404 Not Found`.

## Step 8 — [auto] Verify the guide is self-contained

Confirm every `[live]` step uses only values captured in Step 1 and that the guide has no hidden
setup or unresolved TODOs.

