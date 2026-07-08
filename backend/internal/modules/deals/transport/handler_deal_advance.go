package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// ---------------------------------------------------------------------------
// DealHandler — stage-advance path (advance, checkApprovalGate).
// ---------------------------------------------------------------------------

//nolint:cyclop // HTTP boundary: each advance error maps to a distinct status code; 16 is 1 over the lint max
func (h *DealHandler) advance(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	ifMatch, malformed := parseIfMatch(r)
	if malformed {
		jsonProblem(w, http.StatusBadRequest, "bad_if_match")
		return
	}

	var body struct {
		ToStageID  string  `json:"to_stage_id"`
		Status     string  `json:"status"`
		LostReason *string `json:"lost_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ToStageID == "" {
		jsonValidationError(w, fieldToStageID+" is required.", []fieldError{{Field: fieldToStageID, Code: codeRequired}})
		return
	}

	deal, err := h.store.Get(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}

	fromSemantic, err := h.store.StageSemantic(r.Context(), deal.StageID, wsID)
	if err != nil {
		jsonErr(w, err)
		return
	}
	toSemantic, err := h.store.StageSemantic(r.Context(), body.ToStageID, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonValidationError(w, fieldToStageID+" does not belong to this workspace.",
			[]fieldError{{Field: fieldToStageID, Code: codeStageNotInPipeline}})
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}

	p, _ := crmctx.From(r.Context())
	if !h.checkApprovalGate(r, w, id, wsID, fromSemantic, toSemantic, body.ToStageID, body.LostReason, ifMatch, p) {
		return
	}

	updated, err := h.store.Advance(r.Context(), id, wsID, domain.AdvanceInput{
		ToStageID: body.ToStageID, Status: body.Status, LostReason: body.LostReason,
	}, ifMatch, p.UserID)
	if errors.Is(err, errs.ErrStageNotInPipeline) {
		jsonValidationError(w, fieldToStageID+" does not belong to this deal's pipeline.",
			[]fieldError{{Field: fieldToStageID, Code: codeStageNotInPipeline}})
		return
	}
	if errors.Is(err, errs.ErrStatusMismatch) {
		jsonValidationError(w, "status does not match the target stage's semantic.",
			[]fieldError{{Field: "status", Code: "status_mismatch"}})
		return
	}
	if errors.Is(err, errs.ErrLostReasonRequired) {
		jsonValidationError(w, "lost_reason is required when advancing to a lost stage.",
			[]fieldError{{Field: "lost_reason", Code: "lost_reason_required"}})
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
	jsonOK(w, updated)
}

// checkApprovalGate enforces the X-Approval-Token requirement for agent
// callers on 🟡 transitions via toolgate.Enforce (AC-D2): the
// "target_stage_semantic" dynamic resolver (registered at cmd/api
// composition) re-derives the tier from from_semantic/to_semantic, so this
// call site no longer computes deals.ResolveTier directly. Returns false if
// the request was rejected (w already has a problem response); returns true
// to let the caller proceed. Green-tier transitions and human callers always
// return true without writing.
func (h *DealHandler) checkApprovalGate(r *http.Request, w http.ResponseWriter, id, wsID, fromSemantic, toSemantic, toStageID string, lostReason *string, ifMatch int64, p crmctx.Principal) bool {
	diffFields := map[string]any{
		"deal_id":       id,
		fieldToStageID:  toStageID,
		"status":        toSemantic,
		"from_semantic": fromSemantic,
		"to_semantic":   toSemantic,
	}
	if lostReason != nil {
		diffFields["lost_reason"] = *lostReason
	}
	var targetVersion *int64
	if ifMatch != 0 {
		targetVersion = &ifMatch
	}
	if err := toolgate.Enforce(r.Context(), p, h.verifier, advanceDealTool, wsID, diffFields, targetVersion, r.Header.Get("X-Approval-Token")); err != nil {
		if errors.Is(err, toolgate.ErrApprovalRequired) {
			jsonProblem(w, http.StatusForbidden, "approval_required")
		} else {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid")
		}
		return false
	}
	return true
}
