package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// ApprovalsAdapter implements the Approvals tag's slice of types.ServerInterface.
// Approvals has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type ApprovalsAdapter struct{}

// ListApprovals is unimplemented; see ApprovalsAdapter's doc comment.
func (ApprovalsAdapter) ListApprovals(w http.ResponseWriter, r *http.Request, params types.ListApprovalsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetApproval is unimplemented; see ApprovalsAdapter's doc comment.
func (ApprovalsAdapter) GetApproval(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ApproveApproval is unimplemented; see ApprovalsAdapter's doc comment.
func (ApprovalsAdapter) ApproveApproval(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.ApproveApprovalParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RejectApproval is unimplemented; see ApprovalsAdapter's doc comment.
func (ApprovalsAdapter) RejectApproval(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
