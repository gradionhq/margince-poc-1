package server

import (
	"net/http"
)

// ReportsAdapter implements the Reports tag's slice of types.ServerInterface.
// Reports has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type ReportsAdapter struct{}

// RunReport is unimplemented; see ReportsAdapter's doc comment.
func (ReportsAdapter) RunReport(w http.ResponseWriter, r *http.Request, report string) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ResolveDerivation is unimplemented; see ReportsAdapter's doc comment.
func (ReportsAdapter) ResolveDerivation(w http.ResponseWriter, r *http.Request, handle string) {
	w.WriteHeader(http.StatusNotImplemented)
}
