---
derives-from:
  - architecture/event-bus.md
  - architecture/data-model.md
  - architecture/code-organization.md
  - quality/quality-gates.md
---
<!-- PROCESS DOC — commands, paths, and make targets are this document's subject, so the
     prose-only ban-list (no fences / paths / targets above the appendix) does not apply
     here. Recipe doc: no `## Appendix`. -->
# Add an event — one committed fact, one reliable event

Every domain event lives in the event-bus chapter's catalog; no other chapter — and
no recipe — defines one. Adding an event is therefore a **docs change and a code
change in the same ticket**. The exemplar to mirror: `person.created`, emitted by
`modules/people` and staged through the outbox.

1. **Register the catalog row first (the docs-side pin).** Add the event to the
   catalog in [event-bus.md](../architecture/event-bus.md): type as
   `<entity>.<verb>`, past tense — a fact that already happened; payload of changed
   or relevant fields only, never the full row ([[event-bus#EVT-SEM-6]]); version 1;
   emitting module; consumers. A specific verb **replaces** the generic `updated`
   for its transition, never fires alongside it ([[event-bus#EVT-SEM-2]]); the one
   sanctioned co-fire is pinned as [[event-bus#EVT-SEM-3]]. The relay derives the
   stream from the type's entity segment ([[event-bus#EVT-STREAM-1]]..9). An event
   not in the catalog does not exist — this tree's rule 2.

2. **Emit through the outbox, in the same transaction.** The mutation follows
   one-mutation-one-audit-row-one-event ([[event-bus#EVT-SEM-1]]): the use case
   writes the domain row, the audit row, and the outbox row
   ([[data-model#DM-DDL-9]]) in one transaction — mirror the person store's create
   path. Rolled back means no event; committed means never lost
   ([[event-bus#EVT-DEL-5]]). Never publish inline on the request path and never
   from a DB trigger — the relay publishes post-commit.

3. **Fill the envelope, invent nothing.** Every event carries the standard envelope
   ([[event-bus#EVT-ENV-1]]): time-ordered `event_id`, workspace, structured actor,
   the entity **reference**, and the trace block linking `correlation_id`,
   `causation_id`, and the audit row.

4. **Make every consumer idempotent.** Delivery is at-least-once
   ([[event-bus#EVT-DEL-1]]); consumers dedupe on `event_id` and upsert by natural
   key, never blind-insert ([[event-bus#EVT-DEL-2]]), and close redelivery reorder
   with last-writer-wins on the entity version ([[event-bus#EVT-SEM-8]]). A new
   consumer joins its module's consumer group ([[event-bus#EVT-CG-1]]..7) and
   filters on the envelope's workspace field.

5. **Version additively.** New payload fields never bump the type's version;
   removing, renaming, or retyping one does, with the dual-publish window pinned to
   the retention horizon ([[event-bus#EVT-DEL-6]]).

6. **Test and gate.** An integration test asserts the committed mutation is
   observable on the bus with the right type, envelope, and payload keys — and a
   rolled-back one nowhere; a redelivered event produces no double effect (the
   event-bus chapter's acceptance). Audit coverage and coherence gates
   ([[quality-gates#QG-11]], [[quality-gates#QG-12]]) hold the paired audit row
   honest; finish with `make check`.
