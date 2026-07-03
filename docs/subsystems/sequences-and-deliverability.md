---
status: planned
module: backend/internal/modules/comms · frontend/src/features/sequences
derives-from:
  - margince specs/spec/features/06-deliverability-and-migration.md#part-1--email-sending--deliverability @ 5a0b29c
  - margince specs/spec/features/02-capture-and-comms.md#feature-4--email-templates--sequences--cadences @ 5a0b29c
  - margince specs/spec/features/02-capture-and-comms.md#feature-5--telephony-click-to-call-auto-log @ 5a0b29c
  - margince specs/spec/features/10-operational-depth.md#5-outreach-engine-promotes-d43-reply-tracking-d45-sequences-d411-bulk-activity-d48-telephony @ 5a0b29c
  - margince specs/spec/product/epics/E15-operational-depth.md#s-e155--sequences-reply-tracking--bulk-activity @ 5a0b29c
  - margince specs/spec/product/epics/E11-access-trust-exit.md#s-e119--buyer-facing-preference-center--one-click-unsubscribe @ 5a0b29c
  - margince specs/spec/product/build-backlog/E15.md#e-outreach-engine-features10-5 @ 5a0b29c
  - margince specs/spec/contract/data-model.md#12-deferred-tables @ 5a0b29c
  - margince specs/spec/contract/events.md#511-engagement--signals-e08-warm-room-e15-reply-tracking @ 5a0b29c
  - margince specs/spec/product/30-screen-acceptance.md#sequenceshtml--sequence-builder--reply-tracking-implements-s-e155a-s-e155b @ 5a0b29c
---
# Sequences and deliverability — gated outbound that reaches the inbox and stops the moment someone replies

> The outreach engine: multi-step outbound sequences, honest reply tracking, bulk
> activity, click-to-call telephony, and the transport-and-deliverability guardrails
> under all of them. Every send is downstream of a recorded approval; one workspace
> suppression table is consulted on every send, no exceptions; the only engagement
> signal is a real reply — never a tracking pixel.

## What it's for

The capture-and-comms feature specified templates, sequences, and the confirm-first
send gate but never said how a byte of email leaves the building — and a CRM that
sends is judged on whether mail reaches the inbox, not the spam folder. This
subsystem is that missing half plus the outreach engine promoted into V1 by the
operational-depth decision: the sequence model with steps, delays, and exit-on-reply;
reply tracking; bulk activity; telephony; and the single send subsystem that owns
transport, suppression, unsubscribe, pacing, and sending-domain health for every
outbound path.

Its callers are the manual composer, the sequence scheduler firing steps, agents
sending over the governed tool surface, the bulk-operations engine enrolling record
sets, and the telephony dialer on person and deal records. The content of each step
is drafted by the [[drafting]] chapter (in the user's voice, [[voice-profile]]);
the approval machinery every send waits on is [[approvals-and-concurrency]]; inbound
capture that detects replies is the capture subsystem. This chapter owns what happens
from "approved send intent" to "delivered, suppressed, bounced, or replied" — and the
buyer-facing exit from it all, the preference center.

## Principles it serves

- **P5 — capture-first.** The default transport is the user's own mailbox, because
  most of what we send is a reply or follow-up off auto-captured context, not a cold
  blast; the sent mail and its reply land back in the same mailbox the capture
  connector already reads, closing the loop with no second integration.
- **P12 — gated, audited, attributable.** Outbound is the deliberate exception to
  automation: capture is free, transmit waits for a human. Every transmit is
  audit-logged with its authority and approval; recording is consent-gated; batches
  are single audited acts.
- **P1 — one opinionated model.** One sending model by default, one sequence shape,
  one suppression table, one pacing policy — deliverability knobs are earned, not a
  mail-server console.
- **P7 — no operator dependence on us.** Sending never routes through infrastructure
  we run (ADR-0027); the on-premise profile sends through the client's own mail
  tenant.
- **ADR-0036 — the approval token.** Transport is strictly downstream of the recorded
  🟡 token; the sequence engine can never become an unattended sender.
- **ADR-0026 — autonomy tiers and the amount threshold.** Sequence sends route by
  batch size: under the user's opted-in threshold they may auto-send, over it they
  queue for approval — never a silent mass blast.

## How it works

**One send subsystem, two transports.** The default and V1 model sends through the
user's own connected mailbox — their real address, their provider's reputation,
their existing threads. This inherits deliverability we did not have to build and
keeps replies flowing back into capture. The alternative, an operator-run relay for
true volume, is a deliberate fast-follow — and it is the operator's or customer's
relay on their sending domain, never one of ours (ADR-0027). Provider quotas cap
blast volume by construction; marketing-grade broadcast is explicitly out of scope.

**Every transmit passes one gate, in one place.** Regardless of path — manual,
sequence step, agent-approved batch, bulk enrol — the send subsystem runs the same
deterministic pre-transmit check (SEQDEL-FORM-1): a recorded approval or
under-threshold opt-in must authorize it, the recipient must be absent from the
workspace suppression table, the send purpose must have proven consent
([[gdpr-platform]] default-deny), and the transmit must fit the pacing schedule.
An agent-originated send without a token gets a requires-approval answer and the
transport receives nothing; deliverability adds no new way to send. When a human
approves a fifty-recipient batch, they approve content; the system paces delivery
across the quota schedule, and the audit record carries both the approval and the
actual send timestamps.

**Sequences are steps, delays, and exits.** A sequence is an ordered set of email
steps and call tasks with configured delays, scheduled on the platform job runner.
Each email step is drafted per recipient in the sender's voice by [[drafting]] and
can regenerate from the latest captured signal, shown as an accept-or-dismiss diff.
Sends route by the amount threshold (SEQDEL-PARAM-1, default 20): a step fanning out
under the opted-in threshold may auto-send; over it, the batch queues to the approval
inbox. Enrollment comes from a record, a list, or the governed bulk path; suppressed
and opted-out contacts are excluded at enrol time, not just at send time.

**Reply tracking is the honest signal.** An inbound message that thread-matches a
prior outbound emits one idempotent reply event from capture ([[event-bus#EVT-SEM-14]]).
For an enrolled contact that event pauses the sequence — no further steps fire — and
routes a genuine reply into lead-to-contact promotion ([[leads-and-qualification]]).
There is no open-pixel and no covert engagement mechanism anywhere in the product:
the corrected scoring ruling (RT-PR-H2) names replies, meetings, and disclosed
deal-room views as the V1 signals, the deal-rooms chapter keeps per-recipient pixel
tracking deliberately unbuilt, and a static guard asserts no such path exists.
Open/click tracking, if it ever arrives, is a fast-follow owned by [[lead-scoring]].

**Suppression is one table, consulted always.** Hard bounces, spam complaints,
unsubscribes, and manual additions land in a single workspace-scoped suppression
table; every send path checks it and a suppressed recipient yields zero transport
calls. Membership is a sanctioned runtime surface ([[runtime-config]] RC-6) — a list
of contacts, not logic. This is send suppression; the erasure suppression list that
keeps deleted subjects from reappearing is [[gdpr-platform]]'s and is a different
mechanism.

**Unsubscribe and the preference center.** Every bulk or sequence message carries
the one-click unsubscribe headers (RFC 8058) and a visible link. The click needs no
login: it lands the recipient on the suppression list immediately and opens the
buyer preference center, a public, signed-link, bilingual surface where they adjust
per-purpose consent — marketing, events, research, with transactional messages
locked on while a deal is live. Each choice is recorded with its exact wording and
timestamp as a withdrawable consent proof on the [[gdpr-platform]] substrate, and an
opt-out is enforced instantly and cannot be overridden by any human or agent,
independent of role or passport.

**Deliverability is designed in, not configured.** Before a workspace's first send
and daily thereafter, the sending domain is health-checked — sender authentication
present and aligned, domain not blocklisted — and the result is a green, amber, or
red badge with plain-language remediation, not a DNS console. Bounces are classified
hard versus soft; a hard bounce suppresses the recipient and updates contact status.
Sends pace within provider norms: per-account daily quotas, spread across
working-hours windows (SEQDEL-PARAM-2).

**Telephony completes the activity surface.** Click-to-call from a person or deal
record dials through the one connected provider. Recording is governed by the
per-jurisdiction consent config (RC-7, owned by [[meetings-and-transcripts]]) and
the no-consent case is a hard block enforced at the bridge — the call still places,
the recording is refused, not hidden. A completed call lands as a provenance-tagged
call activity on the right record ([[activities-and-timeline]]); a consented
recording flows into the existing transcript-to-summary pipeline, and the post-call
summary and follow-ups are staged for accept-or-dismiss, never auto-written.

**Bulk activity is one governed batch.** Mass log, mass task, and bulk enrol run on
the shared bulk-operations path: one batch, bounded by the actor's permissions
(records out of scope are excluded and shown, never silently written), one audit row
answering "what did this batch touch", and for bulk enrol the suppressed and
opted-out contacts are dropped from the batch with the exclusion visible.

**Imported sequences arrive paused.** Migrated sequence definitions are created
paused with zero auto-enrollments — re-enrollment into a migrated audience is a
human act. That behavior is owned and pinned by [[import-export-migration]]
(its AC-M12); this chapter's engine simply honors the paused state.

## What's configurable

- **The auto-send amount threshold** — per-user opted-in batch size under which a
  sequence step may send without a fresh approval; default 20 recipients
  (SEQDEL-PARAM-1), adjustable on the sequence screen with a live echo of what the
  next step will do. Above it, batches always queue for approval (ADR-0026).
- **Suppression-list membership** — the RC-6 runtime surface: auto-added on bounce
  and unsubscribe, manually addable and removable; data, not logic
  ([[runtime-config]]).
- **Recording consent** — the RC-7 per-jurisdiction policy is owned by
  [[meetings-and-transcripts]]; this chapter consumes it as a hard server-side gate,
  never a UI preference.
- **The sending model** — per workspace, not per send: user-mailbox by default;
  the operator-relay model is a fast-follow for proven volume needs, with warmup
  ramping (SEQDEL-PARAM-4) owned here when it lands.
- **The mailbox connector** — the injected transport dependency. When a mailbox
  token expires, drafting continues but nothing leaves: queued sends are held, not
  dropped, and the surface says so with a reconnect path.
- **The telephony provider** — one opinionated provider in V1 behind the connector
  seam; swap is a deployment concern, not a user setting.

## Guarantees (enforced)

- **No send without approval.** The transport layer is never invoked for an
  agent-originated send without a recorded approval token; an unconfirmed call
  returns requires-approval and produces zero transport calls (AC-D2, ACX.3 at the
  tool-contract tier, [[approvals-and-concurrency#APPR-WIRE-1]]).
- **Suppression is honoured universally.** A recipient on the suppression table —
  any reason — yields zero transmit calls on every path: manual, sequence,
  agent-approved, bulk (AC-D1).
- **Opt-out is instant and un-overridable.** A suppressed or opted-out address is
  excluded from every future send, and enrolment of an opted-out contact is blocked
  with the reason for humans and agents alike, independent of RBAC or passport
  (S-E11.9; AC-preference-center-7).
- **Unsubscribe is one click and honored end-to-end.** Every bulk/sequence message
  carries the one-click unsubscribe headers; the unsubscribe adds the recipient to
  suppression and unenrolls them from active sequences (AC-D3).
- **A reply stops the machine.** An inbound reply detected against an enrolled
  contact pauses the sequence — no further steps fire — and triggers promotion
  (SEQDEL-AC-2; [[event-bus#EVT-SEM-14]]).
- **No open-pixel path exists.** Reply, not open, is the only engagement signal; a
  static test asserts no tracking-pixel or covert-open mechanism ships (B-E15.18;
  RT-PR-H2; the deal-rooms pixel guard is cited, owned there).
- **Hard bounce suppresses.** A hard-bounce signal adds the recipient to suppression,
  marks contact status, and blocks the next send (AC-D4).
- **Pacing caps are respected.** An approved batch never exceeds the per-account
  daily cap in a window; the remainder schedules after (AC-D6).
- **Every transmit is audited.** Template, resolved body hash, sender authority with
  approval id, recipient, model, and outcome — one audit entry per transmit (AC-D7).
- **Recording without consent is refused, not hidden.** A no-consent jurisdiction
  blocks the recording write at the bridge while the call still places (AC5.3,
  SEQDEL-AC-3).
- **Bulk is one governed batch.** Mass log/task/enrol runs permission-bounded with
  out-of-scope records excluded (never silently written) and exactly one audit row
  (SEQDEL-AC-4).
- **Templates fail loudly.** A merge token with missing data blocks the send rather
  than rendering blank (AC4.1).

## Acceptance

Done means a rep can run a real outbound cadence without leaving the product and
without ever blasting someone who engaged: enrol a set, watch drafted steps go out
under their threshold or queue above it, see the cadence pause the instant a reply
arrives with the reply quoted as evidence, log and enrol in bulk as one audited act,
and dial from a record with the call captured automatically. An admin sees a plain
green/amber/red sending-health badge before the first send. A buyer can leave with
one click, see exactly what they consented to, and never be re-enrolled against
their choice. The surfaces render their honest states: hard bounces shown, held
sends on an expired mailbox token shown with a reconnect path, permission-excluded
and opt-out-excluded records visible rather than silently dropped, and a blocked
jurisdiction's recording toggle locked with the reason. The cross-cutting screen
floor and release gates are inherited from [[acceptance-standards]] and not
restated.

One structural build-order fact is carried honestly: the sequence tables and their
API are a deferred stub in the corpus contract, and the operational-depth promotion
of record makes ratifying that schema, wire surface, and event set the first gate
of this chapter's tickets — see the Schema and Wire appendices for the
pinned gaps (SEQDEL-GAP-1..3).

## Out of scope

- **The bulk-operations screen and engine** — the bulk-actions surface serves both
  the access-and-admin bulk-ops story (primary) and this chapter's bulk-activity
  atom; the screen and the shared batch engine belong to [[access-and-admin]]. This
  chapter owns the enrol operation the batch invokes and pins only its behavior.
- **Starting a sequence from the inbox sidebar** (story S-E12.3) — [[client-surfaces]].
- **Step content, voice, and template drafting** — [[drafting]] and [[voice-profile]].
- **Inbound capture and thread-matching mechanics** — the capture subsystem emits
  the reply event; this chapter only consumes it.
- **Lead-to-contact promotion** — [[leads-and-qualification]].
- **Scoring on engagement signals, and any future open/click store** —
  [[lead-scoring]] (the deferred engagement-event table is theirs).
- **Deal-room view tracking** (disclosed, consent-gated) — [[deal-rooms]].
- **Migration import of sequence definitions, paused-on-arrival** —
  [[import-export-migration]] (AC-M12).
- **Recording-consent policy** (RC-7) and the transcript pipeline —
  [[meetings-and-transcripts]].
- **The approval inbox surface** — [[notifications-and-approval-inbox]].
- **Marketing-grade broadcast, A/B steps, send-time optimization, dedicated IPs,
  autonomous above-threshold sending** — deliberately out or fast-follow per the
  feature cut lines.

## Where it lives

Backend: the communications module (`backend/internal/modules/comms`) — the one
send subsystem the feature spec names, reached through the contract's sequence,
enrolment, and telephony operations and the governed send tools under passport
scopes; scheduling rides the platform job runner. Frontend: the sequences and
telephony features (`frontend/src/features/sequences`). Read next:
[[approvals-and-concurrency]] (the token every send waits on), [[drafting]] (where
step content comes from), [[gdpr-platform]] (the consent substrate under the
preference center), and [[event-bus]] (the reply event that stops a cadence).

## Appendix

### Parameters
Source: margince specs/spec/features/06-deliverability-and-migration.md#12-deliverability-mechanics @ 5a0b29c; margince specs/spec/product/30-screen-acceptance.md#sequenceshtml--sequence-builder--reply-tracking-implements-s-e155a-s-e155b @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| SEQDEL-PARAM-1 | `AUTO_SEND_THRESHOLD` | `20` recipients (default; per-user opted-in, adjustable) | The ADR-0026 amount threshold applied to sequence sends: a step fanning out to at most this many recipients may auto-send when the user opted in; above it the batch queues to the approval inbox (AC-sequences-3). |
| SEQDEL-PARAM-2 | `SEND_DAILY_CAP` | provider-derived (Gmail ≈ 2,000/day per workspace user; Graph throttled) | Model A pacing ceiling per connected account; sends spread within working-hours windows. The corpus pins the class of value (provider quota), not a product constant. |
| SEQDEL-PARAM-3 | `DOMAIN_HEALTH_RECHECK` | daily (and before a workspace's first send) | Cadence of the pre-send sending-domain health evaluation behind the green/amber/red badge (AC-D5). |
| SEQDEL-PARAM-4 | `WARMUP_RAMP_WINDOW` | ~2–4 weeks (Model B, fast-follow) | Staged volume ramp on a new relay sending domain/IP, enforced by the pacing engine when Model B lands (AC-D8). |

Registry note: the corpus §0 parameter registries name no numeric constants for
SEQDEL-PARAM-2..4 — the feature spec pins classes of values (provider quota, daily,
2–4 weeks); exact constants are implementation defaults to fix at ticket time.
SEQDEL-PARAM-1's default 20 is corpus-given on the sequences screen. Adjacent, not
ours: the bulk-ops 🟡 blast-radius threshold of 10 records is pinned with the
bulk-actions screen ([[access-and-admin]], AC-bulk-actions-6); the approval-token
TTLs are [[approvals-and-concurrency#APPR-PARAM-1]]..2.

### Formulas
Source: margince specs/spec/features/06-deliverability-and-migration.md#13-how-this-wires-into-sequences-and-the-agent-send-path @ 5a0b29c; margince specs/spec/features/10-operational-depth.md#5-outreach-engine-promotes-d43-reply-tracking-d45-sequences-d411-bulk-activity-d48-telephony @ 5a0b29c

**SEQDEL-FORM-1 — the send gate (deterministic, ordered; every transmit path).**
Inputs: a send intent (sender principal, recipients, purpose, optional approval
token) and the workspace state (suppression table, consent substrate, pacing
schedule).

```
function may_transmit(intent) -> decision:
  # 1. AUTHORITY — transport is strictly downstream of the 🟡 gate (AC-D2)
  if intent.principal is agent:
      if not valid_approval_token(intent):            # bound to tool+diff, single-use
          if sequence_step(intent)
             and batch_size(intent) <= AUTO_SEND_THRESHOLD   # SEQDEL-PARAM-1
             and user_opted_in(intent.sender):
              pass                                    # under-threshold opted-in auto-send
          else:
              return REQUIRES_APPROVAL                # 0 transport calls
  # a human's own direct action is itself the approval

  # 2. SUPPRESSION — one workspace table, every send, no exceptions (AC-D1)
  drop recipients where suppressed(recipient)          # any reason

  # 3. CONSENT — default-deny per purpose (GDPR-AC-1/2)
  drop recipients where not consent_granted(recipient, intent.purpose)
      -> surfaced as excluded with reason contact_suppressed / consent_not_granted

  # 4. PACING — schedule, never burst (AC-D6)
  schedule remaining within SEND_DAILY_CAP per account and working-hours windows

  return SCHEDULED(remaining, exclusions)              # exclusions visible, never silent
```

Output: a paced transmit schedule plus a visible exclusion list; every executed
transmit writes one audit entry (AC-D7). Tie-breaks: exclusion beats scheduling (a
suppressed recipient is dropped before pacing is computed); an expired mailbox token
holds the schedule rather than dropping it; a reply event arriving mid-cadence
pauses the enrolment before its next step fires.

Worked example (corpus-given, AC-sequences-3/-4): step 4 of "DACH Q3 Outbound" fans
out to 23 recipients; the user's threshold is the default 20. 23 > 20, so the step
does not auto-send — it queues as a reviewable batch of 23 in the approval inbox and
the step's footer reads queued. Had the user raised their opted-in threshold to 25,
the same step would auto-send, paced within the daily cap. One enrolled recipient
(hard-bounced earlier) is on the suppression table and receives nothing either way.

### Schema
Source: margince specs/spec/contract/data-model.md#12-deferred-tables @ 5a0b29c

Ownership verified against the data-model chapter's ownership index: the deferred
sequence tables are assigned to this chapter ([[data-model]] Schema — deferred
tables, row DM-DEF-1: `sequence`, `sequence_step`, `sequence_enrollment`, "owner on
arrival: sequences-and-deliverability"). The corpus names the tables for shape but
carries **no DDL** — per DM-DEF-1, DDL lands with the owner chapter, i.e. with this
chapter's tickets. The DDL below is therefore **net-new normative shape, provisional
until the A59 contract ratification** (the build backlog defers the sequence cluster
on exactly that decision); the ratifying ticket may adjust columns but the pinned
behaviors (threshold routing, exit-on-reply pause, suppression-checked enrolment)
are fixed by the Acceptance pins regardless.

**SEQDEL-DDL-1 — `sequence` (net-new; provisional pending A59).**

```sql
CREATE TABLE sequence (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name          text NOT NULL,
  status        text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','paused','archived')),
  owner_id      uuid NULL REFERENCES app_user(id) ON DELETE SET NULL,
  auto_send_threshold integer NULL,  -- per-sequence override of SEQDEL-PARAM-1; NULL = user/workspace default
  source        text NOT NULL,       -- provenance (GATE-CORE-3); 'import:<batch>' sequences arrive status='paused' (AC-M12, import-export-migration)
  captured_by   text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,
  UNIQUE (workspace_id, name)
);
```

**SEQDEL-DDL-2 — `sequence_step` (net-new; provisional pending A59).**

```sql
CREATE TABLE sequence_step (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  sequence_id   uuid NOT NULL REFERENCES sequence(id) ON DELETE CASCADE,
  position      integer NOT NULL,                     -- deterministic firing order
  kind          text NOT NULL CHECK (kind IN ('email','call_task')),
  delay_days    integer NOT NULL DEFAULT 0 CHECK (delay_days >= 0),
  template_ref  uuid NULL,                            -- drafting starting point; per-recipient body is drafted ([[drafting]])
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (sequence_id, position)
);
```

**SEQDEL-DDL-3 — `sequence_enrollment` (net-new; provisional pending A59).**

```sql
CREATE TABLE sequence_enrollment (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  sequence_id   uuid NOT NULL REFERENCES sequence(id) ON DELETE CASCADE,
  person_id     uuid NULL REFERENCES person(id) ON DELETE CASCADE,
  lead_id       uuid NULL REFERENCES lead(id)   ON DELETE CASCADE,
  status        text NOT NULL DEFAULT 'active'
                CHECK (status IN ('active','paused_reply','paused_bounce','completed','unenrolled')),
  current_step  integer NOT NULL DEFAULT 0,
  paused_reason text NULL,
  enrolled_by   text NOT NULL,                        -- provenance: 'human:<id>' | 'agent:<id>' | 'batch:<audit-id>'
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CHECK (num_nonnulls(person_id, lead_id) = 1)        -- enrol a person or a lead, never both
);
-- one live enrolment per contact per sequence
CREATE UNIQUE INDEX uq_seq_enrol_person ON sequence_enrollment (sequence_id, person_id)
  WHERE person_id IS NOT NULL AND status IN ('active','paused_reply','paused_bounce');
CREATE UNIQUE INDEX uq_seq_enrol_lead   ON sequence_enrollment (sequence_id, lead_id)
  WHERE lead_id   IS NOT NULL AND status IN ('active','paused_reply','paused_bounce');
CREATE INDEX idx_seq_enrol_due ON sequence_enrollment (workspace_id, sequence_id, status);
```

**SEQDEL-GAP-1 — the suppression table (behavior pinned; schema is a named gap,
D-H2 contract-extension item).** The feature spec is normative that suppression is
"a single workspace-scoped `suppression` table (hard bounce / complaint /
unsubscribe / manual) consulted on **every** send regardless of model", yet the
table appears neither in the corpus data-model DDL nor in the ownership index — an
unassigned schema gap. Pinned here as behavior, binding on the ticket that lands
the DDL: workspace-scoped; reason enumeration exactly `hard_bounce`, `complaint`,
`unsubscribe`, `manual`; membership is the RC-6 runtime surface (auto-added on
bounce/unsubscribe, manual add/remove); consulted on every send path (AC-D1);
distinct from [[gdpr-platform]]'s erasure suppression list (GDPR-DDL-1), which
guards re-capture of erased identities, not sends. The DDL must be minted by the
same contract extension that ratifies SEQDEL-DDL-1..3 (A59 lane).

Telephony adds no table: a call lands as an activity row (owned by
[[activities-and-timeline]]); recording bytes go to object storage under the
platform blob conventions.

### Wire
Source: margince specs/spec/contract/crm.yaml (deferred-stub comment block: `/sequences`, `/sequences/{id}/steps`, `/enrollments`, `/calls`, `/telephony/*`) @ 5a0b29c

**Honest contract-coverage finding (SEQDEL-GAP-2, D-H2 contract-extension item):**
at pin time the contract defines **no** sequence, enrolment, telephony,
unsubscribe, or preference-center operation. The sequences/telephony surface exists
in the contract only as the deferred-stub comment block ("Deferred areas — NOT
specified here — see data-model.md §12: sequences/cadences, telephony/click-to-call
… commented stubs at the bottom of paths so the shape is intentional but
unspecified"). The chapter therefore pins the promised surface by planned path +
behavior; operationIds do not yet exist and must be minted by a contract extension
(the A59 ratification) before any docs-cited operationId can resolve. Until then no
prose or ticket may cite a sequence operationId as if it existed. The one existing
operation this chapter binds to is the outbound email send: contract operation
`send_email` (tool tier **yellow**, requires the approval token; a send to a
recipient without proven consent for the purpose answers 409 `consent_not_granted`).

| ID | Element (planned path) | Behavior pinned |
|---|---|---|
| SEQDEL-WIRE-1 | `/sequences` | Sequence CRUD: list/get/create/update/archive; status lifecycle draft → active → paused → archived; imported definitions arrive `paused` (AC-M12, owned by [[import-export-migration]]). |
| SEQDEL-WIRE-2 | `/sequences/{id}/steps` | Ordered step CRUD (position, kind, delay); per-recipient drafting delegated to [[drafting]]. |
| SEQDEL-WIRE-3 | `/enrollments` | Enrol/un-enrol a person or lead. 🟡 for agent callers (sending enrolment is outbound, ACX.3); suppression + consent checked at enrol time — opted-out contact → blocked with reason `contact_suppressed` / `consent_not_granted`, RBAC-/Passport-independent (S-E11.9). Bulk enrol rides the shared bulk-ops batch ([[access-and-admin]]), which invokes this op per in-scope record. |
| SEQDEL-WIRE-4 | enrolment pause/resume | Human pause/resume of one enrolment; the reply-driven pause is system-side (consumes `engagement.reply`) and needs no wire call. |
| SEQDEL-WIRE-5 | `/calls`, `/telephony/*` | Click-to-call dial (🟡 for agent callers, AC5.2 — same gate class as `send_email`); completed call lands as a call activity with provenance; recording flag validated server-side against RC-7 (hard block, AC5.3). |
| SEQDEL-WIRE-6 | one-click unsubscribe endpoint | Public, unauthenticated, signed target of the RFC 8058 `List-Unsubscribe-Post` header; POST writes the suppression row and unenrolls from active sequences (AC-D3). |
| SEQDEL-WIRE-7 | preference-center read/save | Public signed-link surface (no login); reads per-purpose consent state, saves staged changes as append-only consent proof events with wording + timestamp ([[gdpr-platform]]); DE/EN. |

### Events
Source: margince specs/spec/contract/events.md#511-engagement--signals-e08-warm-room-e15-reply-tracking @ 5a0b29c; margince specs/spec/contract/events.md#5-the-catalog @ 5a0b29c

Event definitions live in the central catalog ([[event-bus]]) — cited here, never
redefined.

| ID | Event | Cite |
|---|---|---|
| `engagement.reply` | The honest reply signal: emitted by capture when an inbound message thread-matches a prior outbound; idempotent per reply. Consumed here for exit-on-reply pause, and by promotion ([[leads-and-qualification]]) and the warm room | [[event-bus]] catalog row `engagement.reply`; semantics [[event-bus#EVT-SEM-14]] |
| `consent.changed` | Per-purpose consent transition backed by the append-only consent proof log; consumed by this chapter's outbound suppression path | [[event-bus]] catalog row `consent.changed`; substrate [[gdpr-platform]] |
| `activity.captured` | The specific verb a landed call/sent mail activity emits | [[event-bus]] catalog row `activity.captured`; [[event-bus#EVT-SEM-2]] |
| `approval.requested` / `approval.decided` | The pair every 🟡 send/enrol/dial rides | [[event-bus#EVT-SEM-9]]; owned by [[approvals-and-concurrency]] / [[notifications-and-approval-inbox]] |

**SEQDEL-GAP-3 (D-H2 event-catalog extension item):** the catalog defines no
sequence-lifecycle events (enrolled / step-sent / paused / resumed / exited), no
suppression event, and no call-lifecycle event. The build backlog's deferral note
names "sequence events" as part of the A59 ratification; those IDs must be minted
in the central catalog with that contract extension — this chapter deliberately
does not invent them here.

### Tools
Source: margince specs/spec/features/02-capture-and-comms.md#feature-4--email-templates--sequences--cadences @ 5a0b29c; margince specs/spec/decisions/ADR-0026-per-tool-autonomy-tiers.md @ 5a0b29c

The governed tool registry is owned by [[intent-tools]]; pinned here are only the
tier declarations this chapter's operations carry.

| ID | Tool / verb | Tier | Note |
|---|---|---|---|
| SEQDEL-TOOL-1 | `send_email` | 🟡 confirm-first | Exists in the contract today; unconfirmed call → `requires_approval`, zero transport calls (AC4.2/AC-D2). |
| SEQDEL-TOOL-2 | sequence-enrol | 🟡 confirm-first | Named by the feature spec as the agent cadence verb; ships with SEQDEL-WIRE-3. Under-threshold opted-in auto-send applies to step sends, never to enrolment by an agent. |
| SEQDEL-TOOL-3 | dial / place-call | 🟡 confirm-first | Outbound, touches a customer — same gate class as `send_email` (AC5.2). |
| SEQDEL-TOOL-4 | `draft_email` | 🟢 | Cited for contrast — drafting is free, transmit is gated; owned by [[drafting]]. |

### Acceptance
Source: margince specs/spec/product/epics/E15-operational-depth.md#s-e155--sequences-reply-tracking--bulk-activity @ 5a0b29c; margince specs/spec/product/20-traceability.md @ 5a0b29c

**Owned stories** (primacy verified against the traceability register):

| ID | Story | Tier | Home |
|---|---|---|---|
| S-E15.5 | Sequences + reply tracking + bulk activity (parent; tickets generate from the atoms) | V1-Must | this chapter |
| S-E15.5a | Sequence engine: steps + delays, exit-on-reply, draft-or-🟡-send under the opted-in threshold, Voice-DNA drafts regenerable from latest signal | V1-Must | this chapter |
| S-E15.5b | Reply / engagement tracking — the honest reply signal that pauses the sequence and routes to promotion (not "opens", RT-PR-H2) | V1-Must | this chapter |
| S-E15.5c | Bulk activity: mass log / mass task / bulk enrol as one governed, audited, permission-bounded batch | V1-Must | this chapter (the bulk-ops engine + screen: [[access-and-admin]]) |
| S-E15.6 | Telephony: click-to-call + recording into the timeline | V1-Must | this chapter |
| S-E11.9 | Buyer-facing preference center & one-click unsubscribe — "ships **with** sequences (S-E15.5a)"; the compliance pair to promoting sequences to V1 (A49) | V1-Must | this chapter — **flagged**: the traceability register files it under epic E11, and the scope register's epic-level owning-chapters row for E11 does not list this chapter; its mechanics (RFC 8058 headers, suppression writes, un-overridable opt-out) are entirely this chapter's, so it is pinned here and the E11 row in [[scope]] needs the cross-listing |
| S-E12.3 | Start sequence / insert AI draft from inbox | V1-Must | **[[client-surfaces]]** — cited, not owned here |

**Feature acceptance criteria (verbatim from the feature spec).**
Source: margince specs/spec/features/02-capture-and-comms.md#feature-4--email-templates--sequences--cadences @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC4.1 | A template renders all merge tokens for a record with no unresolved `{{token}}` left in the output; missing-data tokens fail loudly (blocked send), not silently blank. *(test)* | Backend integration lane |
| AC4.2 | **Every** outbound send (human or agent) passes through the confirm-first gate: an unconfirmed `send_email` tool call returns `requires_approval` and sends nothing. *(contract test: tool tier = 🟡; test asserts no SMTP call without approval token)* | Contract conformance lane |
| AC4.3 | A contact on the suppression list is excluded from any send/enrollment. *(test: enroll suppressed contact → 0 sends)* | Backend integration lane |
| AC4.4 | Every send is audit-logged with template id, resolved body, sender authority, and (if agent-originated) the agent id + approval record. *(P12 audit test)* | Backend integration lane (audit-completeness gate) |
| AC4.5 | Bounce/unsubscribe auto-updates contact status and adds them to suppression. *(test)* | Backend integration lane |
| AC4.6 | **(user-observable)** The rep opens a draft reply/follow-up that is already personalized from the contact's captured history (recent activity, deal stage) rather than a blank composer or a raw template, and can edit it; nothing is sent until they explicitly approve the send. The user can always see they are approving, not auto-sending (S-E07.1). | Screen e2e lane (draft content: [[drafting]]) |

Source: margince specs/spec/features/02-capture-and-comms.md#feature-5--telephony-click-to-call-auto-log @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC5.1 | A click-to-call creates a `call` activity on completion with non-null duration, direction, and `source`/`captured_by`. *(integration test against provider sandbox)* | Backend integration lane |
| AC5.2 | Placing an outbound call via the agent path requires confirm-first; an unconfirmed dial places **no** call. *(contract test: tool tier 🟡)* | Contract conformance lane |
| AC5.3 | Where consent config disallows recording, no audio is stored. *(test: assert 0 S3 objects)* | Backend integration lane |
| AC5.4 | Logged call appears on the timeline within **60 s p95** of hangup. | Performance gate |

**Deliverability acceptance criteria (verbatim from the feature spec).**
Source: margince specs/spec/features/06-deliverability-and-migration.md#15-acceptance-criteria @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-D1 | `[MV]` **Suppression is honoured universally.** A recipient on the `suppression` table (any reason) yields **zero** transmit calls for any send path (manual, sequence, agent-approved). *(test: seed suppressed recipient across all three paths → assert 0 transport invocations)* — extends `features/02` AC4.3. | Backend integration lane |
| AC-D2 | `[MV]` **No send without approval.** The transport layer is never invoked for an agent-originated `send_email` without a recorded approval token; an unconfirmed call returns `requires_approval` and produces 0 transport calls. *(contract test, shared with `features/02` ACX.3 — asserted at the transport seam)* | Contract conformance lane |
| AC-D3 | `[MV]` **List-Unsubscribe present.** Every bulk/sequence message carries `List-Unsubscribe` and `List-Unsubscribe-Post` headers; an unsubscribe POST to the endpoint adds the recipient to `suppression` and unenrolls them from active sequences. *(integration test: send → assert headers → POST → assert suppressed + unenrolled)* | Backend integration lane |
| AC-D4 | `[MV]` **Hard bounce suppresses.** A simulated hard-bounce signal for a recipient adds them to `suppression` with reason `hard_bounce` and marks the contact status; a subsequent send to them is blocked. *(test, satisfies `features/02` AC4.5 transport side)* | Backend integration lane |
| AC-D5 | `[MV]` **Domain-health check is deterministic.** Given seeded DNS fixtures (SPF/DKIM/DMARC present/absent/misaligned), the pre-send check returns the expected green/amber/red verdict with the expected remediation code. *(test against DNS fixture set)* | Backend integration lane (fixture set) |
| AC-D6 | `[MV]` **Pacing caps respected.** Given an approved 200-recipient batch and a per-account daily cap of N, the pacing engine schedules at most N transmit calls in the first 24h window and the remainder after. *(test with a fake clock + fake transport asserting call counts per window)* | Backend integration lane |
| AC-D7 | `[MV]` **Send is audited.** Every transmit produces an `audit_log` entry with template id (if any), resolved body hash, sender authority (human/agent + approval id), recipient, model (A/B), and outcome. *(P12 audit-coverage test, extends `features/02` AC4.4)* | Backend integration lane (audit-completeness gate) |
| AC-D8 | **Warmup ramp (Model B, fast-follow gate).** On a new Model B sending domain, daily volume does not exceed the ramp schedule for the configured warmup window. *(operational test once Model B lands)* | Operational (fast-follow; not a V1 build gate) |
| AC-D9 | **Reputation monitoring (operational).** Complaint rate and hard-bounce rate are emitted per workspace to the ops dashboard; crossing a configured threshold raises an alert and (Model B) pauses sending. *(KPI/alert, not a build gate — labelled per F-10)* | Operational KPI/alert |
| AC-D-UX | **User-observable (Devin/Mor).** Because the default is to send from the user's own mailbox, a follow-up Sam approves lands in the recipient's existing thread and arrives in the inbox (not the spam folder) the same as if he'd typed it himself, and the reply comes back to his own inbox. Before the workspace's first send, an admin sees a clear green/amber/red sending-health badge with specific remediation text — not a raw DNS console — so a misconfigured domain is flagged in plain language before any mail goes out. *(observable acceptance; mechanics gated by AC-D5/AC-D7)* | Screen e2e lane (mechanics: AC-D5/AC-D7) |

**Outreach-engine acceptance criteria (verbatim from the promotion of record;
corpus bullets carry no IDs — minted here).**
Source: margince specs/spec/features/10-operational-depth.md#5-outreach-engine-promotes-d43-reply-tracking-d45-sequences-d411-bulk-activity-d48-telephony @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| SEQDEL-AC-1 | A sequence send obeys the 🟡 gate / amount-threshold (ADR-0026, A-thresholds): below threshold may auto-send if the user opted in; above stays draft+approve — asserted by a threshold test. | Backend integration lane (threshold test) |
| SEQDEL-AC-2 | An inbound reply detected against an enrolled contact **pauses** the sequence and triggers promotion (`features/01 §6`) — no further steps fire. | Backend integration lane (no-further-steps test; consumes `engagement.reply`) |
| SEQDEL-AC-3 | Recording honors the per-jurisdiction consent config (RC-7); a no-consent jurisdiction blocks recording, not just hides it. | Backend integration lane (consent-block test; hard block per B-E15.21b) |
| SEQDEL-AC-4 | Bulk enrol/log is a governed batch (one audit entry, RBAC-bounded) like §bulk-ops (`features/04 §8`). | Backend integration lane (batch-governance + privilege-escalation + batch-audit tests) |
| SEQDEL-AC-5 | **User-observable (Sam, S-E15.5/.6):** Sam enrols a set of leads in a voice-drafted sequence that stops the instant someone replies, logs a batch of calls at once, and dials from the record with the call captured automatically. | Screen e2e lane |
| SEQDEL-AC-6 | A **no-open-pixel test** asserts no tracking-pixel / covert-open mechanism exists — reply, not open, is the only engagement signal (the honest-signal decision, BACKLOG §I; RT-PR-H2). | Static check + backend integration lane (per B-E15.18; bus semantics [[event-bus#EVT-SEM-14]]) |

**Sequences screen acceptance criteria (verbatim; corpus IDs preserved).**
Source: margince specs/spec/product/30-screen-acceptance.md#sequenceshtml--sequence-builder--reply-tracking-implements-s-e155a-s-e155b @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-sequences-1 | Given the sequence "DACH Q3 Outbound — Packaging QA", When the screen loads, Then the header shows status "Active" plus a stat strip reading Enrolled 38, Replied · exited 6, 🟡 Queued for approval 2, and In cadence 31. | Screen e2e lane |
| AC-sequences-2 | Given the cadence-steps rail, When the user reads it, Then each step shows its type (Email/Call task), title, delay (Day 0, +3 days, +4 days, +5 days), and status — step 1/2 marked sent ("sent · 36 of 38" / "30 of 36"), step 3 a draft, step 4 over-threshold — and a "+ Add step" control is present below. | Screen e2e lane |
| AC-sequences-3 | Given the send-policy banner with the auto-send threshold slider (default 20), When the user drags it, Then the displayed value updates live and the echo line recomputes whether the next step's 23 recipients will "auto-send" (under threshold, green) or "queue for approval" (over threshold, amber), and step 4's footer/timing switch to match. | Screen e2e lane (threshold: SEQDEL-PARAM-1) |
| AC-sequences-4 | Given step 4 fans out to 23 recipients which is over the threshold, When the user clicks "Queue batch for approval", Then the footer changes to "queued · 23 in approval inbox", the step timing reads "in approval inbox", and the header's 🟡 Queued count increments. | Screen e2e lane (inbox surface: [[notifications-and-approval-inbox]]) |
| AC-sequences-5 | Given the draft step 3, When the user clicks "Regenerate from latest signal", Then a diff card appears explaining the re-derivation (added a Mannheim mention from a captured signal) with accept/dismiss controls; accepting rewrites the draft body and dismissing keeps the current draft. | Screen e2e lane (draft generation: [[drafting]]) |
| AC-sequences-6 | Given Anna Weber replied to step 1, When the user expands her "reply detected" row, Then it shows the cadence is paused for her, the verbatim reply quote with timestamp and "grounded" source, and a proposed Lead → contact promotion (S-E13.3) with confirm/decline controls; confirming relabels her as a contact. | Screen e2e lane (promotion: [[leads-and-qualification]]) |
| AC-sequences-7 | Given the enrolled-contacts table, When the user reviews it, Then each contact shows its current step, a state pill (In cadence / Replied / 🟡 Queued / Bounced), and signal text — and a note states the cadence pauses on a reply, not on an open. | Screen e2e lane |
| AC-sequences-8 | Given the right-rail Bulk activity panel, When the user clicks Enrol / Log / Task, Then the action stages as one governed, audited batch bounded by permissions, with partial failures reported rather than silently dropped. | Screen e2e lane (batch engine: [[access-and-admin]]) |

**Telephony screen acceptance criteria (verbatim; corpus IDs preserved).**
Source: margince specs/spec/product/30-screen-acceptance.md#telephonyhtml--click-to-call--recording-implements-s-e156 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-telephony-1 | Given the call header shows Anna Weber and four dialable numbers tagged by jurisdiction (DE direct/mobile, CH Mannheim site, US ops), When the screen first loads with the DE direct number active, Then the dialer reads "Ready to dial +49 621 4408 217", the consent banner states DE is a two-party consent jurisdiction, and the recording pill reads "Will record". | Screen e2e lane |
| AC-telephony-2 | Given a number is selected, When I click a different number chip (e.g. "US ops · +1 415 555 0182"), Then that chip becomes active, the dialed number updates, the jurisdiction/consent banner re-renders for that policy, and the recording default re-applies per policy (on for two-party DE/CH, off for blocked US). | Screen e2e lane |
| AC-telephony-3 | Given the active number is a two-party jurisdiction (DE/CH), When I click the recording toggle, Then it flips on/off and the recording pill text updates between "Will record" and "Recording off". | Screen e2e lane |
| AC-telephony-4 | Given the active number is California/US, When I attempt to turn on the recording toggle, Then the toggle is locked off, a toast explains recording is "refused at the bridge, not a UI choice," and the consent line states recording is blocked under config RC-7 enforced server-side — while the call itself can still be placed. | Screen e2e lane (policy: [[meetings-and-transcripts]] RC-7) |
| AC-telephony-5 | Given the dialer is idle, When I click "Call", Then the avatar shows a ringing pulse, the status reads "Ringing … ", a "Connecting…" / hang-up control set appears, and after a short delay the state becomes "Connected" with a running mm:ss timer and (when recording is on) a "recording — consent captured" status plus a blinking record pill. | Screen e2e lane |
| AC-telephony-6 | Given a call is live, When I click "End call", Then the timer hides, the status reads "Call ended · transcribing…", and after a brief delay a post-call card appears (and scrolls into view) staging the captured call as a proposed activity not yet on the timeline. | Screen e2e lane (transcript pipeline: [[meetings-and-transcripts]]) |
| AC-telephony-7 | Given the post-call staged card is shown, When I review a proposed follow-up, Then each carries a transcript quote with timestamp and confidence dot, and I can accept it (it turns green and is relabeled "typed by you") or dismiss it (it is removed) individually via the check/x pair. | Screen e2e lane |
| AC-telephony-8 | Given staged follow-ups exist, When I click "Accept all & add to timeline", Then the staged items persist, the dashed staging banner is replaced by a green "Added to the timeline" confirmation linking to Anna Weber and the deal with provenance captured_by=agent:telephony; or When I click "Dismiss", Then the staged card is hidden and a toast notes the call is kept as a bare log entry only. | Screen e2e lane |

**Preference-center screen acceptance criteria (verbatim; corpus IDs preserved).**
Source: margince specs/spec/product/30-screen-acceptance.md#preference-centerhtml--buyer-preference-center-implements-s-e119 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-preference-center-1 | Given the page is opened from the signed unsubscribe link, When it loads, Then a recognition banner shows the recipient's address (anna.weber@brandt-automotive.de), states "no password needed — we recognised this address", and names the contact/company/sender — without any login form. | Screen e2e lane |
| AC-preference-center-2 | Given the recipient arrived via List-Unsubscribe-Post one-click, When the page loads, Then a green "Done — you're off our marketing email" banner confirms marketing was already stopped immediately, the Marketing toggle reads not-subscribed, and an "Undo — keep receiving marketing" control is offered. | Screen e2e lane |
| AC-preference-center-3 | Given the four purpose rows, When they render, Then "Deal & service messages" shows an "always on" lock badge with a disabled toggle and copy that transactional messages are exempt from opt-out while a deal is live, while Marketing, Events, and Research each show an operable toggle and a state line (Subscribed ● / Not subscribed ○). | Screen e2e lane (purposes: [[gdpr-platform]] GDPR-SEED-1..4) |
| AC-preference-center-4 | Given any operable purpose toggle, When I flip it (or click "Unsubscribe from all marketing", which turns Marketing/Events/Research off), Then the change is staged not persisted, and a sticky "Not saved yet" bar appears listing the pending changes (e.g. "Marketing → on") with Discard and Save preferences actions. | Screen e2e lane |
| AC-preference-center-5 | Given staged changes in the save bar, When I click "Save preferences", Then each changed purpose prepends a new entry to the consent record showing the exact wording shown, an opted-in/opted-out verdict, and a current timestamp "via preference center", the save bar clears, and a toast confirms "applied instantly across every send". | Screen e2e lane (proof log: [[gdpr-platform]]) |
| AC-preference-center-6 | Given the locked transactional toggle, When I click it, Then it does not change state and a toast explains "Deal & service messages are required while your deal is live — they can't be switched off here". | Screen e2e lane |
| AC-preference-center-7 | Given Marketing is opted out (on load or after Save), When the page settles, Then an "enforced, not advisory" notice is shown stating no rep or AI agent acting for one can re-enrol the address, and that an enrolment attempt is blocked with reason `contact_suppressed`; the notice is hidden whenever Marketing is subscribed. | Screen e2e lane + backend integration lane (the un-overridable block, both human and MCP enrol paths per B-E11.32) |
| AC-preference-center-8 | Given the consent record panel, When it renders, Then it lists prior consent events newest-first, each with the verbatim wording quote, the purpose, the verdict, and a timestamped source (e.g. "via booking page"), described as the same audit trail a data-protection officer can request. | Screen e2e lane |
| AC-preference-center-9 | Given the Deutsch/English language toggle, When I switch language, Then all banner, purpose, state-line, save-bar, and footer copy re-render in the chosen language (default Deutsch), and the recorded consent wording uses the active language. | Screen e2e lane |

**Cited, not owned here** (each is another chapter's pin; a sanctioned restatement
carries the owner's ID):

| ID | Fact | Owner |
|---|---|---|
| ACX.3 | Confirm-first invariant: no outbound action (email send, call dial, sending sequence enrollment) executes via the agent/MCP path without a recorded human approval token — enforced at the tool-contract tier | [[approvals-and-concurrency]] / [[intent-tools]]; floor form GATE-AI-7 ([[acceptance-standards]]) |
| AC-M12 | Sequences imported paused: imported HubSpot sequence definitions are created in a paused state with 0 auto-enrollments and 0 sends until a human re-enrolls | [[import-export-migration]] (features/06 §2.11) |
| AC-bulk-actions-6 / AC-bulk-actions-9 | Over-threshold bulk ops (reassign/archive/enrol > 10 records) queue 🟡; the enrol panel drops opt-out contacts citing `consent_event` and S-E11.9, un-overridable | [[access-and-admin]] (bulk-actions screen, primary story S-E11.7) |
| GDPR-AC-1 / GDPR-AC-2 | Consent default-deny per purpose; cross-purpose isolation — the substrate every send-purpose check here rides | [[gdpr-platform]] |
| R5c pixel guard | No per-recipient open-pixel path ships (static check); deal-room view tracking is disclosed and consent-gated | [[deal-rooms]] (features/08 §5b) |

**Open build decisions (carried honestly — the build tickets must resolve them).**
Source: margince specs/spec/product/30-screen-acceptance.md#sequenceshtml--sequence-builder--reply-tracking-implements-s-e155a-s-e155b @ 5a0b29c (States & edge cases, "Missing in prototype")

| ID | Decision needed | Verification |
|---|---|---|
| SEQDEL-AC-OPEN-1 | The sequences prototype wires no loading/skeleton state for live data and no empty state for a sequence with zero enrolled contacts; both must be designed at ticket time to meet the [[acceptance-standards]] screen-state floor. | Ticket-gate: the sequences screen ticket must state both states before build |
| SEQDEL-AC-OPEN-2 | The telephony prototype surfaces no error state for a failed/declined call or a dropped provider bridge (provider-disconnected and no-answer states undesigned). | Ticket-gate: the telephony screen ticket must state the failure states before build |
| SEQDEL-AC-OPEN-3 | The preference-center prototype has no expired/invalid signed-link or unrecognised-address state, no save-failure path, and no loading state — for a public, unauthenticated compliance surface the failure states are load-bearing. | Ticket-gate: the preference-center ticket must state the failure states before build |
