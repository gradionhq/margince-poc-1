package ports

import "context"

// Scanner is the injectable virus-scan seam (RD-PARAM-5). Status must be
// "clean" or "blocked" — never "scanning" (that's the row's own default).
// No real scanning product is wired anywhere in this codebase (out of
// scope); AttachmentStore.MarkScanResult is the only caller, invoked
// directly by tests/administration, never a public HTTP endpoint.
type Scanner interface {
	Scan(ctx context.Context, storageKey string) (status string, err error)
}
