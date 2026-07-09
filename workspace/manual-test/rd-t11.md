# RD-T11 Live UAT Guide — Formula-fields screen (formula-fields.html, S-E15.8c)

Exercises the full RD-T11 surface: `frontend/src/features/formula-fields/` mounted on
`CompanyDetailPage.tsx` (after `PartnerPanel`, same `<main>` flow), the additive
`Organization.computed_fields` array on `GET /organizations/{id}`, and the one real backend row
(`open_pipeline`, wired to RD-T08's `organization_open_pipeline_rollup` view) alongside four
honestly `computable: false` rows. Traces to AC-formula-fields-1..8 and STATE-1..4
(`docs/subsystems/records-depth.md` / `docs/quality/acceptance-standards.md`).

**A hard non-negotiable this ticket enforces (do not treat as a bug to work around silently):**
this screen never becomes a runtime formula builder or expression interpreter, and "Send to
development" never opens a real PR/ticket — every "routed"/"reviewed source change" affordance
below is a UI state-toggle only. If any step below appears to let you type or save a new formula,
or appears to actually create a ticket/PR, that is a real regression, not a guide error.

## Bring up the stack

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

In a second terminal:

```bash
make fe-dev
```

(Or the per-worktree equivalent: `make uat_env UAT_SLUG=rd-t11` — read its printed handle for the
derived backend/frontend URLs and use those in place of `:8080`/`:5173` below; skip the two
commands above in that case.)

The commands below assume:

- API at `http://localhost:8080`, web client at `http://localhost:5173` (Vite's `/api` proxy
  forwards to the backend on the same port pair — see `frontend/vite.config.ts`)
- `curl`, `jq`, and `psql` (against `$DATABASE_URL`) are on `PATH`
- `make seed-reset` has (re-)applied `backend/seed/dev.sql`

## Bootstrap — env vars from the standing dev seed

```bash
export API_BASE='http://localhost:8080'
export FE_BASE='http://localhost:5173'
export WS_ID='00000000-0000-0000-0000-000000000001'
export ADMIN_ID='00000000-0000-0000-0010-000000000001'    # admin@example.com / changeme
export REP_ID='00000000-0000-0000-0010-000000000002'      # rep@example.com / changeme
export READONLY_ID='00000000-0000-0000-0010-000000000003' # readonly@example.com / changeme
export MANAGER_ID='00000000-0000-0000-0010-000000000004'  # manager@example.com / changeme
export ORG_ID='00000000-0000-0000-0044-000000000001'      # "Acme Corp" — the only seeded organization
```

The seed's four active roles (`admin`/`rep`/`read_only`/`manager`) each grant
`computed_field:read` (`backend/seed/dev.sql`, mirrors the `quota` grant precedent) — any of the
four logins sees the panel. This guide standardizes on `admin@example.com` for viewing (row_scope
`all` on `organization`, avoiding any "own"-scope ambiguity for a record type with no owner
field), and uses `admin` throughout unless a step says otherwise.

Confirm the seeded organization and its open deals exist (fail fast if `seed-reset` didn't apply):

```bash
psql "$DATABASE_URL" -c "
SELECT id, name, organization_id, amount_minor, currency, status
FROM deal WHERE organization_id = '${ORG_ID}' ORDER BY name;"
```

Expected: four rows — `Acme Expansion` (500000, EUR, open), `LexCorp Renewal` (NULL amount, EUR,
lost), `Umbrella Proposal` (900000, EUR, open), `Wayne Enterprises Renewal` (400000, EUR, open).

---

## Step 1 [live]: Setup — real, non-zero `open_pipeline` (known limitation, not a bug)

**Known limitation, not a bug (mirrors `workspace/manual-test/rd-t10.md` Step 1's scan-status
callout):** `deal.amount_minor_base` is a `GENERATED` column computed as
`round(amount_minor * fx_rate_to_base)` (migration `000075_formula_field_boundary`), and no
seeded deal — nor any UI/API path exercised elsewhere in this guide's prerequisites — ever sets
`fx_rate_to_base` for an **open** deal (the `deal_closed_fx` CHECK only requires it once a deal
transitions off `open`). With `fx_rate_to_base` NULL, `amount_minor_base` is NULL for every
seeded open deal, and `organization_open_pipeline_rollup.open_pipeline_minor_base` is therefore
NULL for `${ORG_ID}` out of the box — which RD-T11's own handler honestly floors to
`value_minor: 0, computable: true` (Global Constraint 3), not a bug. To exercise the row's *real,
non-zero* derived value (as the ticket's own UAT checklist calls for), simulate a real FX freeze
directly, the same way rd-t10 simulated a virus-scan verdict:

```bash
psql "$DATABASE_URL" -c "
UPDATE deal SET fx_rate_to_base = 1.0
WHERE organization_id = '${ORG_ID}' AND status = 'open';"
```

Expected: `UPDATE 3` (Acme Expansion, Umbrella Proposal, Wayne Enterprises Renewal — `LexCorp
Renewal` is `lost`, excluded by the view's own `WHERE d.status = 'open'`).

Verify the view directly:

```bash
psql "$DATABASE_URL" -c "
SELECT open_pipeline_minor_base, open_deal_count
FROM organization_open_pipeline_rollup WHERE organization_id = '${ORG_ID}';"
```

Expected: `open_pipeline_minor_base = 1800000`, `open_deal_count = 3`
(500000 + 900000 + 400000, EUR at a 1:1 rate — the workspace's own `base_currency` is `EUR`).

Confirm the API now serves this on `GET /organizations/{id}`:

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${ADMIN_ID}" \
  "${API_BASE}/organizations/${ORG_ID}" | jq '.computed_fields'
```

Expected: a 5-element array. Element 0: `{"key":"open_pipeline","label":"Open pipeline",
"kind":"currency_minor","value_minor":1800000,"formula_sql":"SUM(deal.amount_minor_base) WHERE
organization_id = ... AND status = 'open' AND NOT archived","dependencies":["deal.amount_minor",
"deal.fx_rate_to_base","deal.status"],"computable":true}` (no `reason` key). Elements 1-4 (in
order): `weighted_pipeline`/"Weighted pipeline", `customer_age`/"Customer age",
`net_revenue_retention`/"Net revenue retention", `blended_gross_margin`/"Blended gross margin" —
each `"kind"` per spec (`currency_minor`, `duration_months`, `percent`, `percent` respectively),
each `"formula_sql":""`, `"dependencies":[]`, `"computable":false`, `"reason":"not_yet_built"`.

Now log in on screen: open `${FE_BASE}/login`, sign in as `admin@example.com` / `changeme`,
navigate to `${FE_BASE}/companies/${ORG_ID}` ("Acme Corp"). Scroll to the bottom of the page,
past the Partner section — the formula-fields panel mounts there, last in the page's `<main>`
flow.

---

## Step 2 [live]: AC-formula-fields-1/-2/-3 — five-row table, badges, lock, mono de-DE EUR value

**Expected**, on the panel:

- A header badge reading "read-only computed" and a note reading "Recomputes on every write."
  (this exact copy is the spec's own literal build instruction for the panel header — accept
  trivial capitalization/punctuation variance only, not a substantive wording change).
- Exactly five rows, in this order: **Open pipeline**, **Weighted pipeline**, **Customer age**,
  **Net revenue retention**, **Blended gross margin**. Each row shows its label plus a
  `Σ Derived` badge.
- **Open pipeline** row: a lock icon with title text `Read-only — computed, cannot be edited`
  (hover it to confirm the tooltip), a subtext reading "Read-only computed value", and a
  **mono-font** value reading exactly `18.000,00 €` (de-DE thousands/decimal separators, two
  decimals, trailing `€`).
- The other four rows: **no lock icon** (nothing to protect — confirm via devtools that no
  element with that title attribute exists in those rows), a subtext reading "Formula unavailable",
  and a value reading exactly `Not computable yet` in place of any number.

---

## Step 3 [live]: AC-formula-fields-4 — Explain toggle, per row

On the **Open pipeline** row, click "Explain this number".

**Expected:** a popover opens showing:
- The field's label, "Open pipeline"
- A "Formula" section with the mono SQL text
  `SUM(deal.amount_minor_base) WHERE organization_id = ... AND status = 'open' AND NOT archived`
- A "Dependencies" section listing three lines, each reading "Input from `<dependency>`" in mono
  for the dependency name: `deal.amount_minor`, `deal.fx_rate_to_base`, `deal.status`
- A "Result" section highlighting `18.000,00 €`

Click "Explain this number" again (or click outside the popover). Expected: the popover closes.

Now open the explain toggle on the **Weighted pipeline** row (or any of the other three
not-computable rows).

**Expected:** the popover shows the field's label, then the text
`Not computable yet — not_yet_built` in place of any formula/dependencies/result — no "Formula"
or "Dependencies" section renders for a not-computable row.

---

## Step 4 [live]: AC-formula-fields-5 — "See it recompute" driver, client-only, no network write

Locate the right-rail "See it recompute" driver (labelled "Right rail driver" above the heading).
It should already read, at rest (default "212k" scenario):

**Expected (baseline):** a win-probability chip reading `40%`; a three-option scenario control
with options `212k` / `177k` / `lost`, `212k` selected; "Simulated open pipeline" reading
`18.000,00 €` (matches Step 2's real value — no delta applied yet); "Delta" reading `0,00 €`; a
copy line reading "Try it - nothing is saved." (or equivalent hyphenated wording — this is the
component's own explicit non-persistence disclosure).

**Before switching scenarios, open browser devtools' Network tab and clear it (or start
recording).** This is the load-bearing check for this step.

Click the `177k` option.

**Expected:** the "Open pipeline" row and/or the driver's own display visibly flashes/highlights
momentarily (a transient visual change signalling a recompute — exact styling is an
implementation detail, but *some* visible transient change must occur, not a silent number swap);
"Simulated open pipeline" updates to `-17.000,00 €` (18.000,00 - 35.000,00, since the driver's
177k-scenario delta is a fixed -€35,000 regardless of the real baseline); "Delta" reads
`-35.000,00 €`; a toast reads `Simulation only -35.000,00 €. Nothing is saved.`

Click the `lost` option.

**Expected:** the win-probability chip now reads `lost`; "Simulated open pipeline" reads
`0,00 €` (the lost scenario's delta always exactly offsets the real baseline to zero); "Delta"
reads `-18.000,00 €`; a toast reads `Simulation only - the open pipeline drops to zero.`

Click back to `212k`.

**Expected:** "Simulated open pipeline" returns to `18.000,00 €`, "Delta" returns to `0,00 €`, win
probability returns to `40%`.

**Network check (must not skip):** inspect the Network tab across all three scenario clicks above.
Expected: **zero** new `PATCH`/`PUT`/`POST` requests of any kind fire as a result of selecting a
scenario — the only network activity for the whole panel is the original page-load
`GET /organizations/${ORG_ID}` (and any other page-level `GET`s already in flight before you
started this step). If you see any write-method request fire after a scenario click, that is a
regression — the driver is contractually client-side-only (Global Constraint 6).

---

## Step 5 [live]: AC-formula-fields-6 — "Send to development" card

Locate the "Send to development" card (AI-proposed formula-field proposal, e.g. "Account
health").

**Expected at rest:** an "AI-proposed" disclosure label (GATE-AI-9) is visible, next to
"Formula field proposal"; heading "Account health"; body text "A reviewed source change owns the
logic; this screen only stages the handoff."; a dashed draft box reading "Draft formula logic is
reviewed source, never runtime editable here."; three actions: "Send to development", "Edit
formula", "Dismiss".

Click "Edit formula".

**Expected:** a toast/status region reads exactly
`Draft edit - formula logic ships as reviewed source, not edited here` — and **no** formula
editor, textarea, or other editable input appears anywhere on screen (confirm via devtools: no
new `<textarea>`/`contenteditable` element exists). The card remains in its original ("proposed")
state.

Click "Send to development".

**Expected:** the card switches to a "routed" state: body text now reads "This logic ships as a
reviewed source change, not as runtime editor state." followed by "This needs the development
path, not this screen."; a link labelled "Development path" is present with `href="/development"`
(a static link — clicking it need not resolve to a real page, this proves it's a static route, not
network-authored); a toast reads `Formula logic is reviewed code, not runtime.`; the "AI-proposed"
label is still visible; the "Send to development" button is no longer present (only "Edit formula"
and "Dismiss" remain).

Click "Edit formula" again (now in the routed state).

**Expected:** the same toast (`Draft edit - formula logic ships as reviewed source, not edited
here`) fires again; still no editor renders; the card stays "routed".

Click "Dismiss".

**Expected:** the entire card is removed from the DOM — confirm the "AI-proposed" text and all
three action buttons are gone. Reload the page: the card reappears in its original "proposed"
state (dismissal is local component state, not persisted).

---

## Step 6 [live]: AC-formula-fields-7/-8 — field-definition rail, real SQL + dependencies + provenance

Locate the field-definition rail (wired to the **Open pipeline** row — the only row with a real
formula).

**Expected:**
- A small-caps label "Field definition", the field name "Open pipeline" as a heading, and — on
  the same header row — the provenance text `computed:server` (never "typed by you", literally,
  per AC-formula-fields-8).
- A "Formula SQL" section with a mono code block reading exactly
  `SUM(deal.amount_minor_base) WHERE organization_id = ... AND status = 'open' AND NOT archived`.
- A "Dependencies" section listing three chips/rows: `deal.amount_minor`, `deal.fx_rate_to_base`,
  `deal.status`.
- A scope note reading "Authoring new formula logic is a reviewed source change, not a runtime
  builder. This needs the development path, not this screen." — confirming (AC-formula-fields-7)
  that no runtime formula builder exists on this screen.

---

## Step 7 [auto]: STATE-1 — empty `computed_fields` array

This state is **not reachable through the seeded/live UI or API today** — say so rather than
inventing a manual repro. `OrganizationHandler.computedFields()`
(`backend/internal/modules/organizations/transport/handler_org.go`) unconditionally returns
exactly five hardcoded rows whenever the caller is visible (never zero, never any other count) —
proven by `TestOrganization_Get_ComputedFieldsVisibleAndFloored`
(`backend/internal/modules/records/organization_computed_fields_http_test.go`), which asserts
`len(got.ComputedFields) != 5` fails the test for *both* an org with an open deal and an org with
none. There is no code path, seeded or otherwise, that returns a present-but-empty array from this
backend. `FormulaFieldsPanel`'s own defensive empty-state branch (an honest "No computed fields
yet" card, per the implementation plan's Task 6 Step 1) exists only to protect a future backend
change and is proven solely by its own component-level fixture test
(`frontend/src/features/formula-fields/components/FormulaFieldsPanel.test.tsx`, Task 6) — not
independently re-walkable against this live stack.

```bash
go test ./backend/internal/modules/records/... -run TestOrganization_Get_ComputedFieldsVisibleAndFloored -tags=integration -v
```

Expected: exits `0`.

---

## Step 8 [live]: STATE-2 — page loading skeleton

Reload `${FE_BASE}/companies/${ORG_ID}` with devtools Network throttled (e.g. "Slow 3G", so
the loading state is visible long enough to inspect) — the formula-fields panel has no private
loading state of its own (Global Constraint 5); it inherits the page's own loading block.

**Expected:** briefly, before content renders, the page shows a skeleton container
(`data-testid="company-detail-skeleton"`) with a single placeholder block — no partial/half-loaded
formula-fields content flashes in ahead of the rest of the page.

---

## Step 9 [live]: STATE-3 — server error card with retry

Navigate to a well-formed but non-existent organization id:

```
${FE_BASE}/companies/00000000-0000-0000-0044-000000000099
```

**Expected:** the page renders an error state — text reading "Failed to load this company." and a
"Retry" link/button — **not** a blank page, not a partial page with an empty formula-fields panel.
This is the same generic page-level error card the rest of `CompanyDetailPage` already uses (the
formula-fields panel inherits it rather than rendering its own — Global Constraint 5); confirm no
formula-fields content (row labels, "read-only computed" badge, etc.) appears anywhere on this
error page. Click "Retry".

**Expected:** the page attempts to refetch and — since the id still doesn't exist — shows the same
"Failed to load this company." / "Retry" state again (not a crash, not a different message).

---

## Step 10 [auto]: STATE-4 — no-permission (array omitted, panel absent)

Every seeded, assigned role (`admin`, `rep`, `read_only`, `manager`) grants `computed_field:read`
(`backend/seed/dev.sql`, mirrors the `quota` grant precedent) — there is **no real seeded role**
that lacks the grant, so this state is not manually reproducible against the live dev stack
without hand-editing seed data. It is proven instead by a dedicated backend integration test that
seeds a fresh no-grant role (mirrors `hierarchy_rollup_http_test.go`'s `seedHTTPUserNoOrgPerm`
pattern):

```bash
go test ./backend/internal/modules/records/... -run TestOrganization_Get_ComputedFieldsHiddenWhenNotGranted -tags=integration -v
```

Expected: exits `0` — the test decodes the raw JSON body into `map[string]any` and asserts the
`"computed_fields"` key is **absent** entirely (not `[]`) for a role without the grant, matching
the spec's "array omitted, panel absent, never blurred/disabled" requirement (STATE-4). If you
want to eyeball the shape this produces, the equivalent live curl (using a role that *does* have
the grant vs. one that doesn't) is:

```bash
curl -sS -H "X-Workspace-ID: ${WS_ID}" -H "X-User-ID: ${ADMIN_ID}" \
  "${API_BASE}/organizations/${ORG_ID}" | jq 'has("computed_fields")'
```

Expected: `true` (admin has the grant). There is no seeded user for which this prints `false` —
that is exactly why this state stays `[auto]`-only.

---

## Step 11 [manual]: Visual pass — Forge tokens, mono value legibility, dark mode

1. With devtools open, inspect the computed `color`/`background-color` of the "read-only
   computed" header badge, the `Σ Derived` chip, and the lock icon. Expected: every computed color
   resolves to a Forge `--gf-*` token value, never a raw hex literal.
2. Confirm the Open pipeline row's mono value (`18.000,00 €`) and the field-definition rail's SQL
   block both render in a genuinely monospace font face, visually distinct from the row labels'
   proportional font.
3. Toggle your OS/browser to dark mode (or the app's own theme toggle, if one exists) and repeat
   step 1. Expected: colors adapt via the same tokens, remain legible, no raw-white/raw-black
   flashes; the "Not computable yet" rows remain legibly distinguishable from the real value row
   in both themes.
