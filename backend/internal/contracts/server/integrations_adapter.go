package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// IntegrationsAdapter implements the Integrations tag's slice of types.ServerInterface.
// Integrations has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type IntegrationsAdapter struct{}

// GetHubSpotConnection is unimplemented; see IntegrationsAdapter's doc comment.
func (IntegrationsAdapter) GetHubSpotConnection(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ConnectHubSpot is unimplemented; see IntegrationsAdapter's doc comment.
func (IntegrationsAdapter) ConnectHubSpot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// HubspotOAuthCallback is unimplemented; see IntegrationsAdapter's doc comment.
func (IntegrationsAdapter) HubspotOAuthCallback(w http.ResponseWriter, r *http.Request, params types.HubspotOAuthCallbackParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RotateHubSpotConnection is unimplemented; see IntegrationsAdapter's doc comment.
func (IntegrationsAdapter) RotateHubSpotConnection(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RevokeHubSpotConnection is unimplemented; see IntegrationsAdapter's doc comment.
func (IntegrationsAdapter) RevokeHubSpotConnection(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// VerifyHubSpotScopes is unimplemented; see IntegrationsAdapter's doc comment.
func (IntegrationsAdapter) VerifyHubSpotScopes(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
