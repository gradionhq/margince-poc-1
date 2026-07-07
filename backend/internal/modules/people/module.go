// Package people is the people module scaffold (WS-E-a).
// Transport handlers live in people/transport/; this module.go provides the
// top-level Module type as a DI handle for future application-layer wiring.
package people

// Module is the people module's dependency-injection handle.
// Future tickets will add datasource.Provider, repository seams, and
// application services here as the module is progressively built out.
type Module struct{}

// New returns a new people Module.
func New() *Module { return &Module{} }
