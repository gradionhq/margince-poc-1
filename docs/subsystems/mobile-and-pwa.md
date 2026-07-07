---
status: planned
module: web (mobile is a rendering mode of the shared frontend shell — no dedicated module)
derives-from:
  - specs/spec/features/04-platform-and-compliance.md#2-mobile
  - specs/spec/product/30-screen-acceptance.md#3-cross-cutting-build-gaps-roll-up-of-the-open-questions
---
# Mobile & PWA — one responsive codebase; the phone is where approvals land

> The product's mobile story: every screen of the one responsive web app works
> on a phone, the approval inbox provably so, and there is no native app to
> maintain or fall behind. Its single hard V1 promise: a manager reviews an
> agent's staged action — full diff — and approves or rejects it from a phone.

## What it's for

Reps and managers live away from their desks; a CRM whose mobile story is a
thin, feature-starved native app is a documented incumbent pain point the
product deliberately beats by being responsive-first and fast instead. The
full CRM must be usable on a phone — view and search records, read the
timeline, log a call or note, advance a deal, action a task, and above all
respond to an approval — served by the same codebase and design system as the
desktop. This chapter owns that doctrine's product commitments and cut lines,
nothing more: it has **no dedicated stories, tables, or screens**. The one V1
mobile commitment is the mobile facet of the approval-inbox story (S-E11.2),
owned by [notifications-and-approval-inbox](notifications-and-approval-inbox.md);
mobile is a *mode of every screen*, not a screen.

## Principles it serves

- **P8 — beautiful, responsive.** Mobile quality comes from the one design
  system reflowing honestly at narrow widths, not from a second product.
- **P4 — fast where it matters.** The perceived speed budgets are not waived
  on mobile networks; they must hold on a throttled profile (MOBILE-PARAM-2).
- **P9 — shared foundations, one codebase.** One web app, one design system;
  a native fork is the rejected alternative, revisited only on proven demand.
- **P12 — governance designed in.** Mobile is the natural confirm-first
  surface: an agent queues an action, the human approves from their phone
  with the full preview — which is why the approval path is the hard V1 line.
- **D11.7 — native app stays OUT of v1.** The recorded cut: native
  iOS/Android is deferred and demand-gated, not assumed.

## How it works

**Responsive-first, as doctrine.** Every V1 screen renders usably at phone
width. The mechanics ride the frontend handbook, not this chapter: the shared
design system and layer model are what reflow, and the visual rubric's
responsive-integrity rule — nothing clips, overflows, or collapses at narrow
widths ([[frontend#V6]]) — is the per-component form of the same doctrine.
This chapter adds the product-level assertion: the *whole flow* (search, open
a record, log activity, advance a deal, approve an agent action) passes
end-to-end at the pinned phone viewport (MOBILE-PARAM-1: 390 px).

**The approval path is the hard commitment.** The corpus is explicit that the
desktop-only prototype leaves mobile unrepresented, names the approval
surface as the one thing S-E11.2 requires on mobile, and confirms it V1 as
responsive web at 390 px with native still out. A pending 🟡 item renders on
a phone with its full deciding context and diff, and approve/reject complete
from that session. Mobile is a *viewport, not a second approval model*: the
same disposition operations, the same audit trail, identical committed
payloads to the desktop path. The surface, its deciding-context contract, and
its honest states are owned by the inbox chapter (S-E11.2, NTFY-AC-9); this
chapter pins only the viewport-shaped acceptance around them.

**PWA, honestly tiered.** V1-must is responsive web working end-to-end plus
mobile approval. The PWA layer — installability, offline *read* of recently
viewed records, push via web-push where supported — is table-stakes
fast-follow, not V1-must, and its gate (Lighthouse installability) arms only
once PWA lands (MOBILE-AC-3). Offline is read-only by promise: the corpus
commits to offline-read of recently viewed records and nothing more; offline
*write* and conflict resolution are explicitly out of v1.

**Speed on mobile networks.** The perceived record-open budget is not
relaxed for mobile; it is re-asserted under a throttled Fast-3G profile in
the perf harness. The budget itself is single-homed in the
acceptance-standards floor ([[acceptance-standards#PERF-1]]); this chapter
pins the mobile measurement condition (MOBILE-PARAM-2).

## What's configurable

Nothing mobile-specific — deliberately. There is no mobile feature flag, no
separate mobile build or bundle, no per-device capability set, and no
runtime-config surface owned here. What varies at phone width is layout,
owned by the design system's responsive behavior; what an installed PWA adds
(offline-read cache, web-push) arrives as the fast-follow tier, not as a
knob. A chapter with no knobs is the point: one codebase, one behavior.

## Guarantees (enforced)

- **Every V1 screen is usable at phone width.** The core flows pass an
  end-to-end test at the 390 px viewport — held by the mobile e2e lane, not
  by per-component review alone (MOBILE-AC-1).
- **A manager can approve from a phone.** The 🟡 approval path — full
  rendered diff, approve/reject — completes from a mobile session against
  the same operations and audit trail as desktop (MOBILE-AC-4; surface
  contract owned by the inbox chapter).
- **Mobile never weakens governance.** Because mobile is a viewport of the
  one app, every RBAC, masking, confirm-first, and audit rule applies
  unchanged — there is no mobile API, so there is no mobile bypass (held
  structurally; the enforcement pins live with their owning chapters).
- **The speed budget holds on mobile networks.** Record open stays within
  the perceived budget on a throttled Fast-3G profile (MOBILE-AC-2, citing
  [[acceptance-standards#PERF-1]]).
- **Installability is gated when promised.** Once the PWA tier lands, the
  Lighthouse installability check is a real gate, not an aspiration
  (MOBILE-AC-3, fast-follow).

## Acceptance

Done means: someone on a phone can run the core of their day — find a
record, read its timeline, log a call, advance a deal — and when their agent
stages an outward action, they can open the item, read the same diff they
would see at a desk, and decide it, with the decision landing identically to
a desktop decision. Every screen's honest states (empty, loading, error,
no-permission, nothing-grounded) render at phone width exactly as the
standard floor requires — inherited from [[acceptance-standards]]
(STATE-1..5) and not restated; the inbox's special states are
[[acceptance-standards#STATE-SP-2]] and are owned by the inbox chapter. The
testable form of every claim here lives in the Acceptance appendix.

## Out of scope

- **Native iOS/Android apps** — explicitly OUT of v1 (D11.7; the corpus cut
  line defers native until demand proves it). No chapter owns a native app.
- **Offline write / conflict resolution** — out of v1 per the corpus cut
  line; offline is read-only of recently viewed records, fast-follow.
- **The approval surface itself** — deciding context, honest states,
  disposition semantics: [notifications-and-approval-inbox](notifications-and-approval-inbox.md)
  and [approvals-and-concurrency](approvals-and-concurrency.md).
- **Native mobile push transport** — already recorded as deferred by the
  inbox chapter (responsive web plus email covers MVP); web-push rides the
  PWA fast-follow tier here.
- **Voice / quick-capture dictation on the go** — the capture pipeline's
  concern ([capture](capture.md), [voice-profile](voice-profile.md)).
- **Responsive component mechanics** — token scales, layer model, the visual
  rubric: the frontend handbook owns them; this chapter only asserts the
  product-level outcome.

## Where it lives

Planned frontend home: the `web` shell — the same features, routes, and
design-system components as desktop, reflowed; there is no backend home
because there is no mobile API. Read next:
[notifications-and-approval-inbox](notifications-and-approval-inbox.md) for
the approval surface the mobile commitment cashes out on, the frontend
handbook for the responsive mechanics, and [[acceptance-standards]] for the
inherited screen-state floor and budgets.

## Appendix

### Parameters
Source: features/04-platform-and-compliance.md#2-mobile @ 5a0b29c; product/30-screen-acceptance.md#3-cross-cutting-build-gaps-roll-up-of-the-open-questions @ 5a0b29c (gap 8)

| ID | Name | Value | Meaning |
|---|---|---|---|
| MOBILE-PARAM-1 | Mobile acceptance viewport | 390 px | The phone width every mobile-path assertion runs at — the corpus's single pinned mobile breakpoint (gap 8: "responsive web at 390px", build story B-E11.6). Not a CSS breakpoint catalog; the design system owns reflow, this pins where acceptance is measured. |
| MOBILE-PARAM-2 | Mobile network profile | throttled Fast-3G | The perf-harness network profile under which the record-open perceived budget must still hold on mobile. The budget itself (< 300 ms perceived) is single-homed at [[acceptance-standards#PERF-1]]; this row pins only the mobile measurement condition. |
| MOBILE-GAP-1 | No distinct mobile-viewport perf budget | **unpinned** | The corpus nonfunctional PERF table has no mobile/390 px row; build story B-E11.6 maps the mobile approval interaction onto the existing budgets (item render → [[acceptance-standards#PERF-1]]; disposition round-trip → the save/mutation budget, [[acceptance-standards#PERF-4]]) measured at the 390 px viewport. If a distinct mobile budget is ever wanted, it must be ratified and pinned — not silently invented. |

### Acceptance
Source: features/04-platform-and-compliance.md#2-mobile @ 5a0b29c (§2 acceptance criteria, pinned verbatim); product/30-screen-acceptance.md#3-cross-cutting-build-gaps-roll-up-of-the-open-questions @ 5a0b29c (gap 8)

The corpus §2 criteria are unnumbered, so they take new chapter-scoped IDs;
the GWT cell is the corpus text verbatim. The mobile approval surface's own
contract is owned by the inbox chapter and cited, never re-pinned.

| ID | Given/When/Then | Verification |
|---|---|---|
| MOBILE-AC-1 | `[MV]` Core flows (search, open record, log activity, advance deal, approve agent action) pass an end-to-end test at a 390 px viewport. | Screen-acceptance e2e suite run at MOBILE-PARAM-1; live-stack lane ([[testing#TEST-LANE-3]]). |
| MOBILE-AC-2 | `[MV]` Record open p95 < 300 ms perceived on a throttled (Fast-3G) profile in the perf harness. | Perf harness under MOBILE-PARAM-2; budget owned by [[acceptance-standards#PERF-1]] — sanctioned restatement. |
| MOBILE-AC-3 | `[MV]` Lighthouse PWA installability passes once PWA lands (fast-follow gate). | Lighthouse CI check; armed at the PWA fast-follow, not a V1 merge-blocker. |
| MOBILE-AC-4 | A 🟡 agent action can be approved from a mobile session with the diff preview rendered. | Live-stack UAT ([[testing#TEST-LANE-3]]) at MOBILE-PARAM-1. The surface contract is [[notifications-and-approval-inbox]]'s (S-E11.2's "from a phone" clause; NTFY-AC-9: build story B-E11.6, 390 px responsive, same disposition operations as desktop). Confirmed V1 by the corpus gap note (30-screen-acceptance §3 gap 8: mobile approval surface required, responsive web at 390 px, prototype desktop-only, native stays OUT per D11.7). |
