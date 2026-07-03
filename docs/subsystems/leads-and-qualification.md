---
status: planned
module: backend/internal/modules/people (lead domain) · frontend/src/features/leads
derives-from:
  - margince specs/spec/features/01-core-objects.md#6-leads--leadcontact-promotion
  - margince specs/spec/contract/formulas-and-rules.md#2-leadcontact-promotion-trigger-genuine-engagement-deterministic
  - margince specs/spec/decisions/ADR-0008-lead-object-and-promotion.md
  - margince specs/spec/contract/data-model.md#8-leads-thin-segregated--adr-0008
  - margince specs/spec/product/epics/E13-leads-and-qualification.md
  - margince specs/spec/product/30-screen-acceptance.md#leadshtml--leads-list-implements-s-e131234
  - margince specs/spec/contract/events.md#54-lead
---
# Leads and qualification — machine-sourced prospects stay out of your contacts until they genuinely engage

> The lead subsystem lets the AI SDR, connectors, imports, and ICP surfacing create
> prospects at volume without ever touching the contact graph. A lead becomes a contact
> only on genuine engagement — inbound reply, meeting booked or held, or a human
> qualifying it — through a non-lossy, merge-aware, reversible, audited promotion.
> Cold outbound never promotes.

## What it's for

A CRM that runs an AI SDR and pulls from data providers, crawls, and list imports
mass-creates unengaged prospects. Written as contacts, those records would destroy the
three things the clean core sells: dedupe, relationship strength, and honest "who do we
actually know" reporting. This subsystem gives those prospects their own first-class,
deliberately thin object — the lead — segregated by construction from the contact graph,
and a single sanctioned exit: promotion into the one person model on genuine engagement.

Its callers are the bulk sourcing paths (CSV/list import, the data-provider connector,
ICP account surfacing, the overnight AI SDR), the capture subsystem (which routes inbound
engagement events to the promotion path), agents acting over the governed tool surface,
and the leads screen where reps triage and qualify. The scoring model that ranks the lead
list belongs to the lead-scoring chapter; person and organization dedupe internals belong
to people-and-organizations; browser-extension capture enforcement of the same invariant
is the acceptance-standards chapter's client-surface gate ([[acceptance-standards#GATE-CS-2]]).

## Principles it serves

- **P11 — standard objects done excellently.** The lead is a real, normalized table with
  real foreign keys, not a metadata row or a lifecycle stage on the contact. Because lead
  and person share one field model, promotion produces no cross-object reporting seam.
- **P5 / P12 — capture-first with provenance and audit.** Every lead carries its source
  and capturing identity; every promotion is one audited transaction recording trigger,
  evidence, prior state, and outcome — and is reversible as a documented recovery.
- **P1 — one opinionated model.** One thin lead shape, one promotion rule, no
  matching-rules or trigger-tuning UI. The single ratifiable extension (the engagement-signal
  trigger) ships off by default (LEADS-PARAM-1).
- **ADR-0008 — the Lead object and lead→contact promotion.** The load-bearing
  anti-pollution decision: rejects Salesforce's lossy, irreversible convert and HubSpot's
  polluted single-Contact lifecycle. Sourcing at volume lands in a segregated pool;
  engagement — never import, never an outbound touch we sent — is the only automatic way out.

## How it works

**Sourcing makes leads, never contacts.** Every bulk or machine path — import, connector,
ICP surfacing, the AI SDR, crawl — writes lead rows only. Creation is idempotent per
source record (re-running an import creates nothing new), and lead-internal dedupe rejects
a second live lead on the same email, returning the existing record instead of a silent
duplicate. Leads deduplicate only against other leads; the person-dedupe machinery never
sees them.

**Segregation is structural, not a flag.** Leads live in their own table, absent from the
queries that make up the contact graph: contact search, person dedupe, relationship
strength, "people we know" reporting. A lead has no link into the organization graph — its
company is free text, with at most a loose candidate key for routing roll-ups. On the bus
the same rule holds: a lead-created event feeds routing, scoring, and lead-list freshness
only, and never enters the contact graph ([[event-bus#EVT-SEM-5]]).

**Working the list.** Reps triage the lead list by score and route. The transparent
weighted scoring and routing model is applied to the lead object but owned by the
lead-scoring chapter; the boundary this chapter enforces is that scoring leads never reads
from or writes to the relationship signals of real contacts.

**Promotion — the one exit.** A lead becomes a person on exactly three deterministic
triggers (LEADS-FORM-1): an inbound reply or inbound contact from the prospect, a meeting
booked or held with the prospect, or a human explicitly qualifying it. An auto-response
(out-of-office, bounce) is not engagement and is filtered by a deterministic header check
(LEADS-FORM-2). Whether a captured message is genuinely inbound versus our own outbound
echo defaults to a deterministic direction check; an optional classifier may overlay it,
and when direction is ambiguous the promotion is staged for confirmation rather than
performed (LEADS-FORM-3). **A cold outbound touch we sent with no response never promotes**
— an SDR blast, a sequence step, a one-off cold email cannot manufacture contacts. This is
the load-bearing anti-pollution line.

**Promotion is a non-lossy merge, not a convert.** The promotion transaction
(LEADS-FORM-5) resolves its target through the same dedupe-merge path the people chapter
owns: if the lead matches an existing live person it merges into that person — never a
duplicate — otherwise exactly one new person is created. The lead's history, provenance,
and activities carry over with nothing orphaned; the conversion pointer runs forward from
the resulting person to its lead of origin. The lead is marked promoted and archived from
the lead list, one audit entry records trigger, evidence, prior state, and outcome, and
the promoted and created/merged events are emitted. Only now does the record become a full
participant of the clean core — searchable, dedupe-eligible, counted in relationship
strength and reporting — and only now can it carry a deal: deals attach to people and
organizations, never to a raw lead. The transition is reversible: re-demotion is a
documented recovery within audit.

**Disqualify is the lead-specific archive.** Disqualifying soft-archives the lead —
removed from default lists, retained in audit, still fetchable by id. Unconverted leads
also carry the shortest default retention of any core object, pinned by the data-model
chapter ([[data-model#DM-SEED-1]]).

## What's configurable

- **The engagement-signal trigger switch** — the optional fourth trigger ("repeated
  link-clicks plus a form fill") ships disabled and requires ratification before enabling;
  get it wrong toward eager promotion and pollution returns (ADR-0008). Off by default
  (LEADS-PARAM-1); its click-count and window thresholds are LEADS-PARAM-2 and
  LEADS-PARAM-3. These are source constants — there is no runtime tuning UI (P1).
- **The inbound-vs-echo classifier** — an optional confidence overlay on the deterministic
  direction check. When absent or low-confidence, the deterministic check is authoritative
  and ambiguous promotions degrade to proposed-and-confirmed rather than automatic
  (LEADS-FORM-3).
- **Runtime custom fields on leads** — post-promotion truth: the bounded runtime
  custom-field concession (A46 / ADR-0002 Amendment 2) includes the lead among its target
  objects, superseding the earlier feature cut-line that excluded custom lead fields.
  That surface is owned by the custom-fields chapter — this chapter only notes the lead
  is in its object set.

## Guarantees (enforced)

- **Cold outbound never promotes.** Sending any outbound touch to a lead with no inbound
  response leaves it a lead; no person row appears. Held by the trigger function returning
  false for every outbound event (LEADS-FORM-1) and the cold-send-no-reply test fixture.
- **Bulk sourcing creates zero contacts.** A batch of N machine-sourced prospects yields
  N leads and 0 persons; the sourcing paths can only ever write leads (LEADS-AC-10).
- **Segregation until promotion.** An unpromoted lead appears in no contact search, is
  never offered by person dedupe, contributes nothing to relationship strength, and is
  excluded from contact reporting — each asserted independently (LEADS-AC-11). A lead has
  no foreign key into the organization graph; its company is plain text, asserted by a
  schema test (LEADS-AC-2).
- **Promotion never duplicates.** A promoted lead matching an existing person merges via
  the people chapter's dedupe path with zero orphaned references; a non-matching lead
  creates exactly one person (LEADS-AC-21).
- **Promotion is non-lossy and reversible.** Every field, activity, and provenance mark on
  the lead survives on the resulting person (round-trip test); the transition is one audit
  transaction with trigger and evidence, and re-demotion is a documented recovery
  (LEADS-AC-22, LEADS-AC-23).
- **Idempotent sourcing, honest collisions.** Re-running an import creates no duplicate
  leads (unique source key); creating a lead whose email a live lead already owns is
  rejected with a conflict carrying the existing id (LEADS-AC-12, LEADS-AC-13).
- **No deal on a raw lead.** A lead must promote before it can carry a deal (LEADS-AC-24).
- **Every mutation is audited and announced.** Each create, update, disqualify, and
  promotion writes exactly one audit row and emits its domain event; promotion emits the
  specific promoted verb, never a generic update ([[event-bus#EVT-SEM-2]]).

## Acceptance

Done means: a rep or admin can source at volume and observe their contacts exactly
unchanged — new prospects appear only in the lead list, visibly raw, each with a source
badge, a score, and an eligibility state. The moment a prospect genuinely engages, it
becomes a contact automatically with its full back-history attached and the triggering
evidence visible; if the person was already known, the record merges instead of
duplicating, visibly and reversibly in the audit trail. The lead list renders its honest
states: an empty list, a not-yet-eligible lead with a locked promote action explaining
why, and disqualified leads absent from default views but retained. The cross-cutting
screen-state floor (empty / loading / error / no-permission) is inherited from the
acceptance-standards chapter and not restated; the client-surface enforcement of
lead-not-contact is likewise owned there ([[acceptance-standards#GATE-CS-2]]).

One known build gap is carried honestly as an open decision the build ticket must
resolve: the prototype's lead rows navigate toward the person detail screen, which
contradicts the invariant that a lead is not a person record — the lead-detail navigation
target is undefined (LEADS-AC-OPEN-1).

## Out of scope

- **The lead-score formula, its parameters, and the manual scoring input** (story
  S-E13.6) — owned by [[lead-scoring]]; this chapter only pins that scoring stays inside
  the segregation boundary.
- **Person/organization dedupe internals and the merge machinery** promotion reuses —
  owned by [[people-and-organizations]] (its Formulas pins for person dedupe).
- **Inbound capture and reply detection** that feed the promotion triggers — owned by
  [[capture]]; sequence auto-pause on reply by [[sequences-and-deliverability]].
- **Runtime custom fields on the lead object** — owned by [[custom-fields]].
- **The event envelope, delivery semantics, and catalog** — owned by [[event-bus]].
- Rich lead 360, lead-to-lead relationship graphs, and cross-source identity resolution
  beyond exact email remain out per the feature cut-line; richness arrives after
  promotion, on the person.

## Where it lives

Backend: the shared domain module for core objects (`backend/internal/modules/people`),
where leads sit beside persons, organizations, and deals; reached through the contract's
lead operations and the governed record verbs agents use under passport scopes.
Frontend: the leads feature (`frontend/src/features/leads`). Read next:
[[people-and-organizations]] (the person model promotion lands in), [[lead-scoring]]
(the score on every lead row), [[capture]] (where engagement events come from), and
[[event-bus]] (the lead event rows).

## Appendix

### Parameters
Source: margince specs/spec/contract/formulas-and-rules.md#24-optional-engagement-signal-trigger--off-by-default-ratifiable @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| LEADS-PARAM-1 | `PROMOTE_ON_SIGNAL` | `false` | Master switch for the optional engagement-signal promotion trigger (§2.4). Ships OFF; enabling requires ratification (ADR-0008 — the one item to ratify). |
| LEADS-PARAM-2 | `SIGNAL_CLICK_COUNT` | `3` | Distinct link-click opens required inside the signal window before the signal trigger may fire (evaluated only when `PROMOTE_ON_SIGNAL` is true). |
| LEADS-PARAM-3 | `SIGNAL_WINDOW_DAYS` | `7` | Rolling window (days) over which the signal trigger's clicks + form fill are counted. |

Registry note: the corpus §0 parameter registry does not list these three §2 tunables
(they are defined in §2.4 / the §2 tunables line only) — flagged for registry sync.
The dedupe tunables (`DEDUPE_*`) are pinned by [[people-and-organizations]]; the scoring
tunables (`LEADSCORE_*`) by [[lead-scoring]]. Cite, don't restate.

### Formulas
Source: margince specs/spec/contract/formulas-and-rules.md#21-the-three-deterministic-triggers @ 5a0b29c

**LEADS-FORM-1 — the promotion trigger (deterministic).**
Inputs: a live lead (`status IN ('new','working')`) and a captured event.
A promotion fires when **any** of the three triggers is true:

```
function should_promote(lead, event) -> bool:
  match event:
    # (a) INBOUND REPLY / inbound contact from the prospect
    INBOUND_MESSAGE:
        return event.direction == 'inbound'
           and email_belongs_to_lead(event.from_address, lead)
           and not is_autoreply(event)           # see §2.2

    # (b) MEETING booked OR held with the prospect
    MEETING_BOOKED | MEETING_HELD:
        return lead_is_attendee(lead, event)      # lead email in attendee set

    # (c) HUMAN qualify / explicit promote
    HUMAN_QUALIFY:
        return event.actor_type == 'human'        # rep clicked "promote" / set status=promoted

  # everything else (esp. OUTBOUND_MESSAGE we sent) → false
  return false
```

The explicit exclusion (asserted by the cold-send-no-reply test):

```
OUTBOUND_MESSAGE (direction='outbound', captured_by='agent:sdr'|'human:*')  →  should_promote = FALSE, always.
```

Event source mapping: `INBOUND_MESSAGE` ← `activity.kind='email' AND
activity.direction='inbound'` with sender resolving to the lead's email;
`MEETING_BOOKED`/`MEETING_HELD` ← `activity.kind='meeting' AND meeting_status IN
('booked','held')` with the lead among external attendees; `HUMAN_QUALIFY` ← an
audit-logged human promote action or a UI status change to promoted.

Output: boolean → if true, run the non-lossy promotion transaction (LEADS-FORM-5).

Worked example (corpus-given):
- SDR sends a cold email to lead `bob@globex.com`. Event = `OUTBOUND_MESSAGE`.
  `should_promote = false`. Lead stays a lead. (Cold-send-no-reply test.)
- Bob replies "tell me more." Event = `INBOUND_MESSAGE`, direction inbound, from
  `bob@globex.com`, not an autoreply → `should_promote = true` → promote, carrying the
  cold-email activity + the reply onto the new/merged person.
- Bob's mailbox sends "Out of Office until Monday." `is_autoreply = true` → no promotion.

Edge cases / tie-breaks: an inbound message from an address not on the lead (a colleague
replies) does not promote *that* lead; a meeting booked then cancelled before being held
stays promoted (booking is engagement; cancellation never auto-demotes — re-demotion is a
documented manual recovery); a lead matching an existing person on promotion merges via
the dedupe path (merge-not-duplicate test).

**LEADS-FORM-2 — `is_autoreply` (deterministic guard).**
Source: margince specs/spec/contract/formulas-and-rules.md#22-is_autoreply-deterministic-guard @ 5a0b29c

```
is_autoreply(msg) = msg.raw has header 'Auto-Submitted' != 'no'
                 OR msg.raw has 'X-Autoreply' / 'X-Autorespond'
                 OR subject matches /^(out of office|automatic reply|undeliverable|delivery status)/i
```

Auto-replies do not promote (they are not engagement). A header/regex check, fully
deterministic.

**LEADS-FORM-3 — inbound-vs-echo classification (deterministic default).**
Source: margince specs/spec/contract/formulas-and-rules.md#23-inbound-vs-echo-classification-the-one-ai-assisted-seam-with-deterministic-default @ 5a0b29c

v1 default is deterministic: the captured `direction` field (from the connector's own
folder/label — Sent vs Inbox — or the SMTP envelope) decides. An L2 classifier is an
optional confidence overlay; if absent or low-confidence, the deterministic direction
check is authoritative, and when direction is ambiguous the promotion is 🟡 (proposed)
rather than 🟢.

**LEADS-FORM-4 — the optional engagement-signal trigger (OFF by default).**
Source: margince specs/spec/contract/formulas-and-rules.md#24-optional-engagement-signal-trigger--off-by-default-ratifiable @ 5a0b29c

When enabled (post-ratification, LEADS-PARAM-1): `link_clicks ≥ 3 distinct opens AND ≥1
form_fill within a 7-day window` (thresholds LEADS-PARAM-2 / LEADS-PARAM-3). Ships
disabled.

**LEADS-FORM-5 — the promotion transaction (non-lossy).**
Source: margince specs/spec/contract/data-model.md#81-lead--person-promotion-mechanics-non-lossy-features01-64-adr-0008-4 @ 5a0b29c

One transaction:
1. Resolve target person: if the lead's email matches a live person email, **merge into
   that person** via the person dedupe/merge path (pinned by [[people-and-organizations]]
   — no duplicate); else **create** a new person.
2. Set the person's `converted_from_lead_id` to the lead (the canonical, non-lossy origin
   pointer; direction is person → lead, never a required reverse FK).
3. Carry provenance: the resulting person rows keep the lead's `source`/`captured_by`
   lineage; lead activity links relink to the person (zero orphaned FKs).
4. Set `lead.status = 'promoted'`, `lead.promoted_person_id`, `lead.promoted_at`,
   `lead.archived_at = now()` (drops from lead lists, stays fetchable by id).
5. One `audit_log` row recording trigger + evidence (which inbound email/meeting) + prior
   lead state + resulting person id; emit `lead.promoted` + `person.*` events.

Cited, not pinned here: the person-dedupe confidence formula and merge mechanics
([[people-and-organizations]] Formulas); the lead-score formula, weights, and decay
([[lead-scoring]] Formulas).

### Schema
Source: margince specs/spec/contract/data-model.md#8-leads-thin-segregated--adr-0008 @ 5a0b29c

Ownership verified against the data-model chapter's ownership index: the `lead` table is
assigned to this chapter ([[data-model]] Schema — ownership index, row `lead`).

**LEADS-DDL-1 — the `lead` table (verbatim).**

```sql
CREATE TABLE lead (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,

  full_name     text NULL,
  email         text NULL,          -- lowercased; lead-internal dedupe key
  title         text NULL,
  company_name  text NULL,          -- FREE TEXT — NOT an organization FK (ADR-0008 §1; schema test asserts no org FK)
  candidate_org_key text NULL,      -- loose text/domain key for ABM routing roll-up WITHOUT creating an org row [TS]

  status        text NOT NULL DEFAULT 'new' CHECK (status IN ('new','working','promoted','disqualified')),
  score         integer NOT NULL DEFAULT 0,    -- lead scoring (features/03 §3); computed from lead-local signals only
  owner_id      uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,

  -- idempotent bulk sourcing (features/01 §6.2 — re-import makes no dupes)
  source_system text NULL,
  source_id     text NULL,

  -- promotion linkage direction is person → lead; lead records its outcome:
  promoted_person_id uuid NULL REFERENCES person(id) ON DELETE SET NULL, -- set on promotion (audit/UX convenience; canonical pointer is person.converted_from_lead_id)
  promoted_at        timestamptz NULL,

  source        text NOT NULL,
  captured_by   text NOT NULL,      -- 'agent:sdr' | 'connector:apollo' | 'import:<batch>' | 'human:<id>'
  raw           jsonb NULL,

  search_tsv    tsvector GENERATED ALWAYS AS (
                  to_tsvector('simple', coalesce(full_name,'') || ' ' || coalesce(company_name,'') || ' ' || coalesce(title,''))
                ) STORED,

  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,

  CONSTRAINT lead_email_norm CHECK (email IS NULL OR email = lower(email))
);

-- lead-internal exact-email dedupe (features/01 §6.2 → 409 on collision among LIVE leads)
CREATE UNIQUE INDEX uq_lead_email_dedupe ON lead (workspace_id, email) WHERE email IS NOT NULL AND archived_at IS NULL;
-- idempotent re-import
CREATE UNIQUE INDEX uq_lead_source ON lead (workspace_id, source_system, source_id) WHERE source_system IS NOT NULL AND source_id IS NOT NULL;

CREATE INDEX idx_lead_ws_live   ON lead (workspace_id, status) WHERE archived_at IS NULL;
CREATE INDEX idx_lead_owner     ON lead (workspace_id, owner_id) WHERE archived_at IS NULL;
CREATE INDEX idx_lead_score     ON lead (workspace_id, score DESC) WHERE archived_at IS NULL AND status IN ('new','working'); -- triage by score
CREATE INDEX idx_lead_cand_org  ON lead (workspace_id, candidate_org_key) WHERE candidate_org_key IS NOT NULL AND archived_at IS NULL; -- ABM roll-up via loose key
CREATE INDEX idx_lead_search    ON lead USING gin (search_tsv);
```

Adjacent facts pinned elsewhere (cite, never restate): the person-side
`converted_from_lead_id` column and its partial index — [[people-and-organizations]]
Schema; the lead never appears in the typed `relationship` table —
[[people-and-organizations]]; the canonical entity-type enum including `lead` —
[[data-model#DM-CONV-17]]; the lead sort/filter allow-list —
[[data-model#DM-VOCAB-5]]; the default retention seed for unconverted leads
(365 days → anonymize) — [[data-model#DM-SEED-1]].

### Wire
Source: margince specs/spec/contract/crm.yaml (paths `/leads`, `/leads/{id}`, `/leads/{id}/promote`) @ 5a0b29c

Operations are cited by contract `operationId` — request/response shapes live in the
contract, never restated here.

| ID | operationId | Operation | Tier | Errors / headers of note |
|---|---|---|---|---|
| LEADS-WIRE-1 | `listLeads` | List leads (their OWN list, distinct from contacts; cursor-paginated; filters: status, owner, min-score, text query) | 🟢 `search_records` | 401, 403 |
| LEADS-WIRE-2 | `createLead` | Create a lead (201 + `Location`; company name is free text — no org FK) | 🟢 `create_record` | `Idempotency-Key`; **409** a live lead already owns this email (problem body carries the existing id); 422 |
| LEADS-WIRE-3 | `getLead` | Get a lead by id (round-trips the created record) | 🟢 `read_record` | 404 |
| LEADS-WIRE-4 | `updateLead` | Partial update | 🟢 `update_record` | `Idempotency-Key`; 404, 422 |
| LEADS-WIRE-5 | `disqualifyLead` | Disqualify = soft-archive (`archived_at` + `status=disqualified`; still fetchable by id) | 🟡 `update_record` | 404 |
| LEADS-WIRE-6 | `promoteLead` | Promote to person on genuine engagement — one transaction, merge-or-create (LEADS-FORM-5); request carries trigger + evidence | 🟡 confirm-first when agent-triggered | `Idempotency-Key`, approval token; **403** approval required (agent 🟡) or RBAC denied; **409** lead already promoted; 404, 422 |

Bulk/connector sourcing uses the `(source_system, source_id)` pair for idempotent
re-import (LEADS-DDL-1, `uq_lead_source`). Leads are a distinct result type in
cross-object search and never appear in the people listing.

### Events
Source: margince specs/spec/contract/events.md#54-lead @ 5a0b29c

Event definitions live in the central catalog ([[event-bus]]) — cited here, not
redefined.

| ID | Event | Cite |
|---|---|---|
| `lead.created` | Emitted on creation (manual or bulk); consumed by the context graph **only into the segregated lead view**, the overnight agent, and workflow routing | [[event-bus]] catalog row `lead.created`; semantics [[event-bus#EVT-SEM-5]] |
| `lead.updated` | Delta updates (status new→working, score recompute, owner) | [[event-bus]] catalog row `lead.updated` |
| `lead.promoted` | The promotion moment — carries promoted person id, dedupe outcome (merged vs created), trigger, and evidence reference; emitted *instead of* a generic update | [[event-bus]] catalog row `lead.promoted`; [[event-bus#EVT-SEM-2]] |
| `lead.disqualified` | The lead-specific archive | [[event-bus]] catalog row `lead.disqualified` |

The load-bearing bus rule — **lead events never enter the contact graph** — is pinned at
**[[event-bus#EVT-SEM-5]]**: a `lead.created` is recorded for routing/scoring/lead-list
freshness only, excluded from person dedupe, relationship strength, and "people we know"
until `lead.promoted` materializes a person carrying the conversion lineage and the
triggering evidence; a cold outbound touch with no reply never fires `lead.promoted`.
Stream: [[event-bus#EVT-STREAM-4]]. One-mutation-one-audit-one-event:
[[event-bus#EVT-SEM-1]].

### Acceptance
Source: margince specs/spec/product/epics/E13-leads-and-qualification.md @ 5a0b29c; margince specs/spec/product/20-traceability.md @ 5a0b29c

**Owned stories** (primacy verified against the traceability register; the epic-to-chapter
split in [[scope]] assigns E13 to this chapter plus [[lead-scoring]]):

| ID | Story | Tier | Home |
|---|---|---|---|
| S-E13.1 | Bulk-sourced prospects land as leads, not contacts | V1-Must | this chapter |
| S-E13.2 | Work a scored, routed lead list, segregated from real relationships | V1-Must | this chapter (the scoring model itself: [[lead-scoring]]) |
| S-E13.3 | Lead auto-becomes a contact on genuine engagement, with full history | V1-WOW | this chapter |
| S-E13.4 | Promotion never duplicates — merges, reversible, audited | V1-Must | this chapter |
| S-E13.5 | AI SDR finds are leads by default; graduate only on engagement | V1-Must | this chapter |
| S-E13.6 | Manual scoring input (human-known signal feeds the transparent score) | V1-Must | **[[lead-scoring]]** — cited, not owned here |

**Feature acceptance criteria (verbatim from the feature spec).**
Source: margince specs/spec/features/01-core-objects.md#61-the-lead-object-thin-raw-machinebulk-sourced @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| LEADS-AC-1 | `POST /leads` with name+email returns 201 + stable UUID; round-trips identically on `GET`. | Backend integration lane (contract round-trip test) |
| LEADS-AC-2 | A `lead` row has **no FK into `organization`**; `company_name` is a plain text column (schema test asserts the absence of an org FK on `lead`). | Schema test |
| LEADS-AC-3 | Every persisted lead row has non-null `source` and `captured_by` (P5/P12). | Schema/provenance gate (provenance-universality release gate) |
| LEADS-AC-4 | A `lead` is **not** a `person`: a schema/test asserts `lead` and `person` are distinct tables sharing the field model, and a lead carries `status` and an eventual `converted_from_lead_id` linkage *direction* (person → lead), never the reverse FK from person being required. | Schema test |
| LEADS-AC-5 | Open lead record **p95 < 100 ms server**; save **p95 < 150 ms server**. | Performance gate (CI against seeded dataset) |
| LEADS-AC-6 | Disqualify sets `archived_at`/`status=disqualified`, removes from default lead lists, retains in audit, still fetchable by id. | Backend integration lane |
| LEADS-AC-7 | Every create/update/disqualify writes one `audit_log` row + one `lead.*` event. | Backend integration lane (audit-completeness gate) |
| LEADS-AC-8 | OpenAPI `crm.yaml` fully types `lead`; generated TS compiles (P3). | Contract-drift gate |
| LEADS-AC-9 | **User-observable:** a rep working the lead list sees raw prospects with a source badge and a score, clearly in a *separate* place from their real contacts — they can tell at a glance "this is a cold prospect, not someone I know" (S-E13.2). | Screen e2e lane |

Source: margince specs/spec/features/01-core-objects.md#62-bulk-machine-sourced-creation-makes-leads-segregated-by-construction @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| LEADS-AC-10 | **The anti-pollution test (load-bearing):** an Apollo/import batch of N prospects creates **0 `person` rows and N `lead` rows** — asserted by an automated test against the connector/import path. | Backend integration lane |
| LEADS-AC-11 | **Segregation test:** the created leads do **not** appear in contact search, are **not** offered as candidates by the §1.3 person-dedupe, are **not** included in the §1.2 relationship-strength computation, and are **excluded by default** from `03` "contact"/relationship reporting — each asserted independently. | Backend integration lane (four independent assertions) |
| LEADS-AC-12 | Re-running the same import is idempotent: same source ids → no duplicate leads (unique constraint on `(source_system, source_id)` for leads). | Backend integration lane |
| LEADS-AC-13 | Lead-internal create with an email already on a non-archived lead returns 409 + the existing lead id (no silent dup). | Backend integration lane |
| LEADS-AC-14 | Routing decision on a new lead **p95 < 250 ms** (`03` §3.5 budget); batch lead-score recompute of 100k leads < 5 min (River, off hot path). | Performance gate (routing budget here; scoring internals: [[lead-scoring]]) |
| LEADS-AC-15 | Every bulk-created lead is audit-logged with the sourcing agent/job. | Backend integration lane |
| LEADS-AC-16 | **User-observable:** after a rep or Mor imports a list / lets the connector run, their **contacts are unchanged** — the new prospects show up only in the lead list, and contact dedupe, relationship strength, and "people we know" reporting are untouched (S-E13.1, S-E13.5). | Screen e2e lane |

Source: margince specs/spec/features/01-core-objects.md#63-lead-scoring--routing-applies-the-existing-3-model-to-the-lead-object @ 5a0b29c
(the scoring/routing AC set — inherited AC-S1..AC-S8 applied to leads — is owned by
[[lead-scoring]]; pinned here are only the segregation-boundary rows this chapter holds)

| ID | Given/When/Then | Verification |
|---|---|---|
| LEADS-AC-17 | A lead's score and behavioral signals are computed from signals on the **lead** and do **not** read from / write to the relationship-strength of the contact graph (the segregation boundary holds inside scoring too). | Backend integration lane |
| LEADS-AC-18 | **User-observable:** a rep working the lead list can sort/triage by score, see who it was routed to and why, and click a score to read the weighted factors and open the source signals behind it — without any of this touching their real contacts (S-E13.2). | Screen e2e lane |

Source: margince specs/spec/features/01-core-objects.md#64-promotion-leadperson-on-genuine-engagement-non-lossy-merge @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| LEADS-AC-19 | **Cold-send-no-reply test (the load-bearing one):** sending a cold outbound message to a lead with **no inbound response** leaves it a **`lead`** — no `person` row is created (automated test). | Backend integration lane (fixture: cold-send-lead) |
| LEADS-AC-20 | **Inbound-reply test:** an inbound reply (or a booked/held meeting) against a lead **promotes** it to a `person` with the lead's history preserved (activities/source/score carried), `converted_from_lead_id` set, the lead marked `status=promoted` and archived from the lead list (automated test). | Backend integration lane (fixture: engaging-lead) |
| LEADS-AC-21 | **Merge-not-duplicate test:** promoting a lead whose email/identity matches an existing non-archived `person` **merges into that person via the §1.3 path** — no duplicate `person` is created; all lead activities relink to the existing person with zero orphaned FKs (referential-integrity test). | Backend integration lane (fixture: promote-into-existing) |
| LEADS-AC-22 | Promotion is **non-lossy**: every field/activity/provenance on the lead survives on the resulting person; asserted by a round-trip test. | Backend integration lane |
| LEADS-AC-23 | Promotion is one `audit_log` transaction recording trigger, evidence (which inbound email/meeting), prior lead state, and resulting person id; emits `lead.promoted` + `person.*` events. Re-demote is reversible within audit (documented recovery). | Backend integration lane |
| LEADS-AC-24 | A promoted person becomes a **full participant** of the clean core: now visible in contact search, eligible for §1.3 dedupe, included in relationship-strength and contact reporting — and **only now** can carry a `deal` (deals attach to person/org, never to a raw lead; ADR-0008 §5). | Backend integration lane |
| LEADS-AC-25 | Promotion path **p95 < 250 ms server** for the synchronous create/merge. | Performance gate |
| LEADS-AC-26 | **User-observable:** the moment a prospect genuinely engages — replies, or a meeting is booked — that lead **becomes a real contact automatically, with the full back-history attached**, and the rep did *not* trigger it by merely importing or blasting them; the contact list stays clean-by-construction (S-E13.3). | Screen e2e lane |
| LEADS-AC-27 | **User-observable:** if we already knew that person, promotion **merges into the one record** rather than creating a second — the rep/Mor never ends up with a duplicate, and the change is visible in the audit trail and reversible (S-E13.4). | Screen e2e lane |

**Leads screen acceptance criteria (verbatim; corpus IDs preserved).**
Source: margince specs/spec/product/30-screen-acceptance.md#leadshtml--leads-list-implements-s-e131234 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-leads-1 | Given the list loads, When it renders, Then a segregation notice states leads are "machine-sourced prospects, kept separate by construction", excluded from contacts/search/dedupe/relationship-strength/reporting, and promote only on inbound reply, meeting booked-or-held, or human qualify — "cold outbound never promotes (ADR-0008)". | Screen e2e lane |
| AC-leads-2 | Given the section label, When it renders, Then it reads "Leads" with count "N sourced · 0 in contacts", and the column header is Lead / Score / Source / Captured / Eligibility / (action). | Screen e2e lane |
| AC-leads-3 | Given each lead row, When it renders, Then it shows initials logo, name, a "Lead" tag, "{title} @ {company}", a why-line with an up-arrow (eligible) or dashed-circle (not) icon, the score block, a source chip (cold-start/data provider/import), captured date, an eligibility chip, and an action cell — visually distinct (accent-tinted) from contact rows. | Screen e2e lane |
| AC-leads-4 | Given a lead score block, When I hover it, Then the popover shows "Transparent weighted-signal model — not ML. 14-day half-life on behavioral points, clamped 0–100", a Fit group with signed point rows, a Behavioral (decayed) group (or "none captured · 0"), a total line showing the arithmetic, and the decay note — every point traceable. | Screen e2e lane (score content: [[lead-scoring]]) |
| AC-leads-5 | Given a lead with genuine engagement, When the row renders, Then Eligibility shows "eligible" and the action cell shows an enabled "Promote" button. | Screen e2e lane |
| AC-leads-6 | Given a lead with only cold outbound or no engagement, When the row renders, Then Eligibility shows "not eligible", the action is a disabled locked "Not eligible" button (title "No genuine engagement yet"), and the cold-outbound behavioral contribution is +0 (ADR-0008). | Screen e2e lane |
| AC-leads-7 | Given an eligible lead, When I click "Promote", Then it is promoted to a contact carrying its full history and the engagement evidence (toast confirms). | Screen e2e lane |
| AC-leads-8 | Given the score color thresholds, When a score renders, Then the bar follows the same ≥60/40–59/<40 convention as contacts/companies. | Screen e2e lane |
| AC-leads-9 | Given I click "Sort by score", When invoked, Then the list reorders highest-first; clicking again restores capture order. | Screen e2e lane |
| AC-leads-10 | Given a behavioral signal subject to decay, When I read the score popover, Then it shows the raw→decayed math (`raw · 2^(−days/14)`) — the half-life is applied and visible. | Screen e2e lane (formula: [[lead-scoring]]) |
| AC-leads-11 | Given the header "New lead", When clicked, Then it signals manual lead entry is the rare path. | Screen e2e lane |
| AC-leads-12 | Given the footer, When it renders, Then it states promotion is "non-lossy and reversible — the lead's full history follows it into the one person model, audited." | Screen e2e lane |

The standard screen-state matrix (empty / loading / error / no-permission /
nothing-grounded) is inherited from [[acceptance-standards]] and applies to the leads
screen without restatement. The client-surface form of the segregation invariant is
[[acceptance-standards#GATE-CS-2]] (profile/page capture creates leads, never person
rows) — cited, owned there.

**Open build decision (carried honestly — the build ticket must resolve it).**
Source: margince specs/spec/product/30-screen-acceptance.md#3-cross-cutting-build-gaps-roll-up-of-the-open-questions @ 5a0b29c (gap 5)

| ID | Decision needed | Verification |
|---|---|---|
| LEADS-AC-OPEN-1 | **Lead segregation vs navigation:** the prototype's lead rows (and the LinkedIn capture surface) link a lead toward the person detail screen, which conflicts with "a lead is not a person record." The lead-detail navigation target is undefined and must be decided at ticket time; whatever is chosen must preserve LEADS-AC-2/LEADS-AC-4 (a lead is not a person) and AC-leads-3 (visually distinct rows). Related per-screen gaps: no merge/reversibility UI on promote, no human-qualify control on this screen, routing/owner assignment not surfaced, live score-decay recompute undefined. | Ticket-gate: the leads screen ticket must state the chosen target before build |
