# OP-T08 Live UAT Guide — `POST /offers/{id}/accept` (OFFER-WIRE-9, accept semantics)

Run this only after the stack is up with:

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

The commands below assume:

- the API is listening on `http://localhost:8080`
- the database is reachable through `DATABASE_URL`
- you can act as a human tenant principal by sending `X-Workspace-Id` + `X-User-ID`
- you can mint an agent passport via `POST /passports` and act as an agent principal with
  `Authorization: Bearer <token>`
- `python3` and `jq` are available for JSON extraction and for hand-minting the approval token
  used in Steps 8-9

Use one shell session so the variables exported in the bootstrap block stay available for the
later steps.

## Prerequisites: `APPROVAL_TOKEN_SIGNING_SECRET`

Steps 8-9 mint an `X-Approval-Token` for an agent principal. The approval-token seam reads this
env var at runtime, so it must be set **both** where `make run` runs and in the shell where you
run this UAT guide. Add it to `.env.local` before starting the server:

```bash
# in .env.local (or export before make run)
APPROVAL_TOKEN_SIGNING_SECRET=dev-op-t08-uat-secret
```

Then in this UAT shell:

```bash
export APPROVAL_TOKEN_SIGNING_SECRET=dev-op-t08-uat-secret
```

Any non-empty string works for dev as long as it matches on both sides.

## Bootstrap

```bash
export API_BASE='http://localhost:8080'
export WS_ID='00000000-0000-0000-0000-000000000038'
export USER_ID='00000000-0000-0038-0000-000000000001'
export ROLE_ID='00000000-0000-0038-0000-000000000010'
export DEAL_ID='00000000-0000-0038-0000-000000000020'
export ORG_ID='00000000-0000-0038-0000-000000000030'
export PIPELINE_ID='00000000-0000-0038-0000-000000000040'
export STAGE_ID='00000000-0000-0038-0000-000000000041'
export OFFER_ID=''
export OFFER2_ID=''
export OFFER3_ID=''
export AGENT_TOKEN=''

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO workspace (id, name, slug, base_currency)
  VALUES ('00000000-0000-0000-0000-000000000038', 'OP-T08 UAT', 'op-t08-uat', 'EUR')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO app_user (id, workspace_id, email, display_name)
  VALUES ('00000000-0000-0038-0000-000000000001', '00000000-0000-0000-0000-000000000038', 'op-t08@example.com', 'OP-T08 UAT User')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO role (id, workspace_id, key, is_system, permissions)
  VALUES (
    '00000000-0000-0038-0000-000000000010',
    '00000000-0000-0000-0000-000000000038',
    'op_t08_admin', true,
    '{"deal":{"create":{"row_scope":"all"},"read":{"row_scope":"all"},"update":{"row_scope":"all"}},"organization":{"create":{"row_scope":"all"},"read":{"row_scope":"all"}},"offer":{"create":{"row_scope":"all"},"read":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}}}'::jsonb
  )
  ON CONFLICT (workspace_id, key) DO UPDATE SET permissions = EXCLUDED.permissions;

INSERT INTO role_assignment (workspace_id, role_id, user_id)
  VALUES ('00000000-0000-0000-0000-000000000038', '00000000-0000-0038-0000-000000000010', '00000000-0000-0038-0000-000000000001')
  ON CONFLICT (role_id, user_id, COALESCE(team_id, '00000000-0000-0000-0000-000000000000'::uuid)) DO NOTHING;

INSERT INTO organization (id, workspace_id, name, source, captured_by)
  VALUES ('00000000-0000-0038-0000-000000000030', '00000000-0000-0000-0000-000000000038', 'OP-T08 Buyer GmbH', 'uat', 'human:op-t08')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline (id, workspace_id, name)
  VALUES ('00000000-0000-0038-0000-000000000040', '00000000-0000-0000-0000-000000000038', 'OP-T08 Pipeline')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
  VALUES ('00000000-0000-0038-0000-000000000041', '00000000-0000-0000-0000-000000000038', '00000000-0000-0038-0000-000000000040', 'Proposal', 1, 'open', 30)
  ON CONFLICT (id) DO NOTHING;

INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, amount_minor, currency, organization_id, source, captured_by)
  VALUES ('00000000-0000-0038-0000-000000000020', '00000000-0000-0000-0000-000000000038', 'OP-T08 UAT Deal', '00000000-0000-0038-0000-000000000040', '00000000-0000-0038-0000-000000000041', 0, 'EUR', '00000000-0000-0038-0000-000000000030', 'uat', 'human:op-t08')
  ON CONFLICT (id) DO NOTHING;
SQL
```

Expected: all `psql` statements succeed, or `DO NOTHING` on rerun. The seeded deal starts with
`amount_minor = 0`, `currency = 'EUR'` so Step 3's sync check has an unambiguous before/after.

Mint an agent passport for this workspace user once (reused in Steps 8-9):

```bash
export AGENT_TOKEN="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"scopes":[],"expires_in_seconds":3600}' \
  "${API_BASE}/passports" | jq -r '.token')"
echo "AGENT_TOKEN=${AGENT_TOKEN}"
```

Expected: a non-empty opaque bearer token.

A small helper function for minting a single-use `accept_offer` approval token bound to a
specific offer id (used in Step 9 — mirrors `op-t06.md`'s send-token minting exactly, only the
`tool` and `diff` fields differ):

```bash
mint_accept_token() {
  local offer_id="$1"
  python3 - "${offer_id}" "uat-op-t08-jti-${offer_id}" <<'PYEOF'
import hmac as _hmac, hashlib, base64, json, os, sys
from datetime import datetime, timezone, timedelta

def b64u(b):
    return base64.urlsafe_b64encode(b).rstrip(b'=').decode()

root = os.environ['APPROVAL_TOKEN_SIGNING_SECRET']
ws_id = os.environ['WS_ID']
offer_id, jti = sys.argv[1], sys.argv[2]

# Per-workspace key: HMAC-SHA256(root, workspace_id)
key = _hmac.new(root.encode(), ws_id.encode(), hashlib.sha256).digest()

# diff_hash: SHA-256 of the canonical (sort_keys) JSON of the bound fields —
# must match the handler's own diffFields := map[string]any{"offer_id": id}.
diff = {'offer_id': offer_id}
diff_hash = b64u(hashlib.sha256(
    json.dumps(diff, sort_keys=True, separators=(',', ':')).encode()
).digest())

exp = (datetime.now(timezone.utc) + timedelta(minutes=5)).strftime('%Y-%m-%dT%H:%M:%SZ')
claims = {'jti': jti, 'approval_id': 'uat-' + jti, 'workspace_id': ws_id,
          'tool': 'accept_offer', 'diff_hash': diff_hash, 'exp': exp, 'single_use': True}

hdr = b64u(json.dumps({'alg': 'HS256', 'typ': 'JWT'}, separators=(',', ':')).encode())
pay = b64u(json.dumps(claims, separators=(',', ':')).encode())
sig = _hmac.new(key, (hdr + '.' + pay).encode(), hashlib.sha256).digest()
print(hdr + '.' + pay + '.' + b64u(sig))
PYEOF
}
```

## Step 1 [live]: Create a deal offer, add a line item, send it

Create the draft offer under the seeded deal:

```bash
export OFFER_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"offer_number":"ANG-OP-T08-0001","currency":"EUR","buyer_org_id":"00000000-0000-0038-0000-000000000030","source":"uat","captured_by":"human:op-t08"}' \
  "${API_BASE}/deals/${DEAL_ID}/offers" | jq -r '.id')"
echo "OFFER_ID=${OFFER_ID}"
```

Add one priced line so `gross_minor` is non-zero:

```bash
curl -i -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"position":1,"description":"Consulting package","quantity":2,"unit_price_minor":125000,"discount_pct":0,"tax_rate":19,"source":"uat","captured_by":"human:op-t08"}' \
  "${API_BASE}/offers/${OFFER_ID}/line-items"
```

Expected: `201 Created`.

Confirm the totals are server-computed, then send the offer as the human principal (no
`X-Approval-Token` needed — `toolgate.Enforce` is a no-op for a non-agent principal):

```bash
curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | jq '{status, net_minor, tax_minor, gross_minor}'

curl -i -sS -X POST \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}/send"

curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | jq '{status, gross_minor}'
```

Expected: the pre-send `GET` shows `net_minor: 250000, tax_minor: 47500, gross_minor: 297500`.
`POST /offers/{id}/send` returns `200 OK`. The follow-up `GET` shows `status: "sent"`,
`gross_minor: 297500`.

## Step 2 [live]: Accept as the human principal — 200, status flips, `accepted_at` populated

```bash
curl -i -sS -X POST \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}/accept"

curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}" | jq '{status, accepted_at, gross_minor, currency}'
```

Expected: `POST /offers/{id}/accept` returns `200 OK` with `status: "accepted"` in the response
body. The follow-up `GET` shows `status: "accepted"`, a non-null `accepted_at`, and
`gross_minor: 297500, currency: "EUR"` unchanged from Step 1.

## Step 3 [live]: The parent deal's `amount_minor`/`currency` synced from the offer's `gross_minor`

```bash
curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/deals/${DEAL_ID}" | jq '{amount_minor, currency}'
```

Expected: `amount_minor: 297500, currency: "EUR"` — exactly the accepted offer's `gross_minor`/
`currency` (the deal started at `amount_minor: 0` in the bootstrap seed).

## Step 4 [live]: `offer.accepted` and `deal.updated` share one `correlation_id`

```bash
psql "$DATABASE_URL" -c "
SELECT topic, entity_id, count(*)
FROM event_outbox
WHERE (topic = 'offer.accepted' AND entity_id = '${OFFER_ID}')
   OR (topic = 'deal.updated' AND entity_id = '${DEAL_ID}')
GROUP BY topic, entity_id
ORDER BY topic;"

psql "$DATABASE_URL" -t -c "
SELECT payload->>'correlation_id' FROM event_outbox WHERE topic = 'offer.accepted' AND entity_id = '${OFFER_ID}';"

psql "$DATABASE_URL" -t -c "
SELECT payload->>'correlation_id' FROM event_outbox WHERE topic = 'deal.updated' AND entity_id = '${DEAL_ID}';"

export CORR_ID="$(psql "$DATABASE_URL" -t -A -c "
SELECT payload->>'correlation_id' FROM event_outbox WHERE topic = 'offer.accepted' AND entity_id = '${OFFER_ID}';")"
echo "CORR_ID=${CORR_ID}"
```

Expected: the first query returns exactly two rows — `deal.updated | <DEAL_ID> | 1` and
`offer.accepted | <OFFER_ID> | 1`. The second and third queries print the SAME non-empty
`correlation_id` value (copy both outputs and diff by eye, or `diff <(...) <(...)`).

## Step 5 [live]: Exactly one `audit_log` row per mutated entity

```bash
psql "$DATABASE_URL" -c "
SELECT actor_type, actor_id, action, entity_type, entity_id, after->>'correlation_id' AS correlation_id
FROM audit_log
WHERE after->>'correlation_id' = '${CORR_ID}'
  AND ((entity_type = 'offer' AND entity_id = '${OFFER_ID}')
    OR (entity_type = 'deal' AND entity_id = '${DEAL_ID}'))
ORDER BY entity_type;"
```

Expected: exactly two rows — one `entity_type = 'deal'` row (the `amount_minor`/`currency` sync)
and one `entity_type = 'offer'` row (the status flip) — both `action = 'update'`, both carrying
the SAME `correlation_id` in their `after` JSON as Step 4's `event_outbox` rows. The query is
scoped to this accept call's own `${CORR_ID}` (captured in Step 4) so it returns exactly these 2
rows even though earlier steps (e.g. the offer's own create) already wrote other audit_log rows
for the same `entity_id`s.

## Step 6 [live]: Re-accepting an already-accepted offer — 409 `offer_not_acceptable`

```bash
curl -i -sS -X POST \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER_ID}/accept"
```

Expected: `409 Conflict` with problem `code: offer_not_acceptable`.

## Step 7 [live]: Accepting a draft (never-sent) offer — 409 `offer_not_acceptable`

```bash
export OFFER2_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"offer_number":"ANG-OP-T08-0002","currency":"EUR","buyer_org_id":"00000000-0000-0038-0000-000000000030","source":"uat","captured_by":"human:op-t08"}' \
  "${API_BASE}/deals/${DEAL_ID}/offers" | jq -r '.id')"
echo "OFFER2_ID=${OFFER2_ID}"

curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER2_ID}" | jq '{status}'

curl -i -sS -X POST \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER2_ID}/accept"
```

Expected: the `GET` shows `status: "draft"`. `POST /offers/{id}/accept` returns `409 Conflict`
with problem `code: offer_not_acceptable`.

## Step 8 [live]: Agent principal without `X-Approval-Token` — 403 `approval_required`

Create and send a third offer (freshly `sent`, untouched by Steps 2/6) for the agent-gating
steps:

```bash
export OFFER3_ID="$(curl -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"offer_number":"ANG-OP-T08-0003","currency":"EUR","buyer_org_id":"00000000-0000-0038-0000-000000000030","source":"uat","captured_by":"human:op-t08"}' \
  "${API_BASE}/deals/${DEAL_ID}/offers" | jq -r '.id')"

curl -i -sS -X POST \
  -H "Content-Type: application/json" \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  -d '{"position":1,"description":"Consulting package","quantity":1,"unit_price_minor":100000,"discount_pct":0,"tax_rate":19,"source":"uat","captured_by":"human:op-t08"}' \
  "${API_BASE}/offers/${OFFER3_ID}/line-items"

curl -i -sS -X POST \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER3_ID}/send"

curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER3_ID}" | jq '{status}'
```

Expected: the line-item `POST` returns `201`, `send` returns `200`, and the follow-up `GET`
shows `status: "sent"`.

Now attempt to accept as an agent principal (`Authorization: Bearer $AGENT_TOKEN`, no
`X-Approval-Token`):

```bash
curl -i -sS -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AGENT_TOKEN}" \
  "${API_BASE}/offers/${OFFER3_ID}/accept"
```

Expected: `403 Forbidden` with problem `code: approval_required`. `GET /offers/{id}` still shows
`status: "sent"` (the accept never ran).

## Step 9 [live]: Agent principal with a valid single-use `accept_offer` token — 200, then replay — 403

Mint the token and accept:

```bash
export APPR_TOKEN="$(mint_accept_token "${OFFER3_ID}")"
echo "APPR_TOKEN=${APPR_TOKEN}"

curl -i -sS -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AGENT_TOKEN}" \
  -H "X-Approval-Token: ${APPR_TOKEN}" \
  "${API_BASE}/offers/${OFFER3_ID}/accept"

curl -sS \
  -H "X-Workspace-Id: ${WS_ID}" \
  -H "X-User-ID: ${USER_ID}" \
  "${API_BASE}/offers/${OFFER3_ID}" | jq '{status, accepted_at}'
```

Expected: `200 OK` with `status: "accepted"` in the response body. The follow-up `GET` confirms
`status: "accepted"` and a non-null `accepted_at`.

Replay the SAME (now-consumed) token:

```bash
curl -i -sS -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AGENT_TOKEN}" \
  -H "X-Approval-Token: ${APPR_TOKEN}" \
  "${API_BASE}/offers/${OFFER3_ID}/accept"
```

Expected: `403 Forbidden` (`code: approval_token_invalid` — the token's `single_use` claim was
already consumed by the prior call). The offer stays `accepted`; no second
`offer.accepted`/`deal.updated` pair is written:

```bash
psql "$DATABASE_URL" -c "
SELECT topic, count(*)
FROM event_outbox
WHERE topic IN ('offer.accepted', 'deal.updated') AND entity_id = '${OFFER3_ID}'
GROUP BY topic;"
```

Expected: exactly one `offer.accepted` row for `${OFFER3_ID}` (the replay attempt inserted
nothing).

## Step 10 [auto]: `DealStore.Update` honors `amount_minor`/`currency` from both `int64` and JSON `float64`

Covered by `TestDealStore_Update_SyncsAmountMinorAndCurrency`
(`backend/internal/modules/deals/adapters/store_deal_update_amount_test.go`) — this is the
Task-1 write path `OfferStore.Accept` calls into for Steps 2-3/9's deal sync above.

```bash
export TEST_DATABASE_URL="$DATABASE_URL"
go test -tags=integration ./backend/internal/modules/deals/adapters -run TestDealStore_Update_SyncsAmountMinorAndCurrency -v
```

Expected: exits `0`. A direct `int64` `amount_minor`/`currency` update round-trips correctly, and
a second update with a JSON-decoded `float64` `amount_minor` (the real `PATCH /deals/{id}`
handler's own shape) round-trips identically while leaving `currency` untouched (`COALESCE`
leaves the prior value when the key is absent from the update map).
