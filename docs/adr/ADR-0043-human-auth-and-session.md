# ADR-0043 — Human authentication & session model (in-app `crm-auth`, email/password baseline, server-side sessions)

- **Status:** Accepted (2026-06-25, founder — Lars)
- **Decision record:** DECISIONS **A58**
- **Composes with:** ADR-0013 (one governed surface + OAuth2-only for *tools*), `features/04 §7` (SSO/MFA/session-security requirements), P3 (contract-first), P11 (static schema), P12 (everything audited), P7 (source-delivered/on-prem)
- **Supersedes:** the `narrative/03 §3.1`/`§3.4` line that auth = *"Google OAuth + JWT via shared `gw-auth`, SSO as a commercial add-on, one login across Dispact + CRM"* — that statement predates `features/04 §7` and is replaced here.

## Context

Building the PoC (`margince-poc` PR #7) surfaced a real gap: the **human interactive auth foundation was never written into the normative spec.** The *requirement* was specified — `features/04 §7` (added 2026-06-23) makes email/password + **SAML 2.0 / OIDC SSO** + **TOTP MFA** + **server-side session security** (idle/absolute timeout, device/session list, **remote revoke**) all `[MVP]`, and EP03 has the build stories (`B-EP03.17` session model, `.18` MFA, `.20–22` SSO). But the *mechanism* never reached the contract or data-model:

- `crm.yaml` had only `/me` — no `/auth/login`, `/auth/logout`, no workspace-bootstrap endpoint.
- `data-model.md` had **no `session` table, no `passport` table, and no `app_user.password_hash`** — `app_user.email` was merely commented *"login identity (shared gw-auth)."*
- No build story covered the *baseline* password login + first-admin bootstrap; the existing stories assume a session already exists.

Two of the spec's own statements also disagreed: `narrative/03` said *Google OAuth + JWT, SSO as paid add-on, shared identity service with Dispact*; `features/04 §7` says *email/password + server-side sessions + SSO included in the one flat tier (A36)*. A coding agent forced to proceed (PR #7) independently chose email/password + an opaque `crm_session` cookie backed by a server-side `session` table — which is the **correct** reading of §7 (JWT cannot satisfy the remote-revoke requirement) and exposed the stale narrative.

## Decision

**1. Auth is owned in-app by the `crm-auth` module — not an external identity service.** "Shared with Dispact" (`narrative/03 §3.3`) means **shared library code / pattern** (`gw-auth` as a reusable package), **not** a shared running SSO service. Rationale: a hosted identity service breaks source-delivered/on-prem deployment (P7) and the "one governed surface, no backdoors" model (ADR-0013). Each Margince workspace authenticates against its own `crm-auth`.

**2. Baseline auth is email + password. Google OAuth / social sign-in is dropped.** Enterprise SSO (SAML 2.0 / OIDC, `B-EP03.20–22`) is the upgrade path, included in the flat tier, not a separate edition. `app_user.password_hash` is `NULL` for SSO-provisioned users; `SSO-enforced` mode (`B-EP03.22`) disables password login per workspace.

**3. Sessions are opaque and server-side — not stateless JWTs.** A login mints a 32-byte cryptographically-random token; only its **SHA-256 hash** is stored (`session.token_hash`, raw token never touches the DB). The raw token is carried in a cookie:

    crm_session=<base64url(32-byte random)>; HttpOnly; Secure; SameSite=Strict; Path=/

Server-side sessions are required because `B-EP03.17` mandates **remote revoke within one event-bus cycle** and a device/session list — neither is honestly achievable with stateless JWTs. Idle + absolute expiry are columns on the row (`idle_expires_at`, `expires_at`), checked at lookup.

**4. New normative surface** (specified in `contract/crm.yaml` + `contract/data-model.md`):
- `POST /workspaces` — bootstrap: create workspace + its first admin `app_user` in one transaction, set the session cookie.
- `POST /auth/login` — email+password (+ MFA challenge when required) → new `session` row, set cookie.
- `POST /auth/logout` — delete the current `session` row, clear the cookie.
- `GET /me` — already in the contract; resolves the session principal.
- Tables: **`session`**, **`passport`** (the Agent Seat Passport — referenced by `audit_log.passport_id`/`agent_connection.passport_id` but previously undefined), and **`app_user.password_hash text NULL`**.

**5. Agent auth is unchanged.** The Agent Seat Passport + OAuth 2.1/PKCE remains the *only* network-auth model for tools (ADR-0013, A25/A26). SSO/MFA/sessions govern **human interactive** sign-in only; they introduce no second authorization model for agents (the `Admit` pipeline still intersects `passport.scopes ∩ human RBAC`).

**6. Every auth event is audited** (login, logout, MFA challenge, SSO assertion, failure, lockout, session revoke) into the `audit_log` stream per `features/04 §7` / P12.

## Consequences

- The PoC PR #7 Section-1 design is **blessed and now normative** — the dev can proceed; the improvised shapes match the spec.
- `narrative/03 §3.1`/`§3.4` updated: no Google OAuth, no shared identity *service*, SSO not a paid add-on.
- `data-model.md` gains `session` + `passport` + `password_hash` (closes a static-schema/P11 hole that also blocked agent-attribution wiring).
- One new baseline build story (`B-EP03.0` — email/password login + workspace bootstrap + session issuance) anchors the EP03 chain that `B-EP03.17`+ build on.

## Alternatives considered

- **Stateless JWT sessions** — rejected: cannot satisfy `B-EP03.17` remote-revoke / device-list without a server-side denylist that recreates session state anyway.
- **Shared hosted identity service across Margince + Dispact** — rejected: breaks on-prem/source-delivery (P7) and adds an external trust dependency contrary to ADR-0013; "shared" stays at the library level.
- **Keep Google OAuth as a baseline** — rejected: extra auth surface for little beachhead value; the regulated-Mittelstand buyer wants corporate SSO, not consumer social login. Email/password baseline + enterprise SSO covers the market.
