// Package transport: activities-module HTTP handlers.
//
// Generic HTTP/JSON helpers (workspace/path extraction, If-Match parsing, the
// problem+json and pagination envelopes, WriteUpdateResult) live in the Tier-0
// shared/kernel/httpkit package.
package transport

import "github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"

const (
	codeBadRequest           = "bad_request"
	codeRequired             = "required"
	codeFieldNotValidForKind = "field_not_valid_for_kind"
	fieldKind                = "kind"
	fieldSource              = "source"
	fieldCapturedBy          = "captured_by"
)

// fieldError is a local alias, matching the organizations/deals transport
// packages' own convention for this Tier-0 type.
type fieldError = httpkit.FieldError

// validLinkEntityTypes mirrors activity_link's activity_link_shape CHECK
// (000003_core_objects.up.sql) — person/organization/deal only, not the
// 4-value ActivityLink wire schema's `lead` (a pre-existing, out-of-scope
// contract/DDL mismatch — CreateActivityRequest.links itself correctly omits
// lead, crm.yaml:5560).
var validLinkEntityTypes = map[string]bool{"person": true, "organization": true, "deal": true}
