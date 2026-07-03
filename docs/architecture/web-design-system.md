---
module: frontend/src/shared
derives-from:
  - margince-poc/docs/architecture/web-design-system.md @ a11d6c08
---
# Web design system — Ledger-Green token layer

> The visual foundation every UI surface is built on: Margince's own Ledger-Green
> palette, light-first and dark-ready, with typography and color locked behind CI gates
> — plus the reuse-as-is component vocabulary, the app shell, and the AI-native trust
> primitives that keep unsourced AI value off the screen.
>
> This chapter owns the design-system trust primitives, the canonical navigation order,
> and the brand-layering model. The FE component layer model (FE layer 1–4 — note that
> scale is distinct from the Go-DAG Tier 0–3) and the token/utility rules (the FE-DS-*
> pins, including the font families) are owned by the [frontend](frontend.md) chapter
> and cited from there, not restated here.

## What it's for

One brand foundation so every screen looks like Margince and cannot regress past CI.
The design system supplies structure, atoms, and a tested dark-mode cascade; Margince's
own palette and typography are pinned on top. Above the tokens sit three layers: a
reuse-as-is component library (the everyday UI vocabulary), the persistent app shell
every authenticated screen renders inside, and the trust primitives that make
AI-derived values visible, sourced, and reviewable.

## Principles it serves

- **P8 — AI you can trust.** No unsourced AI value ever reaches the UI —
  evidence-or-omit, confidence as a first-class glyph, staging before persistence, and
  an Accept/Edit/Dismiss review gate on every AI suggestion (WDS-TRUST-1..4).
- Brand integrity and accessibility by construction — color and font rules are enforced
  by gates, not convention, so the look can't drift (ADR-0040 sets the palette;
  ADR-0026 the autonomy-tier glyphs).

## How it works

**Layer, don't fork.** A thin brand-override stylesheet redefines the base custom
properties by import order — it must load *after* the base imports so Margince's values
win the cascade (WDS-GATE-4). The vendored design-system packages stay untouched, so
design-system updates still apply cleanly. Dark mode is purely a token override: no
component branches on theme. The override only re-asserts brand tokens in dark and
delegates every surface/text/border neutral to the tested dark cascade, so no dark
color is invented or stranded. A theme toggle sets both the dark data attribute and the
dark class together, flipping surfaces and re-asserting the brand in one switch.

**Component layers.** UI is built strictly bottom-up: design-system atoms (CRM-agnostic)
→ CRM domain components (props-only, know CRM types, no fetching) → feature components
(state, hooks, data) → page/layout (routing only). The numbered FE layer model
(FE-LAYER-1..4) and its placement decision tree are pinned in the
[frontend](frontend.md) chapter — this chapter doesn't restate them.

**The reuse-as-is library.** The everyday vocabulary is reused straight from the design
system, not re-skinned: a broad set of atoms (buttons, badges, inputs, modals,
tooltips, toasts, dropdowns, and the like) plus a small set of composed primitives the
base set doesn't ship — search field, user avatar, toast container, context menu,
drawer panel, data table. A single sanctioned component owns the autonomy-tier dots
(auto-approved vs needs-approval; token-bound, semantic not decorative); these are the
only emoji glyphs allowed anywhere in the UI (WDS-GATE-3).

**The app shell.** The persistent chrome every authenticated screen renders inside: a
slim ink-green left rail with the canonical, ordered nav (Home, Contacts, Companies,
Leads, Deals, Tasks, Inbox, Reports, Ask AI — WDS-NAV-1), the Margin-rule "M" mark to
home and a user avatar to settings, count badges, and at most one active item per
screen; plus a contextual top bar whose right-side actions are empty at cold start. All
chrome icons resolve to Lucide. Two surfaces (the client-facing surfaces and
onboarding) are the documented rail-less exceptions.

**The trust primitives** (the P8 guarantee that AI value is always sourced). A concept
group, each member presentational and prop-driven (the backing data layer is separate):

- **Evidence-or-omit** (WDS-TRUST-1) — an AI-derived value carries an evidence chip: a
  source icon plus a confidence indicator that expands to a verbatim snippet and source
  link. No evidence means no value is shown — nothing is fabricated.
- **Confidence-as-glyph** (WDS-TRUST-2) — confidence is a first-class styled dot (never
  an emoji), token-colored high/med/low. Low confidence is shown *as* low, never
  silently hidden.
- **Staging-before-persist** (WDS-TRUST-3) — a staged value is visibly "not yet real"
  (tinted, ghosted border), and staged / real / human-typed are three distinguishable
  styles.
- **Accept / Edit / Dismiss** (WDS-TRUST-4) — the universal review triad on any AI
  suggestion. Edit flips the value to human-typed provenance ("typed by you") while
  retaining the original snippet.
- **Provenance** (WDS-TRUST-5) — a quiet tag distinguishes agent-authored from
  human-authored values.
- **AI-assisted disclosure** (WDS-TRUST-6) — any surface that renders generative AI
  output (an AI-drafted message, a generated summary) carries a visible "AI-assisted"
  disclosure, satisfying the EU AI Act Article-50 transparency obligation. This is a
  hard requirement, not a nicety: a generative surface rendered without it is a defect,
  and the drafts subsystem treats a missing disclosure as a failing test — see the
  drafts chapter.

These compose into named product surfaces: the **Morning Brief** queue item, the
**Pipeline Board** with its **Deal Cards** (AI-derived fields surfaced through evidence
+ confidence), and the **Record View** with a provenance-stamped timeline.

## What's configurable

- **Theme** — light (default) or dark, switched by a single token-level toggle; no
  per-component theming.
- **Typography** — exactly three locked font families; generic fallbacks are allowed
  and any fourth family fails the build. The families themselves are pinned in the
  frontend chapter's token rules ([[frontend#FE-DS-17]]) — this chapter cites the pin
  rather than restating it.

## Guarantees (enforced)

- **Brand pin** — the Ledger-Green palette is the only sanctioned brand-color source;
  no other brand hues can appear. Token tests fail on any hex drift in light or dark
  (part of WDS-GATE-1).
- **Color purity** (WDS-GATE-1) — no raw color literals, no hard-coded pixel font
  sizes, no unmapped utility palettes; only the brand stylesheet may carry literal
  brand hex.
- **Font lock** (WDS-GATE-2) — only the three locked families pass under app source.
- **Cascade order** (WDS-GATE-4) — the brand override is pinned to load after the base
  design-system imports, so the brand can't silently lose the cascade, and the dark
  block can't re-declare surfaces the base set owns.
- **Icon purity** (WDS-GATE-3) — every chrome icon is a Lucide glyph; no
  emoji/pictographic glyph is used as chrome anywhere except the one sanctioned
  autonomy-dot home. (Lesson: Lucide names must be PascalCase, or the icon map misses
  them and renders an empty-box fallback.)
- **Trust distinction carries through** — a cross-layer test asserts the staged / real /
  human-typed distinction (WDS-TRUST-3) survives into the composed surfaces.

All gates run inside the aggregate frontend check, which folds into the repo-wide
aggregate gate. (Note: the frontend lint/format step is part of that aggregate — run
the *full* check before declaring a frontend branch green; partial subsets have let
unformatted files redden an integrated default branch.)

## Acceptance

"Done" for this layer means: every authenticated screen renders inside the shell with
the canonical rail order (WDS-NAV-1) and at most one active item; every AI-derived
value on screen satisfies the six trust primitives (WDS-TRUST-1..6), with the
enforcing release gates owned by the acceptance-standards chapter; and the four drift
gates (WDS-GATE-1..4) are green. The testable form is pinned in the appendix; the
cross-cutting screen-state floor is inherited from the acceptance-standards chapter
and not restated.

## Out of scope

This layer does not include the command palette, the real product screens behind the
nav targets, the report/query-plan compiler, the trust primitives' backing data +
intent-tool layer, or i18n extraction — those are built separately. The token-prefix
rules and semantic utility catalog live in the [frontend](frontend.md) chapter
(FE-DS-1..17).

## Where it lives

The `frontend/` edge — tokens and the theme entry in the application package's single
stylesheet entry point, the reuse-as-is library and trust primitives in the shared kit
under `frontend/src/shared/`, and the app shell in `frontend/src/app/`. Read the
[frontend](frontend.md) chapter for the layer model and data layer these components
plug into.

## Appendix

### Parameters — navigation
Source: margince-poc/docs/architecture/web-design-system.md#how-it-works @ a11d6c08

**WDS-NAV-1 — canonical rail order.** The app-shell left rail renders exactly these
items, in exactly this order, with at most one active item per screen:

1. Home
2. Contacts
3. Companies
4. Leads
5. Deals
6. Tasks
7. Inbox
8. Reports
9. Ask AI

### Acceptance — trust primitives
Source: margince-poc/docs/architecture/web-design-system.md#how-it-works @ a11d6c08

The six trust primitives. Each is presentational and prop-driven; the release gates
that enforce them at the AI boundary are owned by the acceptance-standards chapter and
cited per row.

| ID | Primitive | Requirement | Enforcement |
|---|---|---|---|
| WDS-TRUST-1 | Evidence-or-omit | Every AI-derived value carries an evidence chip (source icon + confidence indicator) expanding to a verbatim snippet and source link; no evidence → no value shown, nothing fabricated | [[acceptance-standards#GATE-AI-1]] |
| WDS-TRUST-2 | Confidence-as-glyph | Confidence is a first-class styled dot (never an emoji), token-colored high/med/low; low confidence is shown *as* low, never silently hidden | [[acceptance-standards#GATE-AI-1]] (the confidence half) |
| WDS-TRUST-3 | Staging-before-persist | A staged value is visibly "not yet real" (tinted, ghosted border); staged / real / human-typed are three distinguishable styles, and a cross-layer test asserts the distinction survives into composed surfaces | [[acceptance-standards#GATE-AI-2]] + the cross-layer trust-distinction test |
| WDS-TRUST-4 | Accept / Edit / Dismiss | The universal review triad on any AI suggestion; Edit flips the value to human-typed provenance ("typed by you") while retaining the original snippet | [[acceptance-standards#GATE-AI-2]] (accept-to-persist) |
| WDS-TRUST-5 | Provenance tag | A quiet tag distinguishes agent-authored from human-authored values everywhere both can appear | cross-layer trust-distinction test |
| WDS-TRUST-6 | AI-assisted disclosure | Any surface rendering generative AI output carries a visible "AI-assisted" disclosure (EU AI Act Art. 50); a generative surface without it is a defect and a failing test in the drafts subsystem | [[acceptance-standards#GATE-AI-9]] |

### Acceptance — enforced gates
Source: margince-poc/docs/architecture/web-design-system.md#guarantees-enforced @ a11d6c08

The design-system drift gates. All run inside the aggregate frontend check and block
merge; registry rows are cited where the gate registry carries them.

| ID | Gate | What it holds | Registry |
|---|---|---|---|
| WDS-GATE-1 | `ds-purity` | No raw hex / `rgba` / `hsl` / `oklch`, no hard-coded pixel font sizes, no unmapped Tailwind palettes; only the brand stylesheet may carry literal brand hex; token tests fail on any brand hex drift in light or dark (the brand pin) | [[quality-gates#QG-19]] |
| WDS-GATE-2 | `font-lock` | Only the three locked font families pass under app source (families pinned at [[frontend#FE-DS-17]]) | [[quality-gates#QG-20]] |
| WDS-GATE-3 | `icon-lint` | Every chrome icon is a Lucide glyph; no emoji/pictographic glyph as chrome except the one sanctioned autonomy-dot component (ADR-0026); Lucide names PascalCase or the icon map falls back to an empty box | [[quality-gates#QG-21]] |
| WDS-GATE-4 | cascade order | The brand-override stylesheet loads after the base design-system imports (the brand can't silently lose the cascade); the dark block never re-declares surfaces the base set owns | no gate-registry row yet — held by the token/import-order tests inside the aggregate frontend gate <!-- reconcile: the poc enforces cascade order via token tests, not a named CI gate; quality-gates.md carries no QG row for it. Flagged rather than invented. --> |
