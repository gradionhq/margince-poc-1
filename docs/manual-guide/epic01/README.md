# Epic 01 manual guide — T01–T23

One ordered, click-through walkthrough for a human tester verifying everything Epic 01
(T01–T23: people/organizations, dedupe/merge, relationships & strength, pipelines & deals,
archive/restore, and the STATE-1..5 floor) shipped. It synthesizes the 23 per-ticket guides at
`workspace/manual-test/t01.md`–`t23.md` into one narrative — read this guide top to bottom
instead of opening all 23 files. Jump to a per-ticket guide only when you want the exact
curl/API-level detail behind a step.

Four ticket clusters are **contract/API-level groundwork** exercised by the automated gates
rather than a UI click-through, so this guide doesn't repeat them as manual steps — see
[Automated counterpart](#automated-counterpart--where-to-look-when-something-doesnt-match)
below, and open the tNN.md guide directly if you want to run them by hand:
`t01`/`t02` (people/org contract), `t03`/`t04` (deals contract + stakeholder-role enum),
`t10`/`t11`/`t12` (pipelines, deal CRUD, and approval-token gating at the API level — all three
are re-exercised through the UI in the Pipelines & Deals section below).

## Setup (do this once)

1. Boot the stack from a clean checkout:
   ```bash
   make infra-up && make migrate-up && make seed-reset && make run
   ```
   **Expected:** Postgres/Redis/MinIO up, migrations applied, dev seed loaded, API serving on
   `:8080`.
2. In a second terminal, start the frontend:
   ```bash
   make fe-dev
   ```
   **Expected:** Vite dev server prints a local URL (typically `http://localhost:5173`); `/api`
   proxies to the backend.
3. Log in as each seeded role in turn (`backend/seed/dev.sql`), password `changeme` for all four:

   | Email | Role |
   |---|---|
   | `admin@example.com` | admin |
   | `rep@example.com` | rep |
   | `readonly@example.com` | read_only |
   | `manager@example.com` | manager |

   **Expected:** each logs in and lands on Home; the nav rail renders People, Companies, Leads,
   Deals, Tasks, Inbox, Reports, Ask AI, Settings, and (admin only) Members.
4. The dev seed loads three people (Alice Müller, Bob Schmidt, Carol Wagner) and a pipeline with
   8 deals against one organization (Acme Corp) — the Deals section below has real data
   out of the box. **The base seed does NOT include organizations, employment relationships, or
   activities for the three people** — without those, every person/org strength card reads "no
   signal yet" and Company 360's people/org-strength cards have nothing to show. Load the People &
   Organizations fixtures now (idempotent — safe to re-run after a `seed-reset`):
   ```bash
   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
   -- Organizations
   INSERT INTO organization (id, workspace_id, name, source, captured_by)
   VALUES
     ('00000000-0000-0000-0050-000000000001',
      '00000000-0000-0000-0000-000000000001',
      'Acme Corp', 'seed', 'human:dev'),
     ('00000000-0000-0000-0050-000000000002',
      '00000000-0000-0000-0000-000000000001',
      'EmptyCo', 'seed', 'human:dev')
   ON CONFLICT (id) DO NOTHING;

   -- Employment: Alice + Bob -> Acme Corp; Carol stays unemployed (no-signal case); EmptyCo
   -- stays contact-free (org no-signal case).
   INSERT INTO relationship
     (id, workspace_id, kind, person_id, organization_id, is_primary, source, captured_by)
   VALUES
     ('00000000-0000-0000-0070-000000000001',
      '00000000-0000-0000-0000-000000000001', 'employment',
      '00000000-0000-0000-0001-000000000001',
      '00000000-0000-0000-0050-000000000001',
      true, 'seed', 'human:dev'),
     ('00000000-0000-0000-0070-000000000002',
      '00000000-0000-0000-0000-000000000001', 'employment',
      '00000000-0000-0000-0001-000000000002',
      '00000000-0000-0000-0050-000000000001',
      true, 'seed', 'human:dev')
   ON CONFLICT DO NOTHING;

   -- Activities for Alice only (email 10d ago, call 3d ago, meeting 1d ago) -- strength-eligible
   -- (PO-F-3); Bob stays employed-but-signal-free so the org's MAX-over-contacts strength has
   -- something to pick between.
   INSERT INTO activity
     (id, workspace_id, kind, subject, occurred_at, source, captured_by)
   VALUES
     ('00000000-0000-0000-0060-000000000001',
      '00000000-0000-0000-0000-000000000001', 'email', 'Initial outreach',
      now() - interval '10 days', 'seed', 'human:dev'),
     ('00000000-0000-0000-0060-000000000002',
      '00000000-0000-0000-0000-000000000001', 'call', 'Discovery call',
      now() - interval '3 days', 'seed', 'human:dev'),
     ('00000000-0000-0000-0060-000000000003',
      '00000000-0000-0000-0000-000000000001', 'meeting', 'Demo walkthrough',
      now() - interval '1 day', 'seed', 'human:dev')
   ON CONFLICT (id) DO NOTHING;

   INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, person_id)
   VALUES
     ('00000000-0000-0000-0061-000000000001',
      '00000000-0000-0000-0000-000000000001',
      '00000000-0000-0000-0060-000000000001', 'person',
      '00000000-0000-0000-0001-000000000001'),
     ('00000000-0000-0000-0061-000000000002',
      '00000000-0000-0000-0000-000000000001',
      '00000000-0000-0000-0060-000000000002', 'person',
      '00000000-0000-0000-0001-000000000001'),
     ('00000000-0000-0000-0061-000000000003',
      '00000000-0000-0000-0000-000000000001',
      '00000000-0000-0000-0060-000000000003', 'person',
      '00000000-0000-0000-0001-000000000001')
   ON CONFLICT DO NOTHING;

   UPDATE person SET title = 'Engineer'
   WHERE id = '00000000-0000-0000-0001-000000000001';

   -- Open deal on Acme so it has its own deals rail entry beyond the pipeline seed's deals.
   INSERT INTO deal
     (id, workspace_id, name, pipeline_id, stage_id, organization_id,
      owner_id, status, source, captured_by)
   VALUES
     ('00000000-0000-0000-0080-000000000001',
      '00000000-0000-0000-0000-000000000001', 'Acme Expansion (manual guide)',
      '00000000-0000-0000-0040-000000000001',
      '00000000-0000-0000-0041-000000000001',
      '00000000-0000-0000-0050-000000000001',
      '00000000-0000-0000-0010-000000000001',
      'open', 'seed', 'human:dev')
   ON CONFLICT (id) DO NOTHING;
   SQL
   ```
   **Expected:** `INSERT 0 N` per statement, no errors. Re-running after another `seed-reset` is
   safe (`ON CONFLICT DO NOTHING`). This is a trimmed, workspace-agnostic version of T18's own
   bootstrap (`workspace/manual-test/t18.md`) — open that file for the full fixture set (it adds
   a second "won" deal and a no-permission test user) if you want more coverage than this guide's
   main path needs.
5. This single Acme Corp fixture is enough for the People & Organizations walkthrough below
   (one org with two linked people — one scored, one not — for the MAX-over-contacts strength
   case; Carol and EmptyCo cover the no-signal cases). It is **not** the full org-variant matrix
   (partner vs non-partner, stalled-deal vs none, multiple staffed orgs) T20's own guide seeds for
   exhaustive company-360 coverage — open `workspace/manual-test/t20.md`'s prereqs if you want to
   walk every variant instead of the representative path below.
6. Keep this stack up for every section below — none of them re-seed unless a step says so
   explicitly (a few dedupe/merge/restore steps call `make seed-reset` again to get a clean
   pre-mutation state; each such step says so, and re-running step 4's fixture load above after
   any such reset).

## Part 1 — People & Organizations

### Contacts and Companies lists (T18)

1. Open **People**. **Expected:** a server-sorted, paginated contacts list; each row shows a
   strength cell (score/bucket, per `docs/quality/acceptance-standards.md`'s no-guess floor — a
   contact with no captured interaction shows an honest "no signal yet", never a fabricated
   score).
2. Sort by strength (server-side sort, not a client re-sort). **Expected:** the URL/query
   reflects the sort param; row order changes; a full-page reload preserves it.
3. Open **Companies**. **Expected:** the same list shape — org rows, strength cells (an org's
   strength is the max of its people's strengths, confirmed in the Company 360 section below).
4. Trigger each screen's STATE-1..5 floor directly from the UI where you can: an empty
   filter/search that legitimately returns zero rows (STATE-1), a fresh navigation (STATE-2
   loading skeleton, visible briefly), and log in as `readonly@example.com` to confirm
   write-gated affordances (archive, merge) are absent, not merely disabled (STATE-4). Full
   coverage of all five states across all six screens is the dedicated sweep in Part 3 below —
   here, spot-check the two list screens as you pass through them.

### Person 360 (T19)

5. Open Alice Müller's person 360 (from the People list). **Expected:** (with Setup step 4's
   fixtures loaded) a strength card with a real score, an "evidence drawer" control that opens
   to the interactions the score derives from (never a bare number with no way to inspect it), a
   Deals tab listing her linked deals, and a Merge action.
6. Open the evidence drawer. **Expected:** the three seeded activities (email/call/meeting) are
   listed with kind, subject, occurred_at — this is the `contributing_activities` array from
   `getPersonStrengthBreakdown` (T01/T09) rendered, not synthesized client-side.
6a. Open Carol Wagner's person 360 (never given an employer or activities). **Expected:** an
    honest "no signal yet" strength card (STATE-1) — never a fabricated score or a blank card.

### Dedupe/merge flow (T06, T07)

7. `make seed-reset`, then re-run Setup step 4's fixture load (both are idempotent) to restore a
   clean pre-mutation state, then create a new person whose email matches an existing seeded
   person exactly (e.g. re-submit Alice Müller's email through the create-person flow).
   **Expected:** the create is refused/redirected to the existing record — create-time dedupe
   (T06), never a silent duplicate.
8. Create a person with a name/email close-but-not-identical to a seeded person (fuzzy match).
   **Expected:** a dedupe candidate surfaces (T07's fuzzy scoring) rather than either an
   auto-merge or an outright block — a 🟡 human decision point.
9. From a person 360, trigger Merge against a genuine duplicate. **Expected:** the merge
   completes, the losing record's activities/relationships re-point to the survivor, and
   attempting to restore the merged-away record later (Part 3) is refused with a pointer to the
   surviving record rather than silently resurrecting a duplicate.

### Relationship strength (T09)

10. On a person or org 360, confirm the strength score changes only in response to real
    interaction signal (frequency/recency/reciprocity) — never a field a human can type into
    directly. **Expected:** no editable "strength" input exists anywhere in the UI; the value is
    always computed and traceable via the evidence drawer.

### Company 360 (T20)

11. The base seed already has a different "Acme Corp" row with no linked contacts (5 deals,
    `org_strength: null`) — Setup step 4 mints a **second** "Acme Corp" row for Alice/Bob. On the
    Companies list, open the "Acme Corp" row whose person shows "Engineer @ Acme Corp" (Alice's
    seeded title) — that's the one with fixtures. Check its **org-strength card**. **Expected:**
    the value equals Alice's strength score (the MAX over
    Acme's linked people — Bob is employed but has no activities, so he contributes no score;
    confirmed by comparing to Alice's Person 360 score from step 5). Then open EmptyCo's company
    360 (zero linked people). **Expected:** an honest "no signal yet" org-strength card (STATE-1),
    not a zeroed/blank card. (For the fuller org-variant matrix — partner vs non-partner, stalled
    vs no-stalled-deal, multiple staffed orgs — see `workspace/manual-test/t20.md`'s own prereq
    seed; this walkthrough's two orgs are the representative path, not exhaustive coverage.)
12. On that same (fixture) Acme Corp's company 360, check the **people rail** and **deals rail**. **Expected:** the people
    rail lists both Alice and Bob (each with their individual strength cell — Alice scored, Bob
    "no signal yet"); the deals rail lists Acme's deals, including the "Acme Expansion (manual
    guide)" deal Setup step 4 created.
13. Check Acme's **partner panel**. **Expected right now:** an honest "not a partner" state (Acme
    isn't registered yet) — never an empty/broken panel. Come back to this same panel after Part
    2 step 8 registers Acme as a partner: **Expected then:** the panel updates to show partner
    status.

## Part 2 — Pipelines & Deals

### Pipeline board/table (T21)

1. Open **Deals**. **Expected:** a pipeline board (Kanban-style, stages as columns) and a table
   view toggle; both render the same underlying deal set.
2. Drag a deal card from one stage to another (drag-to-advance). **Expected:** an outcome dialog
   appears for a 🟡 stage transition (e.g. advancing into a closing stage) requiring an explicit
   confirm/reason before the move commits; a plain 🟢 stage-to-stage move commits immediately.
   (This is T12's approval-token gating surfaced through the UI — see `t12.md` if you want the
   raw `X-Approval-Token` mechanics.)
3. Check the **roll-up strip** above the board. **Expected:** it shows an aggregate (count and/or
   value) per stage.

### Weighted pipeline roll-up (T13)

4. Compare the roll-up strip's value roll-up against a manual sum of `amount × stage
   probability` across the visible deals. **Expected:** they match — this is the weighted
   roll-up, not a raw sum (the golden-number correctness floor,
   `docs/quality/acceptance-standards.md` AC-X1).

### Deal 360 (T22)

5. Open a deal's 360 view. **Expected:** a stepper across the pipeline's ordered stages showing
   the deal's current position, a stakeholders rail (the people/roles on the deal, from T08's
   relationship edges), and advance/reopen controls.
6. Advance the deal one stage, then reopen it (if it's a closed/terminal stage). **Expected:**
   both actions succeed and each appears as a new entry on the **history/timeline** — nothing
   overwrites the prior state silently.

### Stalled flag (T14)

7. Find (or create, then leave untouched) a deal that has had no activity for 60+ days in an
   open, non-suppressed stage. **Expected:** it renders a "Stalled" flag/badge on the board,
   table, and 360 view. Confirm a deal with `wait_until` set in the future does NOT show stalled
   even past 60 days idle — the suppression window works.

### Partner registration (T15)

8. From a company 360 (Part 1, step 13) or a deal 360, register the org as a partner and set
   `partner_org_id` on a deal. **Expected:** the deal 360's partner indicator reflects the
   registration; the company's partner panel updates to match.

## Part 3 — Archive/Restore + STATE-1..5 sanity pass

### Archive/restore across all six screens (T17, T23)

1. From each of the six screens (contacts list, person 360, companies list, company 360,
   pipeline board/table, deal 360), archive a record. **Expected:** an honest "Archived" banner
   replaces the normal view/row (never a silent disappearance) and a Restore action is offered.
2. Restore it. **Expected:** the record returns to its normal state; no confirm dialog gates
   restore (restore carries no data-loss risk, unlike archive which does prompt for confirmation).
3. Archive a person who is a duplicate-merge loser (Part 1, step 9's merged-away record) and
   attempt to restore it. **Expected:** a 409 with a "dedupe-refused" pointer to the surviving
   record — restore is refused, not silently allowed to resurrect a duplicate.

### STATE-1..5 sanity pass (all six screens)

4. On each of the six screens, confirm the five standard states from
   `docs/quality/acceptance-standards.md`'s screen-state matrix render as real UI, never a toast:
   - **STATE-1 (empty)** — an honest empty message, never a blank or spinning-forever screen.
   - **STATE-2 (loading)** — a skeleton/progressive render, chrome first, content streaming in.
   - **STATE-3 (error)** — an honest failure card with cause + retry; one panel's failure never
     blanks the whole screen.
   - **STATE-4 (no-permission)** — log in as `readonly@example.com` and confirm write controls
     (archive/merge/advance) are absent from the response payload, not merely disabled.
   - **STATE-5 (nothing-grounded)** — any AI-sourced panel with no evidence is omitted or shows
     "not found," never a fabricated value.

## Automated counterpart — where to look when something doesn't match

Run these alongside (or instead of) a manual pass — they cover the same ground with the API/DB
directly, and are the merge gate for every ticket above:

| Command | What it proves | Notes |
|---|---|---|
| `make check` | All 19 gates: format, lint, codegen-drift, DAG/invariants, doc-style, Go + frontend static + unit tests | Must be green before anything merges |
| `make test-contracts` | TypeScript contract-compliance tests against `crm.yaml` | Covers T01–T04's contract shapes |
| `make test-integration` | Go integration lane against a real seeded Postgres (`make infra-up` first) | Covers dedupe/merge, relationships, pipeline/deal writes, restore |
| `make test-liveuat` | TEST-LANE-3 Go harnesses (`//go:build liveuat`) against the migrated+seeded dev stack | The scripted counterpart to T10–T15's API-level steps |

If a manual step above doesn't match what you see:

1. Open the per-ticket guide it was synthesized from (see the source mapping in each Part's
   heading, or open `workspace/manual-test/tNN.md` directly by ticket number) — it has the exact
   curl commands / raw API shape behind the UI step.
2. Check the owning subsystem chapter under `docs/subsystems/*.md` for the full acceptance
   criteria and gate IDs a screen/flow is supposed to satisfy.
3. `docs/quality/acceptance-standards.md` is the cross-cutting floor (performance budgets,
   STATE-1..5, release-gate catalogs) every chapter inherits without restating.
