---
status: skeleton
module: backend/internal/platform
derives-from:
  - margince-poc/docs/subsystems/audit-observability.md
  - margince specs/spec/contract/data-model.md#11-audit-log
  - margince specs/spec/narrative/06-nonfunctional.md#66-observability
---
# Audit & observability — the wall every mutation crosses

> The governance wall: every mutation and every agent action is attributable,
> replayable, and observable. One write seam owns audit, the log is append-only and
> fails loud, a merge-blocking gate proves nothing slips past it, and structured
> logs, metrics and traces make the running system legible.

## What it's for

Two jobs. First, **attribution and replay**: there is exactly one place mutations
get recorded, so every change to a core record carries who did it — human, agent,
system or connector — on whose behalf, and what changed; agent actions additionally
capture their inputs, tool calls, outputs and approval state so a decision can be
reconstructed after the fact. Second, **observability**: every service emits
structured logs, metrics and end-to-end traces so an operator can see latency,
queue depth, and one request as it crosses the async boundary. Every module that
writes domain data is a caller of the audit seam; operators and the anomaly
detection described by the threat model (D6) are the readers. The observability
*doctrine* lives here; the concrete log schema, metric registry and exposition
rules are owned by the operations chapter and cited, never restated.

## Principles it serves

- **P12 — governance is designed in.** Audit trails, decision provenance and
  replayable agent traces are core primitives here, not a retrofit — the
  structural answer to AI-Act/GDPR obligations for an agentic system.
- **P5 — auto-capture over manual entry.** A manual-entry-smell measure surfaces
  how much data a human had to type versus what was captured automatically
  (AUD-AC-8); fixture data is excluded from it by provenance ([[testing#TEST-DET-4]]).
- **P4 — blazing fast, always.** The audit row is atomic with the mutation, but
  the downstream event publish is post-commit and off the hot path
  ([[event-bus#EVT-DEL-5]]); the metrics exist precisely to defend the latency
  budgets.

## How it works

- **One write seam.** Audit rows are written in exactly one place, which owns the
  attribution logic: it shapes who-did-what from the request principal — a human
  leaves the agent passport empty, an agent records its passport and the granting
  human as "on behalf of", and no principal means a system write. Each mutation
  writes exactly one audit row inside the caller's own transaction, so the record
  and its audit commit or roll back together — and the seam never swallows an
  error.
- **Append-only log.** The audit log is tenant-isolated and immutable: any attempt
  to update or delete a row raises and aborts the transaction, so tampering fails
  loudly rather than appearing to succeed. The table, trigger and index shapes are
  owned by the data model ([[data-model#DM-DDL-8]]); the privileged erasure path is
  the only principal allowed past the trigger, and every scrub is itself logged.
- **One mutation, one audit row, one event.** Each audit row enqueues exactly one
  appended-audit event through the transactional outbox, published post-commit by
  the relay and idempotent on the audit row it points at
  ([[event-bus#EVT-SEM-1]], [[event-bus#EVT-SEM-12]]). If emission fails, the
  committed mutation is never rolled back — the audit row is the durable record,
  the event only the notification.
- **Agent-trace capture.** A producer-agnostic ingest records an agent action's
  inputs, ordered tool calls, outputs and approval state, linked to its audit row
  by a stable trace id — it works without any agent runner present, so a recorded
  fixture ingests fine. Deterministic replay is driven by the agent runtime, which
  is a planned feature; the trace store and its linkage ship now.
- **Observability doctrine.** Logs are structured JSON carrying the pinned base
  field set — trace, correlation, actor and module identity on every line, with
  mandatory PII redaction ([[operations#OPS-LOG-12]], [[operations#OPS-LOG-13]]).
  Traces span the async hops unbroken: the trace context is seeded at the request,
  carried on the event envelope, and recovered by the consumer
  ([[operations#OPS-LOG-14]]). The metric registry and its exposition surface are
  pinned by operations ([[operations#OPS-MET-1]]..7, [[operations#OPS-MET-8]]).

## What's configurable

- **The log handler** is chosen at service startup; structured JSON to standard
  output is the default, per the operations doctrine ([[operations#OPS-LOG-12]]).
- **Metric labels are constrained by rule, not convention** — only bounded,
  low-cardinality dimensions; never a workspace or entity identifier
  ([[operations#OPS-MET-9]]).
- The audit seam, the immutability trigger and the coverage gate are
  infrastructure, not tunable surface — they are the guarantees below.

## Guarantees (enforced)

- **No unaudited mutation path merges.** A merge-blocking gate asserts there is no
  audit-log insert outside the seam and that every core-table writer references
  the seam; exceptions (the seam itself, migrations, system writers) are explicit,
  never silent. This app-layer wall pairs with the data-layer immutability
  trigger — two walls ([[quality-gates#QG-11]], [[data-model#DM-DDL-8]]).
- **Append-only fails loud.** Updates and deletes against the audit log raise and
  abort the transaction; never a silent no-op ([[data-model#DM-DDL-8]], restating
  DM-AC-5).
- **Exactly one audit row per mutation, exactly one event per audit row** — no
  double-write, no zero-write ([[event-bus#EVT-SEM-1]]).
- **Agent vs human attribution is structural.** Agent rows carry the passport and
  the on-behalf-of human; human rows leave the passport empty; the authorizing
  passport lives on the audit row, not on domain rows ([[data-model#DM-CONV-11]]).
- **The event is post-commit and idempotent**, and emission failure never rolls
  back the mutation ([[event-bus#EVT-DEL-5]], [[event-bus#EVT-SEM-12]]).
- **The audit vocabulary stays coherent** — the action/actor vocabulary in the
  database matches the contract, gate-enforced ([[quality-gates#QG-12]]).
- **Metrics carry no high-cardinality labels** ([[operations#OPS-MET-9]]); **logs
  never leak PII in the clear** ([[operations#OPS-LOG-13]]).
- **Trace capture is producer-agnostic** — every agent action is replayable to its
  passport from the stored trace, without the runner present (threat model D6).

## Acceptance

Done means: an operator can point at any changed record and read who changed it,
under which authority, and what the diff was; can point at any agent action and
replay its inputs, tool calls and outputs against its passport; and can watch the
system's latency, queue depth and a single cross-service trace without grepping
free text. Tamper attempts fail loudly, and the coverage gate being green is the
proof that no unaudited write path exists. The testable form of each claim is
pinned in the Acceptance appendix; the cross-cutting floor is inherited from the
acceptance-standards chapter.

## Out of scope

- The log field schema, metric registry, exposition endpoint and DR doctrine —
  owned by the operations chapter ([[operations#OPS-LOG-1]]..14,
  [[operations#OPS-MET-1]]..9).
- The audit table DDL, trigger and provenance conventions — owned by the data
  model ([[data-model#DM-DDL-8]], [[data-model#DM-CONV-11]]).
- The event envelope, catalog, streams and delivery semantics — owned by the
  event-bus chapter.
- Anomaly detection on the audit stream (unusual read volume, off-hours export) —
  a threat-model control (D6) whose detection surfaces are planned features.

## Where it lives

<!-- 1c-mapping: final call pending -->
The audit write seam and the observability plumbing live in the platform module
directory (backend/internal/platform); every domain module reaches audit through
that one seam, and services wire the log handler, metrics and tracing at
composition time. Read next: [data-model](../architecture/data-model.md) for the
table the seam writes, [event-bus](../architecture/event-bus.md) for what each
audit row emits, [operations](../architecture/operations.md) for what the running
system must expose, and [gdpr-platform](gdpr-platform.md) for the compliance
machinery built on this wall.

## Appendix

### Events
Source: margince-poc/docs/subsystems/audit-observability.md#how-it-works @ a11d6c08

| ID | Direction | Event | Owner of definition |
|---|---|---|---|
| AUD-EVT-1 | emitted | `audit.appended` — thin pointer to the audit row, idempotent, never emitted for non-mutating reads | [[event-bus#EVT-SEM-12]]; stream [[event-bus#EVT-STREAM-9]] |

### Acceptance
Source: margince-poc/docs/subsystems/audit-observability.md#guarantees-enforced-not-aspirational @ a11d6c08

| ID | Given/When/Then | Verification |
|---|---|---|
| AUD-AC-1 | Given any existing audit row, when an UPDATE or DELETE is attempted, then the statement raises and the transaction aborts; the row persists unchanged (append-only by trigger, [[data-model#DM-DDL-8]]). | trigger integration test (the DM-AC-5 shape) |
| AUD-AC-2 | Given the full backend source, when the audit-coverage gate runs, then it is green only if no audit-log insert exists outside the seam and every core-table writer references the seam — green means no unaudited mutation path. | merge-blocking CI gate [[quality-gates#QG-11]] |
| AUD-AC-3 | Given one domain mutation, when it commits, then exactly one audit row exists for it and exactly one appended-audit event is outboxed for that row ([[event-bus#EVT-SEM-1]]); a rolled-back mutation produces neither. | integration test over a representative mutation |
| AUD-AC-4 | Given a human, an agent, and a system principal each performing a write, then the human row has an empty passport, the agent row carries passport + on-behalf-of, and the principal-less write is attributed to system. | seam unit + integration tests |
| AUD-AC-5 | Given a recorded agent-action fixture and no agent runner present, when it is ingested, then the trace (inputs, ordered tool calls, outputs, approval state) links to its audit row by stable trace id and is replayable to its passport (threat model D6). | fixture-driven integration test |
| AUD-AC-6 | Given a committed mutation whose event publish fails, then the mutation and its audit row remain committed; on retry the event is deduplicated (idempotent on the audit row). | outbox/relay integration test ([[event-bus#EVT-DEL-5]]) |
| AUD-AC-7 | Given the deployed schema and the contract, when the coherence gate runs, then the audit action/actor vocabulary in the database matches the contract. | CI gate [[quality-gates#QG-12]] |
| AUD-AC-8 | Given captured and hand-entered records distinguished by provenance ([[data-model#DM-CONV-11]]), then the manual-entry-smell measure exists and is computed correctly from provenance, with fixture rows excluded ([[testing#TEST-DET-4]]); the gate is presence and correctness, not any particular value. | metric presence + correctness test |
