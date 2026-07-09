# CF-T05 Manual/Live UAT Guide — core-field parity + catalog-extensible sort/filter vocabulary

Prereqs: `make infra-up migrate-up seed-reset run` for the local stack, or `make uat_env UAT_SLUG=cf-t05`
for the isolated live-UAT stack. Use a seeded human workspace member (`X-Workspace-ID`/`X-User-ID`
headers, or the stack's session auth) with access to `/people`, `/organizations`, and `/deals`.
Have one workspace with at least one active `cf_*` custom field on each of `person`, `organization`,
and `deal`, and one retired custom field on `deal` for the retirement checks.

## Step 1 - People: custom fields round-trip on list/get/create/update

1. Create a person with a custom field in the request body, for example:
   ```json
   {"full_name":"CF-T05 Person","emails":[{"email":"cft05-person@example.com","email_type":"work","is_primary":true}],"cf_person_note":"hello","source":"ui","captured_by":"human:uat"}
   ```
   Expected: `201 Created`, response includes `cf_person_note: "hello"` at the top level.
2. `GET /people?sort=cf_person_note` and `GET /people?sort=-cf_person_note`.
   Expected: `200 OK`; the active custom field is accepted in the sort vocabulary and returns in the list rows.
3. `GET /people/{id}` for the row created in step 1.
   Expected: `200 OK`; the same `cf_person_note` value is present, and the existing `relationships`,
   `deals`, and `activities` arrays are still present.
4. `PATCH /people/{id}` with `{"cf_person_note":"updated"}`.
   Expected: `200 OK`; response includes `cf_person_note: "updated"`.
5. Repeat the list sort with a retired or unknown `cf_*` key.
   Expected: `422`, `code: sort_field_not_allowed`.

## Step 2 - Organizations: custom fields and exact-match filtering

1. Create an organization with a custom field in the request body, for example:
   ```json
   {"display_name":"CF-T05 Org","cf_org_segment":"enterprise","source":"ui","captured_by":"human:uat"}
   ```
   Expected: `201 Created`, response includes `cf_org_segment: "enterprise"`.
2. `GET /organizations?cf_org_segment=enterprise`.
   Expected: `200 OK`, only live organizations with that exact custom-field value are returned.
3. `GET /organizations?sort=cf_org_segment` and `GET /organizations?sort=-cf_org_segment`.
   Expected: `200 OK`; the active custom field is accepted in the sort vocabulary.
4. `GET /organizations/{id}` for the row created in step 1.
   Expected: `200 OK`; the top-level `cf_org_segment` key is present and the existing sibling arrays are still present.
5. `PATCH /organizations/{id}` with `{"cf_org_segment":"updated"}`.
   Expected: `200 OK`; the custom field updates in place.
6. Query a retired or never-existent `cf_*` filter key.
   Expected: `422`, `code: filter_field_not_allowed`.

## Step 3 - Deals: custom fields, sort/filter, and retired-field rejection

1. Create a deal with an active custom field in the request body, for example:
   ```json
   {"name":"CF-T05 Deal","pipeline_id":"<seeded-pipeline-id>","stage_id":"<seeded-stage-id>","cf_deal_score":42,"source":"ui","captured_by":"human:uat"}
   ```
   Expected: `201 Created`, response includes `cf_deal_score: 42`.
2. `GET /deals?sort=cf_deal_score`, `GET /deals?sort=-cf_deal_score`, and `GET /deals?cf_deal_score=42`.
   Expected: `200 OK`; the active custom field is accepted for sort and exact-match filter.
3. `GET /deals/{id}` for the row created in step 1.
   Expected: `200 OK`; the top-level `cf_deal_score` key is present and the existing `stakeholders`
   and `timeline` arrays are still present.
4. `PATCH /deals/{id}` with `{"cf_deal_score":43}`.
   Expected: `200 OK`; the value updates.
5. Query a retired `cf_*` field on deals, or sort by a retired custom field.
   Expected: `422`, `code: filter_field_not_allowed` for filters and `code: sort_field_not_allowed` for sorts.

## Step 4 - Retired custom fields are omitted from the wire

1. Retire one of the deal custom fields used above.
2. `GET /deals/{id}` for a deal that still stores a value in that retired column.
   Expected: `200 OK`; the retired `cf_*` key is absent from the response body.
3. `GET /deals?sort=<retired-cf-key>` and `GET /deals?<retired-cf-key>=42`.
   Expected: `422` in both cases.

## Step 5 - Live-stack checks

1. Re-run the same requests on the live-UAT stack produced by `make uat_env UAT_SLUG=cf-t05`.
   Expected: the same status codes and response shapes as the local stack.
2. Run the project gate for the branch.
   Expected: `make check` exits `0`.

## Auto/manual split

Mark a step `[auto]` only if the report can point to an integration test that proves the behavior
for the running stack. The live-UAT gate should still exercise the end-to-end requests above, even
when the underlying behavior is covered by tests.
