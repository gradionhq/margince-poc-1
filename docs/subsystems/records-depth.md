---
status: planned
module: backend/internal/modules/records (hierarchy roll-up reads, attachments, quota & attainment, field-history projection) · web (account-hierarchy, attachments, formula-fields, quota, field-history screens)
derives-from:
  - specs/spec/features/10-operational-depth.md#6-records--reporting-depth-promotes-d23-hierarchy-d16-formula-fields-d110-attachments-d96-quota-d18-field-history
  - specs/spec/product/epics/E15-operational-depth.md#s-e158--account-hierarchy-attachments-formula-display-quota--field-history
  - specs/spec/features/01-core-objects.md#22-organization-hierarchy
  - specs/spec/features/03-reporting-and-scoring.md#2-forecasting
  - specs/spec/contract/data-model.md#103-attachment-s3minio-references
  - specs/spec/contract/data-model.md#saved-views-quota-field-mask
  - specs/spec/contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26
  - specs/spec/contract/data-model.md#11-audit-log-append-only
  - specs/spec/product/30-screen-acceptance.md#account-hierarchyhtml--account-hierarchy--roll-up-implements-s-e158a
  - specs/spec/product/30-screen-acceptance.md#attachmentshtml--attachments-on-records-implements-s-e158b
  - specs/spec/product/30-screen-acceptance.md#formula-fieldshtml--formula-computed-fields-implements-s-e158c
  - specs/spec/product/30-screen-acceptance.md#quotahtml--quota--attainment-implements-s-e158d
  - specs/spec/product/30-screen-acceptance.md#field-historyhtml--field-change-history-implements-s-e158e
---
# Records depth — the everyday record and reporting depth buyers expect, built on the spine that already exists

> Five parity promotions in one chapter (S-E15.8): roll-up reporting over the
> corporate account tree, files on records, read-only computed-field display, a
> real quota object with attainment computed from closed-won, and per-field change
> history. The single promise: none of these adds a second engine — each is a
> governed surface over a substrate another chapter already pins (the parent link,
> the blob seam, database-computed columns, the deal spine, the audit spine).

## What it's for

A revenue leader (Riya) and an admin (Mor) evaluating against an incumbent expect
five unglamorous things to simply be there: roll a parent company's whole tree up
into one number, attach a contract to a deal, see a computed field on a record,
track each rep against a quota, and read who changed a field from what to what.
Before this subsystem those expectations either had no surface or were answerable
only by reading raw data. This chapter specifies all five as V1-Must behavior
(S-E15.8, promoted 2026-06-23 from the parity benchmark). Its callers are the
record 360 views (which gain attachments, computed fields, and per-field history),
the reporting surfaces and Morning Brief (which consume roll-ups and attainment),
account-mapping and coverage views (which read the tree), and agents reasoning
over the hierarchy through the governed tool surface. The scope boundary is
deliberate: the parent-link column and its cycle guard belong to the
people-and-organizations chapter — this chapter owns what *rolls up over* that
link; the audit spine belongs to the audit-observability chapter — this chapter
owns the per-field *surface projected from* it.

## Principles it serves

- **P11 — typed edges, real columns.** The corporate tree is a real
  self-referencing link, not an inferred graph; a quota is a first-class object
  with an owner, a period, and an integer-minor-unit target — never a free-typed
  spreadsheet number.
- **P4 — bounded joins on the hot path.** Roll-up reporting traverses the tree
  with a bounded, indexed recursive query inside a pinned budget
  ([[people-and-organizations#PO-AC-28]]); it never becomes an unbounded graph
  walk.
- **P12 — history from the one audit spine.** Field-level history is a read-only
  projection of the append-only audit log ([[data-model#DM-DDL-8]]) — no new PII
  store, no second history mechanism.
- **P7 / ADR-0051 — sovereign blobs by default.** Attachment bytes go through the
  one pluggable blob-storage seam ([[architecture#ARCH-SEAM-9]]) whose default
  endpoint is a sovereign, self-hosted S3-API store; a public cloud bucket is
  opt-in configuration, never the default.
- **P1 / ADR-0002 — logic is source; NEVER-1 holds.** Formula fields are
  *display* of database-computed columns whose defining logic ships as reviewed
  source; there is no runtime formula builder and no expression interpreter on
  the hot path ([[scope#NEVER-1]]).

## How it works

**Hierarchy roll-up.** The parent/child account tree lives on the organization
record as a single parent link with cycle prevention — column and guard owned by
the people-and-organizations chapter ([[people-and-organizations#PO-DDL-4]]).
This chapter owns the roll-up semantics over it: a group view aggregates a node's
own figures plus, recursively, its children's (RD-FORM-1) for three measures —
weighted open pipeline (composing the deals chapter's reconciling weighted value,
[[deals-and-pipeline#DEAL-FORM-2]]), closed-won for the period, and recent
activity count. The traversal is a bounded, indexed recursive query inside the
pinned budget ([[people-and-organizations#PO-AC-28]]); mixed currencies convert
to the workspace base via the frozen-or-daily rate machinery
([[data-model#DM-FX-4]]) — never a raw cross-currency sum. The roll-up is
RBAC-honest: a node the viewer cannot read renders as a restricted placeholder
and its figures are *excluded and disclosed as excluded*, never silently summed.
The viewer can flip between whole-tree and this-account-only scope, and every
aggregate can be decomposed ("explain this roll-up") into self plus children.
Only the corporate parent/child tree is the denormalized link; partner edges
between companies remain typed relationship rows (people-and-organizations
chapter).

**Attachments.** A file attached to a record is a blob-store *reference*: the
bytes go through the blob-storage seam (sovereign default, ADR-0051) and the
database keeps metadata plus the object key only — never binary content in a
table (RD-DDL-1). An attachment is polymorphically bound to its record (the
canonical entity vocabulary, [[data-model#DM-CONV-17]]), carries provenance
(who or which agent captured it), and rides the archive cascade
([[data-model#DM-CONV-15]]). Visibility is inherited from the record — there is
no separate per-file ACL surface in V1; a file the viewer's role cannot see is
shown as a locked, disclosed row with a request-access path, never silently
hidden. Uploads pass a virus scan before they are downloadable (clean / scanning
/ blocked states); a flagged file is quarantined and honestly labeled. Every
attach is written to the record timeline with provenance, and every download is
itself audited as a file access. When the agent grounds field values in an
attached document, they are staged for explicit human accept under
evidence-or-omit — nothing touches the record until accepted.

**Formula-field display.** A formula field is a read-only, database-computed
generated column surfaced on the record — filterable and exportable like any
core field, recomputed by the database on write, provenance always
"computed by the server", never "typed by you". The boundary is hard and tested:
the schema proves the value is database-generated, not an app-side interpreted
expression, and a negative-scope check proves no user-authored formula-logic
surface exists at runtime — defining a *new* formula's logic is a reviewed
source change routed to the development path (ADR-0002; the same refusal posture
as the custom-fields chapter's structural-change refusal). A value that cannot
be computed honestly (a missing input) renders as an honest "not computable
yet", never an invented denominator; a field masked for the viewer's role is
absent from the payload (access-and-admin chapter), not blurred.

**Quota and attainment.** A quota is a first-class object: exactly one owner
*or* one team, a date period, and a human-set target in integer minor units
(RD-DDL-2). Attainment is computed server-side at read time from closed-won
deals on the clean core — deals whose status is won ([[deals-and-pipeline#DEAL-DDL-3]])
with close dates inside the period; open, lost, and forecast-omitted deals are
excluded — and is decomposable ("explain this number") into the summed deals and
the flagged human-set target (RD-FORM-2). A team roll-up is the sum of closed-won
over the sum of targets, auditable like the weighted pipeline. There is no
AI-set quota, no forecast-to-quota auto-fill, and no compensation engine; if the
attainment query fails, the surface shows an honest error with the last
successful compute time — never a stale or guessed figure. Forecast categories,
forecast-vs-quota comparison, coverage ratio, and the period-close
predicted-vs-actual snapshot belong to the forecasting chapter; this chapter
owns the quota object and its attainment number, which that chapter cites.

**Field change history.** Per-field history is reconstructed on read from the
append-only audit spine's before/after diffs ([[data-model#DM-DDL-8]]) — a
read-only projection, not a second store. Its completeness is inherited, not
re-promised: every mutation writes exactly one audit row through the one seam
([[audit-observability#AUD-AC-2]], [[audit-observability#AUD-AC-3]]), so the
projection can miss nothing; its immutability is the spine's own trigger
([[audit-observability#AUD-AC-1]]). The surface shows, per field, the current
value and a diff timeline — who (human, or agent with its Passport and approval
marker) changed it from what to what, with evidence and confidence on
agent-authored changes. An empty history is honest ("set on create and never
changed"), a field masked for the viewer's role withholds its history exactly as
it withholds its value, and because the projection derives from the audit spine
it inherits the erasure scrub — it can never re-surface erased PII (the corpus
closes this explicitly; retention follows the audit spine and the gdpr-platform
chapter's policy rows, never a records-depth copy).

## What's configurable

- **Quota target** — per quota row, human-set at runtime in whole-currency input
  stored as integer minor units (RD-DDL-2); changing it is a logged write that
  recomputes attainment. It is data, not configuration surface.
- **Blob-storage endpoint** — where attachment bytes live varies by deployment
  tier through the blob seam: self-hosted store on customer infra (zero egress),
  the operator's EU-region store, or a dev store; a public cloud endpoint is
  explicit opt-in (ADR-0051; the endpoint is a governed runtime-config entry).
  Absent the store, uploads fail honestly — references are never faked.
- **Virus scanner** — an injected dependency on the upload path; scan outcome
  gates downloadability (RD-PARAM-5). Absent a scanner verdict a file stays in
  the scanning state, not silently clean.
- **Stage win probabilities** — the weighted measure inside the roll-up re-tunes
  with the deals chapter's only runtime-tunable parameter
  ([[deals-and-pipeline#DEAL-PARAM-3]]); nothing else about the roll-up is
  runtime-tunable.

## Guarantees (enforced)

- **The tree is acyclic and the roll-up is bounded.** No organization is its own
  ancestor (column-owner's guard, [[people-and-organizations#PO-DDL-4]]); the
  group roll-up traverses with an indexed recursive query at p95 under 200 ms for
  trees of ≤200 organizations (owner's pin [[people-and-organizations#PO-AC-28]];
  roll-up behavior acceptance lives here, RD-AC-1).
- **A restricted node is excluded and says so.** Roll-up totals never silently
  include or fake a node the viewer cannot read; exclusion is disclosed on the
  surface (RD-AC-1, AC-account-hierarchy-5).
- **No blob ever lands in the database.** Attachment rows hold object keys and
  metadata only (RD-DDL-1); bytes go through the blob seam with the sovereign
  default (ADR-0051). There is no binary-column path to regress to.
- **Attachment visibility is the record's visibility.** Access is inherited, a
  denied file is disclosed-not-hidden, and every download is an audited file
  access (RD-AC-2).
- **A formula field is database-computed and runtime-inert.** The value is
  generated by the database, read-only everywhere, and there is no runtime
  authoring surface — both directions pinned by build-verifiable tests
  (RD-AC-6, RD-AC-7); no expression interpreter exists on the hot path
  ([[scope#NEVER-1]]).
- **Attainment reconciles or refuses.** The attainment number equals the sum of
  the listed closed-won deals divided by the flagged human-set target
  (RD-FORM-2); a failed recompute shows an honest error, never a stale or
  invented figure (RD-AC-4, AC-quota-4).
- **Field history can neither drift nor resurrect.** It is a pure projection of
  the append-only audit spine — complete because no unaudited mutation path
  exists ([[audit-observability#AUD-AC-2]]), immutable because the spine is
  ([[audit-observability#AUD-AC-1]]), and erased PII stays erased because the
  projection inherits the spine's scrub (RD-AC-5).
- **Money is minor units everywhere.** Roll-up totals, quota targets, and
  attainment sums are integer minor units + ISO-4217 with base-currency
  conversion through the one FX machinery ([[data-model#DM-CONV-9]],
  [[data-model#DM-FX-4]]) — never floats, never rate=1 fallbacks.

## Acceptance

Done means Riya and Mor can do the five user-observable things the promotion
promised: roll up a parent account's whole tree and flip honestly between tree
and self scope; attach a contract to a deal and see it on the timeline with
provenance, with restricted and quarantined files disclosed rather than hidden;
read a computed field with its explanation and see it refuse to compute rather
than guess; set a target and watch attainment recompute from closed-won only,
with an inspectable contributing-deals list; and open any field's history to
read who changed what from what, including agent changes with their approval
markers and evidence. Every surface renders the standard honest states — empty,
loading, error, no-permission ([[acceptance-standards#STATE-1]] through
[[acceptance-standards#STATE-4]]) — and AI-adjacent panels obey
nothing-grounded honesty ([[acceptance-standards#STATE-5]]). The testable form
of each claim is pinned in the Acceptance appendix; the cross-cutting floor
(screen-state matrix, performance budgets, release gates) is inherited from the
acceptance-standards chapter, and the roll-up read also respects the reporting
budgets pinned there.

## Out of scope

- **The parent-link column, cycle guard, and organization merge/hierarchy
  editing** — the people-and-organizations chapter; this chapter cites, never
  restates.
- **Forecast categories, forecast-vs-quota views, coverage ratio, and the
  period-close forecast snapshot** — the forecasting chapter (its
  predicted-vs-actual table is owned there; the quota object it divides by is
  owned here).
- **Field-masking policy** (which roles see which fields) — the access-and-admin
  chapter; this chapter only honors masks on formula display and field history.
- **Adding a runtime custom field** (S-E15.7) — the custom-fields chapter;
  distinct by construction: a custom field is a stored scalar the user types, a
  formula field is a computed column whose logic is source.
- **Bulk edit/reassign/delete/export behavior** — the bulk-operations story
  family (S-E15.5 and its E11 build stories); this chapter single-homes only the
  bulk-job record's schema per the ownership index (RD-DDL-3, see its note).
- **Deal splits / multi-owner credit** — explicitly OUT for V1 (backlog).
- **Audit-spine mechanics** (append-only trigger, one-row-per-mutation seam,
  erasure tombstones) — the audit-observability chapter.

## Where it lives

Backend behavior lands in the records module under the backend's modules
directory, reaching organizations, deals, and the audit spine through their
seams and the blob store through the platform blob seam; the five screens are
web features on the record and reporting surfaces. Read next:
people-and-organizations (the tree this chapter rolls up),
deals-and-pipeline (the closed-won and weighted-value spine attainment and
roll-ups consume), audit-observability (the spine field history projects), and
forecasting (the sibling that cites the quota object).

## Appendix

### Parameters
Source: features/10-operational-depth.md#6-records--reporting-depth-promotes-d23-hierarchy-d16-formula-fields-d110-attachments-d96-quota-d18-field-history @ 5a0b29c; contract/data-model.md#saved-views-quota-field-mask @ 5a0b29c; product/30-screen-acceptance.md#quotahtml--quota--attainment-implements-s-e158d @ 5a0b29c; adr/ADR-0051-blob-storage-sovereign-default.md (vendored) @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| RD-PARAM-1 | Roll-up traversal bound | indexed recursive CTE over the parent link; p95 `< 200ms` for trees of ≤200 orgs — owner's pin [[people-and-organizations#PO-AC-28]], sanctioned restatement | The P4 budget the roll-up read must hold; the query-budget acceptance is owned by people-and-organizations, the roll-up *behavior* acceptance is RD-AC-1. |
| RD-PARAM-2 | Roll-up measures | weighted open pipeline ([[deals-and-pipeline#DEAL-FORM-2]] per node), closed-won for the selected period, activity count over a 30-day window | The three tree-aggregated measures (AC-account-hierarchy-1). Mixed currencies convert to workspace base per [[data-model#DM-FX-4]]; never a raw cross-currency sum. |
| RD-PARAM-3 | Quota scope & shape | exactly one of owner XOR team (RD-DDL-2 CHECK); period = `[period_start, period_end]` dates; `target_minor bigint` + `char(3)` ISO-4217 | A quota is per-user or per-team per period; the target is always human-set (no AI-guessed quota — quota.html scope note). |
| RD-PARAM-4 | Attainment display thresholds | met ≥ 100% (arc capped at full circle); 60–99% accent; < 60% behind; pace = attainment vs %-of-period-elapsed | Deterministic display bands pinned by AC-quota-2/-3. |
| RD-PARAM-5 | Attachment storage & scan states | bytes via the `blobstore` seam ([[architecture#ARCH-SEAM-9]]), sovereign S3-API default, cloud opt-in (ADR-0051); DB stores refs + metadata only (RD-DDL-1); scan status ∈ {clean, scanning, blocked}; blocked = quarantined, not downloadable | The never-bytea rule and the scan gate on downloadability (AC-attachments-2/-7); provenance + scan result immutable in the details view (AC-attachments-8). |
| RD-PARAM-6 | Field-history retention | inherits `audit_log` — append-only spine, erasure-scrub inheritance (corpus data-model §11 note, closes RT-PR-H9); policy rows per the gdpr-platform chapter | No records-depth copy of history exists to retain or scrub separately. |

### Formulas
Source: product/30-screen-acceptance.md#account-hierarchyhtml--account-hierarchy--roll-up-implements-s-e158a @ 5a0b29c; product/30-screen-acceptance.md#quotahtml--quota--attainment-implements-s-e158d @ 5a0b29c

**RD-FORM-1 — tree roll-up.**
- Inputs: the account tree under a root (parent link, [[people-and-organizations#PO-DDL-4]]); per-node self figures for each RD-PARAM-2 measure; the viewer's read scope.
- Pseudocode: `roll-up(node) = self(node) + Σ roll-up(child)` over readable children; a non-readable node contributes nothing and is reported as excluded; traversal is the bounded recursive CTE (RD-PARAM-1).
- Output: per-measure tree totals + the count of aggregated accounts; scope toggle returns `self(root)` alone.
- Tie-breaks / edges: a child with no deals contributes `0` (rendered as `0`, not blank); cycle input is impossible by the owner's guard; restricted exclusion is disclosed, never silent.
- Worked example (AC-account-hierarchy-1/-2, seed): weighted pipeline = root self `38.500 €` + visible children's roll-ups, badge "aggregated over 5 accounts"; the restricted node (Brandt Defense Systems) is excluded and named as excluded in the explain box.

**RD-FORM-2 — quota attainment.**
- Inputs: the quota row (RD-DDL-2: owner XOR team, period, `target_minor`); closed-won deals on the clean core — `status = 'won'` ([[deals-and-pipeline#DEAL-DDL-3]]) with close date ∈ period; open/lost/forecast-omitted deals excluded.
- Pseudocode: `attainment = Σ(closed-won base_value_minor) ÷ target_minor`, in integer minor units, base-currency converted per [[data-model#DM-FX-4]]. Team roll-up: `Σ closed-won ÷ Σ targets` across members.
- Output: attainment %, closed-won total, gap-to-target (signed), pace vs period elapsed (RD-PARAM-4).
- Tie-breaks / edges: zero/empty target → no attainment computed (input refused, AC-quota-6); recompute failure → honest error + last successful compute time, never a stale/guessed figure.
- Worked example (AC-quota-1, seed): three closed-won deals totalling `313.872,00 €` ÷ target `280.000,00 €` = **113%**, gap `+33.872,00 €`, ring capped at full circle in the met colour.

### Schema
Source: contract/data-model.md#103-attachment-s3minio-references @ 5a0b29c; contract/data-model.md#saved-views-quota-field-mask @ 5a0b29c; contract/data-model.md#125-cont--bucket-3-decision-tables-2026-06-26 @ 5a0b29c

Three tables, owned here per the schema ownership index
([[data-model#Schema — ownership index]]). Base columns per
[[data-model#DM-CONV-3]]; mutable rows carry `version`
([[data-model#DM-CONV-4]]); money per [[data-model#DM-CONV-9]]. **Field history
deliberately has no table here**: it is a projection of `audit_log`, whose DDL
is owned by the data-model chapter ([[data-model#DM-DDL-8]]) — pinning it again
would break single-home.

RD-DDL-1 — `attachment` (object-store references; never blobs):

```sql
CREATE TABLE attachment (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  entity_type  text NOT NULL CHECK (entity_type IN ('person','organization','deal','activity','lead')),
  entity_id    uuid NOT NULL,
  filename     text NOT NULL,
  content_type text NULL,
  byte_size    bigint NULL,
  storage_key  text NOT NULL,      -- S3/MinIO object key
  checksum     text NULL,          -- sha256 for dedupe/integrity
  source       text NOT NULL,
  captured_by  text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL
);
CREATE INDEX idx_attachment_entity ON attachment (workspace_id, entity_type, entity_id) WHERE archived_at IS NULL;
```

Note RD-DDL-N-1: the corpus flags OQ-7 (S3 object-store refs vs a generic
pluggable-backend file table). ADR-0051 has since settled the *backend* question
(one S3-API seam, sovereign default); the row shape above stands as pinned.

RD-DDL-2 — `quota` (per-owner/team revenue target per period; the forecasting
chapter cites this table, never redefines it):

```sql
CREATE TABLE quota (                                       -- per-owner/team revenue target per period (E09 forecast attainment)
  -- + base columns + version
  owner_id      uuid NULL REFERENCES app_user(id),
  team_id       uuid NULL REFERENCES team(id),
  period_start  date NOT NULL,
  period_end    date NOT NULL,
  target_minor  bigint NOT NULL,
  currency      char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  CHECK ((owner_id IS NOT NULL) <> (team_id IS NOT NULL))   -- exactly one of owner/team
);
```

RD-DDL-3 — `bulk_operation` (async bulk job; single-homed here per the
ownership index):

```sql
CREATE TABLE bulk_operation (                            -- async bulk job over many records (B-E11.21a/.21b)
  kind          text NOT NULL,                           -- 'edit' | 'reassign' | 'delete' | 'export' | … (per-kind handler)
  status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','done','failed')),
  total         int  NOT NULL DEFAULT 0,
  succeeded     int  NOT NULL DEFAULT 0,
  failed        int  NOT NULL DEFAULT 0,
  request_payload jsonb NOT NULL,                         -- the selection + the change to apply
  result_summary  jsonb NULL,                             -- per-item outcomes / error digest
  idempotency_key text  NULL,                             -- client-supplied; dedupes retried submits
  requested_by  uuid NOT NULL REFERENCES app_user(id)
);
CREATE UNIQUE INDEX idx_bulk_idem ON bulk_operation (workspace_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_bulk_status ON bulk_operation (workspace_id, status, created_at DESC);
```

Note RD-DDL-N-2 (honest routing): `bulk_operation`'s *behavior* traces to the
bulk-operations story family (B-E11.21a/.21b; bulk activity S-E15.5c), not to
S-E15.8. The ownership index assigns its DDL single-home to this chapter, so it
is pinned here; its acceptance and surface belong to the owning stories'
chapter, and `audit_log.batch_id` grouping + compensating-UNDO semantics stay
with the corpus note beside the DDL (never an in-place audit rewrite).

### Wire
Source: contract/crm.yaml (NET-NEW V1 RESOURCES pattern block + DEFERRED comment block) @ 5a0b29c

**Honest contract-coverage finding:** at pin time `crm.yaml` defines 81
operations and **none** belongs to this chapter. `/quotas` is promised in the
net-new-V1-resources *pattern block* ("codegen + lint MUST emit/verify"; owner
XOR team), while `/attachments` and the audit-log read API sit in the *DEFERRED*
comment ("endpoints fast-follow") — **despite** features/10 §6 promoting
`/attachments` to MVP ("schema exists — finish it", build story B-E15.23) and
field history (B-E15.26) needing an audit read. That is a corpus-internal
contradiction this chapter reports rather than papers over: the contract
extension must mint these operations before any cited operationId can resolve;
until then no prose or ticket may cite a records-depth operationId as if it
existed. The rows below pin the promised surface by path + behavior.

| ID | Element (planned path) | Behavior pinned |
|---|---|---|
| RD-WIRE-1 | `/attachments` | Round-trips files with provenance + RBAC (features/10 §6 AC): create/list/get/archive bound to an entity per RD-DDL-1; bytes via presigned blob-seam URLs (ADR-0051), never through the JSON API; upload gated by the scan states (RD-PARAM-5); every download audited as a file access; appears on the 360 timeline. Contract status: DEFERRED comment only — must be minted (contested, see finding above). |
| RD-WIRE-2 | `/quotas` | Standard resource shape per the pattern block (list cursor+sort, get, create, update If-Match [[api-conventions#API-CC-2]], archive); owner XOR team enforced (RD-DDL-2 CHECK → 422 on both/neither); target human-set. Contract status: pattern-block promise, operations not yet expanded inline. |
| RD-WIRE-3 | attainment read (on the quota resource) | Attainment is computed server-side at read time per RD-FORM-2 and returned with its decomposition (contributing closed-won deals, flagged human-set target); a failed clean-core query returns an honest error, never a cached guess. No corpus path names this read — honest gap; the contract extension decides whether it rides the quota GET or a sub-resource. |
| RD-WIRE-4 | hierarchy roll-up read | The parent link itself rides the Organization schema (`parent_org_id` is already in `crm.yaml`'s Organization schema; the write surface is the people-and-organizations chapter's wire). The tree + roll-up *read* (RD-FORM-1 totals, per-node self figures, restricted-exclusion disclosure) has no operation in the contract — honest gap, to be minted. |
| RD-WIRE-5 | field-history read | A per-record, per-field projection read over `audit_log` before/after diffs with actor/field filters (AC-field-history-3/-4); masked fields withheld identically to their values. Depends on the DEFERRED audit-log read API — honest gap, to be minted. |

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c

This chapter defines no events, and the central catalog currently defines
**none** for its objects — there is no attachment, quota, or bulk-operation
event at pin time (verified against the catalog). Note RD-EVT-N-1 (honest gap):
the bus invariant "one domain mutation → one audit row + one domain event"
([[event-bus#EVT-SEM-1]], [[audit-observability#AUD-AC-3]]) applies to
attachment and quota writes too, so the catalog needs extending when these
surfaces build; events are defined there, never here. What this chapter *leans
on*: closed-won is `deal.stage_changed` with `to_status = won`
([[deals-and-pipeline#DEAL-EVT-3]]) — attainment reads the deal table at query
time rather than consuming the stream; and field history reads audit *rows*,
not the thin `audit.appended` pointer event
([[audit-observability#AUD-EVT-1]]).

### Acceptance

#### Acceptance — the five atoms (condensed) + the formula-field boundary
Source: product/epics/E15-operational-depth.md#s-e158--account-hierarchy-attachments-formula-display-quota--field-history @ 5a0b29c; product/20-traceability.md#matrix (E15 ticket-atoms note) @ 5a0b29c; product/build-backlog/tickets/E15/B-E15.24.md#acceptance-criteria-build-verifiable @ 5a0b29c

S-E15.8 is a bundled promotion of record; tickets generate from the five child
atoms below (traceability atom note; builds B-E15.22/.23/.24/.25/.26), not the
parent.

| ID | Given/When/Then | Verification |
|---|---|---|
| RD-AC-1 | (S-E15.8a) Given a parent company, when I view it, then I can roll up its child accounts' deals/activity across the whole tree per RD-FORM-1 — bounded traversal (RD-PARAM-1), base-currency money, restricted nodes excluded and disclosed, self-only scope one toggle away. | Integration test over a seeded tree + CI benchmark (budget owned by [[people-and-organizations#PO-AC-28]]) |
| RD-AC-2 | (S-E15.8b) Given a record, when I attach a file, then it is stored as a blob-seam reference (RD-DDL-1, never bytea), appears on the timeline with provenance, inherits the record's RBAC (denied = disclosed lock, not omission), is scan-gated before download, and every download is audited as a file access. | Integration test, attachments lane + RBAC test |
| RD-AC-3 | (S-E15.8d) Given a quota, when I set owner/period/target, then attainment computes from closed-won on the clean core per RD-FORM-2, auditable like the weighted pipeline — decomposable, reconciling exactly to its contributing deals. | Integration + reconciliation test (golden-number shape) |
| RD-AC-4 | (S-E15.8d) Given the attainment read fails or the target is absent, when the surface renders, then it shows the honest error/empty state (last successful compute time; "set a target") and never a stale, padded, or AI-guessed figure. | Integration test + screen E2E ([[acceptance-standards#STATE-1]]/[[acceptance-standards#STATE-3]]) |
| RD-AC-5 | (S-E15.8e) Given a field, when I open its history, then I see who changed it from what to what, reconstructed read-only from `audit_log` before/after (no new PII store), with agent changes carrying passport/approval/evidence; erased PII never re-surfaces (scrub inheritance, RD-PARAM-6); a masked field's history is withheld like its value. | Integration test over audit fixtures + erasure-scrub test |
| RD-AC-6 | (S-E15.8c) A formula-field is a **read-only DB GENERATED column** whose value is computed by the database and surfaced on the record (and filterable/exportable like a core field) — a schema test asserts it is GENERATED, not an app-side interpreted expression. | Schema test (build-verifiable, B-E15.24 verbatim) |
| RD-AC-7 | (S-E15.8c) A **negative-scope test** asserts there is **no user-authored formula-logic surface** at runtime — defining a *new* formula's logic routes to source (ADR-0002); only display of a code-defined GENERATED column is runtime. | Negative-scope test (build-verifiable, B-E15.24 verbatim); [[scope#NEVER-1]] |

Note RD-AC-N-1 (boundary honesty): the corpus pins the formula mechanism as a
database GENERATED column (RD-AC-6) and simultaneously shows a stored generated
definition whose dependency rail reads other rows and "rolls up account tree
S-E15.8a" (AC-formula-fields-7) — but a same-row generated column cannot read
other tables. The normative bound is RD-AC-6/RD-AC-7 (database-computed,
read-only, no runtime authoring, no hot-path interpreter); how cross-record
aggregates are served under that bound (view, trigger-maintained column, or
computed read) is an open reconciliation for the contract extension, flagged
here rather than silently resolved.

#### Acceptance — features/10 §6 capability ACs (verbatim)
Source: features/10-operational-depth.md#6-records--reporting-depth-promotes-d23-hierarchy-d16-formula-fields-d110-attachments-d96-quota-d18-field-history @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| RD-AC-8 | Hierarchy is the **`organization.parent_org_id` self-FK** (`data-model §4.1`) with a cycle-prevention trigger (no org is its own ancestor); roll-up reporting traverses it via a **bounded recursive CTE** (P11/P4 budgets, `<200ms` for ≤200-org trees). Org↔org *partner* edges remain typed `relationship` rows — only the corporate parent/child tree is the denormalized FK. | Column + trigger owned by [[people-and-organizations#PO-DDL-4]]; traversal benchmark [[people-and-organizations#PO-AC-28]]; roll-up behavior RD-AC-1 |
| RD-AC-9 | `/attachments` round-trips files with provenance + RBAC; attachments appear on the 360 timeline. | Integration test (RD-AC-2); wire gap tracked in RD-WIRE-1 |
| RD-AC-10 | A quota is a real object with owner/period/target; attainment computes from closed-won on the clean core (auditable, like weighted pipeline). | RD-DDL-2 schema test + RD-AC-3 |
| RD-AC-11 | Field-level history reconstructs prior values per field from `audit_log` before/after (no new PII store). | RD-AC-5 |
| RD-AC-12 | **User-observable (Riya/Mor, S-E15.8):** roll up a parent account's whole tree, attach a contract to a deal, see a computed field, track each rep against quota, and read a single field's change history inline. | E2E across the five screens below |

#### Acceptance — account-hierarchy.html screen ACs (verbatim; implements S-E15.8a)
Source: product/30-screen-acceptance.md#account-hierarchyhtml--account-hierarchy--roll-up-implements-s-e158a @ 5a0b29c

Verification for every row: UI E2E, screen lane; states floor
[[acceptance-standards#STATE-1]]–[[acceptance-standards#STATE-4]]. Prototype
gaps recorded by the corpus (loading skeleton; over-budget/depth-limit failure
UI; cycle/conflicting-parent rejection on accept; edit-edge picker) are open
build work, not waived acceptance.

| ID | Given/When/Then (corpus text verbatim) |
|---|---|
| AC-account-hierarchy-1 | Given the hierarchy screen for Brandt Automotive Group, When it loads with scope "Whole tree (roll-up)", Then the roll-up totals show the tree-aggregated weighted pipeline, closed-won FY26, and 30-day activities computed as `roll-up(node) = self(node) + Σ roll-up(child)` (e.g. weighted = root self 38.500 € + visible children), and the badge reads "aggregated over 5 accounts". |
| AC-account-hierarchy-2 | Given the roll-up tile band, When the user clicks "Explain this roll-up", Then an explain box expands showing the formula, the root self figure, the summed children figure, the tree weighted total over the open-deal count, and an explicit line that Brandt Defense Systems is restricted and excluded — and clicking again collapses it. |
| AC-account-hierarchy-3 | Given scope "Whole tree (roll-up)" is active, When the user clicks "This account only", Then the segmented control flips active state, the section title changes to "This account only (self)", and the totals (pipeline, deals, won, activities) recompute to the root's self-only figures — and the right-rail "Weighted total" and "open deals" update to match. |
| AC-account-hierarchy-4 | Given a parent node with children (e.g. Brandt Stamping SE), When the user clicks its twist/chevron control, Then that node's child subtree collapses (hidden) and the chevron rotates; clicking again re-expands it; leaf nodes show no actionable twist. |
| AC-account-hierarchy-5 | Given the restricted node Brandt Defense Systems, When the tree renders, Then it appears as a node tagged "restricted" with a lock, its numeric cells show "—" / "no access", it carries no self figures, and an honest note states its figures are excluded from the roll-up rather than silently summed. |
| AC-account-hierarchy-6 | Given the staged "Suggested edge" card ("Add Brandt E-Mobility AG as a child"), When the user clicks "Accept edge", Then nothing is written until that click, then a new "Brandt E-Mobility AG" child is appended to the tree with provenance "typed", the card's status flips to "edge written · audited", the open-deal count and roll-up totals recompute to include it, and a toast confirms it now rolls up. |
| AC-account-hierarchy-7 | Given the staged suggested-edge card, When the user clicks "Dismiss", Then the card is removed, a toast confirms nothing was written, and the tree and roll-up totals are unchanged. |
| AC-account-hierarchy-8 | Given every node in the tree, When figures render, Then monetary values display in EUR de-DE integer minor-units with the label "EUR · ISO-4217 · integer minor-units", and the budget bar reports the bounded join at "depth 2 · 5 nodes · 23% of P11 budget". |

#### Acceptance — attachments.html screen ACs (verbatim; implements S-E15.8b)
Source: product/30-screen-acceptance.md#attachmentshtml--attachments-on-records-implements-s-e158b @ 5a0b29c

Verification for every row: UI E2E, screen lane; the staged AI-extraction rows
additionally ride the deterministic AI lane
([[acceptance-standards#STATE-5]]). Prototype gaps recorded by the corpus (the
"Visible to me" empty card not wired; upload-failure state; whole-screen
no-permission) are open build work, not waived acceptance.

| ID | Given/When/Then (corpus text verbatim) |
|---|---|
| AC-attachments-1 | Given the attachments screen for a deal, When it loads, Then the header names the record (BÄR Pharma · Packaging QA) with a link back to the deal and a "Your role" RBAC chip ("Sales · sees deal-room files"), and the file list states every file is also written to the record timeline with provenance and inherits the record's RBAC. |
| AC-attachments-2 | Given the dropzone, When the user clicks it or drops a file (dragover highlights it), Then a scanning row appears with a "Scanning…" chip + "uploading", a toast says "virus scan in progress", and after the scan completes the row flips to a green "Clean" chip with size, "uploaded by you" provenance and timestamp, and a toast confirms it was attached and written to the timeline with provenance. |
| AC-attachments-3 | Given the file list, When it renders, Then each file row shows a type-coded icon, filename (clickable for human-uploaded/agent files), a scan-status chip (Clean / Scanning / Blocked), size, provenance (human "uploaded by …" vs `bot` "captured by agent:email-sync"), and timestamp, plus Download and Details actions; clicking Download toasts that the access is audited as a file access. |
| AC-attachments-4 | Given the agent-captured QA-Validation-Requirements.docx, When it renders, Then it carries a staged AI-extraction panel headed "AI read this file — 2 fields it can ground, staged for your record (accept to persist)" listing each grounded field with value, a source quote, page/section citation marked "grounded", and a confidence dot (high / medium), AND a field not stated in the file ("Contract value") shows as "— omitted (not stated in this file)". |
| AC-attachments-5 | Given the staged extraction panel, When the user clicks "Accept 2 fields", Then the panel turns green, its heading flips to a human-provenance "2 fields accepted to the deal — original snippets retained", the accept/edit/dismiss controls are removed, and a toast confirms the fields were written to the deal "audited, with their source snippets"; When the user clicks Dismiss instead, Then the panel is removed and a toast confirms nothing was written and the file stays attached; Edit toasts that the field converts to typed-by-you with the original snippet retained. |
| AC-attachments-6 | Given a file restricted to other roles (Margin-analysis.xlsx), When the list renders, Then it appears as a locked row showing "restricted" and "visible to Finance, Deal owner — not your role" (never silently hidden), and its only action is "Request access", which toasts an audited request sent to the deal owner. |
| AC-attachments-7 | Given a file whose scan failed (old-pricelist.zip), When the list renders, Then it shows a red "Blocked" chip and "Quarantined — scan flagged an executable inside. Not downloadable.", offers no download, and a "Why was this blocked?" info action toasts the scanner reason (archive contains an .exe — policy blocks active content). |
| AC-attachments-8 | Given a file's Details action, When clicked, Then a right drawer opens with file-type, byte-exact size, SHA-256, provenance, attach timestamp, virus-scan result (ClamAV + date), visibility scope, and the timeline activity id, plus a note that every download is logged as a file-access activity (RC-7) and provenance/scan are immutable. |
| AC-attachments-9 | Given the "All / Visible to me" segmented filter, When the user switches to "Visible to me", Then rows not visible to the user's role are hidden and the All/Mine toggle reflects the active filter; the right rail "Who can see files here" maps each role (owner / Sales / Finance / client) to its visible scope and states visibility inherits the record's RBAC with no separate per-file ACL UI in V1. |

#### Acceptance — formula-fields.html screen ACs (verbatim; implements S-E15.8c)
Source: product/30-screen-acceptance.md#formula-fieldshtml--formula-computed-fields-implements-s-e158c @ 5a0b29c

Verification for every row: UI E2E, screen lane; RD-AC-6/RD-AC-7 pin the
build-verifiable boundary behind them. The corpus-recorded not-computable and
role-masked states are the honest floor ([[acceptance-standards#STATE-3]]/[[acceptance-standards#STATE-4]]);
the missing recompute-error state is open build work.

| ID | Given/When/Then (corpus text verbatim) |
|---|---|
| AC-formula-fields-1 | Given the Brandt Automotive record, When the page loads, Then the computed-field table renders five rows (Open pipeline value, Weighted pipeline, Customer age, Net revenue retention, Blended gross margin), each tagged with a "derived" Σ badge, and a "read-only computed" badge sits in the record header. |
| AC-formula-fields-2 | Given a computed value cell, When it renders, Then every value carries a read-only lock control (title "Read-only — computed, cannot be edited"), there is no editable input anywhere, and a header note states recompute happens "on every write". |
| AC-formula-fields-3 | Given Open pipeline (Σ €266,500.00 from €212k + €48.5k), Weighted pipeline, and Customer age (months since first_closed_won 2023-02-15), When the page computes, Then each value is shown in mono, two-decimal de-DE EUR with a "€" suffix (or "mo" for age), derived deterministically from the inline state — no free text. |
| AC-formula-fields-4 | Given any computed row, When I click its "Explain" (calculator) control, Then an explain box toggles open showing the named formula, each live input line, and the highlighted result token; clicking again collapses it. |
| AC-formula-fields-5 | Given the "See it recompute" right-rail driver (BÄR Pharma — Packaging QA), When I click €212k / €177k / lost, Then the active segment toggles, the win-prob label updates ("40%" or "lost"), Open and Weighted pipeline recompute and flash, and a toast reports the change ("lost" toast says the roll-up dropped it automatically) — with no save and no typing into the cell. |
| AC-formula-fields-6 | Given the AI-proposed "Account health" formula-field card, When I click "Send to development", Then the card converts to a "routed · review pending" state stating the formula logic ships as a reviewed source change in a new version — authored by the customer's engineers, a partner, or Gradion, not opened by the product itself (A39/ADR-0002 Am.1) — with a link to the development path and a toast that formula logic is reviewed code, not runtime; clicking "Dismiss" removes the card; "Edit formula" shows a draft-edit toast. (The product describes and routes the change; it does not author or open the PR itself.) |
| AC-formula-fields-7 | Given the field-definition right rail, When it renders, Then it shows the open_pipeline_eur GENERATED ALWAYS AS … STORED definition in mono plus dependency rows (reads deal.amount_minor, reads deal.status, rolls up account tree S-E15.8a). |
| AC-formula-fields-8 | Given the scope note and provenance rail, When they render, Then they assert authoring new formula logic is a reviewed source change not a runtime builder (ADR-0002 / S-E15.7 / P1), and a computed field's provenance is always "computed:server", never "typed by you". |

#### Acceptance — quota.html screen ACs (verbatim; implements S-E15.8d)
Source: product/30-screen-acceptance.md#quotahtml--quota--attainment-implements-s-e158d @ 5a0b29c

Verification for every row: UI E2E, screen lane; RD-FORM-2 reconciliation test
behind the numbers. The corpus-designed loading / no-quota / recompute-error /
no-permission states are the honest floor
([[acceptance-standards#STATE-1]]–[[acceptance-standards#STATE-4]]); the
no-permission restriction applies identically to agents under the same role
(ADR-0013).

| ID | Given/When/Then (corpus text verbatim) |
|---|---|
| AC-quota-1 | Given the Attainment state with target 280.000 € and three closed-won deals totalling 313.872 €, When the screen loads, Then the ring center reads "113%", "Closed-won this period" reads 313.872,00 €, "Target" reads 280.000,00 €, and "Gap to target" shows the over-target amount (e.g. +33.872,00 €). |
| AC-quota-2 | Given attainment ≥ 100%, When the ring renders, Then its arc is capped at a full circle and coloured the "online"/met colour; for 60–99% it is accent, below 60% the "away"/behind colour. |
| AC-quota-3 | Given attainment and the mock "64% of period elapsed", When the pace line renders, Then it shows "Target met" when ≥100%, "Ahead of pace" when ≥64%, else "Behind pace", with the matching dot colour. |
| AC-quota-4 | Given the user clicks "Explain this number", When the explain box toggles open, Then it shows the formula attainment = Σ(closed-won base_value) ÷ target in minor units, the three deal values summed, the human-set target flagged, and the resulting percentage — with a "computed server-side" provenance chip beside it. |
| AC-quota-5 | Given the contributing-deals table, When it renders, Then each row shows deal name, close date (∈ Q3 2026), a "Closed-won" pill and a counted EUR amount, the footer sums the counted total, and a note states open/lost/omitted deals are excluded (clean-core only). |
| AC-quota-6 | Given the user types a new value in the Period target field, When they click "Save target", Then the value is parsed as German-grouped integer euros, stored as a human-typed target, the ring/won/gap/pace recompute against it, and a toast confirms "Target saved as human-typed — change logged, attainment recomputed"; a zero/empty entry instead toasts "Enter a target amount in EUR" and does not save. |
| AC-quota-7 | Given the period bar, When the user clicks a non-current period (Q2 closed / Q4 not set), Then a toast explains it is read-only/closed or not yet set, and only the current Q3 2026 chip is active. |
| AC-quota-8 | Given the team roll-up rail, When it renders, Then it lists each rep's attainment percent with a mini-bar and target, and labels the method "team roll-up = Σ closed-won ÷ Σ targets · auditable". |

#### Acceptance — field-history.html screen ACs (verbatim; implements S-E15.8e)
Source: product/30-screen-acceptance.md#field-historyhtml--field-change-history-implements-s-e158e @ 5a0b29c

Verification for every row: UI E2E, screen lane over audit fixtures; RD-AC-5
behind the projection. The corpus-designed empty-filter, honest-empty-per-field,
loading, masked-field no-permission, and refuse-partial error states are the
honest floor ([[acceptance-standards#STATE-1]]–[[acceptance-standards#STATE-4]]);
in the prototype the non-happy states are demoed via toggle chips, which is open
build work, not waived acceptance.

| ID | Given/When/Then (corpus text verbatim) |
|---|---|
| AC-field-history-1 | Given the BÄR Pharma deal's field history, When the screen loads, Then a header reads "7 fields · 14 changes" and a source-of-truth note states the view is "Reconstructed from the append-only audit log audit_log" as a "read-only projection, not editable here". |
| AC-field-history-2 | Given a field group (e.g. Amount deal.amount), When it renders, Then it shows a "Current" value row, a per-field change count, and a diff timeline of changes each with a struck-through from-value, an arrow, and a highlighted to-value (empty origins shown as "— empty —" / "— created —"). |
| AC-field-history-3 | Given the actor segmented control (All actors / Human / Agent), When the user selects "Agent", Then only change rows with data-actor="agent" remain visible, any field group with no matching rows is hidden, and selecting "Human" likewise filters to human-authored rows. |
| AC-field-history-4 | Given the field selector chips (All fields / Amount / Stage / Close date), When the user clicks "Stage", Then only the Stage field group is shown and the other groups are hidden; clicking "All fields" restores all groups. |
| AC-field-history-5 | Given the topbar "Search fields…" box, When the user types text matching no field name, Then all groups hide and an "No changes match this filter" empty state appears with a "Clear filters" button that resets actor, field, and search. |
| AC-field-history-6 | Given an agent-authored change row, When the user clicks its "evidence" button, Then an evidence panel expands showing the grounding quote/event, a source link, and a confidence dot (e.g. "high — computed, not inferred" or "medium — later corrected by the owner"). |
| AC-field-history-7 | Given the computed Amount field, When the user clicks "Explain this number", Then an explain box reveals the calculation (net €148.800,00 + 19% MwSt. €28.272,00 = €177.072,00) noting money is computed in integer minor-units, never free-typed. |
| AC-field-history-8 | Given the Owner field with zero recorded changes, When the group renders, Then instead of a blank timeline it shows "Set on create and never changed — the audit log records no edits. An empty history is honest, not a gap." |
