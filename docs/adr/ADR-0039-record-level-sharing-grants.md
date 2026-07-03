# ADR-0039 — Manual record-level sharing (per-record grants)

**Status:** Accepted (2026-06-24) · **Decision:** A52 · **Supersedes:** the `features/04 §1` "OUT (v1)" line for manual per-record sharing · **Relates to:** ADR-0026/0036 (autonomy tiers + approval token), ADR-0032 (first-class objects), the RLS connection-lifecycle rule (`data-model §1.3c`)

## Context

V1 row-level visibility is **tiered** — `own / team / all`, expressed as `role.permissions.row_scope`, enforced by application-built `WHERE` clauses **plus** the RLS backstop keyed on `app.user_id` / team membership (`data-model §2.4`, `features/04 §1`). The deep-review parity pass (RT-PR-H6) found the parity matrix overstated D10.4 "record-level sharing" as **Covered** when only the *tiers* were V1: there was no way to share **one** record with a person or team who is otherwise out of scope ("give the SE read on these three deals", "show this one deal to the exec team"). Real sales orgs need this constantly — deal-desk reviews, exec escalations, bringing in an SC — and an auditor expects to see *who* was granted *what*, *by whom*, and *why*.

Territory-based sharing (D10.2) is the *other* half of the gap and is deliberately **not** in this ADR — it is the committed first fast-follow (see A52 / `BACKLOG §L4`). This ADR is the small, high-leverage half: a generic, audited grant on a single record.

The thesis constraint (P2): because client/agent-authored code touches the DB directly, any visibility widening must live at the **DB enforcement point**, not only in app code. We do **not** adopt the Salesforce sharing-hierarchy/criteria-rule complexity (still explicitly OUT) — this is a flat, explicit, revocable grant.

## Decision

Add a single generic **`record_grant`** table and make it a first-class term in the visibility predicate.

1. **One generic grant table** (not one per object): `(record_type, record_id, subject_type ∈ {user,team}, subject_id, access ∈ {read,write}, granted_by, reason?, expires_at?)`, `workspace_id`-scoped and `version`-carrying like every mutable entity (ADR-0036). V1 `record_type ∈ {deal, person, organization, lead}`.
2. **Enforcement widens both layers.** The existing visibility predicate becomes `(<own/team/all base scope>) OR <an active matching grant exists>`. The **same** `OR EXISTS (record_grant …)` clause is added to the RLS backstop policy, so a grant that only widened the app query but not the policy would still see nothing — a grant has to widen both or the row stays invisible. Expired grants (`expires_at < now()`) match nothing. A `write` grant satisfies `read`.
3. **Agent path identical, and tiered.** Granting is a mutate, so an **agent-initiated** grant is 🟡 (queued behind the approval gate, ADR-0026/0036); a human with the `manage_sharing` permission grants directly. No agent bypass — the grant obeys the same enforcement path as any human action (`features/04 §1`). An agent can never grant access wider than the granting human holds (scope-intersection, ADR's narrower-or-equal rule).
4. **Audited and provable.** Every grant and revoke writes an append-only `audit_log` row (`action: record_share` / `record_unshare`; actor, subject, record, access, reason). "Who can see this record, and under whose authority?" is answered by looking, not by querying the DB — the regulated-beachhead requirement (A43).
5. **Contract.** A generic `/record-grants` collection (`GET` filter by record or subject · `POST` create · `DELETE {id}` revoke), `share_record` MCP verb (🟡). Not a per-type sub-resource — one resource serves all shareable types.

## Consequences

- **D10.4 flips to Covered (V1).** Parity overstatement RT-PR-H6 is closed by building, not by downgrading.
- **+1 V1-Must story** (S-E11.12, build story B-E11.31) under E11. Reconciled against the concurrent A53 overlay baseline (incl. the 2026-06-24 story red-team C1 split): V1 line → 96 (60 Must + 36 WOW); 125 stories total (canonical in `product/20-traceability.md`).
- **Cost is small:** one table + migration, one clause added to the visibility builder and the RLS policy, three endpoints, one "Share" dialog. It rides the EP02 RLS/visibility substrate and the EP07 audit_log already being built.
- **Bounded by design:** flat explicit grants only — no sharing hierarchies, no criteria-based auto-share rules, no grant-of-grant delegation. Those remain OUT (the SF complexity we declined). Territory-based sharing is the next fast-follow, not this ADR.
- **GDPR:** a `read`/`write` grant widens who can see personal data; the grant's `reason` + audit row is the lawful-basis/accountability trail. Erasure and SAR are unaffected (grants reference records, not copies).
