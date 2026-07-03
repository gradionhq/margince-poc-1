# ADR-0027 — Gradion operates no hosting; every running instance is partner-operated or customer-self-hosted; data residency per tier

**Status:** Accepted (Lars, 2026-06-17). Research parent: [`../../research/r7-deployment-and-residency.md`](../../research/runtime/r7-deployment-and-residency.md) (closes BACKLOG **R7**; R8 remains open). **Canonicalizes** the 2026-06-13 "Gradion runs no cloud of its own" ratification that until now lived only as a floating note in `09-economics`, `10-gtm`, `12-license`, and `licensing/PARTNER-LICENSE.md`. **Composes with** A28/ADR-0020 (we provide no inference — same shape, now extended to hosting), A8 (egress location ladder), A24 (EU residency default), A31/ADR-0023 (release delivery), ADR-0005 (the hosted A2 connector). Resolves the **R-B1** deployment-posture blocker. New decision: **DECISIONS A35**.

## Context

Lars: *"We never host it. Only through partners. This must be updated in all plans."*

This isn't new in spirit — on 2026-06-13 Lars already ratified "Gradion runs no cloud of its own; a partner ecosystem runs the hosted offering." That decision is baked into the economics (wholesale-per-seat to partners), the license, and the GTM model. But it never got a decision number, and the **deployment, residency, and AI-architecture docs never caught up** — they still say "Gradion-hosted EU region" (A8), "SaaS on Gradion infra" (`06 §6.3`), "SaaS: Gradion-managed backups" (`06 §6.8`), "we-push" releases (A31), and "we host it 24/7" for the connector (ADR-0005). R7 fixes that, names a single canonical decision, and pins the residency promise — which is the question R7 was actually opened to answer.

The one genuinely architectural knot: cloud assistants (ChatGPT/Claude routines) **cannot** reach a binary on someone's laptop, so *somebody* must run a public, always-on connector (the A2 service, ADR-0005). The answer under "Gradion never hosts": **the operator runs it** — a hosting partner for partner-hosted tenants, the customer for self-hosted ones. Gradion ships the software; it does not run the service.

## Decision

**Gradion is a software vendor, not an operator. Gradion runs no production hosting, no customer data store, no always-on connector, no inference. Every running instance is operated by either (a) a hosting partner or (b) the customer who self-hosts. Data residency is the operator's named region; the EU default is delivered by choosing an EU partner.** Five parts:

1. **Two operator types, never Gradion.**
   - **Partner-hosted** — a hosting partner (for the DACH market, an EU sovereign-cloud partner) operates the instance: runs the infra, holds the EU region, applies updates, and is the customer's data processor/sub-processor. This is the "SaaS / managed" offering. *(The old "SaaS multi-tenant on Gradion infra" becomes "partner-hosted.")*
   - **Self-hosted** — the customer runs it in their own cloud, on-prem, or air-gapped. Dedicated/source-delivered modes are self-hosted (optionally with a partner or Gradion *consulting* engagement, which is advice, not operations).

2. **The hosted A2 connector and Surface B run on the operator's infra, not Gradion's.** Gradion ships `cmd/crm-mcp-http` (the always-on connector) and the Surface B reasoning loop as software. Whoever operates the instance runs them. "We host it 24/7" (ADR-0005) and "our loop on our infra" (ADR-0009) are restated as **"the operator runs it 24/7 / on the operator's infra."**

3. **Residency = the operator's region; EU by default via an EU partner.** The data-residency promise is the *operator's* commitment, flowed through contractually. The default partner offering is an EU region, named. Honest carve-outs stay: WhatsApp goes through Meta (operator-in-path, A30), and a cloud-frontier AI model only runs if the customer opts in under their own DPA (A8). Gradion holds no customer data in any tier.

4. **EU-hosted inference: partner-provided, provider-agnostic.** The EU-hosted-open AI tier (A8 tier 2) is run by a partner, not Gradion. We do **not** pin a single provider in the spec — we require a **clean EU sub-processor chain** (no US-parent reach) and treat StackIT / IONOS / OVHcloud / Aleph Alpha as candidates the operator/customer picks among. Inference stays customer-supplied (A28): the customer holds the key/contract, or it's a transparent partner pass-through; Gradion never marks it up.

5. **Releases: Gradion publishes, the operator applies.** Gradion's "we-push continuous deploy" (A31) is restated: **Gradion publishes signed releases; the operator deploys them.** For partner-hosted, the partner runs continuous deployment on the customer's behalf (no client-visible version); for self-hosted, the customer pulls and applies per their window. Gradion remains the **CRA "manufacturer" of the software** (A21/A27); the operator is responsible for running it. The supplier incident-notification commitment (A33) flows Gradion → operator → customer.

## Consequences

- **One canonical decision (A35) the deployment docs point at**, instead of a floating correction note. The economics/license docs that already said "no cloud" are now anchored to it.
- **Resolves R-B1** (the deployment-mode posture blocker in `07-risks`). The three modes stay; only the *operator* is clarified — SaaS/managed = partner-operated, never Gradion.
- **Strengthens the DACH GTM frame (A32/ADR-0024)** and the BUSL non-compete (R-E2): "would-be free-riders become paying hosting partners" is now the literal operating model, and hosting/managed-service is a named partner-revenue pillar.
- **No architecture change.** The software is identical; this is a *who-runs-it* decision. The A2 connector and Surface B are unchanged code — they just run on a partner's or customer's infra.
- **Pricing follow-through is R8.** Gradion's take is the wholesale slice to partners + paid self-host seats + service/consulting; the retail seat is the partner's. R8 ratifies the actual numbers.

## Alternatives considered

- **Gradion operates an EU SaaS itself (the old default).** Rejected by Lars — off-strategy for a small dev/consulting shop (capex + 24/7 ops + becoming a data processor at scale), and it undercuts the partner channel that the license and GTM are built around.
- **Name a single preferred EU hosting/inference partner in the spec.** Rejected for now — stay provider-agnostic, require a clean EU sub-processor chain, let the operator choose. (A named partner can still be a sales asset; it just isn't pinned in the spec.)
- **Keep "we host the A2 connector" as the one exception.** Rejected — it would reintroduce Gradion as an operator (public service, OAuth, 24/7, data in transit). The operator runs the connector; Gradion ships it.
