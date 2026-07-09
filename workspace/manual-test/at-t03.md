# AT-T03 manual UAT — listActivities timeline read

Prereqs: `make uat_env UAT_SLUG=at-t03` running; a seeded workspace with activity data; an auth token
for a role with `activity` read access, such as `admin`.

Adjust these placeholders to your environment:
- Base URL: `$BASE`
- Authorization token: `$TOKEN`
- Workspace ID: `$WORKSPACE_ID`
- Deal ID: `$DEAL_ID`
- User ID: `$USER_ID`

## Step 1 — Unfiltered list, newest first
`curl -s -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" "$BASE/activities?limit=10"`

Expected: `200 OK`; `data[]` is sorted by newest `occurred_at` first, and `page.next_cursor` is present if more than 10 rows exist.

## Step 2 — `kind` filter
`curl -s -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" "$BASE/activities?kind=email&limit=10"`

Expected: `200 OK`; every `data[].kind` is `email`.

## Step 3 — Entity-scoped call shape still works
`curl -s -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" "$BASE/activities?entity_type=deal&entity_id=$DEAL_ID"`

Expected: `200 OK`; only activities linked to `$DEAL_ID` via `activity_link` are returned.

## Step 4 — `assignee_id` filter
`curl -s -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" "$BASE/activities?assignee_id=$USER_ID"`

Expected: `200 OK`; every returned task row has `assignee_id == $USER_ID`.

## Step 5 — `q` full-text filter
`curl -s -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" "$BASE/activities?q=proposal"`

Expected: `200 OK`; every returned row matches the `proposal` search term in subject or body.

## Step 6 — `sort` outside vocabulary
`curl -s -i -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WORKSPACE_ID" "$BASE/activities?sort=bogus_field"`

Expected: `422 Unprocessable Entity`; response code is `sort_field_not_allowed`.

## Step 7 — Cursor pagination over more than one page
Call the list endpoint with `limit=2` repeatedly, following `page.next_cursor` each time.

Expected: every row across all pages is unique, and the last page has no `next_cursor`.

## Step 8 — `include_archived`
Archive one activity, then list without and with `include_archived=true`.

Expected: archived rows are excluded by default and included when `include_archived=true`, with `archived_at` set.

All steps above are [auto] and are covered by this ticket's integration tests. This guide exists so a human can spot-check the same behavior against the live stack and so the swarm UAT runner can execute the checklist verbatim.
