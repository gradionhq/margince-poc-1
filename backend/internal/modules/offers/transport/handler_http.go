// Package transport holds the offers module's HTTP handlers for /products and
// /offer-templates.
package transport

import (
	"context"
	"errors"
	"net/http"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
)

const (
	codeBadRequest  = "bad_request"
	fieldSource     = "source"
	fieldCapturedBy = "captured_by"
	codeRequired    = "required"
	fieldExistingID = "existing_id"
	fieldField      = "field"
)

// dispatchCRUD implements the standard /resource + /resource/{id} routing
// shared by every offers HTTP handler.
func dispatchCRUD(w http.ResponseWriter, r *http.Request, pathPrefix string,
	list, create func(http.ResponseWriter, *http.Request),
	get, update, archive func(http.ResponseWriter, *http.Request, string)) {
	id := httpkit.PathID(r.URL.Path, pathPrefix)
	switch {
	case r.Method == http.MethodGet && id == "":
		list(w, r)
	case r.Method == http.MethodPost && id == "":
		create(w, r)
	case r.Method == http.MethodGet && id != "":
		get(w, r, id)
	case r.Method == http.MethodPut && id != "":
		update(w, r, id)
	case r.Method == http.MethodDelete && id != "":
		archive(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

// writeGetResult writes v as 200 OK, or translates errs.ErrNotFound to 404 /
// any other error via httpkit.JSONError — the shared tail of every offers
// get/archive handler.
func writeGetResult[T any](w http.ResponseWriter, v T, err error) {
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, v)
}

// listResults runs fn (a store's List method) with the standard workspace/
// cursor/limit/include_archived query parsing shared by every offers list
// handler, and writes the paginated response.
func listResults[T any](w http.ResponseWriter, r *http.Request,
	fn func(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]T, string, error)) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	includeArchived := q.Get("include_archived") == "true"
	items, next, err := fn(r.Context(), wsID, q.Get("cursor"), httpkit.QueryLimit(r, 20), includeArchived)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, httpkit.PageResponse(items, next))
}
