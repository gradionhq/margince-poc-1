---
derives-from:
  - architecture/code-organization.md
  - architecture/data-model.md
  - architecture/event-bus.md
  - architecture/frontend.md
  - quality/acceptance-standards.md
  - quality/quality-gates.md
  - quality/testing.md
---
<!-- PROCESS DOC — commands, paths, and make targets are this document's subject, so the
     prose-only ban-list (no fences / paths / targets above the appendix) does not apply
     here. Recipe doc: no `## Appendix`. -->
# Recipes — copy the shape of the slice

The doctrine is **exemplar-first**: the skeleton ships one thin vertical slice — the
**Person slice** — one table, one endpoint, one screen, one test at each layer, held
green from the first commit by the running-scaffold gate ([[quality-gates#QG-25]]).
A worker's first move on any ticket is never to invent a shape; it is to **copy the
shape of the slice** — find the Person artifact at the layer being touched, mirror
it, rename it. Every recipe below is that move written out for one concern.

## The slice inventory

What the exemplar demonstrates, layer by layer:

| Layer | Slice artifact | Where |
|---|---|---|
| Migration | the `person` table pair — base columns, triggers, RLS ([[data-model#DM-CONV-3]], [[data-model#DM-CONV-8]]) | `backend/migrations/` |
| Contract | the people paths and `Person` schemas | `backend/api/crm.yaml` |
| Generated types | Go + TS contract types (`make gen-types`, never hand-edited) | beside the shared kernel · `frontend/src/lib/api-client` |
| Domain | the `Person` entity and its invariants | `backend/internal/modules/people/domain/` |
| Store | the person repository over the workspace-scoped tx helper ([[code-organization#CODEORG-RULE-2]]) | `backend/internal/modules/people/adapters/` |
| Handler | `handler_person.go` — thin, sentinel errors, contract-typed | `backend/internal/modules/people/transport/` |
| Wiring | `module.go` manifest; routes/manifests regenerated, never hand-merged | `backend/internal/modules/people/` |
| Event | `person.created` staged through the outbox ([[event-bus#EVT-DEL-5]]) | catalog row in [event-bus.md](../architecture/event-bus.md) |
| FE data | the list hook (`usePeople`) on the query cache | `frontend/src/features/people/api/` |
| FE components | `PersonCard` (layer 2), `PersonList` (layer 3) ([[frontend#FE-LAYER-2]], [[frontend#FE-LAYER-3]]) | `frontend/src/features/people/components/` |
| FE route | `PeoplePage` (layer 4) with the five standard states ([[acceptance-standards#STATE-1]]..5) | `frontend/src/features/people/routes/` |
| Tests | unit (no DB) + integration (real DB, RLS) + stories + page test ([[testing#TEST-LANE-1]], [[testing#TEST-LANE-2]]) | beside what they pin |

## The recipes

| Recipe | Use it when |
|---|---|
| [add-a-vertical-slice.md](add-a-vertical-slice.md) | a new capability needs API + data + screen, end to end |
| [add-a-field.md](add-a-field.md) | a first-class field joins a core object (generator-driven) |
| [add-an-endpoint.md](add-an-endpoint.md) | a new operation joins the contract |
| [add-a-migration.md](add-a-migration.md) | the schema changes |
| [add-a-screen.md](add-a-screen.md) | a new page or view joins the frontend |
| [add-an-event.md](add-an-event.md) | a mutation needs a domain event on the bus |

## The rule

Recipes cite pins and the slice — they **never invent conventions**. Every normative
fact a recipe rests on lives in a pinned chapter; the recipe cites the pin
([[code-organization#CODEORG-STEP-1]] style) instead of re-deriving it. If a recipe
and a pin ever disagree, the pin wins and the recipe is the defect. If a step needs a
convention no chapter pins, that is a chapter change first (this tree's rule 2 — a
new decision has no build effect until it lands in a chapter), then a recipe edit.
