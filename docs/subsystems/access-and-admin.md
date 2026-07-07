---
status: planned
module: backend/internal/modules/identity (RBAC administration, record grants, field masks, seat ceiling, enterprise sign-in, SCIM, the bulk-ops engine, license/operator surfaces) · web (settings/members, share, bulk-actions, security, license, operator, field-security, sandbox, scim, localization screens)
derives-from:
  - specs/spec/features/04-platform-and-compliance.md#1-permissions-teams--roles-rbac-rowfield-level @ 5a0b29c
  - specs/spec/features/04-platform-and-compliance.md#7-authentication--sso-mfa--session-security @ 5a0b29c
  - specs/spec/features/04-platform-and-compliance.md#8-bulk-data-operations-edit--archive--reassign @ 5a0b29c
  - specs/spec/features/10-operational-depth.md#7-enterprise-admin-security--platform-promotes-d103-field-level-security-d105-sandbox-d107-scim-d112-rate-limits-d1111-i18n-d1112-wcag @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e111--roles-that-bound-humans-and-their-agents @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e113--audit-log-every-change-is-attributable @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e117--bulk-operations-on-a-selection-edit--archive--reassign @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e118--enterprise-sign-in-sso--mfa-included-audited @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e1110--license-seats--free-tier-status-self-hosted-admin @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e1111--operator-upgrade-backup--dr--with-evidence @ 5a0b29c
  - specs/spec/product/epics/E11-access-trust-exit.md#s-e1112--share-one-record-with-a-person-or-team @ 5a0b29c
  - specs/spec/product/epics/E15-operational-depth.md#s-e159--field-level-security-sandbox--scim-regulated-admin-readiness @ 5a0b29c
  - specs/spec/product/epics/E15-operational-depth.md#s-e1510--german-ui--accessibility @ 5a0b29c
  - specs/spec/product/epics/E15-operational-depth.md#s-e1511--documented-api-rate-limits--versioning @ 5a0b29c
  - specs/spec/decisions/ADR-0039-record-level-sharing-grants.md @ 5a0b29c
  - specs/spec/decisions/ADR-0047-free-read-seats-paid-full-seats.md @ 5a0b29c
  - specs/spec/decisions/ADR-0029-busl-license-and-seat-enforcement.md @ 5a0b29c
  - specs/spec/contract/data-model.md#125-net-new-v1-objects-signals-deal-rooms-voice-agent-connections-automation-views-quota-field-mask @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#settingshtml--workspace-settings-implements-s-e111345-s-e105 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#bulk-actionshtml--bulk-operations-implements-s-e117-s-e155c @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#securityhtml--sso-mfa--sessions-implements-s-e118 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#licensehtml--license-seats--free-tier-implements-s-e1110 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#operatorhtml--operator-console-implements-s-e1111 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#sharehtml--share-a-record-implements-s-e1112 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#field-securityhtml--field-level-security-implements-s-e159a @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#sandboxhtml--sandbox--staging-workspace-implements-s-e159b @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#scimhtml--scim-provisioning-implements-s-e159c @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#localizationhtml--german--english-localization-implements-s-e1510a @ 5a0b29c
  - margince-poc/docs/subsystems/role-admin.md @ a11d6c08
---
# Access & admin — one authorization substrate, administered and provable

> The admin's chapter: everything Mor uses to govern who — human or agent — may see
> and do what, and to prove it. Role and member administration, per-record sharing,
> field-level security, the seat-type ceiling, enterprise sign-in (SSO + MFA), SCIM
> provisioning, governed bulk operations, the audit-log viewer, license and seat
> stewardship, the operator console, the sandbox workspace, German UI + accessibility,
> and the published API-limits page. The single promise: there is exactly one
> authorization substrate, every widening or narrowing of access crosses it audited,
> and nothing enforces silently.

## What it's for

Every other subsystem assumes an answer to "may this principal do this?"; this
chapter owns how that answer is administered, upgraded to enterprise grade, and
proven to an auditor. It exists because the regulated-Mittelstand beachhead buys
governance before features: roles that bound humans *and* their agents identically,
sign-in security that passes an IT review without an edition upsell, directory-driven
leaver revocation, mass changes that are previewed and reversible rather than silent,
and an operations story (license, upgrade, backup, disaster recovery) a self-hosted
customer can evidence. Its callers are every request in the product — role, row-scope,
field-mask, and seat-ceiling checks run on the one enforcement path the identity
module resolves ([[auth-and-sessions]]) — plus the admin surfaces themselves, the
customer's IdP and SCIM directory, and agents acting under an Agent Seat Passport.
The scope boundary: the substrate *tables* are the data-model chapter's, the
session/passport *lifecycle* is auth-and-sessions', the audit *spine* is
audit-observability's, and the approval *inbox* is notifications-and-approval-inbox's;
this chapter owns the administration surfaces over them, the enforcement semantics
they administer, and the enterprise upgrades (SSO, MFA, SCIM) that were deliberately
placed outside the shipped auth baseline.

## Principles it serves

- **P12 — governance designed in.** Roles bound agents exactly as they bound the
  humans who granted them; every grant, revoke, mask change, sign-in, seat change,
  bulk batch, and upgrade writes an audit row; high-blast-radius actions are held
  for human disposition. Governance is the substrate, not a report bolted on.
- **P1 — opinionated defaults over builders.** Five default roles ship seeded
  (AAD-PARAM-1); there is no runtime custom-role builder, no sharing-hierarchy or
  criteria-rule engine, no per-IdP claim-scripting. Granularity beyond the defaults
  is a source-level extension (P2), never a runtime engine.
- **P7 — included, owned, provable.** SSO, MFA, SCIM, field security, and the
  sandbox carry no edition gate (A36); the license surface reads the same key the
  release service validates; the operator console produces evidence a customer can
  hand to their own auditor. Trust is shipped, not upsold.
- **P11 — real columns, honest shapes.** Field masking is a value transform on real
  columns — a masked field is returned as null with a sibling masked marker, never
  removed from the payload — so RBAC masking and the contract-first machinery
  coexist (AAD-DDL-1).
- **ADR-0039 — manual record-level sharing.** One flat, explicit, revocable,
  audited grant that widens both enforcement layers; deliberately not the
  Salesforce sharing-hierarchy complexity.
- **ADR-0047 / ADR-0029 — seat classes and seat enforcement.** The two business
  ADRs whose *code hooks* live in the product are pinned in this chapter: the seat
  type as a hard capability ceiling below RBAC, and the full-seats-only entitlement
  count the license machinery reports (AAD-PARAM-3/4).
- **ADR-0013 — one governed surface.** SSO/MFA govern human interactive sign-in
  only; agents stay passport-only, and there is no separate agent ACL system —
  RBAC is the single authorization substrate agents map onto
  ([[scope#NEVER-11]], [[threat-model#TM-CTRL-2]]).

## How it works

**One substrate, four terms.** A role bundles object-level permissions
(create/read/update/delete per core object), a row scope (own / team / all,
AAD-PARAM-2), and a field-mask set; users belong to teams, and teams scope
visibility. The role, assignment, team, and grant table shapes are the data-model
chapter's ([[data-model#DM-DDL-3]], [[data-model#DM-DDL-4]],
[[data-model#DM-DDL-5]]). An agent's effective authority is re-derived at admission
as the intersection of its passport scopes, the granting human's current RBAC, and
the seat ceiling — *effective = passport ∩ RBAC ∩ seat ceiling* — so an agent can
never exceed its human ([[threat-model#TM-CTRL-2]]), and a separate agent ACL
system is a scope NEVER ([[scope#NEVER-11]]). Row- and field-level rules apply
identically to agent and human calls: same enforcement path, no agent bypass. The
five-area authorization matrix that pins this is the testing chapter's
([[testing#TEST-RBAC-1]]..5).

**Members and roles are a real screen, shipped in the skeleton.** The management
surface the proof-of-concept already built carries forward: an admin lists members
with their role keys, lists the assignable roles, and assigns or revokes over the
seeded default set. Assignment is idempotent (re-assigning a held role is a no-op
success), the admin gate runs server-side *before any lookup* so a non-admin gets a
flat denial with no existence leak, revoking the final Admin assignment is refused
(a workspace always retains one Admin), and every assign and revoke is audited
(AAD-WIRE-1..4). Role *authoring* stays out: the surface manages the default set
only.

**Per-record sharing widens both layers or nothing.** On top of the own/team/all
tiers, an authorized user shares one record (deal, person, organization, or lead)
with a user or team otherwise out of scope — read or write, optionally time-boxed,
with a reason (ADR-0039). The visibility predicate becomes the base scope *or* an
active matching grant, and the very same clause is added to the row-level-security
backstop policy — a grant that widened only the application query would still see
nothing, so widening is provably dual-layer (AAD-AC-5). Write satisfies read;
an expired grant matches nothing; revocation denies the next request. A human needs
the manage-sharing permission and grants directly; an agent-initiated grant is
always held 🟡 behind the approval gate and can never grant wider than its granting
human holds. Every grant and revoke is an audit row that answers "who can see this,
and under whose authority?" by looking (AAD-WIRE-5..7).

**Field-level security is a value transform, never a shape change.** An admin masks
sensitive fields (margin, comp, PII) per role per object; the declarative policy
table is this chapter's (AAD-DDL-1). A masked field is returned as **null with a
sibling masked-fields marker** — the field is *not omitted*, the response shape
stays fixed — so the value never leaks while contract round-trip machinery still
sees a stable shape (AAD-AC-2). The same mask applies to the UI, the API, and any
agent acting under the role; briefings and AI synthesis respect it too (the
permission-respecting-synthesis obligation cited by the AI chapters). Required
fields cannot be masked and the Admin role masks nothing; mask edits stage first
and enforce only on publish, and every publish is audited (AC-field-security-3..5,
AC-field-security-8).

**The seat type is a hard capability ceiling below RBAC.** Every user carries a
seat type of read or full ([[data-model#DM-DDL-2]], A62/ADR-0047). A read seat —
free, unlimited, never billed, never counted toward any entitlement — may read what
it is granted and use the AI read-only; it can never create, edit, delete, send,
advance a deal, move money, approve a 🟡 action, run a mutating automation, or
export, *regardless of role, record grant, or passport scope*. The ceiling is
orthogonal to the Read-only *role* (which narrows object/row scope and is
assignable to any seat) and cannot be lifted by anything: a write grant to a read
seat is rejected, a passport bind above the read ceiling for a read seat is
rejected, and a read seat's BYO agent is read-only by construction. Violations
resolve to the seat-tier sentinel at the admission gate
([[api-conventions#API-ERR-14]]) — even for an Admin-role read seat (AAD-AC-4).
Billing, entitlement, and the BUSL Additional Use Grant count full seats only —
that counting rule is ADR-0047's and ADR-0029's code hook, pinned here
(AAD-PARAM-3/4).

**Enterprise sign-in ships in the flat tier.** TOTP MFA with recovery codes, a
per-workspace require-MFA policy, and step-up re-authentication for
high-blast-radius admin actions (bulk delete, export, role changes); SSO via SAML
2.0 *and* OIDC against the customer's IdP (Entra ID, Okta, Google Workspace) —
IdP- and SP-initiated, attribute mapping to a CRM user whose assertion never grants
more than the mapped role allows, and an SSO-enforced mode that disables password
login for the workspace (AAD-PARAM-6/7). None of it is an edition gate (A36).
Session security composes with the auth chapter's server-side session model: the
device/session list and remote revoke are this chapter's management surface over
sessions that were built revocable from day one ([[auth-and-sessions]]), and every
authentication event — login, MFA challenge, SSO assertion, failure, lockout,
revoke — lands in the same audit stream as data changes. Agent authentication is
deliberately untouched: passports remain the only network-auth model for agents
(ADR-0013); SSO/MFA introduce no second authorization model.

**SCIM closes the leaver hole within one bus cycle.** SCIM 2.0 directory
provisioning creates, updates, and deactivates users from the IdP, with group→role
mapping. Deprovisioning a leaver disables login, revokes their sessions, and
revokes every dependent agent passport within one event-bus cycle (AAD-PARAM-8) —
the same cascade the RBAC layer promises when a human's object permission is
revoked (AAD-AC-6). The destructive leaver cascade is staged and approved, never
fired blind, and the whole flow is readable from the audit log (AC-scim-3/4/7).

**Bulk operations are one governed batch, not a loop.** Over a selection or a
whole filter result, one operation — set/clear a field, archive, reassign
owner/team, log activity/task, or enrol in a sequence — runs with a pre-commit
count and sample diff. Rows the actor may not edit are excluded from the count and
left untouched, never silently changed. The committed batch is **one** audit entry
(actor, operation, selection, affected ids, before/after) and is undoable as a
batch, with the undo itself audited. Above the workspace threshold (AAD-PARAM-5),
archive/reassign/enrol are 🟡 — held in the same approval inbox as agent actions
([[notifications-and-approval-inbox]]) with the preview diff; an agent invoking a
bulk operation through the tool surface hits the identical gate. Large batches run
as the async bulk job whose table is pinned by records-depth
([[records-depth]], RD-DDL-3) — the *behavior*, screen, and batch/undo semantics
are this chapter's, per the honest-routing note beside that DDL. The same batch
engine is what bulk enrol ([[sequences-and-deliverability]]) and import rollback
([[import-export-migration]]) ride; the consent opt-out check on enrol is
un-overridable by human or agent (S-E11.9, sequences' pin).

**The audit-log viewer makes the spine legible.** The substrate — one write seam,
append-only, one row per mutation — is audit-observability's
([[audit-observability]], AUD-AC-1..8); this chapter owns the *reading surfaces*:
the workspace audit view with live filters (free-text, actor, action type, record
type, date range), agent rows showing the authorizing human, and the per-record
history that renders each row as a plain-language line (AAD-WIRE-8,
AC-settings-14..16). The honest states matter: entries can never be edited or
deleted, and the trail ships with the export.

**License and seats are read, warned, and never enforced silently.** The license
surface shows the key's seat entitlement, the actual active full-seat count, the
plan (flat €25 / free ≤ 10 full seats), and validity — read from the same key the
release service validates (ADR-0029); nothing is free-typed. Approaching or
crossing the entitlement produces a clear warning *before* any enforcement, with
the price named up front; crossing the free tier is shown transparently, and every
key, seat, or plan change is attributable in the audit log (AC-license-1..9).
Read seats never appear in any count (ADR-0047).

**The operator console turns operations into evidence.** For the self-hosted
operator: an available update runs through the upgrade gates — the test suite,
contract-drift, and design-system-drift checks — and a failed gate blocks the
upgrade; a bad update never deploys silently (the fork-upgrade safety machinery).
Backup status shows the last successful backup against the recovery-point and
recovery-time objectives (RPO ≤ 1h, RTO ≤ 4h), and a restore-to-timestamp drill
recovers into a sandbox with erased PII staying suppressed even at a pre-erasure
timestamp — the erasure guarantee survives disaster recovery
([[gdpr-platform]]). Every upgrade, backup, and drill appends to a durable
evidence ledger the operator can hand to an auditor (AC-operator-1..8). The
console itself is permission-gated and renders an honest "denied by policy, not
hidden" state for non-operators.

**The sandbox is where risky change rehearses.** A sandbox is an isolated,
resettable clone of a workspace with an opt-in data subset; isolation is the same
workspace-boundary row-level security that isolates tenants
([[data-model#DM-CONV-5]]..8) — sandbox writes never reach production, asserted by
a boundary test, and reset re-clones from a read-only production. Imports, custom
fields ([[custom-fields]]), and automations ([[automation]]) rehearse there as
dry-runs; promoting a passing change *stages it against production behind an
approval-inbox entry* — promotion never auto-applies (AC-sandbox-1..8).

**German UI and accessibility are CI-checked, not aspirational.** All UI strings
are externalized; a language toggle renders the full UI in German or English with
dates, numbers, and currency formatted to locale — formatting is presentation-only,
stored values never re-typed ([[data-model#DM-FX-7]]). An end-to-end locale test
fails on any hard-coded string in a core flow, and an automated accessibility
check enforces the WCAG 2.2 AA target on core flows in CI, failing the build on
regression (AAD-PARAM-10, AAD-AC-25/26).

**The published API-limits and versioning page is a product surface.** Integrators
and agent operators read documented, enforced per-principal rate limits with the
normative over-limit contract, a defined agent budget, versioned endpoints, and a
published deprecation window and breaking-change definition (S-E15.11). The
enforcement engine and the class table are owned by api-conventions
([[api-conventions]], the RL classes and the 429 contract) and the agent session
quotas by byo-agent-and-mcp ([[byo-agent-and-mcp]]); this chapter owns the *docs
page as a surface* and the versioning-policy commitment (AAD-PARAM-12) — the
promise that nobody is throttled or broken by surprise.

## What's configurable

- **Require-MFA policy** — per workspace, off/on; on, no member signs in
  interactively without a second factor (AAD-PARAM-6). Identity administration is
  deliberately not a runtime-config register row ([[runtime-config]], its
  register-scope rule), and these knobs ride that exemption.
- **SSO configuration and SSO-enforced mode** — the IdP binding (SAML 2.0 or
  OIDC), attribute→role mapping, and the switch that disables password login
  (AAD-PARAM-7); changing enforcement requires the manage-SSO permission.
- **Bulk 🟡 threshold** — the record count above which archive/reassign/enrol
  batches are held for approval; the feature spec mandates the knob, the prototype
  fixes the default at ten (AAD-PARAM-5). ⚠️ Flagged: this is behavior config, not
  identity administration, and the runtime-config register does not yet carry a
  row for it — the register or the exemption note must be reconciled at ticket
  time (AAD-GAP-8).
- **Field-mask policy** — which fields are masked for which role, staged then
  published (AAD-DDL-1).
- **Grant expiry** — optional per grant; the share surface offers no-expiry / 24h /
  7d / 30d with a 7-day default (AAD-PARAM-9).
- **Seat assignment** — read vs full per user; a capability and billing decision,
  warned before any enforcement (AAD-PARAM-3/4).
- **Workspace language** — German or English at launch (AAD-PARAM-10); changing it
  requires the workspace-localize permission.

## Guarantees (enforced)

- **One substrate, no agent bypass.** Agent effective authority is passport ∩
  human RBAC ∩ seat ceiling, re-derived at admission; row and field rules apply
  identically to agent and human calls ([[threat-model#TM-CTRL-2]],
  [[scope#NEVER-11]]; matrix [[testing#TEST-RBAC-1]]..5).
- **Masking never changes the shape.** A masked field is null plus the sibling
  masked marker, never omitted; masking is a value transform, so the contract and
  round-trip machinery stay honest (AAD-DDL-1, AAD-AC-2).
- **A grant widens both layers or nothing.** The record grant appears in the
  application predicate *and* the RLS policy; a single-layer widening is a tracked
  defect, proven by a direct policy probe (AAD-AC-5).
- **The seat ceiling is unliftable.** No role, grant, or passport scope lifts a
  read seat into mutation; the attempt fails closed at admission with the seat-tier
  sentinel, and read seats are never billed or counted (AAD-AC-4, AAD-PARAM-3/4).
- **Revocation cascades within one bus cycle.** Revoking a human's object
  permission invalidates dependent passports; SCIM deprovision disables the user
  and revokes sessions and dependent passports on the same clock (AAD-AC-6,
  AAD-AC-23).
- **A bulk operation is one previewed, bounded, reversible batch.** Exact count
  and sample diff first; un-editable rows excluded, never silently changed; one
  audit entry; undoable as a batch with the undo audited; 🟡-held above threshold
  (AAD-AC-15..18).
- **Sandbox writes never reach production**, and promotion out of the sandbox goes
  through the approval inbox, never auto-applies (AAD-AC-22, AC-sandbox-6).
- **Nothing enforces silently.** License warnings precede any enforcement; a
  failed upgrade gate blocks the upgrade; an unactioned approval never auto-fires
  (AC-license-3, AC-operator-2).
- **The members surface cannot strand or leak.** The admin gate runs before any
  lookup (flat denial, no existence leak), assignment is idempotent, and the last
  Admin cannot be revoked (AAD-WIRE-3/4).
- **DE/EN and WCAG 2.2 AA are build-blocking.** A hard-coded string or an
  accessibility regression on a core flow fails CI (AAD-AC-25/26).
- **Everything here is audited.** Role changes, grants and revokes, mask
  publishes, seat and license changes, auth events, SCIM events, bulk batches and
  undos, upgrades and drills — each writes to the append-only spine owned by
  [[audit-observability]].

## Acceptance

Done means Mor can run the whole trust surface herself and prove it: assign and
revoke roles without stranding the workspace; share one record read-or-write with
an expiry and see exactly who has access and why; mask margin from reps and watch
the API return the same shape with the value honestly withheld; give the whole
company free read seats whose agents provably cannot mutate; turn on MFA and SSO
and force-log-out a lost device; connect the directory and watch a leaver's
passports die within a bus cycle; reassign four hundred records in one previewed,
undoable, threshold-gated batch; read the audit trail for all of it in plain
language; see the seat meter warn before anything degrades; apply an upgrade only
when every gate is green and hand the evidence to an auditor; rehearse a risky
import in a sandbox that provably cannot touch production; and run the whole
product in German at the WCAG 2.2 AA bar. The denied and honest states are
first-class: flat 403s without existence leaks, "denied by policy, not hidden"
consoles, staged-not-enforced mask edits, held-not-executed batches. The testable
form of every claim is pinned in the Acceptance appendix; the cross-cutting screen
floor (standard states, performance budgets) is inherited from
[[acceptance-standards]] and not restated.

## Out of scope

- **Session and passport substrate** — opaque sessions, cookie posture, passport
  mint/revoke mechanics: [[auth-and-sessions]]. This chapter adds the device-list
  management surface and the SSO/MFA/SCIM upgrades that chapter explicitly routed
  here.
- **The audit substrate** — the write seam, append-only trigger, coverage gate,
  trace capture: [[audit-observability]]; table [[data-model#DM-DDL-8]]. Only the
  viewer surfaces are here.
- **The approval inbox** — S-E11.2 and every 🟡 disposition surface:
  [[notifications-and-approval-inbox]]. Bulk over-threshold batches and agent
  grant proposals land there; this chapter only routes to it.
- **Export, migration, and the Data-&-exit / migrate-in settings rows**
  (AC-settings-5/6/7) — [[import-export-migration]].
- **The buyer preference center and un-overridable opt-out** (S-E11.9) —
  [[sequences-and-deliverability]]; the bulk-enrol panel here only surfaces its
  suppression verdicts.
- **RBAC/grant/team/seat table DDL** — [[data-model#DM-DDL-2]]..5; the bulk-job
  table DDL — [[records-depth]] (RD-DDL-3). Behavior here, shapes there.
- **Custom-field mechanics** rehearsed in the sandbox — [[custom-fields]].
- **Rate-limit classes, caps, and the 429 contract** — [[api-conventions]];
  **agent session quotas** — [[byo-agent-and-mcp]]. This chapter owns only the
  published docs page and versioning policy.
- **Territory-based sharing, sharing hierarchies, criteria auto-share,
  grant-of-grant delegation** — deliberately out; territories are the committed
  first fast-follow (ADR-0039, A52).
- **A runtime custom-role builder and per-tool re-tiering** — never (P1/P2,
  ADR-0026).

## Where it lives

The identity module inside the backend's modules tree — RBAC administration,
grants, masks, seats, enterprise sign-in, SCIM, and the bulk-ops engine reach data
through the same admission and mutation seams as every caller — plus the admin and
operator screens in the web shell. Read next: [auth-and-sessions](auth-and-sessions.md)
(the credential substrate this administers),
[data-model](../architecture/data-model.md) (the tables),
[audit-observability](audit-observability.md) (the spine the viewer reads),
[notifications-and-approval-inbox](notifications-and-approval-inbox.md) (where 🟡
batches land), and [import-export-migration](import-export-migration.md) (the
neighbouring settings rows).

## Appendix

### Parameters
Source: specs/spec/features/04-platform-and-compliance.md#1-permissions-teams--roles-rbac-rowfield-level + #7 + #8 @ 5a0b29c; specs/spec/decisions/ADR-0047-free-read-seats-paid-full-seats.md @ 5a0b29c; specs/spec/decisions/ADR-0029-busl-license-and-seat-enforcement.md @ 5a0b29c; specs/spec/product/30-screen-acceptance.md (screen defaults) @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| AAD-PARAM-1 | Default role set | Admin · Manager · Rep · Read-only · Ops/Integrations | Seeded per workspace; `is_system` rows of [[data-model#DM-DDL-4]]. Custom roles beyond these are a code extension on dedicated/source deployments (P1/P2), never a runtime builder. |
| AAD-PARAM-2 | Row scopes | `own` / `team` / `all` | The role's row scope drives query construction and the RLS-backstop predicate; pipelines and lists can be team-scoped. |
| AAD-PARAM-3 | Seat classes (A62/ADR-0047) | `read` = free, unlimited, never billed, never counted; `full` = €25/seat/month, free ≤ 10 full seats, 100+ contact sales | `seat_type` on [[data-model#DM-DDL-2]] is a hard capability ceiling below RBAC: a read seat can never mutate/send/advance/approve/export, and its agent inherits the read-only ceiling. Same price self- or partner-hosted. |
| AAD-PARAM-4 | Entitlement counting rule (ADR-0029 hook) | active `full` seats ≤ license entitlement; read seats never reported | The BUSL Additional Use Grant "Seat" = a full seat; the license key encodes the full-seat entitlement, the instance reports active full seats at true-up, and validation enforces the count server-side via the release service. |
| AAD-PARAM-5 | Bulk 🟡 threshold | 10 records (workspace default; configurable) | Bulk archive / reassign / enrol above the threshold is held 🟡 in the approval inbox with the preview diff; edit/activity ops commit directly. The feature spec (features/04 §8) mandates the knob; the prototype fixes the default. |
| AAD-PARAM-6 | MFA baseline | TOTP authenticator + 10 recovery codes; per-workspace require-MFA; step-up re-auth on high-blast-radius admin actions (bulk delete, export, role change) | Regenerating recovery codes invalidates the current set and itself requires step-up (AC-security-8). |
| AAD-PARAM-7 | SSO protocols | SAML 2.0 **and** OIDC (Entra ID / Okta / Google Workspace); IdP- and SP-initiated; attribute→user/role mapping; SSO-enforced mode disables password login | No edition gate (A36). The assertion never grants more than the mapped role allows. Password column stays empty for SSO-provisioned users ([[auth-and-sessions]]). |
| AAD-PARAM-8 | SCIM deprovision SLA | ≤ 1 event-bus cycle (p95) | From directory delete on the bus to session revocation + dependent passport revocation; measured from audit-log spans (AC-scim-2). |
| AAD-PARAM-9 | Grant expiry presets | none / 24 h / 7 d / 30 d; share-surface default 7 d | `expires_at` on [[data-model#DM-DDL-5]]; an expired grant matches nothing. Write satisfies read. |
| AAD-PARAM-10 | Locales & accessibility bar | de-DE + en(-GB) at launch; WCAG 2.2 AA on core flows, automated check in CI | Locale formatting is presentation-only ([[data-model#DM-FX-7]]); RTL, per-user override, translated AI long-form are post-V1. |
| AAD-PARAM-11 | Shareable record types | `deal` · `person` · `organization` · `lead`; access `read` \| `write` | The V1 record-grant domain (ADR-0039); one generic table, not per-type sub-resources. |
| AAD-PARAM-12 | API versioning policy (S-E15.11b) | versioned endpoints (the v1 path prefix, [[api-conventions#API-CONV-3]]); a published deprecation window + breaking-change definition | ⚠️ The window length and breaking-change definition are not yet ratified anywhere in the corpus — the docs-page ticket must pin them (AAD-GAP-7). Event-schema versioning is [[byo-agent-and-mcp]]'s BYO-EVT-1. |

### Schema
Source: specs/spec/contract/data-model.md#125-net-new-v1-objects-signals-deal-rooms-voice-agent-connections-automation-views-quota-field-mask @ 5a0b29c (the `field_mask` block + its masking note); ownership index [[data-model]] (66-table partition)

This chapter single-homes exactly one table per the ownership index: `field_mask`.
DDL verbatim from the corpus (base columns per [[data-model#DM-CONV-3]] implied):

**AAD-DDL-1 — `field_mask`.** Declarative field-level RBAC: which roles may SEE
which fields.

```sql
CREATE TABLE field_mask (                                  -- declarative field-level RBAC: which roles may SEE which fields (E11, RT-AR-H2)
  -- + base columns
  role_id       uuid NOT NULL REFERENCES role(id),
  entity_type   text NOT NULL,
  field_name    text NOT NULL,
  visibility    text NOT NULL DEFAULT 'visible' CHECK (visibility IN ('visible','masked')),
  UNIQUE (workspace_id, role_id, entity_type, field_name)
);
```

**AAD-DDL-N-1 — the normative masking semantics (closes the corpus's RT-AR-H2).**
A masked field is returned as **`null` with a sibling `_masked: [field,…]` marker**,
NOT omitted from the payload. The response shape stays fixed (the field stays
nullable in the contract, the completeness gate still sees it, and contract diffing
never sees a required-property removal). Masking is a *value* transform, not a
*shape* transform. ⚠️ **Contested restatements, flagged:** the corpus's own RBAC
section note ("a masked field is *absent* from the payload"), the skeleton
data-model prose beside [[data-model#DM-DDL-4]], and a records-depth prose line
repeat the older absent-from-payload wording; the ownership-index row for
`field_mask` ("masked = null + marker, shape-stable") and this pin carry the
deliberate resolution. Build to null + marker; the stale restatements need their
owner's-ID tags reconciled. The field-security screen prototype also drifts
(see the ⚠️ on AC-field-security-6 below).

**AAD-DDL-N-2 — honest table findings (no DDL exists; reported, not invented):**

| ID | Would-be table | Finding |
|---|---|---|
| AAD-GAP-1 | sandbox workspace | **No dedicated table.** A sandbox is a cloned `workspace` row plus an opt-in data subset; isolation is the ordinary workspace-boundary RLS ([[data-model#DM-CONV-5]]..8). Clone/reset lineage (source workspace, subset definition, re-clone timestamp) has no corpus DDL — the sandbox ticket must mint it or justify statelessness. |
| AAD-GAP-2 | license / entitlement | **No table, by design.** The entitlement lives in the signed license key validated by the release service (ADR-0029); the active-seat count derives from `app_user.seat_type` ([[data-model#DM-DDL-2]]). Any local cache of key state / true-up reports needs DDL minted at ticket time. |
| AAD-GAP-3 | SCIM connection state | **No corpus DDL** for the directory binding (endpoint, token hash, group→role map, sync cursor). Token storage should ride the sealed secret vault pattern ([[data-model#DM-DDL-11]], ADR-0048). Contract-extension ticket required. |
| AAD-GAP-4 | operator evidence ledger | **No corpus DDL** for upgrade/backup/DR-drill evidence records (S-E11.11 requires durable evidence). May project from `audit_log` or need its own table — ticket decision. |
| AAD-GAP-5 | MFA enrolment / SSO IdP config | **No corpus DDL** (TOTP secrets, recovery-code hashes, IdP metadata). Secrets must follow the never-returned, hash-or-sealed storage posture (ADR-0043/ADR-0048 patterns). Contract-extension ticket required. |

Cited, not owned here: `app_user` (seat type) [[data-model#DM-DDL-2]]; `team` /
`team_membership` [[data-model#DM-DDL-3]]; `role` / `role_assignment`
[[data-model#DM-DDL-4]]; `record_grant` [[data-model#DM-DDL-5]]; `session` / `passport`
[[data-model#DM-DDL-6]]/7; `audit_log` [[data-model#DM-DDL-8]]; `bulk_operation`
[[records-depth]] RD-DDL-3 (DDL there, behavior here per its RD-DDL-N-2).

### Wire
Source: specs/spec/contract/crm.yaml (Access tag; DEFERRED stub block) @ 5a0b29c; margince-poc/contract/crm.yaml (Identity + Audit tags) @ a11d6c08

Operations are cited by contract operationId; shapes live in the contract and are
never restated. Error semantics: seat ceiling → [[api-conventions#API-ERR-14]]
(`seat_tier_insufficient`); agent 🟡 without token → [[api-conventions#API-ERR-10]]
(`approval_required`); bind-time over-scope → `scope_exceeds_grantor` (pinned by
[[byo-agent-and-mcp]], per the note on [[api-conventions#API-ERR-9]]).

| ID | operationId | Behavior pinned |
|---|---|---|
| AAD-WIRE-1 | `listRoles` (poc) | Lists the workspace's assignable roles (the seeded default set). Admin-gated server-side. |
| AAD-WIRE-2 | `listMembers` (poc) | Lists members with their role keys. The manage-members check runs **before any lookup** — a non-admin gets a flat denial with no existence leak; cross-workspace targets read as not-found. |
| AAD-WIRE-3 | `assignMemberRole` (poc) | Assigns a role to a member; **idempotent** (re-assigning a held role is a no-op success, never a duplicate row). Audited. |
| AAD-WIRE-4 | `revokeMemberRole` (poc) | Revokes a role; revoking the final Admin assignment is refused (**last-admin guard**). Audited. |
| AAD-WIRE-5 | `listRecordGrants` | Lists active manual grants filtered by record ("who has access to this?") or by subject ("what was this user/team granted?"); surfaces only the manual grants on top of the tiered base scope. 🟢 read. |
| AAD-WIRE-6 | `createRecordGrant` | Shares one record read/write with a user or team; widens BOTH the application predicate AND the RLS backstop; idempotent on the (record, subject) tuple — re-asserting upgrades/downgrades access and resets expiry; can never exceed the granter's own access; requires the manage-sharing permission; **🟡 for agents** (approval token, `share_record` verb). Audited as record-share. |
| AAD-WIRE-7 | `revokeRecordGrant` | Revokes a grant; the subject loses the widened access at the next query. **🟡 for agents.** Audited as record-unshare. |
| AAD-WIRE-8 | `getRecordHistory` (poc) | Per-record audit history rendered as plain-language lines with actor and (for agents) the human authority; requires object-level read; empty array, never 404, for no-history ids. ⚠️ The poc masks before/after diffs by *omitting* unreadable fields — reconcile with the null + `_masked` rule (AAD-DDL-N-1) at contract-extension time. |

**Honest wire gaps (contract-extension tickets must mint these before
implementation tickets can cite them):**

| ID | Missing surface | Note |
|---|---|---|
| AAD-GAP-6 | Admin identity CRUD (`/users`, `/teams`, `/roles` beyond the poc's read/assign ops) and the workspace-wide `/audit-log` read API | The corpus contract carries their **schemas** but defers the endpoints ("fast-follow" stubs). The audit-log *viewer* (AC-settings-14..16) cannot build without the read API — the deferral collides with the V1-Must viewer surface; flag to the contract owner. |
| AAD-GAP-7 | SSO/MFA management + session/device list ops; the published rate-limit/versioning docs page | No operations exist for IdP config, MFA enrolment, require-MFA, SSO-enforced mode, session list/remote-revoke, or the S-E15.11 docs surface. |
| AAD-GAP-8 | SCIM 2.0 protocol surface | SCIM is its own standardized endpoint family (Users/Groups), separate from the product API; nothing in the contract yet. Also: the bulk 🟡 threshold knob has no runtime-config register row (see What's configurable). |
| AAD-GAP-9 | Bulk-operations ops | `bulk_operation` has DDL (RD-DDL-3) but no wire surface (submit / status / undo / approve-queue handoff). |
| AAD-GAP-10 | Field-mask admin, sandbox lifecycle (create/reset/promote), license/seat surface, operator console ops, workspace-language op | None specified; each named screen below implies its op set. |

### Events
Source: specs/spec/contract/events.md#5-the-catalog @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#scimhtml--scim-provisioning-implements-s-e159c @ 5a0b29c

| ID | Direction | Event | Note |
|---|---|---|---|
| AAD-EVT-1 | consumed | `audit.appended` | The viewer surfaces read the spine this event announces; definition owned by [[event-bus]] / [[audit-observability]] (AUD-EVT-1). |
| AAD-EVT-2 | emitted (gap) | permission-revocation cascade; SCIM lifecycle (`scim.user.created` / `scim.user.deleted`), session-revoked, passport-revoked | ⚠️ **Not in the event catalog.** The one-bus-cycle cascade (AAD-AC-6, AAD-AC-23) and the SCIM event log (AC-scim-7) depend on these; they must be minted via the event-catalog extension before build. Naming per the catalog's entity.verb convention. |

### Acceptance

**Feature acceptance — RBAC, sharing, masking, seat ceiling (verbatim from the
feature spec; corpus bullets carry no IDs — minted here).**
Source: specs/spec/features/04-platform-and-compliance.md#1-permissions-teams--roles-rbac-rowfield-level @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AAD-AC-1 | `[MV]` A `Rep` cannot read a record owned by another team; API returns `403`/filtered list, never the row. Covered by an authorization test matrix (role × object × action × ownership) in the test suite. | Integration lane; the matrix is [[testing#TEST-RBAC-1]]/2, seed `rbac-matrix` ([[testing]] catalog) |
| AAD-AC-2 | `[MV]` A field masked for a role is returned as **`null` with a sibling `_masked` marker** (the field is **not omitted** from the payload — the response shape stays fixed), so its *value* never leaks while the contract/round-trip machinery still sees a stable shape — not merely hidden in the UI. Verified by a contract-level response-shape test. | Contract response-shape test; [[testing#TEST-RBAC-3]]; semantics AAD-DDL-N-1 |
| AAD-AC-3 | `[MV]` An attempt to bind an Agent Seat Passport with a scope exceeding the granting human's RBAC is rejected (`422`) with reason `scope_exceeds_grantor`. | Agent-surface tests; error owned by [[byo-agent-and-mcp]]; [[testing#TEST-RBAC-4]], seed `passport-overscope` |
| AAD-AC-4 | `[MV]` A `read`-seat user (or an agent acting for one) attempting any mutate/send/advance/approve, an export, or receiving a `write` `record_grant`, is rejected with the seat-tier sentinel (`403 seat_tier_insufficient`) at the admission gate — **even with an Admin role**. A `full`-seat user in the same role succeeds (subject to RBAC). Binding a Passport above the `{read}` ceiling for a `read` seat is rejected `422 scope_exceeds_grantor`. Covered by the authorization test matrix (seat_type × role × action). Billing/entitlement and the BUSL seat cap count `seat_type='full'` only (A62/ADR-0047). | Authorization matrix extension (seat dimension); [[api-conventions#API-ERR-14]] |
| AAD-AC-5 | `[MV]` A user granted `read` on a single out-of-scope record via `record_grant` can fetch exactly that record and no sibling — proven against **both** the application query and a direct RLS-policy probe (a grant that widened only the app query but not the policy is a tracked defect); an expired grant returns `403`/filtered, and revoking it denies the next call. An agent calling `share_record` is **held (🟡)**, never auto-applied, and cannot grant access wider than its granting human holds. Every grant/revoke is attributable in `audit_log`. | Authorization matrix + grant-specific RLS probe test; 🟡 path via [[notifications-and-approval-inbox]] |
| AAD-AC-6 | `[MV]` Revoking a human's object permission invalidates dependent Passports within one event-bus cycle; a subsequent agent call returns `403`. | Integration test against the event bus (events: AAD-EVT-2 gap); composes with [[auth-and-sessions]] AUTH-AC-3 (direct revoke is synchronous) |
| AAD-AC-7 | An authorization decision (allow/deny + governing rule) is attributable in `audit_log` for any mutation. | Rides the audit seam ([[audit-observability]] AUD-AC-4); mutation-coverage gate [[quality-gates#QG-11]] |
| AAD-AC-8 | **User-observable (Mor, S-E11.1/S-E11.3):** when Mor connects Devin's agent and picks a scope, she sees plainly that the agent *cannot* be granted anything Devin himself lacks — the over-broad scope is refused at her screen with the reason, not silently downgraded. Every action the agent later takes shows in the record timeline and audit view attributed to *both* the agent and the human who authorized it (the Passport), so Mor can always answer "who did this, and under whose authority?" by looking, not by querying the DB. | Screen e2e lane (mechanics: AAD-AC-3, AUD-AC-4/5) |
| AAD-AC-9 | **User-observable (Devin, S-E10.1):** when Devin connects his own Claude/Cursor and grants only read+draft, his agent can read and draft against records he can see and *cannot* edit, send, or reach a record he can't — and Devin can see that scope and revoke it himself at any time. | Screen e2e lane; the connection surface is [[byo-agent-and-mcp]]'s (S-E10.1 primary there); restated here because §1's enforcement primitive is this chapter's |

**Feature acceptance — SSO, MFA & session security (verbatim; IDs minted here).**
Source: specs/spec/features/04-platform-and-compliance.md#7-authentication--sso-mfa--session-security @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AAD-AC-10 | `[MV]` A workspace with require-MFA enabled refuses interactive sign-in without a passed second factor; a privileged action triggers step-up re-auth. Asserted by an auth-flow test matrix. | Integration lane, identity module |
| AAD-AC-11 | `[MV]` A SAML assertion and an OIDC token from a seeded IdP both resolve to the mapped CRM user with the correct role; SSO-enforced mode rejects password login (`403`). | Contract/integration test against a mock IdP |
| AAD-AC-12 | `[MV]` Every authentication event (success, MFA challenge, SSO assertion, failure, lockout, session revoke) writes an `audit_log` entry — verified by an auth-event coverage test. | Coverage test over the audit seam ([[audit-observability]]) |
| AAD-AC-13 | `[MV]` A revoked session is rejected on its next request within one event-bus cycle. | Integration test; the shipped session model already fails closed at lookup ([[auth-and-sessions]] AUTH-AC-3) — the bus-cycle bound is the ceiling, not the mechanism |
| AAD-AC-14 | **User-observable (Mor, S-E11.8):** Mor turns on "require MFA" and connects the company IdP once; from then on the team signs in with their existing corporate login, she can force-log-out a lost device, and every sign-in shows in the same audit view as every data change — so the security story she has to pass to a regulated customer's IT is "yes, SSO and MFA, included, audited," not "upgrade to Enterprise." | Screen e2e lane (mechanics: AAD-AC-10..13) |

**Feature acceptance — bulk data operations (verbatim; IDs minted here).**
Source: specs/spec/features/04-platform-and-compliance.md#8-bulk-data-operations-edit--archive--reassign @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AAD-AC-15 | `[MV]` A bulk field-set over a selection of N applies to exactly the rows the actor may edit, excludes the rest from the count, and produces **one** batch `audit_log` entry listing affected ids + before/after. | Backend integration lane (batch engine over RD-DDL-3) |
| AAD-AC-16 | `[MV]` A bulk archive/reassign over the 🟡 threshold is **held** (not executed) and appears in the approval inbox with the preview diff; on approve, exactly the previewed set changes; on reject, nothing does. | Integration lane with [[notifications-and-approval-inbox]]; threshold AAD-PARAM-5 |
| AAD-AC-17 | `[MV]` A committed batch is undoable: the inverse operation restores prior field values / owner / archived state for every row in the batch (round-trip test), and the undo is itself audited. | Round-trip integration test; compensating batch, never an in-place audit rewrite (per the corpus note beside RD-DDL-3) |
| AAD-AC-18 | `[MV]` A bulk op invoked via the agent MCP surface obeys the same RBAC exclusion and 🟡 gate as the UI (no agent bypass). | Shared enforcement test via the tool entry point ([[byo-agent-and-mcp]]) |
| AAD-AC-19 | Bulk op over 10k rows dispatches as a background job (off the hot path) with progress; the synchronous acknowledgement returns p95 < 250 ms. | Perf harness; async job states per [[acceptance-standards]] STATE-SP-5 |
| AAD-AC-20 | **User-observable (Mor/Sam, S-E11.7):** when reps shuffle territories or a rep leaves, Mor selects the affected deals/contacts and reassigns them in one move — sees "412 records will change owner to …" first, approves it, and can undo the whole batch if it was wrong, with the change attributable in the audit trail. | Screen e2e lane (mechanics: AAD-AC-15..17) |

**Feature acceptance — enterprise admin, security & platform (verbatim; IDs minted here).**
Source: specs/spec/features/10-operational-depth.md#7-enterprise-admin-security--platform-promotes-d103-field-level-security-d105-sandbox-d107-scim-d112-rate-limits-d1111-i18n-d1112-wcag @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AAD-AC-21 | A field masked for a role is returned as **`null` with a sibling `_masked` marker** (**not omitted** from the payload — the shape stays fixed; value never leaks) — contract response-shape test (as features/04 §1). | Sanctioned restatement of AAD-AC-2 (same pin, promoted-to-MVP source) |
| AAD-AC-22 | A sandbox is isolated (its writes never reach production; RLS/workspace boundary test) and can be reset. | Boundary integration test over [[data-model#DM-CONV-5]]..8 |
| AAD-AC-23 | A SCIM deprovision disables the user and cascades to dependent Passports within one event-bus cycle (composes with features/04 §1 revocation). | Integration test against the bus (AAD-PARAM-8; events AAD-EVT-2 gap) |
| AAD-AC-24 | Rate limits are enforced + documented with versioned endpoints; agent workloads get a defined budget. | Enforcement owned by [[api-conventions]] (RL classes, the 429 contract) + [[byo-agent-and-mcp]] (session quotas); this chapter's testable slice is the **published docs page** (S-E15.11a/b) and AAD-PARAM-12 |
| AAD-AC-25 | DE/EN UI strings are externalized; a locale switch is covered by an e2e test; CI runs an automated accessibility check at the WCAG 2.2 AA bar on core flows. | e2e locale test + CI a11y gate (build-blocking, S-E15.10a/b) |
| AAD-AC-26 | **User-observable (Mor, S-E15.9/.10):** Mor hides margin from reps, tries an import in a sandbox first, connects SCIM so leavers lose access automatically, and her team uses the product in German — and it passes her accessibility checklist. | Screen e2e lane (mechanics: AAD-AC-21..25) |

**Story acceptance (condensed — one row per owned story; user-side G/W/T
condensed, full text in the epic).**
Source: specs/spec/product/epics/E11-access-trust-exit.md @ 5a0b29c; specs/spec/product/epics/E15-operational-depth.md @ 5a0b29c

| ID | Story | Condensed acceptance | Verification |
|---|---|---|---|
| AAD-AC-27 | S-E11.1 (V1-Must) | Default roles bound humans and agents identically: a Rep sees own/team per role (403/filtered otherwise); an agent's effective rights are the intersection with its passport; over-scope grants are rejected `scope_exceeds_grantor`; revoking the human cascades within one bus cycle, audited. | AAD-AC-1/3/6; [[testing#TEST-RBAC-1]]..4 |
| AAD-AC-28 | S-E11.3 (V1-Must, viewer surface) | Every mutation is attributable with before/after and governing rule; agent entries open a replayable trace; the log rejects UPDATE/DELETE; the trail ships with the export. **This chapter owns only the viewer**; the substrate ACs are [[audit-observability]] AUD-AC-1..7 and the export inclusion is [[import-export-migration]] (AC-settings-5). | Screen e2e over the audit view (AC-settings-14..16); substrate cited |
| AAD-AC-29 | S-E11.7 (V1-Must) | Selection/filter bulk edit·archive·reassign shows the exact count + sample first; un-editable rows excluded, never silently changed; over-threshold held in the approval inbox; the whole batch (and its undo) is auditable and undoable. | AAD-AC-15..20; screen AC-bulk-actions-1..9 |
| AAD-AC-30 | S-E11.8 (V1-Must) | Require-MFA blocks second-factor-less sign-in with step-up on privileged actions; SAML/OIDC maps assertions to user/role with SSO-enforced mode disabling passwords; the device list supports remote revoke; every auth event is in the audit view; no edition upgrade. | AAD-AC-10..14; screen AC-security-1..8 |
| AAD-AC-31 | S-E11.10 (V1-Must) | The license section shows entitlement vs actual active seats, the plan and validity read from the validated key; warnings precede any enforcement (never a silent lockout); the free→paid transition is transparent; every license/seat change is audited and the reported seat count recorded. | Screen AC-license-1..9; AAD-PARAM-3/4 |
| AAD-AC-32 | S-E11.11 (V1-Must) | An update applies via the signed upgrade client only when the test-suite/contract-drift/design-drift gates pass (a failed gate blocks); backup status confirms RPO ≤ 1h / RTO ≤ 4h and a restore-to-timestamp drill runs with erased PII staying suppressed; upgrade/backup/drill each produce durable auditor-ready evidence. | Screen AC-operator-1..8; erasure-on-restore composes with [[gdpr-platform]] |
| AAD-AC-33 | S-E11.12 (V1-Must) | Share adds a user/team with read/write, optional expiry + reason; the subject sees exactly that record and nothing else changes; revoke/expiry denies the next request, proven on the data path; an agent share is 🟡 with who/what/why and can never exceed the granter; every grant/revoke is attributable. | AAD-AC-5; screen AC-share-1..8 |
| AAD-AC-34 | S-E15.9a (V1-Must) | Per-role field masking that API and UI both honor, applying identically to any agent under that role. | AAD-AC-2/21; screen AC-field-security-1..8 |
| AAD-AC-35 | S-E15.9b (V1-Must) | An isolated, resettable sandbox clone where a risky change (import, custom field, automation) never reaches production. | AAD-AC-22; screen AC-sandbox-1..8 |
| AAD-AC-36 | S-E15.9c (V1-Must) | Directory-driven provision; on a leaver, deprovision + dependent agent-Passport revocation within one event-bus cycle. | AAD-AC-23; screen AC-scim-1..8 |
| AAD-AC-37 | S-E15.10a (V1-Must) | All UI strings externalized; the toggle renders the full UI in German; dates/numbers/currency format to locale; the e2e locale test asserts no untranslated string in core flows. | AAD-AC-25; screen AC-localization-1..8 |
| AAD-AC-38 | S-E15.10b (V1-Must) | Core flows meet WCAG 2.2 AA — keyboard-operable, visible focus, AA contrast, labelled controls/landmarks — with an automated CI check that fails the build on regression. | AAD-AC-25 (a11y half); CI gate |
| AAD-AC-39 | S-E15.11a (V1-Must) | Documented per-principal limits enforced server-side; over-limit returns a clear 429 with retry and remaining-budget headers; an agent workload has its own budget and degrades predictably. | Enforcement pins cited (RL classes + 429 contract, [[api-conventions]]; quotas, [[byo-agent-and-mcp]]); this chapter gates the published page |
| AAD-AC-40 | S-E15.11b (V1-Must) | A published versioning policy with versioned endpoints, a documented deprecation window, and a breaking-change definition — integrators are never broken without notice. | AAD-PARAM-12 (window/definition to be ratified at ticket time, AAD-GAP-7) |

**Owned screen acceptance — workspace settings, the series remainder (verbatim;
corpus IDs preserved).** [[import-export-migration]] owns AC-settings-5/6/7; the
rest of the series is pinned here. Rows whose *behavior* is another chapter's carry
the owner in Verification — the screen pin is this chapter's, the mechanics are
cited.
Source: specs/spec/product/30-screen-acceptance.md#settingshtml--workspace-settings-implements-s-e111345-s-e105 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-settings-1 | (nav + scroll-spy): Given the page loads, When rendered inside the app shell, Then a sticky in-page nav lists Members & roles, Data & exit, Connected surfaces, Booking & availability, Suite — Dispact, AI & autonomy, Audit log; scrolling updates the active item (Members default). | Screen e2e lane |
| AC-settings-2 | (members list): Given the Members & roles section, When rendered, Then it lists members each with avatar, name, email, and a role pill — including a service identity (`svc:`) with the Ops/Integrations role — plus an "Invite a member" row. | Screen e2e lane (AAD-WIRE-2) |
| AC-settings-3 | (5-role RBAC matrix): Given the permission-posture table, When viewed, Then it shows exactly the 5 default roles (Admin, Manager, Rep, Read-only, Ops/Integrations) with per-role columns (See records / Deal amount / Create-edit deal / Admin & export), where Rep sees own records with deal amount masked for non-owners, and Ops/Integrations is read-all but explicitly denied deal create/update. | Screen e2e lane (AAD-PARAM-1) |
| AC-settings-4 | (human+agent parity): Given the Members intro, When read, Then it states roles bound humans AND agents identically, an agent's rights are the intersection of the human's RBAC and Passport scope, and it can never exceed the granting human (S-E11.1). | Screen e2e lane ([[threat-model#TM-CTRL-2]]) |
| AC-settings-8 | (right to erasure): Given the Right-to-erasure block, When viewed, Then it is visually distinct, states it is NOT the reversible "delete contact", permanently wipes the person across record/activity/derived-values/embeddings, writes a tombstone to the audit log with no PII re-stored (salted hash only), and adds email/phone as salted hashes to a re-import suppression list; the "Erase a person…" action is itself audited. | Screen e2e lane; **behavior owned by [[gdpr-platform]]** (erasure + suppression pins) |
| AC-settings-9 | (connected surfaces toggles): Given the Connected surfaces section, When viewed, Then it shows the Gmail/Outlook sidebar and LinkedIn capture→lead surfaces, each with an on/off switch, an "egress: your workspace only" badge, a "Preview the surfaces" link → client-surfaces.html, and copy that the extension obeys permissions and is not a back door around the send gate. | Screen e2e lane; **behavior owned by [[capture]] / the client-surfaces chapter** |
| AC-settings-10 | (booking & routing): Given the Booking & availability section, When viewed, Then it shows the booking link, availability config (free-busy, min notice, buffer, durations, timezone), and graph-native routing rules 1–4 (known contact on open deal → deal owner; known account → account manager; new ICP domain → territory; no match → default pool), plus a "rules as code" reviewable-PR preview and a note the same engine is a governed MCP tool under the same RBAC + confirm-first gates. | Screen e2e lane; **behavior owned by [[meetings-and-transcripts]]** |
| AC-settings-11 | (Dispact optional/standalone): Given the Suite — Dispact section, When viewed, Then Dispact shows "Connected" with a Disconnect toggle; toggling flips Connected ↔ Not connected, and copy states the CRM works fully on its own and keeps running unchanged when Dispact is disconnected. | Screen e2e lane; **behavior owned by the dispact-integration chapter** |
| AC-settings-12 | (AI autonomy tiers): Given the AI & autonomy section, When viewed, Then a per-tool table lists tools with a tier badge — Read/enrich and Draft email 🟢 auto; Send email 🟡 confirm; Advance deal stage 🟡 confirm and LOCKED (won/lost always confirm); the locked switch cannot be toggled to auto and explains why on click. | Screen e2e lane; **tier vocabulary owned by [[byo-agent-and-mcp]] / [[intent-tools]]** (ADR-0026: tiers tighten, never loosen) |
| AC-settings-13 | (egress posture — location not redaction): Given the egress-posture selector (A8), When viewed, Then three profiles are selectable — EU-sovereign hosted (default), Sovereign zero-egress (local only), Cloud frontier (under DPA, secrets stripped, no PII pseudonymization) — selecting one marks it and toasts; plus swappable model-tier bindings and a soft (no-hard-stop) per-workspace AI budget. | Screen e2e lane; **behavior owned by [[ai-runtime]]** (egress ladder, D7) and the budget by its owning chapter |
| AC-settings-14 | (audit log — human+agent attributable): Given the Audit log section, When viewed, Then it shows append-only entries each with timestamp, actor (human OR specific agent, agents showing "auth: <human>" + bot icon), action, and record; it states entries can never be edited or deleted and ship with the export. | Screen e2e lane (viewer surface, AAD-AC-28; substrate [[audit-observability]]) |
| AC-settings-15 | (audit filters): Given the audit filter bar, When the user filters by free-text, actor (all/humans/agents/specific), action type, record type, or date range, Then the table re-filters live; no match → "No entries match these filters." | Screen e2e lane (needs the audit-log read API — AAD-GAP-6) |
| AC-settings-16 | (audit shows agent + erasure rows): Given the default audit rows, When viewed, Then they include agent actions (with `auth:` authority), an export row, a role-change row, and a GDPR erasure tombstone row — proving both human and agent attribution. | Screen e2e lane (fixture; tombstone semantics [[gdpr-platform]]) |

**Owned screen acceptance — bulk operations (verbatim; corpus IDs preserved).**
Primary story S-E11.7; the enrol panel's consent verdicts are
[[sequences-and-deliverability]]'s (S-E11.9), cited by AC-bulk-actions-9.
Source: specs/spec/product/30-screen-acceptance.md#bulk-actionshtml--bulk-operations-implements-s-e117-s-e155c @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-bulk-actions-1 | Given the screen at load, When it renders, Then a breadcrumb (Contacts · Filter: Brandt Automotive · Bulk operations) and a "bounded by your role · Sales Manager" provenance pill appear, and the selection bar shows "14 of 16 selected will change" with a "select all 16 in filter" link and a Clear button. | Screen e2e lane |
| AC-bulk-actions-2 | Given the selection includes records owned by another team, When the table renders, Then an honesty note states "2 of 16 are excluded" and those rows render dimmed with a disabled checkbox and an "🔒 other team" lock badge whose hover title cites the RBAC scope (S-E11.1); excluded rows stay visible but are not counted in the change total. | Screen e2e lane (AAD-AC-15) |
| AC-bulk-actions-3 | Given the eligible rows are checked, When I uncheck a row (or use the header checkbox / Clear), Then the selection count recomputes (the header toggles only enabled rows), the preview and the run-button label update to the new count, and disabled (excluded) rows never become selectable. | Screen e2e lane |
| AC-bulk-actions-4 | Given the operation chooser, When I click an op tab (Set/clear a field, Reassign owner, Archive, Log activity/task, Enrol in sequence), Then that tab becomes active and its input panel shows while the others hide; field tabs expose their controls (e.g. Field + New value with a "— Clear field —" option for edit; new-owner select; archive reason; activity type/date/note; sequence select). | Screen e2e lane |
| AC-bulk-actions-5 | Given an operation and selection, When the preview builds, Then a diff panel shows a header naming the changed field and the record count plus a per-record sample (up to 3 rows) rendering old-value → new-value, with a "+ N more records will get the same change" line when the count exceeds the sample. | Screen e2e lane |
| AC-bulk-actions-6 | Given a high-blast-radius op (reassign, archive, or enrol) over the workspace threshold of 10 records, When the gate evaluates, Then a 🟡 banner appears stating it is held for approval, the primary button changes to "Queue N to approval inbox", and the run meta reads "🟡 approval-gated · audited · reversible"; edit and activity ops below/over threshold commit directly with meta "batch · audited · reversible". | Screen e2e lane (AAD-PARAM-5, AAD-AC-16) |
| AC-bulk-actions-7 | Given a direct-commit op, When I click the run button, Then a green "Batch committed." banner appears with a batch id (e.g. batch_8f2a1c), an "Undo whole batch" chip, and a "View in audit log →" link; clicking Undo hides the banner and toasts that values, owner, and archived state are restored and the undo is itself audited. | Screen e2e lane (AAD-AC-17) |
| AC-bulk-actions-8 | Given a 🟡-gated op, When I click "Queue N to approval inbox", Then the banner instead reads "Held for approval." stating nothing has changed yet and the work is queued to the approval inbox with the exact preview diff, and the Undo chip is hidden (there is nothing yet to undo). | Screen e2e lane ([[notifications-and-approval-inbox]]) |
| AC-bulk-actions-9 | Given the enrol-in-sequence op, When the panel renders, Then a consent note flags "1 contact blocked by opt-out" (Julia Roth, citing the consent_event and S-E11.9 that opt-out cannot be overridden by human or agent), that contact is dropped from the enrol count while the selection bar still shows the raw eligible count, and the commit detail notes "(1 opt-out excluded)". | Screen e2e lane (suppression semantics owned by [[sequences-and-deliverability]] / [[gdpr-platform]]) |

**Owned screen acceptance — SSO, MFA & sessions (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#securityhtml--sso-mfa--sessions-implements-s-e118 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-security-1 | Given the screen loads for workspace "Brandt Automotive", When the user views the header, Then a banner states SSO and MFA are "Included in the flat tier — no Enterprise upsell" (€0 extra, A36), and the procurement-posture rail shows MFA "required", SSO "configured, not yet enforced", auth events "audited & append-only", and "no edition upgrade". | Screen e2e lane |
| AC-security-2 | Given "Require MFA for all members" is on (state pill reads "Required"), When the user toggles it off, Then the pill changes to "Optional", the posture rail row flips to a warning reading "MFA optional — weakens procurement posture", and a toast confirms the change. | Screen e2e lane |
| AC-security-3 | Given the SSO card, When the user selects an identity provider (Entra / Okta / Google) or switches protocol between SAML 2.0 and OIDC, Then the selected IdP is highlighted and the field set swaps (SAML shows Entity ID / ACS URL / signing cert; OIDC shows Issuer URL / Client ID), and an assertion-to-role mapping table is shown with the note that the assertion "never grants more than the role allows (S-E11.1)". | Screen e2e lane (AAD-PARAM-7) |
| AC-security-4 | Given the user's role is Manager (not Admin), When they toggle "SSO-enforced mode", Then the change is blocked, an inline note appears stating the manage_sso permission is required with a "403 forbidden" code, the toggle stays read-only, and a toast confirms enforcement was not changed. | Screen e2e lane |
| AC-security-5 | Given an active device session ("Pixel 8 / Anna Weber" or "iPad"), When the user clicks Revoke, Then the row is struck through, its Revoke button is removed, its meta updates to "Revoked just now · next request → 401", and a new "Session revoked" entry is prepended to the auth log attributed to "human:mor". | Screen e2e lane (session model [[auth-and-sessions]]) |
| AC-security-6 | Given the "Revoke all others" control, When the user clicks it, Then every active session except "this device" is revoked (rows struck through, audit entries logged) and a toast confirms it. | Screen e2e lane |
| AC-security-7 | Given the AI-flagged anomalous sign-in card, When the user chooses "Revoke session", Then Anna's device is revoked, the card header changes to "revoked by you · audited", and the action buttons disappear; choosing "Dismiss" removes the card noting the session is kept and the decision audited; choosing "Mark trusted" keeps the device on the list and is audited. | Screen e2e lane (anomaly detection is a threat-model D6 planned surface) |
| AC-security-8 | Given the recovery-codes block shows "2 of 10 used" with used codes struck through, When the user clicks "Regenerate codes", Then a toast warns that regenerating invalidates the current set and requires step-up re-auth. | Screen e2e lane (AAD-PARAM-6) |

**Owned screen acceptance — license, seats & free tier (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#licensehtml--license-seats--free-tier-implements-s-e1110 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-license-1 | (entitlement read, not typed): Given the page loads as admin, When rendered, Then the header shows a masked license key (`MARG-•••• •••• •••• 7F2A`) tagged "validated against release service · ADR-0029", and a state pill that reads "Valid", "At free cap", or "Over free cap → paid" driven by the active-seat count — never a free-typed plan. | Screen e2e lane (AAD-PARAM-4) |
| AC-license-2 | (seat meter vs free cap): Given the seat meter, When active seats = 9 of 10, Then the big read-out shows "9 / 10", the bar fills ~90% in the "ok" (green) colour with a threshold marker at the cap, and a legend states Active = "signed in within 30 days, not invited/disabled". | Screen e2e lane |
| AC-license-3 | (approaching / at-cap / over warning, state-driven): Given the seat count, When seats < 10 / = 10 / > 10, Then the warning banner switches between "approaching your seat entitlement" (amber), "at the free-tier cap" (amber), and "you've crossed the free tier" (red) respectively, each naming the flat €25/mo price up front and stating it is shown before anything changes and never enforced silently. | Screen e2e lane |
| AC-license-4 | (walkthrough toggles): Given the demo controls (9 · free / 10 · at cap / 11 · over → paid), When the user clicks one, Then the meter, state pill, banner, plan card highlight, right-rail facts, and explain-box all re-render to that scenario and a toast confirms the seat count and tier. | Screen e2e lane (prototype walkthrough control) |
| AC-license-5 | (add seat past cap is confirmed, never silent): Given the workspace is at 10 active seats, When the admin clicks "Add a seat", Then a confirm dialog states the 11th seat moves the workspace to the flat €25/mo tier (full product, no per-seat surcharge); cancelling toasts "No change — still on the free tier" and adds nothing; confirming appends the seat, advances to the paid tier, and writes an audit entry. | Screen e2e lane |
| AC-license-6 | (remove seat / plan reversibility): Given a seat row, When the admin clicks "Remove", Then the row is deleted, the active count decrements, the meter/tier re-render (dropping ≤ 10 returns the workspace to free), a toast confirms, and the change is logged. | Screen e2e lane |
| AC-license-7 | (explain this charge): Given the Plan card, When the user clicks "Explain this charge", Then a monospace box reveals the computation `plan = (active_seats > free_cap) ? flat_paid : free` with the current active_seats, free_cap (A36), and resulting charge (€0,00 within free tier, or flat €25,00/mo for the whole workspace, not × seats), noting EUR · ISO-4217 · integer minor-units read from the key. | Screen e2e lane ⚠️ the prototype's "flat €25/mo for the whole workspace, not × seats" phrasing predates/conflicts with ADR-0047's €25 **per full seat** — build to ADR-0047 (AAD-PARAM-3); flag to the prototype iteration |
| AC-license-8 | (reveal key + procurement export): Given the masked key, When the admin clicks the reveal control, Then the full key string is shown and the icon flips to eye-off; and the right-rail "Export for procurement →" action toasts that a license-posture PDF is queued. | Screen e2e lane |
| AC-license-9 | (attributable change log): Given the License change log rail, When any key/seat/plan change occurs, Then a new top entry is inserted timestamped "just now" attributed to the acting admin (`mor@bär-pharma.de`), and the panel notes every key/seat/plan change is attributable (P7, ADR-0029). | Screen e2e lane |

**Owned screen acceptance — operator console (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#operatorhtml--operator-console-implements-s-e1111 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-operator-1 | Given the operator role is active, When the console loads, Then the host header shows the in-entitlement key badge, the FQDN (crm.brandt-automotive.de · Frankfurt), the running version (v2026.5.2), and a three-card status strip for Update (v2026.6.0 available · 2 security patches), Last backup (23 min ago · RPO 1h met), and Last DR drill (6 days ago · RTO 4h · 02:41 actual). | Screen e2e lane |
| AC-operator-2 | Given the upgrade gate list shows Test suite (passed), Contract-drift (passed), and Design-system-drift (blocked), When at least one gate is failed/running/waiting, Then the "Apply v2026.6.0" button is disabled with the title "A failed gate blocks the upgrade" and the red block banner "Upgrade blocked — a failed gate never deploys silently" is shown. | Screen e2e lane |
| AC-operator-3 | Given the design-system-drift gate is still red, When I click "Re-run gates", Then all three gates animate to a running spinner and the design gate returns to "blocked", the toast reads "Design-system-drift still red — upgrade stays blocked", and Apply stays disabled. | Screen e2e lane |
| AC-operator-4 | Given the fork divergence is unresolved, When I click "Mark fork rebased" and then "Re-run gates", Then the rebase button locks to "Fork rebased", all gates resolve to "passed", the block banner disappears, Apply becomes enabled, and the toast reads "All gates green — upgrade unblocked". | Screen e2e lane (fork-upgrade safety, ADR-0017 machinery) |
| AC-operator-5 | Given all gates pass and Apply is enabled, When I click "Apply v2026.6.0", Then the button shows "Applying…" then "Applied v2026.6.0", a new "Upgrade → v2026.6.0 · just now · Mor" entry is prepended to the evidence ledger, and the toast confirms the action was recorded in the audit log (S-E11.3). | Screen e2e lane |
| AC-operator-6 | Given the Backup & restore section with a restore-point datetime input bounded to the 35-day PITR window (2026-05-20 → 2026-06-24 09:12), When I click "Run restore drill", Then a progress bar and step log play through sandbox provisioning, WAL replay to the chosen timestamp, erasure re-suppression (3 contacts withheld), integrity verification, and "Recovery complete", and a "DR restore drill · sandbox" entry is appended to the ledger. | Screen e2e lane |
| AC-operator-7 | Given the restore drill, When the erasure step runs, Then the screen states erased PII stays suppressed — 3 contacts with honored erasure requests will not reappear in the restored sandbox even at a pre-erasure timestamp (B-EP07.21). | Screen e2e lane (erasure-suppression semantics [[gdpr-platform]]) |
| AC-operator-8 | Given the rep role, When I toggle the role bar to "Rep", Then the operator surface is replaced by a no-permission state explaining the console requires the `operate` permission, naming the signed-in rep (Anna Weber), with the line "denied by policy, not hidden" (RBAC · S-E11.1). | Screen e2e lane |

**Owned screen acceptance — share a record (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#sharehtml--share-a-record-implements-s-e1112 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-share-1 | Given the share screen for the BÄR Pharma deal, When it loads, Then the header shows the record being shared ("Deal · BÄR Pharma GmbH — Packaging QA · €212k"), a scope chip "Owner + team only", and a ceiling note stating the granter has write here and may grant read or write, "no wider". | Screen e2e lane |
| AC-share-2 | Given the "Person or team" field, When the user types in the subject input or focuses it, Then a menu lists only workspace identities (colleagues and teams), filters live by name/role as typed, and shows "No colleague or team matches — sharing is to workspace identities only." when nothing matches; an identity that already has access (Mor Adler) is rendered disabled with an "already has read" note and is not selectable. | Screen e2e lane |
| AC-share-3 | Given a subject has been picked, When the user toggles the Access-level segmented control between Read and Write, Then the helper text updates accordingly ("Can open and read this record — cannot edit or send." vs. "Can open, edit, and add to this record — not change ownership or sharing."). | Screen e2e lane |
| AC-share-4 | Given the Expiry select, When the user chooses an option (No expiry / 24 hours / 7 days / 30 days, default 7 days), Then the adjacent note updates to the matching plain-language consequence (e.g. "Auto-revokes in 7 days."). | Screen e2e lane (AAD-PARAM-9) |
| AC-share-5 | Given a subject is picked and a reason is entered, When the user clicks "Grant access", Then a new row is appended to the "Who has access" list showing the subject, the granted access pill (read/write), "granted by you · just now", any expiry badge, and the reason; the access count increments; the compose form resets; and a toast confirms the grant was recorded in the audit log with actor, subject, record, access, reason. Clicking Grant with no subject picked instead toasts "Pick a person or team to share with" and focuses the input. | Screen e2e lane (AAD-WIRE-6) |
| AC-share-6 | Given an existing grant row (e.g. Mor Adler), When the user clicks "Revoke", Then the Revoke button is replaced with "revoked — denied at next request", the count decrements, and for the proven row the right-rail "Last access check" panel flips from "row-grant: found (read) / RLS verdict: 200 — visible" to "row-grant: none (revoked) / RLS verdict: 403 — denied". | Screen e2e lane (the dual-layer proof, AAD-AC-5) |
| AC-share-7 | Given the agent-proposed grant card (🟡, read access for Priya Nair, 14-day expiry), When the user clicks "Approve grant", Then a live grant row for Priya is added to "Who has access" attributed "agent-proposed · approved by you", the count increments, the proposal card is removed, and a toast notes the grant is attributed agent→you in the audit log; clicking "Dismiss" removes the card with no grant created and logs it as declined; clicking "Edit first" toasts that the grant opens prefilled before approval. | Screen e2e lane ([[notifications-and-approval-inbox]] disposition semantics) |
| AC-share-8 | Given the right rail, When the user clicks "Show grant history", Then an audit timeline expands showing each grant/revoke with absolute timestamp, actor (including the agent→approver chain for an agent-proposed entry), subject, access, and reason. | Screen e2e lane |

**Owned screen acceptance — field-level security (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#field-securityhtml--field-level-security-implements-s-e159a @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-field-security-1 | Given the field-security screen, When it loads, Then it shows a masking matrix for the Deal object listing each field with its name, dotted path (e.g. `deal.acv_amount`), type, and a sensitivity badge (sensitive/restricted/open), defaulting to role Sales Rep and the live seed policy (SDR has `acv_amount` and `margin_pct` masked). | Screen e2e lane |
| AC-field-security-2 | Given the Sales Rep role tab is active, When the user clicks the SDR / Finance / Admin role tab, Then the matrix header and live-preview pane re-label to that role and the toggles/status redraw to reflect that role's masked set. | Screen e2e lane |
| AC-field-security-3 | Given an unmasked, non-required, non-admin field row, When the user clicks its mask toggle, Then the toggle turns on, the row status reads "staged", and an "Unsaved masking changes" banner appears with a count of staged fields and the affected role, stating nothing is enforced until published and the live policy is unchanged. | Screen e2e lane |
| AC-field-security-4 | Given staged masking changes, When the user clicks "Publish policy", Then a toast confirms the policy is enforced now across UI, API and agents, the row statuses change from "staged" to "enforced", and the staged banner disappears; When the user instead clicks "Revert", Then the staged copy resets to the live policy and the banner disappears. | Screen e2e lane |
| AC-field-security-5 | Given a required field (e.g. `name`) or the Admin role, When viewing its matrix row, Then its toggle is disabled (locked) and the status reads "required" (required fields) or "sees all" (admin), so a required field cannot be masked and admin masks nothing. | Screen e2e lane |
| AC-field-security-6 | Given the live-preview pane, When the user switches between the UI / API / Agent surface tabs, Then the masked field renders consistently as "hidden for your role" in the UI card, as an omitted key with the path noted in a `_meta.masked_fields` array in the JSON, and as "withheld" with the agent stating it cannot report those fields — with the masked-field set identical across all three. | Screen e2e lane ⚠️ **Prototype drift:** the mockup's JSON preview (omitted key + `_meta.masked_fields`) contradicts the normative contract rule — a masked field is **`null` + a sibling `_masked` marker, never omitted** (AAD-DDL-N-1, AAD-AC-2). Build to the contract rule; the next prototype iteration must correct the preview. The cross-surface-consistency requirement stands. |
| AC-field-security-7 | Given the Deal object is selected, When the user clicks the "Company" segment, Then the matrix and preview switch to the Company field set (name, industry, annual revenue, credit score, employees) with that object's masking state. | Screen e2e lane |
| AC-field-security-8 | Given any published change, When the user views the Policy audit panel, Then it shows a timeline of prior published masks (with editor email and timestamp) and a live "staged · just now · you" entry describing what will be masked/unmasked on publish, noting every publish writes to `audit_log` and is reversible/exportable. | Screen e2e lane |

**Owned screen acceptance — sandbox / staging workspace (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#sandboxhtml--sandbox--staging-workspace-implements-s-e159b @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-sandbox-1 | Given the admin is in a sandbox workspace, When the screen loads, Then a persistent banner reads "You are in a sandbox — writes never reach production," naming the isolated clone `sbx-brandt-3f9a`, its source workspace `Brandt Automotive`, and that isolation is enforced by workspace-boundary RLS. | Screen e2e lane |
| AC-sandbox-2 | Given the "How isolation works" control is clicked, When the explainer expands, Then it states isolation is a row-level-security workspace boundary (every row carries `workspace_id = sbx-brandt-3f9a`), that a boundary test asserts 0 sandbox writes are visible in or affect production, and that reset re-clones from a read-only production. | Screen e2e lane ([[data-model#DM-CONV-5]]..8) |
| AC-sandbox-3 | Given the production-vs-sandbox boundary diagram, When the admin reads it, Then production shows 1,284 deals against the sandbox's 50-deal opt-in subset, separated by a one-way seal whose tooltip states sandbox writes cannot cross into production. | Screen e2e lane |
| AC-sandbox-4 | Given a queued rehearsal (CSV import, custom field, or automation), When the admin clicks Rehearse (or "Run all"), Then the row shows a Running state then Passed, and reveals a dry-run result (e.g. import: "would create 301 contacts, 54 companies, 0 deals") ending with an explicit "no write reached production · boundary asserted 0 cross-writes" line. | Screen e2e lane (rehearsed paths: [[import-export-migration]], [[custom-fields]], [[automation]]) |
| AC-sandbox-5 | Given fewer than 3 rehearsals have passed, When the admin views the promote gate, Then "Promote passing changes…" is disabled and the hint reads "Run the rehearsals first — N/3 passed"; When all 3 pass, Then the button enables and the hint reads "All 3/3 passed — safe to stage against production." | Screen e2e lane |
| AC-sandbox-6 | Given all rehearsals pass, When the admin clicks Promote, Then promotion stages the same change against production behind an approval inbox entry (it does not auto-apply), confirmed by a toast stating it is queued and fully audited. | Screen e2e lane ([[notifications-and-approval-inbox]]) |
| AC-sandbox-7 | Given the admin clicks "Reset sandbox," When the re-clone completes, Then all rehearsals return to idle, the pass count resets to 0/3, the subset counts are restored, the status reads "clean clone · re-cloned just now," and a toast confirms "production untouched." | Screen e2e lane |
| AC-sandbox-8 | Given a rehearsal passes, When it completes, Then a sandbox-activity timeline entry is appended labelled `sandbox-only`, distinguishing it from changes that reach production only after promotion. | Screen e2e lane |

**Owned screen acceptance — SCIM provisioning (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#scimhtml--scim-provisioning-implements-s-e159c @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-scim-1 | Given the connection header for "Brandt Automotive — Azure Entra ID", When the screen loads on the Live sync tab, Then it shows state "Connected" (green dot), 48 users provisioned, last push "42 s ago", "5 mapped" groups→roles, and "1 leaver" pending. | Screen e2e lane |
| AC-scim-2 | Given the deprovision SLA tile, When the user clicks "Explain this number", Then an explainbox expands showing the measured breakdown (t0 = scim.user.deleted → bus, t0+1 cycle = session.revoke + passport.revoke ×N, measured Δ = 0.8 s, p95 target ≤ 1 bus cycle, source = audit_log spans). | Screen e2e lane (AAD-PARAM-8; events AAD-EVT-2 gap) |
| AC-scim-3 | Given the leaver card for Thomas Kessler flagged "🟡 awaiting approval", When the user reads the cascade, Then it lists exactly the three steps to be revoked — User account (disable login + revoke sessions), Dependent agent Passports ×2 (psp_3a8f, psp_7c12), and Owned records (4 deals · 11 contacts) — each badged "pending". | Screen e2e lane |
| AC-scim-4 | Given the staged cascade, When the user clicks "Approve & run deprovision", Then the staged flag becomes "approved by you", the buttons disable, each step badge flips to "revoked" (or "queued" for records) in sequence, pending count drops to 0, a completion card appears, and a toast confirms "2 Passports revoked same-cycle (0.8 s)". | Screen e2e lane (AAD-AC-23) |
| AC-scim-5 | Given the inferred-reassignee suggestion (Anna Weber, medium confidence from co-ownership), When the user clicks the accept (check) or dismiss (x) control, Then a toast confirms either "Reassignee kept: Anna Weber" or that an owner picker opens — the system does not auto-assign without confirmation. | Screen e2e lane |
| AC-scim-6 | Given the tab bar, When the user selects "Attribute mapping", Then the group→role table is shown (grp_Sales_DACH→Sales rep ×31, grp_Sales_Lead→Sales manager ×6, grp_RevOps→Admin ×3, grp_Finance→Read-only margin-unmasked ×5, core identity fields) and the live-sync cards are hidden. | Screen e2e lane |
| AC-scim-7 | Given the tab bar, When the user selects "Event log", Then an append-only list from audit_log renders with actor and timing per event (e.g. scim.user.deleted 08:31:04 actor=directory:azure-entra; scim.user.created with cycle/latency; passport.revoked same-cycle 0.9 s). | Screen e2e lane (events AAD-EVT-2 gap) |
| AC-scim-8 | Given the right-rail Endpoint card, When the user clicks the copy icon or "Rotate token", Then a toast confirms the SCIM base URL copied or token rotation; the rail also surfaces endpoint facts (SCIM 2.0, bearer •••• 4f2a rotated 12d ago, TLS 1.3, EU de-fra residency) and "Orphaned: 0" Passports. | Screen e2e lane (token storage per AAD-GAP-3 / ADR-0048 posture) |

**Owned screen acceptance — German / English localization (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#localizationhtml--german--english-localization-implements-s-e1510a @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-localization-1 | Given the screen boots, When it loads, Then the language toggle defaults to Deutsch (active) and every `data-i18n` element, placeholder, and rich-markup region renders in German (DACH-first). | Screen e2e lane |
| AC-localization-2 | Given German is active, When the user clicks "English", Then the toggle's `aria-selected` flips, all chrome/content/AI-label strings re-render in English, and a toast confirms "Language: English — UI re-rendered". | Screen e2e lane |
| AC-localization-3 | Given a language is selected, When the surface re-renders, Then the live preview pane reformats the pipeline/deal amounts (EUR, integer minor-units), the updated date-time, and the relative "2 days ago" to the locale (e.g. "1.284.000,00 €" / "24.06.2026" in de vs. "€1,284,000.00" / "24/06/2026" in en), and the locale chip shows de-DE or en-GB. | Screen e2e lane ([[data-model#DM-FX-7]] — presentation-only) |
| AC-localization-4 | Given the locale formatting table, When it renders, Then each row shows the de-DE value, the en-GB value, and the CLDR/ICU pattern (Date, Date & time, Number, Currency, Percent, Relative), with a note that currency stays ISO-4217 and the amount is never re-typed. | Screen e2e lane |
| AC-localization-5 | Given the string-coverage panel, When the user toggles between the "de" and "en" target bundles, Then the coverage bar and legend update (de: 1.262 human-reviewed / 48 AI-drafted / 7 missing; en: 1.317 / 0 / 0). | Screen e2e lane |
| AC-localization-6 | Given an AI-drafted translation proposal with high confidence and glossary evidence, When the user clicks Accept, Then the accept/edit/dismiss control is removed, the icon turns green, and the provenance flips to "human-reviewed" with a confirming toast; When the user clicks Dismiss, Then the proposal is removed and a toast states the key stays on the EN fallback. | Screen e2e lane |
| AC-localization-7 | Given the passing end-to-end locale test gate, When the user clicks "simulate a hard-coded string", Then the gate flips to a red fail state naming the offending string ("Save changes") and file location (deal/offer.tsx:84) with an "externalize to unblock" instruction. | Screen e2e lane + the CI locale gate (AAD-AC-25) |
| AC-localization-8 | Given the right-rail "Toggle no-permission state" demo control, When the user clicks it, Then a red read-only card appears explaining the user can view but not change the workspace language and must request the `workspace.localize` permission. | Screen e2e lane |

**Cited, not owned here** (each is another chapter's pin; a sanctioned restatement
carries the owner's ID):

| ID | Fact | Owner |
|---|---|---|
| TEST-RBAC-1..5 | The five-area authorization matrix (object / row-scope / field-mask / agent-below-human / auth-state) and its integration lane | [[testing]] |
| TM-CTRL-2 / TM-CTRL-3 | agent ≤ human scope intersection; the admission choke-point that mints capabilities | [[threat-model]] |
| NEVER-11 | No separate agent ACL system — RBAC is the single substrate agents map onto | [[scope]] |
| AUD-AC-1..8 / DM-DDL-8 | The audit substrate: one seam, append-only trigger, coverage gate, trace capture, attribution shape | [[audit-observability]] / [[data-model]] |
| AUTH-AC-1..6 / DM-DDL-6/7 | Session + passport lifecycle: fail-closed revocation, bind-time over-scope rejection, bootstrap atomicity | [[auth-and-sessions]] / [[data-model]] |
| DM-DDL-2..5 | `app_user` (seat type), `team`/`team_membership`, `role`/`role_assignment`, `record_grant` DDL | [[data-model]] |
| RD-DDL-3 (+ its RD-DDL-N-2) | The `bulk_operation` job table; behavior routed to this chapter | [[records-depth]] |
| AC-settings-5/6/7 | The Data-&-exit and migrate-in settings rows | [[import-export-migration]] |
| S-E11.9 / SEQDEL-WIRE-3 | The buyer preference center and the un-overridable opt-out that blocks bulk enrol | [[sequences-and-deliverability]] |
| GDPR erasure/suppression pins | Tombstone-no-PII, suppression-list-forever, erasure-suppression on restore | [[gdpr-platform]] |
| Approval-item semantics | Staged 🟡 items, fail-closed expiry, edit-then-approve executes the human's version | [[notifications-and-approval-inbox]] |
| RL-READ-LIGHT..RL-BULK-WRITE / CAP-* / API-429-1 | Rate-limit classes, request caps, the normative 429 contract | [[api-conventions]] |
| Agent session quotas / BYO-EVT-1 | Per-agent budgets; event-schema versioning policy | [[byo-agent-and-mcp]] |
| STATE-SP-5 and the screen-state floor | Async job states and the standard screen-state matrix every screen above inherits | [[acceptance-standards]] |

**Ownership verification notes (traceability check, flagged honestly):**

| ID | Note |
|---|---|
| AAD-OWN-N-1 | Story primacy verified against the traceability register: S-E11.1/.7/.8/.10/.11/.12 and S-E15.9a-c/.10a-b/.11a-b are primary here. **S-E11.3 is shared**: the substrate is [[audit-observability]] (skeleton, shipped), the approval-view half of its screen serves [[notifications-and-approval-inbox]]; this chapter owns the audit-log *viewer* surfaces only. **S-E15.11 is shared**: enforcement is [[api-conventions]] + [[byo-agent-and-mcp]]; this chapter owns the published docs page + versioning policy. |
| AAD-OWN-N-2 | The seat-enforcement and seat-ceiling code hooks of the two business ADRs (ADR-0029, ADR-0047) are pinned here (AAD-PARAM-3/4, AAD-AC-4) per the ADR index annotation; the ADRs themselves are indexed-only in the vendored set. |
| AAD-OWN-N-3 | Members/roles have no standalone screen in the corpus screen-acceptance catalog — they are the Members-&-roles rows of the workspace-settings series (AC-settings-1..4). The shipped poc Members screen and its wire ops (AAD-WIRE-1..4 @ a11d6c08) are the baseline the settings build extends. Likewise there is no standalone audit-log screen: the viewer is AC-settings-14..16 plus the per-record history (AAD-WIRE-8). |
