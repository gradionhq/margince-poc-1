// Package transport holds the partners module's HTTP handlers.
//
// Generic HTTP/JSON helpers (workspace/path extraction, the problem+json/
// validation-error and pagination envelopes) live in the Tier-0
// shared/kernel/httpkit package.
package transport

import (
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const (
	codeBadRequest  = "bad_request"
	codeRequired    = "required"
	fieldCapturedBy = "captured_by"
	fieldSource     = "source"
)

// fieldError is the module-local spelling of the shared validation-error entry.
type fieldError = httpkit.FieldError

func provenanceOf(source, capturedBy string) prov.Provenance {
	return prov.Provenance{Source: source, CapturedBy: capturedBy}
}
