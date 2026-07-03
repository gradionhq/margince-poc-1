# ADR-0048 — Connector secret storage (sealed, self-contained, key-provider-pluggable)

**Status:** Accepted (Lars, 2026-06-26). New decision: **DECISIONS A63**. Governs how the overlay connectors (Salesforce / HubSpot / Dynamics — epics **E18 / E19 / E20**) and any other stored OAuth/API credential persist their secrets. The vault primitive lands in ticket **B-E18.13**; the schema lands in **[`../contract/data-model.md`](../contract/data-model.md)** (`connector_secret`). Composes with **[ADR-0013](ADR-0013-one-governed-surface-and-auth.md)** (one governed surface — no privileged backdoor to read secrets), the encryption-at-rest posture, and principles **P7** (on-prem / sovereign) and **P12**.

## Context

The overlay connectors authenticate to incumbent systems of record on the customer's behalf, so each holds a long-lived credential — an OAuth refresh token, an API key, a client secret. These must survive restarts, so they must be persisted somewhere. The naïve answer in a cloud-first product is "put them in a managed secrets manager" (Vault, AWS/GCP KMS Secrets, etc.).

That answer breaks **P7**. Margince must run **on-prem and sovereign**: a regulated Mittelstand customer's instance, or a partner-hosted one, **cannot be assumed to have an external secrets manager**. We cannot make the connectors — a core wedge (P13 augmentation) — depend on infrastructure half our deployments won't have. The store therefore has to be **self-contained in the application database**, with the only environment-specific part being *which key seals it*.

## Decision

**Connector secrets are stored encrypted at rest in the application database, sealed with a key from a pluggable key provider, never returned through any API, and rotatable.**

1. **Ciphertext in the app DB.** A new **`connector_secret`** table holds the encrypted credential (ciphertext + nonce + key reference + metadata). Overlay connectors do not store credentials inline; they hold a `connector_secret` **id** and dereference it through the vault primitive. No plaintext credential ever lands in a connector row, a log, or an event payload.

2. **Sealed by a pluggable key provider.** The data-encryption key is wrapped by a key from a **key provider interface** with two stock implementations:
   - **Hosted / cloud:** a cloud **KMS** holds the wrapping key.
   - **On-prem / sovereign:** a **local key file** (an `age` key / keyfile on disk), no external dependency.
   The provider is the **only swappable part**; the table, the sealing format, and the dereference path are identical in every deployment. This is what makes the store satisfy P7 without forking the connector code per environment.

3. **Never returned through any API.** Consistent with **ADR-0013** (one governed OAuth2 surface, no privileged backdoor), there is **no endpoint, MCP tool, or admin path that reads a secret back out**. Secrets are *write-only from the operator's side and use-only from the connector's side*: callers set or replace a credential and reference it; only the connector runtime, in-process, unseals it to make the outbound call. Reading a `connector_secret` is not a capability the governed surface exposes to anyone — not admins, not agents, not 🟡 approvers.

4. **Rotatable.** Both the wrapping key (re-seal in place via the key provider) and the underlying credential (replace the ciphertext, same id) can be rotated without downtime or schema change. Rotation is an operator action, audited like any other 🟡 mutation.

## Implementation bindings (pinned 2026-07-02, B-E18.13)

These pin the four tech decisions the vault primitive raised so it is unambiguous for the build; they **refine, and do not reverse**, the decision above.

1. **Module.** The vault primitive — the `connector_secret` table and the key-provider interface — is **owned by [`crm-auth`](../architecture/01-module-dag.md)**, the Tier-0 identity/credential module. **Not** a new overlay module: because `crm-auth` is Tier 0, the E18/E19/E20 connectors *and any other credential consumer* reach the vault through the DAG without a new module or an overlay→consumer edge. Overlay connectors hold only a `connector_secret` id and dereference in-process.
2. **Local key-provider crypto.** The on-prem provider uses **stdlib AES-256-GCM with a raw 32-byte keyfile** (`crypto/aes`, no third-party dependency). The "`age` key / keyfile" wording above is **illustrative of the on-prem posture, not prescriptive** of the library. Envelope-wrapping a data-encryption key + rotate-in-place is required regardless of cipher.
3. **Endpoint surface (with ADR-0013).** OAuth `authorize` + `callback` are **untyped HTTP redirect routes** (an OAuth redirect dance, not typed request/response CRUD — kept out of `crm.yaml`). `rotate` + `revoke` are **typed governed 🟡 endpoints** on the one governed surface — mutating operator actions, approval-gated and audited.
4. **V1 SDK posture.** V1 ships the key-provider and HubSpot-client **interfaces plus stub implementations** (`stubKMSClient` / `stubHubSpotClient` returning "not configured"), mirroring the Gmail/Calendar connector precedent. Wiring a real cloud-KMS SDK or live HubSpot credentials is an **ops-provisioning step**, not a build blocker for this ticket.

## Consequences

- **On-prem is unblocked by construction:** the overlay connectors run with zero external secrets infrastructure; an `age` keyfile is enough. The same binary, with the cloud-KMS provider configured, runs hosted. No per-deployment connector fork (P7, P12).
- **Blast radius is bounded:** a database dump alone yields only ciphertext; the wrapping key lives in the key provider (KMS or a separately-managed keyfile), not in the same row.
- **No backdoor surface to audit:** because nothing reads secrets back, there is no "who can see the credential" question to answer in the security model — the answer is "no one, by design" (ADR-0013).
- **Contract / build:** `connector_secret` table + sealing-format fields in [`../contract/data-model.md`](../contract/data-model.md); the key-provider interface and the vault primitive are owned by **B-E18.13**; the E19/E20 connectors consume the primitive by id. Operator-facing rotation is a connector-admin action, not a read path.

## Alternatives considered

- **Depend on an external secrets manager (Vault / cloud-KMS Secrets).** Rejected: breaks **P7** — half of all deployments (on-prem, sovereign Mittelstand, some partner-hosted) cannot assume it exists, and we will not fork the connector code per environment. We keep the *key* pluggable (a KMS *can* be the provider) but never the *store*.
- **Store credentials inline on the connector row, encrypted.** Rejected: couples the credential lifecycle to the connector row, scatters ciphertext across tables, and makes rotation and "never returned" harder to enforce uniformly. One `connector_secret` table with a single sealed path is simpler to reason about and audit.
- **Expose a guarded read-back endpoint for support/debugging.** Rejected: any read path is a privileged backdoor that contradicts ADR-0013 and widens the blast radius. Operators replace credentials; they never read them.
