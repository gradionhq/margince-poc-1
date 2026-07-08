# AT-T02 Manual/Live UAT — activity store + logActivity/getActivity

Prereqs: `make infra-up migrate-up seed-reset run` (or the swarm per-worktree `make uat_env
UAT_SLUG=at-t02` stack). Auth: a seeded human workspace member (`X-Workspace-ID`/`X-User-ID`
headers, or session cookie per the stack's auth mode) with access to `/activities`. Note the
`base_currency`-seeded workspace id and a seeded `deal` id + `person` id to link against (seed
data or create them first via `POST /organizations`, `POST /people`, `POST /deals`).

## Step 1 — Idempotent capture-key create
`POST /activities` with:
```json
{"kind":"email","subject":"Re: proposal","occurred_at":"2026-07-08T10:00:00Z",
 "source_system":"gmail","source_id":"uat-msg-1",
 "links":[{"entity_type":"deal","entity_id":"<seeded-deal-id>"}],
 "source":"email:uat-msg-1","captured_by":"agent:capture"}
```
**Expected:** `201 Created`, a `Location` header, response body's `links` has exactly one entry
`{entity_type: "deal", entity_id: "<seeded-deal-id>"}`. Record the returned `id`.

## Step 2 — Replay is idempotent
Repeat the exact same `POST /activities` request from Step 1.
**Expected:** `200 OK` (not 201), response `id` equals Step 1's `id`. No duplicate row.

## Step 3 — Multi-entity link
`POST /activities` with a new `source_id` (e.g. `uat-msg-2`) and
`"links":[{"entity_type":"person","entity_id":"<seeded-person-id>"},{"entity_type":"deal","entity_id":"<seeded-deal-id>"}]`.
**Expected:** `201 Created`, response `links` has exactly 2 entries, one `person` one `deal`.

## Step 4 — `getActivity` returns links + raw
`GET /activities/{id}` using the `id` from Step 1 (`"raw":{"messageId":"uat-msg-1"}` on the
original POST body if you add it).
**Expected:** `200 OK`; response includes populated `links[]` and `raw` matching what was POSTed.

## Step 5 — Missing provenance rejected before the DB
`POST /activities` with `{"kind":"note","body":"no provenance"}` (omit `source`/`captured_by`).
**Expected:** `422`, `code: validation_error`, no row created (`GET /activities?q=no%20provenance`
returns nothing new).

## Step 6 — Task-only field on a non-task kind rejected
`POST /activities` with `{"kind":"note","due_at":"2026-08-01T00:00:00Z","source":"ui","captured_by":"human:uat"}`.
**Expected:** `422`, `code: field_not_valid_for_kind`.

## Step 7 — Task kind allows task fields, and `is_done` completes it
`POST /activities` with `{"kind":"task","due_at":"2026-08-01T00:00:00Z","source":"ui","captured_by":"human:uat"}`.
**Expected:** `201 Created`. Then `PATCH /activities/{id}` with `{"is_done":true}`.
**Expected:** `200 OK`, response `is_done: true`, `done_at` populated.

## Step 8 — `raw` excluded from the timeline list
`GET /activities?limit=20` (the default timeline read).
**Expected:** `200 OK`; inspect the JSON response for any activity created in Steps 1-3 — its
`raw` field is `null`/absent in the list response even though `GET /activities/{id}` (Step 4)
returned it populated for the same row. [manual: confirm no raw payload text is visible in the
list response body — a quick visual scan, not automatable without the exact fixture data.]
