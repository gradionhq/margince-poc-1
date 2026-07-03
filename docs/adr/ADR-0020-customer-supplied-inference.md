# ADR-0020 — AI inference is customer-supplied (BYOK or self-hosted); Gradion provides none

**Status:** Accepted (Lars, 2026-06-16). Research parent: [`../../research/r1-ai-packaging-boundary.md`](../../research/runtime/r1-ai-packaging-boundary.md) (closes BACKLOG R1). **Supersedes in part** [ADR-0003](ADR-0003-byo-agent-plus-baseline-ai.md) (retires its "Layer 2 baseline AI is bundled in the license, not metered" funding clause). Sharpens, does not unwind, [ADR-0005](ADR-0005-agent-runtime.md), [ADR-0012](ADR-0012-dual-llm-local-and-cloud-tested.md), [ADR-0009](ADR-0009-agent-first-surface.md), [ADR-0013](ADR-0013-one-governed-surface-and-auth.md).

## Context

[ADR-0003](ADR-0003-byo-agent-plus-baseline-ai.md) framed a three-layer AI model in which **Layer 2 ("baseline AI for the agentless majority") was bundled in the seat license, not metered to the user** — Gradion (later, per `09`'s 2026-06-13 correction, a hosting partner) funds the inference and protects margin via cheap/local-first routing and budget guardrails. That framing made the whole margin story rest on `[A-5]` (tokens/seat), a **Low-confidence** estimate (`09` §2.1, §2.4) that, if 3–4× wrong, moves bundled-AI COGS from ~6% to 20%+ of a €25 seat.

The 2026 market shows the opposite of what bundling assumes: incumbents resell intelligence by the unit (HubSpot Breeze ~$0.50/resolved conversation and 100 credits/lead; Salesforce Agentforce three pricing models in 18 months), and buyers are revolting against unpredictable AI bills (78% of IT leaders hit unexpected AI charges in the past year; `R1`). Reselling/bundling inference is a margin liability *and* a trust liability — and it sits awkwardly against **P6** (orchestrate the user's own agents; never reimplement what frontier labs do better) and **P7** (own your data, incl. local inference).

The architecture already supports the alternative: the provider-agnostic model `Client` (ADR-0005, `interfaces.md`) is a one-line BYOK/self-host switch, and the dual local-and-cloud tested path (ADR-0012 / A23) is exactly the two sourcing routes. The decision below makes the economics match the architecture.

## Decision

**Margince provides no AI inference. All inference is customer-supplied; the customer pays the model provider directly.**

1. **Zero Gradion inference.** Gradion does not run, operate, resell, bundle, meter, mark up, or take a slice of model inference for any tier or deployment mode. The seat price is **software only** (the CRM, the governed surface, the AI features, the governance) — never a wrapper around resold intelligence.
2. **Two sourcing routes, customer's choice.** Every AI feature runs on either (a) **BYOK** — the customer's own provider key (Anthropic / OpenAI / Gemini / any provider the `Client` supports), billed by that provider directly; or (b) **self-hosted** — a model the customer runs on their own infrastructure (Ollama / vLLM; the non-Chinese open-weight defaults of ADR-0012: Gemma / Mistral (EU) / Llama). In both, the customer is the model provider's billing counterparty, not Gradion.
3. **All features ship; inference is the only line.** There is no premium "AI feature" tier and no per-feature bundled/BYO split. Every in-product AI feature (auto-capture & enrichment, summarization, draft assistance, NL-search, next-best-action, cold-start read-back, meeting/transcript intelligence, qualification checklist, Morning Brief, Overnight Agent, Voice DNA, warm-room signal) is `feature = we ship it / inference = customer-supplied`. The historical "Layer 1 vs Layer 2" distinction is reduced to *who drives the reasoning loop* — Surface A (the customer's own agent app over MCP) or Surface B + in-app features (our governed code calling the customer's configured `Client`) — never *who pays*.
4. **Cost-control is now a customer convenience, not Gradion margin protection.** The per-workspace budget telemetry, cheap/local-first routing, and graceful degradation specified in `03b`/`ai-operational-spec.md` are retained verbatim and **repositioned** as tools that help the customer see and manage *their own* provider spend — directly answering the market's demand for usage transparency.
5. **AI requires configuration; the core does not.** AI features activate only once the customer has configured a key or a self-hosted endpoint (a first-run onboarding step). The non-AI core — the fast, clean, own-your-data relational CRM with reporting integrity — is fully functional with no model configured.
6. **Partner pass-through is out of band.** A hosting partner *may*, as a separate commercial convenience, offer a managed key or inference endpoint; that lives in partner license terms (`licensing/PARTNER-LICENSE.md`) and is **outside the product's packaging boundary**. The product itself is strictly BYOK/self-host with no Gradion markup.

## Consequences

- **Positive:** removes the `[A-5]` margin bet entirely — the seat is pure software margin, COGS → ~0 for Gradion in every mode; turns the credit-backlash market (`R1`) into a wedge ("BYO AI, no markup, no credits, no surprise bill; your model relationship, your invoice, fully transparent"); strengthens P6/P7 and the own-your-data / sovereignty / source-available thesis; on Sovereign it is BYO-agent + zero egress, which no incumbent can match.
- **Negative / costs:** **hollow-on-arrival risk** — no key and no self-host means no AI features (mitigated by first-run config, bundled self-host defaults, and a non-AI core that stands alone); **quality variance** — features run on whatever model the customer picked, so a weak self-hosted model can underperform (mitigated by published per-task minimum-capability guidance, the A7 eval gates, and graceful degradation); **support attribution** — "the AI is wrong" must separate our prompt from their model (mitigated by model-id/provider in every output's provenance trail, P12); **message risk** — "BYO AI" must read as ownership/transparency, not a hidden upsell.
- **Open questions (→ `07-risks.md`):** none load-bearing for Gradion economics (the former `[A-5]`/`[A-9]` margin risks are retired). Customer-side: per-task minimum-model guidance and onboarding ergonomics for key/self-host config are product-detail follow-ups, not blockers.

## What this does NOT change (guardrails)

- The three-layer **feature** model and every AI feature in `03b`/`features/07` — all ship.
- The governed MCP/REST surface and its invariants — [ADR-0009](ADR-0009-agent-first-surface.md), [ADR-0013](ADR-0013-one-governed-surface-and-auth.md): one governed surface, OAuth 2.1 + Agent Seat Passport, 🟢/🟡 autonomy tiers, `ErrRequiresApproval`, dry-run/diff, full provenance/audit.
- The provider-agnostic `Client` ([ADR-0005](ADR-0005-agent-runtime.md)) and the dual local-and-cloud tested path ([ADR-0012](ADR-0012-dual-llm-local-and-cloud-tested.md) / A23) — reinforced; they *are* the BYOK/self-host mechanism.
- The Gradion Agent Runner (`09` §3.4) — still ships; it is a thin orchestration shell that runs on the customer's configured `Client` (their key or local model), not a Gradion-funded agent.
- The egress posture / location ladder (A8 revised) and secret-stripper hygiene — unchanged; "where the model runs" is now always a customer choice.

## Follow-ups

- `03b-ai-architecture.md §51–69` — reframe Layer 2 from "bundled in the license" to customer-supplied inference; keep the feature set + routing as customer-side cost control.
- `09-economics-and-adoption.md` — retire §1.1 constraint #2; reframe the §1.3 tier "Bundled L2 AI" columns and §1.5 "AI COGS borne by" rows; reposition Part 2 token economics as customer-side cost guidance; re-tag `[A-5]`/`[A-11]`.
- `10-gtm-and-business-model.md` — reframe the "no AI tax" wedge as "BYO AI, no markup, no credits, transparent."
- `06-nonfunctional.md` — reconcile the egress "default SaaS hosted open-weight" tier to customer-key / self-host / explicit partner pass-through.
- `01-prfaq.md` — sweep "bundled / included AI" language to the BYOK frame.
- `DECISIONS.md` — new A-series entry recording this ADR; amend the anchor(s) that encode "baseline AI bundled in license."
- `ADR-0003` — Status note + rewrite of the Layer-2 funding clause and its consequence bullet.
