---
status: planned
module: backend/internal/modules/overlay · frontend/src/features/overlay
derives-from:
  - specs/spec/narrative/03e-overlay-augmentation.md#1-two-product-modes-one-codebase @ 5a0b29c
  - specs/spec/narrative/03e-overlay-augmentation.md#2-architecture-of-overlay-mode @ 5a0b29c
  - specs/spec/narrative/03e-overlay-augmentation.md#3-the-hard-realities-be-honest @ 5a0b29c
  - specs/spec/narrative/03e-overlay-augmentation.md#62-machine-verifiable-acceptance-criteria @ 5a0b29c
  - specs/spec/product/epics/E18-augmentation-hubspot.md#shared-overlay-substrate-built-once-in-e18-e19e20-reuse @ 5a0b29c
  - specs/spec/product/epics/E18-augmentation-hubspot.md#hubspot-specific-stories @ 5a0b29c
  - specs/spec/product/epics/E19-augmentation-salesforce.md#s-e194--map-the-salesforce-schema-and-re-enforce-its-sharingvisibility-the-hard-compliance-critical-one @ 5a0b29c
  - specs/spec/product/epics/E20-augmentation-dynamics.md#s-e204--map-the-dataverse-schema-metadata-driven-onto-the-clean-mirror @ 5a0b29c
  - specs/spec/decisions/ADR-0044-overlay-visibility-snapshot.md @ 5a0b29c
  - specs/spec/contract/data-model.md#12-deferred-tables-later-phases--stubs-only-no-ddl @ 5a0b29c
  - specs/spec/contract/events.md#510-overlay--augmentation-mirror-overlay-mode-only--narrative03e-e18e19e20 @ 5a0b29c
  - specs/spec/contract/crm.yaml @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#hubspot-connecthtml--connect-hubspot-overlay-implements-s-e185 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#overlay-budgethtml--incumbent-api-budget-status-implements-s-e184 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#mode-fliphtml--overlay--sor-mode-flip-implements-s-e1810 @ 5a0b29c
  - specs/spec/product/build-backlog/E18.md @ 5a0b29c
---
# Overlay augmentation — run Margince's AI on top of the CRM they refuse to leave

> The overlay binding of the datasource seam and everything behind it: a cached mirror of an
> incumbent CRM (HubSpot in V1; Salesforce and Dynamics fast-follow), incumbent-first write-back,
> incumbent-wins conflicts, untrusted-by-default incumbent data, a degrade-never-starve budget
> posture, complete purge on disconnect, and the flip that later makes Margince the system of
> record. The single promise: the incumbent stays canonical, and everything we add on top is
> governed, honest about staleness, and removable.

## What it's for

An enterprise with a sunk incumbent CRM will not rip it out to try Margince's AI — but its
budget holder will buy auto-capture, BYO-agent orchestration, and governed approval as a layer
*on top of* the system they keep. Overlay mode is that layer made a first-class product mode
rather than a services one-off: one codebase, two system-of-record modes, chosen per workspace.
The callers are every surface above the datasource seam — the AI layers, capture, the agents
layer, the React UI — none of which know or care which system of record sits beneath them. The
seam itself (the port, its native binding, the mode invariant) is the
[datasource](datasource.md) chapter's; this chapter owns the overlay binding and the incumbent
adapters behind it, the mirror and its lifecycle, the write-back engine, overlay security, the
budget-consuming degrade behaviour, the overlay lifecycle surface and screens, and the
overlay-to-native flip. It is also the strategic on-ramp: prove the AI value with zero migration
risk, then flip the same workspace to native mode when the customer chooses.

## Principles it serves

- **P13 — augment, don't demand rip-and-replace.** This chapter is P13's implementation: the
  enterprise wedge that leaves the incumbent authoritative and sells the AI layers on top.
- **P5 / P6 / P12 — capture, BYO agents, governance — over *their* data.** Auto-capture writes
  activities back into the incumbent so their record gets more complete; the customer's own
  agent acts on incumbent data under Passport scopes, autonomy tiers, and full audit — the
  governed-agent answer the incumbent does not provide.
- **P2 / P3 — customization is code.** The adapter mapping from an incumbent's objects and
  associations onto the clean mirror is declared in test-guarded source, never a runtime
  mapping screen; a bespoke incumbent field becomes a real mapped column via a code change.
- **P1 — bounded choices, not a config engine.** Mode is a per-workspace deployment choice
  against exactly one named incumbent (DS-PARAM-1); only the flip changes it.
- **ADR-0018 — bounded-capability guarantee.** Overlay never forks the tool or data surface:
  every capability either behaves identically or returns a declared, tested
  unsupported-by-this-system-of-record result — never a silent break.
- **ADR-0036 — one concurrency vocabulary.** The seam's optional version token maps to the
  incumbent's change marker in overlay mode, so a stale write is the same matchable
  version-skew refusal in both modes.
- **ADR-0044 — visibility snapshot, fail-closed.** The incumbent's per-user visibility is
  re-enforced from a batched snapshot in the mirror, hiding on doubt, never resolved live per
  read.
- **ADR-0048 — sealed connector secrets.** Incumbent OAuth material lives in the encrypted
  per-tenant vault and is never returned by any API.

Overlay mode also *deliberately weakens* three principles — P11 (clean joins hold only on our
mirror, not the incumbent's canonical tangle), P4 (write-back and force-fresh reads inherit the
incumbent's latency), and P7 (canonical data stays in the incumbent until the flip). These are
accepted, scoped, recoverable trades for enterprise reach — see the honest-degradation section
below; this chapter never pretends they don't exist.

## How it works

**One seam, one named incumbent.** A workspace resolves to native mode or to overlay mode
against exactly one incumbent; the resolution rules and their database-held invariant are the
datasource chapter's (DS-AC-5). Everything above the seam calls only the port — a
merge-blocking dependency lint holds that no module, AI-layer, capture, or UI package imports
an incumbent SDK or another store directly (AC-OV-1, held as DS-AC-1). An agent working an
overlay workspace never learns it is talking to HubSpot; every tool is bounded-equivalent
across the two modes — identical behaviour or a declared, tested unsupported result with a
readable reason (AC-OV-2). The V1 adapter is HubSpot; Salesforce and Dynamics are fast-follow
adapters on the same substrate, and the substrate is deliberately not HubSpot-shaped where the
incumbents differ.

**Connecting an incumbent is a governed, line-of-business action.** The connect flow uses a
public OAuth app with least-privilege scopes shown to the admin before consent, EU-region
routing for EU customers, refresh-token rotation, and one-action revocation. Tokens seal into
the per-tenant connector-secret vault and are referenced, never echoed. The connect screen
discloses **both residency boundaries honestly**: the canonical system of record sits in the
incumbent's region, and the derived mirror replica plus incumbent-derived embeddings and
context-graph nodes sit in *our* store on the operator's region — a second copy a DPO will ask
about, so it is stated, not implied. A scope the app cannot hold (HubSpot custom-schema write)
surfaces as the declared unsupported result, never a silent drop.

**The mirror is a cache, kept honest about freshness.** On connect, a checkpointed, resumable
backfill hydrates a derived read-model of the incumbent's objects — the mirror — inside a
distinct schema namespace so the native core is untouched. Live changes arrive over the
incumbent's webhook or change stream, ingested idempotently, ordered by occurrence time, and
signature-verified, with permanent failures dead-lettered. Because HubSpot's webhooks are
best-effort (no replay window), a reconciliation poller sweeps modification timestamps to heal
gaps, staying within the incumbent's search budget (OVA-PARAM-1). Reads — UI and agent alike —
serve from the mirror and meet the native-mode read budgets; every overlay surface shows a
last-synced affordance, pending-sync optimism, and a staleness warning, and agent tool
responses carry mirror freshness as metadata (AC-OV-3).

**Write-back is canonical; the mirror never lies about authority.** A write goes to the
incumbent *first*; only on the incumbent's acknowledgement is the mirror updated and marked
authoritative. In flight, the UI may show the write optimistically tagged pending-sync, and
the port's honest-authority flag stays false until the ack (DS-AC-7 is the port-level half).
The shared write-back engine carries **two concurrency paths behind one seam**, by design:
HubSpot has no concurrency primitive, so its branch does a stored-baseline drift check — read,
compare the stored change timestamp, patch changed fields only; Salesforce and Dynamics expose
a real conditional-write primitive, so their branch uses it natively. Both map to the one
version-skew refusal of ADR-0036. A rejected write is returned to the caller — a human gets a
"record changed, review" prompt; an agent write becomes a confirm-first re-resolution — and a
write-rejected event is emitted; the mirror is never marked authoritative ahead of the
incumbent (AC-OV-4). Because HubSpot's webhooks carry no trustworthy source discriminator, the
echo of our own write is suppressed through an our-write ledger — a bounded-window,
hash-keyed record of what we just wrote (OVA-PARAM-3, OVA-PARAM-4); an inbound event matching
an open ledger entry is dropped, anything else is ingested as a genuine external change, and a
hash collision halts the mirror rather than mis-suppressing silently.

**Conflicts resolve incumbent-wins, and correctness can buy freshness.** When reconciliation
finds the mirror diverged from a fresh incumbent read, the incumbent value overwrites the
mirror and a conflict event is emitted for observability — we never clobber the incumbent with
stale mirror state (AC-OV-8). A confirm-first high-value action (advancing a deal to won,
anything touching money) does a synchronous force-fresh read-through to the incumbent before
acting, paying latency exactly where correctness matters most.

**Incumbent data is untrusted, and write-back is egress.** Every record read from the
incumbent is tagged T2 — captured/external, attacker-influenceable — exactly as the
[threat-model](../quality/threat-model.md) chapter's tier doctrine demands; the incumbent is
not a trusted source just because it is "the CRM". The T2 taint **rides into derivatives**:
mirror row, embeddings, context-graph nodes, and tool output all carry it, because an
embedding of injected content is still an injection vector. Write-back is itself an
external-egress channel — a manipulated agent could exfiltrate by writing into an incumbent
field another integration ships outward — so incumbent write-back is confirm-first and
content-aware like any external send, and the session volume backstops apply to incumbent
reads and writes exactly as to ours (AC-OV-5). Before overlay GA, the injection red-team gate
and the mass-read step-up are re-run against the adapter, exercising the derived
retrieval path and not only the raw read (AC-OV-6).

**The budget guardrail degrades, never starves.** The incumbent's API allocation is shared
with integrations we cannot see; the per-incumbent consumption meter — its windows, caps, and
warn/shed thresholds — is owned by the [overlay-budget](overlay-budget.md) chapter. This
chapter owns the *consumers*: when the meter answers shed, force-fresh reads degrade to
mirror-with-staleness-warning instead of spending the customer's quota, bulk capture
write-back is shed before interactive writes on the fair-queued bulk path, and a
budget-degraded event records the guardrail firing (AC-OV-7 — the consuming acceptance the
budget chapter defers here).

**Per-user visibility is re-enforced from a snapshot, fail-closed.** A mirrored row the
acting user could not see in the incumbent must never surface through us, and a row whose
incumbent visibility we cannot determine is hidden, not leaked. Per ADR-0044, a batched
per-user visibility snapshot is materialized into the mirror and joined on every read — never
a live per-record visibility query; it is governed by a per-object-class freshness SLO
(OVA-PARAM-5, OVA-PARAM-6) with a hard fail-closed floor past twice the SLO or on any
indeterminate row (OVA-PARAM-7), and the batched refresh is metered against the budget
guardrail with a reserved minimum slice so it degrades to *over-hiding*, never to leaking
(AC-OV-11). The full sharing-model projection is the hard part of the Salesforce and Dynamics
adapters; the mechanism is fixed in the substrate now so it cannot be bolted on later.

**Disconnect purges the copy.** Revoking the connection does more than block the next call:
it triggers mirror teardown — the replica and every incumbent-derived embedding and
context-graph node are purged within the declared retention window (OVA-PARAM-2), because the
copy must not outlive the connection that justified holding it. Our own augmentation that is
not a copy of incumbent data — the audit of our agents' actions, approval records — is
retained per its own policy and stays exportable. Right-to-erasure reaches the mirror and its
derivatives exactly as it reaches native data (OVA-AC-1).

**The flip makes Margince canonical when the customer chooses.** The overlay-to-native flip is
the [import-export-migration](import-export-migration.md) importer run against the incumbent
with cutover semantics owned here: a preflight gate requires a force-fresh sync with the
mirror frozen, all reconciliation conflicts cleared, an honest-scope export available, and a
parity dry-run; the flip itself is a confirm-first, typed-phrase action that imports with
counts and relationships preserved, carries our augmentation over, re-tags incumbent-derived
context from T2 to first-party, detaches write-back, and flips the workspace mode
(AC-OV-9, AC-OV-10). Before the flip, an export bundle contains our augmentation plus a mirror
snapshot and documents that canonical data resides in the incumbent — P7 stays honestly
partial until the customer flips.

**The honest degradations, stated plainly.** P11 is partial: our mirror is clean, but the
system of record underneath is not and is canonical — clean joins and reporting hold only on
analyses we run on the mirror, any number we show is only as fresh as the last sync, and the
customer's own incumbent reports may disagree with ours. P4 is partial: read budgets hold via
the mirror, but write-back and force-fresh reads inherit the incumbent's latency and are
recorded against a separate overlay performance addendum, excluded from the native budgets —
we do not pretend an incumbent write is fast (its numbers are still unset, OVA-PARAM-9). P7 is
partial: the canonical data stays in the incumbent — the customer keeps owning it *via the
incumbent*, our export covers our augmentation plus a mirror snapshot, and full P7 arrives
only with the flip. And the V1 build order is engineering-led, not beachhead-matched: HubSpot
first because it is the cheapest seam-proving adapter reusing the migration mapping, then
Salesforce, then Dynamics — even though the regulated Microsoft-first beachhead runs Dynamics.
Augmentation-on-Dynamics is not a V1 capability and is not marketed as one.

## What's configurable

- **Workspace mode and incumbent** — the per-workspace, deploy-time choice; owned by the
  datasource chapter (DS-PARAM-1). Only the flip changes it.
- **Incumbent budget config** — published ceiling, cap, warn/shed fractions, per incumbent in
  the static config source; owned by the overlay-budget chapter (OVB-PARAM-1 through
  OVB-PARAM-4). This chapter's consumers only read the meter's answer.
- **Reconciliation search budget** — the per-incumbent search-rate bound the poller and
  backfill stay within; HubSpot's is pinned (OVA-PARAM-1).
- **Mirror teardown window** — the declared retention window for the disconnect purge
  (OVA-PARAM-2).
- **Echo-suppression ledger window** — the bounded window an our-write ledger entry stays
  open, configurable, with a pinned hash function (OVA-PARAM-3, OVA-PARAM-4).
- **Visibility freshness SLO** — per object class, two default tiers plus the fail-closed
  multiple (OVA-PARAM-5 through OVA-PARAM-7); ADR-0044 fixes the mechanism, the values are
  spike-tuned.
- **Per-object-class mirror freshness SLOs and the overlay perf addendum budgets** — honestly
  unset; the corpus leaves both open until adapter spikes calibrate them (OVA-PARAM-8,
  OVA-PARAM-9). None of these are runtime-config surfaces; all are deploy-time or config-source
  values.

## Guarantees (enforced)

- **Nothing above the seam knows the incumbent exists** — no incumbent SDK or store import
  above the port; held by the merge-blocking architecture lint (AC-OV-1, via DS-AC-1).
- **No silent capability break** — every tool is bounded-equivalent across modes: identical, or
  a declared, tested unsupported result matching the adapter's published capability manifest
  (AC-OV-2; ADR-0018).
- **Performance honesty** — mirror reads meet the native read budgets; writes and force-fresh
  reads are classified into the overlay perf addendum, never a false pass (AC-OV-3).
- **The mirror never outruns the incumbent** — authoritative only after incumbent ack; a
  version-skew write is rejected to the caller, never applied (AC-OV-4; ADR-0036).
- **Untrusted end-to-end, including derivatives** — T2 labels ride from incumbent through
  mirror, embeddings, and graph nodes into tool output; T2-tainted sensitive egress, including
  write-back, is confirm-first (AC-OV-5); the injection gate passes against the adapter before
  GA (AC-OV-6).
- **Degrade, never starve** — past the shed threshold, force-fresh falls back to
  mirror-with-staleness-warning and no further live calls are spent (AC-OV-7, consuming
  OVB-AC-2).
- **Incumbent-wins, observably** — a diverged mirror row is overwritten from the fresh
  incumbent read and the conflict event is emitted, never the reverse (AC-OV-8).
- **Honest export, faithful flip** — the overlay export contains our augmentation plus a
  mirror snapshot and documents where canonical data lives (AC-OV-9); the flip preserves
  counts and relationships (AC-OV-10).
- **Fail-closed visibility** — a row the user could not see in the incumbent, or whose
  visibility is indeterminate or past twice the SLO, is hidden, not leaked (AC-OV-11;
  ADR-0044; OVA-AC-2).
- **The copy dies with the connection** — after disconnect, no incumbent-derived data remains
  queryable past the teardown window; our own audit survives (OVA-AC-1).
- **No sync loops, no swallowed changes** — our write's webhook echo is suppressed; a genuine
  concurrent external change is ingested; a ledger hash collision halts the mirror rather than
  guessing (OVA-AC-3).

## Acceptance

Done, for overlay mode, means an enterprise admin can connect their incumbent in minutes with
scopes and both residency boundaries disclosed before consent; a rep and their agent then work
records that are visibly fresh-or-honest (last-synced, pending-sync, staleness warnings on
every overlay surface); a write that raced an incumbent-side change comes back as "record
changed, review", never a lost update; the budget screen shows real measured consumption with
unknowable headroom marked unknown, and the degrade ladder engaging is user-observable; a row
the user couldn't see in the incumbent never appears; disconnect verifiably purges the copy;
and the flip runs only through its preflight gates and lands with parity the customer can
read. Degraded and denied states are honest surfaces, not failures to hide: unsupported
capabilities are declared with readable reasons, budget shed shows staleness rather than
silence, and a stale visibility snapshot hides rows rather than leaking them. The testable
form of every claim lives in the Acceptance appendix; the cross-cutting floor (standard screen
states, performance budgets, release gates) is inherited from the acceptance-standards chapter
and not restated.

## Out of scope

- **The port itself** — the datasource seam, its native binding, mode resolution, and the
  provenance/version-token rules are [datasource](datasource.md)'s (the DS-AC series).
- **The consumption meter** — windows, caps, thresholds, fail-fast config, and the
  never-fabricate-headroom rule are [overlay-budget](overlay-budget.md)'s (the OVB series).
  The split on the budget *screen*: the meter mechanics it displays are pinned there; the
  screen and its acceptance rows are pinned here.
- **The importer** — the migration engine the flip reuses, its dry-run/approve lifecycle and
  parity gates, is [import-export-migration](import-export-migration.md)'s; this chapter owns
  only the flip's preflight and cutover semantics.
- **Mirror event definitions** — payloads and semantics live in the central event catalog
  ([event-bus](../architecture/event-bus.md), EVT-SEM-13); this chapter only emits them.
- **Record-to-conversation link verbs** — the datasource chapter's out-of-scope pointer names
  this chapter, but their wire pins live at [dispact-integration](dispact-integration.md)
  (DISP-WIRE-2); what is ours is only their overlay-binding behaviour under the
  bounded-equivalence rule (AC-OV-2).
- **Salesforce and Dynamics adapter specifics** — fast-follow scope-OUT rows (S-E19.1–.6,
  S-E20.1–.6 in the scope chapter); this chapter pins them only at substrate level: the dual
  concurrency contract, the visibility snapshot, and AC-OV-11 the substrate must already
  support.
- **The Dispact-on-Teams/Slack overlay analogue** — named by the corpus as the same pattern,
  specified in the Dispact product spec, not here.

## Where it lives

Planned at `backend/internal/modules/overlay` (the adapters, mirror engine, write-back, and
lifecycle jobs, all behind the datasource port) and `frontend/src/features/overlay` (the
connect, budget-status, and mode-flip screens); the meter it charges lives in the platform
layer. Read next:
[datasource](datasource.md) for the seam, [overlay-budget](overlay-budget.md) for the meter,
[import-export-migration](import-export-migration.md) for the engine the flip reuses, and the
[threat-model](../quality/threat-model.md) for the tier doctrine the mirror inherits.

## Appendix

### Parameters
Source: specs/spec/narrative/03e-overlay-augmentation.md#23-conflict--staleness-handling @ 5a0b29c; specs/spec/decisions/ADR-0044-overlay-visibility-snapshot.md @ 5a0b29c; specs/spec/product/build-backlog/E18.md#b-e1815--disconnect--mirror-teardown-purge-replica--incumbent-derived-embeddingsgraph-nodes-within-the-retention-window @ 5a0b29c; specs/spec/product/build-backlog/E18.md#b-e1820--our-write-ledger-echo-suppression-drop-the-webhook-echo-of-our-own-write-ingest-genuine-third-party-changes @ 5a0b29c

The budget meter's thresholds are not restated here — they are OVB-PARAM-1..5 in
[overlay-budget](overlay-budget.md); the workspace mode flag is DS-PARAM-1 in
[datasource](datasource.md).

| ID | Name | Value | Meaning |
|---|---|---|---|
| OVA-PARAM-1 | HubSpot search budget | 4 req/s | The incumbent search-rate bound the reconciliation poller and backfill stay within (S-E18.6, B-E18.18); metered as the per-second search window of the overlay-budget meter. |
| OVA-PARAM-2 | Mirror teardown window | ≤ 24 h (the declared retention window) | On disconnect/revoke, the mirror replica + incumbent-derived embeddings/context-graph nodes are purged within this window (B-E18.15; `03e §3.4a`); own agent-action audit/approval records are retained per their own policy. |
| OVA-PARAM-3 | Echo-suppression ledger window | 24 h, configurable | How long an our-write ledger entry stays open; an inbound webhook matching an open entry is suppressed as our echo, anything else is ingested (B-E18.20; `03e §8 Q7` — window validated in the HubSpot spike). |
| OVA-PARAM-4 | Echo-suppression value hash | SHA-256 over the canonicalized row | The ledger key's value-hash component `(object, external_id, property, value-hash, t)`; a detected collision flags and halts the mirror, never silently mis-suppresses (B-E18.20). |
| OVA-PARAM-5 | `OVERLAY_VISIBILITY_SLO` — high-sensitivity classes | 15 min | Visibility-snapshot freshness SLO for deals/opportunities, accounts/orgs, contacts (ADR-0044; value spike-tuned, mechanism fixed). |
| OVA-PARAM-6 | `OVERLAY_VISIBILITY_SLO` — lower-volatility classes | 60 min | Visibility-snapshot freshness SLO for other object classes (ADR-0044; value spike-tuned). |
| OVA-PARAM-7 | Visibility fail-closed boundary | 2 × SLO, or any indeterminate row | Past this staleness, or with no snapshot entry, affected rows are hidden, not shown — hide-on-doubt is the non-negotiable floor (ADR-0044; AC-OV-11). |
| OVA-PARAM-8 | Per-object-class mirror freshness SLOs | **unset — open** | The webhook-vs-poll cutover and staleness budgets per object class are uncalibrated (`03e §8` open Q2); arrival-spec at adapter-spike time. |
| OVA-PARAM-9 | Overlay perf addendum budgets | **unset — open** | The explicit write-back / force-fresh latency budgets (separate from the native read budgets) are unset (`03e §8` open Q4); AC-OV-3 classifies into the addendum bucket regardless. |

### Schema
Source: specs/spec/contract/data-model.md#12-deferred-tables-later-phases--stubs-only-no-ddl @ 5a0b29c; specs/spec/decisions/ADR-0044-overlay-visibility-snapshot.md @ 5a0b29c; specs/spec/product/build-backlog/E18.md#b-e1820--our-write-ledger-echo-suppression-drop-the-webhook-echo-of-our-own-write-ingest-genuine-third-party-changes @ 5a0b29c

**Arrival-spec, not DDL.** The corpus defers the overlay cluster as stubs with **no DDL**
(the skeleton's deferred-table index carries it as [[data-model#DM-DEF-6]]); the shapes below
are the corpus-pinned arrival shapes the WP-entry decomposition must land, in a **distinct
schema namespace** so the native core is untouched. The `connector_secret` vault the
connection references is already shipped and owned by the
[data-model](../architecture/data-model.md) chapter (ADR-0048) — cited, not restated. The
workspace mode flag's invariant (overlay-iff-incumbent, held at the database) is pinned at
[datasource](datasource.md) DS-AC-5/DS-PARAM-1.

| ID | Object (arrival-spec) | Shape as pinned by the corpus |
|---|---|---|
| OVA-DDL-1 | `overlay_mirror` (schema namespace; one derived read-model table per object class) | Incumbent `external_id`; the projected clean columns per the code-declared adapter mapping; `last_synced_at`; sync-state ∈ `fresh` \| `pending_sync` \| `stale`; the **lost-update baseline** (HubSpot `updatedAt` snapshot, or SF/Dynamics `ETag`/`If-Match` token) used by the write-back drift-check. Derived and disposable — rebuildable from the incumbent by backfill. |
| OVA-DDL-2 | `overlay_association` | Directional edge table for incumbent associations that don't map to a naive FK: `(from_type, from_id, to_type, to_id, type_id, category, label, direction)` — carries HubSpot associations-v4 semantics (account-specific numeric `associationTypeId`, labeled/paired, direction) rather than losing them. Surfaces through the normal relationships endpoint, not a new path. |
| OVA-DDL-3 | `incumbent_connection` | The overlay's own service identity to the incumbent (distinct from the BYO-agent `agent_connection`): `(workspace_id, incumbent, region, oauth_token_ref → connector_secret, scopes[], status, connected_at, revoked_at)`. Revocation flips `status` and triggers mirror teardown (OVA-PARAM-2). |
| OVA-DDL-4 | workspace SoR-mode flag | `workspace.sor_mode ∈ {native, overlay}` + `workspace.incumbent?` — exactly one mode, overlay iff a named incumbent; fixed at deploy; changed only by the flip. The invariant and mode resolution are pinned at [datasource](datasource.md) (DS-AC-5); this row records the cluster's arrival shape. |
| OVA-DDL-5 | `overlay_mirror.mirror_visibility` | The ADR-0044 deny-projection: `(workspace_id, incumbent, mirror_user_id, object_class, mirror_object_id) → can_see boolean, snapshot_at`. Joined on every overlay read (UI and MCP); `can_see = false` or **no entry** ⇒ row not returned; refreshed in bulk per user/class, never live-per-read. |
| OVA-DDL-6 | our-write ledger | Echo-suppression ledger keyed `(object, external_id, property, value-hash, t)` with the bounded open window (OVA-PARAM-3) and SHA-256 value-hash (OVA-PARAM-4). Collision ⇒ flag + halt the mirror. |

### Wire
Source: specs/spec/contract/crm.yaml (overlay / augmentation management block — comment-only, "B-E18.* trace here") @ 5a0b29c

**Honest contract-coverage finding (OVA-GAP-1, contract-extension item):** at pin time the
overlay lifecycle surface exists in the contract **only as a comment block** — the paths,
schemas, and operationIds below are recorded as being-authored, not parseable operations;
contract-codegen and the tool-annotation lint cannot yet emit or verify them. OperationIds
must be minted by the contract extension before any ticket cites one as existing. Two
invariants the block itself pins: **overlay does not fork the data API** (mirrored
people/orgs/deals/activities/relationships read and write through the same operations as
native mode, per AC-OV-2/ADR-0018), and the lifecycle ops below exist **only** in overlay
mode — in native mode each returns the not-overlay sentinel. The connection resource is
singular (one incumbent per workspace).

| ID | Element (planned path) | Behavior pinned |
|---|---|---|
| OVA-WIRE-1 | read connection (`GET /overlay/connection`) | Current incumbent binding + status `{incumbent, region, status ∈ active\|revoked\|error, connected_at, scopes[]}`. 🟢 read; MCP-exposed as an overlay-connection record read, tier green. |
| OVA-WIRE-2 | connect incumbent (`POST /overlay/connection`) | OAuth connect (HubSpot public app in V1). 🟡 — establishes egress plus ingest of the customer's entire CRM: requires-approval error + ADR-0036 token, Idempotency-Key; EU customers route to the incumbent's EU region with the dual-residency disclosure; conflict sentinel if a binding exists. MCP tier yellow. |
| OVA-WIRE-3 | disconnect (`DELETE /overlay/connection`) | Revoke token + trigger mirror teardown (purge replica and incumbent-derived embeddings/graph nodes within OVA-PARAM-2). 🟡, requires-approval; accepted-with-job-ref response. MCP tier yellow. |
| OVA-WIRE-4 | sync status (`GET /overlay/sync-status`) | Mirror freshness per object class `[{object, last_synced_at, state ∈ fresh\|pending_sync\|stale, backfill_progress}]` — backs the three overlay UI affordances every overlay screen must show. 🟢 read. |
| OVA-WIRE-5 | reconcile (`POST /overlay/reconcile`) | Trigger the reconciliation sweep; heals gaps within the incumbent search budget (OVA-PARAM-1), incumbent-wins on divergence emitting the conflict event (AC-OV-8). 🟢 (read-heavy, budget-metered); accepted-with-job-ref. |
| OVA-WIRE-6 | budget status (`GET /overlay/budget`) | Per-incumbent consumption `{window, consumed, limit, band, policy}` — the read surface over the [overlay-budget](overlay-budget.md) meter (which owns the numbers); surfaces the AC-OV-7 degrade so UI/agents warn instead of starving the quota. 🟢 read. |
| OVA-WIRE-7 | flip preflight (`POST /overlay/flip:preflight`) | The gate: force-fresh sync + conflict reconciliation + honest-scope export check → `{ready, blocking[], unresolved_conflicts[]}`. 🟢. |
| OVA-WIRE-8 | flip execute (`POST /overlay/flip`) | Runs the importer against the mirror with cutover semantics: preserve counts/relationships, carry over our augmentation, re-tag incumbent-derived nodes T2→T1, detach write-back, flip the workspace mode to native (AC-OV-9/10). 🟡 (irreversible mode change), requires-approval + token; accepted-with-migration-job-ref. Blocked with the flip-blocked sentinel while preflight is unsatisfied. |
| OVA-WIRE-9 | error sentinels | `404 mode_not_overlay` (lifecycle op in native mode) · `409 incumbent_already_connected` · `422 unsupported_by_sor` (declared bounded-capability gap, AC-OV-2) · `409 overlay_flip_blocked` · `503 incumbent_budget_exhausted` (degrade-don't-starve, AC-OV-7; the api-conventions budget-exhausted error is the surfaced form). All RFC-7807 problems. |

### Events
Source: specs/spec/contract/events.md#510-overlay--augmentation-mirror-overlay-mode-only--narrative03e-e18e19e20 @ 5a0b29c

The three mirror events are **defined in the central event catalog** — pinned in the skeleton
at [event-bus](../architecture/event-bus.md) (the deferred `mirror.*` rows and semantic rule
[[event-bus#EVT-SEM-13]]) — and only *emitted* by this chapter's adapter, below the datasource
seam, in overlay mode only. Cited, not redefined:

| ID | Event (catalog home: event-bus) | This chapter's emit point |
|---|---|---|
| OVA-EVT-1 | `mirror.conflict` | Reconciliation overwrites a diverged mirror row from a fresh incumbent read — incumbent-wins, never the reverse (AC-OV-8). |
| OVA-EVT-2 | `mirror.write_rejected` | A write-back fails the incumbent's version/precondition check or scope; carries the lost-update rejection back to the caller (AC-OV-4). |
| OVA-EVT-3 | `mirror.budget_degraded` | The budget guardrail fires: a force-fresh read degrades to mirror-with-staleness (AC-OV-7). |

### Acceptance
Source: specs/spec/narrative/03e-overlay-augmentation.md#62-machine-verifiable-acceptance-criteria @ 5a0b29c; specs/spec/product/epics/E18-augmentation-hubspot.md @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#hubspot-connecthtml--connect-hubspot-overlay-implements-s-e185 @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#overlay-budgethtml--incumbent-api-budget-status-implements-s-e184 @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#mode-fliphtml--overlay--sor-mode-flip-implements-s-e1810 @ 5a0b29c; specs/spec/decisions/ADR-0044-overlay-visibility-snapshot.md @ 5a0b29c; specs/spec/product/build-backlog/E18.md @ 5a0b29c

#### The AC-OV series (corpus IDs, preserved verbatim — the chapter's acceptance backbone)

This is the AC-OV series' single home (the datasource and overlay-budget chapters cite into
it). Corpus text verbatim, including the machine-verifiable markers.

| ID | Criterion | How verified (deterministic) |
|---|---|---|
| `[MV]` AC-OV-1 | The **three AI layers and UI** call only the SoR Provider interface — no direct incumbent-API or direct `crm-core` call exists above the seam. | Static-architecture/dependency lint: assert no import of incumbent SDK or `crm-core` from L1/L2/L3/UI packages; only the adapter packages may. |
| `[MV]` AC-OV-2 | The **same Layer-1 MCP tool surface** (`03b`) is **bounded-equivalent** across the HubSpot adapter and the SoR-mode core (same signatures, scopes, 🟢/🟡 tiers): every tool either passes identically **or** returns a **declared, tested `unsupported_by_sor` result** per the ADR-0018 bounded-capability guarantee — never a silent break. (Some incumbent capabilities genuinely don't exist: HubSpot custom-schema write may be private-app-only, S-E18.5; there is no `run_report` analogue.) | Run the `crm-agents` contract suite twice (SoR-mode, Overlay-mode); assert each tool is in {identical-pass, declared-`unsupported_by_sor`}, and the unsupported set matches the adapter's published capability manifest (ADR-0018) — **no undeclared failures**. |
| `[MV]` AC-OV-3 | A mirror-served read meets the SoR-mode read budgets (`03` §3.5); a **write-back** and a **force-fresh read** are recorded against the **overlay perf addendum**, not the SoR budgets (no false pass). | Perf harness with a mocked incumbent; assert mirror-read p95 < budget and write/force-fresh classified into the addendum bucket. |
| `[MV]` AC-OV-4 | A write that fails incumbent version-check is **rejected to the caller** (not applied to the mirror as authoritative); the mirror is never marked authoritative ahead of incumbent ack. | Simulate version skew on the mocked incumbent; assert reject + mirror row stays `pending_sync`/non-authoritative. |
| `[MV]` AC-OV-5 | All records read from the incumbent are tagged **T2** in the mirror and in MCP tool output; egress of T2+sensitive fields (incl. **write-back**) is 🟡-gated. | `05` tier-leak test re-run on the adapter: assert T2 label end-to-end and write-back 🟡 gate. |
| `[MV]` AC-OV-6 | The **`05` injection red-team gate** and **MCP-SESS-READS step-up** (`api-rate-limits-and-abuse.md` AC-MCP-1) **pass against the overlay adapter** before overlay GA. | Re-run the `05` injection probe + AC-MCP-1 with the SoR bound to the (mocked) incumbent. |
| `[MV]` AC-OV-7 | When incumbent-API consumption crosses the budget threshold, force-fresh reads **degrade to mirror-with-staleness-warning** rather than exhausting the customer's allocation. | Metered test double for incumbent quota; drive past threshold; assert degrade + warning + no further live calls. |
| `[MV]` AC-OV-8 | A conflict (mirror vs fresh incumbent read) resolves **incumbent-wins** and emits a `mirror.conflict` event. | Seed divergent values; trigger reconcile; assert mirror overwritten + event on the bus. |
| `[MV]` AC-OV-9 | An export (`features/04` §5) in overlay mode contains **our augmentation** (context, embeddings, audit, capture provenance, mappings) + the mirror snapshot, and **documents** that canonical data resides in the incumbent. | Completeness test against a seeded overlay workspace; assert bundle contents. ⚠ The honest-scope manifest clause is a **manifest-presence assertion** (a documented string is present), the *softest* check in this set — it verifies the disclosure exists, not that a human acts on it. |
| `[MV]` AC-OV-10 | A workspace can be **flipped Overlay-mode → SoR-mode**, reusing the `features/04` §5 HubSpot importer, with record counts/relationships preserved. | Round-trip test: overlay workspace → migrate → SoR-mode instance; assert parity (mirrors `features/04` "leave in an afternoon" gate). |
| `[MV]` AC-OV-11 | **(Salesforce/Dynamics, S-E19.4/S-E20.4)** A mirrored row the acting user **could not see in the incumbent** is **not** shown or acted on through us; a row whose incumbent visibility we **cannot determine** is **hidden, not leaked** (fail-closed). Incumbent sharing/visibility (SF: OWD, role hierarchy, sharing/restriction rules, teams, territories, manual shares; Dataverse: business units, security roles, sharing) is **re-enforced**, not bypassed. | Seed an incumbent with rows the test user cannot see + rows of indeterminate visibility; assert neither surfaces in UI/MCP output. **Visibility is resolved without exhausting the API budget** (per-user visibility is cached/batched, AC-OV-7) — see §8 open Q 6 on the visibility-vs-budget tension. |

Corpus note carried with the series: these are deterministic (dependency lint, dual-run
contract suites, perf bucketing, version-skew simulation, trust-tier assertions, metered quota
doubles, bus events, export-completeness) — genuine CI/integration gates, not model evals.
AC-OV-11's mechanism is resolved by ADR-0044 (the batched visibility snapshot, OVA-DDL-5,
OVA-PARAM-5..7); its SLO values are spike-tuned.

#### Chapter-minted pins (behaviour the AC-OV series does not itself cover)

| ID | Given/When/Then | Verification |
|---|---|---|
| OVA-AC-1 | `[MV]` Given a connected overlay workspace, when the connection is disconnected/revoked, then the next incumbent call is blocked **and** mirror teardown purges the mirror replica plus every incumbent-derived embedding and context-graph node within the teardown window (OVA-PARAM-2) — no incumbent-derived data remains queryable after the window — while own agent-action audit and approval records are retained and stay exportable. | Backend integration lane, end-to-end teardown test (B-E18.15): disconnect a seeded overlay workspace; assert blocked call, empty mirror/derivatives after the window, audit intact. |
| OVA-AC-2 | `[MV]` Given the visibility snapshot (OVA-DDL-5), when a user/class snapshot ages past 2 × `OVERLAY_VISIBILITY_SLO` (OVA-PARAM-7) or a queried row has no snapshot entry, then the affected rows are hidden from UI and MCP output; and when the budget guardrail is degraded, then the batched refresh extends staleness fail-closed while retaining its reserved minimum budget slice — over-hiding is the worst failure mode, never leaking. | Freshness-SLO test against the `2 × SLO` boundary + indeterminate-row case (ADR-0044 consequences); budget-degraded refresh test asserting the reserved slice and continued fail-closed behaviour. |
| OVA-AC-3 | `[MV]` Given a write-back that produces a webhook echo, when the echo arrives within the open ledger window (OVA-PARAM-3), then it is suppressed (no sync loop); when a non-matching webhook arrives (a genuine concurrent third-party/UI change), then it is ingested, not dropped; and when a value-hash collision (OVA-PARAM-4) is detected, then the mirror is flagged and halted, never silently mis-suppressed. | Backend integration lane (B-E18.20): echo-drop test, non-echo-ingest test, window-boundary test, and a SHA-256 collision case asserting flag + halt. |

#### Story roll-up (S-E18.1–.11, condensed; this chapter is their primary home per the scope chapter)

Nine V1-Must plus two V1-WOW ([[scope]] row E18); E19/E20's twelve adapter stories are
fast-follow scope-OUT rows cited at substrate level only.

| ID | Story (condensed) | Tier |
|---|---|---|
| S-E18.1 | One codebase, two modes — the system-of-record provider seam; bounded per-workspace choice; AC-OV-1 lint + AC-OV-2 bounded equivalence. | V1-Must |
| S-E18.2 | Cached read-model + write-back — mirror-served fast reads (AC-OV-3), incumbent-first canonical writes with pending-sync optimism (AC-OV-4), **both** concurrency paths behind one seam (baseline drift-check + If-Match/ETag→412), incumbent-wins conflicts (AC-OV-8), force-fresh for 🟡 high-value actions, last-synced affordances. | V1-Must |
| S-E18.3 | Overlay security — incumbent data T2 end-to-end including derivatives (AC-OV-5); write-back governed as egress; injection re-gate against the adapter before GA (AC-OV-6). | V1-Must |
| S-E18.4 | Incumbent-API budget guardrail — first-class budget; degrade to mirror-with-staleness, never starve (AC-OV-7); bulk write-back shed before interactive writes. Screen: overlay-budget. | V1-Must |
| S-E18.5 | Connect HubSpot — public OAuth app, least-privilege scopes shown, EU (Frankfurt) routing, per-tenant vault with rotation, one-action revoke **triggering mirror teardown** (OVA-AC-1); dual-residency disclosure; scope gaps surface as `unsupported_by_sor`. Screen: hubspot-connect. | V1-Must |
| S-E18.6 | Keep the mirror fresh — checkpointed/resumable backfill; webhooks v3 ingested idempotently, ordered by `occurredAt`, signature-verified, 4xx dead-lettered; reconciliation poller on `hs_lastmodifieddate` heals gaps within the 4 req/s search budget (OVA-PARAM-1). | V1-Must |
| S-E18.7 | Write back without losing updates — field-level PATCH / batch upsert keyed on a unique property; stored-`updatedAt` baseline drift-check (HubSpot has no version primitive); our-write-ledger echo suppression (OVA-AC-3). | V1-Must |
| S-E18.8 | Map HubSpot objects and associations onto the clean mirror — code-declared, test-guarded mapping (P2/P3, no runtime mapping UI); associations v4 resolved dynamically into the directional edge table (OVA-DDL-2); honest scope — clean joins hold on the mirror only. | V1-Must |
| S-E18.9 | The value on top — auto-capture writes back into HubSpot and indexes T2-tagged into our context graph/embeddings; BYO-agent governed over HubSpot data under Passport scopes, tiers, and full audit. (Promoted from WOW: the GTM promise needs the sellable value, not just plumbing, in V1.) | V1-Must |
| S-E18.10 | Overlay → SoR flip — same workspace flips modes reusing the HubSpot importer with counts/relationships preserved (AC-OV-10); augmentation carries over; pre-flip export is honest-scope (AC-OV-9). Screen: mode-flip. | V1-WOW |
| S-E18.11 | The alternative surface — NL search, reporting, and the fast, beautiful UI clean-joined on the mirror, honestly scoped with `last_synced_at` freshness. | V1-WOW |

#### Screen ACs — hubspot-connect (owned here; corpus text verbatim)

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-hubspot-connect-1 | Given the workspace is not connected, When the screen loads, Then the state pill reads "Not connected", the disconnected view shows the four requested scopes (contacts.read 🟢, schemas.read 🟢, contacts.write 🟡, webhooks 🟢) each tagged read or write, and a "least-privilege" note states no marketing, email-send, or admin scopes are requested. | Screen e2e lane |
| AC-hubspot-connect-2 | Given the disconnected view, When the user reads the residency card, Then it discloses both boundaries honestly — canonical system of record HubSpot (EU · eu1, Frankfurt) and a derived mirror replica plus incumbent-derived embeddings/context-graph nodes on our operator's EU region, with the statement the copy must not outlive the connection. | Screen e2e lane |
| AC-hubspot-connect-3 | Given the user clicks "Authorize with HubSpot", When the OAuth flow starts, Then the view switches to the authorizing progress steps (redirect/authorize and token-exchange shown done, backfill shown active with a spinner), the pill changes to "Connecting…", and an audit row logs the authorize initiation with the scope list and EU/eu1 region. | Screen e2e lane |
| AC-hubspot-connect-4 | Given backfill completes, When the connection finishes, Then the view switches to the connected state, the pill reads "Connected", a green banner says "Overlay live" with reads serving from the EU mirror and writes HubSpot-first 🟡 approval-gated, and the audit log records token storage with refresh-rotation armed and webhooks/poller armed. | Screen e2e lane |
| AC-hubspot-connect-5 | Given the connected state, When the user views the token vault, Then it shows the OAuth public app + app_id, an obfuscated refresh-rotated access token, last-rotated timestamp, and granted scope chips, with "Rotate token now" and "Re-verify scopes" actions available. | Screen e2e lane |
| AC-hubspot-connect-6 | Given the connected state, When the user reads the scope-gap card, Then it surfaces that custom-schema write is unavailable because the public app cannot request the scope, and declares the action would return an `unsupported_by_sor` result with a readable reason (AC-OV-2) rather than a silent drop, linking a capability matrix. | Screen e2e lane |
| AC-hubspot-connect-7 | Given the connected state, When the user clicks "Disconnect & purge mirror" and confirms the dialog, Then the view switches to the revoked/purge-in-progress state, the pill reads "Disconnected", and the screen states tokens are revoked (next call blocked), the mirror replica + embeddings + context-graph nodes are queued for teardown within 24h, while own audit/approval records are kept and exportable. | Screen e2e lane (mechanics: OVA-AC-1) |
| AC-hubspot-connect-8 | Given the connected/disconnected views, When the user inspects the incumbent-selector segment, Then HubSpot is the active V1 option and clicking Salesforce or Dynamics toasts that they are fast-follow (E19/E20) — not in V1 — enforcing exactly one bound incumbent (P1). | Screen e2e lane |

Prototype gaps carried honestly (arrival work, per the corpus states-and-edge-cases note): no
live OAuth error/denied-consent state (user cancels at HubSpot) and no token-refresh-failure or
revocation-failure state are rendered — only the happy paths and the declared scope-gap.

#### Screen ACs — overlay-budget (the **screen** is owned here; the meter mechanics it displays are [overlay-budget](overlay-budget.md)'s OVB series — that split is deliberate; corpus text verbatim)

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-overlay-budget-1 | Given the live state, When the screen loads, Then a consumption meter renders the current search rate against the cap (e.g. "3.2 / 4.0 req/s") with warn (70%) and shed (90%) threshold ticks and a colour-banded fill (ok / warn / shed). | Screen e2e lane (thresholds: OVB-PARAM-1/2; cap: OVA-PARAM-1) |
| AC-overlay-budget-2 | Given the meter is in the warn band (81%), When I click the "warn" simulate-load chip, Then the meter, band label, percentage, and state banner all update to the warn copy describing force-fresh reads degrading to the mirror with a staleness warning (cites AC-OV-7), and the policy ladder highlights the "Degrade" row. | Screen e2e lane |
| AC-overlay-budget-3 | Given any consumption level, When I click the "healthy" (45%), "warn" (81%), or "shed" (96%) chip, Then the banner icon/colour, meter fill class, dot, label, and the highlighted degrade-policy row change to match that band, and a toast confirms the simulated percentage and band. | Screen e2e lane |
| AC-overlay-budget-4 | Given the budget is below the warn threshold, When I click "Request force-fresh", Then the simulator reports the read was served live from HubSpot with "quota_spent: 1 req"; Given the budget is at or above warn, When I do the same, Then it reports degradation to the mirror with "quota_spent: 0", a "last_synced_at" staleness value, and notes the agent receives freshness as tool metadata. | Screen e2e lane (mechanics: AC-OV-7) |
| AC-overlay-budget-5 | Given the live state, When I view "Where the quota goes", Then each Margince consumer (force-fresh reads, reconciliation poller, capture write-back) shows a metered req/s and share bar, while "Other integrations" is shown as "~unknown" with an explicit note that HubSpot exposes no per-app breakdown so the budget stays conservatively below the ceiling. | Screen e2e lane (mechanics: OVB-AC-1/5, OVB-PARAM-5) |
| AC-overlay-budget-6 | Given the live state, When I toggle the Window segmented control between "Per-second" and "Daily", Then the meter unit label switches between "HubSpot Search API · rolling 1s" and "HubSpot REST · rolling 24h" and a toast confirms the active window. | Screen e2e lane |
| AC-overlay-budget-7 | Given the recent-events timeline, When I read a "Degrade engaged" event, Then it quotes its measured trigger values (search_rps, threshold, served=mirror, quota_spent=0), links an audit log, and is tagged AC-OV-7 plus a "measured" confidence marker. | Screen e2e lane |

Screen states pinned with it (corpus states-and-edge-cases, condensed): Loading (skeleton, "we
won't render a budget until it's actually read"), just-connected/empty while backfill
hydrates, meter-unavailable error (safely fallen back to mirror-only reads + paused bulk
write-back), and no-permission (overlay-admin Passport scope required). Unobserved metrics
render the unknown sentinel, never a guessed number.

#### Screen ACs — mode-flip (owned here; corpus text verbatim)

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-mode-flip-1 | Given the workspace runs as an overlay on HubSpot, When the screen loads, Then the header shows an "Overlay mode" pill and a two-node track reading "Today · Overlay on HubSpot" → "After the flip · Margince is the SoR", with HubSpot named as canonical (EU · Frankfurt). | Screen e2e lane |
| AC-mode-flip-2 | Given the force-fresh sync has not run (badge "not run"), When the user clicks "Run sync", Then the row shows a spinner and "running" badge, and on completion the badge reads "fresh", the text reports the frozen snapshot id (snap-2026-06-24T09:14Z), and the gate line "Force-fresh sync run, mirror frozen" turns done. | Screen e2e lane |
| AC-mode-flip-3 | Given one unresolved incumbent-wins reconciliation conflict is shown with its diff (Deal "BÄR Pharma — Packaging QA": HubSpot €212,000 vs stale mirror €177,072), When the user clicks "Accept & clear", Then the badge changes to "resolved", the text confirms €212,000 is the reconciled value and the mirror.conflict event is closed in the audit log, and the gate line clears. | Screen e2e lane |
| AC-mode-flip-4 | Given preflight steps are still incomplete, When the user inspects the flip gate, Then the "Flip to System-of-Record mode" button is disabled and stays disabled until sync is fresh, the conflict is cleared, the owner permission holds, AND the user has typed the exact confirmation phrase "FLIP TO SOR". | Screen e2e lane |
| AC-mode-flip-5 | Given the user types into the confirm field, When the entered text is non-empty but does not equal "FLIP TO SOR", Then the input shows an invalid (bad) state; only the exact phrase enables the flip button. | Screen e2e lane |
| AC-mode-flip-6 | Given all gates are green and the phrase is confirmed, When the user clicks the flip button, Then a progress sequence runs four named steps (freeze mirror & seal snapshot → import 8,604 records with counts pinned → re-tag context graph T2→T1 → detach HubSpot write-back), after which a success card "Now your CRM" appears, the header pill switches to "SoR mode", and the track nodes swap so Margince becomes "now". | Screen e2e lane (mechanics: AC-OV-10, OVA-WIRE-8) |
| AC-mode-flip-7 | Given the migration preview dry-run, When the user reads the parity table, Then Companies (1,284), Contacts (6,902), Deals (418) and Associations (19,330) show "match", while Activities show "18 skipped" with an honest note that the 18 are payload-less HubSpot system workflow entries listed in the import report, not silently dropped. | Screen e2e lane |
| AC-mode-flip-8 | Given the screen states reversibility, When the user reads the gate footer and residency panel, Then it asserts HubSpot is not modified or deleted, the flip is reversible by re-export within the retention window, and that until the flip P7 is partial because canonical data still lives in HubSpot (Frankfurt) per A35. | Screen e2e lane |

Screen states pinned with it (corpus, condensed): per-step pending/running/ok preflight; a
no-permission state (workspace-owner required — a rep sees "You can't run this flip" and a
dimmed, disabled gate); an honest sync-failure state where the flip stays blocked on a stale
mirror (degrade-never-starve message) with retry. Missing in prototype, carried honestly: a
first-load skeleton/loading state.
