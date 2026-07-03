# ADR-0036 — Approval-token binding, staged-effect re-validation, and native optimistic concurrency

**Status:** Accepted (2026-06-23, deep spec red-team). Recorded as **DECISIONS A47**.

## Context

The 🟡 confirm-first gate (ADR-0026, `05-agent-security`, `features/05`) is the keystone of the whole
governance and compliance story — DPIA Art. 22, "agent ≤ human", the Overnight Agent's safety. The
2026-06-23 red-team found the keystone underspecified in three load-bearing ways:

1. **The approval token was an opaque `type: string`** (`crm.yaml` `X-Approval-Token`) with no defined
   contents, signing, single-use, TTL, or binding. As written, a token minted to approve action *A*
   could authorize a different action *B*; replay and cross-workspace reuse were unprevented. The
   entire 🟡 model rested on a bare string. *(RT-AI-C1)*
2. **The staged effect was not re-validated at approval time.** The Surface-B runner stages a diff,
   suspends for up to 72h, then on approval "re-submits the same step with the token" and executes
   against possibly-changed live state. There was no precondition check, so approving a stale proposal
   could advance an already-closed deal, email a contact who revoked consent in the gap, or write to a
   merged-away record — with a clean audit trail and a wrong result. *(RT-AI-C2 / RT-AR-H8)*
3. **There was no native optimistic concurrency.** `IfVersion`/`ErrVersionSkew` existed only for
   overlay mode; native (SoR) tables had no version token, yet `409 concurrent edit` was advertised.
   Multiple concurrent writers (UI human, Surface-A agent, Surface-B runner, workflow handler) would
   silently clobber via last-write-wins. *(RT-CT-8 / RT-AR-H1)*

These are three facets of one problem: **a mutation authorized in the past must still be valid, and
still be the thing that was authorized, when it actually runs.**

## Decision

### 1. The approval token is a signed, single-use, effect-bound credential

`X-Approval-Token` carries the `ApprovalToken` claims (`crm.yaml` schema), serialized as a compact JWS.
The gate verifies, on every 🟡 execution:

- **signature** (workspace-scoped signing key);
- **`jti` not yet consumed** — single-use; a replay returns `403 approval_token_invalid`;
- **`exp` in the future** — short TTL (minutes, not hours);
- **`workspace_id`, `passport_id`, `tool`, `diff_hash` all match** the operation being executed. A
  token for `send_email` cannot authorize `advance_deal`; a token for one staged diff cannot authorize
  a different effect (the `diff_hash` binds it).

It is no longer a bare string. The mint point is `POST /approvals/{id}/approve`; the consume point is
the 🟡 operation's admission gate.

### 2. Staged effects re-validate the target's `version` at execute time

Every `Approval` records `target_entity_type`/`target_entity_id`/`target_version` — the row version the
diff was computed against — and `diff_hash`. On approve-execute the server **re-reads** the target row.
If its current `version` ≠ `target_version`, execution is rejected with `409 version_skew`
(`ErrVersionSkew`); the staged action is marked stale and must be re-staged against current state. This
closes the "world changed during the approval gap" race for both Surface A and Surface B. Consent is
re-checked at this point too (a `consent.changed` in the gap blocks the send).

### 3. Native optimistic concurrency: `version` on every mutable entity

Every mutable domain table carries `version bigint NOT NULL DEFAULT 1`, bumped by the same
`BEFORE UPDATE` trigger that maintains `updated_at` (`data-model §1.3a`). The contract surfaces it as
the read-only `version` field (`RowVersion`) and accepts `If-Match: <version>` on native mutating
endpoints; a mismatch → `409 version_skew`. This is the SoR-mode mechanism, no longer overlay-only.
`ErrVersionSkew` (`interfaces.md §0`) now applies to the native path.

### 4. Who may approve, and what happens on edit-then-approve

- **Approver eligibility:** the approving principal must hold the RBAC required to perform the action on
  the target — at least the authority the `on_behalf_of` human would need. A lower-privilege user
  cannot approve an action they could not themselves perform. (Approval **delegation** remains OUT,
  `features/04 §1`.)
- **Modify-then-approve:** if the human edits the staged payload before approving, the edited effect
  **re-enters the admission gate from scratch** — re-tiered, re-RBAC-checked, new `diff_hash`, new
  token — so an edit can never escalate a 🟢 staged note into an ungated external send.

## Consequences

- The 🟡 gate is now a verifiable contract, not a convention. `features/05`, `07-surface-b-runner`, and
  the `Admit` gate must implement token verification + staged-effect re-validation as specified.
- Agents and automated writers SHOULD always send `If-Match`; interactive humans MAY (last-write-wins is
  tolerated for a human's own immediate edit but discouraged elsewhere).
- One residual is accepted: a human approving a correctly-bound, non-stale effect still owns the
  judgment call (the gate guarantees *integrity and freshness*, not *wisdom*).

## Supersedes / amends

Amends ADR-0026 (per-tool autonomy tiers) by specifying the token + re-validation mechanics it relied on.
Cross-refs ADR-0018 (capability governance), ADR-0011 (consent), `interfaces.md §0/§3`,
`data-model §1.3a/§3.4`, `crm.yaml` (`ApprovalToken`, `RowVersion`, `If-Match`, `VersionConflict`).
