# Manual guides — human check-and-test walkthroughs

One click-through / `curl` walkthrough per shipped chapter, for a human tester verifying what
landed without reading the code. Each guide is self-contained: a one-time setup, an ordered set of
steps with an **Expected** result for each, and an **Automated counterpart** table pointing at the
gates that prove the same thing.

| Guide | Checkpoint | What it verifies |
|---|---|---|
| [epic01/](epic01/README.md) | Epic 01 (T01–T23) | People/orgs, dedupe/merge, relationship strength, pipelines & deals, archive/restore, STATE-1..5 |
| [custom-fields/](custom-fields/README.md) | GitHub epic **#103 — CF** | Custom-fields admin screen, governed add/rename/retire engine, core-field parity, contract + catalog |
| [activities-and-timeline/](activities-and-timeline/README.md) | GitHub epic **#83 — AT** | Activity store + idempotent capture, timeline read vocabulary, update/archive events, relink |
| [overnight-agent/](overnight-agent/README.md) | GitHub epic **#104 — ONA** | Overnight reconciliation pass: no-guess gate + tier router, close-date hygiene, reconciliation/integrity/stalled-recovery proposals, staged-not-committed — *test-driven (no UI/endpoint yet)* |
| [offers-and-products/](offers-and-products/README.md) | GitHub epic **#105 — OP** | Products/rate-card + offer-template admin, offer create/line-items/server-computed totals, render + confirm-first send, accept→deal-amount sync, AI authoring, offer builder screen |
| [records-depth/](records-depth/README.md) | GitHub epic **#84 — RD** | Field-history projection, formula GENERATED columns, quotas + attainment, org hierarchy roll-up, attachments (blob seam + scan gate) — five screens + backend depth |

Common prerequisites for every guide (repeated per-guide as well):

```bash
make infra-up && make migrate-up && make seed-reset && make run   # backend + infra
make fe-dev                                                        # frontend (second terminal)
```

Seeded roles are `admin` / `rep` / `readonly` / `manager` at `<role>@example.com`, password
`changeme`. The dev workspace is `00000000-0000-0000-0000-000000000001`.
