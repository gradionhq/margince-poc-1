# ADR-0035 — User-centric automation: a bounded catalog + agent-authored standing automations

**Status:** Accepted (2026-06-23, founder). Recorded as **DECISIONS A45**.

## Context

ADR-0002 (source customization over runtime configuration) and P1 (opinionated over configurable)
rule out a runtime, no-code **workflow builder** — the open, Turing-ish automation engine HubSpot /
Salesforce / Dynamics (via Power Automate) ship. That decision was made for **system-centric,
structural** customization: new objects, new pipeline shapes, bespoke scoring logic — all of which
correctly belong in source.

But it left a real gap, surfaced in the 2026-06-23 parity benchmark review: **everyday operational
automation is user-centric, not system-centric.** "Remind me three days after I send a proposal,"
"create a task for the owner when a deal enters Negotiation," "notify the team channel when a deal is
won" — these are routine, per-user/per-team needs. Expecting a sales rep (Sam) to edit source — or a
partner to bill an engagement — for a personal reminder is wrong. The earlier framing ("workflows are
source") conflated two different things and would have shipped a CRM that *feels* rigid for the most
common, lowest-risk requests every competitor answers trivially.

The constraint, though, still holds: we will **not** build a runtime configurability *engine* (P1, the
`runtime-config-surface.md` four-part test). The answer has to give users no-code automation **without**
becoming the metadata/DSL builder we reject.

## Decision

User-centric operational automation is delivered by **two governed paths, neither of which is a visual
workflow builder**. Bespoke automation *logic* beyond their reach stays source-level (ADR-0002).

**Path 1 — A bounded, opinionated automation catalog (ships as product; no AI required).**
An enumerated set of **pre-built trigger → action templates** the user switches on and parameterizes —
the same kind of bounded, typed control already shipped for routing rules (`runtime-config-surface.md`
RC-4) and SLA timers (RC-5), extended to the everyday cases:
- **Triggers** (closed set): record created/updated, field reaches value, deal enters/leaves stage,
  no-activity-for-N-days, date-field approaching (e.g. close date, renewal date), inbound reply
  received, task overdue.
- **Actions** (closed set): create task, send notification (in-app / email / Dispact channel), assign/
  reassign owner, add to a list, set a field, **draft** (never auto-send) an email, request approval.
- It is **not** a builder: no arbitrary code, no DSL, no user-defined trigger/action types, no
  branching graphs. Each automation is `trigger × action(s) × filter × parameters` chosen from the
  catalog. Anything outside the catalog is either Path 2 or source (ADR-0002) — never an "add custom
  step" escape hatch.

**Path 2 — Agent-authored standing automations (the differentiator).**
A user describes the automation in plain language to their agent ("when a proposal's been out 5 days
with no reply, draft a nudge and remind me"). The agent persists it as a **standing automation** —
a first-class, named, **governed and auditable** object the user can list, edit, pause, and delete.
This is how bespoke-but-personal automation happens without a builder *and* without source edits:
natural language is the authoring surface.

**Both paths are governed identically (ADR-0026 / A34).** Every automation's actions carry the tool's
autonomy tier: 🟢 reversible/internal actions (create task, notify, draft, set a field) run
automatically; 🟡 outward/irreversible actions (send, reassign at scale, close, archive) are held in
the approval inbox (`features/05 §1`). An automation can never grant an action a tier above the
authoring human's permissions (the Passport intersection, `features/04 §1`). Every firing writes to
`audit_log` with the automation id, trigger evidence, and actor — so "why did this happen?" is always
answerable. Standing automations obey the same tighten-only re-tiering floor as agents.

**The bright line (records in `runtime-config-surface.md`):**
- *User-centric operational automation* (the common case) → Path 1 catalog + Path 2 agent-authored,
  bounded and governed. **No-code, no source edit.**
- *System-centric / bespoke automation logic* (new trigger/action types, structural behavior, a
  custom routing/scoring algorithm) → **source-level (ADR-0002)**, Dedicated/Source-delivered modes.

## Consequences

- **Positive:** normal users get the everyday automation every competitor has, with **zero code and no
  builder engine** — preserving P1 and ADR-0002. The catalog serves baseline-tier (no-agent) users
  deterministically; the agent path serves agent users with unbounded *expression* but bounded
  *execution* (the catalog of governed actions). Governance (🟢/🟡 + approval + audit) is reused, not
  rebuilt — automations are just another principal acting through the one governed surface (ADR-0013).
  This is a genuine differentiator: "describe it, don't draw it," safely.
- **Negative / honest limits:** (a) the catalog is deliberately finite — a trigger/action the catalog
  lacks is a product-backlog request or a source change, not a user-extensible slot (we accept this to
  avoid the DSL); (b) Path 2 needs the standing-automation store + a scheduler/event-matcher
  (River-backed) and an editor UI — real V1 build cost; (c) an agent-authored automation that
  misfires is a governance/audit concern — mitigated because every action is tiered, 🟡 outward actions
  are gated, and every automation is pausable and audited.
- **Relationship to other decisions:** does **not** weaken ADR-0002 (no runtime engine for unbounded
  logic) or P1 (the catalog passes the `runtime-config-surface.md` four-part test as a new RC row);
  composes with ADR-0026 (per-tool tiers), ADR-0009 (Surface B reasoning loop is the natural executor
  for agent-authored automations), and ADR-0013 (one governed surface).
- **Scope:** specified in `features/10-operational-depth.md §1`; user stories S-E15.1 (catalog) and
  S-E15.2 (agent-authored). A new runtime-config row (RC-11) records the catalog's bounded surface.
