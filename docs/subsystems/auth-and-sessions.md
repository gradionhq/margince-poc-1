---
status: skeleton
module: backend/internal/modules/identity
derives-from:
  - margince specs/spec/decisions/ADR-0043-human-auth-and-session.md
  - margince specs/spec/contract/data-model.md#26-session--interactive-human-sessions-a58adr-0043
  - margince specs/spec/contract/data-model.md#27-passport--the-agent-seat-passport-a34adr-0026-defined-here
  - margince-poc/docs/architecture/data-model.md#auth-tables
---
# Auth & sessions — every request carries a provable, revocable principal

> The identity module answers one question on every request — who is calling, human
> or agent, with what authority — and keeps the answer revocable on the server.
> Humans sign in with email and password and get an opaque server-side session;
> agents present an Agent Seat Passport whose authority can never exceed the human
> who granted it.

## What it's for

Nothing in the product is anonymous: tenancy, RBAC, audit attribution, and the
agent-governance story all start from a resolved principal, so principal resolution
has to happen once, early, and identically for every caller. This subsystem owns
human interactive sign-in, the session lifecycle behind it, workspace bootstrap,
and the grant/revoke lifecycle of the Agent Seat Passport — the only network-auth
model for agents. Its callers are the request pipeline itself (every authenticated
operation passes through it before any domain code runs), the login and bootstrap
surfaces, and the passport-management surface an agent host is connected through.
The scope boundary: this chapter is the *baseline* that ships in the skeleton;
enterprise SSO, MFA, and SCIM are planned upgrades owned elsewhere (see Out of
scope).

## Principles it serves

- **P12 — everything audited.** Every auth event — login, logout, failure,
  passport grant and revoke — writes an audit row, so the attribution spine starts
  at the front door (ADR-0043).
- **P7 — source-delivered / on-prem.** Auth is owned in-app, not by a hosted
  identity service; each workspace authenticates against its own identity module,
  so an on-prem deployment has no external trust dependency (ADR-0043).
- **ADR-0043 — human auth & session model.** Email/password baseline, opaque
  server-side sessions, and the session/passport tables; the decision this whole
  chapter embodies.
- **ADR-0013 — one governed surface.** Agent network auth stays passport-only;
  human sign-in introduces no second authorization model for agents, and the
  agent-versus-human split is two credentials feeding one enforcement path.

## How it works

**Human sign-in is email and password, deliberately not federated and deliberately
not stateless.** The baseline dropped social sign-in: the regulated-Mittelstand
buyer wants corporate SSO (the planned upgrade), not consumer OAuth. Stateless
JWTs were rejected because the spec requires remote revocation and a device/session
list, and neither is honest without server-side state — a JWT denylist just
recreates the session store with extra steps (ADR-0043).

**A login mints an opaque session.** The server generates a high-entropy random
token (AUTH-PARAM-3), stores only its cryptographic hash, and returns the raw value
in a cookie that scripts cannot read, that travels only over TLS, and that is
strictly same-site (AUTH-PARAM-4). The raw token never touches the database. A
request authenticates only if a live session row matches the hash of the presented
cookie — not revoked, not idle-expired, not absolutely expired. Activity slides the
idle window forward (AUTH-PARAM-2) up to the absolute lifetime cap (AUTH-PARAM-1);
logout deletes the session row on the server and clears the cookie, and is the
API's single no-content response ([[api-conventions#API-CONV-2]]). Passwords are
stored only as adaptive one-way hashes (AUTH-PARAM-5), and the password column is
empty for SSO-provisioned users when that upgrade lands. The session and passport
table shapes are owned by the data-model chapter ([[data-model#DM-DDL-6]],
[[data-model#DM-DDL-7]]) and are not restated here.

**Workspace bootstrap is one transaction.** Creating a workspace creates its first
admin user in the same transaction and signs that user in — either both exist
afterwards or neither does, so there is no window where a workspace exists without
an owner (ADR-0043, AUTH-WIRE-1).

**The Agent Seat Passport is the agent credential.** A human grants a passport
that binds an agent host to a CRM identity plus an explicit scope set; the raw
bearer token is shown exactly once at creation and only its hash persists. At
every admission the effective authority is re-derived as the intersection of the
passport's scopes and the granting human's current RBAC — the *agent ≤ human*
invariant ([[threat-model#TM-CTRL-2]]; see the glossary entry for Agent Seat
Passport). Two consequences follow. At bind time, requesting scopes wider than the
grantor holds is rejected outright with the scope-exceeds-grantor semantics (the
bind-time error is pinned by the byo-agent-and-mcp chapter, per the note on
[[api-conventions#API-ERR-9]]) — an over-broad passport is never minted, rather
than minted and filtered later. At use time, revocation is synchronous: a revoked
or expired passport fails closed at the very next lookup, with no cache-flush or
bus round-trip in the enforcement path.

**One middleware chain resolves every request.** Conceptually the chain is
session, then workspace, then RBAC: first the credential (session cookie or
passport bearer token) resolves to a principal or the request is rejected as
unauthenticated; then the principal's workspace becomes the transaction-scoped
tenant setting the data-model chapter's RLS backstop keys on; only then does
role-based authorization and domain code run. Humans and agents converge after the
first step — same workspace scoping, same RBAC, same audit — which is what makes
"no agent backdoor" a structural claim instead of a convention.

## What's configurable

- **Session absolute lifetime** — how long a session can live regardless of
  activity; default one day as shipped (AUTH-PARAM-1). ADR-0043 mandates the knob
  (absolute expiry checked at lookup) but does not fix a value; the default is the
  skeleton's.
- **Session idle lifetime** — the sliding inactivity window, rolled forward on
  each request and capped by the absolute lifetime; default two hours as shipped
  (AUTH-PARAM-2). Same provenance: the mechanism is ADR-0043, the value is the
  skeleton's.
- **Passport expiry** — every passport carries a mandatory expiry chosen at grant
  time; there is no non-expiring passport (AUTH-PARAM-6).
- **Password hashing cost** — the adaptive hash's work factor is an operational
  knob of the shipped implementation (AUTH-PARAM-5); the spec pins the property
  (never plaintext, never reversible), not the parameterization.

## Guarantees (enforced)

- **No raw credential at rest.** Session tokens, passport tokens, and passwords
  are stored only as one-way hashes; a database dump yields nothing replayable.
  Raw tokens are returned to the caller exactly once, at creation.
- **Sessions are revocable on the server.** Authentication is a live lookup
  against server-side state, so deleting or revoking a session ends it at the next
  request — the property that ruled out stateless JWTs (ADR-0043).
- **Agent ≤ human, re-derived at admission.** An agent's effective scope is the
  intersection of its passport scopes and the granting human's current RBAC, so
  demoting the human instantly narrows every passport they granted
  ([[threat-model#TM-CTRL-2]]).
- **Fail-closed passport lifecycle.** Over-scope binds are rejected before a
  passport exists; revoked and expired passports authenticate nothing, immediately,
  at lookup.
- **Bootstrap atomicity.** Workspace and first admin are created in one
  transaction; a failure leaves neither behind.
- **Every auth event is audited.** Login, logout, failure, grant, and revoke each
  write an audit row into the append-only spine owned by the data-model chapter.

## Acceptance

Done means: a user can bootstrap a workspace, sign in, be recognized on subsequent
requests, and sign out — with the session honestly dead afterwards; an agent host
can be granted a passport, act within the grantor's authority, and be cut off by
revocation with no lag window. The denied states are first-class: wrong
credentials, expired or idle-expired sessions, and revoked passports all resolve to
the unauthenticated outcome rather than a partial one. The testable form of each
claim is pinned in the Acceptance appendix (AUTH-AC-1..6); the cross-cutting floor
(standard screen states, performance budgets) is inherited from the
acceptance-standards chapter.

## Out of scope

- **SSO (SAML 2.0 / OIDC), TOTP MFA, SCIM provisioning** — specified as the
  upgrade path in ADR-0043 but planned, not in the skeleton; owned by the
  access-and-admin chapter.
- **Device/session list and admin remote-revoke surface** — the session model is
  built for them (server-side rows, revocation enforced at lookup), but the
  management surface is planned; owned by access-and-admin.
- **Passport scope vocabulary, tiers, and the admission gate itself** — the
  byo-agent-and-mcp chapter owns the governed tool surface; this chapter only
  issues and revokes the credential it checks.
- **RBAC role and grant administration** — table shapes in the data-model
  chapter, administration in access-and-admin.

## Where it lives

The identity module inside the backend's modules tree, reached through the
request-pipeline seam (credential resolution and principal context) and the
bootstrap/login/passport operations of the contract. Read next: data-model (the
session, passport, and RBAC tables), threat-model (the agent ≤ human control),
byo-agent-and-mcp (what a passport admits an agent to do), and
approvals-and-concurrency (what happens when an admitted agent proposes a gated
action).

## Appendix

### Parameters
Source: decisions/ADR-0043-human-auth-and-session.md @ 5a0b29c; margince-poc/docs/architecture/data-model.md#auth-tables @ a11d6c08 (duration defaults are shipped-skeleton values — ADR-0043 mandates the knobs, not the numbers)

| ID | Name | Value | Meaning |
|---|---|---|---|
| AUTH-PARAM-1 | Session absolute lifetime | 24h (default, as shipped) | Hard cap on session age; checked at every lookup. ADR-0043 pins the mechanism (absolute expiry column), the skeleton supplies the default. |
| AUTH-PARAM-2 | Session idle lifetime | 2h sliding (default, as shipped) | Inactivity window; rolled forward on each authenticated request, capped by AUTH-PARAM-1. Mechanism per ADR-0043, default per skeleton. |
| AUTH-PARAM-3 | Session token entropy | 32 random bytes, base64url in the cookie | ADR-0043: cryptographically random; only the SHA-256 hash is stored. |
| AUTH-PARAM-4 | Cookie posture | `crm_session`; HttpOnly; Secure; SameSite=Strict; Path=/ | ADR-0043 cookie contract; Secure applies over TLS; logout clears the cookie. |
| AUTH-PARAM-5 | Credential storage | bcrypt password hashes; SHA-256 token hashes; raw values never stored | Property pinned by ADR-0043 ("never plaintext"); bcrypt is the shipped algorithm (skeleton-harvest inventory); work factor is operational. |
| AUTH-PARAM-6 | Passport expiry | mandatory, set at grant; no default-infinite | Enforced at lookup together with revocation ([[data-model#DM-DDL-7]]). |

### Wire
Source: contract/crm.yaml (Identity tag) @ 5a0b29c; margince-poc/contract/crm.yaml @ a11d6c08

Operations are cited by contract operationId; shapes live in the contract and are
never restated here. Error semantics: unauthenticated → [[api-conventions#API-ERR-16]];
logout's no-content response → [[api-conventions#API-ERR-5]]; bind-time over-scope →
scope-exceeds-grantor (pinned by byo-agent-and-mcp, see note on [[api-conventions#API-ERR-9]]).

| ID | operationId | Behavior pinned |
|---|---|---|
| AUTH-WIRE-1 | `createWorkspace` | Bootstrap: workspace + first admin user created in one transaction; responds with the session established. |
| AUTH-WIRE-2 | `login` | Email + password; success mints a session row and sets the cookie (AUTH-PARAM-3/4); failure is audited. |
| AUTH-WIRE-3 | `logout` | Deletes the current session row server-side and clears the cookie; the API's single no-content (204) response ([[api-conventions#API-CONV-2]]). |
| AUTH-WIRE-4 | `getCurrentPrincipal` | Resolves the presented credential (session cookie or passport bearer) to the current principal — human or agent. |
| AUTH-WIRE-5 | `createPassport` | Grants an Agent Seat Passport; scopes must be a subset of the grantor's effective RBAC or the mint is rejected (no row created); raw token returned exactly once. |
| AUTH-WIRE-6 | `revokePassport` | Revokes a passport; enforcement is at next lookup, fail-closed, with no propagation delay in the enforcement path. |

### Acceptance
Source: decisions/ADR-0043-human-auth-and-session.md @ 5a0b29c; margince-poc/docs/architecture/data-model.md#auth-tables @ a11d6c08

| ID | Given/When/Then | Verification |
|---|---|---|
| AUTH-AC-1 | Given a provisioned user, when they log in with correct email + password, then a session is created server-side, the cookie is set per AUTH-PARAM-4, and an audit row records the login. | Integration test, identity module lane |
| AUTH-AC-2 | Given a live session, when the user logs out, then the response is the API's single no-content response, the session row is gone, and the next request with the old cookie is unauthenticated. | Integration test, identity module lane |
| AUTH-AC-3 | Given a granted passport, when it is revoked (or its expiry passes), then the very next bearer request fails closed as unauthenticated — no grace window, no cache dependency. | Integration test, identity module lane |
| AUTH-AC-4 | Given a grantor whose RBAC lacks a scope, when a passport bind requests that scope, then the bind is rejected at bind time with the scope-exceeds-grantor semantics and no passport row is minted. | Integration test, identity module lane |
| AUTH-AC-5 | Given a session idle past AUTH-PARAM-2 (or older than AUTH-PARAM-1), when its cookie is presented, then the request is unauthenticated; activity within the idle window slides it forward, capped by the absolute lifetime. | Time-advanced integration test |
| AUTH-AC-6 | Given a workspace-bootstrap request that fails partway, when the transaction aborts, then neither the workspace nor the first admin user exists — never one without the other. | Integration test, identity module lane |
