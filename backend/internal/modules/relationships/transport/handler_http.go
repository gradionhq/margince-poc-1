// Package transport: relationships-module HTTP handlers.
//
// Generic HTTP/JSON helpers (workspace/path extraction, If-Match parsing, the
// problem+json/validation-error and pagination envelopes, WriteUpdateResult)
// live in the Tier-0 shared/kernel/httpkit package.
package transport

import "github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"

const (
	codeBadRequest  = "bad_request"
	codeValidation  = "validation_error"
	codeRequired    = "required"
	fieldCapturedBy = "captured_by"
	fieldSource     = "source"
)

// fieldError is the module-local spelling of the shared validation-error entry.
type fieldError = httpkit.FieldError
