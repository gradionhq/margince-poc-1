---
derives-from:
  - specs/spec/contract/runtime-config-surface.md#what-runtime-configuration-means-here
  - specs/spec/contract/runtime-config-surface.md#the-p1-test-every-entry-must-pass-config-must-be-earned
  - specs/spec/contract/runtime-config-surface.md#1-shipped-runtime-configuration-surfaces-normative-exhaustive
  - specs/spec/contract/runtime-config-surface.md#2-the-bright-line-restated
---
# Runtime config — every knob is earned, and this is all of them

> The product refuses runtime configurability engines (P1); the few settings a
> user or admin can change at run time are enumerated here, exhaustively, each one
> individually justified. A config surface that is not a row in this register does
> not exist — introducing one anywhere else is a spec defect.

## What it's for

P1 says config surface is a liability: it must be earned, never the default. But
bounded-config concessions naturally accumulate across feature chapters, and a
scattered inventory is how an opinionated product quietly grows a settings
sprawl. This chapter is the single register that keeps the line bright: it owns
the definition of what counts as runtime configuration, the test every surface
must pass to exist, and the complete list of surfaces the product ships. Feature
chapters own the mechanics of their own knobs; this register owns their presence
and their boundary. Reviewers and the ticket generator hold every feature spec
against it (RC-REG-1).

## Principles it serves

- **P1 — opinionated defaults, config is earned.** The register is P1 made
  enforceable: a finite list, a four-part admission test, and a bright line
  against metadata engines, no-code builders, and rule DSLs.
- **P2 — customization lives in source.** Everything unbounded — new objects, new
  relationships, new logic — is agent-authored code in single-tenant modes, never
  a settings screen.
- **P11 — reporting stays honest.** No runtime knob may break joins, reporting
  correctness, or the static schema (RC-TEST-3).

## The bright line

A **runtime configuration surface** is a setting a user or admin can change at
run time — without a source edit, a migration, or a deploy — that alters product
behavior. That exact category is what P1 constrains, and three neighboring
categories are deliberately outside it. **Source-level customization** (P2):
custom objects, relationships, workflow and scoring logic, new pipeline shapes —
authored by an agent as code, available only in the dedicated and source-delivered
modes ([[operations#OPS-MODE-5]]), governed by the customization seams, never by
this register. **Operational and infra configuration**: connection strings,
secrets, model endpoints, the profile selector, deploy topology — operator-facing,
P1-exempt, owned by the operations chapter ([[operations#OPS-CFG-9]]). **Per-user
view state**: saved reports, dashboard layouts, column and filter choices,
personal preferences — data a user owns about their own view, with no shared
blast radius, P1-exempt by nature. RBAC roles, team and territory membership, and
user management are likewise standard identity administration, not
product-behavior config; they are acknowledged so the register is not read as
denying them, but they are deliberately not rows (RC-REG-3).

## The earned-config test

Every register row passes the same four-part test, and a surface earns its place
only if all four hold: it is universally needed rather than a one-segment
preference (RC-TEST-1); it is bounded — a small, enumerated, typed set of
choices, never an open engine or a DSL (RC-TEST-2); changing it carries no
structural or relational risk to the schema, joins, or reporting (RC-TEST-3); and
for this specific change, a typed control is genuinely cheaper than the agent-PR
path — a frequent, low-risk, value-only edit (RC-TEST-4). A surface that fails
any one of the four is not runtime config: it is either source customization or
it does not ship.

## The register

The total surface is thirteen rows — twelve live surfaces plus one explicit
non-surface (RC-REG-2). The live rows cover the daily-frequency, value-only
edits of running a sales team: bounded pipeline edits (RC-1), personal-mail
exclusion (RC-2), lead-scoring weights (RC-3), routing rules and SLA windows
(RC-4, RC-5), the suppression list (RC-6), recording consent (RC-7), per-user
capture connection (RC-8), scheduled report delivery (RC-9), the closed
automation catalog (RC-11), bounded custom fields (RC-12), and required-field and
format validation (RC-13). One row is pinned precisely because it is **not** a
surface: dedupe behavior ships as one opinionated rule set with no matching-rules
configuration at all (RC-10) — the canonical example of what P1 refuses to
expose.

Every row names the chapter that owns its mechanics — the register states the
boundary; the owning chapter states the behavior, screens, and acceptance. Two
rows carry open ratifications: the pipeline-edit boundary (RC-1) and the
recording-consent question of whether some jurisdictions need a hard block rather
than a setting (RC-7). Until ratified, their bounded scope is provisional but
their presence in the register is required. Two rows are governance-sensitive by
construction: every automation firing is autonomy-tiered and audit-logged
(RC-11), and adding a custom field is itself an approval-gated, audit-logged
schema change (RC-12).

What the register adds up to is the honest answer to "how configurable is this
product": exactly this much, with the complete list and the justification for
every line — and no metadata-driven object builder, no no-code workflow engine,
no dynamic-schema interpreter, no general rules DSL anywhere in it.

## Out of scope

- **The mechanics of each surface** — screens, wire operations, acceptance — the
  owning chapter named in each row.
- **The operator-facing config layer** (precedence, profiles, secrets) — the
  operations chapter.
- **The source-customization path** everything unbounded falls under — the
  customization seams and their governing decisions (ADR-0002).

## Where it lives

Each surface is implemented inside its owning feature module and exposed through
the same governed contract as every other write; there is no central "settings
engine" module, by design. Read next: the operations chapter for the ops-config
boundary this register excludes, and the owning feature chapters named in the
register rows.

## Appendix

### Parameters — the earned-config test
Source: specs/spec/contract/runtime-config-surface.md#the-p1-test-every-entry-must-pass-config-must-be-earned @ 5a0b29c

| ID | Criterion | A surface earns its place only if |
|---|---|---|
| RC-TEST-1 | Universally needed | Effectively every customer needs *this* knob — not a long-tail preference one segment wants. |
| RC-TEST-2 | Bounded | A small, enumerated, typed set of choices, **not** an open engine: no metadata-driven custom objects, no no-code workflow/automation builder, no dynamic-schema interpreter, no arbitrary-rule DSL. If a knob's value space is unbounded or Turing-ish, it belongs in source (P2), not here. |
| RC-TEST-3 | No structural/relational risk (P11) | Changing it cannot break joins, reporting correctness, or the static schema. Reorder/rename/threshold = safe; "add a new object/association at runtime" = not allowed here, ever. |
| RC-TEST-4 | Cheaper as config than as a PR | For *this specific* change, a typed bounded control is genuinely better than the agent-PR path — a frequent, low-risk, value-only edit. |

A surface that fails any of the four is **not** runtime config — it is either
source-level customization (P2) or it does not ship.

### Parameters — the config register
Source: specs/spec/contract/runtime-config-surface.md#1-shipped-runtime-configuration-surfaces-normative-exhaustive; #2-the-bright-line-restated @ 5a0b29c

Register rules:

| ID | Rule |
|---|---|
| RC-REG-1 | **The register is exhaustive.** Every runtime configuration surface the product ships MUST appear here; a config surface introduced in any feature spec or ticket without a corresponding row is a **spec defect** — reviewers reject it. |
| RC-REG-2 | The total surface is **thirteen rows — twelve live surfaces plus one explicit non-surface (RC-10)**. Modes: **S** = partner-hosted SaaS multi-tenant, **D** = dedicated/on-prem, **X** = source-delivered ([[operations#OPS-MODE-1]]..3). Source-level customization is additionally available in D and X; the register is runtime config only and applies in all modes unless noted. |
| RC-REG-3 | RBAC roles/permissions, team/territory membership, and user management are standard identity administration, not product-behavior config in the P1 sense — acknowledged, but deliberately not register rows. |

The register (corpus IDs preserved verbatim):

| ID | Surface | Bounded scope — what is (and is NOT) configurable | Modes | Mechanics owned by |
|---|---|---|---|---|
| RC-1 | **Pipeline edit (bounded)** | Reorder stages; rename a stage; set per-stage win-probability (0–100 integer); rename the seeded pipeline. **NOT** new structural pipelines/objects; **NOT** stage-change automation (that is RC-11); per-stage required-field gating lives in RC-13. **Boundary pending ratification.** | S·D·X | deals-and-pipeline |
| RC-2 | **Personal-mail exclusion rules** | A bounded rule set: exclude by sender/recipient domain or by mail label, per connected user. **NOT** a general filtering DSL. | S·D·X | capture |
| RC-3 | **Lead-scoring weight tuning** | Numeric weights on the **fixed, opinionated** factor set of the shipped weighted-signal model. **NOT** new factors; **NOT** new scoring logic (a source-level scoring-handler edit, P2). | S·D·X | lead-scoring |
| RC-4 | **Routing / assignment rules (bounded)** | Enumerated rule types only: territory, segment, deal-size band, source, round-robin within a team; capacity caps; out-of-office reassignment. **NOT** arbitrary bespoke routing logic (a typed workflow handler in source). | S·D·X | leads-and-qualification |
| RC-5 | **SLA timers & escalation windows** | Timer durations and escalation targets for the SLA model. | S·D·X | leads-and-qualification |
| RC-6 | **Suppression list** | Membership of the unsubscribe/bounce suppression list — auto-added on bounce/unsubscribe, manual add/remove. A list of contacts, not logic; compliance-critical. | S·D·X | sequences-and-deliverability |
| RC-7 | **Recording-consent config** | Per-workspace (and where needed per-jurisdiction) policy for whether call audio is recorded/stored — a bounded policy (record/don't; one-party/two-party posture). **NOT** arbitrary logic. **Open question:** some jurisdictions may need a *hard block*, not config — resolved in the owning chapter. | S·D·X | meetings-and-transcripts |
| RC-8 | **Capture connection & scope (per user)** | Per-user opt-in/connect of mail + calendar capture; team visibility governed by RBAC. Connect/disconnect plus visibility, not logic; inherently per-user and consent-driven. | S·D·X | capture |
| RC-9 | **Scheduled report/dashboard delivery** | Recipients, channel (email / Slack via the Dispact bus), and schedule for saved-report delivery. Read-only over reports. | S·D·X | reporting |
| RC-10 | **Dedupe behavior — explicit NON-surface** | **NONE in v1 — deliberately not configurable.** One opinionated rule set ships (exact-email auto-merge; fuzzy match → confirm-first review queue); **no matching-rules UI**. A client needing different rules edits the dedupe logic in source (D/X only). The canonical example of what P1 refuses to expose. | — | people-and-organizations |
| RC-11 | **Automation catalog (bounded)** | Enable/parameterize automations from a **closed catalog** of trigger × action templates — triggers: record created/updated, field-reaches-value, deal enters/leaves stage, no-activity-N-days, date approaching, inbound reply, task overdue; actions: create task, notify, assign/reassign, add-to-list, set field, draft email, request approval. **NOT** user-defined triggers/actions; **NOT** branching graphs; **NOT** a DSL or "custom step". Every firing is autonomy-tiered (ADR-0026) + audit-logged. Agent-authored standing automations execute the **same** bounded action set — the executed actions are this row, not the authoring. | S·D·X | automation |
| RC-12 | **Bounded custom fields** | An admin adds **simple typed custom fields** (text / number / date / currency / picklist / boolean) to **existing** core objects, backed by a **real governed schema change** (a real indexed column, never a field-metadata row) + contract/type regeneration so joins and reporting stay honest (P11). **NOT** new objects; **NOT** relationships/associations; **NOT** formula/validation logic (source, P2). The add-field operation is itself **approval-gated + audit-logged**. | S·D·X | custom-fields |
| RC-13 | **Required-field & format validation (bounded)** | Per-stage **required-field gating** (e.g. a deal cannot move to Proposal without an amount and a decision-maker) and **simple format validation** (email/phone/VAT/IBAN pattern, min/max, picklist membership) on existing fields. **NOT** cross-record/conditional validation logic; **NOT** a rules DSL (source, P2). Enforcement rejects the write with field-level errors on the standard validation error contract. | S·D·X | deals-and-pipeline |
