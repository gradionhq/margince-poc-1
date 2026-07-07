---
status: planned
module: backend/internal/modules/agents (governed tool surface, admission, session quotas, webhook fan-out); web (ai + integrations screens)
derives-from:
  - specs/spec/contract/interfaces.md#2-mcp-tool-contract-layer-1 @ 5a0b29c
  - specs/spec/contract/interfaces.md#0-conventions-for-every-interface-here @ 5a0b29c
  - specs/spec/contract/api-rate-limits-and-abuse.md#2-mcp-tool-limits-the-load-bearing-part @ 5a0b29c
  - specs/spec/contract/api-rate-limits-and-abuse.md#3-abuse-prevention @ 5a0b29c
  - specs/spec/features/04-platform-and-compliance.md#3-integrations--mcp-app-connectors-not-a-marketplace @ 5a0b29c
  - specs/spec/narrative/03c-agentic-concept.md#3-agent-execution-topology-the-core-model @ 5a0b29c
  - specs/spec/narrative/03c-agentic-concept.md#4-the-mcp-server-structure-and-why-it-matters @ 5a0b29c
  - specs/spec/contract/events.md#4-redis-streams-layout @ 5a0b29c
  - specs/spec/contract/data-model.md#outbound-webhooks-e10--s-e106-a51--governed-integration-surface-not-a-marketplace @ 5a0b29c
  - specs/spec/contract/data-model.md#agent-connections-e10-byo-agent--features04-3-03c @ 5a0b29c
  - specs/spec/product/epics/E10-byo-agent-and-customization.md @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#aihtml--ask-ai-the-two-surface-agent-hub-implements-s-e10135 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#integrationshtml--webhooks--integrations-implements-s-e106 @ 5a0b29c
  - margince-poc/docs/subsystems/mcp-surface.md @ a11d6c08
---
# BYO agent & MCP — one governed door, and the tables that say what any agent may do

> The bring-your-own-agent surface: any compliant agent — the user's own Claude,
> ChatGPT, or Cursor reaching in, or the resident runner working overnight — operates
> the CRM through one governed tool surface under an Agent Seat Passport. This chapter
> is the single home of the tool tables (verbs, scopes, tiers), the admission rule,
> the session quotas and abuse ladder, and the outbound webhook surface.

## What it's for

The product's bet is that users bring the agent they already pay for instead of renting
a second, metered one from the CRM vendor. That is only safe if the agent's power is a
property of the *surface*, not of the agent's good behavior: which verbs exist, which of
them need a human's confirmation, and how much a session may do before it becomes a
visible, gated event must all be decided server-side, below the model. This subsystem is
that surface. Its callers are every agent path there is — the user's own assistant
connecting from its client or its vendor's cloud, and the first-party resident runner
(the agent-runner chapter) — plus the integrator who subscribes to outbound webhooks
instead of polling. The scope boundary: this chapter owns the governed tool *set* and
its admission economics; the mechanism chapters it composes — the approval inbox
(approvals-and-concurrency), intent-tool composition (intent-tools), the datasource
seam (datasource), the trust envelope (trust-propagation) — own their mechanics and are
cited, never restated. The load-bearing vocabulary — Agent Seat Passport, autonomy
tier, MCP tool surface — is the glossary's ([[glossary]]).

## Principles it serves

- **P6 — embrace the LLMs; don't fight them.** The user's own agent is a first-class
  operator of the product, through governed tools rather than a private side door.
- **P12 — governance is designed in.** Scope, tier, and quota are enforced in the tool
  contract before any side effect; safety never depends on prompt wording or on which
  brain is driving.
- **P2/P7 — own the surface, no marketplace.** Integrations are MCP connectors and a
  first-party outbound event surface the customer owns — deliberately not an
  installable third-party app store.
- **ADR-0026 — two-tier autonomy with an always-confirm floor.** The tier is a
  server-side property of the tool; the floor can never be loosened.
- **ADR-0005 — the connector, not the runner, is the moat.** One MCP server serves
  every brain; BYO autonomy is an API key or a local model, never a chat subscription.
- **ADR-0009 — the agent-first surface.** Verbs over tables: a small CRUD seam plus
  outcome-shaped intent tools, one contract for both.
- **ADR-0036 — approval tokens.** A confirm-first action executes only against a
  single-use, bound token minted by a recorded human decision.

## How it works

**Two surfaces, one door.** An agent reaches the system one of two ways: inbound —
the agent loop runs outside, in the user's client (local transport) or on its vendor's
cloud (hosted transport, operator-run, never Gradion — ADR-0027) — or resident, where
the first-party runner executes the loop inside the system. Both terminate at the same
governed MCP server; identity, admission, tools, and audit are identical regardless of
transport, so there is exactly one place to reason about authority. Bring-your-own is a
statement about the *brain*, not a third surface: for inbound it is the user's own
subscription reaching in; for the resident runner it is an API key or a local model
(vendor terms forbid subscription auth for programmatic use, ADR-0005).

**Identity: the Passport.** A connecting agent is registered as an agent connection
bound to an Agent Seat Passport and to the human it acts on behalf of. The Passport's
scopes are a strict subset of the granting human's effective permissions — an
over-scope bind is rejected at mint time with no row created, and revocation is
enforced fail-closed at the very next lookup; that lifecycle is the auth-and-sessions
chapter's ([[auth-and-sessions#AUTH-WIRE-5]], [[auth-and-sessions#AUTH-WIRE-6]],
[[auth-and-sessions#AUTH-PARAM-6]]). The default grant for a fresh connection is
read-plus-draft, the entry point of the BYO journey (S-E10.1).

**Admission: scope, then quota, then tier.** Every call is admitted only if all three
hold (BYO-FORM-1). Scope answers *which verbs exist for this caller*: the tool's
required scope must be granted by the Passport's scope set (BYO-SCOPE-1..4), which is
itself intersected with the granting human's permissions — agent ≤ human, the
structural control the threat model pins ([[threat-model#TM-CTRL-2]]). Row scope is
orthogonal: it bounds which rows, not which verbs. Quota answers *how much, how fast*:
the session counters below are the only layer that catches legitimate-but-abusive
volume — an in-scope mass read passes scope and tier and is caught by quota alone.
Tier answers *auto or confirm-first*: an auto-approved (🟢) call proceeds; a
confirm-first (🟡) call from an agent without a valid approval token is staged with a
dry-run preview and refused as approval-required, with zero side effects — the staging,
token minting, single-use and expiry rules are the approvals-and-concurrency chapter's
([[approvals-and-concurrency#APPR-WIRE-1]], [[approvals-and-concurrency#APPR-PARAM-3]]).
Only a passing call mints the admitted capability, and the admission gate is the sole
place that can mint one — no capability, no path to data
([[threat-model#TM-CTRL-3]]). Every admitted call appends one audit row.

**The tool set.** The surface is deliberately small: fourteen low-level tools
(BYO-TOOL-1..14) — record-type-generic verbs over the datasource seam, so the same
tools work unchanged in system-of-record mode and overlay mode — and six intent tools
(BYO-INTENT-1..6), fatter outcome-shaped verbs composed from the low-level set. The
composition mechanism — reach is the union of parts, tier is the strictest of parts,
never hand-widened — is the intent-tools chapter's ([[intent-tools#INTENT-AC-1]],
[[intent-tools#INTENT-AC-2]]); the concrete catalogue pins here. Every tool is one
contract operation annotated for the agent surface and generated from it — no second
source of truth ([[contract-pipeline#CP-MCP-1]]); the tier vocabulary is the contract
pipeline's ([[contract-pipeline#CP-MCP-2]]), and the tool tables, tier assignments, and
floor are this chapter's, ceded explicitly by that chapter
([[contract-pipeline#CP-MCP-3]]).

**The tier line is reversibility, not read-versus-write.** Auto-approved means
reversible and logged: reads, drafts, internal create and update, open-stage deal
moves. Confirm-first means it leaves the workspace or cannot be cleanly undone: send,
outbound connector write-back, archive, merge, disqualify, terminal deal moves, enrich.
Three rules harden the line. First, the **always-confirm floor** (BYO-FLOOR-1): the
floor tools can never be re-tiered to auto-execute — not by a customer, not by
configuration — so a hijacked agent always hits an approval gate before anything
irreversible or external ([[threat-model#D4]], ADR-0026). Second,
**re-tiering is tighten-only** (BYO-TIER-1): an admin may raise an auto tool to
confirm-first, never lower the floor. Third, **human-edit precedence** (BYO-PREC-1): an
agent's auto-tier update still cannot overwrite a field whose current value a human
typed; that field splits into a staged confirm-first change while the rest of the
update proceeds, decided per field from the audit trail — no new provenance store.

**One tool resolves its tier per call.** The deal-advance verb is the surface's single
dynamically tiered tool: its tier comes from the transition's stage semantics, resolved
by the admission gate before the handler runs (BYO-FORM-2). A transition is
confirm-first when **either endpoint** is a terminal (won or lost) stage — closing and
reopening are both governed; open-to-open moves are auto-approved. That rule is the
deals-and-pipeline chapter's behavioral rule ([[deals-and-pipeline#DEAL-WIRE-4]]); this
chapter pins the tool row consistently with it and flags the corpus divergence in the
Tools appendix. The resolver reads the target pipeline's stage semantics (and, in
overlay mode, the incumbent stage mapping), never just the request arguments, so a
custom pipeline whose won stage is renamed still resolves confirm-first. The resolver
may only ever raise toward confirm-first; it can never resolve a floor case down.

**Quotas and the abuse ladder.** A session is one Passport token's lifetime; counters
are per-session and also aggregated per Passport over a rolling window so a flood of
short sessions cannot evade them. The session quotas (MCP-SESS-*) and per-tool ceilings
(BYO-LIM-1..11) are the mechanism behind the threat model's egress backstop: crossing
the read threshold forces a human step-up — mass exfiltration becomes a gated, visible
event ([[threat-model#D3]]); write floods batch-confirm; egress
overruns hard-stop the session, fail-closed. Sustained misbehavior climbs a
deterministic ladder — throttle, step-up, session-suspend, connection-suspend, tenant
circuit-breaker (BYO-LADDER-1..5) — and every threshold crossing is an audit event that
feeds anomaly detection on the audit stream (BYO-ANOM-1..5). Under tenant-level
pressure, non-interactive agent traffic is shed first and interactive human work last
(BYO-BRK-1). The per-session cost quota meters agent-invoked model spend against the
workspace AI budget the ai-runtime chapter owns
([[ai-runtime#AIRT-PARAM-8]]); there is no credit meter and no per-AI-seat charge —
that rejection is structural ([[scope#NEVER-4]]).

**Outbound webhooks (S-E10.6).** The integrator surface over the existing event bus: a
subscription names an HTTPS-only target and a set of event types from the published
catalog; matching domain events are delivered as signed HTTP posts the receiver can
verify, retried with backoff, and parked in an inspectable, replayable dead-letter view
after the retry budget — no silent loss (BYO-EVT-1..2). Fan-out is bounded by the
owning principal's visibility at delivery time, not just at registration — a webhook
can never widen who effectively sees a record (BYO-EVT-4) — and confirm-first-gated
event types are delivered only after the approval gate clears; receiving a webhook
never bypasses the gate (BYO-EVT-3). Every registration, change, pause, and delivery is
audit-logged. The catalog is the contract: payloads carry the bus's versioned envelope,
and a breaking change ships a new major version while the old keeps delivering. This is
a first-party outbound surface the customer owns — deliberately not a marketplace.

**Customization is development, not a runtime feature (S-E10.3).** The signature
moment of the epic — bend the CRM by changing its source — is human-led engineering on
code the customer owns, landing as a reviewable pull request against their fork
(ADR-0002 Amendment 1). This chapter owns the story's acceptance; the mechanics live in
the generators chapter: the scaffolding recipes, the per-field contract round-trip
mandate, the upgrade preflight, and the behavior-changelog discipline
([[generators#M1]]–[[generators#M4]], [[generators#GEN-UPG-1]]). The agent hub screen
enforces the boundary from the product side: a schema-change request in chat redirects
to the development path and never scaffolds code in-product (AC-ai-6).

**The screens.** The agent hub (`ai`, AC-ai-1..13) is the two-surface honesty surface:
one zone for the resident runner on the configured brain (read and draft inline,
anything that writes, sends, or advances routed to the approval inbox), one zone for
connecting the user's own agent — a connect-and-literacy panel with the endpoint,
Passport scopes, and copyable outcome-phrased prompts, not a chat. A right-rail palette
lists every governed capability with its tier glyph and scope token. The integrations
screen (AC-integrations-1..8) manages webhook subscriptions: register with HTTPS
validation and once-revealed signing secret, monitor delivery health, replay
dead-letters, inspect the exact signed payload, and read the published catalog with its
gated-event badges and anti-marketplace stance.

## What's configurable

- **Session quotas and per-tool limits** — opinionated, enforced defaults in shared
  deployments; self-tunable on dedicated and source deployments with one exception: the
  read step-up threshold is a security control with a floor, and lowering it below the
  floor requires an ADR and fails configuration validation (MCP-SESS-READS, AC-MODE-1).
  These are operational configuration, outside the product runtime-config inventory
  ([[runtime-config]]).
- **Re-tiering, tighten-only** — a workspace admin may raise an auto tool to
  confirm-first (BYO-TIER-1); loosening below the floor is not an exposed setting,
  for anyone. V1 may ship without the raise control exposed; the rule binds when it
  lands.
- **The connected-agent set** — agent connections and their Passport scopes are
  runtime data, granted and revoked per user; grants can never exceed the grantor
  ([[auth-and-sessions#AUTH-WIRE-5]]).
- **Transport deployment** — the local transport serves an individual at a desk; the
  hosted transport is a public authenticated service an operator runs (a hosting
  partner or the self-hosting customer, never Gradion, ADR-0027). Absent a hosted
  deployment the product degrades gracefully: local and resident paths are unaffected.
- **Webhook subscriptions** — per-workspace runtime data: target, event-type set,
  paused state, secret rotation. The catalog itself is contract, not configuration.

## Guarantees (enforced)

- **Agent never exceeds human.** Effective authority is the Passport scope set
  intersected with the grantor's permissions; an over-scope bind is refused at mint
  time ([[threat-model#TM-CTRL-2]], [[auth-and-sessions#AUTH-AC-4]]).
- **No path around the gate.** The admission gate is the only place that mints the
  capability the mutation seam requires; held by an import-graph invariant, not
  convention ([[threat-model#TM-CTRL-3]]).
- **A refused confirm-first call has zero side effects.** Staging writes only the held
  item and its audit row; nothing commits until a recorded human decision, and the
  token is single-use and bound ([[approvals-and-concurrency#APPR-AC-1]],
  [[approvals-and-concurrency#APPR-AC-7]]).
- **The floor holds.** No configuration, role, or customer action can make a floor
  tool auto-execute (BYO-FLOOR-1); a terminal deal-advance can never resolve below
  confirm-first (BYO-FORM-2).
- **Volume becomes visible.** Crossing the read threshold forces step-up; egress
  overruns hard-stop fail-closed; every crossing is an audit event (AC-MCP-1,
  AC-MCP-2); the per-Passport rolling window closes the short-session evasion
  (AC-MCP-4).
- **Every tool is fully declared.** A tool without a tier, a per-tool limit, and a
  session-quota mapping fails the contract lint — a spec defect, not a runtime
  surprise (AC-MCP-3).
- **Webhooks never escalate.** Fan-out is bounded to the owner's visibility at
  delivery time, deliveries are signed and verifiable, gated events deliver only
  post-approval, and nothing is silently lost — failed deliveries park and replay
  (BYO-AC-9..11).
- **Full parity, no private verbs.** Every shipped core read and write is reachable
  through the governed tool surface under Passport scopes with tiers as declared
  ([[acceptance-standards#GATE-CORE-7]]).

## Acceptance

Done means a developer-founder can point the agent they already pay for at the CRM,
watch it search, read, summarize, and draft under a Passport they granted, and read
every action back in the audit log attributed to that agent under their authority; a
refused action states its reason instead of silently dropping. It means an admin can
inspect what any agent may do — the palette of capabilities, tiers, and scopes — and an
integrator can register a webhook, verify a signed delivery, watch a failure park in
the dead-letter view, and replay it. Honest states are part of done: the agent hub
renders its no-brain-configured and RBAC-denied states distinctly from tier-blocked;
the integrations screen renders failing, paused, and dead-letter-empty states. The
testable form of every claim lives in the Acceptance appendix; the cross-cutting screen
and release floor is inherited from the acceptance-standards chapter. BYO-agent
conformance is verified end-to-end by the eval harness that drives real agents against
seeded workspaces ([[ai-evals#AISA-1]]–[[ai-evals#AISA-7]]).

## Out of scope

- **Promote-to-act (S-E10.2)** — raising a Passport to act-with-approval is
  fast-follow, gated on the injection red-team probe ([[scope]] deferred list;
  [[threat-model#TM-VERIFY-1]]). The staging machinery it needs ships in V1 regardless,
  because the floor tools already require it.
- **No per-AI-seat tax (S-E10.5)** — a product promise held structurally, not a
  feature: credit meters and AI-seat pricing are rejected patterns
  ([[scope#NEVER-4]]); the budget guardrail that degrades instead of hard-stopping is
  the ai-runtime chapter's.
- **Marketing-and-sales-in-a-box modules (S-E10.4)** — backlog ([[scope]]).
- **Approval inbox mechanics** — staging, disposition, tokens, TTL:
  approvals-and-concurrency. **Intent-tool composition mechanism**: intent-tools.
  **Bus delivery semantics**: event-bus. **Connector capture**: capture. **Overlay
  lifecycle and incumbent budget**: overlay-budget. **The resident runner's loop,
  budget, and handoff**: agent-runner. **Lead promotion and conversation-link verbs**
  ride this gate but their rows are owned by leads-and-qualification
  ([[leads-and-qualification#LEADS-WIRE-6]]) and messaging-channels.

## Where it lives

The governed tool surface, admission gate, session limiter, and webhook fan-out live in
the agents module (backend/internal/modules/agents), reached over the two MCP
transports and the REST contract; the screens live in the web shell. Read next:
approvals-and-concurrency for what happens to a held action, intent-tools for the
composition mechanism, datasource for the seam the tools execute against, agent-runner
for the resident loop, threat-model for why the gate is shaped this way, and
contract-pipeline for how the tool list is generated.

## Appendix

### Parameters
Source: specs/spec/contract/interfaces.md#0-conventions-for-every-interface-here @ 5a0b29c

The Passport-scope → tool-scope mapping (the canonical resolution of the three scope
namespaces; row scope `own`/`team`/`all` is orthogonal — it bounds *which rows*, never
*which verbs*). Default grant for a new agent connection: `read_only` + `draft_only`
(S-E10.1 read+draft).

| ID | Passport scope | Tool scopes it satisfies | Tier behavior |
|---|---|---|---|
| BYO-SCOPE-1 | `read_only` | `read` | 🟢 only |
| BYO-SCOPE-2 | `draft_only` | `read`, `draft` | 🟢 only |
| BYO-SCOPE-3 | `act_with_approval` | `read`, `draft`, `write`, `send`, `enrich` | 🟡 actions allowed **with** an approval token |
| BYO-SCOPE-4 | `auto_execute_low_risk` | `read`, `draft`, `write` (🟢 mutations only) | 🟢 auto; 🟡 still needs a token |

### Formulas
Source: specs/spec/contract/interfaces.md#0-conventions-for-every-interface-here @ 5a0b29c; specs/spec/contract/api-rate-limits-and-abuse.md#21-the-composition-model-scope--tier--quota @ 5a0b29c

**BYO-FORM-1 — admission.** Inputs: the tool's declared spec (required scope, tier,
egress flag, per-tool limit, quota mapping), the caller's Passport + grantor RBAC, the
validated call args, the session counters, an optional approval token.

```
admit(call) :=
  scope :  call.tool.required_scope ∈ grants(passport.scopes)          -- BYO-SCOPE-1..4
           AND passport.scopes ⊆ grantor.effective_rbac                -- agent ≤ human (TM-CTRL-2)
           AND call.target_rows ⊆ principal.row_scope                  -- own/team/all, orthogonal
           else refuse scope_exceeded (API-ERR-9)
  quota :  charge session + per-tool counters (MCP-SESS-*, BYO-LIM-*)
           on overflow → step-up / throttle / hard-stop per BYO-STEP-1..4
  tier  :  t := call.tool.tier
           if t = dynamic → t := BYO-FORM-2(call)                      -- may only RAISE to 🟡
           if t = 🟡 AND actor is agent AND no valid bound token
             → stage held item + audit row, refuse approval_required (API-ERR-10); ZERO side effects
  mint  :  append one audit row; return the admitted capability (the only constructor — TM-CTRL-3)
```

Output: an admitted capability carrying narrowed row scope, field mask, and any
consumed token — or a typed refusal. Tie-breaks: checks run in the order scope → quota
→ tier, so an out-of-scope call is never charged quota and never staged; a dynamic tier
is resolved before the token check. Worked example: an agent on `act_with_approval`
(BYO-SCOPE-3) calls the send-email tool (BYO-TOOL-7, `send`, 🟡) with no token — scope
passes, quota charges MCP-SESS-EGRESS, tier stages the send with a dry-run preview and
returns approval-required; the SMTP path is never reached. The same call with the
minted single-use token executes and consumes the token
([[approvals-and-concurrency#APPR-PARAM-3]]).

**BYO-FORM-2 — dynamic-tier resolver (deal-advance only).** Inputs: validated args
plus the *resolved stage semantics* of both endpoints, read from pipeline
configuration (and, in overlay mode, the incumbent→canonical stage mapping) — never
from the raw request alone.

```
resolve(from_stage, to_stage) :=
  🟡  if from_stage.semantic ∈ {won, lost} OR to_stage.semantic ∈ {won, lost}
  🟢  otherwise (open → open, either direction)
```

Invariants: the resolver may only ever raise to 🟡; it may never return dynamic and
never resolves a terminal transition down to 🟢 (the floor, BYO-FLOOR-1). Behavioral
owner: [[deals-and-pipeline#DEAL-WIRE-4]]. Worked example against the seeded pipeline
([[deals-and-pipeline#DEAL-FORM-1]]): Qualified → Proposal is 🟢; Proposal → Closed Won
is 🟡 (target terminal); Closed Won → Proposal (reopen) is 🟡 (source terminal) — the
close timestamp, lost reason, and frozen rate clear under the same gate.

> **Corpus divergence (flagged).** The corpus tool table resolves on the *target*
> semantic only — "🟢 open→open, 🟡 to won/lost" (interfaces.md §2.1, A34/ADR-0026 R6)
> — which leaves a terminal→open reopen 🟢. The deals-and-pipeline chapter supersedes
> this with the either-endpoint rule (reopening is governed too), and this chapter's
> tool row follows the chapter, not the corpus. The divergence should be reconciled
> upstream in a corpus errata pass.

### Schema
Source: specs/spec/contract/data-model.md#outbound-webhooks-e10--s-e106-a51--governed-integration-surface-not-a-marketplace @ 5a0b29c; specs/spec/contract/data-model.md#agent-connections-e10-byo-agent--features04-3-03c @ 5a0b29c

Tables owned per the data-model ownership index ([[data-model]]): `agent_connection`,
`agent_connection_event`, `webhook_subscription`, `webhook_delivery`. Base columns +
`version` per the data-model conventions; RLS on `workspace_id` throughout
([[data-model#DM-CONV-5]]).

**BYO-DDL-1 — `agent_connection`** (a registered BYO agent bound to a Passport and a
human):

```sql
CREATE TABLE agent_connection (                           -- a registered BYO-agent (Claude/Cursor/ChatGPT) bound to a Passport
  -- + base columns + version
  display_name  text NOT NULL,
  surface       text NOT NULL CHECK (surface IN ('local_stdio','hosted_https')),  -- Surface A1 / A2
  passport_id   uuid NOT NULL,                             -- the Agent Seat Passport this connection acts under
  on_behalf_of  uuid NOT NULL REFERENCES app_user(id),     -- bound human (the on-behalf-of identity)
  scopes        jsonb NOT NULL,                            -- effective scopes (≤ human RBAC)
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked')),
  last_seen_at  timestamptz NULL
);
```

**BYO-DDL-2 — `agent_connection_event`** (connect/disconnect/revoke audit for the
connection):

```sql
CREATE TABLE agent_connection_event (
  id uuid PRIMARY KEY, workspace_id uuid NOT NULL REFERENCES workspace(id),
  agent_connection_id uuid NOT NULL REFERENCES agent_connection(id),
  event_type    text NOT NULL CHECK (event_type IN ('connected','disconnected','revoked','scope_changed')),
  occurred_at   timestamptz NOT NULL DEFAULT now()
);
```

**BYO-DDL-3 — `webhook_subscription`** (outbound subscription; HTTPS-only,
owner-scoped delivery):

```sql
CREATE TABLE webhook_subscription (                        -- fan-out of domain events over the bus (events.md §4)
  -- + base columns + version
  target_url     text NOT NULL CHECK (target_url ~ '^https://'),  -- HTTPS-only; http:// rejected at create
  event_types    text[] NOT NULL,                          -- a subset of the PUBLISHED event catalog (events.md §2)
  signing_secret_ref text NOT NULL,                        -- vault ref; HMAC-SHA256 over the raw body (X-Margince-Signature); secret never stored plaintext
  state          text NOT NULL DEFAULT 'active' CHECK (state IN ('active','paused')),
  owner_id       uuid NOT NULL REFERENCES app_user(id),    -- owning principal: fan-out delivers ONLY events for data this principal may see, enforced at delivery not just registration
  CHECK (cardinality(event_types) > 0)
);
```

**BYO-DDL-4 — `webhook_delivery`** (per-attempt delivery log + state; at-least-once,
retry-with-backoff, dead-letter; read through the parent subscription's endpoints, not
standalone CRUD):

```sql
CREATE TABLE webhook_delivery (
  id uuid PRIMARY KEY, workspace_id uuid NOT NULL REFERENCES workspace(id),
  subscription_id uuid NOT NULL REFERENCES webhook_subscription(id),
  event_id       uuid NOT NULL,                            -- the domain event delivered (events.md §2 versioned envelope)
  status         text NOT NULL CHECK (status IN ('pending','delivered','retrying','dead_lettered')),
  attempts       int NOT NULL DEFAULT 0,                   -- exponential backoff 1s→32s, 6 attempts
  last_status_code int NULL,
  next_retry_at  timestamptz NULL,
  dead_lettered_at timestamptz NULL,                       -- parked deliveries keep a 7-day window + inspectable replay
  created_at     timestamptz NOT NULL DEFAULT now()
);
```

### Wire
Source: specs/spec/narrative/03c-agentic-concept.md#41-layered-structure-top-to-bottom @ 5a0b29c; specs/spec/contract/crm.yaml (NET-NEW V1 RESOURCES block) @ 5a0b29c; specs/spec/contract/api-rate-limits-and-abuse.md#24-read-volume--write-quotas--step-up-the-05-d3-mechanism @ 5a0b29c

| ID | Surface | Contract |
|---|---|---|
| BYO-WIRE-1 | **A1 — local transport** | MCP over stdio to a local binary; the agent loop runs in the user's own client on their machine. Config-driven static Passport/scopes. |
| BYO-WIRE-2 | **A2 — hosted transport** | MCP over Streamable HTTP/SSE to the **operator's** public service (hosting partner or self-hosting customer — never Gradion, ADR-0027); **OAuth 2.1 + PKCE + DCR** client auth. The always-on cloud/phone path (DECISIONS A19). |
| BYO-WIRE-3 | **Auth schemes** | Agent calls authenticate as `bearerAuth` under an Agent Seat Passport; humans as `cookieAuth` ([[api-conventions#API-CONV-11]]). 🟡 agent calls carry `X-Approval-Token` ([[approvals-and-concurrency#APPR-WIRE-1]]). |
| BYO-WIRE-4 | **Tool generation** | Every tool is a contract operation annotated `x-mcp-tool`; the tool list is generated by walking the annotations ([[contract-pipeline#CP-MCP-1]], [[contract-pipeline#CP-STAGE-8]]) — never hand-edited. |
| BYO-WIRE-5 | **Agent-connection resource** | `/agent-connections` — same resource shape as every net-new resource (list/get/create/update If-Match/archive); revoke via `POST /agent-connections/{id}/revoke`; connection events read-only. Pattern-specified in the contract's net-new block; codegen must emit the concrete operations and the AC-MCP-3 lint verifies them. |
| BYO-WIRE-6 | **Webhook-subscription resource** | `/webhook-subscriptions` — list/get/create/update (If-Match)/archive per the same pattern (`listWebhookSubscriptions`, `getWebhookSubscription`, `createWebhookSubscription`, `updateWebhookSubscription`, `archiveWebhookSubscription`). **Create/update is 🟡** — registering outbound egress (agent path: `create_record` tier yellow). Deliveries read via `GET /webhook-subscriptions/{id}/deliveries` (`listWebhookDeliveries`); a parked delivery replays via `POST /webhook-subscriptions/{id}/deliveries/{id}/replay` (`replayWebhookDelivery`). |
| BYO-WIRE-7 | **Delivery signature** | `X-Margince-Signature`: HMAC-SHA256 computed over the raw request body with the subscription's signing secret; the receiver must verify before trusting. Secret revealed once at create; rotation keeps the old secret valid for a 24h grace window (AC-integrations-3). |

Errors owned by this chapter (the rest of the agent-path error vocabulary is the
api-conventions chapter's: [[api-conventions#API-ERR-9]], [[api-conventions#API-ERR-10]],
[[api-conventions#API-ERR-11]], [[api-conventions#API-ERR-12]],
[[api-conventions#API-ERR-14]]):

| ID | Error | Shape |
|---|---|---|
| BYO-ERR-1 | `scope_exceeds_grantor` | **422** at Passport bind time: the requested scope set is not a subset of the grantor's effective RBAC; no passport row is minted ([[auth-and-sessions#AUTH-AC-4]]). Distinct from the per-call 403 `scope_exceeded`. |
| BYO-ERR-2 | Quota envelope | MCP 429-equivalent error envelope mirroring the REST 429 contract: `{ "code": "quota_step_up" \| "quota_throttled" \| "quota_suspended", "quota": "<quota id>", "observed": N, "limit": M, "action_required": "human_confirm" }`. |

### Events
Source: specs/spec/contract/events.md#4-redis-streams-layout @ 5a0b29c; specs/spec/contract/data-model.md#outbound-webhooks-e10--s-e106-a51--governed-integration-surface-not-a-marketplace @ 5a0b29c

Webhook fan-out is a consumer of the internal bus; the substrate — envelope, versioning,
at-least-once delivery, outbox relay, dedupe — is the event-bus chapter's
([[event-bus#EVT-ENV-1]], [[event-bus#EVT-DEL-1]]–[[event-bus#EVT-DEL-7]]) and is not
restated. This chapter owns the *outbound* delivery semantics:

| ID | Rule |
|---|---|
| BYO-EVT-1 | **Signed, versioned delivery.** A matching domain event is delivered as an HTTP POST of the same stable, versioned envelope the bus carries ([[event-bus#EVT-ENV-1]]), signed per BYO-WIRE-7. The published catalog is the contract; a breaking payload change ships a new major version while the old keeps delivering (S-E15.11b policy) — no silent schema drift. |
| BYO-EVT-2 | **Retry then park, never lose.** Non-2xx or unreachable → retried with exponential backoff 1s→32s, 6 attempts; after the budget the delivery parks in the dead-letter view (7-day window) where it can be inspected and replayed (BYO-DDL-4). Delivery is at-least-once; receivers dedupe on the envelope's event id ([[event-bus#EVT-DEL-2]]). |
| BYO-EVT-3 | **Gated events deliver post-approval only.** A 🟡-gated event type (e.g. an offer-sent event) is delivered **only after** the ADR-0036 approval gate clears — receiving a webhook never bypasses the gate. |
| BYO-EVT-4 | **RBAC-bounded fan-out.** A subscription emits only events for data its owning principal may see, enforced at delivery time (not just registration) — no privilege escalation via a webhook. Every registration/change/pause/delete/delivery appends an audit row (P12). |

### Limits
Source: specs/spec/contract/api-rate-limits-and-abuse.md#22-per-agent-session-quotas @ 5a0b29c; #23-per-tool-limits @ 5a0b29c; #3-abuse-prevention @ 5a0b29c

**Per-agent-session quotas** (a session = one Passport token's lifetime; counters are
per-session **and** aggregated per-Passport per rolling window, so short-session floods
cannot evade them — AC-MCP-4). Corpus IDs preserved verbatim. Defaults are SaaS
defaults; per-mode tuning has a security floor on the read threshold (AC-MODE-1).

| ID | Quota | 🟢 default (SaaS) | Notes / cross-ref |
|---|---|---|---|
| MCP-SESS-CALLS | Total tool calls / session | 1,000 | Hard ceiling; beyond → session-suspend pending review |
| MCP-SESS-READS | Records returned by read tools / session | **2,000** | **The D3 step-up threshold** ([[threat-model#D3]]). Beyond → step-up confirmation required to continue. **Per-record, not per-call** — a single search returning 5,000 rows trips it |
| MCP-SESS-WRITES | Mutating calls / session | 200 | create/update/delete/advance; beyond → 🟡 batch-confirm |
| MCP-SESS-EGRESS | External-egress tool calls / session | 20 | send/webhook/external API; each already 🟡 + content-aware. Registering a webhook subscription counts here too — it establishes outbound egress |
| MCP-SESS-RATE | Tool calls / second | 5 sustained, 20 burst | Smooths spikes; protects shared infra |
| MCP-SESS-COST | L2-model spend / session | tenant budget ÷ active sessions, soft | A single session cannot consume a disproportionate share of the workspace AI budget ([[ai-runtime#AIRT-PARAM-8]]) |

**Threshold-crossing actions** (every crossing is an audit event with the trust tiers
touched, and feeds anomaly detection):

| ID | Crossing | Action |
|---|---|---|
| BYO-STEP-1 | MCP-SESS-READS exceeded | Next read returns step-up-required (BYO-ERR-2); the connecting human confirms "this agent has read N records this session; continue?" via the approval/notification surface — mass exfiltration becomes a gated, visible event |
| BYO-STEP-2 | MCP-SESS-WRITES exceeded | Subsequent writes batch-confirm (one approval for the batch) — stops a write flood without blocking legitimate bulk updates |
| BYO-STEP-3 | MCP-SESS-EGRESS exceeded | External sends hard-stop for the session; resuming needs a new approved session. Egress is the exfiltration endpoint — fail-closed |
| BYO-STEP-4 | MCP-SESS-CALLS / rate exceeded | Throttle (429-equivalent with retry hint), then suspend on sustained breach (BYO-LADDER-3) |

**Per-tool limits** (layered under the session quotas; declared in the contract next to
each tool's tier — a tool missing either fails the lint, AC-MCP-3):

| ID | Tool | Tier | Per-tool limit | Rationale |
|---|---|---|---|---|
| BYO-LIM-1 | `read_record` | 🟢 | counts toward MCP-SESS-READS | Single-record; cheap |
| BYO-LIM-2 | `search_records` | 🟢 | max 200 rows/call (CAP-RESP), rows count toward MCP-SESS-READS | The mass-read vector — capped per call **and** per session |
| BYO-LIM-3 | `run_report` | 🟢 | 50/session; query-cost budget per call | Heavy; reuses the report complexity cap (CAP-QUERY-COST) |
| BYO-LIM-4 | `create_record` / `update_record` | 🟢 | toward MCP-SESS-WRITES; 100/batch | Reversible, low-risk |
| BYO-LIM-5 | `log_activity` | 🟢 | toward MCP-SESS-WRITES | Reversible internal write |
| BYO-LIM-6 | `enrich` | 🟡 (always-🟡 floor, BYO-FLOOR-1) | toward MCP-SESS-COST | External fetch leaves the workspace → confirm-first; invokes L2 → cost-metered |
| BYO-LIM-7 | `advance_deal` | 🟡 when terminal (BYO-FORM-2) | per contract; toward MCP-SESS-WRITES | High-value transition |
| BYO-LIM-8 | `draft_email` | 🟢 | 50/session | Draft only, never sends |
| BYO-LIM-9 | `send_email`, webhook, external | 🟡 + content-aware | MCP-SESS-EGRESS (20); allow-listed destinations only | The trifecta leg (3) — default-deny, tightest. Webhook-subscription registration counts here |
| BYO-LIM-10 | `connect_incumbent`, `disconnect_incumbent`, overlay `flip` | 🟡 (overlay lifecycle) | 5/session; toward MCP-SESS-WRITES; approval-required | Rare, high-blast-radius; connect/flip also draw on the incumbent-API budget ([[overlay-budget]]) |
| BYO-LIM-11 | overlay `read_sync_status` / `read_budget` / `reconcile` | 🟢 | `reconcile` 10/session, metered against the incumbent budget; status/budget toward MCP-SESS-READS | `reconcile` sweeps the incumbent API — budget-metered, degrades to mirror-with-staleness ([[overlay-budget]]) |

**Anomaly signals on the audit stream** (threshold/baseline rules in V1, not learned
models; detection is paired with the ladder so an anomaly *acts*, not just logs —
[[threat-model#D6]]). Consumed from the audit stream consumer
group ([[event-bus#EVT-CG-7]]):

| ID | Signal | Default trigger | Action |
|---|---|---|---|
| BYO-ANOM-1 | Off-hours bulk export | export/read volume > 3× the user's 30-day baseline outside their working window | throttle + alert + require step-up |
| BYO-ANOM-2 | New egress destination | send/webhook to a domain not seen for this tenant in 90 days | block the send, 🟡 review (composes with the D3 allow-list) |
| BYO-ANOM-3 | Read-rate spike | session read rate > 5× rolling baseline | step-up (anticipates MCP-SESS-READS) |
| BYO-ANOM-4 | Scope-edge probing | repeated denied-by-scope/quota calls (> 50/session) | suspend session; likely a compromised/misbehaving agent |
| BYO-ANOM-5 | Cross-record fan-out | one session touches > N distinct high-sensitivity records (PII/financial labels) | step-up + sensitivity-aware egress gate |

New tenants and users have no baseline; off-hours/spike rules fall back to absolute
defaults until one exists (corpus open item — cold-start policy unspecified).

**Throttle / suspend ladder** (deterministic; each rung is an audit event and surfaces
in the approval/notification inbox):

| ID | Rung | Behavior |
|---|---|---|
| BYO-LADDER-1 | Throttle | 429 / MCP-throttle with backoff; transient breaches stay here |
| BYO-LADDER-2 | Step-up | Require human confirmation to continue (the D3 gate) |
| BYO-LADDER-3 | Session-suspend | The Passport token is suspended; a new approved session is required (revocability, [[threat-model#D5]]) |
| BYO-LADDER-4 | Connection-suspend | The agent connection is disabled pending admin review (repeat offender / scope-probing) |
| BYO-LADDER-5 | Tenant circuit-breaker | See BYO-BRK-1..3 |

**Tenant-level circuit breakers:**

| ID | Breaker | Behavior |
|---|---|---|
| BYO-BRK-1 | Load breaker | Tenant exceeds aggregate ceilings for a sustained window → non-interactive traffic (agent sessions, bulk, export, capture backlog) is shed first; **interactive human CRUD is protected last** — real users keep their performance budgets even under a noisy agent storm (AC-FAIR-1) |
| BYO-BRK-2 | Cost breaker | The workspace AI-budget ladder; owned by the ai-runtime chapter ([[ai-runtime#AIRT-PARAM-17]]) — cited here because MCP-SESS-COST is its per-session share |
| BYO-BRK-3 | Half-open recovery | After a cooldown the breaker admits a trickle and restores full rate if healthy — no manual reset for transient spikes |

**Declaration lint (BYO-LINT-1):** anyone adding a tool to the contract MUST assign a
risk tier **and** a per-tool limit + which session quota it counts toward; a tool
missing either is a spec defect and fails the contract lint (verified by AC-MCP-3).
The numeric defaults above are first-principles starting points, not yet calibrated
against a real workload (corpus open item — tune from the BYO-agent dogfood before GA).

### Tools
Source: specs/spec/contract/interfaces.md#21-canonical-v1-tool-set @ 5a0b29c; #22-intent-level-tools-verbs-not-tables--03c-41-decisions-a18-adr-0009 @ 5a0b29c

**The canonical low-level set.** The MCP surface is record-type-generic (one read tool
with a record-type argument) while the REST contract is per-type, so the "operation"
column names the **logical family** — the contract lint (AC-MCP-3) asserts a concrete
contract operation exists for every tool × supported record type. Each tool delegates
to the datasource seam ([[datasource]]), so the set works unchanged in
system-of-record mode and overlay mode.

| ID | Tool | Scope | Tier | OpenAPI op (family) | Datasource verb | Notes |
|---|---|---|---|---|---|---|
| BYO-TOOL-1 | `search_records` | `read` | 🟢 | `searchRecords` | `Search` | Full-text + filter; counts toward MCP-SESS-READS per-record. |
| BYO-TOOL-2 | `read_record` | `read` | 🟢 | `getRecord` | `Read` | Single-record open; field-masked per RBAC. |
| BYO-TOOL-3 | `create_record` | `write` | 🟢 | `createRecord` | `Create` | Reversible (archive); stamps provenance `captured_by=agent:*`. |
| BYO-TOOL-4 | `update_record` | `write` | 🟢 | `updateRecord` | `Update` | Reversible. Human-owned fields split to 🟡 (BYO-PREC-1). |
| BYO-TOOL-5 | `advance_deal` | `write` | 🟢→🟡 (dynamic) | `advanceDeal` | `AdvanceDeal` | **The invocation-time exception:** tier resolved per call by BYO-FORM-2 — 🟡 when **either endpoint** is a terminal (won/lost) stage, close and reopen alike; 🟢 open→open ([[deals-and-pipeline#DEAL-WIRE-4]]). The resolver reads stage *semantics* from pipeline config (and the overlay stage mapping), not just the request args, so a renamed won stage still resolves 🟡. Enforced in the typed model, never ad-hoc (A34/ADR-0026). *Corpus divergence flagged in Formulas.* |
| BYO-TOOL-6 | `draft_email` | `draft` | 🟢 | `draftEmail` | (L2 model client) | Generation only; never sends. |
| BYO-TOOL-7 | `send_email` | `send` | 🟡 | `sendEmail` | (comms) | **Confirm-first**, egress; no SMTP without approval token. Consent is consulted before transport ([[sequences-and-deliverability]]). |
| BYO-TOOL-8 | `log_activity` | `write` | 🟢 | `logActivity` | `Create(activity)` | Reversible; provenance-stamped (P5). |
| BYO-TOOL-9 | `run_report` | `read` | 🟢 | `runReport` | `RunReport` | Compiled query plan, not free SQL; "explain this number" derivation. |
| BYO-TOOL-10 | `enrich` | `enrich` | 🟡 | `enrichRecord` | `Update` + L2 | **Confirm-first**: external fetch + writes enrichment; agent-decided, budget-guarded (MCP-SESS-COST). |
| BYO-TOOL-11 | `archive_record` | `write` | 🟡 | `archiveRecord` | `Archive` | **Confirm-first**: soft-delete a person/org/deal/lead — hard to undo (visibility change). |
| BYO-TOOL-12 | `disqualify_lead` | `write` | 🟡 | `disqualifyLead` | `Update(lead)` | **Confirm-first**: sets disqualified + archived — lifecycle/visibility change. |
| BYO-TOOL-13 | `merge_records` | `write` | 🟡 | `mergeRecords` | `Merge` | **Confirm-first**: combine two people/orgs — not cleanly reversible. |
| BYO-TOOL-14 | `share_record` | `write` | 🟡 | `createRecordGrant` / `revokeRecordGrant` | (native) | **Confirm-first**: grant/revoke a manual per-record share (ADR-0039) — widens who can see a record. Agent grants are held; never wider than the granting human's own access (scope-intersection). |

> Task creation is `create_record` with an activity of kind task (the activity table is
> polymorphic) — not a separate tool, keeping the surface minimal. The skeleton
> additionally routes lead promotion and conversation link/unlink through the same
> gate; those rows are owned by [[leads-and-qualification#LEADS-WIRE-6]] and the
> messaging-channels chapter respectively (@ a11d6c08), not restated here.

**BYO-FLOOR-1 — the always-🟡 floor (non-negotiable; A34/ADR-0026,
[[threat-model#D4]]).** `send_email`, outbound connector actions,
`archive_record`, `merge_records`, `disqualify_lead`, `advance_deal` where either
endpoint is terminal (per BYO-FORM-2 — the corpus states the target-only form), and
`enrich` can never be made 🟢 — not by a customer, not by configuration, not by anyone.
Loosening below the floor is not an exposed setting.

**BYO-TIER-1 — tighten-only re-tiering.** A workspace admin MAY raise a 🟢 tool to 🟡;
MAY NOT lower the floor. V1 may ship with the control not yet exposed; this is the rule
when it lands.

**BYO-PREC-1 — human-edit precedence.** An agent's 🟢 `update_record` cannot overwrite
a human-typed field without a 🟡 confirm. Mechanism: a field is human-owned if its
*current* value's most recent write had a human actor — decided per field from the
append-only audit trail (its before/after diff is already per-field). The 🟢 update
path checks each field it would change; any human-owned field whose value differs is
split into a 🟡 staged change while the rest of the 🟢 update proceeds. No new
per-field provenance store.

**The intent-tool layer** (verbs, not tables — DECISIONS A18, ADR-0009). Pure
compositions over the low-level set and the datasource seam under the *same* Passport
scope intersection and the *same* 🟡 gates — a 🟡 step inside an intent tool still
requires a token; composition mechanics owned by [[intent-tools#INTENT-AC-1]]–
[[intent-tools#INTENT-AC-3]]. Reads return an assembled, provenance-stamped context
object, not raw rows; results may render as an interactive component in the assistant
(MCP Apps, DECISIONS A19) — a presentation of the same result, never a new authority
path.

| ID | Intent tool | Composes | Scope | Tier | Notes |
|---|---|---|---|---|---|
| BYO-INTENT-1 | `catch_me_up_on{ref}` | `Read`+`Search`+graph assembly | `read` | 🟢 | One account/deal/person → the assembled picture (people, last touches, open questions, history) with provenance. |
| BYO-INTENT-2 | `whats_slipping_this_week` | `RunReport`+stalled rules | `read` | 🟢 | Idle/at-risk deals as a ranked, tappable set — not a row dump ([[deals-and-pipeline#DEAL-FORM-3]]). |
| BYO-INTENT-3 | `prep_for_meeting{event}` | graph assembly + dossier | `read` | 🟢 | Pre-meeting dossier: attendees, deal state, gaps ([[meetings-and-transcripts]]). |
| BYO-INTENT-4 | `qualify_lead{ref}` | `Read`+`Update`+gap-fill | `write` | 🟢 | Agentic gap-only qualification (DECISIONS A15); fills inferable fields, surfaces only gaps ([[leads-and-qualification]]). |
| BYO-INTENT-5 | `progress_deal{ref}` | `advance_deal`+`log_activity` | `write` | 🟢→🟡 (dynamic) | Inherits BYO-TOOL-5's dynamic resolver (BYO-FORM-2), resolved per-call by the admission gate. |
| BYO-INTENT-6 | `draft_follow_ups_for{segment}` | `Search`+`draft_email` | `draft` | 🟢 | Batch drafts in the user's voice ([[voice-profile]]); never sends. |

### Acceptance — stories (owned: S-E10.1, S-E10.3, S-E10.6)
Source: specs/spec/product/epics/E10-byo-agent-and-customization.md @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| BYO-AC-1 | Given a Claude/Cursor agent, when the connector token/OAuth handshake completes, then the agent appears in-product as a connected Agent Seat Passport bound to the connecting human's identity with an explicit scope, default read+draft (S-E10.1). | Integration test, agents lane; BYO-DDL-1 row + [[auth-and-sessions#AUTH-WIRE-5]] |
| BYO-AC-2 | Given a read+draft Passport, when the agent calls search/read/report/draft tools they succeed; when it attempts send, advance-to-terminal, archive, or any 🟡 tool, then the call is refused with a readable reason — never silently dropped (S-E10.1). | Integration test per tool row (BYO-TOOL-1..14); refusal envelope asserted |
| BYO-AC-3 | Given the agent reads auto-captured (T2) content, when it acts, then it cannot exceed the connecting human's RBAC and cannot reach external systems beyond its scope — egress default-deny (S-E10.1). | [[threat-model#TM-VERIFY-1]] probe + egress default-deny test |
| BYO-AC-4 | Given the agent did anything, when the audit log is opened, then every read/draft/tool-call is attributable to *this agent under this human's authority*, with inputs, outputs, and a replayable trace (S-E10.1). | Audit path-coverage invariant ([[audit-observability]]) |
| BYO-AC-5 | Given a needed field/object/rule (S-E10.3), when the engineering is done against the conventions and scaffolding, then the result is a reviewable PR — migration, domain type, contract, UI, and a per-field contract round-trip test — real code, not a settings toggle; CI (full suite + contract-drift + design-drift) must be green before merge; a wrong edit fails to compile or fails a test, never ships silently. | [[generators#M1]] round-trip mandate + [[quality-gates#QG-7]] contract-drift gate |
| BYO-AC-6 | Given the merged customization is deployed (S-E10.3), when the rule fires on real data, then the behavior is observable on qualifying records and the change lives in code the customer owns, versioned in git, on their infrastructure; a later upstream bump survives it via the scaffolded upgrade preflight. | [[generators#GEN-UPG-1]]–[[generators#GEN-UPG-5]]; fork-upgrade e2e |
| BYO-AC-7 | Given the integrations settings (S-E10.6), when a webhook subscription is registered (target URL + event types from the published catalog), then matching domain events are delivered as signed HTTP POSTs (HMAC signature header) the receiver can verify (BYO-EVT-1, BYO-WIRE-7). | Integration test: seeded event → delivery with valid signature |
| BYO-AC-8 | Given a delivery fails (endpoint down / non-2xx), when retries exhaust the budget, then the delivery was retried with backoff and parked in a dead-letter/redelivery view that can be inspected and replayed — no silent loss (BYO-EVT-2). | Integration test with failing receiver; DLQ row + replay asserted |
| BYO-AC-9 | Given a subscription, when it runs, then it emits only events for data the owning principal may see (no privilege escalation via a webhook), and every registration/change/delivery is in the audit log (BYO-EVT-4). | RBAC-bounded fan-out test: out-of-scope event never delivered |
| BYO-AC-10 | Given event payloads, when delivered, then they carry the same stable, versioned event schema as the bus, and a breaking change follows the versioning policy — the published catalog is the contract (BYO-EVT-1). | Contract test: delivered payload = [[event-bus#EVT-ENV-1]] envelope |
| BYO-AC-11 | Given the anti-marketplace stance, when the user looks, then this is a first-party outbound event surface they own (subscriptions live in their workspace), not an installable third-party app store (BYO-AC-F5 holds). | Negative scope check in review; AC-integrations-1 |

### Acceptance — governed integration surface (verbatim)
Source: specs/spec/features/04-platform-and-compliance.md#3-integrations--mcp-app-connectors-not-a-marketplace @ 5a0b29c

| ID | Criterion (verbatim) | Verification |
|---|---|---|
| BYO-AC-F1 | An inbound email to a known `person` creates exactly one `activity` with `source` = message-id and `captured_by` = capture agent; no duplicate on re-sync (idempotency test). | Idempotency integration test (behavior owned by [[capture]]; cited here as the connector-surface criterion) |
| BYO-AC-F2 | A new connector can be added solely by implementing the documented connector interface + registering it — verified by scaffolding producing a compiling connector skeleton. | [[generators]] scaffolding test |
| BYO-AC-F3 | Connector-exposed MCP tools respect Passport scope and 🟢/🟡 tier identically to core tools (shared enforcement test). | Shared admission enforcement test, agents lane |
| BYO-AC-F4 | Every connector-originated mutation carries provenance (`source`, `captured_by`) in the audit log. | Provenance guard test ([[api-conventions#API-CONV-6]]) |
| BYO-AC-F5 | There is **no** runtime UI to install arbitrary third-party apps (asserted by absence; the product has no marketplace surface — negative scope check in review). | Negative scope check, release review |
| BYO-AC-F6 | **User-observable (Devin, S-E10.1):** the connected agent acts through the Gmail/Calendar connector exactly as on core records — drafts (🟢) but the send waits for approval (🟡) — with no separate integration-permission model; connector tools obey the already-granted Passport scope. | e2e through a connector tool; same-gate assertion |

### Acceptance — quotas & abuse (corpus IDs preserved)
Source: specs/spec/contract/api-rate-limits-and-abuse.md#52-machine-verifiable-acceptance-criteria @ 5a0b29c

Deterministic (counters, status codes, audit rows, config validation) — genuine CI /
integration gates, not model-output evals. The REST-side rows (AC-RL-*) and the cost
breaker (AC-COST-1, held by [[ai-runtime#AIRT-AC-3]]) are owned elsewhere.

| ID | Criterion | Verification |
|---|---|---|
| AC-MCP-1 | An agent session reading > MCP-SESS-READS (2,000) records is forced to **step-up** on the next read call (`quota_step_up`), and the crossing emits an audit event with trust tiers touched. | Seeded session reads 2,001 records via search/read; assert step-up error + audit row |
| AC-MCP-2 | An agent exceeding MCP-SESS-EGRESS hard-stops external sends for the session; a send to a non-allow-listed/new-90d domain is blocked + queued for 🟡 review. | Drive 21 sends + one to a novel domain; assert hard-stop + block + review item |
| AC-MCP-3 | Every tool in the contract has **both** a risk tier and a per-tool limit + session-quota mapping; a tool missing either fails the contract lint (BYO-LINT-1). | CI contract-lint over the contract: assert presence for all tools |
| AC-MCP-4 | A short-session flood (10 sessions × 300 reads) trips the **per-Passport rolling-window** read aggregate, not just per-session — evasion is closed. | Scripted multi-session run; assert aggregate step-up fires |
| AC-AB-1 | A session with > 50 denied-by-scope/quota calls is **suspended** (scope-probing); resuming requires a new approved session. | Drive denied calls; assert suspend + audit + token revocation |
| AC-AB-2 | An off-hours read volume > 3× baseline triggers throttle + step-up + alert. | Seed baseline; clock-controlled off-hours burst; assert ladder + notification |
| AC-FAIR-1 | Under a single-tenant agent storm, a second tenant's interactive CRUD p95 stays within the performance budgets (interactive-first shedding works). | Two-tenant load test; assert tenant B budgets hold ([[acceptance-standards]] PERF) |
| AC-MODE-1 | On-prem lowering MCP-SESS-READS below the security floor without an ADR marker fails the config-validation check. | Config-validation test: value < floor → rejection |

### Acceptance — agent hub screen (`ai`, primary story S-E10.1; verbatim)
Source: specs/spec/product/30-screen-acceptance.md#aihtml--ask-ai-the-two-surface-agent-hub-implements-s-e10135 @ 5a0b29c

Claimed per the screen→story index (primary S-E10.1; also serves S-E10.3/.5). Zone 1's
runtime behavior belongs to the agent-runner and ai-runtime chapters; the screen pins
live here. States floor (empty, loading/streaming, zero-grounded-result, error+retry,
RBAC-denied distinct from 🟡 tier-blocked, no-brain-configured) inherited from the
acceptance-standards chapter.

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-ai-1 | Given the screen loads, When the topbar renders, Then it shows an honest brain line for the in-app assistant — a live (green) dot and "Ask Margince · running on `<configured model>`" (e.g. "EU-hosted Llama") — **not** "your Claude agent"; plus a "tool permissions" link → settings. | e2e, web lane |
| AC-ai-2 | Given no conversation, When Zone 1 loads, Then an empty state shows the title, copy stating "Margince's runner reasons on your configured model; our governed tools act under your permissions … read & draft run inline and free … anything that writes, sends, or advances a deal is blocked and routed to your Approval Inbox," a link to the inbox, and three prompt chips. | e2e, web lane |
| AC-ai-3 | Given the governed-tool palette (right rail), When it renders, Then it is headed "What any agent can do here" with sub-label "scopes ⊆ your RBAC", lists each governed capability **capability-phrased** with its tier glyph (eye 🟢, lock 🟡), its raw tool name as secondary mono text (progressive disclosure), and its RBAC scope token; a legend (🟢 inline/free/reversible, 🟡 blocked/routed); a note that an agent can never exceed your permissions and the tier is a property of the tool declared in the contract; and **no source-change tool is present** (A39). Out-of-scope tools render disabled (RBAC-denied if called). | e2e, web lane; palette contents = BYO-TOOL/BYO-INTENT tables |
| AC-ai-4 | Given I click the "stalled deals" chip, When it runs, Then the empty state hides; the reply shows a Plan, a "governed tool calls" log with the deal search (🟢) and draft (🟢) succeeding inline and the send (🟡) "blocked", plus a banner "🟡 N sends queued for your approval … Nothing left the CRM." with "Review & approve" → the inbox. | e2e, web lane |
| AC-ai-5 | Given the stalled-deals result, When result cards render, Then each deal card shows name, sub-line, amount, and an evidence block (quote + provenance source); and a draft email preview labelled "🟢 draft only — not sent". | e2e, web lane |
| AC-ai-6 | Given I click the "Add a renewal_risk field" chip (or type any schema/formula/workflow change), When it runs, Then the reply does **not** scaffold code or open a PR — it **redirects to the development path**, stating that changing the data model is a code change that ships as a new version, done by the customer's engineers, a partner, or Gradion (A39), with a link to that path; no scaffold, no PR/diff card is rendered. | e2e, web lane (S-E10.3 boundary) |
| AC-ai-7 | Given Zone 2 ("Connect your own agent"), When it renders, Then it is **not a chat** — it shows a connection card (the MCP endpoint, an "Add to Claude / ChatGPT / Cursor" affordance, connection status, and the Passport scopes the connection carries), a "where approvals land" line ("a 🟡 action your agent reaches — here or in ChatGPT — appears in your Approval Inbox"), and a note that the agent loop runs in the user's own client, reaching in via MCP. | e2e, web lane |
| AC-ai-8 | Given Zone 2's "what your agent can do" literacy block, When it renders, Then it lists **outcome-phrased, copyable example prompts** drawn from the intent-tool layer (e.g. "Catch me up on the Acme deal and draft a follow-up", "What's slipping this week?", "Prep me for the 2pm with Acme") each with a copy-to-clipboard affordance — and surfaces **no raw tool signatures** as the headline (tool names are progressive-disclosure detail in the palette only). | e2e, web lane; prompts map to BYO-INTENT-1..3 |
| AC-ai-9 | Given I type free text in Zone 1 and press Enter (no Shift) or click Send, When non-empty, Then the empty state hides, my message appends, the assistant posts a canned ack (runs 🟢 inline, queues 🟡 to the inbox), the textarea clears, and a toast appears. | e2e, web lane |
| AC-ai-10 | Given the composer textarea, When I type, Then it auto-grows to a max; Shift+Enter inserts a newline rather than sending. | e2e, web lane |
| AC-ai-11 | Given the composer footer, When the screen renders, Then it states "🟢 read/draft run inline · 🟡 write/send/advance need your approval — all runs are audit-logged". | e2e, web lane |
| AC-ai-12 | Given any Zone 1 reply, When the author line renders, Then it attributes the reply to "Ask Margince · running on `<configured model>`" — never to "your Claude agent". | e2e, web lane |
| AC-ai-13 | Given Zone 3 (proactive), When the page renders, Then it carries a pointer (not a duplicated roster) to the scheduled/overnight agents on the automations surface — "what the AI does while you're away" — consistent with the two-mode story. | e2e, web lane; roster owned by [[overnight-agent]] |

### Acceptance — integrations screen (S-E10.6; verbatim)
Source: specs/spec/product/30-screen-acceptance.md#integrationshtml--webhooks--integrations-implements-s-e106 @ 5a0b29c

Additional states from the screen spec: no-subscriptions empty state and a
loading/fetch state (missing in the prototype, required in the build); the
"What this is — and isn't" honest-scope card (no inbound receiver, no marketplace, no
per-event metered charge).

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-integrations-1 | Given the screen at load, When it renders, Then the header shows "Webhooks & integrations" with a "New subscription" button, an anti-marketplace banner stating subscriptions live in "your workspace", emit only RBAC-bounded events (no privilege escalation), are audit-logged, and that this is an outbound surface you own (P2/P7) — not an installable third-party marketplace. | e2e, web lane |
| AC-integrations-2 | Given the Active subscriptions section, When it renders, Then each subscription card shows a name with a state pill (live / failing / paused), the target HTTPS URL, event-type count, last-delivery status, a 14-bar delivery health sparkline, the subscribed event pills, and a masked signing-secret reference. | e2e, web lane |
| AC-integrations-3 | Given a live subscription card, When I click "Rotate secret", "Send test event", or "Pause", Then a toast confirms the action — rotate states the old secret stays valid for a 24h grace window, test event states it is signed and marked test=true, and Pause flips the card to the paused state with a "Resume" control (and Resume flips it back to live). | e2e, web lane; BYO-WIRE-7 rotation |
| AC-integrations-4 | Given a failing subscription, When it renders, Then it shows a red failing pill, the last-delivery error, failing bars in the sparkline, and a "N parked in dead-letter" count with a "View dead-letter" control that scrolls to the dead-letter card. | e2e, web lane |
| AC-integrations-5 | Given the Dead-letter section, When a parked delivery row renders, Then it shows the event type + version chip, a plain failure reason (e.g. "503 Service Unavailable on all 6 attempts, backoff 1s → 32s"), the linked record, parked-age, retry count (6/6), and "Replay" + "Inspect" controls; clicking "Replay" removes the row, decrements the parked count, and toasts a 200 OK redelivery; "Replay all" clears the remaining batch. | e2e, web lane; BYO-EVT-2 |
| AC-integrations-6 | Given the dead-letter is emptied via replay, When the last parked delivery is replayed, Then the card swaps to an empty state reading "Dead-letter empty — All parked deliveries were replayed successfully. Nothing was lost." | e2e, web lane |
| AC-integrations-7 | Given the Published event catalog ("the contract"), When it renders, Then each event row shows the event name, payload-fields description, a version chip (v1), an autonomy gate badge, and a subscribe toggle; a gated event is shown muted with a 🟡 gated badge and copy that it is delivered only after the approval gate clears (ADR-0036) and receiving the webhook does not bypass the gate; a "the catalog is the contract" note states a breaking change ships a new major (v2) while v1 keeps delivering, with no silent schema drift. | e2e, web lane; BYO-EVT-1/3 |
| AC-integrations-8 | Given I click "New subscription", When the drawer opens, Then it shows a Target URL field (HTTPS-required hint), event-type checkboxes from the catalog (with 🟡 gated tag on gated events), and an RBAC-scope note; submitting with an empty/non-HTTPS URL shows an inline field error ("Must be a valid HTTPS URL — http:// is rejected"), and with no events selected shows "Pick at least one event type"; a valid submission closes the drawer and toasts that the signing secret is revealed once. | e2e, web lane; BYO-DDL-3 checks |

### Acceptance — chapter guarantees
Source: specs/spec/contract/interfaces.md#21-canonical-v1-tool-set @ 5a0b29c; specs/spec/quality (gate registry) @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| BYO-AC-12 | Given any configuration surface, role, or API, when an attempt is made to set a floor tool (BYO-FLOOR-1) to 🟢, then no such setting exists or the attempt is rejected — the floor cannot be loosened by anyone. | Negative config test + contract assertion (tier declared in contract, CP-MCP-2) |
| BYO-AC-13 | Given an agent 🟢 update that would change a field whose current value was last written by a human, when it executes, then that field is split into a 🟡 staged change (approval-required) while the remaining fields commit — determined from the audit trail per BYO-PREC-1. | Integration test, agents lane: seeded human write → agent update → held item for the human-owned field only |
| BYO-AC-14 | Given every shipped core read/write, when the tool surface is enumerated, then each is reachable through a governed tool under Passport scopes with tiers as declared. | [[acceptance-standards#GATE-CORE-7]] MCP-parity gate |
| BYO-AC-15 | Given a real BYO agent driven by natural-language goals against a seeded workspace, when the conformance harness runs, then end-state, audit completeness, scope-subset, and zero unauthorized 🟡 effects hold deterministically, within the step ceiling. | [[ai-evals#AISA-1]]–[[ai-evals#AISA-7]] harness; conformance matrix |
| BYO-AC-16 | Given captured injection payloads and a connected real agent, when exfiltration is attempted, then no payload achieves egress without hitting a 🟡 gate or a volume limit — blocking any promote-to-act (S-E10.2) general availability. | [[threat-model#TM-VERIFY-1]] injection red-team probe |
