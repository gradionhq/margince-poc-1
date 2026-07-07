---
status: planned
module: modules/onboarding (backend cold-start staging + wizard state) · web (onboarding wizard + read-a-company surfaces)
derives-from:
  - specs/spec/features/07-ai-native-moments.md#1-website-cold-start-read-back @ 5a0b29c
  - specs/spec/features/07-ai-native-moments.md#2-impressum--legal-imprint-reader @ 5a0b29c
  - specs/spec/features/09-onboarding-and-voice-tasks.md#a-onboarding-wizard-fd-13--value-first-5-step @ 5a0b29c
  - specs/spec/product/epics/E01-onboarding-coldstart.md @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#21-onboarding--cold-start @ 5a0b29c
  - specs/spec/contract/ai-operational-spec.md#21-cold-start-extraction-07-12 @ 5a0b29c
  - specs/spec/contract/events.md#5-the-catalog @ 5a0b29c
  - margince-poc/docs/subsystems/website-fetch.md @ a11d6c08
---
# Onboarding & cold-start — value before data entry, and never open with a guess

> The first-run subsystem: from a single pasted company URL the CRM reads the user's
> business back — every field carrying the verbatim snippet it was read from,
> ungrounded fields omitted — and stages everything behind an accept gate, so the
> workspace is non-empty before anything is typed and nothing is real until a human
> says so. The same read engine serves the in-app "read a company" surface after
> onboarding.

<!--
Contested / flagged items (for the spec-gate reviewer):
1. Table ownership: the data-model ownership index assigns ZERO tables to this
   chapter — reported honestly, no Schema section pinned. The staged read-back
   is linked to the approval inbox substrate (the approval item's proposal
   pointer; approval_item is notifications-and-approval-inbox's per the index,
   the wire/token mechanics are approvals-and-concurrency's), and accepted rows
   land in person/organization/activity/lead tables owned by their substrate
   chapters. The ticket-coverage gate should not expect DDL here.
2. Business-profile persistence gap: §A (A3.2, A6) and the voice-profile
   chapter both treat "the business profile from onboarding steps 1–2" as a
   real artifact (it plays the knowledge-base role for Voice DNA), but the
   66-table partition has no table for it and the contract has no operation
   reading or writing it. Carried as ONBOARD-AC-OPEN-2, not papered over.
3. Wizard-state persistence gap: A1.2 requires a server-side resumable state
   machine per user; no table and no contract operation exist for it at the
   pinned corpus version (ONBOARD-WIRE-N-2).
4. A5.4 (enter-CRM timing) is pinned here as the open decision of record
   (ONBOARD-AC-OPEN-1); the same question also surfaces on capture's
   activation-screen row (CAP-AC-OPEN-1, "whether Enter is available before
   the stream settles"). One decision, two screens' consequences — the ticket
   that resolves it must update both chapters.
5. The 0.55 confidence floor is a corpus constant of the AI operational spec
   (§2.1 cold-start prompt contract), not a pin of the ai-evals chapter; it is
   pinned here as ONBOARD-PARAM-2 with the eval bands (AIEVAL-4/5/6) cited to
   ai-evals, which owns them.
6. Screen-claim results: AC-onboarding-1..11 and AC-readco-1..10 were pinned
   in no other chapter (grep-verified) and are claimed here verbatim.
   AC-activation-1..9 are capture's and are cited, never re-pinned.
   AC-onboarding-6..8 pin voice-step behaviour on this chapter's screen; the
   panel's content (corpus meter, bands, starter profile) is voice-profile's
   domain — held here only as screen acceptance, mirroring capture's
   AC-activation-6 precedent.
7. read-company commit granularity: the prototype commits all-or-nothing while
   S-E01.4 requires accept-all-or-subset — carried as ONBOARD-AC-OPEN-3.
-->

## What it's for

The empty CRM is the universal first-run failure: every product that earns love is
non-empty before the user types. This subsystem opens earlier and with more trust
than any incumbent — before any mailbox is connected, before any record exists, the
new user pastes their company URL and the CRM reads their business back to them:
ICP, buying center, value proposition, USP, buying intents, each field showing the
exact source snippet it was read from, fields the site does not ground omitted
rather than guessed. It owns the value-first onboarding wizard (read → confirm →
voice → results → connect, in that order, the mailbox deliberately last), the
Impressum reader that fills the workspace's own organization from the legal page
nobody should retype, the accept-to-persist gate that keeps every proposal staged
until a human commits it, and the in-app read-a-company surface that reuses the same
engine on prospect URLs after onboarding. Its callers are the first-run entry (the
wizard is the front door for a fresh workspace and for each invited member) and the
companies surface in the app; downstream it hands off to [[capture]] at the
connect step (the activation moment is capture's) and to [[voice-profile]] at the
voice step (the corpus and artifact are that chapter's). The first-run journey it
carries is the demo script's opening minute ([[journeys]] J1).

## Principles it serves

- **P5 — auto-capture over manual entry.** The first minutes are the proof: the
  workspace becomes recognizably the user's business from one URL, and the one
  field the site could not ground becomes a single deliberate question rather than
  a form.
- **P12 — governance designed in.** The whole surface is a confirm-first staging
  area: evidence on every shown field, zero persistence before accept, provenance
  stamped on every committed row, per-purpose consent recorded as proof at the
  OAuth step ([[acceptance-standards#GATE-AI-1]], [[acceptance-standards#GATE-AI-2]]).
- **P7 — own your data.** The read fetch obeys the workspace egress posture:
  secret-stripping on any model-bound payload, and in the sovereign profile the
  cold-start flow completes with zero external egress
  ([[acceptance-standards#GATE-AI-5]]).
- **ADR-0006 — scrape/enrichment connector seam.** The read rides the shared,
  egress-hardened fetch-and-normalize seam (proven in the build skeleton), so the
  robots/ToS fallback and the egress discipline are structural, not per-surface.
- **ADR-0008 — lead object and promotion.** People read from a prospect's website
  are staged as segregated leads, never contact rows — machine-sourced and
  unengaged until genuine engagement promotes them.
- **FD-13 (provenance).** The wizard's shape is a dated product decision: deliver
  value and build investment *before* the big permission; the mailbox connect is
  the last step, behind an explicit consent screen.

## How it works

**Value before the big ask.** The wizard runs five steps in a fixed order
(ONBOARD-PARAM-3): read your website, confirm and refine, build your voice, see
what's ready, connect your inbox. Everything before the final step works without
granting any scope — the trust ask comes only after the product has proven it
understood the business. Leaving mid-wizard loses nothing: state is an explicit,
resumable machine persisted server-side per user, so a closed tab returns to the
same step with prior input intact.

**Onboarding is two-tier, by role.** The path splits on a decided rule
(ONBOARD-PARAM-4): the workspace creator runs the workspace-level steps — the
website read and the confirm/refine — because an empty workspace is not usable;
an invited member joining an existing workspace skips them (the business profile
already exists) and gets a short personal-only path: voice and inbox, both
skippable. A member can decline both personal steps and still land in a working,
populated workspace; the creator must complete the first two steps, while voice
and mailbox remain skippable even for them.

**The read is evidence-or-omit, progressively rendered.** Pasting a URL routes
through the shared scrape seam: fetch and parse the public site, then extract the
structured read-back. Each returned field carries the verbatim source snippet, a
link to the page section it was read from, and a confidence value; a field the
pages do not ground — or whose confidence falls below the floor
(ONBOARD-PARAM-2) — is omitted, never filled with a plausible-sounding guess. The
famously ungrounded field ("who buys this?") renders as an honest omission card
and becomes an explicit question at the confirm step, never an auto-fill. The
whole pipeline lands within its budget (ONBOARD-PARAM-1), shown field by field,
never spinner-then-dump.

**Failure is a fallback, never a wall.** A JS-only, empty, robots-disallowed, or
unreadable site produces an honest "couldn't read enough from this page" state
with retry and paste-text options — the pasted text flows through the same
normalize-and-extract path as a fetched page, so onboarding is never blocked by a
site we cannot or may not read ([[acceptance-standards#STATE-SP-3]],
[[acceptance-standards#GATE-AI-8]]). Zero fabricated fields on a failed read is a
hard eval gate, not a hope ([[ai-evals]] AIEVAL-6).

**Confirm and refine makes edits human, visibly.** The second step prefills
editable text from the read; an edit is marked typed-by-you — distinguishable from
the read value, with the original snippet retained for reference — and the omitted
buying-center is solicited as the user's own input. What is confirmed here becomes
the workspace's business profile, the artifact the voice model later treats as its
knowledge base ([[voice-profile]]).

**The Impressum reader kills the most-hated retype.** Alongside the read-back, the
engine locates the legal-imprint or about page and proposes the workspace's own
anchor organization pre-filled with legal name, registered address, register and
VAT numbers where present, industry, and stated history — each value carrying its
snippet and distinguishable as read-from-imprint. Where no legal page exists
(common outside DE/AT/CH) it falls back to what the main site grounds and leaves
legal-registry fields empty rather than inventing them. A proposed value that
collides with a human-edited field never overwrites it without a recorded 🟡
confirm ([[acceptance-standards#GATE-AI-4]]).

**Nothing is real until accepted.** The entire proposal — read-back fields, the
anchor organization, people found — is a staging surface: before accept, zero rows
exist in the real person, organization, or activity tables
([[acceptance-standards#GATE-AI-2]]; the stages-never-writes rule is
[[event-bus#EVT-SEM-11]]). Accepting all or a subset writes exactly those rows in
one audited transaction, each stamped as captured from the website with the source
URL, visibly distinguishable from human-typed rows forever after; rejecting or
walking away leaves the workspace unchanged. The accept decision itself rides the
same approval mechanics as every 🟡 action ([[approvals-and-concurrency]]).

**The same engine serves the app, with a lead discipline.** After onboarding, the
read-a-company surface points the identical fetch-extract-stage pipeline at a
prospect's URL from inside the app: company fields ground from the Impressum and
site, and the people it finds are staged as segregated leads — never contact
rows — with unpublished contact details omitted rather than fabricated
([[leads-and-qualification]], [[acceptance-standards#GATE-CS-2]] discipline via
ADR-0008). The commit bar states what will be created and stamps provenance on
accept.

**The connect step is a consent ceremony, not a checkbox.** The final step runs a
real OAuth flow for mail and calendar read scopes, and its four promises — read-only
scopes, never sends without approval, data stays in the workspace, one-click
disconnect — are enforced behaviors mapped to real controls, not copy. Granting
writes a per-purpose consent proof event, append-only, carrying the wording shown
([[gdpr-platform]] — consent defaults to deny per purpose, GDPR-AC-1/2/3);
skipping writes no scopes and leaves the app fully usable; disconnecting revokes
tokens and halts capture. On connect the flow hands off to capture's activation
moment — the watch-it-fill screen and its acceptance are [[capture]]'s
(AC-activation-1..9, cited not re-pinned) — and the sent-mail corpus levels the
starter voice up a band ([[voice-profile]]).

**Egress posture is inherited from the seam.** Any externally fetched, model-bound
payload passes the secret-stripper (hygiene only — no PII pseudonymization, per the
location-ladder decision), and under the sovereign profile the cold-start flow
completes with zero external egress, asserted by a spy transport
([[acceptance-standards#GATE-AI-5]]; the fetch base was proven in the build
skeleton's website connector).

**The funnel is instrumented.** Per-step drop-off, time-to-first-value from URL to
read-back, and the manual-entry smell ride the same telemetry as the capture KPIs,
so onboarding health is a number on the same dashboard, not a separate anecdote.

## What's configurable

- **Egress profile** — normal fetch versus sovereign zero-egress; selected by the
  workspace's deployment posture, not a per-read toggle
  ([[acceptance-standards#GATE-AI-5]]).
- **The extraction binding** — the read-back extraction runs on the workspace's
  configured model tier bindings; quality is certified per provider by the eval
  matrix ([[ai-evals]] AIUC-01/02), the deterministic gates hold on any brain.
- **The wizard path** — not a knob: the creator-versus-member split is derived from
  workspace state at wizard entry (no business profile yet → full path; joining an
  existing workspace → member path), per the decided two-tier rule
  (ONBOARD-PARAM-4).
- **Degradation** — an unreadable or disallowed site degrades to manual paste; an
  absent data provider never blocks anything here (ICP account surfacing is
  deferred outright, [[scope#S-E01.3]]).

## Guarantees (enforced)

- **Every shown field is grounded.** A rendered read-back or imprint field carries
  a non-empty verbatim snippet, a source link, and a confidence at or above the
  floor (ONBOARD-PARAM-2), or it does not render — a shown field with no evidence
  is a hard failure ([[acceptance-standards#GATE-AI-1]]; [[ai-evals]] AIEVAL-1).
- **Zero persistence before accept.** Before the human accepts, no cold-start
  proposal has produced a single row in a real domain table; querying the records
  proves it ([[acceptance-standards#GATE-AI-2]]; [[event-bus#EVT-SEM-11]]).
- **Never an error wall.** Scrape-unreadable and robots-disallowed both land on the
  manual-paste fallback state, and onboarding proceeds
  ([[acceptance-standards#STATE-SP-3]], [[acceptance-standards#GATE-AI-8]]).
- **Sovereign means zero egress.** The sovereign profile completes the cold-start
  read with no external payload leaving at all; the default profile secret-strips
  model-bound fetched payloads ([[acceptance-standards#GATE-AI-5]]).
- **Humans outrank the reader.** A human-edited field is never overwritten by an
  imprint or read value without a recorded 🟡 approval
  ([[acceptance-standards#GATE-AI-4]]).
- **Accepted rows confess their origin.** Every committed row is stamped with the
  website source and agent provenance, in one audit transaction emitting one
  domain event ([[acceptance-standards#GATE-CORE-3]],
  [[acceptance-standards#GATE-CORE-5]]).
- **Consent is per-purpose and default-deny.** The OAuth step records a consent
  proof per purpose granted; an unchecked purpose yields no grant, and skipping
  yields no scopes at all ([[gdpr-platform]]).
- **The wizard never traps.** Any step can be left and resumed with input intact;
  a member can skip every personal step and still reach a working, populated app;
  prospect people from the in-app read stay leads until genuine engagement
  (ADR-0008).

## Acceptance

Done means a new user's first five minutes earn belief: they paste a URL and watch
their own business read back field by field, each with the quote it came from; the
one thing the site never said is asked, not invented; an unreadable site says so
plainly and offers a paste box instead of an error; nothing they saw exists as a
record until they accept, and everything they accept is forever tellable from what
they typed. An invited teammate gets the short path and can decline all of it and
still land in a populated workspace. The connect step states four promises that are
real controls, records consent per purpose, and hands off to the activation moment
— honest about progress, never fake-populated (capture's screen). The testable
forms live in the Acceptance appendix; the standard screen-state floor and
performance budgets are inherited from [[acceptance-standards]] and not restated —
except the manual-paste fallback, which this chapter's screens own
(STATE-SP-3).

## Out of scope

- **The activation moment.** The watch-it-fill screen after connect, its counters,
  stream, and honest degradation are [[capture]]'s (AC-activation-1..9); this
  chapter only navigates into it.
- **The voice artifact.** The build-your-voice step renders on this chapter's
  wizard, but the corpus, meter, bands, profile, and rebuild lifecycle are
  [[voice-profile]]'s; step-3 screen rows here verify against that chapter's pins.
- **ICP account surfacing.** Live ICP-matched accounts during onboarding
  (S-E01.3) are deferred Fast-follow — needs a paid data provider; the honest
  degradation when none is connected is owned by this chapter's no-guess gate
  ([[scope#S-E01.3]]).
- **The shared fetch base.** The egress-hardened fetch-and-normalize connector is
  the scrape seam this chapter composes (ADR-0006, proven in the build skeleton);
  the deep-research profiler that also rides it is [[meetings-and-transcripts]]'s.
- **Approval mechanics.** Token minting, TTL, diff binding, and the inbox UI the
  accept decision rides are [[approvals-and-concurrency]]'s; the staged-item
  table is the notifications-and-approval-inbox chapter's per the data-model
  ownership index.
- **Mailbox and calendar capture.** The OAuth connection's downstream — sync,
  exclusion, auto-create — is [[capture]]'s; this chapter owns only the consent
  ceremony that grants it.

## Where it lives

The backend onboarding module — cold-start staging, wizard state, and the wiring
onto the shared scrape seam — with the onboarding wizard and the read-a-company
screen as its owned surfaces. Read next: [[capture]] (the activation handoff and
the pipeline the connect step feeds), [[voice-profile]] (the step-3 artifact),
[[approvals-and-concurrency]] (the accept gate's mechanics),
[[gdpr-platform]] (the consent substrate), and [[leads-and-qualification]] (where
read-a-company people land).

## Appendix

### Parameters
Source: features/07-ai-native-moments.md#1-website-cold-start-read-back + features/09-onboarding-and-voice-tasks.md#a-onboarding-wizard-fd-13--value-first-5-step + contract/ai-operational-spec.md#21-cold-start-extraction-07-12 @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| ONBOARD-PARAM-1 | Read-back extraction budget | p95 < 8 s end-to-end | Fetch + parse + extract for the five-field read-back, rendered progressively, never spinner-then-dump (features/07 §1); the Impressum locate+parse shares the same budget (features/07 §2). Perf test, model-bound on the extract step. |
| ONBOARD-PARAM-2 | Cold-start confidence floor | 0.55 (below → omit) | Per-field confidence floor for the cold-start extraction — higher than the 0.5 task default because web text is noisier (ai-operational-spec §2.1/§5.1, dropped before render, never shown). The extraction quality bands (AIEVAL-4/5) and the empty-page fabrication zero (AIEVAL-6) are [[ai-evals]]'s — cited, not owned here. |
| ONBOARD-PARAM-3 | Wizard shape | 5 steps: Read · Confirm · Voice · Results · Connect | The value-first order with the mailbox connect deliberately last, behind the explicit consent screen (features/09 §A; FD-13). |
| ONBOARD-PARAM-4 | Two-tier step split | steps 1–2 workspace-scope (creator must run); steps 3+5 per-user (always skippable); member path = steps 3–5 only | Decided 2026-06-10: an invited member (user #2+) skips the business-profile steps and may skip their personal steps entirely; the workspace creator must complete steps 1–2 (features/09 §A0). |
| ONBOARD-PARAM-5 | Voice-build enable minimum | 4,000 words | The wizard's "Build my voice profile" control stays disabled below this corpus size (AC-onboarding-6); the ~30k target and band vocabulary are [[voice-profile]]'s VOICE-PARAM-1/3 — cited, not re-pinned. |

### Wire
Source: contract/crm.yaml @ 5a0b29c

Operations are cited by operationId, never restated.

| ID | Operation | Note |
|---|---|---|
| ONBOARD-WIRE-1 | `coldStartReadback` | This chapter's owned operation ([[capture]] CAP-WIRE-N-3 explicitly assigns it here). A 🟡-tier staging operation on the governed tool surface: takes a URL, returns a staged proposal whose every field carries non-empty evidence + source + confidence or is absent (the schema makes evidence required — no-guess by construction); status is always staged, nothing is written. Its field vocabulary spans both the §1 read-back fields and the §2 imprint fields. Honest degradation is a 422 with error code `coldstart_unreadable` ("couldn't read enough — retry or paste text", zero populated fields), never a fabricated proposal. |

Note ONBOARD-WIRE-N-1: accept and reject do not have cold-start-specific
operations — the staged proposal is decided through the approval inbox surface
(`listApprovals` / `getApproval` / `approveApproval` / `rejectApproval`,
[[approvals-and-concurrency]] APPR-WIRE-3/4); the approval item of kind
`coldstart` carries the proposal pointer linking the staged read-back to its
decision.

Note ONBOARD-WIRE-N-2: **no wizard-state operations exist in the contract at the
pinned corpus version** — A1.2's server-side resumable state machine (step,
read-back, edits, voice-corpus draft, connect state) has no operation and no
table in the 66-table partition. The wizard surface's operations must be added
contract-first at ticket time; this chapter owns them when they land.

Note ONBOARD-WIRE-N-3: the mailbox OAuth connect at step 5 is a per-user OAuth +
consent flow, deliberately not a contract endpoint ([[capture]] CAP-WIRE-N-1);
the consent proof it writes is [[gdpr-platform]]'s substrate (`recordConsent` /
`listConsentPurposes` are that surface's, cited not owned).

Note ONBOARD-WIRE-N-4: **no business-profile operation exists** — the confirmed
step-1/2 artifact (ICP, value proposition, buying center, typed-by-you edits)
that [[voice-profile]] treats as the knowledge base has no read/write surface at
the pinned version. Carried as ONBOARD-AC-OPEN-2.

### Events
Source: contract/events.md#5-the-catalog @ 5a0b29c — definitions live in the central catalog ([[event-bus]]); cited, never redefined. The cold-start stream is [[event-bus#EVT-STREAM-8]].

| ID | Emitted/consumed | Definition |
|---|---|---|
| `coldstart.read_back_proposed` | Emitted once per staged read-back (source URL, the evidenced fields, a degraded flag); writes nothing to real tables — the staging-only half of [[event-bus#EVT-SEM-11]]. Emitter: the capture-side scrape seam (ADR-0006); consumer: the staging-card read model. | [[event-bus]] catalog (Cold-start) |
| `coldstart.accepted` | Emitted once on human accept (source URL, accepted fields, the produced entity ids); the trigger for the actual person/organization/activity writes, stamped `source=coldstart` / `captured_by=agent:coldstart`, whose own created events share one correlation id. Consumers: context graph, read model, audit stream. | [[event-bus]] catalog (Cold-start) |
| `coldstart.rejected` | Emitted once on human reject (source URL, optional reason); nothing was ever written. Consumer: audit stream. | [[event-bus]] catalog (Cold-start) |

Note ONBOARD-EVT-N-1: the stages-never-writes rule is pinned as
[[event-bus#EVT-SEM-11]] — only `coldstart.accepted` triggers real writes, and
every proposed field carries its evidence snippet or is absent. The accept
decision itself also rides the approval pair's correlation semantics
([[event-bus#EVT-SEM-9]], [[approvals-and-concurrency]]).

### Acceptance
Source: product/epics/E01-onboarding-coldstart.md#s-e011--paste-your-website--see-your-business-read-back + #s-e012--impressum--legal-imprint-reader-fills-the-company-so-nobody-retypes-an-address + #s-e014--accept-to-persist-nothing-hits-your-records-until-you-say-so @ 5a0b29c; product/20-traceability.md @ 5a0b29c

**Owned stories** (primacy verified: the scope roll-up assigns epic E01 to this
chapter as sole owning chapter — 3 WOW + 1 FF — and the traceability register
maps S-E01.1/.2/.4 to features/07 §1/§2; S-E01.3 is deferred to the scope OUT
list and owned there, [[scope#S-E01.3]]):

| ID | Story | Tier | Home |
|---|---|---|---|
| S-E01.1 | Paste your website → see your business read back | V1-WOW | this chapter |
| S-E01.2 | Impressum / legal-imprint reader fills the company | V1-WOW | this chapter |
| S-E01.3 | Real accounts and leads surfaced while you read | Fast-follow | **deferred** — [[scope#S-E01.3]] (needs a paid data provider); the V1 honest-degradation behaviour is owned by this chapter's no-guess gate |
| S-E01.4 | Accept-to-persist: nothing hits your records until you say so | V1-WOW | this chapter |

Stories condensed to their Given/When/Then load:

| ID | Given/When/Then | Verification |
|---|---|---|
| S-E01.1 | V1-WOW. Given a fresh workspace and my company URL, when I submit it, then a structured read-back card renders ICP, buying center/roles, value proposition, USP, and buying intents — each field with its verbatim source snippet and page link; an ungrounded field is absent or marked "not found on your site", never guessed; a failed/JS-only fetch yields an honest "couldn't read enough" state with retry/paste-text; an inline edit is marked typed-by and keeps the original snippet. | ONBOARD-AC-1..3/6 below; [[ai-evals]] AIUC-01; live-stack walkthrough ([[testing#TEST-LANE-3]]) |
| S-E01.2 | V1-WOW. Given my company URL, when the read runs, then the Impressum/legal page is located and my own organization is proposed pre-filled (legal name, registered address, register/VAT, industry, history), each value evidenced and distinguishable as read-from-imprint; no Impressum → grounded main-site fallback with legal-registry fields left empty; on accept it becomes the workspace's anchor org, stamped captured-from-website. | ONBOARD-AC-8..12 below; [[ai-evals]] AIUC-02 |
| S-E01.4 | V1-WOW. Given any cold-start proposal, when I have not accepted, then zero rows exist in real person/organization/activity tables; accepting all or a subset writes exactly those rows, stamped with website provenance and visibly distinguishable from typed rows; reject/ignore leaves the workspace unchanged; accepted rows remain findable and removable by their provenance. | ONBOARD-AC-4/5 below; [[acceptance-standards#GATE-AI-2]]/[[acceptance-standards#GATE-AI-3]] |

Source: features/07-ai-native-moments.md#1-website-cold-start-read-back @ 5a0b29c

§1 acceptance criteria, verbatim in load (the corpus bullets are unnumbered; new
chapter-scoped IDs per ACID-2):

| ID | Given/When/Then | Verification |
|---|---|---|
| ONBOARD-AC-1 | Given a fetchable URL, the read-back card renders the five labelled fields within a Day.ai-class time-to-wow; extraction pipeline p95 < 8 s end-to-end (fetch + parse + extract), shown progressively, never as a spinner-then-dump. | Perf test, model-bound on extract (ONBOARD-PARAM-1; [[testing#TEST-LANE-2]]) |
| ONBOARD-AC-2 | Every returned field has a non-empty `evidence_snippet` (verbatim page text) + `source_url` + `confidence`, or the field is absent — a rendered field with an empty/missing snippet is a hard failure. Fixture with an unstated ICP → ICP field absent or explicitly "not found on your site", never populated. | Deterministic test: assert `∀ field: snippet ≠ ""` over the response ([[acceptance-standards#GATE-AI-1]]; [[ai-evals]] AIEVAL-1) |
| ONBOARD-AC-3 | Given a JS-only/empty/failed fetch, the surface returns an honest "couldn't read enough from this page" state with retry/paste-text, and zero fabricated fields. | Deterministic test: empty-page fixture → 0 populated fields ([[ai-evals]] AIEVAL-6; [[acceptance-standards#STATE-SP-3]]) |
| ONBOARD-AC-4 | Given the staged proposal before accept, zero rows exist in real `person`/`organization`/`activity` tables from it. | Deterministic test: query → 0 rows (🟡 gate; [[acceptance-standards#GATE-AI-2]]; [[event-bus#EVT-SEM-11]]) |
| ONBOARD-AC-5 | Given accept-all or accept-subset, exactly those rows are written, each stamped `captured_by=agent:coldstart`, `source=coldstart`, with the source URL, in one `audit_log` transaction emitting one domain event. | Integration test ([[acceptance-standards#GATE-CORE-5]]; [[testing#TEST-LANE-2]]) |
| ONBOARD-AC-6 | Given inline edit of a read value, the row is marked `captured_by=human:*` (typed-by) and the original snippet is retained for reference. | Integration test ([[testing#TEST-LANE-2]]) |
| ONBOARD-AC-7 | External payload destined for a frontier model is recorded as having passed the secret-stripper on egress, and in the `sovereign` profile no external payload leaves at all. | Egress conformance test ([[acceptance-standards#GATE-AI-5]]; spy-transport assert per the skeleton precedent @ a11d6c08) |

Source: features/07-ai-native-moments.md#2-impressum--legal-imprint-reader @ 5a0b29c

§2 acceptance criteria, verbatim in load:

| ID | Given/When/Then | Verification |
|---|---|---|
| ONBOARD-AC-8 | Given a URL with an Impressum, the reader proposes an org pre-filled with legal name + registered address + register/VAT (where present) + industry + history, each field carrying a non-empty snippet + confidence or omitted. DE fixture → address populated with snippet; US fixture with no Impressum → register/VAT fields empty, not invented. | Deterministic test: assert evidence on every populated field ([[ai-evals]] AIUC-02) |
| ONBOARD-AC-9 | Given a value read from the imprint, it is queryable as `captured_by=agent:coldstart` / read-from-imprint, distinct from human entry. | Schema + query test ([[testing#TEST-LANE-2]]) |
| ONBOARD-AC-10 | Given a proposed field that collides with an existing human-edited value on the anchor org, the write is blocked pending a 🟡 confirm; an unconfirmed overwrite changes nothing. | Deterministic test: human-set address + imprint address → no overwrite without approval token ([[acceptance-standards#GATE-AI-4]]) |
| ONBOARD-AC-11 | Given accept, the anchor org is written stamped `source=coldstart` with the source URL, one audit row + one event; the address is never re-presented as a blank field. | Integration test ([[testing#TEST-LANE-2]]) |
| ONBOARD-AC-12 | The Impressum read shares the §1 fetch budget (p95 < 8 s locate+parse) and the egress invariant (secret-strip + sovereign zero-egress). | Perf + egress tests (ONBOARD-PARAM-1; ONBOARD-AC-7) |

Source: features/09-onboarding-and-voice-tasks.md#a-onboarding-wizard-fd-13--value-first-5-step @ 5a0b29c

§A task exit gates, verbatim in load:

| ID | Given/When/Then | Verification |
|---|---|---|
| ONBOARD-AC-13 | (A0) An invited member's wizard shows only steps 3–5 against the existing workspace; skipping their personal steps still reaches a working, populated app. | Screen e2e lane ([[testing#TEST-LANE-3]]); ONBOARD-PARAM-4 |
| ONBOARD-AC-14 | (A1) A user can leave at any step and return to the same step with prior input intact; skip paths reach a coherent app state. | Backend integration + screen e2e lane (resumable server-side state; ONBOARD-WIRE-N-2 must land first) |
| ONBOARD-AC-15 | (A2) Paste a real URL → grounded read-back with per-field sources in p95 < 8 s; an unreadable URL produces the honest-failure state, not a guess. | Live-stack walkthrough ([[testing#TEST-LANE-3]]); backed by ONBOARD-AC-1..3 |
| ONBOARD-AC-16 | (A3) Edits persist as typed-by-you provenance; the buying-center captured at the confirm step flows into the business profile. | Backend integration lane; persistence home is ONBOARD-AC-OPEN-2 |
| ONBOARD-AC-17 | (A4) OAuth grants real scopes; a `consent_event` proof row exists per granted purpose; skip leaves a working app with no mailbox; disconnect revokes tokens and halts capture. | Backend integration lane; proof-log semantics per [[gdpr-platform]] (GDPR-AC-3 append-only wall); capture halt per [[capture]] |
| ONBOARD-AC-18 | (A5) On a real connected mailbox, the activation screen reflects true capture progress and degrades honestly on a slow/empty inbox. | **[[capture]]'s screen** — verified there (AC-activation-1..9, CAP-PARAM-2); cited so the wizard's handoff has a pinned landing |
| ONBOARD-AC-19 | (A6) Funnel drop-off per step, time-to-first-value (URL → read-back), and the manual-entry smell are observable in the same telemetry as the capture KPIs. | Metric-exists test ([[testing#TEST-LANE-2]]; the smell metric itself is [[capture]] CAP-PARAM-3) |

Source: product/30-screen-acceptance.md#indexhtml--onboarding-wizard-value-first-5-step-implements-s-e011-s-e012-s-e014 @ 5a0b29c

*Onboarding wizard — the paste-URL entry surface (implements S-E01.1, S-E01.2,
S-E01.4). Corpus IDs preserved verbatim; claimed by this chapter (pinned nowhere
else, grep-verified). The nav rail is intentionally absent — wizard chrome is a
named exception to the global-chrome floor:*

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-onboarding-1 | Given the wizard at load, When the top bar renders, Then a 5-dot stepper shows Read · Confirm · Voice · Results · Connect with step 1 active, a "Skip setup" link to home.html is present, and the Step-1 panel shows the headline, the "we only read your public website / no login required" trust pill, and a prefilled URL input. | Screen e2e lane ([[testing#TEST-LANE-3]]) |
| AC-onboarding-2 | Given Step 1, When I click "Read my business", Then the button shows a spinner + "Reading…", and three labelled read-back fields (Ideal customer, Value proposition, What makes you different) appear one-by-one, each with a 🟢 "grounded" confidence dot, a "read from site" provenance tag, and a collapsed "source" disclosure. | Screen e2e lane |
| AC-onboarding-3 | Given a rendered read-back field, When I click its "source" toggle, Then it expands to show the verbatim quote snippet and the exact page path it was read from, and the chevron rotates; clicking again collapses it. | Screen e2e lane |
| AC-onboarding-4 | Given the read-back completes, When the final result renders, Then a dashed "Who buys this" omission card appears stating "We couldn't find this on your site — so we won't guess", the read button resets to "Read again", and the footer "Continue" button becomes enabled (disabled until a read has run). | Screen e2e lane |
| AC-onboarding-5 | Given Step 2 (Confirm & refine), When it renders, Then it shows editable "What you sell" and "Ideal customer" textareas prefilled from the read, plus an empty "Who buys this?" input visibly flagged "we couldn't read this — your turn"; copy states edits are saved as "typed by you". | Screen e2e lane |
| AC-onboarding-6 | Given Step 3 (Build your voice), When I toggle source tiles (transcripts, posts, longform, chat, voice memos), Then the word meter sums each source's words toward the 30,000 target, the fill bar and quality label update (thin/good/rich), the spoken/written mix and source count update, and "Build my voice profile" stays disabled until at least 4,000 words are added. | Screen e2e lane; meter/band content verifies with [[voice-profile]] (VOICE-PARAM-1/3); ONBOARD-PARAM-5 |
| AC-onboarding-7 | Given Step 3, When I view the "Sent emails" tile, Then it is rendered locked (🔒, "+18,000 words when connected"); clicking it does not add words and shows a toast explaining it unlocks at inbox-connect. | Screen e2e lane; the level-up arc is [[voice-profile]]'s |
| AC-onboarding-8 | Given enough words added, When I click "Build my voice profile", Then after a modelling spinner a starter-voice card appears with a "good"-level confidence dot, word/source counts, voice stats, signature moves, and a sample draft in my voice, plus copy that connecting the inbox pushes it "good → sharp". | Screen e2e lane; starter-profile content verifies with [[voice-profile]] (VOICE-AC-7) |
| AC-onboarding-9 | Given Step 4 (Results), When it renders, Then it shows four green-checked ready cards (Business profile, Writing voice, Sales pipeline, Sample draft) and a sample email draft, with a "Still nothing connected" lock trust pill; the footer CTA reads "Connect my inbox". | Screen e2e lane |
| AC-onboarding-10 | Given Step 5 (Connect), When it renders, Then it shows a "Continue with Google" button and four explicit scope/consent statements (read mail+calendar, never-sends-without-approval, data-stays-in-your-workspace, one-click disconnect), plus a "Skip for now" link to home.html; clicking Continue shows "Connecting securely…" then navigates to activation.html. | Screen e2e lane; the four statements map to enforced behaviors (ONBOARD-AC-17; [[acceptance-standards#GATE-AI-7]]; per-user connect scope [[runtime-config#RC-8]]) |
| AC-onboarding-11 | Given any step beyond 1, When the footer renders, Then a "Back" control returns to the prior step; steps 2–3 also expose a "Skip this step" link that advances without input; the stepper marks completed steps with a check. | Screen e2e lane; skip semantics per ONBOARD-PARAM-4 |

Source: product/30-screen-acceptance.md#read-companyhtml--in-app-read-a-company-from-url-implements-s-e011-s-e012-s-e014 @ 5a0b29c

*Read a company — the in-app URL read surface (implements S-E01.1, S-E01.2,
S-E01.4 post-onboarding). Corpus IDs preserved verbatim; claimed by this chapter
(the companies list's navigation row into it is [[people-and-organizations]]'s
AC-companies-6 — cited, no overlap):*

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-readco-1 | Given the screen at load, When it renders, Then it shows a breadcrumb (Companies · Read a company), an intro explaining people land as leads until they engage and "nothing is saved until you accept", a prefilled URL input, and a teal lead-segregation note citing ADR-0008. | Screen e2e lane ([[testing#TEST-LANE-3]]) |
| AC-readco-2 | Given the input section, When I click "Read company", Then the input hides and a reading section shows the target domain and a five-step progress list (Fetching · robots.txt OK, Secret-stripped on egress · EU-region, Reading business & intents, Finding Impressum, Grounding every field) advancing one step at a time with checks. | Screen e2e lane |
| AC-readco-3 | Given reading completes, When results render, Then a banner reads "Here's what we read from <domain>" with a "Nothing saved yet" staged flag, and three grouped sections appear: Company (read from Impressum), What they do (grounded only), People found (→ created as leads). | Screen e2e lane |
| AC-readco-4 | Given the Company section, When fields render, Then legal name, registered address, register/VAT, and industry each appear striped (staged), with a 🟢 "grounded" dot, a "read" provenance tag, and an expandable "source" disclosure showing the verbatim quote and source path with "secret-stripped on egress". | Screen e2e lane |
| AC-readco-5 | Given any field's "source" toggle, When I click it, Then the evidence box expands to show the quote and grounded source line; clicking again collapses it. | Screen e2e lane |
| AC-readco-6 | Given the People found section, When it renders, Then each person shows a striped row with avatar, name, role · company, and a LEAD tag — staged as leads, not contacts. | Screen e2e lane; lead-not-contact per ADR-0008 ([[leads-and-qualification]]) |
| AC-readco-7 | Given people cannot be grounded for direct contact details, When the People section renders, Then a dashed omission card states "Direct dials / emails — Not published on the site — we won't guess", offering enrichment-after-engagement instead of fabricating. | Screen e2e lane; [[acceptance-standards#GATE-AI-1]] |
| AC-readco-8 | Given results are shown, When the commit bar appears, Then it summarizes "1 company + N leads ready to create · stamped source=read-company" with a Cancel link (companies.html) and a primary "Create records" button. | Screen e2e lane |
| AC-readco-9 | Given the commit bar, When I click "Create records", Then a toast confirms creation with lead segregation, source=read-company provenance, and one audit entry; the button changes to "Created ✓"; I navigate to company.html. | Screen e2e lane; write path per ONBOARD-AC-5's shape |
| AC-readco-10 | Given the in-app shell, When the screen loads, Then the Ledger-Green nav rail + ⌘K palette are available and the staged commit bar is offset to clear the rail. | Screen e2e lane (global chrome floor) |

Both owned screens additionally assert the manual-paste fallback special state —
scrape-unreadable and robots-disallowed land on paste-text, never an error wall
([[acceptance-standards#STATE-SP-3]]) — and inherit the standard state matrix
(STATE-1..5) without restating it.

**Open build decisions (carried honestly — the build tickets must resolve them).**
Source: features/09-onboarding-and-voice-tasks.md#d-open-decisions + product/30-screen-acceptance.md#21-onboarding--cold-start @ 5a0b29c

| ID | Decision needed | Verification |
|---|---|---|
| ONBOARD-AC-OPEN-1 | **A5.4 — "Enter your CRM" timing (the one §A decision still open):** available immediately after connect (capture continues async) or only after a first captured batch? The decomposition's recommended default: enter immediately, capture continues in the background with a progress indicator on home. The same question surfaces on [[capture]]'s activation screen (CAP-AC-OPEN-1, "whether Enter is available before the stream settles"); one decision, resolved once, both chapters updated. | Ticket-gate: the activation/wizard handoff ticket must record the decision before build |
| ONBOARD-AC-OPEN-2 | **Business-profile persistence home:** the confirmed step-1/2 artifact (read fields + typed-by-you edits + the solicited buying-center) must flow into a durable business profile (A3 exit gate; [[voice-profile]] reads it as the knowledge base), but the 66-table partition assigns no table and the contract no operation for it. The ticket must land its schema + contract surface, contract-first. | Ticket-gate: schema + wire must exist before ONBOARD-AC-16 is testable |
| ONBOARD-AC-OPEN-3 | **Read-a-company commit granularity:** the prototype commits all-or-nothing while S-E01.4 requires accept-all-or-subset; inline edit of staged values exists in the wizard but not on the in-app surface; the staged lead count must be derived, not hardcoded. | Ticket-gate: the read-a-company ticket must reconcile to S-E01.4's accept-subset before build |
| ONBOARD-AC-OPEN-4 | **Prototype gaps carried to build (the flagged §3 cross-cutting gaps applied here):** the honest "couldn't read enough" failure state exists in neither prototype screen (required by S-E01.1/ONBOARD-AC-3 and STATE-SP-3); URL validation/normalization (scheme, host, trailing-slash dedupe) is missing at the wizard input; step-2 edits are not persisted/marked typed-by in the prototype DOM. All are build requirements, not optional polish. | Ticket-gate: the wizard + read-a-company tickets must close these against ONBOARD-AC-3/6/16 |
