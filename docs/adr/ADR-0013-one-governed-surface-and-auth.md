# ADR-0013 — One Core CRM, one governed surface, one network-auth model

**Status:** Accepted (2026-06-10, with Lars; Jan Moser architecture review) — DECISIONS A25, A26. Built into `interfaces.md` (the MCP tool surface + REST contract), `03b-ai-architecture.md`, `03c-agentic-concept.md`, and clarifies ADR-0005 (agent runtime) + ADR-0009 (agent-first surface).

## Context

Two review points from Jan Moser on the agentic whitepaper:
1. There should be **one Core CRM** plus **helper-tools** that ship with it, and those helper-tools should use the **same MCP + endpoints** that third-party integrations use — not a privileged internal path.
2. There should not be **two MCP paths**; one MCP path with **proper OAuth2**, and the **same security mechanism for all web access**.

The whitepaper already states the MCP server is "the single governed entry point," but it also describes A1 (local stdio MCP) and A2 (hosted HTTPS OAuth MCP) and two "surfaces/directions" (A inbound / B proactive), which read as if there might be divergent paths or auth models. Left ambiguous, that invites exactly the drift Jan warns about: a first-party feature taking a shortcut the public API doesn't expose, or two auth mechanisms maintained in parallel.

## Decision

### 1. First-party tools are clients of the public surface (A25)
There is **one Core CRM**. Every first-party feature beyond raw CRUD — the AI-native moments (morning brief, dossier, coaching, transcript→deal, etc.) — is a **client of the same governed MCP tool surface + REST/OpenAPI contract** exposed to third parties. No first-party backdoor. Consequences:
- The public API is **complete by construction** — if we needed an internal-only capability, that's a smell that the public surface is missing something.
- The intent-level tools (A18) and the CRUD tools are the seam everything uses; first-party and third-party agents are governed identically.

**Deliberate, audited exceptions — system services, not backdoors:** the **L3 auto-capture ingestion writer** (`05` D8: a system component that is not bound to a connecting human's RBAC per write, because it ingests the firehose) and **DB migrations** operate below the tool layer. These are trusted system services with their own audit trail, explicitly enumerated, not a general-purpose privileged path.

### 2. One authorization model; OAuth2 + Passport is the only network auth (A26)
- **All network-reachable access** authenticates via **OAuth 2.1 + the Agent Seat Passport** and enforces the identical mechanism: Passport scopes ⊆ the granting human's RBAC, 🟢/🟡 autonomy tiers server-side at the tool boundary, full audit/replay.
- **A1 (local stdio) and A2 (hosted HTTPS) are two transports of the same surface, not two security models.** A2 is the network path (OAuth2 + PKCE + DCR). A1 is a **local-dev-only** transport (operator's own machine, local trust) that still mints a scoped, expiring Passport. There is no second web-auth mechanism to drift.
- **Surface A (inbound — a vendor brain calls our connector) and Surface B (proactive — our loop calls a model)** both bind a Passport at the *same* MCP entry point and pass the *same* gates. They are two *directions*, one governed surface.

## Consequences

- **Positive:** the public API can't rot behind privileged internals; one auth path means one thing to secure, test, and audit; the "governed substrate any brain runs" claim (`03c`) becomes literally true because our own brain uses the same door. Easier security review (Jan's point): a reviewer audits one mechanism.
- **Ripple:** `interfaces.md` states first-party callers use the tool surface; `03b`/`03c` state OAuth2+Passport is the sole network auth and stdio is local-dev-only; ADR-0005/0009 are annotated to remove the "two paths" reading. The system-service exceptions (D8 capture writer, migrations) are enumerated wherever the "no backdoor" rule is stated, so the exception is explicit, not discovered.
- **Negative / to bound:** routing first-party features through the public tool surface can cost a little latency/ceremony vs a direct internal call. Accepted: the completeness + auditability guarantee is worth it, and the hot interactive CRUD path (record open/list/save) is the REST contract directly, not an agent tool, so user-facing latency budgets (`06 §6.1`) are unaffected.
- **Boundary:** this ADR is about the *tool/auth surface*. Model egress posture is A8 (revised); CRA conformity of a modified fork is A27/ADR-0010.
