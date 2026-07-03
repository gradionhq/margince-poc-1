# ADR-0006 — A web-scrape / enrichment connector seam

**Status:** Accepted (2026-06-04) — built into build-plan WP9 and the `03-architecture` module map. Raised by foundation delta **FD-1** (`../foundation-deltas.md`) from the user-story work: the V1-WOW cold-start moment (`stories/epics/E01`) has no home in the current architecture or build plan.

## Context

Three V1-WOW stories depend on reading data that lives **outside** the user's mailbox/calendar:
- **S-E01.1** — paste a website URL → read back ICP / buying center / value prop / USP / buying-intents, each field showing the source snippet it was read from.
- **S-E01.2** — read a company's Impressum / legal imprint → company legal data, industry, history (kills address retyping).
- **S-E02.3 / S-E02.5** — signature scraping and a relationship-strength baseline (partly external enrichment).

The current `03-architecture.md` module map has `crm-capture` (email/calendar/call ingest) but **no module or interface for fetching and parsing arbitrary external web content**, and `11-mvp-build-plan.md` WP0–WP8 does not enumerate it. `04-customization-paradigm.md` already defines a **connector seam** ("add integration: MCP app/connector following the connector interface") — but for *inbound integrations the user authorizes* (mail, telephony), not for outbound web fetching of third-party sites. This is a genuine gap, not a misremembering.

Without a decision, the cold-start — the single highest-leverage activation moment per the competitor teardown (`research/landscape/04-momentum-teardown.md`) — has nowhere to be built.

## Decision

Add a first-class **scrape/enrichment connector seam** as a new capability under the existing connector interface (`04`), with these constraints:

1. **It is a connector, not core.** It implements the documented connector interface (per `AGENTS.md` §4.5), normalizes fetched data with provenance (`source`, `captured_by`), and is bound to scopes + a risk tier. This keeps it on the customizable seam, not in core internals.
2. **Provenance + evidence are mandatory, by construction.** Every field a fetch produces carries the source URL + the snippet it was read from. This is what makes the "never open with a guess" trust mechanic (S-E01.1) testable: a field with no evidence is omitted, never emitted.
3. **Egress ordering (ties to FD-2; A8 revised).** A **secret-stripper** runs on any externally fetched, model-bound payload on the *outbound* path (hygiene — **no PII pseudonymization**). Privacy is the location ladder (A8): in the `sovereign` profile no externally fetched payload leaves at all (zero-egress, tested); on `eu_hosted`/`cloud_frontier` the fetch→extract→model pipeline is gated before it leaves, not only before the model call.
4. **Robots/ToS + graceful degradation.** Fetches respect robots/ToS; failure degrades to a manual form and never blocks onboarding (S-E01.4 accept-to-persist gate stays the commit point).
5. **The third-party data provider (S-E01.3 ICP account-pull) is a separate, pluggable connector** behind the same seam, and is **Fast-follow** (it needs a paid data source) — not part of the V1 scrape seam.

## Consequences

- **Positive:** the V1-WOW cold-start and Impressum stories get a real, seam-aligned home without touching core; provenance-by-construction makes the trust mechanic verifiable; one interface covers website read, Impressum, signature enrichment, and (later) the data-provider pull.
- **Build-plan impact:** `11-mvp-build-plan.md` needs a WP addendum (a "scrape/enrichment connector" work package, WP2-adjacent). `03-architecture.md` module map gains the connector. These are the load-bearing edits this ADR authorizes once accepted.
- **Risk:** outbound web fetching adds an abuse/rate-limit surface (overlaps `api-rate-limits-and-abuse.md`) and a legal surface (scraping ToS, GDPR for company-level data) — both must be addressed in `06-nonfunctional`/`07-risks` before GA. Company-level only, consent-gated where personal data is involved (P12), consistent with the RED-removal of covert profiling.
- **Negative if rejected:** the cold-start moment cannot ship in V1, and E01 drops from V1-WOW to Backlog — materially weakening the launch's "holy shit" activation.
