package crmcore

// Domain string constants shared across crm-core. Centralizing the repeated
// literals keeps the entity-type / actor-type / activity-kind vocabulary in one
// place so a typo cannot silently fork the value across files.

// Entity-type values used as audit/event entity_type and signal subject_type.
const (
	entityTypeDeal         = "deal"
	entityTypeActivity     = "activity"
	entityTypePipeline     = "pipeline"
	entityTypeStage        = "stage"
	entityTypePerson       = "person"
	entityTypeOrganization = "organization"
	entityTypeLead         = "lead"
	entityTypePartner      = "partner"
)

// Deal / lead lifecycle status values.
const (
	statusOpen = "open"
	statusWon  = "won"
	statusLost = "lost"
	statusNew  = "new"
)

// Common JSON / column field names shared across payloads and projections.
const (
	fieldName           = "name"
	fieldKind           = "kind"
	fieldStatus         = "status"
	fieldSource         = "source"
	fieldPersonID       = "person_id"
	fieldOrganizationID = "organization_id"
	fieldNote           = "note"
	fieldSourceID       = "source_id"
)

// Actor-type and audit-action values written to audit_log.
const (
	actorTypeAgent     = "agent"
	actorTypeConnector = "connector"
	actorTypeHuman     = "human"
	actionCapture      = "capture"
	actionApproved     = "approved"
)

// Audit action and resolution-state values.
const (
	actionPromoted   = "promoted"
	resolutionState  = "resolved"
	kpiKeyTiming     = "timing"
	fieldIsDone      = "is_done"
	fieldType        = "type"
	fieldUnit        = "unit"
	fieldParams      = "params"
	fieldWorkspaceID = "workspace_id"
	typeString       = "string"
	statusSent       = "sent"
	statusPending    = "pending"
	statusExpired    = "expired"
	objRelationship  = "relationship"
)

// Mirror/projection column names shared by the overlay mirror and Datasource binding.
const (
	colFullName    = "full_name"
	colDisplayName = "display_name"
	colDealID      = "deal_id"
)

// problem+json response field names and machine-readable codes used by handlers.
const (
	fieldData      = "data"
	fieldCode      = "code"
	codeForbidden  = "forbidden"
	codeBadRequest = "bad_request"
)

// Bulk operation async-dispatch tuning (B-E11.21b). A batch whose resolved
// candidate count exceeds bulkAsyncThreshold dispatches as a River job
// instead of mutating inline in the HTTP request; BulkChunkSize is the
// worker's per-tx batch size.
const (
	bulkAsyncThreshold = 1000
	BulkChunkSize      = 500
)

// Bulk operation lifecycle status literals.
const (
	BulkStatusRunning = "running"
	BulkStatusDone    = "done"
	BulkStatusFailed  = "failed"
)
