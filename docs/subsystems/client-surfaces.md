---
status: planned
module: extension (browser-extension thin client — new top-level client of the governed surface) · frontend/src/features/public-form (hosted inbound form); no owned backend module — the server side is consumed, and the form-submission handler's module home is an open build decision (CS-GAP-2)
derives-from:
  - specs/spec/features/08-client-surfaces.md#1-inbox-sidebar-gmail--outlook @ 5a0b29c
  - specs/spec/features/08-client-surfaces.md#2-linkedin--social-capture-one-click--lead @ 5a0b29c
  - specs/spec/features/08-client-surfaces.md#4-extension-architecture--trust @ 5a0b29c
  - specs/spec/features/08-client-surfaces.md#5-in-assistant-app-surface-mcp-apps-in-chatgpt--claude @ 5a0b29c
  - specs/spec/features/08-client-surfaces.md#6-cross-cutting-acceptance-gates @ 5a0b29c
  - specs/spec/features/08-client-surfaces.md#8-open-questions--07-risksmd @ 5a0b29c
  - specs/spec/product/epics/E12-client-surfaces.md @ 5a0b29c
  - specs/spec/product/build-backlog/E12.md @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#client-surfaceshtml--browser-extension-gmail-sidebar--linkedin-capture-implements-s-e121235 @ 5a0b29c
  - specs/spec/product/30-screen-acceptance.md#public-formhtml--public-inbound-lead-form-implements-s-e126 @ 5a0b29c
  - specs/spec/contract/events.md#54-lead @ 5a0b29c
  - specs/spec/contract/data-model.md#34-consent--retention @ 5a0b29c
---
# Client surfaces — the CRM where the rep already is, and it talks only to their own workspace

> The daily-driver surfaces outside the web app: the in-inbox sidebar (Gmail and
> Outlook at parity), one-click LinkedIn profile→lead capture, the in-assistant
> component surface, and the minimal public inbound form. One promise binds them
> all: every surface is a thin client of the rep's own workspace — it egresses
> nowhere else, obeys the rep's permissions exactly, writes leads not contacts,
> and is never a back door around the send gate.

## What it's for

Reps live in the inbox and on LinkedIn, and they resent tab-switching "to go update
the CRM." Server-side capture already wins the mailbox and calendar — this subsystem
does the three things the server structurally cannot: it shows the CRM's full picture
where the rep already works, it lets the rep act from that surface (associate, task,
draft, send under the gate), and it captures the one source no server connector can
see — the profile in front of the rep's eyes — as a lead, never a contact. Two more
surfaces complete the set: interactive components rendered inside the user's AI
assistant, and a minimal hosted contact form that gives inbound prospects a front
door on the operator's own infrastructure.

Its callers are the rep in Gmail or Outlook, the rep prospecting on LinkedIn, the
assistant user asking from a chat client, and the anonymous visitor submitting the
form. The scope boundary is sharp: this is client-side context and action, **not a
second capture engine** — mailbox sync, auto-create, and enrichment belong to
[[capture]]; the lead object and its promotion belong to
[[leads-and-qualification]]; and the buyer-facing deal room, although a client
surface in the corpus, is its own chapter ([[deal-rooms]]).

## Principles it serves

- **P7 — own your data.** The headline principle: every surface here authenticates
  to the user's own workspace and sends data to no third party, no vendor relay, no
  analytics sink. The own-workspace-only egress invariant is the release-blocking
  trust gate ([[acceptance-standards#GATE-CS-1]]).
- **P5 — capture-first, zero re-typing.** One-click capture of the viewed profile,
  clip-not-retype for inbound, and a sidebar that associates the already-captured
  activity rather than asking the rep to re-log it.
- **P12 — provenance, permission, audit.** Every row carries surface-plus-human
  provenance; every read obeys the human's RBAC on the same server path as the web
  app; every mutation is one audit row and one event; every outbound act is
  confirm-first ([[acceptance-standards#GATE-CS-4]]–[[acceptance-standards#GATE-CS-6]]).
- **ADR-0008 — lead, not contact.** Machine-viewed profiles and public form fills
  are prospects with no genuine interaction yet; they land segregated and promote
  only on genuine engagement ([[acceptance-standards#GATE-CS-2]]).
- **ADR-0013 — one governed surface.** The extension and the form handler are
  first-party clients of the same public contract as the web app and the typed tool
  calls; no privileged endpoint, no client-side business logic.
- **ADR-0026 — per-tool autonomy tiers.** Drafting and reading are free; sending and
  enrolment-that-sends are confirm-first, at the same tier the web app enforces.

## How it works

**The trust substrate comes first — everything rides it (S-E12.5).** The extension
authenticates via the Agent Seat Passport / OAuth handshake to a workspace endpoint
the user configures — cloud, self-hosted, or on-prem — with no hard-coded vendor
host to fall back to. It carries no business logic the server doesn't: every read
and write is a governed call under the human's RBAC, and where the sidebar invokes a
BYO agent, the agent's effective permission is the intersection of the human's RBAC
and the Passport scope — never anything extra ([[byo-agent-and-mcp]]). Every write
is stamped with the capturing surface and the human it acted for, and produces
exactly one audit row and one domain event, identical to the same action taken in
the web app. Revoking the human's access or the Passport invalidates the extension
session within one event-bus cycle. The egress posture is testable by construction:
a network-isolation test over every flow asserts the only destination is the user's
configured workspace, and any page content bound for a frontier model downstream
passes the secret-stripper on egress — under the sovereign zero-egress profile,
nothing external leaves at all.

**The inbox sidebar is context plus action, not capture (S-E12.1).** On an open
email from a known contact, the panel shows who they are, their company, open deals,
the inferred next step, and recent activity — a read-only 360 within the pinned
perceived budget (CS-PARAM-1), without leaving the inbox. It ships on the Gmail host
and the Outlook / M365 add-in host at parity (the A51 Microsoft-first decision); the
parity gate on the M365 host is carried as an open build decision (CS-OPEN-3). An
unknown sender gets an honest "not in your CRM" state offering capture as a lead —
merely opening a mail creates nothing. Acting from the panel reuses what the server
already captured: associate-to-deal relinks the existing captured activity to the
chosen deal — never a re-log, never a duplicate — and create-task writes through the
same operation as the web app, attributed to the human.

**Drafting and sending from the sidebar split cleanly (S-E12.3 — the surface half).**
Insert-draft places a reply personalized from the contact's captured history into
the compose box as a free action; nothing sends until the rep explicitly sends, and
that send passes the same confirm-first gate as everywhere else — an edited draft
sends the rep's edit, not the original. The draft mechanics — voice, grounding,
approved assets — are the [[drafting]] chapter's; this chapter owns only the host
surface that requests and places the draft. Start-sequence is split the same way:
the button, the panel flow, and the honest states are this chapter's; enrolment
mechanics, suppression, and exit-on-reply are [[sequences-and-deliverability]]'s.
The sequence-enrol operation is a pinned contract gap in that chapter, and the
backlog sequences the sidebar's enrol wiring behind the sequence engine — carried
honestly as CS-OPEN-6.

**LinkedIn capture writes exactly one deduped lead (S-E12.2).** Saving the profile
the rep is viewing captures name, title, company, and profile link in one click —
and it lands as a lead, excluded by default from contact lists, search,
person-dedupe, and relationship strength. The dedupe check runs before the write,
against existing leads and people: an exact match returns "already in your CRM" with
a link to the existing record and creates zero rows; an ambiguous near-match
surfaces a confirm-first candidate, never a silent merge (the matching machinery is
[[people-and-organizations#PO-F-1]]; only exact unique-key matches ever auto-merge,
[[people-and-organizations#PO-AC-19]]). The lead's candidate organization is
referenced for routing and scoring without creating a real org-graph node before
promotion. A profile the extension cannot read degrades to an honest
"couldn't read enough" with manual fill — zero fabricated fields. The hard platform
line: capture is a human-initiated act on the profile the human is looking at.
No automated connection requests, no automated messaging, no background or bulk
scraping ships — held by a static guard, and the outbound LinkedIn *drafting*
channel is deliberately draft-only, owned by [[drafting]] (the S-E07.4 split: they
own the draft mechanics, this chapter owns the extension host). Promotion on
genuine engagement rides the shared promotion path owned by
[[leads-and-qualification]]; only the extension-native conversation-sync trigger is
fast-follow, gated on the ToS and consent review (CS-OPEN-1).

**The in-assistant surface renders governed tool output, nothing else.** Inside
ChatGPT or Claude, intent-tool results render as interactive components — a deal
card, a slipping list, an approval card, a draft-review card — riding the tool pins
owned by [[byo-agent-and-mcp]] (BYO-INTENT-1..3 for the launch set). The component
introduces no new data path and no new authority: what it shows is exactly the
governed, field-masked tool result for that principal, and every in-component
action routes through the same tool, Passport scope-intersection, tier, and audit
as a typed call. The confirm-first button is precisely the act of minting the
single-use approval token ([[approvals-and-concurrency#APPR-WIRE-1]]) — a gated
action with no token does nothing and is logged. A client that cannot render
components gets equivalent text with no loss of the action path. This surface
carries no story ID of its own in the epic — it is the client-surface face of the
agent-first direction, pinned here because the corpus files it in this feature area
(CS-OPEN-7 notes the sourcing).

**The public inbound form is the front door, not a form builder (S-E12.6).** A
minimal hosted contact form — a bounded, fixed set of standard fields — runs on the
operator's own infrastructure, styled to the brand. A submission creates exactly one
lead, deduped against existing leads and people (exact match links, ambiguity goes
to a confirm-first candidate), with every field carrying form provenance and the
message kept verbatim. Consent is captured per purpose — contact required, marketing
separately optional — with the exact wording and timestamp stored as withdrawable
proof on the substrate owned by [[gdpr-platform]]. The public endpoint is
abuse-guarded: a server-side rate limit plus a honeypot; a bot that fills the
honeypot or exceeds the limit receives the same success confirmation while no lead
is created — a silent fake-success drop. A form lead is segregated like any external
source and promotes only on genuine engagement; a raw form fill never pollutes
contacts. A structurally new form is a source-level customization (ADR-0002), never
a drag-and-drop builder. The corpus specifies this capability in the epic story —
the feature spec's promised section for it never landed, so the epic is this
chapter's source of record for the form (CS-OPEN-7).

## What's configurable

- **The workspace endpoint** — where the extension points; a required, user-set
  configuration value with no default host (CS-AC-15). This is what makes
  self-hosted and on-prem workspaces first-class; how endpoint discovery works
  without weakening the no-third-party-host invariant is open (CS-OPEN-5).
- **Per-surface enable/disable** — the admin can allow or deny the sidebar and the
  LinkedIn capture surface per workspace; the toggle surface lives on the settings
  screen owned by the access-and-admin chapter.
- **The form's field set and branding** — bounded standard fields plus a small
  picklist, styled to the brand; anything structurally new is a source-level theme
  (ADR-0002), not runtime configuration.
- **The browser target** — Chrome MV3 first; other browsers fast-follow. The MV3
  service-worker constraints must be confirmed against the egress-invariant test
  (CS-OPEN-2).

## Guarantees (enforced)

- **Own-workspace-only egress.** For every extension flow, the only network
  destination is the user's configured workspace; zero third-party, relay,
  analytics, or telemetry hosts — asserted by a deterministic network-isolation
  test, and a regression blocks release ([[acceptance-standards#GATE-CS-1]]).
- **Capture creates leads, never persons.** Profile and page capture, the sidebar's
  unknown-sender path, and the public form all write leads; captured leads stay out
  of contact surfaces until a genuine-engagement promotion
  ([[acceptance-standards#GATE-CS-2]]; promotion owned by [[leads-and-qualification]]).
- **Dedupe-first.** Every capture surface checks existing leads and people before
  writing; exact match returns the existing record, ambiguity becomes a
  confirm-first candidate — never a silent duplicate or auto-merge
  ([[acceptance-standards#GATE-CS-3]]).
- **Provenance universality.** Every row written by a client surface carries
  surface-plus-human provenance and the source reference — queryable as
  extension-sourced, attributable to the human ([[acceptance-standards#GATE-CS-4]]).
- **RBAC and audit parity with the web app.** A record the human cannot see is never
  returned to a client surface; every mutation is exactly one audit row plus one
  event, identical to the web path; agent invocation never exceeds the
  scope-intersection ([[acceptance-standards#GATE-CS-5]]).
- **No back-door sends.** No outbound action executes from any client surface
  without the same recorded approval token the web app requires
  ([[acceptance-standards#GATE-CS-6]]; token semantics at
  [[approvals-and-concurrency]]).
- **No-guess on read content.** Any field shown from a read profile carries evidence
  or is omitted; a fabricated field is a hard failure
  ([[acceptance-standards#GATE-CS-7]]).
- **No LinkedIn automation ships.** A static guard asserts there is no automated
  connect, message, or background-harvest code path anywhere; capture is
  human-viewed-profile only, and outbound LinkedIn content is draft-only (the guard
  on the draft channel is [[drafting]]'s DRAFT-AC-6; the capture-side boundary is
  pinned here, CS-AC-14).
- **The in-assistant component is a view, not an authority.** Rendered data equals
  the governed tool result for that principal, field-masking included; a gated
  action without a token does nothing and is logged (CS-AC-24, CS-AC-25).
- **The public form fails safe.** Honeypot and over-limit submissions create nothing
  while showing fake success; accepted submissions capture per-purpose consent proof
  and are attributable in the audit log (CS-AC-30, CS-AC-32).

## Acceptance

Done means a rep reads an email and has the contact's deals, next step, and history
beside it within the perceived budget, acts from the panel under their own
permissions, and can watch the extension's traffic and see it speak only to their
workspace. It means one click on a LinkedIn profile yields exactly one deduped lead
that stays out of their contacts until the prospect actually engages, and an honest
"already in your CRM" instead of a duplicate. It means an assistant user taps a real
component that shows exactly what the tool returned and approves a send by minting
the token — or gets equivalent text on a client without components. And it means a
visitor submits the form and lands as a staged, consent-proofed, deduped lead on the
operator's own infrastructure, while a bot gets fake success and nothing is written.

The honest states are load-bearing: the unknown-sender "no match" state, the
"couldn't read enough" degradation with zero invented fields, the signed-out and
unreachable-workspace states, the form's inline validation and fake-success drop.
The cross-cutting screen-state floor and performance budgets are inherited from
[[acceptance-standards]] (STATE-1..5, PERF pins) and not restated; the testable form
of every claim lives in the Acceptance appendix.

## Out of scope

- **The digital deal room** — a client surface in the corpus, but its own chapter:
  [[deal-rooms]].
- **The web clipper (S-E12.4)** — deferred fast-follow; the single home of that
  deferral is the [[scope]] OUT register. Its mechanics reuse the scrape/enrichment
  connector seam and the Impressum reader (ADR-0006); nothing of it is pinned here.
- **LinkedIn draft mechanics (S-E07.4)** — voice-styled generation, approved-asset
  grounding, copy-only handoff: [[drafting]]. This chapter owns only the extension
  host surface.
- **Sequence mechanics** — enrolment, suppression, exit-on-reply, the send engine:
  [[sequences-and-deliverability]]. This chapter owns the inbox surface for
  S-E12.3.
- **Server-side capture** — mailbox/calendar sync, auto-create, enrichment:
  [[capture]]. The sidebar associates what capture already wrote.
- **The lead object, segregation, and promotion** — [[leads-and-qualification]].
- **Dedupe internals** — [[people-and-organizations]].
- **Consent substrate and erasure** — [[gdpr-platform]].
- **Approval staging, tokens, TTLs** — [[approvals-and-concurrency]].
- **The public booking page** — shares the hosted public-surface and abuse-guard
  pattern but belongs to [[meetings-and-transcripts]].
- **Agent runtime and tool registry** — the extension and the assistant surface are
  clients of [[byo-agent-and-mcp]]; no in-extension agent runtime exists.

## Where it lives

A new top-level extension client (the browser-extension host surfaces) and the
hosted public-form feature in the web frontend; both reach the server exclusively
through the one governed public surface — this chapter owns no backend module and
no tables (see the Schema appendix). Read next: [[leads-and-qualification]] (what
capture writes), [[drafting]] and [[sequences-and-deliverability]] (what the
sidebar's actions ride), [[byo-agent-and-mcp]] (the tool surface the assistant
components present), [[deal-rooms]] (the other buyer-facing client surface).

## Appendix

### Parameters
Source: specs/spec/features/08-client-surfaces.md#1-inbox-sidebar-gmail--outlook @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#public-formhtml--public-inbound-lead-form-implements-s-e126 @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| CS-PARAM-1 | Sidebar perceived render budget | p95 < 500 ms | Open email from a known person → 360 panel rendered, perceived, without leaving the inbox |
| CS-PARAM-2 | Sidebar 360 server read budget | p95 < 150 ms | The server half of CS-PARAM-1 — the corpus 360 read budget the panel consumes (features/01 §1.1) |
| CS-PARAM-3 | Public-form honeypot field name | `company_url` | Visually hidden bot trap on the public form; a filled honeypot triggers the silent fake-success drop (AC-public-form-5) |
| CS-PARAM-4 | Extension browser target | Chrome MV3 first | Firefox/Edge/Safari fast-follow; MV3 service-worker constraints must hold the egress test (CS-OPEN-2) |

### Schema
Source: skeleton/docs/architecture/data-model.md (Schema — ownership index) @ 5a0b29c; specs/spec/product/build-backlog/E12.md#minimal-public-inbound-form--lead-s-e126-v1-must--a51 @ 5a0b29c

This chapter owns **no tables** — reported honestly against the ownership index:

| ID | Gap / fact | Detail |
|---|---|---|
| CS-GAP-1 | No extension-owned tables | Every extension write lands in tables owned elsewhere: `lead` ([[leads-and-qualification]]), `activity`/`activity_link` ([[activities-and-timeline]]), `audit_log` + consent tables (platform, [[data-model]]). The 66-table ownership index assigns nothing to this chapter, and no deferred stub names a client-surfaces table. |
| CS-GAP-2 | No form-submission table | The public form writes exactly one `lead` plus per-purpose consent records (data-model §3.4). The confirmation-screen reference id (AC-public-form-3) has no pinned storage home; whether a submission record exists beyond the lead — and the handler's backend module home — decompose at WP-entry (build B-E12.20). |
| CS-GAP-3 | No extension session/config table | The workspace endpoint is client-side configuration; extension sessions ride the platform `session`/`passport` tables owned by [[auth-and-sessions]]. |

### Wire
Source: specs/spec/contract/crm.yaml @ 5a0b29c; specs/spec/product/build-backlog/E12.md @ 5a0b29c

The extension is a first-party client of the standard contract (ADR-0013) — it
consumes existing operations and adds none. Cited by operationId, never restated:

| ID | Surface action | Contract operation | Tier / note |
|---|---|---|---|
| CS-WIRE-1 | Sidebar 360 read | `listActivities` + standard person/org/deal reads | 🟢 read; RBAC on the same server path as the web app |
| CS-WIRE-2 | Associate-to-deal | `relinkActivity` | 🟢; relinks the already-captured activity — no re-log, idempotent per (activity, entity) |
| CS-WIRE-3 | Create task | `logActivity` (task kind) | 🟢; same create op as the web app, no extension-private path |
| CS-WIRE-4 | Insert AI draft | `draftEmail` | 🟢; drafting is pure — zero sends, zero outbound activity rows |
| CS-WIRE-5 | Send inserted draft | `sendEmail` | 🟡; requires the approval token header ([[approvals-and-concurrency#APPR-WIRE-1]]); unconfirmed → refused, nothing sent |
| CS-WIRE-6 | LinkedIn capture → lead | `createLead` (dedupe-first before write) · `getLead`/`listLeads` for the already-exists state | 🟢 create; promotion is `promoteLead`, owned by [[leads-and-qualification]] |
| CS-WIRE-7 | **Gap — public form submit** | none | No public form-submission operation exists in the contract; the handler is a new build reusing the governed `createLead` substrate under an anonymous-form posture with rate-limit + honeypot in front (B-E12.20). Mirrors the booking-page gap ([[meetings-and-transcripts]] MEET-GAP-1). |
| CS-WIRE-8 | **Gap — start-sequence enrol** | none | The sequence-enrol operation is a deferred contract stub owned by [[sequences-and-deliverability]] (its pinned SEQDEL gaps); the sidebar wires to it when it lands (CS-OPEN-6). |

### Events
Source: specs/spec/contract/events.md#54-lead @ 5a0b29c; specs/spec/contract/events.md#55-activity @ 5a0b29c; specs/spec/contract/events.md#51-person @ 5a0b29c; specs/spec/contract/events.md#56-approval-the--confirm-first-gate-03b-l1 @ 5a0b29c

Client surfaces emit no new event types — every action produces the same events as
the web path (definitions live in the central catalog; cited, not redefined):

| ID | Event | Role here |
|---|---|---|
| CS-EVT-1 | `lead.created` | Emitted on LinkedIn capture and form submission; enters only the segregated lead view, never the contact graph |
| CS-EVT-2 | `lead.promoted` | Consumed context: fired by the shared promotion path when a captured lead genuinely engages — never by this chapter's surfaces directly |
| CS-EVT-3 | `activity.captured` / `activity.updated` | Relink and task actions ride the activity events; associate emits no second capture |
| CS-EVT-4 | `consent.changed` | Per-purpose consent records written by the form submission (proof wording + timestamp) |
| CS-EVT-5 | `approval.*` | The 🟡 send / in-component approval flows stage and resolve through the approval events |

### Acceptance

**Owned stories (primacy verified against the scope register and sibling chapters).**
Source: specs/spec/product/epics/E12-client-surfaces.md @ 5a0b29c; skeleton/docs/product/scope.md (E12 row + OUT register) @ 5a0b29c

| ID | Story | Tier | Home / split |
|---|---|---|---|
| S-E12.1 | See CRM context and act on a contact without leaving my inbox | V1-WOW | this chapter |
| S-E12.2 | One-click capture a LinkedIn profile as a lead, deduped | V1-WOW | this chapter; promotion-on-engagement rides the shared path in [[leads-and-qualification]]; conversation-sync trigger fast-follow (CS-OPEN-1) |
| S-E12.3 | Start a sequence or drop in an AI draft from the inbox sidebar | V1-Must | **split**: this chapter owns the inbox surface; draft mechanics [[drafting]]; sequence mechanics [[sequences-and-deliverability]] (which pins the same split from its side) |
| S-E12.5 | The extension only ever talks to my own workspace and obeys my permissions | V1-Must | this chapter — the substrate every other story rides |
| S-E12.6 | A minimal public form captures inbound as a lead | V1-Must | this chapter; consent substrate [[gdpr-platform]]; lead segregation [[leads-and-qualification]] |
| S-E12.4 | Clip a company from its website | Fast-follow | **OUT** — deferred in the [[scope]] OUT register; not pinned here |

**Release gates.** The eight client-surface release gates are owned by
[[acceptance-standards#GATE-CS-1]]–[[acceptance-standards#GATE-CS-8]] and bind every
surface in this chapter; they are cited throughout and not restated.

**Feature acceptance — inbox sidebar (verbatim from the feature spec).**
Source: specs/spec/features/08-client-surfaces.md#1-inbox-sidebar-gmail--outlook @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| CS-AC-1 | Given an open email from a **known** `person`, the sidebar renders that person's company, **open deals, the inferred next step, and recent activity** within **p95 < 500 ms perceived** (server read **p95 < 150 ms**), without leaving Gmail. | Integration lane ([[testing#TEST-LANE-2]]) on a seeded workspace + matched address; CI perf benchmark (CS-PARAM-1/2) |
| CS-AC-2 | Given an email whose sender is **not** in the CRM, the sidebar shows an honest "no match" state offering **"capture as lead"** (ADR-0008) — it does **not** silently create a `person`; unknown sender → 0 `person` rows created by merely opening the mail. | Integration lane; GATE-CS-2 |
| CS-AC-3 | Given the rep clicks **associate-to-deal**, the email `activity` (already captured) is linked to the chosen `deal` in one action with `source` retained and one `audit_log` row + one event; **no duplicate activity** is created — the sidebar associates the existing captured row, it does not re-log. | Integration lane: relink → existing activity relinked, row count unchanged; GATE-CS-5 audit parity |
| CS-AC-4 | Given the rep clicks **insert AI draft**, a draft personalized from the contact's captured history is placed in the compose box (🟢); **nothing is sent** until the rep explicitly sends, which passes the confirm-first gate. | Integration lane: insert → 0 SMTP calls; contract conformance: send → gate enforced (GATE-CS-6) |
| CS-AC-5 | Given the rep clicks **start sequence**, enrolment that will send is **🟡 confirm-first** and respects the suppression list. | Contract conformance: tool tier 🟡 — wired when the sequence engine lands (CS-WIRE-8, CS-OPEN-6) |
| CS-AC-6 | All sidebar reads/writes go through the user's **own** workspace API under the human's RBAC; a record the human cannot see is **not** rendered in the panel. | Integration lane: row-level RBAC parity extension vs web, same fixtures (GATE-CS-5) |
| CS-AC-7 | **(user-observable)** Opening an email from a known contact, the rep sees that contact's **open deals and next step in the sidebar without leaving Gmail**, and can log/associate it, start a sequence, or drop in a personalized draft from that panel — they do not switch tabs to the CRM (S-E12.1, S-E12.3). | Live-stack lane ([[testing#TEST-LANE-3]]) |

**Feature acceptance — LinkedIn / social capture (verbatim from the feature spec).**
Source: specs/spec/features/08-client-surfaces.md#2-linkedin--social-capture-one-click--lead @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| CS-AC-8 | Given the rep is viewing a LinkedIn profile and clicks **Save**, exactly **one `lead`** is created (not a `person`) with name/title/company/profile-URL and `source=ext:linkedin`, `captured_by=ext:linkedin+human:<id>`, and it is **excluded by default** from contact lists/search/dedupe-against-`person`/relationship-strength (ADR-0008 §2). | Integration lane: save → 1 `lead`, 0 `person`; lead absent from contact list/search (GATE-CS-2/4) |
| CS-AC-9 | Given that person **already exists** (matching LinkedIn URL, or email, or strong name+company on an existing `lead` or `person`), clicking Save **does not duplicate** — the panel says **"already in your CRM"** and links to the existing record. | Deterministic integration test: pre-seed match → 0 new rows, existing id returned (GATE-CS-3) |
| CS-AC-10 | Given an **ambiguous near-match**, the save surfaces a **🟡 merge candidate** the rep confirms or rejects side-by-side — **never** a silent auto-merge into the wrong record. | Integration lane: ambiguous fixture → 0 auto-merges, candidate surfaced ([[people-and-organizations#PO-AC-19]]) |
| CS-AC-11 | Given a captured lead, its candidate `organization` is referenced for routing/scoring **without** creating or polluting a real org-graph node unless/until promotion (ADR-0008 §5). | Integration lane: lead capture → 0 net-new org-graph `organization` rows |
| CS-AC-12 | Given the conversation sync is enabled ([DIFF]) and the prospect **replies inbound**, the lead is **promoted to `person`** via the promotion path (non-lossy, `converted_from_lead_id`, audit-logged) — a cold capture with no reply **stays a lead** (ADR-0008 §3). | Integration lane on the shared promotion path (owned by [[leads-and-qualification]]); extension adapter emits the same inbound-engagement event — fast-follow (CS-OPEN-1) |
| CS-AC-13 | The profile payload the extension reads is sent **only** to the user's own workspace API (P7 egress invariant); a JS-only/blocked profile degrades to an honest "couldn't read enough" with a manual-fill fallback, **zero** fabricated fields. | Network-isolation gate (GATE-CS-1); empty-fixture test → 0 invented fields (GATE-CS-7) |
| CS-AC-14 | **(user-observable)** Clicking **Save** on a LinkedIn profile creates a **lead** (not in the contact list), and **if that person already exists the panel says so instead of duplicating** (S-E12.2). Capture is one-click on the human-viewed profile only — no automated connection-request/InMail/messaging, no unattended/background harvesting ships. | Live-stack lane; static guard scan asserting no automation code path |

**Feature acceptance — extension architecture & trust (verbatim from the feature spec).**
Source: specs/spec/features/08-client-surfaces.md#4-extension-architecture--trust @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| CS-AC-15 | The extension authenticates only via the Agent Seat Passport / OAuth handshake to the **user-configured** workspace endpoint; there is **no** hard-coded Gradion-hosted endpoint it falls back to. | Config test: endpoint is user-set; static scan of the build artifact: no third-party default host |
| CS-AC-16 | **Egress invariant (the load-bearing trust test):** for every extension action, the **only** network destination is the user's configured workspace API — a network-capture test over capture/sidebar/draft flows asserts **zero** requests to any third-party / relay / analytics / telemetry host (destinations ⊆ {user workspace API}). | Deterministic network-isolation test — release-blocking (GATE-CS-1) |
| CS-AC-17 | Every write from the extension carries `source=ext:*` + the source URL/msg-id and `captured_by=ext:<surface>+human:<id>` — queryable as extension-sourced and attributable to the human. | Schema + query test, integration lane (GATE-CS-4) |
| CS-AC-18 | Every extension read/write is enforced under the human's **RBAC** on the same server path as the web app — a record the human cannot see is never returned; a write the human cannot make is rejected with the same error. | Row/field RBAC parity test extension vs web ([[testing#TEST-LANE-2]] RBAC matrix; GATE-CS-5) |
| CS-AC-19 | Where the sidebar invokes a **BYO agent**, its effective permissions are the **intersection** of the human's RBAC and the Passport scope — the extension grants the agent nothing extra. | Scope-intersection test on the extension-invoked agent path ([[byo-agent-and-mcp]]) |
| CS-AC-20 | Every extension mutation produces exactly **one** `audit_log` row + one domain event, identical to the same action in the web app. | Audit-parity test (GATE-CS-5) |
| CS-AC-21 | Revoking the human's access (or the Passport) invalidates the extension session within one event-bus cycle. | Integration lane against [[auth-and-sessions]] revocation |
| CS-AC-22 | **(user-observable)** The admin/developer can verify the extension **only ever talks to their own workspace and obeys their permissions** — watch its traffic and see it never phones home, every action attributed to the human in the audit log, nothing possible the human couldn't do in the web app (S-E12.5). | Live-stack lane + the GATE-CS-1 network capture |

**Feature acceptance — in-assistant app surface (verbatim from the feature spec).**
Source: specs/spec/features/08-client-surfaces.md#5-in-assistant-app-surface-mcp-apps-in-chatgpt--claude @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| CS-AC-23 | Adding the Gradion connector in ChatGPT/Claude surfaces the components with no extra setup. | Live-stack lane against the hosted connector |
| CS-AC-24 | Data in a component is **identical to and no broader than** the underlying tool result for that principal — same RBAC field-masking/row-scope — asserted by diffing the rendered payload against a typed tool call. | Contract conformance: rendered payload = typed tool result diff ([[byo-agent-and-mcp]] tool pins) |
| CS-AC-25 | Every in-component action routes through the **same MCP tool + Passport + 🟢/🟡 tier + audit** as a typed call; a 🟡 action with no token does nothing and is logged (`ErrRequiresApproval`). | Contract conformance on the tool tier ([[approvals-and-concurrency#APPR-WIRE-1]]; GATE-CS-6) |
| CS-AC-26 | On a client that cannot render components, the tool returns equivalent **text** — no error, no loss of the action path. | Integration lane: text-fallback fixture |
| CS-AC-27 | No component opens a data path or destination unavailable to the tool layer (egress invariant). | Egress conformance shared with GATE-CS-1 |

**Story acceptance — public inbound form (verbatim from the epic; the feature spec
carries no section for it — see CS-OPEN-7).**
Source: specs/spec/product/epics/E12-client-surfaces.md#s-e126--a-minimal-public-form-captures-inbound-as-a-lead-the-inbound-front-door @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| CS-AC-28 | Given a published form, when a visitor submits it, then a **lead** is created (not a contact) with each field carrying provenance (`source=form:<form-id>`), deduped against existing leads/people — a match links to the existing record, never a silent duplicate; an ambiguous near-match surfaces as a 🟡 candidate. | Integration lane: submit → 1 lead, 0 person; pre-seed exact match → 0 new rows; ambiguous → 🟡 candidate (GATE-CS-2/3/4) |
| CS-AC-29 | Given the form, when it renders, then it captures **per-purpose consent** (contact + optional marketing, separately checkable) with wording + timestamp stored as withdrawable proof (A22) — and it runs on the operator's / customer's own infrastructure, never a Gradion-hosted endpoint (A35). | Consent test: two distinct records with wording+timestamp ([[gdpr-platform]]); config test: operator-set endpoint (GATE-CS-1 posture) |
| CS-AC-30 | Given a submission, when it lands, then the lead is segregated like any external source and only **promotes to a contact on genuine engagement** — a raw form fill never pollutes contacts. | Segregation test: form lead absent from contacts until engagement ([[leads-and-qualification]]) |
| CS-AC-31 | Given the admin, when they configure the form, then it is a **bounded, fixed set of standard fields** (name/email/company/message + a small picklist), styled to the brand — **not** a drag-and-drop builder, not a landing-page CMS; a structurally new form is a source-level theme (ADR-0002 boundary). | Render test: fixed field set; static scan: no builder/CMS surface |
| CS-AC-32 | Given any submission, when it is processed, then it is attributable in the audit log and a spam/abuse guard (rate-limit + honeypot/CAPTCHA) protects the public endpoint. | Abuse-guard test: honeypot/over-limit → 0 lead, fake success; accepted → one audit row + one event (GATE-CS-5) |

**Screen acceptance — browser extension (owned screen; corpus IDs verbatim).**
Source: specs/spec/product/30-screen-acceptance.md#client-surfaceshtml--browser-extension-gmail-sidebar--linkedin-capture-implements-s-e121235 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-clientsurfaces-1 | Given the screen loads, When rendered, Then a fixed dark top bar shows the "g" mark, a "Back to Margince" link, a Gmail/LinkedIn tab switcher (Gmail active), and a network-isolation note "This panel only talks to YOUR workspace — no relay, no third party (S-E12.5)". No Ledger-Green nav rail (documented exception). | Screen e2e, live-stack lane ([[testing#TEST-LANE-3]]) |
| AC-clientsurfaces-2 | Given the tab switcher, When the user clicks a tab, Then only the matching view section shows, the tab gets active state, and the page scrolls to top. | Screen e2e |
| AC-clientsurfaces-3 | (Gmail 360) Given an open email from a known sender, When the panel loads, Then it shows the matched contact (name, title · company, "matched on sender"), a relationship-strength score with a meter, open-deals count, the deal mini-card (name, value, stage, "Stalled Nd", inferred next step), and a captured recent-activity timeline. | Screen e2e on seeded fixtures |
| AC-clientsurfaces-4 | (explainable score) Given the relationship-strength block, When the user clicks "How is this computed?", Then a deterministic breakdown expands (recency, two-way touches, stakeholders, last reply, rising delta) and the chevron rotates; collapse toggles. The score reads deterministic/no-black-box. | Screen e2e; formula owned by [[people-and-organizations#PO-F-3]] |
| AC-clientsurfaces-5 | (act in inbox — tiers) Given the "Act on this email" section, When the user views the buttons, Then each is 🟢: "Associate this email to deal", "Insert AI draft reply", "Create task / next step". | Screen e2e |
| AC-clientsurfaces-6 | (associate, no dup) Given the associate action, When clicked, Then it confirms "Associated to <deal> · done", states "Existing activity linked · 1 audit row · no duplicate", and a toast confirms nothing was re-logged. | Screen e2e + CS-AC-3 |
| AC-clientsurfaces-7 | (insert draft, confirm-first send) Given "Insert AI draft reply", When clicked, Then a personalized reply is written into the Gmail compose box, an "AI draft inserted · not sent" marker appears, and the toast states the user's Gmail send stays theirs — nothing auto-sent. | Screen e2e + CS-AC-4 |
| AC-clientsurfaces-8 | (create task) Given "Create task / next step", When clicked, Then it confirms a task (title · due · on the deal) attributed to the user. | Screen e2e |
| AC-clientsurfaces-9 | (unknown sender honest state) Given an email from a sender not in the CRM, When the panel loads, Then it shows "Not in your CRM", states opening the email created nothing, and offers "Add as lead" — capture creates a lead, not a contact (ADR-0008). | Screen e2e + CS-AC-2 (GATE-CS-2) |
| AC-clientsurfaces-10 | (egress/attribution) Given the Gmail panel, When viewed, Then a footer states reads/writes go only to the user's workspace under their RBAC and every action is audit-logged and attributed (`ext:gmail+human:<user>`). | Screen e2e |
| AC-clientsurfaces-11 | (LinkedIn one-click → lead) Given a LinkedIn profile, When the user clicks "Save to Gradion", Then a capture panel opens showing it will save as a LEAD (not a contact), with Name/Title/Company/Profile URL each carrying an evidence snippet, and Email shown as "Not readable — left empty, not guessed." | Screen e2e (GATE-CS-7) |
| AC-clientsurfaces-12 | (dedupe-first) Given the capture panel, When shown, Then it states the profile was deduped against leads AND people with no exact match and "Saving creates exactly one lead"; "Save lead" closes with a toast noting `source=ext:linkedin · 1 audit row` and that it stays out of contacts until engagement. | Screen e2e + CS-AC-8 |
| AC-clientsurfaces-13 | (already-exists state) Given the profile already matches an existing record by LinkedIn URL, When the dedupe state shows, Then the panel says "Already in your CRM", links to the existing lead (no duplication), and notes an ambiguous near-match would surface as a 🟡 candidate to confirm — never a silent merge. | Screen e2e + CS-AC-9/10 |
| AC-clientsurfaces-14 | (promotion rule) Given a saved lead, When displayed, Then the panel states it promotes to a contact only on genuine engagement; a cold capture stays a lead. | Screen e2e; rule owned by [[leads-and-qualification]] |
| AC-clientsurfaces-15 | (LinkedIn egress) Given the LinkedIn panel, When viewed, Then it states the profile read goes only to the user's workspace API — no third party, no relay (S-E12.5). | Screen e2e (GATE-CS-1) |

**Screen acceptance — public inbound lead form (owned screen; corpus IDs verbatim).**
Source: specs/spec/product/30-screen-acceptance.md#public-formhtml--public-inbound-lead-form-implements-s-e126 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-public-form-1 | Given the visitor lens is active, When the page loads, Then a fixed contact form renders with Name (required), Work email (required), Company, "What's this about?" topic select, and Message fields — and no tracking/marketing-builder fields beyond these. | Screen e2e ([[testing#TEST-LANE-3]]) |
| AC-public-form-2 | Given the Name field is empty or the email fails the format check, When the visitor clicks "Send message", Then the form is not submitted, the offending field is marked invalid, and its inline error ("Please enter your name." / "Enter a valid email address.") becomes visible. | Screen e2e |
| AC-public-form-3 | Given Name and a well-formed email are present, When the visitor clicks "Send message", Then the form step is hidden and a confirmation appears thanking the visitor by name, stating marketing was not opted into, and showing a reference id with a received timestamp. | Screen e2e; reference-id storage home open (CS-GAP-2) |
| AC-public-form-4 | Given the consent block, When the visitor reviews it, Then the "Contact me about this enquiry" checkbox is checked and disabled (marked "required") while the "product news & marketing" checkbox is unchecked and independently toggleable, with a note that each choice is stored separately with its wording and a timestamp. | Screen e2e + CS-AC-29 ([[gdpr-platform]]) |
| AC-public-form-5 | Given the hidden honeypot field (CS-PARAM-3) is filled, When the form is submitted, Then the visitor is shown the same success confirmation but no lead is created (silent fake-success drop), with no error surfaced. | Abuse-guard test + screen e2e |
| AC-public-form-6 | Given either lens is active, When the visitor clicks the "How it lands in the CRM" / "What the visitor sees" toggle, Then the operator lens and visitor lens swap visibility and the clicked toggle button gains the active state. | Screen e2e (prototype-reviewer affordance) |
| AC-public-form-7 | Given the operator lens, When it renders, Then the submission appears as a staged lead (not a contact) with each field stamped `source=form:contact-de`, the message shown verbatim, and two separate consent receipts (enquiry granted with ISO timestamp; marketing recorded as declined). | Screen e2e + CS-AC-28/29 |
| AC-public-form-8 | Given the operator lens shows a 🟡 possible-match card, When the operator picks "Link to existing contact", "Keep as a new lead", or "Not the same person", Then the dedupe card is replaced by the corresponding outcome message and no automatic merge occurs before that choice. | Screen e2e (GATE-CS-3; [[people-and-organizations#PO-AC-19]]) |

**Open build decisions (carried from the corpus open-questions register; append-only).**
Source: specs/spec/features/08-client-surfaces.md#8-open-questions--07-risksmd @ 5a0b29c; specs/spec/product/30-screen-acceptance.md#client-surfaceshtml--browser-extension-gmail-sidebar--linkedin-capture-implements-s-e121235 @ 5a0b29c

| ID | Open decision | Status |
|---|---|---|
| CS-OPEN-1 | **LinkedIn ToS / automation boundary:** human-viewed-profile one-click capture is in; background harvest and automated connect/message are out; conversation-sync needs an explicit ToS + consent review before build. Getting this wrong toward automation inherits account-ban/legal risk. | Open — gates the fast-follow conversation-sync trigger (CS-AC-12) |
| CS-OPEN-2 | **Browser support + manifest:** Chrome MV3 first (CS-PARAM-4); confirm the MV3 service-worker constraints hold against the egress-invariant test (GATE-CS-1). | Open — build-time confirmation |
| CS-OPEN-3 | **Outlook/M365 host parity (A51, V1-Must):** the sidebar on the Graph/Outlook add-in host ships at parity with Gmail in V1; the build must confirm the M365 add-in model meets the same egress + RBAC parity gates as the Gmail content-script — a V1 acceptance gate, no longer a deferral question. The screen prototype renders only the Gmail host. | Open — V1 gate |
| CS-OPEN-4 | **Lead capture at volume vs ADR-0008 ratification:** bulk list capture (table-stakes tier) is human-initiated but its volume tests the promotion-trigger line — ratify alongside the ADR-0008 "what engagement counts" open item ([[leads-and-qualification]]). | Open — product ratification |
| CS-OPEN-5 | **Self-hosted/on-prem endpoint discovery:** how the extension is pointed at a self-hosted workspace endpoint without weakening the no-third-party-host invariant (CS-AC-15). | Open — design decision |
| CS-OPEN-6 | **Start-sequence wiring order:** the screen register resolves that the 🟡 enrolment + suppression behavior "must be wired into the inbox sidebar" now that sequences are V1, while the build backlog retags the sidebar-enrol ticket behind the sequence engine, shipping the V1-safe slice (insert-draft + 🟡 send + create-task) first. The surface is this chapter's; the ordering conflict is carried honestly, resolved when the sequence engine's contract stub lands ([[sequences-and-deliverability]], CS-WIRE-8). | Open — sequencing conflict on record |
| CS-OPEN-7 | **Corpus sourcing defects:** (a) the epic's implemented-by line promises a feature-spec section for the public form that never landed — the feature spec's §6 holds the cross-cutting gates instead, so the epic story is this chapter's source of record for S-E12.6; (b) the in-assistant surface carries no story ID in any epic (the backlog coverage note stories the LinkedIn draft channel under E07 and the deal room under E08, and skips it). | Open — spec-governance note, no build blocker |
