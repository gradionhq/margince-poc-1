---
derives-from:
  - specs/spec/product/10-journeys.md
---
# Journeys — the demo script and the dogfood probe in one

Four end-to-end journeys stitch individual stories into the lived experience: the
order a real person moves through the product, and where the WOW lands. They are
**normative twice over** — each is the demo script for its persona's pitch, and each
is a dogfood probe the built product must actually carry a user through. A journey
names its personas and traverses an ordered sequence of story IDs; the sequences are
pinned verbatim in the appendix (J1–J4), and the failure rule (JOURNEY-RULE-1) makes
a broken journey a spec defect, not a demo caveat.

**J1 — First-run cold-start** (Devin, with Mor behind him): the "it read my business
back to me" moment. Devin pastes his company website URL and the CRM narrates his
business back — every field showing the source it was read from, ungrounded fields
omitted, never guessed. He connects his mailbox and watches the system visibly fill
itself; he connects the Claude agent he already pays for, bounded from the first
moment by the default roles Mor accepted — and meets no per-AI-seat tax and no credit
counter. The wow is partly an absence: no paywall at the magic moment.

**J2 — A week in the life of Sam** (Sam, plus the overnight agent under Sam's
Passport): nothing dropped. A morning brief assembled overnight; a dossier waiting
before each meeting; calls auto-captured into activities with provenance; the
inferred next step asking "correct?"; every outward send queued for one-tap approval
with the draft shown; every action attributable. By Friday no follow-up was dropped
and Sam never opened a "please update the CRM" form.

**J3 — A deal from signal to close** (Sam, Pat, Riya): a warm-room signal surfaces a
buying-intent account — consent-gated, company-level, never covert. Sam runs warm,
contextual outreach that Pat experiences as relevance, not spray; the qualifying
call auto-captures; the agent proposes the deal from what was actually said, for Sam
to confirm; advancing to closed-won is a confirm-first, audited action; and Riya's
pipeline view reflects the new deal on the clean relational core, with "explain this
number" giving her the derivation on demand.

**J4 — Devin customizes via source PR** (Devin, with Mor watching the audit trail):
real custom development on source Devin owns. His own coding agent, guided by the
in-repo conventions and generator scaffolding, opens a reviewable pull request
against his fork; the checks run green before merge is offered; Devin reviews,
merges, deploys, and watches his expansion-risk rule flag real deals — in code he
owns. Months later an upstream release lands and the upgrade is itself an
agent-assisted engineering task guarded by the test suite: the customization
survives the bump.

## Appendix

### Acceptance — journeys
Source: product/10-journeys.md @ 5a0b29c

| ID | Journey | Personas | Ordered story sequence |
|---|---|---|---|
| J1 | First-run cold-start ("it read my business back to me") | Devin; Mor (governance behind him) | S-E01.1 → S-E01.2 → S-E01.4 (real accounts/leads surfaced per S-E01.3, nothing persisted until accepted) → S-E02.1 (with S-E02.2, S-E02.3, S-E02.4) → S-E10.1 → S-E11.1 → S-E10.5 |
| J2 | A week in the life of Sam (nothing dropped) | Sam; the Overnight/Brief agent under Sam's Passport | S-E05.1 (with S-E05.2) → S-E04.1 → S-E04.2 → S-E04.3 → S-E11.2 → S-E11.3 |
| J3 | A deal from signal to close | Sam; Pat (external); Riya (forecast end) | S-E08.1 → S-E07.1 → S-E04.2 → S-E03.5 → S-E11.2 → S-E09.1 (V1 demo rides S-E09.1 + S-E03.4; S-E09.3 is Fast-follow) → S-E09.2 |
| J4 | Devin customizes via source PR (PR → green → merged → survives upgrade) | Devin; Mor (audit-trail review) | S-E10.3 → S-E10.1 (prereq: connected agent under Passport) → S-E11.3 → S-E10.3 (upgrade arc) → S-E11.4 |

**JOURNEY-RULE-1 — the failure rule.** If the product cannot carry a user through
each of J1–J4 without **manual data entry**, a **credit wall**, or an
**un-attributable change**, the spec is wrong. A broken journey is a tracked spec
defect, never a demo caveat.

Note JOURNEYS-N-1: J1's third beat cites S-E01.3 (ICP-matched account surfacing) as
the source of surfaced candidates; that story is Fast-follow per the scope OUT list
([[scope#S-E01.3]]). The V1 run of J1 exercises the S-E01.4 accept-to-persist gate
over the candidates cold-start and capture produce; the sequence above is preserved
verbatim from the corpus.
