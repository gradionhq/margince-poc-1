# ADR-0002 — Source customization over runtime configuration

> **Amendment 2 (2026-06-23, founder — parity-benchmark reconciliation; DECISIONS A46).** A **bounded
> runtime custom-field** concession is carved out, distinct from the structural customization this ADR
> governs. An **admin (Mor) may add simple typed custom fields at runtime** — `text / number / date /
> currency / picklist / boolean` — **to existing core objects** (person, organization, deal, lead,
> activity), with no source edit, migration PR, or deploy by the customer. The honest "a non-technical
> admin cannot click to add a field" failure mode in Consequences (b) below is thereby closed for the
> *common* case. **What stays source-level (unchanged):** new **objects**, new **relationships/
> associations**, calculated/formula logic beyond a declared safe expression, validation logic, and any
> field that changes joins or reporting structure. The line is **"a new attribute on an existing table"
> = runtime; "a new table or relationship, or new behavior" = source.** Mechanics that keep this honest
> against P11 (no dynamic-schema interpreter on the hot path): a runtime add-field triggers a **real,
> governed `ALTER TABLE` migration** (a real indexed column, not a `field_metadata` row), regenerates the
> contract/types, is **audit-logged**, and is **🟡 approval-gated** (it is a schema change). There is
> **no** runtime **object** builder and **no** metadata engine — those remain exactly what this ADR
> rejects. Recorded as a new runtime-config surface (RC-12, `runtime-config-surface.md`); specified in
> `features/01 §1` (bounded custom fields) / `features/10-operational-depth.md §2`; story S-E15.7.

> **Amendment 1 (2026-06-19, founder — C1 reconciliation; DECISIONS A39).** Agent-performed source
> editing is repositioned as an **internal build/delivery practice** (how Gradion and partners do
> custom development fast), **not a marketed product capability**. The product is **never** described
> as modifying itself, "AI edits your code," or a "describe in English → PR" in-product path — those
> framings are retired. The **customer-facing** story is: *the product is source-available and adapts
> to your business through real custom development done by you, a partner, or Gradion — human-led
> engineering, no config ceiling, no lock-in.* **The engineering below is unchanged**: clean relational
> core, exhaustive types, the test-suite guardrail, stable extension seams, the `crm gen` scaffolding,
> and the M1–M3 upgrade-safety machinery all stay V1 — they govern source customization regardless of
> *who* performs the edit (human, or human AI-assisted internally). Where this ADR or the spec reads
> "performed by AI coding agents" / "the agent edits," read it as the internal delivery practice, not a
> product feature surfaced to customers. Aligns the spec with the deck (`output/deck-concept.md §1.1`,
> which originated this call); resolves deck open item #7.

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md) — **partially validated by spike (2026-06-03), with one scope limit now proven.** See [`../../research/spike-findings.md`](../../research/spike-findings.md).
- Probes 1–2 **PASSED**: additive customization (field + workflow) and a *conflicting* upstream upgrade, tests green — conditional on mitigations M1/M2/M4, which are now **implemented and proven in the spike** (reflection contract-completeness tests catch dropped mappings; `crm gen upgrade` preflight; `AGENTS.md` recipe fixes).
- Probe 3 (deep core-behavior change) **FAILED — known limitation:** altering/removing an *existing* core invariant has **no safe seam**; the agent had to edit a DO-NOT-TOUCH region and the result is not upgrade-safe. The paradigm is strong for *additive* change and weak for *core-behavior overrides*. This bounds the "infinite flexibility" claim and requires the **policy-seam design response** below (a separate ADR + a follow-up spike).
- ~~Still unproven: the non-technical "describe in English → PR" path.~~ **Retired by Amendment 1** — there is no in-product "describe in English → PR" feature; customization is human-led engineering, so this is no longer a gating probe.

## Context

Incumbents (HubSpot, Salesforce, Twenty, Attio) let clients customize through **runtime configuration**: custom-object builders, no-code workflow UIs, metadata engines. This is the thing they over-build and clients under-use, and it carries a permanent cost — a dynamic-schema interpreter on the hot path, broken joins/reporting from directional associations, artificial tier gates, and a config surface that is itself a liability (P1).

Agentic coding (Claude Code, Cursor) crossed the threshold in 2026 where editing real source is faster and more reliable than clicking through config screens — *if* the codebase is built for it. Gradion's principles already commit to this: P1 (opinionated over configurable), P2 (the source is the configuration layer), P3 (agent-readable by construction), P11 (clean relational core; new objects are code, not metadata rows). We need a foundational decision that locks this in and accepts its costs honestly.

## Decision

Per-client customization happens **in the source code, performed by AI coding agents**, not through runtime configuration. We do not build metadata-driven custom objects, no-code workflow builders, or drag-and-drop config engines.

- Custom fields/objects/workflows/views are **real code**: migrations, domain structs, OpenAPI contract changes, generated types, and UI — authored by an agent in the client's source.
- Safety is engineered into the codebase: exhaustive end-to-end static types, mandatory TDD with coverage gates as the guardrail, stable documented extension seams, convention over cleverness, versioned/reversible migrations, and a review gate (PR pipeline) before deploy.
- A scaffolding generator (`crm gen field|object|workflow|view`) emits correct files across all layers so edits start from a consistent skeleton.
- `AGENTS.md` ships in the repo as the agent-development contract and is itself a release gate (its quality gates the product).
- Three deployment modes scope how much source freedom a client gets: **SaaS multi-tenant** (bounded config only — vertical templates, trivial admin fields), **dedicated/on-prem** (full source customization), **source-delivered** (maximal; client owns repo + cloud project).

The bet is validated, not assumed: Stage-6 probes (the `renewal_risk` worked example, the upstream-upgrade probe on a customized fork) must pass before sign-off. *(Per Amendment 1, the non-technical "describe what you want → PR" probe is dropped — that in-product path is retired; customization is human-led engineering.)*

**Spike update (2026-06-03):** the first two probes PASSED with fresh, context-free coding agents (see `research/spike-findings.md`). The upgrade probe surfaced a real failure mode — conflicts land inside shared functions (`Validate`/`toDTO`/`applyTo`), where a naive resolver can silently drop a field the compiler won't catch (a dropped DTO mapping just zeroes the field; only a round-trip test catches it). This makes the following mandatory, not optional:
- **M1** — every field (core and custom) ships a contract round-trip test, generated by `crm gen field`, so a dropped mapping fails CI.
- **M2** — a scaffolded `crm gen upgrade` preflight: list conflicts, flag shared-function conflicts with "union, don't pick," and run a contract-completeness check (every domain field appears in `toDTO`).
- **M3** — upstream releases declare new invariants/validation in `CHANGELOG.md`; the upgrade recipe re-runs client fixtures against them (a conflict-free merge can still break behavior).

## Consequences

- **Positive:** infinite flexibility with zero config-engine complexity; static schema → real indexes and honest reporting (P4/P11); changes are testable, reviewable, versioned, diffable real code; no artificial tier gates; the same agentic capability powers runtime BYO-agent work (ADR-0003) — one coherent agent story; incumbents cannot follow without dismantling their config-engine moat.
- **Negative / honest failure modes:** (a) a non-technical admin cannot click to add a field — it is a development task handled by a partner or Gradion services (per Amendment 1, not an in-product self-service path), softened by vertical templates and the generator for the people who do the work; (b) hosted multi-tenant clients cannot fork the shared codebase freely; (c) clients who diverge must absorb upstream upgrades — mitigated by strict core/custom separation, contract-stable seams, semantic versioning + migration guides, and an "upgrade agent" recipe. If (a) can't be made effortless and (c) can't be made safe, the paradigm fails — hence the gating probes.
- **Eliminates** an entire category of incumbent complexity (metadata engine, no-code builders) and the support/consulting motion around it — replaced by implementation/consulting on source customization as a revenue pillar.
- **Proven scope limit (probe 3) — and why it is acceptable (P14):** the model supports *additive* customization safely; *overriding existing core behavior* (relaxing/replacing a core invariant or computation) has no safe seam today and forces an upgrade-unsafe core edit. **We deliberately do not engineer this away fully.** Per P14 (consulting funnel), a client who edits core and later hits upgrade pain is a **billable engagement**, not a product failure — and over-building extension points to make every change upgrade-safe fights P1 (simplicity) and the small-team reality. The calibrated response is: (a) provide a *small curated* set of **overridable policy seams** for the behaviors clients vary most often (a named validation-rule registry; replaceable strategies for scoring/dedupe/routing/stage-transition) so the *common* cases stay additive and upgrade-safe; (b) give crystal-clear "do this / don't touch that" guidance + a clean disciplined upgrade path; (c) explicitly allow core edits as a supported-but-discouraged path with a **disclosed** upgrade cost, with Gradion consulting as the paid safety net. Curated, not exhaustive — making everything overridable would recreate the runtime-config engine P1 rejects. A follow-up spike may prototype the validation-rule registry, but full upgrade-safety for arbitrary core changes is explicitly a non-goal.
