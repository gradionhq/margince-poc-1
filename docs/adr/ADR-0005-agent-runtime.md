# ADR-0005 — Agent runtime: build our own loop, BYO API key / local

**Status:** Accepted (ratified 2026-06-04; status normalized at vendoring — see README.md) (grounded in web-verified June-2026 facts — see `research/agent-runtime-research.md`)

## Context
The product's AI layers (`03b`) need autonomous, server-side, event/schedule-triggered agents (the overnight agent, the morning brief, company-aware drafting). Verified reality as of June 2026:

- A "Claude/OpenAI agent" built on the **Agent SDK runs the loop in your own process**; the vendor only does inference. So "independent in-system agents" are mostly **our** code with a swappable model engine.
- **Anthropic's June 15, 2026 change** moves programmatic use to a metered credit pool and **explicitly forbids subscription (OAuth) auth for the Agent SDK** — API key required. So "run autonomous agents on the customer's Claude/Copilot *subscription*" is vendor-blocked.
- **Vendor-hosted autonomous agents** (Copilot cloud agent on GitHub Actions; Cursor background agents on Cursor cloud) run on *their* infra; you delegate tasks, you don't host them.
- **Managed Agents** (Anthropic) / **Agent Builder** (OpenAI) offer hosted orchestration, with the sandbox either vendor-cloud or **self-hosted on your infra**.

## Decision
1. **Build our own thin, provider-agnostic agent loop** (the "first-party runner") on top of the Agent SDK / raw APIs, with a swappable model `Client` (Anthropic / OpenAI / Gemini / **local** Gemma/Llama, Mistral EU alt; Qwen config-selectable, not a default per `ADR-0012`/A23 — German-market trust). The loop, triggers (River), memory, tools (MCP over the context graph), and governance (approval inbox, two-tier autonomy, audit) are **ours**.
2. **BYO model = API key or local endpoint.** Not subscriptions (vendor-blocked for the SDK). State this plainly to customers.
3. **Local model is a first-class engine**, not a fallback — it is both the sovereignty path (P7) and a cost path (no per-token fees, no credit-pool limits).
4. **Expose an `Agent Worker Protocol`** so external vendor agents (Copilot cloud agent, Cursor) plug in as **delegated workers** for the tasks they fit — chiefly coding/customization (which is where their seats live), via MCP + GitHub.
5. **Do NOT outsource core orchestration to Managed Agents/Agent Builder** by default — it ties orchestration to one vendor and fights provider-agnosticism + local models. Managed Agents is an allowed *accelerator* and, if used, **only with the self-hosted sandbox** so customer data stays on customer infra.

## Consequences
- **Positive:** full control of the autonomous loop; provider- and host-agnostic; local-model autonomy for regulated/cost-sensitive clients; clean BYO economics (customer's API key = customer's token cost, ties to `09`); not exposed to vendor subscription/credit policy churn (e.g. the June 15 change).
- **Negative / cost:** we build and maintain the loop, memory, retries, context management (the Agent SDK does much of this, so the cost is bounded). We must track fast-moving vendor APIs (verify before each integration).
- **Security:** an external delegated worker (or a Managed-Agents cloud sandbox) reading the context graph is the `05` prompt-injection/egress surface — delegated workers get scoped tools + egress controls, and Managed Agents (if ever used) must use the self-hosted sandbox.
- **Messaging:** "bring your own agent" is precise = bring your API key or local model for autonomy; delegate your Copilot/Cursor for coding tasks. We never claim to run agents on a customer's chat subscription.

---

## Amendment 1 (2026-06-03) — the runner is plural; the connector is the moat

**Trigger:** hands-on review of **Claude Desktop "Routines"** (the shipped no-code scheduled-agent builder: Name + Instructions + model + cron trigger + **Connectors** + **Permissions** + a managed **Environment**). A non-technical user built our *Morning Brief* in it in ~90 seconds. This is Managed Agents, productized as GUI, already in users' hands — and it sharpens, not contradicts, the original decision.

**What it changes:** the original ADR framed the headline as *"build our own loop."* That over-weighted the runner. The correct headline is **"own the governed connector over the context graph; the runner is plural and mostly not ours."** The loop was never the differentiator — the **connector + governance** is. A Routine (or ChatGPT, or Cursor) is useless over a CRM until our MCP connector appears in its Connectors list; once it does, our flagship "agents" (Morning Brief, Overnight Agent) ship on *Anthropic's* runner with **zero runner code from us**.

**Two surfaces, one shared tools+governance layer:**

| | **Surface A — user's agent host** | **Surface B — CRM's own server agents** |
|---|---|---|
| Runner | Claude Desktop Routines / ChatGPT / Cursor (vendor infra) | our first-party loop (Agent SDK / API / local) |
| Configured by | the individual user, per host | the company, centrally |
| Runs | on the user's vendor account | server-side, headless, multi-tenant |
| Local model / own-your-data | ❌ vendor-only | ✅ (P7) |
| Central, company-wide governance + audit | ❌ per-user permissions only | ✅ control plane |
| **What we build** | **the MCP connector + Passport scopes** | **same connector + scopes**, *plus* the loop |

**Decision deltas (additive to the five points above):**
6. **Build the governed MCP connector FIRST**, before investing further in our own loop. It is the single artifact both surfaces consume; it is where the context-graph moat and the governance live. The in-process tool registry in the spike is to be exposed as a real MCP server (`crm-agents`).
7. **Surface A is a first-class, supported path, not a fallback.** For individual users, "point your Claude Desktop Routine at our connector" is the *fastest* route to a meaningful agent over the CRM and costs us no runner code. Document it as the recommended entry path (03b "Insertion UX").
8. **Surface B (our loop) is justified only by what Surface A structurally cannot do:** local-model/own-your-data sovereignty (P7), company-level governance/audit as a control plane, and headless multi-tenant/server-side runs (e.g. brief the whole team with nobody's laptop open). Build it for those cases — not to re-implement what Routines already does well.

**Risk this surfaces (and the defense):** if our agent story collapses to "be a connector Claude Desktop points at," **Anthropic owns the user relationship and the orchestration; we become a connector vendor.** Defense = the three standing bets: the **unified context graph** (no competitor's connector has the cross-system view of CRM+Dispact+email+calls+invoices), **central governance + local + own-your-data** (impossible in per-user Routines), and the **open-source + consulting GTM**. If those hold, "the connector everyone points their agent at" is a strong position. If they don't, no amount of runner code saves it. This is the bet to watch.

**Status:** these deltas are **Accepted** as the build ordering for the agent work; the original five points stand unchanged.

---

## Amendment 2 (2026-06-03) — Surface A splits: local (A1) vs hosted (A2); cloud routines force us to *operate* the connector

**Trigger:** wired the spike's stdio MCP connector into Claude Desktop. It connects and works **in chat and local routines** — but it **cannot** be selected in a **cloud Routine** (the always-on, scheduled kind that runs when the laptop is closed). Confirmed from logs: the local binary handshakes fine; cloud routines simply can't reach a binary on the user's machine.

**Finding — Surface A is two surfaces:**

> **Amendment 3 (2026-06-17, A35/ADR-0027):** the hosted A2 connector is **operated by the instance operator — a hosting partner or the self-hosting customer — not by Gradion.** Gradion *ships* `cmd/crm-mcp-http` and the governance; it does not run the service. Everywhere below that says "we host / ours to operate," read "**the operator runs it** (partner or customer)." The moat is owning the *governed connector design*, not running servers.

| | Transport | Where it works | Infra the operator runs |
|---|---|---|---|
| **A1 — local connector** | stdio binary (`cmd/crm-mcp`) | chat, **local** routines (awake-only) | none |
| **A2 — hosted connector** | HTTPS + OAuth 2.1 (`cmd/crm-mcp-http`) | **cloud routines**, web, mobile | **the operator runs it 24/7** (partner or self-hoster — A35) |

**The load-bearing correction:** Amendment 1 said "zero runner code." True. But the autonomous, always-on agent the product actually promises (Morning Brief at 07:30 whether or not anyone's laptop is on) is a **cloud routine**, and a cloud routine can only use an **A2 hosted connector**. Therefore:

> **"Zero runner code" ≠ "zero infra." The runner is Anthropic's; the connector is ours to SHIP and the operator's to RUN — a public, authenticated, always-on HTTPS MCP service.**

This is not a code problem (tools + governance already exist); it is an **ops commitment by the operator** (a hosting partner, or the customer who self-hosts — A35/ADR-0027). The "always-on" capability is structurally a hosting capability — something must run 24/7, run by the operator, never by Gradion. A tunnel (cloudflared/ngrok) lets us *test* a cloud routine against a local server but is not always-on (laptop must be up); only real hosting delivers the promise.

**Decision deltas (additive):**
9. **The product must ship a hosted MCP connector (A2)** — HTTPS Streamable-HTTP transport + OAuth 2.1 — as a first-class, supported service **that the operator runs** (partner-hosted or self-hosted; never Gradion-operated — A35). A1 (local stdio) remains for chat/dev/local-routine use; A2 is what makes autonomous scheduled agents real.
10. **A2 is the natural home for the "Agent Seat Passport"** (auth + per-connection scopes, 03b) and the `05` egress controls — because A2 is the connection a *remote* runner makes into customer data, exactly the prompt-injection/exfiltration surface. Auth/scoping is therefore not optional polish on A2; it is the gate.
11. **This sharpens the moat statement once more:** not "operate the connector ourselves" but **"own the hosted, *governed* connector design"** — the uptime/auth/scoping/audit/residency machinery — which the **operator** runs. That governance is defensible work an incumbent's per-user OAuth connector list does not replicate, and it is where own-your-data / on-prem (P7) lives: a regulated client runs *their own* A2 instance; a partner runs A2 for its tenants.

**Status:** Accepted. A runnable A2 spike (`cmd/crm-mcp-http`, tunnel-testable) is the proof; production hosting + OAuth is the follow-on ops decision.
