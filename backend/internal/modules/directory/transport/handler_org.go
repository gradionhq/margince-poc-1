package transport

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

var orgSortAllowed = map[string]bool{
	"": true, "id": true, "strength": true, "-strength": true,
}

// OrganizationHandler routes /organizations and /organizations/{id} requests
// to the OrgStore.
type OrganizationHandler struct {
	store *directory.OrgStore
	db    *sql.DB // used only for the merge endpoint's VerifyAndConsume (🟡 gate)
}

// NewOrganizationHandler returns an OrganizationHandler.
func NewOrganizationHandler(store *directory.OrgStore, db *sql.DB) *OrganizationHandler {
	return &OrganizationHandler{store: store, db: db}
}

func (h *OrganizationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/merge") {
		id := pathID(strings.TrimSuffix(r.URL.Path, "/merge"), "/organizations")
		h.merge(w, r, id)
		return
	}
	id := pathID(r.URL.Path, "/organizations")
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

// merge implements POST /organizations/{id}/merge (mergeOrganization,
// APPR-WIRE-1, x-mcp-tool merge_records/organization/yellow). A human
// principal's direct call is itself the approval — no token required,
// mirroring checkApprovalGate's human bypass in handler_deal.go. An agent
// principal must present a single-use X-Approval-Token bound to this exact
// (workspace, tool, diff).
func (h *OrganizationHandler) merge(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	var body struct {
		TargetID string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TargetID == "" {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	p, _ := crmctx.From(r.Context())
	if p.IsAgent {
		token := r.Header.Get("X-Approval-Token")
		if token == "" {
			jsonProblem(w, http.StatusForbidden, "approval_required")
			return
		}
		diffHash := crmapprovals.HashDiff(map[string]any{"organization_id": id, "target_id": body.TargetID})
		if err := crmapprovals.VerifyAndConsume(r.Context(), h.db, token, crmapprovals.Binding{
			// Tool MUST be the contract's declared x-mcp-tool verb ("merge_records",
			// crm.yaml:557), not a per-entity string — see handler_person.go's merge
			// for the full rationale. Person vs. org is disambiguated by diff_hash
			// (organization_id/target_id here) alone.
			WorkspaceID: wsID, Tool: "merge_records", DiffHash: diffHash,
		}); err != nil {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid")
			return
		}
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

func (h *OrganizationHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
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
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.DisplayName == "" || body.Source == "" || body.CapturedBy == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_required_fields")
		return
	}

	org := directory.Organization{
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
		org.Domains = make([]directory.OrganizationDomain, len(body.Domains))
		for i, d := range body.Domains {
			org.Domains[i] = directory.OrganizationDomain{
				Domain:    d.Domain,
				IsPrimary: d.IsPrimary != nil && *d.IsPrimary,
			}
		}
	}

	created, err := h.store.Create(r.Context(), org)
	if err != nil {
		var dup *directory.ErrDuplicateDomain
		if errors.As(err, &dup) {
			jsonProblemDetails(w, http.StatusConflict, "duplicate_domain",
				"An active organization already owns this domain.",
				map[string]any{fieldExistingID: dup.ExistingID, "field": dup.Field})
			return
		}
		if errors.Is(err, errs.ErrNullProvenance) {
			jsonValidationError(w, "source and captured_by are required.",
				[]fieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})
			return
		}
		jsonErr(w, err)
		return
	}
	jsonCreatedAt(w, created, "/organizations/"+created.ID)
}

func (h *OrganizationHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	o, err := h.store.GetAny(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, o)
}

func (h *OrganizationHandler) update(w http.ResponseWriter, r *http.Request, id string) {
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

	o, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
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
	jsonOK(w, o)
}

func (h *OrganizationHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
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

func (h *OrganizationHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	sortVal := r.URL.Query().Get("sort")
	if !orgSortAllowed[sortVal] {
		jsonProblem(w, http.StatusUnprocessableEntity, "sort_field_not_allowed")
		return
	}
	q := r.URL.Query()
	filter := directory.OrgListFilter{
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
	limit := queryLimit(r, 20)
	items, next, err := h.store.List(r.Context(), wsID, cursor, limit, sortVal, filter)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

// jsonProblemDetails writes a problem+json body with an arbitrary details map,
// for errors whose machine-readable code needs request-specific data beyond the
// plain status+code jsonProblem covers.
func jsonProblemDetails(w http.ResponseWriter, status int, code, detail string, details map[string]any) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,gosec
		fieldStatus:  status,
		fieldCode:    code,
		"detail":     detail,
		fieldDetails: details,
	})
}
