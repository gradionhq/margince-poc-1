# Records Depth chapter — manual guide (RD-T01..T13)

Checkpoint for GitHub epic **#84 — RD (Records Depth)**. One ordered walkthrough for a human tester
verifying everything the chapter shipped. It has **five screens and four backend read/write
surfaces**, so each Part below pairs the `curl` check with its screen. Tickets:

- **RD-T07 / RD-T13** — field-history: a read-only per-field diff projection over `audit_log`
  (actor/field filters, evidence panel, honest empty) + the field-history screen.
- **RD-T08 / RD-T11** — formula fields: read-only **DB-GENERATED** columns (never a runtime authoring
  surface) + the formula-fields panel (read-only locks, explain boxes, simulate-only recompute,
  send-to-development).
- **RD-T06 / RD-T12** — quotas: CRUD (owner **XOR** team, human-set target) + attainment from
  closed-won + the quota screen (attainment ring/bands, pace line, contributing deals, target editor,
  team roll-up).
- **RD-T04 / RD-T09** — org hierarchy roll-up (three measures, bounded CTE, RBAC-honest exclusion,
  self/tree scope) + the account-hierarchy screen (roll-up tiles, explain box, scope toggle,
  restricted disclosure, staged-edge accept).
- **RD-T05 / RD-T10** — attachments: blob-seam references (never bytea), presigned upload/download,
  scan gate, download-audited + the attachments panel (dropzone, scan states, provenance, restricted/
  quarantined disclosure, staged AI-extraction, details drawer).
- **RD-T01/T02/T03** — the contract (quota / attainment / attachments / hierarchy / field-history
  resources) and the migration (`attachment` / `quota` / `bulk_operation` + the GENERATED column),
  exercised by everything below.

> **Three things to know before you start**
> 1. **No nav-rail links — you type URLs.** Like the custom-fields admin screen, none of the five RD
>    screens are in the left rail. Hierarchy / quota / field-history are direct URLs; the attachments
>    and formula-fields panels are **sections you scroll to** inside a Deal 360 / Company 360.
> 2. **The dev seed is deliberately thin here** — it ships **no quotas, no attachments, no org
>    parent-links, and no closed-won deals**, and open deals have no FX rate. That is *by design* so
>    you see the **honest empty / not-computable states first**, then build fixtures to light up the
>    populated states. Each Part tells you exactly what to create.
> 3. **Ignore the `501` stubs in `contracts/server/`.** `quotas_adapter.go` / `audit_adapter.go`
>    return `501` for interface-conformance only; they are **not** what the server routes. The running
>    `make run` server serves **real** handlers (`backend/cmd/api/routes.go`). Everything below returns
>    real data.

---

## Setup (do this once)

- [ ] **1. Boot the stack:**
  ```bash
  make infra-up && make migrate-up && make seed-reset && make run
  ```
  **Expected:** infra up, migrations applied (through `000075_formula_field_boundary` and the
  attachment/quota tables), dev seed loaded, API on `:8080`. *(For Part 5 attachments you also need
  `BLOBSTORE_*` env configured for `make run`, or uploads will fail — see that Part.)*
- [ ] **2. Start the frontend** in a second terminal:
  ```bash
  make fe-dev
  ```
  **Expected:** Vite on `http://localhost:5173`; `/api` proxies to `:8080`.
- [ ] **3. Log in as `admin@example.com`** / `changeme` (admin has all-scope, incl. `quota` CRUD and
  `computed_field:read`).

**Constants** (from `backend/seed/dev.sql`):

| Name | Value |
|---|---|
| Workspace ID | `00000000-0000-0000-0000-000000000001` |
| Admin user ID | `00000000-0000-0000-0010-000000000001` |
| Rep user ID (owns most deals) | `00000000-0000-0000-0010-000000000002` |
| Org "Acme Corp" (the only org) | `00000000-0000-0000-0044-000000000001` |
| Deal "Umbrella Proposal" (rich history) | `00000000-0000-0000-0042-000000000004` |
| Deal "Acme Expansion" | `00000000-0000-0000-0042-000000000001` |
| Person "Alice Müller" | `00000000-0000-0000-0001-000000000001` |
| API base | `http://localhost:8080` |

Export the dev headers once — abbreviated **`$HDRS`** below:
```bash
HDRS=(-H 'Content-Type: application/json' \
  -H 'X-Workspace-ID: 00000000-0000-0000-0000-000000000001' \
  -H 'X-User-ID: 00000000-0000-0000-0010-000000000001')
```

---

## Part 1 — Field history (RD-T07, RD-T13) — *works against the seed as-is*

The seed's `audit_log` already backs real diffs, so start here.

- [ ] **1.1 Read a rich per-field diff timeline** (Umbrella Proposal = create + 3 stage advances):
  ```bash
  curl -s 'http://localhost:8080/field-history?entity_type=deal&entity_id=00000000-0000-0000-0042-000000000004' "${HDRS[@]}" | jq '.data[] | {field, from:.old_value, to:.new_value, actor:.actor_type}'
  ```
  **Expected:** `200` with a **per-field** timeline — the `stage` field shows its from→to steps, each
  entry carrying who changed it and when. This is a **projection over the append-only audit log**, not
  a second store.
- [ ] **1.2 Filter by field + actor** (AC-field-history-3/4):
  ```bash
  curl -s 'http://localhost:8080/field-history?entity_type=deal&entity_id=00000000-0000-0000-0042-000000000001&field=stage&actor_type=human' "${HDRS[@]}" | jq '.data | length'
  ```
  **Expected:** only human `stage` changes. `entity_type` + `entity_id` are **required**; a valid but
  history-less id returns an **honest `200 {data:[]}`** (never a guessed history).
- [ ] **1.3 The field-history screen.** Open:
  ```
  http://localhost:5173/records/deal/00000000-0000-0000-0042-000000000004/field-history
  ```
  **Expected:** header **"Field change history"** with an *"N fields · M changes"* count and a
  **"read-only projection, not editable here"** caption; **actor / field / free-text** filter
  controls; one card per field with a diff timeline and an **evidence panel**. Clear/narrow a filter
  to an empty match → *"No changes match this filter."*
- [ ] **1.4 Honest empty history.** Visit the same screen for the org (no field changes):
  `http://localhost:5173/records/organization/00000000-0000-0000-0044-000000000001/field-history`.
  **Expected:** an honest empty state — never a fabricated row. (A backend outage shows a contained
  *"Couldn't load … won't show a partial or guessed history"* card.)

---

## Part 2 — Formula fields / GENERATED columns (RD-T08, RD-T11)

- [ ] **2.1 Read the org's computed fields — honest "not computable yet" by default:**
  ```bash
  curl -s http://localhost:8080/organizations/00000000-0000-0000-0044-000000000001 "${HDRS[@]}" | jq '.computed_fields'
  ```
  **Expected:** an `open_pipeline` computed field reporting the **not-computable** honest state
  (`computable: false`, `value_minor: null`) — because the seeded open deals have **no
  `fx_rate_to_base`**, so the GENERATED base-amount (`round(amount_minor * fx_rate_to_base)`) is NULL,
  and `SUM` over NULLs stays NULL. **This is the correct floor, not a bug** — the migration comment
  itself calls this "a real, not a contrived, missing-input case."

> **Why you can't set the FX rate through the API here.** `fx_rate_to_base` is **"Ignored while
> open"** on `PATCH /deals/{id}` (see the field's own contract description) — the `deal_closed_fx`
> CHECK only *requires* it once a deal closes (won/lost). So there is a real catch-22 over the public
> API: only **open** deals feed `open_pipeline`, but an open deal won't accept an FX rate, and closing
> it drops it out of the open-pipeline view. That's intended — the honest NULL above **is** the
> RD-T08 demonstration. If you PATCH `{"fx_rate_to_base":"1.0"}` on an open deal, the write succeeds
> (version bumps) but the field stays `null` — expected, not a failure.

- [ ] **2.2 (optional) See it compute a real number — set the FX rate directly in the DB.** Because
  the GENERATED column recomputes on *any* write to `fx_rate_to_base` (and the CHECK permits it on an
  open row at the SQL level), a direct `UPDATE` is the way to light up the populated state:
  ```bash
  PGPASSWORD=margince psql -h localhost -U margince -d margince -c \
    "UPDATE deal SET fx_rate_to_base = 1.0 WHERE id = '00000000-0000-0000-0042-000000000001';"
  curl -s http://localhost:8080/organizations/00000000-0000-0000-0044-000000000001 "${HDRS[@]}" \
    | jq '.computed_fields[] | select(.key=="open_pipeline") | {key,computable,value_minor}'
  ```
  **Expected:** `{"key":"open_pipeline","computable":true,"value_minor":500000}` — the DB
  **recomputed the GENERATED column** the instant `fx_rate_to_base` was set (no application code
  path), and the `organization_open_pipeline_rollup` view summed it. That is the whole point of
  RD-T08: the value is a server-generated column, never client-set. *(`make seed-reset` restores the
  NULL state.)*
- [ ] **2.3 The formula-fields panel.** Open the Company 360 and scroll to the bottom **"Formula
  fields"** section: `http://localhost:5173/companies/00000000-0000-0000-0044-000000000001`.
  **Expected:** a computed-field table with a **"read-only computed"** chip and a *"Recomputes on
  every write"* note; per-field **explain boxes**; a **RecomputeDriver** on the right with scenario
  radios that show a *simulated* pipeline + delta and flash the row — captioned **"Simulation only …
  Nothing is saved."**; and a **"Send to development"** card whose **"Edit formula" button is disabled**
  and routes to the development path (*"Draft formula logic is reviewed source, never runtime editable
  here"*).
- [ ] **2.4 The boundary is build-verified both ways** (RD-AC-6/7). This is cleanest as a test:
  ```bash
  cd backend && go test ./internal/modules/records/ -run 'TestRDAC7' && cd ..
  ```
  **Expected:** green — proves there is **no formula-eval dependency, no eval import anywhere in the
  backend, and no runtime formula-authoring contract operation**. (The GENERATED-column schema proof
  is `TestDealAmountMinorBaseIsGenerated`, run in Part 6.)

---

## Part 3 — Quotas & attainment (RD-T06, RD-T12)

The seed ships **no quotas**, so create one.

- [ ] **3.1 Create an owner quota:**
  ```bash
  QUOTA=$(curl -s -X POST http://localhost:8080/quotas "${HDRS[@]}" -d '{
    "owner_id":"00000000-0000-0000-0010-000000000002",
    "period_start":"2026-01-01","period_end":"2026-12-31",
    "target_minor":10000000,"currency":"EUR"}' | jq -r .id)
  echo "QUOTA=$QUOTA"
  ```
  **Expected:** `201`; the quota targets €100 000.00 for rep-2 over FY26.
- [ ] **3.2 The owner-XOR-team guard** (DB CHECK → 422). Send both owner *and* team:
  ```bash
  curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:8080/quotas "${HDRS[@]}" -d '{
    "owner_id":"00000000-0000-0000-0010-000000000002","team_id":"00000000-0000-0000-0010-000000000001",
    "period_start":"2026-01-01","period_end":"2026-12-31","target_minor":10000000,"currency":"EUR"}'
  ```
  **Expected:** **`422`** (`owner_xor_team_required`) — a quota belongs to exactly one owner or one
  team, never both/neither.
- [ ] **3.3 Read attainment** (computed server-side from **closed-won only**):
  ```bash
  curl -s "http://localhost:8080/quotas/$QUOTA/attainment" "${HDRS[@]}" | jq '{target_minor,closed_won_minor,attainment_pct,contributing_deals:(.contributing_deals|length)}'
  ```
  **Expected:** `closed_won_minor: 0` / `0%` — the seed has **no won deals**, so attainment is
  honestly zero (open/lost/forecast are excluded). *(Optional: create a `status:"won"` deal owned by
  rep-2 with a close date in 2026 to see a nonzero number and a contributing-deals row.)* A quota with
  `target_minor:0` → **`422 attainment_target_zero`** (never a divide-by-zero).
- [ ] **3.4 The quota screen.** Open `http://localhost:5173/quotas/<QUOTA>` (paste the id from 3.1).
  **Expected:** header **"Quota & Attainment"**; an **attainment ring** with bands (met ≥100% /
  accent 60–99% / behind <60%) and a **pace line** (*"Behind pace — X% attained vs Y% of period
  elapsed"*); an **explain box**; a **period bar**; a **contributing-deals table** (empty on the seed);
  a **target editor**; and a **team roll-up rail**. With the zero-attainment seed you'll see the honest
  *behind/0%* state; a `target_minor:0` quota shows *"No target set for this period."*

---

## Part 4 — Account hierarchy roll-up (RD-T04, RD-T09)

The seed has a **single org with no children**, so self and tree are equal until you build a tree.

- [ ] **4.1 Read the self-scope roll-up:**
  ```bash
  curl -s "http://localhost:8080/organizations/00000000-0000-0000-0044-000000000001/hierarchy-rollup?scope=self" "${HDRS[@]}" | jq '{weighted_pipeline, closed_won, activity_count_30d, aggregated_account_count, restricted_excluded}'
  ```
  **Expected:** `200` with the three measures (weighted open pipeline, closed-won for the period, a
  30-day activity count) for Acme alone, and an **empty `restricted_excluded`**. *(If the open deals
  still lack an FX rate, the weighted-pipeline measure may report `422 fx_rate_unavailable` — set an
  FX rate as in Part 2.2 first, or read `scope=self` which is FX-light.)*
- [ ] **4.2 Build a real tree — create a child org and link it under Acme:**
  ```bash
  CHILD=$(curl -s -X POST http://localhost:8080/organizations "${HDRS[@]}" -d '{"display_name":"Acme Sub GmbH","source":"ui","captured_by":"human:00000000-0000-0000-0010-000000000001"}' | jq -r .id)
  curl -s -X PATCH http://localhost:8080/organizations/$CHILD "${HDRS[@]}" -d "{\"parent_org_id\":\"00000000-0000-0000-0044-000000000001\"}" | jq '{id,parent_org_id}'
  curl -s "http://localhost:8080/organizations/00000000-0000-0000-0044-000000000001/hierarchy-rollup?scope=tree" "${HDRS[@]}" | jq '{weighted_pipeline, closed_won, aggregated_account_count}'
  ```
  **Expected:** the `tree` roll-up now aggregates Acme **+ the readable child** (`roll-up(node) =
  self + Σ roll-up(readable child)`), where `tree` ≥ `self`.
- [ ] **4.3 The account-hierarchy screen.** Open:
  ```
  http://localhost:5173/companies/00000000-0000-0000-0044-000000000001/hierarchy
  ```
  **Expected:** header **"Account Hierarchy — Acme Corp"** with a **scope toggle** ("Whole tree
  (roll-up)" / "This account only (self)"); **roll-up tiles** (Weighted Pipeline, Closed-Won, node
  count/depth); an **explain box** (self figure vs children-sum, *derived from the server, never
  recomputed client-side*); the **tree** with your child node; a separate **"Restricted"** disclosure
  section for any RBAC-excluded node; and **"Suggested connections"** staged-edge cards whose **Accept**
  fires a real `PATCH /organizations/{id}` setting `parent_org_id`. Before 4.2 (no children) the screen
  shows the honest *"No sub-accounts in this hierarchy yet."* empty state.

---

## Part 5 — Attachments (RD-T05, RD-T10)

> **Requires `BLOBSTORE_*` env** for `make run`, or uploads fail. The seed ships **no attachments**,
> so the panel starts empty; you upload one to see the scan/provenance states. The **AI-extraction**
> panel only appears when an `agent:`-captured attachment exists (needs backend/agent seeding — an
> honest gap for the extraction states; see below).

- [ ] **5.1 Create an attachment (metadata + presigned upload URL):**
  ```bash
  curl -s -X POST http://localhost:8080/attachments "${HDRS[@]}" -H 'Idempotency-Key: rd-att-001' -d '{
    "entity_type":"deal","entity_id":"00000000-0000-0000-0042-000000000004",
    "filename":"proposal.pdf","content_type":"application/pdf","byte_size":12345,
    "source":"ui","captured_by":"human:00000000-0000-0000-0010-000000000001"}' | jq '{id,scan_status,upload_url:(.upload_url!=null)}'
  ```
  **Expected:** `201`; the row starts **`scan_status: "scanning"`** and carries a **presigned
  `upload_url`** (a PUT target on the blob seam) — the DB holds **metadata + object key only, never the
  bytes**. Copy the `id` as `<ATT_ID>`.
- [ ] **5.2 A scanning row has no download URL:**
  ```bash
  curl -s http://localhost:8080/attachments/<ATT_ID> "${HDRS[@]}" | jq '{scan_status, download_url}'
  ```
  **Expected:** `{"scan_status":"scanning","download_url":null}` — bytes are not downloadable until a
  clean verdict.

> **The row stays `scanning` forever in dev — this is correct, not a hang.** A new attachment lands
> `scanning` and **never auto-transitions** (RD-PARAM-5); only an external virus scanner applies the
> `clean`/`blocked` verdict, via an **internal `MarkScanResult` call with no public HTTP endpoint**.
> The dev stack runs no scanner worker, so there is nothing to advance it — the honest `scanning`
> state is exactly what you should see. The `clean → download_url + audit` transition is proven by
> the integration tests (which call `MarkScanResult` directly); see the Automated counterpart.

- [ ] **5.2b (optional) See the clean → download transition — apply a verdict directly in the DB.**
  Since only `MarkScanResult` moves the row and it has no public endpoint, force the verdict in SQL
  (mirrors what the scanner would do):
  ```bash
  PGPASSWORD=margince psql -h localhost -U margince -d margince -c \
    "UPDATE attachment SET scan_status='clean' WHERE id='<ATT_ID>';"
  curl -s http://localhost:8080/attachments/<ATT_ID> "${HDRS[@]}" | jq '{scan_status, download_url:(.download_url!=null)}'
  ```
  **Expected:** `{"scan_status":"clean","download_url":true}` — a clean row makes `GET` **mint a
  presigned download URL and write a "file access" audit row** (RD-WIRE-1: every download is audited).
  Set `scan_status='blocked'` instead to confirm a quarantined row keeps `download_url: null`.
  *(The presigned URL points at the blob key; the bytes only exist if you actually PUT them to the
  Part-5.1 `upload_url` — the field populating is the point here, not the download itself.
  `make seed-reset` clears the row.)*
- [ ] **5.3 The attachments panel.** Open the Deal 360 and scroll to the **"Attachments"** section:
  `http://localhost:5173/deals/00000000-0000-0000-0042-000000000004`.
  **Expected:** a header with your role chip (*"sees deal-room files"*); a **dropzone** (upload shows a
  *"Virus scan in progress"* toast); **provenance rows** with a **scan-status chip** — which in dev
  stays **"Scanning…"** (the same reason as 5.2: no scanner worker), so the follow-up *"attached and
  written to the timeline"* toast won't fire until a verdict lands; a **details drawer** on row click
  (your screenshot — provenance *"uploaded by you"*, *"Scan result is read-only"*); and, on rows that
  qualify, a **restricted** row (*"Request access"*), a **quarantined/blocked** row (*"Blocked because
  the file was quarantined."*), and — only if an agent-sourced attachment exists — a staged
  **AI-extraction** panel with *"Accept N field(s)"*. To see a **clean** chip + working download on the
  screen, apply the 5.2b verdict first, then reload.

---

## Automated counterpart

Run these to cover the same ground at the API/DB/component level (they are the merge gate):

| Command | What it proves |
|---|---|
| `make check` | Format, lint, contract-drift (RD-WIRE-1..5 + types), DAG/invariants, Go + FE unit tests — including the **RD-AC-7 negative-scope** unit test (no formula-eval anywhere) |
| `make test-it DIR=backend/internal/modules/records` | RD-T04 hierarchy roll-up (formula/scopes/restricted-grant/FX-422/closed-won-boundary/PO-AC-28 bound), RD-T06 quota CRUD + attainment (golden-number/scoping/period/cross-currency/target-zero), RD-T08 **GENERATED-column schema proof** + computed-fields visibility floor |
| `make test-it DIR=backend/internal/modules/records/transport` | RD-T06 quota HTTP (create/get/list/update/archive/attainment/auth/422s) + RD-T05 attachment HTTP (201+upload-url, scanning/blocked→null download, clean→download-url+audit, not-visible→disclosed-locked) |
| `make test-it DIR=backend/internal/modules/records/adapters` | RD-T05 attachment store round-trip + **download-audit-on-timeline**, RBAC visibility, archive cascade; quota pace/band units |
| `make test-it DIR=backend/internal/modules/records/fieldhistory` | RD-T07 diff projection, attribution, **masking**, honest-empty, pagination, newest-first; erasure-tombstone diff unit tests |
| `make fe-test` | The five screens + their components (hierarchy tiles/tree/scope/edge, quota ring/pace/contributing/target/team, field-history controls/group/evidence, formula recompute/send-to-dev, attachments) |

If a step doesn't match: the owning spec is `docs/subsystems/records-depth.md` (RD-AC-*, RD-WIRE-1..5,
RD-DDL-1..3, RD-FORM-1/2, RD-PARAM-*), and `docs/quality/acceptance-standards.md` is the STATE floor.

> **Known gaps flagged during testing** (all seed/env, not code defects):
> 1. **No nav-rail links** to the five screens — URLs are typed; attachments/formula-fields are
>    scroll-to sections in Deal 360 / Company 360.
> 2. **Thin seed:** no quotas, no attachments, no org parent-links, no closed-won deals, and open
>    deals have no `fx_rate_to_base`. This guide builds each fixture inline; the screens otherwise show
>    their honest empty / not-computable states.
> 3. **AI-extraction states (attachments) need an `agent:`-captured attachment**, which the dev stack
>    doesn't produce — those states are only observable in the integration tests.
> 4. **Attachment uploads need `BLOBSTORE_*` env** for `make run`.
> 5. The `contracts/server/*_adapter.go` `501` stubs are interface-conformance only — the running
>    server serves real handlers from `backend/cmd/api/routes.go`.
