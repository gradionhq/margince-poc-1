<!--
AUTHORING TEMPLATE FOR A SUBSYSTEM CHAPTER — copy this file to docs/subsystems/<name>.md
and DELETE this comment block.

A chapter has two regions with different rules:

  PROSE (everything above the `## Appendix` marker) — a SYSTEM EXPLANATION, not a code
  walkthrough. It documents intent and behaviour so it stays true across refactors; the
  code is the source of truth for HOW. Litmus test: if a reader would need the source
  open to follow your prose, it is wrong — rewrite it.

  APPENDIX (everything below the marker) — the chapter's normative facts, pinned in
  tight tables and blocks with stable IDs. The ticket generator reads the chapter AND
  the appendix; a fact promised in prose with no pin here is a gate BLOCKER, and an
  orphaned pin no prose sentence explains is a finding too.

NEVER put any of these in the PROSE region (the doc-style gate rejects them):
  - code blocks / fences, or Go/TS/SQL of any kind — no interface, struct, type,
    function or method signatures, field lists, or test names;
  - file paths or filenames, migration numbers, PR numbers, or commit SHAs;
  - a "Key files" inventory table, an HTTP status-code table, or any other
    line-by-line map of the source.
DO express every prose fact as words. Keep ADR ids, P-numbers, and pin IDs as light
provenance. Name seams and concepts as VOCABULARY, not code symbols. Point to module
directories (never files) for depth. Cross-link sibling chapters by name.

PROSE MAY repeat a pinned value for readability only when tagged with its pin ID so
the gate can check the repetition for equality.

FRONT-MATTER is required:
  status: skeleton | planned   — skeleton chapters describe code in this repo (path
                                 claims gate-verified); planned chapters specify code
                                 the factory builds (ticket-coverage gate-verified).
  module:                      — the owning module directory (backend and/or frontend).
  derives-from:                — the corpus paths + section anchors this distills.

APPENDIX RULES:
  - Exactly one `## Appendix` marker per file; nothing but appendix subsections below it.
  - Fixed subsection vocabulary, only what the chapter needs, in this order:
      ### Parameters   — tunables/constants: ID, name, value, meaning
      ### Formulas     — inputs → pseudocode → output → tie-breaks → WORKED EXAMPLE
      ### Schema       — table DDL + named constraints/indexes this chapter owns
      ### Wire         — endpoints cited by contract operationId (never restated),
                         error codes, headers
      ### Events       — event IDs emitted/consumed (definitions live in the central
                         event catalog; cite, don't redefine)
      ### Limits       — rate limits, caps, quotas this chapter owns
      ### Tools        — MCP tool rows this chapter owns (scope / tier / operation)
      ### Acceptance   — stable-ID'd acceptance criteria incl. this chapter's screen
                         ACs, each with its verification shape (test lane / gate)
      ### Seed         — seed/fixture rows this chapter owns
  - Every subsection opens with a source citation line:
      Source: <corpus path>#<anchor> @ <foundation sha>
  - Tables are GFM, first column always `ID`. Fences (sql/json/yaml) are legal only here.
  - IDs: corpus IDs are preserved verbatim; new pins use <CHAPTER-SLUG>-<CLASS>-<n>.
    IDs are append-only — never reuse a retired ID.
  - Single home: every fact is pinned in exactly one chapter; elsewhere cite the
    owning ID. A sanctioned restatement carries the owner's ID tag.

Keep the section order below. Sections marked (optional) may be dropped when they
don't apply. When the chapter lands, add its one-to-two-line row to docs/overview.md —
the prose ban-list applies to those cells too.
-->
---
status: skeleton | planned
module: <owning module directory>
derives-from:
  - <corpus path>#<anchor>
---
# <Subsystem name> — <the one phrase a reader should leave with>

> A one-to-three-line summary: what this subsystem is, who calls it, and the single
> promise it makes. Written so someone who reads only this line knows whether to keep
> reading.

## What it's for

The problem this subsystem exists to solve, in plain prose — the need, not the
mechanism. Two to four sentences. Name the callers (which surfaces / other subsystems
bind to it) and draw the scope boundary.

## Principles it serves

- **P<n> — <name>.** Why this subsystem is an expression of that principle.
- **ADR-<nnnn> — <decision>.** The design decision this subsystem embodies, and what
  it buys.

## How it works

The behaviour, as prose, in the order it happens, naming the moving parts as
vocabulary rather than as code. State the *guaranteed* behaviour, not the call that
implements it. If a step is deterministic-vs-AI, degraded-vs-full, or gated-vs-free,
say so here — those distinctions are the chapter's real content.

## What's configurable

- **<Knob>** — what it changes and its default, in words, tagged with its Parameters
  pin ID. If it is a runtime surface it must appear in the runtime-config boundary.
- **<Injected dependency>** — what varies by deployment (cloud / local / fake), and
  how the system degrades when it's absent.

## Guarantees (enforced)

Each bullet is a falsifiable promise the system keeps, phrased so a reader could
imagine the test that pins it. Prefer claims a real gate or test backs — do not
over-claim.

- **<Guarantee>** — the invariant, and (in words) how it's held.

## Acceptance

What "done" means for this subsystem, in words: the observable behaviour a user or
operator can check, and the honest states (empty, degraded, denied) the surface must
render. The testable form of every claim here lives in the Acceptance appendix; the
cross-cutting floor (standard screen states, performance budgets, release gates) is
inherited from the acceptance-standards chapter and not restated.

## Out of scope (optional)

What a reader might reasonably expect here but that lives elsewhere — with a pointer.

## Where it lives

One or two lines naming the module directory (or directories) and the seam(s) callers
reach it through — directories and seams as vocabulary, never individual files. Link
the sibling chapters a reader should follow next.

## Appendix

### Parameters
Source: <corpus path>#<anchor> @ <sha>

| ID | Name | Value | Meaning |
|---|---|---|---|

### Acceptance
Source: <corpus path>#<anchor> @ <sha>

| ID | Given/When/Then | Verification |
|---|---|---|
