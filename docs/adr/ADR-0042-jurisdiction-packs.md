# ADR-0042 — Jurisdiction packs: country-specific code lives in its own compile-time module behind a Tier-0 seam

**Status:** Accepted (2026-06-24, founder). Recorded as **DECISIONS A57**. Introduces the
**jurisdiction-pack** module category and the Tier-0 `jurisdiction` seam. Applies ADR-0014 §1's
existing "own `go.mod`" promotion clause; back-referenced from ADR-0014 and ADR-0016.

## Context

The locked beachhead is the regulated DACH Mittelstand (A43/ADR-0033), and the **Germany Package**
(E17, ADR-0038/A51) bundles the DACH go-live plumbing: XRechnung/ZUGFeRD e-invoicing, DATEV export,
eIDAS e-signature, GoBD retention/immutability, and the buyer-facing trust pack. The other workstream
is decomposing E17's large stories into PR-sized build tickets.

E17 is today a **product/build-backlog** construct only. Nothing in the architecture keeps the German
code it generates *out of the core modules*. The E17 build stories name core entities (`invoice`,
`audit_log`, the retention engine) and, as written, would land XRechnung/DATEV/GoBD logic directly in
`crm-core` and friends. That has two failures:

1. **It ships German regulatory code to every tenant** — a French or (later) Vietnamese deployment
   would compile and carry German tax/e-invoice/retention logic it never runs.
2. **It clutters every module with country logic.** Reading `crm-core` you'd meet `XRechnung`,
   `DATEV`, `GoBD` inline. When a second jurisdiction (`crm-vn`) arrives, the country-specific code is
   smeared across the codebase with no boundary — the opposite of the "wow on sight" legibility Goal 1
   and ADR-0016 promise.

The architecture already has the machinery to solve this cleanly: a dependency-free Tier-0 seam layer
(ADR-0014 §2), self-registration into registries via generated manifests (`02-composition-and-registries.md`),
mechanically-enforced import boundaries (ADR-0014 §3), and an explicit clause — **ADR-0014 §1** — that
*"promote[s] a module to its own `go.mod` … where a fork story demands independent versioning,"* tied
via `go.work` exactly as the CRM↔Dispact↔`@gradion/contracts` seam already is. A per-country regulatory
module is precisely that promotion case.

## Decision

**Country-specific code lives in a jurisdiction pack: a self-contained Go module per jurisdiction
(`crm-de`, later `crm-vn`/`crm-eu`), behind a new Tier-0 `jurisdiction` seam, composed into a binary at
compile time by the edge composition root.** Core never names a country and cannot import a pack.

**1. The `jurisdiction` Tier-0 seam (new leaf package).** A dependency-free interface package
(alongside `sor`, `connector`, `mcp`, `model`) defining a `jurisdiction.Pack` and a
`jurisdiction.Registry` keyed by jurisdiction id, plus the contribution interfaces a pack implements:
`FiscalFormatter` (emit/parse/validate e-invoice formats), `RetentionPolicy` (classify → statutory
window + immutability), `ConformityRegime` (which conformity artifacts a jurisdiction requires — e.g.
the CRA Declaration of Conformity — plus its template/fields, fed per-fork by the core EP08 build),
`TrustArtifactSet` (country compliance-artifact descriptors), `ExportProfile` (a jurisdiction-scoped
audit-export variant). Frozen, additive-only (`04-seam-evolution.md`). eIDAS e-signature needs **no new
seam** — it rides the existing `connector.Connector`. **i18n is *not* a pack concern** — locale (UI
translations + formatting) is a per-user axis orthogonal to country-of-operation and stays core (§3).
Core resolves a workspace's **applicable jurisdiction set** (`Applicable(country) → []Pack`: the country
plus its region, V1 = just the country) and merges contributions; packs never import one another. Core
consumes only these interfaces + the registry; it never references `XRechnung`, `DATEV`, `GoBD`, or any
country.

**2. Three buckets — draw the line explicitly.**
- **Generic engine (core, country-agnostic):** the retention engine, append-only `audit_log`, the
  offer engine, export-bundle machinery, the approval inbox, the connector seam, the **reproducible-build
  + SBOM rails** (EP08; SBOM is universal supply-chain hygiene), the **locale-resolution + formatting
  engine *and* the DE/EN UI translations** (per-user locale, EP09 — §3). Works with **zero** packs; every
  country reuses it. E17 already *extends* these — it does not rebuild them.
- **Jurisdiction pack (`crm-de`, country behavior + data):** XRechnung/ZUGFeRD/DATEV formatters +
  parsers + the pinned EN-16931 Schematron, GoBD retention classes + immutability policy, the eIDAS-TSP
  connector, the **CRA Declaration-of-Conformity regime** (per-fork DoC, bound to the core SBOM/provenance —
  §6/flag-3), the German trust artifacts (BSI C5, TISAX, §393 SGB V, BetrVG), the GoBD audit-export
  profile, and the German *regulatory document text* (invoice layout strings, GoBD notes — **not** app-UI
  translation). All of it in one module.
- **Composition root (edge, the switch):** the edge module (`cmd/server`) `require`s + blank-imports
  the enabled packs. Which packs a binary links is chosen here, at compile time.

**3. i18n stays core — locale ⊥ jurisdiction (corrected from the first draft).** Locale and jurisdiction
are **orthogonal axes**: locale is a per-*user* display preference ("I want German UI"), jurisdiction is
a per-*workspace* country-of-operation regime ("I must file XRechnung"). A German-speaking employee at a
US company wants German UI but never touches German tax law. So **UI translations *and* the formatting
engine stay core**, driven by `user.locale` (BCP-47, doc 10) — `EP09` already makes **DE/EN i18n a core
platform capability** and ADR-0038 says German UI/localization is V1 substrate (S-E15.10a), "referenced,
not duplicated." Putting translations in `crm-de` would (a) deny German UI to a German user in a non-DACH
build unless the regulatory pack were compiled in, and (b) contradict EP09/S-E15.10a. The pack therefore
carries **no app-UI translations** — only *regulatory document text* (the strings on a German invoice/DoC),
which rides `FiscalFormatter`/`ConformityRegime` as part of the artifact, not the UI. There is **no
`LocaleBundle` in the seam.**

**4. Compile-time composition (separate Go modules).** Core is one `go.mod` (ADR-0014 §1 unchanged for
the seven CRM modules). Each pack is its **own `go.mod`** (`require`-ing core for the seam interfaces),
tied in via `go.work` for local dev — the same mechanism already used for the Dispact/contracts seam.
The **edge module's `require`-set is the switch**: a DACH build `require`s + blank-imports `crm-de`
(its `init()` registers into `jurisdiction.Registry`); a build that omits it never links the pack, never
runs its `init()`, never migrates its tables. Each pack owns its migrations (ADR-0017), aggregated by
the runner only when enabled.

**5. Enforcement (the module boundary plus belt-and-suspenders).** Because a pack isn't in core's
module graph, core *cannot* import it (compile error) — the strongest boundary. Additionally:
`go-arch-lint` declares core ↛ pack, pack ↛ pack, only `cmd/*` → pack; a new **"no jurisdiction strings
in core"** fitness function (`03-invariant-enforcement.md`) fails the build if a country identifier
(`XRechnung`/`ZUGFeRD`/`DATEV`/`GoBD`/`eIDAS`/ISO-3166 literal) appears in a core module outside the
seam; a **`JURISDICTION: <cc>`** source marker (mirroring the `CONTROL: D1–D8` markers, doc 06) makes
packs greppable; the `module-ownership.yaml` (ADR-0014 Amendment 1) assigns DE tables/events/keys
(`datev_export`, `xrechnung_doc`, …) to `crm-de`, generic `invoice`/`audit_log` to core; `crm-de` ships
its own `AGENTS.md` (ADR-0016 §4).

**6. Jurisdiction-id granularity + the EU-regulatory layer.** Ids are free-form strings — a country
(`de`) or a supranational region (`eu`). Two contributions are **EU-wide, not German**: the **eIDAS**
e-signature regime and the **CRA Declaration of Conformity**. For V1 there is only one jurisdiction, so
**both land in `crm-de`** (out of core for CRA — see flag-3 below), and core's `Applicable(country)`
resolution returns just `[de]`. The clean future home for both is a `crm-eu` **region pack**: when a
non-German jurisdiction appears, `Applicable("de")` returns `[de, eu]` and core **merges** the
contributions — `crm-de` never imports `crm-eu` (the pack ↛ pack rule holds because *core composes the
set*, packs don't compose each other). Building `crm-eu` now is **deliberately deferred** (YAGNI: one
member, no V1 benefit); the resolution model above is what makes the later hoist purely additive.

**Flag resolutions (2026-06-24, founder — A57 review).**
- **i18n → core** (§3): translations + engine are core; no `LocaleBundle` in the seam.
- **eIDAS → `crm-de` for V1**, `crm-eu` later via the `Applicable` set (above).
- **Per-fork SBOM/CRA → split:** the **SBOM rebuild stays core/EP08 permanently** (universal supply-chain
  hygiene; the reproducible-build + signing + SBOM rails are shared). The **CRA Declaration of Conformity
  moves out of core into the pack now**: the jurisdiction pack supplies the `ConformityRegime` (which
  artifacts apply + the DoC template/fields); EP08 keeps the generic build/sign/SBOM + DoC-assembly
  *engine*, and the per-fork DoC (B-E17.13/.14) is *bound* to the core SBOM/provenance but *shaped* by the
  pack's regime — so a non-CRA jurisdiction simply contributes a different (or no) regime. CRA rides the
  same `crm-eu` hoist path as eIDAS.

## Consequences

- **Positive:** core stays jurisdiction-neutral and renderable with zero packs; reading `crm-core` you
  never meet a German tax format, reading `crm-de/` you see *all* of it in one module (Goal 1, the
  user's "crystal clear, not cluttered" requirement). Adding Vietnam later is "drop in `crm-vn/`, add it
  to that deployment's `require`-set." A non-DE build cannot accidentally carry or expose DE code. The
  pattern generalizes E17 from a bespoke epic into the first instance of a reusable framework.
- **Stays inside the locked thesis (ADR-0002):** a jurisdiction pack is **compile-time additive
  source**, not a runtime config/plugin engine. A deployment includes a pack by editing its build
  (`require` + blank-import) and rebuilding — exactly the agent-extensible-source model, not a metadata
  toggle. No core-behavior override; packs only *add* via seams. ADR-0002 holds.
- **Negative / honest limits:** (a) multi-module adds `replace`/version surface — bounded by keeping
  *core* single-module and only promoting packs (ADR-0014 §1's stated cost, accepted here for the
  isolation it buys); (b) a new seam + registry + fitness function is real platform work (EP10) that
  must land before E17 code; (c) the locale ⊥ jurisdiction line (§3) must be *taught*: German UI is a
  core per-user capability, German tax law is a pack — easy to conflate; (d) moving the CRA DoC out of
  core (flag-3) splits the conformity story across EP08 (SBOM/build engine) and the pack (regime), so the
  per-fork DoC (B-E17.13/.14) spans both — a seam handshake to keep honest.
- **Relationship to other decisions:** **amends ADR-0014** (applies its §1 promotion clause to a new
  module category; extends its DAG + ownership-manifest rules) and **ADR-0016** (adds the
  jurisdiction-pack layout + per-pack `AGENTS.md`); composes with ADR-0038/A51 (the Germany Package is
  the first pack), ADR-0002 (additive source), doc 10 (i18n engine stays core), ADR-0017 (per-module
  migrations), ADR-0037 (offer → invoice totals stay in the core engine). Does **not** weaken any locked
  decision.
- **Scope:** new Tier-0 seam `jurisdiction` + `jurisdiction.Registry` (contributions `FiscalFormatter`,
  `RetentionPolicy`, `ConformityRegime`, `TrustArtifactSet`, `ExportProfile`; `Applicable(country) →
  []Pack` resolution); new architecture doc `architecture/14-jurisdiction-packs.md`; module-DAG update
  (`architecture/01`); new platform epic **EP10** (the framework build stories); the E17 build stories
  retagged `core` vs `crm-de` (incl. B-E17.13/.14 split: SBOM core, CRA DoC pack). i18n unchanged (core,
  per EP09/doc-10).
