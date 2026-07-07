---
status: planned
module: backend/internal/platform (governed add-field engine + field catalog) · web (custom-fields admin surface)
derives-from:
  - specs/spec/features/10-operational-depth.md#2-bounded-runtime-custom-fields-a46--adr-0002-amendment-2
  - specs/spec/contract/data-model.md#9-custom-fields-are-columns-not-metadata-the-worked-example
  - specs/spec/decisions/ADR-0002-source-customization-over-runtime-config.md
  - specs/spec/contract/runtime-config-surface.md#1-shipped-runtime-configuration-surfaces-normative-exhaustive
  - specs/spec/product/epics/E15-operational-depth.md#s-e157--add-a-simple-custom-field-without-code
  - specs/spec/product/30-screen-acceptance.md#custom-fieldshtml--custom-field-admin-implements-s-e157
  - specs/spec/product/build-backlog/tickets/E15/B-E15.7.md#context
---
# Custom fields — a real column in a minute, never a metadata engine

> An admin adds a simple typed field to an existing core object at runtime; after one
> 🟡 confirm it becomes a real, indexable column through a single governed schema
> change, and shows up on the 360, in search and filters, in lists, export, and the
> API like any core field. Anything structural — a new object, a relationship, logic —
> is refused with a pointer to the source path. Runtime convenience, and the
> metadata-engine tax is never paid.

## What it's for

Nearly every customer needs to track one or two attributes the product didn't ship —
a renewal date, a procurement route, a budget ceiling. The general Margince answer to
customization is source-level engineering (ADR-0002), but that decision's honestly
recorded failure mode was that a non-technical admin cannot click to add a field.
This subsystem is the single, deliberately bounded concession that closes the common
case (ADR-0002 Amendment 2): a workspace admin adds a simple typed scalar field to an
existing core object herself, at runtime, with no developer, no migration PR, and no
deploy — while everything structural stays exactly where ADR-0002 put it, in source.

Its callers are the custom-fields admin screen where the field is defined and its
lifecycle managed; the approval inbox that admits the change; every surface that then
renders the field (record 360, list and filter builders, export, the contract and the
governed agent tools); and the AI runtime, which may propose a field it repeatedly
extracts but cannot create one un-gated. The scope boundary is the chapter's whole
point: attributes on existing objects only — never new objects, relationships, or
logic.

## Principles it serves

- **P1 — opinionated over configurable.** The surface is a closed, enumerated set:
  six scalar types on five existing core objects, and nothing else. It is a bounded
  concession inside the runtime-config register ([[runtime-config#RC-12]]), not the
  first step toward a field builder.
- **P2 — the source is the configuration layer.** The concession keeps P2 honest
  rather than eroding it: by giving the common "one more attribute" case a governed
  runtime path, everything beyond it can stay source without apology — and requests
  beyond the line are refused with a pointer, never quietly absorbed.
- **P11 — clean relational core.** A custom field is a real, indexed column on the
  real table. There is no field-metadata table and no dynamic-schema interpreter on
  any hot path ([[scope#NEVER-1]]); reporting and filtering over a custom field are
  honest SQL at core-column speed, asserted by a schema test.
- **P12 — governance is designed in.** Adding a field is itself a schema change, so
  the add is 🟡 approval-gated and audit-logged — and rename and retire travel the
  same governed path.
- **ADR-0002 Amendment 2 — the bounded runtime concession.** The load-bearing line:
  *a new attribute on an existing table = runtime; a new table or relationship, or
  new behavior = source.* This chapter is that amendment's mechanics.

## How it works

**Two customization paths, one bright line.** Margince has exactly two ways to change
what the product stores. The deploy-time path is source customization: the generator
recipe scaffolds a field through migration, domain struct, contract, and types, and
ships as a reviewed release ([[generators#GEN-CMD-1]]) — that is how new objects,
relationships, and logic arrive, and the generators chapter owns the split between
the two paths. The runtime path is this chapter: a workspace admin adds a bounded
scalar field through the product itself ([[runtime-config#RC-12]]). The line between
them is structural, not procedural — an attribute on an existing table may be added
at runtime; a new table, a relationship, or behavior is source, always.

**Defining a field.** On the custom-fields admin screen the admin picks the target
object, names the field, and picks one of the six types — text, number, date,
currency, picklist, or boolean (CUSTOM-FIELDS-PARAM-1). A currency field demands its
ISO currency code; a picklist demands at least one option. The field's permanent API
identity derives from the label as a namespaced, slug-derived column name
(CUSTOM-FIELDS-PARAM-3) shown immutable before anything is created, and the screen
previews exactly the pending schema change the confirm will stage. The admin never
types anything that reaches the database as raw text — the definition is validated
against the closed type and object sets before it can even be staged.

**Approval, then one atomic act.** Confirming does not create the field; it stages a
🟡 approval, because a schema change is precisely the class of action the approval
gate exists for ([[approvals-and-concurrency]]). When a human approves, the engine —
one chokepoint, the only code in the system allowed to alter a table at runtime —
performs a single transaction: the real schema change adding the column, the catalog
row that records the field's definition, and exactly one audit entry naming who added
what type to which object and when. All three land together or not at all. The
generated change is derived only from the validated definition, never from user
free-text, and adds a nullable column — a metadata-only operation on the live table,
which inherits the table's row-level tenancy enforcement automatically.

**A real column, everywhere a core field goes.** What exists afterwards is an honest
column: indexable, filterable, groupable at the same speed as a shipped field,
backing the promise that counting by a custom field is real SQL — no interpreter, no
sidecar rows ([[data-model#DM-CONV-16]], [[scope#NEVER-1]]). On the wire the field
surfaces additively through the contract's extension mechanism: core object schemas
admit extra properties, and an extension field is marked as such so tooling can
distinguish it from core fields without the core contract changing
([[contract-pipeline#CP-EXT-1]], [[contract-pipeline#CP-EXT-2]]) — which is how the
contract-drift gate stays green while workspaces diverge. The field then participates
in search, advanced filtering, dynamic lists, reports, and export per its type,
exactly like a core field.

**Rename and retire are governed the same way.** The field's definition lives in the
catalog, and its lifecycle is deliberately conservative. A rename is an audited
catalog update of the display label — the physical column identity never moves, so
no data-moving schema change ever runs. Retiring a field is a soft retire: an
audited, 🟡-gated state change that hides the field from the API and filtering while
the column and every value in it are preserved — the engine never drops a column as
a side effect. A genuine hard purge is explicitly deferred to a separate,
more-restricted administrative operation that is its own later story. On the admin
screen this reads as archive, not delete: hidden from new records, retained in audit
and history, reversible.

**The refusal is the feature.** Ask this surface for anything structural — a new
object, a relationship or lookup between records, a formula or validation rule — and
it says no, visibly and with a route: the request is rejected with a reason pointing
to the human-led development path (the customer's engineers, a partner, or Gradion,
shipping as a reviewed source change in a new version — the ADR-0002 Amendment 1
posture; the product never scaffolds code or opens a pull request for it). The admin
screen enforces the same line pre-emptively, disabling confirmation when a field
definition smells structural. A silent acceptance beyond the boundary would be the
first brick of the metadata engine this product refuses to become.

**AI assistance stays inside the gate.** The AI layer may notice it repeatedly
extracts an attribute the schema doesn't store and *suggest* the field; the admin's
🟡 accept is what creates it, through the identical engine path. After acceptance,
the field can be backfilled from previously captured raw material — the retroactive
backfill pattern (ADR-0022) — and captured or automated writes may fill it from then
on like any other column.

## What's configurable

- **The surface itself is the knob.** Bounded runtime custom fields are one of the
  register's thirteen runtime-config rows ([[runtime-config#RC-12]]); this chapter
  owns the mechanics behind that row. There is no configuration *of* the mechanism —
  no way to widen the type set, add target objects, or relax the gate.
- **Field type and target object** — chosen per field from the closed sets
  (CUSTOM-FIELDS-PARAM-1, CUSTOM-FIELDS-PARAM-2); an unsupported type or object is
  rejected outright.
- **Picklist options** — editable per field after creation; editing the option list
  regenerates the column's value constraint from the catalog, and removing the last
  option is blocked (CUSTOM-FIELDS-PARAM-5). There is no separate options table in
  V1.
- **Required-or-not and format validation** — the bounded validation surface exists
  but is its own register row (RC-13), owned elsewhere; this chapter's fields are
  created nullable and gain validation through that surface, not through logic
  attached here.
- **No per-object field cap is pinned.** The corpus sets no maximum number of custom
  fields per object (CUSTOM-FIELDS-PARAM-6) — carried honestly as a decision the
  build ticket must make or explicitly decline.

## Guarantees (enforced)

- **A custom field is a real column — never a metadata row.** Every add executes a
  real, governed schema change producing a real, indexable column, and a schema test
  asserts no field-metadata, dynamic-schema, EAV, or JSON-blob table backs any field
  ([[scope#NEVER-1]], [[data-model#DM-CONV-16]]; CUSTOM-FIELDS-AC-1).
- **The add is atomic and governed.** No column appears without the gate admitting
  it, and gate-admitted execution lands the schema change, the catalog row, and
  exactly one audit entry in a single transaction — a test asserts the three are
  inseparable (CUSTOM-FIELDS-AC-2, CUSTOM-FIELDS-AC-10).
- **No user text ever becomes DDL.** The generated change derives only from the
  validated, catalog-constrained definition; the column identifier is namespaced and
  slug-derived, never free text, and an injection-attempt test asserts the engine
  rejects it (CUSTOM-FIELDS-AC-12).
- **Parity with core fields.** A custom field participates in search, advanced
  filtering, lists, reports, and export exactly like a core field, verified by a
  round-trip test; the contract and generated types stay coherent through the
  extension mechanism and the drift gate stays green (CUSTOM-FIELDS-AC-3,
  [[contract-pipeline#CP-EXT-2]]).
- **The boundary refuses, with a pointer.** An attempt to create a new object or
  relationship through this surface is rejected with a reason routing to the source
  path — never silently accepted, never scaffolded into code (CUSTOM-FIELDS-AC-4,
  AC-custom-fields-5).
- **Retire never destroys data.** Soft retire hides the field while preserving the
  column and its values; a test asserts retire does not drop the column, and rename
  never changes the physical column identity (CUSTOM-FIELDS-AC-13).
- **Every lifecycle act is attributable.** Add, rename, and retire each write their
  audit entry with human attribution through the single audit seam
  ([[audit-observability]]); the admin screen shows who added each field.

## Acceptance

Done means Mor adds a "Contract end date" field to companies in about a minute:
label, type, one 🟡 confirm — and the field is live on every company record,
filterable, exportable, and present in the API, with the add sitting in the audit
trail under her name; she never touched code or waited on a deploy. Done equally
means the refusal works: asking this screen for a new object or a relationship
produces a visible refusal that routes to the development path, and the option to
do it anyway simply does not exist. The admin surface renders its honest states —
objects with no custom fields say so rather than showing an empty table, a staged
field is visibly pending until the schema change commits, archiving visibly dims
rather than deletes, and confirmation is disabled until the definition is valid.
The testable form of every claim lives in the Acceptance appendix; the
cross-cutting screen-state and gate floor is inherited from the acceptance-standards
chapter and not restated.

## Out of scope

- **Source customization** — new objects, relationships, workflows, formula and
  validation logic, and the generator recipes that scaffold them — owned by the
  generators chapter ([[generators]]) under ADR-0002; this chapter only enforces the
  refusal that routes there.
- **The approval gate's own mechanics** — token minting, expiry, staged re-validation
  — owned by [[approvals-and-concurrency]]; this chapter is one client of the gate.
- **Bounded required-field and format validation** (the register's RC-13 row) — a
  separate runtime surface; fields created here are nullable until it applies.
- **Formula/calculated-field display and per-field change history** — the records &
  reporting depth story (S-E15.8), not this surface.
- **Field-level visibility masking** — the role-scoped field-mask machinery is owned
  by the access-and-admin work; a custom field is subject to it like any core field.
- **Sandbox rehearsal of a risky field add** (S-E15.9b) — owned by the sandbox story;
  this chapter's path is the production path it rehearses.

## Where it lives

Backend: the platform layer (`backend/internal/platform`), beside the migration
machinery it extends — the engine is a schema-change chokepoint above any single
domain module, and the columns it creates land on the domain tables the object
chapters own. Frontend: the custom-fields admin surface in the web shell. Read next:
[[runtime-config]] (the register row this implements), [[generators]] (the other
side of the bright line), [[approvals-and-concurrency]] (the gate every add passes),
and [[contract-pipeline]] (how an extension field rides the contract).

## Appendix

### Parameters
Source: specs/spec/features/10-operational-depth.md#2-bounded-runtime-custom-fields-a46--adr-0002-amendment-2 @ 5a0b29c
Spec-gate decisions (GH margince-poc #129, approved 2026-06-29): specs/spec/product/build-backlog/tickets/E15/B-E15.7.md#context @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| CUSTOM-FIELDS-PARAM-1 | Field types | `text / number / date / currency / picklist / boolean` | The closed set of six scalar types. An unsupported type is rejected (gate Q3). |
| CUSTOM-FIELDS-PARAM-2 | Target objects | `person / organization / deal / lead / activity` | Existing core objects only; the lead is in-set (supersedes the earlier feature cut-line excluding custom lead fields — see [[leads-and-qualification]]). |
| CUSTOM-FIELDS-PARAM-3 | Column namespace | `cf_` prefix, slug-derived | Physical column identifier is `cf_`-prefixed and derived from the slug, never user free-text; surfaced as API key `<object>.cf_<slug>`, immutable once live (gate Q1/Q2; AC-custom-fields-3). |
| CUSTOM-FIELDS-PARAM-4 | Type → storage mapping | `text`→`text` · `number`→`numeric` (string round-trip, no float) · `date`→`date` · `currency`→`bigint` minor-units + ISO-4217 code in the catalog row · `boolean`→`boolean` · `picklist`→`text` + generated `CHECK` from the catalog's allowed values | Gate Q3 → a. Money follows [[data-model#DM-CONV-9]]; the picklist `CHECK` is regenerated when the option list is edited. |
| CUSTOM-FIELDS-PARAM-5 | Picklist minimum options | 1 | Removing the last option is blocked ("A picklist needs at least one option" — AC-custom-fields-4); no separate options table in V1 (gate Q3). |
| CUSTOM-FIELDS-PARAM-6 | Max custom fields per object | **not pinned** | The corpus sets no cap at 5a0b29c. Honest gap: the build ticket must pin a cap or explicitly decline one. |

### Schema
Source: specs/spec/product/build-backlog/tickets/E15/B-E15.7c.md#acceptance-criteria-build-verifiable @ 5a0b29c

**Registry finding.** The corpus data-model deliberately assigns custom fields no
table in its ownership index — a custom field is a real column added by migration,
and the mechanics are delegated to this chapter ([[data-model]] note DM-OWN-N-1).
The spec-gate decision record (Q2 → a) then ratifies a **`custom_field` catalog
table as the system-of-record** for every runtime-added column, with the column
inventory `workspace_id, object, slug, label, type, status, column_name,
created_by, …` — but **no verbatim DDL exists anywhere in the corpus at 5a0b29c**.
The DDL below is therefore this chapter's normative proposal (planned status): the
ratified inventory verbatim, completed per the universal conventions
([[data-model#DM-CONV-1]]–[[data-model#DM-CONV-4]], [[data-model#DM-CONV-12]]–[[data-model#DM-CONV-14]]).

**CUSTOM-FIELDS-SCHEMA-1 — the `custom_field` catalog (system-of-record; gate Q2 → a):**

```sql
CREATE TABLE custom_field (
  -- + base columns (id, workspace_id, created_at, updated_at, archived_at — DM-CONV-3)
  -- + version (DM-CONV-4)
  object       text NOT NULL CHECK (object IN ('person','organization','deal','lead','activity')),
  slug         text NOT NULL,                    -- admin-facing key; column_name derives from it
  label        text NOT NULL,                    -- display label; rename updates label/slug only
  type         text NOT NULL CHECK (type IN ('text','number','date','currency','picklist','boolean')),
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),  -- retire = soft (gate Q4)
  column_name  text NOT NULL,                    -- physical cf_-prefixed identifier; STABLE across rename
  currency     char(3) NULL CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),  -- required when type='currency' (gate Q3)
  options      jsonb NULL,                       -- picklist allowed values; source of the generated CHECK (gate Q3)
  created_by   uuid NOT NULL REFERENCES app_user(id)
);
CREATE UNIQUE INDEX idx_custom_field_slug ON custom_field (workspace_id, object, slug);
CREATE UNIQUE INDEX idx_custom_field_col  ON custom_field (workspace_id, object, column_name);
```

**CUSTOM-FIELDS-SCHEMA-2 — the generated per-field change (gate Q1/Q2 → a):** the
engine emits `ALTER TABLE <object> ADD COLUMN cf_<slug> <mapped-type> NULL` —
generated only from the validated, catalog-constrained spec, never raw user SQL;
runtime-executed through the one engine chokepoint in the same transaction as the
catalog row and one audit entry — **not** a per-field migration file. `ADD COLUMN`
of a nullable column is metadata-only (no table rewrite), and the column inherits
the core table's RLS. A sidecar `*_custom` table and any `field_metadata`/EAV/JSONB
backing are prohibited and schema-test-asserted (DM-AC-6 via
[[data-model#DM-CONV-16]]). Contrast: the corpus's worked example of a plain-named
column added by a source migration is the *deploy-time* path
(contract/data-model.md §9); the runtime path always namespaces with
CUSTOM-FIELDS-PARAM-3.

**Tenancy note (flagged, not resolved).** The catalog row is workspace-scoped, but
an `ADD COLUMN` on a shared multi-tenant table is physically datastore-wide; the
catalog is what scopes the field's *visibility* to its workspace. The corpus does
not address this asymmetry at 5a0b29c — carried as an open consequence for the
build ticket (single-workspace dedicated/on-prem deployments do not exhibit it).

### Wire
Source: specs/spec/contract/crm.yaml @ 5a0b29c

**Honest gap: `crm.yaml` defines no custom-field admin operations at 5a0b29c** — no
path or operationId matches the custom-field surface. The rows below pin the
operation set the screen and stories require; each is a contract addition the build
ticket must land through the contract pipeline.

| ID | Operation (required behavior) | Status @ 5a0b29c |
|---|---|---|
| CUSTOM-FIELDS-WIRE-1 | List custom fields per object (admin read backing the field table, incl. retired) | **GAP** — no operationId |
| CUSTOM-FIELDS-WIRE-2 | Create field: stages a 🟡 approval; unapproved execution returns the `ErrRequiresApproval` sentinel (contract/interfaces.md §0); on approval the engine transaction runs | **GAP** — no operationId |
| CUSTOM-FIELDS-WIRE-3 | Rename field: audited catalog `label`/`slug` update; `column_name` immutable | **GAP** — no operationId |
| CUSTOM-FIELDS-WIRE-4 | Retire field: audited, 🟡-gated soft retire; never drops the column | **GAP** — no operationId |
| CUSTOM-FIELDS-WIRE-5 | Structural request (new object / relationship / formula) → rejected with a reason pointing to the source path (ADR-0002) | **GAP** — behavior pinned by CUSTOM-FIELDS-AC-4 + AC-custom-fields-5; the error shape is the ticket's to define within the RFC 7807 conventions ([[api-conventions#API-ERR-1]]) |
| CUSTOM-FIELDS-WIRE-6 | Field *values* on core object payloads: extra properties under `additionalProperties: true`, marked `x-extension: true` — no new operations needed | Covered — [[contract-pipeline#CP-EXT-1]], [[contract-pipeline#CP-EXT-2]] (cited, not restated) |

### Events
Source: specs/spec/contract/events.md#5-the-catalog @ 5a0b29c

The event catalog defines **no custom-field lifecycle events** at 5a0b29c
(no `custom_field.*` verbs). Custom-field *value* changes ride the owning entity's
`*.updated` delta events ([[event-bus]] pins the delta rows, e.g.
`organization.updated` / `deal.updated` explicitly including custom columns). The
add/rename/retire trail is the audit entry (CUSTOM-FIELDS-AC-10), not a bus event;
whether a lifecycle event is warranted is left to the build ticket — honest gap,
not an omission by this chapter.

### Acceptance
Source: specs/spec/features/10-operational-depth.md#2-bounded-runtime-custom-fields-a46--adr-0002-amendment-2 @ 5a0b29c

Feature acceptance criteria, verbatim:

| ID | Given/When/Then | Verification |
|---|---|---|
| CUSTOM-FIELDS-AC-1 | "Adding a field triggers a **real governed `ALTER TABLE`** producing a real, indexable column — a schema test asserts there is **no** `field_metadata`/dynamic-schema table backing it (P11 honest reporting holds)." | Schema test (the [[scope#NEVER-1]] contract test; DM-AC-6) |
| CUSTOM-FIELDS-AC-2 | "The add-field op is **🟡 approval-gated** (it is a schema change) and **audit-logged** (who added what, when); contract + generated TS types regenerate so the field is first-class on the API (contract-drift check passes, P3)." | Gate + audit integration test; contract-drift gate |
| CUSTOM-FIELDS-AC-3 | "A custom field participates in search, advanced filtering (§3), lists, reports, and export exactly like a core field — verified by a round-trip test." | Round-trip test |
| CUSTOM-FIELDS-AC-4 | "An attempt to create a new **object** or **relationship** via this surface is rejected with a reason pointing to the source path (ADR-0002) — the bounded-concession guard." | Rejection test (the guard's negative half) |
| CUSTOM-FIELDS-AC-5 | "**User-observable (Mor, S-E15.7):** Mor adds a 'Contract end date' field to companies in a minute, it shows on every company and is filterable/exportable, and she never touched code or waited on a deploy — but adding a whole new *object* still routes to a development task." | End-to-end UAT walkthrough |

Source: specs/spec/product/epics/E15-operational-depth.md#s-e157--add-a-simple-custom-field-without-code @ 5a0b29c

Story S-E15.7 user-side acceptance, condensed (this chapter's single story atom —
verified: the screen index and the E15 scope cluster map only S-E15.7 to this
surface):

| ID | Given/When/Then | Verification |
|---|---|---|
| CUSTOM-FIELDS-AC-6 | Given an existing object, when I add a field of one of the six types, then after a 🟡 confirm it appears on the 360, in search/filter, lists, export, and the API. | E2E test over add → render surfaces |
| CUSTOM-FIELDS-AC-7 | Given the field is added, when I check, then it is a real column (filters and reports like any core field) and the add is in the audit trail. | Schema + audit assertion |
| CUSTOM-FIELDS-AC-8 | Given I try to create a new object or a relationship this way, when I do, then it is refused with a pointer to the development path — the concession is fields-on-existing-objects only. | Rejection test (same fixture as CUSTOM-FIELDS-AC-4) |

Source: specs/spec/product/build-backlog/tickets/E15/B-E15.7d.md#acceptance-criteria-build-verifiable @ 5a0b29c

Build-verifiable pins from the ratified spec-gate decisions (B-E15.7c/7d):

| ID | Given/When/Then | Verification |
|---|---|---|
| CUSTOM-FIELDS-AC-9 | The column is `cf_`-prefixed on the target core table, and the `custom_field` catalog row is the system-of-record; a sidecar `*_custom` table is not used (gate Q2 → a). | Schema test |
| CUSTOM-FIELDS-AC-10 | After the gate admits, the engine runs the `ALTER TABLE` in-process inside one transaction that also inserts the catalog row and writes one audit entry — the three are atomic. | Atomicity test (composition test over B-E15.7c–d) |
| CUSTOM-FIELDS-AC-11 | The six types map exactly per CUSTOM-FIELDS-PARAM-4; an unsupported type is rejected. | Type-mapping test |
| CUSTOM-FIELDS-AC-12 | The DDL is generated only from the validated, catalog-constrained field spec — never raw user SQL; the identifier is slug-derived, never free text. | Injection-attempt test |
| CUSTOM-FIELDS-AC-13 | Retire is soft (`status='retired'`; column + data preserved, hidden from API + filtering, never auto-`DROP`); rename updates `label`/`slug` with `column_name` stable; hard `DROP COLUMN` is deferred to a separate restricted op. | Retire-preserves-column test; rename-stability test |

Source: specs/spec/product/30-screen-acceptance.md#custom-fieldshtml--custom-field-admin-implements-s-e157 @ 5a0b29c

Screen acceptance (custom-fields admin), corpus IDs preserved verbatim:

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-custom-fields-1 | Given the page loads on the Deal object, When the admin views the field table, Then the three existing Deal custom fields (Renewal date, Procurement route, Budget ceiling) are listed with their immutable `cf_`-prefixed API key, type chip, and human-provenance "Added by" attribution, and core fields are explicitly excluded ("Core fields are not shown — they aren't editable here"). | Screen AC (UI test lane) |
| AC-custom-fields-2 | Given the admin clicks an object scope chip (Deal/Company/Contact/Lead), When the selection changes, Then the field table swaps to that object's fields, the builder/gate object labels update to match, and objects with no custom fields (Contact, Lead) show an honest empty state rather than an empty table. | Screen AC (UI test lane) |
| AC-custom-fields-3 | Given the builder is open, When the admin types a field Label, Then the API key auto-derives as `<object>.cf_<slug>` shown disabled (immutable once live), and the gate preview updates to the exact pending DDL (e.g. `ALTER deal ADD COLUMN cf_… (text) · backfilled NULL · reversible`). | Screen AC (UI test lane) |
| AC-custom-fields-4 | Given the admin picks a Type, When the type is Currency, Then a required ISO-4217 currency-code input appears (values stored as integer minor-units); When the type is Picklist, Then an options editor appears where options can be added and removed, and removing the last option is blocked with "A picklist needs at least one option". | Screen AC (UI test lane) |
| AC-custom-fields-5 | Given the admin enters a label containing a structural word (e.g. "object", "relationship", "link to", "lookup to"), When the label is parsed, Then a refusal banner appears stating the runtime builder is fields-on-existing-objects only (A46 / ADR-0002 Am.2), the Confirm button is disabled, and a link routes the structural change to the **human-led development path** (the customer's engineers, a partner, or Gradion — it ships as a reviewed source change in a new version, not an in-product PR; A39/ADR-0002 Am.1). | Screen AC (UI test lane) |
| AC-custom-fields-6 | Given a valid label and type, When the admin clicks "Confirm & add field", Then a staged row appears in the table marked "writing…", and on commit it loses the staged tint, gains edit/archive actions, the object count chip increments, an immutable audit-log entry is prepended (actor, field, type, object, timestamp, `audit#` id), and a toast confirms the field is live on the 360, filters, export & API. | Screen AC (UI test lane) |
| AC-custom-fields-7 | Given an existing custom field row, When the admin clicks Archive, Then the row dims and a message confirms it is hidden from new records but retained in audit & history (reversible) — fields are archived, not hard-deleted. | Screen AC (UI test lane) |
| AC-custom-fields-8 | Given the Confirm button, When no label has been entered, Then the button is disabled (and a toast "Give the field a label first" guards the action), preventing empty-field creation. | Screen AC (UI test lane) |

Open reconciliation, carried honestly:

| ID | Given/When/Then | Verification |
|---|---|---|
| CUSTOM-FIELDS-AC-OPEN-1 | The contract's sort/filter vocabulary is a closed per-resource allow-list of core columns (contract/data-model.md §13.5), yet CUSTOM-FIELDS-AC-3 promises custom fields filter like core fields. The allow-list must become catalog-extensible (active custom columns join their object's vocabulary per type; retired ones leave it) — the corpus does not state this mechanism at 5a0b29c. | Open decision for the build ticket; then a vocabulary-extension test |
