# ADR-0044 — Overlay per-user visibility: a batched snapshot in the mirror, fail-closed on a freshness SLO

- **Status:** Accepted (2026-06-25) — resolves `narrative/03e §8` open question Q6 (the visibility-vs-budget tension, AC-OV-11). Default chosen now so `B-E19.10a` is buildable; **re-confirm the SLO values and the incumbent bulk-visibility surface in the Salesforce spike** (the values, not the mechanism, are what the spike tunes).
- **Owners:** Eng + Lars.
- **Composes with:** `narrative/03e` (overlay strategy, the `overlay_mirror` schema, AC-OV-1/7/11), ADR-0039 (record-level visibility model), the incumbent-API budget guardrail (`03e §3.1`), P7 (sovereignty / least-leak), P12 (governed).

## Context

In Overlay-mode (E18/E19/E20) we must **re-enforce the incumbent's per-user visibility** — a mirrored row
the acting user could not see in Salesforce/Dynamics must never surface through us (AC-OV-11). The hard part
(`03e §8 Q6`): per-user visibility is **time-varying** and a generic object mirror can't hold it, while
resolving it **live per read** (SF `UserRecordAccess`, Dataverse `RetrievePrincipalAccess`) would burn the
24h API allocation the budget guardrail (`03e §3.1`) exists to protect. The spec listed three candidate
mechanisms without picking one; `B-E19.10a` ("cached/batched per-user sharing snapshots with a freshness
SLO") cannot be built — nor can its freshness-SLO test be written — until one is chosen.

## Decision

Adopt a **batched per-user visibility snapshot materialized into the mirror**, served from the mirror on
every read, governed by a **per-object-class freshness SLO**, with a **fail-closed** floor.

1. **`mirror_visibility` projection.** In the `overlay_mirror` schema, maintain a compact projection keyed
   `(workspace_id, incumbent, mirror_user_id, object_class, mirror_object_id) → can_see boolean, snapshot_at`.
   Every overlay read (UI and MCP) joins against it; a row with `can_see=false` or **no entry** is **not**
   returned. This is the same widen-both-or-invisible posture as `record_grant` (ADR-0039), inverted to a
   deny-projection.

2. **Batched refresh, never live-per-read.** A scheduled job refreshes the projection in **bulk** per user
   and object class using the incumbent's batch visibility surface (SF: bulk `UserRecordAccess` / shareable
   sharing tables in chunked SOQL; Dataverse: batched `RetrievePrincipalAccess` / security-role + BU
   projection; HubSpot: app-scope/owner projection). No read path ever issues a live per-record visibility
   query against the incumbent.

3. **Freshness SLO (`OVERLAY_VISIBILITY_SLO`, tunable, calibrated in the spike).**
   - High-sensitivity classes (deals/opportunities, accounts/orgs, contacts): **15 minutes**.
   - Lower-volatility classes: **60 minutes**.
   The snapshot carries `snapshot_at`; agent tool responses surface visibility freshness as metadata
   (consistent with `03e §2.3` mirror-freshness metadata).

4. **Fail-closed floor (non-negotiable, the compliance gate).** If a user/class snapshot is **older than
   `2 × SLO`**, or a queried row has **no snapshot entry** (indeterminate), the affected rows are **hidden,
   not shown** — hide-on-doubt. Staleness never silently degrades into leaking.

5. **Budget-bounded refresh.** The batched refresh is itself metered against the `03e §3.1` incumbent-API
   budget guardrail. When the budget is degraded, the refresh **extends staleness and stays fail-closed**
   (rows past `2 × SLO` drop out of view) rather than exhausting the customer's allocation. Visibility
   refresh gets a **reserved minimum slice** of the budget so it is never fully starved.

## Consequences

- `B-E19.10a` is now buildable: it builds the `mirror_visibility` projection + the batched refresher + the
  read-path join; its freshness-SLO test asserts against `OVERLAY_VISIBILITY_SLO` and the `2 × SLO`
  fail-closed boundary. `B-E19.10b` rides the projection for behaviour (re-confirm at WP-entry per its note).
- **Correctness is conservative by construction** — the worst failure mode is *over-hiding* (a visible row
  briefly hidden after a sharing change until the next refresh), never *leaking*. Acceptable and on-thesis (P7).
- Adds an `overlay_mirror.mirror_visibility` table + a scheduled refresher to the overlay substrate (built
  once in E18's shared substrate, consumed by E19/E20).
- The **SLO numbers and the exact incumbent bulk surface remain spike-tuned** (`03e §8` Q1/Q2 calibration);
  the mechanism and the fail-closed contract are fixed by this ADR.
