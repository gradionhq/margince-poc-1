//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/records/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/ports/session"
)

// ownerPerms returns RolePermissions granting read on entityType with the given row_scope.
func ownerPerms(entityType, rowScope string) session.RolePermissions {
	return session.RolePermissions{
		entityType: session.PermissionEntry{
			Actions: map[string]session.ActionRule{
				"read": {RowScope: rowScope},
			},
		},
	}
}

// seedUser inserts an app_user and returns its UUID id.
func seedUser(t *testing.T, db *sql.DB, wsID string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(context.Background(),
		`INSERT INTO app_user (workspace_id, email, display_name) VALUES ($1, $2, 'Visibility Test User') RETURNING id`,
		wsID, "vis-test-"+pgtest.Uniq()+"@example.com").Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

// seedPerson inserts a person with the given ownerID (may be empty for NULL owner_id) and returns the person id.
func seedPersonWithOwner(t *testing.T, db *sql.DB, ctx context.Context, wsID, ownerID string) string {
	t.Helper()
	var id string
	var err error
	if ownerID == "" {
		err = db.QueryRowContext(ctx,
			`INSERT INTO person (workspace_id, full_name, source, captured_by) VALUES ($1,'Vis Person','test','human:vis-test') RETURNING id`,
			wsID).Scan(&id)
	} else {
		err = db.QueryRowContext(ctx,
			`INSERT INTO person (workspace_id, full_name, owner_id, source, captured_by) VALUES ($1,'Vis Person',$2::uuid,'test','human:vis-test') RETURNING id`,
			wsID, ownerID).Scan(&id)
	}
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return id
}

// seedActivity inserts a bare activity row (no owner_id column) and returns its id.
func seedActivityRow(t *testing.T, db *sql.DB, ctx context.Context, wsID string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO activity (workspace_id, kind, occurred_at, source, captured_by, version, is_done)
		 VALUES ($1,'note',now(),'test','human:vis-test',1,false) RETURNING id`,
		wsID).Scan(&id); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	return id
}

func TestRecordVisible_RowScopeAll_VisibleRegardlessOfOwner(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:vis-test", TenantID: ws})

	personID := seedPersonWithOwner(t, db, ctx, ws, "")

	// Any principal can see a row when row_scope is "all".
	principal := crmctx.Principal{UserID: "some-other-user", TenantID: ws}
	perms := ownerPerms(domain.EntityTypePerson, "all")

	visible, err := adapters.RecordVisible(ctx, db, ws, domain.EntityTypePerson, personID, principal, perms)
	if err != nil {
		t.Fatalf("RecordVisible: %v", err)
	}
	if !visible {
		t.Fatal("expected visible=true for row_scope=all, got false")
	}
}

func TestRecordVisible_RowScopeOwn_MatchingOwner_Visible(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:vis-test", TenantID: ws})

	ownerID := seedUser(t, db, ws)
	personID := seedPersonWithOwner(t, db, ctx, ws, ownerID)

	principal := crmctx.Principal{UserID: ownerID, TenantID: ws}
	perms := ownerPerms(domain.EntityTypePerson, "own")

	visible, err := adapters.RecordVisible(ctx, db, ws, domain.EntityTypePerson, personID, principal, perms)
	if err != nil {
		t.Fatalf("RecordVisible: %v", err)
	}
	if !visible {
		t.Fatal("expected visible=true when principal is the owner, got false")
	}
}

func TestRecordVisible_RowScopeOwn_DifferentOwner_NoGrant_NotVisible(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:vis-test", TenantID: ws})

	ownerID := seedUser(t, db, ws)
	otherID := seedUser(t, db, ws)
	personID := seedPersonWithOwner(t, db, ctx, ws, ownerID)

	principal := crmctx.Principal{UserID: otherID, TenantID: ws}
	perms := ownerPerms(domain.EntityTypePerson, "own")

	visible, err := adapters.RecordVisible(ctx, db, ws, domain.EntityTypePerson, personID, principal, perms)
	if err != nil {
		t.Fatalf("RecordVisible: %v", err)
	}
	if visible {
		t.Fatal("expected visible=false for non-owner with no grant, got true")
	}
}

func TestRecordVisible_RowScopeOwn_DifferentOwner_WithLiveGrant_Visible(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:vis-test", TenantID: ws})

	ownerID := seedUser(t, db, ws)
	granteeID := seedUser(t, db, ws)
	personID := seedPersonWithOwner(t, db, ctx, ws, ownerID)

	// Grant granteeID access via record_grant (no expiry = permanent grant).
	if _, err := db.ExecContext(ctx,
		`INSERT INTO record_grant (workspace_id, record_type, record_id, subject_type, subject_id, access, granted_by)
		 VALUES ($1, 'person', $2::uuid, 'user', $3::uuid, 'read', $4::uuid)`,
		ws, personID, granteeID, ownerID); err != nil {
		t.Fatalf("seed record_grant: %v", err)
	}

	principal := crmctx.Principal{UserID: granteeID, TenantID: ws}
	perms := ownerPerms(domain.EntityTypePerson, "own")

	visible, err := adapters.RecordVisible(ctx, db, ws, domain.EntityTypePerson, personID, principal, perms)
	if err != nil {
		t.Fatalf("RecordVisible: %v", err)
	}
	if !visible {
		t.Fatal("expected visible=true for grantee with active record_grant, got false")
	}
}

func TestRecordVisible_RowScopeOwn_ExpiredGrant_NotVisible(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:vis-test", TenantID: ws})

	ownerID := seedUser(t, db, ws)
	granteeID := seedUser(t, db, ws)
	personID := seedPersonWithOwner(t, db, ctx, ws, ownerID)

	expiredAt := time.Now().Add(-24 * time.Hour)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO record_grant (workspace_id, record_type, record_id, subject_type, subject_id, access, granted_by, expires_at)
		 VALUES ($1, 'person', $2::uuid, 'user', $3::uuid, 'read', $4::uuid, $5)`,
		ws, personID, granteeID, ownerID, expiredAt); err != nil {
		t.Fatalf("seed expired record_grant: %v", err)
	}

	principal := crmctx.Principal{UserID: granteeID, TenantID: ws}
	perms := ownerPerms(domain.EntityTypePerson, "own")

	visible, err := adapters.RecordVisible(ctx, db, ws, domain.EntityTypePerson, personID, principal, perms)
	if err != nil {
		t.Fatalf("RecordVisible: %v", err)
	}
	if visible {
		t.Fatal("expected visible=false for expired record_grant, got true")
	}
}

func TestRecordVisible_Activity_AnyRowScope_Visible(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:vis-test", TenantID: ws})

	activityID := seedActivityRow(t, db, ctx, ws)

	// activity has no owner_id column; any row_scope that passed the
	// object-level gate is treated as visible (Constraint 6).
	principal := crmctx.Principal{UserID: "some-other-user-entirely", TenantID: ws}
	perms := ownerPerms(domain.EntityTypeActivity, "own")

	visible, err := adapters.RecordVisible(ctx, db, ws, domain.EntityTypeActivity, activityID, principal, perms)
	if err != nil {
		t.Fatalf("RecordVisible: %v", err)
	}
	if !visible {
		t.Fatal("expected visible=true for activity (no owner_id model), got false")
	}
}

func TestRecordVisible_AbsentObjectEntry_NotVisible(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:vis-test", TenantID: ws})

	ownerID := seedUser(t, db, ws)
	personID := seedPersonWithOwner(t, db, ctx, ws, ownerID)

	// perms does not contain an entry for "person" at all.
	perms := session.RolePermissions{}
	principal := crmctx.Principal{UserID: ownerID, TenantID: ws}

	visible, err := adapters.RecordVisible(ctx, db, ws, domain.EntityTypePerson, personID, principal, perms)
	if err != nil {
		t.Fatalf("RecordVisible: %v", err)
	}
	if visible {
		t.Fatal("expected visible=false when object entry is absent from perms, got true")
	}
}
