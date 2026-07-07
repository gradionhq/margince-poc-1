//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/lib/pq" // registers the "postgres" database/sql driver

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// seedRecordGrantDealFixtures seeds an owner app_user, a subject app_user,
// and a pipeline+stage+deal (named dealName) owned by nobody in particular —
// the common fixture shape shared by TestRecordGrantStore_CreateIdempotentUpsert
// and TestRecordGrantStore_RevokeRemovesRow, which previously wrote this out
// twice inline (identical shape, only the deal name differed).
func seedRecordGrantDealFixtures(ctx context.Context, t *testing.T, db *sql.DB, ws, dealName string) (ownerID, subjectID string, deal crmcore.Deal) {
	t.Helper()

	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "owner@t.test", "Owner").Scan(&ownerID); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "subject@t.test", "Subject").Scan(&subjectID); err != nil {
		t.Fatalf("seed subject: %v", err)
	}

	pl, err := deals.NewPipelineStore(db).Create(ctx, deals.Pipeline{
		WorkspaceID: ws, Name: "test-pipeline", IsDefault: false, Position: 1,
	})
	if err != nil {
		t.Fatalf("create pipeline: %v", err)
	}
	st, err := deals.NewStageStore(db).Create(ctx, deals.Stage{
		WorkspaceID: ws, PipelineID: pl.ID, Name: "Open", Position: 1, Semantic: "open", WinProbability: 10,
	})
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	d := crmcore.NewDeal(dealName, pl.ID, st.ID, prov.Provenance{Source: "api", CapturedBy: "human:test"})
	d.WorkspaceID = ws
	deal, err = crmcore.NewDealStore(db).Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}
	return ownerID, subjectID, deal
}

// TestRecordGrantStore_CreateIdempotentUpsert proves createRecordGrant is
// idempotent on (record_type, record_id, subject_type, subject_id): a second
// call with a different access/expires_at upgrades the existing row (upsert),
// never creates a duplicate.
func TestRecordGrantStore_CreateIdempotentUpsert(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	ctx := context.Background()

	ownerID, subjectID, deal := seedRecordGrantDealFixtures(ctx, t, db, ws, "Grant Test Deal")

	store := crmcore.NewRecordGrantStore(db)
	g1, err := store.Create(ctx, crmcore.CreateRecordGrantInput{
		WorkspaceID: ws, GrantedBy: ownerID, RecordType: "deal", RecordID: deal.ID,
		SubjectType: "user", SubjectID: subjectID, Access: "read", GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	g2, err := store.Create(ctx, crmcore.CreateRecordGrantInput{
		WorkspaceID: ws, GrantedBy: ownerID, RecordType: "deal", RecordID: deal.ID,
		SubjectType: "user", SubjectID: subjectID, Access: "write", GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("second Create (upgrade): %v", err)
	}
	if g1.ID != g2.ID {
		t.Errorf("second Create with same natural key must upsert the same row, got a new id")
	}
	if g2.Access != "write" {
		t.Errorf("upsert should upgrade access to write, got %q", g2.Access)
	}

	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM record_grant WHERE record_id=$1::uuid`, deal.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("want exactly 1 record_grant row after upsert, got %d", count)
	}
}

// TestRecordGrantStore_RejectsScopeExceedingGrant proves a grant cannot exceed
// the granting principal's own access — a user with only "read" on a record
// cannot grant "write" to someone else.
func TestRecordGrantStore_RejectsScopeExceedingGrant(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	ctx := context.Background()

	store := crmcore.NewRecordGrantStore(db)
	// ErrGrantExceedsGrantorAccess is checked before any DB operation, so
	// record_id and subject_id do not need to reference real rows.
	_, err := store.Create(ctx, crmcore.CreateRecordGrantInput{
		WorkspaceID:      ws,
		GrantedBy:        ws, // any UUID; check fires before the DB INSERT
		RecordType:       "deal",
		RecordID:         ws, // any UUID
		SubjectType:      "user",
		SubjectID:        ws, // any UUID
		Access:           "write",
		GrantorOwnAccess: "read",
	})
	if !errors.Is(err, crmcore.ErrGrantExceedsGrantorAccess) {
		t.Errorf("want ErrGrantExceedsGrantorAccess, got %v", err)
	}
}

// TestRecordGrantStore_RevokeRemovesRow proves Revoke deletes the row and a
// subsequent widened-visibility check (Task 7's integration test) no longer
// sees the previously-granted record.
func TestRecordGrantStore_RevokeRemovesRow(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	ctx := context.Background()

	ownerID, subjectID, deal := seedRecordGrantDealFixtures(ctx, t, db, ws, "Revoke Test Deal")

	store := crmcore.NewRecordGrantStore(db)
	g, err := store.Create(ctx, crmcore.CreateRecordGrantInput{
		WorkspaceID: ws, GrantedBy: ownerID, RecordType: "deal", RecordID: deal.ID,
		SubjectType: "user", SubjectID: subjectID, Access: "read", GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Revoke(ctx, g.ID, ws); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, g.ID).Scan(&count); err != nil {
		t.Fatalf("count after Revoke: %v", err)
	}
	if count != 0 {
		t.Errorf("want 0 record_grant rows after Revoke, got %d", count)
	}
}
