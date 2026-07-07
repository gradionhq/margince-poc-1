// Package transport holds the people module's HTTP handler for /people
// (extracted from crm-core's handler_person.go, 1c restructure,
// task-3-brief.md). Person's domain type and store stayed in
// modules/directory rather than splitting into modules/people/{domain,adapters}
// (see task-3-report.md's mapping-deviations section: Person is fused with the
// Lead-promotion spine transaction and the DatasourceProvider seam via
// unexported same-package access that a split would have forced to export —
// not authorized by this task). This handler references directory's exported
// Person/PersonStore/NewPerson/NewPersonStore API only; the small set of HTTP
// helpers and problem-code constants below were previously shared,
// package-private, with handler_audit_history.go (which stayed in directory)
// and are duplicated here rather than exported solely for this handler's
// benefit — same minimal-duplication class as httpserver's keyStatus (see
// internal/platform/httpserver/middleware.go).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

// problem+json response field names and machine-readable codes, and the
// contract-envelope field name, this handler needs. Duplicated from
// directory's consts.go (see package doc above).
const (
	fieldData       = "data"
	fieldCode       = "code"
	fieldStatus     = "status"
	fieldDetails    = "details"
	fieldExistingID = "existing_id"
	fieldErrors     = "errors"
	codeForbidden   = "forbidden"
	codeBadRequest  = "bad_request"
	codeValidation  = "validation_error"
)

type fieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

var personSortValues = map[string]bool{
	"": true, "id": true, "strength": true, "-strength": true,
}

// PersonHandler routes /people and /people/{id} requests to the PersonStore.
type PersonHandler struct {
	store         *directory.PersonStore
	relStore      *directory.RelationshipStore
	dealStore     *directory.DealStore
	activityStore *directory.ActivityStore
	verifier      approvalsport.Verifier // used only by the merge endpoint's toolgate.Enforce call (🟡 gate)
}

// NewPersonHandler returns a PersonHandler.
func NewPersonHandler(store *directory.PersonStore, relStore *directory.RelationshipStore, dealStore *directory.DealStore, activityStore *directory.ActivityStore, verifier approvalsport.Verifier) *PersonHandler {
	return &PersonHandler{store: store, relStore: relStore, dealStore: dealStore, activityStore: activityStore, verifier: verifier}
}

// ServeHTTP dispatches on method + path suffix.
func (h *PersonHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.serveSuffixRoutes(w, r) {
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/merge") {
		id := pathID(strings.TrimSuffix(r.URL.Path, "/merge"), "/people")
		h.merge(w, r, id)
		return
	}
	id := pathID(r.URL.Path, "/people")
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

func (h *PersonHandler) serveSuffixRoutes(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/strength-breakdown") {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/people/"), "/strength-breakdown")
		h.strengthBreakdown(w, r, id)
		return true
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/restore") {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/people/"), "/restore")
		h.restore(w, r, id)
		return true
	}
	return false
}

func (h *PersonHandler) strengthBreakdown(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	result, err := h.store.StrengthBreakdown(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	activities := make([]map[string]any, len(result.ContributingActivities))
	for i, a := range result.ContributingActivities {
		activities[i] = map[string]any{
			"id": a.ID, "kind": a.Kind, "subject": a.Subject, "occurred_at": a.OccurredAt,
		}
	}
	jsonOK(w, map[string]any{
		"person_id":               id,
		"score":                   result.Score,
		"bucket":                  result.Bucket,
		"recency":                 result.Recency,
		"frequency":               result.Frequency,
		"reciprocity":             result.Reciprocity,
		"contributing_activities": activities,
	})
}

func (h *PersonHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	sort := r.URL.Query().Get("sort")
	if !personSortValues[sort] {
		jsonProblem(w, http.StatusUnprocessableEntity, "sort_field_not_allowed")
		return
	}
	cursor := r.URL.Query().Get("cursor")
	limit := queryLimit(r, 20)
	items, next, err := h.store.List(r.Context(), wsID, cursor, limit, sort)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

func (h *PersonHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
	var body struct {
		FullName   string  `json:"full_name"`
		FirstName  *string `json:"first_name"`
		LastName   *string `json:"last_name"`
		Title      *string `json:"title"`
		OwnerID    *string `json:"owner_id"`
		Source     string  `json:"source"`
		CapturedBy string  `json:"captured_by"`
		Emails     []struct {
			Email     string `json:"email"`
			EmailType string `json:"email_type"`
			IsPrimary bool   `json:"is_primary"`
			Position  int    `json:"position"`
		} `json:"emails"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.FullName == "" || body.Source == "" || body.CapturedBy == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_required_fields")
		return
	}
	p := directory.NewPerson(body.FullName, prov.Provenance{Source: body.Source, CapturedBy: body.CapturedBy})
	p.WorkspaceID = wsID
	p.FirstName = body.FirstName
	p.LastName = body.LastName
	p.Title = body.Title
	p.OwnerID = body.OwnerID
	emails := make([]directory.PersonEmailInput, len(body.Emails))
	for i, e := range body.Emails {
		emails[i] = directory.PersonEmailInput{Email: e.Email, EmailType: e.EmailType, IsPrimary: e.IsPrimary, Position: e.Position}
	}
	created, err := h.store.Create(r.Context(), p, emails)
	if err != nil {
		var dup *directory.ErrDuplicateEmail
		if errors.As(err, &dup) {
			jsonProblemDetails(w, http.StatusConflict, "duplicate_email",
				"An active person already owns this email.",
				map[string]any{fieldExistingID: dup.ExistingID, "field": dup.Field})
			return
		}
		jsonErr(w, err)
		return
	}
	// Audit is written by PersonStore.Create at the store layer (covers direct
	// store callers too), so no handler-level audit write is needed here.
	jsonCreated(w, created)
}

// personDetailResponse is the person-360 composite read — the person itself
// plus relationships, deals, and activities. Its own Relationships/Deals/
// Activities fields shadow the embedded Person's `omitempty`-tagged fields of
// the same Go field name (same class as deals/transport's dealDetailResponse:
// list responses must omit these keys when unset, but a single-record read
// must always show `[]`, never `null` or absent, when the composite result
// set is legitimately empty — not expressible via one struct/tag serving both
// list and get semantics).
type personDetailResponse struct {
	directory.Person
	Relationships []directory.Relationship `json:"relationships"`
	Deals         []directory.Deal         `json:"deals"`
	Activities    []directory.ActivityRef  `json:"activities"`
}

func (h *PersonHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	p, err := h.store.GetAny(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	if err := h.assembleComposite(r.Context(), wsID, &p); err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, personDetailResponse{
		Person:        p,
		Relationships: p.Relationships,
		Deals:         p.Deals,
		Activities:    p.Activities,
	})
}

// assembleComposite fans out to related stores for the person-360 read.
func (h *PersonHandler) assembleComposite(ctx context.Context, wsID string, p *directory.Person) error {
	rels, _, err := h.relStore.List(ctx, wsID, "", 50, directory.RelationshipListFilter{PersonID: p.ID})
	if err != nil {
		return err
	}
	p.Relationships = rels

	deals, _, err := h.dealStore.ListFiltered(ctx, wsID, "", 50, directory.DealListFilter{PersonID: p.ID})
	if err != nil {
		return err
	}
	p.Deals = deals

	acts, _, err := h.activityStore.List(ctx, wsID, "person", p.ID, "", 50)
	if err != nil {
		return err
	}
	refs := make([]directory.ActivityRef, len(acts))
	for i, a := range acts {
		refs[i] = directory.ToActivityRef(a)
	}
	p.Activities = refs
	return nil
}

func (h *PersonHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	// If-Match is optional: omitting is last-write-wins (version=0 bypasses check).
	// A present-but-malformed If-Match is a client error (400), never a silent skip.
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
	p, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
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
	// Audit written by PersonStore.Update (store layer).
	jsonOK(w, p)
}

func (h *PersonHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
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
	// Audit written by PersonStore.Archive (store layer).
	jsonOK(w, archived)
}

// ---------------------------------------------------------------------------
// shared HTTP helpers (unexported, used across handler files in this package)
// ---------------------------------------------------------------------------

// pathID extracts the trailing path segment after prefix.
// "/people/abc123" with prefix "/people" → "abc123"
func pathID(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	// strip any further segments
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func workspaceID(r *http.Request) string {
	p, _ := crmctx.From(r.Context())
	return p.TenantID
}

// requireWorkspace writes a 400 and returns false when the request carries no
// workspace ID (i.e. before WP6 auth lands and the frontend sends the header).
func requireWorkspace(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := workspaceID(r)
	if id == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_workspace")
		return "", false
	}
	return id, true
}

// queryLimit reads the ?limit query parameter as an int, falling back to def when
// the parameter is absent or non-numeric.
func queryLimit(r *http.Request, def int) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}

// parseIfMatch parses the optional If-Match header into an optimistic-concurrency
// version. A malformed header is reported distinctly so it is never silently
// downgraded to last-write-wins:
//   - absent      -> (0, malformed=false): last-write-wins is intended
//   - present, ok -> (version, false)
//   - present, bad -> (0, malformed=true): caller MUST reject with 400, not skip the check
func parseIfMatch(r *http.Request) (version int64, malformed bool) {
	s := r.Header.Get("If-Match")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, true
	}
	return v, false
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec // best-effort response body; the status line already conveys success
}

func jsonCreated(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec // best-effort response body; the status line already conveys success
}

// jsonErr maps a domain error to its HTTP status via the single errs.HTTPStatus
// choke point, then emits a problem+json body. Previously it discarded err and always
// emitted 500, swallowing sentinel→status (e.g. a 422 null-provenance read as 500).
func jsonErr(w http.ResponseWriter, err error) {
	status := errs.HTTPStatus(err)
	jsonProblem(w, status, problemCode(status))
}

// problemCode is the stable machine-readable code string for an HTTP status used in
// problem+json bodies, matching the codes the handlers emit at their explicit sites.
func problemCode(status int) string {
	switch status {
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusForbidden:
		return codeForbidden
	case http.StatusTooManyRequests:
		return "budget_exceeded"
	case http.StatusUnprocessableEntity:
		return "unprocessable"
	case http.StatusUnavailableForLegalReasons:
		return "suppressed"
	case http.StatusBadRequest:
		return codeBadRequest
	default:
		return "internal_error"
	}
}

// jsonProblem writes an RFC 7807 application/problem+json response.
func jsonProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{fieldStatus: status, fieldCode: code}) //nolint:errcheck,gosec // best-effort problem body; the status line already conveys the failure
}

// pageResponse wraps a data slice and cursor into the contract's pagination envelope.
func pageResponse(data any, nextCursor string) map[string]any {
	var next any
	hasMore := nextCursor != ""
	if hasMore {
		next = nextCursor
	}
	return map[string]any{
		fieldData: data,
		"page": map[string]any{
			"next_cursor": next,
			"has_more":    hasMore,
		},
	}
}
