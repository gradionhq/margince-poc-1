// Package transport holds the deal module's HTTP handler for /deals
// (createDeal, updateDeal, listDeals — T11). Mirrors
// modules/people/transport's package layout and its documented
// minimal-duplication convention: the shared HTTP helpers in handler_http.go
// are deliberately duplicated from people/transport rather than exported
// solely for this handler's benefit.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// advanceDealTool is the advanceDeal x-mcp-tool declaration (DEAL-WIRE-4,
// advance_deal/deal/dynamic — see tools_gen.go's generated table). Its tier
// is resolved per-call by the "target_stage_semantic" resolver (registered
// at cmd/api composition, backed by deals.ResolveDynamicTier).
var advanceDealTool = mcp.GeneratedTool{OperationID: "advanceDeal", Verb: "advance_deal", RecordType: "deal", Tier: mcp.TierDynamic, Resolver: "target_stage_semantic"}

const (
	codeStageNotInPipeline = "stage_not_in_pipeline"
	fieldToStageID         = "to_stage_id"
	fieldExistingID        = "existing_id"
)

// stageSemanticReader is the full DealStore seam DealHandler uses — an
// interface so the 403/422 pre-store advance gate paths are unit-testable
// without Postgres.
type stageSemanticReader interface {
	Get(ctx context.Context, id, workspaceID string) (directory.Deal, error)
	GetAny(ctx context.Context, id, workspaceID string) (directory.Deal, error)
	StageSemantic(ctx context.Context, stageID, workspaceID string) (string, error)
	Advance(ctx context.Context, id, workspaceID string, in directory.AdvanceInput, ifMatch int64, changedBy string) (directory.Deal, error)
	FindByIdempotencyKey(ctx context.Context, workspaceID, key string) (directory.Deal, bool, error)
	Create(ctx context.Context, d directory.Deal, idempotencyKey string) (directory.Deal, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (directory.Deal, error)
	ListFiltered(ctx context.Context, workspaceID, cursor string, limit int, filter directory.DealListFilter) ([]directory.Deal, string, error)
	Restore(ctx context.Context, id, workspaceID string) (directory.Deal, error)
	Archive(ctx context.Context, id, workspaceID string) (directory.Deal, error)
}

type dealStakeholderReader interface {
	List(ctx context.Context, workspaceID, cursor string, limit int, filter directory.RelationshipListFilter) ([]directory.Relationship, string, error)
}

// DealHandler routes /deals, /deals/{id}, /deals/{id}/advance, and
// /deals/{id}/stakeholders requests to the DealStore and relationship store.
type DealHandler struct {
	store         stageSemanticReader
	relStore      dealStakeholderReader
	activityStore *directory.ActivityStore
	verifier      approvalsport.Verifier // used only by the 🟡 advance path's toolgate.Enforce call
}

// NewDealHandler returns a DealHandler. relStore backs listDealStakeholders
// and the deal-360 stakeholders array; activityStore backs the deal-360
// timeline array; verifier is the approval-token verify/consume seam
// toolgate.Enforce calls on the 🟡 advance path — kept separate from store
// so store tx boundaries are untouched.
func NewDealHandler(store *directory.DealStore, relStore *directory.RelationshipStore, activityStore *directory.ActivityStore, verifier approvalsport.Verifier) *DealHandler {
	return &DealHandler{store: store, relStore: relStore, activityStore: activityStore, verifier: verifier}
}

// ServeHTTP dispatches on method + path suffix.
func (h *DealHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.serveSuffixRoutes(w, r) {
		return
	}
	id := pathID(r.URL.Path, "/deals")
	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodGet && id != "":
		h.get(w, r, id)
	case r.Method == http.MethodPost && id == "":
		h.create(w, r)
	case r.Method == http.MethodPatch && id != "":
		h.update(w, r, id)
	case r.Method == http.MethodDelete && id != "":
		h.archive(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

// serveSuffixRoutes handles the /deals/{id}/advance, /deals/{id}/stakeholders,
// and /deals/{id}/restore sub-resource routes, reporting whether it handled
// the request (mirrors people/transport's PersonHandler.serveSuffixRoutes).
func (h *DealHandler) serveSuffixRoutes(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/advance") {
		id := pathID(strings.TrimSuffix(r.URL.Path, "/advance"), "/deals")
		h.advance(w, r, id)
		return true
	}
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/stakeholders") {
		id := pathID(strings.TrimSuffix(r.URL.Path, "/stakeholders"), "/deals")
		h.stakeholders(w, r, id)
		return true
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/restore") {
		id := pathID(strings.TrimSuffix(r.URL.Path, "/restore"), "/deals")
		h.restore(w, r, id)
		return true
	}
	return false
}

func (h *DealHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
	var body struct {
		Name              string  `json:"name"`
		AmountMinor       *int64  `json:"amount_minor"`
		Currency          *string `json:"currency"`
		PipelineID        string  `json:"pipeline_id"`
		StageID           string  `json:"stage_id"`
		OrganizationID    *string `json:"organization_id"`
		OwnerID           *string `json:"owner_id"`
		PartnerOrgID      *string `json:"partner_org_id"`
		ExpectedCloseDate *string `json:"expected_close_date"`
		Source            string  `json:"source"`
		CapturedBy        string  `json:"captured_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.Name == "" || body.PipelineID == "" || body.StageID == "" || body.Source == "" || body.CapturedBy == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_required_fields")
		return
	}

	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" {
		if existing, ok, err := h.store.FindByIdempotencyKey(r.Context(), wsID, idemKey); err != nil {
			jsonErr(w, err)
			return
		} else if ok {
			jsonCreatedAt(w, existing, "/deals/"+existing.ID)
			return
		}
	}

	d := directory.NewDeal(body.Name, body.PipelineID, body.StageID,
		provenanceOf(body.Source, body.CapturedBy))
	d.WorkspaceID = wsID
	d.AmountMinor = body.AmountMinor
	d.Currency = body.Currency
	d.OrganizationID = body.OrganizationID
	d.OwnerID = body.OwnerID
	d.PartnerOrgID = body.PartnerOrgID
	if body.ExpectedCloseDate != nil {
		t, err := time.Parse("2006-01-02", *body.ExpectedCloseDate)
		if err != nil {
			jsonProblem(w, http.StatusBadRequest, "bad_expected_close_date")
			return
		}
		d.ExpectedCloseDate = &t
	}

	created, err := h.store.Create(r.Context(), d, idemKey)
	if errors.Is(err, errs.ErrStageNotInPipeline) {
		jsonValidationError(w, "stage_id does not belong to pipeline_id.",
			[]fieldError{{Field: "stage_id", Code: codeStageNotInPipeline}})
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonCreatedAt(w, created, "/deals/"+created.ID)
}

func (h *DealHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	ifMatch, malformed := parseIfMatch(r)
	if malformed {
		jsonProblem(w, http.StatusBadRequest, "bad_if_match")
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}

	d, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	if errors.Is(err, errs.ErrStageNotInPipeline) {
		jsonValidationError(w, "stage_id does not belong to pipeline_id.",
			[]fieldError{{Field: "stage_id", Code: codeStageNotInPipeline}})
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
	jsonOK(w, d)
}

func (h *DealHandler) restore(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	restored, err := h.store.Restore(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, restored)
}

func (h *DealHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	archived, err := h.store.Archive(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, archived)
}

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

	updated, err := h.store.Advance(r.Context(), id, wsID, directory.AdvanceInput{
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

var dealSortColumns = map[string]bool{
	"created_at":          true,
	"updated_at":          true,
	"amount_minor":        true,
	"expected_close_date": true,
	"last_activity_at":    true,
}

func (h *DealHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	sort := q.Get("sort")
	for _, f := range strings.Split(sort, ",") {
		f = strings.TrimSpace(strings.TrimPrefix(f, "-"))
		if f != "" && !dealSortColumns[f] {
			jsonProblem(w, http.StatusUnprocessableEntity, "sort_field_not_allowed")
			return
		}
	}

	stalled := false
	if s := q.Get("stalled"); s != "" {
		stalled, _ = strconv.ParseBool(s)
	}

	filter := directory.DealListFilter{
		PipelineID:       q.Get("pipeline_id"),
		StageID:          q.Get("stage_id"),
		OwnerID:          q.Get("owner_id"),
		OrganizationID:   q.Get("organization_id"),
		Status:           q.Get("status"),
		Stalled:          stalled,
		ForecastCategory: q.Get("forecast_category"),
		PartnerOrgID:     q.Get("partner_org_id"),
		PersonID:         q.Get("person_id"),
		Sort:             sort,
	}

	items, next, err := h.store.ListFiltered(r.Context(), wsID, q.Get("cursor"), queryLimit(r), filter)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

// stakeholders serves GET /deals/{id}/stakeholders: the live
// deal_stakeholder rows for the deal, backed by idx_rel_deal_stakeholders.
func (h *DealHandler) stakeholders(w http.ResponseWriter, r *http.Request, id string) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	if _, err := h.store.Get(r.Context(), id, wsID); err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			jsonProblem(w, http.StatusNotFound, "not_found")
			return
		}
		jsonErr(w, err)
		return
	}
	items, next, err := h.relStore.List(r.Context(), wsID, "", 100, directory.RelationshipListFilter{
		DealID: id,
		Kind:   "deal_stakeholder",
	})
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

// dealTimelineRef is DealDetail's timeline entry — identity fields only,
// never the full activity body (mirrors crm.yaml's DealTimelineRef). Kept
// local to this package rather than reusing directory.ActivityRef: same
// shape, deliberately duplicated per this package's established
// per-package-helper convention.
type dealTimelineRef struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Subject    *string   `json:"subject"`
	OccurredAt time.Time `json:"occurred_at"`
}

// dealDetailResponse is the deal-360 composite read — the deal itself plus
// stakeholders and timeline.
type dealDetailResponse struct {
	directory.Deal
	Stakeholders []directory.Relationship `json:"stakeholders"`
	Timeline     []dealTimelineRef        `json:"timeline"`
}

func (h *DealHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	d, err := h.store.GetAny(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}

	stakeholders, _, err := h.relStore.List(r.Context(), wsID, "", 100, directory.RelationshipListFilter{
		DealID: id,
		Kind:   "deal_stakeholder",
	})
	if err != nil {
		jsonErr(w, err)
		return
	}

	acts, _, err := h.activityStore.List(r.Context(), wsID, "deal", id, "", 50)
	if err != nil {
		jsonErr(w, err)
		return
	}
	timeline := make([]dealTimelineRef, len(acts))
	for i, a := range acts {
		timeline[i] = dealTimelineRef{ID: a.ID, Kind: a.Kind, Subject: a.Subject, OccurredAt: a.OccurredAt}
	}

	jsonOK(w, dealDetailResponse{Deal: d, Stakeholders: stakeholders, Timeline: timeline})
}
