package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// DraftingAssetsAdapter implements the DraftingAssets tag's slice of types.ServerInterface.
// DraftingAssets has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type DraftingAssetsAdapter struct{}

// ListDraftingAssets is unimplemented; see DraftingAssetsAdapter's doc comment.
func (DraftingAssetsAdapter) ListDraftingAssets(w http.ResponseWriter, r *http.Request, params types.ListDraftingAssetsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateDraftingAsset is unimplemented; see DraftingAssetsAdapter's doc comment.
func (DraftingAssetsAdapter) CreateDraftingAsset(w http.ResponseWriter, r *http.Request, params types.CreateDraftingAssetParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetDraftingAsset is unimplemented; see DraftingAssetsAdapter's doc comment.
func (DraftingAssetsAdapter) GetDraftingAsset(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateDraftingAsset is unimplemented; see DraftingAssetsAdapter's doc comment.
func (DraftingAssetsAdapter) UpdateDraftingAsset(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateDraftingAssetParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ArchiveDraftingAsset is unimplemented; see DraftingAssetsAdapter's doc comment.
func (DraftingAssetsAdapter) ArchiveDraftingAsset(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ApproveDraftingAsset is unimplemented; see DraftingAssetsAdapter's doc comment.
func (DraftingAssetsAdapter) ApproveDraftingAsset(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ExpireDraftingAsset is unimplemented; see DraftingAssetsAdapter's doc comment.
func (DraftingAssetsAdapter) ExpireDraftingAsset(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
