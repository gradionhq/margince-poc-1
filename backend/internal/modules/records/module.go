// Package records is the records domain module: attachment metadata and
// presigned blob-store access (RD-DDL-1, ADR-0051). The module exposes an
// AttachmentStore adapter and an AttachmentHandler for HTTP routing.
package records

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/records/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/modules/records/transport"
	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
)

// Attachment is a type alias for domain.Attachment, re-exported so callers can
// refer to records domain types via this package.
type Attachment = domain.Attachment

// AttachmentStore is a type alias for adapters.AttachmentStore.
type AttachmentStore = adapters.AttachmentStore

// NewAttachmentStore returns an AttachmentStore backed by db.
func NewAttachmentStore(db *sql.DB) *AttachmentStore { return adapters.NewAttachmentStore(db) }

// Module is the records module's dependency-injection handle (D6 convenience
// constructor — the actual composition root in Task 8 constructs pieces
// individually, exactly like offers.New/activities.New today).
type Module struct {
	AttachmentStore   *adapters.AttachmentStore
	AttachmentHandler *transport.AttachmentHandler
}

// New constructs the records Module. actStore must implement adapters.ActivityCreator
// (i.e. *activities.ActivityStore satisfies this structurally). db is used for
// both the AttachmentStore and the handler's visibility gate.
func New(db *sql.DB, blob blobstore.Store, actStore adapters.ActivityCreator) *Module {
	store := adapters.NewAttachmentStore(db)
	audit := adapters.NewDownloadAuditWriter(actStore)
	return &Module{
		AttachmentStore:   store,
		AttachmentHandler: transport.NewAttachmentHandler(store, blob, audit, db),
	}
}
