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

Common prerequisites for every guide (repeated per-guide as well):

```bash
make infra-up && make migrate-up && make seed-reset && make run   # backend + infra
make fe-dev                                                        # frontend (second terminal)
```

Seeded roles are `admin` / `rep` / `readonly` / `manager` at `<role>@example.com`, password
`changeme`. The dev workspace is `00000000-0000-0000-0000-000000000001`.
