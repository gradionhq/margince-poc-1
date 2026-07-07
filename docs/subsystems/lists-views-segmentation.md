---
status: planned
module: backend/internal/modules/lists (filter engine + lists/tags/views transport) · frontend/src/features/lists
derives-from:
  - specs/spec/features/10-operational-depth.md#3-segmentation-lists--views-promotes-d52--d53--d54--d66 @ 5a0b29c
  - specs/spec/contract/data-model.md#10-lists-segments-tags-attachments @ 5a0b29c
  - specs/spec/contract/data-model.md#saved-views-quota-field-mask @ 5a0b29c
  - specs/spec/product/epics/E15-operational-depth.md#s-e153--dynamic-lists-advanced-filtering-saved-views--filtered-export @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#filters-and-viewshtml--advanced-filters-saved-views--export-implements-s-e153a-s-e153b-s-e153c @ 5a0b29c
  - specs/spec/contract/crm.yaml (Lists + Tags paths) @ 5a0b29c
---
# Lists, views & segmentation — one filter engine behind every slice of the data

> The segmentation subsystem: dynamic and static lists, the nested AND/OR filter
> builder, per-user saved views, and filtered export — all driven by ONE canonical
> filter representation compiled to bounded, indexable queries over real columns.
> Build a filter once and it is a list, a view, and an export slice; it never forks
> into per-surface variants.

## What it's for

Every CRM user needs to carve the workspace into working sets: "German manufacturers
we haven't touched in thirty days", "my open deals over fifty thousand", "contacts
tagged for the trade fair". This subsystem is where those slices are defined, saved,
kept current, and taken out the door. Its callers are every object list surface in
the web app, the automation subsystem (whose add-to-list action writes static
membership through this surface), natural-language search (which compiles into this
subsystem's filters rather than replacing them), and the export path. The scope
boundary: this chapter owns the filter representation, lists, tags, and saved views;
the export *mechanism* (async job, formats on the wire, the leave-in-an-afternoon
promise) is the import-export-migration chapter's, and cross-object join queries are
the reporting subsystem's, not a filter-builder feature.

## Principles it serves

- **P1 — Opinionated over configurable.** One filter engine, one canonical
  representation, one closed operator set — not per-surface filter dialects and not
  a user-defined query language. Saved views are the sanctioned P1 *exemption*:
  per-user view state is data the user owns about their own view, with no shared
  blast radius (the runtime-config inventory records this explicitly).
- **P4 — Blazing fast, always.** Filters compile to bounded, indexable predicates
  drawn from a closed field vocabulary; a filtered list is never a full-table scan,
  and the standard list-view latency budget applies (PERF-2, inherited).
- **P11 — Clean relational core.** Filters run over real columns — including
  runtime custom fields, which are real columns by construction — never over a
  metadata interpreter, so a segment's membership is honest, explainable SQL truth.

## How it works

The moving part everything else leans on is the **canonical filter
representation**: a tree of nested AND/OR groups whose leaves are typed predicates —
a field from the closed per-resource vocabulary (DM-VOCAB, plus custom fields), an
operator from the closed operator set (LVS-PARAM-1), and a value typed to the field.
The engine validates a tree against the vocabulary (an out-of-vocabulary field is a
validation error, per the data-model chapter's vocabulary rules), then compiles it to
bounded, indexable SQL. That one representation is consumed identically by lists,
saved views, and filtered export — the one-engine guarantee — so a filter built once
behaves the same everywhere it is reused.

**Lists** come in two kinds. A *static* list is an explicit membership set, curated
by hand or written by the automation subsystem's add-to-list action; membership rows
are unique per entity and structurally typed, so a lead list can never hold a
contact (the lead-segregation requirement, ADR-0008). A *dynamic* list stores a
validated filter definition — a saved query, not a frozen row set: per the corpus,
the stored truth of a dynamic list is its query-plan definition, and the curated
members surface applies to static lists only. Dynamic membership is derived from the
definition and **re-evaluates on the relevant created/updated domain events** —
eventual, with bounded latency — so a record that starts matching enters the list
without anyone refreshing, and one that stops matching drops out. Whether an
implementation materializes that derived membership as a cache is deliberately not
pinned (LVS-GAP-1); the stored-query semantics and the event-driven freshness
promise hold either way.

**Tags** are the lightweight cross-cutting label: workspace-scoped, name-unique
case-insensitively, applied to people, organizations, deals, and leads through a
polymorphic join that follows the shared archive-cascade and canonical entity-type
conventions (data-model chapter). Tag *semantics* — what a tag is, where it applies,
and how it participates in the filter vocabulary — are owned here; the
people-and-organizations chapter's vocabulary reconcile note (PO-N-VOCAB) explicitly
cedes the tag vocabulary to this chapter.

**Saved views** capture per-object, per-user table state — column choice, sort, and
the active filter — and restore it exactly on return. A view's query speaks the same
canonical representation and the same closed vocabulary; view state is private to
its owner in V1 (LVS-PARAM-3), with shared and team views a schema-forward
fast-follow, not shipped behaviour.

**Natural-language search is a fast path in, not a bypass.** An utterance compiles
into a structured filter in the canonical representation that the user can inspect,
edit, and save as a dynamic list — never an opaque, unsaveable result set. A
compiled filter, once saved, is indistinguishable from a hand-built one.

**Filtered export rides the same engine.** Exporting a list or view emits exactly
the filtered slice — the matching rows and the visible columns, never a silent
full-table dump — in open formats (LVS-PARAM-2), and every export is audit-logged.
The export mechanism itself (the async job, its states, the wire operation) is the
import-export-migration chapter's; this chapter contributes the filter
representation the export honors and owns the screen where the slice is chosen.

## What's configurable

- **Nothing, at runtime, about the engine.** The operator set (LVS-PARAM-1), the
  field vocabularies (DM-VOCAB, cited), and the compile rules are closed source-level
  vocabulary — the P1 posture; there is no query DSL and no user-defined operator.
- **Per-user view state** — saved views, column choices, sort, active filters — is
  the deliberate P1 exemption (runtime-config inventory, out-of-scope section): user-
  owned view data, no shared blast radius. Sharing scope is pinned to private in V1
  (LVS-PARAM-3).
- **Export formats** — CSV and JSON in the first cut (LVS-PARAM-2); the format
  choice is a per-export selection, not workspace config.

## Guarantees (enforced)

- **One engine, provably.** The same filter representation drives lists, saved
  views, and filtered export through one parser/evaluator — asserted by a
  shared-engine test, not a convention (a compiled-from-NL filter is equivalence-
  tested against a hand-built one).
- **Closed vocabulary.** A filter or sort referencing a field outside the allowed
  vocabulary is rejected as a validation error (DM-VOCAB-ERR-1, cited); custom
  fields join the vocabulary as real columns, not as an escape hatch.
- **Dynamic means current.** A dynamic list re-evaluates membership on the relevant
  created/updated events with bounded latency — the add-a-matching-record test pins
  entry without refresh, and its converse pins drop-out.
- **Views restore exactly.** A saved view returns its columns, sort, and filter
  state precisely, per user; one user's view never alters another's default.
- **Exports are exact and attributable.** Exported rows equal filtered rows,
  including custom-field columns; every export writes one audit entry.
- **Segregation is structural.** A membership row must match its list's entity
  type — a lead can never appear in a person list (rejected as a typed validation
  error) — preserving the lead quarantine by construction.
- **Bounded queries.** Compiled predicates are indexable and bounded (no full-table
  scan on a filtered list at seed scale); the list-view p95 budget (PERF-2) is
  enforced in CI.

## Acceptance

Done means: a user builds a nested AND/OR filter over any allowed field including a
custom field, watches the match count recompute live, saves it as a dynamic list
that stays current as data changes, saves the table shape as a view that restores
exactly, and exports precisely that slice with the export recorded in the audit
trail — with natural language available as a compiled-and-editable way in. The
filters-and-views screen renders the standard honest states (empty, loading, error,
no-permission, nothing-grounded) inherited from the acceptance-standards chapter;
its screen-specific behaviour is pinned verbatim in the Acceptance appendix. The
testable form of every claim above lives there.

## Out of scope

- **The export mechanism** — the async job lifecycle, wire operation, and the
  open-format round-trip promise — is the import-export-migration chapter's; this
  chapter supplies the filter the export honors.
- **Cross-object join queries in the UI** are reporting (the corpus draws this line
  explicitly); a list filters one object.
- **Custom-field mechanics** (the governed add-a-column path) are the custom-fields
  chapter's; this chapter only consumes the resulting real columns.
- **The add-to-list automation action** is the automation chapter's behaviour; it
  writes static membership through this chapter's surface.
- **Shared/team views and view-level permissions** are a declared fast-follow, not
  V1 behaviour (the sharing enum is schema-forward only, LVS-PARAM-3).

## Where it lives

A lists module in the backend's module layout (domain, use cases, adapters,
transport per the code-organization chapter), owning the filter engine and the
lists/tags/views surfaces; the screens ship in the web app's lists feature area on
design-system primitives. Read next: the data-model chapter for the vocabulary and
polymorphic conventions this chapter builds on, people-and-organizations for the
records being segmented, import-export-migration for the export mechanism, and
automation for the standing rules that write into lists.

## Appendix

### Parameters
Source: features/10-operational-depth.md#3-segmentation-lists--views-promotes-d52--d53--d54--d66; product/build-backlog/E15.md#b-e1510a--canonical-typed-andor-predicate-builder-incl-custom-fields; contract/data-model.md#saved-views-quota-field-mask @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| LVS-PARAM-1 | Filter operator set | `eq, neq, gt, lt, gte, lte, in, contains, exists` | The closed, existing filter-DSL operator set, typed per field type (text/number/date/currency/picklist/boolean). No new operator grammar is invented (B-E15.10a); a non-allowed field → `422`. |
| LVS-PARAM-2 | Export formats | CSV, JSON | The [MVP] open formats filtered export round-trips (features/10 §3 AC). See note LVS-N-2 on the mockup's extra format. |
| LVS-PARAM-3 | Saved-view sharing scope, V1 | `private` | V1 enforces private per-user views; the `team`/`workspace` enum values are schema-forward for the [TS] shared-views fast-follow (RT-PR-H1), not shipped behaviour. |
| LVS-PARAM-4 | Dynamic re-evaluation latency | eventual, bounded — **no numeric bound pinned** | The corpus pins "eventual, bounded latency" and no number; the concrete bound arrives with the dynamic-lists ticket (B-E15.11). Honest gap, recorded. |

Note LVS-N-1 (vocabulary ownership): the `tag` vocabulary is owned by this chapter —
the people-and-organizations chapter's PO-N-VOCAB reconcile note cedes it here. A tag
predicate applies to the four taggable entity types via the polymorphic join
(LVS-DDL-2); DM-VOCAB remains the oracle for scalar field allow-lists, and aligning
`tag` into the per-resource vocabulary rows is a contract-extension item (LVS-EXT-7).

Note LVS-N-2 (screen-vs-feature drift): the mockup's export card renders CSV/Excel/
JSON (AC-filters-and-views-8) while the feature AC pins CSV/JSON round-trip
(LVS-AC-4). The feature spec is normative for the round-trip guarantee; Excel as a
third emit-only format is a ticket-level decision, not pinned here.

Note LVS-N-3 (vocabulary width): the mockup filters on fields outside today's
DM-VOCAB rows (e.g. a region picklist, a last-emailed recency); the engine's contract
is the DM-VOCAB allow-list *plus custom fields* (B-E15.10a). Fields the product needs
beyond DM-VOCAB arrive either as custom fields or as DM-VOCAB extension rows in the
owning chapters — this chapter never widens the vocabulary unilaterally.

### Formulas
Source: features/10-operational-depth.md#3-segmentation-lists--views-promotes-d52--d53--d54--d66; contract/data-model.md#101-list--list_member @ 5a0b29c

**LVS-F-1 — Dynamic-list membership evaluation.** Single home for the dynamic-list
semantics; the screen and the automation add-to-list action cite this pin.

Inputs: a validated filter definition in the canonical representation (nested AND/OR
groups; leaves = allowed field × LVS-PARAM-1 operator × typed value), the list's
entity type, the workspace's live rows of that type.

```
membership(list) :=
  if list.list_type = 'static':
      the curated member rows (explicit, unique per entity)
  if list.list_type = 'dynamic':
      rows(list.entity_type) WHERE compile(list.definition)   -- bounded, indexable SQL
                             AND archived_at IS NULL          -- live rows only

re-evaluate(dynamic list) on: <entity_type>.created / .updated
                              (+ .archived / .merged via the same consumer lane)
  → eventual, bounded latency (LVS-PARAM-4)
```

Output: the current member set, ordered by the caller's sort with `id` as the
implicit final tie-breaker (DM-VOCAB-ERR-2, cited — a total order for keyset
pagination). Tie-breaks: none beyond ordering; membership is a set.

Worked example (**derived** — illustrative values, not corpus rows): a dynamic
organization list with definition "industry = Automotive AND owner is Mor" holds 3
matching live organizations. A fourth organization is updated to industry
Automotive → its updated event triggers re-evaluation → membership becomes 4 without
a manual refresh. The same organization is archived → it leaves the set on the next
re-evaluation. The stored truth throughout is the definition, not the row set.

Note LVS-GAP-1 (honest pin of what the corpus says): per the corpus, a dynamic list
is "a stored, typed query definition — not free-form SQL" and the members wire
surface is documented for *static* lists; dynamic membership is therefore
definition-derived. Whether an implementation materializes the derived membership as
a cache (versus computing at read) is unpinned — the B-E15.11 ticket decides;
LVS-F-1's semantics and the event-driven freshness AC bind either implementation.

### Schema
Source: contract/data-model.md#101-list--list_member + #102-tag--taggable + #saved-views-quota-field-mask @ 5a0b29c

This chapter owns five tables per the data-model chapter's ownership index:
`list`, `list_member`, `tag`, `taggable`, `saved_view`.

**LVS-DDL-1 — `list` / `list_member`** (corpus DDL, verbatim):

```sql
CREATE TABLE list (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name         text NOT NULL,
  entity_type  text NOT NULL CHECK (entity_type IN ('person','organization','deal','lead')),
  list_type    text NOT NULL DEFAULT 'static' CHECK (list_type IN ('static','dynamic')),
  definition   jsonb NULL,          -- dynamic: the validated query-plan; static: NULL
  owner_id     uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  team_id      uuid NULL REFERENCES team(id) ON DELETE SET NULL, -- team-scoped visibility (features/04 §1)
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL
);

CREATE TABLE list_member (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  list_id      uuid NOT NULL REFERENCES list(id) ON DELETE CASCADE,
  entity_type  text NOT NULL CHECK (entity_type IN ('person','organization','deal','lead')),
  entity_id    uuid NOT NULL,       -- polymorphic by (entity_type, entity_id); FK integrity by trigger/app
  added_by     text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT list_member_unique UNIQUE (list_id, entity_type, entity_id)
);
CREATE INDEX idx_list_member_list   ON list_member (list_id);
CREATE INDEX idx_list_member_entity ON list_member (workspace_id, entity_type, entity_id);
```

Corpus-normative notes carried with the DDL: `entity_id` is polymorphic-by-convention
(integrity by application + archive-cascade cleanup, DM-CONV-15 cited);
`list.entity_type` keeps lead lists structurally separate from contact lists (the
ADR-0008 "own list" requirement); a member row MUST match its list's entity type —
the API rejects a mismatch with `422 code: entity_type_mismatch`; the polymorphic
entity-type enum is the canonical one (DM-CONV-17, cited).

**LVS-DDL-2 — `tag` / `taggable`** (corpus DDL, verbatim):

```sql
CREATE TABLE tag (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name         text NOT NULL,
  color        text NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL,
  CONSTRAINT tag_name_unique UNIQUE (workspace_id, lower(name))
);

CREATE TABLE taggable (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  tag_id       uuid NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
  entity_type  text NOT NULL CHECK (entity_type IN ('person','organization','deal','lead')),
  entity_id    uuid NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT taggable_unique UNIQUE (tag_id, entity_type, entity_id)
);
CREATE INDEX idx_taggable_entity ON taggable (workspace_id, entity_type, entity_id);
CREATE INDEX idx_taggable_tag    ON taggable (tag_id);
```

**LVS-DDL-3 — `saved_view`** (corpus DDL, verbatim — the corpus writes this table in
its net-new-V1 stub style, base columns + version elided by the shared convention,
DM-CONV rows cited):

```sql
CREATE TABLE saved_view (                                 -- a saved list filter/sort (E15). V1 = per-user; shared/team views fast-follow (RT-PR-H1)
  -- + base columns + version
  owner_id      uuid NOT NULL REFERENCES app_user(id),
  shared_scope  text NOT NULL DEFAULT 'private' CHECK (shared_scope IN ('private','team','workspace')),  -- V1 enforces 'private'
  resource      text NOT NULL CHECK (resource IN ('people','organizations','deals','activities','leads','partners')),
  name          text NOT NULL,
  query         jsonb NOT NULL                              -- filter+sort using the §13.5 vocabulary only
);
```

Note LVS-DDL-N-1: `saved_view.query` and dynamic `list.definition` carry the same
canonical filter representation (B-E15.10a traces) — one engine at the storage layer
too. Note LVS-DDL-N-2: the stub's elided base columns/version and its named
constraints/indexes are completed by this chapter's schema ticket under the shared
conventions (DM-CONV-1..17, cited) — a build item, not a corpus fact.

### Wire
Source: contract/crm.yaml (Lists + Tags paths) @ 5a0b29c — operations cited by operationId, never restated.

| ID (operationId) | Purpose | Notes |
|---|---|---|
| `listLists` | List lists (static + dynamic) | `entity_type` filter + include-archived. |
| `createList` | Create a static list or dynamic segment | Dynamic carries the validated definition; invalid → `422`. |
| `getList` | Read a list | `404` when absent. |
| `archiveList` | Soft-delete a list | Returns the archived entity. |
| `listListMembers` | Page a (static) list's members | Cursor + limit; static-curation surface (see LVS-GAP-1). |
| `addListMember` | Add a member to a static list | `409` duplicate (list_member_unique); `422 entity_type_mismatch`. |
| `listTags` | List tags | Include-archived. |
| `createTag` | Create a tag | `409` on case-insensitive name collision (tag_name_unique). |
| `archiveTag` | Soft-delete a tag | |
| `applyTag` | Apply a tag to person/org/deal/lead | `201` join row; `409` duplicate (taggable_unique). |

**Wire — contract extensions (D-H2).** Contract-first extension tickets: crm.yaml
grows before any handler is written.

| ID | Extension | Notes |
|---|---|---|
| LVS-EXT-1 | Saved-view CRUD operations | The views surface (create/list/get/update/archive) is named in the build traces but absent from crm.yaml today. |
| LVS-EXT-2 | Update a list | Rename / edit a dynamic definition; no update operation exists (create/get/archive only). |
| LVS-EXT-3 | Remove a static-list member | Only add exists today. |
| LVS-EXT-4 | Un-apply a tag from an entity | Only apply exists today. |
| LVS-EXT-5 | Filtered-export operation accepts the canonical filter representation | The export operation, job lifecycle, and formats are owned by the import-export-migration chapter (cite); this row pins only that its filter parameter is this chapter's representation — one engine on the wire. |
| LVS-EXT-6 | NL-compile operation returning an editable canonical filter | The fast path in (B-E15.14); rides the search seam; the response is a filter tree, never an opaque result set. |
| LVS-EXT-7 | `tag` filter-vocabulary alignment | Reconcile the tag predicate into the per-resource vocabularies (PO-N-VOCAB cession; DM-VOCAB rows currently lack it while the people list operation carries it). |

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c — definitions live in the central catalog ([[event-bus#events--catalog]]); cited, never redefined.

| ID | Direction | Note |
|---|---|---|
| `person.created` / `person.updated` / `person.archived` / `person.merged` | consumed | Dynamic-list re-evaluation triggers for person lists (LVS-F-1). |
| `organization.created` / `organization.updated` / `organization.archived` / `organization.merged` | consumed | Same, for organization lists. |
| `deal.created` / `deal.updated` / `deal.stage_changed` / `deal.owner_changed` / `deal.archived` | consumed | Same, for deal lists (stage/owner have their own events — both are membership-relevant deltas). |
| `lead.created` / `lead.updated` / `lead.promoted` / `lead.disqualified` | consumed | Same, for lead lists; promotion removes the lead from lead lists via the archive/cleanup path. |

Note LVS-GAP-2 (D-H2 event-catalog extension item): the catalog defines **no**
list/tag/view lifecycle events, and the audit floor requires every core mutation to
produce one audit entry and one domain event ([[acceptance-standards#GATE-CORE-5]]).
Whether list/tag mutations are in-scope "core mutations" for the event side — and
which events they emit — is an event-catalog extension to resolve with the tickets;
the audit-entry half is unconditional.

### Acceptance
Source: product/epics/E15-operational-depth.md#s-e153--dynamic-lists-advanced-filtering-saved-views--filtered-export @ 5a0b29c

**Owned story atoms** (condensed per-atom; tiers per the epic — S-E15.3 is V1-Must).

| ID | Given/When/Then (condensed) | Verification |
|---|---|---|
| S-E15.3a | Given a nested AND/OR filter built over any allowed field including custom fields, when saved as a dynamic list, then membership stays current as records change (event-driven, bounded latency); an NL query compiles into the same editable structured representation and, once saved, behaves identically to a hand-built filter. | Predicate-correctness unit test against independent ground truth ([[testing#TEST-LANE-1]]); add-a-matching-record + shared-engine integration tests ([[testing#TEST-LANE-2]]); NL compile-then-edit equivalence (B-E15.10/.11/.14). |
| S-E15.3b | Given a table whose columns/sort/filter I set and save as a view, when I return, then it restores exactly; view state is per-user and never alters another user's default. | Save→reload round-trip integration test ([[testing#TEST-LANE-2]]); per-user isolation test (B-E15.12). |
| S-E15.3c | Given a filter or view, when I export, then I get exactly that slice in open formats and the export is audit-logged. | Round-trip test: exported rows == filtered rows incl. custom-field columns; one audit entry per export ([[testing#TEST-LANE-2]]); the export job itself is verified by [[import-export-migration]] (B-E15.13). |

Source: features/10-operational-depth.md#3-segmentation-lists--views-promotes-d52--d53--d54--d66 @ 5a0b29c

**Feature-doc acceptance criteria** (verbatim). Cross-cutting floors (standard screen
states STATE-1..5, PERF-2, release gates) are inherited from
[[acceptance-standards]] and not repeated per row.

| ID | Acceptance criterion (verbatim) | Verification |
|---|---|---|
| LVS-AC-1 | A dynamic list re-evaluates membership on the relevant `*.created/updated` events (eventual, bounded latency) — asserted by an add-a-matching-record test. | Integration ([[testing#TEST-LANE-2]]) over the event consumer (LVS-F-1). |
| LVS-AC-2 | The filter builder supports nested AND/OR over typed operators per field type; the same filter representation drives lists, views, and filtered export (one engine). | Shared-engine test — one parser/evaluator, not per-surface variants ([[testing#TEST-LANE-2]]). |
| LVS-AC-3 | A saved view restores columns/sort/filter exactly; it is per-user view state (P1-exempt, `runtime-config-surface.md §3`). | Round-trip integration test ([[testing#TEST-LANE-2]]). |
| LVS-AC-4 | Filtered export round-trips to CSV/JSON honoring the active filter; export is audit-logged (P7/P12). | Round-trip + audit-entry integration test ([[testing#TEST-LANE-2]]); mechanism per [[import-export-migration]]. |
| LVS-AC-5 | **User-observable (Mor/Sam, S-E15.3):** build a filter once, save it as a list and a view, and it stays current as data changes — and export exactly that slice. | Live-stack lane ([[testing#TEST-LANE-3]]) across the filters-and-views screen. |

Source: product/30-screen-acceptance.md#filters-and-viewshtml--advanced-filters-saved-views--export-implements-s-e153a-s-e153b-s-e153c @ 5a0b29c

**Owned screen ACs** (verbatim; this chapter owns the filters-and-views screen —
ACID-4). All rows verify in the screen-acceptance e2e suite ([[testing#TEST-LANE-3]]);
the standard state matrix (STATE-1..5) applies on top and is not restated.

| ID | Given/When/Then (verbatim) |
|---|---|
| AC-filters-and-views-1 | Given the screen at load on the Contacts object, When it renders, Then it shows a breadcrumb (Contacts · Filters & views), a Contacts/Companies/Deals object segmented control with Contacts active, the NL describe-a-segment bar, a predicate builder card, a results table with `3 contacts match`, a "Dynamic — recomputes on every event" live badge, and right-rail Saved views, Export, and Preview-states cards. |
| AC-filters-and-views-2 | Given the NL bar, When I type a description and press Enter, Then a "Compiled into an editable filter" panel appears stating it is my fast path in (not a black box), showing the structured clauses it produced (e.g. `industry = Automotive`, `region ∈ {DE, AT, CH}`, `last_emailed_at < now − 30d`) plus a "1 clause low-confidence — review" confidence note, and a toast confirms the clauses are editable before saving. |
| AC-filters-and-views-3 | Given the predicate builder, When it renders, Then the root group exposes an ALL·AND / ANY·OR segmented toggle (AND active) over its clauses, contains a genuinely nested sub-group whose toggle is set to ANY·OR, and one nested clause (`QA program owner`) carries a "custom field" badge — proving the tree nests AND/OR and reaches custom fields. |
| AC-filters-and-views-4 | Given any group, When I click "Add clause", remove a clause via its × control, switch a group's AND/OR toggle, or change a field/value select, Then the matching clause is added/removed/updated and the `N contacts match` count recomputes live (recount runs on each of these actions). |
| AC-filters-and-views-5 | Given the results table, When I click the "Columns" control, Then a popover lists the five columns (Industry, Region, Relationship strength, Last emailed, Provenance) with on/off checkboxes; toggling one shows/hides that column in the table and updates the export "visible columns" count, and clicking a sortable header (Contact / Strength / Last emailed) re-sorts and flips the asc/desc chevron. |
| AC-filters-and-views-6 | Given a dynamic list, When I click "Simulate event" (or a captured event fires), Then the results enter a recomputing/loading state, membership is re-evaluated, and a row whose recency now falls outside the 30-day clause drops out with a toast ("1 contact dropped (now within 30d)") — demonstrating membership auto-updates on events rather than being a frozen snapshot. |
| AC-filters-and-views-7 | Given the Saved views rail, When it renders, Then it lists views tagged `dynamic` (membership auto-updates, columns+sort restored) and one explicitly marked "Static snapshot · frozen 12 Jun", each with a match count; clicking a view marks it active and toasts that columns, sort & filter restore exactly; a "Save current as view" button captures the current columns+sort+filter as a per-object, per-user view. |
| AC-filters-and-views-8 | Given the Export card, When it renders, Then a format selector (CSV/Excel/JSON) is shown, a scope box states it exports "exactly the filtered slice" (the N matching rows and the visible-column count, "no silent full-table dump"), a 🟡 gate note states export is approval-logged and queues to the approval inbox, and the primary button reads "Export N rows · <FMT>"; clicking it toasts that the export was queued to the approval inbox and logged to `audit_log`. Format and column changes keep the row/column counts and button label in sync. |

Note LVS-N-4 (screen-vs-feature reconcile, recorded honestly): AC-filters-and-views-8
renders export as a 🟡 approval-inbox-queued action; the feature AC (LVS-AC-4) pins
audit-logging and says nothing of an approval gate. The approval-tier call for export
is the export mechanism's to pin ([[import-export-migration]] with the governance
chapters); this chapter's screen renders whichever gate that pin declares.
