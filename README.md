# Margince skeleton

This is the factory's **input package** — the verified-running scaffold the dark
factory clones and builds on top of (`factory/2026-07-02-input-package-and-spec-gate-design.md`
§3.1). It is the architecture made real before any feature exists: monorepo layout,
`make check` green on day zero, docker-compose infra with RLS, migration harness with
the base schema, a backend that serves, a frontend shell that renders, the contract
pipeline wired end to end, CI config, and one thin vertical slice (Person) as the
executable recipe a worker copies the shape of for its first ticket.

It was harvested from the frozen `margince-poc` reference repo. See `HARVEST.md` for
full provenance — what was lifted, adapted, or dropped, and why.

## Boot it

From a clean checkout, in this directory (`skeleton/`):

```
make infra-up && make migrate-up && make seed-dev && make run
```

This starts Postgres (pgvector, RLS-enabled) + Redis + MinIO, applies all migrations,
loads the dev seed data, and starts the API server on `:8080`.

In a second terminal, start the frontend:

```
make fe-dev
```

The Vite dev server proxies `/api` to the backend. Open the URL it prints (typically
`http://localhost:5173`).

## Isolated UAT env (per worktree)

To run a full UAT stack that won't collide with another worktree's — one shared infra,
a private database per slug, derived host ports:

```
make uat_env UAT_SLUG=<slug>          # spin: own db crm_uat_<slug> + backend/FE on derived ports
make uat_env_stop UAT_SLUG=<slug>     # stop (add DROP=1 to also drop the db)
```

`uat_env` keeps the ONE shared Postgres/Redis/MinIO on their default ports, but gives the
slug its own database `crm_uat_<slug>` (created, migrated, seeded like the dev workspace)
and derives the backend + Vite ports deterministically from the slug — so two worktrees
can run live UAT at once without clashing on data or ports. The FE's `/api` proxy follows
its backend via `BACKEND_PORT`. All logs stream to one file and the ready banner prints the
paths:

```
UAT env 'smoke' ready
  backend  http://localhost:8722
  frontend http://localhost:5815
  logs     .tmp/uat/smoke/uat.log
  stop     make uat_env_stop UAT_SLUG=smoke
```

The swarm's UAT runner passes only `UAT_SLUG` (derived from the worktree). This replaces the
factory's old `swarm uat env` / `COMPOSE_PROJECT_NAME` approach.

## Fast fe-only UAT: component capture

For a frontend-only change, `make fe-uat` renders just the *changed* component's Storybook
stories in a headless Chromium and screenshots them — no live stack, no DB:

```
make fe-uat                 # scoped to the branch diff; artifact: .tmp/fe-uat/manifest.json
make fe-uat ARGS="--build"  # force a fresh static Storybook build first
```

It **fails** if a story does not render clean (thrown error, console error, or empty root),
and if a changed component has no story at all (the swarm then has `react-dev` author one).
It is not wired into `make check` — it is the fe-only UAT lane the coordinator runs instead
of booting the full stack.

## Log in

The dev seed (`backend/seed/dev.sql`) creates four users in the same dev workspace, all
with password **`changeme`**:

| Email | Role |
|---|---|
| `admin@example.com` | admin |
| `rep@example.com` | rep |
| `readonly@example.com` | read_only |
| `manager@example.com` | manager |

It also seeds three people — **Alice Müller**, **Bob Schmidt**, **Carol Wagner** — so
the People screen has real data to render.

## What to expect once you're in

Log in, then click through the rail: every route renders (Home, People, Companies,
Leads, Deals, Tasks, Inbox, Reports, Ask AI, Settings, and Members for admins). Epic 01
(T01–T23) shipped real, working screens for the CRM core:

- **People** and **Companies** — sortable, paginated lists with relationship-strength
  cells, dedupe/merge on create, and a strength-scoring engine (no user-editable
  "strength" field — it's always computed and traceable).
- **Person 360** and **Company 360** — full record views: strength card + evidence
  drawer, linked deals, stakeholders, and (on companies) a partner panel.
- **Deals** — a pipeline board and table (drag-to-advance with 🟡 approval gating on
  sensitive transitions), a weighted pipeline roll-up, a stalled-deal flag, partner
  registration, and a deal 360 with stepper/stakeholders/history.
- **Archive/restore** on all six screens above, plus the STATE-1..5 honest-states floor
  (empty/loading/error/no-permission/nothing-grounded) audited across every one of them.

The remaining rail items (Leads, Tasks, Inbox, Reports, Ask AI) are still placeholders —
they render, but carry no feature yet, same as day zero. See
[`docs/manual-guide/epic01/README.md`](docs/manual-guide/epic01/README.md) for a full
manual-test walkthrough of everything above.

## Verify it boots without clicking anything

```
bash scripts/verify-boot.sh
```

Scripts the same story as a curl-only smoke test: logs in as the seeded admin, reads
`/people` with the resulting session cookie and asserts Alice/Bob/Carol are present,
and confirms the frontend build produces real output. Exits non-zero on any failure —
this is gate D-H0's scripted half, and it's what the foundation's `factory-g0` CI
workflow runs on every change to `skeleton/**`. See the script's header comment for the
exact preconditions it assumes.

## The gate suite

```
make check
```

runs all 19 gates in a fixed, cheapest-first order: format → lint → file-length →
codegen-drift → DAG/invariants (architecture, jurisdiction, audit-coverage,
audit-coherence, RLS store-path) → doc-style/craft-doc/image-pins → Go test lanes →
frontend static checks → Go tests → frontend tests. It must be green before anything
merges. `make help` lists every target; `make tools` bootstraps the lint/codegen
binaries a fresh machine needs.

`make check` splits into `make check-backend` (format → lint → invariants → Go tests) and
`make check-fe` (frontend static + tests) — CI runs these as two parallel jobs
(`backend-gates`, `fe-gates`), so the full local gate suite is now enforced in CI with no
gap for a local-only gate to slip through.

## Docs

`docs/` is populated: [`docs/README.md`](docs/README.md) is the entry point into the
full spec — product, architecture, subsystems, quality, recipes, and decision records.
Start there for how the spec is organized and how to read it; `AGENTS.md` still carries
the one `## Craftsmanship` section a deterministic gate depends on directly.

For hands-on verification of what's shipped so far, see
[`docs/manual-guide/epic01/README.md`](docs/manual-guide/epic01/README.md) — an ordered,
click-through manual-test guide covering Epic 01 (T01–T23).
