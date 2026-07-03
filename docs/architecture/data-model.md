---
derives-from:
  - margince-poc/docs/architecture/data-model.md
  - margince specs/spec/contract/data-model.md#1-conventions
  - margince specs/spec/contract/data-semantics.md#1-multi-currency--fx-roll-ups
  - margince specs/spec/contract/data-semantics.md#2-timezones
---
# Data model — the schema rules every table obeys

> One Postgres schema, one set of invariants: every table is tenant-keyed and
> RLS-enforced, every mutable row is version-guarded, every captured row says where
> it came from, and nothing is deleted — it is archived. This chapter owns the
> universal conventions, the identity/tenancy/governance tables, and the index that
> assigns every other table to exactly one owning feature chapter.

## What it's for

Every module in the backend persists through the same relational core, and every
guarantee the product makes about isolation, attribution, and correctness is held
*in the database*, not left to caller discipline. This chapter states those
cross-cutting rules once so no feature chapter restates them, defines the tables
that belong to the platform itself (workspace, users, roles, sessions, passports,
audit, consent, secrets, FX), and partitions the rest of the schema across the
feature chapters that own it. Readers are the ticket generator and any engineer or
agent writing a migration or a store.

## Principles it serves

- **P11 — clean relational core.** Real columns, real constraints, real indexes;
  reporting is honest SQL over an honest schema, never a metadata interpreter.
- **P12 — governance by construction.** Tenancy, attribution, and the append-only
  audit spine are enforced by policies and triggers, so the EU-AI-Act/GDPR posture
  is structural, not procedural.
- **P2 — source customization.** Because client- and agent-authored code touches
  the database directly, the single honest enforcement point is the database itself;
  and custom fields are real columns added by migration, not runtime metadata.
- **P5 — zero manual entry.** Provenance columns make "who typed this" measurable:
  the share of rows captured by agents versus humans is the product's own health
  metric, and it only works because provenance is non-null everywhere.

## How it works

**One tenant key, enforced by the database.** Every tenant table carries the
workspace key, and enforcement is row-level security, not application filtering
alone. Policies are forced even for the table owner, and domain queries run as a
dedicated non-superuser application role, because superusers bypass row security.
The tenant identity travels as a transaction-scoped setting issued by
connection-acquisition middleware before any domain statement; a connection with no
tenant setting matches nothing — it reads zero rows and cannot write. Pooling is
pinned to transaction mode so the setting can never leak across borrowed
connections. The four lifecycle rules are pinned as DM-CONV-5 through DM-CONV-8,
and each has a blocking conformance test. Row- and field-level RBAC layers on top
of tenant isolation, widened per record by explicit grants; the same enforcement
path serves human and agent calls, so an agent can never exceed its granting
human's permissions.

**Concurrent writers cannot silently clobber.** Humans in the UI, inbound agents,
the overnight agent, and workflow handlers hit the same rows by design, so every
mutable domain table carries a version counter bumped by the same trigger that
maintains the updated-at timestamp. A mutating request may send its last-seen
version; a stale write affects zero rows and surfaces as a version-skew conflict
rather than a lost update (DM-CONV-4; the wire error catalog is owned by the
api-conventions chapter). The staged-approval path re-checks the version at execute
time, so an approved-but-stale effect cannot apply to a row that changed during the
approval gap.

**Every captured row carries its origin.** Tables holding captured or user-entered
domain data carry non-null source and captured-by columns using fixed principal
prefixes (DM-CONV-11), plus an optional raw payload — the re-parseable original
that the memory-first pipeline extracts from and backfills against. Raw stays off
the query hot path: never filtered or joined in list and report queries, no index
by default, large blobs held as object-store references. Pure infrastructure rows
carry no provenance. The authorizing passport for an agent write is recorded once,
on the audit row, not duplicated per domain row.

**Archive, don't delete.** Normal operation soft-deletes by stamping the archive
timestamp; hard deletes exist only on the privileged erasure path. Archiving is
cascading and normative (DM-CONV-15): a parent archives its owned children in the
same transaction, edges into an archived endpoint are archived so live timelines
never point at dead rows, and polymorphic memberships, tags, and attachments are
cleaned in the same transaction with a periodic integrity job as defence in depth.
Merge is a different path: it relinks children instead of archiving them.

**Money and time are exact.** Money is always two columns — integer minor units
plus an ISO-4217 code — and floats are banned outright. Roll-ups never sum native
amounts across currencies; they aggregate base-currency values through rates frozen
at deal close or resolved from the stored daily rate table, so every report is
reproducible (the full rule set is pinned as DM-FX-1..7). Timestamps are stored in
UTC without exception; IANA zone names live on the workspace and the user and are
applied by purpose — reporting buckets in the workspace zone, personal display and
send-times in the user zone, idle and SLA thresholds as absolute UTC durations that
are DST-immune (DM-TZ-1..7).

**Names and enums stay migration-friendly.** Tables are singular snake case, and
closed value sets are text columns with check constraints, never native Postgres
enum types — a check constraint is trivially altered in a migration, which is what
lets agents add states in source. Genuinely client-extensible sets graduate to
small reference tables. The polymorphic tables share one canonical entity-type
vocabulary, restricted per table by check (DM-CONV-17).

**Indexes follow the acceptance criteria.** Every foreign key is indexed, every
listed access path in a feature acceptance criterion gets a tenant-prefixed
covering index, soft-delete-aware indexes are partial over live rows, and
full-text columns are generated tsvector columns with GIN indexes (DM-CONV-14).
Multi-column uniques are workspace-prefixed so the planner prunes by tenant first.

**Migrations are the only schema authority.** The schema changes exclusively
through sequential, reversible migration pairs, and a custom field is a real column
added by a migration — there is no field-metadata or EAV table anywhere, and a test
asserts it (DM-CONV-16). Expression-based unique keys are realized as unique
indexes, not table constraints. The decision points the corpus left open for
ratification are resolved: the defaults written there are adopted here as facts
(see the note closing the conventions appendix).

## What this chapter owns — and what it deliberately does not

The full corpus schema is sixty-six tables. Sixteen of them — identity, tenancy,
governance, audit, consent, secrets, and FX — are pinned here with complete DDL,
plus the event outbox relay table defined by the event catalog. Every other table
is pinned, DDL and all, in exactly one owning feature chapter; the ownership index
in the appendix lists each of the remaining fifty tables with its owner and purpose
and is the completeness contract the gate verifies the partition against. A handful
of tables the corpus names but defers are stubbed with their owner-on-arrival so
their absence is intentional, not an oversight.

## What's configurable

- **Workspace base currency** — declared once at initialization, immutable after
  the first deal (DM-FX-1); single-currency workspaces make the whole FX mechanism
  inert.
- **Workspace timezone** — the reporting-period zone (DM-TZ-2); changing it is an
  explicit, audited admin action that re-buckets going forward and never restates
  history.
- **User timezone** — personal display and send-time scheduling only (DM-TZ-2).
- **Retention policies** — per-object rules with an action ladder, seeded with
  conservative defaults (DM-SEED-1..5) and editable per workspace; a row under
  legal hold is never auto-acted.

## Guarantees (enforced)

- **Tenant isolation survives buggy queries.** A query that forgets its tenant
  filter still cannot cross workspaces — the row-security backstop holds, and the
  tenant-blindness, unset-setting, and pool-reuse cases are each pinned by a
  blocking test (DM-AC-1..3).
- **A connection without a tenant identity does nothing.** Zero rows read, zero
  rows written — deny-on-unset is match-nothing, never wildcard (DM-AC-2).
- **The audit log cannot be quietly rewritten.** Any update or delete against it
  raises and aborts the transaction — tampering fails loudly (DM-AC-5). Erasure is
  a logged, role-gated exception, never an open update path.
- **No anonymous rows.** Source and captured-by are non-null on every domain
  table, asserted by a cross-cutting test (DM-AC-4).
- **No metadata engine.** A test asserts no standard or custom field is backed by
  a field-metadata or EAV table (DM-AC-6).
- **No fast-but-wrong numbers.** Mixed-currency native sums are forbidden and
  caught by test (AC-DS-FX1); closed-deal base values are byte-stable forever
  (AC-DS-FX2); idle and stalled flags are stable booleans under a fixed clock in
  any zone (AC-DS-TZ1).

## Acceptance

Done, for this chapter, means the conventions are mechanically true across the
whole schema — not just on the tables pinned here. Every tenant table passes the
isolation trio; every domain table passes the provenance assertion; the audit spine
rejects mutation; FX and timezone behavior matches the seven acceptance criteria
the semantics corpus defines. The testable forms are pinned in the Acceptance
appendix; the cross-cutting screen and performance floors live in the
acceptance-standards chapter and are not restated here.

## Out of scope

Wire envelopes, error codes, and pagination belong to api-conventions. The event
catalog and delivery semantics belong to event-bus (the outbox table shape is
pinned here because it is schema; its relay behavior is not). RBAC administration
and sharing UX belong to access-and-admin; consent and erasure user surfaces to
gdpr-compliance-surfaces; the prebuilt report-key catalog to reporting. Every
feature table's behavior belongs to its owning chapter per the ownership index.

## Where it lives

Migrations live under backend/migrations/ — sequential, reversible pairs, applied
by the standard migration runner. Every backend module's store layer reaches the
database through the shared connection-acquisition seam that sets the tenant
identity; no store binds around it. Sibling chapters to read next: architecture
(module seams), api-conventions (the contract surface over this schema), and
event-bus (what the outbox feeds).

## Appendix

### Parameters — FX rules
Source: contract/data-semantics.md#1-multi-currency--fx-roll-ups @ 5a0b29c

| ID | Rule |
|---|---|
| DM-FX-1 | Each workspace declares exactly one `base_currency` (ISO-4217) at init; **immutable after the first deal**. Single-currency workspaces set base = their currency and FX is a no-op. |
| DM-FX-2 | Every `deal` keeps its native `amount_minor` + `currency`; the native amount is the source of truth and is never overwritten. |
| DM-FX-3 | The FX rate is captured and frozen at the economically real moment: on close (won/lost) persist `fx_rate_to_base` + `fx_rate_date` on the deal — a closed deal's base-currency value is immutable thereafter. Open deals convert via the workspace's current daily reference rate (one rate per currency pair per UTC day, single configured source, cached). |
| DM-FX-4 | Roll-ups always aggregate the base-currency value, never raw native integers. Mixing currencies in a `SUM` of native `amount_minor` is forbidden and must be caught by test. |
| DM-FX-5 | Reproducibility = stored rates, not live calls: daily reference rates persist in `fx_rate` keyed (currency pair, rate date); any "as of date X" roll-up uses the stored rate for X; no report depends on a live FX call at render time. |
| DM-FX-6 | Display honesty: a mixed-currency roll-up is labeled with the base currency and the as-of date; a single-currency roll-up shows native. "Explain This Number" on a converted figure exposes native amount + rate + rate date in the lineage. |
| DM-FX-7 | No client-currency reformatting of stored values: locale formatting is presentation-only; stored values are minor units + code. |

### Parameters — timezone rules
Source: contract/data-semantics.md#2-timezones @ 5a0b29c

| ID | Rule |
|---|---|
| DM-TZ-1 | Storage is always UTC (`timestamptz`), no local-time storage anywhere; capture-ingested timestamps are normalized to UTC at ingest with the original offset retained in `raw`. |
| DM-TZ-2 | A zone is attached by purpose, never assumed: `workspace.timezone` (IANA) is canonical for reporting periods, dashboards, and workspace aggregates (set at init; change is an explicit audited admin action, forward-only); `app_user.timezone` is for personal display and that user's send-time scheduling; recipient-zone send optimization uses the recipient's zone, falling back to the sender's. |
| DM-TZ-3 | Durations and idle/stalled thresholds are absolute-duration comparisons on UTC instants (e.g. idle > 60 days = `now_utc - last_activity_at_utc > 60*24h`), zone-independent and DST-immune — never calendar-day counts. |
| DM-TZ-4 | Reporting period boundaries are calendar-aligned in the workspace zone: convert UTC instants to `workspace.timezone`, bucket on local calendar boundaries; two viewers in different personal zones get identical period membership. |
| DM-TZ-5 | DST is handled by the IANA database, never fixed offsets: store/operate with IANA names; scheduled sends and SLA timers crossing a DST boundary resolve via IANA rules at execution time. |
| DM-TZ-6 | SLA timers are absolute durations from the triggering UTC instant, scheduled on River; escalation fires at the same absolute moment regardless of viewer; only the deadline's display is localized. |
| DM-TZ-7 | "As-of-date" snapshots (stage history, pipeline-as-of-date, FX rate date) interpret the date in the workspace zone, then resolve to the corresponding UTC instant — "as of a date" means end-of-day in the workspace zone, consistently. |

### Schema — universal conventions
Source: contract/data-model.md#1-conventions @ 5a0b29c

| ID | Convention | Rule |
|---|---|---|
| DM-CONV-1 | Primary keys | Every table: `id uuid PRIMARY KEY DEFAULT uuidv7()`. UUIDv7 is time-ordered → append-mostly B-tree inserts and a free coarse creation-time sort. IDs are opaque to clients — ordering semantics are never exposed in the contract. |
| DM-CONV-2 | PG16 shim | Postgres 16 has no native `uuidv7()`: a single canonical shim function is installed by the first migration, and **every** insert path — app, data migrations, `INSERT … SELECT` backfills, generated seeds, admin SQL — calls that one generator. On PG18 the identical column default resolves to the native function. |
| DM-CONV-3 | Base columns | Every tenant table carries: `id` (DM-CONV-1); `workspace_id uuid NOT NULL` FK → `workspace(id)` `ON DELETE RESTRICT`; `created_at timestamptz NOT NULL DEFAULT now()` (immutable after insert); `updated_at timestamptz NOT NULL DEFAULT now()` (bumped by the shared `set_updated_at` trigger on every UPDATE); `archived_at timestamptz NULL` (soft delete; NULL = live; default views filter `archived_at IS NULL`). |
| DM-CONV-4 | Optimistic concurrency | Every mutable domain table carries `version bigint NOT NULL DEFAULT 1`, incremented by the same `BEFORE UPDATE` trigger (`NEW.version = OLD.version + 1`). The contract surfaces it read-only; a mutating request MAY send the last-seen value in `If-Match`; the write becomes `UPDATE … WHERE id = $1 AND version = $ifmatch`, and zero affected rows → `409 version_skew`. The staged-approval path re-checks `version` at execute time. |
| DM-CONV-5 | RLS lifecycle 1 | Every domain query runs inside a transaction, and `SET LOCAL app.workspace_id = '<uuid>'` (transaction-scoped, auto-reset at COMMIT/ROLLBACK) is issued by connection-acquisition middleware before any domain statement. Bare session-level `SET` is forbidden. |
| DM-CONV-6 | RLS lifecycle 2 | Deny-on-unset: policies use `current_setting('app.workspace_id', true)` (the `missing_ok=true` form); a NULL/empty setting is match-nothing, never wildcard. A setting-less connection sees zero rows and writes nothing (writes additionally fail a `WITH CHECK`). |
| DM-CONV-7 | RLS lifecycle 3 | Pooling is pinned to transaction-pooling mode (`pgxpool`, and PgBouncer if interposed) so `SET LOCAL` scopes to the borrowed transaction. Session pooling with session-scoped settings is banned — it is the cross-tenant leak path. |
| DM-CONV-8 | RLS lifecycle 4 | Every tenant table gets `ENABLE` + `FORCE ROW LEVEL SECURITY` and a `*_tenant_isolation` policy. RLS binds only against a non-superuser role: domain queries run as the dedicated `margince_app` application role, never as a superuser/owner (which bypass RLS). |
| DM-CONV-9 | Money | Two columns, no floats ever: `amount_minor bigint` (smallest currency unit — `100000` EUR-cents = €1,000.00) + `currency char(3)` ISO-4217 uppercase with `CHECK (currency ~ '^[A-Z]{3}$')`. Roll-ups never `SUM` native minor units across currencies (DM-FX-4). |
| DM-CONV-10 | Timestamps | Every time column is `timestamptz` stored in UTC; no naive `timestamp`, no fixed-offset storage. IANA zone names live in `workspace.timezone` / `app_user.timezone` and apply by purpose (DM-TZ-2). |
| DM-CONV-11 | Provenance | On every table holding captured or user-entered domain data: `source text NOT NULL` (what produced the row: `email:<message-id>`, `calendar:<event-id>`, `whatsapp:<message-id>`, `telegram:<message-id>`, `connector:<name>:<record-id>`, `api`, `import:<batch-id>`, `ui`) and `captured_by text NOT NULL` (typed principal: `human:<uuid>`, `agent:<id>`, `connector:<name>`); plus `raw jsonb NULL` — the re-parseable original, the memory-first source of truth, kept off the query hot path (never filtered/joined in list/report queries, no GIN index by default, large blobs as S3/MinIO refs). Not on pure infrastructure rows (`role_assignment`, `team_membership`, `audit_log`, `fx_rate`). Memory-first backfilled fields carry `captured_by = agent:*`. The authorizing passport id lives on `audit_log.passport_id`, not per domain row. |
| DM-CONV-12 | Naming | `snake_case`, singular table names (`person`, not `people`); the users table is `app_user` (`user` is reserved). Join tables are `<a>_<b>` or a descriptive noun. Enum/check tokens are lowercase. |
| DM-CONV-13 | Enum strategy | Status/kind/role columns are `text` + `CHECK (col IN (...))`, never native Postgres `ENUM` (values can't be removed; alters take heavy locks; breaks the agent-edited-migration story). Genuinely client-extensible value sets become small reference tables instead. |
| DM-CONV-14 | Indexing baseline | Every FK column is indexed (Postgres does not auto-index FKs). Every "list/find by" access path named in an acceptance criterion gets a tenant-prefixed covering composite index. Soft-delete-aware indexes are partial (`WHERE archived_at IS NULL`). Full-text columns are `tsvector` GENERATED columns + GIN. Multi-column uniques and most indexes are `workspace_id`-prefixed. |
| DM-CONV-15 | Archive-cascade | Soft delete is the dominant delete path; DDL `ON DELETE` clauses fire only on the privileged erasure backstop. Archiving a parent archives its owned children in the same transaction (person → emails/phones; org → domains); archiving a deal does NOT archive shared activities. Edges referencing an archived endpoint are archived (`relationship`, `activity_link`). Polymorphic rows (`list_member`, `taggable`, `attachment`) pointing at the archived row are archived in the same transaction; a periodic integrity job is defence in depth. Merge relinks children instead of archiving — archive and merge are distinct paths. |
| DM-CONV-16 | Migration design | Migrations are sequential, reversible up/down pairs. An expression unique key (e.g. on a lowercased column) is a `CREATE UNIQUE INDEX`, not a `UNIQUE` table constraint. Custom fields are real columns added by migration — there is NO `field_metadata`/EAV table, and a schema test asserts it (DM-AC-6). |
| DM-CONV-17 | Canonical EntityType | The polymorphic tables (`activity_link`, `taggable`, `list_member`, `attachment`) and the contract's `EntityType` draw from one enum — `{person, organization, deal, lead, activity}`; per-table CHECKs restrict the subset each table allows; codegen emits a single `EntityType`. |

The canonical shared trigger and tenant-isolation policy patterns, verbatim:

```sql
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$ LANGUAGE plpgsql;
-- per table: CREATE TRIGGER trg_<t>_updated BEFORE UPDATE ON <t>
--   FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- tenant isolation, applied to every tenant table (deny-on-unset form, DM-CONV-6):
ALTER TABLE person ENABLE ROW LEVEL SECURITY;
ALTER TABLE person FORCE ROW LEVEL SECURITY;
CREATE POLICY person_tenant_isolation ON person
  USING (workspace_id = current_setting('app.workspace_id', true)::uuid);
```

Note DM-CONV-N-1 (source: contract/data-model.md#14-open-questions @ 5a0b29c): the
corpus's ratification list carries chosen defaults, adopted here as settled facts —
UUIDv7 keys with the PG16 shim; RLS as the tenancy backstop; text + CHECK enums with
reference tables only for client-extensible sets; one typed `relationship` table;
composite-FK stage-in-pipeline enforcement; a single polymorphic `activity` table;
object-store attachment references; the dual lead-promotion pointer pair; and an
app-layer base-currency immutability guard with an optional trigger backstop.

### Schema — identity, tenancy & governance tables
Source: contract/data-model.md#2-identity-tenancy--rbac + #34-consent--retention + #64-deal-stage-history--fx-rate + #11-audit-log + #125-net-new-v1-objects; contract/events.md#42-outbox-relay @ 5a0b29c

DDL is copied verbatim from the corpus contract. All DM-CONV rules apply; comments
inside the fences are the corpus's own.

**DM-DDL-1 — `workspace`.** The tenant root; not itself tenant-scoped.

```sql
CREATE TABLE workspace (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  name          text NOT NULL,
  slug          text NOT NULL,
  base_currency char(3) NOT NULL,            -- ISO-4217; IMMUTABLE after first deal (data-semantics §1.2)
  timezone      text NOT NULL DEFAULT 'UTC', -- IANA name; reporting-period zone (data-semantics §2)
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,
  CONSTRAINT workspace_slug_unique UNIQUE (slug),
  CONSTRAINT workspace_base_currency_iso CHECK (base_currency ~ '^[A-Z]{3}$')
);
```

**DM-DDL-2 — `app_user`.** Human seats and first-party agent identities; `seat_type`
is the billing/capability tier (a hard ceiling below the RBAC role: a `read` seat
cannot mutate even with a write-capable role, a write `record_grant` to a `read`
seat is rejected, and enforcement is scope-intersection — effective = passport
scopes ∩ role RBAC ∩ seat ceiling; billing counts `full` seats only).
`password_hash` arrives with the session model (DM-DDL-8).

```sql
CREATE TABLE app_user (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  email         text NOT NULL,                 -- login identity; password_hash + session in §2.6 (ADR-0043)
  display_name  text NOT NULL,
  timezone      text NOT NULL DEFAULT 'UTC',   -- IANA; personal display + send-time (data-semantics §2)
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deactivated')),
  is_agent      boolean NOT NULL DEFAULT false, -- a first-party Agent Runner identity vs a human seat
  seat_type     text NOT NULL DEFAULT 'full' CHECK (seat_type IN ('read','full')), -- A62/ADR-0047: 'read' = free unlimited viewer (read + read-only AI, no mutate); 'full' = billable acting seat
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,
  CONSTRAINT app_user_email_unique UNIQUE (workspace_id, lower(email)),
  CONSTRAINT app_user_agent_is_full CHECK (NOT is_agent OR seat_type = 'full') -- an agent identity is never a read seat
);
CREATE INDEX idx_app_user_ws ON app_user (workspace_id) WHERE archived_at IS NULL;
```

**DM-DDL-3 — `team`, `team_membership`.** Named user groups with optional shallow
nesting; pure infrastructure rows (no provenance).

```sql
CREATE TABLE team (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name         text NOT NULL,
  parent_team_id uuid NULL REFERENCES team(id) ON DELETE SET NULL, -- optional shallow nesting
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL,
  CONSTRAINT team_name_unique UNIQUE (workspace_id, name)
);

CREATE TABLE team_membership (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  team_id      uuid NOT NULL REFERENCES team(id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT team_membership_unique UNIQUE (team_id, user_id)
);
CREATE INDEX idx_team_membership_user ON team_membership (user_id);
CREATE INDEX idx_team_membership_team ON team_membership (team_id);
```

**DM-DDL-4 — `role`, `role_assignment`.** A role bundles object- and field-level
permissions as a policy document (`row_scope` own/team/all + `field_masks` drive
query construction and field projection — a masked field is absent from the
payload, not merely UI-hidden). A small opinionated default set is seeded per
workspace; custom roles beyond it are a code extension, not a runtime builder.

```sql
CREATE TABLE role (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  key          text NOT NULL,   -- 'admin' | 'manager' | 'rep' | 'read_only' | 'ops' | <code-defined>
  name         text NOT NULL,
  is_system    boolean NOT NULL DEFAULT false, -- seeded default vs code-added
  permissions  jsonb NOT NULL DEFAULT '{}'::jsonb, -- {object: {crud}, field_masks: [...], row_scope: own|team|all}
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL,
  CONSTRAINT role_key_unique UNIQUE (workspace_id, key)
);

CREATE TABLE role_assignment (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  role_id      uuid NOT NULL REFERENCES role(id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  team_id      uuid NULL REFERENCES team(id) ON DELETE CASCADE, -- optional: role scoped to a team
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT role_assignment_unique UNIQUE (role_id, user_id, COALESCE(team_id, '00000000-0000-0000-0000-000000000000'::uuid))
);
CREATE INDEX idx_role_assignment_user ON role_assignment (user_id);
```

**DM-DDL-5 — `record_grant`.** Flat, audited per-record sharing that widens the
own/team/all base scope for one record — one table for all shareable types, no
sharing hierarchies or criteria rules. Enforcement widens both layers: the
application visibility predicate AND the RLS backstop policy gain the same
`OR EXISTS (active matching record_grant)` clause. An agent-initiated grant is
approval-gated and can never exceed the granting human's own access; every
grant/revoke writes an audit row.

```sql
CREATE TABLE record_grant (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  record_type   text NOT NULL CHECK (record_type IN ('deal','person','organization','lead')),
  record_id     uuid NOT NULL,                          -- the shared record (no cross-type FK; type-checked in app)
  subject_type  text NOT NULL CHECK (subject_type IN ('user','team')),
  subject_id    uuid NOT NULL,                          -- app_user(id) or team(id) per subject_type
  access        text NOT NULL CHECK (access IN ('read','write')),  -- 'write' satisfies 'read'
  granted_by    uuid NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
  reason        text NULL,                              -- lawful-basis / accountability trail (surfaced in audit)
  expires_at    timestamptz NULL,                       -- optional TTL; an expired grant matches nothing
  created_at    timestamptz NOT NULL DEFAULT now(),
  version       bigint NOT NULL DEFAULT 1,              -- optimistic concurrency (ADR-0036)
  CONSTRAINT record_grant_unique UNIQUE (workspace_id, record_type, record_id, subject_type, subject_id)
);
CREATE INDEX idx_record_grant_record  ON record_grant (workspace_id, record_type, record_id);
CREATE INDEX idx_record_grant_subject ON record_grant (workspace_id, subject_type, subject_id) WHERE expires_at IS NULL OR expires_at > now();
```

**DM-DDL-6 — `session`.** Human interactive auth is opaque and server-side: a login
mints a random token, only its SHA-256 hash persists, and the raw token rides the
HttpOnly cookie. A request authenticates iff a row matches the hash AND is neither
revoked nor idle- nor absolutely-expired; remote revoke sets `revoked_at` and is
enforced at lookup. Every auth event writes an audit row.

```sql
ALTER TABLE app_user ADD COLUMN password_hash text NULL;   -- NULL for SSO-provisioned users; Argon2id/bcrypt, never plaintext

CREATE TABLE session (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  user_id         uuid NOT NULL REFERENCES app_user(id)  ON DELETE CASCADE,
  token_hash      text NOT NULL UNIQUE,                   -- SHA-256(raw token); raw never stored
  idle_expires_at timestamptz NOT NULL,                   -- rolls forward on activity, capped by expires_at
  expires_at      timestamptz NOT NULL,                   -- absolute timeout (DST-immune UTC instant, §1)
  last_seen_at    timestamptz NOT NULL DEFAULT now(),
  user_agent      text NULL,                              -- device/session-list display
  ip              inet NULL,
  revoked_at      timestamptz NULL,                       -- remote revoke; an expired/revoked row authenticates nothing
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_session_user ON session (workspace_id, user_id) WHERE revoked_at IS NULL;
```

**DM-DDL-7 — `passport` (Agent Seat Passport).** The only network-auth model for
agents: binds a BYO/external agent to a CRM identity plus an explicit scope set.
Effective scope = passport scopes ∩ the on-behalf-of human's effective RBAC,
re-derived at admission (revocation is synchronous at admit, not bus-dependent);
minting an over-broad passport is rejected as exceeding the grantor.

```sql
CREATE TABLE passport (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id)   ON DELETE RESTRICT,
  on_behalf_of  uuid NOT NULL REFERENCES app_user(id)    ON DELETE CASCADE,  -- the human whose RBAC bounds this passport
  granted_by    uuid NOT NULL REFERENCES app_user(id)    ON DELETE RESTRICT, -- who minted it (often == on_behalf_of)
  label         text NULL,                               -- "Claude Desktop", "Cursor", etc.
  scopes        text[] NOT NULL,                         -- explicit; rejected at bind if not ⊆ on_behalf_of's RBAC (scope_exceeds_grantor)
  token_hash    text NOT NULL UNIQUE,                    -- SHA-256(raw bearer token); raw never stored
  expires_at    timestamptz NOT NULL,                    -- enforced at lookup
  revoked_at    timestamptz NULL,                        -- manual revoke; cascades within one event-bus cycle (B-EP03.10)
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_passport_obo ON passport (workspace_id, on_behalf_of) WHERE revoked_at IS NULL;
```

**DM-DDL-8 — `audit_log` + append-only trigger.** The immutable spine: every
mutation attributable to a human or a specific agent, with the authorizing
passport, before/after diff, and governing rule. Append-only is trigger-enforced
and loud — a tamper attempt raises and aborts rather than silently no-oping.
Erasure never updates in place: it is a privileged, separately-audited path whose
elevated role is the only principal allowed past the trigger, and every scrub is
itself logged. Every domain mutation writes exactly one audit row and emits one
domain event application-side post-commit.

```sql
CREATE TABLE audit_log (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,

  actor_type    text NOT NULL CHECK (actor_type IN ('human','agent','system')),
  actor_id      text NOT NULL,            -- user uuid, agent id, or 'system'
  passport_id   uuid NULL,                -- Agent Seat Passport that authorized an agent action (03b §Layer1)
  on_behalf_of  uuid NULL REFERENCES app_user(id) ON DELETE SET NULL, -- the human authority for an agent action

  action        text NOT NULL CHECK (action IN ('create','update','archive','merge','promote','restore','export','erase','login','assign','advance_stage')),
  entity_type   text NOT NULL,            -- 'person','deal','lead','activity',...
  entity_id     uuid NULL,                -- NULL allowed for non-entity actions (login/export)

  before        jsonb NULL,               -- prior state / changed fields
  after         jsonb NULL,               -- new state / changed fields (the diff is before∆after)
  authorization_rule text NULL,           -- which RBAC/scope rule allowed it (features/04 §1 AC)
  evidence      jsonb NULL,               -- e.g. which inbound email/meeting triggered a promotion

  occurred_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_entity ON audit_log (workspace_id, entity_type, entity_id, occurred_at DESC);
CREATE INDEX idx_audit_actor  ON audit_log (workspace_id, actor_id, occurred_at DESC);
CREATE INDEX idx_audit_time   ON audit_log (workspace_id, occurred_at DESC);

-- append-only enforcement (features/04 §4 AC: UPDATE/DELETE rejected at DB layer)
-- A BEFORE trigger that RAISEs (not a `DO INSTEAD NOTHING` rule): a tamper attempt must FAIL LOUDLY,
-- not silently succeed-as-no-op. Silent swallow would hide the attempt — the opposite of tamper-evidence.
CREATE OR REPLACE FUNCTION audit_log_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'audit_log is append-only (attempted % on row %)', TG_OP, OLD.id
    USING ERRCODE = 'check_violation';
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_audit_no_mutate BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();
```

**DM-DDL-9 — `event_outbox`.** Written inside the domain transaction; a relay job
polls unpublished rows in creation order, publishes to the stream, and stamps
`published_at` (at-least-once → consumer dedupe). The event catalog, envelope, and
delivery semantics are owned by the event-bus chapter; only the table shape is
pinned here, as the corpus states it:

```sql
event_outbox(id uuid pk, stream text, envelope jsonb, published_at timestamptz null, created_at)
```

**DM-DDL-10 — consent & retention (`consent_purpose`, `person_consent`,
`consent_event`, `retention_policy`).** Per-purpose consent with an append-only
proof log plus a retention-policy engine. Enforcement is default-deny: an outbound
action for a purpose is blocked/suppressed unless an active, proven `granted` state
exists for that purpose — `unknown` and `withdrawn` both suppress, and a grant for
a different purpose does not authorize the send. Capture surfaces must pass the
purpose and wording through; they may not synthesize a blanket grant. A nightly
River job applies due retention actions with full audit provenance; a row with
`legal_hold = true` is never auto-acted. The consent/erasure user surfaces are
owned by gdpr-compliance-surfaces; the tables live here.

```sql
CREATE TABLE consent_purpose (
  id           uuid PRIMARY KEY,
  workspace_id uuid NOT NULL REFERENCES workspace(id),
  key          text NOT NULL,                       -- e.g. 'marketing_email'
  label        text NOT NULL,
  archived_at  timestamptz NULL,
  UNIQUE (workspace_id, key)
);

CREATE TABLE person_consent (
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL REFERENCES workspace(id),
  person_id     uuid NOT NULL REFERENCES person(id),
  purpose_id    uuid NOT NULL REFERENCES consent_purpose(id),
  state         text NOT NULL DEFAULT 'unknown'
                  CHECK (state IN ('unknown','granted','withdrawn')),
  lawful_basis  text NULL,                           -- 'consent' | 'legitimate_interest' | 'contract' | …
  captured_at   timestamptz NULL,
  source        text NULL,                           -- channel: 'booking' | 'import' | 'form' | 'manual' | …
  policy_version text NULL,                          -- the version of the wording shown at grant time
  UNIQUE (workspace_id, person_id, purpose_id)
);

CREATE TABLE consent_event (                          -- append-only / tamper-evident, like audit_log
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL REFERENCES workspace(id),
  person_id     uuid NOT NULL REFERENCES person(id),
  purpose_id    uuid NOT NULL REFERENCES consent_purpose(id),
  new_state     text NOT NULL CHECK (new_state IN ('granted','withdrawn')),
  lawful_basis  text NULL,
  source        text NOT NULL,                        -- channel/surface that captured it
  policy_text   text NOT NULL,                        -- the verbatim wording presented
  policy_version text NOT NULL,
  double_opt_in_confirmed_at timestamptz NULL,        -- set when a confirmation step is required + done
  captured_at   timestamptz NOT NULL,
  captured_by   text NOT NULL                          -- human user / connector:booking / import:* (provenance, §… )
);

CREATE TABLE retention_policy (
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL REFERENCES workspace(id),
  object_type   text NOT NULL,                        -- 'lead' | 'person' | 'activity' | 'deal' | …
  category      text NULL,                            -- optional finer scope (e.g. 'cold_lead')
  retain_days   int  NOT NULL,                        -- age after which the action fires
  action        text NOT NULL CHECK (action IN ('archive','anonymize','erase')),
  lawful_basis  text NULL,
  enabled       boolean NOT NULL DEFAULT true,
  UNIQUE (workspace_id, object_type, category)
);

ALTER TABLE person       ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;
ALTER TABLE organization ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;
ALTER TABLE deal         ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;
ALTER TABLE lead         ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;
```

**DM-DDL-11 — `connector_secret`.** The sealed connector-credential vault: the
ciphertext is sealed by a pluggable key provider and is NEVER returned via any API
— the table has no public surface and is read only by the connector runtime through
a privileged seam. Carries the base columns per the corpus's net-new-table
convention.

```sql
CREATE TABLE connector_secret (                          -- encrypted overlay-connector credential (ADR-0048; B-E18.13)
  connector  text  NOT NULL,                             -- 'salesforce' | 'hubspot' | 'dynamics' | …
  ciphertext bytea NOT NULL,                             -- sealed secret; NEVER returned via any API
  kms_key_id text  NOT NULL,                             -- key-provider ref (cloud KMS id when hosted, local key id on-prem)
  rotated_at timestamptz NULL
);
CREATE UNIQUE INDEX idx_connector_secret ON connector_secret (workspace_id, connector);
```

**DM-DDL-12 — `fx_rate`.** One stored reference rate per currency pair per UTC day
(DM-FX-5); open-deal pipeline value converts via the daily rate for the as-of date,
closed deals use the rate frozen on the deal row. Pure infrastructure (no
provenance columns).

```sql
CREATE TABLE fx_rate (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  from_currency char(3) NOT NULL,
  to_currency   char(3) NOT NULL,              -- = workspace.base_currency
  rate          numeric(20,10) NOT NULL,
  rate_date     date NOT NULL,                 -- one rate per pair per UTC day (data-semantics §1.2 r5)
  created_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT fx_rate_pair_day UNIQUE (workspace_id, from_currency, to_currency, rate_date)
);
CREATE INDEX idx_fx_rate_lookup ON fx_rate (workspace_id, from_currency, to_currency, rate_date);
```

### Schema — ownership index
Source: contract/data-model.md §3–§12.6 (every CREATE TABLE) @ 5a0b29c

The corpus schema defines **66 tables**. **16 are owned by this chapter**
(DM-DDL-1..12: `workspace`, `app_user`, `team`, `team_membership`, `role`,
`role_assignment`, `record_grant`, `session`, `passport`, `audit_log`,
`consent_purpose`, `person_consent`, `consent_event`, `retention_policy`,
`connector_secret`, `fx_rate`; `event_outbox` is additional, defined by the event
catalog, not among the 66). The remaining **50 tables** are indexed below, one row
per table — each pins its full DDL in exactly one owning feature chapter. This
index is the completeness contract for the 66-table partition.

| ID (table) | Owning chapter | Purpose |
|---|---|---|
| `person` | people-and-organizations | contact record; merge + lead-origin pointers, full-text |
| `person_email` | people-and-organizations | ordered emails; the exact-email dedupe key (collision → conflict + existing id) |
| `person_phone` | people-and-organizations | ordered phones, ≤1 primary per type |
| `organization` | people-and-organizations | company record; classification, logo, hierarchy, merge |
| `organization_domain` | people-and-organizations | lowercased domains, unique per workspace; employer-inference lookup |
| `partner` | people-and-organizations | 1:1 partner-program extension of an organization (cert status, role, margin tier) |
| `relationship` | people-and-organizations | typed edges: employment, deal stakeholders, partner edges (deals-and-pipeline cites) |
| `pipeline` | deals-and-pipeline | pipeline; exactly one seeded default per workspace |
| `stage` | deals-and-pipeline | ordered stages with semantics + win probability |
| `deal` | deals-and-pipeline | the deal: native money, FX freeze, stalled flag, forecast category |
| `deal_stage_history` | deals-and-pipeline | append-only stage-transition snapshots (as-of-date reporting) |
| `activity` | activities-and-timeline | single polymorphic timeline row with typed kind + idempotent capture key |
| `activity_link` | activities-and-timeline | activity ↔ {person, organization, deal} join for the 360 timeline |
| `lead` | leads-and-qualification | thin segregated lead; no org FK by construction |
| `list` | lists-views-segmentation | static list or dynamic segment (typed query definition) |
| `list_member` | lists-views-segmentation | polymorphic list membership, unique per entity |
| `tag` | lists-views-segmentation | workspace-scoped label, unique lowercased name |
| `taggable` | lists-views-segmentation | tag ↔ entity polymorphic join |
| `saved_view` | lists-views-segmentation | saved filter/sort per user (query uses the DM-VOCAB vocabulary only) |
| `attachment` | records-depth | object-store file references + metadata; never blobs |
| `bulk_operation` | records-depth | async bulk job (edit/reassign/delete/export) with idempotency key |
| `quota` | records-depth | per-owner/team revenue target per period (forecasting cites) |
| `signal` | signals-and-warm-room | surfaced attention item with resolution state + evidence |
| `signal_resolution` | signals-and-warm-room | append-only match-basis + human-outcome log per signal |
| `deal_room` | deal-rooms | consent-gated buyer space; slug is the access credential |
| `deal_room_engagement_event` | deal-rooms | high-volume buyer engagement events (view/open/download/accept) |
| `voice_profile` | voice-profile | a user's/team's voice DNA with build status |
| `voice_corpus_source` | voice-profile | consented samples a profile was built from |
| `drafting_asset` | drafting | governed reusable claims/snippets; only approved assets are agent-usable |
| `agent_connection` | byo-agent-and-mcp | registered BYO agent bound to a passport + human |
| `agent_connection_event` | byo-agent-and-mcp | connect/disconnect/revoke audit for a connection |
| `webhook_subscription` | byo-agent-and-mcp | outbound webhook subscription (HTTPS-only, owner-scoped delivery) |
| `webhook_delivery` | byo-agent-and-mcp | per-attempt delivery log with retry/dead-letter state |
| `automation` | automation | standing automation (catalog or agent-authored), bounded trigger/action |
| `automation_run` | automation | execution provenance per automation firing |
| `forecast_snapshot` | forecasting | predicted-vs-actual captured at period close |
| `field_mask` | access-and-admin | declarative field-level RBAC (masked = null + marker, shape-stable) |
| `ai_feedback` | ai-runtime | human suppress/correct/confirm ledger for AI-surfaced claims (stable claim_key) |
| `conversation_link` | dispact-integration | CRM entity ↔ external conversation link, unique per pair, bidirectional lookup |
| `deal_channel` | dispact-integration | a deal's dedicated chat channel (one per deal) |
| `gtm_module_install` | dispact-integration | installed suite/GTM module + config |
| `data_subject_request` | gdpr-compliance-surfaces | GDPR access/rectify/erasure request tracking with statutory deadline |
| `brief_run` | morning-brief | one generated daily brief per rep with data cutoff |
| `brief_item` | morning-brief | ranked brief item with acted/dismissed state |
| `approval_item` | notifications-and-approval-inbox | staged approval-inbox item; fail-closed expiry |
| `signature_evidence` | germany-package | e-sign proof (provider envelope, signer set, evidence hash) |
| `product` | offers-and-products | optional rate-card entry; data, not a configurator |
| `offer` | offers-and-products | versioned quote bound to one deal; server-computed totals, FX freeze at send |
| `offer_line_item` | offers-and-products | typed line with price snapshot; derived totals in code |
| `offer_template` | offers-and-products | branded, governed PDF layout (bounded params, not a CMS) |

Note DM-OWN-N-1: custom fields deliberately have no table in this index — a custom
field is a real column added by migration (DM-CONV-16); the mechanics are owned by
the custom-fields chapter. Note DM-OWN-N-2: the corpus's acceptance-to-constraint
traceability rows pin in each owning feature chapter's Acceptance appendix, not
here.

### Schema — deferred tables (stubs, owner-on-arrival)
Source: contract/data-model.md#12-deferred-tables @ 5a0b29c

Named so the schema's shape is intentional; none are built in V1, and none carry
DDL until their owner chapter lands it.

| ID | Deferred table(s) | Owner on arrival | Status note |
|---|---|---|---|
| DM-DEF-1 | `sequence`, `sequence_step`, `sequence_enrollment` | sequences-and-deliverability | outbound cadences; sends gated, lands with the comms spec |
| DM-DEF-2 | `workflow_run` | automation | run records only (workflows are code); likely reuses audit + a thin run table |
| DM-DEF-3 | `embedding` | search-and-retrieval | pgvector store with HNSW index; capability is V1, the dedicated table lands with the retrieval work; right-to-deletion must purge a subject's embeddings |
| DM-DEF-4 | `engagement_event` | lead-scoring | high-volume open/click tracking; fast-follow — behavioral scoring degrades gracefully until it exists |
| DM-DEF-5 | `flow_link` | dispact-integration | **promoted to V1** as `conversation_link` (see the ownership index) — no longer deferred |
| DM-DEF-6 | overlay cluster: `overlay_mirror`, `overlay_association`, `incumbent_connection`, workspace SoR-mode flag | overlay-augmentation | distinct schema namespace so the native core is untouched; mirror teardown on disconnect within the declared retention window |
| DM-DEF-7 | `fx_rate` backfill / restatement tooling | reporting (tooling, not a table) | the `fx_rate` table itself ships here (DM-DDL-12); historical restatement tooling is OUT V1 |

### Schema — sort & filter vocabulary
Source: contract/data-model.md#135-sort--filter-vocabulary @ 5a0b29c

The contract's `sort` and report `filters`/`group_by` are closed vocabularies, not
free identifier passthrough. For each list resource the allowed fields are exactly
the indexed columns below.

| ID | Resource | Allowed sort fields | Allowed filter fields |
|---|---|---|---|
| DM-VOCAB-1 | people | `created_at`, `updated_at`, `full_name`, `owner_id` | `owner_id`, `organization_id` (via employment), `archived`, `q` |
| DM-VOCAB-2 | organizations | `created_at`, `updated_at`, `display_name`, `owner_id` | `owner_id`, `industry`, `size_band`, `classification`, `archived` |
| DM-VOCAB-3 | deals | `created_at`, `updated_at`, `amount_minor`, `expected_close_date`, `last_activity_at` | `pipeline_id`, `stage_id`, `owner_id`, `organization_id`, `status`, `forecast_category`, `partner_org_id` |
| DM-VOCAB-4 | activities | `occurred_at`, `created_at`, `due_at` | `kind`, `entity_type`+`entity_id`, `direction`, `assignee_id`, `is_done` |
| DM-VOCAB-5 | leads | `created_at`, `updated_at`, `score` | `status`, `owner_id`, `candidate_org_key` |
| DM-VOCAB-6 | partners | `created_at`, `updated_at` | `partner_role`, `cert_status` |

Rule DM-VOCAB-ERR-1: any field outside the allow-list → `422` with
`sort_field_not_allowed` / `report_field_not_allowed`. Rule DM-VOCAB-ERR-2: `id` is
always an implicit final sort tie-breaker (keyset pagination). The eight prebuilt
report keys and their dimension/measure vocabularies are pinned by the reporting
chapter — cited here, not restated; ad-hoc reports compile against the per-resource
lists above.

### Acceptance
Source: contract/data-semantics.md#14-acceptance + #24-acceptance; contract/data-model.md#1-conventions + #11-audit-log @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-DS-FX1 | Given deals in ≥2 currencies, when rolled up, the total equals the independently computed sum of each deal's base-currency value at its frozen/applicable rate; a roll-up that sums native `amount_minor` across currencies fails by construction. | golden-number test on a mixed-currency fixture |
| AC-DS-FX2 | Given a closed deal, when the roll-up re-runs at different wall-clock times, its base value is byte-stable. | frozen-rate reproducibility test |
| AC-DS-FX3 | Given a seeded rate history, when "as-of-date X" pipeline value is computed, it uses the stored daily rate for X and is reproducible. | test against seeded `fx_rate` history |
| AC-DS-TZ1 | Given a fixed test clock, the stalled/idle flag is a stable boolean identical regardless of any user/workspace zone (duration-based, DST-immune). | fixed-clock unit test |
| AC-DS-TZ2 | Given two users in different personal zones, a "this quarter / last month" report returns identical row membership (bucketing follows `workspace.timezone`). | golden test |
| AC-DS-TZ3 | Given an SLA timer spanning a DST transition, it fires at the correct absolute instant. | IANA-resolved River-schedule test |
| AC-DS-TZ4 | No timestamp column stores non-UTC local time; no scheduling code uses fixed numeric offsets instead of IANA names. | schema/lint assertion test |
| DM-AC-1 | Given tenant A's workspace setting on a connection, when B's rows are queried, zero of tenant B's rows are visible. | blocking CI integration test (live Postgres) |
| DM-AC-2 | Given a connection with no `app.workspace_id` set, reads return zero rows and inserts/updates fail. | blocking CI integration test |
| DM-AC-3 | Given a pool checkout that previously served tenant A, when reused for tenant B without re-setting the setting, it does not see A's rows. | blocking CI pool-reuse test |
| DM-AC-4 | Every domain table has `source` and `captured_by` NOT NULL. | cross-cutting schema conformance test |
| DM-AC-5 | Any UPDATE or DELETE on `audit_log` raises and aborts its transaction; existing rows persist unchanged. | trigger integration test |
| DM-AC-6 | No standard or custom field is backed by a `field_metadata`/EAV table. | schema assertion test |

### Seed — default retention policies
Source: contract/data-model.md#34-consent--retention @ 5a0b29c

A new workspace is seeded with these `retention_policy` rows (DM-DDL-10) —
compliant out of the box, editable per workspace; one action per row, ladders
compose as separate rows at increasing `retain_days`; a jurisdiction pack may
override per country.

| ID | object_type | category | retain_days | action |
|---|---|---|---|---|
| DM-SEED-1 | `lead` | `unconverted` | 365 | `anonymize` |
| DM-SEED-2 | `activity` | — | 1095 (3y) | `archive` |
| DM-SEED-3 | `activity` | `transcript` | 365 | `erase` (special-category free text) |
| DM-SEED-4 | `person` | `no_consent_no_deal` | 730 (2y) | `anonymize` |
| DM-SEED-5 | `deal` | `lost` | 1825 (5y) | `archive` |
