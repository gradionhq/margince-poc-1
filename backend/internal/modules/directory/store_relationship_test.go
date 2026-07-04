//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const wsRelStore = "00000000-0000-0000-0000-0000000000b1"

func seedRelWorkspace(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t08-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, wsRelStore, "t08-ws-"+ids.New()); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func setRelRLS(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, wsRelStore); err != nil {
		t.Fatalf("set RLS: %v", err)
	}
}

func strPtr(s string) *string { return &s }

func seedRelPersonOrg(t *testing.T, db *sql.DB) (personID, orgID string) {
	t.Helper()
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	ps := crmcore.NewPersonStore(db)
	p, err := ps.Create(ctx, crmcore.Person{
		WorkspaceID: wsRelStore,
		FullName:    "Rel Person " + ids.New(),
		Source:      p0.Source,
		CapturedBy:  p0.CapturedBy,
	}, nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}

	os := crmcore.NewOrgStore(db)
	o, err := os.Create(ctx, crmcore.Organization{
		WorkspaceID:    wsRelStore,
		DisplayName:    "Rel Org " + ids.New(),
		Classification: strPtr("prospect"),
		Source:         p0.Source,
		CapturedBy:     p0.CapturedBy,
	})
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return p.ID, o.ID
}

func TestRelationshipStore_CreateEmployment_ThenGet(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, orgID := seedRelPersonOrg(t, db)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})
	s := crmcore.NewRelationshipStore(db)
	created, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID:      wsRelStore,
		Kind:             "employment",
		PersonID:         &personID,
		OrganizationID:   &orgID,
		Role:             strPtr("vp_engineering"),
		IsCurrentPrimary: true,
		Source:           "test",
		CapturedBy:       "human:test",
	})
	if err != nil {
		t.Fatalf("create employment: %v", err)
	}
	if created.ID == "" || created.DealID != nil || created.Version != 1 {
		t.Fatalf("unexpected created row: %+v", created)
	}

	got, err := s.Get(ctx, created.ID, wsRelStore)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PersonID == nil || *got.PersonID != personID || got.OrganizationID == nil || *got.OrganizationID != orgID {
		t.Fatalf("employment fields not round-tripped: %+v", got)
	}
}

// TestRelationshipStore_CreateEmployment_NullProvenanceRejected covers P5 —
// no relationship row can be captured without source+captured_by.
func TestRelationshipStore_CreateEmployment_NullProvenanceRejected(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, orgID := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})

	s := crmcore.NewRelationshipStore(db)
	_, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID:    wsRelStore,
		Kind:           "employment",
		PersonID:       &personID,
		OrganizationID: &orgID,
	})
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Fatalf("err = %v, want ErrNullProvenance", err)
	}
}

// TestRelationshipStore_SecondCurrentPrimaryEmployment_Conflicts covers PO-AC-12
// (the chosen behavior: 409, not auto-demote — see Create's doc comment).
// Historical (non-current) rows are additive: the first row must still be
// readable afterward.
func TestRelationshipStore_SecondCurrentPrimaryEmployment_Conflicts(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, orgA := seedRelPersonOrg(t, db)
	_, orgB := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})
	s := crmcore.NewRelationshipStore(db)

	first, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID:      wsRelStore,
		Kind:             "employment",
		PersonID:         &personID,
		OrganizationID:   &orgA,
		IsCurrentPrimary: true,
		Source:           "test",
		CapturedBy:       "human:test",
	})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	_, err = s.Create(ctx, crmcore.Relationship{
		WorkspaceID:      wsRelStore,
		Kind:             "employment",
		PersonID:         &personID,
		OrganizationID:   &orgB,
		IsCurrentPrimary: true,
		Source:           "test",
		CapturedBy:       "human:test",
	})
	if !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict (uq_rel_current_primary_employer)", err)
	}

	stillThere, err := s.Get(ctx, first.ID, wsRelStore)
	if err != nil || stillThere.ID != first.ID {
		t.Fatalf("first employment row must remain untouched (additive history): got=%+v err=%v", stillThere, err)
	}
}

// TestRelationshipStore_CreateDealStakeholder_DuplicateRoleConflicts covers
// DEAL-AC-9 (uq_rel_deal_person_role).
func TestRelationshipStore_CreateDealStakeholder_DuplicateRoleConflicts(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, _ := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: wsRelStore, Name: "T08 " + ids.New()})
	if err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{WorkspaceID: wsRelStore, PipelineID: pl.ID, Name: "S", Position: 1, Semantic: "open", WinProbability: 50})
	if err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	ds := crmcore.NewDealStore(db)
	dealSeed := crmcore.NewDeal("T08 Deal "+ids.New(), pl.ID, st.ID, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	dealSeed.WorkspaceID = wsRelStore
	d, err := ds.Create(ctx, dealSeed, "")
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}

	s := crmcore.NewRelationshipStore(db)
	if _, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID: wsRelStore,
		Kind:        "deal_stakeholder",
		DealID:      &d.ID,
		PersonID:    &personID,
		Role:        strPtr("champion"),
		Source:      "test",
		CapturedBy:  "human:test",
	}); err != nil {
		t.Fatalf("create first stakeholder: %v", err)
	}

	_, err = s.Create(ctx, crmcore.Relationship{
		WorkspaceID: wsRelStore,
		Kind:        "deal_stakeholder",
		DealID:      &d.ID,
		PersonID:    &personID,
		Role:        strPtr("champion"),
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict (uq_rel_deal_person_role)", err)
	}
}

// TestRelationshipStore_Create_ShapeViolation_RejectedNotPanicked covers
// PO-AC-11's explicit requirement: a shape-violating insert (here, an
// employment row missing organization_id) is rejected by rel_employment_shape
// (a DB CHECK constraint violation, code 23514) — propagated as a wrapped
// error, never a panic.
func TestRelationshipStore_Create_ShapeViolation_RejectedNotPanicked(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, _ := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})
	s := crmcore.NewRelationshipStore(db)

	_, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID: wsRelStore,
		Kind:        "employment",
		PersonID:    &personID, // OrganizationID deliberately nil
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err == nil {
		t.Fatal("expected rel_employment_shape CHECK violation, got nil error")
	}
}

// TestRelationshipStore_Create_DealStakeholder_AuditAndEventOnOwningStream
// covers GATE-CORE-3/5 for the stakeholder write path (audit/event on the
// deal stream, not a "relationship" stream), complementing the employment
// case in TestRelationshipStore_Create_AuditAndEventOnOwningStream below.
func TestRelationshipStore_Create_DealStakeholder_AuditAndEventOnOwningStream(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, _ := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: wsRelStore, Name: "T08Audit " + ids.New()})
	if err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{WorkspaceID: wsRelStore, PipelineID: pl.ID, Name: "S", Position: 1, Semantic: "open", WinProbability: 50})
	if err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	ds := crmcore.NewDealStore(db)
	dealSeed := crmcore.NewDeal("T08Audit Deal "+ids.New(), pl.ID, st.ID, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	dealSeed.WorkspaceID = wsRelStore
	d, err := ds.Create(ctx, dealSeed, "")
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}

	s := crmcore.NewRelationshipStore(db)
	created, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID: wsRelStore,
		Kind:        "deal_stakeholder",
		DealID:      &d.ID,
		PersonID:    &personID,
		Role:        strPtr("influencer"),
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create stakeholder: %v", err)
	}

	// The deal's own DealStore.Create already wrote a create-audit row on
	// (entity_type='deal', entity_id=d.ID, action='create') — counting the
	// entity_id+action alone would double-count that seed row. Discriminate
	// this relationship's own audit row via after->>'id' (crmaudit.WriteTx
	// serializes the `after` argument, which Create passes as the relationship
	// itself), so this proves exactly one audit row for THIS write, not just
	// "at least one create happened on the deal stream".
	var auditCount int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE workspace_id=$1::uuid AND entity_type='deal' AND entity_id=$2::uuid AND action='create' AND after->>'id'=$3`,
		wsRelStore, d.ID, created.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit rows for this relationship create on deal stream = %d, want 1", auditCount)
	}

	var eventCount int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE workspace_id=$1::uuid AND entity_id=$2::uuid AND topic='deal.stakeholder_created'`,
		wsRelStore, d.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count event: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("event rows on deal stream = %d, want 1", eventCount)
	}
}

// TestRelationshipStore_Create_AuditAndEventOnOwningStream proves GATE-CORE-3/5:
// one audit row + one outbox event on the owning stream (person for employment,
// deal for stakeholder), not on a "relationship" stream.
func TestRelationshipStore_Create_AuditAndEventOnOwningStream(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, orgID := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})

	s := crmcore.NewRelationshipStore(db)
	created, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID:    wsRelStore,
		Kind:           "employment",
		PersonID:       &personID,
		OrganizationID: &orgID,
		Source:         "test",
		CapturedBy:     "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// PersonStore.Create (seedRelPersonOrg, in Task 2 Step 1) already wrote its
	// own create-audit row on (entity_type='person', entity_id=personID,
	// action='create') — counting entity_id+action alone would double-count
	// that seed row (2, not 1). Discriminate this relationship's own audit row
	// via after->>'id' (crmaudit.WriteTx serializes the `after` argument,
	// which Create passes as the relationship itself).
	var auditCount int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE workspace_id=$1::uuid AND entity_type='person' AND entity_id=$2::uuid AND action='create' AND after->>'id'=$3`,
		wsRelStore, personID, created.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit rows for this relationship create on person stream = %d, want 1", auditCount)
	}

	var eventCount int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE workspace_id=$1::uuid AND entity_id=$2::uuid AND topic='person.employment_created'`,
		wsRelStore, personID).Scan(&eventCount); err != nil {
		t.Fatalf("count event: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("event rows on person stream = %d, want 1", eventCount)
	}
}

func TestRelationshipStore_List_FiltersByKindAndOrg(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, orgID := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})
	s := crmcore.NewRelationshipStore(db)

	created, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID:    wsRelStore,
		Kind:           "employment",
		PersonID:       &personID,
		OrganizationID: &orgID,
		Source:         "test",
		CapturedBy:     "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	items, _, err := s.List(ctx, wsRelStore, "", 20, crmcore.RelationshipListFilter{
		Kind:           "employment",
		OrganizationID: orgID,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, r := range items {
		if r.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected created relationship %s in filtered list, got %d rows", created.ID, len(items))
	}
}

// TestRelationshipStore_Update_StaleIfMatchVersionSkews covers the standard
// If-Match/version convention (409 version_skew on a stale token).
func TestRelationshipStore_Update_StaleIfMatchVersionSkews(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, orgID := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})
	s := crmcore.NewRelationshipStore(db)

	created, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID:    wsRelStore,
		Kind:           "employment",
		PersonID:       &personID,
		OrganizationID: &orgID,
		Role:           strPtr("cto"),
		Source:         "test",
		CapturedBy:     "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := s.Update(ctx, created.ID, wsRelStore, map[string]any{"role": "vp_eng"}, created.Version)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if updated.Role == nil || *updated.Role != "vp_eng" {
		t.Fatalf("updated role = %+v, want vp_eng", updated.Role)
	}

	_, err = s.Update(ctx, created.ID, wsRelStore, map[string]any{"role": "ceo"}, created.Version)
	if !errors.Is(err, errs.ErrVersionSkew) {
		t.Fatalf("err = %v, want ErrVersionSkew on stale If-Match", err)
	}
}

func TestRelationshipStore_Archive_ExcludedFromDefaultList_VisibleWithIncludeArchived(t *testing.T) {
	db := sqlDB(t)
	seedRelWorkspace(t, db)
	setRelRLS(t, db)
	personID, orgID := seedRelPersonOrg(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelStore, UserID: "human:test"})
	s := crmcore.NewRelationshipStore(db)

	created, err := s.Create(ctx, crmcore.Relationship{
		WorkspaceID:    wsRelStore,
		Kind:           "employment",
		PersonID:       &personID,
		OrganizationID: &orgID,
		Source:         "test",
		CapturedBy:     "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	archived, err := s.Archive(ctx, created.ID, wsRelStore)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("expected archived_at set")
	}

	live, _, err := s.List(ctx, wsRelStore, "", 20, crmcore.RelationshipListFilter{PersonID: personID})
	if err != nil {
		t.Fatalf("list live: %v", err)
	}
	for _, r := range live {
		if r.ID == created.ID {
			t.Fatal("archived relationship must be excluded from the default list")
		}
	}

	withArchived, _, err := s.List(ctx, wsRelStore, "", 20, crmcore.RelationshipListFilter{
		PersonID:        personID,
		IncludeArchived: true,
	})
	if err != nil {
		t.Fatalf("list include_archived: %v", err)
	}
	found := false
	for _, r := range withArchived {
		if r.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected archived relationship in include_archived=true list")
	}
}
