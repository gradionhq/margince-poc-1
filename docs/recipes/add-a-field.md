---
derives-from:
  - architecture/generators.md
  - architecture/contract-pipeline.md
  - architecture/data-model.md
  - quality/quality-gates.md
---
<!-- PROCESS DOC — commands, paths, and make targets are this document's subject, so the
     prose-only ban-list (no fences / paths / targets above the appendix) does not apply
     here. Recipe doc: no `## Appendix`. -->
# Add a field — the generator-driven canonical recipe

The P2 source-customization path for a **first-class field a developer commits** to
person / organization / deal / activity / lead. This file is the recipe's home — the
generators chapter announced the move here and stays the command reference
([[generators#GEN-CMD-1]]). For a *workspace* adding its own runtime custom field
without a deploy, use the RC-12 path instead — never this recipe.

The exemplar to mirror: any existing column on the `person` table — the generator
walks your field through the same struct → contract → types → test chain the slice's
fields already ride.

1. **Scaffold.** Run the field generator ([[generators#GEN-CMD-1]]):

   ```bash
   make gen-field ARGS="person nickname text string"   # <table> <column> <sql-type> [go-type]
   ```

   It writes a sequenced migration pair under `backend/migrations/` and emits the
   round-trip test stub, then prints the remaining steps.

2. **Domain struct.** Add the field to the owning module's entity under
   `backend/internal/modules/<name>/domain/` (for `person`: `modules/people`). A
   default value belongs here (or as the migration's column `DEFAULT`) so the zero
   value preserves behavior ([[contract-pipeline#CP-BREAK-7]]); an enum-valued field
   extends the column's CHECK constraint ([[data-model#DM-CONV-13]]) and the contract
   enum together — that placement is the M4 mandate ([[generators#M4]]).

3. **Contract.** Add the property to the schema in `backend/api/crm.yaml` — an
   extension field on a core object carries `x-extension: true`
   ([[contract-pipeline#CP-EXT-2]]); adding an optional property is additive-safe
   ([[contract-pipeline#CP-BREAK-14]]).

4. **Regenerate.** `make gen-types` — Go + TS contract types, committed as generated.

5. **Migrate and check.** `make migrate-up && make gen-types-check && make check`.

6. **Round-trip assertion — the M1 mandate** ([[generators#M1]]). Fill in the
   generator-emitted test: the field survives create → read as identity, so a dropped
   DTO mapping fails CI instead of silently zeroing the field. This is guardrail G-a
   ([[quality-gates#G-a]]); a field without its round-trip test is not done.
