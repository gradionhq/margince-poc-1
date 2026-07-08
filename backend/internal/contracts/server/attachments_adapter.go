package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// AttachmentsAdapter implements the Attachments tag's slice of
// types.ServerInterface. RD-T02 mints the contract only — no handler exists
// yet (the blob-seam presigned-URL minting, virus scanning, and RBAC
// evaluation are explicitly out of scope) — every method returns 501 via the
// same shape oapi-codegen's own generated types.Unimplemented stub uses
// (AC-D3/D10, same pattern as TagsAdapter/ListsAdapter for an unwired tag).
type AttachmentsAdapter struct{}

// ListAttachments is unimplemented; see AttachmentsAdapter's doc comment.
func (AttachmentsAdapter) ListAttachments(w http.ResponseWriter, r *http.Request, params types.ListAttachmentsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateAttachment is unimplemented; see AttachmentsAdapter's doc comment.
func (AttachmentsAdapter) CreateAttachment(w http.ResponseWriter, r *http.Request, params types.CreateAttachmentParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetAttachment is unimplemented; see AttachmentsAdapter's doc comment.
func (AttachmentsAdapter) GetAttachment(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ArchiveAttachment is unimplemented; see AttachmentsAdapter's doc comment.
func (AttachmentsAdapter) ArchiveAttachment(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
