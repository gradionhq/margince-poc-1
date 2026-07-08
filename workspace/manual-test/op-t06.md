# OP-T06 Live UAT Guide

Run this only after the stack is up with:

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

The commands below assume:

- the API is listening on `http://localhost:8080`
- the database is reachable through `DATABASE_URL`
- you can act as a tenant by sending `X-Workspace-Id` + `X-User-ID`
- Python 3 is available for a few small JSON extractions and for minting approval tokens

Use one shell session so the variables exported in the bootstrap block stay available for the later steps.

## Prerequisites: `APPROVAL_TOKEN_SIGNING_SECRET`

Step 3 mints an `X-Approval-Token` for an agent principal. The approval-token seam
reads this env var at runtime, so it must be set **both** where `make run` runs
and in the shell where you run this UAT guide. Add it to `.env.local` before
starting the server:

```bash
# in .env.local (or export before make run)
APPROVAL_TOKEN_SIGNING_SECRET=dev-op-t06-uat-secret
```

Then in this UAT shell:

```bash
export APPROVAL_TOKEN_SIGNING_SECRET=dev-op-t06-uat-secret
```

Any non-empty string works for dev as long as it matches on both sides.

## Bootstrap

```bash
export API_BASE='http://localhost:8080'
export WS_ID='00000000-0000-0000-0000-000000000026'
export USER_ID='00000000-0000-0000-0026-000000000001'
export ROLE_ID='00000000-0000-0000-0026-000000000010'
export DEAL_ID='00000000-0000-0000-0026-000000000020'
export ORG_ID='00000000-0000-0000-0026-000000000030'
export OFFER_ID=''
export OFFER_REV2_ID=''

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO workspace (id, name, slug, base_currency)
  VALUES ('00000000-0000-0000-0000-000000000026', 'OP-T06 UAT', 'op-t06-uat', 'EUR')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO app_user (id, workspace_id, email, display_name)
  VALUES ('00000000-0000-0000-0026-000000000001', '00000000-0000-0000-0000-000000000026', 'op-t06@example.com', 'OP-T06 UAT User')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO role (id, workspace_id, key, is_system, permissions)
  VALUES (
    '00000000-0000-0000-0026-000000000010',
    '00000000-0000-0000-0000-000000000026',
    'op_t06_admin', true,
    '{"deal":{"create":{"row_scope":"all"},"read":{"row_scope":"all"}},"organization":{"create":{"row_scope":"all"},"read":{"row_scope":"all"}},"offer":{"create":{"row_scope":"all"},"read":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}}}'::jsonb
  )
  ON CONFLICT (workspace_id, key) DO UPDATE SET permissions = EXCLUDED.permissions;

INSERT INTO role_assignment (workspace_id, role_id, user_id)
  VALUES ('00000000-0000-0000-0000-000000000026', '00000000-0000-0000-0026-000000000010', '00000000-0000-0000-0026-000000000001')
  ON CONFLICT (role_id, user_id, COALESCE(team_id, '00000000-0000-0000-0000-000000000000'::uuid)) DO NOTHING;

INSERT INTO organization (id, workspace_id, display_name, source, captured_by)
  VALUES ('00000000-0000-0000-0026-000000000030', '00000000-0000-0000-0000-000000000026', 'OP-T06 Buyer GmbH', 'uat', 'human:op-t06')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline (id, workspace_id, name)
  VALUES ('00000000-0000-0000-0026-000000000040', '00000000-0000-0000-0000-000000000026', 'OP-T06 Pipeline')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
  VALUES ('00000000-0000-0000-0026-000000000041', '00000000-0000-0000-0000-000000000026', '00000000-0000-0000-0026-000000000040', 'Proposal', 1, 'open', 30)
  ON CONFLICT (id) DO NOTHING;

INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, amount_minor, currency, organization_id, source, captured_by)
  VALUES ('00000000-0000-0000-0026-000000000020', '00000000-0000-0000-0000-000000000026', 'OP-T06 UAT Deal', '00000000-0000-0000-0026-000000000040', '00000000-0000-0000-0026-000000000041', 0, 'EUR', '00000000-0000-0000-0026-000000000030', 'uat', 'human:op-t06')
  ON CONFLICT (id) DO NOTHING;
SQL
```

Expected: all `psql` statements succeed, or `DO NOTHING` on rerun.

## Step 1: Create a draft offer with one line item

Create the draft offer under the seeded deal:

```bash
export OFFER_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"offer_number":"ANG-OP-T06-0001","currency":"EUR","buyer_org_id":"00000000-0000-0000-0026-000000000030","template_id":null,"source":"uat","captured_by":"human:op-t06"}' \
  "${API_BASE}/deals/${DEAL_ID}/offers" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")"
echo "OFFER_ID=${OFFER_ID}"
```

Add one priced line so render/send/regenerate have real content to work with:

```bash
curl -i -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"position":1,"description":"Consulting package","quantity":2,"unit_price_minor":125000,"discount_pct":0,"tax_rate":19,"source":"uat","captured_by":"human:op-t06"}' \
  "${API_BASE}/offers/${OFFER_ID}/line-items"
```

Expected: `201 Created`.

Verify the offer totals are server-computed:

```bash
curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | python3 -c "import sys,json; o=json.load(sys.stdin); print(o['net_minor'], o['tax_minor'], o['gross_minor'])"
```

Expected: `250000 47500 297500`.

## Step 2: Render the branded PDF

Render the offer:

```bash
curl -i -sS -X POST \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}/render"
```

Expected: `200 OK`.

Confirm the render wrote a PDF asset ref and left the offer in draft:

```bash
curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | python3 -c "import sys,json; o=json.load(sys.stdin); print(o['status'], o['pdf_asset_ref'])"
```

Expected: `draft` and a non-empty `pdf_asset_ref`.

## Step 3: Send as an agent, first without approval, then with approval

Mint an agent passport for this workspace user:

```bash
export AGENT_TOKEN="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"scopes":[],"expires_in_seconds":3600}' \
  "${API_BASE}/passports" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")"
echo "AGENT_TOKEN=${AGENT_TOKEN}"
```

Try to send without `X-Approval-Token`:

```bash
curl -i -sS -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AGENT_TOKEN}" \
  "${API_BASE}/offers/${OFFER_ID}/send"
```

Expected: `403 Forbidden` with `code: approval_required`.

Mint the approval token for the exact offer send, then retry:

```bash
export APPR_TOKEN="$(python3 - <<'PY'
import base64, json, hmac, hashlib, os, time

secret = os.environ["APPROVAL_TOKEN_SIGNING_SECRET"].encode()
payload = {
    "actor": os.environ["USER_ID"],
    "workspace_id": os.environ["WS_ID"],
    "verb": "send_offer",
    "entity_type": "offer",
    "entity_id": os.environ["OFFER_ID"],
    "expires_at": int(time.time()) + 300,
}
raw = json.dumps(payload, separators=(',', ':'), sort_keys=True).encode()
sig = hmac.new(secret, raw, hashlib.sha256).digest()
print(base64.urlsafe_b64encode(raw).decode().rstrip('=') + "." + base64.urlsafe_b64encode(sig).decode().rstrip('='))
PY
)"
echo "APPR_TOKEN=${APPR_TOKEN}"
```

Send with the approval token:

```bash
curl -i -sS -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AGENT_TOKEN}" \
  -H "X-Approval-Token: ${APPR_TOKEN}" \
  "${API_BASE}/offers/${OFFER_ID}/send"
```

Expected: `200 OK`.

Verify the send froze the record:

```bash
curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | python3 -c "import sys,json; o=json.load(sys.stdin); print(o['status'], o['fx_rate_to_base'], o['fx_rate_date'], o['buyer_snapshot'] is not None, o['issuer_snapshot'] is not None)"
```

Expected: `sent`, a non-empty FX rate/date pair, and both snapshot flags `True`.

## Step 4: Regenerate the sent offer into a new draft revision

Regenerate the sent offer:

```bash
export OFFER_REV2_ID="$(curl -sS -X POST \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}/regenerate" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")"
echo "OFFER_REV2_ID=${OFFER_REV2_ID}"
```

Expected: the response is a new offer id, not the original one.

Confirm the prior revision was superseded and the new revision is a draft:

```bash
curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | python3 -c "import sys,json; o=json.load(sys.stdin); print(o['status'], o['revision'])"

curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_REV2_ID}" | python3 -c "import sys,json; o=json.load(sys.stdin); print(o['status'], o['revision'], o['offer_number'])"
```

Expected:

- the original offer prints `superseded 1`
- the regenerated offer prints `draft 2 ANG-OP-T06-0001`

Confirm the regenerated revision kept the line-item clone:

```bash
curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_REV2_ID}/line-items" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['data']))"
```

Expected: `1`.

## Step 5: Confirm the domain events were emitted once each

Check the outbox rows directly:

```bash
psql "$DATABASE_URL" -c "SELECT topic, count(*) FROM event_outbox WHERE entity_id IN ('${OFFER_ID}', '${OFFER_REV2_ID}') GROUP BY topic ORDER BY topic;"
```

Expected:

- one `offer.sent` row for `${OFFER_ID}`
- one `offer.superseded` row for `${OFFER_REV2_ID}`

The render step does not emit a domain event; it only persists `pdf_asset_ref`.
