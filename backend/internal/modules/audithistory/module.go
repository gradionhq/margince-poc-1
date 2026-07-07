// Package audithistory is the audit history module.
// It provides read-only access to the audit_log table for any entity type,
// with configurable field masking (AC2) and human-readable summary composition.
//
// Sub-packages:
//   - domain: pure types and formatting logic (AuditHistoryEntry, field masking, summary)
//   - ports: persistence seams (HistoryReader interface)
//   - adapters: database-backed implementations (AuditHistoryReader)
//   - app: thin application-service layer (Service)
//   - transport: HTTP handlers (HistoryHandler)
package audithistory

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/audithistory/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/audithistory/app"
	"github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
	"github.com/gradionhq/margince/backend/internal/modules/audithistory/ports"
	"github.com/gradionhq/margince/backend/internal/modules/audithistory/transport"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/authz"
)

// ---------------------------------------------------------------------------
// Domain type aliases
// ---------------------------------------------------------------------------

// AuditHistoryEntry is one rendered history line for a record mutation.
type AuditHistoryEntry = domain.AuditHistoryEntry

// EntityFieldMask is the set of field names to hide in before/after for an entity type.
type EntityFieldMask = domain.EntityFieldMask

// DefaultFieldMasks is the minimal per-entity-type mask (seam for AC2 tests).
var DefaultFieldMasks = domain.DefaultFieldMasks

// ---------------------------------------------------------------------------
// Port type aliases
// ---------------------------------------------------------------------------

// HistoryReader is the read seam for audit history entries.
type HistoryReader = ports.HistoryReader

// ---------------------------------------------------------------------------
// Adapter type aliases
// ---------------------------------------------------------------------------

// AuditHistoryReader is the database-backed HistoryReader implementation.
type AuditHistoryReader = adapters.AuditHistoryReader

// ---------------------------------------------------------------------------
// App type aliases
// ---------------------------------------------------------------------------

// Service is the audithistory application service.
type Service = app.Service

// ---------------------------------------------------------------------------
// Module is the audithistory module's dependency-injection handle.
// ---------------------------------------------------------------------------

// Module holds the wired-up components of the audithistory module.
type Module struct {
	Handler *transport.HistoryHandler
}

// New wires and returns a ready-to-use audithistory Module.
func New(db *sql.DB, az authz.Authorizer) *Module {
	reader := adapters.NewAuditHistoryReader(db)
	svc := app.New(reader)
	handler := transport.NewHistoryHandler(svc, az)
	return &Module{Handler: handler}
}

// NewAuditHistoryReader returns a database-backed AuditHistoryReader.
func NewAuditHistoryReader(db *sql.DB) *AuditHistoryReader {
	return adapters.NewAuditHistoryReader(db)
}

// NewService returns an audithistory Service backed by the given HistoryReader.
func NewService(reader HistoryReader) *Service {
	return app.New(reader)
}
