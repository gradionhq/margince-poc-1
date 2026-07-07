---
status: skeleton
module: backend/internal/platform
derives-from:
  - margince-poc/docs/subsystems/overlay-budget.md @ a11d6c08
  - specs/spec/narrative/03e-overlay-augmentation.md#31-incumbent-api-rate-limits--latency-bottleneck-the-agents @ 5a0b29c
---
# Overlay budget — the per-incumbent consumption meter

> A first-class, server-side budget for an incumbent CRM's API allocation, metered from
> **our own** call counts. It keeps overlay traffic a conservative, truthful distance
> below a rate ceiling we share with integrations we can't see — and never fabricates
> the headroom it can't account for.

## What it's for

When a workspace runs in overlay mode — the datasource seam bound to an incumbent CRM —
every read, search, or write the overlay serves may become an outbound API call to that
incumbent. The incumbent publishes a rate ceiling, but that ceiling is **shared** with
every other integration the customer runs, and we can't see their usage. This is the
guardrail that keeps us safely below the ceiling, metered from our own counts. The
skeleton ships the meter seam itself; its consumers — the overlay adapters that charge
it and degrade on its answer — are planned and owned by the overlay-augmentation
chapter.

## Principles it serves

- **P13 — overlay augmentation.** Augment the incumbent without breaking it.

See ADR-0018 (bounded-capability guarantee) and A53 (the overlay decision). The
operating rule: *"we never assume headroom we can't see."*

## How it works

The meter is self-contained and Redis-backed, using fixed-window counters with expiry,
and wires in as a decorator over the live overlay seam. It imports nothing from core
(ADR-0014).

- **Two rolling windows per incumbent connection** — a per-second search window and a
  daily REST window, each metered from our own call counts, never inferred from the
  incumbent telling us its usage. Consumption is reported per window.
- **Conservative cap plus thresholds.** Each incumbent's cap (governing the daily REST
  window) is strictly below its published ceiling (OVB-PARAM-3). Warn and shed
  thresholds — defaults seventy and ninety percent of the cap (OVB-PARAM-1,
  OVB-PARAM-2), tunable per incumbent — split every charge's answer into one of three
  states: ok, warn, or shed, with shedding signalled at or over the shed threshold.
  What a consumer *does* on shed — degrading force-fresh reads to mirror-with-staleness
  rather than starving the customer's allocation — is the consumers' contract, pinned
  as AC-OV-7 in the overlay-augmentation chapter; surfaced to API clients as the
  budget-exhausted error the api-conventions chapter owns
  ([[api-conventions#API-ERR-21]]).
- **Per-source breakdown plus an unknown sentinel.** Each charge is attributed to one
  metered source — force-fresh, reconciliation poller, or capture write-back — and the
  breakdown sums to the REST total. Headroom we cannot attribute (foreign integrations
  sharing the ceiling) renders as an explicit unknown sentinel (OVB-PARAM-5), never a
  fabricated number. This is what the overlay-admin budget surface reads.

## What's configurable

Per incumbent, in the YAML config source only: the published ceiling, the (lower) cap,
and the warn/shed fractions (OVB-PARAM-1, OVB-PARAM-2). Load-time validation rejects an
unsafe config — it requires the cap below the ceiling and the thresholds in order and in
bounds (OVB-PARAM-3, OVB-PARAM-4) — so a misconfiguration fails fast rather than
silently overrunning. Redis is the injected counter store; without it the meter cannot
truthfully account, so it fails closed rather than inventing headroom.

## Guarantees (enforced)

- **Never assume invisible headroom** — metering is from our own counts only;
  unattributable headroom is the unknown sentinel, never invented (OVB-AC-1).
- **Stay below the ceiling** — cap below the published ceiling is validated at load
  (OVB-AC-4).
- **Degrade, not starve** — at or over the shed threshold the meter answers shed so
  consumers can fall back instead of exhausting the customer's allocation (OVB-AC-2).
- **Per-incumbent isolation** — one connection's consumption never dents another's
  windows or headroom (OVB-AC-3).
- **Fail-fast config** — an out-of-bounds cap or threshold is rejected at load, not at
  runtime (OVB-AC-4).
- **Breakdown reconciles** — per-source charges sum to the REST total (OVB-AC-5).
- **Core-clean** — imports nothing from core (ADR-0014).

## Acceptance

Done means an operator can trust every number the budget surface shows: consumption
counted from our own calls, headroom that is either real or honestly marked unknown, and
a meter that answers warn before shed and shed before the customer's allocation is
touched. When the meter says shed, overlay consumers degrade gracefully rather than
going dark — that consuming behavior is verified under AC-OV-7 in the
overlay-augmentation chapter. The testable form of each claim here is pinned in the
Acceptance appendix; the cross-cutting floor is inherited from the acceptance-standards
chapter.

## Out of scope

The consumers: the overlay adapters that charge the meter, the degrade-never-starve
policy ladder (force-fresh falling back to mirror-with-staleness), and the
budget-degraded event are owned by the overlay-augmentation chapter, with AC-OV-7 as the
consuming acceptance. The overlay-admin read endpoint and budget UI are separate
surfaces. The 80/100/120 ladder some corpus readers will have met is the **L2 AI-cost
guardrail** — a different budget protecting the operator's model bill, not the
incumbent's API allocation — and is not owned or restated here. The meter itself is
Redis counters plus static YAML config: no migration, no UI, no contract change.

## Where it lives

The meter lives in the platform layer (backend/internal/platform) as overlay budget
infrastructure, reached as a decorator over the live overlay datasource seam. Read next:
the overlay-augmentation chapter for the consumers and the degrade ladder, and the
api-conventions chapter for the budget-exhausted error contract.

## Appendix

### Parameters
Source: margince-poc/docs/subsystems/overlay-budget.md#how-it-works @ a11d6c08; specs/spec/narrative/03e-overlay-augmentation.md#31-incumbent-api-rate-limits--latency-bottleneck-the-agents @ 5a0b29c

These are the skeleton's shipped defaults and this chapter is their single home — the
corpus leaves incumbent-budget calibration open, so these values stand until ratified
per incumbent. (The 80/100/120 cost ladder is the separate L2 AI-spend guardrail and is
deliberately not pinned here.)

| ID | Name | Value | Meaning |
|---|---|---|---|
| OVB-PARAM-1 | Warn fraction (default) | 0.70 | Fraction of the cap at which a charge answers warn; tunable per incumbent. |
| OVB-PARAM-2 | Shed fraction (default) | 0.90 | Fraction of the cap at or over which a charge answers shed; tunable per incumbent. |
| OVB-PARAM-3 | Cap bound | cap < published ceiling | Required at load: each incumbent's cap is strictly below its published rate ceiling. |
| OVB-PARAM-4 | Threshold bounds | 0 < warn < shed ≤ 1 | Required at load: thresholds are ordered fractions of the cap. |
| OVB-PARAM-5 | Unknown-headroom sentinel | `~unknown` | Rendered wherever headroom cannot be attributed to our own counts; never replaced by a fabricated number. |

### Acceptance
Source: margince-poc/docs/subsystems/overlay-budget.md#guarantees-enforced @ a11d6c08; specs/spec/narrative/03e-overlay-augmentation.md#62-machine-verifiable-acceptance-criteria @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| OVB-AC-1 | Given any budget read, when headroom is reported, then it is derived from our own call counts only, and headroom that cannot be attributed renders as the `~unknown` sentinel (OVB-PARAM-5) — the meter never overreports headroom. | Meter unit tests + admin-surface contract test asserting the sentinel, never a number, for unattributable share. |
| OVB-AC-2 | Given consumption at or over the shed threshold (OVB-PARAM-2), when a consumer charges the meter, then the answer is shed and further live calls are declined so consumers degrade rather than starve the customer's allocation; the consuming degrade behavior (force-fresh → mirror-with-staleness) verifies under AC-OV-7 in the overlay-augmentation chapter. | Metered test double driven past the threshold; assert shed state and no further live charges succeed. |
| OVB-AC-3 | Given two incumbent connections, when one is charged to any state, then the other's windows, states, and headroom are unchanged — per-incumbent isolation. | Multi-connection isolation test in the platform lane. |
| OVB-AC-4 | Given a config with cap ≥ ceiling or thresholds out of bounds (OVB-PARAM-3, OVB-PARAM-4), when it loads, then the load is rejected — fail-fast, never a silent overrun. | Config-validation unit tests over the boundary cases. |
| OVB-AC-5 | Given any sequence of charges attributed to the metered sources (force-fresh, reconciliation poller, capture write-back), when the breakdown is read, then the per-source charges sum exactly to the REST-window total. | Reconciliation property test. |
