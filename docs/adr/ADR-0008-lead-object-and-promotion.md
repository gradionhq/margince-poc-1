# ADR-0008 — The Lead object and lead→contact promotion

**Status:** Accepted (2026-06-04, with Lars) — built into `03-architecture §3.2` (the `lead` table) and build-plan WP1+/WP2+. The promotion-trigger set is now **ratified** (DECISIONS A2 / `formulas-and-rules.md §2`): inbound reply OR meeting booked-or-held OR human qualify; cold outbound never promotes; optional engagement-signal trigger ships OFF. Raised when auditing table-stakes coverage: the spec modelled "lead" only as a *score/status* (`features/03 §3`), with no Lead object and no lifecycle — yet the product mass-creates machine-sourced prospects (AI SDR `stories/E06`, ICP account surfacing `features/07 §3`, the data-provider connector `ADR-0006 §5`, cold-start accounts `S-E01.3`). Without a Lead object those would pollute `contacts`.

## Context

A CRM that runs an AI SDR and pulls from Apollo / web crawl / list imports creates **large volumes of unengaged, machine-sourced prospects**. If these are written as `person` (contact) rows they destroy the three things that make our model good: **dedupe**, **relationship-strength**, and honest **"who do we actually know"** reporting. This is acute, not cosmetic, at SDR volume.

Two incumbent models, both rejected:
- **Salesforce** — a separate hard `Lead` object with a one-way *convert* to Contact+Account+Opportunity. Solves pollution but the convert is **lossy/irreversible** and cross-boundary reporting is the directional-association mess P11 exists to avoid.
- **HubSpot** — one Contact object + `lifecycle_stage`, no separate object. Clean to report on, but **pollutes** the contact graph with cold scraped records.

## Decision

Introduce **`lead` as a first-class core object** (a real, normalized table with real FKs, per P11 — a *standard object done excellently*, not a metadata row), distinct from `person`, with a **non-lossy promotion to `person`** on genuine engagement.

1. **What a lead is:** a deliberately-thin, raw, *machine- or bulk-sourced* prospect that has **not** had a genuine interaction — created by the AI SDR, ICP surfacing, the data-provider connector, crawl/scrape, or a list/CSV import. Carries provenance (`source`, `captured_by=agent:sdr | connector:apollo | import:*`) like everything else (P5/P12).
2. **Segregation by construction (the anti-pollution rule):** until promoted, a lead is **excluded by default** from: contact lists/search, dedupe-against-`person`, the relationship-strength computation, the context graph's "people we know," and all "contact"/relationship reporting. Leads have their **own** list, their own scoring (the existing `features/03 §3` lead-scoring/routing applies to leads), and their own dedupe *within leads*.
3. **Promotion trigger = genuine engagement or qualification, NOT import and NOT an outbound touch we initiated.** A lead is promoted to `person` (contact) on:
   - an **inbound reply** or inbound contact from the prospect,
   - a **meeting booked or held**,
   - a **human rep explicitly qualifying/promoting** it,
   - (optional, conservative, ratifiable) a strong positive engagement signal (e.g. repeated link-clicks + form fill).
   A **cold outbound touch we sent with no response does NOT promote** — otherwise an SDR blast re-pollutes contacts. This is the load-bearing line.
4. **Promotion is a non-lossy merge into the one `person` model:** on promotion the lead becomes a `person` (or **merges into an existing matching `person`** via the §1.3 dedupe path), carrying its history + a `converted_from_lead_id` pointer; the transition is **audit-logged and reversible**. Because `lead` and `person` share the same field model, there is **no cross-object reporting seam** (this is how we get Salesforce's pollution protection without its lossy-convert + dual-object reporting pain).
5. **Deals attach to `person`/`organization`, not to raw leads.** A lead must promote (→ person, and its org resolved) before it can carry a `deal` — engagement precedes opportunity. (A lead may reference a candidate `organization` for routing/scoring without polluting the org graph; ratify whether candidate-orgs are also segregated.)

## Consequences

- **Positive:** the AI SDR and bulk enrichment can run at volume without polluting contacts; clean-core dedupe/relationship-strength/reporting stay trustworthy; promotion is auditable and non-lossy; one `person` model means no cross-object reporting seam.
- **Ripple (must be reflected):** `features/07 §3` (ICP surfacing), `stories/E06` (overnight/SDR), and `S-E01.3` (cold-start accounts) must state they **create leads, not contacts**; `features/03 §3` scoring/routing applies to the `lead` object; `features/01` gains a Leads section; capture (`features/02`) must route inbound engagement to the **promotion** path.
- **Negative / to bound:** a second object adds surface (lead list, lead → person promotion UI, the "is this an interaction?" classifier). Mitigated by keeping `lead` thin and sharing the `person` field model. The promotion-trigger definition (esp. what engagement counts, and the optional signal trigger) is the **one item to ratify** — get it wrong toward "promote eagerly" and pollution returns; toward "promote never" and real contacts get stuck as leads.
- **Boundary:** lead is a *standard* core object shipped by us, not a client metadata addition — consistent with P1/P2/P11.
