// Package datasource defines the Provider seam (ADR-0013): the data
// substrate every surface targets, implemented by crm-core (Datasource mode) or an
// incumbent adapter (overlay mode). Tier-0, dependency-free.
package datasource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// EntityType is a string-kind enum for the canonical CRM object types.
type EntityType string

// The canonical CRM object types.
const (
	EntityPerson       EntityType = "person"
	EntityOrganization EntityType = "organization"
	EntityDeal         EntityType = "deal"
	EntityActivity     EntityType = "activity"
	EntityLead         EntityType = "lead"
)

// EntityRef identifies an entity in the system of record.
type EntityRef struct {
	Type EntityType
	ID   string
}

// Record is an opaque entity payload (typed at the binding layer).
type Record = any

// SearchQuery parameterises a cross-entity search.
type SearchQuery struct {
	Type   EntityType
	Filter map[string]any
	Limit  int
}

// SearchResult is the response to a Search call.
type SearchResult struct {
	Records []Record
	Total   int
}

// ObjectDef describes a canonical CRM object type.
type ObjectDef struct {
	Type  EntityType
	Label string
}

// FieldDef describes a field on a canonical CRM object.
type FieldDef struct {
	Name     string
	Type     string
	Label    string
	Required bool
}

// ReportPlan identifies a named report and its parameters.
type ReportPlan struct {
	Name   string
	Params map[string]any
}

// ReportResult is an opaque report response (typed at the binding layer).
type ReportResult = any

// CreateInput carries a provenance-stamped create intent.
// Source and CapturedBy are mandatory (errs.ErrNullProvenance if absent).
type CreateInput struct {
	Type       EntityType
	Fields     any
	Source     string
	CapturedBy string
}

// UpdateInput carries a provenance-stamped patch intent.
// IfVersion is the optimistic-concurrency token (ADR-0036); nil means no version check.
type UpdateInput struct {
	Type       EntityType
	ID         string
	Patch      any
	Source     string
	CapturedBy string
	IfVersion  *string
}

// AdvanceDealInput drives a deal-status transition (won/lost/etc.).
// ChangedBy is reserved for provenance; it is accepted by callers but not yet
// consumed by the Datasource binding (the underlying status-write path does not carry
// a changed_by column at this spec scope).
type AdvanceDealInput struct {
	DealID    string
	ToStatus  string
	ChangedBy string
}

// LinkConversationInput carries a provenance-stamped request to join a CRM record
// to an external conversation. Source and CapturedBy are mandatory
// (errs.ErrNullProvenance if absent). Idempotent: re-linking the same pair is a no-op.
type LinkConversationInput struct {
	WorkspaceID        string
	EntityType         string
	EntityID           string
	ConversationSystem string
	ConversationID     string
	Source             string
	CapturedBy         string
}

// UnlinkConversationInput carries a request to hard-delete one conversation_link row
// by its id within the caller's workspace.
type UnlinkConversationInput struct {
	WorkspaceID string
	ID          string
}

// FreshnessInfo describes how current a provider's data is.
type FreshnessInfo struct {
	LastSyncedAt  time.Time
	Authoritative bool
}

// ErrVersionSkew is the optimistic-concurrency sentinel (ADR-0036).
// Aliased from errs.ErrVersionSkew so errors.Is works against either.
var ErrVersionSkew = errs.ErrVersionSkew

// ErrNotImplemented signals an overlay scaffold method not yet implemented.
var ErrNotImplemented = errors.New("datasource: not implemented")

// ErrUnsupported is the sentinel for UnsupportedError; callers use
// errors.Is(err, datasource.ErrUnsupported) to detect any unsupported verb.
var ErrUnsupported = errors.New("datasource: unsupported by system of record")

// UnsupportedError is the typed result an overlay adapter returns for a
// Datasource verb it cannot support (e.g. RunReport on HubSpot). It implements error
// and supports errors.Is against ErrUnsupported plus errors.As extraction
// so callers can read the Reason.
type UnsupportedError struct {
	Verb      string // Datasource method name, e.g. "RunReport"
	Incumbent string // incumbent name, e.g. "hubspot"
	Reason    string // human-readable reason, e.g. "no run_report analogue on HubSpot"
}

// Error implements the error interface.
func (e UnsupportedError) Error() string {
	return fmt.Sprintf("datasource: %s unsupported by %s: %s", e.Verb, e.Incumbent, e.Reason)
}

// Is reports whether e matches target. It matches the ErrUnsupported
// sentinel so errors.Is chains work through wrapping.
func (e UnsupportedError) Is(target error) bool {
	return target == ErrUnsupported
}

// CapabilitySupport declares whether an L1 tool is supported by a given
// incumbent and, when not, a human-readable reason.
type CapabilitySupport struct {
	Supported bool
	Reason    string // empty when Supported==true
}

// CapabilityManifest maps L1 tool names (the toolTiers keys in crmagents.go)
// to their per-incumbent support declaration. Published by the adapter via
// OverlayProvider.CapabilityManifest() so the seam — not a test constant — is
// the single source of truth for runtime vs. declared support.
type CapabilityManifest map[string]CapabilitySupport

// Provider is the Tier-0 seam (ADR-0013). The interface now has 11
// methods: the original 9 plus LinkConversation/UnlinkConversation (B-E14.2).
// Every implementer (DatasourceProvider, overlay, test fakes) must implement all 11.
type Provider interface {
	Read(ctx context.Context, ref EntityRef) (Record, error)
	Search(ctx context.Context, query SearchQuery) (SearchResult, error)
	ListObjects(ctx context.Context) ([]ObjectDef, error)
	ListFields(ctx context.Context, t EntityType) ([]FieldDef, error)
	RunReport(ctx context.Context, plan ReportPlan) (ReportResult, error)
	Create(ctx context.Context, in CreateInput) (EntityRef, error)
	Update(ctx context.Context, in UpdateInput) (EntityRef, error)
	AdvanceDeal(ctx context.Context, in AdvanceDealInput) (EntityRef, error)
	Freshness(ctx context.Context, ref EntityRef) (FreshnessInfo, error)
	// LinkConversation creates (or no-ops on) a conversation_link row — idempotent.
	// Returns an EntityRef whose ID is the conversation_link.id.
	LinkConversation(ctx context.Context, in LinkConversationInput) (EntityRef, error)
	// UnlinkConversation hard-deletes a conversation_link row by id.
	// Returns errs.ErrNotFound if the row doesn't exist in the workspace.
	UnlinkConversation(ctx context.Context, in UnlinkConversationInput) error
}
