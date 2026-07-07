---
status: planned
module: backend/internal/modules/people
derives-from:
  - specs/spec/features/01-core-objects.md#5-activity-timeline
  - specs/spec/contract/data-model.md#7-activity-timeline
  - specs/spec/contract/crm.yaml
---
# Activities & timeline — every touch is one row, linked to everything it touched

> The substrate under every record's history: a single polymorphic activity row —
> email, call, meeting, note, or task — joined to any number of people,
> organizations, and deals, carrying provenance on every row and an idempotent
> capture key so auto-capture can re-run forever without duplicating history.

## What it's for

Every 360 view answers "what actually happened with this person, company, or deal"
from one place, and in this product most of that history is written by machines,
not typed by reps. This chapter owns the substrate that makes both true: the
activity row, its typed links to the core objects, the constraints that keep
capture idempotent and attribution honest, and the operations that read and
maintain them. Its writers are the capture pipeline (the dominant one, per P5),
humans logging the exceptional manual note, and agents acting through the governed
tools; its readers are the 360 screens of the sibling core-CRM chapters, the task
work-queue, and the reporting and scoring chapters that lean on direction and
recency. The boundary is sharp: this chapter is the timeline's data and behavior,
never the surfaces that render it.

## Principles it serves

- **P5 — auto-capture over manual entry.** The table is designed to be written by
  machines safely: provenance on every row plus an idempotent capture key make
  re-ingestion harmless and make "who typed this" a measurable metric rather than
  a guess.
- **P11 — clean relational core.** One real table with real constraints and real
  indexes; the kinds are a checked closed set, not a reference table, and a task
  is a kind of activity, not a parallel object ([[glossary]] `activity`).
- **P12 — governance is designed in.** Every row states what produced it and who
  captured it; re-associating a captured row is an audited association event,
  never a rewrite of its origin.
- **P4 — blazing fast, always.** The timeline read is one indexed scan inside the
  list budget ([[acceptance-standards#PERF-2]]); that budget is *why* the model is
  single-table.
- **ADR-0022 — capture build/borrow boundary.** The decision that admitted the
  messaging kinds into the activity kind set alongside the five core kinds.

The single-polymorphic-table shape was the corpus's flagged open question; the
data-model chapter ratifies it as settled ([[data-model]] note DM-CONV-N-1).

## How it works

**One table, a checked set of kinds.** An activity is one row whose kind
discriminates email, call, meeting, note, or task — the five core kinds the
vocabulary pins ([[glossary]] `activity`) — plus the two messaging kinds admitted
by ADR-0022. Type-specific fields stay nullable outside their kind, and the set is
a check constraint, not a reference table: a closed set that changes by migration
is exactly the enum posture the schema conventions mandate
([[data-model#DM-CONV-13]]), and it keeps agents able to extend kinds in source
(P2) without a metadata engine. The rejected alternative — one table per kind —
would turn every timeline read into a union across tables and fragment full-text
search; single-table keeps the read one indexed scan inside the performance budget
and lets capture write exactly one row. The cost, nullable columns, is contained
by shape constraints: task fields exist only on tasks, and a done task always
carries its completion moment (ACT-DDL-1).

**Links, not a single foreign key.** A captured email is about a person *and* the
deal it advances; a single parent pointer would force a copy or a coin-flip. So
the activity never points at one owner record — a separate typed link row joins it
to any number of people, organizations, and deals, each link shaped to exactly one
endpoint and structurally unique per (activity, endpoint), so the same association
can never exist twice (ACT-DDL-2). Per-endpoint indexes are what make "all
activity for this record" an indexed join, and the deal's recency field is
maintained from these links on write ([[deals-and-pipeline]]).

**Capture is idempotent by key.** A captured row carries the identity of the
provider record it came from — the originating system plus that system's record
id — and that pair is unique per workspace (ACT-DDL-1, the capture-key index).
Re-running capture over the same mailbox or calendar therefore cannot duplicate
history: a replay resolves to the existing row and answers as a no-op
(ACT-WIRE-2). This is the database half of the bus-side dedupe rule the event
chapter owns ([[event-bus#EVT-DEL-2]]).

**Provenance separates machine-written from human-typed.** Every row carries what
produced it and which principal captured it, non-null, in the fixed principal
vocabulary ([[data-model#DM-CONV-11]]), with the re-parseable original payload
kept off the query hot path. That is what makes the P5 health metric computable —
the share of a workspace's activities captured by agents versus typed by humans —
and it is why a human correction of a captured field is emitted with the human
attribution, the "typed-by" flag, rather than silently blending into the capture.

**Relink is association, not re-capture.** When capture guessed the wrong deal, or
a rep attaches an email to a second entity, the fix adds or moves a typed link in
one idempotent action: replaying the same relink creates nothing new, the row's
original provenance is preserved byte-for-byte — the relink says who associated
it, never who captured it — and exactly one audit row records the change
(ACT-WIRE-6). It is an internal association, so it sits on the 🟢 tier.

**Assignee is not owner.** A task carries an assignee — the person who must do the
work. That is deliberately not the ownership concept that drives record
visibility scoping ([[glossary]] `owner`): assigning a task confers an obligation,
not access, and access questions stay with the access chapters.

**Archive follows the edges.** Soft-deleting an activity archives it out of every
timeline; archiving a linked record archives the links into it so live timelines
never point at dead rows — but archiving a deal does not archive the activities it
shared with a person. Those cascade rules are owned by the schema conventions
([[data-model#DM-CONV-15]]); this chapter's tables obey them.

## What's configurable

- **Nothing at runtime of its own.** Extending the kind set is a source change (a
  migration, per P2); there is no kind admin surface.
- **Retention of activity rows** — the three-year archive default and the
  one-year transcript-erase default are seeded and owned by the data-model
  chapter ([[data-model#DM-SEED-2]], [[data-model#DM-SEED-3]]).
- **Timeline filter and sort vocabulary** — a closed allow-list, pinned at
  [[data-model#DM-VOCAB-4]]; anything outside it is rejected, never silently
  ignored.

## Guarantees (enforced)

- **No duplicate history.** The same provider record ingested twice yields one
  activity, held by the unique capture key (ACT-DDL-1) — re-running capture is
  always safe (ACT-AC-3).
- **No duplicate association.** The same (activity, endpoint) link cannot exist
  twice, held by the link uniqueness constraint (ACT-DDL-2); relink replays are
  no-ops (ACT-AC-9).
- **Origin survives correction.** Relinking preserves the row's capture
  provenance unchanged and writes exactly one audit row (ACT-AC-10).
- **Kinds keep their shape.** Task fields are impossible on non-tasks, and a done
  task always has a completion timestamp, both database-checked (ACT-AC-11).
- **Nothing anonymous.** Provenance is non-null on every row, asserted by the
  cross-cutting conformance test the data-model chapter owns
  ([[data-model#DM-AC-4]]).
- **The timeline stays fast.** The filtered 50-item read and full-text search
  hold their p95 budgets ([[acceptance-standards#PERF-2]],
  [[acceptance-standards#PERF-3]]); any AI summary renders asynchronously and
  never blocks the load (ACT-AC-6).

## Acceptance

Done, for this substrate, means the timeline fills itself and stays trustworthy:
captured emails and meetings land on the right records with visible provenance,
re-running capture changes nothing, an activity can honestly belong to more than
one record, a task completes with an audited state change, and the read path holds
its budgets with the summary arriving asynchronously. The share of machine-written
versus human-typed activity is reportable per workspace — the manual-entry-smell
metric. The testable forms are pinned in the Acceptance appendix; screen states
and the cross-cutting floor are inherited from [[acceptance-standards]] and owned
by the screens' chapters.

## Out of scope

- **Timeline rendering.** The timeline appears inside the person, organization,
  and deal 360 screens; those screens and their acceptance series belong to
  [[people-and-organizations]] and [[deals-and-pipeline]]. This chapter owns no
  screens and pins no screen ACs.
- **The task work-queue.** Operating the day from open tasks — the E16 stories —
  is [[tasks-and-work-queue]]; this chapter only guarantees the task-kind rows it
  reads.
- **The capture pipeline.** How email, calendar, and transcripts become activity
  rows — and the E02 stories (S-E02.2, S-E02.6) — belong to [[capture]]; this
  chapter owns the rows capture writes into.
- **Drafting and sending email.** Both operations hang off an activity on the
  wire, but they are [[drafting]]'s (sending is 🟡 and consent-gated); see the
  Wire note.
- **AI timeline summary and commitment extraction.** The L2 moments are owned by
  the AI-native chapters; this chapter's only promise about them is that the
  timeline load is never blocked (ACT-AC-6).
- **Links to external conversations.** The conversation-link table belongs to
  [[dispact-integration]], not here.

## Where it lives

The activity slice of the backend's people module — the shared CRM domain core —
with capture writing into it through the connector sink seam and agents through
the governed tool surface. The web timeline component renders inside sibling
chapters' screens. Read next: [[capture]] (who writes most rows),
[[tasks-and-work-queue]] (who works the task kind), [[data-model]] (the
conventions these tables obey), and [[event-bus]] (what a committed activity
emits).

## Appendix

### Schema
Source: contract/data-model.md#7-activity-timeline @ 5a0b29c

DDL is copied verbatim from the corpus contract; all DM-CONV rules apply; comments
inside the fences are the corpus's own. The capture-key index on ACT-DDL-1
(`uq_activity_source`) is the idempotency constraint this chapter owns.

**ACT-DDL-1 — `activity`.** The single polymorphic timeline row.

```sql
CREATE TABLE activity (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  kind          text NOT NULL CHECK (kind IN ('email','call','meeting','note','task','whatsapp','telegram')),  -- messaging kinds per ADR-0022

  subject       text NULL,
  body          text NULL,          -- normalized text (email body, note text, meeting notes)
  occurred_at   timestamptz NOT NULL DEFAULT now(),  -- when it happened (UTC)

  -- task-specific (nullable unless kind='task')
  due_at        timestamptz NULL,
  assignee_id   uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  is_done       boolean NOT NULL DEFAULT false,
  done_at       timestamptz NULL,

  -- meeting/call-specific
  duration_seconds integer NULL,
  direction     text NULL CHECK (direction IS NULL OR direction IN ('inbound','outbound')),  -- inbound/outbound for email/call; NULL for note/task. Load-bearing for lead-promotion (formulas §2), reciprocity (§4), inbound-vs-echo (§2.3)
  meeting_status text NULL CHECK (meeting_status IS NULL OR meeting_status IN ('booked','held','no_show','canceled')),  -- set only when kind='meeting'; drives the promotion meeting-trigger (§2) and lead scoring (§3)

  -- idempotent capture key (features/01 §5.1 — re-running capture makes no dupes)
  source_system text NULL,          -- 'gmail','gcal','outlook','transcript',...
  source_id     text NULL,          -- provider message/event id

  source        text NOT NULL,
  captured_by   text NOT NULL,
  raw           jsonb NULL,         -- re-parseable original; OFF the hot path (§1.6)

  search_tsv    tsvector GENERATED ALWAYS AS (
                  to_tsvector('simple', coalesce(subject,'') || ' ' || coalesce(body,''))
                ) STORED,

  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,

  CONSTRAINT activity_task_fields CHECK (kind = 'task' OR (due_at IS NULL AND assignee_id IS NULL AND is_done = false)),
  CONSTRAINT activity_done_at CHECK (is_done = false OR done_at IS NOT NULL)
);

-- idempotency: same provider record never creates two activities (features/01 §5.1 AC)
CREATE UNIQUE INDEX uq_activity_source
  ON activity (workspace_id, source_system, source_id)
  WHERE source_system IS NOT NULL AND source_id IS NOT NULL;

CREATE INDEX idx_activity_ws_time ON activity (workspace_id, occurred_at DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_activity_kind    ON activity (workspace_id, kind, occurred_at DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_activity_tasks   ON activity (workspace_id, assignee_id, due_at) WHERE kind = 'task' AND is_done = false AND archived_at IS NULL;
CREATE INDEX idx_activity_direction ON activity (workspace_id, direction, occurred_at DESC) WHERE direction IS NOT NULL AND archived_at IS NULL; -- reciprocity (§4) / lead-promotion (§2) inbound/outbound lookups
CREATE INDEX idx_activity_search  ON activity USING gin (search_tsv);
```

**ACT-DDL-2 — `activity_link`.** One activity ↔ many of {person, organization,
deal}; the per-endpoint indexes are what hold the 360-timeline join inside the
list budget, and the deal's `last_activity_at` is maintained from these rows on
write (deal DDL: [[deals-and-pipeline]]).

```sql
-- polymorphic links: one activity ↔ many of {person, organization, deal}
CREATE TABLE activity_link (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  activity_id   uuid NOT NULL REFERENCES activity(id) ON DELETE CASCADE,
  entity_type   text NOT NULL CHECK (entity_type IN ('person','organization','deal')),
  person_id       uuid NULL REFERENCES person(id) ON DELETE CASCADE,
  organization_id uuid NULL REFERENCES organization(id) ON DELETE CASCADE,
  deal_id         uuid NULL REFERENCES deal(id) ON DELETE CASCADE,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT activity_link_shape CHECK (
    (entity_type='person'       AND person_id IS NOT NULL AND organization_id IS NULL AND deal_id IS NULL) OR
    (entity_type='organization' AND organization_id IS NOT NULL AND person_id IS NULL AND deal_id IS NULL) OR
    (entity_type='deal'         AND deal_id IS NOT NULL AND person_id IS NULL AND organization_id IS NULL)
  )
);
CREATE UNIQUE INDEX uq_activity_link ON activity_link (activity_id, entity_type, coalesce(person_id,organization_id,deal_id));
CREATE INDEX idx_alink_person ON activity_link (person_id) WHERE person_id IS NOT NULL;
CREATE INDEX idx_alink_org    ON activity_link (organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_alink_deal   ON activity_link (deal_id) WHERE deal_id IS NOT NULL;
```

### Wire
Source: contract/crm.yaml (Activities tag) @ 5a0b29c

Operations are cited by operationId, never restated; envelopes, errors, and
pagination per [[api-conventions]].

| ID | operationId | Behavior pinned |
|---|---|---|
| ACT-WIRE-1 | `listActivities` | The timeline read: cursor-paginated, newest-first, filterable by kind / linked entity / assignee / full-text per the closed vocabulary [[data-model#DM-VOCAB-4]]. 🟢 search verb. |
| ACT-WIRE-2 | `logActivity` | Create (the `log_activity` verb, 🟢). A captured payload carries the capture key; replaying the same (source system, source id) answers 200 with the existing activity — never a duplicate ([[api-conventions#API-ERR-1]], ACT-DDL-1). |
| ACT-WIRE-3 | `getActivity` | One activity with its links and raw capture payload; out-of-scope reads answer as not-found ([[api-conventions#API-ERR-6]]). |
| ACT-WIRE-4 | `updateActivity` | Merge-patch semantics ([[api-conventions#API-ERR-3]]); the task-completion path (emits per ACT-EVT rows). |
| ACT-WIRE-5 | `archiveActivity` | Soft-delete; the entity returns with its archive timestamp set ([[api-conventions#API-ERR-4]]). |
| ACT-WIRE-6 | `relinkActivity` | Adds or moves one typed link in one idempotent action — replaying the same (activity, entity type, entity id) creates no duplicate link (ACT-DDL-2); original `source`/`captured_by` provenance is preserved; writes exactly one audit row; 🟢 (internal association, not outbound). |

Note ACT-WIRE-N-1: `draftEmail` and `sendEmail` are routed under an activity on
the wire but are the [[drafting]] chapter's operations (drafting 🟢, sending 🟡 +
consent-gated) — cited here only so the routing is unsurprising; nothing about
them is pinned in this chapter.

Note ACT-WIRE-N-2 (reconcile): the contract describes the relink audit row as
action `activity_relink`, a token the audit action check set ([[data-model]]
DM-DDL-8) does not currently include — one of the two must move; the audit action
vocabulary is the data-model chapter's to amend.

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c

Event definitions, payloads, and consumers live in the [[event-bus]] catalog —
cited, not redefined. This chapter's tables are the source of three event types.

| ID | Emitted when | Definition |
|---|---|---|
| `activity.captured` | Once per normalized captured activity — idempotent with the capture key (ACT-DDL-1); emitted by the capture module; drives deal last-activity maintenance, the stalled-deal sweep, and the manual-entry-smell metric. | [[event-bus]] catalog (Activity) |
| `activity.updated` | A human or agent edit — including a task's done transition and a human correction of a captured field (the delta carries the typed-by attribution). | [[event-bus]] catalog (Activity) |
| `activity.archived` | Soft-delete of an activity. | [[event-bus]] catalog (Activity) |

Note ACT-EVT-N-1 (reconcile): the feature corpus's acceptance wording names a
`task.completed` event; the ratified 43-event catalog has no such type — task
completion rides `activity.updated` with the done-state delta. The catalog is
authoritative; ACT-AC-5's event clause verifies against `activity.updated`.

### Acceptance
Source: features/01-core-objects.md#51-polymorphic-activity-model @ 5a0b29c

ACT-AC-1..8 are the feature doc's §5.1 acceptance bullets, wording verbatim (the
corpus leaves them unnumbered, so the IDs are new pins). ACT-AC-9..11 pin
behavioral guarantees those bullets imply but do not state.

| ID | Given/When/Then | Verification |
|---|---|---|
| ACT-AC-1 | `activity` is polymorphic over `{person, org, deal}` with typed FK links via the relational model (an activity can link to >1 entity — e.g. an email tied to a person *and* a deal). | Schema test on ACT-DDL-2 + integration test linking one activity to two entities |
| ACT-AC-2 | Captured activities store **raw JSONB** (re-parseable) **alongside** normalized columns; JSONB is **not** on the query hot path (§3.2). | Schema conformance + query-plan assertion ([[data-model#DM-CONV-11]]) |
| ACT-AC-3 | Every captured activity has `source` (which email/event id) + `captured_by` (which capture agent) — re-running capture is idempotent (same source id → no duplicate activity; unique constraint on `(source_system, source_id)`). | Capture-replay integration test against ACT-DDL-1 (`uq_activity_source`); provenance non-null via [[data-model#DM-AC-4]] |
| ACT-AC-4 | Timeline view (50 items, filtered) **p95 < 150 ms** server; full-text search over activities **p95 < 200 ms** (§3.5). | CI benchmark ([[acceptance-standards#PERF-2]], [[acceptance-standards#PERF-3]]) |
| ACT-AC-5 | A task transition to done writes one audit row + `task.completed` event. | Integration test: done transition → exactly one audit row + one `activity.updated` with the done-state delta (event name per note ACT-EVT-N-1) |
| ACT-AC-6 | L2 timeline summary: **first token < 1.5 s**, rendered async, never blocking the timeline load (§3.5). | RUM ([[acceptance-standards#PERF-5]]); the never-blocking clause is this chapter's — timeline load test with the summary stubbed slow |
| ACT-AC-7 | Auto-capture coverage is measurable: a workspace can report the % of activities `captured_by` agent vs human (the "manual entry is a smell" metric, P5). | Integration test computing the ratio on a mixed-provenance fixture |
| ACT-AC-8 | **User-observable:** the rep watches the timeline fill itself — emails and meetings they never logged appear on the right person and deal, each tagged with where it came from — so the common case is reading a captured history, not typing notes (S-E02.2, S-E02.6). | Verified end-to-end with the [[capture]] chapter's E02 stories (owned there); this substrate's contribution is ACT-AC-1..3 |
| ACT-AC-9 | Given an existing link, when the same relink (activity, entity type, entity id) is replayed, then no duplicate link exists and the response carries the unchanged link set. | Integration test on ACT-WIRE-6 against `uq_activity_link` (ACT-DDL-2) |
| ACT-AC-10 | Given a captured activity, when it is relinked to another entity, then its `source` and `captured_by` are byte-identical afterwards and exactly one audit row records the relink. | Integration test on ACT-WIRE-6 |
| ACT-AC-11 | Given a non-task kind, task fields cannot be persisted; given a task marked done, its completion timestamp is set. | Constraint tests on ACT-DDL-1 (`activity_task_fields`, `activity_done_at`) |
