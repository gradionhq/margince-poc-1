---
derives-from:
  - margince specs/spec/product/00-overview.md#tier-taxonomy-the-product-priority-axis
  - margince specs/spec/product/20-traceability.md#tier-roll-up
  - margince specs/spec/product/epics/ (E01–E20)
---
# Scope — what ships, and what explicitly does not

This chapter is the scope of record. A capability is in the build if and only if it
is inside the V1 line (or the V0.5 launch gate) pinned below; everything else is
explicitly OUT — deferred with a named tier, or rejected outright on the never list.
A ticket that builds something not covered here is a spec defect, and so is a sales
or docs claim that promises something the OUT list defers.

Cut-lines in this spec are **already reconciled**: the source corpus accumulated its
scope through dated promotion decisions, and every chapter here states the
post-promotion truth. There is no override chain to apply when reading these docs —
that reconciliation happened at distillation and is this chapter's reason to exist.

## The tiers

Every story in the product carries exactly one tier. **V1-Must** is credibility: the
product cannot replace an incumbent for the core flow without it. **V1-WOW** is a
differentiating moment shipped in V1 — earned on emotion and demo, riding only on
V1-Must substrate. Together they are the V1 line. **V0.5** is the regulated-German
launch-readiness package — not a product feature that changes the core flow, and not
deferrable past the DACH beachhead launch (ADR-0038). **Fast-follow** lands one beat
after launch and is specified now so its shape is known. **Backlog** is deliberately
unsequenced.

Three OUT-side stories are fast-follow for reasons other than the context graph,
which is V1 substrate by decision (ADR-0007): the autonomous calendar-filling SDR
(channel terms-of-service risk), act-with-approval agent autonomy (gated on the
injection red-team), and ICP account surfacing (needs a paid data provider).

## How scope changes

Scope moves only by a docs change to this chapter — a tier flip is a reviewed,
human-merged PR citing a decision, and the affected chapters and their tickets
regenerate from it. Nothing is promoted by editing a ticket, a backlog entry, or a
chapter elsewhere.

## Appendix

### Scope — tier definitions
Source: product/00-overview.md#tier-taxonomy @ 5a0b29c

| ID | Tier | Assignment test |
|---|---|---|
| TIER-V1-MUST | V1-Must | "If this is missing, can the product dogfood-replace HubSpot for the core flow?" No → V1-Must. |
| TIER-V1-WOW | V1-WOW | All three: (a) a moment an incumbent's user has never felt, (b) on the senior-sales GREEN or founder must-have list, (c) rides only on V1-Must substrate. |
| TIER-V05 | V0.5 | "Can we sell to and operate for the regulated German Mittelstand without it?" No → V0.5. Own line, outside the V1 Must+WOW total; ready by V1 GA in DACH. |
| TIER-FF | Fast-follow | Consciously sequenced one beat past launch, or needs substrate not committed to V1. |
| TIER-BACKLOG | Backlog | Not yet sequenced: parked, north-star, or vision-only. |

Invariant TIER-INV-1: V1-Must ∪ V1-WOW = the V1 line = every capability this spec's
feature chapters mark IN. A story whose tier disagrees with its chapter's cut-line is
a tracked defect, never silently resolved.

### Scope — the V1 line (roll-up)
Source: product/20-traceability.md#tier-roll-up @ 5a0b29c

Totals: **97 V1 stories (61 Must + 36 WOW) across 17 product epics; + 5 V0.5
(Germany Package); + 21 Fast-follow (9 core + 12 overlay adapters); + 3 Backlog =
126 stories across 20 epics.** Story-level pins live in each owning chapter's
Acceptance appendix; this table is the epic-level inventory.

| ID | Epic | Must | WOW | V0.5 | FF | Backlog | Owning chapters (primary) |
|---|---|---|---|---|---|---|---|
| E01 | Onboarding & cold-start | 0 | 3 | — | 1 | — | onboarding-and-coldstart |
| E02 | Zero-entry capture | 6 | 2 | — | — | — | capture; people-and-organizations |
| E03 | Pipeline & deals | 7 | 0 | — | — | — | deals-and-pipeline; offers-and-products |
| E04 | Meetings & calls | 1 | 5 | — | — | — | meetings-and-transcripts |
| E05 | Morning Brief | 0 | 6 | — | — | — | morning-brief |
| E06 | Overnight agent | 0 | 2 | — | 1 | — | overnight-agent |
| E07 | Voice & drafting | 1 | 3 | — | — | — | drafting; voice-profile |
| E08 | Warm room & signals | 0 | 4 | — | 1 | 1 | signals-and-warm-room; deal-rooms |
| E09 | Reporting & forecast | 2 | 3 | — | 1 | — | reporting; forecasting |
| E10 | BYO agent & customization | 3 | 1 | — | 1 | 1 | byo-agent-and-mcp |
| E11 | Access, trust & exit | 12 | 0 | — | — | — | access-and-admin; import-export-migration; notifications-and-approval-inbox; gdpr-compliance-surfaces; sequences-and-deliverability (S-E11.9) |
| E12 | Client surfaces | 3 | 2 | — | 1 | — | client-surfaces |
| E13 | Leads & qualification | 5 | 1 | — | — | — | leads-and-qualification; lead-scoring |
| E14 | Suite with Dispact | 1 | 0 | — | 3 | 1 | dispact-integration |
| E15 | Operational depth | 10 | 1 | — | — | — | automation; custom-fields; lists-views-segmentation; data-hygiene; sequences-and-deliverability; records-depth; access-and-admin |
| E16 | Tasks & work queue | 1 | 1 | — | — | — | tasks-and-work-queue |
| E17 | Germany Package | — | — | 5 | — | — | germany-package |
| E18 | HubSpot overlay (substrate) | 9 | 2 | — | — | — | overlay-augmentation |
| E19 | Salesforce overlay | — | — | — | 6 | — | overlay-augmentation |
| E20 | Dynamics overlay | — | — | — | 6 | — | overlay-augmentation |

Note SCOPE-N-1: E15's eleven parent stories decompose into 25 ticket atoms (21
children + 4 single-atom parents); requirements derive from the children.

### Scope — OUT: deferred with a tier
Source: product/20-traceability.md#tier-roll-up @ 5a0b29c

This list is the single home of deferred stories (they have no owning feature
chapter in the V1 build).

| ID | Deferred capability | Tier | Why deferred |
|---|---|---|---|
| S-E01.3 | First ICP-matched accounts surfaced | Fast-follow | needs a paid data-provider connector |
| S-E06.3 | Fill-the-calendar virtual SDR | Fast-follow | outreach-channel ToS risk, not graph-gated |
| S-E08.3 | Resolve signal → company & person | Fast-follow | rides signal sensing maturity |
| S-E09.3 | Forecasting (weighted + scenario) | Fast-follow | consciously sequenced past launch |
| S-E10.2 | Promote agent to act-with-approval | Fast-follow | gated on the injection red-team probe |
| S-E12.4 | Clip a company from its website | Fast-follow | sequenced past launch |
| S-E14.1 | Deal channel auto-sync (Dispact) | Fast-follow | deeper suite integration |
| S-E14.2 | Act on a record from inside Dispact/Slack | Fast-follow | deeper suite integration |
| S-E14.3 | Owner change → Dispact briefing | Fast-follow | deeper suite integration |
| S-E19.1–.6 | Salesforce overlay adapter (6 stories) | Fast-follow | reuses the E18 substrate; sequenced after the HubSpot lead |
| S-E20.1–.6 | Dynamics overlay adapter (6 stories) | Fast-follow | reuses the E18 substrate; sequenced after the HubSpot lead |
| SCOPE-FF-TERR | Territory-based sharing | Fast-follow | the committed first fast-follow after record-level grants (ADR-0039) |
| S-E08.2 | Social/web signal sensing (scored) | Backlog | consent + quality bar not yet met |
| S-E10.4 | Marketing-and-sales-in-a-box modules | Backlog | north-star; code modules, never a builder |
| S-E14.4 | Internal champion/risk signals | Backlog | governance-gated: deal-scoped, evidence-backed, default-off, no per-employee profiles — or it does not ship |

### Scope — never (rejected patterns)
Source: contract/glossary.md, features/03 §5, narrative/05, narrative/06, epics @ 5a0b29c

Structural rejections. These are not deferrals; a ticket or chapter reintroducing one
is a defect. Each cites its deciding source.

| ID | Never | Decided by |
|---|---|---|
| NEVER-1 | A field-metadata table / dynamic-schema interpreter on the hot path — custom fields are real columns by migration | P1/P2/P11; ADR-0002 |
| NEVER-2 | A no-code visual workflow builder or rules DSL — automations are a closed catalog over typed in-code handlers | ADR-0035 |
| NEVER-3 | Hard delete in v1 — archive is the delete | data-model conventions |
| NEVER-4 | Per-AI-seat pricing, AI credit meters, or credit hard stops | S-E10.5; ADR-0047 |
| NEVER-5 | Gradion-operated production infrastructure — partners or customers host | ADR-0027 |
| NEVER-6 | A PII pseudonymization layer on model egress — privacy is the location ladder; the secret-stripper handles keys/tokens only | threat-model D7 (A8 revised) |
| NEVER-7 | Live-call surfaces and biometric/emotion/body-language inference — any sentiment output is post-call, text-only, draft, labeled | RED-removed guard (GATE-AI-6) |
| NEVER-8 | Covert profiling of external prospects — buyer signals are consent-gated and company-level | personas (Pat); ADR-0011 |
| NEVER-9 | Cloud-proxy capture aggregators — capture transport stays in-boundary; the messaging-channel exception is named and sovereign-excluded | ADR-0022; ACX.6 |
| NEVER-10 | A dedicated graph datastore in V1 — the graph is relational + vector; a graph store is trigger-gated, not roadmapped | ADR-0007; ADR-0021 |
| NEVER-11 | A separate agent ACL system — RBAC is the single authorization substrate agents map onto via Passport intersection | narrative 06 §6.2; ADR-0013 |
| NEVER-12 | An app/API marketplace — integrations are governed MCP connectors | features 04 §3 |
