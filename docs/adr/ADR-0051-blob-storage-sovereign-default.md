# ADR-0051 — Blob/object storage is sovereign-by-default; the S3-compatible endpoint is pluggable (cloud is opt-in)

**Status:** Accepted (Lars, 2026-06-29). New decision: **DECISIONS A66**. Governs where large binary
payloads (call/meeting transcripts first — ticket **B-E04.1**, then any future attachments/exports/blobs)
are persisted. The `crm/blobstore` seam + the dev MinIO adapter already shipped (B-E04.1, PR #62); this ADR
locks the **default residency posture** so the build cannot drift into a US-cloud default. Composes with
**[ADR-0048](ADR-0048-connector-secret-storage.md)** (same "self-contained, provider-pluggable, no assumed
cloud infra" pattern for secrets), **[ADR-0013](ADR-0013-one-governed-surface-and-auth.md)**, the
deployment-tier model in **[DECISIONS.md](DECISIONS.md)** (on-prem/sovereign · EU-sovereign hosted · A24),
and principle **P7** (on-prem / sovereign, zero-egress).

## Context

Blobs — transcripts today, attachments/exports later — are customer data and so fall squarely under the
data-sovereignty promise (P7), our **most load-bearing differentiator**. The naïve cloud-first answer is
"store them in AWS S3." But AWS S3 is, by default, **US-operated infrastructure**, and a sovereign or
regulated-Mittelstand customer's data **must not leave its residency boundary** (on-prem zero-egress, or the
named EU region for the partner-hosted managed tier).

The subtlety that caused the spec gate (B-E04.1 / GH margince-poc #58) to read wrong: **"S3" is overloaded.**
It can mean *Amazon S3* (a specific US-default service) **or** the *S3 API* (a wire protocol that
**self-hosted** stores — chiefly **MinIO** — also speak, running on the customer's own hardware or in an EU
datacenter). The shipped design used the S3 **API** via a self-hosted MinIO adapter — sovereign by
construction — but the build note said "prod wires real S3," which wrongly implied an Amazon-by-default
posture. This ADR removes that ambiguity.

## Decision

**Blob storage goes through one pluggable `crm/blobstore` seam whose default endpoint is a sovereign,
self-hosted, S3-API-compatible store bound to the deployment tier. A specific public cloud (AWS S3, etc.) is
opt-in configuration only — never the default.**

1. **One seam, S3-API contract.** All blob reads/writes go through the `crm/blobstore` interface; callers
   never name a concrete backend. The wire contract is the S3 API, so any S3-compatible endpoint is reachable
   by configuration alone — **no code change to switch backends**.

2. **Default endpoint follows the deployment tier (sovereign in every tier):**
   - **On-prem / sovereign:** **self-hosted MinIO** (or the customer's own S3-compatible store) on client
     infra — **zero egress**, air-gapped-capable (P7).
   - **EU-sovereign hosted (the default managed posture, A24):** the **partner's EU-region** S3-compatible
     object store — data stays in the operator's named EU region (the residency promise).
   - **Local dev / POC:** MinIO via `infra/docker-compose.dev.yml` (`minio` + `minio-init`) — dev/prod parity,
     no external dependency.

3. **Public cloud is opt-in, explicit, and never the default.** A customer who *wants* AWS S3 (or Azure
   Blob / GCS via an S3 gateway) sets the endpoint + credentials for that backend. The default ships pointing
   at the sovereign option; reaching a US-operated bucket is always a deliberate operator choice, logged.

4. **The endpoint/region is a governed runtime-config-surface entry**, defaulting to the sovereign option,
   so "where do blobs live" is an explicit, auditable deployment setting — not a hard-coded constant and not
   an implicit cloud default.

## Consequences

- **The #1 promise holds by default:** out of the box, no customer's blobs land on US infrastructure;
  sovereignty is the path of least resistance, cloud is the exception you opt into (P7).
- **"Support S3 if the client wants it" costs nothing:** because the seam speaks the S3 API, AWS support is a
  config value, not a feature build — exactly the flexibility the gate asked for, without weakening the default.
- **Consistent with ADR-0048:** blobs and secrets now share one pattern — self-contained, provider/endpoint
  pluggable, no assumed external cloud infra — so on-prem is unblocked by construction and there is no
  per-deployment fork of the storage code.
- **Build impact:** the shipped `crm/blobstore` seam + MinIO adapter (B-E04.1) are unchanged; the only deltas
  are (i) the default config points at the sovereign endpoint, (ii) a `runtime-config-surface.md` entry for the
  blob endpoint/region, and (iii) the B-E04.1 ticket wording corrected from "prod wires real S3" to the
  sovereign-default rule above.

## Alternatives considered

- **AWS S3 as the prod default (the original "prod wires real S3" reading).** Rejected: defaults are
  promises, and a US-cloud default contradicts P7 and the residency commitment for on-prem and EU-sovereign
  customers. A default must be safe for the most-constrained customer, not the most-convenient deployment.
- **Filesystem-only stub for dev, no MinIO (the original gate recommendation).** Superseded by the shipped
  choice: a dev MinIO adapter gives dev/prod parity and exercises the real S3-API path in tests, at the cost
  of one dev-compose service — a worthwhile trade for a sovereignty-critical seam.
- **Forbid cloud object storage entirely.** Rejected: some customers will legitimately want their own cloud
  bucket; the right control is "sovereign default + explicit, logged opt-in," not prohibition.
