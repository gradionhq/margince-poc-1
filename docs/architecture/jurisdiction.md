---
module: jurisdictions/ (packs) + backend/internal/shared/ports/jurisdiction (the port)
derives-from:
  - margince-poc/docs/architecture/jurisdiction.md @ a11d6c08
  - architecture/14-jurisdiction-packs.md#the-three-buckets-the-line-you-must-draw-on-every-e17-story @ 5a0b29c
---
# Jurisdiction packs

> Where does German (then Vietnamese, …) code live? **In its own Go module per country,
> behind a dependency-free port, composed at compile time by the edge.** Core stays
> jurisdiction-neutral, renders with zero packs, and **cannot import a pack**. Ratified
> by ADR-0042.

## What it's for

A country brings regulatory weight — e-invoice formats, retention law, conformity
declarations, trust artifacts, export profiles — that has no business polluting the
neutral core. This subsystem gives each country its own module that *extends* core
through a fixed port, and lets the edge decide at build time which countries are
linked. A DACH build links the German pack; a non-DACH build omits it and still
compiles, boots, and runs. The boundary is structural, not conventional: core literally
cannot reach country code.

This covers the port and registry, the German pack wired as a build-switchable module,
per-pack migration aggregation, and the country-isolation gates. German behavior lives
behind the fiscal hook — the **XRechnung e-invoice generator + validator** (see below).
Further German implementations (retention classes, conformity, ZUGFeRD, DATEV) extend
the same hooks.

**The three-bucket rule** (the line to draw on every country story, lifted from the
corpus blueprint — the poc doc states the boundary but not the classification rule):

1. **Generic engine** (country-agnostic) — lives in **core**: the retention engine, the
   audit log, the offer engine, export machinery, the approval inbox, the connector
   port, and the locale/formatting engine with the app's UI translations. It works with
   **zero** packs; every country reuses it; a country package *extends* it.
2. **Jurisdiction pack** (country behavior + data) — lives in the country's **own Go
   module**: national formatters and parsers, statutory retention classes and
   immutability, conformity regimes, country trust artifacts, country export profiles,
   and the country's *regulatory document text*. Compiled in only for builds that link
   it; all of a country's code in one module.
3. **Composition root** (the switch) — the **edge** requires and blank-imports the
   enabled packs, selecting which countries link into *this* binary.

## Principles it serves

- **P7 — own your data / sovereignty.** Country regulatory regimes are first-class and
  isolated, not bolted onto a US-centric core.
- **P12 — governance designed in.** The neutral/country boundary is enforced by the
  module graph and by merge-blocking gates, not by reviewer discipline.

## How it works

- **The boundary is the module graph.** The port is its own dependency-free leaf
  package under `backend/internal/shared/ports/jurisdiction`; each country is its own
  Go module beside `backend/` (the German pack is `jurisdictions/de/`) that requires
  the backend for the port interfaces only and self-registers in its init. The edge —
  the composition root at `backend/cmd/api` — requires the country modules, so their
  init runs and they become linked. The backend's module manifest requires no pack,
  which makes core→pack a **compile error, not a lint** (JUR-GUAR-1). Only the edge
  requires packs (JUR-GUAR-2).
  <!-- 1c-mapping: final call pending — pack placement: a separate Go module beside backend/, per ADR-0042 (matches the architecture and code-organization chapters' flag). -->
- **The port.** A pack exposes a contribution surface: its code, its own migrations
  directory, and five capability hooks — fiscal formatter, retention policy, conformity
  regime, trust-artifact set, export profiles (JUR-HOOK-1..5). A pack returns
  nil/empty for anything it doesn't yet contribute, and core has a sensible default for
  each (generic PDF instead of a national e-invoice, core retention only, no conformity
  declaration, …). So a country with no pack, or a partial pack, still works. The port
  is stdlib-only and **frozen, additive-only**; an import-purity test fails the build
  if any non-stdlib import appears (JUR-GUAR-4).
- **Locale is not jurisdiction** (JUR-GUAR-7). There is deliberately no locale hook.
  Display language is core, driven by the user's locale preference (a German speaker at
  a US company gets German UI but no German tax law). The pack carries only *regulatory
  document text* — strings on an invoice or conformity declaration — which rides the
  fiscal/conformity hooks.
- **The registry.** Packs register once at init (a duplicate code panics — a
  registration-bug guard, not a runtime path). Core looks a pack up directly, or asks
  for the **applicable set** for a country. The applicable set is **core-composed**: a
  country→region map *in the port package* resolves a country to its region packs, so a
  future EU region pack is reached without any pack→pack import (ADR-0042 — packs never
  import each other). Germany is the registered pack; an unknown or unlinked country
  returns nothing.
- **The compile-time switch.** A build-tag-gated blank import in the edge is the
  switch: the default build links the German pack so its lookup succeeds; the no-pack
  build links no pack, compiles, and boots with the lookup failing and no German tables
  ever migrated (JUR-GUAR-5). A print-jurisdictions probe prints the linked codes and
  exits before any infra is dialed, making the switch observable with zero infra. A
  composition test builds the real binary in *both* configs and asserts Germany present
  / absent, both exiting cleanly.
- **Per-pack migration aggregation (P3).** Each pack ships its **own** migrations
  directory; the runner applies only the linked packs' migrations. Ownership is
  namespaced so a pack never collides with core's numbering (ADR-0017) — each pack owns
  its own numbering from the start, and **no core migration number is consumed by a
  pack** (JUR-GUAR-6). The aggregator is registry-driven and jurisdiction-string-free:
  it applies core migrations into the core version table, then applies each linked
  pack's migrations into its **own** per-pack version table, under a short lock timeout
  so a stuck migration fails fast. The migrate entrypoint (`backend/cmd/migrate`) runs
  in the pre-infra zone, and because the linked set *is* the compile-time switch, a
  no-pack build migrates core only.
- **Country-isolation gates with proven teeth (P4).** Two invariants are
  build-verifiable with negative fixtures. A **no-jurisdiction-strings-in-core** check
  (parameterizable, widened to named regulatory identifiers plus a conservative
  ISO-3166 grep — [[quality-gates#QG-10]]) is shelled by a test that proves it goes RED
  on a seeded German-regulatory file and GREEN on a clean tree and on the port
  carve-out (JUR-GUAR-3). A **pack-boundary** test proves, via the module graph, that
  core imports no pack and that only the composition root imports the German pack; it
  also proves the intra-module architecture gate ([[quality-gates#QG-9]]) has teeth by
  making a synthetic forbidden edge fail the linter. (The cross-module edge is a
  module-graph assertion, since the architecture linter only sees the backend module.)

## German fiscal hook — XRechnung e-invoice

The German pack's fiscal hook returns a formatter that emits a **standard-conformant
XRechnung (EN-16931 / UBL 2.1, XRechnung 2.3.1 CIUS)** from an invoice and **refuses to
emit an invalid one** — with the validity gate runnable in the Go repo with **no JVM**.
The field-level bindings are pinned in the Wire appendix (JUR-XR-1..10); the e-invoice
*feature* itself is V0.5 and its stories are owned by the germany-package chapter — the
bindings live here because the port types carry them.

- **The port carries the EN-16931 fields.** The port's invoice and invoice-line types
  hold the BT-* fields the standard needs — totals in minor units with the ISO-4217
  currency, buyer and order references, tax category/rate, and per-line quantity, unit,
  and line amount (JUR-XR-1..6).
- **Core owns the call-site mapper.** The invoice-owning core module (under
  `backend/internal/modules/`) reads the core invoice and its offer line items and
  builds the port type — applying the discount percentage and propagating the tax rate
  per line via exact rational arithmetic truncated to integer minor units. So the
  German pack imports only the jurisdiction port, never core internals (ADR-0014,
  ARCH-IMPORT-11); the architecture lint allows the module→port edge as a port edge,
  not a pack dependency.
- **Amounts are bound, never free-typed.** The generator binds the document totals
  directly to the invoice's server-computed minor-unit totals, and each line's amount
  to the stored offer line item; the summed lines reconcile exactly to the document net
  (JUR-XR-2, JUR-XR-6 — a build test pins it).
- **Pure-Go validator + refuse-to-emit gate.** Emit validates **before returning** —
  any failure aborts with a refuse-to-emit error, so no invalid XML escapes
  (JUR-XR-7). Validation is two layers: the vendored XSD via a libxml2-based checker
  (no JRE), then twelve mandatory rules transcribed as Go assertions pinned to the
  XRechnung 2.3.1 release. Coverage is the documented subset (no KoSIT parity claim);
  the vendored artifacts are a fixed checked-in release, not "latest" (JUR-XR-8). The
  formatter also exposes an inbound parse entry for reading XRechnung documents
  (JUR-XR-9).
- **Footprint:** no migration, no contract or type generation, no frontend, no API
  route — a pure formatter + validator behind the port (JUR-XR-10).

## What's configurable

- **The linked country set** — the build tag (default build links the German pack; a
  no-pack build links none).
- **Each pack's contributions** — a pack opts into any of the five capability hooks
  (JUR-HOOK-1..5) and supplies its own migrations; everything it omits falls back to a
  core default.

## Guarantees (enforced)

- **Core cannot import a pack** — it's a compile error, not a lint; proven by the
  pack-boundary test (JUR-GUAR-1).
- **Only the composition root imports a pack** (JUR-GUAR-2).
- **No country strings leak into core** — proven RED on a negative fixture, GREEN on
  the real tree and the port carve-out (JUR-GUAR-3).
- **The port stays stdlib-only** — an import-purity test fails the build otherwise
  (JUR-GUAR-4).
- **A no-pack build still compiles and boots** with no country tables migrated — proven
  by the composition test (JUR-GUAR-5).
- **Migration namespaces never collide** — per-pack numbering and version tables
  (JUR-GUAR-6).
- Every gate is a required CI check that blocks merge.

## Acceptance

"Done" means both build configs are proven, not assumed: the composition test builds
the real binary with and without the German pack and asserts presence/absence and a
clean boot; the isolation gates are proven to have teeth by negative fixtures, not just
to pass on a clean tree; and the fiscal path refuses to emit an invalid document. The
testable form is pinned in the appendix (JUR-GUAR-*, JUR-XR-*).

## Further German implementations

The German pack extends these interfaces with ZUGFeRD hybrid PDF/A-3 output, DATEV
export, inbound XRechnung parsing, the needs-approval issue-gate and invoice
immutability, the e-invoice screen, and the retention-class and conformity logic. The
ownership-manifest extension, the jurisdiction marker convention, and per-pack agent
docs round out the subsystem. Those are V0.5 stories owned by the germany-package
chapter.

## Out of scope

eIDAS e-signature needs no port of its own — the trust service provider rides the
existing connector port inside the German pack. App-UI translations are never pack
content — locale is core (JUR-GUAR-7).

## Where it lives

The jurisdiction port (`backend/internal/shared/ports/jurisdiction` — including the
EN-16931 invoice port types), the migration aggregator behind the migrate entrypoint
(`backend/cmd/migrate`), the German pack (`jurisdictions/de/` — its fiscal hook, the
XRechnung generator/validator, and its vendored rule assets), the call-site mapper in
the invoice-owning core module under `backend/internal/modules/`, and the compile-time
switch in the composition root (`backend/cmd/api`).
<!-- 1c-mapping: final call pending — pack placement (jurisdictions/ beside backend/) per ADR-0042. -->

## Appendix

### Parameters — capability hooks
Source: margince-poc/docs/architecture/jurisdiction.md#how-it-works @ a11d6c08

The pack contribution surface: five hooks, each optional per pack, each with a core
default when absent.

| ID | Hook | A pack contributes | Core default when nil/empty |
|---|---|---|---|
| JUR-HOOK-1 | Fiscal formatter | national e-invoice emit / validate / inbound parse | generic PDF invoice, no national e-invoice |
| JUR-HOOK-2 | Retention policy | statutory retention classes + immutability flags | core retention only |
| JUR-HOOK-3 | Conformity regime | required conformity artifacts + declaration shape | no conformity declaration |
| JUR-HOOK-4 | Trust-artifact set | country trust-artifact descriptors | none |
| JUR-HOOK-5 | Export profiles | country export profiles | core exports only |

### Acceptance — pack guarantees
Source: margince-poc/docs/architecture/jurisdiction.md#guarantees-enforced @ a11d6c08

| ID | Guarantee | Held by |
|---|---|---|
| JUR-GUAR-1 | Core cannot import a pack — a **compile error, not a lint** (the backend Go module requires no pack) | module graph + pack-boundary test |
| JUR-GUAR-2 | Only the composition root imports a pack; packs never import each other | pack-boundary test (module-graph assertion); the intra-module linter's teeth proven by a synthetic forbidden edge ([[quality-gates#QG-9]]) |
| JUR-GUAR-3 | No jurisdiction strings in core (named regulatory identifiers + conservative ISO-3166 grep, port carve-out excepted) | negative-fixture-proven check: RED on a seeded German-regulatory file, GREEN on a clean tree ([[quality-gates#QG-10]]) |
| JUR-GUAR-4 | The port stays stdlib-only, frozen, additive-only | import-purity test fails the build on any non-stdlib import |
| JUR-GUAR-5 | A no-pack build compiles, boots, and migrates core only; a duplicate pack registration panics at init | composition test builds the real binary in both configs; print-jurisdictions probe observable pre-infra |
| JUR-GUAR-6 | Per-pack migration namespacing: each pack owns its own numbering and its own version table; no core migration number is consumed by a pack; apply order core-then-pack under a short lock timeout | registry-driven aggregator + migration tests (ADR-0017) |
| JUR-GUAR-7 | Locale is not jurisdiction: there is no locale hook; display language is core (per-user preference); a pack carries only regulatory document text riding JUR-HOOK-1/3 | port surface (no locale hook exists to implement) |

### Wire — XRechnung bindings
Source: margince-poc/docs/architecture/jurisdiction.md#german-fiscal-hook--xrechnung-e-invoice @ a11d6c08

> **Scope note:** the e-invoice feature itself ships in **V0.5**; the germany-package
> chapter owns its stories and screens. These bindings are pinned here because the
> jurisdiction port types carry the EN-16931 fields and the port contract must not
> drift.

| ID | Binding |
|---|---|
| JUR-XR-1 | The port's `Invoice` / `InvoiceLine` types hold the EN-16931 BT-* fields; all amounts in **minor units + ISO-4217 currency** |
| JUR-XR-2 | Document totals `TaxExclusiveAmount` / `TaxAmount` / `TaxInclusiveAmount` / `PayableAmount` (BT-109/110/112/115) bind **directly** to the invoice's server-computed minor-unit totals — never free-typed |
| JUR-XR-3 | `BuyerReference` = BT-10 |
| JUR-XR-4 | `OrderReference` = BT-13 (the offer number + revision) |
| JUR-XR-5 | Tax category / rate = BT-118/119, propagated per line by the core call-site mapper via `big.Rat`, truncated to int64 minor units (discount percentage applied) |
| JUR-XR-6 | Per-line quantity / unit / `LineExtensionAmount` = BT-131, bound to the stored offer line item; **summed lines reconcile exactly to the document net** (a build test pins it) |
| JUR-XR-7 | `Emit` validates **before returning** — any failure aborts with a refuse-to-emit error; no invalid XML escapes. Two validation layers: vendored XSD via `xmllint`/libxml2 (no JVM/JRE), then **12 mandatory BR-*/BT-* rules as Go assertions** pinned to **XRechnung 2.3.1** (EN-16931 / UBL 2.1, XRechnung 2.3.1 CIUS). Coverage is the documented subset — no KoSIT parity claim |
| JUR-XR-8 | Vendored artifacts (mandatory-field catalog + structural XSD) are a fixed checked-in release, never "latest" |
| JUR-XR-9 | An inbound `Parse` entry reads XRechnung documents (supplier side) |
| JUR-XR-10 | Footprint: no migration, no contract/type generation, no frontend code, no API route — a pure formatter + validator behind the port |
