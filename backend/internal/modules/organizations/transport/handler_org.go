// Package transport holds the organizations module's HTTP handler for /organizations
// (extracted from directory/transport/handler_org.go, WS-E-a restructure).
// The OrgStore comes from organizations/adapters; cross-module stores (rel, deal,
// activity) are accessed through local seam interfaces so this package has no
// compile-time dependency on modules/directory.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	dealdomain "github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	reldomain "github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

var orgSortAllowed = map[string]bool{
	"": true, "id": true, "strength": true, "-strength": true,
}

// mergeOrgTool is the mergeOrganization x-mcp-tool declaration (crm.yaml:557,
// merge_records/organization/yellow — see tools_gen.go's generated table).
var mergeOrgTool = mcp.GeneratedTool{OperationID: "mergeOrganization", Verb: "merge_records", RecordType: "organization", Tier: mcp.TierYellow}

// relStoreSeam is the subset of a relationship store this handler needs for
// the organization-360 composite read. Satisfied by *relationships/adapters.RelationshipStore.
type relStoreSeam interface {
	List(ctx context.Context, workspaceID, cursor string, limit int, filter reldomain.RelationshipListFilter) ([]reldomain.Relationship, string, error)
}

// dealStoreSeam is the subset of a deal store this handler needs for the
// organization-360 composite read. Satisfied by *deals/adapters.DealStore.
type dealStoreSeam interface {
	ListFiltered(ctx context.Context, workspaceID, cursor string, limit int, f dealdomain.DealListFilter) ([]dealdomain.Deal, string, error)
}

// actStoreSeam is the subset of an activity store this handler needs for the
// organization-360 composite read. Satisfied by *activities/adapters.ActivityStore.
type actStoreSeam interface {
	List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]actdomain.Activity, string, error)
}

// OrganizationHandler routes /organizations and /organizations/{id} requests
// to the OrgStore.
type OrganizationHandler struct {
	store         *adapters.OrgStore
	relStore      relStoreSeam
	dealStore     dealStoreSeam
	activityStore actStoreSeam
	verifier      approvalsport.Verifier // used only by the merge endpoint's toolgate.Enforce call (🟡 gate)
}

// NewOrganizationHandler returns an OrganizationHandler.
func NewOrganizationHandler(store *adapters.OrgStore, relStore relStoreSeam, dealStore dealStoreSeam, activityStore actStoreSeam, verifier approvalsport.Verifier) *OrganizationHandler {
	return &OrganizationHandler{store: store, relStore: relStore, dealStore: dealStore, activityStore: activityStore, verifier: verifier}
}

func (h *OrganizationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.serveSuffixRoutes(w, r) {
		return
	}
	id := httpkit.PathID(r.URL.Path, "/organizations")
	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodPost && id == "":
		h.create(w, r)
	case r.Method == http.MethodGet && id != "":
		h.get(w, r, id)
	case r.Method == http.MethodPatch && id != "":
		h.update(w, r, id)
	case r.Method == http.MethodDelete && id != "":
		h.archive(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

// serveSuffixRoutes dispatches the /restore and /merge suffix routes, keeping
// ServeHTTP's cyclomatic complexity within the T1 lint budget (mirrors
// people/transport's handler_person.go serveSuffixRoutes).
func (h *OrganizationHandler) serveSuffixRoutes(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/restore") {
		id := httpkit.PathID(strings.TrimSuffix(r.URL.Path, "/restore"), "/organizations")
		h.restore(w, r, id)
		return true
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/merge") {
		id := httpkit.PathID(strings.TrimSuffix(r.URL.Path, "/merge"), "/organizations")
		h.merge(w, r, id)
		return true
	}
	return false
}

// merge implements POST /organizations/{id}/merge (mergeOrganization,
// APPR-WIRE-1, x-mcp-tool merge_records/organization/yellow). A human
// principal's direct call is itself the approval — no token required,
// mirroring checkApprovalGate's human bypass in handler_deal.go. An agent
// principal must present a single-use X-Approval-Token bound to this exact
// (workspace, tool, diff).
func (h *OrganizationHandler) merge(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	var body struct {
		TargetID string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TargetID == "" {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	p, _ := crmctx.From(r.Context())
	diffFields := map[string]any{"organization_id": id, "target_id": body.TargetID}
	if err := toolgate.Enforce(r.Context(), p, h.verifier, mergeOrgTool, wsID, diffFields, nil, r.Header.Get("X-Approval-Token")); err != nil {
		if errors.Is(err, toolgate.ErrApprovalRequired) {
			httpkit.JSONProblem(w, http.StatusForbidden, "approval_required")
		} else {
			httpkit.JSONProblem(w, http.StatusForbidden, "approval_token_invalid")
		}
		return
	}
	merged, err := h.store.Merge(r.Context(), id, body.TargetID, wsID)
	if errors.Is(err, adapters.ErrSelfMerge) {
		httpkit.JSONValidationError(w, "target_id must not equal id.", []fieldError{{Field: "target_id", Code: "self_merge"}})
		return
	}
	var already *adapters.ErrAlreadyMerged
	if errors.As(err, &already) {
		httpkit.JSONProblemDetails(w, http.StatusUnprocessableEntity, "already_merged",
			"This record was already merged.", map[string]any{fieldExistingID: already.SurvivorID})

		return
	}
	var targetInvalid *adapters.ErrMergeTargetInvalid
	if errors.As(err, &targetInvalid) {
		httpkit.JSONProblemDetails(w, http.StatusUnprocessableEntity, "merge_target_invalid",
			"The merge target is archived or itself already merged.", map[string]any{fieldExistingID: targetInvalid.SurvivorID})

		return
	}
	if errors.Is(err, errs.ErrVersionSkew) {
		httpkit.JSONProblem(w, http.StatusConflict, "version_skew")
		return
	}
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, merged)
}

func (h *OrganizationHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := httpkit.WorkspaceID(r)
	var body struct {
		DisplayName    string  `json:"display_name"`
		Website        *string `json:"website,omitempty"`
		Classification *string `json:"classification,omitempty"`
		Relevance      *int    `json:"relevance,omitempty"`
		OwnerID        *string `json:"owner_id,omitempty"`
		Source         string  `json:"source"`
		CapturedBy     string  `json:"captured_by"`
		Domains        []struct {
			Domain    string `json:"domain"`
			IsPrimary *bool  `json:"is_primary,omitempty"`
		} `json:"domains,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.DisplayName == "" || body.Source == "" || body.CapturedBy == "" {
		httpkit.JSONProblem(w, http.StatusBadRequest, "missing_required_fields")
		return
	}

	org := domain.Organization{
		WorkspaceID: wsID,
		DisplayName: body.DisplayName,
		Website:     body.Website,
		Source:      body.Source,
		CapturedBy:  body.CapturedBy,
	}
	org.Classification = body.Classification
	if body.Relevance != nil {
		org.Relevance = *body.Relevance
	}
	org.OwnerID = body.OwnerID
	if len(body.Domains) > 0 {
		org.Domains = make([]domain.OrganizationDomain, len(body.Domains))
		for i, d := range body.Domains {
			org.Domains[i] = domain.OrganizationDomain{
				Domain:    d.Domain,
				IsPrimary: d.IsPrimary != nil && *d.IsPrimary,
			}
		}
	}

	created, err := h.store.Create(r.Context(), org)
	if err != nil {
		var dup *adapters.ErrDuplicateDomain
		if errors.As(err, &dup) {
			httpkit.JSONProblemDetails(w, http.StatusConflict, "duplicate_domain",
				"An active organization already owns this domain.",
				map[string]any{fieldExistingID: dup.ExistingID, "field": dup.Field})

			return
		}
		if errors.Is(err, errs.ErrNullProvenance) {
			httpkit.JSONValidationError(w, "source and captured_by are required.",
				[]fieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})

			return
		}
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONCreatedAt(w, created, "/organizations/"+created.ID)
}

// organizationDetailResponse is the organization-360 composite read — the
// organization itself plus relationships, deals, and activities. Its own
// Relationships/Deals/Activities fields shadow the embedded Organization's
// `omitempty`-tagged fields so that a single-record read always shows `[]`,
// never `null` or absent, when the composite result set is legitimately empty.
type organizationDetailResponse struct {
	domain.Organization
	Relationships []domain.RelationshipRef `json:"relationships"`
	Deals         []domain.DealRef         `json:"deals"`
	Activities    []domain.ActivityRef     `json:"activities"`
}

func (h *OrganizationHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	o, err := h.store.GetAny(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	rels, deals, acts, err := h.assembleComposite(r.Context(), wsID, o.ID)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, organizationDetailResponse{
		Organization:  o,
		Relationships: rels,
		Deals:         deals,
		Activities:    acts,
	})
}

// assembleComposite fans out to related stores for the organization-360 read,
// converting each store's domain type to the organization module's view-model.
func (h *OrganizationHandler) assembleComposite(ctx context.Context, wsID, orgID string) ([]domain.RelationshipRef, []domain.DealRef, []domain.ActivityRef, error) {
	rawRels, _, err := h.relStore.List(ctx, wsID, "", 50, reldomain.RelationshipListFilter{OrganizationID: orgID})
	if err != nil {
		return nil, nil, nil, err
	}
	rels := make([]domain.RelationshipRef, len(rawRels))
	for i, r := range rawRels {
		rels[i] = domain.RelationshipRef{
			ID:                r.ID,
			WorkspaceID:       r.WorkspaceID,
			Kind:              r.Kind,
			PersonID:          r.PersonID,
			OrganizationID:    r.OrganizationID,
			DealID:            r.DealID,
			CounterpartyOrgID: r.CounterpartyOrgID,
			Role:              r.Role,
			IsCurrentPrimary:  r.IsCurrentPrimary,
			StartedAt:         r.StartedAt,
			EndedAt:           r.EndedAt,
			Version:           r.Version,
			Source:            r.Source,
			CapturedBy:        r.CapturedBy,
			Provenance:        r.Provenance,
			CreatedAt:         r.CreatedAt,
			UpdatedAt:         r.UpdatedAt,
			ArchivedAt:        r.ArchivedAt,
		}
	}

	rawDeals, _, err := h.dealStore.ListFiltered(ctx, wsID, "", 50, dealdomain.DealListFilter{OrganizationID: orgID})
	if err != nil {
		return nil, nil, nil, err
	}
	deals := make([]domain.DealRef, len(rawDeals))
	for i, d := range rawDeals {
		deals[i] = domain.DealRef{
			ID:                d.ID,
			WorkspaceID:       d.WorkspaceID,
			Name:              d.Name,
			AmountMinor:       d.AmountMinor,
			Currency:          d.Currency,
			FxRateToBase:      d.FxRateToBase,
			FxRateDate:        d.FxRateDate,
			PipelineID:        d.PipelineID,
			StageID:           d.StageID,
			OrganizationID:    d.OrganizationID,
			OwnerID:           d.OwnerID,
			PartnerOrgID:      d.PartnerOrgID,
			Status:            d.Status,
			LostReason:        d.LostReason,
			ExpectedCloseDate: d.ExpectedCloseDate,
			ClosedAt:          d.ClosedAt,
			ForecastCategory:  d.ForecastCategory,
			WaitUntil:         d.WaitUntil,
			LastActivityAt:    d.LastActivityAt,
			Stalled:           d.Stalled,
			StageEnteredAt:    d.StageEnteredAt,
			StakeholderCount:  d.StakeholderCount,
			Version:           d.Version,
			Source:            d.Source,
			CapturedBy:        d.CapturedBy,
			Provenance:        d.Provenance,
			CreatedAt:         d.CreatedAt,
			UpdatedAt:         d.UpdatedAt,
			ArchivedAt:        d.ArchivedAt,
		}
	}

	rawActs, _, err := h.activityStore.List(ctx, wsID, "organization", orgID, "", 50)
	if err != nil {
		return nil, nil, nil, err
	}
	acts := make([]domain.ActivityRef, len(rawActs))
	for i, a := range rawActs {
		acts[i] = domain.ActivityRef{ID: a.ID, Kind: a.Kind, Subject: a.Subject, OccurredAt: a.OccurredAt}
	}
	return rels, deals, acts, nil
}

func (h *OrganizationHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	ifMatch, malformed := httpkit.ParseIfMatch(r)
	if malformed {
		httpkit.JSONProblem(w, http.StatusBadRequest, "bad_if_match")
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}

	o, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	if errors.Is(err, errs.ErrVersionSkew) {
		httpkit.JSONProblem(w, http.StatusConflict, "version_skew")
		return
	}
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, o)
}

func (h *OrganizationHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	archived, err := h.store.Archive(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, archived)
}

func (h *OrganizationHandler) restore(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	restored, err := h.store.Restore(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	var dup *adapters.ErrDuplicateDomain
	if errors.As(err, &dup) {
		httpkit.JSONProblemDetails(w, http.StatusConflict, "duplicate_domain",
			"An active organization already owns this domain.",
			map[string]any{"existing_id": dup.ExistingID, "field": dup.Field})

		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, restored)
}

func (h *OrganizationHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	sortVal := r.URL.Query().Get("sort")
	if !orgSortAllowed[sortVal] {
		httpkit.JSONProblem(w, http.StatusUnprocessableEntity, "sort_field_not_allowed")
		return
	}
	q := r.URL.Query()
	filter := domain.OrgListFilter{
		Classification: q.Get("classification"),
		Domain:         q.Get("domain"),
		OwnerID:        q.Get("owner_id"),
	}
	if s := q.Get("relevance_gte"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			filter.RelevanceGTE = &n
		}
	}
	cursor := r.URL.Query().Get("cursor")
	limit := httpkit.QueryLimit(r, 20)
	items, next, err := h.store.List(r.Context(), wsID, cursor, limit, sortVal, filter)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, httpkit.PageResponse(items, next))
}
