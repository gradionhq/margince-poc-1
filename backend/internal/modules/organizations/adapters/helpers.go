// Package adapters — module-local constants for the organizations adapters layer.
// Generic, domain-free store helpers (provenance guard, JSON (un)marshalling,
// bounded-update field readers, pagination cursors) live in the Tier-0
// shared/kernel/sqlutil package.
package adapters

// entity-type constant used in audit log entries.
const entityTypeOrganization = "organization"

// Common field name constants used in event payloads and audit entries.
const (
	fieldOrganizationID = "organization_id"
	fieldMergedIntoID   = "merged_into_id"
)
