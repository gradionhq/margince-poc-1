# RD-T10 Live UAT Guide — Attachments screen (dropzone, scan-status, staged AI-extraction, restricted disclosure)

Exercises the full RD-T10 surface: `frontend/src/features/attachments/` mounted on
`DealDetailPage.tsx`, the additive `Attachment.access` field + restricted-row redaction
(`checksum`/`download_url` withheld, `scan_status` always disclosed), and the three new
`/attachments/{id}/extraction`, `/attachments/{id}/extraction:accept`,
`/attachments/{id}/request-access` endpoints. Traces to AC-attachments-1..9 and STATE-1..5
(`docs/subsystems/records-depth.md` / `docs/quality/acceptance-standards.md`).

## Bring up the stack

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

In a second terminal:

```bash
make fe-dev
```

(Or the per-worktree equivalent: `make uat_env UAT_SLUG=rd-t10` — read its printed handle for the
derived backend/frontend URLs and use those in place of `:8080`/`:5173` below; skip the two
commands above in that case.)

The commands below assume:

- API at `http://localhost:8080`, web client at `http://localhost:5173` (Vite's `/api` proxy
  forwards to the backend on the same port pair — see `frontend/vite.config.ts`)
- `curl`, `jq`, and `psql` (against `$DATABASE_URL`) are on `PATH`
- `make seed-reset` has (re-)applied `backend/seed/dev.sql`, which is reused as-is for every
  fixture this guide needs — no bespoke workspace bootstrap required (seed sufficiency)

## Bootstrap — env vars from the standing dev seed

```bash
export API_BASE='http://localhost:8080'
export FE_BASE='http://localhost:5173'
export WS_ID='00000000-0000-0000-0000-000000000001'
export ADMIN_ID='00000000-0000-0000-0010-000000000001'   # admin@example.com / changeme
export REP_ID='00000000-0000-0000-0010-000000000002'     # rep@example.com / changeme
export READONLY_ID='00000000-0000-0000-0010-000000000003' # readonly@example.com / changeme
export DEAL_REP_ID='00000000-0000-0000-0042-000000000001'    # "Acme Expansion" — owned by rep
export DEAL_ADMIN_ID='00000000-0000-0000-0042-000000000002'  # "Globex Renewal" — owned by admin
```

Confirm the two seeded deals exist and carry the expected owners (fail fast if `seed-reset` didn't
apply):

```bash
psql "$DATABASE_URL" -c "SELECT id, name, owner_id FROM deal WHERE id IN ('${DEAL_REP_ID}', '${DEAL_ADMIN_ID}');"
```

Expected: two rows — `Acme Expansion` with `owner_id = ${REP_ID}`, `Globex Renewal` with
`owner_id = ${ADMIN_ID}`. Both are used below to exercise "own"-scope visibility without seeding
anything new.

---

## Step 1 [live]: Upload via the dropzone — Scanning → Clean, byte-exact, "uploaded by you"

Log in as the deal owner:

1. Open `${FE_BASE}/login`, sign in as `rep@example.com` / `changeme`.
2. Navigate to `${FE_BASE}/deals/${DEAL_REP_ID}` ("Acme Expansion").
3. Scroll the left column past the Tasks card to the new **Attachments** section.

Create a small local file and note its exact byte size:

```bash
printf 'RD-T10 UAT sample attachment.\n' > /tmp/rd-t10-qa-notes.txt
wc -c /tmp/rd-t10-qa-notes.txt
```

4. Drag `/tmp/rd-t10-qa-notes.txt` onto the dropzone (or click it and browse to the file).

**Expected:** a row for `rd-t10-qa-notes.txt` appears **immediately** with a neutral
"Scanning…" chip; a toast reads "virus scan in progress" (AC-attachments-2). Confirm the row was
actually persisted:

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments?entity_type=deal&entity_id=${DEAL_REP_ID}" \
  | jq '.data[] | select(.filename=="rd-t10-qa-notes.txt")'
```

Expected: exactly one row, `scan_status: "scanning"`, `byte_size` equal to the `wc -c` output
above, `content_type: "text/plain"`, `source: "ui"` (the dropzone's own send value — matches the
`source: ui` convention used by every other UI-driven write in this schema; `source` is a free-form
string, not an enum, so this is a convention, not a schema constraint), `captured_by:
"human:${REP_ID}"`, `download_url: null`, `checksum` present-or-null (browser upload need not
compute one).

Capture its id:

```bash
export ATT_CLEAN_ID="$(curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments?entity_type=deal&entity_id=${DEAL_REP_ID}" \
  | jq -r '.data[] | select(.filename=="rd-t10-qa-notes.txt") | .id')"
echo "ATT_CLEAN_ID=${ATT_CLEAN_ID}"
```

**Known limitation, not a bug (mirrors `workspace/manual-test/rd-t05.md` Step 3):** no real
virus-scanning product is integrated (RD-PARAM-5); nothing in the live stack ever flips
`scan_status` on its own. Simulate the scanner's verdict directly:

```bash
psql "$DATABASE_URL" -c "UPDATE attachment SET scan_status='clean' WHERE id='${ATT_CLEAN_ID}';"
```

Within ~3s (the panel's own scanning-row poll interval) the row on screen flips to a green
"Clean" chip, showing the same byte size, "uploaded by you" provenance, and a timestamp; a toast
confirms the file was attached and written to the timeline. Re-poll to confirm the flip
server-side too:

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments/${ATT_CLEAN_ID}" | jq '{scan_status, download_url}'
```

Expected: `scan_status: "clean"`, `download_url` now a non-null presigned URL string.

---

## Step 2 [live]: `access: "visible"` on the uploader's own list

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments?entity_type=deal&entity_id=${DEAL_REP_ID}" \
  | jq '.data[] | {id, filename, access, scan_status}'
```

Expected: `200 OK`; every row — including `${ATT_CLEAN_ID}` — carries `access: "visible"` (rep owns
`Acme Expansion`, so `RecordVisible` returns true for rep's own row_scope="own" read). This is the
"restricted" field's negative case, proven before Step 7 exercises the positive one.

---

## Step 3 [live]: Download — bytes flow, client-side toast, existing download-audit unchanged

Still logged in as `rep@example.com` on `${FE_BASE}/deals/${DEAL_REP_ID}`, click **Download** on
the `rd-t10-qa-notes.txt` row.

**Expected:** the browser downloads the file (the presigned `download_url` opens/saves); a
client-side toast reads "Downloaded — access logged" **synchronously on click** — no extra
network round-trip fires for the toast itself (Global Constraint 11). Confirm
`download_audit.go`'s existing server-side write-on-read still fired exactly once, unlinked to any
new mechanism (RD-T05's own precedent, `workspace/manual-test/rd-t05.md` Step 7):

```bash
psql "$DATABASE_URL" -c "
SELECT a.subject, a.captured_by, al.deal_id
FROM activity a
JOIN activity_link al ON al.activity_id = a.id
WHERE a.subject = 'Attachment downloaded: rd-t10-qa-notes.txt'
  AND a.captured_by = 'system:attachment-download-audit';"
```

Expected: exactly one row, `deal_id = ${DEAL_REP_ID}`. Reloading the page and clicking Download a
second time produces a second such row (one per read that populates `download_url`) — download
audit is unchanged by this ticket, only the client toast is new.

---

## Step 4 [live]: Details drawer — type, byte-exact size, SHA-256, provenance, scan, visibility, timeline id

Click **Details** on the `rd-t10-qa-notes.txt` row.

**Expected:** a right-anchored drawer opens (`role="dialog"`, closes on backdrop click or
Escape) showing, all read-only with no edit controls on provenance/scan (AC-attachments-8):

- Type: `text/plain`
- Size: the exact byte count from Step 1's `wc -c` (not rounded/humanized-only — the raw byte
  count must be present somewhere in the field)
- **SHA-256** (labeled that way in the UI; the underlying field is still `checksum` on the wire —
  confirm via `curl -sS ... "${API_BASE}/attachments/${ATT_CLEAN_ID}" | jq .checksum`)
- Provenance: "uploaded by you" / `human:${REP_ID}`
- Attach timestamp: matches `created_at` from `GET /attachments/${ATT_CLEAN_ID}`
- Scan result: "Clean"
- Visibility scope: "Visible" (from `access: "visible"`)
- Timeline activity id: either a real stored linking-activity id on the `Attachment` row, or — if
  none is stored — a value labeled honestly "closest matching timeline entry" naming the nearest
  `activity_link` row for this deal by `created_at` proximity to the attachment (Step 3's download
  audit row is the nearest candidate right now). It must not be blank, and it must not be a
  fabricated id not traceable to any real `activity`/`activity_link` row — cross-check:

```bash
psql "$DATABASE_URL" -c "
SELECT a.id, a.subject, a.created_at
FROM activity a JOIN activity_link al ON al.activity_id = a.id
WHERE al.deal_id = '${DEAL_REP_ID}'
ORDER BY a.created_at DESC LIMIT 3;"
```

The id shown in the drawer must be one of these rows' `id` (or the drawer must honestly say no
timeline entry exists yet, never an invented id).

Press Escape. Expected: drawer closes, underlying attachments list is unaffected.

---

## Step 5 [live]: `GET /attachments/{id}/extraction` on a fresh attachment — empty-seam default, panel absent

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments/${ATT_CLEAN_ID}/extraction" | jq .
```

Expected: `200 OK`, body exactly `{"fields": [], "omitted": []}` — the honest, permanent V1
production default (`extraction.NoOpExtractor`, Global Constraint 3; no
document-extraction/OCR/LLM pipeline exists or is built by this ticket).

On screen (still `${FE_BASE}/deals/${DEAL_REP_ID}`): the staged AI-extraction panel is **absent
entirely** — no card, no "0 fields" placeholder, no empty state for it (STATE-5). Confirm via
devtools: search the DOM for any element carrying the panel's own heading text
("AI read this file") — zero matches.

---

## Step 6 [auto]: Fixture-backed grounded extraction + accept — no live seeding hook by design

Global Constraint 3 makes production's `Extractor` permanently empty (`NoOpExtractor`); there is
no HTTP/DB seam in the live stack to inject a populated extraction for a demo attachment. The
grounded-fields path (2 grounded + 1 omitted field; "Accept 2 fields" persists to the deal with
per-field audit rows and provenance flip; "Dismiss" writes nothing; "Edit" flips one field's
provenance to human) is proven by the test-only `FixtureExtractor` seam instead:

```bash
go test ./backend/internal/modules/records/... -run TestAttachmentHandler -v
```

Expected: exits `0`; covers (per Task 4's own test list) `GET .../extraction` partitioning into
`fields`/`omitted`, `POST .../extraction:accept` with 2 of 3 fields (no edits) writing
`captured_by: "agent:attachment-extractor"` per accepted field, the same call with one field also
present in `edits` writing `captured_by: "human:<principal>"` for that field only, a non-deal
attachment 422'ing with `unsupported_entity_type`, and an unknown `field_keys` entry 422'ing the
whole request (Global Constraint 8, no partial accept).

```bash
cd frontend && npx vitest run src/features/attachments/components/ExtractionPanel
```

Expected: exits `0`; covers STATE-5 (default fixture → panel renders nothing), the populated
fixture (heading text, source-quote citations, confidence dots, omitted-field copy), "Accept 2
fields" (green post-accept heading, controls removed, toast fires), "Dismiss" (panel removed
locally, no mutation call asserted), and "Edit" (field becomes editable, post-accept copy reflects
"typed-by-you" with the original snippet retained).

---

## Step 7 [live]: Restricted-row disclosure — cross-role (admin uploads, rep reads restricted)

Create an attachment on the deal **rep does not own** (`Globex Renewal`, owned by admin), as
admin:

```bash
export ATT_RESTRICTED_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${ADMIN_ID}" \
  -d '{"entity_type":"deal","entity_id":"'"${DEAL_ADMIN_ID}"'","filename":"margin-analysis.xlsx","content_type":"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet","byte_size":20480,"source":"manual","captured_by":"human:'"${ADMIN_ID}"'"}' \
  "${API_BASE}/attachments" | jq -r '.id')"
echo "ATT_RESTRICTED_ID=${ATT_RESTRICTED_ID}"

psql "$DATABASE_URL" -c "UPDATE attachment SET scan_status='clean' WHERE id='${ATT_RESTRICTED_ID}';"
```

As **admin** (the owner), confirm it is visible with full content fields:

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${ADMIN_ID}" \
  "${API_BASE}/attachments/${ATT_RESTRICTED_ID}" | jq '{access, checksum, download_url, scan_status}'
```

Expected: `access: "visible"`, `download_url` non-null, `scan_status: "clean"`.

Now as **rep** (row_scope `own` on `deal`; does NOT own `Globex Renewal`):

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments/${ATT_RESTRICTED_ID}" | jq .
```

Expected: `200 OK` — **the row is disclosed, not hidden, and not 404**. `access: "restricted"`;
`checksum` and `download_url` are **absent** from the JSON (`omitempty` fields, not merely
`null`) — verify with `jq 'has("checksum"), has("download_url")'` both print `false`;
`scan_status: "clean"` is **present and correct** (Global Constraint 1a: a required,
non-nullable schema field, disclosed on every row as a coarse safety signal, never withheld). Also
confirm the list endpoint discloses (not omits) the same row:

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments?entity_type=deal&entity_id=${DEAL_ADMIN_ID}" \
  | jq '.data[] | select(.id=="'"${ATT_RESTRICTED_ID}"'")'
```

Expected: one matching row (not filtered out of the list), same restricted shape as above.

On screen: open `${FE_BASE}/login`, sign out and sign back in as `rep@example.com` / `changeme`
(or open a private window so the rep session doesn't clobber the earlier one), navigate to
`${FE_BASE}/deals/${DEAL_ADMIN_ID}` ("Globex Renewal"). Expected: the `margin-analysis.xlsx` row
renders in a locked visual state with "restricted"/"not your role" copy; the scan-status chip
still shows "Clean" (Global Constraint 1a — the chip is not hidden alongside the redacted content
fields); the **only** action available on the row is "Request access"; no Download/Details
actions render.

Click "Request access":

```bash
curl -sS -X POST -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments/${ATT_RESTRICTED_ID}/request-access" | jq .
```

Expected (both the browser click and the equivalent curl call): `200 {"requested": true}`; a
toast confirms an audited access request. Confirm the audit row:

```bash
psql "$DATABASE_URL" -c "
SELECT subject, captured_by FROM activity
WHERE subject LIKE 'Access requested:%' AND captured_by = 'human:${REP_ID}';"
```

Expected: exactly one row (two, if both the browser click and the curl call above both ran —
`request-access` is not idempotent by design, each click audits again). Re-`GET` the attachment as
rep once more:

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments/${ATT_RESTRICTED_ID}" | jq '{access}'
```

Expected: `access: "restricted"`, unchanged — requesting access has no side effect on visibility
itself (audit row only, per the spec's explicit "no notification system" scope cut).

---

## Step 8 [live]: Blocked scan-status row — quarantined, no download, reason on demand

Seed a second attachment on `Acme Expansion` (rep's own deal, so this exercises the blocked-scan
branch independent of the restricted-access branch from Step 7) and mark it blocked directly (no
public endpoint transitions scan verdicts, per Step 1's known limitation):

```bash
export ATT_BLOCKED_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  -d '{"entity_type":"deal","entity_id":"'"${DEAL_REP_ID}"'","filename":"old-pricelist.zip","content_type":"application/zip","byte_size":40960,"source":"manual","captured_by":"human:'"${REP_ID}"'"}' \
  "${API_BASE}/attachments" | jq -r '.id')"

psql "$DATABASE_URL" -c "UPDATE attachment SET scan_status='blocked' WHERE id='${ATT_BLOCKED_ID}';"

curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${REP_ID}" \
  "${API_BASE}/attachments/${ATT_BLOCKED_ID}" | jq '{access, scan_status, download_url}'
```

Expected: `access: "visible"` (rep owns this deal — blocked is an orthogonal axis from RBAC
visibility), `scan_status: "blocked"`, `download_url: null` (scan gate withholds it regardless of
visibility, unchanged existing behavior).

On screen (`${FE_BASE}/deals/${DEAL_REP_ID}`, as rep): the `old-pricelist.zip` row shows a red
"Blocked" chip and "Quarantined — not downloadable" plus a reason string; no Download action
renders. Click the "Why was this blocked?" info action. Expected: a toast surfaces the reason text
(a fixed/documented reason string is acceptable — no real scanner exists to source a dynamic one;
the toast must not be blank or say "unknown").

---

## Step 9 [live]: "All / Visible to me" filter + visibility rail

Still as rep, navigate (or stay) on `${FE_BASE}/deals/${DEAL_ADMIN_ID}` ("Globex Renewal" — the
restricted-row fixture from Step 7). Confirm the "All / Visible to me" segmented control is
present, defaulted to "All".

**Expected on "All":** the `margin-analysis.xlsx` row renders (locked, restricted state from
Step 7).

Click **"Visible to me"**.

**Expected:** the restricted row disappears — this is a pure client-side filter over the
already-fetched list (no new network request fires on toggle; confirm via devtools Network tab —
zero new `GET /attachments` calls after the click). Because every attachment mounted under one
`AttachmentsPanel` shares the same bound deal (and therefore the same `RecordVisible` verdict for
a given viewer — `access` is a record-visibility reuse, not a per-file ACL, Global Constraint /
AC-9's own "no per-file ACL UI in V1"), this deal's list is 100% restricted for rep, so "Visible to
me" empties it entirely — expect STATE-1's honest empty copy ("No files attached yet" or an
equivalent non-fabricated message), not a blank panel.

Toggle back to **"All"**: the restricted row reappears (client-side round-trip, still no new
network call).

Confirm the right rail: static copy stating visibility inherits the record's RBAC and that there
is no per-file access-control UI in V1 (e.g. "Who can see files here" mapping role → scope,
matching AC-attachments-9's rail requirement) is present and unaffected by the filter toggle.

---

## Step 10 [manual]: Visual pass — dropzone highlight, drawer motion, forge-token chip colors

1. On `${FE_BASE}/deals/${DEAL_REP_ID}`, drag a file over the dropzone without dropping it.
   Expected: the dropzone visibly highlights (border/background change) while the file is over it,
   and reverts when the drag leaves without a drop.
2. Open Details on any row (Step 4). Expected: the drawer slides/animates in from the right edge
   rather than popping instantly; closing animates out.
3. With devtools open, inspect the computed `background-color`/`color` of each scan-status chip
   (Scanning/Clean/Blocked) and the confidence dot from Step 6's Storybook/vitest fixtures if
   exercised live. Expected: every computed color resolves to a Forge `--gf-*` token value (e.g.
   `var(--gf-success)`-derived), never a raw hex literal — Scanning reads neutral, Clean reads
   green, Blocked reads red, consistent with the semantic state, not an arbitrary palette choice.
4. Toggle your OS/browser to dark mode (or use the app's own theme toggle if one exists) and repeat
   step 3. Expected: chip colors adapt via the same tokens, remain legible, no raw-white/raw-black
   flashes.
