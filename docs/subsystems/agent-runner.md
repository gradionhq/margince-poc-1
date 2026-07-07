---
status: planned
module: backend/internal/modules/agents (runner + run scheduling; no UI of its own)
derives-from:
  - specs/spec/architecture/07-surface-b-runner.md#1-where-it-lives-and-the-invariant-it-must-not-break @ 5a0b29c
  - specs/spec/architecture/07-surface-b-runner.md#4-budget-termination-and-graceful-degrade @ 5a0b29c
  - specs/spec/architecture/07-surface-b-runner.md#5-the--mid-loop-handoff-suspend-never-block @ 5a0b29c
  - specs/spec/narrative/03c-agentic-concept.md#32-surface-b-resident-the-agent-loop-runs-inside-the-system @ 5a0b29c
  - specs/spec/narrative/03c-agentic-concept.md#41-layered-structure-top-to-bottom @ 5a0b29c
  - margince-poc/docs/subsystems/agent-runner.md @ a11d6c08
---
# Agent runner — a first-class agent, never a privileged one

> The resident reason-act-observe loop that does the product's headless judgment work —
> the overnight sweeps and standing automations that must run with nobody's laptop open.
> Its one promise: the runner gets **no more authority and no less oversight** than an
> outside agent reaching in, and no run it starts can ever be unbounded, unaccountable,
> or blocked waiting on a human.

## What it's for

Some of the product's most valuable work happens when no human is present: the overnight
at-risk sweep, the reconciliation pass, a standing automation reacting to a deal going
quiet. No outside assistant wakes itself to do this; server-side, scheduled, org-wide
proactive work — including in sovereign deployments — can only originate inside the
system. The runner is the engine that executes it: given a triggered goal, it reasons,
proposes a tool call, observes the result, and repeats until it has an answer or a
ceiling fires. Its consumers are the overnight-agent chapter (whose background runs and
reconciliation work it executes) and the automation chapter (whose judgment-shaped
standing automations it carries); this chapter owns only the execution mechanics — the
loop, its governance, its budgets, and its suspend/resume behaviour — never the goals or
content of the runs. Predictable, enumerable automations stay on the deterministic
workflow path owned by the automation chapter; the runner exists solely for work that
needs judgment (ADR-0005).

## Principles it serves

- **P12 — Governance is designed in.** Every tool call the runner proposes clears the
  same admission gate as any other caller, writes the same audit row, and stages the
  same approval item; supervision is structural, not advisory.
- **P6 — Embrace the LLMs; don't fight them.** The loop contains no intelligence — it
  calls a swappable brain and confines it with governed tools, so a commodity model over
  grounded context does real work safely.
- **P7 — Own your data.** Headless runs work with a local brain in the sovereign
  zero-egress profile; proactive autonomy never requires a cloud dependency.

Provenance: ADR-0005 (our thin, provider-agnostic loop — justified only by what the
inbound surface structurally cannot do), ADR-0009 and ADR-0013 (two surfaces, one
governed tool surface and one audit stream), ADR-0026 (per-tool autonomy tiers),
ADR-0036 (the approval token the resumed step presents), ADR-0020 (customer-paid
inference — why the cost ceiling is hard), ADR-0014 (the runner imports seams; nothing
in the core imports the runner).

## How it works

**The alarm clock is not the worker.** The platform's Postgres-backed job queue (the
operations chapter's vocabulary) turns a schedule tick or a domain event — a deal
changing stage, an activity landing — into an enqueued run job. The queue contributes
timing, durability, retries, and idempotency, nothing more. The job is
**authority-free**: it names the workspace, the goal, the trigger reference, the budget,
and a reference to the seat the run acts under — never tools, never a model, never
credentials. Authority exists only at execution time, resolved where every agent's
authority is resolved.

**Identity, then admission, then tools — the same three layers as everyone else.** The
runner's identity is an agent-flagged seat: its passport resolves to a principal whose
effective authority is the passport's scopes intersected with the granting human's —
the *agent ≤ human* rule ([[threat-model#TM-CTRL-2]]). Each turn, the model proposes a
tool call; the runner never executes one directly. The proposal is submitted to the
admission gate — scope, autonomy tier, and budget checked before anything runs, an
audit row written for every admitted call ([[threat-model#TM-CTRL-3]]). The tools on
the far side are the **same governed tool surface** the byo-agent-and-mcp chapter owns
for outside agents, including the intent-tool bundles whose composition rules the
intent-tools chapter pins (INTENT-AC-1..3). There is no privileged registry, no back
door, no ambient credential: the runner is just another caller, and a run that could
act through any path the gate does not see would break the product's core invariant
(ADR-0013).

**The loop is grounded and bounded.** Each turn assembles a least-context window — only
the records the goal's neighbourhood needs, never the workspace (the D5 defense,
[[threat-model]]) — through the retrieval seam, with every element carrying its
provenance and trust tier; untrusted content stays labelled data-not-instructions
end-to-end (owned by the trust-propagation chapter). The model is reached through the
ai-runtime chapter's routing — a configured cloud key or a local brain; the sovereign
profile hard-blocks egress — and the egress path runs the secret-stripper the threat
model pins as D7. The observation from each admitted call feeds the next turn,
**including refusals**: a scope or budget refusal comes back as an observation the model
re-plans around, never as a crash and never as an escalation (RUNNER-AC-6).

**The governor makes runaway impossible.** Three independent ceilings bound every run:
a step ceiling checked before every model call (RUNNER-PARAM-1), a hard per-run output
cost ceiling (RUNNER-PARAM-2), and a wall-clock deadline enforced as the job timeout
(RUNNER-PARAM-3). The cost ceiling is deliberately per-run and hard — the shared
workspace session quota is soft and divisible, and a lone overnight session could
otherwise claim all of it; because inference is customer-paid (ADR-0020), the hard
ceiling is the customer-cost guarantee. Exhaustion on any axis is **not an error**: the
run degrades to the best partial result gathered so far and its record is marked
budget-terminated (RUNNER-AC-3). A per-workspace kill-switch stops all resident runs at
once (RUNNER-PARAM-4).

**A 🟡 need suspends the run — it never blocks and never bypasses.** Overnight there is
no human to ask, so when the gate answers a proposed step with needs-approval, the
runner stages the proposal into the approval inbox — diff, evidence, trust tiers, and
the target's captured version — together with a snapshot of the run's window, then
**parks the job**. No worker is held, nothing spins, and the target is not mutated. The
staging, token minting, time-to-live, fail-closed expiry, and version re-validation are
all the approvals-and-concurrency chapter's machinery, cited not restated: an approval
resumes the run from its saved window and re-submits the same step carrying the
single-use token, so the effect executes exactly once and only against unchanged state
([[approvals-and-concurrency#APPR-AC-1]], [[approvals-and-concurrency#APPR-AC-3]]); a
rejection or a TTL expiry ([[approvals-and-concurrency#APPR-PARAM-1]]) resumes with the
refusal fed back as an observation, target untouched. The suspended-then-resumed run is
one continuous trace, and resume is idempotent — a re-delivered decision never
double-resumes (RUNNER-AC-2, RUNNER-AC-5).

**Every run is on the record, and duplicates collapse.** Every step is audit-logged
before it is observed — actor, authority, tool and inputs, trust tiers touched, output,
approval state, budget charge — on the same append-only stream inbound agents write to
(the D6 defense, [[threat-model]]). The trace records the ordered
proposal-decision-result tuples plus the seed grounding and the model identity, so a
replay reconstructs every decision and effect end-to-end **without re-calling the
model** (RUNNER-AC-4). The honest limit is stated plainly: model output is
non-deterministic, so replay proves what the run did and why — it does not re-derive
the model's reasoning. Run jobs are idempotent on trigger reference and workspace, and
staged proposals dedupe on the same key, so a retried or re-delivered trigger never
starts a duplicate run (RUNNER-AC-5).

## What's configurable

- **Per-run budget** — the step ceiling (RUNNER-PARAM-1), the hard output-token ceiling
  (RUNNER-PARAM-2), and the wall-clock deadline (RUNNER-PARAM-3); each carries the
  ratified default and may be set per job, never unset — steps always have a ceiling.
- **Workspace kill-switch** — disables all resident runs at once (RUNNER-PARAM-4).
- **The brain** — injected through the ai-runtime chapter's routing: a customer cloud
  key or a local model; in the sovereign profile the brain is local and egress is
  hard-blocked. Swapping it changes nothing about triggers, tools, or governance.
- **Injected seams** — the admission gate, retrieval seam, and secret-stripper are
  construction-time dependencies; tests substitute fakes, production wires the real
  gate, and the no-privileged-path property holds either way.

## Guarantees (enforced)

- **No privileged path** — every runner tool call clears the same passport
  intersection, admission gate, tier check, and audit write as an inbound agent's call;
  there is no way to reach a tool around the gate ([[threat-model#TM-CTRL-2]],
  [[threat-model#TM-CTRL-3]]; RUNNER-AC-1).
- **Suspend, never block** — a needs-approval step stages a proposal and parks the run;
  the target changes only on an approved resume, exactly once; reject and expiry leave
  it untouched (RUNNER-AC-2).
- **Never unbounded, terminates honestly** — every run is capped on steps, cost, and
  wall clock; exhaustion degrades to the best partial result, marked budget-terminated,
  never a silent failure (RUNNER-AC-3).
- **Replayable end-to-end** — every decision and effect of a run, including across an
  approval gap, is reconstructable from the audit record without re-calling the model
  (RUNNER-AC-4).
- **Idempotent triggers and resumes** — a duplicate trigger yields one run; a
  re-delivered decision resumes once; staged proposals never double-stage
  (RUNNER-AC-5).
- **Refusals are observations** — scope and budget refusals feed the next turn instead
  of crashing the loop or escalating around it (RUNNER-AC-6).

## Acceptance

Done means an operator can trust the overnight fleet unattended: a triggered run
executes headless within its ceilings, anything needing a human waits in the approval
inbox rather than executing or blocking a worker, and the morning record shows exactly
what every run did, proposed, and was refused — including runs that ended by budget,
honestly marked. The runner owns no screen; its work becomes visible through surfaces
owned elsewhere (the approval inbox, the morning brief, the audit view), and those
chapters own the corresponding screen acceptance. The testable form of every claim here
is pinned in the Acceptance appendix; the cross-cutting floor is inherited from the
acceptance-standards chapter.

## Out of scope

The content of the runs — the overnight sweeps' goals and the daily brief — belongs to
the overnight-agent and morning-brief chapters. The bounded trigger-and-action catalog,
agent-authored standing automations, and the deterministic workflow path belong to the
automation chapter. The governed tool surface itself, session quotas, and BYO-agent
connections belong to the byo-agent-and-mcp chapter; model providers, routing, and the
AI spend guardrail to the ai-runtime chapter; approval staging, tokens, TTL, and the
inbox UI to the approvals-and-concurrency and notifications-and-approval-inbox
chapters; anomaly detection over the audit stream to the audit-observability and
security chapters.

## Where it lives

The runner and its run scheduling live in the agents module
(backend/internal/modules/agents), behind the same governed tool seam every agent
enters through; it owns no tables of its own — run state rides the job queue, the
staged approval item, and the audit log, each pinned by its owning chapter. Read next:
byo-agent-and-mcp for the tool surface the runner consumes, ai-runtime for the brain,
approvals-and-concurrency for the suspend/resume machinery, and overnight-agent and
automation for the work it executes.

## Appendix

### Parameters
Source: specs/spec/architecture/07-surface-b-runner.md#4-budget-termination-and-graceful-degrade @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| RUNNER-PARAM-1 | Step ceiling | default 40 reason-act cycles per run (RATIFY) | Checked before every model call — the loop is bounded even if the model never emits a terminal answer. Sized to one deal-bundle overnight pass. |
| RUNNER-PARAM-2 | Cost ceiling | hard 50k output tokens per run (RATIFY) | A fixed per-run **hard** ceiling, independent of active-session count — the soft, divisible workspace session quota (MCP-SESS-*, owned by byo-agent-and-mcp) is additionally in force but gameable by a lone unattended session; the hard ceiling is the customer-cost guarantee under customer-paid inference (ADR-0020). |
| RUNNER-PARAM-3 | Wall-clock ceiling | default 15 minutes per run (RATIFY) | Enforced as the job timeout; cancellation unwinds the loop at the next cancellation-aware step. |
| RUNNER-PARAM-4 | Workspace kill-switch | per-workspace on/off | Disables all resident runs at once. |

Retry/backoff: the corpus assigns timing, durability, retries, and idempotency to the
job queue and pins no numeric retry count or backoff curve — an implementation constant
of the queue, not a spec constant. The 72-hour approval TTL that bounds a suspended
run's wait is [[approvals-and-concurrency#APPR-PARAM-1]], cited not re-pinned.

### Wire
Source: specs/spec/architecture/07-surface-b-runner.md#5-the--mid-loop-handoff-suspend-never-block @ 5a0b29c

| ID | Surface | Note |
|---|---|---|
| RUNNER-WIRE-1 | none owned | The runner exposes no HTTP surface of its own: the contract defines no operationId for run inspection, run listing, or manual re-trigger. Honest gap — supervision today is the audit record and the job queue's operational surface. |
| RUNNER-WIRE-2 | approval decision path | Suspend/resume rides the approvals wire: staging and decisions via [[approvals-and-concurrency#APPR-WIRE-3]]..[[approvals-and-concurrency#APPR-WIRE-5]]; the resumed 🟡 step presents the single-use token per [[approvals-and-concurrency#APPR-WIRE-1]]. |
| RUNNER-WIRE-3 | kill-switch gap | RUNNER-PARAM-4 has no contract operation yet — no admin endpoint is defined to flip it. Honest gap flagged for the contract. |

### Events
Source: specs/spec/contract/events.md#5-the-catalog @ 5a0b29c; specs/spec/architecture/07-surface-b-runner.md#6-determinism-replay-and-audit-p12 @ 5a0b29c

Definitions live in the central event catalog ([[event-bus]]); cited, never redefined.

| ID | Direction | Events | Note |
|---|---|---|---|
| RUNNER-EVT-1 | consumes | `deal.stage_changed`, `activity.captured`, `lead.created`, `offer.accepted` | Domain triggers, consumed via the runner's consumer group [[event-bus#EVT-CG-2]] (`cg:overnight-agent`); each enqueues an idempotent run job (RUNNER-AC-5). |
| RUNNER-EVT-2 | consumes | `approval.decided` | The resume signal for a suspended run, via [[event-bus#EVT-CG-2]]; approved decisions carry the token mint, rejections resume as observations. |
| RUNNER-EVT-3 | emits | `approval.requested` | Emitted at suspend when a 🟡 step is staged; the event row is defined in the central catalog and the staging semantics are the approvals-and-concurrency chapter's. |
| RUNNER-EVT-4 | emits (via audit) | `audit.appended` | Every admitted step lands on the audit stream; anomaly detection over it is [[event-bus#EVT-CG-7]], owned by the security/audit chapters, not here. |

### Acceptance
Source: specs/spec/architecture/07-surface-b-runner.md#1-where-it-lives-and-the-invariant-it-must-not-break @ 5a0b29c; specs/spec/architecture/07-surface-b-runner.md#4-budget-termination-and-graceful-degrade @ 5a0b29c; margince-poc/docs/subsystems/agent-runner.md#guarantees-enforced @ a11d6c08

| ID | Given/When/Then | Verification |
|---|---|---|
| RUNNER-AC-1 | Given any tool call proposed by a resident run, when it executes, then it has cleared the same admission pipeline an inbound agent clears — passport-intersected authority ([[threat-model#TM-CTRL-2]]), scope ∧ tier ∧ budget admission before any effect ([[threat-model#TM-CTRL-3]]), one audit row per admitted call — and no code path can reach a tool without an admitted capability. | No-privileged-path property test wired against the real gate, agents lane, plus the admission-choke import-graph invariant. |
| RUNNER-AC-2 | Given the gate answers a proposed step with needs-approval, when the runner handles it, then the proposal is staged with its diff and window snapshot, the job parks holding no worker, and the target is not mutated; an approval resumes the same step with the single-use token and it executes exactly once (token and version-skew semantics owned by [[approvals-and-concurrency#APPR-AC-1]] and [[approvals-and-concurrency#APPR-AC-3]]); a rejection or TTL expiry resumes with the refusal observed, target untouched. | Suspend/resume integration test, agents lane, covering approve, reject, and expiry paths. |
| RUNNER-AC-3 | Given a run that exhausts any ceiling (RUNNER-PARAM-1..3), when the ceiling fires, then the run ends as the best partial result gathered so far, its record marked budget-terminated — never an unbounded loop, never a crash; the step ceiling is checked before every model call. | Budget property test (loop bounded with a never-terminating fake brain) + degrade integration test. |
| RUNNER-AC-4 | Given any finished or suspended-then-resumed run, when its trace is replayed from the audit record, then the ordered proposal → admission-decision → tool-result tuples plus seed grounding and model identity reconstruct every decision and effect without re-calling the model, as one continuous trace across the approval gap (the D6 defense, [[threat-model]]). | Replay integration test over the audit stream, agents lane. |
| RUNNER-AC-5 | Given a retried or re-delivered trigger for the same trigger reference and workspace, when jobs enqueue, then at most one run starts; a resumed run never double-stages its proposals and a re-delivered decision never double-resumes. | Idempotency integration test (duplicate trigger + duplicate decision delivery). |
| RUNNER-AC-6 | Given a scope or budget refusal from the gate mid-run, when the loop continues, then the refusal is fed back as an observation and the run proceeds within authority — the refusal neither crashes the run nor opens any path around the gate. | Refusal-as-observation integration test, agents lane. |
