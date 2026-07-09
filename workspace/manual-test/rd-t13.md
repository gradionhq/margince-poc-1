# RD-T13 Live-UAT Guide — Field-history screen (field-history.html, S-E15.8e)

Exercises the full RD-T13 surface: `frontend/src/features/records/routes/FieldHistoryPage.tsx`
(mounted at `/records/:entityType/:entityId/field-history`) plus its composed pieces —
`FieldHistoryControls`, `FieldHistoryGroupCard`, `FieldHistoryExplainBox`, the
`useFieldHistoryView` filter/derivation hook, and the `api/fieldHistory.ts` data layer — all inside
the existing `frontend/src/features/records/` module. Traces to AC-field-history-1..8 and
STATE-1..4 (`workspace/specs/rd-t13.md`, `workspace/plans/2026-07-10-rd-t13.md`).

**This ticket is frontend-only** — `GET /field-history` (RD-T07) and the per-entity-type `GET
/{collection}/{id}` reads already shipped and are fully tested; no `crm.yaml`/Go change is in
scope. Per the scope-aware gate matrix (`.ai/agents/swarm-coordinator.md`, § Scope-aware gates,
"fe-only → component-capture lane (B8)") this ticket runs in **fe-uat mode**: `make fe-uat`, a
change-scoped Storybook render+capture against `FieldHistoryPage.stories.tsx`'s `AllFields`,
`HonestEmptyField`, and `Loading` stories — **no live stack, no `make uat_env`, no `psql`/`curl`
anywhere in this guide.** Every `[live]` step below is a browser/Storybook action (render a story,
click a control inside it) against the seeded fixtures baked into that story's `makeClient()`;
every `[auto]` step names the exact test file (and, where useful, the exact test name) in this
branch that proves the corresponding acceptance criterion. There is no `[manual]` step — nothing
here needs human/visual-only judgment beyond what the capture already proves.

## Fixture reference (from `FieldHistoryPage.stories.tsx` — read once, reuse below)

```
entity: deal-baer-pharma (BÄR Pharma — Packaging QA)     currency: EUR

DEAL record's scalar field catalog (5 fields):
  name            "BÄR Pharma — Packaging QA"
  amount_minor    17707200            -> 177.072,00 €
  currency        "EUR"
  stage_id        "s-proposal"
  owner_id        "u-anna"

ENTRIES (4 rows, AllFields story):
  e1  amount_minor  21200000 -> 17707200   2026-06-18T09:42:00Z  agent  psp_7Q3f
      evidence: { quote: "offer.accepted · offer_id=of_8842 · gross_minor=17707200 · currency=EUR",
                  source_url: "https://example.com/offer/8842",
                  confidence: "high", confidence_note: "computed, not inferred" }
  e2  amount_minor  null -> 21200000       2026-05-29T16:08:00Z  human  u-anna
  e3  stage_id      "Qualified" -> "Proposal sent"  2026-06-12T11:20:00Z  human  u-anna
  e4  stage_id      null -> "Discovery"     2026-05-29T16:09:00Z  human  u-anna

HonestEmptyField story: entries filtered to amount_minor only (stage_id entries removed) — the
Stage group still renders with 0 recorded changes (AC-field-history-8).

Loading story: a fresh QueryClient with nothing seeded — the field-history/entity-record queries
never resolve.
```

## Bring up the fe-uat lane

From the repo root (`external/target/` in this factory checkout, or the branch worktree root when
run by the `swarm-uat-runner`):

```bash
make fe-uat
```

This builds a fresh static Storybook (forced, never cached — so it renders the current diff, not a
stale build), then drives every in-scope story (every story co-located with a component this
branch touched, which includes `FieldHistoryPage.stories.tsx`'s `AllFields`, `HonestEmptyField`,
and `Loading` stories) headlessly via Playwright, screenshots each, and writes
`.tmp/fe-uat/manifest.json`.

For the interactive `[live]` steps below (clicking a field chip, clicking "evidence", clicking
"Explain this number"), also start the interactive Storybook dev server in a second terminal:

```bash
make storybook
```

Then open `http://localhost:6006` and use the sidebar to navigate to **CRM → Records →
FieldHistoryPage → AllFields** (or go straight to
`http://localhost:6006/?path=/story/crm-records-fieldhistorypage--all-fields` if that resolves — if
the exact slug differs, the sidebar path above always works). This is the same `AllFields` story
`make fe-uat` captures — the interactive server just lets you click inside it instead of only
screenshotting it at rest. Use the sidebar's `HonestEmptyField` and `Loading` entries for Steps 8
and 9.

---

## Step 0 [auto]: `make fe-uat` — the stories render clean

```bash
make fe-uat
```

**Expected:** exits `0`; `.tmp/fe-uat/manifest.json` has `"pass": true`; its `stories` array
contains three entries whose `id`s correspond to `FieldHistoryPage.stories.tsx` (title
`CRM/Records/FieldHistoryPage`, exports `AllFields`, `HonestEmptyField`, `Loading`), each with
`"pass": true` and an empty `"errors"` array — no `pageerror`, no console error,
`#storybook-root` non-empty for each. This is the base render-reality check every step below builds
on: if this step fails, none of the `[live]` steps below can be trusted (they all read off these
same three stories).

---

## Step 1 [live]: Header count + source-of-truth note on `AllFields` (AC-field-history-1)

Render `FieldHistoryPage.stories.tsx`'s `AllFields` story (via `make storybook`, per "Bring up"
above, or inspect the `.tmp/fe-uat/<story-id>.png` screenshot `make fe-uat` just captured in
Step 0).

**Expected:** the header reads `5 fields · 4 changes` — the `DEAL` fixture's scalar field catalog
is `name`, `amount_minor`, `currency`, `stage_id`, `owner_id` (5 fields, per
`scalarFieldKeys`/`SYSTEM_FIELD_KEYS`); `ENTRIES` has 4 rows. Beneath the header, the source-of-truth
note states `Reconstructed from the append-only audit log` and `read-only projection, not editable
here`. This count is read straight off the full, unfiltered dataset — confirm it does not change as
later steps apply filters (Steps 3 and 5).

---

## Step 2 [auto]: Diff row rendering — struck-through from, arrow, highlighted to; null-token
handling never a blank cell (AC-field-history-2)

```bash
cd frontend && npx vitest run src/features/records/components/FieldHistoryGroupCard.test.tsx
```

**Expected:** exits `0`. In particular:
- `"AC-2: a diff row shows struck-through from, an arrow, and a highlighted to"` — renders a diff
  row and asserts both the from-value and to-value text render.
- `"AC-2: a null old_value renders — created — for this field's oldest entry"` — asserts `— created
  —` renders for the true oldest entry in a field's own timeline.
- `"AC-2: a null new_value renders — removed —, never a blank cell"` — asserts `— removed —`
  renders, never an empty cell.
- `"AC-2 (finding 4): a non-oldest null-old_value entry (cleared, then re-set) renders — empty —"`
  — asserts both `— created —` (the true oldest entry) and `— empty —` (a later cleared-then-reset
  entry) render correctly in the same timeline, proving `DiffRow` passes the field's full
  `allEntries` list (not just the filtered `visibleEntries`) to `originLabel`.

---

## Step 3 [auto]: Actor filter hides field groups whose visible rows are zeroed out, never a
genuinely zero-entry group (AC-field-history-3)

```bash
cd frontend && npx vitest run src/features/records/routes/FieldHistoryPage.test.tsx -t "AC-field-history-3"
cd frontend && npx vitest run src/features/records/hooks/useFieldHistoryView.test.ts
```

**Expected:** both exit `0`. In particular:
- `FieldHistoryPage.test.tsx`'s end-to-end filtering test clicks the `Agent` radio and asserts
  `field-history-group-stage_id` disappears (the Stage group's only entry is human-authored).
- `useFieldHistoryView.test.ts`'s `"AC-3: selecting Agent hides a group whose entries are all
  human-authored"` and `"AC-3 (finding 7): a genuinely zero-entry group is NOT hidden by an actor
  filter"` together prove the distinction: a group the actor filter zeroed out is hidden, but a
  field with zero total entries (AC-8's case) always stays visible regardless of which actor tab is
  selected.

---

## Step 4 [live]: Field chip narrows to one group; "All fields" restores every group
(AC-field-history-4)

On the rendered `AllFields` story, click the **Stage** field chip in the controls bar.

**Expected:** only the Stage group is visible (the Amount group disappears). Click **All fields**.

**Expected:** every group is visible again (Amount and Stage both render).

---

## Step 5 [auto]: Search hides every group with an honest empty message; Clear filters resets
actor/field/search together (AC-field-history-5)

```bash
cd frontend && npx vitest run src/features/records/routes/FieldHistoryPage.test.tsx -t "AC-field-history-3/4/5"
```

**Expected:** exits `0` — `"AC-field-history-3/4/5: filtering and Clear filters compose end-to-end"`
selects the `Agent` actor, types a non-matching search (`zzz-no-match`) and asserts the text `No
changes match this filter` renders, then clicks `Clear filters` and asserts the Stage group
reappears — proving `Clear filters` resets actor, field, and search together in one click, not
individually.

---

## Step 6 [live]: Evidence toggle on an agent-authored row — grounding quote, source link,
confidence note (AC-field-history-6)

On the rendered `AllFields` story, in the Amount group's newest diff row (the agent-authored
`21.200.000` → `17.707.200` row), click **evidence**.

**Expected:** the panel expands showing the grounding quote `offer.accepted · offer_id=of_8842 ·
gross_minor=17707200 · currency=EUR`, a `source` link, and a confidence dot with the note `high —
computed, not inferred`. Confirm the Stage group's human-authored rows never show an `evidence`
button at all.

---

## Step 7 [live]: "Explain this number" on the computed money field — net + 19% MwSt. = gross
(AC-field-history-7)

On the rendered `AllFields` story, in the Amount group, click **Explain this number**.

**Expected:** the box reveals `net 148.800,00 € + 19% MwSt. 28.272,00 € = 177.072,00 €` (exact
worked numbers — 177072.00 EUR gross / 1.19 = 148800.00 EUR net, tax = gross − net = 28.272,00 €),
alongside the always-visible `computed server-side · never free-typed` provenance chip. Confirm the
Stage group never shows an "Explain this number" link (it is not a money field). Click the link
again — the box closes.

---

## Step 8 [live]: Honest empty history for a zero-entry field (AC-field-history-8)

Render the `HonestEmptyField` story (sidebar: **CRM → Records → FieldHistoryPage →
HonestEmptyField**, or the equivalent `.tmp/fe-uat/<story-id>.png` capture).

**Expected:** the Stage group shows the text `Set on create and never changed — the audit log
records no edits. An empty history is honest, not a gap.` instead of a blank timeline or any diff
rows — a field with genuinely zero recorded changes gets an honest message, never fabricated or
omitted content.

---

## Step 9 [live]: STATE-2 — chrome renders immediately, groups area shows a skeleton while loading

Render the `Loading` story (sidebar: **CRM → Records → FieldHistoryPage → Loading**, or the
equivalent `.tmp/fe-uat/<story-id>.png` capture).

**Expected:** the header (`Field change history`) and the source-of-truth note render immediately;
the groups area beneath the controls shows a loading skeleton (`data-testid="field-history-
skeleton"`), never a blank area or a premature error/empty state.

---

## Step 10 [auto]: STATE-3 — a field-history fetch failure renders the honest error card, never a
partial timeline

```bash
cd frontend && npx vitest run src/features/records/routes/FieldHistoryPage.test.tsx -t "STATE-3"
```

**Expected:** exits `0` — `"STATE-3: a field-history fetch failure shows the honest error card,
never a partial timeline"` mocks `GET /field-history` failing (the entity record fetch still
succeeds) and asserts the text `Couldn't load the change history.` renders — the honest failure
card, never a half-rendered or guessed timeline.

---

## Step 11 [auto]: STATE-4 — a 403 renders the distinct no-access card, checked before the generic
error card

```bash
cd frontend && npx vitest run src/features/records/routes/FieldHistoryPage.test.tsx -t "STATE-4"
```

**Expected:** exits `0` — `"STATE-4: a 403 on the field-history fetch shows the distinct no-access
card, not the generic error"` mocks `GET /field-history` returning a 403 and asserts text matching
`/you don't have access/i` renders while `Couldn't load the change history.` (the generic STATE-3
text) is explicitly absent — proving the 403 branch is checked before the generic error fallback,
not conflated with it.

---

## Verdict

`PASS` only if Step 0's `make fe-uat` manifest is `pass: true` AND every `[live]` step's rendered
Storybook output matches its Expected AND every `[auto]` step's named test(s) exit `0`. Any
mismatch is a `BLOCK` — cite the step, the exact command run, and the expected-vs-actual text/value
observed, and route the fix to `react-dev` (including a fix to this guide itself, if the guide's
own fixture-value transcription is what's wrong — recompute from `FieldHistoryPage.stories.tsx`'s
`makeClient()` directly, the single source of truth for every number in this guide).
