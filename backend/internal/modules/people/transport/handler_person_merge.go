// Package transport (see handler_person.go's package doc) — this file holds
// PersonHandler's POST /people/{id}/merge endpoint and its two supporting
// problem+json writers, split out of handler_person.go to stay under the
// 500-LOC-per-file cap (architecture/18 §3.2) once the merge endpoint
// (mergePerson, APPR-WIRE-1) and the person-360 composite read landed in the
// same file.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// mergePersonTool is the mergePerson x-mcp-tool declaration (crm.yaml:335,
// merge_records/person/yellow — see tools_gen.go's generated table).
var mergePersonTool = mcp.GeneratedTool{OperationID: "mergePerson", Verb: "merge_records", RecordType: "person", Tier: mcp.TierYellow}

// jsonProblemDetails writes a problem+json body with request-specific detail data.
// Duplicated from directory/transport's handler_org.go (see package doc above).
func jsonProblemDetails(w http.ResponseWriter, status int, code, detail string, details map[string]any) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{fieldStatus: status, fieldCode: code, "detail": detail, fieldDetails: details}) //nolint:errcheck,gosec
}

// jsonValidationError writes a 422 problem+json body with the field-level
// details.errors shape the contract's ValidationError schema declares.
// Duplicated from directory/transport's handler_deal.go (see package doc above).
func jsonValidationError(w http.ResponseWriter, detail string, errs []fieldError) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,gosec
		fieldStatus:  http.StatusUnprocessableEntity,
		fieldCode:    "validation_error",
		"detail":     detail,
		fieldDetails: map[string]any{"errors": errs},
	})
}

// merge implements POST /people/{id}/merge (mergePerson, APPR-WIRE-1, x-mcp-tool
// merge_records/person/yellow). A human principal's direct call is itself the
// approval — no token required, mirroring checkApprovalGate's human bypass in
// directory/transport/handler_deal.go. An agent principal must present a
// single-use X-Approval-Token bound to this exact (workspace, tool, diff).
func (h *PersonHandler) merge(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	var body struct {
		TargetID string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TargetID == "" {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	p, _ := crmctx.From(r.Context())
	diffFields := map[string]any{"person_id": id, "target_id": body.TargetID}
	if err := toolgate.Enforce(r.Context(), p, h.verifier, mergePersonTool, wsID, diffFields, nil, r.Header.Get("X-Approval-Token")); err != nil {
		if errors.Is(err, toolgate.ErrApprovalRequired) {
			jsonProblem(w, http.StatusForbidden, "approval_required")
		} else {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid")
		}
		return
	}
	merged, err := h.store.Merge(r.Context(), id, body.TargetID, wsID)
	if errors.Is(err, directory.ErrSelfMerge) {
		jsonValidationError(w, "target_id must not equal id.", []fieldError{{Field: "target_id", Code: "self_merge"}})
		return
	}
	var already *directory.ErrAlreadyMerged
	if errors.As(err, &already) {
		jsonProblemDetails(w, http.StatusUnprocessableEntity, "already_merged",
			"This record was already merged.", map[string]any{fieldExistingID: already.SurvivorID})
		return
	}
	var targetInvalid *directory.ErrMergeTargetInvalid
	if errors.As(err, &targetInvalid) {
		jsonProblemDetails(w, http.StatusUnprocessableEntity, "merge_target_invalid",
			"The merge target is archived or itself already merged.", map[string]any{fieldExistingID: targetInvalid.SurvivorID})
		return
	}
	if errors.Is(err, errs.ErrVersionSkew) {
		jsonProblem(w, http.StatusConflict, "version_skew")
		return
	}
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, merged)
}
