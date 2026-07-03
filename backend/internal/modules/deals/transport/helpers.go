// Package transport holds the deals module's HTTP handlers for
// /pipelines and /stages (T10). Mirrors modules/directory/transport's and
// modules/people/transport's package layout and documented
// minimal-duplication convention: the small HTTP helpers below are
// deliberately duplicated rather than exported solely for this package's
// benefit — same class as those two packages' own copies.
package transport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const (
	fieldStatus    = "status"
	fieldCode      = "code"
	fieldData      = "data"
	fieldDetails   = "details"
	codeConflict   = "conflict"
	codeNotFound   = "not_found"
	codeBadRequest = "bad_request"
	codeValidation = "validation_error"
)

func pathID(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func workspaceID(r *http.Request) string {
	p, _ := crmctx.From(r.Context())
	return p.TenantID
}

func requireWorkspace(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := workspaceID(r)
	if id == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_workspace")
		return "", false
	}
	return id, true
}

func queryLimit(r *http.Request, def int) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}

// No parseIfMatch helper here (unlike handler_deal.go/handler_person.go):
// PipelineStore.Update/StageStore.Update take no ifMatch/optimistic-concurrency
// parameter (see Task 2 Step 4's note on the dropped param) and these handlers
// never read the If-Match header, so an unused parseIfMatch would trip
// golangci-lint's `unused` (U1000) check at the FINAL GATE — do not add it.

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

func jsonErr(w http.ResponseWriter, err error) {
	status := errs.HTTPStatus(err)
	jsonProblem(w, status, problemCode(status))
}

func problemCode(status int) string {
	switch status {
	case http.StatusNotFound:
		return codeNotFound
	case http.StatusConflict:
		return codeConflict
	case http.StatusUnprocessableEntity:
		return codeValidation
	case http.StatusBadRequest:
		return codeBadRequest
	default:
		return "internal_error"
	}
}

func jsonProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{fieldStatus: status, fieldCode: code}) //nolint:errcheck,gosec
}

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
