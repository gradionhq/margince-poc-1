package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// QuotasAdapter implements the Quotas tag's slice of types.ServerInterface.
// Quotas has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover; RD-T01 mints the contract only — no handler,
// no service/repo code, no quota table migration) — every method returns 501 via the
// same shape oapi-codegen's own generated types.Unimplemented stub uses.
type QuotasAdapter struct{}

// ListQuotas is unimplemented; see QuotasAdapter's doc comment.
func (QuotasAdapter) ListQuotas(w http.ResponseWriter, r *http.Request, params types.ListQuotasParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateQuota is unimplemented; see QuotasAdapter's doc comment.
func (QuotasAdapter) CreateQuota(w http.ResponseWriter, r *http.Request, params types.CreateQuotaParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetQuota is unimplemented; see QuotasAdapter's doc comment.
func (QuotasAdapter) GetQuota(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateQuota is unimplemented; see QuotasAdapter's doc comment.
func (QuotasAdapter) UpdateQuota(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateQuotaParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ArchiveQuota is unimplemented; see QuotasAdapter's doc comment.
func (QuotasAdapter) ArchiveQuota(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetQuotaAttainment is unimplemented; see QuotasAdapter's doc comment.
func (QuotasAdapter) GetQuotaAttainment(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
