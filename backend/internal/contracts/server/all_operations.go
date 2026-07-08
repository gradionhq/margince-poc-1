// Package server hosts the oapi-codegen ServerInterface conformance layer
// (AC-D3/D10): one adapter per crm.yaml tag, aggregated into AllOperations.
// This is interface-generation scope only — cmd/api/routes.go's live route
// registration remains authoritative; nothing here is wired to serve traffic.
package server

// AllOperations aggregates every crm.yaml tag's adapter so the struct's
// promoted method set covers all of types.ServerInterface. Every embedded
// adapter implements a disjoint set of operationIds (the two crm.yaml
// operations carrying two tags — upsertPartner/getPartner and
// draftEmail/sendEmail — are each declared on exactly one of their two
// adapters, see PartnersAdapter/AIAdapter), so no method name collides
// across two embedded fields and nothing here is ambiguous.
type AllOperations struct {
	PeopleAdapter
	OrganizationsAdapter
	DealsAdapter
	PipelinesAdapter
	PartnersAdapter
	RelationshipsAdapter
	ActivitiesAdapter
	AuditAdapter
	IdentityAdapter

	AIAdapter
	AccessAdapter
	ApprovalsAdapter
	AttachmentsAdapter
	AutomationsAdapter
	ComplianceAdapter
	ConversationsAdapter
	CustomFieldsAdapter
	DealRoomsAdapter
	DraftingAssetsAdapter
	ExportsAdapter
	ImportsAdapter
	IntegrationsAdapter
	InvoicesAdapter
	LeadsAdapter
	ListsAdapter
	OfferTemplatesAdapter
	OffersAdapter
	ProductsAdapter
	QuotasAdapter
	ReportsAdapter
	SearchAdapter
	TagsAdapter
}

// NewAllOperations aggregates the already-constructed per-tag adapters (built
// from the same handler instances cmd/api/routes.go's manual registration
// uses) into one AllOperations. Tags with no wired handler in this pruned
// skeleton tree need no adapter argument — their zero-value adapter already
// stubs every operation 501. CustomFields (CF-T03) is no longer one of
// those — its adapter now carries the real *customfields.Handler.
func NewAllOperations(
	people PeopleAdapter,
	organizations OrganizationsAdapter,
	deals DealsAdapter,
	pipelines PipelinesAdapter,
	partners PartnersAdapter,
	relationships RelationshipsAdapter,
	activities ActivitiesAdapter,
	audit AuditAdapter,
	identity IdentityAdapter,
	customFields CustomFieldsAdapter,
) *AllOperations {
	return &AllOperations{
		PeopleAdapter:        people,
		OrganizationsAdapter: organizations,
		DealsAdapter:         deals,
		PipelinesAdapter:     pipelines,
		PartnersAdapter:      partners,
		RelationshipsAdapter: relationships,
		ActivitiesAdapter:    activities,
		AuditAdapter:         audit,
		IdentityAdapter:      identity,
		CustomFieldsAdapter:  customFields,
	}
}
