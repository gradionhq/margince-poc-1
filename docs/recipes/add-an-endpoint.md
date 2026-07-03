---
derives-from:
  - architecture/contract-pipeline.md
  - architecture/api-conventions.md
  - architecture/code-organization.md
  - quality/testing.md
  - quality/quality-gates.md
---
<!-- PROCESS DOC — commands, paths, and make targets are this document's subject, so the
     prose-only ban-list (no fences / paths / targets above the appendix) does not apply
     here. Recipe doc: no `## Appendix`. -->
# Add an endpoint — contract first, always

The contract is the single source of truth for the wire ([[contract-pipeline#CP-STAGE-1]]);
the code follows it, never the reverse. The exemplar to mirror: the people paths in
`backend/api/crm.yaml` and `handler_person.go` in `modules/people/transport/`.

1. **Add the operation to `backend/api/crm.yaml`.** Copy the shape of the nearest
   people operation and follow the conventions register rather than inventing wire
   shapes: PATCH-merge for updates ([[api-conventions#API-CONV-1]]), DELETE archives
   and returns the entity ([[api-conventions#API-CONV-2]]), create is 201 + Location
   ([[api-conventions#API-CONV-3]]), snake_case names derived from the columns
   ([[api-conventions#API-CONV-4]]), the list envelope and cursor rules for
   collections ([[api-conventions#API-LIST-1]]..6). If agents should reach it, annotate
   `x-mcp-tool` with verb, record type, and tier ([[contract-pipeline#CP-MCP-1]],
   [[contract-pipeline#CP-MCP-2]]) — the tool surface generates from the same document.

2. **Regenerate types.** `make gen-types` — Go and TS types, plus the derived server
   interfaces and tool list ([[contract-pipeline#CP-STAGE-8]]). Generated output is
   committed and never hand-edited ([[code-organization#CODEORG-RULE-4]]).

3. **Implement transport → app → store.** The handler goes in the owning module's
   `transport/` as `handler_<thing>.go`, thin like the person handler: decode, call
   the `app/` use case, return typed sentinels and let the centralized mapper produce
   problem+json ([[code-organization#CODEORG-STEP-4]], [[api-conventions#API-ERR-6]]..21).
   The use case lives in `app/`, data access in `adapters/` through the
   workspace-scoped tx helper ([[code-organization#CODEORG-RULE-2]]). Honor If-Match
   on versioned mutations ([[api-conventions#API-CC-2]]..4) and Idempotency-Key on
   POSTs ([[api-conventions#API-CC-6]]).

4. **Wire it.** Register through the module's `module.go`; regenerate the route
   table/manifests (`make gen-manifests`) — never hand-merge
   ([[code-organization#CODEORG-STEP-5]]).

5. **Test it.** A unit test for the use case (no DB, [[testing#TEST-LANE-1]]) and an
   integration test driving the real handler over a real database
   ([[testing#TEST-LANE-2]]), asserting contract-typed responses — mirror the person
   endpoint's pair.

6. **Green the two contract gates.** `make gen-types-check` — the drift gate
   ([[quality-gates#QG-6]]) fails if committed types went stale; the breaking-change
   gate ([[quality-gates#QG-7]]) fails if the contract change breaks an existing
   consumer ([[contract-pipeline#CP-BREAK-20]]). Additive changes pass; removing
   anything requires deprecate-before-remove ([[contract-pipeline#CP-BREAK-24]]).
   Finish with `make check`.
