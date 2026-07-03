---
derives-from:
  - margince-poc/docs/architecture/api-conventions.md
  - margince specs/spec/contract/README.md#conventions
  - margince specs/spec/contract/interfaces.md#0-conventions-for-every-interface-here
  - margince specs/spec/contract/api-rate-limits-and-abuse.md#1-rest-api-limits
  - margince specs/spec/features/05-notifications-and-collaboration.md#the-versionetag-approach-normative
---
# API conventions — one wire contract for humans and agents

Two kinds of caller hit this product: humans through the app, and agents through the
governed tool surface. Both speak the same wire contract, and this chapter owns the
conventions of that contract — the payload shapes, error semantics, concurrency and
idempotency rules, approval gating, and REST-side limits that hold across every
operation. The contract is authoritative (P3, contract-first): endpoints, schemas, and
their inventory live in the contract file — `backend/api/crm.yaml` in the target layout
— and when handwritten code and the contract disagree, the contract wins and a drift
check blocks the merge. Individual endpoints are never restated here or in any other
chapter; they are cited by their contract operation id.

**One shape for every payload.** Every JSON field name on the wire is snake_case,
derived mechanically from the database column names, so the generated server types and
the generated client types can never disagree about a name (API-CONV-4). Machine error
codes, tool verb names, and enum strings follow the same casing. Collections come back
in a single list envelope — a data array plus a page object carrying an opaque cursor
and a has-more flag — and pagination is cursor-based only; clients never parse a cursor
and offset pagination is not offered (API-LIST-2). Page size is bounded and defaulted
(API-LIST-1), sorting is a per-resource allow-list with a stable tie-breaker appended
so pages never shuffle (API-LIST-3), and archived rows stay out of lists unless
explicitly asked for (API-LIST-4). Updates are merge-PATCH: an omitted field is
unchanged, an explicit null clears a nullable field, and the full updated entity comes
back — there is no replace-style update (API-CONV-1).

**Errors are typed and machine-branchable.** Every failure is an RFC 7807 problem body
carrying a stable machine code alongside the human-readable parts, so an agent branches
on the code and a person reads the detail (API-ERR-6..21). Handlers never hand-write an
error body: they return a typed sentinel and one central mapper renders it for REST,
while the tool surface maps the same sentinel to its own envelope — one error
vocabulary, two transports. The authentication split is strict: 401 means the caller
is not authenticated at all (API-ERR-16), 403 means authenticated but blocked — by
role, by a missing approval token, or by seat tier (API-ERR-9, API-ERR-10, API-ERR-14).
A resource that exists but sits outside the caller's visibility answers 404 exactly as
if it did not exist, so the API never leaks existence (API-ERR-6). Two credential
schemes — an agent bearer token and a human session cookie — converge on one
authorization path, and only workspace signup and login are reachable without
credentials (API-CONV-11).

**Concurrent writers are detected, not silently merged.** Every mutable entity carries
a read-only integer version, bumped in the same transaction as each committed mutation
(API-CC-1). A mutating request states the version it read via the If-Match header —
the raw integer, not an opaque validator hash (API-CC-2). Concurrency-safe write paths
— the first-party UI and the tool surface, which is all of them — must send it; a write
without it is rejected with 428 rather than applied last-write-wins (API-CC-3). A stale
version is rejected with 409 and the response body carries the current entity and its
current version, so the caller can diff and retry without a second round-trip
(API-CC-4). The staged-approval execute path re-checks the version captured at approval
time and fails with the same error when the record moved underneath the approval
(API-CC-5, ADR-0036).

**Writes are safe to retry.** Every create accepts an idempotency key; replaying the
same key returns the original result instead of duplicating (API-CC-6). Capture paths
additionally dedupe on the source system and source id, so re-running a capture is a
no-op that answers 200 where a fresh create would answer 201 (API-CC-7). A successful
create answers 201 with a location header and the full created entity, identical to a
subsequent read (API-CONV-3).

**Archive returns the entity.** There is no hard delete. Deleting a record archives it
— it drops out of default lists but stays fetchable and audited — and the response is
200 with the full archived entity so the client can render the outcome without a
second fetch; 204 is wrong everywhere except logout, the API's single no-body response
(API-CONV-2). Disqualifying a lead rides the same delete-is-archive semantics.

**Approval gating is a wire concern.** When an agent principal invokes a
needs-approval operation, the request must carry a single-use signed approval token —
minted the moment a human approves the staged action in the approval inbox — whose
claims bind it to one action, one diff, and one record version (API-CONV-10). Without
a valid token the operation fails closed as approval-required (API-ERR-10, API-ERR-11).
A human's own direct call is itself the approval and needs no token. A floor of
always-needs-approval operations exists that no configuration can lower to
auto-execute; the tool tables, tier assignments, and that floor are owned by the
byo-agent-and-mcp chapter and cited from here, never restated.

**Domain values have one wire form.** Money is an integer minor-unit amount plus a
currency code, never a float (API-CONV-7). Timestamps are UTC RFC 3339, with created
and updated times always present and the archive time null until archived
(API-CONV-8). Identifiers are UUIDs — time-ordered underneath, but opaque to clients,
with no ordering guarantee expressed or implied (API-CONV-9). Every domain object
carries provenance — what produced it and who captured it — required on create and
returned on every response (API-CONV-6). Client-specific custom fields are real
columns that surface additively through the same contract, flagged with an extension
marker so tooling can tell core from custom (API-CONV-5).

**Limits are part of the contract.** The REST surface is rate-limited server-side at
the boundary, keyed on tenant, user, and route class — six classes tuned so
interactive use never feels them and only abnormal automation does (RL-READ-LIGHT
through RL-BULK-WRITE), plus a small set of request caps on page size, body size,
expansion depth, batch size, and query cost (CAP-*). A limit breach answers a uniform
429 contract with standard rate-limit headers and an authoritative retry hint, while a
cap breach is a client bug and answers 400 — retrying does not help (API-429-1). The
tool-surface session quotas, step-up thresholds, and the abuse/anomaly ladder are owned
by the byo-agent-and-mcp and security chapters respectively; this chapter owns only the
REST-side rows pinned below.

## Appendix

### Wire — status codes & error codes
Source: margince-poc/docs/architecture/api-conventions.md#status-codes-at-a-glance; contract/interfaces.md#0-conventions-for-every-interface-here @ 5a0b29c

Status codes by operation:

| ID | Operation | Success | Notes |
|---|---|---|---|
| API-ERR-1 | Create (POST) | 201 + `Location` header + full entity | Idempotency-key replay → original status; capture dedupe → 200 |
| API-ERR-2 | Read (GET) | 200 + entity | 404 also used for RBAC "not visible" (no existence leak) |
| API-ERR-3 | Update (PATCH) | 200 + full entity | Merge — omitted fields unchanged; no PUT |
| API-ERR-4 | Archive (DELETE) | 200 + full entity | Soft-delete; entity carries non-null `archived_at` |
| API-ERR-5 | Logout | 204 | The only no-body response in the API |

Every 4xx/5xx body is RFC 7807 `application/problem+json`: the standard
`type/title/status/detail/instance` plus a required stable machine `code` and a
structured `details` object:

```json
{
  "type": "https://errors.margince.com/<code>",
  "title": "…",
  "status": 409,
  "detail": "…",
  "code": "version_skew",
  "details": {}
}
```

Typed sentinel → HTTP status → machine code (handlers return the sentinel; one central
mapper writes the REST body; the MCP mapper renders the same sentinel for tools):

| ID | Sentinel | HTTP | `code` | Notes |
|---|---|---|---|---|
| API-ERR-6 | `ErrNotFound` | 404 | `not_found` | Also returned for out-of-RBAC-scope resources |
| API-ERR-7 | `ErrConflict` | 409 | `duplicate_email` | The dedupe path; carries `details.existing_id` |
| API-ERR-8 | `ErrVersionSkew` | 409 | `version_skew` | Body carries current entity + current version (API-CC-4) |
| API-ERR-9 | `ErrScopeExceeded` | 403 | `scope_exceeded` | Tool/operation not in the Passport's effective scope. (Bind-time over-scope Passport minting is a distinct 422 `scope_exceeds_grantor`, owned by the byo-agent-and-mcp chapter) |
| API-ERR-10 | `ErrRequiresApproval` | 403 | `approval_required` | 🟡 operation invoked by an agent without a valid approval token |
| API-ERR-11 | `ErrApprovalTokenInvalid` | 403 | `approval_token_invalid` | Token expired, replayed, or bound to a different action/diff/version |
| API-ERR-12 | `ErrBudgetExceeded` | 429 + `Retry-After` | `budget_exceeded` | Session quota / budget exhausted (quota rows owned by byo-agent-and-mcp) |
| API-ERR-13 | `ErrConsentNotGranted` | 409 | `consent_not_granted` | Send suppressed — recipient lacks active consent for the purpose |
| API-ERR-14 | `ErrSeatTierInsufficient` | 403 | `seat_tier_insufficient` | A read seat (or an agent acting for one) attempted mutate/send/approve (A62/ADR-0047) |
| API-ERR-15 | validation failure | 422 | `validation_error` | Carries `details.errors: [{field, code, message}]` |
| API-ERR-16 | unauthenticated | 401 | `unauthorized` | Bad/missing session or token |
| API-ERR-17 | `ErrModeNotOverlay` | 404 | `mode_not_overlay` | Overlay operation called while the workspace is not in overlay mode |
| API-ERR-18 | `ErrUnsupportedBySoR` | 422 | `unsupported_by_sor` | Declared, tested bounded-capability gap (AC-OV-2/ADR-0018) — never a silent break |
| API-ERR-19 | `ErrIncumbentAlreadyConnected` | 409 | `incumbent_already_connected` | Second incumbent connect rejected |
| API-ERR-20 | `ErrOverlayFlipBlocked` | 409 | `overlay_flip_blocked` | Flip preflight unsatisfied |
| API-ERR-21 | `ErrIncumbentBudgetExhausted` | 503 | `incumbent_budget_exhausted` | Degrade-don't-starve (AC-OV-7); reads fall back to mirror-with-staleness |

### Wire — list envelope & pagination
Source: margince-poc/docs/architecture/api-conventions.md#list-envelope; contract/README.md#pagination--cursor-based @ 5a0b29c

```json
{
  "data": [],
  "page": { "next_cursor": "opaque-or-null", "has_more": true }
}
```

| ID | Rule |
|---|---|
| API-LIST-1 | `?limit=` bounds 1–200, default 50 (= CAP-PAGE). |
| API-LIST-2 | Cursors are opaque; clients never parse them. Cursor pagination only — offset pagination is not offered. |
| API-LIST-3 | `?sort=` is a comma list, `-` prefix = descending, per-resource allowed-field list; `id` is always appended as tie-breaker; default sort is `-created_at,id`. |
| API-LIST-4 | `?include_archived=true` opts into soft-deleted rows; the default excludes them. |
| API-LIST-5 | A cursor reused with different query params → 422 `cursor_param_mismatch`; re-issue the query without the cursor. |
| API-LIST-6 | A sort field outside the resource's allow-list → 422 `sort_field_not_allowed`. |

### Wire — concurrency & idempotency
Source: margince-poc/docs/architecture/api-conventions.md#optimistic-concurrency--integer-version--if-match; features/05-notifications-and-collaboration.md#the-versionetag-approach-normative; contract/README.md#idempotency @ 5a0b29c

| ID | Rule |
|---|---|
| API-CC-1 | Every mutable entity carries a read-only integer `version`, incremented in the same transaction as every committed mutation (DB trigger). |
| API-CC-2 | Mutating requests (PATCH/advance/merge) send `If-Match: <version>` — the raw integer, not an HTTP ETag hash. |
| API-CC-3 | A write without `If-Match` on a concurrency-safe path (first-party UI and the MCP tool surface — both opted in; legacy unconditional writes are not offered) → 428 Precondition Required; the write is never applied last-write-wins. |
| API-CC-4 | `If-Match` mismatch → 409 `version_skew`; the 409 body returns the current record + current version so the caller diffs and retries without a second round-trip. |
| API-CC-5 | The staged-approval execute path re-checks `version == approval.target_version` and returns the same `version_skew` sentinel on mismatch (ADR-0036). |
| API-CC-6 | All POSTs accept `Idempotency-Key: <≤255 chars>`; the same key replays the original status + body instead of duplicating. |
| API-CC-7 | Capture endpoints additionally dedupe on `(source_system, source_id)`; a re-run capture is a no-op and may answer 200 where a fresh create answers 201. |

### Wire — conventions register
Source: contract/README.md#conventions; margince-poc/docs/architecture/api-conventions.md#domain-wire-invariants @ 5a0b29c

| ID | Convention | Rule |
|---|---|---|
| API-CONV-1 | PATCH-merge | Updates are PATCH (no PUT). Omitted fields unchanged; explicit `null` clears a nullable field; returns 200 + full updated entity. |
| API-CONV-2 | Archive semantics | DELETE archives (sets `archived_at`, drops from default lists, stays fetchable by id + audited) and returns 200 + the full archived entity — never 204. Lead delete = disqualify (`status=disqualified` + archive, same rule). `/auth/logout` is the API's only 204. |
| API-CONV-3 | Create — 201 + Location | Successful create → 201 + `Location: /v1/<resource>/<id>` + full created entity identical to the subsequent GET. Exceptions: idempotency replay → original status; capture dedupe → 200 (API-CC-6/7). |
| API-CONV-4 | snake_case derivation | All wire field names are snake_case, derived from the Postgres column names via the contract; generated Go structs must carry `json:"<snake_case_name>"` tags (bare serialisation emits PascalCase and diverges from the generated TS types). Error `code` values, MCP verb names, and enum strings are snake_case too. |
| API-CONV-5 | Custom-field marker | Custom client fields are real columns added by migration, surfaced additively: extensible object schemas set `additionalProperties: true` and carry the vendor extension `x-extension: true`. |
| API-CONV-6 | Provenance on write | `source` + `captured_by` required on every create, returned on every response; `raw` (the re-parseable original) is off the hot path and usually omitted from list views. |
| API-CONV-7 | Money | `{ "amount_minor": int64, "currency": "EUR" }` — never a float. |
| API-CONV-8 | Timestamps | `created_at`, `updated_at` always present; `archived_at` is `string\|null`; all UTC RFC 3339. |
| API-CONV-9 | IDs | UUID format, UUIDv7 underneath but opaque — no ordering guarantees to clients. |
| API-CONV-10 | Approval token | Agent invocations of 🟡 operations require a signed single-use JWS `X-Approval-Token` (claims: `jti`, `tool`, `diff_hash`, `target_version`, `exp`), minted by the approve operation in the approval inbox. Missing/invalid → API-ERR-10/API-ERR-11. Human direct calls need no token. |
| API-CONV-11 | Auth schemes | Two security schemes, one authorization path: `bearerAuth` (agent JWT under an Agent Seat Passport) and `cookieAuth` (human session cookie). Unauthenticated endpoints: workspace signup and login only. 401 = unauthenticated (API-ERR-16); 403 = authenticated but blocked. |

### Limits — REST route classes & caps
Source: contract/api-rate-limits-and-abuse.md#11-rate-limits-defaults--saas @ 5a0b29c

Enforced server-side at the boundary, keyed on (tenant, user, route-class),
token-bucket. Defaults are SaaS; per-mode tuning is owned by the rate-limits source.

| ID | Route class examples | Sustained (per user) | Burst | Per-tenant aggregate ceiling |
|---|---|---|---|---|
| RL-READ-LIGHT | record open, single GET | 30 req/s | 60 | 300 req/s |
| RL-READ-HEAVY | list, search, report run | 10 req/s | 20 | 100 req/s |
| RL-WRITE | create/update/delete, save | 10 req/s | 20 | 100 req/s |
| RL-AUTH | login, token refresh, OAuth | 1 req/s | 10 | per-IP 5 req/s |
| RL-EXPORT | full export, bulk download (P7) | 2 req/min | 2 | 5 req/min |
| RL-BULK-WRITE | import, batch upsert | 1 req/s | 5 | 10 req/s |

Source: contract/api-rate-limits-and-abuse.md#12-pagination-payload-and-complexity-caps @ 5a0b29c

| ID | Cap | Default | Hard max | Rationale |
|---|---|---|---|---|
| CAP-PAGE | Page size (cursor-based) | 50 | 200 | Protects the PERF-2 budget; offset pagination not offered (cursor only). |
| CAP-DEPTH | Relationship expansion depth | 1 hop | 2 hops | Prevents join-bomb queries against the relational core (P11). |
| CAP-BODY | Request body size | 1 MB | 10 MB (import only) | Memory/DoS guard. |
| CAP-RESP | Response row ceiling per request | = page size | 200 | No unbounded result sets; force pagination. |
| CAP-BATCH | Items per batch write | 100 | 500 | Bounds transaction size; protects PERF-4. |
| CAP-QUERY-COST | Report/search complexity budget | model in features/03 | — | Reporting query-plan cost ceiling; over-budget → 400 with the offending plan node. |

Source: contract/api-rate-limits-and-abuse.md#13-standard-429-contract-normative @ 5a0b29c

**API-429-1 — the normative 429 contract.** On limit breach the API returns HTTP 429
Too Many Requests. These headers appear on every rate-limited response (success and
429 alike): `RateLimit-Limit` (ceiling for the current window for this route class),
`RateLimit-Remaining` (tokens left), `RateLimit-Reset` (seconds until refill), and —
on 429 only — `Retry-After` (seconds the client must wait, honored as authoritative).
The 429 body is the standard problem shape with:

```json
{
  "code": "rate_limited",
  "scope": "user|tenant|ip",
  "retry_after_s": 30,
  "limit_class": "RL-READ-HEAVY"
}
```

Cap violations (CAP-PAGE/CAP-BODY/CAP-DEPTH/CAP-BATCH/CAP-RESP) are client bugs, not
transient: they return 400 `cap_exceeded` with the offending cap ID, never 429 —
no retry helps. MCP session quotas (MCP-SESS-*) and the anomaly/abuse ladder are owned
by the byo-agent-and-mcp and security chapters; the quota-breach error envelope mirrors
this contract.
