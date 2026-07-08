//go:build integration

// Package adapters_test proves the archive-cascade contract (DM-CONV-15): when any
// of the five owning entity types is archived, all live attachment rows bound to it
// are archived in the same transaction. Tests are consolidated here (rather than one
// per owning module) because the cascade is a cross-module behavioral contract owned
// by the records module, not a detail of each individual module.
package adapters_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"

	activitiesadapters "github.com/gradionhq/margince/backend/internal/modules/activities/adapters"
	dealsadapters "github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	leadsadapters "github.com/gradionhq/margince/backend/internal/modules/leads/adapters"
	orgadapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	peopleadapters "github.com/gradionhq/margince/backend/internal/modules/people/adapters"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// seedAttachment inserts one live attachment row bound to entityType/entityID and
// returns its id. Requires app.workspace_id to be set on the connection (RLS).
func seedAttachment(t *testing.T, db *sql.DB, wsID, entityType, entityID string) string {
	t.Helper()
	storageKey := "attachments/" + wsID + "/" + entityID + "/cascade-test.pdf"
	var id string
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO attachment
		  (workspace_id, entity_type, entity_id, filename, content_type, byte_size, storage_key, source, captured_by)
		VALUES ($1, $2, $3, 'cascade-test.pdf', 'application/pdf', 1024, $4, 'test', 'human:cascade-test')
		RETURNING id`,
		wsID, entityType, entityID, storageKey).Scan(&id); err != nil {
		t.Fatalf("seed attachment (entity_type=%s): %v", entityType, err)
	}
	return id
}

// assertAttachmentArchivedAt asserts that the attachment row has (or does not have)
// archived_at set, providing a clear failure message in both cases.
func assertAttachmentArchivedAt(t *testing.T, db *sql.DB, wsID, attachmentID string, wantArchived bool) {
	t.Helper()
	// Re-set the workspace GUC on this connection so RLS allows the SELECT.
	pgtest.SetRLS(t, db, wsID)
	var archivedAt sql.NullTime
	if err := db.QueryRowContext(context.Background(),
		`SELECT archived_at FROM attachment WHERE id=$1::uuid`, attachmentID).Scan(&archivedAt); err != nil {
		t.Fatalf("select attachment %s: %v", attachmentID, err)
	}
	if wantArchived && !archivedAt.Valid {
		t.Fatalf("attachment %s: want archived_at SET after cascade, got NULL", attachmentID)
	}
	if !wantArchived && archivedAt.Valid {
		t.Fatalf("attachment %s: want archived_at NULL (not cascaded), got %v", attachmentID, archivedAt.Time)
	}
}

// ─── Person ─────────────────────────────────────────────────────────────────

func TestArchiveCascade_Person_ArchivesAttachment(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:cascade-test", TenantID: ws})

	var personID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO person (workspace_id, full_name, source, captured_by) VALUES ($1,'Cascade Person','test','human:cascade-test') RETURNING id`,
		ws).Scan(&personID); err != nil {
		t.Fatalf("seed person: %v", err)
	}

	// Seed a second person to bind an unrelated attachment.
	var otherPersonID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO person (workspace_id, full_name, source, captured_by) VALUES ($1,'Other Person','test','human:cascade-test') RETURNING id`,
		ws).Scan(&otherPersonID); err != nil {
		t.Fatalf("seed other person: %v", err)
	}

	att := seedAttachment(t, db, ws, "person", personID)
	otherAtt := seedAttachment(t, db, ws, "person", otherPersonID)

	store := peopleadapters.NewPersonStore(db)
	if _, err := store.Archive(ctx, personID, ws); err != nil {
		t.Fatalf("archive person: %v", err)
	}

	assertAttachmentArchivedAt(t, db, ws, att, true)
	assertAttachmentArchivedAt(t, db, ws, otherAtt, false)
}

// ─── Organization ────────────────────────────────────────────────────────────

func TestArchiveCascade_Organization_ArchivesAttachment(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:cascade-test", TenantID: ws})

	var orgID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO organization (workspace_id, name, source, captured_by, version) VALUES ($1,'Cascade Org','test','human:cascade-test',1) RETURNING id`,
		ws).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	var otherOrgID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO organization (workspace_id, name, source, captured_by, version) VALUES ($1,'Other Org','test','human:cascade-test',1) RETURNING id`,
		ws).Scan(&otherOrgID); err != nil {
		t.Fatalf("seed other org: %v", err)
	}

	att := seedAttachment(t, db, ws, "organization", orgID)
	otherAtt := seedAttachment(t, db, ws, "organization", otherOrgID)

	store := orgadapters.NewOrgStore(db)
	if _, err := store.Archive(ctx, orgID, ws); err != nil {
		t.Fatalf("archive org: %v", err)
	}

	assertAttachmentArchivedAt(t, db, ws, att, true)
	assertAttachmentArchivedAt(t, db, ws, otherAtt, false)
}

// ─── Deal ────────────────────────────────────────────────────────────────────

func seedDealFixture(t *testing.T, db *sql.DB, ws string, ctx context.Context) (pipelineID, stageID string) {
	t.Helper()
	if err := db.QueryRowContext(ctx,
		`INSERT INTO pipeline (workspace_id, name) VALUES ($1,'Cascade Pipeline') RETURNING id`, ws).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO stage (workspace_id, pipeline_id, name, position, semantic) VALUES ($1,$2,'Open',0,'open') RETURNING id`,
		ws, pipelineID).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	return pipelineID, stageID
}

func TestArchiveCascade_Deal_ArchivesAttachment(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:cascade-test", TenantID: ws})

	pipelineID, stageID := seedDealFixture(t, db, ws, ctx)

	var dealID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, source, captured_by, version)
		 VALUES ($1,'Cascade Deal',$2,$3,'test','human:cascade-test',1) RETURNING id`,
		ws, pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}

	var otherDealID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, source, captured_by, version)
		 VALUES ($1,'Other Deal',$2,$3,'test','human:cascade-test',1) RETURNING id`,
		ws, pipelineID, stageID).Scan(&otherDealID); err != nil {
		t.Fatalf("seed other deal: %v", err)
	}

	att := seedAttachment(t, db, ws, "deal", dealID)
	otherAtt := seedAttachment(t, db, ws, "deal", otherDealID)

	store := dealsadapters.NewDealStore(db)
	if _, err := store.Archive(ctx, dealID, ws); err != nil {
		t.Fatalf("archive deal: %v", err)
	}

	assertAttachmentArchivedAt(t, db, ws, att, true)
	assertAttachmentArchivedAt(t, db, ws, otherAtt, false)
}

// ─── Lead ────────────────────────────────────────────────────────────────────

func TestArchiveCascade_Lead_ArchivesAttachment(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:cascade-test", TenantID: ws})

	var leadID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO lead (workspace_id, full_name, source, captured_by, version)
		 VALUES ($1,'Cascade Lead','test','human:cascade-test',1) RETURNING id`,
		ws).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	var otherLeadID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO lead (workspace_id, full_name, source, captured_by, version)
		 VALUES ($1,'Other Lead','test','human:cascade-test',1) RETURNING id`,
		ws).Scan(&otherLeadID); err != nil {
		t.Fatalf("seed other lead: %v", err)
	}

	att := seedAttachment(t, db, ws, "lead", leadID)
	otherAtt := seedAttachment(t, db, ws, "lead", otherLeadID)

	store := leadsadapters.NewLeadStore(db)
	if _, err := store.Archive(ctx, leadID, ws); err != nil {
		t.Fatalf("archive lead: %v", err)
	}

	assertAttachmentArchivedAt(t, db, ws, att, true)
	assertAttachmentArchivedAt(t, db, ws, otherAtt, false)
}

// ─── Activity ────────────────────────────────────────────────────────────────

func TestArchiveCascade_Activity_ArchivesAttachment(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:cascade-test", TenantID: ws})

	var activityID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO activity (workspace_id, kind, occurred_at, source, captured_by, version, is_done)
		 VALUES ($1,'note',now(),'test','human:cascade-test',1,false) RETURNING id`,
		ws).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	var otherActivityID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO activity (workspace_id, kind, occurred_at, source, captured_by, version, is_done)
		 VALUES ($1,'note',now(),'test','human:cascade-test',1,false) RETURNING id`,
		ws).Scan(&otherActivityID); err != nil {
		t.Fatalf("seed other activity: %v", err)
	}

	att := seedAttachment(t, db, ws, "activity", activityID)
	otherAtt := seedAttachment(t, db, ws, "activity", otherActivityID)

	store := activitiesadapters.NewActivityStore(db)
	if _, err := store.Archive(ctx, activityID, ws); err != nil {
		t.Fatalf("archive activity: %v", err)
	}

	assertAttachmentArchivedAt(t, db, ws, att, true)
	assertAttachmentArchivedAt(t, db, ws, otherAtt, false)
}
