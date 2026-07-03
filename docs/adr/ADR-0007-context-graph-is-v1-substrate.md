# ADR-0007 — The context graph is V1 substrate

**Status:** Accepted (2026-06-04, Lars). Resolves foundation delta **FD-7** (`../foundation-deltas.md`). Reverses the provisional "Fast-follow" stance in `stories/00-overview.md`. **Amended by [ADR-0021](ADR-0021-relational-pgvector-graph-trigger.md) (2026-06-17, R2)** — which names the measurable trigger this ADR's "add a graph store only if multi-hop reasoning at scale proves insufficient" deferral left open.

## Context

The product thesis (founder session, `BACKLOG.md`) is a "sales intelligence organism" whose moat is a **unified context graph** — every email, call, meeting, deal, and relationship connected as one picture, reasoned across as a whole. Several differentiating stories depend on it:
- **E05 Morning Brief** ("7 deals you can win this week") — ranks the *entire* pipeline, needs cross-everything reasoning.
- **E06 Overnight Agent** ("cleaned your mess while you slept", stalled-deal recovery) — reasons across all accounts at once.
- **Company-Aware Generation** and conversation-inferred deal health that feeds the brief.

The open question (FD-7) was whether to build this assembly + cross-pipeline reasoning layer in V1, or ship the one-source-at-a-time WOW moments (cold-start, Impressum, transcript→deal, dossier, warm-room) first and add the graph next release.

**Decision-maker's call:** build it from day one — *"foundation needs to be done properly."*

## Decision

The **context graph is V1 substrate.** Morning Brief (E05) and the field-hygiene / stalled-deal parts of the Overnight Agent (E06.1, E06.2) move into the **V1 line**.

**Crucially, "context graph in V1" defines a capability, not a new datastore.** It means we build, in V1:
1. The **assembly layer** — capture continuously links people/orgs/deals/activities/commitments into the clean relational core (real FKs + the typed `relationship` table, P11) so the connected picture exists, not just isolated records.
2. The **cross-pipeline reasoning layer** — queries/AI that reason over the *whole* graph (rank all deals, detect what changed, who's warm/blocking), with evidence + confidence on every inference (trust-or-it-dies).
3. **Hybrid retrieval** on Postgres: relational joins + `tsvector` full-text + `pgvector` embeddings — all in the one Postgres instance.

**What this decision does NOT do:** it does **not** adopt a dedicated graph database (Neo4j / GraphRAG). That remains deferred and ADR-gated per `03b` — we start on relational + pgvector (the relational core already encodes the relationships) and add a graph store only if multi-hop reasoning at scale proves insufficient. Building the context graph "properly" in V1 = the assembly + reasoning capability on the existing stack, done well — not a premature datastore bet (P1).

## Consequences

- **Positive:** the signature "organism" moments ship at launch, not a release later; the foundation (capture→link→reason) is built once, properly, instead of retrofitted; the moat that no field-by-field competitor can copy is present on day one.
- **Scope/timeline (honest):** this materially **expands the V1 line** — the V1 story count rises from 28 to ~37 (and, after the later table-stakes pass adding leads + client surfaces, to 48 of 61 total — see the live count in `stories/20-traceability.md`). V1 takes longer and carries more risk than the leaner "WOW-moments-first" path. This is the accepted trade for a properly-built foundation.
- **Build-plan impact:** `11-mvp-build-plan.md` needs a **context-graph substrate work package** (assembly + reasoning + hybrid retrieval) in Phase 1 — currently WP0–WP8 do not name it. This ADR authorizes that addendum.
- **Roadmap impact:** `08-roadmap.md` Phase-1 scope gains the context-graph substrate; predictive "next-best-action / brief" capabilities that `08` had in Phase 3 are partly pulled forward.
- **`03b` update:** the L3 "context & retrieval" item is no longer "evaluate a graph *layer* later" in the sense of the assembly/reasoning capability — that is now V1. Only the **dedicated graph-store** question stays deferred.
- **What stays Fast-follow (independent of the graph):** the **fill-the-calendar autonomous SDR** (S-E06.3) — blocked by channel ToS / outbound-voice / deliverability risk, not by the graph; **act-with-approval agent autonomy** (S-E10.2) — gated behind the prompt-injection red-team (`05-agent-security`), a separate safety milestone; **ICP account-pull** (S-E01.3, needs a paid data provider). These do not ride on the graph decision. *(Voice DNA S-E07.2 was later promoted to V1 — FD-14 — and is no longer Fast-follow.)*
