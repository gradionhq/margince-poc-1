# CF-T04 Manual UAT Guide — Field lifecycle: rename, retire, picklist options

Run against the target worktree's backend (`cd backend && go test -tags integration ./...`
proves the automated paths below; this guide is the literal step-by-step for the live-stack gate).

## 1. Rename a text field (🟢, no token)
1. `POST /custom-fields` as a human caller: `{"object":"deal","label":"Renewal date","type":"text","source":"ui","captured_by":"human:<id>"}`.
   **Expected:** 201, body has `column_name` starting `cf_`.
2. `PATCH /custom-fields/{id}` as the same human caller, no `X-Approval-Token`: `{"label":"Renewal date (v2)"}`.
   **Expected:** 200; `label` updated; `column_name` byte-identical to step 1's.
3. Query `audit_log` for `entity_id={id} AND action='update' AND entity_type='custom_field'`.
   **Expected:** exactly one row.

## 2. Retire a field (🟡)
1. `POST /custom-fields` as a human caller with `type=text`. **Expected:** 201.
2. `POST /custom-fields/{id}/retire` as the same human caller, no token.
   **Expected:** 200; `status="retired"`; `archived_at=null`.
3. `SELECT <column_name> FROM <object> WHERE id=...` directly against the database for a row that
   had a value in that column. **Expected:** the value is still there — the column was never dropped.
4. `POST /custom-fields/{id2}/retire` (a second field) as an **agent** caller, no token.
   **Expected:** 403 `code=approval_required`.
5. Repeat with a valid single-use `X-Approval-Token` for that exact (workspace, tool, diff).
   **Expected:** 200.

## 3. Edit picklist options (🟡, CHECK regeneration)
1. `POST /custom-fields` with `type=picklist`, `options=["direct","reseller"]`. **Expected:** 201.
2. `PATCH /custom-fields/{id}/options` as a human caller: `{"options":["direct","marketplace"]}`.
   **Expected:** 200; `options` reflects the new set.
3. Directly `UPDATE <object> SET <column_name>='marketplace' WHERE id=...`.
   **Expected:** succeeds (new option satisfies the regenerated CHECK).
4. Directly `UPDATE <object> SET <column_name>='reseller' WHERE id=...`.
   **Expected:** fails — CHECK constraint violation (the removed option is now rejected).
5. `PATCH /custom-fields/{id}/options` with `{"options":[]}`.
   **Expected:** 422, `detail: "A picklist needs at least one option"`.

## 4. Never a destructive path
1. Attempt `PATCH /custom-fields/{id}/options` against a `type=text` field's id.
   **Expected:** 422 `code=not_picklist`, never a 500.
2. Attempt rename/retire/options-edit against a nonexistent id.
   **Expected:** 404 for all three, never a 500.
3. Grep the diff for any new endpoint/handler whose name/shape could drop a column.
   **Expected:** none exists — no code path in this ticket issues `DROP COLUMN`.
