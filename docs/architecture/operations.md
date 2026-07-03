---
derives-from:
  - margince specs/spec/narrative/06-nonfunctional.md#63-the-three-deployment-modes--the-customization-posture
  - margince specs/spec/narrative/06-nonfunctional.md#66-observability
  - margince specs/spec/narrative/06-nonfunctional.md#67-scalability
  - margince specs/spec/narrative/06-nonfunctional.md#68-backup--disaster-recovery
  - margince specs/spec/narrative/06-nonfunctional.md#610-operational-runtime-addendum-deep-red-team-2026-06-23
  - margince specs/spec/architecture/09-build-release-config.md#part-a--build-determinism--releaseversioning-m6
  - margince specs/spec/architecture/09-build-release-config.md#part-b--config--secrets-plumbing-as-code-structure-m7
---
# Operations — Gradion ships software; an operator keeps it alive

> One codebase, three deployment modes, and a hard division of labor: Gradion
> publishes signed releases and never runs a server; a hosting partner or the
> customer operates the deployment. This chapter owns how the product is deployed,
> configured, observed, and recovered — the operational contract every mode
> inherits.

## What it's for

Every promise the product makes — tenant isolation, honest AI posture, data
residency, recoverability — is ultimately kept or broken at run time, by whoever
operates the deployment. This chapter states the operational doctrine so that an
operator, a worker building the plumbing, and an auditor all read the same rules:
which deployment modes exist and who runs each, how a deployment is configured,
what every service must emit so the system is observable, and what "kept alive"
means as measurable recovery targets. It is the home of the operational pins the
audit-observability subsystem and the platform plumbing implement.

## Principles it serves

- **P4 — budgets and observability are requirements.** Dashboards, metrics, and
  recovery targets are contract obligations, not aspirations; a breach alarms.
- **P7 — self-hostable, exportable, no lock-in.** Every mode up to air-gapped is a
  first-class deployment; export in open formats is also the recovery path of last
  resort (OPS-DR-9).
- **P12 — governance designed in.** Observability doubles as the trust surface:
  every action attributable, every trace replayable, no PII leaking through logs.
- **ADR-0027 — Gradion operates nothing.** "SaaS" means partner-hosted; the
  operational obligations in this chapter bind the operator contractually.
- **ADR-0023 — signed releases, graded delivery.** Delivery differs per mode, but
  every mode consumes the same signed artifact.

## Deployment modes — who runs what

The product is the same codebase in all three modes; the mode determines how much
source-level freedom a client has and who carries the pager. **Gradion operates
none of them** (OPS-MODE-4): partner-hosted multi-tenant "SaaS" is run by a hosting
partner in the partner's EU region (OPS-MODE-1); dedicated and on-prem instances —
including air-gapped ones — run on partner-managed or customer infrastructure
(OPS-MODE-2); source-delivered clients own the repository and their own
infrastructure outright (OPS-MODE-3). Gradion ships software, publishes signed
releases, and holds no customer data in any tier.

Customization posture follows tenancy (OPS-MODE-5): heavy source customization and
shared multi-tenant hosting are never offered in the same mode. The partner-hosted
tier gets bounded configuration and vertical templates — the register in the
runtime-config chapter — and never a per-tenant source fork; full agent-driven
source customization is a single-tenant property of the dedicated and
source-delivered modes.

Delivery follows the same split. Partner-hosted deployments are continuously
deployed by the partner with no client-visible version; dedicated instances take
managed pushes on a schedule, with air-gapped sites importing a signed offline
bundle into their private mirror; source-delivered forks pull a signed tag through
the upgrade preflight (OPS-MODE-1..3). Releases move on independent version lines
for the binary, the wire contract, and the customization seams, on a regular stable
cadence with a long-lived support track for regulated deployments — the signing,
provenance, SBOM, and patch-SLA machinery behind those releases is owned by the
security chapter and not restated here.

## Configuration doctrine — config is loaded once, validated, and never secret

A deployment is configured through one loading layer with a fixed precedence:
explicit flags override environment variables, which override the deployment's
config file, which overrides compiled-in opinionated defaults (OPS-CFG-1). Config
is read once at the composition root into a typed, validated structure; modules
receive their slice by injection and never read the environment directly, and
validation is fail-fast — an unknown key, a missing required value, or an
impossible combination aborts startup with a clear error (OPS-CFG-2).

The single sovereignty switch is the profile selector. It is a location choice,
not a redaction setting: the sovereign profile forces local models on every tier
under a hard egress deny (OPS-CFG-4), the EU-hosted profile — the partner-hosted
default — routes to EU-hosted open-weight models with no US frontier egress
(OPS-CFG-5), and the cloud-frontier profile allows US frontier models under the
customer's DPA with secret stripping still applied (OPS-CFG-6). A sovereign
deployment cannot be misconfigured into egress: any cloud-tier model binding under
the sovereign profile is rejected at startup (OPS-CFG-7).

No secret value ever lives in a config layer — flags, environment, and files carry
only references into a secret store (OPS-CFG-3). Credentials are injected at the
tool-execution boundary, after the model has decided to act, so the model never
holds them and they never enter a context window; a secret stripper additionally
scrubs any credential that leaked into captured text bound for a model
(OPS-CFG-8). Finally, the bright line: everything in this section is operational
configuration — it shapes whether and where the software runs, is set by the
operator, and is exempt from the product-config discipline; anything that shapes
what the product does for users belongs to the runtime-config register instead
(OPS-CFG-9).

## Observability doctrine — one log shape, one metric set, one trace

Observability is a first-class ops surface and simultaneously the trust and
compliance surface. The audit log — append-only, tamper-evident, covering human
and agent actions alike — and the per-record capture provenance are owned by the
data-model chapter; agent-action provenance (the replayable trace of agent, human
authority, inputs, tool calls, outputs, approval state) is owned by the security
and agent chapters. This chapter owns the telemetry those surfaces ride on.

Logs are structured JSON, one object per line, with no free-text logging in
services (OPS-LOG-12). Every line carries the pinned base field set — timestamp,
level, message, trace and span identifiers, the domain correlation identifier, a
cardinality-safe workspace identifier, the acting principal, the emitting module,
and error fields on failures (OPS-LOG-1..11) — injected by request and consumer
middleware, never hand-rolled by application code. Redaction is mandatory:
auto-captured, untrusted, PII-bearing content is never logged in the clear, on any
field (OPS-LOG-13). Traces survive async hops: the event envelope carries the
standard trace context alongside the domain correlation identifier, so one trace
spans outbox, relay, stream, and consumer unbroken, and a long-running agent run
reads as one trace with a span per reason-act cycle (OPS-LOG-14).

Metrics are a named, closed Prometheus set — request latency, job-queue depth,
approval-queue age, outbox depth, consumer lag, and AI token and cost counters
(OPS-MET-1..7) — exposed by every service on the standard exposition endpoint
(OPS-MET-8). Labels are bounded by rule: never a workspace or entity identifier;
per-tenant detail belongs to logs and traces, so metric cardinality cannot explode
at multi-tenant scale (OPS-MET-9). The performance budgets those metrics watch are
owned by the acceptance-standards chapter ([[acceptance-standards#PERF-1]]..7);
their live counterpart is the p95 dashboard with alerting on budget breach, and
per-workspace AI cost telemetry feeds the cost guardrails.

## Scalability posture — stateless replicas, async by default

The backend scales vertically first — one fast binary — and horizontally as
stateless application replicas behind a load balancer, with all state in Postgres,
the cache, and object storage. Anything that is not interactive-latency work —
capture, enrichment, re-embedding, agent runs — moves through the Postgres-backed
job queue and executes in the worker process, protecting the interactive budgets.
Capacity is planned against the pinned volume tiers
([[acceptance-standards#AS-SCALE-2]]..4), and the budgets must hold at the
mid-market tier ([[acceptance-standards#AS-SCALE-3]]); noisy neighbors in the
multi-tenant mode are contained by per-tenant rate and budget limits owned by the
api-conventions chapter. The relational core plus the vector extension is the
context substrate; a dedicated graph store is trigger-gated on the pinned
context-assembly budget ([[acceptance-standards#PERF-7]]), never roadmapped.

## Queue and stream hygiene — the honesty rules for the bus

Every domain mutation is write-amplified by design: the row, the audit record, and
the outbox row commit together, then a relay hops the event onto the stream for
consumers. The operational rules keep that machinery honest. The outbox is a
drain, not an archive — drained rows are trimmed on a schedule with vacuum tuning
to match, and the audit log remains the permanent record for replay and rebuild
(OPS-QUEUE-1). The relay is bounded and never lossy: past a depth watermark it
sheds to a slower cadence and raises an ops alarm, but it never drops an event
(OPS-QUEUE-2). Stream retention is sized to at least the 72-hour horizon
(OPS-QUEUE-3), and a trim that would discard an un-acknowledged pending entry is
blocked outright — no silent event loss (OPS-QUEUE-4). The whole
three-writes-plus-relay path carries a quantified write-amplification budget: it
must stay inside the pinned save budget at the pinned benchmark seed
(OPS-QUEUE-5).

## Disaster recovery — targets are contract, drills are quarterly

Recovery is a contract obligation, not an intention. The targets are pinned: at
most one hour of data loss via point-in-time recovery (OPS-DR-1) and at most four
hours to restored service (OPS-DR-2), proven by quarterly restore drills — an
untested backup is treated as no backup (OPS-DR-3). Because Gradion operates no
infrastructure, these targets bind the hosting partner through the operational
contract, together with encrypted off-site retention, a DR runbook, and the
operator-filled availability attestation in the compliance pack (OPS-DR-4); the
compliance pack itself is owned by the security chapter. Self-hosted modes receive
a documented backup procedure and own their DR and targets (OPS-DR-5).

Restore semantics are part of the promise: a restored backup re-applies the
erasure suppression list, so PII erased under GDPR cannot resurrect from a backup
(OPS-DR-6). Schema migrations are versioned and reversible, tested against a seed
database — recovery covers agent-introduced schema change, not just
infrastructure failure (OPS-DR-7). Object storage is versioned and backups are
encrypted and restore-tested in the operator's EU region (OPS-DR-8), and the
trivial full export in open formats stands as the recovery and exit path of last
resort (OPS-DR-9).

## Secrets and storage operations

Per-workspace connector credentials are envelope-encrypted rows — a per-workspace
data key wraps each token, and a KMS-held master key wraps the data key — scoped,
rotatable, and revocable per workspace; the store is the operator's duty but this
shape is mandated, because generic orchestrator secrets do not model per-workspace
tokens at thousands-of-tenants cardinality (OPS-STOR-1). Vector search never
post-filters: the shared embedding index applies the workspace as a pre-filter
predicate, partitioning per tenant if recall collapses (OPS-STOR-2), and re-embed
storms are throttled through a content-hash cache and batched jobs billed to the
customer's inference budget (OPS-STOR-3). Consumers are idempotent through one
shared library — an event-identity guard, the bus's dedupe window, and
version-keyed last-writer-wins where order matters — with expensive work
additionally guarded by content hash so a redelivery is a no-op, not a re-spend
(OPS-STOR-4). And a writer always reads its own write: the commit path busts the
entity's read-model cache key synchronously, with other readers converging via
the async consumer (OPS-STOR-5).

## Out of scope

- **CRA conformity, SBOM, signed-release and provenance machinery, CVD, and patch
  SLAs** — the security chapter.
- **Performance budget values and volume tiers** — the acceptance-standards
  chapter; this chapter only cites them.
- **Table shapes**, including the outbox table — the data-model chapter
  ([[data-model#DM-DDL-9]]).
- **The event catalog, envelope, and retention-window definition** — the event-bus
  chapter.
- **Product-behavior configuration** — the runtime-config chapter; this chapter
  owns only the operator-facing layer.

## Where it lives

Config loading, logging, and the metrics plumbing live in the shared platform
packages under the backend's internal platform directory (config, logger,
httpserver); the outbox, relay, and stream plumbing live in the platform events
package; async jobs execute in the worker process under the backend's worker
command directory. Read next: the event-bus chapter for the bus semantics this
chapter keeps healthy, the runtime-config chapter for the product-config
boundary, and the security chapter for the release and conformity machinery.

## Appendix

### Parameters — deployment modes
Source: margince specs/spec/narrative/06-nonfunctional.md#63-the-three-deployment-modes--the-customization-posture @ 5a0b29c

| ID | Mode | Operated by | Customization posture | Delivery mechanism |
|---|---|---|---|---|
| OPS-MODE-1 | Partner-hosted ("SaaS") multi-tenant — shared core on a hosting partner's infra, EU region; row/schema-level tenant isolation | Hosting partner (the customer's data processor); never Gradion | **Bounded only:** deliberately exposed configuration (the runtime-config register) + vertical templates; **no source forking** — one shared binary | Continuous-deploy **partner-push**: Gradion publishes signed releases, the partner deploys them automatically; no client-visible version |
| OPS-MODE-2 | Dedicated / on-prem — single-tenant instance on partner-managed or customer infra, incl. air-gapped; physical isolation | Partner/operator or the customer | **Full source customization via agents** in the client's own instance; the test suite + review gate are the guardrail | **Managed-push** on a schedule by the partner/operator; air-gapped: a **signed offline bundle** imported into the client's private mirror |
| OPS-MODE-3 | Source-delivered — client owns the repo + their own cloud/on-prem project; physical isolation | The customer | **Maximal:** client owns and forks the source; Gradion consulting is advice, not operations | **Client-pull of a signed tag**, consumed by the upgrade preflight; core/custom separation + upgrade agent |

| ID | Rule |
|---|---|
| OPS-MODE-4 | **Gradion operates none of the modes** (A35/ADR-0027): "SaaS/managed" is partner-hosted; dedicated/on-prem and source-delivered are customer-self-hosted. Gradion ships software and publishes signed releases only — it never runs the servers and holds no customer data in any tier. |
| OPS-MODE-5 | **Source customization and shared multi-tenant hosting are never offered in the same mode** (P1/P2/P7): source freedom is a single-tenant property (OPS-MODE-2/3); the partner-hosted tier gets bounded configuration + vertical templates instead (the runtime-config register). No runtime metadata engine exists in any mode. |

### Parameters — config precedence & profiles
Source: margince specs/spec/architecture/09-build-release-config.md#b1-the-config-loading-layer-precedence; #b2-the-profile-selector--egress-posture--model-routing-a8-ladder; #b3-the-credential-injection-seam-credentials-never-enter-model-context; #b4-opsdeploy-flags-vs-product-runtime-config-disambiguation--review-correction-f @ 5a0b29c

| ID | Rule |
|---|---|
| OPS-CFG-1 | Config precedence, highest wins: **explicit flags** (operator override) → **environment variables** (`CRM_*`, the container/orchestrator surface) → **config file** (checked-in defaults per deployment) → **compiled-in defaults** (the opinionated baseline; P1 — config is earned, not the default). |
| OPS-CFG-2 | Config is loaded **once at the composition root** into a typed, validated struct; modules receive their slice by dependency injection and never read the environment directly. Validation is **fail-fast**: an unknown key, a missing required value, or an impossible combination aborts startup with a clear error. |
| OPS-CFG-3 | **No secret values in any config layer** — flags/env/file carry only *references* into a secret store (e.g. a vault ref). No secrets in source or images; the repo carries only refs. |
| OPS-CFG-4 | `profile: sovereign` — chat-tier bindings **forced local** for every tier; **hard egress-deny**; cloud route → local fallback → honest degrade. Self-contained, air-gap-capable. |
| OPS-CFG-5 | `profile: eu_hosted` — **the partner-hosted default**; EU-hosted open-weight models; egress stays in the EU, no US frontier; EU processor, in-region. |
| OPS-CFG-6 | `profile: cloud_frontier` — US frontier models under the customer's DPA; data flows; the secret stripper still runs on every cloud egress. The customer is the controller. |
| OPS-CFG-7 | A sovereign deployment cannot be misconfigured into egress: under `profile: sovereign` any cloud-tier model binding is **rejected at startup** (the fail-fast rule of OPS-CFG-2, enforced in code, not docs). |
| OPS-CFG-8 | **Credential-injection seam:** the model orchestrates but never holds credentials. A connector declares a secret *ref*, resolved from the workspace-scoped secret store **at the tool-execution boundary**, after the model decides to call the tool; credentials never enter model context. The secret stripper scrubs credentials that leaked into captured text on every model-bound payload — hygiene, not PII redaction. |
| OPS-CFG-9 | **Ops flags vs product config:** operational/deploy configuration (the profile selector, kill switches, rollout gates, endpoints, topology) shapes *whether/where the software runs*, is operator-set through this layer, and is **P1-exempt** — it never appears in the runtime-config register. Anything that shapes *what the product does for users* is product runtime config and must be a register row ([[runtime-config#RC-REG-1]]). |

### Parameters — DR
Source: margince specs/spec/narrative/06-nonfunctional.md#68-backup--disaster-recovery; #610-operational-runtime-addendum-deep-red-team-2026-06-23 @ 5a0b29c

| ID | Name | Value | Meaning |
|---|---|---|---|
| OPS-DR-1 | RPO | ≤ 1 h | Maximum data loss on recovery; point-in-time recovery via WAL archiving |
| OPS-DR-2 | RTO | ≤ 4 h | Maximum time to restored service |
| OPS-DR-3 | RESTORE_DRILL_CADENCE | quarterly | Restore drills are part of ops; an untested backup is treated as no backup; a DR runbook is required |

| ID | Rule |
|---|---|
| OPS-DR-4 | Because Gradion operates no infra (ADR-0027), OPS-DR-1..3 are **binding obligations in the partner-hosting operational contract**: backups, encrypted off-site retention, quarterly restore drills, a DR runbook. The compliance pack (owned by the security chapter) includes a **DR/availability attestation slot the operator fills**. |
| OPS-DR-5 | Self-hosted modes (OPS-MODE-2/3): a **documented backup procedure ships with the deployment**; the customer owns DR and sets their own targets — Gradion ships the tooling. |
| OPS-DR-6 | **Erasure-vs-restore:** a restored backup **re-applies the erasure suppression list** (A13) so erased PII cannot resurrect. |
| OPS-DR-7 | Schema migrations are **versioned and reversible**, tested against a seed database — DR for agent-introduced schema changes, not just infra failure. |
| OPS-DR-8 | Postgres backups are automated (PITR per OPS-DR-1); object storage is **versioned**; backups are encrypted and restore-tested in the operator's EU region. |
| OPS-DR-9 | **Trivial full export in open formats** (P7) is both the anti-lock-in feature and the recovery/exit path of last resort. |

### Parameters — secrets & storage ops
Source: margince specs/spec/narrative/06-nonfunctional.md#610-operational-runtime-addendum-deep-red-team-2026-06-23 @ 5a0b29c

| ID | Rule |
|---|---|
| OPS-STOR-1 | Per-workspace connector credentials are **envelope-encrypted rows**: a per-workspace DEK wraps the token; the DEK is wrapped by a KMS/secrets-manager master key. Scoped, rotatable, revocable per workspace. The store is the operator's duty, but this shape is **mandated, not convention** — orchestrator-native secrets do not model per-workspace tokens at thousands-of-tenants cardinality. Credentials reach code only through the injection seam (OPS-CFG-8, ADR-0018). |
| OPS-STOR-2 | **Vector-index workspace pre-filter:** the shared embedding table's HNSW index applies `workspace_id` as a **pre-filter** (a composite predicate the planner uses, or per-tenant partitioning if recall collapses) — never a post-filter, avoiding the post-filter recall cliff. |
| OPS-STOR-3 | **Re-embed throttle:** the re-embed storm (capture backfill, content edits) is throttled via a **content-hash cache + job-queue-batched re-embeds**, billed to the customer's inference budget. HNSW build memory and insert cost belong to the vector work-package perf model. |
| OPS-STOR-4 | **Consumer idempotency is one shared library**, never per-consumer reinvention: a processed-event-id guard + the bus dedupe-window TTL (value owned by the event-bus chapter) + version-keyed last-writer-wins for order-sensitive consumers. Expensive idempotent work (e.g. re-embedding) additionally guards on content hash so a redelivery is a no-op, not a re-spend. |
| OPS-STOR-5 | **Read-your-writes:** a write busts the entity's read-model cache key **synchronously in the commit hook** (not only via the async read-model consumer), so the writer reads its own write; other readers converge eventually. Connection pooling is transaction-mode ([[data-model#DM-CONV-7]]), compatible with transaction-scoped settings. |

### Wire — structured log schema
Source: margince specs/spec/narrative/06-nonfunctional.md#610-operational-runtime-addendum-deep-red-team-2026-06-23 @ 5a0b29c

Fixed base fields — every log line carries all of them (error fields on errors only):

| ID | Field | Presence | Meaning |
|---|---|---|---|
| OPS-LOG-1 | `ts` | always | RFC 3339 timestamp |
| OPS-LOG-2 | `level` | always | severity level |
| OPS-LOG-3 | `msg` | always | the message (structured; never free text carrying data) |
| OPS-LOG-4 | `trace_id` | always | from the active W3C `traceparent` |
| OPS-LOG-5 | `span_id` | always | from the active W3C `traceparent` |
| OPS-LOG-6 | `correlation_id` | always | the domain event-envelope id |
| OPS-LOG-7 | `workspace_id` | always | bucketed/hashed where raw values would explode cardinality |
| OPS-LOG-8 | `actor` | always | `human:<id>` / `agent:<passport>` / `system` |
| OPS-LOG-9 | `module` | always | the emitting module (`crm-*`) |
| OPS-LOG-10 | `error` | errors only | the error message |
| OPS-LOG-11 | `error_class` | errors only | the stable error class |

| ID | Rule |
|---|---|
| OPS-LOG-12 | Logs are **JSON via Go `log/slog`**, one object per line; **no free-text logs in services**. Request/consumer middleware injects the trace and correlation fields (OPS-LOG-4..6); application code never hand-rolls them. |
| OPS-LOG-13 | **PII redaction is mandatory:** auto-captured, untrusted, PII-bearing content is never logged in the clear — the redaction path applies to **every logged field**. Logging raw captured email or transcript bodies would be a GDPR breach. |
| OPS-LOG-14 | **Tracing spans async hops:** the event envelope carries **W3C `traceparent`** alongside the domain `correlation_id`, so one trace spans outbox → relay → stream → consumer unbroken; a long-running agent run (including its suspend window) is one trace, with a span per reason-act cycle. (The envelope definition is owned by the event-bus chapter.) |

### Wire — metrics
Source: margince specs/spec/narrative/06-nonfunctional.md#610-operational-runtime-addendum-deep-red-team-2026-06-23 @ 5a0b29c

The named V1 metric set:

| ID | Metric | Type | Labels | Meaning |
|---|---|---|---|---|
| OPS-MET-1 | `http_request_duration_seconds` | histogram | `route`, `method`, `status` — never a raw id | request latency, the live face of the interactive budgets |
| OPS-MET-2 | `job_queue_depth` | gauge | `queue` | queued jobs per queue |
| OPS-MET-3 | `approval_queue_age_seconds` | histogram | — | age of pending approvals |
| OPS-MET-4 | `event_outbox_depth` | gauge | — | undrained outbox rows (table shape: [[data-model#DM-DDL-9]]) |
| OPS-MET-5 | `consumer_lag_seconds` | gauge | `consumer` | stream-consumer lag |
| OPS-MET-6 | `ai_task_tokens_total` | counter | `tier` | tokens spent per model tier |
| OPS-MET-7 | `ai_task_cost_minor_total` | counter | `tier` | AI cost in minor units per model tier |

| ID | Rule |
|---|---|
| OPS-MET-8 | Every service exposes **Prometheus exposition at `GET /metrics`** (OpenTelemetry SDK → Prometheus exporter). |
| OPS-MET-9 | **All labels are bounded.** No `workspace_id` or entity-id labels, ever — per-workspace and per-entity detail goes to logs and traces; metric cardinality is bounded/bucketed at multi-tenant scale. |

### Limits — queue & stream hygiene
Source: margince specs/spec/narrative/06-nonfunctional.md#610-operational-runtime-addendum-deep-red-team-2026-06-23 @ 5a0b29c

| ID | Rule |
|---|---|
| OPS-QUEUE-1 | The event outbox is **drained-then-deleted, never retained**: a scheduled trim plus vacuum tuning keep the high-churn table healthy. The audit log — not the outbox — is the permanent record for replay/rebuild. |
| OPS-QUEUE-2 | The relay has a **bounded queue depth**; past a watermark it **sheds to a slower cadence and raises an ops alarm**. It **never drops events**. |
| OPS-QUEUE-3 | Stream retention: `MAXLEN` is sized **≥ the 72 h retention horizon** (the horizon is defined by the event-bus chapter). |
| OPS-QUEUE-4 | A stream trim that would pass an **un-acked PEL entry is blocked** — no silent event loss. |
| OPS-QUEUE-5 | **Write-amplification budget:** every domain mutation costs ≥ 3 writes (row + audit + outbox) plus a relay read/write and N consumer reads, all on the one Postgres. The full path must stay within the save budget [[acceptance-standards#PERF-4]] at the benchmark seed [[acceptance-standards#AS-SCALE-1]]; the quantified budget is part of the perf-test work package. |
