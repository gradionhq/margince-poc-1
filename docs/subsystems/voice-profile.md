---
status: planned
module: modules/voice (backend) · web (voice management surface)
derives-from:
  - margince specs/spec/features/07-ai-native-moments.md#7-voice-dna
  - margince specs/spec/features/09-onboarding-and-voice-tasks.md#b-voice-dna-fd-14--v1-wow
  - margince specs/spec/product/epics/E07-voice-and-drafting.md#s-e072--voice-dna-the-draft-sounds-like-me
  - margince specs/spec/contract/data-model.md#voice--drafting-e07--features07-features09
  - margince specs/spec/product/30-screen-acceptance.md#voicehtml--voice-profile-management-implements-s-e0712
  - margince-poc/docs/subsystems/voice.md @ a11d6c08
---
# Voice profile — drafts that sound like you, learned only from your own words

> The per-rep Voice DNA: a versioned, inspectable style artifact derived from a
> multi-source corpus of the user's *own* writing, which [[drafting]] consumes to write in
> the rep's register. Style transfers, judgment does not: the profile can never send,
> never advance a deal, regardless of confidence — enforced at the tool layer, not in copy.

## What it's for

A generic AI draft reads like a robot, so the rep rewrites it and the drafting feature
loses its point. The voice profile fixes that: it turns a corpus of the rep's own writing
and speech — posts, long-form, sent emails, chat, and especially speaker-filtered call
transcripts — into a quantified, user-owned Voice Profile the drafter generates in, so a
draft arrives recognizably closer to "how I write" than the generic baseline. Its callers
are [[drafting]] (the only consumer of the profile at generation time), the onboarding
wizard's build-your-voice step (owned by the onboarding chapter, which seeds the corpus
before capture is live), the capture pipeline (which feeds sent mail and transcripts into
continuous learning), and the voice management screen where the rep inspects, edits,
rebuilds, and rolls back their profile. Style transfer into drafts, the send gate, and the
LinkedIn channel are all [[drafting]]'s; this chapter owns the artifact, the corpus, and
the lifecycle.

## Principles it serves

- **P7 — own your data.** The profile is learned from the user's own corpus through the
  workspace's own model (ADR-0020) — inspectable and user-owned, pointing at the user's
  writing, never a black box trained on other people.
- **P12 — governance designed in.** The profile is versioned with append-only history;
  excluded sources are recorded with reasons, never silently dropped; and no amount of
  learned judgment ever raises a draft past the confirm-first send gate
  ([[acceptance-standards#GATE-AI-7]], the always-🟡 floor at [[threat-model#D4]]).
- **P5 — capture-first.** After the onboarding seed, the corpus grows automatically from
  every sent email and captured call — no re-training step the user must trigger.
- **FD-14 (provenance).** Promoted from Fast-follow to V1-WOW because the founder's
  proven prior-product corpus-ingest and profile-builder pipeline is reused, not rebuilt —
  the build risk that deferred it is retired.

## How it works

**Three artifacts, one strict human/machine split (decided).** The model mirrors the
founder's proven system. First, a *knowledge base* of domain facts and positioning — in
this product that role is played by the business profile from onboarding's website
read-back, so no separate knowledge-base authoring surface exists in V1. Second, a
**human-authored identity document**: a short free-text statement of who the rep is,
edited only as a **full-document replacement** (no field-level merge) and never touched by
the machine. Third, the **machine-derived style document**: a structured write-up of the
rep's voice — an identity narrative, a stats snapshot, signature moves, structural and
opening/closing patterns, punctuation and point-of-view rules, vocabulary, forbidden
anti-patterns, register notes, and few-shot examples by format — regenerated on every
rebuild and never hand-edited. Each rebuild bumps a version. Because the identity document
is a separate artifact outside the regenerated one, the human's words survive every
rebuild and rollback by construction.

**The corpus and its manifest.** Sources are ingested by kind and register: spoken
(call and meeting transcripts — highest-signal, plus voice memos), written (posts,
long-form, sent emails), and casual chat. Sent email is the primary source — the most
on-domain corpus for the email drafts the product generates. Every ingested source is
recorded in a manifest with its kind, register, weight, label, and word count; a source a
guard rejects is kept in the manifest marked excluded with its reason, never silently
discarded, so the corpus is auditable. A live word-count and register-mix meter tracks
progress toward the corpus target (~30,000 words, VOICE-PARAM-1) and yields a quality band
(VOICE-PARAM-3).

**Two privacy guards run before anything is modeled.** Transcripts are
**speaker-filtered to the user's own turns only** — a both-sided transcript contributes
zero words of the other party, a privacy and quality invariant, not a best effort. And a
personal-mail guard classifies non-work mail out of the email source; flagged mail is
recorded as excluded and never modeled.

**The sequencing arc.** Onboarding builds a *starter* voice from sources the user can
provide without granting inbox access — genuinely useful, labelled a starter, typically
landing mid-band. The moment the mailbox is connected (deliberately the last onboarding
step), the sent-email corpus is ingested and the profile jumps a band — the value pull for
the connect step. Thereafter the profile is self-improving: new sent mail and captured
transcripts feed the corpus continuously, with a weekly "what changed" delta the rep can
inspect.

**Rebuild is explicit, versioned, and cost-gated.** Adding or removing a source never
triggers a model call by itself: a rebuild costs real money each time (VOICE-PARAM-2), so
it runs only behind an explicit rebuild action. A rebuild recomputes the derived style
document from all non-excluded sources, writes an append-only version snapshot, and bumps
the current version — leaving the human identity document untouched. **Rollback** restores
a prior style document byte for byte, but as a new forward-moving version: history is
append-only, a rollback adds to the past rather than rewriting it. The management surface
warns before commit when removing a source or rebuilding could lower the band.

**Learning from judgment, honestly scoped.** The rep's accepted, edited, and rejected
drafts are signal — this phrasing was kept, that one rejected — and later drafts reflect
it; the model learns from the proud/rejected distinction, not raw sent volume. The corpus
half of that loop (new sends and transcripts flowing in) is defined; **where edited and
rejected drafts feed back, and what the rep sees of it, has no defined surface yet** — the
feature decomposition flags that interface as undefined, and this chapter carries it as an
open decision (VOICE-AC-OPEN-1) rather than pretending it is designed. The weekly
tone-drift "apply?" nudge — an observation framed as a question, never applied silently —
remains post-V1 polish with its surface likewise undecided (VOICE-AC-OPEN-3).

**What the profile can never do.** The profile personalizes drafts; it holds no send
capability. Even when the model has learned the rep's judgment patterns — when they push,
concede, or go quiet — that judgment surfaces only as a draft the rep reviews. Voice DNA
never raises a draft to auto-send, never advances a deal, never sends on the rep's behalf,
regardless of confidence. Enforcement is at the tool layer, not in copy: a voice draft
cannot reach the send operation without an approval token
([[acceptance-standards#GATE-AI-7]]; the non-configurable always-🟡 floor is
[[threat-model#D4]]), voice drafts route through the same approval inbox as any 🟡 action
([[approvals-and-concurrency]]), and every draft carries the voice model version that
produced it, so attribution is traceable. The corresponding eval pins any auto-send path
from Voice DNA as an instant fail ([[ai-evals]] AIUC-13).

**The management surface.** The voice screen renders the profile as something the rep
owns: the band and quantified stats hero, the editable identity, the derived style
sections, sample drafts (each marked draft-only), the corpus source list with registers
and word counts, the register mix, and the rebuild control. Editing the identity opens a
free-text editor that saves as a full-document replacement, marked typed-by-you and kept
on rebuild.

## What's configurable

- **The model client** — the builder derives the profile through the workspace's
  customer-supplied inference (ADR-0020); inference stays on the customer's model.
- **Source weights and registers** — each corpus source carries a weight and register tag
  that shape the mix; the rep can add, remove, or exclude sources at any time.
- **The personal-mail guard** — an injectable classifier on the email source; production
  heuristics in place, deterministic in tests.
- **Profile scope** — the schema admits user, team, and workspace scopes (VOICE-DDL-1);
  every V1 story and screen is per-user, so team/workspace profiles are a latent schema
  capability, not a V1 surface.

## Guarantees (enforced)

- **Only the user's own words are modeled.** Transcripts contribute the user's turns and
  nothing else — a both-sided transcript adds zero other-party words (test-pinned); and
  personal mail is excluded before it can be modeled (VOICE-AC-6, AC-voice-13).
- **The profile can never send.** No send capability exists on the voice path; a voice
  draft leaves only through [[drafting]]'s 🟡 send gate, and a red-team attempt to
  auto-send a voice draft fails closed (VOICE-AC-8; GATE-AI-7 / [[threat-model#D4]]).
- **Rebuild only on explicit command, cost-gated.** Adding a source never silently
  triggers a paid model call (VOICE-PARAM-2); proven by a call-counting test.
- **The identity document survives.** The builder is forbidden to write the human-authored
  identity document; rebuild and rollback touch only the derived style document
  (VOICE-AC-7).
- **Versioned, append-only, rollback-able.** Every rebuild writes a version snapshot and
  bumps the version; rollback restores a prior snapshot byte for byte as a new forward
  version — history is never rewritten (VOICE-AC-7, AC-voice-12).
- **Inspectable and editable.** The rep can see what the profile learned, which corpus
  sources it points at, and why a draft sounds the way it does; the identity is theirs to
  edit as a full-document replacement (VOICE-AC-5, AC-voice-4).
- **Auditable exclusions.** A guard-rejected source is recorded with its reason in the
  manifest, never silently dropped.
- **Tenant isolation.** Profiles, corpus sources, and version history are
  workspace-scoped and row-level isolated (VOICE-DDL-1/2).

## Acceptance

Done means: a rep who feeds the product their own writing gets drafts in their register —
their greeting, sentence length, sign-off, and bluntness — and can open the voice screen
to see exactly what was learned, from which sources, at what quality band, and edit or
rebuild it deliberately. Nothing the profile learns ever sends anything: the rep can look
for a send path from the voice surface and find none. The honest states matter here: a
fresh profile shows a building/thin band rather than faking sharpness (the prototype's
missing cold-start and below-target states are the build ticket's to close,
VOICE-AC-OPEN-2), and a source removal warns about band impact before commit. The
cross-cutting screen-state floor is inherited from [[acceptance-standards]] (STATE-1..5)
and not restated; the voice-draft eval band and its auto-send instant-fail live in
[[ai-evals]] (AIUC-13, riding AIEVAL-12/13). The testable form of every claim is pinned in
the Acceptance appendix.

## Out of scope

- **Voice-matched draft generation, style transfer, and every send/gate mechanic** —
  [[drafting]] (which re-asserts the never-auto-send rule on its side).
- **The LinkedIn draft-only channel** — [[drafting]] (S-E07.4).
- **The onboarding wizard step that seeds the corpus** and the business profile that plays
  the knowledge-base role — the onboarding chapter.
- **The mailbox and transcript connectors** that deliver sent email and call transcripts —
  the capture chapter; this chapter consumes their output as corpus sources.
- **The approval inbox and token mechanics** voice drafts ride — [[approvals-and-concurrency]].
- **The tone-drift "apply?" nudge** — post-V1 polish; its surface is an open decision
  carried here (VOICE-AC-OPEN-3) so it is not lost.

## Where it lives

The backend voice module (`modules/voice`) with its corpus-ingest pipeline, consumed by
[[drafting]] through a narrow profile seam; the web surface is the voice management
screen. Read next: [[drafting]] (the consumer), [[approvals-and-concurrency]] (the gate
voice drafts ride), and [[ai-evals]] (the voice-draft quality band).

## Appendix

### Parameters
Source: margince specs/spec/features/09-onboarding-and-voice-tasks.md#b-voice-dna-fd-14--v1-wow @ 5a0b29c; margince specs/spec/features/07-ai-native-moments.md#7-voice-dna @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| VOICE-PARAM-1 | Corpus target | `~30,000` words (a few hundred samples) | The aspirational corpus size the word-count meter measures against; the 30k fixture reaches the top band (VOICE-AC-7). Source constant, no runtime tuning. |
| VOICE-PARAM-2 | Rebuild cost gate | `~$0.30` per rebuild → explicit action only | A rebuild recomputes the derived style document and costs real money; it is gated behind an explicit "Rebuild" action and never auto-runs on source add (features/09 B2.3). |
| VOICE-PARAM-3 | Corpus quality bands | `thin / good / rich / sharp` | The ingest meter's band vocabulary over word count + register mix (features/09 B1.4). Registry note (honest): features/07 §7 lists the band arc as "thin → good → rich" while B1.4 and the voice screen add "sharp" — resolved here to the four-band B1.4 scale; flagged for corpus sync. The builder's cold-start maturity scale is a deliberately distinct vocabulary (see the skeleton's precedent @ a11d6c08) whose V1 names are part of VOICE-AC-OPEN-2. |

The cross-cutting AI budgets and screen-state floor are [[acceptance-standards]]'s; the
draft-quality eval thresholds are [[ai-evals]]'s (AIEVAL-12/13) — cited, not owned here.

### Schema
Source: margince specs/spec/contract/data-model.md#voice--drafting-e07--features07-features09 @ 5a0b29c

Ownership verified against the data-model chapter's ownership index: `voice_profile` and
`voice_corpus_source` are assigned to this chapter ([[data-model]] Schema — ownership
index); `drafting_asset` is [[drafting]]'s.

**VOICE-DDL-1 — the `voice_profile` table (verbatim).**

```sql
CREATE TABLE voice_profile (                              -- a user's/team's "voice DNA"
  -- + base columns + version
  owner_id      uuid NULL REFERENCES app_user(id),        -- null = workspace/team profile
  scope         text NOT NULL DEFAULT 'user' CHECK (scope IN ('user','team','workspace')),
  model_ref     text NULL,                                -- derived style descriptor / embedding ref
  status        text NOT NULL DEFAULT 'building' CHECK (status IN ('building','ready','stale'))
);
```

**VOICE-DDL-2 — the `voice_corpus_source` table (verbatim).**

```sql
CREATE TABLE voice_corpus_source (                        -- consented samples the profile was built from
  -- + base columns
  voice_profile_id uuid NOT NULL REFERENCES voice_profile(id),
  sample_kind   text NOT NULL CHECK (sample_kind IN ('sent_email','note','upload')),
  sample_ref    text NOT NULL,                            -- activity id / attachment ref
  excluded      boolean NOT NULL DEFAULT false            -- user can exclude a sample
);
```

Note VOICE-DDL-N-1 (reconcile at ticket time — the corpus DDL predates the decided §B
model): features/09 §B **decides** a three-artifact model — a human-authored
`personality_md` (full-document replacement, untouched by rebuild), a machine-derived,
**versioned** `voice_profile_md` with append-only version snapshots and rollback, and a
corpus-source manifest carrying `{kind: post|transcript|email|chat|longform|voice_memo,
register: spoken|written|casual|formal, weight: 0.1–5.0, source_label}` plus word count
and an exclusion reason. The pinned DDL above cannot express that: `voice_profile` has
only `model_ref` + a `status` whose vocabulary also differs from the band scales
(VOICE-PARAM-3), there is no version-snapshot table, no identity-document column, and
`sample_kind IN ('sent_email','note','upload')` is narrower than §B's six kinds with
register/weight/label. The data-model contract must be brought up to the §B decisions when
the build ticket lands; §B is the decided source of truth for the shape, the DDL above is
the current corpus baseline.

### Wire
Source: margince specs/spec/contract/crm.yaml @ 5a0b29c; margince specs/spec/contract/data-model.md#voice--drafting-e07--features07-features09 @ 5a0b29c

Honest report: **no voice operations exist in the contract at the pinned corpus version**
— `crm.yaml` @ 5a0b29c defines no `operationId` for profile read, identity edit
(full-document replacement), corpus-source add/remove/exclude, rebuild, or rollback. The
data-model contract-surface note declares `voice_profile` gets a first-class REST+MCP
surface; the skeleton shipped the lifecycle domain-only without a contract surface
(@ a11d6c08). Note VOICE-WIRE-N-1: the voice surface's operations must be added to the
contract at ticket time; this chapter owns them when they land. Whatever lands must keep
the tool tiering: every voice operation is profile-scoped and 🟢-at-most (internal,
reversible via versioning); no voice operation may ever be or invoke a send —
[[drafting]]'s send operation (its DRAFT-WIRE-2) is the only outbound door and stays 🟡.

### Events
Source: margince specs/spec/contract/events.md#5-the-catalog @ 5a0b29c

Honest report: the central catalog defines **no voice events** @ 5a0b29c — no
profile-rebuilt, version, or corpus-source events exist. Continuous learning consumes the
capture stream (catalog rows `activity.captured` / `capture.normalized`, definitions at
[[event-bus]]) as its input signal. Note VOICE-EVT-N-1: when the voice wire surface lands
(VOICE-WIRE-N-1), its mutations (identity edit, source add/exclude, rebuild, rollback)
fall under the one-mutation-one-audit-one-event rule ([[event-bus#EVT-SEM-1]]) and need
catalog rows added catalog-first with the contract work.

### Acceptance
Source: margince specs/spec/product/epics/E07-voice-and-drafting.md#s-e072--voice-dna-the-draft-sounds-like-me @ 5a0b29c; margince specs/spec/product/20-traceability.md @ 5a0b29c

**Owned stories** (primacy verified against the traceability register; the epic-to-chapter
split in [[scope]] assigns E07 to [[drafting]] plus this chapter):

| ID | Story | Tier | Home |
|---|---|---|---|
| S-E07.1 | Baseline AI drafts the reply | V1-Must | **[[drafting]]** — cited, not owned here |
| S-E07.2 | Voice DNA: the draft sounds like *me* | V1-WOW | this chapter (style transfer at draft time: [[drafting]]) |
| S-E07.3 | Governed asset library | V1-WOW | **[[drafting]]** |
| S-E07.4 | LinkedIn draft in my voice | V1-WOW | **[[drafting]]** |

**S-E07.2 user-side acceptance (Given/When/Then, verbatim from the epic; new IDs — the
source bullets are unnumbered).**

| ID | Given/When/Then | Verification |
|---|---|---|
| VOICE-AC-1 | Given Sam has accepted, edited, and rejected drafts over time, when Sam asks for a draft, then it reads in Sam's register — Sam's typical greeting, sentence length, sign-off and bluntness — recognizably closer to "how I write" than the generic baseline draft. | Eval band (voice-vs-baseline closeness on a fixed set — [[ai-evals]] AIUC-13, KPI band not hard gate) |
| VOICE-AC-2 | Given Sam edits a draft before sending, when Sam sends it, then those edits become *signal* (this phrasing was rejected, that one kept) and later drafts reflect it — the model learns from the proud/rejected distinction, not just raw sent volume. | Backend integration lane (signal recorded); surfacing UI: VOICE-AC-OPEN-1 |
| VOICE-AC-3 | Given Sam's tone has shifted in a deal segment, when the CRM notices, then it surfaces an **observation framed as a question** — e.g. "your tone got shorter in enterprise deals — apply that to enterprise drafts?" — that Sam can accept, decline, or ignore; it is never applied silently. | Post-V1 `[TS]` polish (epic: remains polish on top of the V1 voice model); surface: VOICE-AC-OPEN-3 |
| VOICE-AC-4 | Given the voice model has learned Sam's *judgment* patterns (when Sam tends to push, concede, or go quiet), when a draft is produced, then that judgment still surfaces only as a **draft Sam reviews** — Voice DNA **never** raises a draft to auto-send, never advances a deal, never sends on Sam's behalf, regardless of confidence. | Backend integration lane + red-team fixture (fails closed; GATE-AI-7 / [[threat-model#D4]]; [[ai-evals]] AIUC-13 instant-fail) |
| VOICE-AC-5 | Given Sam wants to see why a draft sounds the way it does, when Sam asks, then the CRM can point to the kind of past messages the voice was learned from (Sam's own corpus), so the personalization is inspectable and Sam-owned, not a black box trained on others. | Screen e2e lane (corpus provenance renders — AC-voice-1/9) |

**Task-level exit gates (verbatim from the feature decomposition).**
Source: margince specs/spec/features/09-onboarding-and-voice-tasks.md#b-voice-dna-fd-14--v1-wow @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| VOICE-AC-6 | (B1) Each source ingests with correct word-count + register tagging; transcripts are speaker-filtered to the user; the meter and band reflect the real corpus. | Backend integration lane (both-sided-transcript fixture → 0 other-party words) |
| VOICE-AC-7 | (B2) A 30k-word corpus yields a "sharp"-band profile; a 4k seed yields a coherent "building" profile; rebuild versions and preserves the human-edited identity. | Backend integration lane (fixed corpus fixtures; VOICE-PARAM-1/3) |
| VOICE-AC-8 | (B3) A voice draft can never be sent or advance a deal without passing the approval inbox; a red-team attempt to auto-send a voice draft fails closed. | Backend integration lane + red-team fixture (tool-layer gate; [[approvals-and-concurrency]]) |
| VOICE-AC-9 | (B4) The corpus grows from capture without a manual trigger; "see what changed" shows real week-over-week deltas. | Backend integration lane (capture-fed growth, no rebuild call — VOICE-PARAM-2) + screen lane (AC-voice-3) |
| VOICE-AC-10 | (B5) The management surface round-trips every Voice Profile field; rebuild + source edits behave per B2/B4 with honest band-change warnings. | Screen e2e lane |
| VOICE-AC-11 | (B0) The ported pipeline produces a Voice Profile artifact from a sample corpus, persisted under the schema, with a version. | Backend integration lane (the FD-14 port gate — do first) |

**Voice screen acceptance criteria (verbatim; corpus IDs preserved).** The screen→story
index tags this screen to S-E07.1/.2; the surface is the voice management screen and is
owned here — its sample-draft rows exercise [[drafting]]'s baseline path, cited not owned.
Source: margince specs/spec/product/30-screen-acceptance.md#voicehtml--voice-profile-management-implements-s-e0712 @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| AC-voice-1 | Given the screen loads, When the status hero renders, Then it shows a quality band ("Sharp"), the title, a "your corpus · model vN" provenance tag, and copy asserting it is built from the user's own words and "points at your corpus, not a model trained on other people." | Screen e2e lane |
| AC-voice-2 | Given the hero stats, When they render, Then they show words-in-corpus, samples, sources, and quality band — the quantified Voice Profile. | Screen e2e lane |
| AC-voice-3 | Given the "Learning continuously" bar, When it renders, Then it shows the weekly delta ("+N words this week from M sent emails & K calls"), states there is no re-training step to trigger, and offers "see what changed →" surfacing what shifted. | Screen e2e lane |
| AC-voice-4 | Given the Identity card, When it renders, Then it shows a prose voice description and an "Edit identity" control; editing saves as typed-by-you and is "kept on rebuild". | Screen e2e lane (full-document replacement — features/09 B5.2, decided) |
| AC-voice-5 | Given the Stats-snapshot card, When it renders, Then it shows quantified style metrics (sentence length, questions, exclaims, em-dash rate, etc.) "per 100 words where rated" and a "Top words" list with counts. | Screen e2e lane |
| AC-voice-6 | Given the Signature moves and Anti-patterns cards, When they render, Then signature moves list positive patterns (checks) and anti-patterns list forbidden patterns (x), e.g. "Never em-dashes." | Screen e2e lane |
| AC-voice-7 | Given the "Sample drafts in your voice" card, When it renders, Then it shows ≥2 live example drafts each with a "🟡 draft only" pill and "every real draft lands draft-only, never sent automatically." | Screen e2e lane |
| AC-voice-8 | Given the never-send gate note, When it renders, Then it states drafts are always yours to approve; Voice DNA style-transfers but never sends/advances a deal regardless of confidence; the 🟡 send gate is inherited, never relaxed. | Screen e2e lane |
| AC-voice-9 | Given the Corpus sources section, When it renders, Then it lists all sources each with icon, label, register tag (spoken/written/casual), word count, and a hint; sent emails marked "primary," transcripts carry a "highest-signal" star, connected/growing sources show badges. | Screen e2e lane |
| AC-voice-10 | Given the register-mix card, When it renders, Then it shows a proportional bar + key for spoken/written/casual. | Screen e2e lane |
| AC-voice-11 | Given a corpus source row, When I use its add/remove icons or "Add a source", Then a toast indicates the action (add via upload/paste, or remove + rebuild to apply); the profile is versioned. | Screen e2e lane |
| AC-voice-12 | Given I click "Rebuild profile," When it runs, Then the button shows a disabled "Rebuilding…" state, then restores and fires a toast "rebuilt from N words · model v{n+1} · still draft-only" (version increments; never-send preserved). | Screen e2e lane |
| AC-voice-13 | Given the spoken-source hint, When it renders, Then it states transcripts are "speaker-filtered to your turns only — we never model the other side," and "a handful of transcripts beats 30 posts for cadence." | Screen e2e lane |

The standard screen-state matrix (empty / loading / error / no-permission /
nothing-grounded) is inherited from [[acceptance-standards]] (STATE-1..5) and not
restated.

**Open build decisions (carried honestly — the build tickets must resolve them).**
Source: margince specs/spec/features/09-onboarding-and-voice-tasks.md#b-voice-dna-fd-14--v1-wow @ 5a0b29c; margince specs/spec/product/30-screen-acceptance.md#voicehtml--voice-profile-management-implements-s-e0712 @ 5a0b29c

| ID | Decision needed | Verification |
|---|---|---|
| VOICE-AC-OPEN-1 | **The edited/rejected-draft feedback loop has no UI** (features/09 B4.2): S-E07.2 asserts edits become signal (VOICE-AC-2), but where that signal feeds back and how it is surfaced to the rep is undefined. The ticket must define the capture point and its surface. | Ticket-gate: the continuous-learning ticket must state the design before build |
| VOICE-AC-OPEN-2 | **Cold-start / below-target states are missing** (features/09 B2.2; the screen's flagged gaps): what the profile and screen look like right after the onboarding seed and below the ~30k target — the "Building"/"Thin" band names, which features gate until enough corpus exists, and the no-permission state. Must compose with the inherited STATE-1 honest-empty floor. | Ticket-gate: the voice-screen ticket must define the states before build |
| VOICE-AC-OPEN-3 | **The tone-drift "apply?" nudge surface** (features/09 B4.3; VOICE-AC-3): explicitly post-V1 `[TS]` polish, but its surface must be parked deliberately so the epic's third acceptance bullet is not silently lost. | Ticket-gate: recorded against the post-V1 backlog, not a V1 blocker |
