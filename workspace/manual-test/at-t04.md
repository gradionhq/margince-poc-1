# AT-T04 — Live-UAT Guide: updateActivity / archiveActivity audit + event

Proves that `PATCH /activities/{id}` and `DELETE /activities/{id}` each write exactly one
`audit_log` row and one domain-topic `event_outbox` row through the real HTTP + DB stack —
including the task done-transition (always `activity.updated`, never `task.completed` — ACT-EVT-N-1)
and idempotent repeat-archive (no double-fire).

**Prereqs:** `make infra-up migrate-up run` (or `make uat_env UAT_SLUG=at-t04`).
Set shell variables:

```bash
BASE="http://localhost:8080"
WS="<your-seeded-workspace-id>"   # X-Workspace-ID header value
USER="<your-seeded-user-id>"      # X-User-ID header value
H='-H "Content-Type: application/json" -H "X-Workspace-ID: '"$WS"'" -H "X-User-ID: '"$USER"'"'
```

`psql "$DATABASE_URL"` is used for DB-state assertions in every step.

---

## Step 0 — [live] Seed two activities (one note, one task)

```bash
NOTE_ID=$(curl -s -X POST $BASE/activities \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -d '{"kind":"note","subject":"UAT note","source":"ui","captured_by":"human:uat"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "NOTE_ID=$NOTE_ID"

TASK_ID=$(curl -s -X POST $BASE/activities \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -d '{"kind":"task","subject":"UAT task","due_at":"2026-08-01T00:00:00Z","source":"ui","captured_by":"human:uat"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "TASK_ID=$TASK_ID"
```

**Expected:** Both return `201 Created` with a UUID `id`. Both `NOTE_ID` and `TASK_ID` are
non-empty strings.

---

## Step 1 — [live] Plain field update writes one audit_log row + one activity.updated event

```bash
curl -s -X PATCH $BASE/activities/$NOTE_ID \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -d '{"subject":"UAT note (corrected)"}'
```

**Expected:** `200 OK`, response body `subject` is `"UAT note (corrected)"`.

```bash
psql "$DATABASE_URL" -c "
  SELECT action, entity_type, entity_id
  FROM audit_log
  WHERE entity_type='activity' AND entity_id='$NOTE_ID'::uuid AND action='update';"
```

**Expected:** exactly **1 row** with `action=update`, `entity_type=activity`.

```bash
psql "$DATABASE_URL" -c "
  SELECT topic, entity_id
  FROM event_outbox
  WHERE topic='activity.updated' AND entity_id='$NOTE_ID'::uuid;"
```

**Expected:** exactly **1 row** with `topic=activity.updated`. (`audit.appended` rows from
`crmaudit.WriteTx` are also present — those are expected, out-of-ticket behavior.)

---

## Step 2 — [live] Task done-transition emits activity.updated, never task.completed (ACT-EVT-N-1)

```bash
curl -s -X PATCH $BASE/activities/$TASK_ID \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER" \
  -d '{"is_done":true,"subject":"UAT task (done)"}'
```

**Expected:** `200 OK`, response body has `"is_done": true` and a non-null `done_at` timestamp.
One logical mutation (done-transition + subject edit in the same request) → exactly one pair of
rows, never two.

```bash
psql "$DATABASE_URL" -c "
  SELECT action, entity_type, entity_id
  FROM audit_log
  WHERE entity_type='activity' AND entity_id='$TASK_ID'::uuid AND action='update';"
```

**Expected:** exactly **1 row**.

```bash
psql "$DATABASE_URL" -c "
  SELECT topic, entity_id
  FROM event_outbox
  WHERE topic='activity.updated' AND entity_id='$TASK_ID'::uuid;"
```

**Expected:** exactly **1 row** (`topic=activity.updated`).

```bash
psql "$DATABASE_URL" -c "
  SELECT count(*) FROM event_outbox WHERE topic='task.completed';"
```

**Expected:** `count = 0` — no `task.completed` topic exists anywhere in the table. The 43-event
catalog has no such type (ACT-EVT-N-1).

---

## Step 3 — [live] Archive writes one audit_log row + one activity.archived event

Optionally seed an attachment first to confirm the RD-T05 cascade also runs (not required for the
audit/event assertion):

```bash
# Optional: seed an attachment on $NOTE_ID before archiving
# curl -s -X POST $BASE/attachments ...
```

Archive the note:

```bash
curl -s -X DELETE $BASE/activities/$NOTE_ID \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"
```

**Expected:** `200 OK`, response body has a non-null `archived_at` timestamp.

```bash
psql "$DATABASE_URL" -c "
  SELECT action, entity_type, entity_id
  FROM audit_log
  WHERE entity_type='activity' AND entity_id='$NOTE_ID'::uuid AND action='archive';"
```

**Expected:** exactly **1 row** with `action=archive`.

```bash
psql "$DATABASE_URL" -c "
  SELECT topic, entity_id
  FROM event_outbox
  WHERE topic='activity.archived' AND entity_id='$NOTE_ID'::uuid;"
```

**Expected:** exactly **1 row** with `topic=activity.archived`. The attachment cascade (RD-T05)
updates `attachment.archived_at` inside the same transaction but does **not** add a second
`audit_log` or `event_outbox` row for the activity itself — total for the activity remains 1/1.

---

## Step 4 — [live] Repeat archive is idempotent — no double-fire

```bash
curl -s -X DELETE $BASE/activities/$NOTE_ID \
  -H "X-Workspace-ID: $WS" -H "X-User-ID: $USER"
```

**Expected:** `200 OK` (not 404 — the handler returns the already-archived row via `getAny`),
`archived_at` still set to the **original** timestamp from Step 3. The `if n > 0` guard in
`Archive` detects zero rows changed and skips the audit+event block entirely.

```bash
psql "$DATABASE_URL" -c "
  SELECT count(*) FROM audit_log
  WHERE entity_type='activity' AND entity_id='$NOTE_ID'::uuid AND action='archive';"
```

**Expected:** `count = 1` — still exactly one row, not two.

```bash
psql "$DATABASE_URL" -c "
  SELECT count(*) FROM event_outbox
  WHERE topic='activity.archived' AND entity_id='$NOTE_ID'::uuid;"
```

**Expected:** `count = 1` — still exactly one row, not two.
