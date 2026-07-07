---
status: planned
module: modules/signals (backend) · web (warm-signal card, coaching screen)
derives-from:
  - specs/spec/features/07-ai-native-moments.md#9-warm-room-signal
  - specs/spec/features/07-ai-native-moments.md#8b-deal-coaching-on-demand
  - specs/spec/contract/formulas-and-rules.md#9-warm-room-join--signal-company-resolves-to-an-org-with-a-non-archived-contact
  - specs/spec/contract/formulas-and-rules.md#106-signal-score--transparent-weighted-signal-model-e08-warm-room-features03-32-explain-this-score
  - specs/spec/contract/data-model.md#signals-e08-warm-room--features08
  - specs/spec/product/epics/E08-warm-room-and-signals.md
  - specs/spec/contract/events.md#511-engagement--signals-e08-warm-room-e15-reply-tracking
  - specs/spec/product/30-screen-acceptance.md#coachinghtml--deal-coaching-implements-s-e085
  - specs/spec/product/30-screen-acceptance.md#personhtml--contact-360-implements-s-e02256-s-e081
  - margince-poc/docs/subsystems/signals.md @ a11d6c08
---
# Signals & warm room — a signal is warm because we already know someone there, and cold spray dies

> The durable store of "something changed, worth attention" facts, plus the two
> moments built on it: the warm-room join (an inbound signal that resolves to a
> company where we have a live contact is presented warm, with an evidenced intro
> path) and on-demand deal coaching (a rep opens a stalled deal and gets an
> evidence-backed unstick move, drafted but never sent). Its promise: no signal
> is ever a guess, a dossier, or an auto-send.

## What it's for

Reps miss the things buried in noise — a deal gone quiet, a champion who left,
an account showing fresh intent — and when an external signal does surface,
every incumbent treats it as a cold lead even when the rep already knows someone
at that company. This subsystem records signals as discrete, evidenced,
company-attributed facts and makes the join nobody else can make as well: it
lands each signal against our own contact graph, so a signal at a company with a
live contact is warm, with the route in named, and everything else is honestly
cold or dropped. It also owns the rep-initiated coaching moment on a stalled
deal — same evidence discipline, on demand instead of overnight. Its callers are
the contact 360 (the warm-signal card), the morning-brief and work-queue read
models (ranked by the signal score), the coaching screen on a stalled deal, and
the deal-rooms chapter, whose consent-gated engagement is one of the channels a
signal arrives on. The boundary: this chapter owns the signal store, the
warm/cold/drop branch, the signal score, and the coaching surface — not the
upstream sensing or resolution pipelines, which the scope of record defers.

## Principles it serves

- **P11 — the join is the relational-core moat.** Warmth is a deterministic
  query against real people, organizations, and typed employment relationships —
  powerful precisely because it needs no speculative data.
- **P12 — consent-gated, company-level, evidenced.** Every signal names its
  evidence or is dropped; every "why warm" and every score decomposes to
  inspectable factors; every proposed outbound is confirm-first
  ([[acceptance-standards#GATE-AI-7]]).
- **P5 — surfaced, not searched-for.** The rep is told who is warm and handed
  the unstick move, instead of mining a feed or staring at a dead thread.
- **ADR-0011 / [[personas#PERSONA-PAT-GUARD-1]] — Pat is never covertly
  profiled.** Buyer-side behavior enters only as consent-gated, company-level
  facts; covert profiling is a structural rejection ([[scope#NEVER-8]]).
- **ADR-0007 — context graph as V1 substrate.** Coaching reasons over the
  captured deal context and cross-links the graph already in V1.
- **ADR-0006 — the scrape/enrichment seam.** Coaching's public-activity read
  rides the governed connector seam and degrades honestly when it is absent.

## How it works

**A signal is a recorded fact, not an inference left loose.** Each signal says
what kind of thing happened — a stalled deal, a champion leaving, a
re-engagement, buying intent, a risk — and which channel it arrived on: derived
internally by sibling subsystems, an inbound message, the web, social, or
deal-room engagement. It keeps a pointer to its raw source, carries a per-claim
evidence list, a severity, and a lifecycle from open through acknowledged,
resolved, or dismissed. In V1 most signals are born already attributed: the
derived and inbound channels know their entity when they write the row. The
broad external resolver (S-E08.3) is a deferred substrate — the columns it will
stamp exist now, so V1 operates on whatever resolved, company-level signals
exist without a schema change later.

**Attribution is split into machine reasoning and human outcome, append-only.**
The match basis — what the signal was matched on (a domain, a name, a prior
interaction), which organization, at what confidence — is written as an
inspectable log entry, never overwritten; a confident match marks the signal
resolved, an ambiguous one low-confidence (surfaced but flagged), no candidate
unresolved, and noise dropped. The human outcome — acknowledged, resolved, or
dismissed, with a note and by whom — is recorded in the same append-only log but
never collapsed into the machine's reasoning.

**The warm/cold/drop branch is deterministic (SIG-F-1).** A signal that cannot
be attributed to a company is dropped: no row of person data is created or kept
from it — a guessed signal is never kept. A signal resolved to an organization
with at least one live, non-archived contact via a typed employment relationship
is warm; otherwise it is cold and queued differently — a real branch, not a
cosmetic badge. For a warm signal the proposed route in is the strongest
existing contact by relationship strength ([[people-and-organizations#PO-F-3]]),
with a fixed deterministic tiebreak; the warm-intro proposal names the contact,
the relationship, and a suggested next move, and asking "why warm" returns the
source signal, the resolved organization, and the specific contact identifiers —
no mystery score. Leads never make a signal warm (a warm room needs a real
contact, per ADR-0008), archived contacts don't count, and V1 matches the exact
resolved organization only, with no hierarchy roll-up into warmth.

**Ranking is a transparent weighted composite (SIG-F-2).** Surfaced signals are
ordered by a deterministic 0-to-1 score over four factors — reach, engagement,
ICP fit, and recency with a seven-day half-life (SIG-PARAM-1..3) — and the
per-factor breakdown is exposed for "Explain this score". An unresolved or
low-confidence signal is never ICP-scored on a guess: its fit factor is zero,
so uncertainty visibly down-weights instead of silently asserting.

**Coaching is the same evidence discipline, on demand.** On a stalled deal the
rep asks for coaching and gets three things: a specific re-engagement angle
drawn from the deal's real history and the contact's public activity via the
ADR-0006 seam (including the likely competing priority that stalled it), a
follow-up drafted in the rep's voice through the drafting seam, and a channel
suggestion with its stated reason — reasoned from the deal's reply cadence,
never from a behavioral profile of the person. Every recommendation carries the
evidence it derived from or is honestly omitted
([[acceptance-standards#GATE-AI-1]]); a fact that cannot be grounded is shown as
an explicit omission. The draft is unsent: it queues through the approval inbox
([[approvals-and-concurrency]]), and every generative output renders the
AI-assisted disclosure ([[acceptance-standards#GATE-AI-9]]). When the
public-activity source is unreachable, coaching degrades to deal-context-only
rather than guessing ([[acceptance-standards#GATE-AI-8]]).

## What's configurable

- **Signal-score weights and shape** — the four factor weights (SIG-PARAM-1),
  the reach saturation point (SIG-PARAM-2), and the recency half-life
  (SIG-PARAM-3) are named source constants with defaults; no runtime tuning UI
  in V1 (P1).
- **Nothing structural in the warm branch** — the route tiebreak order is fixed
  (SIG-PARAM-4); warm/cold/drop has no knobs by design.
- **The resolver** — a deferred injected substrate (S-E08.3, scope OUT): with it
  absent, V1 consumes signals that arrive already attributed (derived, inbound,
  deal-room engagement) and never fabricates a resolution.
- **The public-activity source** — the ADR-0006 scrape seam varies by deployment
  and consent posture; absent or unreachable, coaching completes on deal context
  alone and says so.

## Guarantees (enforced)

- **Drop the orphan, never guess.** A signal not attributable to a company is
  dropped — zero person rows, zero person links, zero retained person-level
  dossier ([[acceptance-standards#GATE-AI-1]], [[scope#NEVER-8]]).
- **Company-level and consent-gated, always.** Signals attribute to
  organizations; a person link is optional and consent-bound; no named-individual
  behavioral profile is ever created from a signal
  ([[personas#PERSONA-PAT-GUARD-1]]).
- **The warm/cold distinction is a real branch.** With a live contact the signal
  is warm with a proposed route; without one it is cold and routed differently —
  tested as behavior, not styling (SIG-AC-3).
- **Warm means evidenced.** "Why warm" always returns the source signal, the
  resolved organization, and the contact identifiers behind it — decomposable,
  reproducible, no mystery score (SIG-F-1, SIG-F-2).
- **Resolved means inspectable.** A confidently resolved signal carries at least
  one append-only match-basis entry naming what it matched on; machine reasoning
  and human outcome are never merged.
- **Proposed outbound is confirm-first.** The warm-intro draft and the coaching
  follow-up are staged; nothing sends without a recorded human approval token
  ([[acceptance-standards#GATE-AI-7]]).
- **Deterministic and reproducible.** The branch and the score are pure
  functions of pinned inputs — a fixed fixture and clock yield a stable class,
  route, and score.
- **Workspace isolation.** Signals and their resolution log are workspace-scoped
  under row-level security like every table.

## Acceptance

Done means: a signal landing at a company where we have a live contact shows up
warm with an explicit marker, a named route in, and one-click evidence; the same
signal at an unknown company queues cold; an unattributable one vanishes without
residue. The rep on a stalled deal can ask for coaching and get an evidenced
angle, a voice draft that only ever exits through the approval inbox, and a
channel suggestion with its reason — or an honest "not enough to coach". The
surfaces render the standard honest states ([[acceptance-standards]] STATE-1..5),
including the nothing-grounded omission on every AI claim, the coaching error
path that retries or falls back to deal-context-only, and honest dismissal
(nothing saved, nothing sent). Testable forms live in the Acceptance appendix;
the AI-behavior rows ride the eval catalog (AIUC-18, AIUC-19). One open consent
question is carried here from the people chapter: whether profiling-consent
withdrawal suppresses the strength compute behind the warm route or only
outbound (SIG-N-1).

## Out of scope

- **Signal sensing and scoring of raw social/web activity** (S-E08.2) — Backlog,
  on the scope OUT list; consent and quality bar not yet met ([[scope]]).
- **Signal-to-company/person resolution at large** (S-E08.3) — Fast-follow, on
  the scope OUT list; this chapter consumes a resolved organization and pins the
  columns it will stamp.
- **The deal room and its engagement capture** — [[deal-rooms]]; its events
  arrive here as one signal channel.
- **The rules that derive signals** — the stalled-deal rule, deal-health
  at-risk flag, and their thresholds belong to the chapters that own those
  formulas; this chapter owns the signal row they land in.
- **Relationship strength** — the formula is
  [[people-and-organizations#PO-F-3]]; this chapter only reads it for the route
  tiebreak and the engagement factor.
- **Drafting mechanics** — the coaching follow-up rides the drafting seam
  ([[drafting]]); voice and approved-asset grounding are pinned there.
- **The coverage view** (S-E09.6) — the account-mapping screen is the reporting
  chapter's, fed by this chapter's signals.

## Where it lives

The signals module in the backend (modules/signals), with the warm-signal card
on the contact 360 and the coaching screen in the web app. Read next:
[[deal-rooms]] (the engagement channel), people-and-organizations (the graph the
join lands against and the strength formula), [[drafting]] (the draft path
coaching hands to), and event-bus (the signal event family).

## Appendix

### Parameters
Source: contract/formulas-and-rules.md#0-parameter-registry-all-tunables-one-place @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| SIG-PARAM-1 | `W_SIG_REACH / ENG / ICP / REC` | `0.20 / 0.30 / 0.30 / 0.20` | signal-score factor weights (sum=1.0) |
| SIG-PARAM-2 | `SIGSCORE_REACH_NORM` | `5` | distinct touched entities at which reach saturates |
| SIG-PARAM-3 | `SIGSCORE_HALFLIFE_DAYS` | `7` | signal recency decay half-life |
| SIG-PARAM-4 | Warm-route tiebreak | fixed (not tunable) | highest relationship strength → most recent last interaction → lowest person id; corpus §9: "Tunables: none structural; route tiebreak order is fixed." |

### Formulas

#### SIG-F-1 — Warm-room join (warm/cold/drop, deterministic)
Source: contract/formulas-and-rules.md#9-warm-room-join--signal-company-resolves-to-an-org-with-a-non-archived-contact @ 5a0b29c

Inputs: the resolved `organization.id` for a signal (resolution itself is
S-E08.3, Fast-follow; this rule consumes a resolved org), and
`relationship(kind='employment')` linking live persons to that org. Rule
(verbatim):

```
classify_signal(signal):
    org_id = resolve_company(signal)          # upstream; may be null
    if org_id is null:    return { class: DROP }     # not attributable to a company → dropped, no person row

    contacts = SELECT DISTINCT r.person_id
               FROM relationship r JOIN person p ON p.id = r.person_id
               WHERE r.workspace_id=:ws AND r.organization_id=org_id
                 AND r.kind='employment' AND r.archived_at IS NULL
                 AND p.archived_at IS NULL
    if count(contacts) >= 1:
        return { class: WARM, org_id, route_via: pick_route(contacts) }
    else:
        return { class: COLD, org_id }
```

`pick_route` (the proposed intro path) picks the **strongest** existing contact
by relationship-strength ([[people-and-organizations#PO-F-3]]) as the warm route
— deterministic tiebreak by highest strength, then most recent
`last_interaction`, then lowest `person.id` (SIG-PARAM-4).

Output: `{ class: WARM|COLD|DROP, org_id, route_via_person_id?, evidence:
{signal_id, org_id, contact_ids[]} }`. WARM → propose warm-intro path naming the
contact + relationship; COLD → cold queue; DROP → no row created (orphan signal
→ 0 person rows).

Worked example (verbatim):

- Signal resolves to Acme (org_id X). Two live contacts at Acme (strengths 47,
  12). → **WARM**, `route_via = person(strength 47)`, evidence lists both
  contact ids + org id.
- Signal resolves to Globex, no live contacts → **COLD**.
- Signal cannot resolve to any org → **DROP** (no person dossier created).

Edge cases (verbatim): only archived contacts at the org → COLD (the join
requires `p.archived_at IS NULL`); leads at the org are ignored — leads are not
in `relationship`/`person` (ADR-0008); multiple orgs (parent/subsidiary) → v1
matches the exact resolved org only (no hierarchy roll-up into warmth;
hierarchy roll-up is [TS]).

#### SIG-F-2 — Signal score (transparent weighted composite, 0..1)
Source: contract/formulas-and-rules.md#106-signal-score--transparent-weighted-signal-model-e08-warm-room-features03-32-explain-this-score @ 5a0b29c

Formula (verbatim):

```
# all factors normalized to 0..1
reach      = min(1.0, distinct_touched_entities / REACH_NORM)         # REACH_NORM = SIGSCORE_REACH_NORM
engagement = strongest_related_relationship_strength / 100            # §4 relationship-strength of the resolved org's best stakeholder
icp_fit    = icp_fit_score(resolved_org)                              # 1.0 full ICP match · 0.5 partial · 0.0 unknown/none (no-guess: unresolved org → 0)
recency    = exp(-ln(2) * days_since(detected_at) / SIGSCORE_HALFLIFE_DAYS)   # exponential decay

signal_score = W_SIG_REACH*reach + W_SIG_ENG*engagement + W_SIG_ICP*icp_fit + W_SIG_REC*recency
   # default weights: W_SIG_REACH=0.20, W_SIG_ENG=0.30, W_SIG_ICP=0.30, W_SIG_REC=0.20  (sum=1.0)
```

`icp_fit_score` reads the resolved org's firmographics against the workspace ICP
profile: full match on industry+size band → 1.0, partial → 0.5, no resolved org
(`signal.resolution_state` ≠ `resolved`) → 0.0 — an unresolved signal is never
ICP-scored on a guess (the no-guess gate). Output: `{signal_id, signal_score,
factors:{reach,engagement,icp_fit,recency}, evidence_ids[]}` — the per-factor
breakdown is exposed for Explain-This-Score; a low-confidence resolution
(`resolution_state='low_confidence'`) is shown but down-weighted via
`icp_fit=0`.

Worked example (SIG-F-2-EX, derived here — the corpus §10.6 pins no worked
example; arithmetic from the pinned defaults): a signal touching 3 distinct
entities (reach = 3/5 = 0.60) at a fully ICP-matched org (icp_fit = 1.0) whose
strongest stakeholder has strength 47 (engagement = 0.47), detected exactly 7
days ago (recency = 0.50) scores
`0.20·0.60 + 0.30·0.47 + 0.30·1.0 + 0.20·0.50 = 0.661`.

### Schema
Source: contract/data-model.md#signals-e08-warm-room--features08 @ 5a0b29c

Two tables, owned here per the schema ownership index
([[data-model#Schema — ownership index]]). Each carries the universal base
columns (`id`, `workspace_id`, `created_at`, `updated_at`, `archived_at`);
`signal` also carries `version`.

SIG-DDL-1 — `signal`:

```sql
CREATE TABLE signal (                                   -- a surfaced "something changed / worth attention" item
  -- + base columns + version
  kind          text NOT NULL CHECK (kind IN ('stalled_deal','champion_left','reengagement','buying_intent','risk','other')),
  source_channel text NOT NULL DEFAULT 'derived' CHECK (source_channel IN ('derived','inbound','web','social','deal_room_engagement')),  -- where the raw signal came from (E08.1 ingest)
  raw_ref       text NULL,                               -- pointer to the raw source payload (inbound email id / web event / engagement event id)
  entity_type   text NOT NULL CHECK (entity_type IN ('deal','organization','person')),
  entity_id     uuid NOT NULL,
  resolution_state text NOT NULL DEFAULT 'resolved' CHECK (resolution_state IN ('resolved','low_confidence','unresolved','dropped')),  -- raw→entity match outcome (E08.2 resolver)
  resolution_confidence numeric NULL CHECK (resolution_confidence IS NULL OR (resolution_confidence >= 0 AND resolution_confidence <= 1)),
  resolved_org_id    uuid NULL REFERENCES organization(id),  -- the org the raw signal resolved to (null until/unless resolved)
  resolved_person_id uuid NULL REFERENCES person(id),        -- optional person resolution
  severity      text NOT NULL DEFAULT 'info' CHECK (severity IN ('info','warn','urgent')),
  summary       text NOT NULL,
  evidence      jsonb NOT NULL DEFAULT '[]',             -- per-claim {snippet, source_type, source_id}
  status        text NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved','dismissed')),
  detected_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_signal_open ON signal (workspace_id, status, severity, detected_at DESC);
CREATE INDEX idx_signal_unresolved ON signal (workspace_id, resolution_state, detected_at DESC);
```

SIG-DDL-2 — `signal_resolution` (append-only):

```sql
CREATE TABLE signal_resolution (                         -- append-only log: BOTH the raw→entity match basis (E08.2) and the human outcome (acknowledge/resolve/dismiss)
  -- + base columns
  signal_id     uuid NOT NULL REFERENCES signal(id),
  -- match basis (inspectable: how the resolver matched the raw signal to an entity)
  matched_on    text NULL CHECK (matched_on IS NULL OR matched_on IN ('domain','name','prior_interaction','manual','none')),
  matched_org_id uuid NULL REFERENCES organization(id),
  match_confidence numeric NULL CHECK (match_confidence IS NULL OR (match_confidence >= 0 AND match_confidence <= 1)),
  match_detail  jsonb NULL,                              -- {candidates:[…], chosen, reason}
  -- human outcome (null for a pure match-basis row)
  outcome       text NULL CHECK (outcome IS NULL OR outcome IN ('acknowledged','resolved','dismissed')),
  note          text NULL,
  resolved_by   uuid NULL REFERENCES app_user(id)
);
```

Corpus semantics note (verbatim): a raw signal arrives on a `source_channel`
with a `raw_ref`; the resolver (E08.2) writes a `signal_resolution` row
recording the **match basis** (`matched_on` + `matched_org_id` +
`match_confidence` + `match_detail`) and stamps the parent `signal` with
`resolution_state` + `resolution_confidence` +
`resolved_org_id`/`resolved_person_id`. A confident match →
`resolution_state='resolved'`; an ambiguous one → `'low_confidence'` (still
surfaced, flagged for review); no candidate → `'unresolved'`; noise →
`'dropped'`. The same `signal_resolution` table later carries the **human
outcome** (`outcome`/`note`/`resolved_by`) — match-basis rows and outcome rows
are distinguished by which columns are non-null.

### Wire
Source: contract/crm.yaml (NET-NEW V1 RESOURCES planned block, comments only) @ 5a0b29c

**Honest contract-coverage finding:** at pin time `crm.yaml` defines 81
operations and **none** is a signal or coaching operation — the signals surface
exists in the contract only as the net-new-resources comment block ("/signals —
E08; MCP search_records/read_record; resolve via POST /signals/{id}/resolve").
The chapter pins the promised surface by path + behavior; operationIds must be
minted by a contract extension before any docs-cited operationId can resolve.
Until then, no prose or ticket may cite a signal operationId as if it existed.

| ID | Element (planned path) | Behavior pinned |
|---|---|---|
| SIG-WIRE-1 | `/signals` | Standard §12.5 resource shape — list (cursor+sort), get, create, update (If-Match), archive; schemas 1:1 from SIG-DDL-1. MCP `search_records`/`read_record`, tier 🟢 (read/stage only). |
| SIG-WIRE-2 | `POST /signals/{id}/resolve` | Records a match-basis and/or human-outcome entry (SIG-DDL-2) and stamps the parent signal's resolution columns; append-only, never overwrites a prior entry. |
| SIG-WIRE-3 | `signal_resolution` | No standalone CRUD — written/read through the parent signal's endpoints, per the corpus contract-surface note (high-volume/append-only tables ride their parent). |
| SIG-WIRE-4 | Coaching (S-E08.5) | Honest gap: **no** coaching operation exists anywhere in the contract, not even as a comment. The generative pass must be minted by a contract extension; its drafted follow-up exits only via the approval inbox ([[approvals-and-concurrency]]), never a direct send path. |

### Events
Source: contract/events.md#511-engagement--signals-e08-warm-room-e15-reply-tracking @ 5a0b29c

Defined and owned by the central event catalog ([[event-bus]]); cited here,
never redefined.

| ID | Event | This chapter's role |
|---|---|---|
| SIG-EVT-1 | `signal.detected` | Emitted (modules/ai / modules/capture) when a signal row lands — carries kind, source channel, entity, resolution state + confidence, severity. Consumed by the context graph, the warm-room read model, notifications, and the audit stream. |
| SIG-EVT-2 | `signal.resolved` | Emitted (modules/ai) when resolution lands — carries resolution state, resolved org/person, match basis + confidence. |
| SIG-EVT-3 | `engagement.reply` | Consumed: the warm-room read model ingests reply facts (channel email or deal_room). Reply-based, never an open-pixel — semantics pinned at [[event-bus#EVT-SEM-14]]; the event stream's deal-room channel originates in [[deal-rooms]]. |

Note SIG-EVT-N-1: the stream home for the signal/engagement event family is not
yet pinned — the event-bus chapter carries the reconcile note (a SPEC-DISPUTE
candidate); this chapter takes no dependency on a specific stream name.

### Acceptance

#### Acceptance — stories (condensed G/W/T)
Source: product/epics/E08-warm-room-and-signals.md#s-e081--warm-room-signal-a-signal-lands-where-we-already-know-someone @ 5a0b29c; product/epics/E08-warm-room-and-signals.md#s-e085--on-demand-coaching-when-i-open-a-stalled-deal @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| SIG-AC-1 | Given an inbound signal that resolves to a company, when we already have a contact there, then it is presented warm with an explicit marker (distinct from cold at a glance), a proposed warm-intro path (route contact + relationship + suggested next move — not just a notification), one-click "why warm" evidence (source signal, resolved company, contact ids), and any proposed outbound still 🟡 confirm-first; the same signal at a no-contact company is cold and routed differently. (S-E08.1, V1-WOW) | Deterministic branch test on SIG-F-1 fixtures; eval row AIUC-19 ([[ai-evals]]); [[acceptance-standards#GATE-AI-7]] |
| SIG-AC-2 | Given a stalled deal, when the rep asks for coaching, then they get a specific re-engagement angle (deal context + public activity + likely competing priority), a follow-up drafted in their voice, and a channel suggestion with the reason — each evidence-traceable, the draft 🟡 and never auto-sent. (S-E08.5, V1-WOW) | Deterministic AI-lane test; eval row AIUC-18 ([[ai-evals]]); [[acceptance-standards#GATE-AI-1]]/[[acceptance-standards#GATE-AI-7]] |

#### Acceptance — warm-room capability ACs (verbatim)
Source: features/07-ai-native-moments.md#9-warm-room-signal @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| SIG-AC-3 | Given a signal resolved to a company where we have ≥1 contact, it is presented **warm** with an explicit marker, distinct in the UI from cold; given the same signal resolved to a company with **no** contact, it is **cold** and routed differently — the distinction is a real branch, not cosmetic. *(test: with-contact fixture → warm; without-contact fixture → cold)* | Integration test, signals lane |
| SIG-AC-4 | Given a warm signal, a warm-intro path is proposed naming the existing contact, the relationship, and a suggested next move (not merely a notification). *(test)* | Integration test, signals lane |
| SIG-AC-5 | Given "why is this warm?", the response returns the **evidence**: source signal id, resolved org id, and the specific contact id(s) in our graph that make it warm — traceable, **no mystery score**. *(deterministic test: payload includes the contact id(s) and resolved org id)* | Deterministic payload test |
| SIG-AC-6 | The join resolves only to **company-level** data against our own relational core; **no** named-individual profile is created from the signal, and a signal not attributable to a company is **dropped**, not retained as a person-level dossier. *(test: orphan signal → 0 person rows created)* | Integration test; [[scope#NEVER-8]]; [[personas#PERSONA-PAT-GUARD-1]] |
| SIG-AC-7 | Any proposed outbound (intro draft, outreach) is **🟡 confirm-first**; an unconfirmed send sends nothing. *(contract test: tool tier 🟡)* | Contract/tool-tier test; [[acceptance-standards#GATE-AI-7]] |

#### Acceptance — deal-coaching capability ACs (verbatim)
Source: features/07-ai-native-moments.md#8b-deal-coaching-on-demand @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| SIG-AC-8 | Given a rep opens a stalled deal and requests coaching, the response returns a re-engagement angle, a voice-drafted follow-up, and a channel suggestion — each **carrying the evidence/source** it derived from; an ungrounded recommendation is a hard failure. *(deterministic test: assert evidence id on the angle; no-signal fixture → honest "not enough to coach", 0 fabricated angles)* | Deterministic AI-lane test ([[acceptance-standards#GATE-AI-1]]) |
| SIG-AC-9 | The drafted follow-up is **unsent (🟡)**; before the rep confirms, **zero** outbound occurs; edit-then-send sends the edited version, logged with provenance to the coaching suggestion. *(test)* | Integration test riding the approval gate |
| SIG-AC-10 | The channel suggestion states its reason (e.g. "they reply faster on email — 3 of last 4"). *(test)* | Deterministic payload test |
| SIG-AC-11 | Output renders the Art. 50 AI-assisted disclosure (§11 gate 9). | UI assertion ([[acceptance-standards#GATE-AI-9]]) |

#### Acceptance — screen: warm-signal card on the contact 360 (verbatim)
Source: product/30-screen-acceptance.md#personhtml--contact-360-implements-s-e02256-s-e081 @ 5a0b29c

The contact 360 screen belongs to people-and-organizations; the warm-signal
card's content ACs are owned here (that chapter's explicit hand-off). Corpus
screen-AC IDs preserved verbatim.

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-person-5 | Given the warm-room signal card (E08), When it renders, Then it shows a "Warm signal" flag, a headline, a "high confidence" indicator, an AI suggestion, and a "Show the evidence" toggle revealing the captured quote with source + why-warm rationale. | Screen/E2E test |
| AC-person-6 | Given the warm-signal action footer, When I click "Draft a reply" / "Send booking link" / "Create follow-up", Then each is confirm-first / accept-to-persist (nothing sent or persisted without approval); "Dismiss" dismisses the signal and feeds agent learning. | Screen/E2E test ([[acceptance-standards#GATE-AI-7]]) |

Note SIG-N-1 (open consent question, carried from people-and-organizations):
whether withdrawal of profiling consent suppresses the strength compute behind
the warm route, or only outbound to that person, is unresolved — the strength
formula is people's (PO-F-3), the warm-card behavior is this chapter's; the
answer must be pinned here before the warm-card ticket cuts.

#### Acceptance — screen: deal coaching (verbatim)
Source: product/30-screen-acceptance.md#coachinghtml--deal-coaching-implements-s-e085 @ 5a0b29c

This chapter owns the coaching screen (primary story S-E08.5). The coverage
screen (S-E09.6) is the reporting chapter's, per the screen→story index. Corpus
screen-AC IDs preserved verbatim.

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-coaching-1 | Given the deal header shows BÄR Pharma — Packaging QA flagged "Stalled · 23d no reply" and the coaching output is not yet shown, When the rep clicks "Ask for coaching", Then the button shows a spinner ("Coaching…"), the streaming progress list appears, and the four read steps (deal context, public activity, competing-priority inference, drafting) advance one at a time rather than dumping after a single spinner. | Screen/E2E test |
| AC-coaching-2 | Given the streaming pass completes, When the four steps finish, Then the progress list hides, the button relabels to "Re-run coaching", and the coaching output reveals three blocks — Re-engagement angle, Channel suggestion, and Drafted follow-up — scrolled into view. | Screen/E2E test |
| AC-coaching-3 | Given the Re-engagement angle block is shown, When the rep reads its supporting facts, Then each grounded fact carries an evidence quote, a source line with date and link (e.g. "Email · Dr. Bär · 02 Jun · grounded"; "Public post · resolved to Dr. Bär · 20 Jun · ADR-0006 scrape"), and a confidence chip (high / "medium — public, company-level"). | Screen/E2E test ([[acceptance-standards#GATE-AI-1]]) |
| AC-coaching-4 | Given a fact the model could not ground (budget-cycle timing), When the rep reviews the angle, Then it is shown as an explicit honest omission stating the date is not asserted, rather than filled with a guess. | Screen/E2E test (STATE-5) |
| AC-coaching-5 | Given the Channel suggestion block, When it renders, Then "A short call" is marked "Suggested" (highlighted) with a stated reason and "Email" is shown as a fallback, and a note clarifies the reasoning is from the deal's reply cadence, not a behavioural profile of the person (P12). | Screen/E2E test |
| AC-coaching-6 | Given the Drafted follow-up shows the German email with a "drafted in your voice" provenance chip, When the rep edits the draft text, Then the provenance chip changes to "edited by you". | Screen/E2E test |
| AC-coaching-7 | Given the draft, When the rep clicks "Queue to approval inbox", Then a toast confirms it was queued and that a human approves before it sends — the draft is not sent from this screen; the gate banner states the same 🟡 approval-gated behaviour. | Screen/E2E test ([[acceptance-standards#GATE-AI-7]]) |
| AC-coaching-8 | Given the coaching output is shown, When the rep clicks "Regenerate", Then a toast confirms a fresh revision is produced from the same evidence and the provenance chip resets to "drafted in your voice"; When the rep clicks "Dismiss", Then the output hides, the button resets to "Ask for coaching", and a toast confirms nothing was saved or sent. | Screen/E2E test |

Note SIG-AC-N-1: the coaching prototype's error state exists as markup but no
script path triggers it, and the no-permission case is descriptive text only —
the planned build must ship both as real rendered states per the standard
screen-state floor ([[acceptance-standards#Acceptance — standard screen-state matrix]]).
