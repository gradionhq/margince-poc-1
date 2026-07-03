# ADR-0016 — Repository layout and documentation conventions

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md) (2026-06-11, architecture-blueprint research phase). Synthesizes T1 (`foundation/research/t1-wow-codebases.md`), verified in `verification-log.md`. Refines `03-architecture.md §3.3`. Serves Goal 1 ("wow" on sight) and Goal 2 (the source ships to and is edited by clients/agents).

## Context

The source itself is the product surface (P3: code quality is a product feature) and is shipped to clients whose AI agents edit it (ADR-0002). "Wow in 90 seconds" is not a vibe; admired single-binary Go repos (pocketbase, caddy, gitea — **verified** layouts) share concrete, copyable properties. The decision codifies which we adopt and which we reject.

## Decision

**1. Domain-legible root.** Top-level `crm-*` module directories keep the domain readable straight off the repo root (beating a buried `packages/` monorepo). The §3.3 module tree in the README — not raw `ls` — carries the legibility, since `internal/` (ADR-0014) pushes module guts one level down (correction F-T1). **Reject `pkg/`** (golang-standards/project-layout self-disclaims it — **verified**); the public seam is `crm-contracts` + the seam-interface packages, not a `pkg/` dumping ground.

**2. One golden-path command.** A single pasteable command (cal.com's `yarn dx` ergonomic — **verified**) brings up Postgres+Redis, runs migrations, seeds demo data (`--with-demo`, B36), and prints the URL + credentials. This is the single biggest 90-second lever. Cold start < 2s (`§3.5`) is part of the same promise.

**3. README-driven, one screen, ending in that command.** RDD-style: what it is, the one differentiator, the one command, links out. Reference docs live in-repo under Diátaxis structure `docs/{tutorials,how-to,reference,explanation}/`. **Reject docs-only-off-site** — our source is forked and agent-edited, so docs must travel with it.

**4. Operational `AGENTS.md` (+ `CLAUDE.md`) at root**, now mainstream (gitea/twenty ship them — **verified**): build/test/seed commands, the seam list, per-module ownership, and DO-NOT-TOUCH boundaries (generated contracts, `imports_gen.go`). Marketing prose stays out. Plus `CONTRIBUTING.md`, `SECURITY.md` (security.txt / CVD per ADR-0010), and a numbered ADR trail (this directory).

**5. File/package norms:** small files; Mat Ryer service shape (per-module route registration — **generated** per ADR-0015 F-X1, not hand-maintained; explicit-deps `NewServer`; thin `main()`→`run() error`). A soft ~400–500-line norm is documented guidance; whether it becomes a lint is an ADR-0014/blueprint-E decision, not asserted here.

## Consequences

- **Positive:** a developer or agent landing cold orients in seconds and is running in one command; the repo *looks* as disciplined as it is, reinforcing Goal 1.
- **Negative / bound:** in-repo Diátaxis docs add a doc-code drift risk — mitigated by the doc-code-sync fitness function (blueprint E) that compiles AGENTS.md recipes and checks the seam catalog against the real interfaces.
- **Boundary:** layout interacts with ADR-0014 (`internal/` placement) and ADR-0015 (generated route/manifest files); those own the enforcement, this owns the human-facing conventions.
- **Amended by [ADR-0042](ADR-0042-jurisdiction-packs.md) (2026-06-24, A57):** adds the **jurisdiction-pack** layout to the domain-legible root — a top-level `crm-<cc>/` satellite module (own `go.mod`) holding *all* of one jurisdiction's code (fiscal/retention/esign/trust/locale/migrations), with its own `AGENTS.md` (§4) listing the seams it implements and its DO-NOT-TOUCH boundaries. The DE UI lives under `web/jurisdictions/de/`. Reading the root, "where is the German code?" has one answer: `crm-de/`.
