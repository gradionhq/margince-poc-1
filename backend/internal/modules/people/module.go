// Package people is the people module (WS-E-a).
// Transport handlers live in people/transport/; domain types in people/domain/;
// SQL adapters in people/adapters/; port interfaces in people/ports/;
// application services in people/app/.
package people

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/people/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/people/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// Re-exported type aliases so callers can import from the module root without
// reaching into sub-packages for the most commonly used types.

// Person is the people module's contact-record domain type.
type Person = domain.Person

// PersonEmailInput is one entry of createPerson's emails[] request field.
type PersonEmailInput = domain.PersonEmailInput

// PersonStore is the SQL adapter for person rows.
type PersonStore = adapters.PersonStore

// NewPersonStore returns a PersonStore backed by db.
func NewPersonStore(db *sql.DB) *PersonStore { return adapters.NewPersonStore(db) }

// RecordGrant mirrors the contract's RecordGrant schema (crm.yaml).
type RecordGrant = adapters.RecordGrant

// CreateRecordGrantInput is the store-level create/upsert request.
type CreateRecordGrantInput = adapters.CreateRecordGrantInput

// RecordGrantListFilter holds optional predicates for RecordGrantStore.List.
type RecordGrantListFilter = adapters.RecordGrantListFilter

// RecordGrantStore is the SQL adapter for record_grant rows (GH-209 WS-B).
type RecordGrantStore = adapters.RecordGrantStore

// NewRecordGrantStore returns a RecordGrantStore backed by db.
func NewRecordGrantStore(db *sql.DB) *RecordGrantStore { return adapters.NewRecordGrantStore(db) }

// ErrGrantExceedsGrantorAccess is returned by RecordGrantStore.Create when the
// granting principal attempts to grant an access level exceeding their own.
var ErrGrantExceedsGrantorAccess = adapters.ErrGrantExceedsGrantorAccess

// NewPerson returns a Person with a fresh ID, version 1, and copied provenance.
func NewPerson(fullName string, p prov.Provenance) Person { return domain.NewPerson(fullName, p) }

// Module is the people module's dependency-injection handle.
// Future tickets will add datasource.Provider, repository seams, and
// application services here as the module is progressively built out.
type Module struct{}

// New returns a new people Module.
func New() *Module { return &Module{} }
