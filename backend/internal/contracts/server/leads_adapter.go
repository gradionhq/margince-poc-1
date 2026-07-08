package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// LeadsAdapter implements the Leads tag's slice of types.ServerInterface.
// Leads has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type LeadsAdapter struct{}

// ListLeads is unimplemented; see LeadsAdapter's doc comment.
func (LeadsAdapter) ListLeads(w http.ResponseWriter, r *http.Request, params types.ListLeadsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateLead is unimplemented; see LeadsAdapter's doc comment.
func (LeadsAdapter) CreateLead(w http.ResponseWriter, r *http.Request, params types.CreateLeadParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetLead is unimplemented; see LeadsAdapter's doc comment.
func (LeadsAdapter) GetLead(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateLead is unimplemented; see LeadsAdapter's doc comment.
func (LeadsAdapter) UpdateLead(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateLeadParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// DisqualifyLead is unimplemented; see LeadsAdapter's doc comment.
func (LeadsAdapter) DisqualifyLead(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// PromoteLead is unimplemented; see LeadsAdapter's doc comment.
func (LeadsAdapter) PromoteLead(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.PromoteLeadParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetLeadScore is unimplemented; see LeadsAdapter's doc comment.
func (LeadsAdapter) GetLeadScore(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// OverrideLeadScore is unimplemented; see LeadsAdapter's doc comment.
func (LeadsAdapter) OverrideLeadScore(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
