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
)

// Attachment is a type alias for domain.Attachment, re-exported so callers can
// refer to records domain types via this package.
type Attachment = domain.Attachment

// AttachmentStore is a type alias for adapters.AttachmentStore.
type AttachmentStore = adapters.AttachmentStore

// NewAttachmentStore returns an AttachmentStore backed by db.
func NewAttachmentStore(db *sql.DB) *AttachmentStore { return adapters.NewAttachmentStore(db) }

// Note: this package intentionally has no Module/New() convenience constructor. An earlier
// version did (wiring transport.NewAttachmentHandler), but that made records depend on
// records/transport while records/transport (RD-T06's QuotaHandler) depends back on records
// for its Quota alias surface — an import cycle. records.New() was never actually called
// (routes.go always constructs recordstransport handlers directly, e.g.
// recordstransport.NewAttachmentHandler(records.NewAttachmentStore(db), ...) and
// recordstransport.NewQuotaHandler(records.NewQuotaStore(db))), so removing the dead
// convenience wrapper — rather than reversing recordstransport's established
// depends-on-records seam (mirrors offerstransport -> offers) — is the non-breaking fix.

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

type (
	// Quota is a per-owner or per-team revenue target for one period (RD-DDL-2).
	Quota = adapters.Quota
	// QuotaListFilter narrows a List call to a specific owner or team.
	QuotaListFilter = adapters.QuotaListFilter
	// QuotaStore executes parameterized SQL against the quota table.
	QuotaStore = adapters.QuotaStore
	// Attainment is the computed attainment for one quota (RD-FORM-2).
	Attainment = adapters.Attainment
	// AttainmentDeal is one closed-won deal contributing to a quota's attainment.
	AttainmentDeal = adapters.AttainmentDeal
)

var (
	// ErrOwnerXorTeamRequired fires when owner_id XOR team_id is not satisfied (RD-DDL-2).
	ErrOwnerXorTeamRequired = adapters.ErrOwnerXorTeamRequired
	// ErrAttainmentTargetZero is returned when a quota's target_minor is zero.
	ErrAttainmentTargetZero = adapters.ErrAttainmentTargetZero
)

// NewQuotaStore returns a QuotaStore backed by db.
func NewQuotaStore(db *sql.DB) *QuotaStore { return adapters.NewQuotaStore(db) }
