---
status: planned
module: backend/internal/modules/activities (work-queue + proposal lifecycle); web (tasks surface)
derives-from:
  - specs/spec/product/epics/E16-tasks-and-work-queue.md
  - specs/spec/features/01-core-objects.md#5-activity-timeline
  - specs/spec/features/07-ai-native-moments.md#5-meeting--transcript-intelligence
  - specs/spec/product/30-screen-acceptance.md#taskshtml--tasks-no-numbered-story-s-e033e043-write-here
---
# Tasks & work queue — one queue for everything you owe, and nothing in it you didn't agree to

> The rep's personal to-do surface: a single queue of owned commitments spanning
> deal-linked and standalone work, plus the agent's proposed follow-ups waiting —
> visibly separate, evidence in hand — for an explicit accept, dismiss, or snooze.
> Its promise: nothing you owe is dropped, and nothing appears as yours that you
> never agreed to.

## What it's for

Real selling work constantly produces to-dos that fit no pipeline stage: a phone
call to make, a hallway promise, a nudge next week. Without one home for them,
they scatter into memory, sticky notes, and the incumbent tool the rep was
supposed to leave behind. This subsystem gives every user one personal work
queue — deal-linked and standalone tasks side by side, grouped by due window,
scoped to mine / my team / all — and it is where the agent's inferred follow-ups
surface as proposals. Its callers are the tasks surface (the rep's daily driver),
the deal and record views that show a record's open tasks, the agent verbs that
log tasks on a rep's behalf, and the reminder-delivery path. The boundary: the
task data model and timeline mechanics belong to
[activities-and-timeline](activities-and-timeline.md); the inference that
produces a proposed next step belongs to
[meetings-and-transcripts](meetings-and-transcripts.md). This chapter owns the
queue doctrine and the proposal lifecycle on it.

## Principles it serves

- **P5 — auto-capture over manual entry.** The queue is where extracted
  commitments and inferred follow-ups land, so the common case is reviewing work
  the system found rather than remembering to type it. The inverse case is also
  deliberate: a manually added task is first-class — a tracked, owned, audited
  commitment, not an untyped note (decision A11).
- **P11 — clean relational core.** A task is a kind of activity, not a new
  table or a metadata row ([[glossary]] "activity"); the queue is a read over the
  one activity model, so tasks inherit linking, search, and archive semantics for
  free.
- **P12 — governance is designed in.** An AI-proposed task is a proposal, not a
  row: evidence-or-omit governs what may be shown ([[acceptance-standards#GATE-AI-1]]),
  accept-to-persist governs what may be written ([[acceptance-standards#GATE-AI-2]]),
  and every accept, dismiss, and snooze is an audited, attributable decision.
- Light provenance: the epic exists because the story-ticket red-team found the
  prototype's task surface mapped to no numbered story (decision A49).

## How it works

**One queue.** The surface presents a single personal queue: every open task
assigned to the viewer, whether it is linked to a deal, a person, an
organization — or to nothing at all. Standalone tasks are not second-class;
they live in the same groups, sort by the same due dates, and carry the same
ownership and audit semantics as deal-linked ones. Tasks group by due window —
Overdue, Today, This week, Done — with empty groups omitted and overdue work
visually urgent (AC-tasks-1). Each row shows its title, a source badge saying
whether a human or the agent originated it, the assignee, the due date, an
optional reminder indicator, and — when linked — the record it belongs to
(AC-tasks-2).

**Owned commitments.** Adding a task takes one line of typing: a title, with
optional due date, reminder, and assignee, defaulting to the creator
(AC-tasks-4). Creation is attributed in history with exactly one audit row — a
manual task is a tracked commitment, not a scratchpad entry. Toggling a task
done moves it to the done group and is reversible (AC-tasks-3). A due date the
rep sets is a *commitment* and is kept semantically and visually distinct from
the system-computed stalled-deal health signal — one is a promise the rep made,
the other is an observation the system made, and the surface never conflates
them (AC-tasks-7). The reminder is a quiet nudge at a chosen time, not a second
due date; the reminder timestamp is a planned addition to the task kind's fields
and rides the activity table, whose schema is owned by
[activities-and-timeline](activities-and-timeline.md) (see the ownership index
in the data-model chapter, [[data-model#schema--ownership-index]]).

**Scope, honestly enforced.** The scope control switches the queue between mine,
my team, and all — always within the viewer's permissions (AC-tasks-6). Scoping
is a server-side property of the response, not a client-side filter: tasks the
viewer may not see are absent from the payload, never merely hidden
([[acceptance-standards#STATE-4]]).

**Proposals are not rows.** When the agent infers a follow-up from captured
activity — a transcript line, an email thread — it arrives on the queue as a
*proposed* task: visibly badged as not yet the rep's, carrying an evidence chip
that cites the exact source it was derived from, and a confidence. Under
evidence-or-omit, an inference with no citable grounding is simply not shown
([[acceptance-standards#GATE-AI-1]]). Until the rep acts, the proposal has
produced zero rows in real domain tables and written no record field
([[acceptance-standards#GATE-AI-2]]); it is never counted as a commitment, an
overdue item, or a missed task. The inference itself — how a next step is read
out of a conversation — is the
[meetings-and-transcripts](meetings-and-transcripts.md) chapter's; this chapter
begins where the proposal reaches the queue.

**Accept, dismiss, snooze — all first-class, all audited.** Accepting a
proposal is the moment it becomes real: a task owned by the accepter, logged as
theirs, entering the queue with full commitment semantics — and with capture
provenance that permanently distinguishes a proposed-then-accepted task from a
human-typed one, pointing back at the evidence it came from
([[acceptance-standards#GATE-CORE-3]]). Dismissing removes the proposal and is
remembered: the same inference does not re-propose the same thing. Snoozing
returns the proposal at the chosen time, still marked proposed. Each of the
three decisions is attributable in the audit trail — who decided, on what
evidence — the same provenance spine as the deal-view next-step card.

**The gap the build must close.** The corpus flags that the prototype's tasks
screen rendered agent-originated tasks as if already accepted — no evidence
chip, no explicit accept, dismiss, or snooze control — which the proposal
doctrine forbids. That correction is carried as a pinned build note
(TASK-NOTE-1): every proposed row on the built surface must show its evidence
chip and its explicit three-way control before it can count as anyone's.

## What's configurable

- **Nothing, deliberately.** The due-window grouping, the scope vocabulary, and
  the proposal lifecycle are the screen contract and the trust mechanic — fixed
  behaviour, not tunables. This chapter pins no Parameters rows; if snooze
  durations or reminder lead times become configurable, they will be pinned here
  first.
- **The proposal producer** — an injected dependency on the inference the
  [meetings-and-transcripts](meetings-and-transcripts.md) chapter owns. When it
  is absent or degraded, the queue is fully functional as a purely human task
  list: the V1-Must story stands alone, and the surface shows no proposals
  rather than ungrounded ones.
- **Reminder delivery** — the channel that fires a due reminder rides the
  notification path (see
  [notifications-and-approval-inbox](notifications-and-approval-inbox.md));
  without it, reminders remain visible as in-queue indicators.

## Guarantees (enforced)

- **Zero rows before accept.** A proposed task, however confident, creates no
  task row and writes no record field until a human accepts it; inspecting the
  domain tables mid-proposal finds nothing
  ([[acceptance-standards#GATE-AI-2]], pinned as TASK-AC-1).
- **Evidence or omission.** Every rendered proposal carries a non-empty
  evidence citation and a confidence; an ungrounded inference is absent, never
  shown bare ([[acceptance-standards#GATE-AI-1]], TASK-AC-2).
- **Provenance forever.** An accepted proposal and a human-created task remain
  distinguishable for the life of the row, and the accepted row's provenance
  resolves to its evidence ([[acceptance-standards#GATE-CORE-3]], TASK-AC-3).
- **Dismissal is memory.** A dismissed proposal is not re-proposed; the
  dismissal is an audited decision, not a UI vanish (TASK-AC-4).
- **A proposal is never a commitment.** Unaccepted proposals are excluded from
  overdue and missed-task accounting (TASK-AC-6).
- **Every lifecycle step is audited once.** Creating, completing, reopening,
  accepting, dismissing, and snoozing each leave exactly one attributable audit
  row.
- **Scope is a server property.** The mine / my team / all control never
  reveals a task the viewer's permissions deny; denied rows are absent from the
  response ([[acceptance-standards#STATE-4]]).

## Acceptance

Done means: a rep opens one queue and sees everything they owe — deal-linked and
standalone together, grouped by due window, each row saying who owns it, when it
is due, and where it came from; adding, completing, and reopening tasks behaves
as an owned, audited commitment; proposals appear visibly not-yet-mine with
their evidence, and accept, dismiss, and snooze each do exactly what they say
and leave a trace. The honest states — an empty queue, a loading queue, a
failed load, a denied scope, and a nothing-grounded proposal panel — render per
the standard screen-state floor inherited from the acceptance-standards chapter
(STATE-1 through STATE-5) and are not restated here. The testable form of every
claim lives in the Acceptance appendix.

## Out of scope

- **The task data model.** A task is an activity of kind task; the activity
  table's DDL, constraints, indexes, capture idempotency, and timeline mechanics
  are owned by [activities-and-timeline](activities-and-timeline.md) (ownership
  index: [[data-model#schema--ownership-index]]). This chapter owns no tables —
  the planned reminder-timestamp column also rides that table and lands there.
- **Next-step inference.** How a follow-up is read out of a transcript or
  thread, and the transcript-to-proposal staging, belong to
  [meetings-and-transcripts](meetings-and-transcripts.md).
- **Approval mechanics.** The token, single-use, and re-validation machinery
  behind any confirm-first commit is
  [approvals-and-concurrency](approvals-and-concurrency.md); the inbox surface
  where staged actions are decided in bulk is
  [notifications-and-approval-inbox](notifications-and-approval-inbox.md).
- **Reminder delivery channels** and notification preferences —
  [notifications-and-approval-inbox](notifications-and-approval-inbox.md).
- **The permission model** the scope control obeys —
  [access-and-admin](access-and-admin.md).

## Where it lives

Planned backend home: `backend/internal/modules/activities` — the work-queue
read and the proposal-lifecycle use cases over the activity store, reached
through the datasource port like every record surface. Planned frontend home:
the tasks surface in `web`. Read
[activities-and-timeline](activities-and-timeline.md) for the model,
[meetings-and-transcripts](meetings-and-transcripts.md) for where proposals are
born, and [approvals-and-concurrency](approvals-and-concurrency.md) for how an
accept commits.

## Appendix

### Wire
Source: contract/crm.yaml (Activities + Approvals tags) @ 5a0b29c

The contract has **no dedicated task paths**: tasks ride the activity
operations (the polymorphic activity surface covers the task kind end to end),
and the proposal lifecycle rides the approvals surface. Honest coverage report,
including the contract-extension needs this chapter surfaces for docs-layer
resolution (per D-H2, contract ships complete — these are drift to reconcile,
not silent additions):

| ID | Operation (operationId) | Role in this chapter |
|---|---|---|
| TASK-WIRE-1 | `listActivities` | The queue read: `kind=task` + `assignee_id` ("Open tasks for an assignee") + entity filters + `q`; due-window grouping is a read-model/client concern, not a wire shape. |
| TASK-WIRE-2 | `logActivity` | Create a task (`kind=task`, `due_at`/`assignee_id`; `links` optional — standalone tasks are contract-legal). 🟢 `log_activity` MCP verb; the agent `create_task` verb rides it. |
| TASK-WIRE-3 | `getActivity` | Task detail incl. links and raw capture payload. |
| TASK-WIRE-4 | `updateActivity` | Complete/reopen (`is_done`), due/assignee edits; per-kind field constraints enforced server-side to match the DB `activity_task_fields` CHECK. |
| TASK-WIRE-5 | `archiveActivity` | Soft-delete a task. |
| TASK-WIRE-6 | `relinkActivity` | Re-associate a task to a chosen record, idempotent and source-preserving. |
| TASK-WIRE-7 | `listApprovals` / `getApproval` | Proposed-task list/detail: staged 🟡 items carrying proposed change + evidence + confidence. The `kind` filter is an open string; a task-proposal kind is not among the enumerated examples (coldstart, send_email, advance_deal, overnight). |
| TASK-WIRE-8 | `approveApproval` | Accept: commits the (optionally edited) proposal in one audit transaction, minting the approval token. |
| TASK-WIRE-9 | `rejectApproval` | Dismiss: discards the proposal; nothing commits. |

Contract-extension needs (D-H2 docs-layer drift to resolve; traceability already
decomposes S-E16.1/.2 into B-E16.1–.9 expecting these):

| ID | Gap | Detail |
|---|---|---|
| TASK-GAP-1 | Reminder field | No `remind_at` (or similar) exists on the `Activity` / `CreateActivityRequest` / `UpdateActivityRequest` schemas nor in the corpus activity DDL; build atom B-E16.1 (product/20-traceability.md) adds it. The column rides the `activity` table owned by activities-and-timeline ([[data-model#schema--ownership-index]]); the contract needs the matching field. |
| TASK-GAP-2 | Snooze | Approvals expose approve/reject only; `status` enum is `pending, approved, rejected` — no snooze operation, no snoozed/return-at state. S-E16.2 makes snooze first-class; contract extension required. |
| TASK-GAP-3 | Dismissal memory | "Dismiss does not re-propose the same thing" (S-E16.2) has no wire or schema surface — the no-re-propose memory needs a specified home. |
| TASK-GAP-4 | Reminder delivery | No operation or event exists for a due reminder firing (B-E16.6 reminder delivery). |

### Events
Source: contract/events.md#55-activity @ 5a0b29c (activity events; approval pair per its §5.6)

| ID | Event | Role in this chapter |
|---|---|---|
| TASK-EV-1 | `activity.captured` | Emitted once when a task row materializes — human-created or accepted-proposal — with `captured_by` distinguishing the actor (the P5 manual-entry-smell metric input). |
| TASK-EV-2 | `activity.updated` | Task lifecycle deltas: done/reopen (`is_done`), due/assignee edits, human corrections (`captured_by=human:*`). |
| TASK-EV-3 | `activity.archived` | Task soft-deleted. |
| TASK-EV-4 | `approval.requested` | A proposed task staged 🟡 with proposed effect, evidence, dry-run diff, expiry. |
| TASK-EV-5 | `approval.decided` | Accept/dismiss decision: `approved`/`rejected`/`expired`, decider, optional edited effect, approval token; on approve, the resulting TASK-EV-1 shares its correlation id. |

Catalog drift (honest report): features/01 §5 acceptance and the contract's
`updateActivity` description both promise a `task.completed` event, but the
central event catalog defines no such ID — completion rides `activity.updated`.
Either the catalog gains `task.completed` or the two sources reconcile to the
delta event (docs-layer resolution, same D-H2 lane as TASK-GAP-1..4). No
reminder-due event exists (TASK-GAP-4).

### Acceptance
Source: product/epics/E16-tasks-and-work-queue.md @ 5a0b29c; product/30-screen-acceptance.md#taskshtml--tasks-no-numbered-story-s-e033e043-write-here @ 5a0b29c

Story primacy verified against product/20-traceability.md @ 5a0b29c: S-E16.1
(V1-Must) and S-E16.2 (V1-WOW) are owned here; the scope chapter maps epic E16
to this chapter alone. AC-tasks-1..7 wire to S-E16.1 per the corpus screen
section.

| ID | Given/When/Then | Verification |
|---|---|---|
| S-E16.1 | Given the tasks surface, when I open it, then I see one queue of my tasks grouped by due window — deal-linked and standalone together — each with title, optional due date, optional reminder, owner, and linked record; adding a title creates a task assigned to me by default, attributed with one audit row; toggling done is reversible; a due date I set is a commitment, distinct from the system stalled-deal signal; the mine / my team / all control re-scopes within my RBAC. | Ticket-coverage gate; integration lane ([[testing#TEST-LANE-2]]) + live-stack UAT ([[testing#TEST-LANE-3]]). |
| S-E16.2 | Given captured activity, when the agent infers a follow-up, then it appears as a proposed task carrying its evidence and confidence (ungrounded → not shown); accepting makes it a real task owned and logged as mine; dismissing removes it without re-proposal; snoozing returns it at the chosen time; an unacted proposal is visibly not mine and never counted as a commitment; every accept/dismiss/snooze is audit-attributable with its evidence. | Ticket-coverage gate; integration lane ([[testing#TEST-LANE-2]]); gates [[acceptance-standards#GATE-AI-1]] + [[acceptance-standards#GATE-AI-2]]. |
| AC-tasks-1 | Given the tasks page, When it loads, Then tasks are grouped Overdue / Today / This week / Done, each with icon, label, count; empty groups omitted; Overdue in red. | Live-stack UAT ([[testing#TEST-LANE-3]]); screen-state floor [[acceptance-standards#STATE-1]]–[[acceptance-standards#STATE-4]]. |
| AC-tasks-2 | Given each task row, When it renders, Then it shows a checkbox, title, a source badge ("You" = human, "AI" = agent-proposed), assignee with avatar, a due date (red when late), an optional reminder indicator, and an optional link to the related record. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-tasks-3 | Given a task checkbox, When clicked, Then the task toggles done (strikethrough, moves to Done, toast); toggling a done task re-opens it. | Live-stack UAT ([[testing#TEST-LANE-3]]); emits TASK-EV-2. |
| AC-tasks-4 | Given the new-task card, When the user types a title and clicks Add (or Enter), Then a task is created in Today, assigned to "You" (toast); an empty title refocuses instead of adding. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| AC-tasks-5 | Given the new-task card, When Due / Remind / Assign are clicked, Then each opens its respective picker. | Live-stack UAT ([[testing#TEST-LANE-3]]); pickers are new build (unimplemented in the prototype per the corpus screen section). |
| AC-tasks-6 | Given the scope control (Mine / My team / All), When switched, Then the active scope updates with a toast. | Live-stack UAT ([[testing#TEST-LANE-3]]); server-side scoping asserted in integration lane ([[testing#TEST-LANE-2]]), TASK-AC-7. |
| AC-tasks-7 | Given the source legend, When the page renders, Then it explains "You" vs "AI" (accepting an AI task logs it as yours) and the reminder indicator, AND distinguishes a user-set "due date" (a commitment) from the system "Stalled Nd" deal-health signal. | Live-stack UAT ([[testing#TEST-LANE-3]]). |
| TASK-AC-1 | Given a proposed task not yet accepted, when domain tables are inspected, then no task-kind activity row exists for it and no record field has been written. | Integration lane ([[testing#TEST-LANE-2]]); [[acceptance-standards#GATE-AI-2]]. |
| TASK-AC-2 | Given an inferred follow-up with no citable evidence, when the queue renders, then that proposal is absent; every rendered proposal carries a non-empty evidence citation and a confidence. | Integration lane ([[testing#TEST-LANE-2]]); [[acceptance-standards#GATE-AI-1]] / [[acceptance-standards#STATE-5]]. |
| TASK-AC-3 | Given an accepted proposal and a human-created task, when their rows are read, then capture provenance permanently distinguishes proposed-accepted from human-typed, and the accepted row's provenance resolves to the evidence it was derived from. | Integration lane ([[testing#TEST-LANE-2]]); [[acceptance-standards#GATE-CORE-3]]. |
| TASK-AC-4 | Given a dismissed proposal, when the same inference recurs over the same evidence, then it is not re-proposed; the dismissal is audited with decider and evidence reference. | Integration lane ([[testing#TEST-LANE-2]]); memory surface pending TASK-GAP-3. |
| TASK-AC-5 | Given a snoozed proposal, when the chosen time arrives, then it returns to the queue still marked proposed and still uncounted; the snooze is audited. | Integration lane ([[testing#TEST-LANE-2]]); wire surface pending TASK-GAP-2. |
| TASK-AC-6 | Given an unacted proposal whose suggested due date has passed, when overdue and missed-task accounting runs, then the proposal is excluded — only accepted tasks count as commitments. | Integration lane ([[testing#TEST-LANE-2]]). |
| TASK-AC-7 | Given a viewer whose permissions deny a task, when any scope of the queue is requested, then that task is absent from the response payload, not merely hidden. | Integration lane ([[testing#TEST-LANE-2]]) RBAC matrix; [[acceptance-standards#STATE-4]]. |
| TASK-NOTE-1 | Build note (pinned): the prototype's tasks screen rendered AI tasks as already-accepted — no evidence chip, no explicit accept/dismiss/snooze — which S-E16.2 forbids. The built surface must add the evidence chip + explicit accept/dismiss/snooze control on every proposed row (corpus flags: 30-screen-acceptance §2 tasks section "AI-native mechanics present" + §4 UI notes). | Ticket-coverage gate; UI review against [[acceptance-standards#GATE-AI-2]]. |
