package crmcore

import (
	"context"
	"regexp"
)

var reUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Authorizer reports whether the ctx principal may perform action on object.
// A nil return authorizes; any non-nil denies. It is injected from cmd/server
// so crm-core never queries the role / role_assignment tables directly.
type Authorizer func(ctx context.Context, object, action string) error
