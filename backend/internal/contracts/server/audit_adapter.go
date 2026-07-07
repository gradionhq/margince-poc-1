package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	audittransport "github.com/gradionhq/margince/backend/internal/modules/audithistory/transport"
)

// AuditAdapter implements the Audit tag's slice of types.ServerInterface by
// delegating to the real HistoryHandler cmd/api/routes.go already wires for
// GET /records/{entity_type}/{id}/history. HistoryHandler reads entity_type
// and id itself via r.PathValue, so the typed entityType/idParam arguments
// oapi-codegen generates are intentionally unused (D10).
type AuditAdapter struct {
	H *audittransport.HistoryHandler
}

// GetRecordHistory delegates to the wired handler; see the struct doc comment above.
func (a *AuditAdapter) GetRecordHistory(w http.ResponseWriter, r *http.Request, entityType string, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}
