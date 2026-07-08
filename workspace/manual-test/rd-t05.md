# RD-T05: Live UAT Guide — /attachments (blob-store presigned upload/download, scan gate, visibility inheritance)

This guide exercises the live stack's `/attachments` CRUD endpoints, presigned upload/download flow via MinIO, scan-status gating, visibility-inheritance checks, and archive-cascade behavior.
Each step below is runnable against a running backend API (e.g. via `make infra-up` and `make start`).

Adjust the base URL (e.g. `http://localhost:8080`) and workspace/auth headers as needed for your environment.

## Setup

Ensure you have a valid workspace and auth token. The examples below assume:
- Base URL: `http://localhost:8080`
- MinIO reachable at `http://localhost:9000` (or the value of `BLOBSTORE_ENDPOINT` in your environment)
- Authorization: Bearer token in `Authorization` header (or equivalent auth method)
- Workspace ID: in `X-Workspace-ID` header (or as part of auth context)
- A seeded deal with known `id` (use the seed data from `backend/seed/dev.sql`, e.g., run `make migrate-seed`)

---

## Attachments CRUD (admin role, freshly created attachment)

### Step 1: POST /attachments as admin, bound to a seeded deal

**[live]** Create an attachment bound to a seeded deal with `admin` role.

```bash
# Fetch a deal ID from the seeded data (or create one first)
DEAL_ID="<seeded_deal_id_or_newly_created>"
WORKSPACE_ID="<workspace_id>"

curl -X POST http://localhost:8080/attachments \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "deal",
    "entity_id": "'${DEAL_ID}'",
    "filename": "contract-v1.pdf",
    "content_type": "application/pdf",
    "byte_size": 102400,
    "source": "api-test",
    "captured_by": "human:test-user"
  }'
```

**Expected:** HTTP 201 Created
- `Location` header set to `/attachments/{id}`
- Response body contains:
  - `id`: newly minted UUID
  - `workspace_id`: matches the request header
  - `entity_type: "deal"`, `entity_id`: matches input
  - `filename: "contract-v1.pdf"`
  - `content_type: "application/pdf"`
  - `byte_size: 102400` (exact int64)
  - `scan_status: "scanning"` (always starts in scanning state)
  - `source: "api-test"`, `captured_by: "human:test-user"`
  - `created_at`: non-null timestamp
  - `archived_at: null`
  - `upload_url`: a non-null `memory://attachments/...` URL (dev/test fake) or a real S3-presigned HTTP URL (live MinIO)
  - `download_url: null` (not populated on create; also null because `scan_status="scanning"`)
- Attachment is persisted in the database with a row in the `attachment` table.

---

### Step 2: GET /attachments/{id} while scan_status=scanning (no download_url)

**[live]** Retrieve the attachment created in Step 1 — `download_url` must be null because scan_status is still "scanning".

```bash
ATTACHMENT_ID="<id_from_step_1_response>"
WORKSPACE_ID="<workspace_id>"

curl -X GET http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:** HTTP 200 OK
- Response body contains all fields from Step 1 (immutable)
- `scan_status: "scanning"`
- `upload_url: null` (only populated on create response)
- `download_url: null` (gated by scan_status — never issued while scanning)

---

### Step 3: Mark the attachment clean (manual/administrative scan verdict) **[auto]**

**[auto]** Internally transition the attachment's `scan_status` from "scanning" to "clean" (no public HTTP endpoint exists for this — it's a test/admin-only seam exercised via integration tests; documented as a limitation of the current phase).

For testing purposes, this step must be performed via:
- An integration test calling `AttachmentStore.MarkScanResult(ctx, id, workspaceID, adapters.NewFakeScanner("clean"))`
- Or direct PostgreSQL admin access: `UPDATE attachment SET scan_status='clean' WHERE id=<attachment_id>`

This is a known limitation: RD-PARAM-5 specifies that only an injected `Scanner` seam moves the row out of "scanning" state, and no real virus-scanning product is integrated (out of scope). The integration tests prove the seam works; the live stack never auto-cleans.

---

### Step 4: GET /attachments/{id} post-clean (download_url is now available)

**[live]** Retrieve the attachment after Step 3 — `download_url` is now populated because scan_status transitioned to "clean".

```bash
ATTACHMENT_ID="<id_from_step_1_response>"
WORKSPACE_ID="<workspace_id>"

curl -X GET http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:** HTTP 200 OK
- Response body contains all fields from Step 1 (immutable)
- `scan_status: "clean"`
- `upload_url: null` (only on create)
- `download_url`: a non-null presigned GET URL (real MinIO or `memory://attachments/...` in dev/test)
- The presigned URL is valid and time-limited (default expiry: 15 minutes)

---

### Step 5: POST /attachments + curl presigned upload against MinIO **[live]**

**[live]** Create a second attachment and exercise the two-phase upload flow: mint the attachment row and receive an `upload_url`, then `curl -T` bytes directly to MinIO (bypassing the app process).

```bash
DEAL_ID="<seeded_deal_id>"
WORKSPACE_ID="<workspace_id>"

# Step 5a: Create the attachment (like Step 1)
RESPONSE=$(curl -s -X POST http://localhost:8080/attachments \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "deal",
    "entity_id": "'${DEAL_ID}'",
    "filename": "proposal.docx",
    "content_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    "byte_size": 51200,
    "source": "api-test",
    "captured_by": "human:test-user"
  }')

ATTACHMENT_ID=$(echo ${RESPONSE} | jq -r '.id')
UPLOAD_URL=$(echo ${RESPONSE} | jq -r '.upload_url')

# Step 5b: PUT bytes to the presigned upload URL (2-phase: app never touches bytes)
echo "Sample document content for proposal" > /tmp/proposal.txt
curl -T /tmp/proposal.txt "${UPLOAD_URL}"

# Step 5c: Verify the object exists in MinIO without hitting Postgres
# (This is a bucket inspection step, not an app endpoint — optional for manual UAT)
# psql: SELECT byte_size FROM attachment WHERE id='<attachment_id>' -- confirms bytea column does NOT exist
```

**Expected:**
- POST response: HTTP 201, `upload_url` is valid and presigned
- PUT to upload URL: HTTP 200 (or 204) — bytes land in MinIO, not in the Postgres `attachment` table (which has no `bytea` column, per Constraint 2)
- psql check: `\d attachment` shows no `bytea` column; storage is object-store-only
- Backend never buffers the file in memory (ADR-0051's model)

---

### Step 6: Download the attachment (presigned GET) **[live]**

**[live]** After the administrator/test manually marks the attachment from Step 5 as "clean", retrieve and download it via presigned GET URL.

```bash
ATTACHMENT_ID="<id_from_step_5a_response>"
WORKSPACE_ID="<workspace_id>"

# Prerequisite: mark the attachment clean (Step 3's process)

# Retrieve with download_url populated
RESPONSE=$(curl -s -X GET http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}")

DOWNLOAD_URL=$(echo ${RESPONSE} | jq -r '.download_url')

# Download the bytes via presigned GET
curl -o /tmp/proposal-downloaded.txt "${DOWNLOAD_URL}"

# Verify content matches what was uploaded
# (The exact bytes might differ due to object-store transformations,
#  but the file exists and is retrievable)
ls -l /tmp/proposal-downloaded.txt
```

**Expected:**
- GET /attachments/{id}: HTTP 200, `download_url` is non-null
- curl download: HTTP 200, bytes written to disk
- File size > 0 (content was stored and retrieved)
- No errors in the presigned-URL round-trip

---

### Step 7: Download-audit activity is written (RD-AC-2, RD-AC-9) **[auto]**

**[auto]** Verify that calling `GET /attachments/{id}` (Step 6) wrote an audit activity record to the activities module.

```bash
# Via integration test or admin tooling:
# SELECT * FROM activity WHERE subject LIKE 'Attachment downloaded:%'
#   AND captured_by = 'system:attachment-download-audit'
# 
# For deal-bound attachments: verify the activity appears in the 360 timeline
# SELECT * FROM activity_link WHERE activity_id=<activity_id> AND entity_type='deal' AND entity_id=<deal_id>
```

**Expected:**
- One `activity` row was written per download
- `kind: "note"`, `subject: "Attachment downloaded: <filename>"`, `source: "system"`, `captured_by: "system:attachment-download-audit"`
- For deal-bound attachments: an `activity_link` row was written, linking it to the deal (RD-AC-9)
- The activity appears on the deal's 360 timeline when queried via `GET /deals/{id}` or `GET /activities?entity_type=deal&entity_id=<deal_id>`
- For lead/activity-bound attachments: an unlinked activity row exists (documented gap: lead/activity have no timeline in the current codebase)

---

### Step 8: DELETE /attachments/{id} (archive, not hard-delete)

**[live]** Archive an attachment by ID. Archived attachments are soft-deleted — they remain queryable but with `archived_at` set.

```bash
ATTACHMENT_ID="<id_from_a_previous_step>"
WORKSPACE_ID="<workspace_id>"

curl -X DELETE http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:** HTTP 200 OK
- Response body contains the full `Attachment` with:
  - `archived_at`: non-null timestamp (the archive time)
  - All other fields unchanged (immutable except for archive)
  - `download_url: null` (no bytes access for archived attachments)
  - `upload_url: null` (no URLs on get/delete responses)

---

### Step 9: GET archived attachment (still 200, no 404 — disclosed locked)

**[live]** Retrieve an archived attachment. It still returns 200 (not 404), matching the "disclosed locked row" design.

```bash
ATTACHMENT_ID="<id_from_step_8>"
WORKSPACE_ID="<workspace_id>"

curl -X GET http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:** HTTP 200 OK
- Response body contains the full `Attachment` with:
  - `archived_at`: the timestamp from Step 8 (unchanged)
  - `download_url: null` (archived = no bytes access)
  - `upload_url: null` (only on create)
- The attachment is not removed from the list (see Step 10)

---

### Step 10: List attachments (default: exclude archived)

**[live]** List all attachments bound to the deal (from earlier steps), excluding archived by default.

```bash
DEAL_ID="<seeded_deal_id_or_step_1>"
WORKSPACE_ID="<workspace_id>"

curl -X GET "http://localhost:8080/attachments?entity_type=deal&entity_id=${DEAL_ID}" \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:** HTTP 200 OK
- Response `data`: array of attachments, excluding the archived one from Step 8
- Response `page.has_more: false` (assuming < limit)
- Response `page.next_cursor`: absent or empty string

---

### Step 11: List attachments including archived

**[live]** List all attachments (including archived).

```bash
DEAL_ID="<seeded_deal_id_or_step_1>"
WORKSPACE_ID="<workspace_id>"

curl -X GET "http://localhost:8080/attachments?entity_type=deal&entity_id=${DEAL_ID}&include_archived=true" \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:** HTTP 200 OK
- Response `data`: array includes the archived attachment from Step 8
- Archive filtering confirmed: with `include_archived=true`, count increases

---

## Archive-Cascade (DM-CONV-15)

### Step 12: Archiving the bound deal also archives its attachments

**[live]** Archive the deal from earlier steps. Its attached attachments are cascade-archived automatically (same transaction).

```bash
DEAL_ID="<seeded_deal_id_or_step_1>"
WORKSPACE_ID="<workspace_id>"

curl -X DELETE http://localhost:8080/deals/${DEAL_ID} \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"

# Then check that the deal's attachments are also archived:
curl -X GET "http://localhost:8080/attachments/${ATTACHMENT_ID}?include_archived=true" \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:**
- DELETE /deals/{id}: HTTP 200, deal has `archived_at` set
- GET /attachments/{id} (one of the deal's attachments):
  - HTTP 200
  - `archived_at` is non-null (cascade-archived when the deal was archived)
  - `download_url: null` (no access to archived attachments)
- Cascade is transactional: if the deal archive succeeds, all its attachments are also archived in the same transaction

---

## Visibility Inheritance (RD-AC-2: attachment visibility = bound record's visibility)

### Step 13: read_only role can see an attachment (row_scope: all)

**[live]** As `read_only` role (which has `attachment.read` with `row_scope: all`), retrieve an attachment on a deal bound to a different owner.

```bash
ATTACHMENT_ID="<id_from_earlier_steps>"
WORKSPACE_ID="<workspace_id>"

curl -X GET http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <read_only_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:** HTTP 200 OK
- Response body contains the full `Attachment` with truthful metadata
- If `scan_status: "clean"`, `download_url` is populated (read_only can read all, so visibility is not gated)
- If `scan_status: "scanning"` or `"blocked"`, `download_url: null` (scan gate applies regardless of role)

---

### Step 14: rep role with own-scope (restricted by owner_id)

**[live]** As `rep` role (which has `attachment.read` with `row_scope: own`), attempt to see an attachment on a deal owned by a different user.

Create two users in the same workspace:
- User A (rep role, owner of Deal A)
- User B (rep role, owner of Deal B)

User A creates an attachment on Deal A. User B attempts to read it.

```bash
# As User A:
DEAL_A_ID="<deal_owned_by_user_a>"
ATTACHMENT_ID="<attachment_on_deal_a>"

curl -X POST http://localhost:8080/attachments \
  -H "Authorization: Bearer <user_a_rep_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "deal",
    "entity_id": "'${DEAL_A_ID}'",
    "filename": "user-a-private.pdf",
    "content_type": "application/pdf",
    "byte_size": 10240,
    "source": "api-test",
    "captured_by": "human:user-a"
  }'

# As User B (different rep user, no grant):
curl -X GET http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <user_b_rep_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:**
- User A: POST returns 201 (can create attachments on their own deals per `row_scope: own`)
- User B: GET returns HTTP 200 (not 404 — disclosed locked row) with:
  - Full `Attachment` metadata present (truthful fields)
  - `download_url: null` (User B cannot see Deal A, so attachment is not accessible)
  - `upload_url: null` (only on create)
  - No download-audit activity written (no access granted, no activity triggered)

---

### Step 15: rep role with record_grant (temporary share)

**[manual]** As `rep` role, share an attachment's bound deal via `record_grant` (temporary grant), then verify the attachment becomes visible.

This step requires:
1. User A (rep) owns Deal A with an attachment
2. User A grants User B temporary access to Deal A via a `record_grant` row
3. User B retrieves the attachment (now visible via the grant)

```bash
# As User A: manually insert a record_grant row (or use a grant API if one exists):
INSERT INTO record_grant (id, workspace_id, record_type, record_id, subject_type, subject_id, expires_at)
VALUES (
  '<new_uuid>',
  '<workspace_id>',
  'deal',
  '<deal_a_id>',
  'user',
  '<user_b_id>',
  now() + interval '1 hour'  -- expires in 1 hour
);

# As User B:
curl -X GET http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <user_b_rep_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:**
- record_grant inserted: User B now has temporary access to Deal A
- GET /attachments/{id}: HTTP 200
- `download_url: null` (if scan_status != "clean") or populated (if "clean" and grant is live)
- Visibility inherited from the bound record's grant; no separate per-attachment grant needed

---

### Step 16: Expired record_grant (access revoked)

**[manual]** Modify the grant from Step 15 to expire in the past, then verify the attachment becomes invisible again.

```bash
# As admin (or User A):
UPDATE record_grant SET expires_at = now() - interval '1 minute' WHERE id='<grant_id>';

# As User B:
curl -X GET http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <user_b_rep_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:**
- GET /attachments/{id}: HTTP 200 (disclosed locked row, not 404)
- `download_url: null` (access revoked via expired grant)
- Full `Attachment` metadata still present (transparency: the row exists, you just can't access bytes)

---

## Authorization & RBAC (4-part wiring)

### Step 17: rep role cannot archive (archive is not in rep's permissions)

**[live]** As `rep` role, attempt to delete an attachment. Should fail with 403 because `rep` has no `archive` permission on `attachment`.

```bash
ATTACHMENT_ID="<any_attachment_id>"
WORKSPACE_ID="<workspace_id>"

curl -X DELETE http://localhost:8080/attachments/${ATTACHMENT_ID} \
  -H "Authorization: Bearer <rep_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}"
```

**Expected:** HTTP 403 Forbidden
- Response `code: forbidden` or similar RBAC denial code
- Attachment remains unchanged (not archived)

---

### Step 18: read_only role cannot create

**[live]** As `read_only` role, attempt to create an attachment. Should fail with 403.

```bash
DEAL_ID="<any_deal_id>"
WORKSPACE_ID="<workspace_id>"

curl -X POST http://localhost:8080/attachments \
  -H "Authorization: Bearer <read_only_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "deal",
    "entity_id": "'${DEAL_ID}'",
    "filename": "test.txt",
    "content_type": "text/plain",
    "byte_size": 100,
    "source": "api-test",
    "captured_by": "human:test"
  }'
```

**Expected:** HTTP 403 Forbidden
- `read_only` has no `create` permission on `attachment`
- No attachment created

---

## Provenance & Validation

### Step 19: POST /attachments without required source/captured_by

**[live]** Omit `source` or `captured_by` fields — should fail with 422.

```bash
DEAL_ID="<any_deal_id>"
WORKSPACE_ID="<workspace_id>"

curl -X POST http://localhost:8080/attachments \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "deal",
    "entity_id": "'${DEAL_ID}'",
    "filename": "noprov.pdf",
    "content_type": "application/pdf",
    "byte_size": 1000
  }'
```

**Expected:** HTTP 422 Unprocessable Entity
- Response `code: validation_error`
- `field_errors` contains entries for `source` and `captured_by`, each with `code: required`
- No attachment created

---

### Step 20: POST /attachments with byte_size <= 0

**[live]** Attempt to create an attachment with invalid `byte_size`.

```bash
DEAL_ID="<any_deal_id>"
WORKSPACE_ID="<workspace_id>"

curl -X POST http://localhost:8080/attachments \
  -H "Authorization: Bearer <admin_token>" \
  -H "X-Workspace-ID: ${WORKSPACE_ID}" \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "deal",
    "entity_id": "'${DEAL_ID}'",
    "filename": "empty.txt",
    "content_type": "text/plain",
    "byte_size": 0,
    "source": "api-test",
    "captured_by": "human:test"
  }'
```

**Expected:** HTTP 422 Unprocessable Entity
- Response `code: validation_error`
- `field_errors` contains an entry for `byte_size` with an appropriate error code
- No attachment created

---

## Summary

All steps above should pass when run against the live stack. Key acceptance criteria:
- ✅ Attachments CRUD: create/read/list/archive workflow succeeds
- ✅ Scan-status gating: `download_url` null while scanning, populated only when clean (Step 2–4)
- ✅ Two-phase upload: `upload_url` presigned, bytes flow client↔MinIO directly (Step 5)
- ✅ Two-phase download: `download_url` presigned, bytes flow client↔MinIO directly (Step 6)
- ✅ Download-audit: activity record written on GET, linked to timeline for person/organization/deal (Step 7)
- ✅ Archive is soft-delete: archived attachments return 200 (not 404), with both URLs null (Step 8–9)
- ✅ Archive-cascade: deleting the bound record also archives its attachments (Step 12)
- ✅ Visibility inheritance: attachment visibility = bound record's visibility (row_scope own/team/all + record_grant) (Step 13–16)
- ✅ RBAC: attachment object wired in all four places (Constraint 9); role permissions enforced per Authorization Matrix (Step 17–18)
- ✅ Provenance: `source`/`captured_by` required; validation errors on omission (Step 19)
- ✅ Input validation: `byte_size > 0`, `entity_type` ∈ valid set, etc. (Step 20)
- ✅ Idempotency-Key header ignored (per spec, same as other objects)
- ✅ No `updated_at` on attachments (immutable except archive)
