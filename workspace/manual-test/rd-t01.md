# RD-T01 — Manual/Live UAT: quota resource + attainment read contract

Contract-only ticket — no handler/business logic ships in this branch (spec Out-of-scope), so
every step is contract/gate inspection rather than a live HTTP round-trip against a real `quota`
table (that table doesn't exist yet — a separate migration ticket). All steps are `[auto]`.

1. **[auto]** Run `make gen-types` then `git status --short`.
   Expected: exits 0; diff (if any, from a prior dirty state) touches only
   `backend/api/crm.yaml`, `backend/internal/contracts/types/crm_gen.go`,
   `frontend/src/lib/api-client/generated/crm.d.ts` — clean regeneration, no drift.

2. **[auto]** Run `grep -n "operationId: listQuotas\|operationId: createQuota\|operationId: getQuota\b\|operationId: updateQuota\|operationId: archiveQuota\|operationId: getQuotaAttainment" backend/api/crm.yaml`.
   Expected: all six operationIds present, on `GET /quotas`, `POST /quotas`, `GET
   /quotas/{id}`, `PATCH /quotas/{id}`, `DELETE /quotas/{id}`, `GET /quotas/{id}/attainment`
   respectively.

3. **[auto]** Run `grep -n -A6 "operationId: archiveQuota" backend/api/crm.yaml`.
   Expected: the `200` response with a full `Quota` body — no `204` anywhere in this block
   (confirms the `/automations/{id}` pitfall was NOT copied).

4. **[auto]** Run `grep -n -A3 "operationId: getQuota\b" backend/api/crm.yaml | grep x-mcp-tool`
   and `grep -n -A3 "operationId: listQuotas" backend/api/crm.yaml | grep x-mcp-tool`.
   Expected: both carry `x-mcp-tool: { verb: search_records, record_type: quota, tier: green }`
   — literal to the spec's acceptance criteria (see plan Global Constraints for why `getQuota`
   uses `search_records` rather than the more common `read_record`).

5. **[auto]** Run `grep -n -A20 "operationId: createQuota" backend/api/crm.yaml | grep -B3 -A8 "owner_xor_team_required"`.
   Expected: two documented `422` examples (`bothSet`/`neitherSet`), each carrying `code:
   validation_error` at the top level and `details.errors[].code: owner_xor_team_required`
   naming the specific violation.

6. **[auto]** Run `grep -n -A6 "CreateQuotaRequest:" backend/api/crm.yaml`.
   Expected: `owner_id`/`team_id` are each individually nullable — no OpenAPI `oneOf`/`required`
   forcing exactly one; the XOR contract is documented in prose + the 422 example (step 5), not
   schema-enforced.

7. **[auto]** Run `grep -n -A15 "operationId: getQuotaAttainment" backend/api/crm.yaml`.
   Expected: no `x-mcp-tool` key in this operation's block; a documented `422` with two
   examples, `attainment_target_zero` and `attainment_computation_failed`.

8. **[auto]** Run `grep -n -A12 "QuotaAttainment:" backend/api/crm.yaml`.
   Expected: `required` includes `attainment_pct`, `gap_minor`, `pace_pct`, `band`,
   `contributing_deals`; `band` is an enum `[met, accent, behind]` (RD-PARAM-4).

9. **[auto]** Run `grep -n "Quota:" backend/api/crm.yaml -A20 | grep -i "source\|captured_by\|created_by"`.
   Expected: no matches — the `Quota` schema carries no provenance fields at all (Global
   Constraints).

10. **[auto]** Run:
    ```bash
    oasdiff breaking origin/main:backend/api/crm.yaml backend/api/crm.yaml --fail-on ERR
    ```
    Expected: exits 0 — confirms every RD-WIRE-2/3 change landed additive-only.

11. **[auto]** Run `make gen-mcp-tools-check`.
    Expected: exits 0; `tools_gen.go` carries `listQuotas`/`createQuota`/`getQuota`/
    `updateQuota`/`archiveQuota` rows (not `getQuotaAttainment` — it has no `x-mcp-tool`), no
    drift.

12. **[auto]** Run `go test ./backend/internal/contracts/...`.
    Expected: exits 0 — `conformance_test.go`'s `ServerInterface` assertion passes with
    `QuotasAdapter` registered.

13. **[auto]** Run `make check-q`.
    Expected: exits 0; `GATE-CORE-1` (`gen-types-check`) passes.

14. **[auto]** Run `make test-contracts`.
    Expected: all TS contract-compliance tests `PASS`, including the new RD-WIRE-2/3 describe
    blocks (Tasks 1-5).

15. **[auto]** Run `make fe-typecheck`.
    Expected: exits 0 — the hand-maintained `index.ts` exports (`Quota`/`QuotaListResponse`/
    `CreateQuotaRequest`/`UpdateQuotaRequest`/`QuotaAttainment`/`QuotaAttainmentDeal`)
    type-check against the regenerated `crm.d.ts`.
