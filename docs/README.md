---
distills: margince-foundation@5a0b29c
---
# Margince — the product spec

This tree is **the spec**: the complete, buildable definition of the Margince product.
Every ticket the factory builds derives from a chapter here; every normative fact a
ticket needs — formulas with weights, schema shapes, thresholds, cut-lines, behavioral
rules — lives here or in the contract file this package ships. The research corpus
this tree distills from (`margince specs/` in the foundation repository) is decision
history: the *why*. Anything normative that lives only there does not exist for the
build.

Three rules keep that true:

1. **Nobody edits downstream artifacts directly.** A wrong ticket is never fixed in
   the ticket — the chapter gets fixed and the affected tickets regenerate.
2. **Upstream changes arrive only through a docs change.** A new decision has no
   build effect until it lands in a chapter here.
3. **Versions pin.** This tree pins the foundation commit it distills (front-matter
   above); every ticket pins the docs commit and section anchors it derives from.

## How to read

1. `product/product.md` — what this is, for whom, and the bets.
2. `product/principles.md` — the P1–P14 rubric everything cites.
3. `product/scope.md` — what ships in V1, and the explicit OUT list.
4. `architecture/architecture.md` — the modular monolith and its enforced seams.
5. One subsystem chapter as an example of the common shape.
6. `quality/quality-gates.md` — every gate, what it checks, where it blocks.
7. `quality/acceptance-standards.md` — the cross-cutting acceptance floor every
   chapter inherits.

`overview.md` is the full chapter map. `getting-started.md` is the set-up-and-run
path.

## The tree

- **Entry** — `getting-started.md`, `overview.md`, `glossary.md`.
- **`product/`** — product, personas, journeys, scope (with the OUT list),
  principles, voice-and-copy.
- **`architecture/`** — module layout and seams, data model, API and contract
  conventions, event bus, frontend, design system, generators, jurisdiction packs,
  runtime-config boundary, operations.
- **`subsystems/`** — one chapter per subsystem. This is where the product is
  actually specified; see the status legend below.
- **`quality/`** — gate registry, craftsmanship, testing, security, acceptance
  standards, AI evals, threat model.
- **`recipes/`** — exemplar-first how-tos; every recipe points at the sample
  vertical slice as the executable reference.
- **`adr/`** — the decision index and the load-bearing decisions vendored in full.

## Chapter statuses

Every `subsystems/` chapter declares one of two statuses in its front-matter:

- **`skeleton`** — describes code that exists in this repository today. Its claims
  about paths, commands, and behavior are held to the tree by the consistency gate.
- **`planned`** — specifies code the factory will build. Its requirements must be
  fully covered by generated tickets; its code locations name where the code *will*
  live under the ratified layout.

Both statuses are equally normative. The difference is only which gate holds them
honest.

## The convention: prose explains, appendices pin

Every chapter is readable prose up to a single `## Appendix` marker; below it, the
chapter's normative facts are pinned in tight tables and blocks with stable IDs.
Humans read the chapter; the ticket generator reads the chapter *and* the appendix.
A fact promised in prose with no pin in the appendix is a gate blocker. The template
(`subsystems/_TEMPLATE.md`) carries the full authoring rules.

## What this tree is not

Not a roadmap (sequencing belongs to the backlog generator), not a research archive
(the foundation corpus keeps the why), and not a code walkthrough (the code is the
source of truth for how — chapters state intent, guarantees, and the facts that must
survive refactors).
