---
module: frontend/src
derives-from:
  - margince-poc/docs/architecture/frontend.md @ a11d6c08
---
# Frontend — features mirror modules, the query cache owns server state

> The frontend is one pnpm workspace: an application package plus generated contract
> types. Components live in a four-layer model that keeps generic, domain, feature,
> and routing concerns apart; all server data flows through the query cache over a
> typed generated client; the design system supplies every token and atom; Storybook
> is the review surface where each component proves its states. This chapter covers
> the workspace layout, the layer model, the data-layer doctrine, design-system and
> brand layering, Storybook conventions, the dev proxy, and import rules.

## Workspace structure

The frontend is a pnpm workspace linking two packages: the application package and a
contracts package of TypeScript types generated from the backend's API contract. A
small make-driven command set covers the whole lifecycle — install, dev server,
lint, typecheck, format, unit and story tests, the Storybook dev server, contract
compliance tests — and one aggregate frontend gate runs lint, typecheck, tests, and
design-system purity together. Dependencies are added per package through the
workspace filter; a package that the workspace will not hoist is declared as a
direct dependency of the application package rather than reached transitively.

Application source is organized into four top-level homes:

- **frontend/src/app/** — the router, global providers, and the layout shell. This
  is the only place that knows the whole application exists.
- **frontend/src/features/<name>/** — one directory per feature, mirroring the
  backend modules (people, identity, deals, and so on). Each feature owns four
  subdirectories: its data-access hooks under api, its components, its shared
  in-feature hooks, and its routes.
- **frontend/src/shared/** — the cross-feature kit: the Forge design-system barrel
  of generic atoms, shared utilities, and shared hooks. Nothing here knows about any
  feature.
- **frontend/src/lib/api-client/** — the typed HTTP client generated from the
  backend's OpenAPI contract, the single wire entry point.

## The component layer model

Every component answers one question before it gets a home: *could another product
use this unmodified?* The answer places it on one of four layers (FE-LAYER-1..4).

Layer 1 is the design-system atom — buttons, avatars, badges, modals — imported from
the Forge barrel in the shared kit and never re-implemented locally. Layer 2 is the
domain component: it knows CRM types but is props-only and never fetches; it lives
in the components directory of the feature that owns it, and graduates to the shared
kit only when a second feature genuinely needs it. Layer 3 is the feature component:
it owns hooks and state, consuming the feature's own data-access hooks — this is
where app data enters the component tree. Layer 4 is the page and layout level: the
feature's routes directory plus the application shell, which know about routing and
nothing else.

Placement follows a fixed decision procedure, applied in order (FE-PLACE-1..6): use
an existing design-system atom when one covers the need; propose a variant upstream
rather than forking a local copy when an atom nearly covers it; send fully generic
new components upstream to the design system; give props-only domain components a
layer-2 home; give data-fetching components a layer-3 home next to their hooks; and
compose one-off pages at layer 4. The procedure's whole point is that reaching for a
lower-numbered answer first keeps the reusable layers honest.

## The data layer

The wire contract itself — status codes, the problem shape, the concurrency header,
the list envelope — is owned by the api-conventions chapter; this section is the
frontend consumption counterpart. The rule of thumb: **all server data flows through
the query cache over the generated client; the client store holds only ephemeral UI
and session state, never a second copy of server data.**

**One typed client.** The generated client under lib/api-client is the single HTTP
entry point, typed end to end against the contract, so path, parameters, body, and
response are all compile-time-checked. Hand-rolled fetches and string-built URLs are
banned. Cross-cutting request behavior belongs in client middleware, not call sites
— most importantly one global unauthorized handler that clears auth and redirects to
login, so a mid-session expiry triggers re-auth once instead of surfacing as a
generic error on every in-flight query. Auth and workspace headers are never set by
frontend code: development injects them at the proxy, production carries a session
cookie.

**One hook module per resource.** Data access lives in each feature's api directory
as hook modules built on TanStack Query — the query cache. Layer-3 components
consume hooks and never touch the client directly; that is what keeps the layer
model honest. Every hook types its error generic to the problem shape so callers can
branch on machine-readable code and status — an untyped error generic is a bug, not
a style choice. Query and mutation functions throw the wire error so the query cache
owns the error state, and they forward the abort signal the cache passes in so
superseded in-flight requests cancel. A mutation invalidates the query keys it
affects on success — read-your-writes — with no manual cache surgery without a
measured reason. Keys come from a per-resource key factory used by both the reading
query and the invalidating mutation; a key literal re-typed at an invalidation call
site is the silent-drift bug, because renaming the key then breaks invalidation with
no type error.

**Configured once at the top.** The query client is created once, with deliberate
defaults instead of the library's: a tuned staleness window (FE-PARAM-1), a retry
policy that retries only server errors and never a client error (FE-PARAM-2),
window-focus refetching off by default and opted into per query (FE-PARAM-3), and a
single global error sink where every query error is reported (FE-PARAM-4). Alongside
it, an app-level error boundary paired with the query cache's reset boundary wraps
the routed shell, so a render-time throw degrades to a recoverable error surface
instead of white-screening the application. A build that ships bare library defaults
or no boundary is not done.

**Server state versus client state.** Server data — people, deals, tasks, approvals
— lives in the query cache, cached and invalidated. The client store (Zustand) holds
only what the server does not own: the auth principal, selection, open dialogs,
unsaved filters. Copying server data into the client store is never correct; if it
came from the API, it lives in a query.

**Errors render the standard states.** Error handling branches on the machine code
and status of the problem shape, never by parsing human-readable detail, and a
shared code-to-message mapper keeps error copy consistent. Every data surface
renders the standard screen states pinned in the acceptance-standards chapter
(STATE-1..5) — a surface missing its empty or no-permission state is not done. Any
surface that renders generative AI output carries the AI-assisted disclosure, an
enforced trust primitive owned by the web-design-system chapter; a generative
surface without it is not done either.

**Optimistic concurrency.** A mutation on a versioned entity that two actors can
edit sends the entity's version with the request, per the api-conventions chapter.
On a version-skew rejection the frontend refetches and reconciles — it shows the
current server state and never silently re-applies a stale edit — and distinguishes
that from a semantic conflict such as an approval already decided.

**Live updates.** Single-actor freshness comes from the query cache itself:
invalidate-on-mutation plus the configured staleness window. When a screen must
reflect another actor's change live — a board, the approval inbox — the pattern is
server push over the event bus that invalidates the matching query keys; the stable
key factories are what make that a one-liner. Live updates are a deliberate shared
piece, never ad-hoc per-component polling or a bespoke socket.

**Composite reads.** A screen that spans entities — a deal 360, a company 360, the
brief — consumes one server-side composite read endpoint, a cached read model
invalidated via the event bus, as a single typed query owned by the feature that
needs it. The frontend never fans out N calls from a component to assemble what the
server can assemble once.

## The design system and brand layering

The design system contributes two things through the shared kit: the token layer —
all semantic CSS variables bridged into the utility framework — and the Forge barrel
of generic React atoms. One stylesheet entry point imports the utility framework and
the design-system variables; there is no second door.

Utility naming follows the prefix rule (FE-DS-1, FE-DS-2): design-system semantic
names keep the design-system prefix, framework-default names drop it. The full
semantic utility catalog — backgrounds, text, borders, spacing, the typography
scale, status colors, z-index bands, and motion durations — is pinned in the
appendix (FE-DS-3..10). The hard rules are absolute: no hardcoded hex, no hardcoded
pixel values, no raw framework palette for state colors, and never a dark-mode
override on a design-system primitive — primitives flip themes themselves
(FE-DS-11..14). A set of framework palettes is banned outright with mandated
substitutes (FE-DS-15). Typography is exactly three locked font families
(FE-DS-17), enforced by the font-lock gate. All of this is held by the
design-system-purity gate (FE-DS-16), which runs inside the aggregate frontend
gate. The design system's trust primitives and navigation order are owned by the
web-design-system chapter and cited from there, not restated here.

## Storybook doctrine

Storybook is the review surface — the thing a human or a reviewer agent looks at to
judge whether a component is well composed — and every exported story doubles as a
headless test, so stories must be deterministic (FE-DS-20). Story files sit next to
their components (FE-DS-21), titles follow the fixed two-family naming convention
(FE-DS-18), and every story declares its props as args with no setup code that
hides the component contract (FE-DS-19).

Presentation is centralized, not per-story: a global decorator renders every story
on the real page surface with a consistent gutter, and each story picks the right
surface mode — padded by default, centered for single small atoms so they read as
deliberate rather than stranded, fullscreen for full-bleed shells (FE-DS-22). A
component that fills its parent in the app is pinned to a representative width in
the catalog rather than left to stretch (FE-DS-23). Every component stories its
states — above all the empty state, designed to read as empty by design rather than
broken, plus loading, error, and overflow where they exist (FE-DS-24) — and every
story is checked in both themes (FE-DS-25). Where a mockup frame exists, the story
maps to it at the meta level so a design reviewer can render and compare
(FE-DS-26). The story catalog never hardcodes a background palette; surface
switching happens through the semantic utility classes (FE-DS-27).

The quality bar itself is the visual-craft rubric, V1–V8, pinned verbatim in the
appendix. It is the heuristic sibling of the deterministic drift gates: purity,
font-lock, and icon lint prove a component uses the right tokens, fonts, and icons,
but a token-pure component can still be visually unbalanced. The rubric is judged
by eye — advisory, applied by a reviewer against the mockup — and never blocks a
merge mechanically. The pre-submit self-check is the rubric in question form: does
each story frame cleanly, is every state covered, does the hierarchy read at a
glance, does it hold in dark mode, would a designer recognize the mockup?

One operational gotcha: restructuring the Storybook preview configuration can leave
a stale dependency cache that surfaces at story load as a misleading dynamic-import
fetch failure. Clear the Storybook cache and re-run — the failure is infrastructure,
not the stories.

## Dev proxy and auth in development

In development the dev server proxies API calls to the local backend and injects
the workspace and acting-user headers that match the development seed workspace.
Frontend code never sets those headers itself. In production a real session cookie
carries auth instead; the header injection exists only at the proxy seam.

## Import conventions

Local imports carry the explicit ESM extension, which TypeScript resolves. Generic
atoms are imported from the Forge barrel in the shared kit, never re-implemented.
Contract types are imported from the generated contracts package, never hand-typed.
These three rules are what keep the layer model and the contract-first doctrine
visible at every import site.

## Appendix

### Parameters — data layer
Source: margince-poc/docs/architecture/frontend.md#queryclient-configuration--the-error-boundary @ a11d6c08

| ID | Name | Value | Meaning |
|---|---|---|---|
| FE-PARAM-1 | staleTime | 30_000 ms | queries serve cached data for ~30 s before background refetch; tune per surface |
| FE-PARAM-2 | retry | `(n, err) => err.status >= 500 && n < 2` | retry server errors at most twice; never retry a 4xx |
| FE-PARAM-3 | refetchOnWindowFocus | false | no refetch on focus by default; opt in per query where freshness matters |
| FE-PARAM-4 | queryCache.onError | one global error sink | the single place query errors are logged/reported |

Reference shape (created once, passed to the provider at the application root):

```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,                                            // FE-PARAM-1
      retry: (n, err) => (err as Problem)?.status >= 500 && n < 2,  // FE-PARAM-2
      refetchOnWindowFocus: false,                                  // FE-PARAM-3
    },
  },
  queryCache: new QueryCache({ onError: reportError }),             // FE-PARAM-4
});
```

### Parameters — design tokens
Source: margince-poc/docs/architecture/frontend.md#the-design-system, #storybook, #story-presentation-conventions; font families from margince-poc/docs/architecture/web-design-system.md (typography) @ a11d6c08

Token-prefix rules:

| ID | Rule | Detail |
|---|---|---|
| FE-DS-1 | Design-system semantic names KEEP the `gf-` prefix | `bg-gf-card`, `text-gf-primary`, `p-gf-md` |
| FE-DS-2 | Tailwind-default names DROP `gf-` | `rounded-md`, `font-mono`, `max-w-2xl` |

Semantic utility catalog (keep `gf-`):

| ID | Purpose | Utilities |
|---|---|---|
| FE-DS-3 | Background | `bg-gf-page` `bg-gf-elevated` `bg-gf-card` `bg-gf-hover` `bg-gf-accent` |
| FE-DS-4 | Text | `text-gf-primary` `text-gf-secondary` `text-gf-tertiary` `text-gf-muted` `text-gf-accent` |
| FE-DS-5 | Border | `border-gf-subtle` `border-gf-strong` |
| FE-DS-6 | Spacing | `p-gf-xs/sm/md/lg/xl`, `m-gf-*`, `gap-gf-*`, `mt-gf-*` |
| FE-DS-7 | Typography scale | `text-gf-display` `text-gf-heading` `text-gf-title` `text-gf-body` `text-gf-caption` `text-gf-label` `text-gf-micro` |
| FE-DS-8 | Status | `text-gf-status-danger` `text-gf-status-success` `text-gf-status-warning` `text-gf-status-info` |
| FE-DS-9 | Z-index | `z-gf-modal` `z-gf-toast` `z-gf-tooltip` `z-gf-dropdown` `z-gf-overlay` |
| FE-DS-10 | Motion | `duration-gf-fast` `duration-gf-base` `duration-gf-slow` |

Hard rules and gates:

| ID | Rule |
|---|---|
| FE-DS-11 | Never hardcode hex (`bg-[#FF6B00]` → `bg-gf-accent`) |
| FE-DS-12 | Never hardcode px (`p-[12px]` → `p-gf-md`) |
| FE-DS-13 | Never raw Tailwind palette for state (`text-red-500` → `text-gf-status-danger`) |
| FE-DS-14 | Never `dark:` overrides on design-system primitives — they auto-flip via the `.dark` selector |
| FE-DS-15 | Banned palettes (fail ds-purity): `emerald`, `rose`, `slate`, `sky`, `cyan`, `lime`, `fuchsia`, `purple`, `indigo`. Substitute `emerald→green`, `rose→pink`, `slate→neutral` |
| FE-DS-16 | ds-purity gate fails on: raw hex literals, `text-[Npx]`, a design-system-only utility without `gf-`, numeric `duration-N`, `z-50` and above, unmapped Tailwind palette. Runs inside the aggregate frontend gate |
| FE-DS-17 | Exactly three font families: Outfit (display), DM Sans (body), JetBrains Mono (mono) — only these pass the font-lock gate under app source |

Storybook title and story conventions:

| ID | Convention |
|---|---|
| FE-DS-18 | Title format: `"UI/<Name>"` for reuse-set primitives, `"CRM/<Name>"` for CRM domain components |
| FE-DS-19 | Every story declares `args` — no setup code that hides the component contract |
| FE-DS-20 | Every exported story runs as a headless Vitest/Playwright test; stories are deterministic |
| FE-DS-21 | Story file colocated with its component (`<Name>.stories.tsx` next to `<Name>.tsx`) |
| FE-DS-22 | Surface per story via `parameters.surface`: `"padded"` (default — page surface + gutter), `"centered"` (single small atoms), `"fullscreen"` (full-bleed shells that supply their own padding). The global surface decorator owns the page background and gutter; stories never re-add surface chrome |
| FE-DS-23 | Fluid components are pinned to a representative width with a meta-level decorator (e.g. a `w-80` wrapper), never a hardcoded px and never left to fill the canvas |
| FE-DS-24 | Every component stories its states: empty (reading as "empty by design"), plus loading, error, and overflow where they exist |
| FE-DS-25 | Every story is checked in both themes via the Theme toolbar |
| FE-DS-26 | Mockup mapping at the meta level: `meta.parameters.design = { node: "<node>" }` pointing at the component's frame in the tracked mockup file; a single-story override only when a state has a distinct mockup frame |
| FE-DS-27 | No Storybook `backgrounds` palette with literal color values (would hardcode hex and skip dark-mode flipping); surface switching is a decorator applying `bg-gf-page` / `bg-gf-elevated` utilities |

### Wire — layer model
Source: margince-poc/docs/architecture/frontend.md#component-layer-model @ a11d6c08

> "Could another product (CRM, cv-screening, perf-review) use this unmodified?"

| ID | Layer | Location | Knows about | CRM examples |
|---|---|---|---|---|
| FE-LAYER-1 | 1 — design-system atom | the Forge barrel in `frontend/src/shared/` | nothing (generic) | Button, Avatar, Badge, Modal |
| FE-LAYER-2 | 2 — CRM domain component | `frontend/src/features/<name>/components/` (props-only; graduates to `frontend/src/shared/` when a second feature needs it) | CRM types, props-only, no fetching | `PersonCard`, `DealStageChip` |
| FE-LAYER-3 | 3 — feature component | `frontend/src/features/<name>/components/`, consuming the feature's `hooks/` + `api/` | app data, owns hooks + state | `PersonList`, `DealDetailPanel` |
| FE-LAYER-4 | 4 — route / layout | `frontend/src/features/<name>/routes/`; app shell in `frontend/src/app/` | routing only | `PeoplePage`, `AppLayout` |

Placement decision procedure, applied in order:

| ID | Question | Placement |
|---|---|---|
| FE-PLACE-1 | Does a design-system atom cover it? | Import from the Forge barrel in `frontend/src/shared/`. Never re-implement Button / Avatar / Badge locally |
| FE-PLACE-2 | An atom exists but lacks a variant? | Propose the variant upstream in the design system; don't fork a local copy |
| FE-PLACE-3 | No atom, fully generic? | Belongs in the design system (upstream), not in this repo's feature code |
| FE-PLACE-4 | CRM domain types, props-only, no fetching? | `frontend/src/features/<name>/components/` (FE-LAYER-2); promote to `frontend/src/shared/` on second-feature use |
| FE-PLACE-5 | Needs data fetching / app state? | `frontend/src/features/<name>/components/` with the feature's `hooks/` + `api/` (FE-LAYER-3) |
| FE-PLACE-6 | One-off page composition? | `frontend/src/features/<name>/routes/` or the shell in `frontend/src/app/` (FE-LAYER-4) |

### Acceptance — visual rubric
Source: margince-poc/docs/architecture/frontend.md#visual-craft--the-ui-quality-rubric-v1v8 @ a11d6c08

Advisory: judged by eye (a human, or a reviewer agent against a mockup), never a
deterministic merge-blocker. The drift gates (ds-purity / font-lock / icon-lint,
FE-DS-16..17) are the mechanical complement; this rubric is the quality check they
cannot express.

| ID | Tell | The rule |
|---|---|---|
| V1 | **Framing & surface** | Renders on `bg-gf-page` with the standard gutter (the global decorator); the right `parameters.surface` for the component class. No element stranded in the canvas corner. |
| V2 | **Intrinsic sizing** | Nothing stretches unnaturally. Inputs/cards/tables present at a realistic width; fluid components are pinned to a representative width in the story, not left to fill the canvas. |
| V3 | **Spacing rhythm** | Spacing comes from the `p/m/gap-gf-*` scale, applied consistently. Related items grouped, unrelated separated. Nothing cramped, nothing floating in dead space. |
| V4 | **Hierarchy & alignment** | Clear type hierarchy via the `text-gf-*` scale (heading > body > caption). Edges align to a grid. One primary action per surface; secondary actions read as secondary. |
| V5 | **State coverage** | Empty / loading / error / overflow states are designed and storied, not afterthoughts. The empty state reads as "empty by design," never "broken." |
| V6 | **Responsive integrity** | Components that reflow have viewport variants; nothing clips, overflows, or collapses at narrow widths. |
| V7 | **Density & restraint** | Calm by default (Ledger-Green). Whitespace is deliberate; no over-decoration, no gratuitous borders/shadows. The smallest composition that reads clearly wins. |
| V8 | **Theme parity** | Correct in both light and dark (Theme toolbar). Contrast and emphasis hold across the flip; nothing relies on a single theme's accident. |
