# RD-T12 Live-UAT Guide — Quota screen (quota.html, S-E15.8d)

Exercises the full RD-T12 surface: `frontend/src/features/records/routes/QuotaPage.tsx` (mounted
at `/quotas/:id`) plus its five components — `AttainmentRing`, `QuotaExplainBox`,
`ContributingDealsTable`, `PeriodBar`, `TargetEditor`, `TeamRollupRail` — all inside the existing
`frontend/src/features/records/` module. Traces to AC-quota-1..8 and STATE-1..4
(`workspace/specs/rd-t12.md`, `workspace/plans/2026-07-09-rd-t12.md`).

**This ticket is frontend-only** — `GET/PATCH /quotas/{id}` and `GET /quotas/{id}/attainment`
already shipped and are fully tested in RD-T06; no `crm.yaml`/Go change is in scope. Per the
scope-aware gate matrix (`.ai/agents/swarm-coordinator.md`, § Scope-aware gates,
"fe-only → component-capture lane (B8)") this ticket runs in **fe-uat mode**: `make fe-uat`, a
change-scoped Storybook render+capture against `QuotaPage.stories.tsx`'s `Attainment` story — **no
live stack, no `make uat_env`, no `psql`/`curl` anywhere in this guide.** Every `[live]` step below
is a browser/Storybook action (render the story, click a control inside it) against the one seeded
fixture baked into that story's `makeClient()`; every `[auto]` step names the exact test file (and,
where useful, the exact test name) in this branch that proves the corresponding acceptance
criterion. There is no `[manual]` step — nothing here needs human/visual-only judgment beyond what
the capture already proves.

## Fixture reference (from `QuotaPage.stories.tsx`'s `Attainment` story — read once, reuse below)

```
quota_id: quota-riya-q3        period: 2026-07-01 .. 2026-09-30 (Q3 2026)
target_minor: 28000000         -> 280.000,00 EUR
closed_won_minor: 31387200     -> 313.872,00 EUR
gap_minor: 3387200             -> +33.872,00 EUR
attainment_pct: 112.1          -> ring reads "112%"
pace_pct: 64, band: "met"      -> pace line reads "Target met"
contributing_deals:
  deal-baer   "BÄR Pharma — Packaging QA"      closed 2026-08-14  base_value_minor 17707200 -> 177.072,00 EUR
  deal-brandt "Brandt — Line QA Retrofit"       closed 2026-07-29  base_value_minor  9450000 ->  94.500,00 EUR
  deal-meyer  "Meyer Logistik — Audit Trail"    closed 2026-09-02  base_value_minor  4230000 ->  42.300,00 EUR
members: [{ user_id: "u-riya", display_name: "Riya Patel" }]   (viewer: rep "Riya Patel")
```

## Bring up the fe-uat lane

From the repo root (`external/target/` in this factory checkout, or the branch worktree root when
run by the `swarm-uat-runner`):

```bash
make fe-uat
```

This builds a fresh static Storybook (forced, never cached — so it renders the current diff, not a
stale build), then drives every in-scope story (every story co-located with a component this
branch touched, which includes `QuotaPage.stories.tsx`'s `Attainment` story plus each of the five
components' own stories if they carry one) headlessly via Playwright, screenshots each, and writes
`.tmp/fe-uat/manifest.json`.

For the interactive `[live]` steps below (clicking "Explain this number", clicking a period-bar
chip), also start the interactive Storybook dev server in a second terminal:

```bash
make storybook
```

Then open `http://localhost:6006` and use the sidebar to navigate to **CRM → Records → QuotaPage →
Attainment** (or go straight to
`http://localhost:6006/?path=/story/crm-records-quotapage--attainment` if that resolves — if the
exact slug differs, the sidebar path above always works). This is the same `Attainment` story
`make fe-uat` captures — the interactive server just lets you click inside it instead of only
screenshotting it at rest.

---

## Step 0 [auto]: `make fe-uat` — the story renders clean

```bash
make fe-uat
```

**Expected:** exits `0`; `.tmp/fe-uat/manifest.json` has `"pass": true`; its `stories` array
contains an entry whose `id` corresponds to `QuotaPage.stories.tsx`'s `Attainment` story (title
`CRM/Records/QuotaPage`, export `Attainment`) with `"pass": true` and an empty `"errors"` array —
no `pageerror`, no console error, `#storybook-root` non-empty. This is the base render-reality
check every step below builds on: if this step fails, none of the `[live]` steps below can be
trusted (they all read off the same story).

---

## Step 1 [live]: Ring center, closed-won, target, gap (AC-quota-1)

Render `QuotaPage.stories.tsx`'s `Attainment` story (via `make storybook`, per "Bring up" above, or
inspect the `.tmp/fe-uat/<story-id>.png` screenshot `make fe-uat` just captured in Step 0).

**Expected:** the attainment ring's center reads `112%`; the panel's "Closed-won this period" row
reads `313.872,00 €`; the "Target" row reads `280.000,00 €`; the "Gap to target" row reads
`+33.872,00 €` (signed, positive here since closed-won exceeds target) — all four numbers exactly
match the fixture table above, read straight from the server-shaped `QuotaAttainment` object, never
re-derived client-side. The pace line beneath reads `Target met` (fixture's `attainment_pct` 112.1
is ≥ 100).

---

## Step 2 [auto]: Ring color/pace line key off the server's `band`/`pace_pct`, never a client
threshold (AC-quota-2/3)

```bash
cd frontend && npx vitest run src/features/records/components/AttainmentRing.test.tsx
```

**Expected:** exits `0`. In particular:
- `"AC-quota-2: never client-recomputes the band — a 'behind' server band still renders the danger
  color even at a high pct"` — passes a fixture with `attainment_pct: 95` (a naive ≥60% threshold
  read would call this "accent") but `band: "behind"`, and asserts the danger color class
  (`.text-gf-status-danger`) still renders — proving the ring trusts the server's `band` field, not
  a client-side percentage cutoff.
- `"AC-quota-3: pace line reads 'Target met' at >=100%"`, `"...'Ahead of pace' when attainment_pct
  >= pace_pct but < 100"`, `"...'Behind pace' when attainment_pct < pace_pct"` — all three pace
  wordings, each keyed off comparing the server's own `attainment_pct` to its own `pace_pct`.

---

## Step 3 [live]: "Explain this number" — formula, summed deal values, human-set target flag,
provenance chip (AC-quota-4)

On the rendered `Attainment` story, click **"Explain this number"** (the accent link on the
attainment panel).

**Expected:** a box opens (this is the element carrying `data-testid="quota-explain-box-content"`)
showing:
- The formula text `attainment = Σ(closed-won base_value) ÷ target, calculated to the cent`.
- The three contributing deals' values summed, in order: `177.072,00 € + 94.500,00 € + 42.300,00 €`
  followed by `= 313.872,00 € (3 deals, close_date in this period)`.
- The target line `target = 280.000,00 € (human-set)` — the target is explicitly flagged
  human-set, not server-derived.
- The final line `attainment = 313.872,00 € ÷ 280.000,00 € = 112%`.

Independent of the toggle, confirm the "computed server-side" provenance chip is visible on the
panel at all times (it sits next to the "Explain this number" link, not inside the toggled box).
Click "Explain this number" again — the box closes.

---

## Step 4 [live]: Contributing-deals table — three rows, pill, footer sum, exclusion note
(AC-quota-5)

On the same rendered story, scroll to the contributing-deals table beneath the attainment panel.

**Expected:** exactly three rows, one per fixture deal:

| Deal | Closed | Status | Counted amount |
|---|---|---|---|
| BÄR Pharma — Packaging QA | 2026-08-14 | Closed-won | 177.072,00 € |
| Brandt — Line QA Retrofit | 2026-07-29 | Closed-won | 94.500,00 € |
| Meyer Logistik — Audit Trail | 2026-09-02 | Closed-won | 42.300,00 € |

Each "Status" cell renders as a pill/badge reading "Closed-won" (not plain text). The footer row
reads "Counted total" `313.872,00 €` — the same figure as Step 1's "Closed-won this period", read
straight off `attainment.closed_won_minor`, never re-summed from the three visible rows client-side.
Beneath the footer, a caption notes open/lost/omitted deals are excluded from this table (reads
"... open / lost / omitted deals excluded").

---

## Step 5 [auto]: Target editor — save PATCHes `target_minor` + `If-Match`, human-typed toast;
zero/empty refuses without a PATCH (AC-quota-6)

```bash
cd frontend && npx vitest run src/features/records/components/TargetEditor.test.tsx
```

**Expected:** exits `0`. In particular:
- `"AC-quota-6: saving a new German-grouped value PATCHes target_minor and toasts the human-typed
  confirmation"` — types `300.000` into the target input, clicks "Save target", and asserts
  `apiClient.PATCH` was called on `/quotas/{id}` with `params.header["If-Match"]` set to the
  quota's current `version` (`"3"`) and `body: { target_minor: 30000000 }` (the German-grouped
  input parsed to minor units), and that the success toast reads
  `"Target saved as human-typed — change logged, attainment recomputed"` (via `onToast("success",
  ...)`).
- `"AC-quota-6: a zero/empty entry toasts the refusal and never PATCHes"` — clears the input,
  clicks "Save target", and asserts the toast reads `"Enter a target amount in EUR"` (via
  `onToast("error", ...)`) and `apiClient.PATCH` was never called — the client-side guard runs
  before any network round-trip, never round-tripping a known-invalid value.

*(This step stays `[auto]`, not `[live]`: `QuotaPage.stories.tsx`'s fixture wires a real
`QueryClientProvider` with no `apiClient` mock, so clicking "Save target" inside the Storybook
capture would attempt a genuine network PATCH with nothing to receive it — the fe-uat lane has no
live stack. The unit test above exercises the identical component/click path against a mocked
`apiClient`.)*

---

## Step 6 [auto]: Period-bar chips — only the current quarter is active; prior/next toast
read-only/not-yet-set (AC-quota-7)

```bash
cd frontend && npx vitest run src/features/records/components/PeriodBar.test.tsx
```

**Expected:** exits `0`. In particular:
- `"AC-quota-7: renders the current quota's own quarter as the only active chip"` — asserts `Q3
  2026` (the fixture quota's own `period_start` quarter) renders as the accent/active chip.
- `"AC-quota-7: clicking the prior-quarter chip toasts read-only/closed"` — clicks the `Q2 2026`
  chip and asserts the toast text matches `/closed/i`.
- `"AC-quota-7: clicking the next-quarter chip toasts not-yet-set"` — clicks the `Q4 2026` chip and
  asserts the toast text matches `/not yet set/i`.

On the live-rendered `Attainment` story (visual cross-check, not the load-bearing assertion): the
period bar under the page header reads `Q2 2026 · closed` (plain chip), `Q3 2026 · current`
(accent-bordered, the only one styled as active), `Q4 2026 · not set` (plain chip) — matching the
fixture's `period_start: "2026-07-01"`.

---

## Step 7 [live]: Team roll-up rail — at least one rep row, mini-bar, percent, method note
(AC-quota-8)

On the rendered `Attainment` story's right rail, locate the "Team attainment" section.

**Expected:** at least one rep row renders — the fixture seeds exactly one member (`Riya Patel`,
`user_id: u-riya`), matching the quota's own `owner_id`. That row shows the name `Riya Patel`, a
mini-bar (a horizontal fill proportional to its percent, capped at 100% width), and the percent
`112%` (the same server-computed `attainment_pct` as Step 1's ring, rounded, reused rather than
re-fetched). Beneath the row(s), a caption reads exactly `team roll-up = Σ closed-won ÷ Σ targets ·
auditable`.

---

## Step 8 [auto]: STATE-2 — chrome renders immediately, ring shows a skeleton while loading

```bash
cd frontend && npx vitest run src/features/records/routes/QuotaPage.test.tsx -t "STATE-2"
```

**Expected:** exits `0` — `"STATE-2: renders chrome immediately with a loading skeleton, then the
ring once data resolves"` mounts `QuotaPage` with the quota/attainment/members `GET`s all
in-flight, then awaits `screen.findByText("112%")` resolving once the mocked responses settle,
proving the page's header/period-bar chrome is present before the ring's own data-driven content
paints (the ring's own STATE-2 skeleton — `data-testid="attainment-ring-skeleton"` — is the
loading placeholder shown in the gap, per `AttainmentRing.test.tsx`'s own
`"STATE-2: shows a skeleton when isLoading"` case).

---

## Step 9 [auto]: STATE-1 — `attainment_target_zero` 422 renders the honest "no target set"
message, not the generic error card

```bash
cd frontend && npx vitest run src/features/records/routes/QuotaPage.test.tsx -t "STATE-1"
```

**Expected:** exits `0` — `"STATE-1: 422 attainment_target_zero renders the honest 'set a target'
message, not the generic error"` mocks `GET /quotas/{id}/attainment` returning a 422 with `error:
{ code: "attainment_target_zero" }` and asserts the page renders text matching `/no target set/i`
— this is the distinct STATE-1 card (`AttainmentRing`'s `isTargetZero` branch), never the generic
"Couldn't recompute attainment." STATE-3 card.

---

## Step 10 [auto]: STATE-3 — generic attainment failure renders the honest error card, and — once
a prior successful fetch happened — states the last successful compute time (not a stale figure)

```bash
cd frontend && npx vitest run src/features/records/components/AttainmentRing.test.tsx -t "STATE-3"
```

**Expected:** exits `0` — `AttainmentRing.test.tsx`'s `"STATE-3: shows the generic honest error
card otherwise"` renders the ring with `isError={true}` and no `isForbidden`/`isTargetZero` flag,
and asserts the text `/couldn't recompute/i` is present — the generic honest failure card, distinct
from STATE-1/STATE-4's own dedicated messages. On the composed `QuotaPage`
(`frontend/src/features/records/routes/QuotaPage.tsx`), this same branch is followed by a caption
reading either `"Last successful compute: <as_of_date>."` (once `attainmentQuery.data?.as_of_date`
has been observed at least once — tracked in `lastGoodComputeAt` state, never re-showing the stale
attainment object itself) or `"No successful compute yet."` before any success has landed — read
this caption directly in `QuotaPage.tsx`'s render branch (`attainmentQuery.isError && !isForbidden
&& !isTargetZero`) since no standalone `QuotaPage.test.tsx` case isolates the last-good-time text
independent of the STATE-1/STATE-4 cases above; the branch is exercised whenever an attainment
fetch errors on a page that has previously fetched successfully.

---

## Step 11 [auto]: STATE-4 — a 403 renders the honest no-access message, checked before the
generic error card

```bash
cd frontend && npx vitest run src/features/records/routes/QuotaPage.test.tsx -t "STATE-4"
cd frontend && npx vitest run src/features/records/components/AttainmentRing.test.tsx -t "STATE-4"
```

**Expected:** both exit `0`.
- `QuotaPage.test.tsx`'s `"STATE-4: a 403 on attainment renders the honest no-access message"` mocks
  `GET /quotas/{id}/attainment` returning a 403 and asserts text matching `/don't have access/i`.
- `QuotaPage.test.tsx`'s `"STATE-4 (PLAN-review finding): a 403 on GET /quotas/{id} itself renders
  the honest no-access message, never the generic 'quota not found' fallback"` mocks the **base**
  `GET /quotas/{id}` call itself returning a 403 and asserts the same no-access text renders while
  `/quota not found/i` is explicitly absent — proving the 403-on-the-quota-itself path is checked
  before the generic not-found fallback, not mislabeled.
- `AttainmentRing.test.tsx`'s `"STATE-4: shows a distinct no-access message when isForbidden,
  checked before the generic error"` renders the ring directly with `isForbidden={true}` and
  asserts `/don't have access/i` is present while `/couldn't recompute/i` (the generic STATE-3
  text) is absent — the component-level proof that the ordering holds even in isolation from the
  page.

---

## Verdict

`PASS` only if Step 0's `make fe-uat` manifest is `pass: true` AND every `[live]` step's rendered
Storybook output matches its Expected AND every `[auto]` step's named test(s) exit `0`. Any
mismatch is a `BLOCK` — cite the step, the exact command run, and the expected-vs-actual text/value
observed, and route the fix to `react-dev` (including a fix to this guide itself, if the guide's
own fixture-value transcription is what's wrong — recompute from `QuotaPage.stories.tsx`'s
`makeClient()` directly, the single source of truth for every number in this guide).
