# RD-T02 — Manual/Live UAT: attachments + hierarchy-rollup + field-history contract mint

Contract-only ticket — no handler/business logic ships in this branch, and none of the six new
operations is registered on the real live mux. Every `[auto]` step below is a build/codegen/
gate-level check; the two `[live]` steps boot the real stack only to confirm honest 404s (no
handler is wired), not to exercise business behavior.

1. **[auto]** Run `make gen-types` from repo root. Expected: exits 0, prints `gen-types: wrote
   backend/internal/contracts/types/crm_gen.go + frontend/src/lib/api-client/generated/crm.d.ts`.
2. **[auto]** Run `git diff --name-only` (against `main`). Expected: only
   `backend/api/crm.yaml`, `backend/internal/contracts/types/crm_gen.go`,
   `frontend/src/lib/api-client/generated/crm.d.ts`,
   `backend/internal/contracts/server/attachments_adapter.go` (new),
   `backend/internal/contracts/server/organizations_adapter.go`,
   `backend/internal/contracts/server/audit_adapter.go`,
   `backend/internal/contracts/server/all_operations.go`, plus this guide and the plan file —
   no migration, no other handler file.
3. **[auto]** Run `grep -n "operationId: listAttachments\|operationId: createAttachment\|operationId: getAttachment\|operationId: archiveAttachment\|operationId: getOrganizationHierarchyRollup\|operationId: getFieldHistory" backend/api/crm.yaml | wc -l`.
   Expected: `6` — all six new operationIds present.
4. **[auto]** Run `grep -n "^    Attachment:\|^    CreateAttachmentRequest:\|^    AttachmentListResponse:\|^    OrganizationHierarchyRollup:\|^    FieldHistoryEntry:\|^    FieldHistoryListResponse:" backend/api/crm.yaml | wc -l`.
   Expected: `6` — all six new schema components present.
5. **[auto]** Run `grep -A2 "^    Attachment:" backend/api/crm.yaml | grep -c "version"`.
   Expected: `0` — no `version` field on `Attachment` (RD-DDL-1 has none).
6. **[auto]** Run `grep -n "entity_type: { type: string, enum: \[person, organization, deal, lead, activity\] }" backend/api/crm.yaml | wc -l`.
   Expected: a non-zero count — the five-value entity_type enum is used consistently across the
   new `Attachment`/`CreateAttachmentRequest`/list-filter/field-history-filter param sites.
7. **[auto]** Run `grep -n "scan_status" backend/api/crm.yaml | wc -l`. Expected: non-zero — the
   `scanning`/`clean`/`blocked` vocabulary (RD-PARAM-5) is present.
8. **[auto]** Run `grep -n "upload_url\|download_url" backend/api/crm.yaml | wc -l`. Expected:
   non-zero — both presigned-URL fields documented on `Attachment`.
9. **[auto]** Run `grep -n "restricted_excluded" backend/api/crm.yaml | wc -l`. Expected:
   non-zero — the hierarchy-rollup's disclosed-exclusion field is present.
10. **[auto]** Run `grep -n "operationId: getRecordHistory" backend/api/crm.yaml | wc -l`.
    Expected: `1` — the pre-existing whole-mutation history read is untouched, still present
    exactly once (RD-WIRE-5's `getFieldHistory` is additive, not a replacement).
11. **[auto]** Run `bash scripts/gen-types.sh check`. Expected: exits 0,
    `gen-types-check: generated types are up to date`.
12. **[auto]** Run `git fetch origin main && bash scripts/check-contract-breaking.sh`. Expected:
    exits 0, `contract-breaking-check: no breaking API changes since origin/main`.
13. **[auto]** Run `cd backend && go build ./... && go vet ./... && go test
    ./internal/contracts/... ./cmd/...`. Expected: exits 0, including
    `TestAllOperationsSatisfiesServerInterface` and `TestEveryServedContractOpIsRouted`.
14. **[auto]** Run `make check`. Expected: exits 0 — the full 19-gate suite (including
    `contract-breaking-check`, `gen-types-check`, `gen-manifests-check`, `gen-mcp-tools-check`,
    `arch-lint`, `audit-coverage`, `audit-coherence`, `rls-store-path`, and both Go + frontend
    test suites) is green.
15. **[live]** Boot the real stack (`make infra-up && make migrate-up && make seed-reset && make
    run`), then `curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/attachments`
    with a valid session/bearer credential. Expected: **not** `200`/`201`/`501` — the real mux
    has no route registered for `/attachments` at all in this branch (`buildAllOperations` is
    compile-time-conformance only, never wired into `routes.go`'s mux), so this honestly 404s
    (or the same generic auth/not-found response any unmounted path gets). This is the expected,
    correct behavior for a contract-only ticket — it is not a bug to fix here.
16. **[live]** Same stack, `curl -s -o /dev/null -w "%{http_code}\n"
    http://localhost:8080/organizations/<any-existing-id>/hierarchy-rollup`. Expected: the
    generic `/organizations/` subtree handler receives it (mux-level match) but the real
    `OrganizationHandler`'s own internal routing has no case for `hierarchy-rollup`, so it
    answers its own "not found"/"method not handled" response — again, not `200`, and not a
    panic/500. Confirms the route-mount gate's forward-direction pass at Step 13 reflects real
    server behavior, not just a test artifact.
