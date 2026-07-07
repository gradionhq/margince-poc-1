package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// InvoicesAdapter implements the Invoices tag's slice of types.ServerInterface.
// Invoices has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type InvoicesAdapter struct{}

// ListInvoices is unimplemented; see InvoicesAdapter's doc comment.
func (InvoicesAdapter) ListInvoices(w http.ResponseWriter, r *http.Request, params types.ListInvoicesParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GenerateInvoice is unimplemented; see InvoicesAdapter's doc comment.
func (InvoicesAdapter) GenerateInvoice(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetInvoice is unimplemented; see InvoicesAdapter's doc comment.
func (InvoicesAdapter) GetInvoice(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
