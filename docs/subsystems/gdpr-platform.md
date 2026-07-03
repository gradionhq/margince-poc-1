---
status: skeleton
module: backend/internal/modules/gdpr
derives-from:
  - margince-poc/docs/subsystems/gdpr.md
  - margince specs/spec/contract/data-model.md#34-consent--retention
---
# GDPR platform — consent that defaults to no, and erasure that stays erased

> The compliance substrate under the data-subject rights: per-purpose consent with
> an append-only proof log, retention policy as per-workspace data, legal hold, and
> the suppression semantics that keep an erased subject erased — even through a
> backup restore. The tables and their semantics ship now; the workflows built on
> them are a planned sibling chapter.

## What it's for

GDPR obligations are only as strong as the substrate they stand on. This subsystem
is that substrate: it answers "may we do X to this person?" with a default of no
and an evidence trail for every yes; it holds the per-workspace retention schedule
a sweep will enforce; it marks records that litigation pulls out of any expiry
clock; and it remembers — as salted hashes, never plaintext — which identities
were erased, so nothing re-creates them. Callers are every outbound or profiling
surface (consent checks), the single capture writer (the suppression guard), the
future retention worker (the policy rows), and the restore procedure (the
suppression list). The scope boundary is deliberate: this chapter owns the
platform half — tables, seeds, triggers, semantics; the user-facing workflows
(subject-access export, erasure UI, the sweep wiring, the compliance pack) are
owned by the planned gdpr-compliance-surfaces chapter and return by ticket.

## Principles it serves

- **P12 — governance is designed in.** Consent, retention, legal hold and
  suppression are first-class schema with their own seams — the structural answer
  to GDPR for an agentic CRM, not a compliance retrofit.
- **P11 — clean relational core.** Compliance state is real rows with real
  constraints — a current-state row per person/purpose, an append-only proof
  event, a policy row per retention rule — never interpreted metadata.
- **ADR-0042 — jurisdiction packs, never core.** The seeded defaults are
  jurisdiction-neutral; country-specific ladders layer above through the
  jurisdiction seam, gate-enforced ([[quality-gates#QG-10]]).

## How it works

- **Consent is a mutable state plus an append-only proof.** Each person/purpose
  pair has one current state that flips between granted, withdrawn and unknown,
  and every change also appends an immutable proof event carrying the verbatim
  policy wording and version shown at the time. The check reads current state
  only, is strictly per-purpose, and defaults to deny: missing, unknown and
  withdrawn all read as no, and a grant for one purpose never satisfies a check
  for another. Capture surfaces must pass the purpose and wording through — they
  may not synthesize a blanket grant. The table shapes are owned by the data model
  ([[data-model#DM-DDL-10]]).
- **Retention is policy-as-data.** Each workspace holds enabled policy rows —
  object type, optional category, retain-days, and one action per row on an
  archive→anonymize→erase ladder — seeded with jurisdiction-neutral defaults
  ([[data-model#DM-SEED-1]]..5) and editable per tenant. The nightly evaluator
  that walks these rows is a planned feature; the rows, their uniqueness rules and
  the semantics it must honor ship now.
- **Legal hold beats the schedule.** A per-record hold flag on the core objects
  pulls a record out of any retention action at the query level, so holds survive
  the expiry clock ([[data-model#DM-DDL-10]]). Setting or clearing a hold is an
  audited mutation; the surface that does so is planned.
- **Erasure leaves a suppression record, and suppression is forever.** Erasure
  itself is a privileged, separately audited path whose tombstone carries no PII
  ([[data-model#DM-DDL-8]]); the workflow that drives it is planned. What ships
  here is the memory that outlives it: erased identities are recorded as salted
  email hashes on a tenant-isolated suppression list (GDPR-DDL-1), the single
  capture writer must consult that list before upserting a person (the guard
  concept; the writer itself lands with the capture chapter), and a restored
  backup re-applies the list so erased PII cannot resurrect from a backup
  ([[operations#OPS-DR-6]]).
- **Everything crosses the audit wall.** Every consent change, retention action,
  hold change and erasure is exactly one audit row through the one write seam,
  emitting exactly one event ([[event-bus#EVT-SEM-1]]; see
  [audit-observability](audit-observability.md)).

## What's configurable

- **Retention policies are data, per workspace** — seeded compliant out of the
  box ([[data-model#DM-SEED-1]]..5), editable per tenant; ladders compose as
  separate rows at increasing ages.
- **Consent purposes are a small seeded reference set** (GDPR-SEED-1..4); checks
  are always by purpose, never blanket.
- **Legal hold is a per-record flag**, set and cleared through an audited
  mutation.
- Nothing else is tunable: the default-deny rule, the append-only proof trigger
  and the suppression semantics are guarantees, below.

## Guarantees (enforced)

- **Consent defaults to deny, falsifiably.** Unknown, withdrawn and no-row all
  deny, and a grant for one purpose never satisfies another — pinned by the
  named fixtures the test suite ships (`consent-default-deny` and
  `consent-withdrawn` in the [[testing]] fixture catalog).
- **The consent proof log is append-only and fails loud** — updates and deletes
  raise and abort the transaction, the same wall the audit log uses
  ([[data-model#DM-DDL-10]], mirroring [[data-model#DM-DDL-8]]).
- **Suppressed rows never resurface** — the suppression list is append-only in
  practice (insert-and-read only for the application role), the capture writer
  must reject a suppressed identity, and a restore re-applies the list
  ([[operations#OPS-DR-6]]), proven on the restore-drill cadence
  ([[operations#OPS-DR-3]]).
- **Retention skips legal hold at the query level**, never by caller discipline
  ([[data-model#DM-DDL-10]]; the `retention-expiry` fixture pins the skip).
- **Every table here is tenant-isolated** with forced row-level security and the
  workspace setting per transaction ([[data-model#DM-CONV-5]]..8).
- **Seeded defaults are jurisdiction-neutral** — no country strings in core,
  gate-enforced ([[quality-gates#QG-10]], ADR-0042).

## Acceptance

Done means: a check for any purpose against a person with no recorded grant comes
back no; every consent flip leaves an immutable proof row that survives tamper
attempts; a new workspace wakes up with the neutral retention defaults and the
seeded purposes already present; and an identity that was erased cannot be
re-created by capture, import, or a database restore. The testable form of each
claim lives in the Acceptance appendix; the cross-cutting floor is inherited from
the acceptance-standards chapter.

## Out of scope

Dropped from the skeleton, planned to return by ticket in
[gdpr-compliance-surfaces](gdpr-compliance-surfaces.md): the subject-access (SAR)
export flow (scope decision open, [[threat-model#TM-DPIA-6]]), the erasure
workflow and its UI, the nightly retention worker wiring, and the DPA/compliance
pack. The request-tracking table belongs to that chapter too (see the data-model
ownership index). The capture writer that enforces the suppression guard is owned
by the planned capture chapter; the trust-tier and Art. 9 handling of transcript
content is owned by the threat model ([[threat-model#TM-DPIA-2]]).

## Where it lives

The gdpr module directory (backend/internal/modules/gdpr) owns the consent,
retention and suppression substrate; its tables land in the platform migration
block and their DDL is pinned by the data model ([[data-model#DM-DDL-10]], plus
GDPR-DDL-1 below). Callers reach it through the consent-check and suppression
seams. Read next: [data-model](../architecture/data-model.md) for the DDL,
[audit-observability](audit-observability.md) for the wall every action here
crosses, and [operations](../architecture/operations.md) for the restore rules.

## Appendix

### Schema — erasure suppression list
Source: margince-poc/infra/migrations/000023_retention_legal_hold.up.sql @ a11d6c08

GDPR-DDL-1 — the suppression list erasure writes and capture/restore read. Not
among the corpus's 66-table partition; pinned here as its single home. Salted
email hashes only — the list must never contain plaintext PII. The application
role can select and insert, never update or delete.

```sql
CREATE TABLE erasure_suppression (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  email_hash   text NOT NULL,
  reason       text NOT NULL DEFAULT 'gdpr_erasure',
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, email_hash)
);
CREATE INDEX idx_erasure_suppression_ws ON erasure_suppression (workspace_id);
-- ENABLE + FORCE ROW LEVEL SECURITY with the standard tenant-isolation policy
-- (DM-CONV-8); GRANT SELECT, INSERT only to the application role.
```

### Acceptance
Source: margince-poc/docs/subsystems/gdpr.md#guarantees-enforced-not-aspirational @ a11d6c08

| ID | Given/When/Then | Verification |
|---|---|---|
| GDPR-AC-1 | Given a person with no consent row, an `unknown` row, or a `withdrawn` row for a purpose, when consent is checked for that purpose, then the answer is deny in all three cases — default-deny is falsifiable, not assumed. | unit + integration tests on the `consent-default-deny` and `consent-withdrawn` fixtures ([[testing]] named fixtures) |
| GDPR-AC-2 | Given a `granted` state for purpose A, when purpose B is checked for the same person, then the answer is deny — cross-purpose isolation. | consent-check unit test |
| GDPR-AC-3 | Given any existing consent proof event, when an UPDATE or DELETE is attempted, then the statement raises and the transaction aborts; the row persists unchanged. | trigger integration test (the DM-AC-5 shape, applied to the consent proof log per [[data-model#DM-DDL-10]]) |
| GDPR-AC-4 | Given an identity on the suppression list (GDPR-DDL-1), when capture or import attempts to upsert a person with that identity, then the write is rejected — an erased subject does not silently reappear. | suppression-list integration test now; the end-to-end capture-guard test lands with the planned capture chapter |
| GDPR-AC-5 | Given a backup taken before an erasure, when it is restored, then the suppression list is re-applied and the erased identity remains suppressed ([[operations#OPS-DR-6]]). | restore-drill procedure on the pinned cadence ([[operations#OPS-DR-3]]) + suppression re-apply integration test |
| GDPR-AC-6 | Given a fresh deployment after migration, then the seeded consent purposes GDPR-SEED-1..4 are present, and a new workspace carries the default retention rows [[data-model#DM-SEED-1]]..5. | migration/seed integration test |
| GDPR-AC-7 | Given an over-age record matching an enabled `erase` policy and an identical record under legal hold, when retention is evaluated, then the first is acted on (audited, suppression-listed) and the held one is skipped at the query level. | pinned by the `retention-expiry` fixture ([[testing]]); the evaluator test executes when the planned retention worker lands |

### Seed — consent purposes
Source: margince-poc/infra/migrations/000022_consent.up.sql @ a11d6c08

The reference set the platform migration block installs; consent checks are always
by purpose name. The shipped skeleton installs these as an instance-global lookup;
the corpus target shape is workspace-scoped ([[data-model#DM-DDL-10]]) —
reconciling the two is owned by the data-model chapter, the purpose vocabulary by
this one.

| ID | Purpose | Meaning |
|---|---|---|
| GDPR-SEED-1 | `marketing_email` | Email marketing communications |
| GDPR-SEED-2 | `marketing_phone` | Phone marketing communications |
| GDPR-SEED-3 | `profiling` | Profiling and personalisation |
| GDPR-SEED-4 | `product_updates` | Product update notifications |
