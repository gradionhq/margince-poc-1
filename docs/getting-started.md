---
derives-from:
  - margince-poc/docs/getting-started.md @ a11d6c08
---
<!-- PROCESS DOC — commands, paths, and make targets are this document's subject, so the
     prose-only ban-list (no fences / paths / targets above the appendix) does not apply
     here. Entry doc: no `## Appendix`. -->
# Getting started — set up and run Margince locally

> The first-15-minutes path: prerequisites, one-time setup, the run loop, and how to confirm the
> quality gate is green. This is a **process doc** — commands are its subject. For *what* the system is
> read [product.md](product/product.md); for *how* it's built read
> [architecture.md](architecture/architecture.md). Everything below is driven from the repo root with
> `make` (run `make help` for the full list).

## Prerequisites

| Tool | Why | Notes |
|---|---|---|
| **Go 1.26** | the backend under `backend/` (the toolchain is pinned there) | one Go workspace, one server binary |
| **Docker + Docker Compose** | local Postgres (pgvector) + Redis (+ MinIO for the integration lane) | via the dev compose stack `make infra-up` manages |
| **Node + pnpm** | the `frontend/` React app (a pnpm workspace) | only needed for the frontend |
| `make tools` | installs the codegen + lint binaries (`oapi-codegen`, `go-arch-lint`, `golangci-lint`, `gofumpt`) | run once |
| `make fe-install` | `pnpm install` for the `frontend/` package | run once, only if you touch the frontend |

## One-time setup

```bash
make tools          # codegen + lint binaries (Go side)
make fe-install     # frontend deps (only if working in frontend/)
```

## Run it — the backend

```bash
make infra-up       # start Postgres (pgvector) + Redis in Docker
make migrate-up     # apply all migrations from backend/migrations/
make seed-dev       # seed the default dev workspace (idempotent)
make run            # build + run the server at http://localhost:8080
```

The default dev workspace id is `00000000-0000-0000-0000-000000000001`. Open a SQL shell against the
dev database any time with `make psql`. Stop the infra with `make infra-down`.

## Run it — the frontend

```bash
make fe-dev         # Vite dev server, run from the frontend/ package
```

In development Vite proxies `/api` → `localhost:8080` and injects the dev workspace/user headers, so the
backend must be running (`make run`). See [frontend.md](architecture/frontend.md) for the proxy and the
design-system rules.

## What a fresh clone should give you

The commands above are the whole setup — a fresh clone that has run them is a working product, not a
scaffold. Concretely: the stack boots clean, you can log in as a seeded user, every route on the
navigation rail renders, and the seeded people are visible in the app with real loading, empty, and
error states behind them. That is the human-visible boot criterion; if any part of it fails on a fresh
clone, the repository is broken — not your machine.

## Confirm the gate is green

```bash
make check          # the fast, deterministic dev gate — run before every commit
```

`make check` runs format → lint → invariants → tests (Go, then frontend). If it's red, the work isn't
done. The heavier **integration** and **live-stack UAT** lanes need the live services and run
separately — see [testing.md](quality/testing.md). For the full list of gates and where each one blocks,
see [quality-gates.md](quality/quality-gates.md).

## Where to go next

- The reading order for understanding the system: [README.md](README.md).
- Where code lives and the end-to-end path to add a feature: [code-organization.md](architecture/code-organization.md).
- The full chapter map: [overview.md](overview.md).
- The full command list: `make help`.
