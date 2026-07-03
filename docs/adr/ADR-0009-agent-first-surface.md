# ADR-0009 — Agent-first surface: intent tools, MCP-Apps UI, hosted connector as product, governed autonomous loop

**Status:** Accepted (Lars, 2026-06-10). Narrative parent: [`../03c-agentic-concept.md`](../narrative/03c-agentic-concept.md). Sharpens, does not supersede, [ADR-0005](ADR-0005-agent-runtime.md) and [`../03b-ai-architecture.md`](../narrative/03b-ai-architecture.md). Consumer translation: [`../../AI-CONCEPT.md`](../../output/md/AI-CONCEPT.md).

> **Amendment 1 (A35/[ADR-0027](ADR-0027-gradion-operates-no-infrastructure.md), [ADR-0005](ADR-0005-agent-runtime.md) Am.3):** Gradion operates no infrastructure. The hosted connector (Surface A2) and the Surface B reasoning loop run on the **operator's** infra — a hosting partner or the self-hosting customer, **never Gradion**. The "operate A2 24/7" / "A2 24/7 operating cost" references below are therefore the **operator's** cost, not Gradion's; Gradion ships the product, the operator runs it.

## Context

The June-2026 market shifted hard toward **agentic-for-consumers** (research in `../research/landscape/`):

- OpenAI is turning ChatGPT into an agentic "superapp" (*"Chat is dead"*); Codex passed 5M weekly users, knowledge workers the fastest-growing cohort. Users delegate work — increasingly from their **phone**.
- **MCP won as the integration substrate**, and **MCP Apps** (Jan 2026) + the **Apps SDK** extended it into the **UI layer**: third-party apps render interactive UI inside ChatGPT/Claude and are discovered via in-assistant **app directories** (Salesforce/Clay/monday/Slack launch partners).
- CRM incumbents (Agentforce, Breeze) moved from "suggest a draft" to "run the task autonomously."

Our existing bet (governed MCP tools + hosted connector — `03b`, ADR-0005) is **directionally correct and ahead of the wave**, but `03b`'s surface is thin in three places and one direction was under-specified. The founder's prompt: *"this is not really agentic… GPT/Codex should let any salesperson steer or automate the system from their phone, and the current architecture isn't good enough for that."*

The mis-framing to kill first: the question conflated **(A)** inbound, human-initiated access from an external assistant with **(B)** a self-initiated proactive loop. They are architecturally opposite and both required (see Decision 5).

## Decision

Adopt the **agent-first substrate** frame (`03c`): we are the governed body/memory any brain acts through, not a competing brain. Concretely, four additive changes on top of `03b`/ADR-0005 — **none unwinds the relational core, the three-layer model, or the governance.**

1. **Intent-level tools (verbs, not tables).** Add a higher tool layer of fewer, fatter, outcome-shaped tools (`qualify_lead`, `prep_for_meeting`, `progress_deal`, `whats_slipping_this_week`, `catch_me_up_on`, …) composed over the `SystemOfRecordProvider` (`interfaces.md §3`), each grounded by the context graph (ADR-0007), each declaring its own Passport scope + 🟢/🟡 tier. The CRUD tools (`interfaces.md §2.1`) **remain** as the low-level seam. Reads return assembled, provenance-stamped context, not raw rows.

2. **MCP-Apps / Apps-SDK UI layer.** Tools may return **interactive UI components** that render inside ChatGPT/Claude (deal card, pipeline board, approve-and-send button) per MCP Apps + the Apps SDK. The 🟡 approval becomes a button in the assistant. Net-new workstream; the web app is no longer the only front end.

3. **Hosted connector (Surface A2) is a V1 product cornerstone + directory distribution.** Promote A2 (OAuth 2.1 HTTPS MCP service, operated 24/7 with a real Agent Seat Passport login/consent — `03b`) from fast-follow to V1 cornerstone, and **list it in the assistant app directories** as a primary discovery channel (alongside AI-SEO, `10-gtm`).

4. **One governed autonomous loop (Surface B).** Extend Surface B from deterministic typed-Go handlers (`features/03 §5`) to a real **reasoning loop** on River cron/event triggers, invoking a model (API key or **local** — ADR-0005) as its brain, under the *same* Passport + approval + audit. Deterministic handlers stay for predictable automations; the reasoning loop covers judgment cases (overnight agent, morning brief).

5. **Codify the two directions** (the on-rail invariant, `03c §2`): **A = inbound**, the vendor's assistant calls into our connector (brain on vendor infra); **B = proactive**, our loop calls a model (brain invoked on the **operator's** infra — a hosting partner or the self-hosting customer, **never Gradion**; A35/ADR-0027). B does **not** drive A and A does **not** drive B; the shared, ours-to-own layer is the governed tool surface + Passport + gates + audit beneath both (the *design* is ours; the operator *runs* it). Server-side/headless/sovereign/org-wide-proactive can only be B; phone/ad-hoc/human-in-the-loop is A.

## Consequences

- **Positive:** rides the dominant 2026 distribution + UX shift while keeping the moat (context graph + governance + own-your-data + OSS/consulting, `03c §3`); same governance under both directions; local-model path means the autonomous loop works in sovereign mode where no external assistant can reach.
- **Cost / build:** new intent-tool layer, an MCP-Apps UI workstream, running A2 24/7 (public HTTPS, OAuth, consent — already flagged in ADR-0005 Am.2; the 24/7 operating cost is the **operator's**, not Gradion's — Amendment 1, A35/ADR-0027), and hardening Surface B into a reasoning loop. The relational core, schema, events, and SoR seam are unchanged — these sit *above* them.
- **Security:** more inbound reach (A2 in a public directory) and an autonomous reasoning loop both widen the `05` prompt-injection/egress surface — mitigated by the **existing** controls (Passport `agent ≤ human`, 🟡 gates at the tool contract, egress controls, capture isolation, injection red-team GA gate). No new trust model; more traffic through the same one.
- **Messaging:** "agentic" is reframed from *"we have an agent"* to *"the CRM any agent can safely run, from your phone"* — `AI-CONCEPT.md`.

## What this does NOT change (guardrails — `03c §5`)

No competing brain (P6/P10); no daemon competing with OpenClaw/Codex (B is only for what A structurally can't do); no trading the relational core/governance for "agentic" (P11/P12 are the moat); agent-first ≠ agent-required (L2 baseline + fast core still work seatless); 🟡 stays 🟡 (no autonomous writes on money/outbound).

## Follow-ups

- `interfaces.md §2` — add the intent-tool layer spec + the MCP-Apps return-type seam (additive).
- `features/07` / `08` — the in-assistant UI components + the phone A-direction journeys.
- `09-economics` — A2 24/7 operating cost (the **operator's**, not Gradion's — Amendment 1, A35/ADR-0027); directory-listing as a distribution line.
- DECISIONS A17–A20 (ratified 2026-06-10) record the committed values.
