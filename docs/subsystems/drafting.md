---
status: planned
module: modules/drafting (backend) · web (composer draft surface, asset library, LinkedIn draft)
derives-from:
  - specs/spec/features/02-capture-and-comms.md#feature-6--baseline-ai-comms-summaries-draft-replies-nl-search
  - specs/spec/features/07-ai-native-moments.md#7b-governed-proof-point--asset-library-for-drafting
  - specs/spec/features/08-client-surfaces.md#2b-linkedin-draft-only-channel
  - specs/spec/product/epics/E07-voice-and-drafting.md
  - specs/spec/contract/data-model.md#voice--drafting-e07--features07-features09
  - specs/spec/product/30-screen-acceptance.md#asset-libraryhtml--approved-asset-library-implements-s-e073
  - specs/spec/product/30-screen-acceptance.md#linkedin-drafthtml--linkedin-message-in-my-voice-implements-s-e074
  - margince-poc/docs/subsystems/drafts.md @ a11d6c08
---
# Drafting — the CRM writes the reply; only a human sends it

> The baseline "write this for me" surface: on-demand summaries and draft replies grounded
> in the captured record, restyled to the rep's own voice, built only from admin-approved
> claims — and always **returned for review**. Drafting is 🟢 and changes nothing; sending
> is a separate 🟡 governed step that no draft can trigger by itself.

## What it's for

A rep who has to re-read a thread and compose from a blank composer loses deals to the
follow-up they never got around to writing. Drafting turns the context the CRM already
captured — the live thread, the deal stage, the open commitment — into a ready-to-edit
draft and a cited summary, bundled in the license for every user with no agent seat and no
per-seat AI metering. Its callers are the composer on a captured thread, the record and
deal surfaces that ask for summaries, the LinkedIn drafting surface, the recommenders that
pre-draft a next move (the overnight agent's recovery drafts and the coaching moment reuse
this path from their own chapters), and BYO agents through the governed draft verb. The
natural-language-search half of the same baseline feature is not this chapter's — it
belongs to the retrieval chapter ([[retrieval-seam]]). The approval mechanics behind the
send step belong to [[approvals-and-concurrency]]; the rep's learned writing style belongs
to [[voice-profile]].

## Principles it serves

- **P6 — ship the baseline the agent-less majority needs.** Summaries and draft replies
  are the L2 baseline, in the license, never metered; depth escalates to the user's own
  agent rather than out-labbing the labs.
- **P5 — capture-first pays off here.** Drafts and summaries read the auto-captured
  timeline; no manual notes are needed for the draft to know what was actually said.
- **P12 — governance designed in.** Every draft is grounded and cited; every claim-bearing
  statement traces to an approved asset; sending rides the confirm-first gate; every send
  is one audited, evented mutation.
- **ADR-0020 — customer-supplied inference.** Drafting runs on the workspace's own model
  (BYOK or self-host): no vendor markup, no metered-credit hard stop.
- **ADR-0003 — BYO agent plus baseline AI.** The baseline draft is also the starting input
  a Layer-1 agent refines — one continuous flow, not a separate tool.
- **ADR-0009 — substrate, not brain.** The asset library is governed retrieval for
  drafting, deliberately not a content-management product.

## How it works

**The baseline draft.** From a captured activity, the rep (or an agent, over the governed
tool surface) asks for a reply or follow-up, optionally steering it with a stated intent.
The drafter assembles the thread and deal context through the retrieval seam, generates
through the workspace's own model, and returns the drafted subject and body **plus the
identifiers of the captured records it drew from** — the draft is traceable to its
grounding, never free generation (GATE-AI-1). The draft lands in the composer, freely
editable; nothing is sent, logged, or persisted as outbound by the act of drafting. The
draft-reply quality bands (usefulness, hallucinated-fact rate) are pinned in the eval
catalog ([[ai-evals]] AIEVAL-12/13; use cases AIUC-21, AIUC-13).

**Summaries.** The same surface answers "summarize this thread / account / deal" from the
captured timeline. Every summary point cites the records it derives from, clickable back
to the source activities; an empty timeline yields an honest empty summary. Summaries
render asynchronously and never block the record view. Factuality and citation-validity
bands are the eval catalog's ([[ai-evals]] AIEVAL-10/11; AIUC-22).

**The send step is a different operation.** Sending is outbound and irreversible, so it is
🟡 confirm-first: an agent caller must present an approval token minted by a human decision
([[approvals-and-concurrency]], APPR-WIRE-1); a human pressing send *is* the approval and
carries no token. The send is additionally consent-gated default-deny per purpose — without
an active granted consent for the send's purpose it is suppressed, with zero transport. On
success exactly one audit row is written and the sent email lands as an outbound activity
on the timeline. Until that confirm, a draft has no "sent" state and no outbound activity
exists — visibly true to the rep (GATE-AI-7). Both operations ride under an activity on the
wire but are this chapter's, ceded by the timeline chapter at its ACT-WIRE-N-1 note.

**Voice-matched drafting.** When the rep has a ready voice profile, the draft is generated
in their register — greeting, sentence length, sign-off, bluntness — honouring the
profile's anti-patterns, and stamped with the voice model version that produced it. With
no profile, drafting falls back cleanly to the generic baseline. The profile itself — the
corpus, the learning loop, rebuild and rollback — is [[voice-profile]]'s; this chapter
consumes it and re-asserts the same send gate on the voice path, so "style transfers,
judgment never auto-acts" cannot be bypassed by sounding confident.

**The governed asset library.** A small, admin-curated store of *approved things to say* —
the sanctioned claim, the current case-study link, the live one-pager — each with a kind,
an owner, an approval state, and an optional expiry. When a draft needs a claim or proof
point, the drafter retrieves from the **available set only** (approved and not expired) and
surfaces which asset it used inline, so the rep sees the source before sending and the
admin can trust every rep's draft is built from sanctioned material. A stale or expired
asset is flagged and withheld, never silently inserted. Curation (create, update, approve,
expire, retire) is role-gated to the curator; a rep gets read-only retrieval. The hard
scope line: this is retrieval-for-drafting, **not a CMS** — no folders-of-folders, no
publishing workflow, no authoring/rendering or campaign surface, enforced by a static
retrieval-only scope guard (AIUC-14). Citing an asset shapes the draft only; the send gate
is untouched.

**The LinkedIn draft-only channel.** From a CRM contact, lead, or deal, the rep asks for a
LinkedIn message — a connection note or a longer InMail — drafted in their voice and built
from approved assets, grounded in the real relationship context. The product hands the rep
the text to **copy and send themselves**: Margince never auto-posts, auto-connects, or
auto-messages, because LinkedIn's terms forbid automation and the rep's account must stay
safe. There is no send control on this surface at all — the strictest form of the send
gate. When the rep later records that they sent it, the draft and the outreach are logged
against the record as an activity with drafted-vs-typed provenance, without Margince ever
having touched LinkedIn.

<!-- S-E07.4 surface-split: this chapter owns the LinkedIn *draft mechanics* — voice-styled
generation, approved-asset grounding, copy-only handoff, and the log-it-yourself activity.
The browser-extension host surface (and the entirely separate S-E12.2 one-click
profile→lead capture) is the client-surfaces chapter's, per features/08 §2/§4. -->

**Escalation and degradation.** A user with a Layer-1 agent hands the baseline draft to it
as input for research and refinement — the same flow, deeper. When the per-workspace AI
budget guardrail trips, drafting and summaries degrade gracefully (disabled or queued)
while core CRM operations are unaffected; the feature hides, the CRM never blocks
(GATE-AI-8). Every generative output of this surface renders the AI-generated /
AI-assisted disclosure (GATE-AI-9), and generated user-facing text follows the product
copy rules ([[voice-and-copy]]).

## What's configurable

- **The model client** — drafting generates through the workspace's customer-supplied
  inference (ADR-0020); with no model configured, the draft surface is unavailable and the
  CRM runs on without it.
- **The voice profile** — a per-rep learned register consumed when present
  ([[voice-profile]]); absent or not yet ready, drafting falls back to the baseline voice.
- **The asset provider** — with a curated library present, drafts cite approved assets;
  with none, drafting behaves exactly as the baseline (no asset context, no citations).
- **Intent steering** — a draft request may carry a stated intent that steers generation.
- **The AI budget guardrail** — the per-workspace spend boundary drafting degrades under;
  a runtime surface owned by the runtime-config boundary ([[runtime-config]]).

## Guarantees (enforced)

- **Drafting never sends.** Producing a draft creates zero outbound mail and zero outbound
  activity rows; there is no sent state until a human confirms. Held by the draft
  operation returning the draft rather than acting, and pinned by the never-auto-send
  test (AC6.2).
- **The send gate cannot be bypassed.** An agent caller without a valid approval token is
  refused before any side effect, whatever the draft's voice, citations, or confidence
  (GATE-AI-7; token semantics at [[approvals-and-concurrency]]).
- **Sends are consent-gated, audited, and evented.** A send without an active granted
  consent for its purpose is suppressed with zero transport; a successful send writes
  exactly one audit row and one outbound activity ([[event-bus#EVT-SEM-1]]).
- **Grounded or absent.** Drafts carry the identifiers of the captured records they derive
  from; summary points cite resolvable sources or the summary is honestly empty
  (GATE-AI-1, AC6.6).
- **Only approved, in-date content is citable.** The drafting seam reads only the
  available set, so an unapproved, expired, or retired asset is structurally unreachable
  from a draft — excluded and flagged, not filtered at the last minute (DRAFT-AC-1).
- **The asset store stays a store.** No content-authoring, rendering, or campaign surface
  exists; a static check asserts the store is retrieval-only (DRAFT-AC-4).
- **No LinkedIn automation ships.** A static guard asserts there is no auto-post,
  auto-connect, or auto-message code path; the only exit for a LinkedIn draft is the rep's
  own copy-and-send (DRAFT-AC-6).
- **Disclosure is mandatory.** Every generative draft renders the Art. 50 AI-assisted
  disclosure; absence is a hard failure (GATE-AI-9).
- **Tenant isolation.** Drafts, summaries, and assets are workspace-scoped and RBAC-bound;
  curation is a distinct gated permission (DRAFT-AC-3).

## Acceptance

Done means: from a captured thread a rep gets an editable, evidenced draft in the composer
in the rep's register when a voice profile exists, with any claim traced to a visible
approved asset — and can verify nothing was sent by looking. Sending is a distinct,
deliberate act; an agent's proposed send waits in the approval inbox. The asset library
renders honestly for both roles: the rep sees read-only sanctioned material with stale
items visibly withheld; the curator sees the curation controls the rep never gets. The
LinkedIn surface offers copy-only output and an explicit way to log a manually sent
message. Empty, loading, error, and no-permission states inherit the cross-cutting floor
from [[acceptance-standards]] (STATE-1..5) and are not restated; the AI release gates
(GATE-AI-1..10) apply to this surface as pinned there. The testable form of every claim
lives in the Acceptance appendix.

## Out of scope

- **Natural-language search** — the other half of the baseline-AI comms feature; owned by
  the retrieval chapter ([[retrieval-seam]]).
- **The voice profile itself** — corpus, speaker filtering, rebuild, rollback, and the
  learning loop: [[voice-profile]].
- **Approval staging, tokens, TTLs, and the inbox** — [[approvals-and-concurrency]].
- **The browser-extension host and LinkedIn profile→lead capture** (S-E12.2) — the
  client-surfaces chapter.
- **Email templates, sequences, and deliverability transport** — the sequences and
  deliverability chapter; this chapter drafts content, it does not schedule cadences.
- **Overnight recovery drafting and deal coaching** — those moments call this draft path
  but are owned by their own chapters.
- **Relevance ranking across many assets and per-vertical asset packs** — table-stakes
  deferred by the feature cut line; **any CMS surface** — explicitly out, permanently.

## Where it lives

The backend module for drafting (`modules/drafting`), reached through the contract's
draft and send operations under an activity (ceded to this chapter at the timeline
chapter's ACT-WIRE-N-1 note) and the governed draft/send tool verbs agents use; the web
surfaces are the composer draft flow, the asset library, and the LinkedIn draft screen.
Read next: [[voice-profile]] (the register drafts are written in),
[[approvals-and-concurrency]] (the 🟡 send), [[activities-and-timeline]] (where sent mail
lands), and [[retrieval-seam]] (context assembly and NL search).

## Appendix

### Parameters
Source: specs/spec/product/30-screen-acceptance.md#linkedin-drafthtml--linkedin-message-in-my-voice-implements-s-e074 @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| DRAFT-PARAM-1 | LinkedIn connection-note limit | `300` chars | Character budget for the "Connection note" variant; the counter turns over-limit red past it (AC-linkedin-draft-2/3). Mirrors LinkedIn's own limit; source constant, no runtime tuning. |
| DRAFT-PARAM-2 | LinkedIn InMail limit | `1900` chars | Character budget for the "InMail / message" variant (AC-linkedin-draft-2). Source constant. |

The summary first-token, NL-search, and core-op latency budgets are release-gate floors
pinned by [[acceptance-standards]] and the feature ACs below (AC6.1), not tunables of this
chapter. The per-workspace AI budget guardrail is a runtime-config surface
([[runtime-config]]) — cited, not owned here.

### Schema
Source: specs/spec/contract/data-model.md#voice--drafting-e07--features07-features09 @ 5a0b29c

Ownership verified against the data-model chapter's ownership index: `drafting_asset` is
assigned to this chapter ([[data-model]] Schema — ownership index, row `drafting_asset`);
`voice_profile` and `voice_corpus_source` are [[voice-profile]]'s.

**DRAFT-DDL-1 — the `drafting_asset` table (verbatim).**

```sql
CREATE TABLE drafting_asset (                             -- governed reusable claim/snippet store (NOT email templates, §M4)
  -- + base columns + version
  kind          text NOT NULL CHECK (kind IN ('claim','case_study','boilerplate','objection_response')),
  title         text NOT NULL,
  body          text NOT NULL,
  approved      boolean NOT NULL DEFAULT false,            -- only approved assets are agent-usable
  approved_by   uuid NULL REFERENCES app_user(id)
);
```

Note DRAFT-DDL-N-1 (reconcile at ticket time): the feature spec's MVP cut line
(features/07 §7b) requires an **owner**, an **approval state with a distinct
expired/retired value**, and an **optional expiry** on each asset, and the asset-library
screen renders freshness states (Current / Re-review / Stale / Retired) and per-asset
versions — the corpus DDL above carries only a boolean `approved` + `approved_by`. The
gap (expiry timestamp, state enum vs boolean, owner column, review date) must be resolved
in the contract when the build ticket lands; the boolean shape as pinned cannot express
DRAFT-AC-2 or AC-asset-library-1. Adjacent fact cited, not owned: an offer's boilerplate
terms may reference a `drafting_asset` — pinned by [[offers-and-products]] (its Schema,
A42 note).

### Wire
Source: specs/spec/contract/crm.yaml (paths `/activities/{id}/draft-email`, `/activities/{id}/send-email`) @ 5a0b29c

Operations are cited by contract `operationId` — request/response shapes live in the
contract, never restated here. Both are routed under an activity on the wire but are this
chapter's operations ([[activities-and-timeline]] note ACT-WIRE-N-1).

| ID | operationId | Operation | Tier | Errors / headers of note |
|---|---|---|---|---|
| DRAFT-WIRE-1 | `draftEmail` | Draft a reply/follow-up for a captured activity's context; optional `intent` steering; returns the draft (never sends) with its grounding-source identifiers | 🟢 `draft_email` | 404 |
| DRAFT-WIRE-2 | `sendEmail` | Send a (possibly edited) draft — outbound + irreversible | 🟡 `send_email` confirm-first | `Idempotency-Key`, `X-Approval-Token` (agent callers — [[approvals-and-concurrency]] APPR-WIRE-1); **403** approval token missing / RBAC denied; **409** `consent_not_granted` (default-deny per purpose, ADR-0011); 422; 202 on accept (queued, resulting activity logged) |

Note DRAFT-WIRE-N-1 (contract gap, honest): the asset library has **no operations in the
contract at the pinned corpus version**. The data-model contract-surface note declares
`drafting_asset` gets a first-class REST+MCP surface, and the feature spec requires admin
approve/expire actions and a rep-visible retrieval set — but no `operationId`s exist in
`crm.yaml` @ 5a0b29c. The asset CRUD + approve/expire + available-only-filter surface must
be added to the contract at ticket time; this chapter owns those operations when they land.
Summaries likewise have no dedicated operation pinned (the feature promises on-demand
thread/account/deal summaries) — same ticket-time contract work.

### Events
Source: specs/spec/contract/events.md#55-activity @ 5a0b29c

Event definitions live in the central catalog ([[event-bus]]) — cited here, not redefined.
The catalog defines **no drafting-specific events** @ 5a0b29c: drafting itself is 🟢 and
read-shaped (a draft is returned, not persisted as a domain mutation), and a completed
send materializes on the existing activity and approval streams.

| ID | Event | Cite |
|---|---|---|
| `activity.captured` | The sent email lands as an outbound email activity on the timeline (emitted instead of a generic update) | [[event-bus]] catalog row `activity.captured`; [[event-bus#EVT-SEM-2]] |
| `approval.requested` / `approval.decided` | An agent-proposed send stages a 🟡 item and executes on approval | [[event-bus]] catalog rows; semantics owned by [[approvals-and-concurrency]] |
| `audit.appended` | The send's single audit row on the audit stream | [[event-bus]] catalog row `audit.appended` |

Note DRAFT-EVT-N-1 (reconcile at ticket time): once asset mutations gain their wire
surface (DRAFT-WIRE-N-1), the one-mutation-one-audit-one-event rule
([[event-bus#EVT-SEM-1]]) requires asset domain events (create/approve/expire) that the
catalog does not yet define — to be added with the contract work, catalog-first.

### Acceptance
Source: specs/spec/product/epics/E07-voice-and-drafting.md @ 5a0b29c; specs/spec/product/20-traceability.md @ 5a0b29c

**Owned stories** (primacy verified against the traceability register; the epic-to-chapter
split in [[scope]] assigns E07 to this chapter plus [[voice-profile]]):

| ID | Story | Tier | Home |
|---|---|---|---|
| S-E07.1 | Baseline AI drafts the reply, in the composer, never behind my back | V1-Must | this chapter |
| S-E07.2 | Voice DNA: the draft sounds like *me* | V1-WOW | **[[voice-profile]]** — cited, not owned here |
| S-E07.3 | A governed library of approved things to say | V1-WOW | this chapter |
| S-E07.4 | Draft a LinkedIn message in my voice | V1-WOW | this chapter (draft mechanics; the extension host surface is the client-surfaces chapter's) |

**Feature acceptance criteria (verbatim from the feature spec).**
Source: specs/spec/features/02-capture-and-comms.md#feature-6--baseline-ai-comms-summaries-draft-replies-nl-search @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC6.1 | Summary first-token **< 1.5 s p95**, rendered async without blocking the record view (§3.5). *(perf test)* | Performance gate (CI against seeded dataset) |
| AC6.2 | A draft reply is never sent without the confirm-first gate (shared with AC4.2). *(test)* | Backend integration lane (never-auto-send test; gate semantics at [[approvals-and-concurrency]]) |
| AC6.5 | When the per-workspace AI budget guardrail is hit, baseline AI features degrade gracefully (disabled/queued) and **core CRM operations are unaffected** (record open/list/save budgets still met). *(test: simulate budget exhaustion → assert core ops green)* | Backend integration lane (budget-exhaustion fixture) |
| AC6.6 | Summaries cite the captured records they're derived from (provenance, P12) — clickable back to source activities. *(test: summary payload includes source activity ids)* | Backend integration lane |

AC6.3 and AC6.4 (NL-search plan compilation eval + full-text latency budget) are the
NL-search half of the feature, owned by the retrieval chapter ([[retrieval-seam]]) —
cited, not pinned here. The model-bound draft quality bands ride the eval catalog:
usefulness ≥ 80% and hallucinated-fact ≤ 2% ([[ai-evals#AIEVAL-12]], [[ai-evals#AIEVAL-13]];
use cases [[ai-evals]] AIUC-21 draft-reply, AIUC-13 voice, AIUC-14 assets), summary
factuality ≥ 95% / citation validity ≥ 98% ([[ai-evals#AIEVAL-10]], [[ai-evals#AIEVAL-11]];
AIUC-22) — owned there, cited here.

**Governed asset library acceptance (verbatim from the feature spec; new IDs — the source
bullets are unnumbered).**
Source: specs/spec/features/07-ai-native-moments.md#7b-governed-proof-point--asset-library-for-drafting @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| DRAFT-AC-1 | Given a draft that makes a factual/claim-bearing statement, the agent **cites which approved asset** it drew from (asset id surfaced on the draft); a draft that uses an **unapproved or expired** asset is a hard failure. *(deterministic test: drafts reference only approved, in-date assets; expired-asset fixture → excluded)* | Backend integration lane (deterministic; AIUC-14) |
| DRAFT-AC-2 | An admin can approve / expire an asset; an expired asset stops appearing in new drafts and is flagged for review. *(test)* | Backend integration lane |
| DRAFT-AC-3 | The store is RBAC-bound and workspace-scoped (no cross-workspace asset leakage). *(test)* | Backend integration lane (RLS/RBAC test) |
| DRAFT-AC-4 | Scope guard: there is **no** content-authoring/rendering or campaign surface in V1 — a static check asserts the asset store is retrieval-only. *(static check)* | Static negative-scope guard, CI |

**LinkedIn draft-only channel acceptance (verbatim from the feature spec; new IDs — the
source bullets are unnumbered).**
Source: specs/spec/features/08-client-surfaces.md#2b-linkedin-draft-only-channel @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| DRAFT-AC-5 | Given a CRM record, the rep gets a LinkedIn message **drafted in their voice** (inherits §7 Voice DNA) referencing real context; the draft cites any approved asset it used (inherits §7b). *(test: draft uses voice profile + only approved assets)* | Backend integration lane |
| DRAFT-AC-6 | **No automated LinkedIn action ships** — a static check asserts there is no auto-post/auto-connect/auto-message code path; the rep sends manually. *(static guard test)* | Static negative-scope guard, CI |
| DRAFT-AC-7 | The draft is logged against the record with provenance (`captured_by=…+human:<id>`); **nothing is sent by Margince** (the platform send is the human's act on LinkedIn). *(test: 0 outbound from Margince)* | Backend integration lane |
| DRAFT-AC-8 | Output renders the Art. 50 AI-assisted disclosure where shown in-product. | Backend + screen lane (GATE-AI-9) |

**Asset-library screen acceptance criteria (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#asset-libraryhtml--approved-asset-library-implements-s-e073 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-asset-library-1 | Given the library loads, When the rep views the list, Then six approved assets render, each showing its kind badge (Claim / Case study / One-pager), a freshness pill (Current / Re-review in N days / Stale / Retired), the verbatim approved statement or file/link, and a metaline naming the approver (Mor Hadad), review date, and "Used in N drafts" count. | Screen e2e lane |
| AC-asset-library-2 | Given the filter bar with chips All (6) / Claims (2) / Case studies (2) / One-pagers (2) / Needs review (2), When the rep clicks a chip, Then only matching assets stay visible (kind match, or `data-state==='stale'` for Needs review) and the chip shows active styling. | Screen e2e lane |
| AC-asset-library-3 | Given a filter that matches nothing, When the resulting list is empty, Then an honest empty-state card appears reading "Nothing approved in this category yet" explaining reps see only sanctioned material rather than a guess. | Screen e2e lane |
| AC-asset-library-4 | Given the role toggle defaults to "View as Sam", When the rep is in Sam mode, Then no curation controls (Edit / Retire / Approve v4 / Add asset) are shown and the governance banner states "Mor curates what counts as approved … you can't add or retire claims yourself." | Screen e2e lane |
| AC-asset-library-5 | Given the rep switches to "View as Mor (admin)", When Mor mode is active, Then curation controls become visible (Edit, Retire, Approve v4 → replace, Add an approved asset) and the governance banner rewrites to "You curate what counts as approved." | Screen e2e lane |
| AC-asset-library-6 | Given the stale Pharma QA one-pager (v3), When any user views it, Then a staleband states the agent will NOT attach this superseded version — it surfaces the flag instead — and the Sam-only note reads "Mor needs to refresh this — it won't appear in your drafts meanwhile." | Screen e2e lane |
| AC-asset-library-7 | Given Mor clicks "Approve v4 → replace" on the stale one-pager, When the action runs, Then the card loses its stale styling, the freshness pill becomes "Current · v4", the staleband is removed, the file updates to v4, and a toast confirms "v4 approved — now the asset the agent will attach"; the same actions invoked as Sam are rejected with "Only a curator can approve/retire". | Screen e2e lane |
| AC-asset-library-8 | Given the right-rail draft preview to Dr. Bär, When the rep clicks "See where it was used" on the GxP claim or Brandt case asset, Then the view scrolls to the draft and the corresponding inline citation chip is briefly outlined; clicking a citation chip in the draft scrolls back to and highlights its approved source asset. | Screen e2e lane |
| AC-asset-library-9 | Given the right-rail "Nothing is sent" panel, When the rep reviews the draft built from approved assets, Then it states citing approved material does not send anything — the draft leaves only on an explicit send (🟡 confirm gate inherited from S-E07.1). | Screen e2e lane |

**LinkedIn-draft screen acceptance criteria (verbatim; corpus IDs preserved).**
Source: specs/spec/product/30-screen-acceptance.md#linkedin-drafthtml--linkedin-message-in-my-voice-implements-s-e074 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-linkedin-draft-1 | Given the screen loads tied to Anna Weber / Brandt Automotive / BÄR Pharma — Packaging QA, When it renders, Then the composer shows a "Connection note" draft in the user's voice ending "Beste Grüße, Sam", a "Draft only" status pill, and a Voice DNA bar attributing it to 214 kept messages and 37 rejected drafts. | Screen e2e lane |
| AC-linkedin-draft-2 | Given the "Connection note" variant is active, When the user clicks "InMail / message", Then a streaming skeleton shows briefly and the composer fills with the longer InMail draft, the kind label updates to "InMail / message", and the character counter limit switches from 300 to 1900. | Screen e2e lane (DRAFT-PARAM-1/2) |
| AC-linkedin-draft-3 | Given a draft is shown, When the user types into the textarea, Then the provenance chip changes from "drafted · voice-dna" to "typed by you", and the character counter updates live, turning red (.over) once the text exceeds the variant's limit. | Screen e2e lane |
| AC-linkedin-draft-4 | Given a draft is shown, When the user clicks "Regenerate" or "Try another", Then the skeleton streams and the draft re-renders, resetting the provenance chip back to "drafted · voice-dna". | Screen e2e lane |
| AC-linkedin-draft-5 | Given the draft is ready, When the user clicks "Copy text", Then the text is copied to the clipboard, a toast confirms "Copied — now paste it into LinkedIn yourself", and the previously-disabled "I sent this on LinkedIn" log button becomes enabled. | Screen e2e lane |
| AC-linkedin-draft-6 | Given the user has copied and manually sent the message, When they click "I sent this on LinkedIn", Then the log block is replaced with a confirmation that a "LinkedIn message (sent by you)" activity is now on Anna Weber's timeline (dated 24 Jun) with a "View on timeline" link, carrying provenance "voice-dna draft, sent by you" (or "human · typed by you" if the draft was edited). | Screen e2e lane |
| AC-linkedin-draft-7 | Given the screen is open, When the user looks for a send control, Then there is none — the footer states "Copy-only — Margince never opens LinkedIn", and a dedicated panel explains LinkedIn ToS forbids automation so the surface is draft-only by design. | Screen e2e lane |
| AC-linkedin-draft-8 | Given the screen shows what the draft is built from, When the user reads the approved-material section, Then the approved+current "GxP QA case study" asset is shown as grounded with its source (asset:case-study/gxp-qa-2026, curated by Mor), while the stale "40% fewer QA escapes" claim is visibly held back (expired 02 Jun) and was not inserted. | Screen e2e lane |

The standard screen-state matrix (empty / loading / error / no-permission /
nothing-grounded) is inherited from [[acceptance-standards]] (STATE-1..5) and applies to
both screens without restatement; the prototype's known state gaps (asset library: loading
and error statecards styled but never invoked; LinkedIn draft: no clipboard-failure or
generation-failure state) are the build ticket's to close under that inherited floor. The
AI release gates GATE-AI-1/2/7/8/9 apply as pinned in [[acceptance-standards]].
