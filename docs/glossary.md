---
derives-from:
  - specs/spec/contract/glossary.md
  - specs/spec/product/00-overview.md#tier-taxonomy
---
# Glossary — the canonical vocabulary

The single authority on the words code, schema, contract, generated types, UI copy,
and these docs must use consistently. When the docs and the code disagree on a term,
this table wins; when this table and the data-model chapter disagree on a *table
name*, the data-model chapter wins and this table is a defect to fix.

The governing rule: code and schema use the **canonical code term**; the UI may use a
friendlier **display term** only where a row sanctions it, and the two must never
drift into the same surface. There are exactly two sanctioned splits (pinned below);
everywhere else the code term is also the user-facing term.

## Appendix

### Terms
Source: contract/glossary.md#terms @ 5a0b29c

| ID | Definition (one line) | Do NOT call it / rules | Owning chapter |
|---|---|---|---|
| **person** | A real human contact in the relational core — someone we actually know; full participant in dedupe, relationship-strength, and contact reporting. | Table is `person` (singular), not `people`, not `contact` in code/schema. "Contact" is the UI word (GLOSS-SPLIT-1). Never `user` — a person is a CRM subject, not a seat. | architecture/data-model; subsystems/people-and-organizations |
| **contact** | The UI-facing display name for a person. | Not a separate table or type. A lead is NOT a contact until promoted. | subsystems/people-and-organizations; ADR-0008 |
| **app_user** | A login identity / seat (human or first-party agent) inside a workspace; carries RBAC, owns records; has a seat type (read / full). | Table is `app_user` (`user` is reserved SQL). Never call a person a "user", never call an app_user a "person". Human seats and the agent runner are both app_user rows. | architecture/data-model |
| **read seat** (viewer) | A free, unlimited app_user (seat type read) that may read granted records and use the AI read-only; cannot create/edit/send/advance/approve. Its agent inherits the read-only ceiling. Never billed or counted. | UI word: "viewer". Distinct from the Read-only *role* (a role narrows object scope; seat type is a hard capability ceiling below RBAC). | architecture/data-model; subsystems/access-and-admin; ADR-0047 |
| **full seat** (acting seat) | A billable app_user (seat type full) with complete capability incl. approving 🟡 actions; the unit counted for billing and the license grant's "Seat". | The only seat type that counts toward the free tier and the subscription. | architecture/data-model; ADR-0047; ADR-0029 |
| **organization** | A company/account in the relational core: names, domains, hierarchy, owner. | Table + code term is `organization`, not `company`, not `account`. "Account" is the UI word (GLOSS-SPLIT-2). | architecture/data-model; subsystems/people-and-organizations |
| **account** | UI/display word for an organization. | Not a table; never create an `Account` model. (Unrelated to billing "account".) | subsystems/people-and-organizations |
| **deal** | A revenue opportunity: amount + currency, one pipeline + one stage, owner, stakeholders, won/lost. | Code term is `deal`, not `opportunity` ("opportunity" is acceptable UI copy only). A deal attaches to person/organization, never to a raw lead. | architecture/data-model; subsystems/deals-and-pipeline |
| **lead** | A deliberately-thin, raw, machine/bulk-sourced prospect that has NOT genuinely engaged; segregated by construction from the contact graph. | A lead is NOT a person/contact (distinct tables, shared field model). No FK to organization — company name is free text. Excluded from contact search/dedupe/relationship-strength/reporting until promoted. | subsystems/leads-and-qualification; ADR-0008 |
| **promotion (lead→person)** | The non-lossy conversion of a lead into a person on genuine engagement (inbound reply, meeting booked/held, or human qualify), reusing the dedupe-merge path. | Promotion ≠ import and ≠ an outbound touch we sent — cold-send-no-reply does NOT promote (the load-bearing anti-pollution line). The canonical pointer runs person → lead, never a required reverse FK. | subsystems/leads-and-qualification; ADR-0008 |
| **activity** | A single polymorphic timeline event — email, call, meeting, note, or task — linkable to more than one of person/org/deal. | Table is `activity` (singular). The five kinds are a constrained set, not a reference table. A "task" is an activity of kind task, not a separate table. | architecture/data-model; subsystems/activities-and-timeline |
| **relationship** | The one typed, indexed edge table carrying both person↔org employment and deal↔person stakeholder links. | The anti-HubSpot core: never "directional associations". Person↔org is NOT a single FK column; stakeholders are NOT a comma field; a lead never appears in it. | architecture/data-model; subsystems/people-and-organizations |
| **pipeline** | A named, ordered set of stages that deals move through; exactly one default per workspace. | A structurally new pipeline is a source change (P2), not runtime config; reorder/rename/probability of the seeded one is bounded runtime config. | subsystems/deals-and-pipeline; architecture/runtime-config |
| **stage** | An ordered step within a pipeline with a semantic (open/won/lost) and a 0–100 win probability (won=100, lost=0). | Stage position is unique per pipeline. A deal's stage must belong to the deal's pipeline (composite-key guarded). | subsystems/deals-and-pipeline |
| **workspace** | The tenant root; every domain row carries the workspace id; isolation enforced by row-level security. | Not "tenant", not "org" (collides with the company object), not "account". | architecture/data-model |
| **capture** | Auto-ingestion of email/calendar/calls into normalized activity/person/org rows with provenance — the P5 flagship that makes manual entry the exception. | "Auto-capture" and "capture" are the same thing. A human-typed record is a smell (tracked metric), not the norm. | subsystems/capture |
| **provenance** | The source (what produced a row) + captured-by (who: human / agent / connector / system) on every domain row; the audit/trust spine. | Principal prefixes are fixed (human, agent, connector, system). Both columns required on domain tables. The share captured by agents is the P5 metric. | architecture/data-model |
| **context graph** | The V1 capability (capture→link assembly + cross-pipeline reasoning) built on the relational core + vector index — NOT a dedicated graph datastore. | The graph *capability* ships in V1; a graph *datastore* is deferred and trigger-gated (ADR-0021), not roadmapped. Do not introduce a graph DB. | subsystems/context-graph; ADR-0007; ADR-0021 |
| **Agent Seat Passport** | The binding of a BYO/first-party agent to a CRM identity with explicit scopes; the agent's effective permission is the **intersection** of the granting human's RBAC and the Passport scope — never wider. | Capitalize as "Agent Seat Passport". Over-scope binds are rejected with the scope-exceeds-grantor error. The authorizing passport is recorded on every audit row. | subsystems/byo-agent-and-mcp; subsystems/access-and-admin |
| **autonomy tier (🟢/🟡)** | The risk class of an agent/tool action: 🟢 auto-execute low-risk/reversible (log, draft, read); 🟡 confirm-first for outbound/irreversible/high-value. | The tier is a property of the **tool**, declared in the contract — not of the role. Never silently auto-send/auto-close. Enrich is on the always-🟡 floor and no admin can lower it. | subsystems/byo-agent-and-mcp; ADR-0026 |
| **overlay mode** | Deployment where the AI + UI runs on top of an incumbent (Salesforce/HubSpot/Dynamics) which stays the system of record. | One of the two modes from one codebase; the opposite is SoR mode. Do not assume SoR-only. | subsystems/overlay-augmentation; P13 |
| **SoR mode** | Deployment where Margince **is** the system of record. | "SoR" = system-of-record. Distinct from overlay mode. | subsystems/overlay-augmentation; P13 |
| **V1-Must** | Tier: ships in the V1 line; the product is not a credible replacement without it. | One of five story tiers: V1-Must / V1-WOW / V0.5 / Fast-follow / Backlog. V1-Must ∪ V1-WOW = the V1 line. | product/scope |
| **V1-WOW** | Tier: in the V1 line, the differentiating moments. | Still V1, not deferred. | product/scope |
| **V0.5** | Tier: the regulated-German-Mittelstand launch-readiness package (E17) — compliance evidence, fiscal/legal integration, trust pack, per-fork conformity rebuild. | Counted as its own line, not in the V1 Must+WOW total; not deferrable past the DACH beachhead launch. | product/scope; ADR-0038 |
| **Fast-follow** | Tier: needed for credibility but lands one beat after V1; specified now so the shape is known. | Not in the V1 line. Distinct from Backlog. | product/scope |
| **Backlog** | Tier: deliberately deferred / moat-deepening; not yet sequenced. | The furthest-out tier; do not build in V1. | product/scope |
| **Sam** | Persona — the Sales Rep (primary): carries a number, lives in inbox + calendar. | Persona, not a real user. "Don't ask me to update the CRM." | product/personas |
| **Riya** | Persona — the Revenue Leader: owns the forecast and the team; needs a true, explainable pipeline number. | Persona. | product/personas |
| **Devin** | Persona — the Developer-Founder (SMB buyer + customizer): bends the CRM via their own agent in source. | Persona. The P2 source-customization buyer. | product/personas |
| **Mor** | Persona — the CRM Admin / Sales Ops: pipelines, roles, migration, data quality, agent governance. | Persona. | product/personas |
| **Pat** | Persona — the Prospect / Buyer (external; never logs in): experienced through outreach and the deal room. | Persona. Guards against covert profiling — Pat's signals are consent-gated and company-level. | product/personas |
| **owner** | The app_user accountable for a record; drives row-level own/team scope. | Not "assignee" (that is the task-specific assignee on an activity). | architecture/data-model; subsystems/access-and-admin |
| **archive (soft-delete)** | Setting the archived timestamp; removes from default lists, retains in audit, still fetchable by id. | The default delete. No hard delete in v1. "Disqualify" is the lead-specific archive. | architecture/data-model |
| **money (amount_minor + currency)** | Money is two columns: an integer amount in the smallest unit + a three-letter ISO-4217 currency. | No floats, ever. Never sum native minor units across currencies — sum the base-currency value via the frozen/daily FX rate. | architecture/data-model |
| **MCP tool surface** | The governed Model Context Protocol server exposing CRM reads/writes to any compliant agent under Passport scopes + autonomy tiers. | The single artifact both agent surfaces (user's own host + first-party runner) consume. Not an API or app marketplace (explicit non-goal). | subsystems/byo-agent-and-mcp |
| **field metadata** | A pattern we **reject**: there is no field-metadata table and no dynamic-schema interpreter on the hot path. | Custom fields are real columns added by migration (P2: source is the config layer; bounded runtime exception per ADR-0002 Amendment 2). Never build a metadata-driven custom-object engine. | architecture/data-model; subsystems/custom-fields; P1/P2/P11 |
| **Margince** | The product name (locked 2026-06-16). Vendor: **Gradion** (Gradion Pte. Ltd., Singapore). **Dispact** — Gradion's sibling workspace product: separate product, same stack, shares infrastructure libraries, auth, design system, and event bus; both fully standalone. | Earlier drafts used the working title "Gradion CRM". | product/product |

### Terms — sanctioned UI-vs-code splits
Source: contract/glossary.md#notes-on-the-two-deliberate-ui-vs-code-splits @ 5a0b29c

| ID | Rule |
|---|---|
| GLOSS-SPLIT-1 | `person` (code/schema/contract/generated types) ↔ "Contact" (UI copy only). A lead is not a contact until promoted. |
| GLOSS-SPLIT-2 | `organization` (code/schema/contract) ↔ "Account" or "Company" (UI copy only). Never name a model `Account` or `Company`. |
