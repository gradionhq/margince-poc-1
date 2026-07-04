package transport

import (
	"net/http"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
)

var orgSortAllowed = map[string]bool{
	"": true, "id": true, "strength": true, "-strength": true,
}

// OrganizationHandler routes GET /organizations to OrgStore.List.
// Create/get-by-id/update/archive are deferred (org-360 flow, not T18 scope).
type OrganizationHandler struct{ store *directory.OrgStore }

// NewOrganizationHandler returns an OrganizationHandler.
func NewOrganizationHandler(store *directory.OrgStore) *OrganizationHandler {
	return &OrganizationHandler{store: store}
}

func (h *OrganizationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path, "/organizations")
	if r.Method == http.MethodGet && id == "" {
		h.list(w, r)
		return
	}
	http.NotFound(w, r)
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
	cursor := r.URL.Query().Get("cursor")
	limit := queryLimit(r, 20)
	items, next, err := h.store.List(r.Context(), wsID, cursor, limit, sortVal)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}
