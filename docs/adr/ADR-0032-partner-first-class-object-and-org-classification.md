# ADR-0032 — Partner is a first-class object; organization classification is a V1 core field

**Status:** Accepted (Lars, 2026-06-22). **Composes with** [ADR-0008](ADR-0008-lead-object-and-promotion.md) (the precedent for adding a first-class noun to the data model), [ADR-0030](ADR-0030-partner-program-scenario-c.md) / A38 (the Scenario-C partner program whose economics this object now persists), [ADR-0006](ADR-0006-scrape-enrichment-connector-seam.md) (firmographic enrichment is the source for classification), and [ADR-0007](ADR-0007-context-graph-is-v1-substrate.md) (partner relationships are edges in the same `relationship` graph). New decision: **DECISIONS A41.** Origin: the 2026-06-22 backlog walkthrough — resolves the §I structural flag ("is *partner* a first-class concept?") and the §A item **A3** (company classification), which were explicitly tied together.

## Context

Two backlog items, deliberately linked, came to a head together:

1. **§I structural flag — "is *partner* a first-class concept?"** Our data-model nouns are `deal` / `lead` / `organization` / `person`. But our own GTM (Scenario C partner program, A38/ADR-0030) leans hard on partner orgs, deal registration, referrals, co-sell, certification tiers and margin tiers — and the beachhead persona (Marcus Greven, a *partner manager*) runs a whole toolkit on partner relationships. This is the one place the beachhead's world did not map onto our nouns.
2. **§A item A3 — company classification** (agency / reseller / tech-vendor / platform / competitor + relevance). Previously Phase-2. It matters **more for us than for a generic CRM** because the agency beachhead and the partner program both depend on knowing *what kind of company* an org is — and "partner" is itself one of those classes.

The founder's call (2026-06-22): **partner is a first-class object**, and **company classification is elevated from Phase-2 to V1.** The two decisions share one mechanism, so they are recorded in one ADR.

The design tension: a naïve "new `partner` table that duplicates company fields" would fork company data and violate the single-source-of-record principle (P1/P11). The relational-clean answer reuses `organization` as the company record and makes "partner" first-class through a dedicated **program-state extension** plus typed graph edges — the same pattern `lead` used (ADR-0008) but, unlike `lead`, *inside* the relationship graph rather than segregated from it.

## Decision

**Partner is a first-class object, realized as four composing pieces over the existing relational core — not a duplicate company table.**

1. **`organization.classification`** — a new V1 core field on `organization`, a check-constrained enum: `prospect` (default) · `customer` · `agency` · `reseller` · `tech_vendor` · `platform` · `partner` · `competitor` · `other`, plus a separate nullable `relevance` score. This **is** the A3 company-classification capability, now V1. Classification is set by firmographic enrichment (ADR-0006 seam) or by hand, and — like every inference — is surfaced with evidence and is a 🟢 reversible internal write (ADR-0026); it never auto-overwrites a human-set value. `competitor` here is the durable org label; the Phase-2 competitor-*mention* detection (R6) writes into this same field's evidence.
2. **`partner` — a first-class program-state table, 1:1 extension of `organization`.** `partner.organization_id` is a `UNIQUE NOT NULL` FK to `organization(id)`; an org is "a partner" iff it has a `partner` row (and carries `classification = 'partner'`). The table holds the **Scenario-C program state** (A38/ADR-0030) that has no home today: certification status (`applied` → `certified` → `suspended`), **margin tier** (`tier1_15` / `tier2_20` / `tier3_25` — the tiered buy-low margin, *not* a recruiter override), certified-staff count, retention/volume metrics that gate the tier, program join/renewal dates. It does **not** restate firmographics — those stay on `organization`.
3. **Partner relationships are typed edges in the existing `relationship` table** (ADR-0007), not new tables: `kind` values `partner_of`, `referred_by`, `co_sell_with` join orgs/people/deals already in the graph. This is the "co-sell / referral / partner-fit" surface Marcus's toolkit needs, and it makes partner relationships traversable by the same context-graph queries as everything else.
4. **`deal.partner_org_id`** — a nullable FK on `deal` for **deal registration / attribution** (the Scenario-C exclusive 90-day deal registration, A38). A deal sourced or co-sold by a partner points at the partner org; this is what the 15% referral / tiered-margin economics and the partner-pipeline views read.

**Scope of "first-class":** partner gets its own table, its own lifecycle (apply → certify → tier → renew/suspend), its own deal-attribution FK, and its own relationship edge-kinds — so it is a genuine noun, queryable and reportable, not a tag. It is **not** a parallel company record: company identity, domains, hierarchy, and firmographics live once on `organization`.

## Consequences

- **`data-model.md` §4 gains** the `organization.classification` + `relevance` columns (with the CHECK enum and an index for classification filters) and a new **§4.x `partner`** table; §5 (`relationship`) gains the three partner edge-kinds; `deal` (§6) gains `partner_org_id`. The `relationship` graph overview ERD adds the partner edges.
- **A3 moves out of the Phase-2 backlog into V1.** `features/02 §3` (firmographic enrichment) now writes `classification`; a thin classification surface + filter is V1. Competitor-*mention* detection (R6) stays Phase-2 but now has a durable field to write to.
- **The partner object is V1 substrate, but the *partner program* stays a build item, not executed** (A38/ADR-0030 unchanged): the table makes the program *representable*; recruiting/certification operations are still §14's checklist.
- **`14-partner-program.md`** gains a pointer: program state (tier, certification, deal registration) is persisted on the `partner` object / `deal.partner_org_id`, not in a spreadsheet.
- **Reporting & the context graph** can now answer partner-pipeline, partner-fit, single-vs-multi-threaded-partner-coverage questions natively (feeds the §C M6 account-mapping views, also promoted to V1).
- **Stories/traceability:** a new V1 story for org classification + a partner-object foundation note; the V1 line is recomputed in the README session pickup.
- **Out of scope:** a partner *portal* (partner-facing self-service surface) is **not** in V1 — partner orgs are managed internally for now; revisit when the program is actually executing. Marketing-partner / co-marketing modules remain out (Charlotte C13, §H).

## Alternatives considered

- **"Partner" as just an org type/tag (the lighter option).** Rejected by the founder: it cannot hold program state (tier, certification, deal registration) without scattering it across tags + custom fields, and it makes partner-pipeline reporting a second-class query. The classification *enum* still exists (that's A3) — but the program object is its own table.
- **Defer the decision.** Rejected: the partner program (A38) is a committed GTM pillar and the beachhead persona is a partner manager; the data model owes this noun now, while V1 schema is still being set, not after WP0 has frozen it.
- **A standalone `partner` table that duplicates company fields.** Rejected: forks the system of record (violates P1/P11); the 1:1 extension keeps company identity single-sourced on `organization`.
