---
derives-from:
  - margince-poc/docs/principles.md (frozen)
  - margince specs/spec/principles.md#the-principles
---
# Product principles — the P1–P14 rubric everything cites

> The rubric every consequential decision is tested against. When two principles conflict, the
> **higher-numbered loses** as a default tie-breaker — unless a decision record argues otherwise or
> the conflict is a flagged open tension still being designed through (PRIN-TIEBREAK-1). Cite the
> relevant `Pn` when justifying a change.

The product is **Margince**, an AI-native, open-source CRM.

## The principles

### P1 — Opinionated over configurable
One excellent way to do each thing. We do **not** build runtime-configurability engines
(metadata-driven custom objects, no-code workflow builders, drag-and-drop everything) — the thing
incumbents over-build and clients under-use. Configuration surface is a liability: it must be earned,
never the default.

### P2 — The source is the configuration layer
<!-- reconcile: corpus says the customization work is done "by the customer, a partner, or Gradion" and cites A39/ADR-0002 Amendment 1 for the agent-accelerated delivery practice; the poc phrasing names only customer/partner. -->
Per-client customization (custom fields, objects, workflows, views) happens **in the source code, as
real custom development** — never through a settings screen or a runtime config engine. This is the
core paradigm: the codebase is the configuration layer and a product surface. The work is
**human-led engineering** (by the customer or a partner); AI coding agents accelerate it as
an internal delivery practice — the product is never marketed as editing its own code. Every
customization story must have a clean, safe *code* path.

### P3 — Agent-readable by construction
Because P2 makes the code the config layer, **code quality is a product feature**: exhaustive types,
predictable conventions, comprehensive tests (the test suite is the safety guardrail for source
edits), clear extension seams, and in-repo engineering guidance are requirements, not
hygiene. If the codebase can't be safely and quickly extended — including by an AI coding agent — the
customization paradigm has failed.

### P4 — Blazing fast, always
Performance is a requirement with numbers. Enforced p95 budgets for the operations users feel (list,
search, open record, save). Speed is a primary, defensible differentiator over bloated incumbents. A
change that regresses a budget does not ship.

### P5 — Auto-capture over manual entry
Manual data entry is a failure mode to design out. Capture from email, calendar, calls and signals
automatically; a record a human had to type is a smell. (~70% of CRM data is incomplete because entry
is manual — attack that directly.)

### P6 — Embrace the LLMs; don't fight them
We are not a frontier lab. The foundational AI capability is orchestrating the **user's own agent**
(their Claude/Cursor/Copilot license) inside the product through governed tools. We add only the
built-in **baseline AI** the majority who have no agent seat genuinely need. We never ship an AI
feature whose pitch is "we reimplemented what the big labs do better."

### P7 — Own your data
Trivial export, open formats, documented schema, no proprietary lock-in (the anti-HubSpot /
anti-Salesforce stance). Supports SaaS, on-prem, and source-delivered deployment. Where required
(regulated/data-sensitive clients), the whole thing — including local LLM inference — must run on the
client's own infrastructure.

### P8 — Beautiful by default
Design quality is a moat incumbents abandoned. Distinctive and polished out of the box, not a
configurable-but-ugly toolkit. Visual and interaction quality are acceptance criteria. Verified by a
**design-quality rubric** (heuristic expert review) — *not* by the design-system drift check, which
only proves consistency (a consistently mediocre UI still passes drift).

### P9 — Standalone first, integrations optional
<!-- reconcile: corpus P9 is "Independent products, shared foundations" — it names the sibling workspace product Dispact, the shared gw-* foundations (defaults, design language, auth, infra/data substrate align unless an ADR diverges), and the FD-12 standalone constraint; the poc rewrote it product-neutrally around standalone-first. Poc phrasing kept — it survived a build. -->
Margince installs, deploys, and runs **fully standalone** — it never requires another product to
function. Any integration with an outside system (shared identity, cross-links, an external event
bus) is **optional and additive**: present only when a customer turns it on, never a precondition for
the core product working on its own. The overlay/augmentation mode (P13), where Margince layers onto
an incumbent system of record, is one such optional mode — the standalone system-of-record mode owes
nothing to any incumbent.

### P10 — First principles, no sacred cows
<!-- reconcile: corpus names the prior concepts explicitly ("Dispact's current AI, the old prototype"); the poc generalizes to "prior concepts and earlier prototypes". -->
The best idea wins on merit. Prior concepts and earlier prototypes are references to *learn from and
beat*, never defaults to inherit. Guard actively against "we already decided this."

### P11 — Clean relational core
The data model is normalized and relational, deliberately rejecting HubSpot's directional-association
complexity that breaks reporting and API joins. Standard objects done excellently; new objects are
*code* (P2), not metadata rows.

### P12 — Governance is designed in
Audit trails, decision provenance, human-in-the-loop approval gates and rollback are **core
primitives, not retrofits**. This is both a trust feature and the structural answer to EU AI Act /
GDPR for agentic systems.

### P13 — Augment, don't demand rip-and-replace
<!-- reconcile: corpus adds "with full replacement as a possible later step" and notes the sibling product Dispact follows the same overlay pattern on Teams/Slack; the poc drops both. -->
Enterprises will not tear out a $5M Salesforce/Teams install. So the product runs in **two modes from
one codebase**: **system-of-record** (SMB/startups adopt us fully) and **overlay/augmentation** (our
AI layer + UI on *top* of the incumbent, which stays system of record). Overlay is the enterprise
entry point — "try our AI without migrating."

### P14 — Curated safe seams, not total upgrade-safety
<!-- reconcile: corpus P14 is "The product is the top of a consulting funnel" — the open-source product as a sales vehicle for consulting/dev services, reframing broken upgrade paths as billable engagements; the poc kept only the engineering conclusion (curated seams, no total upgrade-safety) and dropped the business-model rationale. -->
We do **not** over-engineer the codebase to make every possible core change upgrade-safe — that
fights P1 and small-team reality. Instead the product offers curated, safe extension seams for the
common customizations; a change that goes outside them and breaks its own upgrade path is an explicit
custom-development undertaking, not a supported config path.

## Open tension (flagged, not hidden)
<!-- reconcile: corpus flags the same tension but resolves it in 04-customization-paradigm.md and 06-nonfunctional.md rather than stating the deployment-mode split inline; the poc carries the resolving principle in-text. -->
**P1/P2 (opinionated, source-customized) vs P7/P9 (SaaS hosting, multi-tenant):** if customization
lives in source forks, how does a hosted multi-tenant offering work? The resolving principle
(PRIN-TENSION-1) is that the two apply to *different deployment modes* — hosted multi-tenant gets
**bounded configuration only**, while source-fork customization (P2) is reserved for the dedicated /
source-delivered / self-hosted modes, so one tenant's custom code never has to share a codebase with
another's. The authoritative design — the deployment modes, the customization paradigm, and the
portability limits it implies — lives in the **product foundation specs** (the stakeholder
requirements, outside this spec tree), not here. This is the sharpest product tension; it is worked
through there, not hand-waved.

## How they're used
- Every feature/architecture decision cites the principles it serves and any it tensions against.
- A new load-bearing decision is justified against these principles and recorded as a decision/ADR.
- Adversarial review re-audits artifacts for principle contradictions.

## Appendix

### Parameters — the rubric
Source: margince-poc/docs/principles.md @ a11d6c08; principles.md#the-principles @ 5a0b29c

| ID | Name | Kernel |
|---|---|---|
| P1 | Opinionated over configurable | One excellent way per thing; configuration surface is a liability that must be earned, never the default. |
| P2 | The source is the configuration layer | Per-client customization is real custom development in source — never a settings screen or runtime config engine. |
| P3 | Agent-readable by construction | Code quality is a product feature: types, conventions, tests, and seams that let a human or AI agent extend the source safely and fast. |
| P4 | Blazing fast, always | Enforced p95 budgets for the operations users feel; a change that regresses a budget does not ship. |
| P5 | Auto-capture over manual entry | Capture from email, calendar, calls and signals automatically; a hand-typed record is a smell. |
| P6 | Embrace the LLMs; don't fight them | Orchestrate the user's own agent through governed tools; ship only the baseline AI that users without an agent seat genuinely need. |
| P7 | Own your data | Trivial export, open formats, documented schema, no lock-in; runs fully on client infrastructure including local inference where required. |
| P8 | Beautiful by default | Visual and interaction quality are acceptance criteria, verified by a design-quality rubric — not by the drift check, which only proves consistency. |
| P9 | Standalone first, integrations optional | Runs fully standalone; every outside integration is optional and additive, never a precondition. |
| P10 | First principles, no sacred cows | The best idea wins on merit; prior concepts and prototypes are references to beat, never defaults to inherit. |
| P11 | Clean relational core | Normalized relational model, standard objects done excellently; new objects are code (P2), not metadata rows. |
| P12 | Governance is designed in | Audit trails, decision provenance, approval gates, and rollback are core primitives, not retrofits. |
| P13 | Augment, don't demand rip-and-replace | Two modes from one codebase: system-of-record for full adoption, overlay/augmentation on top of an incumbent. |
| P14 | Curated safe seams, not total upgrade-safety | Curated safe extension seams for common customizations; an out-of-seam change that breaks its upgrade path is explicit custom development. |

**PRIN-TIEBREAK-1 — tie-breaker.** When two principles conflict, the lower-numbered principle wins
by default — unless an ADR argues otherwise, or the conflict is a flagged open tension still being
designed through.

**PRIN-TENSION-1 — the P1/P2 vs P7/P9 resolution.** The pairs apply to different deployment modes:
hosted multi-tenant offers bounded configuration only; source-fork customization (P2) is reserved
for the dedicated / source-delivered / self-hosted modes, so one tenant's custom code never shares a
codebase with another's. The authoritative deployment-mode design lives in the product foundation
specs.
