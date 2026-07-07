// Package transport: shared HTTP/JSON helpers for the activities transport layer.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const (
	fieldData      = "data"
	fieldCode      = "code"
	fieldStatus    = "status"
	codeBadRequest = "bad_request"
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

func queryLimit(r *http.Request) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return 20
}

// parseIfMatch mirrors the three-way contract: empty header → (0, false),
// valid int → (v, false), non-int → (0, true).
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
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

func jsonErr(w http.ResponseWriter, err error) {
	status := errs.HTTPStatus(err)
	jsonProblem(w, status, problemCode(status))
}

func problemCode(status int) string {
	switch status {
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusTooManyRequests:
		return "budget_exceeded"
	case http.StatusUnprocessableEntity:
		return "validation_error"
	case http.StatusUnavailableForLegalReasons:
		return "suppressed"
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

// writeUpdateResult maps a store Update/Patch error to its problem+json
// response, or writes v as a 200 on success.
func writeUpdateResult[T any](w http.ResponseWriter, v T, err error) {
	if errors.Is(err, errs.ErrConflict) {
		jsonProblem(w, http.StatusConflict, "conflict")
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
	jsonOK(w, v)
}
