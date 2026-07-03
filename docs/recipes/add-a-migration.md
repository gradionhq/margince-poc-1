---
derives-from:
  - architecture/data-model.md
  - architecture/code-organization.md
  - architecture/event-bus.md
  - quality/quality-gates.md
---
<!-- PROCESS DOC — commands, paths, and make targets are this document's subject, so the
     prose-only ban-list (no fences / paths / targets above the appendix) does not apply
     here. Recipe doc: no `## Appendix`. -->
# Add a migration — a sequenced, reversible pair

Every schema change is a sequential, reversible up/down pair under
`backend/migrations/` ([[data-model#DM-CONV-16]]). The exemplar to mirror: the
person table's migration — every obligation below is already demonstrated there.

1. **Create the pair.**

   ```bash
   make migrate-create NAME=add_<thing>    # → NNNNNN_add_<thing>.up.sql / .down.sql
   ```

   ([[code-organization#CODEORG-CMD-12]]). Never renumber or edit an applied
   migration; a fix is a new pair.

2. **Write the up.** A new tenant table copies the person table's spine:
   - `id uuid PRIMARY KEY DEFAULT uuidv7()` via the canonical shim
     ([[data-model#DM-CONV-1]], [[data-model#DM-CONV-2]]);
   - the base columns — `workspace_id` FK, `created_at`, `updated_at`,
     `archived_at` ([[data-model#DM-CONV-3]]);
   - `version bigint` with the shared `BEFORE UPDATE` trigger bumping it and
     `updated_at` ([[data-model#DM-CONV-4]]);
   - `ENABLE` + `FORCE ROW LEVEL SECURITY` and a `<table>_tenant_isolation` policy in
     the deny-on-unset form ([[data-model#DM-CONV-8]], [[data-model#DM-CONV-6]]);
   - `source` / `captured_by` / `raw` where the table holds captured or user-entered
     domain data ([[data-model#DM-CONV-11]]);
   - an index on every FK column, tenant-prefixed composites for pinned list paths,
     partial where soft-delete-aware ([[data-model#DM-CONV-14]]).
   Names are snake_case and singular; enums are `text` + CHECK, never native `ENUM`
   ([[data-model#DM-CONV-12]], [[data-model#DM-CONV-13]]). Note the audit row is an
   **application-layer** obligation written in the same transaction as each mutation
   ([[event-bus#EVT-SEM-1]], gated by [[quality-gates#QG-11]]) — never a table trigger.

3. **Write the down.** It must cleanly reverse the up — drop what was created,
   restore what was altered. A pair that cannot round-trip is a defect
   ([[data-model#DM-CONV-16]]).

4. **Apply and prove reversibility.** `make migrate-up`, then round-trip the pair
   (down, up again) against the dev database and confirm the runner reports a clean,
   non-dirty status. Migrations applying cleanly to a fresh database is part of the
   running-scaffold gate ([[quality-gates#QG-25]]).

5. **Finish the chain.** A schema change is never alone: carry the field/endpoint
   work per [add-a-field.md](add-a-field.md) or
   [add-a-vertical-slice.md](add-a-vertical-slice.md), and run `make check` — the
   RLS store-path gate ([[quality-gates#QG-13]]) and integration lane
   ([[quality-gates#QG-23]]) hold the new table's isolation honest.
