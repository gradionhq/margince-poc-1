# Margince skeleton — scaffold notes

This is the `skeleton/` scaffold produced by the harvest from `margince-poc` (see
`HARVEST.md` for provenance). It is a verified-running root layout — infra boots,
`go.work` resolves — with the specification and reference docs under `docs/`; the
index below is the on-ramp.

The `## Craftsmanship` section below is copied verbatim from the poc's `AGENTS.md`
because the `check-craft-doc` gate requires it to be present here.

## Documentation index — read before you touch that area

The authoritative maps are [`docs/overview.md`](docs/overview.md) (the full chapter
map) and [`docs/README.md`](docs/README.md) (the tree + reading order) — start there.
This is the fast on-ramp to the load-bearing docs; consult the one that governs what
you are changing and follow it, don't improvise:

| Area | Doc | Read it before… |
|---|---|---|
| Set up & run | [docs/getting-started.md](docs/getting-started.md) | first boot, the run loop, the green-gate check |
| Product rubric | [docs/product/principles.md](docs/product/principles.md) | any decision — P1–P14 is what every change is tested against |
| Module map & seams | [docs/architecture/architecture.md](docs/architecture/architecture.md) | crossing a module boundary |
| Data model & tenancy | [docs/architecture/data-model.md](docs/architecture/data-model.md) | a migration, an RLS/tenancy question, a unique-index gotcha (DM-CONV-16) |
| API & contract | [docs/architecture/api-conventions.md](docs/architecture/api-conventions.md), [contract-pipeline.md](docs/architecture/contract-pipeline.md) | touching `crm.yaml` or a handler |
| Frontend | [docs/architecture/frontend.md](docs/architecture/frontend.md), [web-design-system.md](docs/architecture/web-design-system.md) | any `frontend/` change |
| The gate registry | [docs/quality/quality-gates.md](docs/quality/quality-gates.md) | understanding what blocks a merge |
| Testing | [docs/quality/testing.md](docs/quality/testing.md) | writing tests / choosing a lane |
| Decisions | [docs/adr/README.md](docs/adr/README.md) | anything load-bearing — cite the ADR |

**Recipes — the exemplar-first how-tos** in [docs/recipes/](docs/recipes/): follow the one
that matches your task end to end — [add a field](docs/recipes/add-a-field.md) ·
[add an endpoint](docs/recipes/add-an-endpoint.md) · [add a migration](docs/recipes/add-a-migration.md) ·
[add a screen](docs/recipes/add-a-screen.md) · [add an event](docs/recipes/add-an-event.md) ·
[add a vertical slice](docs/recipes/add-a-vertical-slice.md).

**Authoring or updating a subsystem chapter?** Copy [docs/subsystems/_TEMPLATE.md](docs/subsystems/_TEMPLATE.md)
and follow it exactly: prose explains, and below the single `## Appendix` marker the normative
facts are pinned in tables/blocks with stable IDs. A fact promised in prose but left unpinned is
a gate blocker — these conventions are enforced by `make check`, not optional.

## Craftsmanship

The **beauty bar** — the code-quality analogue of P8's UI rubric, applied to the source. `make check`
proves code is *correct, safe, consistent*; this proves it reads as a senior human's work. The full
standard (anti-tell catalog T1–T10, positive rubric, severity model) is
[docs/quality/craftsmanship.md](docs/quality/craftsmanship.md); the machine-readable rubric the gate consumes is
[cli/craft/rubric/rubric.json](cli/craft/rubric/rubric.json). Re-read §2/§3 before writing non-trivial code.

**Authoring rules (write it right the first time):**
- Write as a senior engineer reviewing their own work would: **match this file's style**, justify every
  comment, every abstraction, every dependency. Comments say *why*, not *what* (T1). No defensive noise
  (T2). No abstraction without a second concrete caller today (T3). Domain names, not textbook filler
  (T4). No `any`/`as`/unchecked assertions (T6). Handle the hard edge cases honestly (T7). No dead or
  speculative code, no `TODO` without an issue ref (T8). Prefer reuse (`gw-*`, existing helpers) and the
  one opinionated default (P1) over new structure.
- **Pre-submit self-check** (run before opening a PR): *Would a senior engineer write it this way? Does
  it match the surrounding file? Can I justify every comment, abstraction, and dependency? Are the hard
  edge cases actually handled? Is this the smallest diff that does the job?*

**Loop contract (how to respond to a craftsmanship-gate BLOCK):**
- The gate writes `// CRAFT-FIX[<id>] …` markers into the source. Read every marker, **fix the
  underlying code, and delete the marker.** A marker left in the tree fails the deterministic **residue
  gate** — markers are self-cleaning by construction and must never reach a merge.
- You **cannot override** a block. If a finding is genuinely wrong, replace its `CRAFT-FIX` with a
  `CRAFT-DISPUTE[<id>]: <reasoning>` marker (also residue-blocking) — it routes that one finding to human
  adjudication, not to a merge.

**Two checkpoints — deterministic gates per push, taste review once per branch:**
CI runs the full gate suite (deterministic gates, craft-residue, craftsmanship, dco) on every push to
main and on ready-for-review PRs; a local pre-push hook (`.claude/hooks/pre-push-gate.sh`, wired in
`.claude/settings.json`) runs the cheap deterministic subset on every push as a fast pre-check.
- **Per push** — the hook blocks `git push` until, for the current `HEAD`: **craft-residue** is clean,
  **craft tests** pass, and **deterministic gates** (`make check-craft-doc fmt-check vet`) pass. These
  are cheap and run for real in the hook. (`dco` sign-off stays a CI/advisory concern, not enforced
  here.)
- **Once at branch-finish** — the expensive **craftsmanship taste review**. It is deliberately *not*
  per-push: re-reviewing the growing cumulative `origin/main...HEAD` diff on every push is redundant
  work. Before opening the PR (the `finishing-a-development-branch` checkpoint, alongside the
  `swarm-reviewer` FINAL-UAT + `code-reviewer` gates), dispatch the `craftsmanship-reviewer` agent
  **once** over the whole `origin/main...HEAD` (the agent form of `craft review` — same rubric, same
  calibrated verdict). On **BLOCK**, fix the findings and re-review — do not open the PR. CI's
  craftsmanship job is the backstop on the PR.

`AGENTS.md` is a signpost, not a guardrail: the teeth are the craftsmanship gate + the residue gate.
There is one `AGENTS.md` (this file); its `## Craftsmanship` section is the single standard for every module. There are no per-module `AGENTS.md` files — match the surrounding code in whichever module you edit.
