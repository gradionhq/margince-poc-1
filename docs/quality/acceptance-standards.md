---
derives-from:
  - margince specs/spec/narrative/06-nonfunctional.md#61-performance-budgets
  - margince specs/spec/features/03-reporting-and-scoring.md#how-to-read-this-spec
  - margince specs/spec/product/30-screen-acceptance.md#3a-standard-screen-state-matrix
  - margince specs/spec/features/01-core-objects.md#7-cross-cutting-acceptance-gates
  - margince specs/spec/features/07-ai-native-moments.md#11-cross-cutting-acceptance-gates
  - margince specs/spec/features/08-client-surfaces.md#6-cross-cutting-acceptance-gates
---
# Acceptance standards — the floor every chapter inherits

Every subsystem chapter carries its own acceptance criteria; this chapter carries the
cross-cutting floor those criteria stand on. Chapters inherit everything here without
restating it — a feature chapter says "standard states apply" and pins only its
screen-specific behaviour; the states themselves, the performance budgets, the
release-gate catalogs, and the acceptance-ID conventions are owned here.

**Performance budgets are requirements, not aspirations** (P4). The contract is p95 —
tail latency is what users feel — measured in CI against a seeded representative
dataset on fixed hardware, with results tracked over time so slow drift is caught. A
change whose benchmark regresses past budget does not merge; a budget change requires
a decision record, never a silent bump. Budgets are part of every feature's
acceptance: a feature is not done until it meets them. **Correctness is a budget
too**: a reporting number that is fast but wrong is a failure — every aggregate
surface carries a golden-number test against independently computed ground truth.

**Every screen renders its honest states.** The product's thesis is honesty under
uncertainty, so the unhappy paths are the acceptance contract, not an afterthought:
every screen ships the five standard states pinned below as real rendered states —
never as toasts — plus the named special-case states that carry the honesty thesis
(the brief that admits a slow week, the approval inbox that surfaces a bounced
downstream send, the scrape that falls back to a manual form instead of an error
wall). The screen-acceptance test suite asserts them per screen.

**Release gates are area-wide.** Three catalogs of machine-verifiable gates guard the
core-objects area, the AI-native surface, and the client surfaces respectively. A
gate failing blocks the release of its area regardless of which ticket introduced the
regression. Feature chapters cite the gates that bind them by ID.

**Acceptance IDs.** Every acceptance criterion in this spec carries a stable ID.
Inherited corpus IDs are preserved verbatim; new criteria use chapter-scoped IDs; IDs
are append-only and never reused. Screen ACs follow the per-screen series and live in
the chapter that owns the screen; every screen is owned by exactly one chapter.

## Appendix

### Parameters — performance budgets
Source: narrative/06-nonfunctional.md#61-performance-budgets @ 5a0b29c

| ID | Operation | p95 budget | Gate |
|---|---|---|---|
| PERF-1 | Record open (person/org/deal) | < 100 ms server, < 300 ms perceived | CI benchmark + RUM alert |
| PERF-2 | List/table view (50 rows, filtered) | < 150 ms server | CI benchmark |
| PERF-3 | Search (full-text) | < 200 ms | CI benchmark |
| PERF-4 | Save/mutation | < 150 ms server | CI benchmark |
| PERF-5 | AI baseline action (summary/draft) | first token < 1.5 s | RUM; model-bound, not a merge-blocker |
| PERF-6 | Cold start (single binary) | < 2 s | CI smoke |
| PERF-7 | Context-graph assembly (brief candidate set) | < 300 ms server at mid-market tier | CI benchmark — the graph-store trigger (ADR-0021) |

### Parameters — reporting & scoring budgets
Source: features/03-reporting-and-scoring.md#how-to-read-this-spec @ 5a0b29c
(new IDs; the source table is unnumbered)

| ID | Operation | p95 budget |
|---|---|---|
| PERF-R1 | Single-object report (filter + group + aggregate, ≤50 rows) | < 300 ms server |
| PERF-R2 | Cross-object report (1–2 joins) | < 500 ms server |
| PERF-R3 | Dashboard load (≤8 widgets, cached read models) | < 800 ms server, < 1.5 s perceived |
| PERF-R4 | Saved-report refresh (cache miss, full recompute) | < 1.5 s server |
| PERF-R5 | NL-report compile (utterance → validated plan) | < 1.5 s to first plan |
| PERF-R6 | Explain-this-number derivation (drill to source rows) | < 400 ms server |
| PERF-R7 | Workflow handler dispatch (event → handler start) | < 200 ms from emit |
| PERF-R8 | Lead score recompute (single record, incremental) | < 150 ms server |
| PERF-R9 | Lead score batch recompute (full workspace) | async; 100k records < 5 min |
| PERF-R10 | Routing decision (assign owner on new lead) | < 250 ms, synchronous on capture |

### Parameters — benchmark scale
Source: features/03 (benchmark dataset); narrative/06-nonfunctional.md#67-scalability @ 5a0b29c

| ID | Parameter | Value |
|---|---|---|
| AS-SCALE-1 | Reporting benchmark dataset | 1M activities / 100k people / 20k orgs / 5k deals, seeded |
| AS-SCALE-2 | Volume tier — SMB | ~10k contacts |
| AS-SCALE-3 | Volume tier — mid-market | 250k–1M contacts (budgets must hold here) |
| AS-SCALE-4 | Volume tier — enterprise | > 1M contacts |

### Acceptance — standard screen-state matrix
Source: product/30-screen-acceptance.md#3a-standard-screen-state-matrix @ 5a0b29c

Every screen in the product MUST implement, and assert in its screen-acceptance e2e
suite, these states — as real rendered states, never toasts:

| ID | State | Requirement |
|---|---|---|
| STATE-1 | empty / zero-data | Honest empty render (no fabricated counts/rows); a clear "nothing here yet / why" message. Never a blank or an unresolving spinner. |
| STATE-2 | loading | Skeleton or progressive render within the screen's perceived budget; chrome renders immediately, content streams in. |
| STATE-3 | error | Honest failure card with cause + retry; one panel's failure never blanks the screen and never blocks core CRM. |
| STATE-4 | no-permission | Denied content is absent from the response payload (not merely UI-hidden); the control is disabled or omitted, never a dead button. |
| STATE-5 | nothing-grounded | Any AI field/panel with no evidence clearing the no-guess gate is omitted or shown as "not found" with honest-degradation copy — never a fabricated value. |

Named special-case states that MUST exist (owned by the named chapters, asserted by
the same suite):

| ID | Screen | Required states |
|---|---|---|
| STATE-SP-1 | Morning Brief home | empty-queue ("nothing needs you this morning") AND honest-short-week (fewer items shown honestly, never padded) |
| STATE-SP-2 | Approval inbox | read-only viewer (controls absent without approve scope); failed-downstream-execution (approved but bounced → surfaced + re-queued); per-row partial-batch approve/reject; live 72h TTL countdown (ADR-0036) |
| STATE-SP-3 | Cold-start / company read / deep research | scrape-unreadable and robots-disallowed → manual-paste fallback, never an error wall |
| STATE-SP-4 | Overlay connect (per incumbent) | consent-denied; token-refresh-failure; revocation-failure; admin-consent blocked — each an explicit state that starts zero mirror rows |
| STATE-SP-5 | Async jobs (export, erasure, backup/DR, bulk ops) | queued / running / ready / failed, with a partial-failure path for bulk operations |

### Acceptance — release gates: core objects
Source: features/01-core-objects.md#7-cross-cutting-acceptance-gates @ 5a0b29c
(new IDs; the source list is numbered 1–8)

| ID | Gate |
|---|---|
| GATE-CORE-1 | Contract-first: every object/field is in the contract; generated types compile; the drift check passes (P3). |
| GATE-CORE-2 | Static-schema honesty: a test asserts no dynamic-schema table backs standard fields; every standard field is a real, indexed column (P1/P2/P4). |
| GATE-CORE-3 | Provenance universality: source + captured-by are non-null on every core row (P5/P12). |
| GATE-CORE-4 | Referential integrity: archive/merge leave zero orphaned references; relationships resolve via the typed relationship table, never directional associations (P11). |
| GATE-CORE-5 | Audit completeness: every core mutation produces exactly one audit entry and one domain event (P12). |
| GATE-CORE-6 | Performance budgets: the pinned p95 targets are enforced in CI against the seeded dataset; a regression blocks merge (P4). |
| GATE-CORE-7 | MCP parity: every shipped core read/write is reachable through the governed tool surface under Passport scopes, with tiers as declared (P6/P12). |
| GATE-CORE-8 | Beautiful by default: the 360 views, board, and timeline ship on design-system primitives and pass the drift check (P8). |

### Acceptance — release gates: AI-native surface
Source: features/07-ai-native-moments.md#11-cross-cutting-acceptance-gates @ 5a0b29c
(new IDs; the source list is numbered 1–9 with two items numbered 9 — a corpus
numbering defect resolved here as ten gates)

| ID | Gate |
|---|---|
| GATE-AI-1 | Evidence-or-omit (the no-guess gate): every AI-returned field/claim/signal carries a non-empty evidence snippet and a confidence, or is absent. A rendered ungrounded value is a hard failure (P12). |
| GATE-AI-2 | Accept-to-persist: before a human accept, AI proposals produce zero rows in real domain tables and write to no record field (P12). |
| GATE-AI-3 | Provenance universality: every row this surface commits carries non-null AI/connector provenance, distinguishable from human entry (P5/P12). |
| GATE-AI-4 | Human-edit precedence: no enrichment or inference overwrites a human-edited field without a recorded 🟡 approval token (P12). |
| GATE-AI-5 | Egress posture: secret-stripping runs on every externally fetched, model-bound payload (no PII pseudonymization — the location ladder is the privacy control); the sovereign profile completes cold-start, read-back, and transcript flows with zero external egress, tested (P7). |
| GATE-AI-6 | RED-removed guard: a static check asserts no live-call surface and no biometric/emotion/voice/body-language inference path exists; any sentiment output is post-call, text-only, draft, labeled. |
| GATE-AI-7 | Confirm-first invariant: no outbound action executes without a recorded human approval token, enforced at the tool-contract tier. |
| GATE-AI-8 | Graceful degradation: fetch failure and absent providers degrade to honest states or manual forms and never block onboarding or core CRM (P4). |
| GATE-AI-9 | AI transparency (EU AI Act Art. 50): every generative output renders an AI-generated / AI-assisted disclosure; the assistant entry point declares it is AI; machine-readable marking where feasible (ADR-0025). |
| GATE-AI-10 | Beautiful by default: AI surfaces ship on design-system primitives and pass the drift check (P8). |

### Acceptance — release gates: client surfaces
Source: features/08-client-surfaces.md#6-cross-cutting-acceptance-gates @ 5a0b29c
(new IDs; the source list is numbered 1–8)

| ID | Gate |
|---|---|
| GATE-CS-1 | Own-workspace-only egress (the headline P7 gate): a network-isolation test over every extension flow asserts the only network destination is the user's configured workspace — zero third-party, relay, analytics, or telemetry hosts. |
| GATE-CS-2 | Lead-not-contact invariant: profile/page capture creates leads, never person rows; captured leads stay excluded from contact surfaces until a genuine-engagement promotion (ADR-0008). |
| GATE-CS-3 | Dedupe-first: every capture surface checks existing leads and people before writing; exact match returns the existing record, ambiguity goes to a 🟡 candidate — never a silent duplicate or auto-merge. |
| GATE-CS-4 | Provenance universality: every row written by a client surface carries extension + human provenance (P5/P12). |
| GATE-CS-5 | RBAC + audit parity: every extension read obeys the human's RBAC and every write produces exactly one audit row + one event — identical to the web app (P12). |
| GATE-CS-6 | Confirm-first invariant: no outbound action executes from an extension without the same recorded approval token the web app requires — the extension is not a back door. |
| GATE-CS-7 | No-guess on clipped/read content: any field from a clipped page or read profile carries evidence + confidence or is omitted; fabricated fields are a hard failure. |
| GATE-CS-8 | Beautiful by default: client surfaces ship on design-system primitives where the host page allows, and pass the drift check (P8). |

### Acceptance — correctness invariant
Source: features/03-reporting-and-scoring.md (correctness-is-a-budget) @ 5a0b29c

| ID | Requirement |
|---|---|
| AC-X1 | Golden-number correctness: every aggregate widget/report has a golden-number test asserting the displayed number equals independently computed SQL ground truth on the seed dataset — zero tolerance. Fast-but-wrong is a failure. |

### Acceptance — ID conventions
Source: this spec's authoring convention (deliverable C) @ 5a0b29c

| ID | Rule |
|---|---|
| ACID-1 | Corpus-inherited IDs are preserved verbatim (AC-*, AIUC-*, RC-*, RL-*, CAP-*, MCP-SESS-*, PERF-*, D1–D8, AC-OV-*, S-E*, INV-*, P1–P14). |
| ACID-2 | New pins use chapter-scoped IDs: <CHAPTER-SLUG>-<CLASS>-<n>. |
| ACID-3 | IDs are append-only; a retired ID is never reused. |
| ACID-4 | Every screen's AC series is owned by exactly one chapter; the ownership partition has no orphans and no duplicates. |
