package server

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// DealRoomsAdapter implements the DealRooms tag's slice of types.ServerInterface.
// DealRooms has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type DealRoomsAdapter struct{}

// PublishDealRoom is unimplemented; see DealRoomsAdapter's doc comment.
func (DealRoomsAdapter) PublishDealRoom(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ResolveDealRoomByToken is unimplemented; see DealRoomsAdapter's doc comment.
func (DealRoomsAdapter) ResolveDealRoomByToken(w http.ResponseWriter, r *http.Request, params types.ResolveDealRoomByTokenParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
