package server

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ConversationsAdapter implements the Conversations tag's slice of types.ServerInterface.
// Conversations has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type ConversationsAdapter struct{}

// LinkConversation is unimplemented; see ConversationsAdapter's doc comment.
func (ConversationsAdapter) LinkConversation(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UnlinkConversation is unimplemented; see ConversationsAdapter's doc comment.
func (ConversationsAdapter) UnlinkConversation(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	w.WriteHeader(http.StatusNotImplemented)
}
