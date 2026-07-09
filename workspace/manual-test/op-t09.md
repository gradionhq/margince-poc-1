# OP-T09 Live-UAT Guide — Offer builder screen

Exercises the full OP-T09 surface: the new `frontend/src/features/offers/` module (routes,
components, api hooks, `offerMath`/`offerCopy`/`money` libs) mounted at
`/deals/:id/offers/:offerId`, plus the "Offers" entry-point card added to
`DealDetailPage.tsx`. Traces to AC-offer-1..8, OFFER-AC-N-1, STATE-1..5, and GATE-AI-1/2/9
(`workspace/plans/2026-07-09-op-t09.md`). **Frontend-only ticket** — every backend endpoint this
screen calls (`createDealOffer`/`listDealOffers`/`getOffer`/line-item CRUD/`regenerateOffer`/
`renderOffer`/`sendOffer`) already shipped in OP-T01/T02/T05/T06/T07/T08; no `crm.yaml`/Go change
is in scope. Per the scope-aware gate matrix this ticket runs in **fe-uat mode** — every numbered
step below is a browser/component action (click, type, read rendered copy), never a `curl`/API
call. The full end-to-end flow this guide drives (create → edit → regenerate → accept/dismiss →
send, across four roles) needs live data and real role-switching that Storybook's isolated
component capture can't reproduce, so this guide runs as a live dev-server session — the matrix's
documented fallback "when a component needs live data" — with `make fe-uat`'s change-scoped
Storybook render+capture kept as a fast pre-check (Step 0), not the guide's only lane.

**Two real defects were found while spot-checking the shipped `swarm/op-t09` source against this
guide's steps (not invented, not a plan re-interpretation).** Both are now **fixed** on
`swarm/op-t09` (commits `aba93f3`/`a3ccc63`) and re-verified against the current code before this
guide was corrected:

1. `RegenerateBanner.tsx` used to gate on `offer.status === "draft"` — the opposite of what the
   backend's `regenerateOffer` requires (`requireSent`, `store_offer_actions.go`) — so Regenerate
   was unreachable in every real offer state. It now gates on `offer.status === "sent"`
   (`RegenerateBanner.tsx:37-39`), decoupled from the draft-only `canMutateOffer` edit gate.
2. `offer_line_item` has no `source`/`captured_by` DB columns (confirmed in the table DDL,
   `backend/migrations/000071_offers_and_products.up.sql:120-136` — `offer`/`product` both have
   real columns of that name; `offer_line_item` alone doesn't), so a GET-based staged-line
   predicate could never fire. Staged-line membership is now tracked as session-local state
   (`stagedLineIds: Set<string>` in `OfferBuilderPage.tsx`), seeded straight from the
   `regenerateOffer` mutation's own response (`RegenerateBanner`'s `onRegenerated(newOfferId,
   aiLineIds)` callback) — the one place `captured_by` is ever reliably `agent:...`, because it's
   the live INSERT's echoed-back input, not a re-read.

Steps 5-7 below are rewritten as genuinely `[live]`-executable steps again. Read the session-local
staged-lines note at the top of Step 6 before running it — it describes a real, disclosed
limitation of the backend's persistence model, not a residual bug.

## Step 0 [auto]: `make fe-uat` pre-check

```bash
make fe-uat
```

**Expected:** exits with `manifest.pass == true` in `.tmp/fe-uat/manifest.json` — every changed
Offers component under `frontend/src/features/offers/` has a story and renders clean (no
`pageerror`, no console error, non-empty `#storybook-root`). This is a fast smoke check only; it
does not substitute for the live steps below (Storybook renders components in isolation with mock
props, not the composed multi-role flow this guide drives).

## Bring up the app

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

In a second terminal:

```bash
make fe-dev
```

(Or the per-worktree equivalent: `make uat_env UAT_SLUG=op-t09` — read its printed handle for the
derived backend/frontend URLs and use those in place of `:8080`/`:5173` below.)

The commands below assume:

- API at `http://localhost:8080`, web client at `http://localhost:5173` (Vite's `/api` proxy).
- `psql` (against `$DATABASE_URL`) is on `PATH` — used **once**, in the Bootstrap section below, to
  attach the seeded `ops` role to a fixture user (no `ops`-role user is seeded and no UI/API path
  can create a `role_assignment` — this is setup, not a guide-step verification; every actual
  guide-step Expected below is read from the browser, never from `psql`).
- `make seed-reset` has (re-)applied `backend/seed/dev.sql`.

## Bootstrap — seeded users (+ the one seed gap: no `ops`-role user exists)

| Role | Email | Password |
|---|---|---|
| admin | `admin@example.com` | `changeme` |
| rep | `rep@example.com` | `changeme` |
| manager | `manager@example.com` | `changeme` |
| read_only | `readonly@example.com` | `changeme` |
| ops | *(none seeded — create below)* | `changeme` |

`backend/seed/dev.sql` seeds the `ops` role (`role.key = 'ops'`, `'{"report":{"read":
{"row_scope":"all"}}}'` — zero `offer` permission, not even read) but assigns it to no user. Attach
it to a fresh fixture user before Step 12 (same bcrypt hash the other seeded users share, so the
password stays `changeme`):

```bash
psql "$DATABASE_URL" -c "
INSERT INTO app_user (id, workspace_id, email, display_name, password_hash)
VALUES ('00000000-0000-0000-0010-000000000005', '00000000-0000-0000-0000-000000000001',
        'ops@example.com', 'Ops User',
        '\$2a\$10\$bclrO7qYuxFBHUyKVkYGu.dpaZXWcg/u3S5NnQsBX75VLBD.3j2tu')
ON CONFLICT DO NOTHING;
INSERT INTO role_assignment (id, workspace_id, role_id, user_id)
VALUES ('00000000-0000-0000-0030-000000000005', '00000000-0000-0000-0000-000000000001',
        '00000000-0000-0000-0020-000000000005', '00000000-0000-0000-0010-000000000005')
ON CONFLICT DO NOTHING;"
```

Deal fixture used throughout: `Initech Discovery Call`
(`00000000-0000-0000-0042-000000000003`, owned by rep, zero seeded offers).

---

## Step 1 [live]: Create the first draft offer (STATE-1, AC-offer-1)

1. Open `${FE_BASE}/login`, sign in as `rep@example.com` / `changeme`.
2. Navigate to `${FE_BASE}/deals/00000000-0000-0000-0042-000000000003` ("Initech Discovery Call").
3. Scroll to the "Offers" card (`data-testid="deal-offers-card"`).

**Expected:** the card shows "No offers yet." and — rep is an allowlisted role
(`canCreateOfferForRole`, `frontend/src/features/deals/routes/DealDetailPage.tsx`) — a "New offer"
button. Click it.

**Expected:** `useCreateOffer` fires `POST /deals/{id}/offers` (offer_number `ANG-<timestamp>`,
`currency: "EUR"`, an `Idempotency-Key` header); on success the page navigates to
`/deals/{id}/offers/{newOfferId}`.

## Step 2 [live]: Fresh draft, zero line items (STATE-1, OFFER-AC-N-1)

On the new builder page.

**Expected:** header reads `<offer_number> v1` with a status pill reading `draft`; a "Back to
deal" link returns to `/deals/{id}`; the versions bar (`data-testid="offer-versions-bar"`) shows
exactly one pill — `v1`, accent-bordered (current draft), no lock icon; the line-items area shows
"No line items yet" / "Add the first line to start building this offer." — no fabricated rows.

## Step 3 [live]: Add a line item — instant client recompute, server reconcile (AC-offer-3,
OFFER-PARAM-4)

1. Click "Add line" — a "New line" row appears (`useCreateLineItem` fires immediately with
   `quantity: 1, unit_price_minor: 0, discount_pct: 0, tax_rate: 0`).
2. Edit the row: quantity `2`, unit price `15000` (minor units — €150.00), discount `10`, tax `19`.

**Expected:** the row's own "Net" column recomputes on every keystroke (client-side
`computeLineNet`, no network round-trip) — once all four fields read `2`/`15000`/`10`/`19` the Net
column reads `27000` (2 × 15000 × (1 − 10%)). Tab or click out of the last field (`onBlur`) —
`useUpdateLineItem` fires `PATCH /offers/{id}/line-items/{lineId}` with an `Idempotency-Key`
header. After it settles, the editor's own footer ("Net minor units: …") and, after clicking
"Explain this total", the Explain-total panel's per-line formula text
(`2 × 15000 × (1 - 10%) = 27000; 27000 × 19% = 5130`) both reconcile to `27000` net / `5130` tax —
matching OFFER-PARAM-4's server formula exactly.

## Step 4 [auto]: `offerMath` byte-exact parity

```bash
cd frontend && npx vitest run src/features/offers/lib/offerMath
```

**Expected:** exits 0 — `computeLineNet`/`computeLineTax`/`computeOfferTotals` assert byte-exact
parity against `backend/internal/modules/offers/adapters/store_offer.go`'s `computeLineTotals`
fixture values (fractional discount%, non-zero tax, a zero-price line contributing exactly 0).

## Step 5 [live]: Regenerate → navigation + locked revision + visible diff (AC-offer-2)

Regenerate only appears on a **sent** offer, for a mutate-capable role (admin/rep/manager) —
`RegenerateBanner.tsx:39-41`. If you're running the steps in numeric order and haven't reached Step
10 (Send) yet, either send the draft from Steps 1-3 first (jump to Step 10, then come back), or
continue and revisit this step once you have a sent offer.

1. On a **sent** offer, confirm the "Regenerate" section and its button render at all — before the
   fix they were hidden on every offer status, so this alone proves the gate fix.
2. Click "Regenerate".

**Expected:** `useRegenerateOffer` fires `POST /offers/{id}/regenerate`; on success the page
navigates to `/deals/{id}/offers/{newOfferId}` — header revision increments, status pill reads
`draft`; back in the versions bar (`data-testid="offer-versions-bar"`), the OLD revision's pill now
shows the lock icon (`data-testid="locked-revision-icon"`) — superseded, no edit affordance.

**The diff summary, AI-disclosure banner, and full diff table are now genuinely visible live** (a
real defect here — content computed right before `navigate()` fired but immediately hidden by the
component's own gate — was found and fixed; see `RegenerateBanner.tsx` history). The fix decouples
the trigger button's visibility (`canRegenerate`, gated on `offer.status === "sent"`) from the diff
content's visibility (`showResult = result != null && result.id === offer.id`,
`RegenerateBanner.tsx:39-42`): on the newly-landed-on draft, `offer.id` matches the mutation
response's `id`, so `showResult` stays true even though `canRegenerate` is now false (the new offer
is a `draft`) — the section keeps rendering its diff content, it just drops the trigger button. The
click handler also seeds the query cache (`qc.setQueryData(offersKeys.detail(next.id), next)`,
`RegenerateBanner.tsx:73-75`) right after the mutation resolves and before `navigate()` fires, so
`OfferBuilderPage`'s `useOffer(newOfferId)` finds cached data immediately on the new route — no
loading-skeleton flash before the diff content paints.

**Expected, on the new draft's page right after the navigation from step 2 above:** the diff-summary
caption reads `vN → vN+1 — X added, Y removed, Z changed` (the actual counts from the response's
`diff_from_previous`); the AI-disclosure banner renders (or, on a plain regenerate with
`diff_from_previous: null`, is omitted along with the diff table); a "View full diff" button is
present — click it to expand a table of added/removed/changed rows (before → after for changed
lines), each with a `LineProvenanceBadge`.

**`[auto]` — same rendering logic, proven against a fixture (handy for exercising the "plain
regenerate, no diff" branch, which a plain live click may or may not land on depending on what the
mutation actually returns):**

```bash
cd frontend && npx vitest run src/features/offers/components/RegenerateBanner
```

**Expected:** exits 0 — covers the "v1 → v2 — 1 added, 1 removed, 1 changed" summary, the
AI-disclosure banner text, "View full diff" expanding a table with added/removed/changed rows
(before→after for changed), and the "plain regenerate" branch (`diff_from_previous: null`)
correctly omitting both the disclosure banner and the diff table.

## Step 6 [live→auto]: Staged AI lines (Accept/Edit/Dismiss) + unpriced placeholder (AC-offer-4/5,
GATE-AI-1/2/9) — **read the session-local note before running**

The "Staged AI lines" panel (`data-testid="staged-lines-panel"`) is now real and interactive
against session-local state (`stagedLineIds` in `OfferBuilderPage.tsx`, seeded straight from the
regenerate response — see the guide intro), not the broken GET-based `captured_by` predicate this
guide originally flagged. But production's retriever wiring is hardcoded to
`transport.NewNoOpRetriever()` (`module.go:79`), which always yields an empty `retrieval.Context`
(`retrieval_noop.go:30-32`) — `decodeOfferLineSignals` (`ai_signals.go:10-19`) therefore always
returns `nil`, `domain.FilterGroundedSignals` always sees zero grounded signals, and
`OfferStore.Regenerate` always takes the verbatim-clone branch
(`store_offer_regenerate.go:232`, `len(grounded) > 0` is always false). **A plain live regenerate
call, in this build, can never produce AI-authored lines.** There's no config flag or UI lever to
swap in a different retriever — doing so is a code change, out of a manual tester's reach. So:

- **What you CAN do live, right now:** on a sent offer, click "Regenerate" (Step 5). The response
  comes back with `ai_generated: false`, `diff_from_previous: null`; `onRegenerated` computes
  `aiLineIds = []`, and the panel correctly renders nothing (its own empty-return,
  `StagedLinesPanel.tsx:25`, `staged.length === 0`). That's the honest, expected live state, not a
  residual gap — the plumbing from "regenerate response" → `stagedLineIds` → "panel presence" is
  real and exercised end-to-end here; it's just fed zero AI lines by this build's retriever wiring.
- **To see a populated panel, with real staged lines, Accept/Edit/Dismiss, and the unpriced
  placeholder, you need a fixture-backed regenerate response — not something reachable by plain
  clicking today.** Use the `[auto]` tests below; they drive the same production components (and,
  for the second one, the same production page) with a regenerate response that carries
  AI-authored lines, so they aren't a weaker substitute for a live click — they're the only current
  way to exercise this half of the flow.

**`[auto]` fallback 1 — isolated component, badge/citation/Accept-Edit-Dismiss/unpriced placeholder:**

```bash
cd frontend && npx vitest run src/features/offers/components/{StagedLineRow,StagedLinesPanel}
```

**Expected:** exits 0 — covers the "AI-proposed" badge + `evidence.snippet` citation
(`LineProvenanceBadge`), the unpriced-line "We won't guess a number for this line." copy with
Accept disabled until a positive price is typed, Accept flipping `captured_by` to `human:<uid>`
(message "Accepted — now part of your draft."), Edit-then-accept ("Save edits") sending edited
values + the same flip, Dismiss (message "Dismissed — removed from this draft."), and the panel
rendering nothing for zero staged lines.

**`[auto]` fallback 2 — the full composed page, click-driven, the closest thing to a live
walkthrough of the whole Regenerate → staged → Accept/Dismiss loop:**

```bash
cd frontend && npx vitest run src/features/offers/routes/OfferBuilderPage
```

**Expected:** exits 0 — includes `"tracks staged ids from regenerate and clears them on accept or
dismiss"`, which mounts the real `OfferBuilderPage` at `/deals/d1/offers/o2`, mocks only the API
hooks (every component is the real one), clicks the actual "Regenerate" button, and asserts: the
staged-lines panel appears with the AI-authored line, the "Excludes 1 staged AI-proposed line(s)
from this total." caption shows on the line-editor footer, the Explain-total panel's "1 staged
AI-proposed line(s) and 0 unpriced line(s) are excluded…" caption is correct, clicking "Accept"
removes the panel and folds the line into the committed table, and a second Regenerate + Dismiss
removes the newly staged line again — the exact click sequence a live tester would perform, run
against a fixture regenerate response instead of the real (always-empty) retriever.

## Step 7 [live+auto]: "Excludes N staged" captions — real math, verified both ways (a related
wiring fix)

There was a second, related bug (now fixed alongside Step 5/6's, `a3ccc63`): `OfferBuilderPage`
used to pass `LineItemEditor`/`ExplainTotalPanel` an already-filtered `committedLines` array
instead of the full `lineItems` + `stagedLineIds`, so their own internal staged-count math could
only ever compute `0` — the "Excludes N staged…" caption was silently wrong even in the
(then-unreachable) AI-staged-line case. Both components now receive the unfiltered lines list plus
`stagedLineIds` and do their own filtering (`LineItemEditor.tsx:29-30`,
`ExplainTotalPanel.tsx:52-57`).

**[live] on the draft from Steps 1-3, with zero staged lines** (this build's honest live
state per Step 6): the line-editor footer shows no "Excludes N staged AI-proposed line(s)…"
caption, and the Explain-total panel's caption reads "0 staged AI-proposed line(s) and 0 unpriced
line(s) are excluded…". `stagedCount === 0` on every live offer today, for the reason Step 6
explains (no lever to get AI-authored lines from a live regenerate in this build) — not because the
counting logic itself is broken.

**[auto], proving the counting logic is correct for a non-zero count (not observable live right
now):**

```bash
cd frontend && npx vitest run src/features/offers/components/{LineItemEditor,ExplainTotalPanel}
```

**Expected:** exits 0 — `LineItemEditor`'s test asserts "excludes 1 staged ai-proposed line(s)"
against a fixture with one staged + one committed line; `ExplainTotalPanel`'s test asserts "1
staged AI-proposed line(s) and 1 unpriced line(s) are excluded…" against a three-line fixture (one
human-priced, one staged, one human-unpriced). Step 6's page-level `[auto]` fallback additionally
proves the same captions read correctly (1 staged, not 0) after a real click-through, closing the
loop between "the math is right" and "the wiring feeds it the right inputs".

## Step 8 [live]: Explain-this-total, steady state (AC-offer-6)

Click "Explain this total" on the draft from Steps 1-3/7.

**Expected:** gross is captioned "Gross minor units are computed server-side." (the
`stagedCount === 0` branch — always true live, per Step 6/7); a per-line formula breakdown line
for the priced line item added in Step 3, matching that row's own values; the caption
"amounts in {currency} minor units (cents); ISO 4217" is present; "Persisted record total:" shows
the same `gross_minor` figure as the header/editor footer.

## Step 9 [live]: DE/EN preview + PDF render (AC-offer-7)

1. In the "Angebot"/"Offer" preview panel, note the default locale is DE (title reads "Angebot",
   meta labels read "Angebotsnummer"/"Deal"/"Gültig bis", totals read "Netto"/"MwSt."/"Brutto",
   legal text "Dieses Angebot ist freibleibend und gilt bis zum angegebenen Datum.").
2. Click "EN".

**Expected:** title switches to "Offer", meta labels to "Offer number"/"Deal"/"Valid until", line
headers to "Description"/"Qty"/"Unit"/"Unit price"/"Discount"/"Tax rate"/"Net", totals to
"Net"/"Tax"/"Gross", legal text to "This offer is non-binding and remains valid until the listed
date."; every money figure re-formats from `de-DE` (e.g. `150,00 €`) to `en-US`
(e.g. `€150.00`) style via `Intl.NumberFormat` — confirm at least the unit-price/net cell for the
Step 3 line item visibly differs between the two locales.

3. Click "PDF erzeugen"/"Generate PDF".

**Expected:** `useRenderOffer` fires `POST /offers/{id}/render` (no body, `Idempotency-Key`
header); on success a "PDF ansehen"/"View PDF" link appears
(`<a href={pdf_asset_ref} target="_blank" rel="noreferrer">`), unaffected by which locale is
currently toggled — clicking it opens `pdf_asset_ref` in a new tab.

## Step 10 [live]: Send card + 🟡-gate copy + Sent lock (AC-offer-8)

On the same draft (`data-testid="send-card"`).

**Expected copy, verbatim:** "Sending queues this offer to the approval inbox for an automated or
agent send; your own click here is the approval and sends immediately." Click "Send offer".

**Expected:** a confirm dialog opens — title "Send this offer?", body "Your click is the approval
for this human-operated builder.", confirm button "Confirm send". Click it.

**Expected:** `useSendOffer` fires `POST /offers/{id}/send`; on success a toast reads "Offer sent —
locked. Any further change starts the next revision (regenerate)."; the status pill flips to
`sent`; re-render confirms every edit control on the page disappears — no editable line-item
inputs (all render as plain text now), no "Add line", no Send card itself
(`offer.status !== "draft"`) — and Regenerate now appears for the first time on this offer
(`offer.status === "sent"`, `RegenerateBanner.tsx:39-41`): this is the one moment the backend
actually accepts a regenerate call, and it's also the one moment the trigger button renders. If you
skipped Step 5 earlier for lack of a sent offer, run it now against this one — see Step 5 for the
full expected flow (navigation, the old revision's lock icon, and the new draft's diff
summary/AI-disclosure banner/full-diff-table, all visible live).

## Step 11 [live]: `read_only` role — every mutate control omitted, not disabled (STATE-4, Global
Constraint 6)

1. Log back in as `rep@example.com` and get a second, still-`draft` offer on the same deal — either
   create a fresh one ("New offer" — the simplest path, no precondition), or, if you completed Step
   10, click "Regenerate" on that now-`sent` offer (Step 5) to derive a new draft revision from it.
   Either way, you need a `draft` offer for this step.
2. Sign out, sign in as `readonly@example.com` / `changeme` (or use a private window), open that
   same offer's URL directly.

**Expected:** the whole screen renders read-only — every field in the line-item table is plain
text (no inputs), no "Add line" button, no Regenerate button, no Send card
(`data-testid="send-card"` absent) — all **omitted from the render tree**, not
greyed-out/disabled. The page-bottom caption (`data-testid="offer-builder-shell"`) reads "This
draft is view-only for your role."

## Step 12 [live]: `ops` role — no `offer` permission at all, not even read (STATE-4, Global
Constraint 6)

Using the `ops@example.com` / `changeme` fixture created in Bootstrap, open the same offer URL
from Step 11 directly.

**Expected:** `getOffer` 401/403s (the `ops` role's permissions JSON is
`{"report":{"read":{"row_scope":"all"}}}` — zero `offer` grant) and the page shows the honest
permission card (`data-testid="offer-builder-permission-card"`) — heading "You don't have
permission to view this offer", body "Ask an admin or a rep to grant offer access." — content
genuinely absent, not a blank/crashed page, and no controls of any kind render.

## Step 13 [manual]: Visual pass

1. Versions-bar lock iconography (`data-testid="locked-revision-icon"`, the Forge `Lock` icon) on
   a superseded/sent revision's pill — legible against both the accent (current) and neutral
   (locked) pill backgrounds.
2. Staged-AI-line badge color (`LineProvenanceBadge`'s "AI-proposed" chip,
   `bg-gf-status-info/15 text-gf-status-info border-gf-status-info/30`) and the unpriced-line
   warning chip (`bg-gf-status-warning/15 text-gf-status-warning border-gf-status-warning/30`) —
   Forge `--gf-*` tokens only, no raw hex (verify via devtools computed styles) — check these in
   the Storybook capture from Step 0 (or the `[auto]` fixture tests from Step 6), since a plain
   live regenerate in this build never produces AI-authored lines to inspect these on (Step 6).
3. DE/EN toggle transition (Step 9) — no layout jump/overflow when label lengths change between
   locales.
4. Send-confirm dialog (`ConfirmDialog`, Step 10) styling — consistent with other Forge
   `ConfirmDialog` usages elsewhere in the app (e.g. deal reopen/close flows).
