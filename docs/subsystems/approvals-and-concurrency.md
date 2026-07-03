---
status: skeleton
module: backend/internal/modules/agents
derives-from:
  - margince specs/spec/decisions/ADR-0036-approval-token-and-concurrency.md
  - margince specs/spec/contract/interfaces.md#0-conventions-for-every-interface-here
  - margince specs/spec/contract/interfaces.md#2-mcp-tool-contract-layer-1
  - margince-poc/docs/architecture/api-conventions.md#approval-gating-x-approval-token
---
# Approvals & concurrency — what executes is exactly what was approved, against the world as it is now

> The platform seam behind the 🟡 confirm-first gate: a staged agent action is
> approved by minting a signed, single-use, short-lived token bound to that exact
> effect, re-validated against the live record at execution, and composed with the
> same optimistic-concurrency rule every writer obeys. Its promise: an approval can
> authorize one thing, once, and only while the world still matches what the
> approver saw.

## What it's for

A confirm-first action is meaningless if the action can drift between approval and
execution: a token that is just a string could authorize a different action than
the one approved, a stale approval could execute against a record that changed
during the human's think time, and concurrent writers could silently clobber each
other underneath both. This subsystem exists so that human approval is a *binding*
authorization, not a ritual. Its callers are the agent admission gate (every
🟡-tier tool execution consumes a token here), the approval decision operations a
human drives, and every mutating write path, which inherits the version-check
mechanics. The boundary: this chapter owns the token and re-validation mechanics
and how they compose with optimistic concurrency; the inbox surface where humans
decide is a planned sibling (see Out of scope), and the general If-Match wire rules
are owned by the api-conventions chapter.

## Principles it serves

- **P12 — governance by construction.** The 🟡 gate is a verifiable contract —
  signature, single-use, binding, freshness — not a convention an agent could argue
  with; every decision and expiry is audited.
- **ADR-0036 — approval-token binding, staged-effect re-validation, native
  optimistic concurrency.** The decision this chapter embodies: three facets of one
  problem — a mutation authorized in the past must still be valid, and still be the
  thing that was authorized, when it actually runs.
- **ADR-0026 — per-tool autonomy tiers.** The tier model that makes some actions
  confirm-first at all; ADR-0036 supplies the mechanics that tier relied on. The
  always-🟡 floor that no configuration can lower is pinned by the threat model
  ([[threat-model#D4]]).

## How it works

**Why approval mints a token.** When a human approves a staged 🟡 action, the
system mints a signed, single-use credential bound to that specific staged effect —
because the alternative, a bare opaque header, was found load-bearing and hollow: a
token minted for one action could authorize another, be replayed, or cross
workspaces (ADR-0036). Minting happens at the approve operation; consumption
happens at the 🟡 operation's admission gate, and nowhere else.

**What the token binds.** The claims tie the token to one tenant, one passport,
one tool, and one staged diff by hash — a token for an email send cannot authorize
a deal advance, and a token for one diff cannot authorize a different effect even
through the same tool. It expires on a short fuse, minutes not hours
(APPR-PARAM-2), and its unique identifier is recorded as consumed on first use, so
a replay fails closed as an invalid token. Verification checks all of it —
signature, unconsumed, unexpired, and every binding claim matching the operation
being executed ([[api-conventions#API-ERR-11]]).

**Staged effects re-validate at execution.** The approval records the row version
the diff was computed against. On approve-execute the server re-reads the target:
if the live version differs — the deal advanced, the contact revoked consent, the
record was merged away during the approval gap — execution is rejected with the
version-skew semantics, the staged action is marked stale, and the proposal goes
back to the inbox to be re-staged against current state
([[api-conventions#API-CC-5]]). Consent is re-checked at the same moment. Approval
means "this exact change, against the state I saw" — never "whatever this turns
into later."

**One concurrency rule under everything.** The re-validation above is not a
special case; it is the same optimistic concurrency every writer obeys. Every
mutable entity carries a version counter ([[api-conventions#API-CC-1]]), mutating
requests state the version they read ([[api-conventions#API-CC-2]]), and a
mismatch yields the version-skew conflict with the current record in the response
so the caller can diff and retry ([[api-conventions#API-CC-4]]). The staged-execute
path simply substitutes "the version captured at staging" for "the version the
caller read." Humans, inbound agents, the overnight runner, and workflow handlers
all hit the same rows; this rule is why none of them can silently clobber another.

**Decisions are exactly-once and edits are never an escalation.** A pending item
is claimed by a conditional state transition, so of two concurrent approvers
exactly one wins and the action fires once; the loser learns the item was already
decided. When the human modifies the staged payload before approving, the edited
effect re-enters the admission gate from scratch — re-tiered, re-checked, new diff
hash — and both the original proposal and the human's delta are audit-logged. An
edit that re-resolves to 🟡 does not execute under the original approval: the
spent item is closed and the edited effect is re-staged for its own approval
cycle, so a modification can never turn a staged note into an ungated external
send (ADR-0036). Approver eligibility follows the same *agent ≤ human* logic: the
approving principal must hold the authority to perform the action themselves.

**Unactioned means rejected.** An approval item that nobody decides expires after
its time-to-live — 72 hours by default (APPR-PARAM-1) — and expiry is an
auto-reject, never an auto-approve. Fail-closed is the only safe default for a
confirm-first action; the expiry is logged like any other decision, attributed to
the system. The inbox surface renders the live countdown
([[acceptance-standards#STATE-SP-2]]).

**What ships versus what is planned.** The skeleton ships the seam: staging,
decision mechanics with the exactly-once claim, token mint-and-verify, expiry, and
the platform-wide version concurrency. The approval inbox — the queue, diff
preview, and notification delivery where humans actually decide — is a planned
feature owned by the notifications-and-approval-inbox chapter.

## What's configurable

- **Approval item TTL** — how long a staged action waits before fail-closed
  expiry; default 72 hours, overridable per staged item at staging time
  (APPR-PARAM-1).
- **Approval token TTL** — the minted token's expiry; pinned by ADR-0036 as
  minutes-not-hours (APPR-PARAM-2); the exact minute count is an implementation
  default of the skeleton, not a spec constant.
- **Tier resolution** — which actions resolve 🟡 is per-tool policy owned by the
  byo-agent-and-mcp chapter; this seam takes the resolved tier as input. The
  always-🟡 floor is not configurable ([[threat-model#D4]]).
- **Signing key** — the token signature is workspace-scoped key material
  (ADR-0036), provisioned per deployment; a token from one workspace verifies
  nowhere else.

## Guarantees (enforced)

- **Single-use.** A token's identifier is consumed on first use; replay fails
  closed as an invalid token, and the effect cannot fire twice on one approval.
- **Effect-bound.** Tenant, passport, tool, and diff hash must all match the
  executing operation; a token can authorize exactly the staged effect it was
  minted for, nothing adjacent.
- **Fresh or nothing.** Execution re-checks the target's live version against the
  version captured at staging; drift rejects with version-skew and the proposal
  must be re-staged — approving stale state is structurally impossible.
- **Fail-closed expiry.** An undecided item auto-rejects at TTL; there is no path
  by which silence becomes consent.
- **Exactly-once decision.** The conditional claim transition guarantees one
  winner among concurrent deciders; a crash after execution leaves a terminal,
  non-refireable state rather than a re-approvable one.
- **Edits never escalate.** A modified payload is re-admitted and re-tiered; a
  🟡-resolving edit re-stages instead of executing, and the human delta plus the
  original proposal are both audited.
- **No silent clobber anywhere.** The same version rule guards every mutable
  entity for every writer class ([[api-conventions#API-CC-1..5]]).

## Acceptance

Done means: a 🟡 action proposed by an agent does not commit until approved;
approval executes exactly the approved effect exactly once; anything stale,
replayed, expired, or drifted resolves to a visible rejection state rather than a
quiet success or a quiet loss. The honest states are part of the contract:
already-decided, expired, and version-skewed outcomes must each be distinguishable
to the caller and the inbox. Testable forms are pinned in the Acceptance appendix
(APPR-AC-1..7); inbox screen states inherit from the acceptance-standards chapter
([[acceptance-standards#STATE-SP-2]]) and are verified with the planned inbox
feature.

## Out of scope

- **The approval inbox surface** — queue, dry-run diff rendering, deciding
  context, notification delivery, mobile response flow: planned, owned by
  notifications-and-approval-inbox.
- **Tier policy and the governed tool set** — which tools exist, their scopes and
  tiers, and the admission gate's scope arithmetic: owned by byo-agent-and-mcp
  (with the always-🟡 floor pinned at [[threat-model#D4]]).
- **The If-Match wire contract itself** — header shape, precondition-required
  behavior, and the conflict body are owned by api-conventions
  ([[api-conventions#API-CC-1..7]]); this chapter composes with them.
- **Content-aware egress flagging** — the rule that routes sensitive-content sends
  into mandatory review belongs to the threat model's egress controls
  ([[threat-model#D3]]).

## Where it lives

The approval seam inside the backend's agents module — staging, decision,
token mint-and-verify, expiry — with the version-concurrency half enforced
platform-wide through the data-model conventions and the api-conventions wire
rules. Read next: api-conventions (the concurrency and error contract),
threat-model (why the gate exists), byo-agent-and-mcp (who gets gated),
notifications-and-approval-inbox (where humans decide), and auth-and-sessions (the
passport principal a token is bound to).

## Appendix

### Parameters
Source: decisions/ADR-0036-approval-token-and-concurrency.md @ 5a0b29c; features/05-notifications-and-collaboration.md#1-notifications--the-agent-approval-inbox @ 5a0b29c; margince-poc/docs/architecture/api-conventions.md#approval-gating-x-approval-token @ a11d6c08

| ID | Name | Value | Meaning |
|---|---|---|---|
| APPR-PARAM-1 | Approval item TTL | 72h default, per-item override at staging | An unactioned 🟡 item transitions to expired → auto-reject, never auto-approve; the expiry is logged with a system actor. (features/05; shipped as the staging default in the skeleton.) |
| APPR-PARAM-2 | Approval token TTL | short — minutes, not hours (`exp` claim) | ADR-0036 pins the class of value, not a number; the shipped default is an implementation constant of the skeleton, not a spec constant. |
| APPR-PARAM-3 | Token single-use rule | `jti` recorded as consumed on first use | Replay → invalid token ([[api-conventions#API-ERR-11]]); one approval, one execution. |
| APPR-PARAM-4 | Signing scope | workspace-scoped signing key | A token verifies only in the tenant it was minted for; cross-workspace reuse is structurally dead (ADR-0036). |

### Wire
Source: contract/crm.yaml (`ApprovalToken` schema + `X-Approval-Token` parameter, Approvals tag) @ 5a0b29c; margince-poc/contract/crm.yaml @ a11d6c08

| ID | Element | Behavior pinned |
|---|---|---|
| APPR-WIRE-1 | `X-Approval-Token` header | Required when an agent principal invokes a 🟡 operation; a human's direct call is itself the approval and carries no token. Missing → [[api-conventions#API-ERR-10]]; expired/replayed/mis-bound → [[api-conventions#API-ERR-11]]. |
| APPR-WIRE-2 | `ApprovalToken` claims | Serialized as a compact JWS; the claims shape (contract schema, restated here as the chapter's normative binding set): |
| APPR-WIRE-3 | `listApprovals`, `getApproval` | The staged-item read surface: list pending/decided items; fetch one with its full proposed diff and evidence. Ships as the seam's read path; the inbox UI over it is planned. |
| APPR-WIRE-4 | `approveApproval` | The mint point: claims the pending item (exactly-once), executes the (optionally edited) effect, returns the approval carrying the minted token; an already-decided item answers with the conflict semantics. |
| APPR-WIRE-5 | `rejectApproval` | Discards the staged action — no records changed; already-decided answers with the conflict semantics. |

APPR-WIRE-2 claims shape (`ApprovalToken`, compact JWS payload):

```json
{
  "jti": "unique token id — consumed on first use",
  "approval_id": "uuid of the Approval this token authorizes",
  "workspace_id": "uuid",
  "passport_id": "uuid | null — Agent Seat Passport it was minted for",
  "on_behalf_of": "uuid | null",
  "tool": "the exact 🟡 operation authorized, e.g. send_email",
  "diff_hash": "hash of the approved effect — binds the token to one staged change",
  "target_version": "row version the diff was staged against | null — re-checked at execute",
  "exp": "expiry timestamp (short TTL, APPR-PARAM-2)",
  "single_use": true
}
```

### Acceptance
Source: decisions/ADR-0036-approval-token-and-concurrency.md @ 5a0b29c; features/05-notifications-and-collaboration.md#acceptance-criteria @ 5a0b29c; margince-poc/docs/architecture/api-conventions.md#approval-gating-x-approval-token @ a11d6c08

| ID | Given/When/Then | Verification |
|---|---|---|
| APPR-AC-1 | Given a minted approval token already consumed once, when it is presented again, then the operation fails closed as an invalid token and no effect fires a second time. | Integration test, agents module lane |
| APPR-AC-2 | Given a pending approval item unactioned past its TTL (APPR-PARAM-1), when the expiry sweep runs, then the item transitions to expired → auto-reject (never auto-approve) and the expiry is audit-logged with a system actor. | Time-advanced integration test |
| APPR-AC-3 | Given a staged effect whose target row's live version no longer equals the version captured at staging, when approve-execute runs, then execution is rejected with the version-skew semantics ([[api-conventions#API-CC-5]]), the item is marked stale, and the proposal returns to the inbox for re-staging — nothing commits. | Integration test, agents module lane |
| APPR-AC-4 | Given a human who edits the staged payload and then approves, when the edited effect executes, then the audit trail carries both the original agent proposal and the human delta, and the edited payload is what committed. | Integration test, agents module lane |
| APPR-AC-5 | Given an edit that re-resolves the action to the 🟡 tier, when the human confirms the modify, then the original item is spent without executing and the edited effect is re-staged as a fresh pending item for its own approval cycle. | Integration test, agents module lane |
| APPR-AC-6 | Given two concurrent approvers of the same pending item, when both decide, then exactly one wins the claim and the effect fires exactly once; the other receives the already-decided outcome. | Concurrent-decision integration test |
| APPR-AC-7 | Given a valid unexpired token minted for one tool and diff, when it is presented on a different tool or a different staged diff, then verification fails closed as an invalid token — binding claims must match the executing operation. | Integration test, agents module lane |
