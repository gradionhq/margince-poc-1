---
status: planned
module: backend/internal/modules/people (transport) + backend/internal/modules/directory (spine stores) + frontend/src/features/people
derives-from:
  - margince specs/spec/features/01-core-objects.md#1-people--contacts @ 5a0b29c
  - margince specs/spec/features/01-core-objects.md#2-organizations--companies @ 5a0b29c
  - margince specs/spec/contract/formulas-and-rules.md#1-dedupe-matching--person--org @ 5a0b29c
  - margince specs/spec/contract/formulas-and-rules.md#4-relationship-strength-baseline--recency--frequency--reciprocity @ 5a0b29c
  - margince specs/spec/contract/data-model.md#3-people @ 5a0b29c
  - margince specs/spec/contract/data-model.md#4-organizations @ 5a0b29c
  - margince specs/spec/contract/data-model.md#5-the-typed-relationship-table @ 5a0b29c
  - margince specs/spec/product/epics/E02-zero-entry-capture.md @ 5a0b29c
  - margince specs/spec/product/30-screen-acceptance.md#22-people--orgs @ 5a0b29c
---
# People & organizations — the relational spine that builds itself

> The contact and account base every other subsystem hangs off: people, organizations, and the one
> typed relationship edge between them — created mostly by capture, kept duplicate-free by an
> opinionated two-tier dedupe, and carrying three artifacts a hand-fed CRM never has: an explainable
> relationship-strength read, an evidence-backed company classification, and a face (logo or
> monogram) on every company.

<!--
Contested / flagged items (for the spec-gate reviewer):
1. S-E02.5 primacy: traceability maps it to features/07 §4 (capture activation view). The visible
   "activation" screen belongs to the capture/onboarding chapter; THIS chapter owns the strength
   formula (single home) and the strength surfaces on the contacts/person/companies/company screens.
2. formulas-and-rules §4 tags the org-level strength roll-up "[TS]", but the V1 screen acceptance
   (AC-companies-4/5, AC-company-2/3) and S-E02.5 (V1-WOW) require it on V1 surfaces. This chapter
   states the max-over-contacts roll-up as V1 truth per the screens; the stale tag should be
   reconciled upstream.
3. Contract/schema drift found: crm.yaml listPartners filters cert_status over
   {prospect, in_certification, certified, suspended, churned} while the partner DDL CHECK allows
   {applied, certified, suspended}. Needs an upstream fix; the DDL is pinned as written. Wider
   Partner drift: crm.yaml's Partner schema also diverges on partner_role nullability, models the
   gate metrics as a gate_metrics jsonb instead of typed columns, and omits
   id/workspace_id/joined_at/renews_at/provenance. The DDL is truth; contract alignment is a D-H2
   extension item (PO-EXT-5).
4. Org contract drift: crm.yaml's Organization schema lags PO-DDL-4 — the classification enum is
   missing values and is nullable vs NOT NULL DEFAULT 'prospect'; the relevance, logo_object_key
   and logo_origin columns are absent; listOrganizations lacks classification/relevance filter
   params. The DDL is truth; contract alignment is a D-H2 extension item (PO-EXT-4).
5. Corpus formula drift: the corpus formulas §1.1 worked example carries the wrong
   name_sim/confidence values (Jaro-Winkler arithmetic; corpus fix needed) — this chapter's worked
   example is corrected.
-->

## What it's for

A CRM is only as good as its answer to "who do we actually know, and how well?" This subsystem holds
that answer: the people and the organizations behind them, linked by one typed, history-preserving
relationship edge instead of a single overwritable company field. Its records are created mostly by
capture (a hand-typed record is a tracked smell), so its second job is defending its own quality —
never handing the user a silent duplicate, never letting machine enrichment clobber a human's
correction, and never showing a number or a label it cannot explain. Capture and enrichment write
into it; the leads chapter promotes engaged prospects into it; deals, reporting, the warm room and
the Morning Brief all read their people and accounts from it; four screens — the contacts list, the
person 360, the companies list, and the company 360 — are its user surface.

## Principles it serves

- **P11 — Clean relational core.** Real normalized tables, real keys; person-to-organization is the
  typed relationship edge with role and dates, never a directional association or a comma field.
  This is the headline anti-HubSpot win the whole spec leans on.
- **P5 — Auto-capture over manual entry.** People and companies create themselves from the domains
  and participants capture sees; dedupe and provenance exist so the auto-built population stays
  trustworthy. Manual creation works but is flagged as the rare path.
- **P12 — Governance designed in.** Every row carries its origin; every mutation is one audit entry
  plus one domain event; merges are one reversible-within-audit transaction; enrichment proposals
  carry evidence and never silently overwrite a human-set value (the A26 guard).
- **P4 — Blazing fast, always.** Record open, list, search and save budgets are acceptance criteria
  enforced in CI, not aspirations ([[acceptance-standards#PERF-1]]–[[acceptance-standards#PERF-4]]).
- **P8 — Beautiful by default.** Every organization renders a clean avatar everywhere it appears —
  a resolved logo or a deterministic monogram, never a broken image or a blank block (A55).
- **ADR-0032 — company classification + first-class partner object.** Classification is a real
  queryable field with an evidence-backed proposal path; partner-program state lives in a one-to-one
  extension of the organization, never duplicated firmographics.
- **ADR-0008 — lead segregation.** Machine-sourced prospects are structurally invisible to this
  chapter's dedupe, strength computation and reporting until genuine engagement promotes them.
- **ADR-0006 — the scrape/enrichment connector.** Classification, firmographics and the logo are
  derived from pages already being read — no third-party enrichment egress in V1 (P7/A8).

## How it works

**Creation is capture-first.** When capture meets a new participant, it creates exactly one person
and links them to a company derived from their email domain; the domain lookup is the
employer-inference path, and a proposed employer link is shown with its evidence for the user to
accept or correct. The free-mail-domain blocklist that prevents gmail-class domains becoming
organizations is the capture chapter's. Manual creation exists and is visibly the exception.

**Dedupe runs in two tiers, on every create and every capture.** Tier one is an exact unique-key
check — an email already on a live person, or a domain already on a live organization — and is the
only tier allowed to block or merge automatically: the API answers with a conflict carrying the
existing record's identity, and capture lands on the existing record. Tier two is a fuzzy confidence
score over name similarity and organization affinity; a score at or above the review threshold of
0.72 (DEDUPE_REVIEW_THRESHOLD) surfaces both records side by side as a confirm-first review item,
and below it the create proceeds. Fuzzy matches are never auto-merged, at any confidence
(DEDUPE_FUZZY_AUTOMERGE). Fuzzy candidate detection is V1 — promoted from the original fast-follow
cut line (S-E15.4); the review screen itself is the data-hygiene chapter's surface, built on this
chapter's formula and merge path.

**Merge is non-lossy and singular.** Merging record A into record B relinks A's emails, phones,
relationships and activity links to B with zero orphans, archives A with a pointer to B, and commits
as one audit transaction that is reversible within audit. Where survivor and loser conflict on a
uniquely-held slot — the primary email or phone, the current-primary employer — the survivor's value
wins and the loser's conflicting rows are demoted to non-primary on relink, never dropped. Both the
manual merge, the dedupe-review confirmation, and lead promotion reuse this same path — there is one
merge in the system. A merged
record announces itself with its own merged event, never as two generic updates
([[event-bus#events--semantic-rules]] EVT-SEM-2).

**Employment is an edge with history.** A person may have many employments over time but at most
one current primary employer, held by a database constraint. Title and role are attributes of the
relationship; a job change adds an edge with dates rather than overwriting a company field. The same
edge table carries deal stakeholders (pinned here, cited by the deals chapter) and the three
org-to-org partner edge kinds.

**Organizations enrich themselves, with evidence and a floor.** On create from a domain, enrichment
reads the page it already scraped: firmographics under evidence-or-omit, a proposed classification
with a relevance score, and the logo (site mark, favicon, or Impressum header). The logo is
normalized once into stored scaled variants; when none resolves, the render layer shows a
deterministic monogram — the company's initials on a colour stably hashed from its name — so no
surface ever shows a broken or blank avatar. Renaming a company changes its monogram colour by
design. A later machine-resolved value never silently replaces
a human-set classification or a human-uploaded logo; it surfaces as a confirm-first proposal.
Classification defaults honestly: an unconfident enrichment leaves the field at its default rather
than fabricating a label.

**Hierarchy is a tree, and it is multi-level in V1.** Parent/child company links form a real tree
with cycle prevention (no organization is its own ancestor); the multi-level tree and its roll-up
reporting were promoted into V1 (S-E15.8) and the traversal/roll-up acceptance lives with the
records-depth chapter. Partner relationships between companies are typed edges, deliberately
distinct from the corporate tree.

**Partner state is an extension, not a copy.** An organization is a partner exactly when it carries
the one-to-one partner extension — certification status, functional role, margin tier, and the gate
metrics. Role (what the partner does) and margin tier (what they earn) are orthogonal axes; the tier
is earned through application logic, not set freely. The partner object makes the program
representable in V1; running the program is later work.

**Relationship strength is deterministic and explainable.** For each person, strength is recency ×
frequency × reciprocity over the captured interaction history — a pure function: same interactions
plus a fixed clock yield the same 0–100 integer, decomposable to the exact activities behind it
("never a black-box badge"). A relationship with no captured interaction shows "no signal yet"
rather than a fabricated score, and a quiet relationship visibly decays. An account's strength is
the maximum over its people's strengths — a single warm champion makes the account warm; an average
would hide them. Leads are excluded from the computation by construction.

## What's configurable

All of this chapter's tunables are named source constants — one opinionated default each, no runtime
tuning UI in V1 (P1); changing one is a code edit and redeploy.

- **Dedupe review threshold** — the fuzzy confidence at which a candidate enters the review queue;
  default 0.72 (DEDUPE_REVIEW_THRESHOLD). Below it, no match; auto-merge is never fuzzy
  (DEDUPE_FUZZY_AUTOMERGE).
- **Dedupe factor weights** — name similarity 0.55 (DEDUPE_NAME_WEIGHT) and organization/domain
  affinity 0.45 (DEDUPE_ORGDOMAIN_WEIGHT); they sum to one so confidence stays in the unit interval.
- **Strength decay half-life** — 30 days (RELSTRENGTH_HALFLIFE_DAYS): how fast recency fades.
- **Strength frequency saturation** — 20 interactions in the window (RELSTRENGTH_FREQ_SATURATION):
  past this, more volume stops raising the score.
- **Strength reciprocity floor** — 0.25 (RELSTRENGTH_RECIPROCITY_FLOOR): a purely one-directional
  relationship still scores above zero but is heavily penalized.
- **The enrichment/scrape connector** (ADR-0006) — injected; deployment-dependent (sovereign
  deployments keep it zero-egress). When it is absent or a page is unreadable, the system degrades
  honestly: no classification proposal (the field stays at its default), no firmographics (omitted,
  not guessed), no logo (the monogram floor renders instead). Core record CRUD, dedupe and strength
  never depend on it.
- **Capture connectors** — strength is computed from captured history; a workspace with no capture
  shows "no signal yet" everywhere rather than inventing warmth.

## Guarantees (enforced)

- **No silent duplicate.** Creating a person with an email already on a live person, or an
  organization with a domain already mapped, is refused with the existing record's identity — held
  by live-scoped unique keys at the database, not handler discipline.
- **Fuzzy never auto-merges.** The only automatic merge trigger is an exact unique-key collision;
  every fuzzy candidate, at any confidence, becomes a side-by-side confirm-first review item.
- **Merge leaves nothing behind.** Merge relinks every child row and edge to the survivor with zero
  orphaned references, archives the loser with a pointer to the survivor, and is one audit
  transaction, reversible within audit ([[acceptance-standards#GATE-CORE-4]]).
- **At most one current primary employer per person** — a partial unique index; employment history
  is additive, never overwritten.
- **A domain maps to at most one live organization per workspace** — the employer-inference lookup
  can never be ambiguous.
- **Machine writes never clobber human writes.** Enrichment, classification and logo resolution
  propose with evidence; a human-set value is only replaced through an explicit confirm (A26).
- **Strength is reproducible and explainable.** A fixed seeded interaction set plus a fixed clock
  yields a stable score whose factor breakdown multiplies to exactly the displayed number, traceable
  to the contributing activities; no interactions means no number.
- **Every company has a face.** A resolved logo or a deterministic monogram — the same mark for the
  same company everywhere; never a broken image, never an empty slot.
- **Leads are invisible here.** Dedupe candidates, strength inputs and this chapter's lists draw
  from people and organizations only; a bulk-sourced prospect cannot pollute them until promoted
  (ADR-0008; [[event-bus#events--semantic-rules]] EVT-SEM-5).
- **Provenance and audit universality.** Origin and capturer are non-null on every row
  ([[acceptance-standards#GATE-CORE-3]]); every mutation emits exactly one audit entry and one
  domain event ([[acceptance-standards#GATE-CORE-5]]).
- **The budgets hold.** Record open, list, search and save meet the pinned p95 targets against the
  seeded dataset in CI ([[acceptance-standards#PERF-1]]–[[acceptance-standards#PERF-4]],
  [[acceptance-standards#GATE-CORE-6]]).

## Acceptance

Done means a rep can live in the contacts and companies lists and the two 360 views and trust every
pixel: each contact and account shows who it belongs to, where every field came from, how warm the
relationship is and why — with the arithmetic one click away — and each company shows its
classification with the evidence that proposed it and a clean mark in every context. It means the
system refused every silent duplicate along the way, routed every ambiguous match to a human, and
survived a merge with nothing orphaned and nothing lost. The honest states are part of done: an
empty workspace says so; a contact with no captured history says "no signal yet"; an unclassifiable
company stays unset; a logo that will not resolve becomes a stable monogram; a degraded connector
degrades the enrichment, never the record. The testable form of every claim lives in the Acceptance
appendix; the cross-cutting floor — standard screen states, performance budgets, release gates — is
inherited from the acceptance-standards chapter and not restated.

## Out of scope

- **The capture pipeline itself** — how mail and calendar become activities and new participants —
  is the capture chapter; this chapter owns what capture writes into.
- **The dedupe review screen** and the data-quality score belong to the data-hygiene chapter, built
  on this chapter's dedupe formula and merge path (cite the pins here, don't restate).
- **Leads and the promotion trigger** are the leads-and-qualification chapter; promotion re-enters
  this chapter only through the shared merge path.
- **Deals and stakeholder behaviour** are the deals-and-pipeline chapter; the stakeholder edge shape
  is pinned here because the relationship table has one home.
- **Consent, retention and legal hold** on people are the gdpr-platform chapter's substrate; the
  person 360 renders its consent card from there.
- **Hierarchy roll-up reporting and attachments** (S-E15.8) are the records-depth chapter; the
  parent link itself is pinned here.
- **Tenancy, provenance and audit mechanics** shared by every table are the data-model chapter's
  conventions (DM-CONV-12, DM-CONV-15, DM-CONV-17).
- **The warm-signal card content** (AC-person-5/6) is owned by the signals-and-warm-room chapter
  (not yet drafted); whether profiling-consent withdrawal suppresses the strength compute is that
  chapter's open consent question — noted here because the strength formula is ours.

## Where it lives

The people transport module at `backend/internal/modules/people`, with the entity and
store code in the shared spine at `backend/internal/modules/directory` — the skeleton
ships Person as transport + spine, and completing `modules/people` is the recorded
"decouple Person from the spine transaction" follow-up (skeleton@45aa8a7; pilot work
extends the spine in place rather than absorbing that decoupling). Every surface
reaches it through the datasource port ([datasource](datasource.md)); its screens ship
in the web app on design-system primitives. Read next: [datasource](datasource.md) for the seam, the data-model chapter for
tenancy/provenance conventions, and the capture, data-hygiene, leads-and-qualification and
deals-and-pipeline chapters for the neighbours that write into and read out of this spine.

## Appendix

### Parameters
Source: contract/formulas-and-rules.md#0-parameter-registry-all-tunables-one-place @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| `DEDUPE_FUZZY_AUTOMERGE` | Fuzzy auto-merge | *(never)* | Fuzzy never auto-merges; exact-key only. |
| `DEDUPE_REVIEW_THRESHOLD` | Fuzzy review threshold | `0.72` | Fuzzy confidence ≥ this → 🟡 review queue; below → ignored. |
| `DEDUPE_NAME_WEIGHT` | Name-similarity weight | `0.55` | Name-similarity contribution to fuzzy confidence. |
| `DEDUPE_ORGDOMAIN_WEIGHT` | Org/domain-match weight | `0.45` | Org/domain-match contribution to fuzzy confidence. |
| `RELSTRENGTH_HALFLIFE_DAYS` | Strength recency half-life | `30` | Recency decay half-life (days). |
| `RELSTRENGTH_RECIPROCITY_FLOOR` | Reciprocity floor | `0.25` | Minimum reciprocity multiplier (one-directional contact). |
| `RELSTRENGTH_FREQ_SATURATION` | Frequency saturation | `20` | Interaction count at which the frequency term saturates. |
| PO-PARAM-1 | Legal-suffix strip list | `{inc, llc, ltd, gmbh, ag, sa, sas, bv, oy, plc, co, corp, kg, ug}` | Case-insensitive trailing suffixes stripped by org-name normalization in the fuzzy tier (formulas §1 tunables line). |
| PO-PARAM-2 | Strength frequency window | `90d` | Absolute-duration window for the frequency count and the inbound/outbound reciprocity counts (formulas §4). |
| PO-PARAM-3 | Strength display buckets | `0–24 weak · 25–59 moderate · 60–100 strong` | Opinionated display bucketing of the 0–100 strength (formulas §4 output). |
| PO-PARAM-JW-1 | Name-similarity variant | standard Jaro-Winkler, prefix scale `p=0.1`, max prefix length `4`, no boost threshold | The exact string metric behind `name_sim` — pinned so the worked examples are reproducible. |
| PO-PARAM-JW-2 | Name-similarity preprocessing | casefold + unaccent | Inputs are casefolded and unaccented before comparison. |

All registry rows are source constants — no runtime tuning UI in V1 (P1); they do not appear in the
runtime-config boundary.

### Formulas
Source: contract/formulas-and-rules.md#11-person-dedupe + #12-org-dedupe + #4-relationship-strength-baseline--recency--frequency--reciprocity @ 5a0b29c

**PO-F-1 — Person dedupe (two tiers).** Single home; the data-hygiene chapter's review queue cites
this pin.

Inputs: `person_email.email` (normalized lowercase), `person.full_name`, the candidate's
current-primary employer `organization.id` via `relationship(kind='employment',
is_current_primary)`, and `organization_domain.domain`.

```
function dedupe_person(candidate):
  # --- TIER 1: EXACT KEY → block/merge, deterministic, no score ---
  hit = SELECT person_id FROM person_email
        WHERE workspace_id = :ws AND email = lower(candidate.email)
              AND archived_at IS NULL
  if hit exists:
      return { decision: EXACT_COLLISION, person_id: hit, action: BLOCK_OR_MERGE }
      # API: 409 + existing id (features/01 §1.3). Capture: land on existing person.

  # --- TIER 2: FUZZY → confidence score → review queue only ---
  best = 0; best_id = null
  for p in candidate_set(candidate):          # trigram-or-shared-org restricted, see edge cases
      c = confidence(candidate, p)
      if c > best: best = c; best_id = p.id
  if best >= DEDUPE_REVIEW_THRESHOLD:          # 0.72
      return { decision: FUZZY_REVIEW, person_id: best_id, confidence: best, action: QUEUE_🟡 }
  return { decision: NO_MATCH, action: CREATE }

confidence = DEDUPE_NAME_WEIGHT      * name_sim(a.full_name, b.full_name)
           + DEDUPE_ORGDOMAIN_WEIGHT * org_match(a, b)

name_sim  = Jaro-Winkler on lower(trim(unaccent(full_name)))   # 0..1 (PO-PARAM-JW-1/2)
org_match = 1.0  if same current-primary organization_id
          = 0.8  if email domains share the same org via organization_domain
          = 0.5  if free-text company strings normalize-equal (lowercase, strip legal suffixes, PO-PARAM-1)
          = 0.0  otherwise
```

Output: one of `{EXACT_COLLISION, FUZZY_REVIEW(confidence), NO_MATCH}` plus the matched person id.
EXACT → block (API conflict) or merge-onto (capture). FUZZY_REVIEW → a review-queue item showing
both records side by side (never an auto-merge). NO_MATCH → create. Weights sum to 1.0 so
`confidence ∈ [0,1]`.

Tie-breaks: exact-collision checks every email on the candidate; first hit wins, deterministic by
lowest person id. Fuzzy tier keeps the single best-scoring candidate; equal-confidence candidates
resolve to the lowest person id (a total order).

Worked example:
- New person `jane.doe@acme.com`, name "Jane Doe"; a live email row already holds
  `jane.doe@acme.com` → **EXACT_COLLISION**, return 409 + existing id. No scoring runs.
- New person "Jon Doe", `j.doe@acme.com`, no email collision; existing "John Doe" at Acme.
  `Jaro("jon doe","john doe") = (7/7 + 7/8 + 7/7)/3 = 0.9583`; Winkler prefix `l=2`, `p=0.1`
  (PO-PARAM-JW-1) → `name_sim = 0.9667`; both current-primary org = Acme → `org_match = 1.0`.
  `confidence = 0.55*0.9667 + 0.45*1.0 = 0.982 ≥ 0.72` → **FUZZY_REVIEW(0.982)** → 🟡 queue. Not merged.
- New person "Jon Doe" at Globex vs existing "John Doe" at Acme: `name_sim = 0.9667`, `org_match = 0`
  → `confidence = 0.55*0.9667 + 0.45*0.0 = 0.532 < 0.72` → **NO_MATCH**, create.

Edge cases: empty/null name → fuzzy tier skipped (exact-email only; a nameless captured contact
never fuzzy-matches). `candidate_set` is restricted to persons sharing a name trigram (GIN trigram
index — a build item, not a tunable) or sharing the candidate's org, to stay inside the create
budget. An archived person's email is excluded by `uq_person_email_dedupe`, so a merged-away person
frees its email. Leads never appear (ADR-0008 segregation).

**PO-F-2 — Org dedupe.**

Inputs: `organization_domain.domain`, `organization.display_name`, `organization.legal_name`.

```
function dedupe_org(candidate):
  # EXACT: any candidate domain already maps to a live org (uq_org_domain)
  hit = SELECT organization_id FROM organization_domain
        WHERE workspace_id=:ws AND domain = lower(candidate.domain) AND archived_at IS NULL
  if hit: return { decision: EXACT_COLLISION, organization_id: hit }

  # FUZZY: name similarity only (no domain to anchor)
  for o in name_trigram_candidates(candidate.display_name):
      c = name_sim(norm(candidate.display_name), norm(o.display_name))   # norm strips legal suffixes (PO-PARAM-1)
      track best
  if best >= DEDUPE_REVIEW_THRESHOLD: return FUZZY_REVIEW(best, best_id)
  return NO_MATCH
```

Output and tie-breaks mirror PO-F-1. Domain collision → exact merge onto the existing organization
(this is also the capture employer-inference path). Worked example: `"Acme Inc"` vs `"Acme GmbH"`
normalize to `"acme"` → `name_sim = 1.0 ≥ 0.72` → 🟡 review (different legal entities, a human
decides). No domain and no name match → create.

**PO-F-3 — Relationship strength (person) + org roll-up.** Deterministic; leads excluded
(ADR-0008); computed over live activities only.

Inputs: for a person, all live `activity` rows linked via `activity_link(person_id=...)` of
`kind ∈ {email, call, meeting}`, each with `occurred_at` and a direction (`inbound`/`outbound`,
from the real direction column). Workspace-wide (team-wide, not per-rep — AC-person-2).

```
strength = round( 100 * recency * frequency * reciprocity )

recency    = 2^( -days_since(last_interaction.occurred_at) / RELSTRENGTH_HALFLIFE_DAYS )   # 0..1, 30-day half-life
frequency  = min(1.0, interaction_count_90d / RELSTRENGTH_FREQ_SATURATION)                 # 0..1, saturates at 20
reciprocity = RELSTRENGTH_RECIPROCITY_FLOOR
            + (1 - RELSTRENGTH_RECIPROCITY_FLOOR) * balance
            where balance = 1 - |inbound - outbound| / (inbound + outbound)                 # 1=two-way, 0=one-way

org_strength = max over the org's people's strengths      # a strong single relationship makes the account warm
```

Output: integer 0–100 per person plus the contributing activity ids (clickable, "no mystery
number"); display buckets per PO-PARAM-3. Org output: the max, plus the top contact named. No
interactions → strength is undefined: shown as "no interactions yet" / "no signal yet", never 0
rendered as a score.

Tie-breaks: none needed for the person score (pure arithmetic). When naming the org's strongest
contact and several tie at the max, the surfaced contact follows the deterministic order pinned for
warm-route picking — highest strength, then most recent last interaction, then lowest person id
(sanctioned restatement of formulas §9 `pick_route`).

Worked example (fixed clock `now = 2026-06-04`): last interaction 5 days ago →
`recency = 2^(-5/30) = 0.891`; 12 interactions in 90 days → `frequency = min(1, 12/20) = 0.60`;
7 inbound / 5 outbound → `balance = 1 - |7-5|/12 = 0.833`,
`reciprocity = 0.25 + 0.75*0.833 = 0.875`;
`strength = round(100 * 0.891 * 0.60 * 0.875) = round(46.8) = 47` → moderate.

Edge cases: all one-directional (0 inbound) → `balance = 0`, `reciprocity = 0.25` → capped low even
if recent and frequent. A single interaction today → strong recency but tiny frequency → low
strength (one email is not a relationship). Zero-window edge: a person with interactions ever but a
windowed (90d, PO-PARAM-2) inbound+outbound count of 0 → `strength = 0`, displayed as weak with
"no recent activity" — distinct from "no signal yet", which is zero interactions ever; the balance
term is never evaluated as 0/0. Decay is computed from `occurred_at` at read time, so
recompute is idempotent under a fixed clock.

<!-- formulas §4 tags the org roll-up "[TS]"; the V1 screens (AC-companies-4/5, AC-company-2/3)
and S-E02.5 require it — pinned as V1 here, tag reconciliation flagged upstream. -->

### Schema
Source: contract/data-model.md#31-person + #32-person_email--ordered-1-primary-per-type + #33-person_phone--same-shape + #41-organization + #42-organization_domain--normalized-lowercased-unique-per-org + #43-partner--first-class-partner-program-object-11-extension-of-organization + #5-the-typed-relationship-table @ 5a0b29c

Seven tables, per the data-model chapter's ownership index. Conventions (UUIDv7 keys, tenancy +
RLS, provenance columns, archive-cascade, naming) are the data-model chapter's DM-CONV pins and are
not restated; the legal-hold column added to person and organization ships with that chapter's
retention substrate.

**PO-DDL-1 — `person`**

```sql
CREATE TABLE person (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  first_name    text NULL,
  last_name     text NULL,
  full_name     text NOT NULL,           -- always present (display); split names optional
  title         text NULL,               -- denormalized current title for display; authoritative title is on relationship (§5)
  owner_id      uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  social        jsonb NOT NULL DEFAULT '{}'::jsonb, -- {linkedin, twitter, github,...} [TS]
  address       jsonb NULL,              -- structured postal address [TS]

  -- dedupe / merge (features/01 §1.3)
  merged_into_id        uuid NULL REFERENCES person(id) ON DELETE SET NULL, -- set when this row was merged AWAY
  -- lead promotion (ADR-0008 §4; features/01 §6.4)
  converted_from_lead_id uuid NULL REFERENCES lead(id) ON DELETE SET NULL,  -- forward pointer; person knows its lead origin

  -- provenance (§1.6)
  source        text NOT NULL,
  captured_by   text NOT NULL,
  raw           jsonb NULL,

  -- full-text (search budget < 200ms, 03 §3.5)
  search_tsv    tsvector GENERATED ALWAYS AS (
                  to_tsvector('simple',
                    coalesce(full_name,'') || ' ' || coalesce(title,''))
                ) STORED,

  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL
);

CREATE INDEX idx_person_ws_live      ON person (workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_person_owner        ON person (workspace_id, owner_id) WHERE archived_at IS NULL;
CREATE INDEX idx_person_search       ON person USING gin (search_tsv);
CREATE INDEX idx_person_merged_into  ON person (merged_into_id) WHERE merged_into_id IS NOT NULL;
CREATE INDEX idx_person_from_lead    ON person (converted_from_lead_id) WHERE converted_from_lead_id IS NOT NULL;
```

Note: dedupe enforcement is on `person_email` (the exact-email unique key), not here — that is what
makes "create with an existing email returns the conflict + existing id" honest. Merge of A→B sets
`A.merged_into_id = B.id`, archives A, and relinks A's `person_email`/`person_phone`/
`relationship`/`activity_link` rows to B in one transaction (zero orphaned FKs).

**PO-DDL-2 — `person_email`**

```sql
CREATE TABLE person_email (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  person_id    uuid NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  email        text NOT NULL,                 -- stored lowercased
  email_type   text NOT NULL DEFAULT 'work' CHECK (email_type IN ('work','personal','other')),
  is_primary   boolean NOT NULL DEFAULT false,
  position     integer NOT NULL DEFAULT 0,    -- explicit ordering (features/01 §1.1 "ordered rows")
  source       text NOT NULL,
  captured_by  text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL,

  CONSTRAINT person_email_norm CHECK (email = lower(email))
);

-- ≤1 primary per (person, type)  [features/01 §1.1 DB-enforced]
CREATE UNIQUE INDEX uq_person_email_primary
  ON person_email (person_id, email_type)
  WHERE is_primary AND archived_at IS NULL;

-- dedupe key: an email is unique across LIVE persons in a workspace (features/01 §1.3 → 409 on collision)
CREATE UNIQUE INDEX uq_person_email_dedupe
  ON person_email (workspace_id, email)
  WHERE archived_at IS NULL;

CREATE INDEX idx_person_email_person ON person_email (person_id) WHERE archived_at IS NULL;
```

`uq_person_email_dedupe` is the structural anti-duplicate guarantee; it is workspace-scoped and
excludes archived rows so a merged-away person's email frees up.

**PO-DDL-3 — `person_phone`**

```sql
CREATE TABLE person_phone (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  person_id    uuid NOT NULL REFERENCES person(id) ON DELETE CASCADE,
  phone        text NOT NULL,                  -- E.164 normalized at write
  phone_type   text NOT NULL DEFAULT 'work' CHECK (phone_type IN ('work','mobile','home','other')),
  is_primary   boolean NOT NULL DEFAULT false,
  position     integer NOT NULL DEFAULT 0,
  source       text NOT NULL,
  captured_by  text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz NULL
);
CREATE UNIQUE INDEX uq_person_phone_primary
  ON person_phone (person_id, phone_type)
  WHERE is_primary AND archived_at IS NULL;
CREATE INDEX idx_person_phone_person ON person_phone (person_id) WHERE archived_at IS NULL;
```

**PO-DDL-4 — `organization`**

```sql
CREATE TABLE organization (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  display_name  text NOT NULL,
  legal_name    text NULL,
  industry      text NULL,        -- [TS] taxonomy later
  size_band     text NULL CHECK (size_band IS NULL OR size_band IN ('1-10','11-50','51-200','201-500','501-1000','1001-5000','5000+')),
  address       jsonb NULL,
  owner_id      uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,

  -- company classification (A3 / A41 / ADR-0032) — V1 core field; set by enrichment (ADR-0006) or by hand,
  -- 🟢 reversible inference surfaced with evidence, never auto-overwriting a human-set value.
  classification text NOT NULL DEFAULT 'prospect'
                  CHECK (classification IN ('prospect','customer','agency','reseller','tech_vendor','platform','partner','competitor','other')),
  relevance     smallint NULL CHECK (relevance IS NULL OR relevance BETWEEN 0 AND 100),

  -- visual identity (A55) — logo resolved on enrichment from the already-scraped page (ADR-0006:
  -- og:image → favicon → Impressum mark; zero new egress, no third-party logo API, P7/A8) and
  -- normalized to stored square variants; null → the render layer shows a deterministic monogram
  -- (initial(s) + a colour hashed from the org NAME, per the screen AC — rename changes the
  -- monogram colour by design). A 🟢 display-asset write; an auto-resolved
  -- value never overwrites a human-uploaded logo without a 🟡 confirm (A26). Provenance is carried
  -- on the row's source/captured_by (captured_by='agent:logo-resolve' when auto-resolved).
  -- Stored as object-store references only (the §10.3 attachment convention — S3/MinIO, never blobs in the DB).
  logo_object_key text NULL,      -- normalized logo variants in object storage (base key; sm/md/lg derived)
  logo_origin      text NULL,     -- resolved source URL (logo-specific provenance, beyond row captured_by)

  -- hierarchy (features/01 §2.2) — single-level FK + cycle prevention
  parent_org_id uuid NULL REFERENCES organization(id) ON DELETE SET NULL,

  merged_into_id uuid NULL REFERENCES organization(id) ON DELETE SET NULL,

  source        text NOT NULL,
  captured_by   text NOT NULL,
  raw           jsonb NULL,

  search_tsv    tsvector GENERATED ALWAYS AS (
                  to_tsvector('simple',
                    coalesce(display_name,'') || ' ' || coalesce(legal_name,'') || ' ' || coalesce(industry,''))
                ) STORED,

  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,

  CONSTRAINT organization_not_own_parent CHECK (parent_org_id IS NULL OR parent_org_id <> id)
);
CREATE INDEX idx_org_ws_live   ON organization (workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_org_owner     ON organization (workspace_id, owner_id) WHERE archived_at IS NULL;
CREATE INDEX idx_org_parent    ON organization (parent_org_id) WHERE parent_org_id IS NOT NULL;
CREATE INDEX idx_org_class     ON organization (workspace_id, classification) WHERE archived_at IS NULL;
CREATE INDEX idx_org_search    ON organization USING gin (search_tsv);
```

Note: the CHECK blocks the trivial self-parent cycle; deeper cycle prevention (no org is its own
ancestor) is enforced by a `BEFORE INSERT/UPDATE` trigger running a recursive ancestor walk, since a
plain CHECK cannot express transitive acyclicity. The "group view" roll-up is a recursive CTE over
`parent_org_id` (budget `< 200ms` for ≤200-org trees); the multi-level tree + roll-up reporting is
V1 (S-E15.8 promotion of record), with the roll-up acceptance owned by the records-depth chapter.

**PO-DDL-5 — `organization_domain`**

```sql
CREATE TABLE organization_domain (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  organization_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
  domain          text NOT NULL,        -- lowercased, no scheme, no www
  is_primary      boolean NOT NULL DEFAULT false,
  source          text NOT NULL,
  captured_by     text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL,
  CONSTRAINT org_domain_norm CHECK (domain = lower(domain))
);
-- a domain maps to at most one org per workspace (features/01 §2.1 "unique per org")
CREATE UNIQUE INDEX uq_org_domain ON organization_domain (workspace_id, domain) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX uq_org_domain_primary ON organization_domain (organization_id) WHERE is_primary AND archived_at IS NULL;
CREATE INDEX idx_org_domain_org ON organization_domain (organization_id) WHERE archived_at IS NULL;
```

The domain → org index is also the auto-create / employer-inference lookup: capture parses an email
domain, looks it up here, and proposes the org link.

**PO-DDL-6 — `partner`**

```sql
CREATE TABLE partner (
  id                 uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id       uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  organization_id    uuid NOT NULL UNIQUE REFERENCES organization(id) ON DELETE CASCADE,  -- 1:1 with the company record

  -- program lifecycle (A38/ADR-0030)
  cert_status        text NOT NULL DEFAULT 'applied'
                       CHECK (cert_status IN ('applied','certified','suspended')),
  partner_role       text NULL CHECK (partner_role IS NULL OR partner_role IN ('hosting','consulting','strategic')),  -- functional role (A44/ADR-0034): what the partner does; ORTHOGONAL to margin_tier. Primary role only in V1 (multi-hat firms — e.g. hosting+consulting Systemhäuser — are common; roles[] is a Phase-2 nicety). No 'implementation'/'developer': source dev is Gradion's turf, not a recruited channel.
  margin_tier        text NULL CHECK (margin_tier IS NULL OR margin_tier IN ('tier1_15','tier2_20','tier3_25')),  -- buy-low margin; NOT a recruiter override; a DIFFERENT axis from partner_role
  certified_staff    smallint NOT NULL DEFAULT 0,   -- gates the tier (certified-staff headcount)
  retention_rate     smallint NULL CHECK (retention_rate IS NULL OR retention_rate BETWEEN 0 AND 100),  -- a tier gate metric
  joined_at          date NULL,
  renews_at          date NULL,

  source             text NOT NULL,
  captured_by        text NOT NULL,
  raw                jsonb NULL,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  archived_at        timestamptz NULL
);
CREATE INDEX idx_partner_ws_live ON partner (workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_partner_tier    ON partner (workspace_id, margin_tier) WHERE archived_at IS NULL;
```

The tier is earned, not set freely: `margin_tier` is gated on `certified_staff` + volume +
`retention_rate` per A38 — application logic (not a DB constraint) enforces the gate; tier changes
are auditable like any field. Partner-sourced revenue/attribution is read from the deal's partner
pointer (deals-and-pipeline chapter), not stored here.

**PO-DDL-7 — `relationship`**

```sql
CREATE TABLE relationship (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  -- partner edge-kinds (A41/ADR-0032): partner_of (org served-by partner org), referred_by (org referred by
  -- partner org), co_sell_with (org co-sold with partner org). Deal-level registration/attribution is
  -- `deal.partner_org_id` (§6), not an edge here.
  kind          text NOT NULL CHECK (kind IN ('employment','deal_stakeholder','partner_of','referred_by','co_sell_with')),

  -- participants (exactly the pair required by `kind`; enforced by the CHECKs below)
  person_id          uuid NULL REFERENCES person(id) ON DELETE CASCADE,
  organization_id    uuid NULL REFERENCES organization(id) ON DELETE CASCADE,
  counterparty_org_id uuid NULL REFERENCES organization(id) ON DELETE CASCADE,  -- the partner org, for org↔org partner edges
  deal_id            uuid NULL REFERENCES deal(id) ON DELETE CASCADE,

  role          text NULL,          -- employment: 'cto','vp_sales',... ; stakeholder: 'champion','economic_buyer','blocker','influencer','user'
  is_current_primary boolean NOT NULL DEFAULT false, -- employment: the one current primary employer
  started_at    date NULL,
  ended_at      date NULL,          -- NULL = current/ongoing

  source        text NOT NULL,
  captured_by   text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,

  -- shape per kind
  CONSTRAINT rel_employment_shape CHECK (
    kind <> 'employment' OR (person_id IS NOT NULL AND organization_id IS NOT NULL AND deal_id IS NULL)
  ),
  CONSTRAINT rel_stakeholder_shape CHECK (
    kind <> 'deal_stakeholder' OR (deal_id IS NOT NULL AND person_id IS NOT NULL AND organization_id IS NULL)
  ),
  -- partner edges are org↔org: organization_id (the served/referred/co-sold org) + counterparty_org_id (the partner org)
  CONSTRAINT rel_partner_shape CHECK (
    kind NOT IN ('partner_of','referred_by','co_sell_with')
    OR (organization_id IS NOT NULL AND counterparty_org_id IS NOT NULL
        AND organization_id <> counterparty_org_id AND person_id IS NULL AND deal_id IS NULL)
  ),
  CONSTRAINT rel_dates CHECK (ended_at IS NULL OR started_at IS NULL OR ended_at >= started_at)
);

-- exactly ≤1 CURRENT-PRIMARY employer per person (features/01 §1.2 constraint)
CREATE UNIQUE INDEX uq_rel_current_primary_employer
  ON relationship (person_id)
  WHERE kind = 'employment' AND is_current_primary AND archived_at IS NULL;

-- "all people at org X" indexed join  (budget < 150ms, features/01 §1.2)
CREATE INDEX idx_rel_org_people
  ON relationship (workspace_id, organization_id)
  WHERE kind = 'employment' AND archived_at IS NULL;

-- reverse: a person's orgs / employment history
CREATE INDEX idx_rel_person_orgs
  ON relationship (person_id)
  WHERE kind = 'employment' AND archived_at IS NULL;

-- deal stakeholders, both directions (budget < 150ms, features/01 §3.2)
CREATE INDEX idx_rel_deal_stakeholders
  ON relationship (workspace_id, deal_id)
  WHERE kind = 'deal_stakeholder' AND archived_at IS NULL;
CREATE INDEX idx_rel_stakeholder_deals
  ON relationship (person_id)
  WHERE kind = 'deal_stakeholder' AND archived_at IS NULL;

-- a stakeholder appears once per (deal, person, role)
CREATE UNIQUE INDEX uq_rel_deal_person_role
  ON relationship (deal_id, person_id, role)
  WHERE kind = 'deal_stakeholder' AND archived_at IS NULL;

-- partner edges, both directions (A41/ADR-0032): "which orgs does partner X serve/refer/co-sell" and reverse
CREATE INDEX idx_rel_partner_counterparty
  ON relationship (workspace_id, counterparty_org_id)
  WHERE kind IN ('partner_of','referred_by','co_sell_with') AND archived_at IS NULL;
CREATE INDEX idx_rel_partner_org
  ON relationship (workspace_id, organization_id)
  WHERE kind IN ('partner_of','referred_by','co_sell_with') AND archived_at IS NULL;
```

One table with a `kind` discriminator + shape CHECKs is chosen over two tables because the spec
consistently calls it "the typed `relationship` table" (singular) and reporting wants one edge
surface to join. `uq_rel_current_primary_employer` is the DB-level "≤1 current-primary org"
guarantee. The `deal_stakeholder` rows' behaviour belongs to the deals-and-pipeline chapter, which
cites this pin.

### Wire
Source: contract/crm.yaml (paths /people, /people/{id}, /people/{id}/merge, /organizations, /organizations/{id}, /organizations/{id}/merge, /organizations/{id}/partner, /partners) @ 5a0b29c

Operations cited by `operationId`; schemas are never restated here. Status/error semantics follow
the conventions register ([[api-conventions#wire--conventions-register]] API-CONV-1..6,
[[api-conventions#wire--concurrency--idempotency]] API-CC-1..7). Sort/filter vocabularies:
DM-VOCAB-1 (people), DM-VOCAB-2 (organizations), DM-VOCAB-6 (partners) — data-model chapter.

| ID (operationId) | Operation | Tool verb / tier | Notes |
|---|---|---|---|
| `listPeople` | People list (live by default, cursor-paginated) | `search_records` / 🟢 | Leads never appear here (ADR-0008). Full-text `q`, `owner_id`, `tag` filters. |
| `createPerson` | Create person | `create_record` / 🟢 | 201 + Location + full entity (API-ERR-1); live-email collision → 409 `duplicate_email` with `details.existing_id` (API-ERR-7); idempotency key honoured (API-CC-6). |
| `getPerson` | Person 360 read | `read_record` / 🟢 | Fetchable by id even when archived. |
| `updatePerson` | Partial update | `update_record` / 🟢 | PATCH-merge (API-CONV-1); version concurrency (API-CC-2..4, API-ERR-8). |
| `archivePerson` | Archive (soft delete) | `update_record` / 🟡 | Sets the archive timestamp, drops from default lists, stays fetchable + audited; 200 + full entity (API-CONV-2). |
| `mergePerson` | Merge A into target B (non-lossy) | `merge_records` / 🟡 | Approval-token bound — the always-🟡 floor includes merge ([[threat-model#D4]]); crm.yaml currently lacks the ApprovalToken parameter on it (PO-EXT-8). Relinks emails/phones/relationships/activity links with zero orphaned FKs; archives A with the merged-into pointer; one audit transaction; reversible within audit. |
| `listOrganizations` | Organizations list | `search_records` / 🟢 | `domain` filter is the employer-inference lookup. |
| `createOrganization` | Create organization | `create_record` / 🟢 | Domains normalized/lowercased, unique per workspace — collision → 409 Problem. |
| `getOrganization` | Org 360 read | `read_record` / 🟢 | |
| `updateOrganization` | Partial update | `update_record` / 🟢 | Same PATCH-merge + concurrency semantics. |
| `archiveOrganization` | Archive (soft delete) | `update_record` / 🟡 | |
| `mergeOrganization` | Merge org A into target B (non-lossy) | `merge_records` / 🟡 | Approval-token bound; relinks domains, employment, deals, relationships, activity links; mirrors the person merge; the org half of the `merge_records` verb. |
| `upsertPartner` | Create/update the partner extension on an org | — / 🟢 | Requires the admin RBAC permission — "admin" is a permission, not a tier. Sets classification to partner; company identity never duplicated. |
| `getPartner` | Read the partner extension | — | 404 when the org is not a partner. |
| `listPartners` | List partner orgs, filter by role/cert status | `search_records` / 🟢 | <!-- cert_status filter enum drifts from the partner DDL CHECK — flagged in the header comment. --> |

**PO-N-VOCAB** (reconcile note) — DM-VOCAB-1/2 and crm.yaml disagree on the sort/filter
vocabularies: for people, `organization_id` + `archived` vs `tag`; for organizations,
`industry`/`size_band`/`classification` vs `domain`/`q`. DM-VOCAB is the docs oracle; crm.yaml
alignment is a contract-extension item (see Wire — contract extensions). The `tag` vocabulary
itself is owned by the lists-views-segmentation chapter.

Consent read/grant operations on a person are the gdpr-platform chapter's wire surface, not
restated here.

### Wire — contract extensions (D-H2)
Source: chapter review 2026-07-03 (contract-drift findings against contract/crm.yaml @ 5a0b29c)

These are contract-first extension tickets — crm.yaml grows before any handler is written.

| ID | Extension | Notes |
|---|---|---|
| PO-EXT-1 | Strength fields on person list/detail reads | `score`, `bucket`, and the three factor values on the person list and detail reads; `strength` as a sort key (AC-contacts-3/4/7, PO-N-FILTER). |
| PO-EXT-2 | Strength-breakdown read | The contributing activities behind a person's score, for the drawer (AC-person-3/4). |
| PO-EXT-3 | Composite person-360 and organization-360 reads | Per the one-composite-read doctrine (architecture/frontend.md §composite reads): record + relationships + deals + recent activities in one round trip. |
| PO-EXT-4 | Organization classification/relevance/logo schema + filter alignment | Aligns crm.yaml with PO-DDL-4: classification enum values + NOT NULL DEFAULT 'prospect', relevance, logo_object_key, logo_origin; classification/relevance filter params on `listOrganizations` (header drift flag 4). |
| PO-EXT-5 | Partner schema alignment | Aligns crm.yaml with PO-DDL-6: cert_status enum, partner_role nullability, typed gate-metric columns instead of a gate_metrics jsonb, plus id/workspace_id/joined_at/renews_at/provenance (header drift flag 3). |
| PO-EXT-6 | Restore (un-archive) operations for person/organization | The pinned `person.restored`/organization events currently have no trigger operation. |
| PO-EXT-7 | `createOrganization` 409 carries `details.existing_id` | Parity with the person path (API-ERR-7). |
| PO-EXT-8 | `mergePerson` ApprovalToken parameter | Binds the 🟡 approval token to the merge (see the Wire row). |

Compact response shapes where a shape is load-bearing:

```json
// PO-EXT-1 — strength block on person list/detail reads
"strength": { "score": 47, "bucket": "moderate",
              "recency": 0.891, "frequency": 0.60, "reciprocity": 0.875 }
```

```json
// PO-EXT-3 — composite person-360 read (organization-360 mirrors it)
{ "person": { }, "relationships": [ ], "deals": [ ], "activities": [ ] }
```

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c — definitions live in the central catalog ([[event-bus#events--catalog]]); cited, never redefined.

All nine are emitted by this module (`modules/people` in target-layout vocabulary) on the person and
organization streams (EVT-STREAM-1/2).

| ID | Direction | Note |
|---|---|---|
| `person.created` | emitted | Carries the lead-origin pointer when the create came from promotion. |
| `person.updated` | emitted | Delta-carrying; never fired for a merge (EVT-SEM-2). |
| `person.archived` | emitted | |
| `person.merged` | emitted | Own verb, not two updates: the context graph collapses two nodes and relinks edges atomically (EVT-SEM-2). |
| `person.restored` | emitted | |
| `organization.created` | emitted | Carries primary domain + parent pointer when present. |
| `organization.updated` | emitted | Includes enriched firmographics/classification deltas. |
| `organization.archived` | emitted | |
| `organization.merged` | emitted | |

`consent.changed` and `retention.applied` ride the person stream but belong to the gdpr-platform
chapter; `lead.promoted` is emitted by the leads module and lands in this chapter only as the
`person.created`/merge it causes (EVT-SEM-5).

### Acceptance
Source: product/epics/E02-zero-entry-capture.md#s-e025--relationship-strength-baseline-as-instant-value + #s-e027--company-classification-set-with-evidence + #s-e028--every-company-shows-its-face-logo-on-create-never-a-blank-block @ 5a0b29c

**PO-N-PILOT** (pilot scope note) — the first-restart pilot excludes the enrichment/capture-dependent
acceptance set: S-E02.7 (classification, including its confirm-surface AC PO-AC-29), S-E02.8 (logo;
variant sizes/normalization are unpinned — they arrive with its ticket), AC-company-4/5/6
firmographics (no schema/wire exists — the schema extension arrives with the enrichment tickets),
AC-contacts-1/AC-companies-1 capture-banner counts, and PO-AC-10. Strength (S-E02.5) stays IN the
pilot — deterministic, computed from existing activity data.

**Owned stories** (tiers verified against the traceability register).

| ID | Tier | Given/When/Then (condensed) | Verification |
|---|---|---|---|
| S-E02.5 | V1-WOW | Given captured email/meeting history, when the backfill completes, then each contact/account shows a relationship-strength indicator computed from real interaction frequency, recency and direction — never a field the user filled; any score is traceable to the interactions behind it; no captured interaction → an honest "no signal yet"; a relationship going quiet visibly decays. | PO-F-3 golden test (fixed seed + fixed clock → stable value), unit lane ([[testing#TEST-LANE-1]]); strength surfaces via AC-contacts-3..5, AC-person-2..4, AC-companies-4/5, AC-company-2/3 (live-stack lane [[testing#TEST-LANE-3]]). |
| S-E02.7 | V1-Must | Given an auto-created or existing organization, when enrichment runs, then it proposes a classification + relevance score with the evidence it inferred from; a human-set value is marked as such and never silently overwritten (re-enrichment → 🟡 confirm); the workspace can filter/segment by classification and relevance as real queryable fields; an unconfident enrichment shows unset/"other", never a fabricated label; prior values and who/what set each version are audit-preserved. | Integration lane ([[testing#TEST-LANE-2]]): classification column + index (PO-DDL-4), filter vocabulary DM-VOCAB-2, never-overwrite guard; audit floor [[acceptance-standards#GATE-CORE-5]]. |
| S-E02.8 | V1-Must | Given a company created from a domain, when enrichment runs, then its logo resolves from the page already being read (site mark / favicon / Impressum header) with no upload; the same mark renders consistently everywhere, scaled, never stretched; no resolvable logo → a polished deterministic monogram (stable per company), never a broken image or blank slot; a hand-uploaded logo is never silently replaced (🟡 confirm); the fetched logo carries provenance like every captured value. | Integration lane ([[testing#TEST-LANE-2]]) on resolve/normalize/provenance + the A26 guard; monogram determinism unit test ([[testing#TEST-LANE-1]]); render consistency via AC-companies-3, AC-company-1 ([[testing#TEST-LANE-3]]); design floor [[acceptance-standards#GATE-CORE-8]]. |

Source: features/01-core-objects.md#11-contact-record--360-view + #12-person--organization-relationships + #13-deduplication--data-quality + #21-organization-record--360-view + #22-organization-hierarchy @ 5a0b29c

**Feature-doc acceptance criteria** (verbatim; new pin IDs, corpus anchor per row). Cross-cutting
release gates are inherited ([[acceptance-standards#GATE-CORE-1]]–[[acceptance-standards#GATE-CORE-8]])
and not repeated per row.

| ID | Acceptance criterion (verbatim) | Anchor · Verification |
|---|---|---|
| PO-AC-1 | `POST /people` with name+email returns 201 and a stable `id` (UUID); round-trips identically on `GET`. | §1.1 · integration ([[testing#TEST-LANE-2]]) |
| PO-AC-2 | Multiple emails/phones persist as ordered rows with one `is_primary` per type (DB constraint enforces ≤1 primary). | §1.1 · integration; PO-DDL-2/3 partial unique indexes |
| PO-AC-3 | Every persisted row has non-null `source` and `captured_by`. | §1.1 · [[acceptance-standards#GATE-CORE-3]] |
| PO-AC-4 | Open person record **p95 < 100 ms server / < 300 ms perceived** (§3.5). | §1.1 · CI benchmark ([[acceptance-standards#PERF-1]]) |
| PO-AC-5 | Save/mutation **p95 < 150 ms server**. | §1.1 · CI benchmark ([[acceptance-standards#PERF-4]]) |
| PO-AC-6 | Archive sets `archived_at`, removes from default lists, retains in audit; row still fetchable by id. | §1.1 · integration |
| PO-AC-7 | Every create/update/archive writes one `audit_log` row and emits one `person.*` event. | §1.1 · [[acceptance-standards#GATE-CORE-5]] |
| PO-AC-8 | OpenAPI `crm.yaml` fully types the resource; generated TS compiles (P3). | §1.1 · [[acceptance-standards#GATE-CORE-1]] |
| PO-AC-9 | **User-observable:** on the 360 view every field visibly shows whether it was *captured* (with the source and date, e.g. "from email 2026-05-02") or *typed by* a named human — the user can tell at a glance and never has to guess where a value came from (S-E02.6). | §1.1 · live-stack ([[testing#TEST-LANE-3]]); AC-person-1/8 |
| PO-AC-10 | **User-observable:** an auto-captured person that has not yet been accepted is visually marked as proposed and persists to the user's working lists only after they accept it (one click); the user can see the difference between a confirmed contact and a pending one (S-E01.4). | §1.1 · live-stack: the pending/proposed state lives in the capture chapter's staging construct (no column in person per GATE-AI-2); verified by the capture chapter's flow driving this chapter's list rendering — cross-chapter e2e. PILOT-EXCLUDED (capture-dependent, PO-N-PILOT) |
| PO-AC-11 | person↔org modeled in the typed `relationship` table (`03` §3.2), **not** a single FK column, so multi-org and history work. | §1.2 · schema assertion, integration |
| PO-AC-12 | A person can have ≥1 historical and exactly ≤1 current-primary org (constraint). | §1.2 · PO-DDL-7 `uq_rel_current_primary_employer`, integration |
| PO-AC-13 | Querying "all people at org X" is an indexed join returning **p95 < 150 ms** for 50 rows. | §1.2 · CI benchmark ([[acceptance-standards#PERF-2]]) |
| PO-AC-14 | Reporting joins across person/org/deal require **no** post-query directional-association resolution (the explicit anti-HubSpot check). | §1.2 · [[acceptance-standards#GATE-CORE-4]]; asserted as schema shape via GATE-CORE-4 (no directional-association table exists) |
| PO-AC-15 | **User-observable:** when capture infers an employer from an email domain or signature, the proposed org link is shown with its evidence ("inferred from acme.com") and the user can accept or correct it; a person who has moved jobs shows both the former and current org with dates, not a single overwritten company. | §1.2 · live-stack ([[testing#TEST-LANE-3]]) |
| PO-AC-16 | Creating a person with an email already present on a non-archived person returns a 409 + the existing id (no silent duplicate). | §1.3 · integration; PO-DDL-2 `uq_person_email_dedupe`; API-ERR-7 |
| PO-AC-17 | Merge of A→B reassigns all activities, relationships, and deals from A to B with zero orphaned FKs (verified by a referential-integrity test); A is archived with a `merged_into` pointer. | §1.3 · referential-integrity test, integration ([[acceptance-standards#GATE-CORE-4]]) |
| PO-AC-18 | Merge is one `audit_log` transaction; reversible within audit (re-split is a documented recovery, not necessarily v1 UI). | §1.3 · integration + audit assertion |
| PO-AC-19 | Auto-merge fires **only** on exact unique-key match; all else routes to the 🟡 queue. | §1.3 · unit ([[testing#TEST-LANE-1]]) over PO-F-1/PO-F-2; `DEDUPE_FUZZY_AUTOMERGE` |
| PO-AC-20 | **User-observable:** the user is never silently handed a duplicate — when capture hits an existing email it lands on the existing record, and an ambiguous fuzzy match surfaces as a review item the user can confirm or reject side-by-side (the user sees both candidates, not a guessed merge) (S-E02.6). | §1.3 · live-stack; review UI owned by the data-hygiene chapter |
| PO-AC-21 | `organization` keyed by id; domains are normalized, lowercased, unique per org (constraint). | §2.1 · PO-DDL-5 `uq_org_domain`, integration |
| PO-AC-22 | 360 view assembles people + deals + activities in **one** server round-trip, **p95 < 150 ms** server for an org with ≤50 related rows (server-driven pagination beyond that). | §2.1 · CI benchmark ([[acceptance-standards#PERF-2]]) |
| PO-AC-23 | Auto-created orgs carry `captured_by = <capture-agent>` and are visually distinguishable from human-created. | §2.1 · integration + live-stack |
| PO-AC-24 | Enrichment writes are provenance-tagged and never overwrite a human-edited field without a 🟡 confirm. | §2.1 · integration (the A26 guard) |
| PO-AC-25 | **Logo (A55):** an org's logo is resolved on enrichment from the scraped page (no third-party logo API — zero new egress, P7/A8), stored as normalized scaled variants, and stamped with provenance (`captured_by=agent:logo-resolve` + source) like any enriched field; resolution is **🟢** (a display asset) and a later auto-resolved logo **never overwrites a human-uploaded logo without a 🟡 confirm** (same A26 guard as every other field). When no logo resolves, the record exposes a stable deterministic monogram (initial(s) + a colour hashed from the org name) — the render layer never shows a broken image or an empty slot. | §2.1 · integration + unit (monogram determinism) |
| PO-AC-26 | **User-observable:** the user can see, on the org 360, that the org was created for them automatically from a new email domain (the contacts who triggered it are already linked) — they did not have to create the company before the people showed up (S-E02.2); and the company shows its **logo** (or a clean monogram) without the user uploading anything (S-E02.8). | §2.1 · live-stack ([[testing#TEST-LANE-3]]); AC-company-1/11 |
| PO-AC-27 | Parent link is a real FK to `organization` with a cycle-prevention constraint/check (no org is its own ancestor). | §2.2 · PO-DDL-4 CHECK + ancestor-walk trigger, integration |
| PO-AC-28 | "Group view" roll-up query is an indexed recursive CTE returning **p95 < 200 ms** for a tree of ≤200 orgs. | §2.2 · CI benchmark; roll-up reporting acceptance owned by the records-depth chapter (S-E15.8) |

**New pins** (this review, 2026-07-03) — merge conflict semantics and the classification confirm
surface.

| ID | Given/When/Then | Verification |
|---|---|---|
| PO-AC-M1 | Given survivor and loser conflict on primary email, primary phone, or current-primary employer, When A merges into B, Then the survivor's values win and the loser's conflicting primary/current rows are demoted to non-primary on relink (the unique constraints hold). | integration ([[testing#TEST-LANE-2]]) |
| PO-AC-M2 | Given duplicate (deal, person, role) stakeholder rows across survivor and loser, When merged, Then they collapse to the survivor's single row. | integration |
| PO-AC-M3 | Given source = target (self-merge), When requested, Then 422 `validation_error`. | integration |
| PO-AC-M4 | Given the target is archived or already merged away, When a merge is requested, Then 422 `validation_error` with the pointer to the surviving record. | integration |
| PO-AC-M5 | Given two concurrent merges of the same loser, When both commit, Then the second write loses with 409 `version_skew`. | integration |
| PO-AC-M6 | Given records in different workspaces, Then a cross-workspace merge is impossible by construction (RLS). | covered by the tenant-isolation floor |
| PO-AC-29 | Given a pending classification proposal, When the org 360 renders, Then a classification proposal card appears with its evidence and confirm/reject actions, never silently overwriting a human-set value. | e2e lane; PILOT-EXCLUDED (S-E02.7, PO-N-PILOT) |

Source: product/30-screen-acceptance.md#22-people--orgs @ 5a0b29c

**Owned screens** (corpus IDs preserved verbatim; no other chapter pins these). Verification for
every row: the live-stack UI lane ([[testing#TEST-LANE-3]]); the standard screen-state floor
([[acceptance-standards#STATE-1]]–[[acceptance-standards#STATE-5]]) and the design-system gate
([[acceptance-standards#GATE-CORE-8]]) apply to all four screens and are not repeated per row.

*Contacts list (implements S-E02.2/.5/.6, S-E08.1-adjacent):*

| ID | Given/When/Then (verbatim) |
|---|---|
| AC-contacts-1 | Given a connected mailbox, When the list loads, Then a capture banner reads "N contacts and M companies captured from your inbox this week", names the connectors (Gmail + Calendar), states "You typed 0", and a "Review capture" link → home.html. |
| AC-contacts-2 | Given the list renders, When I read the section label, Then it shows "Contacts we actually know" with the count "N captured · M hand-typed", and a column header row exists (Contact / Relationship / Last / Source). |
| AC-contacts-3 | Given each contact row, When it renders, Then it shows avatar, name, "{title} @ {company}", a relationship-strength block, a last-touch value, and a source chip; clicking the row → person.html. |
| AC-contacts-4 | Given a relationship-strength block, When it renders, Then it shows the integer score, a bar whose width = score% and color is accent for ≥60, "med" for 40–59, "low" for <40, and the caption "recency·frequency·reciprocity". |
| AC-contacts-5 | Given I hover a relationship-strength block, When the popover appears, Then it shows "Relationship-strength {score}", three labeled component bars (Recency / Frequency / Reciprocity) with numeric values, the per-contact note, and "Computed from the captured timeline — never a black-box badge." |
| AC-contacts-6 | Given a contact's source, When I read the Source column, Then a connector-sourced contact shows a "connector" chip and a hand-typed contact shows a "typed by you" chip — provenance is never blank. |
| AC-contacts-7 | Given the toolbar, When I click "Strength", Then the list sorts by relationship-strength; When I click "Filter", Then a filter affordance is invoked. |
| AC-contacts-8 | Given the header, When I click "New contact", Then a blank record opens and the UI signals this is the rare path. |
| AC-contacts-9 | Given the search input, When I type a query, Then the list filters to matching contacts. |

**PO-N-BUCKETS** (reconcile note) — PO-PARAM-3 is the oracle: weak 0–24 / moderate 25–59 / strong
60–100. AC-contacts-4's bar-color thresholds (40/60) are prototype drift — the build renders accent
≥60 / med 25–59 / low <25. The AC row text stays verbatim above; this note names the winner.

*Contact 360 (implements S-E02.2/.5/.6, S-E08.1):*

| ID | Given/When/Then (verbatim) |
|---|---|
| AC-person-1 | Given the record header, When it loads, Then it shows the person's name, "{title} · {company}" (company → company.html), and contact methods rendered with provenance-styled mono text. |
| AC-person-2 | Given the relationship-strength card, When it renders, Then it shows "{score}/100", "Relationship strength", a "computed · deterministic from captured cadence" chip, a fill bar at score%, and a "Team-wide" caption (counts every rep's captured activity, not just mine). |
| AC-person-3 | Given the strength inputs, When they render, Then exactly three factor tiles appear — Recency (with half-life note), Frequency (with saturation note), Reciprocity (in/out) — each with a mini-bar. |
| AC-person-4 | Given I click "Show the activities behind this score", When it expands, Then an evidence box shows the interaction breakdown and the literal arithmetic "Score = 100 × recency × frequency × reciprocity = N", plus a source line to the Activity tab and "formula §4"; collapse toggles the label. |
| AC-person-5 | Given the warm-room signal card (E08), When it renders, Then it shows a "Warm signal" flag, a headline, a "high confidence" indicator, an AI suggestion, and a "Show the evidence" toggle revealing the captured quote with source + why-warm rationale. |
| AC-person-6 | Given the warm-signal action footer, When I click "Draft a reply" / "Send booking link" / "Create follow-up", Then each is confirm-first / accept-to-persist (nothing sent or persisted without approval); "Dismiss" dismisses the signal and feeds agent learning. |
| AC-person-7 | Given the Record tabs (Activity / Deals / Notes), When I select a tab, Then only that pane shows; Activity is the default. |
| AC-person-8 | Given the Activity tab, When it renders, Then every timeline row carries a kind (Email/Meeting/Call), a body, and a provenance chip ("connector · Gmail thread" etc.), with a caption "You logged none of this — every row carries its source." |
| AC-person-9 | Given the Notes tab with no notes, When it renders, Then it shows an empty state plus a textarea labelled "will be typed-by-you"; saving a non-empty note marks it typed-by-you (🟢); saving empty is rejected. |
| AC-person-10 | Given the Consent (per purpose) card, When it renders, Then it lists each purpose with grant/withdrawal timestamp + source + policy version, states outbound is default-deny per purpose (unknown = no-consent; a grant for one purpose never authorises another), and "View consent history" opens the append-only proof log. |
| AC-person-11 | Given the "Enriched from signature" card, When it renders, Then it shows the captured signature quote, a source line, an "agent:enrich" chip, and a confidence indicator. |
| AC-person-12 | Given the top-bar Edit action, When invoked, Then editing marks touched fields typed-by-you (🟢); Email drafts but does not send (🟡). |

*Companies list (implements S-E01.2, S-E02.2):*

| ID | Given/When/Then (verbatim) |
|---|---|
| AC-companies-1 | Given the list loads, When it renders, Then the same capture banner appears ("You typed 0", Review capture → home.html). |
| AC-companies-2 | Given the section label, When it renders, Then it reads "Companies" with count "N captured · org strength = strongest contact" and a column header row (Company / Contacts / Open deals / Org strength). |
| AC-companies-3 | Given each company row, When it renders, Then it shows a colored logo, name, industry sub-line, contact count, open-deal count, and an org-strength block; clicking the row → company.html. |
| AC-companies-4 | Given an org-strength block, When it renders, Then it shows the score, a bar at score% with the same color thresholds, and the caption "max over {N} contacts". |
| AC-companies-5 | Given I hover an org-strength block, When the popover appears, Then it states the score equals the strongest single relationship's score, names that top contact, and asserts "the org score is the max over its contacts — not an average that hides one warm champion." |
| AC-companies-6 | Given the header, When I click "Read a company", Then it navigates to read-company.html; "New" opens a blank company record flagged as the rare path. |
| AC-companies-7 | Given the toolbar, When I click "Strength", Then the list sorts by org strength; "Filter" invokes filtering. |
| AC-companies-8 | Given the search input, When I type, Then the list filters to matching companies. |

**PO-N-FILTER** (verification note) — AC-contacts-7 / AC-companies-7 verify as: the control renders
and opens the filter panel; the strength sort applies server-side (PO-EXT-1 pins the sort key).

*Org 360 (implements S-E01.2, S-E02.2):*

| ID | Given/When/Then (verbatim) |
|---|---|
| AC-company-1 | Given the header card, When it loads, Then it shows logo, name, and a meta row (industry, website link new-tab, banded staff, location). |
| AC-company-2 | Given the org-strength card, When it renders, Then it shows "{score}/100", "Account relationship strength", a "computed · MAX over contacts" chip, a fill bar, and a source line naming the strongest contact (→ person.html) and stating strength is the max over known people, "not an average, not a black box." |
| AC-company-3 | Given I click "Show the per-contact scores behind this", When it expands, Then it lists each contact's score, shows the literal "MAX(...)=N capped/normalized to {score} after recency decay", and cites "formula §4 · DECISIONS A4"; collapse toggles. |
| AC-company-4 | Given the Firmographics & legal card, When it renders, Then a sub-line states fields were read off `<domain>/impressum` and "every field carries its source snippet, or is omitted. Nothing here was typed." |
| AC-company-5 | Given each firmographic row (Legal name, Registered address, Register no., VAT ID, Founded), When it renders, Then it shows the value, a "read-from-impressum" chip, a confidence indicator (High/Medium), and an "evidence" toggle revealing the exact source snippet + "view source" link. |
| AC-company-6 | Given fields not present on the Impressum, When firmographics render, Then an omission notice explicitly names them (exact employee count, parent company) as "omitted (no guess)". |
| AC-company-7 | Given the "People at the org" rail, When it renders, Then each person shows avatar, name, title, a role flag (Champion vs Stakeholder), and a "{score}/100" strength, top contact highlighted; each row → person.html. |
| AC-company-8 | Given the Deals rail, When it renders, Then it lists open and won deals with name, amount, and stage/flag pills, each → deal.html. |
| AC-company-9 | Given the Account activity card, When it renders, Then every timeline row is provenance-tagged and a footer states "You logged none of this — capture linked every item." |
| AC-company-10 | Given the Account signal card, When it renders, Then it warns the account is single-threaded on the top contact, cites the stalled deal, and shows an evidence quote with an "open the deal" link. |
| AC-company-11 | Given the Quick facts rail, When it renders, Then it shows Owner/Open deals/Won lifetime/People known/First seen/Last touch and a "Created — from email domain" chip. |
| AC-company-12 | Given the top-bar actions, When invoked, Then "Summarize this account" is read-only (🟢), "Edit" marks fields typed-by-you (🟢), and "New deal" stages a deal with org/contacts pre-linked (🟡). |

**PO-N-ORGSTRENGTH** (reconcile note) — PO-F-3's plain max is the only formula; there is no
cap/normalize/post-decay step. AC-company-3's literal copy is corpus-flagged drift — the screen
renders the max value, and N always equals the displayed score. The AC row text stays verbatim
above; this note names the winner.

**PO-N-CHAMPION** (decision) — the people-rail champion flag (AC-company-7) derives from a
`deal_stakeholder` row with role `champion` on any OPEN deal of that organization; with multiple
deals, any one suffices. An engineering default resolving the corpus open question — flagged as a
decision.
