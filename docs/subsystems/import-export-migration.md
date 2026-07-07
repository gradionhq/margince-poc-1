---
status: planned
module: backend/internal/modules/migration (importer engine + source connectors + export bundle — the poc's import and export packages fold in here) · web (settings "Data & exit" + migrate-in surfaces)
derives-from:
  - specs/spec/features/04-platform-and-compliance.md#5-data-ownership--migration-p7--the-anti-lock-in-story @ 5a0b29c
  - specs/spec/features/06-deliverability-and-migration.md#part-2--migration-from-hubspot-first-class-onboarding @ 5a0b29c
  - specs/spec/features/06-deliverability-and-migration.md#213-csv-import-ga--salesforce-connector-v1--a42 @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e114--own-your-data-leave-in-an-afternoon @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e115--hubspot-migration-in @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e116--csv-import-ga--salesforce-migration @ 5a0b29c
  - specs/spec/product/build-backlog/E11.md#d-own-your-data-leave-in-an-afternoon-s-e114--export--the-round-trip-gate @ 5a0b29c
  - specs/spec/product/build-backlog/E11.md#e-migration-in-hubspot-csv-salesforce-s-e115--s-e116--features06-part-2--213 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#settingshtml--workspace-settings-implements-s-e111345-s-e105 @ 5a0b29c
  - specs/spec/decisions/ADR-0008-lead-object-and-promotion.md @ 5a0b29c
  - margince-poc/docs/subsystems/import.md @ a11d6c08
  - margince-poc/docs/subsystems/export.md @ a11d6c08
---
# Import, export & migration — in without a services project, out in an afternoon

> The two ends of data ownership on one engine: a guided importer that brings a
> workspace in from a spreadsheet, HubSpot, or Salesforce — dry-run first, human-approved,
> idempotent, checkpoint-resumable — and a complete open-format export bundle that
> proves nothing is locked in. The round-trip gate is the promise: export a workspace,
> re-import the bundle into a clean instance, lose nothing.

## What it's for

Migration friction is the number-two incumbent pain — lock-in on the way out
(egress restrictions, five-figure migration projects) and a services-only importer on
the way in. This subsystem makes both directions a product feature: an admin migrates
a HubSpot, Salesforce, or CSV estate in herself as an onboarding step, and can at any
time take everything out as a self-serve, open-format bundle with the schema and the
audit trail included. Ease of leaving is deliberately a feature — the proof the
product never needed exit friction to retain anyone.

Its callers are the admin's workspace-settings surface (the "Data & exit" and
migrate-in sections), the onboarding journey for switchers, the release pipeline's
round-trip gate, and — as a reuser, not a caller this chapter specifies — the
overlay-to-native mode flip, which re-imports a mirrored estate through this same
engine. The scope boundary: this chapter owns the shared importer engine, the source
connectors, the export bundle, and their run records; it does not own the dedupe
formula ([[people-and-organizations]]), the lead object ([[leads-and-qualification]]),
the post-import rollback review surface ([[data-hygiene]]), or the erasure and
consent substrate ([[gdpr-platform]]).

## Principles it serves

- **P7 — own your data.** Trivial export, open formats only, documented schema, no
  throttling designed to discourage exit; import is the inbound mirror — nothing with
  data is silently dropped, and "leave in an afternoon" is sized honestly rather than
  sloganeered.
- **P11 — the clean relational core.** Migration is where the schema advantage is
  *earned*: incumbent directional associations are de-tangled into real foreign keys
  and typed relationship rows, never carried over as residue. The same honesty makes
  export re-importable without reverse-engineering — the schema ships with the data.
- **P12 — governance designed in.** Nothing lands without a mandatory dry-run, a
  plain-language validation report, and a recorded human approval; ambiguous merges
  and ambiguous association edges are surfaced as 🟡 decisions, never silently
  resolved; every stage is audited.
- **P1/P2 — opinionated, code-first.** Import never mutates the schema: unmapped
  custom properties land in a raw holding column, and promoting one to a real field
  is the separate, code-authored customization path. Incumbent workflows are
  inventoried for re-authoring, never auto-translated into config soup.
- **ADR-0008 — leads as leads.** Bulk and machine-sourced prospect rows land as
  segregated leads, never as contacts — the anti-pollution rule applied at the
  import chokepoint by construction.

## How it works

**One engine, many sources.** There is exactly one importer engine — one mapping
step, one dry-run, one dedupe path, one resume mechanism — and the sources plug into
it as connectors: the CSV general-availability importer, the HubSpot connector, and
the Salesforce connector all ride the same mapping → detangling → dry-run → approve →
resumable-run pipeline. A connector owns only reading its source honestly (pagination,
rate limits, messy-export tolerance); everything from classification onward is shared.

**The dry-run gate comes first, and it writes nothing.** Requesting an import
enqueues a dry-run that reads the source, applies the mapping, and classifies every
row — create, update, skip, or error — keyed on the natural key of source system plus
source id, without touching any entity table (AC-M5). The output is the validation
report: per-object counts of what will land, the custom-property disposition table,
the association-detangling summary with every ambiguous or discarded edge listed, the
dedupe and owner-mapping summaries, the messy-data counts, and an honest estimate of
how long the real run will take given current source rate-limit headroom. The run
advances from pending through validating to awaiting-approval and stops there.

**A human approves; then the run is idempotent and convergent.** Approval is valid
only from the awaiting-approval state and enqueues the real run as a background job,
off the hot path. Every imported row carries its source identity, so the run upserts
rather than inserts: re-running the same source converges to the same state and
creates no duplicates (AC-M6). A checkpoint cursor advances after each upsert, so a
crash, a rate-limit pause, or an overnight gap resumes from the checkpoint and ends
in the same state as an uninterrupted run (AC-M7) — the checkpoints this engine keeps
are also what the [[data-hygiene]] rollback review surface anchors batch identity to.

**Dedupe rides the one existing path.** Contacts dedupe by exact email then the
fuzzy tier, organizations by domain — the same two-tier formula the rest of the
product uses ([[people-and-organizations]] PO-F-1/PO-F-2), not a second
implementation. Exact matches merge; ambiguous candidates are surfaced for review and
never auto-merged (AC-M9). Owners map by email to existing users; unmatched owners
become placeholder owners so no deal arrives ownerless, and deactivated source users
map to archived tombstones that preserve attribution without creating seats (AC-M2).

**Custom properties never become schema.** The schema is static code, so an
incumbent's arbitrary custom properties cannot silently become columns. Every custom
property with data lands either in a mapped existing column or in the raw-import
holding column on the record — zero properties with data are dropped, and the
report's disposition table must sum to the source property count (AC-M4). Promoting
a held property to a real field is the deliberate, code-authored customization path
owned by the custom-fields chapter — a post- or pre-import act, never an import-time
schema mutation.

**Detangling is the P11 payoff.** Incumbent directional associations resolve into
the clean core: a contact's primary company becomes the current-primary employment
row on the typed relationship table (there is deliberately no organization column on
a person), secondary affiliations become additional typed relationship rows with
their role labels, deal-to-company becomes the deal's organization key,
parent-and-child companies become the organization self-reference, and
multi-object engagements attach to their primary subject with typed rows for the
rest. Ambiguity — two "primary" companies, a contact spanning orgs — is never
silently resolved: it defaults to the highest-confidence key, surfaces as a 🟡
decision, and records every discarded edge in the report. No directional-association
artifact survives (AC-M3 — the P11 gate). The Salesforce connector gets the same
treatment: its relationship model de-tangles through the same rules.

**Bulk prospect rows land as leads, by construction.** Machine-sourced intake — a
CSV of prospects, a data-provider batch — routes through a single chokepoint that
coerces the effective object kind to lead, so no adapter can write a person: a batch
of N prospects yields zero person rows and N leads, idempotent on re-run, each
audit-logged with its sourcing job. This is the ADR-0008 anti-pollution rule applied
where it bites; the segregation and promotion semantics are
[[leads-and-qualification]]'s.

**Imported sequences arrive paused.** Migrated sequence definitions are created in
a paused state with zero auto-enrollments and zero sends — re-enrolling a migrated
audience is a human act (AC-M12, owned here). When that human act happens it passes
the [[sequences-and-deliverability]] engine's own suppression and consent checks at
enrol time; the transport rules are entirely that chapter's. Unsupported branch
logic is reported as unsupported, not silently dropped, and incumbent workflows are
inventoried for re-authoring rather than auto-translated.

**Erasure and consent guard the door.** An identity on the erasure suppression list
is rejected on import — an erased subject does not reappear through a bundle
([[gdpr-platform]] GDPR-AC-4). Imported contacts carry no presumed consent: where
the source carries provable consent it lands as an import-sourced consent record on
the [[gdpr-platform]] substrate, and otherwise default-deny stands — nothing outbound
touches a migrated audience without proof.

**Rate limits are a first-class constraint, not a failure mode.** The HubSpot and
Salesforce connectors pace themselves under the source's published limits —
token-bucket pacing, exponential backoff with jitter, honoring the server's
retry-after signal — and complete without data loss and within a configured request
budget (AC-M8). Messy-export reality (duplicate contacts, no-email contacts,
orphaned deals, malformed values, vanished owners, failed attachments) never aborts
a run: each anomaly class has a defined behaviour and is counted in the report
(AC-M11). A large estate therefore imports in hours, not seconds — and the sizing is
told honestly (Parameters).

**Export is the complete bundle, off the hot path.** A sufficiently-scoped admin
requests a full export self-serve — no ticket, no sales call, no throttle designed to
drag. The request checks the export permission, writes the single audit entry for the
export at enqueue time (so a worker retry can never double-audit), records a durable
run handle, and returns; a background worker assembles the bundle into the blob
store. The bundle is open formats only: one delimited file per object — people,
organizations, deals, pipelines, stages, activities, leads, the typed relationship
edges, and the full audit trail with actor attribution intact — plus a relational
JSON dump for re-import fidelity, a files manifest, and the published schema
document generated from the canonical schema, never hand-maintained. The exported
row counts reconcile against the requester's permission-scoped queryable counts, so
nothing visible is silently missing. A seat without the export grant is refused
without disclosure, and a run in another workspace reads as not found.

**The round-trip gate closes the loop.** The build-verifiable cash-out of "leave in
an afternoon": export a seeded workspace, re-import the bundle into a clean instance
through this same engine, and record counts, relationships, and key field values
reproduce with zero data loss; re-importing the same bundle twice converges (the
idempotency guarantee doing double duty). This gate is a release blocker, and it is
the same importer the overlay mode flip later reuses.

## What's configurable

- **The mapping** — per run, not per deployment: the effective object kind and the
  column mapping. The object-kind override is what routes machine-sourced intake to
  leads; the engine honours the mapping's kind, never a per-row kind. Auto-suggested
  for CSV columns and standard incumbent objects; custom properties default to the
  raw holding column so nothing is lost by inaction.
- **The source connector** — the injected dependency that varies by run: CSV upload,
  HubSpot, or Salesforce in V1 (Pipedrive is the committed fast-follow). Connectors
  with no real backing degrade to honest no-ops behind the connector seam rather
  than fabricating data.
- **The request budget and backoff posture** — per-connector pacing under the
  source's limits (IEM-PARAM-4/5); deployment-configured, no runtime tuning UI.
- **The blob store backend** — where dry-run reports and finished bundles land; an
  injected deployment concern (cloud object storage or local store).
- **No runtime-config surface.** This chapter registers nothing in the
  [[runtime-config]] boundary: mappings are per-run data, budgets are deployment
  constants, and everything else is behaviour.

## Guarantees (enforced)

- **The dry-run writes nothing.** Validation classifies every source row and
  produces the full report with zero entity rows written; a counting test pins the
  zero-write (AC-M5).
- **Nothing lands without approval.** The real run is reachable only from the
  awaiting-approval state; there is no import path around the gate (IEM-FORM-1).
- **Runs are idempotent.** Re-running the same source upserts by natural key and
  creates zero new rows — a repeat is an update, never a duplicate (AC-M6; for
  leads, [[leads-and-qualification]] LEADS-AC-12).
- **Runs are resumable and convergent.** Kill mid-run and restart: the run resumes
  from the stored checkpoint and the final state equals one uninterrupted run
  (AC-M7).
- **Import never mutates the schema.** No DDL runs during an import; every custom
  property with data lands in a mapped column or the raw holding column, and the
  disposition table sums to the source count — nothing silently dropped (AC-M4).
- **Detangling leaves no residue.** Directional associations resolve to real FKs
  and typed relationship rows with every ambiguous edge surfaced and recorded —
  no directional-association artifacts survive (AC-M3, the P11 gate).
- **Ambiguous merges are never auto-merged.** Exact dupes collapse; fuzzy candidates
  surface for human review (AC-M9, riding PO-F-1/PO-F-2).
- **Machine-sourced intake yields zero contacts.** N bulk prospects → 0 person rows,
  N lead rows, by construction through the single chokepoint (ADR-0008;
  [[leads-and-qualification]] LEADS-AC-10).
- **Imported sequences cannot send.** Migrated sequence definitions arrive paused
  with zero enrollments and zero sends until a human re-enrolls (AC-M12).
- **Erased subjects stay erased.** An import upsert matching the erasure suppression
  list is rejected ([[gdpr-platform]] GDPR-AC-4 — cited, owned there).
- **Rate-limited sources are drained without loss.** Backoff on throttling responses
  completes the run without data loss and within the configured request budget
  (AC-M8); messy fixtures complete without aborting, every anomaly counted (AC-M11).
- **Every stage is audited.** Dry-run, approval, and real run each write exactly one
  run-level audit entry with actor, mapping version, and counts (AC-M10); the export
  is audited exactly once, at enqueue, so worker retries cannot double-audit.
- **The export bundle is complete and honest.** Every object, relationship,
  activity, files manifest, and the audit trail in open formats plus the generated
  schema doc; exported counts reconcile against the permission-scoped queryable
  counts; CSV parses and JSON validates against the shipped schema (IEM-AC-1/5).
- **The round-trip loses nothing.** Export → re-import into a clean instance
  reproduces record counts, relationships, and key field values with zero data loss;
  the gate is a release blocker (IEM-AC-2).
- **Workspace isolation and no-leak refusals hold.** Runs and bundles are scoped to
  one workspace at the database; a foreign run reads as not found, and an ungranted
  seat is refused without disclosure.

## Acceptance

Done means Mor migrates in and could leave, both without asking anyone. She points
the importer at a spreadsheet, her old HubSpot, or a Salesforce org; before anything
is written she reads a plain-language report — what will be created, merged, or
skipped, where every custom property goes and that none with data are dropped, which
associations resolved cleanly and which are waiting on her, and how long the real
run will honestly take. She approves; the run survives interruption and converges;
afterwards her data is queryable on the clean relational core with relationships
intact, her prospects are in the lead list and her contacts untouched, and the whole
run is attributable in the audit log. In the other direction she clicks export and
gets the complete open-format bundle — schema doc and audit trail included — that
re-imports into a clean instance with zero loss.

The honest states are part of done: a dry-run with errors renders them as row-level
findings, not a wall; a failed or interrupted run shows as resumable, not vanished;
the export and import runs surface the standard async-job states — queued, running,
ready, failed ([[acceptance-standards#STATE-SP-5]]); a seat without the grant gets a
refusal that leaks nothing; and an empty source imports to an honest zero, not an
error. The cross-cutting screen floor and release gates are inherited from
[[acceptance-standards]] and not restated.

Two structural build-order facts are carried honestly: the corpus contract pins no
import or export endpoints, tables, or events — the run records, wire surface, and
lifecycle events are minted by this chapter's tickets (IEM-GAP-1..3) — and the
prototype's admin surface renders neither the async-job states nor the mapping and
detangling review UI (IEM-AC-OPEN-1..2).

## Out of scope

- **The import-rollback review surface and data-quality queue** — [[data-hygiene]]
  (parallel draft). This engine provides the run checkpoints and batch identity that
  rollback anchors to; the review surface, the rollback UX, and the batch-undo
  mechanic it shares with bulk operations are pinned there and in
  [[access-and-admin]].
- **The bulk-operations engine and screen** — [[access-and-admin]]; a bulk import is
  not a bulk edit.
- **Dedupe formula and merge mechanics** — [[people-and-organizations]] (PO-F-1/2);
  this chapter only feeds candidates through them.
- **The lead object, segregation, and promotion** — [[leads-and-qualification]].
- **Sequence transport, suppression, and enrolment rules** —
  [[sequences-and-deliverability]]; this chapter pins only that imported definitions
  arrive paused (AC-M12).
- **Erasure, the suppression list, and consent semantics** — [[gdpr-platform]].
- **Promote-to-real-field** — the custom-fields chapter's code-authored path
  (fast-follow from here; the mapping surface only dispatches to it).
- **The overlay mode flip** — the overlay chapters; it reuses this importer but owns
  its own preflight and flip semantics.
- **Deliberately not built (V1):** auto-translation of incumbent workflows, live
  bidirectional sync with another CRM, tickets/products/quotes import, auto-resumed
  sequence enrollments, incremental/scheduled export (fast-follow), and any
  proprietary or binary export format — open formats by principle.

## Where it lives

Backend: the migration module (`backend/internal/modules/migration`) — the
RLS-scoped run stores, the shared importer engine, the source connectors behind the
connector seam, and the export bundle writer; runs execute as background jobs on the
platform job runner, bundles and reports land in the platform blob store. Frontend:
the settings "Data & exit" and migrate-in surfaces (`frontend/src/features/settings`).
Read next: [[people-and-organizations]] (the dedupe this engine rides),
[[leads-and-qualification]] (where bulk rows land), [[data-hygiene]] (what happens
when an import was wrong), and [[gdpr-platform]] (the consent and erasure substrate
at the door).

## Appendix

### Parameters
Source: specs/spec/features/06-deliverability-and-migration.md#29-leave-in-an-afternoon--sized-honestly @ 5a0b29c; specs/spec/features/06-deliverability-and-migration.md#25-hubspot-api-rate-limits--messy-export-reality @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| IEM-PARAM-1 | `SIZING_SMALL` | ≤ ~10k contacts, few hundred deals → **an afternoon** | Honest wall-clock for the beachhead-sized tenant: dry-run + review + real run within a few hours; review/mapping time dominates. The "leave in an afternoon" claim applies to this class (and IEM-PARAM-2's low end) — data + standard objects only; workflow re-authoring and promote-to-field are separate sized work. |
| IEM-PARAM-2 | `SIZING_MID` | ~10k–100k contacts, tens of thousands of engagements → **overnight to a day** | Source API rate limits on engagement + attachment reads dominate. |
| IEM-PARAM-3 | `SIZING_LARGE` | >100k contacts, heavy custom-property/workflow estate → **days, staged, often services-assisted** | Never claimed as an afternoon — that overclaim is exactly what the corpus review flagged. |
| IEM-PARAM-4 | `CONNECTOR_BACKOFF` | token-bucket pacing + exponential backoff with jitter; server `Retry-After` honored as authoritative | The class of behaviour is corpus-pinned; exact bucket rates are implementation defaults fixed at ticket time (registry note below). |
| IEM-PARAM-5 | `RUN_REQUEST_BUDGET` | deployment-configured cap on source-API requests per run | The budget AC-M8 asserts the connector never exceeds; no corpus numeric constant exists. |

Registry note: the corpus pins the sizing classes and the backoff behaviour, not
numeric constants for IEM-PARAM-4/5 — those are implementation defaults to fix at
ticket time. Adjacent, not ours: the REST-side rate limits on this surface are
[[api-conventions]] rows RL-EXPORT (2 req/min), RL-BULK-WRITE (1 req/s), and
CAP-BODY (10 MB request-body cap, import only) — cited in Limits, owned there.

### Formulas
Source: specs/spec/features/06-deliverability-and-migration.md#27-idempotent-resumable-dry-run-first-import @ 5a0b29c; margince-poc/docs/subsystems/import.md#how-it-works @ a11d6c08

**IEM-FORM-1 — the import pipeline (three gates, deterministic).** Inputs: a source
(seeded, uploaded, or connector-backed), a mapping (effective object kind + column
map), and the workspace state (existing rows, erasure suppression list).

```
function run_import(source, mapping) -> final_state:
  # GATE 1 — dry-run: classify, write NOTHING (AC-M5)
  for row in source.rows():
      key = (workspace, row.source_system, row.source_id)      # natural key
      kind = mapping.object_kind                               # effective-kind override —
      if source.is_machine_sourced: kind = LEAD                # the ADR-0008 chokepoint
      if erasure_suppressed(row.identity): classify ERROR      # GDPR-AC-4, owned by gdpr-platform
      elif invalid(row, mapping):          classify ERROR      # bad enum / missing mandatory / bad map
      elif exists(key):                    classify UPDATE
      elif dedupe_hit(row):                classify MERGE-CANDIDATE   # PO-F-1/PO-F-2; ambiguous → surfaced 🟡
      else:                                classify CREATE
  write validation_report to blobstore; state = AWAITING_APPROVAL

  # GATE 2 — approve, then idempotent + resumable run (AC-M6/M7)
  require state == AWAITING_APPROVAL                           # the only door
  for row in source.rows[checkpoint.offset:]:
      upsert_by(key); detangle_associations(row)               # IEM-FORM-2
      checkpoint.offset += 1                                   # resume point, advanced per upsert

  # GATE 3 — audited at each gate (AC-M10)
  exactly one run-level audit entry at: dry-run start, approve, completion
```

Output: a converged workspace plus the validation report. Tie-breaks: suppression
beats everything (an erased identity is an error before it is a dupe); an ambiguous
dedupe candidate surfaces rather than merging; a kill mid-run resumes from the
checkpoint, never from zero and never past it. Worked example (the corpus
`import-batch-N` fixture, N=50): 50 machine-sourced prospects import as 0 person
rows and 50 leads; re-running the same batch classifies 50 UPDATEs and creates 0 new
rows; killing the run at offset 30 and restarting yields the identical 50-lead end
state as one uninterrupted run.

**IEM-FORM-2 — association detangling (the P11 rules).**
Source: specs/spec/features/06-deliverability-and-migration.md#22-the-directional-association--clean-fk-detangling-the-p11-advantage @ 5a0b29c

Inputs: the source's directional, typed, many-to-many association edges. Output:
real FKs + typed relationship rows, never edge residue.

```
function detangle(edge) -> target:
  contact → primary company    => relationship employment row, is_current_primary
                                  (person.title is a denormalized display copy;
                                   there is NO person.organization_id column)
  contact → secondary company  => additional relationship employment rows (role kept)
  contact ↔ deal (role)        => relationship deal↔person stakeholder rows (role kept)
  company ↔ company parent     => organization.parent_org_id FK (cycle-guarded; not an edge)
  deal → company               => deal.organization_id FK (primary)
  engagement ↔ many objects    => activity FK to primary subject + relationship rows for the rest

  if ambiguous(edge):          # two "primaries", contact across orgs, …
      default to highest-confidence FK
      surface as 🟡 decision; record every discarded edge in the report   # never silent
```

Tie-breaks: ambiguity is never silently resolved (AC-M3). Worked example
(the AC-M3 fixture): a contact carrying two "primary" company associations imports
with the higher-confidence company as the single current-primary employment row, the
other as a plain employment row, and the demotion recorded in the
detangling summary as an ambiguous edge awaiting Mor's 🟡 disposition; the fixture's
parent/child companies land on the organization self-FK, and zero
directional-association artifacts survive. The same rules run for Salesforce
(IEM-AC-8).

### Schema
Source: specs/spec/contract/data-model.md (ownership index — no import/export tables among the 66) @ 5a0b29c; margince-poc/docs/subsystems/import.md#how-it-works @ a11d6c08; margince-poc/docs/subsystems/export.md#how-it-works @ a11d6c08

Ownership verified against the data-model chapter's ownership index: the corpus
66-table partition contains **no** import-run, export-run, or checkpoint table, and
the deferred-tables stub list names none either — this chapter's run records are an
honest gap the corpus never pinned. The DDL below is therefore **net-new normative
shape** following the `event_outbox` precedent (additional to the 66-table
partition, defined by its owning chapter), **provisional until the contract
extension that ratifies this chapter's wire surface** (IEM-GAP-2); the ratifying
ticket may adjust columns, but the pinned behaviours (dry-run zero-write,
approval-only transition, checkpoint resume, per-gate audit) are fixed by the
Acceptance pins regardless. Shape is seeded by the shipped poc's run records.

**IEM-DDL-1 — `import_run` (net-new; provisional).**

```sql
CREATE TABLE import_run (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  connector     text NOT NULL CHECK (connector IN ('csv','hubspot','salesforce','bundle')),
  status        text NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','validating','awaiting_approval','running','complete','failed')),
  mapping       jsonb NOT NULL,            -- effective object kind + column map (per run, versioned)
  source_ref    text NOT NULL,             -- blobstore payload / connector cursor context
  report_ref    text NULL,                 -- validation report (blobstore, open-format JSON)
  checkpoint    integer NOT NULL DEFAULT 0, -- absolute offset into source rows; 0 = not started
  source        text NOT NULL,             -- provenance (DM-CONV-11)
  captured_by   text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_import_run_ws ON import_run (workspace_id, status);
```

**IEM-DDL-2 — `export_run` (net-new; provisional).**

```sql
CREATE TABLE export_run (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  status        text NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','running','complete','failed')),
  bundle_ref    text NULL,                 -- finished bundle in the blob store
  source        text NOT NULL,             -- provenance; the enqueue step writes the ONE audit entry
  captured_by   text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_export_run_ws ON export_run (workspace_id, status);
```

Both tables run under forced row-level security with the workspace policy (the
tenant-isolation floor); the run-status lifecycle maps onto the STATE-SP-5 async-job
UX vocabulary (queued/running/ready/failed) at the surface.

**IEM-GAP-1 — the raw-import holding column (naming collision to resolve at ticket
time).** The feature spec is normative that unmapped custom properties land in a
typed `raw_import` JSONB sidecar; the contract data-model pins only the DM-CONV-11
`raw jsonb` provenance column, and the build backlog names a "`raw`/`raw_import`
JSONB column convention" dependency. Whether `raw_import` is the existing DM-CONV-11
`raw` column or a distinct sidecar column on person/organization/deal must be
resolved by the ticket that lands custom-property preservation; the pinned behaviour
(AC-M4 — zero properties with data dropped, disposition table sums to source count)
binds either way. Imported rows otherwise reuse existing owned tables — `lead`
([[leads-and-qualification]] LEADS-DDL-1, `uq_lead_source`), `person`/`organization`/
`relationship` ([[people-and-organizations]]), `deal`/`pipeline`/`stage`
([[deals-and-pipeline]]), `activity` ([[activities-and-timeline]]) — no new entity
schema.

### Wire
Source: specs/spec/contract/crm.yaml (no import/export paths; not in the deferred-stub block either) @ 5a0b29c; specs/spec/product/build-backlog/E11.md#d-own-your-data-leave-in-an-afternoon-s-e114--export--the-round-trip-gate @ 5a0b29c

**Honest contract-coverage finding (IEM-GAP-2, contract-extension item):** at pin
time the contract defines **no** import or export operation — the backlog itself is
explicit that "no `/exports` path is pinned in crm.yaml" and the endpoint/job shape
is this story's own; unlike sequences, these paths do not even appear in the
deferred-stub comment block. The chapter therefore pins the promised surface by
planned path + behaviour; operationIds must be minted by the contract extension
before any ticket cites one as existing.

| ID | Element (planned path) | Behavior pinned |
|---|---|---|
| IEM-WIRE-1 | create export run | Permission-gated (export grant; deny → refusal that leaks nothing); writes the single export audit entry at enqueue; returns a durable run handle immediately (off the hot path); rate class RL-EXPORT; double-gated by the anomaly detector's bulk-egress watch ([[api-conventions]]). |
| IEM-WIRE-2 | export run status / bundle fetch | STATE-SP-5 states; once complete, a reference to the finished bundle; a foreign workspace's run answers not-found, never forbidden. |
| IEM-WIRE-3 | create import run | Accepts connector + mapping + source ref; enqueues the dry-run; permission-gated (workspace-import grant: admin/manager granted, rep/read-only denied); rate class RL-BULK-WRITE; body cap CAP-BODY (10 MB, import only). |
| IEM-WIRE-4 | dry-run validation report | Read of the report (per-row resolved action + validation errors, disposition, detangling, dedupe, owner, messy-data, duration estimate); available from awaiting-approval onward. |
| IEM-WIRE-5 | approve import run | Valid **only** from `awaiting_approval` (any other state → conflict); enqueues the real run and returns immediately; the approval is one of the three audited gates. |
| IEM-WIRE-6 | import run status | Lifecycle per IEM-DDL-1; resumable-failure surfaces as a resumable state, not a dead end; foreign-workspace lookup → not-found. |

### Events
Source: specs/spec/contract/events.md (catalog — no import/export lifecycle events defined) @ 5a0b29c

Event definitions live in the central catalog ([[event-bus]]) — cited, never
redefined. Import writes rows through the core sink, so the standard entity events
fire with import provenance; export emits no domain events.

| ID | Event | Cite |
|---|---|---|
| `lead.created` | Emitted on lead insert only (never on idempotent re-run update) for every machine-sourced imported prospect | [[event-bus]] catalog row `lead.created`; anti-pollution semantics ADR-0008 / [[leads-and-qualification]] |
| `person.created` / `person.updated` | Imported contacts landing through the core sink, `source = import:<batch>` / connector-tagged | [[event-bus]] catalog rows; provenance convention [[data-model#DM-CONV-11]] |
| `deal.*` / `activity.*` / `organization.*` | Same pattern for the other imported objects | [[event-bus]] catalog |

**IEM-GAP-3 (event-catalog extension item):** the catalog defines no
import/export **run-lifecycle** events (run created / awaiting-approval / approved /
completed / failed; export ready). If the surfaces need push-driven state (rather
than polling IEM-WIRE-2/6), those IDs must be minted in the central catalog with the
IEM-GAP-2 contract extension — this chapter deliberately does not invent them here.

### Limits
Source: specs/spec/contract/api-rate-limits-and-abuse.md @ 5a0b29c

Owned upstream — cited, not restated: this surface's rate classes are
[[api-conventions]] RL-EXPORT (full export / bulk download, 2 req/min, deliberately
tight — export is the bulk-egress surface and is double-gated by the anomaly
detector), RL-BULK-WRITE (import / batch upsert, 1 req/s), and CAP-BODY (10 MB
request-body cap, import only). Outbound toward the *source* system, the only limit
this chapter owns is the per-run request budget (IEM-PARAM-5) under the
IEM-PARAM-4 backoff posture.

### Acceptance
Source: specs/spec/product/epics/E11-access-trust-exit.md#s-e114--own-your-data-leave-in-an-afternoon @ 5a0b29c; specs/spec/product/20-traceability.md @ 5a0b29c

**Owned stories** (primacy verified against the traceability register and the
[[scope]] E11 owning-chapters row, which lists this chapter):

| ID | Story | Tier | Home |
|---|---|---|---|
| S-E11.4 | Own your data: full open-format export incl. the audit log; the "leave in an afternoon" round-trip gate | V1-Must | this chapter |
| S-E11.5 | HubSpot migration in — association→FK detangling, dedupe, owner mapping, audited | V1-Must | this chapter |
| S-E11.6 | CSV import GA + Salesforce migration — dry-run, leads-as-leads, reversible (the batch-rollback mechanic itself: [[access-and-admin]] / [[data-hygiene]]) | V1-Must | this chapter |

**Data-ownership acceptance criteria (verbatim from the feature spec; corpus
bullets carry no IDs — minted here).**
Source: specs/spec/features/04-platform-and-compliance.md#5-data-ownership--migration-p7--the-anti-lock-in-story @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| IEM-AC-1 | `[MV]` A full workspace export is initiated and completed self-serve (no admin/PS intervention) and the bundle contains every core object, relationships, activities, files manifest, and `audit_log` in CSV + JSON + a schema document — asserted against a seeded workspace by a completeness test. | Backend integration lane (completeness test) |
| IEM-AC-2 | `[MV]` **"Leave in an afternoon" gate:** a round-trip test exports a seeded workspace and re-imports the bundle into a clean instance, reproducing record counts, relationships, and key field values with zero data loss. (This is the P7 verification gate.) | Backend integration lane — **release blocker** (count-reconciliation + relationship-fidelity + field-diff assertions) |
| IEM-AC-3 | `[MV]` HubSpot import of a fixture dataset produces correct FKs (no directional-association artifacts), dedupes overlapping contacts, and maps owners to CRM users — verified against an expected-result fixture. *(The single fixture AC the corpus started from; materially expanded by AC-M3 below, which is the operative P11 gate.)* | Backend integration lane (expected-result fixture) |
| IEM-AC-4 | `[MV]` Export and import operations are themselves audited (who exported what, when) in `audit_log`. | Backend integration lane (audit-completeness gate) |
| IEM-AC-5 | Export format files are valid open formats (CSV parses; JSON validates against the published schema). | Backend integration lane (format-validity test) |
| IEM-AC-6 | **User-observable (Mor, S-E11.4):** Mor exports the whole workspace herself, in one action, without contacting anyone at Gradion and without a throttle that drags it out to discourage her — and when she opens the bundle she recognizes her data in readable CSV/JSON with a schema doc that tells her what every column means, so she can hand it to another system without reverse-engineering anything. | Screen e2e lane (mechanics: IEM-AC-1/2/5) |

**Migration acceptance criteria (verbatim from the feature spec; corpus IDs
preserved — AC-M12 is owned here, honored by [[sequences-and-deliverability]]).**
Source: specs/spec/features/06-deliverability-and-migration.md#211-acceptance-criteria @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-M1 | `[MV]` **Core import correctness.** Importing a seeded HubSpot fixture produces the expected `person`/`organization`/`deal`/`pipeline`/`stage`/`activity` row counts and key field values against an expected-result fixture. *(extends `features/04` §5 AC)* | Backend integration lane |
| AC-M2 | `[MV]` **Owner mapping.** Owners are mapped to CRM users by email; unmatched owners become `unmapped_owner` placeholders (no ownerless deals); inactive owners map to archived-user tombstones. *(test on a fixture with matched/unmatched/inactive owners)* | Backend integration lane |
| AC-M3 | `[MV]` **Directional associations → clean FKs (the P11 gate).** A fixture with primary+secondary company associations, multi-deal stakeholders, and parent/child companies imports to the correct targets — a primary-company `relationship` employment row (`is_current_primary`), `deal.organization_id`, `organization.parent_org_id` — plus typed `relationship` rows for the secondary edges (there is **no** `person.organization_id` column), with **no directional-association artifacts** and every ambiguous edge recorded in the report. *(the materially expanded version of `features/04`'s single fixture AC)* | Backend integration lane (the P11 gate fixture) |
| AC-M4 | `[MV]` **Custom-property preservation.** Every HubSpot custom property with data lands either in a mapped column or in `raw_import` JSONB — **zero** properties with data are silently dropped; the report's disposition table sums to the source property count. *(test: fixture with N custom properties → assert disposition completeness)* | Backend integration lane (+ a no-DDL-during-import assertion, per the backlog) |
| AC-M5 | `[MV]` **Dry-run writes nothing.** A dry-run produces the validation report and creates **0** CRM rows. *(test: dry-run → assert row counts unchanged)* | Backend integration lane (zero-write counter test) |
| AC-M6 | `[MV]` **Idempotent re-run.** Running the real import twice over the same source produces the same final row counts (upsert by HubSpot id, no duplicates). *(test: double-run → counts unchanged)* | Backend integration lane |
| AC-M7 | `[MV]` **Resumable.** Killing the import mid-run and restarting completes with the same final state as an uninterrupted run (checkpointed cursors). *(chaos test: kill worker mid-import → resume → assert state equality)* | Backend integration lane (chaos test) |
| AC-M8 | `[MV]` **Rate-limit resilience.** Against a HubSpot mock returning `429` with `Retry-After`, the importer backs off and completes without data loss and without exceeding the configured request budget. *(test against a throttling mock)* | Backend integration lane (throttling-mock test) |
| AC-M9 | `[MV]` **Dedupe on import.** Overlapping contacts (same email) collapse to one `person`; ambiguous matches are surfaced, never auto-merged. *(reuses the `features/02` AC3.5 dedupe harness on import input)* | Backend integration lane (shared dedupe harness; formula: [[people-and-organizations]] PO-F-1/2) |
| AC-M10 | `[MV]` **Migration is audited.** Dry-run, approval, and real run each produce `audit_log` entries with actor, mapping version, and counts. *(P12 audit test, extends `features/04` §5)* | Backend integration lane (audit-completeness gate) |
| AC-M11 | `[MV]` **Messy-data tolerance.** A fixture seeded with no-email contacts, orphaned deals, malformed custom values, and a failed attachment completes the run (no abort) and reports each class with correct counts. *(test on a deliberately messy fixture)* | Backend integration lane (messy fixture) |
| AC-M-UX | **User-observable (Mor, S-E11.5).** Mor runs the migration herself — no Gradion engineer, no services engagement for the standard path. Before anything is written she sees a plain-language dry-run report: how many contacts/companies/deals will land, which custom properties go where (and that *none with data are dropped*), which HubSpot associations resolved cleanly and which are ambiguous and waiting on her, and an honest estimate of how long the real run will take. She approves, and when it finishes she can see her HubSpot data in Gradion with relationships intact and nothing silently lost. *(observable acceptance, not a CI gate; the underlying mechanics are gated by AC-M1..M11)* | Screen e2e lane (mechanics: AC-M1..M11) |
| AC-M12 | **Sequences imported paused (fast-follow gate).** Imported HubSpot sequence definitions are created in a paused state with **0** auto-enrollments and **0** sends until a human re-enrolls. *(test once sequence import lands)* | Backend integration lane (fast-follow gate; engine honoring the paused state: [[sequences-and-deliverability]]) |

**CSV GA + Salesforce acceptance criteria (verbatim from the feature spec; corpus
bullets carry no IDs — minted here).**
Source: specs/spec/features/06-deliverability-and-migration.md#213-csv-import-ga--salesforce-connector-v1--a42 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| IEM-AC-7 | `[MV]` A seeded CSV fixture imports to the expected `person`/`lead`/`organization`/`deal` rows; **bulk prospect rows create `lead`s, not `person`s** (ADR-0008 anti-pollution); the dry-run report previews create/merge/skip counts and **no rows are written before approval**. *(test: dry-run → 0 writes; approve → expected counts; bulk → 0 `person`, N `lead`)* | Backend integration lane (anti-pollution assertion shared with [[leads-and-qualification]] LEADS-AC-10) |
| IEM-AC-8 | `[MV]` A seeded Salesforce fixture imports core objects through the same dry-run→approve→resumable path with association→FK detangling and ambiguity surfaced (§2.2). *(test)* | Backend integration lane (SF fixture through the shared engine) |
| IEM-AC-9 | `[MV]` Both importers are idempotent + resumable + fully audited (re-run creates no duplicates). *(test)* | Backend integration lane (the shared coverage gate across all three sources, per the backlog) |

**Owned screen acceptance criteria (verbatim; corpus IDs preserved).** The
workspace-settings screen serves four stories; only the Data-&-exit and migrate-in
rows below are this chapter's — the remainder of the AC-settings series (roles,
audit view, autonomy, surfaces) belongs to [[access-and-admin]] and siblings.
Source: specs/spec/product/30-screen-acceptance.md#settingshtml--workspace-settings-implements-s-e111345-s-e105 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-settings-5 | (export — leave in an afternoon): Given the Data & exit section, When viewed, Then it shows "Leave in an afternoon" with an "Export everything" action listing what's included (people/orgs/deals/activities, real relational FKs CSV, relational JSON, published schema doc, append-only audit log, files manifest); the export is itself audited. | Screen e2e lane |
| AC-settings-6 | (export caveat): Given the export hero, When read, Then an honest caveat states data leaves cleanly but custom source code runs on the Gradion core/fork and is not portable to a foreign CRM. | Screen e2e lane |
| AC-settings-7 | (migrate in): Given the Migrate-in subsection, When viewed, Then it offers a HubSpot guided importer, a CSV/spreadsheet field-mapping path (both dedupe + resolve associations into real FKs), **and a Salesforce migration importer (same engine, V1-Must per S-E11.6 / A42).** ⚠️ **Prototype drift:** the 2026-06-10 mockup still shows Salesforce as a *disabled "Post-V1"* connector — that predates the S-E11.6 promotion (2026-06-22). Build to the story (Salesforce migration is V1), not the stale prototype; the next prototype iteration must enable it. | Screen e2e lane (build to the story, not the stale prototype) |

**Cited, not owned here** (each is another chapter's pin; a sanctioned restatement
carries the owner's ID):

| ID | Fact | Owner |
|---|---|---|
| LEADS-AC-10 / LEADS-AC-12 / LEADS-AC-16 | The anti-pollution test (N prospects → 0 person, N lead rows), idempotent re-import via `uq_lead_source`, and the contacts-unchanged user observation | [[leads-and-qualification]] (ADR-0008) |
| PO-F-1 / PO-F-2 | The two-tier person dedupe and the org (domain) dedupe this engine feeds import candidates through — one dedupe implementation, not two | [[people-and-organizations]] |
| GDPR-AC-4 / GDPR-DDL-1 | An identity on the erasure suppression list is rejected on import/capture upsert — an erased subject does not silently reappear; the list itself | [[gdpr-platform]] |
| GDPR-AC-1 | Consent is default-deny per purpose — imported contacts carry no presumed consent; import-sourced consent proof lands on the same substrate | [[gdpr-platform]] |
| AC4.3 / suppression semantics | A contact on the send-suppression table is excluded from any send/enrolment — the check a re-enrolled migrated audience passes | [[sequences-and-deliverability]] |
| STATE-SP-5 | Async jobs (export, bulk ops) render queued / running / ready / failed with a partial-failure path | [[acceptance-standards]] |
| RL-EXPORT / RL-BULK-WRITE / CAP-BODY | The REST rate classes and body cap on this surface | [[api-conventions]] |
| batch-undo / import rollback | A committed batch is undoable as one compensating batch; the import-rollback review surface (S-E15.4c) rides this engine's run checkpoints | [[access-and-admin]] (batch engine) / [[data-hygiene]] (review surface) |

**Open build decisions (carried honestly — the build tickets must resolve them).**
Source: specs/spec/product/30-screen-acceptance.md#settingshtml--workspace-settings-implements-s-e111345-s-e105 @ 5a0b29c (Open questions for build)

| ID | Decision needed | Verification |
|---|---|---|
| IEM-AC-OPEN-1 | The settings prototype asserts the export round-trip but renders **no async-job UX** — the queued/running/ready/failed states (STATE-SP-5) for export and import runs must be designed at ticket time. | Ticket-gate: the Data-&-exit ticket must state the job-state UX before build |
| IEM-AC-OPEN-2 | The CSV field-mapping screen, the HubSpot association→FK review, and the dedupe-confirm (🟡) UI are **not rendered** in the prototype — the mapping surface (name, type, fill rate, sample values, destination dropdown defaulting to keep-in-raw) and the ambiguous-edge disposition UX must be designed at ticket time. | Ticket-gate: the migrate-in ticket must state the mapping + review UX before build |
| IEM-AC-OPEN-3 | Contract, schema, and event gaps IEM-GAP-1..3: the raw-import column naming, the import/export wire surface, and any run-lifecycle events must be minted by a contract extension before implementation tickets can cite them. | Ticket-gate: the contract-extension ticket precedes the implementation tickets |

### Seed
Source: specs/spec/contract/seed-and-fixtures.md (fixture catalog) @ 5a0b29c

The corpus fixture catalog defines one fixture on this chapter's path —
`import-batch-N` (N=50 machine-sourced prospects; asserts anti-pollution,
idempotency, segregation), owned by the [[testing]] catalog and asserted by
[[leads-and-qualification]]. The fixtures this chapter's ACs mandate but the catalog
does not yet name — the seeded HubSpot fixture + expected-result pair (AC-M1), the
AC-M3 detangling fixture, the deliberately messy fixture (AC-M11), the throttling
mock (AC-M8), the CSV and Salesforce fixtures (IEM-AC-7/8), and the round-trip
seeded workspace (IEM-AC-2) — must be minted with this chapter's tickets and
registered in the [[testing]] fixture catalog; none is pinned here as seed data.
