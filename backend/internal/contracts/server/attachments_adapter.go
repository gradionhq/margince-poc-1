package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	recordstransport "github.com/gradionhq/margince/backend/internal/modules/records/transport"
)

// AttachmentsAdapter implements the Attachments tag's slice of
// types.ServerInterface by delegating to the real AttachmentHandler
// cmd/api/routes.go wires for /attachments (RD-WIRE-1).
type AttachmentsAdapter struct {
	H *recordstransport.AttachmentHandler
}

// ListAttachments delegates to the wired handler; see the struct doc comment above.
func (a *AttachmentsAdapter) ListAttachments(w http.ResponseWriter, r *http.Request, params types.ListAttachmentsParams) {
	a.H.ServeHTTP(w, r)
}

// CreateAttachment delegates to the wired handler; see the struct doc comment above.
func (a *AttachmentsAdapter) CreateAttachment(w http.ResponseWriter, r *http.Request, params types.CreateAttachmentParams) {
	a.H.ServeHTTP(w, r)
}

// GetAttachment delegates to the wired handler; see the struct doc comment above.
func (a *AttachmentsAdapter) GetAttachment(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// ArchiveAttachment delegates to the wired handler; see the struct doc comment above.
func (a *AttachmentsAdapter) ArchiveAttachment(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}
