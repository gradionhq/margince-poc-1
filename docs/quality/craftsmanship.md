---
derives-from:
  - margince-poc/docs/quality/craftsmanship.md (frozen)
  - specs/spec/architecture/15-code-craftsmanship.md#2-the-anti-tell-catalog
  - specs/spec/architecture/18-code-quality-operating-model.md#1-the-three-tier-enforcement-model
---
# Code craftsmanship — the quality bar, in two layers

> The quality bar for Margince source: a **deterministic layer** (toolchain, gates, layout
> conventions) that proves code is correct, safe, and consistent, and a **taste layer** (a heuristic
> review of idiom and polish) that proves it reads as the work of a senior human engineer.
> Load-bearing for **P3** (code quality *is* a product feature) and **P8** (beautiful by default) —
> applied to the *source*, not the UI. This code is written largely by AI coding agents and then
> open-sourced for public review: it must not only work, be safe, and be consistent — it must read
> as senior-grade work (ADR-0045).

The deterministic layer's rules ship **as code**: the committed lint ruleset, the architecture-lint
DAG declaration, the file-length check, and the build targets that run them. This chapter never
restates that ruleset — the committed configuration is the normative artifact; this chapter names it
as vocabulary and describes the concerns it defends. The taste layer's machine-readable rubric ships
alongside the review tooling and stays in sync with the anti-tell catalog pinned below.

## Two layers: consistency vs. taste (the P8 split, applied to code)

P8 separates **consistency with the system** (necessary, mechanically checked) from **quality**
(distinctiveness, polish — assessed by a heuristic rubric, because a consistently mediocre UI still
passes the consistency check). The same split holds for source:

- **The deterministic layer is the consistency check for code.** Import boundaries, jurisdiction
  fitness, contract/codegen drift, formatting and vet, the lint ruleset, and the file-length cap,
  plus the test suite — they prove the code is *correct, safe, and consistent*. A consistently
  mediocre codebase passes all of them.
- **The taste layer is the quality rubric for code.** Craftsmanship — idiom, restraint, taste, the
  absence of the tells that mark machine-generated code — is a *distinct* concern, assessed by
  heuristic review (the taste gate). The two are not the same check.

**Baseline, not re-invented.** The idiomatic floor is the Google Go Style Guide + Style Decisions,
the Uber Go Style Guide, and "Go Code Review Comments" for Go; the project's React/TS conventions
(the frontend chapter) for the frontend. This standard **binds** them and adds the anti-tell layer
they predate. Where this standard and a generic style guide disagree, this standard wins.

**The one rule.** *No convention without a mechanism and an owner.* A rule that cannot be (a) failed
by the compiler, (b) failed by a deterministic merge-blocking check, or (c) caught by a heuristic
review that blocks is advice, not architecture — and advice does not survive a fleet of build
agents. Every rule in this chapter names which of the three holds it.

## The enforcement model (where a rule lives)

A rule lands in the **cheapest mechanism that can hold it** — pushed down toward the compiler, never
up toward "the reviewer will catch it."

- **The compiler** (owner: the architect). Module boundaries via internal packages, generated
  contract types, the typed error sentinels — and the **capability-as-argument seam**: the
  system-of-record mutation path requires an admitted capability as an argument, so "no admitted
  capability means no reachable mutation path" is a compiler fact, not a convention, and tier
  decisions belong to the admission gate rather than inside handlers (CRAFT-DECAY-6).
- **The lint gate** (owner: the architect plus the gate itself). Formatting, vet, the committed lint
  ruleset, the file-length cap, the dependency-DAG check, codegen drift, and the test suite: style,
  idiom, complexity, swallowed errors, SQL injection, DAG edges, and doc–code drift. The **`craft static`**
  subcommand (the `static` package of this `cli/craft` binary, ADR-0045 Am.1) is the deterministic arm of
  *this* gate for the objective anti-tells — `swallowed-errors` and `test-sleep` as BLOCKER; `boolean-trap`,
  `naked-any`, `panic-in-domain`, `assertion-free-test`, `large-file`, `long-func` as MAJOR;
  `todo-without-ref` as MINOR. It runs before the taste gate, stdlib-only, no tokens; false positives are
  waived in-source with a reason (`//craft:ignore <check> <reason>`).
- **The taste gate** (owner: the taste gate). An LLM rubric over the diff against the anti-tell
  catalog: over-abstraction, textbook naming, papered-over edge cases — the tells a linter cannot
  see.

**Sequencing:** the lint gate must be green before the taste gate is worth running — a taste review
of code that doesn't compile or leaks tenants is wasted tokens. **CI is the source of truth and the
only thing that blocks merge.** A local pre-flight hook may run a fast subset, but it never
*substitutes* for the gate — and it never gates only on agent-side triggers, because humans and
external contributors don't fire them (CRAFT-DECAY-3).

## The deterministic ruleset (cited, never restated)

The canonical ruleset is code: it ships as the committed lint configuration, it is **normative and
architect-owned**, and changing it is an ADR-traceable decision. The gate always runs the committed
configuration, never a developer's local defaults. The concerns it defends, in words:

- **Idiom.** Canonical stricter-than-default formatting, grouped imports, doc-comment and
  receiver-name discipline, idiomatic-Go diagnostics.
- **Complexity and size (the god-func guards).** Function-length, cyclomatic and cognitive
  complexity, maintainability, duplication, and nesting guards — plus the file-length cap: any Go
  production file over 500 lines (CRAFT-PARAM-1) fails the gate, generated and test files excluded.
  A file at the ~400–500-line norm (CRAFT-PARAM-2) is a god-file split candidate; the cap is
  overridable only for a measured, ADR-traceable reason — the default is the gate.
- **Correctness and safety.** No swallowed or discarded errors, result sets and response bodies
  always closed and their errors checked, no network or database call without a request context,
  security static analysis including SQL string-building, no unchecked type assertions, no masked
  failures.
- **Anti-tell-adjacent (the deterministic slice of the taste layer).** Bans on print-style output in
  favor of structured logging, on writing JSON error bodies through the plain-text helper in favor
  of the centralized writer, on repeated bare literals, speculative parameters, dead exports, and
  returned interfaces (accept interfaces, return concrete types).
- **Suppression discipline.** A bare lint suppression is itself a failure: every suppression names
  the specific rule *and* a reason, and the taste gate reviews every one (CRAFT-DECAY-5). A
  suppression is never a shortcut around a real fix — that is tell T6.

## The anti-tell catalog

Each documented "tell" of LLM-generated code is turned into a checkable rule; the meta-rule is that
*the addition is indistinguishable in style from a senior human's edit to this file*. The ten tells
(T1–T10, pinned verbatim in the appendix) cover over-commenting, defensive-programming noise,
premature abstraction, textbook naming, style drift, type escape hatches, surface polish over
substance, dead or speculative code, untrustworthy dependencies, and oversized unexplained changes.
These are the block-eligible categories the taste gate reviews every diff against, with the severity
model below deciding what actually blocks.

## Severity — why a hard block with no override is safe

The taste gate is **merge-blocking with no human override** (ADR-0045). Taste is fuzzy, so a blunt
block would wedge the pipeline on false positives. The reconciliation is a **calibrated
block-eligibility rule** (CRAFT-SEV-4): the gate may only block on findings it can state objectively
and with high confidence. A clear, objectively-statable tell blocks merge and is written into the
source as a fix marker; a plausible-but-uncertain craft issue or a nit is a non-blocking comment,
tracked for calibration. Subjective taste therefore *informs* but never *blocks* — which is what
keeps a no-override hard block honest (block precision stays ~100%, CRAFT-SEV-4) without disabling
it. The three severities and their gate behavior are pinned in the appendix.

## Declaration and file-layout conventions

These are the senior-Go layout rules the architecture chapter leaves implicit; each is held by the
lint gate except where noted as taste-gate judgement.

**File-internal order** (one concept per file): the package doc comment (on exactly one file per
package); then grouped imports — standard library, third-party, intra-repo; then package-level
constants and variables; then each **type immediately followed by its constructors and then its
methods** — never types collected at the top with behavior scattered, so a reader sees a type and
its behavior together; unexported helpers last.

**Declarations:**

- Every **exported** symbol carries a doc comment beginning with its name. No doc comment on a
  self-evident *unexported* helper — over-commenting is a taste block, not a virtue (T1).
- **Receiver names** are short, consistent across all methods of a type, and never self-referential.
- **Interfaces are defined at the consumer, not the producer**, and as small as the call site needs.
  Return concrete types, accept interfaces. The taste gate judges whether an interface is *earned* —
  no interface without a second concrete caller today (T3).
- **Constructors do all validation**; methods then assume a valid receiver and do not re-validate
  (T2, the defensive-noise block).
- **Errors** wrap with a context prefix while preserving the shared sentinel taxonomy — never a
  parallel taxonomy. Handlers return the sentinel and the *centralized* writer maps it: a
  per-handler status ladder is a taste block, and shipping a JSON body through the plain-text error
  helper is a deterministic block.

**What only the taste gate can judge** — because a consistently mediocre file passes every linter:
names that read like the domain versus textbook filler; whether the *next* line writes itself;
honest handling of the empty-list, timezone, concurrent-write, and cross-tenant edges; whether an
abstraction is earned. That is the taste layer's job, and precisely why both layers exist.

## The DAG gate — hardened

The module dependency DAG (the architecture chapter, ADR-0014) is declared once and checked by the
architecture-lint gate, which fails on a forbidden edge and is **never** skip-if-not-installed — a
keystone invariant is not optional (CRAFT-DECAY-4). The declaration is hardened three ways: **every
package must be claimed by a component**, so a new package cannot silently escape the DAG; **no
blanket vendor allowance** — each component declares exactly the external vendors it may import, so
an unscoped vendor import (an incumbent SDK in the AI or agent modules, say) fails the gate; and **a
single test exemption** replaces any per-file list, so production cross-layer edges stay fully
checked while integration tests may wire sibling modules.

## The positive rubric (what senior-grade looks like here)

The catalog says what to avoid; this says what good *is*, so the bar is reachable, not just
punitive.

- **Idiomatic to *this* codebase.** The shared error sentinels, the established service shape,
  context-first signatures, provenance-on-write, the seam-versus-internal rule, generated code left
  untouched.
- **Small, focused, legible.** Prefer many small files over a few large; the ~400–500-line norm
  (CRAFT-PARAM-2) is the smell threshold. One concept per file. A reader lands cold and the *next*
  line writes itself.
- **Tests read as specifications.** Pure-unit mapping tests favored; table-driven where it
  clarifies; names state the behavior, not the mechanics. Tests cover the honest edge cases of T7,
  not just the happy path.
- **The PR tells a story.** A title that names the change, a body that says what / why /
  how-verified, a commit history a reviewer can follow. Conventional, present-tense commit
  subjects.
- **Restraint.** The best diff is often the smallest. Deleting code, reusing an existing helper, or
  picking the one opinionated default (P1) beats adding a clever new abstraction.

## Extending the gate (architect-owned, ADR-traceable)

The ruleset is normative, so changing it is a deliberate decision, not a convenience. Each change is
architect-owned and names its reason (and an ADR when it shifts an invariant): a **new linter** that
surfaces existing findings lands the same way the original ruleset did — fix or suppress-with-reason,
never a silent disable — and **dropping** one needs a recorded rationale, because it weakens the
gate. **Raising a threshold** (function length, complexity, the file cap) is debt, not a fix —
prefer splitting the offending code. **Allowing a new vendor** scopes it to only the components that
need it; the AI and agent modules are never widened to an incumbent SDK — that edge is the thing the
scoping exists to forbid (T9). **Suppressing a finding** requires the named rule and a reason, only
where the rule is genuinely wrong for that site; the taste gate reviews every suppression.

## Anti-decay invariants (the "never again" list)

The proof-of-concept that preceded this spec had this entire standard *designed* — and the code
still drifted, because the enforcement layer was present but switched off: gates disabled in CI, the
DAG check skip-if-not-installed, the test suite running nowhere automatically. Each diff passed
taste review while the macro-invariants no single change owned decayed. The invariants pinned below
(CRAFT-DECAY-1..6) make that failure mode structurally impossible: the full deterministic check runs
and blocks, the tests run in the gate, local hooks never substitute for CI, the keystone gates
resist silent weakening, every suppression is reviewed, and the capability choke-point stays a
compile-time fact.

## The authoring loop (authoring + response to a block)

The root in-repo engineering guidance carries the craftsmanship section: the standard the agent
reads *before* writing, plus the loop contract it follows *after* a block. It is the single standard
for every module — there are no per-module guidance files, so match the surrounding code in the
module you edit.

- **Pre-submit self-check** (before opening a PR): *Would a senior engineer write it this way? Does
  it match the surrounding file? Can I justify every comment, abstraction, and dependency? Are the
  hard edge cases (T7) actually handled? Is this the smallest diff that does the job?*
- **On a block,** the taste gate writes fix markers (CRAFT-FIX) into the source. Read every marker,
  **fix the underlying code, and delete the marker.** A marker left in the tree fails the
  deterministic **residue gate** — markers are self-cleaning by construction and must never reach a
  merge.
- You **cannot override** a block. If a finding is genuinely wrong, replace its fix marker with a
  dispute marker (CRAFT-DISPUTE, also residue-blocking) carrying your reasoning — it routes that one
  finding to human adjudication, not to a merge.

The guidance is a *signpost*, not a guardrail — it teaches the standard but does not enforce it. The
teeth are the taste gate plus the deterministic residue gate. The most frequent blocker categories
are promoted *back* into the authoring rules over time, so authoring quality rises and the block
rate falls.

## Appendix

### Parameters
Source: margince-poc/docs/quality/craftsmanship.md#3.2 @ a11d6c08; architecture/18-code-quality-operating-model.md#32-complexity--size @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| CRAFT-PARAM-1 | GO_FILE_LINE_CAP | 500 | Hard cap: any Go production file over 500 lines fails the deterministic gate, excluding generated and test files. Overridable only for a measured, ADR-traceable reason — the default is the gate. |
| CRAFT-PARAM-2 | FILE_SIZE_SMELL_THRESHOLD | ~400–500 LOC | The per-file smell norm: a file in this band is a god-file split candidate; prefer splitting over relaxing the cap. |

### Acceptance — anti-tell catalog
Source: margince-poc/docs/quality/craftsmanship.md#3.6 @ a11d6c08; architecture/15-code-craftsmanship.md#2-the-anti-tell-catalog @ 5a0b29c

Each row is a documented "tell" of LLM-generated code turned into a checkable rule — the
block-eligible categories the taste gate reviews against (severity per CRAFT-SEV-1..4). The
meta-rule: *the addition is indistinguishable in style from a senior human's edit to this file.*

| ID | Tell | The rule |
|---|---|---|
| T1 | **Over-commenting** | Comments explain *why*, never *what*. No comment that restates the code (`i++ // increment i`). No docstring on a self-evident function. Match the surrounding file's comment density. |
| T2 | **Defensive-programming noise** | No redundant `nil` check or re-validation of an input already validated upstream or guaranteed by the type. Errors flow via the `margince/errs` sentinels (the architecture chapter); handlers return them, the choke-point maps them. No swallowing errors into generic logs. |
| T3 | **Over-engineering / premature abstraction** | No interface, factory, generic, or "base" type without a *second concrete caller today* (YAGNI; mirrors ADR-0042 pack-deferral). A 20-line job is a function, not a hierarchy. The Mat-Ryer service shape is the ceiling of ceremony, not the floor. |
| T4 | **Textbook uniformity** | Names read like the domain (`deal`, `passport`, `captureRun`), not CS-textbook filler (`data`, `result`, `tmp`, `helper`, `manager`, `processData`). No verbose identifiers where the package already gives context (`core.Person`, not `corePersonEntityObject`). |
| T5 | **Style drift** | Naming, formatting, error-wrapping, file structure, and test shape match the module they land in (the architecture chapter + the surrounding code). The reviewer compares every hunk against its *surrounding* code, not an abstract ideal. |
| T6 | **Type escape hatches** | No Go `interface{}`/`any` or unchecked type assertion, no TS `any`/`as`/`@ts-ignore`, no `//nolint` to dodge a real type/lint fix. Fix the type. A genuine `any` at a serialization boundary is justified *in the PR*, not silently. |
| T7 | **Surface polish over substance** | Edge cases handled honestly, not papered over: the empty list, the timezone boundary, the concurrent write (`If-Match`/`version`), the cross-tenant query (RLS). A PR that *looks* complete but drops the hard case is incomplete work. |
| T8 | **Dead / speculative code** | No commented-out blocks, no unreferenced "for future use" exports, no `TODO` without an issue ref. The diff contains only what this change needs. |
| T9 | **Untrustworthy dependencies** | No new dependency without justification in the PR; no import of a package absent from the lockfile (hallucination guard). Prefer the stdlib and the existing shared libraries. |
| T10 | **Oversized, unexplained PRs** | The change is scoped and tells a story (the positive rubric). A huge PR the author "didn't write a line of" is the canonical rejected-by-maintainers case; PR hygiene is in scope. |

### Acceptance — severity model
Source: margince-poc/docs/quality/craftsmanship.md#4 @ a11d6c08; architecture/15-code-craftsmanship.md#4-severity-model @ 5a0b29c

| ID | Severity | Examples | Gate behavior |
|---|---|---|---|
| CRAFT-SEV-1 | BLOCKER | A clear, objectively-statable anti-tell (T1–T10): a comment that restates code; an `any` cast dodging a type; a redundant nil check; a premature abstraction with one caller; a dropped T7 edge case. | **Blocks merge** — *only when* confidence is high. Emitted as an in-source `CRAFT-FIX` marker. |
| CRAFT-SEV-2 | MAJOR | Plausible craft issue the reviewer is less than certain about; a stylistic call that could go either way. | **Non-blocking** inline comment. Tracked for calibration. |
| CRAFT-SEV-3 | MINOR | Nit, subjective polish, optional improvement. | **Non-blocking** inline comment. |

CRAFT-SEV-4 — **block only on high confidence.** The taste gate is merge-blocking with no human
override (ADR-0045), and may block only on findings it can state objectively and with high
confidence. Subjective taste informs but never blocks; block precision stays ~100%. A disputed
finding routes to human adjudication via a `CRAFT-DISPUTE` marker, never to a merge.

### Acceptance — anti-decay invariants
Source: margince-poc/docs/quality/craftsmanship.md#9 @ a11d6c08; architecture/18-code-quality-operating-model.md#64-anti-decay-invariants @ 5a0b29c

| ID | Invariant |
|---|---|
| CRAFT-DECAY-1 | The deterministic gate runs the **full** check (build + lint + the test suite) as blocking CI jobs — no disabled jobs, no manual-trigger-only workflows, no skip-if-not-installed, no empty linter config. |
| CRAFT-DECAY-2 | The test suite runs in the gate. Tests that run nowhere are documentation, not a gate. |
| CRAFT-DECAY-3 | Local hooks may *pre-flight* but never *substitute* for CI, and never gate on agent-only triggers (humans and external contributors don't fire them). CI is the source of truth and the only thing that blocks merge. |
| CRAFT-DECAY-4 | The DAG, RLS conformance, and codegen-drift gates are merge-blocking and resist silent weakening — a control cannot be deleted while its check stays green. |
| CRAFT-DECAY-5 | Every lint suppression carries a specific rule id + reason and is reviewed by the taste gate. |
| CRAFT-DECAY-6 | The capability choke-point is compile-time: the system-of-record mutation seam requires an admitted capability as an argument, so "no admitted capability ⇒ no reachable mutation path" is a type-checker fact, and tier decisions live in the admission gate, never inside a handler. |
