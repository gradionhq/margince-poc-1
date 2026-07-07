// Package ports defines the GDPR module's port interfaces.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/gdpr/domain"
)

// ConsentRepository is the GDPR consent read seam for per-call consent checks.
// workspaceID is an explicit param (not derived from ctx) so callers without a
// crmctx principal can still query — matching the seam doc (N3).
type ConsentRepository interface {
	FindForPurpose(ctx context.Context, workspaceID, personID, purpose string) (domain.ConsentState, error)
}
