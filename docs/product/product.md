---
derives-from:
  - margince-poc/docs/product.md (frozen)
  - margince specs/spec/narrative/01-prfaq.md#press-release
---
# What Margince is

A plain, no-jargon overview of the product: what it is, who it's for, the problem it
solves, and the bets it makes. No architecture here — for how it's built, see the
architecture chapter; for the rules behind decisions, see principles; for what ships
and what explicitly does not, see scope.

## In one paragraph

Margince is a **CRM built for the age of AI assistants, delivered as source the
customer holds.** It's a place to keep your customers, deals, and conversations —
like HubSpot or Salesforce — but with two differences that shape everything: it
**fills itself in from your email, calendar, and calls** instead of making people
type, and it's **built so an AI assistant can do real work in it safely** (read,
draft, and — with approval — act), whether that's your own AI tool or the one built
in.

## Who it's for

- **Small and mid-sized teams** who want a fast, clean CRM that doesn't need an admin
  to configure it.
- **Regulated or data-sensitive businesses** who need to keep their data (and even
  the AI) on their own servers.
- **Larger companies** who already run Salesforce or HubSpot and won't rip it out —
  Margince can run *on top of* what they have (PROD-BET-6).

## The problem we're attacking

- **Manual data entry kills CRMs.** Most CRM data is stale or missing because someone
  has to type it in. A record a person had to type is a failure, not a feature.
- **The big CRMs are bloated and lock you in.** Endless configuration screens few
  people use, data that's hard to get back out, and proprietary formats. We take the
  opposite stance: opinionated and simple, your data is yours, open formats, easy
  export.
- **AI assistants are arriving, and CRMs weren't built for them.** People
  increasingly work through an AI assistant, yet the incumbents bolt AI onto a data
  model never designed for it — a sidebar that summarizes what you already typed. A
  CRM should let that assistant help — safely and with a clear record of what it did.
  We don't resell intelligence; we put the intelligence you already have to work
  where your data lives.

## What makes it different (the bets)

- **It captures instead of asking you to type** (PROD-BET-1). Email, calendar, calls,
  and signals flow in automatically.
- **Your agents do the work, inside the system** (PROD-BET-2). Bring your own
  assistant (Claude, Cursor, Copilot, …) or use the built-in one. The assistant can
  read and draft freely; anything that sends or changes data is **held for your
  approval**, and every action is recorded. There is no AI seat to buy and no credit
  meter waiting to run out.
- **Your data is yours, and so are your customizations** (PROD-BET-3). Open formats,
  a documented data model, one-click export — leave in an afternoon and take
  everything. Customizations are real source code the customer owns, on a
  source-available core that converts to full open source on a fixed two-year clock
  (PROD-BET-3) — never a vendor-held config blob that dies when you stop paying.
- **It's simple on purpose** (PROD-BET-4). One good way to do each thing, not a
  thousand settings. When a customer genuinely needs something custom, it's built as
  real software — by them, a partner, or Gradion — not bolted on through a config
  screen. No config ceiling, no waiting on a vendor roadmap.
- **It's trustworthy by design** (PROD-BET-5). Who did what, when, and why is
  recorded; sensitive actions pause for a human; and sensitive actions can be
  reversed. This is also how it stays on the right side of GDPR and the EU AI Act.
- **Works with your existing CRM** (PROD-BET-6). If you already run Salesforce or
  HubSpot, Margince can sit on top of it — adding the AI and the nicer interface —
  while your existing system stays the system of record. Adopt it fully later if you
  want.

## Source-available on purpose

The product literally doesn't work unless customers get the source to customize:
customization is **real software development**, not a runtime config engine (see
principles P2 and P14). That is also why the quality of the codebase itself is
treated as a product feature, not just internal hygiene — and why the release
license guarantees the customer can run and fork the version they hold.

## What we deliberately do not build

The other half of the product definition is the refusal list: no runtime
custom-object builders, no no-code workflow UIs, no per-AI-seat pricing or credit
meters, no covert profiling. The binding inventory is the scope chapter's never list
([[scope#NEVER-1]]–[[scope#NEVER-12]]) — cited here, pinned there.

## Where to go next

The decision rules behind everything live in the principles chapter; the cut-lines in
scope; the five people the product is judged by in personas; the end-to-end proof in
journeys.

## Appendix

### Parameters — the bets
Source: margince-poc/docs/product.md @ a11d6c08; narrative/01-prfaq.md#press-release @ 5a0b29c

| ID | Bet | One line |
|---|---|---|
| PROD-BET-1 | Zero-entry capture | The system fills itself from email, calendar, calls, and signals; a record a person had to type is a failure. |
| PROD-BET-2 | Agent-native, approval-gated | BYO agent or built-in AI; read and draft are free, anything that sends or changes data is held for approval, every action recorded; no AI seat, no credit meter. |
| PROD-BET-3 | Own your data, own your customizations | Open formats, documented schema, one-click export; customizations are real source the customer owns on a source-available (BUSL-1.1) core, each release converting to Apache-2.0 two years after it ships. |
| PROD-BET-4 | Opinionated over configurable | One good way per thing; genuine custom needs are built as real software (customer, partner, or Gradion), never through a config engine. |
| PROD-BET-5 | Trustworthy by design | Attribution, human approval for sensitive actions, reversibility — the same posture that satisfies GDPR and the EU AI Act. |
| PROD-BET-6 | Overlay before replace | Runs on top of an existing Salesforce/HubSpot, which stays system of record; full adoption is optional and later. |
