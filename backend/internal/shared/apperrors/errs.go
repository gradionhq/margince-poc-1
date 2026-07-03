// Package errs is the Tier-0 fixed error-sentinel registry (interfaces.md §0).
// Every domain maps to these; HTTP/MCP edges translate them to status codes.
package errs

import "errors"

// The Tier-0 error sentinels; the trailing comment on each is its HTTP status.
var (
	ErrNotFound         = errors.New("not found")         // -> 404
	ErrConflict         = errors.New("conflict")          // -> 409 (dedupe)
	ErrScopeExceeded    = errors.New("scope exceeded")    // -> 403 (agent <= human)
	ErrRequiresApproval = errors.New("requires approval") // -> 409 (yellow gate)
	ErrVersionSkew      = errors.New("version skew")      // -> 409 (optimistic concurrency)
	ErrBudgetExceeded   = errors.New("budget exceeded")   // -> 429
	ErrForbidden        = errors.New("forbidden")         // -> 403
	ErrNullProvenance   = errors.New("null provenance")   // -> 422 (capture rejected: missing source/captured_by)
	ErrSuppressed       = errors.New("suppressed")        // -> 451 (GDPR erasure suppression)
)

// HTTPStatus maps a domain error to its HTTP status code — the single sentinel→status
// choke point for HTTP edges (interfaces.md §0). Unknown errors map to 500. The mapping
// matches the per-sentinel comments above; keep them in sync. nil maps to 200.
func HTTPStatus(err error) int {
	switch {
	case err == nil:
		return 200
	case errors.Is(err, ErrNotFound):
		return 404
	case errors.Is(err, ErrConflict),
		errors.Is(err, ErrRequiresApproval),
		errors.Is(err, ErrVersionSkew):
		return 409
	case errors.Is(err, ErrScopeExceeded),
		errors.Is(err, ErrForbidden):
		return 403
	case errors.Is(err, ErrBudgetExceeded):
		return 429
	case errors.Is(err, ErrNullProvenance):
		return 422
	case errors.Is(err, ErrSuppressed):
		return 451
	default:
		return 500
	}
}
