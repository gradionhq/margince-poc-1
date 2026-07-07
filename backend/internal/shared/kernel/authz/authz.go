// Package authz provides the dependency-free Authorizer seam and UUID validation
// pattern used across the CRM domain modules (WS-E-b, D5/D8).
package authz

import (
	"context"
	"regexp"
)

// ReUUID matches a well-formed UUID v4/v7 string.
var ReUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Authorizer reports whether the ctx principal may perform action on object.
// A nil return authorizes; any non-nil denies. Injected from cmd/api so no
// module queries the role/role_assignment tables directly.
type Authorizer func(ctx context.Context, object, action string) error
