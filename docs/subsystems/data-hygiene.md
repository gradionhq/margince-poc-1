---
status: planned
module: modules/directory-adjacent — hygiene surfaces over the people/import spines (dedupe review queue, quality-score read, rollback review); this chapter owns surfaces and their queue storage, never the spine itself · frontend/src/features/data-hygiene
derives-from:
  - specs/spec/features/10-operational-depth.md#4-data-hygiene-at-scale-promotes-d27-fuzzy-dedupe-d28-quality-scoring-d64-import-rollback @ 5a0b29c
  - specs/spec/contract/formulas-and-rules.md#1071-data-quality-score-b-e1516 @ 5a0b29c
  - specs/spec/product/epics/E15-operational-depth.md#s-e154--data-hygiene-at-scale-fuzzy-dedupe-quality-score-import-rollback @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#dedupehtml--dedupe-review--merge-queue-implements-s-e154a @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#data-qualityhtml--per-record-data-quality-implements-s-e154b @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#import-rollbackhtml--import-history--rollback-implements-s-e154c @ 5a0b29c
  - specs/spec/contract/runtime-config-surface.md#1-shipped-runtime-configuration-surfaces-normative-exhaustive @ 5a0b29c
---
# Data hygiene — the clean core stays clean: a human decides every merge, every score explains itself, every import can be undone

> The admin's cleanup surfaces over the record spine: a 🟡 review queue for fuzzy
> duplicate candidates (never auto-merged), a transparent per-record data-quality
> score with its factors on display, and one-click rollback of a committed import
> batch. Detection formulas and the merge path belong to the people chapter; the
> import engine belongs to the migration chapter — this chapter is where a human
> reviews, decides, and undoes.

## What it's for

Right after a migration — and steadily thereafter — duplicates and thin records
flood in, and the workspace's value depends on the core staying clean. This
subsystem gives the CRM admin three levers: see probable duplicates with the
evidence and decide, see which records are thin or stale and why, and roll a
regretted import back wholesale. Its callers are the capture pipeline and the
importer (both of which surface fuzzy candidates into the review queue — they feed
it, they never merge), the record 360s (which render the quality signal), and the
admin working the three screens. The boundary is deliberate: this chapter owns
*surfaces and dispositions* over data whose stores, formulas, and mutation paths
live in the people-and-organizations and import-export-migration chapters.

## Principles it serves

- **P11 — Clean relational core.** The whole subsystem exists to protect it:
  duplicates are caught before they take root, and a bad batch can be excised
  without archaeology.
- **P12 — Governance is designed in.** Every merge is a human decision on an
  evidence-bearing 🟡 item, executed through the audited, reversible merge path;
  every rollback is confirm-first and writes its own audit trail.
- **P5 — Auto-capture over manual entry.** The quality score is the signal feeding
  the "manual entry is a smell" metric and points at records worth enriching rather
  than retyping.
- **P6 — transparency of the score.** The quality number is a deterministic
  weighted rule that reconciles exactly to its visible factors — never a black box
  (the corpus's "transparent, like the scoring model" requirement).
- **P1 — Opinionated over configurable.** One matching rule set, no matching-rules
  UI — the runtime-config inventory's RC-10 records dedupe behaviour as the
  canonical *explicit non-surface*; a client needing different rules edits source.

## How it works

**Dedupe review.** Detection is not this chapter's: the two-tier person and
organization dedupe formulas are the people-and-organizations chapter's single-home
pins (PO-F-1, PO-F-2 — cited, never re-pinned). What arrives here is their fuzzy
output: a candidate pair at or above the review threshold, carrying its confidence
and the per-field evidence of what matched. The queue renders both records side by
side with that evidence — which fields agree, which collide, and the
calculator-grounded arithmetic behind the score — and a human disposes of it:
approve the merge (choosing the survivor and, field by field, the surviving
values), or mark the pair not-a-duplicate, which suppresses it from future sweeps.
An approved merge executes the people chapter's merge path — non-lossy, zero
orphaned references, loser archived with a pointer to the survivor, one reversible
audit transaction (PO-AC-M1..M6, cited). Fuzzy candidates **never** auto-merge, at
any confidence — the only automatic merge in the product is the exact unique-key
tier, and that happens upstream, not here. Medium-confidence cases are rendered
more cautiously still: no survivor is pre-staged, and the differing fields are
flagged before the human may confirm.

**Quality score.** Every core record carries a data-quality score computed by a
deterministic four-factor formula — completeness, freshness, validity, and a dedupe
factor — pinned verbatim in this chapter's Formulas appendix (DH-F-1, single home).
The score is transparent by construction: the 360 surfaces the number *and* the
per-factor breakdown (which fields are present, missing, or stale), and an
explain-this-score view shows the weighted arithmetic summing to the displayed
value. Staleness measures each field's last-*verified* event, not last-edited, so a
re-save does not reset the clock. Where the system could fill a gap it proposes,
never writes: a staged suggestion is accepted, dismissed, or edited by the human
(the accept-to-persist floor, GATE-AI-2 inherited), and a field with no grounded
evidence stays honestly blank rather than guessed.

**Import rollback.** Every imported row carries its batch in provenance (the
shared provenance convention: an import source names its batch id), and the
importer runs dry-run-first with checkpointed, idempotent jobs — that machinery,
the batch ledger, and the import side of the wire are the import-export-migration
chapter's (cited). This chapter owns the *review* surface over it: the import
history lists committed batches with their created/updated counts and a preview of
affected records; rollback is confirm-first behind a typed confirmation; rows a
human has edited since the import are kept — the action reverts only the clean
rows and says so before it runs; a rolled-back batch shows its reverted state and
audit line, and re-import is a new batch, never a restore. Rollback restores the
pre-batch state for every clean row in the batch and is itself audit-logged —
dry-run plus rollback together mean no irreversible import surprise.

## What's configurable

- **Nothing user-facing.** Matching behaviour is deliberately not configurable
  (RC-10: no matching-rules UI in V1; different rules are a source change). The
  detection tunables — review threshold, factor weights — are the people chapter's
  source-constant pins, cited here.
- **Quality-score weights** (DH-PARAM-1..5) — source constants with ratified
  defaults; configurable in code, not by users, per the runtime-config posture.
  They do not appear in the runtime-config boundary.

## Guarantees (enforced)

- **No silent merge, ever.** Fuzzy candidates enter the 🟡 queue and nothing else;
  a no-silent-auto-merge test asserts zero auto-merges in V1, at any confidence
  (the people chapter's never-auto-merge parameter, cited).
- **Merges are non-lossy and reversible.** An approved merge reuses the people
  chapter's merge path: zero orphaned references (referential-integrity test),
  loser archived with the merged-into pointer, one audit transaction, reversible
  within audit (PO-AC-M1..M6, cited).
- **The score reconciles exactly.** The displayed quality number equals the
  weighted sum of its displayed factors — a factor-decomposition test; no black
  box.
- **No guessed values.** A staged proposal writes nothing until accepted; an
  ungroundable field renders empty with honest copy, never a fabricated value
  (GATE-AI-1/2 inherited).
- **Rollback is exact and honest.** A rollback restores pre-import state for every
  clean row in the batch (import → rollback round-trip test), keeps human-edited
  rows and says how many, and writes its own audit entry.
- **No rules UI exists.** A negative-scope test asserts there is no matching-rules
  configuration surface (RC-10) — the engine is the canonical P1 non-surface.

## Acceptance

Done means: after a messy import, the admin sees flagged probable duplicates with
evidence and confirms or rejects each one; every record shows a clarity signal on
what is thin or stale and why the number is what it is; and a regretted batch
returns to its pre-import state on one confirmed click, with human edits preserved
and the whole episode in the audit trail. The three owned screens render the
standard honest states (empty, loading, error, no-permission, nothing-grounded)
inherited from the acceptance-standards chapter — the import-history screen pins
its loading/empty/error renders explicitly, including error disabling rollback
until the ledger can be read. The testable form of every claim lives in the
Acceptance appendix.

## Out of scope

- **Detection formulas, thresholds, and the merge mechanics** — the
  people-and-organizations chapter (PO-F-1/PO-F-2, the DEDUPE_* registry rows, the
  merge wire operations and PO-AC-M pins). This chapter renders and dispositions;
  it never rescores or re-pins.
- **The import engine** — dry-run, mapping, checkpoints, the batch ledger, and the
  import/rollback wire — the import-export-migration chapter; this chapter owns
  the rollback *review* surface over it.
- **High-confidence fuzzy auto-merge and quality-driven cleanup automations** —
  declared fast-follows ([TS], ratify first); not V1 behaviour.
- **A configurable matching-rules UI** — an explicit non-goal (RC-10); different
  rules are source.
- **Leads** — structurally excluded from person dedupe and quality surfaces by the
  lead segregation (ADR-0008, via the leads-and-qualification and people chapters).

## Where it lives

Directory-adjacent hygiene surfaces: transport and screens over the people spine's
dedupe output and merge path, and over the importer's batch machinery — this
chapter owns the review-queue storage when it lands (DH-GAP-1), the three screens,
and nothing of the spine. Screens ship in the web app's data-hygiene feature area
on design-system primitives. Read next: people-and-organizations for the formulas
and merge path underneath, import-export-migration for the importer this chapter
undoes, and capture for the pipeline that feeds the queue.

## Appendix

### Parameters
Source: contract/formulas-and-rules.md#1071-data-quality-score-b-e1516 + #0-parameter-registry-all-tunables-one-place @ 5a0b29c

Single home for the data-quality tunables (ratified 2026-06-26, Bucket-3).

| ID | Name | Value | Meaning |
|---|---|---|---|
| DH-PARAM-1 | `W_DQ_COMPLETE` | `0.40` | Completeness-factor weight. |
| DH-PARAM-2 | `W_DQ_FRESH` | `0.30` | Freshness-factor weight. |
| DH-PARAM-3 | `W_DQ_VALID` | `0.20` | Validity-factor weight. |
| DH-PARAM-4 | `W_DQ_DEDUPE` | `0.10` | Dedupe-factor weight. |
| DH-PARAM-5 | `DQ_FRESH_WINDOW_DAYS` | `90` | Freshness recency-decay window (days). |

Note DH-N-1 (corpus defect, recorded): formulas §10.7 states its weights are
"tunables registered in §0", but the §0 parameter-registry table contains **no**
`W_DQ_*` / `DQ_FRESH_WINDOW_DAYS` rows — the values above exist only in §10.7.1.
This chapter is their single home; the corpus fix is to add the five rows to §0.
All five are source constants — no runtime tuning UI (P1); not in the
runtime-config boundary.

Cited, not owned (single home: people-and-organizations Parameters):
`DEDUPE_FUZZY_AUTOMERGE` *(never)*, `DEDUPE_REVIEW_THRESHOLD` `0.72`,
`DEDUPE_NAME_WEIGHT` `0.55`, `DEDUPE_ORGDOMAIN_WEIGHT` `0.45` — sanctioned
restatement carrying the owner's registry.

### Formulas
Source: contract/formulas-and-rules.md#1071-data-quality-score-b-e1516 @ 5a0b29c

**DH-F-1 — Data-quality score.** Single home. Corpus formula, verbatim:

> `quality = W_DQ_COMPLETE·completeness + W_DQ_FRESH·freshness + W_DQ_VALID·validity + W_DQ_DEDUPE·dedupe`, each factor normalized 0–1.
> - **completeness** = share of expected fields populated; **freshness** = recency decay over `DQ_FRESH_WINDOW_DAYS` (default 90); **validity** = share of fields passing format/range checks; **dedupe** = `1 − duplicate_likelihood`.
> - **Tunables:** `W_DQ_COMPLETE=0.40 / W_DQ_FRESH=0.30 / W_DQ_VALID=0.20 / W_DQ_DEDUPE=0.10`, `DQ_FRESH_WINDOW_DAYS=90`.

Inputs: the record's expected-field set (fixed, per object type), each field's
last-verified event (freshness uses last-*verified*, not last-edited —
AC-data-quality-8), the format/range checks, and the record's duplicate likelihood.
Output: `quality ∈ [0,1]`, surfaced as a 0–100 number with its per-factor breakdown
(weights sum to 1.0). Tie-breaks: none — pure arithmetic; deterministic given a
fixed clock.

Worked example (**derived** — illustrative factor values, not corpus rows): a
person record with 6 of 8 expected fields populated (`completeness = 0.75`),
last verified about half the freshness window ago (`freshness = 0.50`), all
populated fields passing checks (`validity = 1.00`), and no open duplicate
candidate (`dedupe = 1.00`):
`quality = 0.40·0.75 + 0.30·0.50 + 0.20·1.00 + 0.10·1.00 = 0.30 + 0.15 + 0.20 + 0.10 = 0.75`
→ displayed **75/100**, with the four factor lines summing visibly to 75.

Note DH-N-2 (factor internals are build items): the corpus pins the four factors,
their weights, and the window — it does not pin the per-object expected-field
sets, the freshness decay curve shape, the format/range check list, or the mapping
from the people chapter's fuzzy confidence to `duplicate_likelihood`. Those land
with the score ticket (B-E15.16); whatever they resolve to, the
reconciles-exactly guarantee binds them (the score must equal the weighted sum of
its displayed factors).

Note DH-N-3 (contested — prototype vs ratified formula): the data-quality mockup's
arithmetic (completeness 58/70 + freshness 12/20 + validity 10/10 = 70/100; three
factors, implied weights 0.70/0.20/0.10, no dedupe factor) contradicts DH-F-1's
ratified four-factor 0.40/0.30/0.20/0.10. The formula is normative — the prototype
predates the 2026-06-26 ratification; build the factor panel to DH-F-1 and treat
the screen ACs' specific numbers as layout/behaviour pins, not arithmetic pins.

Cited, not owned: **PO-F-1** (person dedupe, two tiers) and **PO-F-2** (org
dedupe) — the people-and-organizations chapter's single-home pins produce this
chapter's queue items (decision `FUZZY_REVIEW(confidence)` with the matched
record); the review queue renders their output and never re-pins the formulas.

### Schema
Source: contract/data-model.md §2–§12.6 (all CREATE TABLEs — absence verified) + the data-model chapter's ownership index @ 5a0b29c

This chapter owns no corpus table today — honestly: the corpus has a known gap
here.

**DH-GAP-1 (D-H2 schema-extension item — the dedupe review queue has no storage).**
PO-F-1/PO-F-2's fuzzy output is defined as "route to a 🟡 review queue", but no
table in the corpus schema stores a queue item, and the ownership index partitions
all 66 corpus tables with none for dedupe review. A candidate-pair table must be
added contract-first, owned by this chapter when it lands, carrying at least: the
pair (entity type + both record ids), the confidence, the per-field match evidence
(what the queue renders — AC-dedupe-2/3), disposition state (open / merged /
not-a-duplicate), and the not-a-duplicate suppression that keeps a dismissed pair
out of future sweeps (AC-dedupe-7), under the shared tenancy/provenance/audit
conventions (DM-CONV-*, cited). Whether it is a new table or a specialization of
the approval-inbox item store is the ticket's decision; the queue semantics pinned
in this chapter bind either shape.

**DH-GAP-2 (dependency recorded — the import batch ledger is not this chapter's).**
Rollback needs the batch tracked as a unit (id, file, counts, status,
touched-since-import detection). No corpus table exists for it either; its home is
the import-export-migration chapter (the import side of rollback), cited here so
the review surface's dependency is explicit. Row-level batch membership is already
recoverable from the shared provenance convention (an imported row's source names
its batch id — DM-CONV-11, cited).

**DH-GAP-3 (note — the quality score has no storage pinned).** No column or table
stores the score; it is deterministic from existing data (DH-F-1) and may be
computed at read or cached in a read model. If a persisted column proves necessary
it arrives as a schema-extension ticket; nothing is pinned here.

### Wire
Source: contract/crm.yaml @ 5a0b29c — absence verified; merge operations cited to their owner.

Cited, not owned: `mergePerson` and `mergeOrganization` — the
people-and-organizations chapter's wire rows (🟡, approval-token bound; its
PO-EXT-8 extension adds the missing token parameter). The queue's approve action
executes them; this chapter adds no second merge verb.

**Wire — contract extensions (D-H2).** No dedupe-queue, quality-score, or
import-batch operation exists in crm.yaml today; contract-first extension tickets:

| ID | Extension | Notes |
|---|---|---|
| DH-EXT-1 | Dedupe review-queue reads | List open/resolved candidate pairs with confidence + per-field evidence; read a single case (AC-dedupe-1/2). |
| DH-EXT-2 | Queue disposition operations | Approve-merge (binds survivor + per-field survivor values, then executes the owner's merge verb), not-a-duplicate (with sweep suppression), and undo of either (AC-dedupe-4..8). |
| DH-EXT-3 | Quality-score block on record reads | Score + per-factor breakdown + per-aspect staleness in the 360 read payload — score-only is insufficient (B-E15.16; AC-data-quality-1/3/8). |
| DH-EXT-4 | Explain-this-score derivation read | The calculator-grounded weighted arithmetic behind the displayed number (AC-data-quality-2; the same explain pattern as the scoring surfaces). |
| DH-EXT-5 | Staged-proposal disposition on quality gaps | Accept / dismiss / edit of a staged fill-the-gap proposal, accept-to-persist with human provenance (AC-data-quality-4..6; GATE-AI-2 floor). |
| DH-EXT-6 | Import-batch list/detail + rollback operation | Owned by [[import-export-migration]] (the build ticket names import-batch endpoints); recorded here as the review surface's requirement — this chapter consumes, it does not define the import wire. |

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c — definitions live in the central catalog ([[event-bus#events--catalog]]); cited, never redefined.

| ID | Direction | Note |
|---|---|---|
| `person.created` / `person.updated` | consumed | Candidate-detection sweep triggers (detection itself per PO-F-1, cited). |
| `organization.created` / `organization.updated` | consumed | Same, for org candidates (PO-F-2, cited). |
| `person.merged` / `organization.merged` | consumed (and caused) | Emitted by the people module when a queue approval executes the merge; consumed here to resolve the queue item. Owned by people-and-organizations. |

Note DH-GAP-4 (D-H2 event-catalog extension item): the catalog defines **no**
candidate-detected, candidate-resolved, or import-rolled-back events. The audit
floor ([[acceptance-standards#GATE-CORE-5]]: one audit entry and one domain event
per core mutation) implies at least the rollback needs a domain event; which
events the queue and rollback emit is an event-catalog extension to resolve with
their tickets. The audit-entry half is already unconditional (rollback is
audit-logged per DH-AC-3).

### Acceptance
Source: product/epics/E15-operational-depth.md#s-e154--data-hygiene-at-scale-fuzzy-dedupe-quality-score-import-rollback @ 5a0b29c

**Owned story atoms** (condensed per-atom; S-E15.4 is V1-Must).

| ID | Given/When/Then (condensed) | Verification |
|---|---|---|
| S-E15.4a | Given beyond-exact-email match detection, when candidates surface, then they land in a 🟡 review queue with evidence; an approved merge reuses the non-lossy, reversible, audited merge path; no silent auto-merge in V1. | No-silent-auto-merge + negative-scope (no rules UI, RC-10) tests ([[testing#TEST-LANE-2]]); referential-integrity test over the owner's merge path (PO-AC-M1..M6 cited) (B-E15.15). |
| S-E15.4b | Given any record, when opened, then a transparent completeness/staleness signal shows the factors behind the score. | Factor-decomposition (score reconciles exactly), fixed-clock staleness, and 360-exposure tests ([[testing#TEST-LANE-1]]/[[testing#TEST-LANE-2]]) (B-E15.16). |
| S-E15.4c | Given a committed import batch, when rolled back, then it returns to pre-import state, audited (inverse of dry-run→commit). | Import → rollback round-trip + audit-entry test ([[testing#TEST-LANE-2]]); importer machinery per [[import-export-migration]] (B-E15.17). |

Source: features/10-operational-depth.md#4-data-hygiene-at-scale-promotes-d27-fuzzy-dedupe-d28-quality-scoring-d64-import-rollback @ 5a0b29c

**Feature-doc acceptance criteria** (verbatim). Cross-cutting floors (STATE-1..5,
release gates) are inherited from [[acceptance-standards]] and not repeated per row.

| ID | Acceptance criterion (verbatim) | Verification |
|---|---|---|
| DH-AC-1 | Fuzzy candidates never auto-merge in v1: they enter a 🟡 queue; merge reuses the `features/01 §1.3` non-lossy merge path (zero orphaned FKs) — referential-integrity test. | Integration ([[testing#TEST-LANE-2]]) over PO-F-1/PO-F-2 output + the owner's merge path. |
| DH-AC-2 | The data-quality score is computed from presence/recency of a fixed field set (transparent, like the scoring model) and exposed with its factors on the record. | DH-F-1 golden test (fixed seed + fixed clock → stable value), unit ([[testing#TEST-LANE-1]]); factor exposure via integration ([[testing#TEST-LANE-2]]). |
| DH-AC-3 | Import rollback restores pre-batch state for every row in the batch and is itself audit-logged (P12); dry-run + rollback together mean no irreversible import surprise. | Round-trip integration test ([[testing#TEST-LANE-2]]); dry-run side owned by [[import-export-migration]]. |
| DH-AC-4 | **User-observable (Mor, S-E15.4):** after a messy import, Mor sees flagged probable duplicates to confirm, a clarity signal on which records are thin, and can roll the whole batch back if it was wrong. | Live-stack lane ([[testing#TEST-LANE-3]]) across the three owned screens. |

Source: product/30-screen-acceptance.md#dedupehtml--dedupe-review--merge-queue-implements-s-e154a @ 5a0b29c

**Owned screen ACs — dedupe review & merge queue** (verbatim; screen owned here per
ACID-4). All rows verify in the screen-acceptance e2e suite ([[testing#TEST-LANE-3]]);
STATE-1..5 apply on top.

| ID | Given/When/Then (verbatim) |
|---|---|
| AC-dedupe-1 | Given the screen loads, When it boots, Then the left queue shows the Open tab active with a count (4) listing candidate pairs (Anna Weber, BÄR Pharma, Lukas Brandt, M. Schulz), each labelled with object type (person/company), a "why" reason line, and a match score badge styled high/med/low (0.91, 0.88, 0.64, 0.41), and the first case (Anna Weber) opens in the right canvas automatically. |
| AC-dedupe-2 | Given a high-confidence case is selected, When the review canvas renders, Then it shows a match-signal evidence strip (per-field name/employer/phone/email comparisons with ≈ or ✕ and a confidence dot + label such as "exact" / "different domain"), two candidate columns A and B with creation/provenance metadata, and a 🟡 "review-first" match pill. |
| AC-dedupe-3 | Given a case is open, When the user clicks "Explain this match score", Then a calculator-grounded explanation box toggles open showing the weighted formula, per-field weights, the arithmetic summing to the displayed score, and the threshold interpretation. |
| AC-dedupe-4 | Given a field row with two differing values, When the user clicks a candidate cell, Then that cell becomes the chosen survivor value (marked with an accent dot) and the other cell in the row deselects; multi-value fields (e.g. Email for Anna) render as a non-selectable dashed "union" cell stating both are kept with a named primary. |
| AC-dedupe-5 | Given the user clicks "Keep A as primary" or "Keep B as primary", When the action fires, Then a toast confirms which record was set as the primary survivor. |
| AC-dedupe-6 | Given a high-confidence case, When the canvas renders, Then a "Resulting record (preview)" panel lists the post-merge outcome (0 orphaned records, activities/deals relink, loser archived with a merged_into pointer, reversibility) and a 🟡 approval-gate notice states the merge runs as one audited, reversible, non-lossy transaction. |
| AC-dedupe-7 | Given the user clicks "Approve merge", When the merge commits, Then the canvas shows a success state ("Merge approved & committed") with View-audit and Undo-merge actions, the resolved item is removed from the Open list, and the open count decrements; clicking "Not a duplicate" instead shows a "Marked as not a duplicate" state with an Undo action and suppresses the pair from future sweeps. |
| AC-dedupe-8 | Given the Lukas Brandt medium-confidence case (shared phone but differing employer), When it opens, Then no survivor is pre-staged: instead of a merge preview a red guard states it is "not pre-staged for merge", the primary action reads "Confirm same & merge", and a per-field warning flags the differing employer. |

Note DH-N-4: AC-dedupe-1's evidence strip includes a phone comparison and its badges
include scores below the 0.72 review threshold (0.64, 0.41) — PO-F-1's pinned
confidence uses name + org/domain only and sub-threshold candidates are ignored,
never queued. Prototype drift against the ratified formula: the queue admits only
at-or-above-threshold candidates, and the evidence strip renders whatever fields the
owner's formula actually compares. The formula pins are normative.

Source: product/30-screen-acceptance.md#data-qualityhtml--per-record-data-quality-implements-s-e154b @ 5a0b29c

**Owned screen ACs — per-record data quality** (verbatim; see note DH-N-3 on the
factor arithmetic).

| ID | Given/When/Then (verbatim) |
|---|---|
| AC-data-quality-1 | Given a scored record, When the screen loads in the "Scored" state, Then a numeric gauge shows the score out of 100 (e.g. 70), a band label ("Needs attention"), a "6 of 8 factors complete" summary, and a "last recomputed today · 06:12" timestamp. |
| AC-data-quality-2 | Given the gauge is shown, When the user clicks "Explain this score", Then a panel expands showing the weighted line items (completeness 58/70, freshness 12/20, validity 10/10, total 70/100) and states the score is a deterministic rule (Σ factor_weight × factor_state), not a model, with weights living in runtime config. |
| AC-data-quality-3 | Given the factor breakdown panel, When the user reviews it, Then each of the factors renders an icon-state (ok/warn/bad), a name, a weighted score (e.g. 14/14, 4/12, 0/12), a fill bar, and per-factor detail listing the fields present (have) or missing (miss). |
| AC-data-quality-4 | Given the "Decision-maker identified" factor is missing (0/12) with a staged AI proposal, When the user clicks Accept, Then the proposal row is removed, the factor icon turns green to 12/12 with a full bar, the field is shown as "applied, audited" with human provenance, the gauge recomputes 70 → 82, the summary updates to "7 of 8", and a toast confirms "score recomputed 70 → 82, audited". |
| AC-data-quality-5 | Given the staged decision-maker proposal, When the user clicks Dismiss, Then the proposal row is removed, the field stays empty, the score is unchanged, and a toast confirms "Proposal dismissed — field stays empty, score unchanged". |
| AC-data-quality-6 | Given the staged decision-maker proposal, When the user clicks Edit, Then a toast explains the field converts to a contact the user picks and provenance flips to typed-by-you. |
| AC-data-quality-7 | Given the "Industry / NACE code" factor, When no captured evidence grounds a value, Then the field is left blank (0/8, "Not set") with an explicit note that the system won't guess because a fabricated code would be a defect — the user can set it themselves. |
| AC-data-quality-8 | Given the Staleness panel, When the user reads it, Then per-aspect staleness is shown with a colored pip and an age ("18 days ago", "62 days ago", "never"), with a note that staleness uses each field's last-verified event, not last-edited, so a re-save doesn't reset the clock. |

Note DH-N-5: AC-data-quality-2's "weights living in runtime config" is prototype
copy that contradicts the parameter posture (DH-PARAM-1..5 are source constants,
not in the runtime-config boundary — DH-N-1); the copy should say the weights are
named source constants. The deterministic-rule and reconcile-exactly behaviour it
pins stands.

Source: product/30-screen-acceptance.md#import-rollbackhtml--import-history--rollback-implements-s-e154c @ 5a0b29c

**Owned screen ACs — import history & rollback** (verbatim; the import side under
it is [[import-export-migration]]'s).

| ID | Given/When/Then (verbatim) |
|---|---|
| AC-import-rollback-1 | Given the History view is showing committed batches, When the user clicks a batch header (e.g. `contacts_hubspot_export.csv`), Then the batch body expands to show a created / updated-existing / touched-since count grid and a preview table of affected records with per-row operation badges (created / field updated). |
| AC-import-rollback-2 | Given an expanded clean batch with 0 records touched since import, When the user clicks "Roll back this import", Then a confirm-first modal opens restating the batch id, file name, and record count, and the Confirm button stays disabled until the user types `ROLLBACK` into the typed-confirmation field. |
| AC-import-rollback-3 | Given the rollback confirm modal with a valid `ROLLBACK` entry, When the user clicks "Confirm rollback", Then the batch status flips to "Rolled back", its rollback button becomes a disabled "Reverted just now", a new audit line is prepended to the Rollback audit trail, and a toast reports the reverted count and the audit id (e.g. AUD-2026-3402). |
| AC-import-rollback-4 | Given a batch where 3 records were edited by a human since import (`deals_q2_pipeline.csv`), When the user expands it, Then a warning surfaces that those 3 edits will be kept and only the 70 clean rows reverted, the action button reads "Roll back 70 clean rows", and the confirm modal shows a separate "Human-edited · kept" count row. |
| AC-import-rollback-5 | Given an expanded batch, When the user clicks the "Explain the N affected records" / "Explain" control, Then an inline derivation box appears showing affected = created + updated, the exclusion of human-edited rows, and the resulting reverted count. |
| AC-import-rollback-6 | Given a batch already rolled back (`leads_tradeshow_scan.csv`), When the user expands it, Then it shows a "Rolled back" status, reverted count, a read-only audit confirmation, no active rollback button, and a note that re-import is a new batch rather than a restore. |
| AC-import-rollback-7 | Given the rollback confirm modal is open, When the user presses Escape or clicks Cancel, Then the modal closes and no rollback is performed. |
| AC-import-rollback-8 | Given the prototype demo state switch, When the user selects Loading / Empty / Error, Then the main column shows skeletons / a "No imports yet" empty state / a "Couldn't load import history" error that explicitly disables rollback until the ledger can be read, with a Retry control on error. |

Note DH-N-6 (scope reconcile with the feature AC): DH-AC-3 says rollback "restores
pre-batch state for every row in the batch"; the screen (AC-import-rollback-4/5)
pins the keep-human-edits refinement — clean rows revert, human-touched rows are
kept and counted honestly. The screen behaviour is the buildable resolution of the
feature sentence (a human edit outranking an automated revert is the
human-precedence posture, GATE-AI-4's spirit); the exact conflict semantics pin
with the rollback ticket on the importer side ([[import-export-migration]]).
