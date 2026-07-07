// Package activities is the activities domain module: timeline events linked to
// people, organizations, and deals (data-model §7). The module exposes an
// ActivityStore adapter and an ActivityHandler for HTTP routing.
package activities

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/activities/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/activities/app"
	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/modules/activities/transport"
)

// Domain type aliases
type Activity = domain.Activity
type ActivityRef = domain.ActivityRef

// Adapter type aliases
type ActivityStore = adapters.ActivityStore

// NewActivityStore returns an ActivityStore backed by db.
func NewActivityStore(db *sql.DB) *ActivityStore { return adapters.NewActivityStore(db) }

// ToActivityRef narrows a full Activity to the lightweight ActivityRef shape.
func ToActivityRef(a Activity) ActivityRef { return domain.ToActivityRef(a) }

// Module is the activities module's dependency-injection handle.
type Module struct {
	Store   *adapters.ActivityStore
	Service *app.Service
	Handler *transport.ActivityHandler
}

// New constructs the activities Module wiring adapters, app service, and HTTP
// handler together.
func New(db *sql.DB) *Module {
	store := adapters.NewActivityStore(db)
	svc := app.NewService(store)
	handler := transport.NewActivityHandler(store)
	return &Module{
		Store:   store,
		Service: svc,
		Handler: handler,
	}
}
