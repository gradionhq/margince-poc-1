// Package transport holds the records module's HTTP handlers for /quotas.
package transport

import "github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"

const codeBadRequest = "bad_request"

// fieldError is the module-local spelling of the shared validation-error entry.
type fieldError = httpkit.FieldError
