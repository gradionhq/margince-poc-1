---
status: skeleton
module: backend/internal/modules/agents
derives-from:
  - margince-poc/docs/subsystems/intent-tools.md @ a11d6c08
  - margince specs/spec/contract/interfaces.md#22-intent-level-tools-verbs-not-tables--03c-41-decisions-a18-adr-0009 @ 5a0b29c
  - margince specs/spec/narrative/03c-agentic-concept.md#43-tool-surface-verbs-over-tables-plus-ui @ 5a0b29c
---
# Intent tools — bundles that can never do more than their parts

> Higher-level tools that bundle several low-level tools to do one thing a user actually
> wants — and that, by construction, can never read more, change more, or skip an
> approval that their parts wouldn't allow on their own.

## What it's for

The low-level tools an agent can call — search, read, create, update, log an activity,
run a report — are deliberately small and single-purpose. Real work means stringing
several of them together: "catch me up on this account", "show me what's slipping this
week", "prep me for this meeting". An intent tool is that bundle, given a name and
offered as one tool. The whole reason this layer exists is to make that convenience
safe: bundling several tools must never quietly let an agent do more than the tools
would let it do separately. The skeleton ships the **mechanism** — composition plus the
authority-narrowing enforcement; the concrete tool set an agent sees is contract-defined
and owned by the byo-agent-and-mcp chapter, which consumes this mechanism.

## Principles it serves

- **P6 — embrace the LLMs, via governed tools.** The agent does its work through one
  governed seam, whether it calls a small tool or a bundle (the agent-first surface —
  ADR-0009).

Provenance: ADR-0009 (the agent-first surface), ADR-0007 (the context graph is the v1
data layer), ADR-0026 (one governed surface with approval tiers).

## How it works

An intent tool names the set of low-level tools it draws on, and two things about it are
worked out from that set rather than set by hand. Both are the heart of the design.

First, **what it is allowed to touch is the combination of what its parts are allowed to
touch** — nothing more. If one part can read records and another can update a deal, the
bundle can do both, but it can never reach beyond that combined reach, and it can never
touch rows outside the narrowed row scope its parts were admitted with. The rule that
any given call stays within that combination is checked and held by tests
(INTENT-AC-1, INTENT-AC-3).

Second, **how much human oversight it needs is the strictest of what its parts need.**
If any part is 🟡 (ask-first, ADR-0026), the whole bundle is 🟡; a bundle can only come
out 🟢 (auto-approved) when every one of its parts would be. When an approval-gated
bundle is called without a valid approval, it is refused, and the step that would have
made a change is never run — there are no side effects from a refused call. The approval
check is the same admission gate a single low-level call goes through
([[threat-model#TM-CTRL-3]]), reused exactly rather than re-created (INTENT-AC-2).

A read-only intent tool returns an assembled picture rather than raw rows: a set of
elements, each carrying where it came from and how much it can be trusted, wrapped in
the same trust envelope the low-level tools use. Trust travels with the data — a result
that came in untrusted stays untrusted all the way through the bundle and keeps its
"treat this as data, not as instructions" warning. Bundling never quietly upgrades
untrusted material to trusted; that never-launder rule is owned and pinned by the
trust-propagation chapter ([[trust-propagation#TRUST-AC-2]]).

The shipped tools split into two families — read-and-assemble tools that are 🟢 and
read-only, and read-plus-change tools that pair a read with one mutating step and take
their tier from that step. Every tool in both families derives its reach and its tier
from a single declared set of parts, so a tool's declared shape and its actual behavior
can't drift apart, and every element a read tool returns names its source — there are no
unsourced elements. The concrete set of six intent tools, their part lists, and their
per-tool acceptance rows are contract-defined and pinned in the byo-agent-and-mcp
chapter; this chapter deliberately does not restate that table. Each tool is registered
alongside the low-level tools so the full set is always present, held by tests.

The layer draws only on the agent-tool, datasource, and error seams plus its own code;
it never reaches back into core internals. Context reads come in through a local read
port filled by an adapter, so dependencies always point the legal direction (ADR-0014).

## What's configurable

Nothing. An intent tool's reach and its approval tier are entirely worked out from the
set of parts it declares — by design, there is nothing to tune.

## Guarantees (enforced)

- **Reach never widens** — a bundle's reach stays within the combination of its parts'
  reach, and its row scope within the admitted scope of its parts (INTENT-AC-1,
  INTENT-AC-3).
- **Oversight floor preserved** — if any part is 🟡, the whole bundle is 🟡
  (INTENT-AC-2).
- **Trust never laundered** — an untrusted part's result stays untrusted through the
  bundle ([[trust-propagation#TRUST-AC-2]]).
- **No approval back door** — an approval-gated change made without a valid approval is
  refused, with no side effects, through the same gate a single call uses
  (INTENT-AC-2).
- **Stays in its lane** — the layer never reaches back into core internals.

## Acceptance

Done means an operator can compose a new intent tool by declaring its parts and get its
reach and its approval tier for free — never hand-set, never wrong. Calling an
approval-gated bundle without approval yields a clean refusal and an unchanged system;
calling a read bundle yields an assembled, fully sourced picture whose untrusted
elements arrive labelled. The testable form of each claim is pinned in the Acceptance
appendix; the cross-cutting floor is inherited from the acceptance-standards chapter.

## Out of scope

The concrete intent-tool catalogue — the six contract-defined tools, their part sets,
and their per-tool acceptance — is owned by the byo-agent-and-mcp chapter. The surfaces
that *consume* bundles (the Morning Brief, the forecast view), the scheduler that
triggers proactive work, and the transport servers that expose bundles to an agent over
a connection are separate chapters. This layer is pure composition over tools that
already exist: no data migration, no contract change, and no UI of its own.

## Where it lives

The intent-tools layer lives in the agents module (backend/internal/modules/agents),
behind the governed tool seam. Read next: byo-agent-and-mcp for the tool set itself,
trust-propagation for the label the bundles carry, and the threat-model chapter for the
admission gate the compositions pass through.

## Appendix

### Acceptance
Source: margince-poc/docs/subsystems/intent-tools.md#guarantees-enforced @ a11d6c08; margince specs/spec/contract/interfaces.md#22-intent-level-tools-verbs-not-tables--03c-41-decisions-a18-adr-0009 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| INTENT-AC-1 | Given an intent tool with a declared set of parts, when any call executes, then everything it reads or changes lies within the union of what its parts could read or change — composed authority ⊆ union of parts, never hand-widened. | Per-tool composition property test in the agents lane; declared shape vs. actual behavior drift turns it red. |
| INTENT-AC-2 | Given a composition containing at least one 🟡 part, when its tier is derived, then the composition is 🟡; when called without a valid approval it is refused through the same admission gate a single call uses ([[threat-model#TM-CTRL-3]]), and the mutating part never runs — no side effects. | Tier-derivation unit test + refused-call no-side-effect integration test. |
| INTENT-AC-3 | Given the narrowed row scope admitted for a call, when the composition's parts execute, then no part touches rows outside that scope — no composition widens row scope beyond what its parts were admitted with. | Row-scope property test against the admitted capability. |
