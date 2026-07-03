---
derives-from:
  - architecture/code-organization.md
  - architecture/contract-pipeline.md
  - architecture/api-conventions.md
  - architecture/data-model.md
  - architecture/frontend.md
  - quality/testing.md
  - quality/quality-gates.md
---
<!-- PROCESS DOC — commands, paths, and make targets are this document's subject, so the
     prose-only ban-list (no fences / paths / targets above the appendix) does not apply
     here. Recipe doc: no `## Appendix`. -->
# Add a vertical slice — the full end-to-end path

A new capability with an API, data, and a screen. The steps run in the pinned order
([[code-organization#CODEORG-STEP-0]]..8); at every step, the move is the same:
**mirror what the people module does for Person**, renamed for your concept.

0. **Agree the port first — only if the feature crosses a module boundary**
   ([[code-organization#CODEORG-STEP-0]]). Add the interface under
   `backend/internal/shared/ports/`, dependency-free, before anyone codes. The one
   pre-parallel step; skip it for a single-module slice like Person.

1. **Contract first** ([[code-organization#CODEORG-STEP-1]]). Add the operations and
   schemas to `backend/api/crm.yaml`, shaped like the people paths (list envelope,
   PATCH-merge, 201 + Location — per the api-conventions register,
   [[api-conventions#API-CONV-1]]..11). Then `make gen-types`; generated Go/TS types
   are committed, never hand-edited ([[contract-pipeline#CP-STAGE-6]]).

2. **Schema** ([[code-organization#CODEORG-STEP-2]]). A sequenced migration pair via
   `make migrate-create NAME=add_<thing>`, mirroring the person table: base columns,
   triggers, RLS, provenance ([[data-model#DM-CONV-3]]). Full walkthrough:
   [add-a-migration.md](add-a-migration.md).

3. **Domain + store** ([[code-organization#CODEORG-STEP-3]]). In the owning module —
   entity in `domain/`, use case in `app/`, repository in `adapters/` — the way
   `modules/people` holds Person. All DB access through the workspace-scoped tx
   helper, never the raw pool ([[code-organization#CODEORG-RULE-2]]); audit row and
   provenance where the rules require it.

4. **Handler** ([[code-organization#CODEORG-STEP-4]]). `handler_<thing>.go` in the
   module's `transport/`, thin like `handler_person.go`: typed sentinel errors, the
   centralized problem+json mapper, If-Match per [[api-conventions#API-CC-2]].

5. **Wiring** ([[code-organization#CODEORG-STEP-5]]). Register through the module's
   `module.go`; the import manifest and route table are generated — `make
   gen-manifests`, never a hand-merge ([[code-organization#CODEORG-RULE-4]]). The
   composition root stays untouched.

6. **Frontend** ([[code-organization#CODEORG-STEP-6]]). A feature directory
   `frontend/src/features/<name>/{api,components,hooks,routes}` mirroring
   `features/people`: the list hook, the layer-2 card, the layer-3 list, the layer-4
   page. Full walkthrough: [add-a-screen.md](add-a-screen.md).

7. **Tests at each layer** ([[code-organization#CODEORG-STEP-7]]). Mirror the slice's
   four: a unit test (no DB, [[testing#TEST-LANE-1]]), an integration test (real DB +
   RLS, fails rather than skips — [[code-organization#CODEORG-RULE-3]]), stories for
   each component state ([[frontend#FE-DS-24]]), and a page test. Each lives beside
   what it pins.

8. **Green the gates** ([[code-organization#CODEORG-STEP-8]]). `make check`, then
   `make test-integration`. Drift ([[quality-gates#QG-6]]), breaking-change
   ([[quality-gates#QG-7]]), manifests ([[quality-gates#QG-8]]), arch-lint
   ([[quality-gates#QG-9]]) all red-block; a red gate means the slice is not done.
