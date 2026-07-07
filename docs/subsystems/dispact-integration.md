---
status: planned
module: backend/internal/modules/interop (the Dispact bridge — home of the flow-bridge consumer group, the one group the event-bus roster leaves unmapped to a module) · frontend/src/features/dispact-link
derives-from:
  - specs/spec/features/04-platform-and-compliance.md#3a-dispact--slack-integration-p9--shared-foundations-with-dispact
  - specs/spec/product/epics/E14-suite-with-flow.md#s-e145--cross-link-a-crm-persondeal-to-a-dispactslack-thread
  - specs/spec/contract/data-model.md#suite-with-flow--messaging-links-e14e15
  - specs/spec/product/30-screen-acceptance.md#dispact-linkhtml--crm--dispact-cross-link-implements-s-e145
  - specs/spec/use-cases/UC-E14-01-deal-conversation-crosslink.md
---
# Dispact integration — one link, both directions, and nothing deeper until it's earned

> The seam between Margince and Dispact, Gradion's sibling workspace product. V1 ships
> exactly one capability: a CRM person or deal linked to a Dispact channel or Slack
> thread, resolvable from either end, with messages in the linked conversation landing
> on the record as provenance-stamped activities. Everything deeper is deferred, and
> the whole integration is optional — Margince works fully without it.

## What it's for

Deals get decided in conversations, and the conversation and the record are usually
two places that drift apart. For a customer who runs both independent products, this
subsystem removes that drift at its floor: the record knows its conversation, the
conversation knows its record, and a decision posted in the thread appears on the
timeline without anyone copying it across. The callers are the deal and person
record surfaces (which render the linked-conversation block and the link picker),
the capture path (which turns linked-conversation messages into activity rows), and
the user's agent (which may suggest — never silently create — a link). The scope
boundary is deliberately tight: this chapter owns the link itself and the screen
that manages it. It does not own deal channels, in-conversation cards, or handoff
briefings — those are deferred, and their cut lines live in the scope chapter.

Margince and Dispact are separate products that share foundations — auth, design
language, and the cross-product event bus — as the glossary's naming entry records
([[glossary]], the Margince/Dispact row). This subsystem is the CRM-side half of
that sharing, not a bundle: it exists only when a customer installs both.

## Principles it serves

- **P9 — Standalone first, integrations optional.** The whole chapter is P9 made
  concrete ([[principles]] P9): the link is additive, never a precondition. A
  workspace with no Dispact connection has an empty link table and an honest
  "connect in settings" state — no half-configured limbo, and no CRM behaviour
  gated on the sibling product being present.
- **P11 — real references.** The link is a first-class stored row with a uniqueness
  guarantee, not a metadata tag or a pasted URL. That is what makes round-trip
  resolution testable.
- **P5 — capture over data entry.** A linked conversation is a capture surface: the
  decision lands on the timeline because the shared bus carried it, not because a
  rep transcribed it.
- **P12 — provenance and consent.** Every captured message names its source and
  capturing actor; every link and unlink is audited to a human or an approved agent
  action. The deferred internal-signals capability is governed by the hardest P12
  gate in the product — pinned verbatim in this chapter's appendix (DISP-GOV-1)
  because it is the non-negotiable floor for that Backlog item.

## How it works

**Linking.** From a record's empty link state, a user opens a picker of the
workspace's Dispact conversations. Candidate channels may arrive tagged as
suggested, with the reason for the suggestion stated (a company-and-deal-name
match, for instance) — but the link is never auto-created: no query match means an
honest "no channel matches" message, never a silent best guess, and an ambiguous
match asks the user to pick. An agent can propose a link through the same governed
tool surface it uses everywhere else, at the confirm-first tier — "agent-suggested"
means a proposal the human approves, full stop. Creating the link writes exactly
one row (DISP-DDL-1's uniqueness key makes re-linking the same pair a no-op, not a
duplicate), stamps who linked it, and appends one audit entry.

**Both directions.** The link is one row read from both ends. The record shows its
linked conversation with a live-sync status pill and a jump-through affordance; the
conversation carries a pinned banner that links back to the record. A round-trip
resolution — record to conversation to record — lands where it started. This is
the bidirectionality the epic's story promises, and it comes from the one stored
reference, not from two writes that could drift.

**Capture.** A message posted in a linked conversation emits one event on the
shared cross-product bus, and the capture pipeline turns it into one activity row
on the linked record — source set to the message identity, captured-by naming the
capture agent, redelivery deduplicated so a replayed event never doubles the row.
On the timeline the captured item carries a from-Dispact badge and a clickable
source quote that jumps back to the originating thread: provenance preserved, not
flattened into anonymous text. The bus-side home for this flow is the flow-bridge
consumer group the event-bus chapter pins ([[event-bus#EVT-CG-5]]); the timeline
that renders the resulting rows belongs to [[activities-and-timeline]], which
explicitly routes the conversation-link storage here.

**Unlinking.** Unlink removes the reference and stops future post-backs — and
nothing else. Activities already captured stay on the record; the link footer
attests who linked it, when, and under which audit entry. A link is a live seam,
not the retroactive owner of the history that flowed through it.

**Degrading.** Absence of Dispact is a first-class state, not an error path. No
connector configured: the link surface says so and points at settings. The viewer
lacks permission on the conversation's own side: the link is visible but the
channel contents are not — the sibling product's access control is never bypassed,
and the agent's scope is never silently widened to compensate. Whatever state the
integration is in, the CRM record remains fully usable: a broken link degrades the
link block, never the record (P9).

**The deferred ladder.** Deal channels that auto-sync, acting on a record from
inside the conversation, and owner-change handoff briefings are real designs in
the corpus — and all three are out of V1, held at the fast-follow tier because
they are co-development with Dispact-side surfaces this spec does not command.
Their cut lines are owned by [[scope]] (the OUT rows for the three fast-follow
stories), not restated here. The internal champion-and-risk-signals capability
sits further back still, at Backlog behind a governance gate: it reads only
channels explicitly linked to the deal, shows its evidence, defaults to off, and
maintains no per-employee profile store — and if those cannot be guaranteed it
does not ship (DISP-GOV-1). This chapter specifies none of the deferred rungs
beyond acknowledging the floor they will stand on.

## What's configurable

- **The Dispact/Slack connection** — an injected, per-workspace dependency. Absent,
  the subsystem holds no rows and renders its honest not-connected state; present,
  it names which conversation system each link targets (the stored row carries the
  system discriminator, DISP-DDL-1). No knob turns the integration on for a
  workspace that has not connected it — absence is the default.
- **Nothing else in V1.** The deferred capabilities bring their own switches (a
  capability flag for bidirectional channel sync, an explicit opt-in with recorded
  consent for internal signals) — those arrive with their tiers and are noted here
  only so nobody expects them early.

## Guarantees (enforced)

- **Exactly one link per pair.** Linking a record to a conversation creates exactly
  one row; repeating the link is idempotent. Held by the uniqueness key in
  DISP-DDL-1 and asserted by a schema test.
- **Round-trip resolution.** The record resolves to its conversation and the
  conversation resolves back to the record, from the one row — a resolution test
  walks both directions (DISP-AC-4).
- **One message, one activity, once.** A linked-conversation message becomes one
  provenance-stamped activity row; event redelivery never duplicates it
  (DISP-AC-5). The correlation discipline is the bus chapter's
  ([[event-bus#EVT-CG-5]] and its capture-chain semantics).
- **No silent link.** Every link is a human act or a human-confirmed agent
  proposal; an unmatched picker query says so instead of guessing (AC-dispact-link-6).
- **Unlink is not erasure.** Removing a link stops future sync and leaves captured
  history on the record, with the link's attestation preserved (AC-dispact-link-7).
- **Optional and additive.** No CRM operation requires the integration; every
  degraded state (not connected, no permission, ambiguous match, syncing) renders
  honestly rather than blocking the record (P9; the screen's states series).
- **Audited.** Link and unlink each append exactly one audit entry attributing the
  actor (AC-dispact-link-5, AC-dispact-link-7).

## Acceptance

Done means: a user on a deal can link it to a conversation through the picker, see
the same link from both ends, jump through in either direction, watch a decision
posted in the thread arrive on the timeline with its badge and source quote, and
unlink without losing history. The surface must render its honest states — empty,
syncing, not connected, no permission on the far side, ambiguous match — rather
than pretending. The testable form of each claim is pinned in the Acceptance
appendix: the story's user-side criteria condensed, the feature spec's criteria
verbatim, the governance floor for the Backlog item verbatim, and the screen's
eight criteria verbatim. The cross-cutting floor (standard screen states,
performance budgets, release gates) is inherited from the acceptance-standards
chapter and not restated.

## Out of scope

- **Deal channel auto-sync, in-conversation record cards, owner-change briefings**
  — the three fast-follow stories; their deferral rows and reasons are owned by
  [[scope]] (Scope — OUT: deferred with a tier). This chapter builds the floor
  they will stand on and nothing above it.
- **Internal champion/risk signals** — Backlog, governance-gated; the gate is
  pinned here (DISP-GOV-1) because this chapter owns the floor it defends, but the
  deferral itself is [[scope]]'s row.
- **Timeline rendering and activity storage** — [[activities-and-timeline]]; this
  chapter owns the link table it routed here, not the rows capture writes.
- **Capture pipeline mechanics** — [[capture]]; this chapter consumes the
  captured result.
- **Bus plumbing, envelopes, delivery semantics** — [[event-bus]], including the
  flow-bridge consumer group itself.

## Where it lives

The Dispact bridge sits in its own backend interop module — the flow-bridge
consumer group's home — with the link surface in the web shell's dispact-link
feature. Callers reach it through the record surfaces (the link block and picker)
and the governed tool seam (the agent's link proposal). Read next:
[[activities-and-timeline]] (where captured decisions land), [[capture]] (how a
message becomes a row), and [[event-bus]] (the shared bus both products ride).

## Appendix

### Schema
Source: specs/spec/contract/data-model.md#suite-with-flow--messaging-links-e14e15 @ 5a0b29c

DDL copied verbatim from the corpus contract; all data-model conventions (base
columns, workspace RLS, provenance) apply; comments inside the fence are the
corpus's own. [[activities-and-timeline]] routes this table's ownership here, and
the data-model ownership index concurs. The corpus's earlier deferred stub for
this table is closed: [[data-model#DM-DEF-5]] records the promotion of the
deferred `flow_link` to this V1 `conversation_link`.

**DISP-DDL-1 — `conversation_link`**

```sql
CREATE TABLE conversation_link (                           -- the §12 flow_link, promoted to V1: CRM entity ↔ Dispact/Slack/Teams thread
  -- + base columns
  entity_type   text NOT NULL CHECK (entity_type IN ('person','organization','deal','lead')),
  entity_id     uuid NOT NULL,
  conversation_system text NOT NULL,                       -- 'dispact' | 'slack' | 'teams'
  conversation_id text NOT NULL,
  UNIQUE (workspace_id, entity_type, entity_id, conversation_system, conversation_id)
);
```

| ID | Note |
|---|---|
| DISP-DDL-2 | `deal_channel` and `gtm_module_install` are also routed to this chapter by the data-model ownership index, but they serve deferred stories (S-E14.1 fast-follow; suite modules) — owner-on-arrival. Their corpus DDL (same source section) is **not** pinned as V1 schema; it lands with the fast-follow work, mirroring the [[data-model#DM-DEF-1]] pattern. |

### Wire
Source: specs/spec/contract/crm.yaml (NET-NEW V1 RESOURCES comment block + contract-surface note) @ 5a0b29c; specs/spec/product/build-backlog/E14.md#b-e142--linkunlink-endpoint--agent-suggested-link-manual--mcp-verb @ 5a0b29c

| ID | Surface | Status |
|---|---|---|
| DISP-WIRE-1 | Link/unlink operations | **Honest gap — corpus-internal contradiction.** `crm.yaml` @ 5a0b29c defines **no operationId** for conversation-link create/unlink; its contract-surface note lists `conversation_link` among tables "surfaced through admin/parent endpoints, not standalone CRUD". The build backlog (B-E14.2) records a later spec-gate decision (2026-06-29, GH margince-poc #109) approving the opposite: a **standalone polymorphic resource** — `POST /conversation-links` + `DELETE /conversation-links/{id}`, body carrying `entity_type`/`entity_id`/`conversation_system`/`conversation_id` — with the `crm.yaml` comment to be updated when the ticket lands. Until `crm.yaml` carries the operations, the backlog ticket's shape is the pinned intent and the contract is behind it. SPEC-DISPUTE candidate; the ticket generator must treat B-E14.2's endpoint shape as normative. |
| DISP-WIRE-2 | MCP verbs `link_conversation` / `unlink_conversation` | Per B-E14.2 (same source): pure executors over the extended system-of-record provider seam (ADR-0013 single-surfaced), autonomy tier 🟡 (confirm-first) — an agent proposes, a human approves; an out-of-scope link attempt is refused identically to in-app. Not yet in `crm.yaml`'s tool annotations — same gap as DISP-WIRE-1. |

### Events
Source: specs/spec/contract/events.md#5-the-catalog @ 5a0b29c

Event definitions live in the central catalog owned by [[event-bus]]; cited here,
never redefined.

| ID | Role | Citation |
|---|---|---|
| DISP-EVT-1 | Consumer-group home | [[event-bus#EVT-CG-5]] — `cg:flow-bridge`, "Dispact interop", subscribes to person, deal, activity: "Cross-link CRM ↔ Dispact conversations; mirror to Dispact's view of the shared bus." The one V1 consumer group the event-bus roster maps to no target-layout module — this chapter's interop module is the proposed home. |
| DISP-EVT-2 | Linked message → timeline | `activity.captured` (owned by the [[event-bus]] catalog; flow-bridge is a listed consumer). The capture correlation chain (capture received → normalized → per-entity events, one correlation id) is [[event-bus]]'s EVT-SEM-10; idempotent redelivery is EVT-DEL-2. |
| DISP-EVT-3 | Schema-stub closure | [[data-model#DM-DEF-5]] — `flow_link` "**promoted to V1** as `conversation_link` — no longer deferred"; DDL pinned here as DISP-DDL-1. |
| DISP-EVT-4 | Link lifecycle events | **Honest gap:** the corpus catalog defines no `conversation_link.*` event type. Link/unlink is attested in `audit_log` (one row per mutation, B-E14.2), not on the bus. Any future bus-visible link event is new catalog work owned by [[event-bus]], not assumed here. |

### Acceptance
Source: specs/spec/product/epics/E14-suite-with-flow.md#s-e145--cross-link-a-crm-persondeal-to-a-dispactslack-thread @ 5a0b29c; specs/spec/features/04-platform-and-compliance.md#3a1-cross-link-crm-persondeal--dispactslack-conversation-mvp @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#dispact-linkhtml--crm--dispact-cross-link-implements-s-e145 @ 5a0b29c

**S-E14.5 user-side acceptance (condensed from the epic; V1-Must — the epic's only V1 story; S-E14.1/.2/.3 and S-E14.4 are deferred per [[scope]]):**

| ID | Given/When/Then | Verification |
|---|---|---|
| DISP-AC-1 | Given a deal, when Sam links it to a Dispact channel / Slack thread, then the deal shows the linked conversation and the conversation surfaces the linked deal — both directions, not just one. | integration test (round-trip resolution) |
| DISP-AC-2 | Given a linked conversation, when a decision or message is posted in it, then it appears on the deal's timeline as an activity, with its source, without Sam copying it across. | integration test over the bus (seeded transcript) |
| DISP-AC-3 | Given a linked record, when Sam clicks the link, then he lands in the right Dispact/Slack conversation directly — no searching. | e2e test |

**features/04 §3a.1 acceptance criteria (verbatim):**

| ID | Given/When/Then | Verification |
|---|---|---|
| DISP-AC-4 | `[MV]` "Linking a `deal` to a Dispact channel / Slack thread creates exactly one bidirectional link row (FK to the record, stable conversation id); the record resolves to the conversation and the conversation resolves to the record — verified by a round-trip resolution test." | round-trip resolution test |
| DISP-AC-5 | `[MV]` "A message in a linked Dispact conversation emits one capture event → one `activity` with `source` = message id and `captured_by` = capture agent; re-delivery is idempotent (no duplicate)." | event-replay test (row count unchanged) |
| DISP-AC-6 | "**User-observable (Sam, S-E14.5):** from a deal Sam clicks straight through to the linked Dispact/Slack conversation (and back from the conversation to the deal), and a decision posted in that thread shows up on the deal timeline without Sam copying anything over." | e2e test |

**The §3a.5 governance floor (verbatim — the non-negotiable gate on the Backlog item S-E14.4; the deferral row itself is [[scope]]'s):**

| ID | Pinned constraint | Verification |
|---|---|---|
| DISP-GOV-1 | "**Transparent rep-copilot, not a manager-snitch feed.** Signals serve **the deal and the rep working it**, surfaced to the people on the deal — **not** assembled into a covert per-employee behavioral profile or a manager dashboard of who-said-what about whom." · "**Consent + scope bounded.** It reads **only Dispact channels explicitly linked to the deal** (a Deal Channel, §3a.2) under workspace consent — **never** the org's general chatter, DMs, or unrelated channels. Off by default; opt-in per workspace with the constraint stated at enable time." · "**Evidence, not a mystery score.** A surfaced signal always shows its source message(s) and why it was read as advocacy/risk — inspectable and contestable, never a black-box sentiment number on a person." · "**Deal-scoped, company/role-level — not a dossier on a named colleague.** It characterizes the *deal's* internal health (champion present / blocker raised), not an individual employee's disposition tracked over time." · "If those cannot be guaranteed, the capability does not ship." | release gate on the Backlog item: scope-isolation test (unlinked channel/DM probe yields nothing), negative scope check (no per-employee profile store exists), default-off + consent-record test — per §3a.5's `[MV]` criteria |

**Screen ACs — dispact-link.html (verbatim; corpus IDs preserved):**

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-dispact-link-1 | Given the deal is in the linked state, When the screen renders, Then the deal card shows a "Linked conversation" block naming `#bär-packaging-qa` (6 members, last activity 9 min ago) with a "two-way · live" status pill and an "Open in Dispact" affordance, and the Dispact end at right shows a "Linked to CRM deal" pinned banner that links back to the deal record — the same link surfaced from both directions. | e2e test |
| AC-dispact-link-2 | Given the linked state, When the user clicks "Post a decision" in the Dispact compose box, Then a new decision message plus a "Gradion CRM" app card ("🟢 logged") append to the channel feed, and ~650ms later the same decision is prepended to the deal timeline (flagged "from Dispact", flashed in) with its activity count incremented and a toast confirming the decision landed on the timeline automatically, source-linked, nothing copied. | e2e test (the ~650ms is illustrative; UC-E14-01 flags the absent latency budget as a spec gap — the test asserts arrival, not a bound) |
| AC-dispact-link-3 | Given a decision posted back from Dispact, When it appears on the deal timeline, Then it carries a "from Dispact" badge plus a clickable source quote (e.g. `Lars: "Decision: we'll go with the 3-line QA package…"`) that, when clicked, jumps to the linked thread — provenance is preserved, not lost. | e2e test |
| AC-dispact-link-4 | Given the user toggles to the "Not yet linked" tab (or unlinks), When the empty state renders, Then the deal shows a dashed "No conversation linked yet" card with a "Link a conversation" button, and the sync banner is hidden. | e2e test |
| AC-dispact-link-5 | Given the empty state, When the user clicks "Link a conversation", Then a search picker opens (autofocused) listing Dispact channels where `#bär-packaging-qa` is tagged "suggested" with the reason "matches company + deal name", and typing filters the list; selecting a channel switches the deal to the linked state and toasts that both ends now show it with `linked_by=human:lars` and one audit row. | e2e test |
| AC-dispact-link-6 | Given the picker is open, When the user's query matches no channel, Then the list shows "No channel matches that. The link is never auto-created — pick an existing conversation, or type a thread URL." (no silent auto-link), and pressing Escape closes the picker. | e2e test |
| AC-dispact-link-7 | Given a linked deal, When the user clicks "Unlink", Then the deal returns to the empty state and a toast states the conversation no longer posts back while past activities stay on the deal (nothing deleted); the link footer attests `linked_by=human:lars · 14 Mar · audit row #a2c-7741`. | e2e test |
| AC-dispact-link-8 | Given any state, When the screen renders, Then a top-bar note states "Shared foundations (P9) — independent products, link is optional & additive" and a frame hint explains the link lives on both ends — the cross-link is positioned as optional/additive per P9, not a bundle. | e2e test |

The screen's honest-states series (syncing shimmer, integration-not-configured,
no-permission with the far side's access control unbypassed, ambiguous match held
for a human pick) is specified in the same source's "States & edge cases" and
inherits the standard-states floor from the acceptance-standards chapter; the
source notes those four render as static illustrative cards in the prototype.
