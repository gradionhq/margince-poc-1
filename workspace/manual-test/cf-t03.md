# CF-T03 — Governed add-field engine: manual/live-UAT guide

Walks a human through adding a "Contract end date" field to companies (organization) via
`curl`/`psql`, and proves the refusal + approval-gate paths. Mirrors cf-t01.md/cf-t02.md's structure.

Prereqs: `make uat_env UAT_SLUG=cf-t03` (or `make run` against a local stack), `psql` on PATH, an
admin session cookie/passport for the seeded `admin@example.com` workspace.

## 1. [auto] Add a text field to companies as a human admin

```bash
curl -s -X POST http://localhost:8080/custom-fields \
  -H "Content-Type: application/json" -b cookies.txt \
  -d '{"object":"organization","label":"Contract end date","type":"date","source":"ui","captured_by":"human:<admin-user-id>"}'
```
Expected: `201` with a JSON body containing `"column_name":"cf_contract_end_date"`, `"status":"active"`,
`"object":"organization"`, and a `"version":1`. No approval token was required — the human's direct
call is itself the approval.

## 2. [live] Confirm the real column exists and is queryable

```bash
psql "$DATABASE_URL" -c "\d organization" | grep cf_contract_end_date
psql "$DATABASE_URL" -c "SELECT column_name, data_type FROM information_schema.columns WHERE table_name='organization' AND column_name='cf_contract_end_date';"
```
Expected: the column is listed on `organization` with `data_type = date` — a real, indexable column,
not a metadata row.

## 3. [live] Confirm the catalog row and audit entry

```bash
psql "$DATABASE_URL" -c "SELECT object, slug, label, type, status, column_name FROM custom_field WHERE column_name='cf_contract_end_date';"
psql "$DATABASE_URL" -c "SELECT actor_type, action, entity_type FROM audit_log WHERE entity_type='custom_field' ORDER BY occurred_at DESC LIMIT 1;"
```
Expected: exactly one `custom_field` row (`status=active`) and exactly one `audit_log` row
(`actor_type=human`, `action=create`, `entity_type=custom_field`) — the add is in the audit trail.

## 4. [auto] Structural request refused

```bash
curl -s -X POST http://localhost:8080/custom-fields \
  -H "Content-Type: application/json" -b cookies.txt \
  -d '{"object":"organization","label":"Link to parent account","type":"text","source":"ui","captured_by":"human:<admin-user-id>"}'
```
Expected: `422` with `"code":"structural_change_refused"` and `"details":{"route":"source_development_path"}`
— never silently accepted, never scaffolded into code.

## 5. [live] Agent without an approval token is refused

Raw `X-User-ID`/`X-Workspace-ID` dev-proxy headers (`backend/cmd/api/routes.go`'s `workspaceWrap`)
are a pre-auth dev-bypass convenience for a human session, not a way to impersonate an agent — they
can never set `IsAgent:true` (see `crmctx.Principal` construction in `workspaceWrap`). To genuinely
exercise the agent path, mint a Passport as the admin first: `LookupPassport`
(`internal/modules/identity/adapters/session_verifier.go`) always resolves a Bearer passport token
to `IsAgent:true`, regardless of who granted it.

```bash
# Mint a passport as the human admin (reuses step 1's cookies.txt session).
AGENT_TOKEN=$(curl -s -X POST http://localhost:8080/passports \
  -H "Content-Type: application/json" -b cookies.txt \
  -d '{"scopes":[],"expires_in_seconds":3600}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

curl -s -X POST http://localhost:8080/custom-fields \
  -H "Content-Type: application/json" -H "Authorization: Bearer ${AGENT_TOKEN}" \
  -d '{"object":"deal","label":"Agent-proposed field","type":"text","source":"agent","captured_by":"agent:<agent-user-id>"}'
```
Expected: `403` with `"code":"approval_required"`. `psql` confirms no new `custom_field` row and no new
`audit_log` row were written.

## 6. [manual] Currency and picklist types render correctly on the admin screen

Given the custom-fields admin screen (a later ticket's UI), When an admin picks Currency, Then a
required ISO-4217 currency-code input appears; When Picklist, Then an options editor appears and
removing the last option is blocked. Not exercised by this ticket (frontend is out of scope) —
recorded here as the still-open manual/visual step this API enables.
