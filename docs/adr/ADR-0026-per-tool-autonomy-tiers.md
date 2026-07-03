# ADR-0026 — Per-tool autonomy tiers (🟢 runs free / 🟡 asks first) + the tighten-only re-tier rule

**Status:** Accepted (Lars, 2026-06-17). Research parent: [`../../research/r6-autonomy-tiers.md`](../../research/runtime/r6-autonomy-tiers.md) (closes BACKLOG **R6**; R7–R8 remain open). Resolves the **R-C1** blocker in [`../07-risks.md`](../narrative/07-risks.md). Builds on A12 (two-tier autonomy), A14 (approval inbox + 72h expiry), A18 (intent tools), A26 (one auth model, tiers enforced server-side). New decision: **DECISIONS A34**.

## Context

Margince's whole safety story is the split between **🟢 (the agent acts on its own)** and **🟡 (the agent stops and waits for a human)**. Until now that split was a *principle*, not a *list*. Every tool needed an explicit tier, and one thing was genuinely undecided: **can a customer change the tiers on their own install?**

R6 found the spec had already tiered nearly every tool (`interfaces.md §2.1/§2.2/§5`). So this ADR mostly **confirms** the existing marks, **fills the gaps** (the irreversible actions that only lived in the OpenAPI contract), **resolves the one open question** (`advance_deal`), and **adds the missing rule** about customer re-tiering.

## Decision

**A tool's tier is a fixed property of the tool, checked by the server before the tool runs. The line is reversibility, not read-vs-write. Customers can make a tool stricter but never looser than its baseline for the dangerous actions.** Five parts:

1. **🟢 runs free** — reads, report runs, read-only "assemble a picture" tools, drafts (which never send), and **reversible internal writes**: create/edit a contact/company/deal/lead, log an activity, create a task, move a deal between *open* stages, fill qualification gaps. Reversible, logged, and an agent can't silently overwrite a field a human typed (that needs 🟡).

2. **🟡 asks first** — anything that **leaves the building** (send email, dial out, write back to HubSpot/Salesforce, reach the internet to enrich) or **can't be cleanly undone** (archive/soft-delete, merge records, disqualify a lead, **close a deal Won or Lost**). The tool returns `ErrRequiresApproval`, stages the action, and waits; unanswered approvals expire in 72h (A14) and nothing happens.

3. **`advance_deal` is the one dynamic tool** — 🟢 moving between open stages, 🟡 to Won/Lost (money + forecast + hard to reverse). This resolves the open question in `interfaces.md §6`. Everything else has a static tier.

4. **Tighten-only re-tiering (the new rule).** A customer/admin may make a 🟢 tool **stricter** (force it to ask), but may **never** lower the baseline for the always-🟡 class: **send, outbound, archive/delete, merge, disqualify, close-deal, enrich**. Those have a floor nobody can drop. Loosening below the floor is not a setting that exists. (V1 may ship with re-tiering not yet exposed; when it is, this is the rule.)

5. **The overnight agent (Surface B) follows the same tiers, unattended.** Reversible internal work runs 🟢 with a "here's what I did" summary; anything that sends or escalates is 🟡 and waits for the morning review. The gate is the same whether a human is present or not — it lives in the tool, not in the prompt.

## Consequences

- **Resolves R-C1** (the per-tool tiering blocker) and closes the matching open item in `06 §6.9`.
- **The floor is the real promise.** The defensible line for a security/regulated buyer is "send, delete, close, and merge can never be switched to run-free, by anyone" — stronger than a default that could be reconfigured away.
- **Doubles as prompt-injection defence.** Because every outbound/irreversible action stops to ask, a tricked agent (`05`) still can't send or delete on its own. R6 and the `05` threat model share one gate.
- **Rate limits ride the same table later.** Per-tool call limits (R-C5) are a separate setting but belong on the same tool list; R6 sets the tiers, not the numbers — those calibrate at the rate-limit WP.
- **No architecture change.** `ToolSpec.Tier` already exists; this fills the table, adds the re-tier rule, and decides `advance_deal`.

## Alternatives considered

- **Make every write ask first.** Rejected — the agent would interrupt for routine bookkeeping (logging a call, fixing a number) and people would disable it. Reversibility, not write-vs-read, is the right line.
- **Lock tiers fully in V1 (no customer changes ever).** Reasonable and safe, and close to today's spec; rejected as the *long-term* rule because a power user legitimately wants their agent to do more. Tighten-only gives them room without ever lowering the floor. (V1 can still ship with the control unexposed — the rule is what matters when it lands.)
- **Fully flexible re-tiering.** Rejected — letting a customer set "send email" to 🟢 invites the exact accident (an agent blasting the wrong recipients) we'd then be blamed for.
