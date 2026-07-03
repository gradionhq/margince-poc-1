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
Leads, Deals, Tasks, Inbox, Reports, Ask AI, Settings, and Members for admins) — the
ones without a real feature yet show a placeholder page, not a dead link or a 404.
`/people` is the one real vertical slice: it loads, lists, and lets you inspect Alice,
Bob, and Carol end to end (API → contract types → hook → list → card), with real
loading/empty/error states.

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

## Docs

There is no `docs/` content in this skeleton yet — `skeleton/docs/` exists only as an
empty scaffold (`subsystems/.gitkeep`). The full spec (product, architecture,
subsystems, quality, recipes, decisions, glossary — see the design doc §3.2) is
**Phase 2** work, authored once the skeleton itself is settled. `AGENTS.md` carries the
one section (`## Craftsmanship`) that a deterministic gate already depends on; nothing
else has landed.
