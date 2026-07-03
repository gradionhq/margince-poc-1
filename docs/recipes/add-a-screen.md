---
derives-from:
  - architecture/frontend.md
  - architecture/code-organization.md
  - quality/acceptance-standards.md
  - quality/testing.md
  - quality/quality-gates.md
---
<!-- PROCESS DOC — commands, paths, and make targets are this document's subject, so the
     prose-only ban-list (no fences / paths / targets above the appendix) does not apply
     here. Recipe doc: no `## Appendix`. -->
# Add a screen — mirror the People page

The exemplar to mirror: `frontend/src/features/people/` — the `usePeople` list hook,
`PersonCard`, `PersonList`, and `PeoplePage`, one artifact per layer of the model
([[frontend#FE-LAYER-1]]..4).

1. **Create (or extend) the feature directory.** One directory per feature, mirroring
   the backend module: `frontend/src/features/<name>/{api,components,hooks,routes}`
   ([[code-organization#CODEORG-LOC-9]]).

2. **The data hook first.** One hook module per resource under the feature's `api/`,
   built on the query cache over the generated client — mirror `usePeople`: keys from
   a per-resource key factory, the error generic typed to the problem shape,
   mutations invalidating the keys they affect. Hand-rolled fetches are banned; the
   generated client under `frontend/src/lib/api-client` is the single wire entry.

3. **Place each component with the FE-PLACE ladder, in order**
   ([[frontend#FE-PLACE-1]]..6): a Forge atom from the shared barrel if one covers it
   (never re-implement Button/Avatar/Badge — [[frontend#FE-PLACE-1]]); a variant
   proposed upstream if an atom nearly covers it; props-only domain components into
   `components/` at layer 2 (the `PersonCard` shape); data-fetching components at
   layer 3 beside their hooks (the `PersonList` shape); the one-off page composition
   into `routes/` at layer 4 (the `PeoplePage` shape). Reach for the lower-numbered
   answer first — that is what keeps the reusable layers honest.

4. **Build on tokens, not raw values.** Semantic `gf-` utilities only — no hardcoded
   hex or px, no raw palette for state colors ([[frontend#FE-DS-11]]..13); the
   ds-purity gate ([[quality-gates#QG-19]]) fails the build otherwise.

5. **Render the five standard states** ([[acceptance-standards#STATE-1]]..5) as real
   rendered states, never toasts: honest empty, skeleton loading, error card with
   retry, no-permission (denied content absent from the payload, controls omitted),
   and nothing-grounded for any AI-fed panel. A surface missing its empty or
   no-permission state is not done. A generative surface also carries the AI-assisted
   disclosure ([[acceptance-standards#GATE-AI-9]]).

6. **Stories.** Colocate `<Name>.stories.tsx` with each component
   ([[frontend#FE-DS-21]]); story every state — empty reading as "empty by design",
   loading, error, overflow ([[frontend#FE-DS-24]]); check both themes
   ([[frontend#FE-DS-25]]). Every exported story runs as a headless test
   ([[frontend#FE-DS-20]]), mirroring the PersonCard/PersonList stories.

7. **Tests per the placement ladder** (testing chapter): pure logic and hook state as
   unit tests in the simulated DOM; rendering and callbacks as unit tests with the
   testing library (feature components mock their hooks); real-browser behavior only
   as a story interaction test with a one-line browser-only justification; visual
   states as render-only stories. The page test mirrors PeoplePage's. Then green the
   frontend gates: `fe-lint`, `fe-typecheck`, `fe-test`
   ([[quality-gates#QG-17]], [[quality-gates#QG-18]], [[quality-gates#QG-22]]) via
   `make check`.
