// Package workflow is the Tier-0 seam for event handlers (Surface-B / the
// bounded automation catalog, ADR-0035). Handlers self-register at the edge.
package workflow

import "context"

// Handler reacts to a domain event.
type Handler interface {
	Match(event string) bool
	Plan(ctx context.Context, event string, payload any) error
}

var registry []Handler

// Register adds h to the global workflow registry. Called from init() in each workflow package.
func Register(h Handler) { registry = append(registry, h) }

// All returns every registered Handler.
func All() []Handler { return registry }
