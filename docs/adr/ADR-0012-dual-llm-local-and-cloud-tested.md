# ADR-0012 — Both local and cloud LLM paths are first-class, tested, and shipped in V1; non-Chinese local default

**Status:** Accepted (2026-06-10, with Lars) — DECISIONS A23 (amends A9), A24. Built into `ai-operational-spec.md §1`, `03b-ai-architecture.md`, build-plan WP3. Raised by the security-practice cross-assessment (`foundation/spec/feedback/security-practice-crossref.md`): the practice's strongest argument is CLOUD-Act / US-dependency risk, yet the CRM's default brain was US-frontier and the local path read as a roadmap option; and the seeded local models were Chinese-origin (Qwen), which carries a trust/reputation problem in the German market we target.

## Context

The CRM already had a provider-agnostic `Client` and a `profile: sovereign` switch that forces local models + egress-deny (A8, `ai-operational-spec.md §1.4`). But:
- The **cloud path was the default and the local path was under-committed** — described as the regulated-segment option, not a co-equal, tested shipping path. For a product that sells "own your data" into a market primed (by the practice's own `politischer-wille.md`) to distrust US cloud dependency, the local path cannot be a maybe.
- **Seeded local models were Qwen (Alibaba, Chinese-origin).** Independent of technical merit, Chinese-origin AI carries a reputation problem with the German Mittelstand and public-sector buyers the practice targets. Shipping it as the *default* sovereign brain contradicts the sovereignty pitch.

## Decision

1. **Ship both paths in V1, both tested.** The fully-local inference path and the hosted-frontier path are both first-class V1 deliverables. Both are covered by **unit and integration tests**. The A7 baseline-AI eval gates (extraction precision/recall, summary factuality, NL→query plan-correctness, schema-validity, the non-negotiable no-guess = 0 and injection-egress = 0) **must pass on both** the local-default and cloud-default bindings before WP3 exits. The `sovereign` zero-egress conformance test (`features/02` ACX.6 — asserts zero external calls for capture/summary/draft/search/every `07` moment) gates the local path.
2. **Default local models become non-Chinese open weights.** `local_small` default = **Gemma (latest, Gemma-3/4-class)**; `local_large` default = **Llama-3.x-70B-class**. Both run on the workspace's own Ollama/vLLM.
3. **Mistral (French / EU-origin) is the recommended swappable alternative**, called out as the preferred choice where the sovereignty narrative is sharpest — an EU-origin model is the strongest version of the "no foreign dependency" story.
4. **Qwen is dropped from the recommended defaults** on German-market trust grounds. It remains *config-selectable* because tier→model is config, not code (`§1.4`) — a customer who wants it can bind it, but we do not seed or recommend it.

This amends A9's tier→model bindings; the tier *architecture* (local-small / cheap-cloud / premium-frontier / local-large + separate embedding/STT lanes) is unchanged.

## Consequences

- **Positive:** the sovereignty pitch is honest — a regulated customer gets a tested, zero-egress, non-Chinese, EU-capable brain by default (A24); the cloud path remains for the cost-conscious AI-native beachhead; "both, tested" removes the "is local actually supported?" objection.
- **Ripple:** `ai-operational-spec.md §1.1/§1.4` updates the tier table and the `ai-routing.yaml` defaults (`gemma`/`llama`, Mistral noted, Qwen removed from defaults); `03b` notes both paths are tested V1; WP3's exit gate now requires green integration tests **and** passing evals on **both** bindings; the seed/config (`seed-and-fixtures.md`, `runtime-config-surface.md`) reflects the new default bindings.
- **Negative / to bound:** maintaining eval quality across two default brains is more work than one — bounded by the tier abstraction (code targets tiers, not models) and by the eval harness running both as a gate rather than ad hoc. Local-large quality on a customer's own GPU is hardware-dependent; the eval gate is run against the reference binding, and `RATIFY` notes in `§1.2` already allow per-task demotion if budget/quality demands.
- **Boundary:** this is about which models back the tiers and that both paths ship tested. The *middle* case is now **EU-hosted open-weight models** (data stays in the EU), **not** PII redaction-then-frontier (A8 revised retired pseudonymization).
