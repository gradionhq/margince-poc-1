package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// AIAdapter implements the AI tag's slice of types.ServerInterface.
// AI has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type AIAdapter struct{}

// DraftEmail is unimplemented; see AIAdapter's doc comment.
func (AIAdapter) DraftEmail(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// SendEmail is unimplemented; see AIAdapter's doc comment.
func (AIAdapter) SendEmail(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.SendEmailParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ColdStartReadback is unimplemented; see AIAdapter's doc comment.
func (AIAdapter) ColdStartReadback(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
