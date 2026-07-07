---
status: planned
module: backend/internal/modules/gdpr (workflow layer over the shipped substrate) · web (privacy, erasure and data-subject-request surfaces in settings)
derives-from:
  - specs/spec/features/04-platform-and-compliance.md#4-gdpr--compliance-eu-residency-audit-trails-sar-deletion-consent @ 5a0b29c
  - specs/spec/features/04-platform-and-compliance.md#6-eu-cra-conformity-the-product-meets-the-standard-the-practice-sells @ 5a0b29c
  - specs/spec/compliance/DPIA.md @ 5a0b29c
  - specs/spec/contract/data-model.md#34-consent--retention @ 5a0b29c
  - specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c
  - specs/spec/contract/crm.yaml (Compliance tag) @ 5a0b29c
  - specs/spec/product/build-backlog/E11.md#i-data-subject-rights-sar--erasure-requestfulfilment-surface-rides-ep07-machinery @ 5a0b29c
  - specs/spec/product/build-backlog/EP07.md @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#settingshtml--workspace-settings-implements-s-e111345-s-e105 @ 5a0b29c
---
# GDPR compliance surfaces — the workflows that make the substrate answer a data subject

> The workflow and surface layer over the [gdpr-platform](gdpr-platform.md)
> substrate: the tracked data-subject-request queue, admin-mediated
> subject-access assembly, the erasure workflow that erases everywhere and
> proves it without re-storing PII, the nightly retention worker that walks the
> policy ladder, the consent read/write surface every capture point funnels
> through, and the DPA / privacy-notice documents a controller hands their
> auditor. The tables and semantics already ship (GDPR-AC-1..7); this chapter
> is what a human can *do* with them.

## What it's for

The substrate chapter guarantees that consent defaults to no, that proof is
append-only, and that an erased identity stays erased — but a compliance
officer cannot exercise a guarantee. This chapter exists so a workspace admin
can *receive* a data-subject request and evidence its fulfilment inside the
statutory window, run a subject-access export that is provably complete, erase
a person across every layer the product touches — normalized records, the raw
capture layer, and the AI substrate's embeddings — and let the retention
schedule actually fire every night instead of remaining policy-as-data. Its
callers are the workspace-settings privacy surfaces, the consent card on the
person 360 ([[people-and-organizations]] renders it; the read/write seam is
ours), every consent-capturing surface (booking, public form, import,
preference center — each passes purpose and wording through, never a blanket
grant), and the nightly scheduler. The boundary: the substrate tables and
their semantics stay with [gdpr-platform](gdpr-platform.md); the send-time
suppression check and the buyer preference center stay with
[sequences-and-deliverability](sequences-and-deliverability.md); the workspace
export engine stays with [import-export-migration](import-export-migration.md).

No user story is dedicated to this chapter, and none is orphaned: the E11
privacy stories route elsewhere (S-E11.4 own-your-data export is
[[import-export-migration]]'s; S-E11.9 preference center is
[[sequences-and-deliverability]]'s), and the request surface itself entered
the backlog as a build ticket riding S-E11.4 (B-E11.30, red-team finding
RT-BL-M11 — the platform could fulfil but nothing let an admin receive and
action a request). The backbone of this chapter is therefore the feature
spec's machine-verifiable GDPR acceptance criteria themselves, pinned in the
appendix.

## Principles it serves

- **P12 — governance is designed in.** Data-subject rights are tracked,
  deadline-bearing, audited workflows with evidence at every step — not a
  support-ticket process bolted on after a regulator asks.
- **P7 — own your data.** The subject-access bundle, the DPA and
  sub-processor list, and the privacy notice are self-serve artifacts a
  controller can produce and hand over without a services engagement.
- **P11 — clean relational core.** Erasure is honest because the schema is:
  it removes real rows and real columns across every layer, rather than
  orphaning interpreted metadata.
- **ADR-0011 — consent and retention are V1.** This chapter is the workflow
  half of that decision: the substrate the ADR pulled into MVP only pays off
  when the evaluator runs nightly and the consent surface writes proof.
- **ADR-0025 — EU compliance posture.** The DPA, sub-processor list and DPIA
  template surfaced here are this chapter's slice of the one-posture
  compliance pack; the pack's assembly gate is the security chapter's
  (SEC-CRA-6).

## How it works

- **Consent surfaces render per-purpose state with its proof trail.** The
  read seam returns, for one person, the current granted / withdrawn /
  unknown state per purpose plus the full append-only proof history — the
  Art. 7 demonstrability view the person-360 consent card and the DPO's
  export both draw from ([[people-and-organizations]] AC-person-10 pins the
  card; the seam is pinned here). The write seam accepts exactly one
  transition for one purpose, appends one proof event carrying the verbatim
  wording and version shown, honors the double-opt-in rule where a purpose
  requires it, is idempotent on re-asserting the same state, and emits one
  consent-changed event. Capture surfaces — booking, the public form, import
  mapping, the preference center — must pass the purpose and wording through
  this seam; none may synthesize a blanket grant (substrate rule, GDPR-AC-1/2).
- **A data-subject request is a tracked record, not an email thread.** An
  admin logs an access, rectification or erasure request against a subject
  reference; the record carries a statutory due date (GCS-PARAM-1), an
  assignee, and a status that walks open → in progress → fulfilled or
  rejected, every transition audited. Fulfilment is not a parallel
  implementation: an access request runs the subject-access assembly, an
  erasure request runs the right-to-deletion path — one machinery, one
  surface over it (B-E11.30).
- **Subject-access assembly is admin-mediated in V1, and provably complete.**
  One operation assembles everything held about a subject — across normalized
  objects, activities, and the raw capture layer — into an export package,
  and the operation is itself audited. V1 deliberately ships no public
  authenticated data-subject surface: the Art. 15 *duty* is met by staff
  fulfilment, and the contact-facing self-service portal is fast-follow — the
  scope call the threat model carried as open (TM-DPIA-6) was decided
  2026-06-22 (G-RT-6b); this chapter records the closure.
- **Erasure erases everywhere and proves it without re-storing PII.** The
  erasure workflow removes or irreversibly anonymizes the subject across
  normalized tables (including agent-enriched fields), the raw capture layer,
  and the AI substrate's embeddings; writes a PII-free tombstone through the
  append-only audit wall — the resolution of the right-to-erasure versus
  immutable-audit tension; and lands the identity on the suppression list as
  salted hashes (GDPR-DDL-1) so capture, import, and even a database restore
  cannot resurrect it ([[operations#OPS-DR-6]], substrate GDPR-AC-4/5). The
  surface makes the irreversibility visually distinct from the reversible
  archive action (AC-settings-8).
- **The retention worker wires the ladder to the clock.** A nightly job
  (GCS-PARAM-2) walks the enabled per-workspace policy rows the substrate
  seeds and stores ([[data-model#DM-SEED-1]]..5), acts on over-age records —
  archive, anonymize, or erase, each policy row one action, ladders composing
  as separate rows at increasing ages — writes one audit row per action,
  emits one retention-applied event per action, and skips any record under
  legal hold at the query level (substrate GDPR-AC-7 pins the fixture; this
  chapter lands the evaluator that executes it). The erase step reuses the
  erasure path above — no second implementation. Setting or clearing a legal
  hold is an audited mutation surfaced here.
- **The document surfaces close the DPIA's named gaps.** The workspace
  compliance surface offers the current DPA and enumerated sub-processor list
  (the LLM vendors on the cloud path; none on the sovereign path) for
  download, per release. The privacy notice shipped with the product states
  in plain language that captured content is processed by AI — the first of
  the two build gaps the DPIA recorded ([[threat-model#TM-DPIA-5]]) becomes a
  concrete acceptance criterion here (GCS-AC-7); the second, the SAR scope
  call ([[threat-model#TM-DPIA-6]]), is closed by the decision above. The
  breach runbook — detect, assess, notify the supervisory authority within
  the pinned window and affected subjects without undue delay — is wired to
  the anomaly detection on the audit stream ([[audit-observability]]).

## What's configurable

- **Retention policies are per-workspace data** — seeded compliant defaults,
  editable per tenant; the values are the data-model chapter's pins
  ([[data-model#DM-SEED-1]]..5), the editing and evidence surface is this
  chapter's. The nightly cadence itself is fixed (GCS-PARAM-2).
- **Consent purposes are workspace-manageable** — the seeded reference set
  (GDPR-SEED-1..4) can be extended by an admin, including whether a purpose
  requires double opt-in (the German email norm).
- **The statutory due date is set at request intake** — defaulting to the
  one-month clock (GCS-PARAM-1), adjustable per request where the law allows
  extension; the queue orders by it.
- **Legal hold is a per-record toggle** through an audited mutation; nothing
  about its retention-beating effect is configurable (substrate guarantee).
- Nothing else is tunable: erasure scope, tombstone content, suppression
  semantics, and admin-mediation of SAR in V1 are guarantees, below.

## Guarantees (enforced)

- **A consent change through any surface leaves exactly one complete proof
  row** — timestamp, source, lawful basis, and the verbatim policy wording
  and version shown — and the log rejects tampering (proof completeness
  pinned as GCS-AC-3; append-only wall is the substrate's, GDPR-AC-3).
- **A subject-access bundle is complete against a seeded subject** — data
  from all linked objects and activities present, the operation audited, and
  fulfilment evidenced inside the request's statutory window (GCS-AC-2,
  GCS-AC-5).
- **Erasure leaves nothing findable and nothing resurrectable** — search
  returns nothing across normalized, raw, and embedding layers; the tombstone
  exists and carries no PII; the suppression list blocks re-capture and
  re-import and survives a restore ([[operations#OPS-DR-6]], substrate
  GDPR-AC-4/5) (GCS-AC-1).
- **The retention worker fires nightly and never touches a held record** —
  one audit row and one event per applied action; the legal-hold skip is
  enforced at the query level, not by worker discipline (GCS-AC-4, executing
  the substrate's GDPR-AC-7 fixture).
- **V1 exposes no public authenticated data-subject surface** — subject
  access is admin-mediated by decision (G-RT-6b), asserted as a negative
  scope check, so the duty is met without an unreviewed public door
  (GCS-AC-10).
- **The privacy notice states AI processing of captured content** — the
  DPIA build gap is closed by presence test, not by intention (GCS-AC-7,
  closes [[threat-model#TM-DPIA-5]]).

## Acceptance

Done means a compliance officer can run the whole obligation loop without
leaving the product: log a request and watch its deadline; produce a complete
subject bundle; erase a person and show the tombstone, the empty search, and
the suppression entry; point at last night's retention actions in the audit
view with the held record untouched; and download the DPA, sub-processor
list, and a privacy notice that admits the AI processing. The honest states
the surfaces must render: an empty request queue, a request past due, an
erasure in progress (async job states per the acceptance-standards floor),
and the fulfilled request with its evidence trail. The testable form of every
claim lives in the Acceptance appendix; the cross-cutting floor is inherited
from the acceptance-standards chapter.

## Out of scope

The consent, retention, suppression and legal-hold *substrate* — tables,
seeds, triggers, default-deny semantics — is
[gdpr-platform](gdpr-platform.md)'s (GDPR-AC-1..7). The send-time consent
suppression and the buyer-facing preference center (S-E11.9) are
[sequences-and-deliverability](sequences-and-deliverability.md)'s. The
workspace export engine the SAR bundle rides, and the settings data-and-exit
screen rows, are [import-export-migration](import-export-migration.md)'s
(AC-settings-5..7). The person-360 consent card rendering is
[people-and-organizations](people-and-organizations.md)'s (AC-person-10).
The CRA compliance-pack assembly gate and its contents checklist are the
security chapter's (SEC-CRA-6, SEC-CVD-7); the buyer-facing trust pack is the
germany-package chapter's (E17.3). The audit wall every action here crosses
is [audit-observability](audit-observability.md)'s; restore mechanics are
[operations](../architecture/operations.md)'s. The works-council qualifier
and the M9 counsel gate are the threat model's ([[threat-model#TM-DPIA-4]]).

## Where it lives

The gdpr module directory (backend/internal/modules/gdpr) grows the workflow
layer — request tracking, subject-access assembly, the erasure orchestration,
the retention evaluator job — over its own substrate; the web shell's
settings area carries the privacy, erasure, and request-queue surfaces.
Callers reach it through the consent read/write seam and the
data-subject-request seam. Read next:
[gdpr-platform](gdpr-platform.md) for the substrate,
[audit-observability](audit-observability.md) for the wall,
[sequences-and-deliverability](sequences-and-deliverability.md) for where
consent meets the send path.

## Appendix

### Parameters
Source: specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c; specs/spec/features/04-platform-and-compliance.md#4-gdpr--compliance-eu-residency-audit-trails-sar-deletion-consent @ 5a0b29c; specs/spec/contract/data-model.md#34-consent--retention @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| GCS-PARAM-1 | DSR_STATUTORY_WINDOW | 30 days | Default `due_at` set at request intake — the GDPR Art. 12(3) one-month clock; required on every request; the queue orders open requests by it. |
| GCS-PARAM-2 | RETENTION_EVAL_CADENCE | nightly | The retention worker evaluates enabled policy rows once per night (background job runner). Not tunable. |
| GCS-PARAM-3 | BREACH_NOTIFY_SA | 72 h | Breach runbook: notify the supervisory authority within 72 hours (Art. 33). |
| GCS-PARAM-4 | BREACH_NOTIFY_SUBJECTS | without undue delay | Breach runbook: notify affected data subjects without undue delay (Art. 34). |
| GCS-PARAM-5 | SAR_SELF_SERVICE_SCOPE | admin-mediated (V1) | Decided 2026-06-22 (Lars, G-RT-6b): V1 ships admin-mediated Art. 15 fulfilment; the contact-facing self-service portal is fast-follow. Closes the open flag in [[threat-model#TM-DPIA-6]]. |

The retention windows themselves (365 / 730 / 1095 / 1825 days and the
transcript special case) are pinned once as [[data-model#DM-SEED-1]] through
[[data-model#DM-SEED-5]] and are not restated here.

### Schema — data-subject request tracking
Source: specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c

GCS-DDL-1 — the request-tracking table the ownership index assigns to this
chapter ([[data-model]] ownership partition). Distinguishing columns below;
the corpus's standard base columns (id, workspace_id, timestamps) and the
optimistic-concurrency `version` apply as on every mutable table, with forced
row-level security per the tenant-isolation conventions
([[data-model#DM-CONV-5]]..8). Every status transition is audited.

```sql
CREATE TABLE data_subject_request (                      -- GDPR Art.15/16/17 request tracking (B-E11.30)
  kind        text NOT NULL CHECK (kind IN ('access','rectify','erasure')),
  subject_ref text NOT NULL,                             -- the data subject (person id or external identifier)
  status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','fulfilled','rejected')),
  due_at      timestamptz NOT NULL,                      -- statutory deadline (GCS-PARAM-1)
  assignee_id uuid NULL REFERENCES app_user(id),
  resolution  text NULL                                  -- ties to retention/erasure machinery; each transition audited
);
CREATE INDEX idx_dsr_open ON data_subject_request (workspace_id, status, due_at);
```

Honest gaps, owned elsewhere or deliberately absent: the consent and
retention tables are [[data-model#DM-DDL-10]]; the erasure suppression list
is GDPR-DDL-1 ([[gdpr-platform]]); legal hold is a flag on the core objects
([[data-model#DM-DDL-10]]), not a table here; the subject-access bundle is a
produced artifact plus an audit row, not a stored table.

### Wire
Source: specs/spec/contract/crm.yaml (Compliance tag) @ 5a0b29c

Operations cited by contract operationId, never restated. The
people-and-organizations chapter's wire note defers the person consent
operations to the gdpr layer; the substrate chapter pins no wire surface, so
their single home is here.

| ID | Operation | Notes |
|---|---|---|
| listDataSubjectRequests | GET on the request collection | Status filter + cursor paging; exposed as an MCP search tool at tier 🟢 (record type `data_subject_request`). |
| createDataSubjectRequest | Open an access / rectify / erasure request | Idempotency-Key header; `due_at` required (GCS-PARAM-1). |
| updateDataSubjectRequest | Status / assignee / resolution transition | Fulfilment side effects run the SAR or erasure machinery; each transition audited (GCS-AC-5). |
| listConsentPurposes | List the workspace's purposes | Seeded set GDPR-SEED-1..4 plus workspace additions. |
| createConsentPurpose | Define a purpose | 🟢 admin write; carries `requires_double_opt_in` + Art. 6 basis. |
| getPersonConsent | Per-purpose state + full proof log | Read-only; never a blanket flag (substrate GDPR-AC-1/2). |
| recordConsent | Grant/withdraw one purpose | Appends one proof row; DOI token required where the purpose demands it; idempotent; audited (`consent_grant`/`consent_withdraw`); emits `consent.changed`; 404/422 per contract. |

Gaps the build must close (pinned honestly, no invented operationIds):

| ID | Gap |
|---|---|
| GCS-GAP-1 | No contract operation yet for subject-access bundle assembly/download — V1 fulfilment rides the request transition (updateDataSubjectRequest) plus the export machinery; the build pins the seam. |
| GCS-GAP-2 | No contract operations yet for retention-policy administration or legal-hold set/clear — both are audited admin mutations this chapter owes the contract. |
| GCS-GAP-3 | The send-time `409 consent_not_granted` suppression is the send operation's contract, owned at the send seam by [[sequences-and-deliverability]] — cited, not pinned here. |

### Events
Source: specs/spec/contract/events.md#51-person @ 5a0b29c

Definitions live in the central catalog ([[event-bus]]); cited, not redefined.

| ID | Emitted by | Note |
|---|---|---|
| `consent.changed` | The consent write seam (recordConsent) | One event per proof row appended; consumed by the read model and the outbound-suppression workflow ([[sequences-and-deliverability]]). |
| `retention.applied` | The nightly retention worker | One event per applied action (archive / anonymize / erase, with policy provenance); consumed by read model and audit stream. |

### Acceptance
Source: specs/spec/features/04-platform-and-compliance.md#4-gdpr--compliance-eu-residency-audit-trails-sar-deletion-consent @ 5a0b29c; specs/spec/compliance/DPIA.md @ 5a0b29c; specs/spec/product/build-backlog/E11.md#i-data-subject-rights-sar--erasure-requestfulfilment-surface-rides-ep07-machinery @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#settingshtml--workspace-settings-implements-s-e111345-s-e105 @ 5a0b29c

The feature spec's `[MV]` criteria this chapter owns are carried verbatim
(GCS-AC-1..4); the substrate's GDPR-AC-1..7 are the sibling's pins and are
cited, never re-owned. The consent-check default-deny, cross-purpose
isolation, and send-time suppression criteria live with [[gdpr-platform]]
(GDPR-AC-1/2) and [[sequences-and-deliverability]] (send seam); the
audit-mutation-coverage and replayable-trace criteria live with
[[audit-observability]] and the agent chapters.

| ID | Given/When/Then | Verification |
|---|---|---|
| GCS-AC-1 | `[MV]` Right-to-deletion removes the subject from normalized tables, raw JSONB capture, and pgvector embeddings, while leaving an erasure tombstone — verified by a "search returns nothing + tombstone exists + no PII in tombstone" test. Erasure removes agent-enriched fields and embeddings tied to the subject, not just human-entered fields; the identity lands on the suppression list (GDPR-DDL-1) so re-capture, re-import, and restore cannot resurrect it (substrate GDPR-AC-4/5, [[operations#OPS-DR-6]]). | Backend integration lane (erasure fixture); restore half rides the substrate's GDPR-AC-5 drill |
| GCS-AC-2 | `[MV]` SAR export for a subject includes data from all linked objects and activities (completeness assertion against a seeded subject) — gates the SAR fast-follow. The operation is staff-mediated in V1 and itself audited. | Backend integration lane (seeded-subject completeness test) |
| GCS-AC-3 | `[MV]` Every consent grant/withdrawal writes a proof row carrying timestamp, source, lawful basis, and the policy wording+version shown — asserted by a proof-completeness test; append-only enforcement is the substrate's wall (GDPR-AC-3). Where the purpose requires double opt-in, a grant is effective only after the confirmed DOI event. | Backend integration lane (proof-completeness + DOI tests) |
| GCS-AC-4 | `[MV]` The retention evaluator acts on a seeded over-age record per its policy (archive → anonymize → erase), audits the action, and skips a record under legal hold — verified by a retention + legal-hold test. Runs nightly (GCS-PARAM-2); one audit row and one `retention.applied` event per action; the erase step reuses GCS-AC-1's path. This is the evaluator execution the substrate's GDPR-AC-7 fixture awaits. | Backend integration lane (`retention-expiry` fixture, [[gdpr-platform]] GDPR-AC-7) |
| GCS-AC-5 | Given a logged data-subject request (kind access/rectify/erasure, subject reference, `due_at`), when fulfilment is actioned, then the matching machinery runs (access → GCS-AC-2 assembly; erasure → GCS-AC-1 path), the row walks open → in_progress → fulfilled/rejected, and every transition is audited — request → fulfilment → completion asserted end-to-end (B-E11.30). | Backend integration lane (end-to-end DSR lifecycle test) |
| GCS-AC-6 | Given a record, when an authorized admin sets or clears legal hold, then the mutation is audited and a subsequent retention evaluation skips the held record (the skip itself is the substrate's query-level guarantee, GDPR-AC-7). | Backend integration lane (hold set/clear audit test) |
| GCS-AC-7 | Given the shipped privacy notice, when its content is checked, then it states that captured content is processed by AI (Art. 13/14) — closing the DPIA build gap [[threat-model#TM-DPIA-5]]. | Document presence/content check in the release checklist; screen e2e where the notice is surfaced |
| GCS-AC-8 | Given the workspace compliance surface, when viewed, then the current DPA and the enumerated sub-processor list (LLM vendors on the cloud path; none on the sovereign path) are downloadable and per-release current — the surface slice of the compliance pack whose assembly gate is SEC-CRA-6. | Screen e2e lane + pack-contents check (cited to [[security]] SEC-CRA-6) |
| GCS-AC-9 | Given the breach-notification runbook, when a seeded breach case is exercised, then the flow runs detect → assess → notify the supervisory authority within 72 h (GCS-PARAM-3) and affected subjects without undue delay (GCS-PARAM-4), wired to the audit-stream anomaly detection. | Process test against a fixture (runbook drill) |
| GCS-AC-10 | V1 exposes no public authenticated data-subject surface: subject access is admin-mediated (GCS-PARAM-5, G-RT-6b); the contact-facing portal is fast-follow. | Negative scope check in review (asserted by absence) |
| AC-settings-8 | (right to erasure): Given the Right-to-erasure block, When viewed, Then it is visually distinct, states it is NOT the reversible "delete contact", permanently wipes the person across record/activity/derived-values/embeddings, writes a tombstone to the audit log with no PII re-stored (salted hash only), and adds email/phone as salted hashes to a re-import suppression list; the "Erase a person…" action is itself audited. | Screen e2e lane (owned here; the rest of the AC-settings series belongs to [[import-export-migration]] and [[access-and-admin]]) |

**Open build decisions (carried honestly — the build tickets must resolve
them).** How a data-subject's identity is verified before fulfilment is
unspecified (the prototype only toasts intent); the erasure
confirmation/verification modal UX is undesigned; the request queue has no
dedicated mockup (the erasure block lives on the settings screen; the queue
UI is flagged for work-package-entry decomposition under B-E11.30); async-job
states for export and erasure inherit the acceptance-standards floor
(STATE-SP-5) but the concrete surface is undrawn.
