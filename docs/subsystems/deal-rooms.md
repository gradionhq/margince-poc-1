---
status: planned
module: modules/dealrooms (backend) · web (buyer-facing room, account-free, no app shell)
derives-from:
  - specs/spec/features/08-client-surfaces.md#5b-digital-deal-room-buyer-facing-consent-gated
  - specs/spec/product/epics/E08-warm-room-and-signals.md#s-e084--a-shareable-deal-room-that-tells-me-whos-really-buying
  - specs/spec/product/epics/E08-warm-room-and-signals.md#s-e086--pat-receives-reviews-and-accepts-the-offer-in-the-deal-room
  - specs/spec/contract/data-model.md#deal-rooms-e08e12--consent-gated-buyer-facing-space-features08features12
  - specs/spec/contract/events.md#511-engagement--signals-e08-warm-room-e15-reply-tracking
  - specs/spec/product/30-screen-acceptance.md#deal-roomhtml--deal-room-buyer-facing-implements-s-e084-s-e086
---
# Deal rooms — one honest buyer-facing page per deal: disclosed tracking, company-level signal, no pixel

> The single shareable, consent-disclosed page where the buyer reads the offer,
> downloads the branded PDF, messages the rep, and accepts or requests a change —
> without an account — while their engagement becomes company-level
> buying-committee intel. Its promise: the buyer is always told what the room
> records, the signal is never a covert per-person dossier, and acceptance works
> even with tracking off.

## What it's for

Once a rep emails a proposal PDF they go blind: no idea who opened it, who it
was forwarded to, or who else on the buying side is involved. The deal room
replaces the scatter of attachments with one access-controlled page per deal —
proposal, documents, and the conversation thread — served from the workspace's
own infrastructure, and turns the buyer's disclosed, consented engagement into
the real buying-committee picture: who opened, what was forwarded, which
sections drew attention, single-threaded or broad. It is also where the buyer
half of the offer lifecycle happens: the buyer reviews line items and computed
totals, downloads the PDF, accepts, or asks for a change, with no login. Its
callers are the rep's deal surface (assemble and publish a room), the offers
chapter (the sent offer is shared here and accepted here), and the
signals-and-warm-room chapter (room engagement is a signal channel feeding the
warm room and coverage views). The boundary: this chapter owns the room, its
access and consent model, the engagement events, and the buyer flow — not what
acceptance *means* for the offer and the deal, which is pinned once at
[[offers-and-products]] (OFFER-AC-1..3) and cited here.

## Principles it serves

- **P7 — own-infra hosting.** The room is served only from the user's own
  workspace; no third-party scheduler-style intermediary holds buyer data
  ([[acceptance-standards#GATE-CS-1]]).
- **P12 / ADR-0011 / [[personas#PERSONA-PAT-GUARD-1]] — the honest signal beats
  the creepy one.** Tracking is disclosed before anything is recorded,
  attribution is company-level, and covert per-recipient tracking is a
  structural rejection ([[scope#NEVER-8]]) — the reply/view facts this chapter
  records are the product's only sanctioned "view" signal.
- **P11 — engagement joins the real deal graph.** Every event hangs off the
  room and its deal; the committee read is a join, not a bought data feed.
- **P5 — signal instead of guessing.** The rep multi-threads on observed
  engagement rather than on hunches about who matters.
- **ADR-0037 — the offer accepted in the room.** The buyer journey this chapter
  owns completes the bounded Angebote engine; the accept semantics stay pinned
  in the offers chapter.
- **Client-surface gates.** As a buyer-facing client surface, the room is bound
  by the applicable release gates: [[acceptance-standards#GATE-CS-1]] (egress),
  [[acceptance-standards#GATE-CS-4]] (provenance),
  [[acceptance-standards#GATE-CS-5]] (RBAC + audit parity on the rep side),
  [[acceptance-standards#GATE-CS-6]] (confirm-first — the room is not a back
  door for outbound), and [[acceptance-standards#GATE-CS-8]] (design-system
  primitives).

## How it works

**A room is born on one deal, and its link is the credential.** The rep
assembles a room — the sent offer, shared documents, the relevant thread — and
publishes it as a single unguessable link: the slug in the URL is the access
token, not a guessable identifier (DEALROOM-PARAM-1). An email-gated mode
additionally requires the opener's address to match a shared recipient, and the
link can carry an expiry so access is revocable (DEALROOM-PARAM-6). There is no
public unlisted mode: access control is structural, and the buyer never creates
an account.

**Consent is disclosed before the room goes live.** A room cannot activate
until the disclosure flag is set: the buyer-facing page states that engagement
is tracked (DEALROOM-PARAM-2), and no engagement event is recorded before that
disclosed-consent state. The buyer can turn tracking off in the room itself —
the page keeps working as a share-and-accept surface, activity logging simply
stops. Acceptance and its audit trail never depend on engagement tracking.

**Room content is governed, never auto-published.** What the rep shares is
content they assembled: the rendered offer PDF from the offers chapter and
documents drawn from the governed asset store, so a claim in the room traces to
an approved, in-date asset ([[drafting#DRAFT-AC-1]], [[drafting#DRAFT-AC-2]]).
Publishing a room is the rep's deliberate act.

**The buyer flow is read, decide, respond.** The buyer sees the offer's line
items and server-computed totals read-only — with an "explain this total" panel
deriving the arithmetic — downloads the branded PDF, and switches between
offer, documents, and conversation. Accepting runs the accept semantics owned
by the offers chapter: status flips, the deal's amount syncs from the accepted
gross, the accepted event pairs a deal update, all audited
([[offers-and-products]] OFFER-AC-2); V1 acceptance is a tracked in-room action
plus the audit trail, not a legally-binding e-signature, and the page says so
honestly. Requesting a change never lets the buyer edit the priced offer — the
request lands as a signal to the rep, who regenerates the next revision as a
diffed draft (OFFER-AC-1); the rep owns the offer, the buyer requests.
Messages in the conversation pane are attributed and timestamped.

**Engagement becomes company-level intel, never a dossier.** With tracking on,
views, opens, asset opens, downloads, forwards, section attention, replies, and
offer acceptance are recorded as high-volume events on the room
(DEALROOM-PARAM-4), each attributed at most coarsely — a recipient email or
role label, or anonymous — and presented to the rep as the buyer organization's
engagement (DEALROOM-PARAM-5). The rep sees the committee form — a new name
means the committee is widening — with the evidence behind every flag: which
room, what was opened or forwarded, when. There is no cross-site tracking
pixel and no per-recipient email open beacon anywhere in the product; the
bus-level rule that engagement is reply-based, never an open-pixel, is pinned
at [[event-bus#EVT-SEM-14]], and this chapter carries the product's pixel guard
(DEALROOM-AC-6) that the sequences-and-deliverability chapter cites. Room
engagement then feeds outward: it is one of the signal channels the
[[signals-and-warm-room]] chapter ingests, so a buyer re-engaging in a room can
surface warm in the rep's queue.

**Acting on the intel stays gated.** When the room reveals a newly-engaged
contact, any outbound the rep takes from that discovery — an intro draft, a
follow-up — is confirm-first like every outbound in the product
([[acceptance-standards#GATE-AI-7]]); the room surfaces the committee, the rep
decides and sends.

## What's configurable

- **Access mode** — token-only (default) or email-gated per room
  (DEALROOM-PARAM-1); an optional expiry on the link (DEALROOM-PARAM-6).
- **Shared content** — which assets, documents, and thread the rep surfaces to
  the buyer, per room.
- **Tracking** — on only after disclosure, and the buyer can switch it off in
  the room; the share-and-accept function is unaffected.
- **What is not configurable** — the consent-before-active gate, the
  company-level attribution ceiling, the absence of any pixel path, and the
  buyer's inability to edit the priced offer are invariants, not knobs.

## Guarantees (enforced)

- **No engagement before disclosed consent.** A visitor is informed tracking is
  on before any engagement is recorded; a room cannot go active undisclosed
  (DEALROOM-PARAM-2, DEALROOM-AC-3).
- **Company-level, never per-person covert.** Engagement events attribute to
  the buyer organization with at most coarse actor labels; no covert
  per-individual behavioral profile is created ([[scope#NEVER-8]],
  [[personas#PERSONA-PAT-GUARD-1]], DEALROOM-AC-4).
- **No pixel ships.** A static check asserts no per-recipient open-pixel path
  exists (DEALROOM-AC-6); the only V1 "view" signal is the room's disclosed
  engagement, and reply semantics stay thread-matched
  ([[event-bus#EVT-SEM-14]]).
- **Own-infra only.** The room is served exclusively from the user's own
  workspace infrastructure — no relay, no third-party host
  ([[acceptance-standards#GATE-CS-1]], DEALROOM-AC-5).
- **Access is controlled and revocable.** The slug is an unguessable credential,
  unique per workspace; email gating and expiry tighten it; no uncontrolled
  public rooms exist (DEALROOM-DDL-1).
- **Accept works without tracking.** Turning tracking off dims logging, never
  acceptance — the decision of record and its audit trail are independent of
  the engagement stream (AC-deal-room-6).
- **Accept means what offers says it means.** The room invokes the accept whose
  semantics are pinned once — sent offers immutable, accept syncs the deal
  amount, totals server-computed ([[offers-and-products]] OFFER-AC-1..3).
- **The buyer never mutates the offer.** Change requests leave the priced offer
  intact and route to the rep's regenerate path (AC-deal-room-3).

## Acceptance

Done means: a rep publishes one access-controlled, consent-disclosed link per
deal; the buyer opens it without an account, reads the offer with its computed
totals, downloads the PDF, messages, and accepts or requests a change; the rep
watches the buying committee form as evidenced, company-level engagement, and
anything they send in response still passes the approval gate. The honest
states are part of the contract: the accepted state with its audit stamp, the
change-requested state with the offer left intact, the tracking-off state where
logging dims but acceptance still works, and the blocked empty change-request
note. The prototype lacks loading, expired-/revoked-link, and
submission-failure states — the planned build must add them per the standard
screen-state floor inherited from [[acceptance-standards]], plus honest failure
copy that in-room acceptance is not a legally-binding e-signature. Testable
forms live in the Acceptance appendix.

## Out of scope

- **Accept semantics, offer immutability, server-priced totals** — pinned once
  at [[offers-and-products]] (OFFER-AC-1..3); this chapter's surface invokes
  them and its tests cite them.
- **The warm-room join and the signal store** — [[signals-and-warm-room]]; room
  engagement is one channel feeding it.
- **E-signature** — legally-binding e-sign is a fast-follow owned by the
  germany-package chapter; V1 acceptance is the tracked in-room action.
- **Per-recipient email open tracking** — deliberately not built anywhere;
  sequences-and-deliverability cites this chapter's pixel guard rather than
  shipping its own view signal.
- **Section-level attention analytics, per-vertical room templates, e-sign
  handoff** — table-stakes fast-follows named by the corpus cut line.
- **Payment collection and public unlisted rooms with no access control** —
  OUT, per the corpus cut line.

## Where it lives

The deal-rooms module in the backend (modules/dealrooms), with the buyer-facing
page shipped account-free outside the app shell and the rep-side room controls
on the deal surface. Read next: [[offers-and-products]] (what acceptance
means), [[signals-and-warm-room]] (where engagement lands as signal),
[[drafting]] (the governed assets a room shares), and event-bus (the
engagement event family).

## Appendix

### Parameters
Source: contract/data-model.md#deal-rooms-e08e12--consent-gated-buyer-facing-space-features08features12 @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| DEALROOM-PARAM-1 | Access mode | `access_mode` default `'token'`; `'token'` or `'email_gated'` | The slug is the access credential (an unguessable token in the share URL); email-gated additionally requires the opener's email to match a shared recipient. |
| DEALROOM-PARAM-2 | Consent gate | `consent_disclosed` default `false`; must be `true` before `status='active'` | The buyer-facing space discloses that engagement is tracked before the room goes live; no engagement event is recorded without the disclosed-consent state. |
| DEALROOM-PARAM-3 | Room lifecycle | `status` in `draft → active → closed`, default `draft` | A draft room is the rep's staging area; only an active room is buyer-reachable. |
| DEALROOM-PARAM-4 | Engagement vocabulary | `view`, `open`, `asset_open`, `download`, `forward`, `section_attention`, `reply`, `offer_accept` | The only V1 "view" signal (RT-PR-H2); `offer_accept` added for the offers V1 (A48). |
| DEALROOM-PARAM-5 | Attribution ceiling | `actor_ref` nullable coarse label (recipient email / role), `NULL` when anonymous | Company-level attribution — never a cross-site tracking pixel or per-person covert profile. |
| DEALROOM-PARAM-6 | Link expiry | `access_expires_at` nullable, no default | Optional expiry on the access token; the revocation half of the scoped, revocable room link. |

### Schema
Source: contract/data-model.md#deal-rooms-e08e12--consent-gated-buyer-facing-space-features08features12 @ 5a0b29c

Two tables, owned here per the schema ownership index
([[data-model#Schema — ownership index]]). `deal_room` carries the universal
base columns plus `version`; the engagement table is high-volume append-only
with its own explicit columns.

DEALROOM-DDL-1 — `deal_room`:

```sql
CREATE TABLE deal_room (
  -- + base columns + version
  deal_id       uuid NOT NULL REFERENCES deal(id),
  slug          text NOT NULL,                            -- the unguessable access token in the URL (the access credential, not a guessable id)
  access_mode   text NOT NULL DEFAULT 'token' CHECK (access_mode IN ('token','email_gated')),  -- 'token' = slug only; 'email_gated' = slug + recipient email match
  access_expires_at timestamptz NULL,                     -- optional expiry on the access token
  consent_disclosed boolean NOT NULL DEFAULT false,       -- the buyer was told engagement is tracked (consent gate, features/08) — a room may not go 'active' with this false
  status        text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','closed')),
  shared_assets jsonb NOT NULL DEFAULT '[]',              -- attachment refs surfaced to the buyer
  UNIQUE (workspace_id, slug)
);
```

DEALROOM-DDL-2 — `deal_room_engagement_event`:

```sql
CREATE TABLE deal_room_engagement_event (                 -- high-volume; buyer views/downloads/forwards (the only V1 "view" signal, RT-PR-H2)
  id uuid PRIMARY KEY, workspace_id uuid NOT NULL REFERENCES workspace(id),
  deal_room_id  uuid NOT NULL REFERENCES deal_room(id),
  event_type    text NOT NULL CHECK (event_type IN ('view','open','asset_open','download','forward','section_attention','reply','offer_accept')),  -- forward/section_attention/open added E08.5; offer_accept A48/§12.6
  asset_ref     text NULL,
  actor_ref     text NULL,                                -- coarse buyer-side attribution (e.g. recipient email / role label); NULL when anonymous
  detail        jsonb NULL,                               -- e.g. {section, dwell_ms} for section_attention
  occurred_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_dre_room ON deal_room_engagement_event (workspace_id, deal_room_id, occurred_at DESC);
```

Corpus access-and-consent note (verbatim): the `slug` is the access credential
(an unguessable token in the share URL) — `access_mode='email_gated'`
additionally requires the opener's email to match a shared recipient.
`consent_disclosed` must be `true` before a room transitions to `active`: the
buyer-facing space discloses that engagement is tracked.
`deal_room_engagement_event` now covers `forward` and `section_attention` with
optional coarse `actor_ref` attribution (never a cross-site tracking pixel —
features/10 §5).

Note DEALROOM-DDL-N-1: the offer acceptance recorded here as `offer_accept` is
the offers chapter's semantics (OFFER-AC-2) landing on this chapter's table —
the event row is owned here, its meaning there.

### Wire
Source: contract/crm.yaml (NET-NEW V1 RESOURCES planned block, comments only) @ 5a0b29c

**Honest contract-coverage finding:** at pin time `crm.yaml` defines 81
operations and **none** is a deal-room operation — the surface exists in the
contract only as the net-new-resources comment block ("/deal-rooms — E08/E12;
engagement events are read via /deal-rooms/{id}/engagement"). The
unauthenticated buyer-facing surface (open by slug, download, message, accept,
request change) appears nowhere at all. The chapter pins the promised surface
by path + behavior; operationIds must be minted by a contract extension before
any docs-cited operationId can resolve. Until then, no prose or ticket may cite
a deal-room operationId as if it existed.

| ID | Element (planned path) | Behavior pinned |
|---|---|---|
| DEALROOM-WIRE-1 | `/deal-rooms` | Standard §12.5 resource shape — list (cursor+sort), get, create, update (If-Match), archive; schemas 1:1 from DEALROOM-DDL-1. Rep-side, under the human's RBAC ([[acceptance-standards#GATE-CS-5]]). Activation must enforce the DEALROOM-PARAM-2 consent gate. |
| DEALROOM-WIRE-2 | `/deal-rooms/{id}/engagement` | Engagement events are read through the parent room, per the corpus contract-surface note; `deal_room_engagement_event` has no standalone CRUD. |
| DEALROOM-WIRE-3 | Buyer-facing room surface (slug URL) | Honest gap: the public, account-free buyer flow — open by slug (+ email gate), tab/PDF/download, conversation message, accept, request-change, tracking toggle — has **no** contract coverage, not even a comment. It must be minted with the slug-as-credential access model and the no-engagement-before-consent rule enforced server-side. |
| DEALROOM-WIRE-4 | Offer accept from the room | Owned by offers as its planned accept action (OFFER-WIRE-9); the room invokes it and records the `offer_accept` engagement row — semantics per [[offers-and-products]] OFFER-AC-2, never redefined here. |

### Events
Source: contract/events.md#511-engagement--signals-e08-warm-room-e15-reply-tracking @ 5a0b29c

Room engagement is deliberately **not** a per-event bus family: views, opens,
downloads, forwards, and section attention are high-volume table rows
(DEALROOM-DDL-2) read through the parent room, not domain events. The
bus-visible facts this chapter leans on, defined and owned by the central
catalog ([[event-bus]]) and cited here:

| ID | Event | This chapter's role |
|---|---|---|
| DEALROOM-EVT-1 | `engagement.reply` | A buyer reply in the room emits the reply fact with channel `deal_room` — thread-matched, idempotent per reply, **never an open-pixel** ([[event-bus#EVT-SEM-14]], the reply-not-pixel rule). Consumed by the context graph, the warm-room read model, workflows, and the audit stream. |
| DEALROOM-EVT-2 | `offer.accepted` | Emitted by the offers accept path the room invokes; pairs a deal update on one correlation id — owned by the catalog and [[offers-and-products]] (OFFER-AC-2), cited here because the buyer's click happens on this surface. |
| DEALROOM-EVT-3 | `signal.detected` | Downstream: room engagement enters the signal store as the deal-room-engagement channel; the signal row and its event are [[signals-and-warm-room]]'s. |

### Acceptance

#### Acceptance — stories (condensed G/W/T)
Source: product/epics/E08-warm-room-and-signals.md#s-e084--a-shareable-deal-room-that-tells-me-whos-really-buying @ 5a0b29c; product/epics/E08-warm-room-and-signals.md#s-e086--pat-receives-reviews-and-accepts-the-offer-in-the-deal-room @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| DEALROOM-AC-1 | Given a deal, when Sam creates a room, then he gets one shareable buyer-facing page (proposal + documents + thread, one link — not scattered attachments); with consented tracking on, engagement surfaces as a buying-committee signal (opened, forwarded, section attention) that is company-level and consent-gated, evidenced on ask (which room, what, when), still works as a share page with tracking off, and any outbound Sam takes on a newly-engaged contact is 🟡 confirm-first. (S-E08.4, V1-WOW) | Integration + E2E, deal-rooms lane; [[acceptance-standards#GATE-AI-7]] |
| DEALROOM-AC-2 | Given Sam sent the offer, when Pat opens the room link, then Pat sees the line items and computed totals, downloads the branded PDF with no account (scoped, revocable link), and the view is a tracked engagement event (consent-gated, company-level, the only V1 "view" signal); Accept runs the offers accept semantics (status, deal-amount sync, event + audit — [[offers-and-products]] OFFER-AC-2), with no third-party e-sign in V1; a change request lands as a signal for regenerate-from-signal and Pat can never edit the priced offer (OFFER-AC-1); every buyer action is attributable and consent-gated, and with consent absent the room still works as a share+accept page. (S-E08.6, V1-WOW) | Integration + E2E citing OFFER-AC-1..3; event assertion |

#### Acceptance — digital-deal-room capability ACs (verbatim)
Source: features/08-client-surfaces.md#5b-digital-deal-room-buyer-facing-consent-gated @ 5a0b29c

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| DEALROOM-AC-3 | Given a deal, the rep publishes a **consent-disclosed**, access-controlled room; a visitor is informed tracking is on **before** any engagement is recorded. *(test: no engagement event recorded without the disclosed-consent state)* | Integration test, deal-rooms lane |
| DEALROOM-AC-4 | Engagement (opens, forwards, section attention) is recorded as **deal-linked events** with provenance, and surfaces a **buying-committee/multi-threading** read with the evidence (who/role/when) — **company-level, consent-gated**; **no** covert per-individual behavioral profile is created. *(deterministic test: events link to the deal; orphan/unconsented engagement → 0 person-profile rows)* | Deterministic test; [[scope#NEVER-8]]; [[personas#PERSONA-PAT-GUARD-1]] |
| DEALROOM-AC-5 | The room is served only from the user's own workspace infra (P7 egress invariant); access is controlled (not a public unlisted URL). *(egress + access-control test)* | Egress + access-control test ([[acceptance-standards#GATE-CS-1]]) |
| DEALROOM-AC-6 | No per-recipient open-pixel path ships (R5c guard). *(static check)* | Static check — the product-wide pixel guard; cited by sequences-and-deliverability; [[event-bus#EVT-SEM-14]] |

#### Acceptance — screen: deal room, buyer-facing (verbatim)
Source: product/30-screen-acceptance.md#deal-roomhtml--deal-room-buyer-facing-implements-s-e084-s-e086 @ 5a0b29c

This chapter owns the buyer-facing room screen (stories S-E08.4 and S-E08.6);
the offer-builder screen is offers-and-products'. Corpus screen-AC IDs
preserved verbatim.

| ID | Given/When/Then (corpus text verbatim) | Verification |
|---|---|---|
| AC-deal-room-1 | Given the room loads on the Offer tab, When the buyer reads the offer card, Then the line items, Netto-Zwischensumme (148.800,00 €), 19% MwSt. (28.272,00 €) and Gesamt (177.072,00 €) are shown read-only with the status pill "Sent — awaiting your decision", and no field is editable by the buyer. | Screen/E2E test (read-only per OFFER-AC-1) |
| AC-deal-room-2 | Given the offer card is shown, When the buyer clicks "Accept offer · 177.072,00 €", Then the accept band is replaced by a confirmation box ("Offer accepted. Thank you, Anna."), the status pill flips to "Accepted", an audit line stamps `offer.accepted · Anna Weber · <timestamp>`, and a toast confirms the deal amount synced to 177.072,00 € and the rep was notified. | Screen/E2E test citing [[offers-and-products]] OFFER-AC-2 |
| AC-deal-room-3 | Given the buyer does not want to accept as-is, When they click "Request a change", enter a note, and click "Send request", Then the priced offer stays unchanged, the status pill flips to "Change requested", and a confirmation tells them the rep will prepare a new revision; When the note is empty, Then sending is blocked with a toast asking for a short note. | Screen/E2E test (offer intact per OFFER-AC-1) |
| AC-deal-room-4 | Given tabs Offer / Documents / Conversation, When the buyer switches tabs, Then only the selected pane is visible; the Documents pane lists the offer PDF, GxP validation plan and Security/DPA each with a download affordance, and the Conversation pane shows the message thread. | Screen/E2E test |
| AC-deal-room-5 | Given the Conversation tab, When the buyer types a message and presses Enter or the send button, Then the message is appended to the thread attributed to the buyer with a timestamp and a "Message sent" toast appears. | Screen/E2E test |
| AC-deal-room-6 | Given the consent banner shows "Tracking on", When the buyer toggles it off, Then a toast confirms "the offer and Accept still work", the recorded-activity list dims, and tab/PDF/change activity stops being logged — while Accept remains fully functional. | Screen/E2E test (DEALROOM-PARAM-2 consent honesty) |
| AC-deal-room-7 | Given tracking is on, When the buyer opens a tab, downloads the offer PDF, or requests a change, Then a corresponding company-level entry ("attributed to BÄR Pharma GmbH" / "company-level engagement signal") is appended to the "What this room records" list. | Screen/E2E test (DEALROOM-PARAM-5 attribution ceiling) |
| AC-deal-room-8 | Given the totals card, When the buyer clicks "Explain this total", Then a panel reveals the line-by-line computation in minor units (Σ qty × unit_price × (1−disc%), tax = net × 19%, gross = net + tax) and states prices are fixed by the seller and the buyer may only accept or request a change. | Screen/E2E test citing [[offers-and-products]] OFFER-AC-3 |

Note DEALROOM-AC-N-1: the prototype ships no loading state, no
expired-/revoked-link state, and no error state on accept/change/message
submission (actions are optimistic with success toasts only) — the planned
build must add them per the standard screen-state floor
([[acceptance-standards#Acceptance — standard screen-state matrix]]), alongside
the existing honest-failure notice that in-room acceptance is the decision of
record but not a legally-binding e-signature.
