# Custom Fields chapter — manual guide (CF-T01..T06)

Checkpoint for GitHub epic **#103 — CF (Custom Fields)**. A click-by-click walkthrough for a human
tester. Follow it top to bottom; each step is written as **Do** (what to click/type) → **Expected**
(what you should see). Tick the checkbox as you go.

What you're verifying:

- **CF-T06** — the admin screen at `/admin/custom-fields` (object chips, builder with API-key + DDL
  preview, structural-word refusal, staged→committed row, rename/retire, audit trail).
- **CF-T03/T04** — the governed engine behind it (real `ALTER TABLE`, atomic column+catalog+audit,
  closed-set validation, picklist CHECK, soft retire).
- **CF-T05** — parity: a custom value reads/writes on a core object and joins sort/filter (API-level).
- **CF-T01/T02** — contract + catalog migration underneath (exercised by the gates).

> **Two things to know before you start**
> 1. **A human never needs an approval token.** Adding/retiring a field from the UI just works.
>    The 🟡 approval gate only applies to AI agents, not to you clicking Confirm.
> 2. **You must be on the fixed backend.** The `listCustomFields` read (`GET /custom-fields`) was
>    unimplemented (returned 501) until recently — if the field table shows *"Something went wrong
>    in this section,"* your backend is stale. Restart it: `make run` (see Setup step 1).

---

## Setup (once)

- [ ] **1. Boot the backend** (rebuilds with the list-endpoint fix):
  ```bash
  make infra-up && make migrate-up && make seed-reset && make run
  ```
  **Expected:** infra up, migrations applied (through `000072_custom_field_catalog` and beyond),
  dev seed loaded, API serving on `:8080`.

- [ ] **2. Start the frontend** in a second terminal:
  ```bash
  make fe-dev
  ```
  **Expected:** Vite prints a local URL (e.g. `http://localhost:5173`; yours may differ, e.g.
  `:5751`). `/api` proxies to the backend and injects the dev workspace header for you.

- [ ] **3. Log in as `admin@example.com`** / password `changeme`.
  **Expected:** you land on Home with the nav rail on the left.

- [ ] **4. Open the admin screen directly** — there is **no nav-rail link yet**. Put this in the
  address bar (use your Vite port):
  ```
  http://localhost:5173/admin/custom-fields
  ```
  **Expected:** a **"Custom Fields"** page. Header on the left ("Custom Fields" + subtitle *"Manage
  fields for deal, organization, contact, lead, and activity objects."*), a green **"+ Add field"**
  button on the right, a row of object chips, and — on a fresh seed — an empty-state card. **You
  should NOT see a red "Something went wrong" card.** If you do, your backend is stale (see the note
  above).

- [ ] **5. Confirm the clean starting point.** Seed ships **zero** custom fields. Every object chip
  should show the empty state on first load. You'll create the fixtures yourself in Part 2.

**Reference values** (only needed for the optional API steps in Parts 3–4):

| | |
|---|---|
| Workspace ID | `00000000-0000-0000-0000-000000000001` |
| Admin user ID | `00000000-0000-0000-0010-000000000001` |
| API base | `http://localhost:8080` |

---

## Part 1 — The screen shell (CF-T06 AC-1, AC-2)

- [ ] **1.1 Read the object chips.** Just under the header: **Deal · Company · Contact · Lead ·
  Activity**. **Deal** is selected by default (highlighted, with a small count badge showing `0`).
  **Expected:** all five chips present.

- [ ] **1.2 Click each chip** in turn (Company, Contact, Lead, Activity, back to Deal).
  **Expected:** the table area re-scopes to that object; on a fresh seed each shows the empty-state
  card titled **"No custom fields on this object yet"** — never an empty table with just headers.

- [ ] **1.3 Confirm the core-fields note.** Above the table area there is the line **"Core fields
  are not shown — they aren't editable here."**
  **Expected:** present on every object. (Core fields like `name`/`amount` are deliberately absent —
  this screen only manages *custom* fields.)

---

## Part 2 — Build three Deal fields through the real engine (CF-T03, CF-T06 AC-3..8)

You'll create the chapter's three fixtures on the **Deal** object. Make sure the **Deal** chip is
selected first.

### 2A — "Renewal date" (a date field)

- [ ] **2A.1 Click "+ Add field"** (top-right).
  **Expected:** a modal opens, title **"New custom field"**, subtitle *"Add a scalar attribute to
  deals"*. Fields visible: **Field label**, **API key** (greyed/disabled), **DDL preview** (a
  monospace box), **Field type** (dropdown). Footer buttons: **Cancel** and **Confirm & create**.

- [ ] **2A.2 Confirm the empty-label guard.** With the label still blank, look at **Confirm &
  create**.
  **Expected:** it is **disabled** (greyed, not clickable). You cannot create a field with no label.

- [ ] **2A.3 Type the label** `Renewal date` into **Field label**.
  **Expected, live as you type:**
  - **API key** auto-fills to `deal.cf_renewal_date` and stays disabled (you can't edit it).
  - **DDL preview** shows exactly:
    `ALTER deal ADD COLUMN cf_renewal_date (text) · backfilled NULL · reversible`
    (it still says `text` because the type dropdown is still on its default — you'll change it next).

- [ ] **2A.4 Set Field type** to **date** (from the dropdown; options are `text, number, date,
  currency, picklist, boolean`).
  **Expected:** the DDL preview updates its type to `(date)`.

- [ ] **2A.5 Click "Confirm & create".**
  **Expected, in sequence:**
  - A **staged row** appears at the top of the table with the label showing **"writing…"** (API Key
    `—`, a type chip, no row actions).
  - On commit (near-instant locally) the staged row is **replaced by the real row**: **Label** =
    `Renewal date`, **API Key** = `deal.cf_renewal_date` (monospace), **Type** = a `date` chip,
    **Added by** = `Admin User`.
  - The selected **Deal chip's count badge** ticks up to `1`.
  - A green **toast**: *"Renewal date is live on the 360, filters, export & API."*
  - A new line appears in the **Audit trail** card at the bottom: *"Admin User added Renewal date
    (date) to deal"* with today's date.

### 2B — "Budget ceiling" (a currency field)

- [ ] **2B.1 "+ Add field"**, label `Budget ceiling`, then set **Field type** to **currency**.
  **Expected:** a new required input **ISO-4217 code** appears (placeholder *"e.g., USD"*) with the
  helper text *"Stored as integer minor-units (e.g. cents)."* Note that while it's empty, **Confirm
  & create** is disabled.

- [ ] **2B.2 Type** `EUR` into the ISO-4217 code, then **Confirm & create**.
  **Expected:** same staged→committed sequence; the new row's **Type** chip reads `currency`; the
  Deal count badge → `2`; audit line *"…added Budget ceiling (currency) to deal"*.

### 2C — "Procurement route" (a picklist field)

- [ ] **2C.1 "+ Add field"**, label `Procurement route`, then set **Field type** to **picklist**.
  **Expected:** a **Picklist options** editor appears with one empty option row (an input +
  **Remove** button) and an **Add option** button below.

- [ ] **2C.2 Test the last-option guard.** With only the single (empty) option row present, click its
  **Remove**.
  **Expected:** an error **toast**: *"A picklist needs at least one option"* — the row is **not**
  removed.

- [ ] **2C.3 Fill options.** Type `Direct` in the first option, click **Add option**, type `Tender`;
  **Add option** again, type `Framework`. Then **Confirm & create**.
  **Expected:** staged→committed; new row **Type** chip `picklist`; Deal count badge → `3`; audit
  line *"…added Procurement route (picklist) to deal"*.

### 2D — The structural-word refusal (CF-T06 AC-5 — no override exists)

- [ ] **2D.1 "+ Add field"** and type a label containing a structural word, e.g.
  `Link to primary contact` (triggers on `link to`; `object`, `relationship`, `lookup to` also
  trigger).
  **Expected, live:** a **red refusal banner** appears at the top of the modal:
  > *"This looks like a new object, relationship, or logic — not a scalar attribute on an existing
  > object. Runtime custom fields only add bounded scalar columns; a structural change ships as a
  > reviewed source change instead."*
  > *"This needs the development path, not this screen."*
  …and **Confirm & create is disabled**. There is **no "do it anyway"** option.

- [ ] **2D.2 Fix the label** — change it to something scalar, e.g. `Primary contact note`.
  **Expected:** the red banner disappears and Confirm & create re-enables. **Cancel** out (don't
  create this one).

---

## Part 3 — Rename & retire lifecycle (CF-T04, CF-T06 AC-1, AC-7)

Row actions live behind a **⋮ (three-dots) menu** at the right end of each active field's row.

### 3A — Rename (label changes, API key does NOT)

- [ ] **3A.1** On the `Renewal date` row, note its **API Key** column reads `deal.cf_renewal_date`.
- [ ] **3A.2** Click the row's **⋮** menu → **Edit**.
  **Expected:** a **"Rename field"** modal, subtitle *"Update the label for Renewal date"*, with the
  label pre-filled. The **Save** button is disabled until you change the text.
- [ ] **3A.3** Change the label to `Contract renewal date` and click **Save**.
  **Expected:** a **"Field renamed"** toast; the row's **Label** now reads `Contract renewal date`
  **but the API Key column is still `deal.cf_renewal_date`** — the physical column is immutable, only
  the display label changed. (Note: rename does **not** add a separate audit-trail line here; the
  card re-derives from current fields, so you'll simply see the field under its new label.)

### 3B — Retire (soft, reversible — hidden, never dropped)

- [ ] **3B.1** On the `Budget ceiling` row, click **⋮** → **Archive**.
  **Expected:** a confirm dialog **"Retire this field?"** with the description *"Budget ceiling will
  be hidden from new Deal records. Every existing value stays in place and the field remains in the
  audit trail."* and a **Confirm** button.
- [ ] **3B.2** Click **Confirm**.
  **Expected:** a **"Field retired"** toast; the `Budget ceiling` row is now **dimmed**, its **Type**
  column shows a grey **"Retired"** badge (instead of the type chip), and its **⋮ actions are gone**.
  The row **stays listed** (retired fields aren't deleted — this admin view keeps them). The Audit
  trail gains a line *"Admin User retired Budget ceiling"*.

---

## Part 4 — Parity & wire-level checks (CF-T05, CF-T01) — *optional, API-level*

These have no UI of their own, so they use `curl`. Skip if you only need the screen walkthrough.
Each write sends the dev headers the Vite proxy normally injects:

```bash
H=(-H 'Content-Type: application/json'
   -H 'X-Workspace-ID: 00000000-0000-0000-0000-000000000001'
   -H 'X-User-ID: 00000000-0000-0000-0010-000000000001')
```

- [ ] **4.1 A custom value round-trips on a core object.** Grab a deal id
  (`curl "${H[@]}" http://localhost:8080/deals | jq -r '.data[0].id'`), write the date, read it back:
  ```bash
  curl -s -X PATCH http://localhost:8080/deals/<DEAL_ID> "${H[@]}" -d '{"cf_renewal_date":"2026-12-01"}'
  curl -s http://localhost:8080/deals/<DEAL_ID> "${H[@]}" | jq '.cf_renewal_date'
  ```
  **Expected:** `200`, and the read-back carries `cf_renewal_date: "2026-12-01"` as a top-level
  property on the deal — a custom value stored on the real object, not a side table.

- [ ] **4.2 The active custom column joins the sort/filter vocabulary.**
  ```bash
  curl -s -o /dev/null -w '%{http_code}\n' 'http://localhost:8080/deals?sort=-cf_renewal_date' "${H[@]}"
  ```
  **Expected:** `200`. A retired column (e.g. `cf_budget_ceiling` after Part 3B) or an unknown one
  → **`422`** (`filter/sort_field_not_allowed`) — refused, never silently ignored.

- [ ] **4.3 The structural refusal at the wire.**
  ```bash
  curl -s -X POST http://localhost:8080/custom-fields "${H[@]}" \
    -d '{"object":"deal","label":"relationship to account","type":"text","source":"ui","captured_by":"human:admin"}'
  ```
  **Expected:** `422` with `"code":"structural_change_refused"` and
  `"details":{"route":"source_development_path"}`.

- [ ] **4.4 The list read itself** (the endpoint that was broken):
  ```bash
  curl -s 'http://localhost:8080/custom-fields?object=deal' "${H[@]}" | jq '.data[].column_name'
  curl -s -o /dev/null -w '%{http_code}\n' 'http://localhost:8080/custom-fields' "${H[@]}"
  ```
  **Expected:** the first returns your Deal columns (`cf_renewal_date`, etc.); the second (no
  `object`) → **`400`** (`object` is required).

---

## Part 5 — STATE-1..5 screen-state pass (CF-T06 STATE-1..5)

- [ ] **5.1 STATE-1 (empty)** — click the **Lead** chip (you added no Lead fields).
  **Expected:** the **"No custom fields on this object yet"** card — honest empty, not a blank table.

- [ ] **5.2 STATE-2 (loading)** — hard-reload the page (Cmd-R) and watch the table area on first
  paint.
  **Expected:** a brief **skeleton** placeholder before the rows render.

- [ ] **5.3 STATE-3 (error)** — stop the backend (Ctrl-C the `make run` terminal), then reload the
  screen.
  **Expected:** a red **"Something went wrong in this section."** card with a **Try again** button
  (and the Audit trail card shows *"Something went wrong"*) — a contained error, not a blank page.
  Restart `make run` and click **Try again** to recover. *(This is also exactly what the original
  501 bug looked like — now it only appears on a real outage.)*

- [ ] **5.4 STATE-4 (no-permission)** — log out, log in as **`readonly@example.com`** / `changeme`,
  reopen `/admin/custom-fields`.
  **Expected:** **no "+ Add field" button**, and rows have **no ⋮ action menu** — write affordances
  are absent, not merely disabled. ("Added by" names may render masked.) Log back in as admin
  afterwards.

- [ ] **5.5 STATE-5 (nothing-grounded)** — confirm no screen area shows fabricated placeholder
  content for something absent (empty objects show the honest empty card, not fake rows).

---

## Automated counterpart

Run these to cover the same ground at the API/DB level (they are the merge gate):

| Command | What it proves |
|---|---|
| `make check` | Format, lint, contract-drift (CF-T01 + `x-extension`), Go + FE unit tests |
| `make test-contracts` | Contract-compliance for CF-T01 admin ops and CF-T05 `x-extension` parity |
| `make test-integration` | CF-T03 governed ALTER+catalog+audit, injection refusal, CF-T04 rename-stability / retire-preserves-column / picklist CHECK, CF-T05 round-trip + vocabulary, **and `listCustomFields` object-scoping + status filter** (`list_http_integration_test.go`) |

Run just the custom-fields backend tests:
```bash
go test -tags=integration ./backend/internal/platform/customfields/
```

If a step doesn't match: the owning spec is `docs/subsystems/custom-fields.md` (CUSTOM-FIELDS-AC-*,
WIRE-*, PARAM-*), and `docs/quality/acceptance-standards.md` is the STATE-1..5 floor.

> **Known gap flagged during testing:** there is no `frontend/e2e/custom-fields.spec.ts`. A live
> Playwright spec that loads `/admin/custom-fields` against the real backend would have caught the
> original 501 on page load; the existing component tests mock the data layer and could not. Worth
> adding as a regression guard.
