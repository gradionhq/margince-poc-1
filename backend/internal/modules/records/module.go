// Package records is the records domain module. It covers attachment
// metadata and presigned blob-store access (RD-DDL-1, ADR-0051), exposing an
// AttachmentStore adapter and an AttachmentHandler for HTTP routing, as well
// as the records-depth read side: the organization hierarchy roll-up
// (RD-FORM-1) that aggregates three RD-PARAM-2 measures over an
// organization's parent tree via RollupStore.
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

// RollupStore is a type alias for adapters.RollupStore. It computes
// GET /organizations/{id}/hierarchy-rollup (RD-FORM-1) over the
// organization.parent_org_id self-FK.
type RollupStore = adapters.RollupStore

// RollupResult is a type alias for adapters.RollupResult, the computed
// hierarchy roll-up for one root organization.
type RollupResult = adapters.RollupResult

// RestrictedNode is a type alias for adapters.RestrictedNode, a descendant
// the viewer's row_scope cannot read, disclosed in the roll-up (RD-AC-1)
// rather than silently summed.
type RestrictedNode = adapters.RestrictedNode

// NewRollupStore returns a RollupStore backed by db.
func NewRollupStore(db *sql.DB) *RollupStore { return adapters.NewRollupStore(db) }
