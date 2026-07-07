package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// ComplianceAdapter implements the Compliance tag's slice of types.ServerInterface.
// Compliance has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type ComplianceAdapter struct{}

// ListConsentPurposes is unimplemented; see ComplianceAdapter's doc comment.
func (ComplianceAdapter) ListConsentPurposes(w http.ResponseWriter, r *http.Request, params types.ListConsentPurposesParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateConsentPurpose is unimplemented; see ComplianceAdapter's doc comment.
func (ComplianceAdapter) CreateConsentPurpose(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetPersonConsent is unimplemented; see ComplianceAdapter's doc comment.
func (ComplianceAdapter) GetPersonConsent(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RecordConsent is unimplemented; see ComplianceAdapter's doc comment.
func (ComplianceAdapter) RecordConsent(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RecordConsentParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
