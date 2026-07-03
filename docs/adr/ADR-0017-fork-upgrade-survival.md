# ADR-0017 — Fork-upgrade survival: seam versioning and migration architecture

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md) (2026-06-11, architecture-blueprint research phase). Synthesizes T5 (`foundation/research/t5-fork-survival.md`), verified in `verification-log.md`. Extends ADR-0002 and `customization-portability.md §5`. Serves Goal 2 (clients modify their fork and survive upstream upgrades). **De-risks the one spike probe (probe 3) that already failed.** **Complemented by [ADR-0023](ADR-0023-release-delivery-and-graded-patch-policy.md) (A31):** this ADR owns *fork-merge survival at upgrade time*; ADR-0023 owns the *delivery channel* and adds a **patch-only mode** to the `crm gen upgrade` preflight that classifies a standalone security diff against the fork (Green = auto-apply, Yellow = supervised, Red = human-required) so a security fix can be cherry-picked onto a modified install without a full feature upgrade. The three conflict buckets this ADR's preflight classifies (marked-region / shared-function / core-override) are the Green/Yellow/Red patchability grades ADR-0023 promises against.

## Context

ADR-0002 makes a client's customization a set of source edits on their fork. The spike proved the sharpest risk is upstream-merge pain: conflicts land inside shared functions (`toDTO`/`Validate`) and can silently drop a field, and a clean text-merge can still break behavior (probe 2). `customization-portability.md` *promises* semver'd seams but never stated the rules. This ADR states them.

## Decision

**1. Seam interfaces that forks implement are frozen; evolve additively only.** Adding a method to a Go interface a fork implements is a breaking change (**verified** against the Go compatibility promise; stdlib evolves via new interfaces — `io.Seeker`, `http.Flusher`). Therefore: every exported type is explicitly classified `seam` (fork-facing, frozen) or sealed-`internal`. A seam evolves by introducing a **new interface (`ProviderV2`) + a runtime capability probe** (`if v2, ok := s.(ScoringStrategyV2); ok { … }`), never by mutating the existing one. Structs evolve additively (new fields, never repurposed). The HTTP contract evolves Stripe-style: additive, deprecate-before-remove, **`oasdiff`-gated** on release (oasdiff breaking-change detection **verified**), with `x-extensible-enum` on any response enum a fork switches on.

**2. Two migration namespaces eliminate the collision class.** `core/` migrations are sequential and upstream-owned; `custom/` migrations are timestamp-ordered and client-owned; each has its own tracking table; apply order is core-then-custom. Custom columns carry an `x_` prefix so an upstream column can never name-collide (the migration form of SuiteCRM's `custom/` rule).

**3. Expand/contract is mandatory upstream for any column rewrite or new constraint.** Spanning ≥2 releases (Fowler Parallel Change) gives the fork an announced transition window and converts probe 2's silent "clean-merge-but-broken-behavior" into a scheduled, flagged change.

**4. `crm gen upgrade` preflight.** Resolves remote-vs-local, then classifies conflicts: **loud "UNION, DON'T PICK"** on shared-function hunks (`toDTO`/`Validate`) to kill silent field-drop; **RED, human-required** on core-override conflicts (probe-3 class). It runs the reflection-based contract-completeness check (ADR-0002 M1/M2) + `oasdiff`, validates migration namespacing, parses `BEHAVIOR:`-tagged changelog entries, and re-runs client fixtures. **Nothing auto-merges; a green fixture run is the only go signal.**

**5. Structured `CHANGELOG.md`** with `BREAKING / BEHAVIOR / EXPAND / CONTRACT / DEPRECATED / SEAM` tags, sourced from the `oasdiff`/detector output (not memory), tells a fork exactly what to re-verify — closing the conflict-free-but-behavior-breaking hole.

## Consequences

- **Positive:** a fork survives many release cycles; the upgrade path is mechanical and gated, not heroic; divergence stays monetizable consulting (P14), not a support catastrophe.
- **Custom-column drift guard (correction F-T5, RESOLVED in Amendment 1 below):** the proposed Atlas column-pattern `exclude` (`table.*.column.x_*`) is **NOT valid Atlas syntax**; the guard is instead a small owned comparator (naming-convention invariant + ownership-partitioned table diff + collision assertion). See Amendment 1.
- **Boundary:** the *generator scaffold shape* (what `crm gen field` emits) is blueprint H; `oasdiff` *wiring* is ADR-0015. This ADR owns the *policy*.

## Amendment 1 (2026-06-23, deep red-team) — the custom-column drift guard is a designed mechanism, not Atlas's `exclude`

The "client `x_` columns survive an upstream upgrade" guarantee — which the Green-lane mechanical-patch promise (ADR-0023) leans on — was left as an unsolved spike because Atlas's column-pattern `exclude` is not valid syntax. **Resolved (closes RT-AR-C2 / RT-BL-C3):** the guard does **not** rely on an Atlas exclude glob. It is a **custom, owned check**, run in the `crm gen upgrade` preflight and in CI, with three concrete parts:

1. **Naming-convention invariant.** Client-added columns MUST carry the `x_` prefix (enforced by `crm gen field`); upstream columns MUST NOT. A lint over both the upstream migrations and the fork's `custom/` migrations fails the build *before* any diff if violated.
2. **Ownership-partitioned table diff.** We own the comparator (Atlas or plain `information_schema` introspection — not Atlas's glob): it computes desired schema from `core/` migrations, then **partitions observed columns by the `x_` prefix** — upstream columns diffed normally; `x_` columns asserted present-and-unchanged and **never** emitted as drop/alter. A generated migration touching an `x_` column is a hard preflight failure, not a silent drop. (This is exactly the part Atlas couldn't express — it now lives in our comparator, so it is buildable.)
3. **Collision assertion.** The preflight asserts the upstream and client column-name sets are disjoint and fails loudly if upstream ever introduces an `x_` name, turning a latent collision into a caught CI error.

This makes the *client-custom-column* edge collision-proof rather than discipline-dependent; Green (ADR-0023) may rely on it. Upstream expand/contract (Decision 3) still governs *upstream* rewrites.

## Amendment 2 (2026-06-23, deep red-team) — online-DDL execution discipline (multi-tenant, zero-downtime)

`ALTER TABLE`/index creation on a large shared multi-tenant table takes an `ACCESS EXCLUSIVE` lock and stalls every tenant. Migration *execution* is therefore disciplined (closes RT-AR-C3):

- **`lock_timeout` on every migration session** (e.g. `SET lock_timeout='2s'`) — a migration that can't grab its lock fast fails-and-retries, never queues behind live traffic.
- **Indexes built `CONCURRENTLY`**; new `CHECK`/`NOT NULL` added `NOT VALID` then `VALIDATE`d separately; column adds are nullable-then-backfilled-in-batches (expand/contract, mechanized).
- **Backfills batched** as River jobs, never inline in the migration.
- **Runtime add-field (ADR-0002 Amendment 2) obeys all of the above.** On a multi-tenant shared binary it runs under the online-DDL discipline and, for large tables, is gated behind a maintenance-window scheduler so a tenant-initiated `ALTER TABLE` cannot stall other tenants; on single-tenant deployments it runs immediately. Removes the "tenant-initiated DDL is an availability bomb" hole.
