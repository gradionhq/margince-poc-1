# CF-T01 — Manual/Live UAT: custom-fields admin operations contract

Contract-only ticket — no handler/business logic ships in this branch (spec Out-of-scope), so
every step is contract/gate inspection rather than a live HTTP round-trip against a real
`custom_field` catalog table (that table doesn't exist yet — CF-T02's migration). All steps are
`[auto]`.

1. **[auto]** Run `make gen-types` then `git status --short`.
   Expected: exits 0; diff (if any, from a prior dirty state) touches only
   `backend/api/crm.yaml`, `backend/internal/contracts/types/crm_gen.go`,
   `frontend/src/lib/api-client/generated/crm.d.ts` — clean regeneration, no drift.

2. **[auto]** Run `grep -n "operationId: listCustomFields\|operationId: createCustomField\|operationId: renameCustomField\|operationId: retireCustomField" backend/api/crm.yaml`.
   Expected: all four operationIds present, on `GET /custom-fields`, `POST /custom-fields`,
   `PATCH /custom-fields/{id}`, `POST /custom-fields/{id}/retire` respectively.

3. **[auto]** Run `grep -n -A3 "^  /custom-fields:" backend/api/crm.yaml | grep "required: true"`.
   Expected: the `object` query parameter on `listCustomFields` is `required: true` (enum
   `person|organization|deal|lead|activity`, CUSTOM-FIELDS-PARAM-2).

4. **[auto]** Run `grep -n -B2 -A3 "name: status" backend/api/crm.yaml | grep -A3 "in: query"`
   and inspect around `listCustomFields`.
   Expected: `status` is an optional `active|retired` filter with no default — the admin list
   includes retired fields when omitted.

5. **[auto]** Run `grep -n -A2 "operationId: createCustomField" backend/api/crm.yaml` then
   `grep -n -A2 "x-mcp-tool" backend/api/crm.yaml | grep -A2 "custom_field"`.
   Expected: `createCustomField` carries `x-mcp-tool: { verb: create_record, record_type:
   custom_field, tier: yellow }` (statically 🟡, never dynamic) and the
   `#/components/parameters/ApprovalToken` reference in its `parameters:` list.

6. **[auto]** Run `grep -n -A8 "structuralChangeRefused" backend/api/crm.yaml`.
   Expected: a documented 422 example on `createCustomField` with `code:
   structural_change_refused`, `status: 422`, and `details.route: source_development_path`.

7. **[auto]** Run `grep -n "API-ERR-22" docs/architecture/api-conventions.md`.
   Expected: one row naming `ErrStructuralChangeRefused` / `structural_change_refused` / 422.

8. **[auto]** Run `grep -n -A6 "operationId: renameCustomField" backend/api/crm.yaml`.
   Expected: `RenameCustomFieldRequest` request body; no `column_name`, `object`, or `type`
   property anywhere in that schema (`grep -n -A6 "RenameCustomFieldRequest:" backend/api/crm.yaml`
   shows only `label`).

9. **[auto]** Run `grep -n -A10 "operationId: retireCustomField" backend/api/crm.yaml`.
   Expected: 200 response description explicitly notes `archived_at` stays null; 403 carries an
   `approval_required` example; `#/components/parameters/ApprovalToken` present.

10. **[auto]** Run:
    ```bash
    oasdiff breaking origin/main:backend/api/crm.yaml backend/api/crm.yaml --fail-on ERR
    ```
    Expected: exits 0 — confirms every CUSTOM-FIELDS-WIRE-1..5 change landed additive-only.

11. **[auto]** Run `make check-q`.
    Expected: exits 0; `GATE-CORE-1` (`gen-types-check`) passes.

12. **[auto]** Run `make test-contracts`.
    Expected: all TS contract-compliance tests `PASS`, including the new CUSTOM-FIELDS-WIRE-1
    through -4/5 describe blocks.

13. **[auto]** Run `make fe-typecheck`.
    Expected: exits 0 — the hand-maintained `index.ts` exports
    (`CustomField`/`CreateCustomFieldRequest`/`RenameCustomFieldRequest`/
    `CustomFieldListResponse`) type-check against the regenerated `crm.d.ts`.
