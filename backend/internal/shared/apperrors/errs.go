// Package errs is the Tier-0 fixed error-sentinel registry (interfaces.md §0).
// Every domain maps to these; HTTP/MCP edges translate them to status codes.
package errs

import "errors"

// The Tier-0 error sentinels; the trailing comment on each is its HTTP status.
var (
	ErrNotFound                  = errors.New("not found")                            // -> 404
	ErrConflict                  = errors.New("conflict")                             // -> 409 (dedupe)
	ErrScopeExceeded             = errors.New("scope exceeded")                       // -> 403 (agent <= human)
	ErrRequiresApproval          = errors.New("requires approval")                    // -> 409 (yellow gate)
	ErrVersionSkew               = errors.New("version skew")                         // -> 409 (optimistic concurrency)
	ErrBudgetExceeded            = errors.New("budget exceeded")                      // -> 429
	ErrForbidden                 = errors.New("forbidden")                            // -> 403
	ErrNullProvenance            = errors.New("null provenance")                      // -> 422 (capture rejected: missing source/captured_by)
	ErrStageNotInPipeline        = errors.New("stage not in pipeline")                // -> 422 (validation)
	ErrTerminalProbabilityPinned = errors.New("terminal stage probability is pinned") // -> 422 (validation: won=100, lost=0)
	ErrWinProbabilityOutOfRange  = errors.New("win_probability out of range")         // -> 422 (validation: 0-100)
	ErrFXRateUnavailable         = errors.New("fx rate unavailable")                  // -> 422 (no stored rate for as-of lookup)
	ErrSuppressed                = errors.New("suppressed")                           // -> 451 (GDPR erasure suppression)
	ErrNotArchived               = errors.New("not archived")                         // -> 422 (restore refused: record is already live)
	ErrMergedRecord              = errors.New("merged record")                        // -> 422 (restore refused: merged_into_id set, PO-AC-18)
	ErrApprovalTokenInvalid      = errors.New("approval token invalid")               // -> 403 (expired, replayed, or mis-bound token — API-ERR-11)
	ErrApprovalRequired          = errors.New("approval required")                    // -> 403 (agent on a 🟡 transition, no token — API-ERR-10)
	ErrStatusMismatch            = errors.New("status mismatch")                      // -> 422 (explicit status != target stage's derived semantic)
	ErrLostReasonRequired        = errors.New("lost reason required")                 // -> 422 (advancing to a lost stage needs lost_reason)
	ErrFieldNotValidForKind      = errors.New("field not valid for kind")             // -> 422 (task-only field set on a non-task activity kind)
	ErrOrganizationCycle         = errors.New("organization cycle")                   // -> 422 (parent_org_id would create a cycle; DB trigger prevent_org_cycle/org_no_cycle)
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
		errors.Is(err, ErrForbidden),
		errors.Is(err, ErrApprovalTokenInvalid),
		errors.Is(err, ErrApprovalRequired):
		return 403
	case errors.Is(err, ErrBudgetExceeded):
		return 429
	case errors.Is(err, ErrNullProvenance),
		errors.Is(err, ErrStageNotInPipeline),
		errors.Is(err, ErrTerminalProbabilityPinned),
		errors.Is(err, ErrWinProbabilityOutOfRange),
		errors.Is(err, ErrFXRateUnavailable),
		errors.Is(err, ErrNotArchived),
		errors.Is(err, ErrMergedRecord),
		errors.Is(err, ErrStatusMismatch),
		errors.Is(err, ErrLostReasonRequired),
		errors.Is(err, ErrFieldNotValidForKind),
		errors.Is(err, ErrOrganizationCycle):
		return 422
	case errors.Is(err, ErrSuppressed):
		return 451
	default:
		return 500
	}
}
