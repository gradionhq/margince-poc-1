---
status: skeleton
module: backend/internal/shared/ports/datasource
derives-from:
  - margince-poc/docs/subsystems/datasource.md @ a11d6c08
---
# Datasource — Margince's own store, or an overlay on an incumbent CRM

> The one data-access port **every** surface reads and writes through — a Tier-0 seam (a swappable
> boundary). Two bindings, chosen **per workspace**: **native** (Margince's own Postgres is the system
> of record) or **overlay** (an incumbent CRM stays the system of record, with Margince's AI and UI
> layered on top). The skeleton ships the port and its native binding, exercised end to end by the
> Person slice; the overlay binding point ships too, but its incumbent adapters are planned work owned
> by the [overlay-augmentation](overlay-augmentation.md) chapter.

## What it's for

Margince has to serve two kinds of customer from one product: those who let Margince *be* their system
of record, and those who keep an incumbent CRM authoritative and want Margince layered on top. The
seam makes that a per-workspace switch rather than a fork. Everything above it — agents, tools,
approvals, UI — speaks one vocabulary of reads, writes, listing and record mutation; whether those
land in Margince's own Postgres or in an incumbent is decided once, at the binding, and is invisible
to every caller. The port is ARCH-SEAM-1 in the architecture chapter's seam inventory.

## Principles it serves

- **P13 — augment, don't demand rip-and-replace.** This subsystem is the enterprise overlay entry
  point: the same codebase runs in native (own system-of-record) and overlay/augmentation modes, so an
  enterprise can layer Margince's AI and UI on top of an incumbent that stays authoritative rather
  than migrating off it.
- **P9 — standalone first, integrations optional.** Overlay is an optional, additive mode. Native mode
  owes nothing to any incumbent and runs fully standalone; the incumbent binding is present only when
  a workspace turns it on.
- **P7 — own your data.** Native mode keeps the customer's records in their own Postgres; overlay mode
  leaves the incumbent authoritative and never silently captures it.
- **P12 — governance designed in.** Provenance is mandatory on every write; optimistic-concurrency
  tokens are part of the seam, not an afterthought.

## How it works

- **One port, two bindings.** A single provider interface covers reading a record, listing and
  searching records, create, update, and freshness. Native mode binds it over the per-entity Postgres
  stores inside the owning module; overlay mode binds it to a named incumbent adapter. The port is
  dependency-free (it stays Tier 0): records are opaque at the port and typed only at the binding, so
  the native core depends on the port, never the reverse (ADR-0013).
- **Mode resolution.** A workspace is resolved to exactly one mode — native resolves to the Postgres
  binding, overlay to the named incumbent. Illegal combinations (overlay without an incumbent, native
  with one, an unknown mode) are rejected. Mode is fixed at deploy; flipping a workspace from overlay
  to native is a separate flow.
- **Every read is workspace-scoped.** Reads through the port run inside the tenant-isolating
  transaction path; row-level security at the database — not application filtering — is what holds the
  boundary (the data-model chapter owns the mechanics).
- **Provenance is mandatory on writes.** Every create and update carries its source and who captured
  it; a write missing either is refused before it touches storage.
- **Optimistic concurrency (ADR-0036).** Updates carry an optional version token. Native mode maps it
  to the native version path; overlay maps it to the incumbent's change marker. A stale token fails
  with a version-skew error callers can match regardless of which binding raised it. The shared update
  shape is frozen and additive-only: it is the one the future incumbent write-back consumes unchanged.
- **Freshness and honest authority.** A high-value, needs-approval action can force a live
  read-through before acting. Every read reports whether it is authoritative: native mode is always
  authoritative; staleness is only real in overlay mode, and an overlay read never claims authority it
  does not have.

Everything deeper on the overlay side — the derived mirror the overlay binding reads from,
incumbent-first write-back, conflict reconciliation, and the concurrency strategies for different
incumbents — is adapter behaviour behind the port, specified and pinned by the
[overlay-augmentation](overlay-augmentation.md) chapter.

## What's configurable

- **Per-workspace mode** — native or overlay, with the named incumbent for overlay; fixed at deploy
  (DS-PARAM-1). The skeleton wires native.
- **The bound provider** — injected at the composition edge: the native Postgres binding in
  production-shaped deployments, a fake binding in unit tests. Callers cannot observe which one they
  got except through the honest-authority flag.

## Guarantees (enforced)

- **Seam isolation (AC-OV-1 — enforced by a merge-blocking lint).** No layer above the port — modules,
  the AI layer, the capture layer, the agents layer — imports another module's store internals or an
  incumbent SDK directly; only the adapter behind the binding may. The architecture lint holds the
  allowed-import matrix ([[quality-gates#QG-9]]; the matrix rows are [[architecture#ARCH-IMPORT-3]]
  through [[architecture#ARCH-IMPORT-8]]), and the lint ruleset's import allow/deny guard backs the
  SDK half ([[quality-gates#QG-3]]). This is what makes everything above the seam work on an incumbent
  for free.
- **Workspace scoping on every read.** A query through the port returns only the calling workspace's
  rows; a cross-tenant query returns nothing — held by row-level security at the database and the
  store-path gate ([[quality-gates#QG-13]]), not by handler discipline.
- **Mode invariant.** A workspace is overlay **if and only if** it names an incumbent — enforced at
  the database, not just in code.
- **Provenance-or-reject.** A write missing source or capturer is refused
  ([[acceptance-standards#GATE-CORE-3]] is the cross-cutting floor).
- **Binding swap is invisible.** A caller written against the port behaves identically over the native
  binding, an incumbent adapter, or a test fake; the port shape is frozen and additive-only.

## Acceptance

Done, for this subsystem, means: every surface reaches records through this one door and no other;
the Person slice proves the native binding end to end — a person created through the port is listed,
read, and updated through it, workspace-scoped at every step; a write without provenance is refused
with a matchable error, not silently accepted; and a stale concurrency token is a version-skew
refusal, never a lost update. The testable form of each claim is pinned in the Acceptance appendix;
the cross-cutting floor (standard screen states, performance budgets, release gates) is inherited from
the acceptance-standards chapter and not restated.

## Out of scope

The incumbent adapters and everything that makes overlay real — the derived mirror and its sync
states, incumbent-first write-back, backfill, webhooks, drift reconciliation, the bounded-equivalence
check, the overlay→native flip, and the record-to-conversation link verbs — are planned work owned by
[overlay-augmentation](overlay-augmentation.md); the AC-OV series pinned there is that work's
acceptance home. The native stores themselves belong to
[people-and-organizations](people-and-organizations.md) and its sibling record chapters; the tenancy
and provenance schema belongs to the data-model chapter.

## Where it lives

The port at `backend/internal/shared/ports/datasource` (Tier 0), its native binding inside
`backend/internal/modules/people`, wired at the composition edge. Read the architecture chapter for
the seam inventory and import matrix, the data-model chapter for the tenancy mechanics, and
[overlay-augmentation](overlay-augmentation.md) for the incumbent side.

## Appendix

### Parameters
Source: margince-poc/docs/subsystems/datasource.md#whats-configurable @ a11d6c08

| ID | Name | Value | Meaning |
|---|---|---|---|
| DS-PARAM-1 | Workspace data mode | `native` (skeleton default) \| `overlay` + named incumbent | Chosen per workspace, fixed at deploy. Overlay requires a named incumbent; native forbids one; any other combination is rejected (DS-AC-5). Flipping modes is a separate flow, not a config edit. |

### Acceptance
Source: margince-poc/docs/subsystems/datasource.md#guarantees-enforced @ a11d6c08

| ID | Given/When/Then | Verification |
|---|---|---|
| DS-AC-1 | Given any layer above the seam (a module, the AI layer, capture, agents), when it reads or writes records, then it does so only through the datasource port — no import of another module's store internals or an incumbent SDK exists in that layer. | Architecture lint over the allowed-import matrix, merge-blocking ([[quality-gates#QG-9]], rows [[architecture#ARCH-IMPORT-3]]–[[architecture#ARCH-IMPORT-8]]); import allow/deny guard ([[quality-gates#QG-3]]). |
| DS-AC-2 | Given records in two workspaces, when any read runs through the port under the first workspace's context, then only that workspace's rows return and a cross-tenant query returns nothing. | Integration lane ([[testing#TEST-LANE-2]]) RLS assertions + store-path gate ([[quality-gates#QG-13]]). |
| DS-AC-3 | Given a caller written against the port, when the workspace's binding is swapped (native store, incumbent adapter, or test fake), then the caller's behaviour is unchanged — the port shape is frozen and additive-only. | Port contract-compliance suite driven against every binding, unit lane ([[testing#TEST-LANE-1]]); contract-breaking gate on the shared shapes ([[quality-gates#QG-7]]). |
| DS-AC-4 | Given a create or update missing its source or its capturer, when it reaches the port, then it is refused before touching storage with a matchable error. | Unit lane ([[testing#TEST-LANE-1]]); cross-cutting provenance floor [[acceptance-standards#GATE-CORE-3]]. |
| DS-AC-5 | Given a workspace configuration, when its mode is resolved, then overlay-without-incumbent, native-with-incumbent, and unknown modes are all rejected — the overlay-iff-incumbent invariant is held at the database. | Schema constraint + integration lane ([[testing#TEST-LANE-2]]). |
| DS-AC-6 | Given an update carrying a stale version token, when it runs against either binding, then it fails with the one version-skew error callers match on (ADR-0036); a missing token preserves the happy path. | Unit lane ([[testing#TEST-LANE-1]]), the same update driven through both bindings. |
| DS-AC-7 | Given any read, when it returns, then it reports whether it is authoritative: native reads always are; an overlay read never claims authority before the incumbent acknowledges. | Port contract-compliance suite, unit lane ([[testing#TEST-LANE-1]]); the overlay half re-verified by the AC-OV series when the adapters land. |
