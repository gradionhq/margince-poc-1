//go:build integration

package records_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/records"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// seedUser inserts an app_user for use as a quota owner and returns its id.
func seedUser(t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO app_user (workspace_id, email, display_name)
		 VALUES ($1,$2,'Quota Owner') RETURNING id`,
		ws, "quota-owner-"+pgtest.Uniq()+"@example.com",
	).Scan(&id); err != nil {
		t.Fatalf("seed app_user: %v", err)
	}
	return id
}

// seedTeam inserts a team and returns its id.
func seedTeam(t *testing.T, db *sql.DB, ws, name string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO team (workspace_id, name) VALUES ($1,$2) RETURNING id`,
		ws, name+"-"+pgtest.Uniq(),
	).Scan(&id); err != nil {
		t.Fatalf("seed team: %v", err)
	}
	return id
}

func auditCount(t *testing.T, db *sql.DB, entityID, action string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='quota' AND entity_id=$1::uuid AND action=$2`,
		entityID, action,
	).Scan(&n); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return n
}

func TestQuotaStore_CreateGetList(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	userID := seedUser(t, db, ws)
	teamID := seedTeam(t, db, ws, "sales")
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	t.Run("create owner-only succeeds", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 10000000,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create owner-only: %v", err)
		}
		if q.ID == "" {
			t.Error("id is empty")
		}
		if q.OwnerID == nil || *q.OwnerID != userID {
			t.Errorf("owner_id = %v, want %s", q.OwnerID, userID)
		}
		if q.TeamID != nil {
			t.Errorf("team_id = %v, want nil", q.TeamID)
		}
		if q.TargetMinor != 10000000 {
			t.Errorf("target_minor = %d, want 10000000", q.TargetMinor)
		}
		if q.Currency != "EUR" {
			t.Errorf("currency = %s, want EUR", q.Currency)
		}
		if q.ArchivedAt != nil {
			t.Errorf("archived_at = %v, want nil", q.ArchivedAt)
		}
		if auditCount(t, db, q.ID, "create") != 1 {
			t.Error("expected exactly 1 create audit_log row")
		}
	})

	t.Run("create team-only succeeds", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			TeamID:      &teamID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 5000000,
			Currency:    "USD",
		})
		if err != nil {
			t.Fatalf("Create team-only: %v", err)
		}
		if q.TeamID == nil || *q.TeamID != teamID {
			t.Errorf("team_id = %v, want %s", q.TeamID, teamID)
		}
		if q.OwnerID != nil {
			t.Errorf("owner_id = %v, want nil", q.OwnerID)
		}
		if auditCount(t, db, q.ID, "create") != 1 {
			t.Error("expected exactly 1 create audit_log row")
		}
	})

	t.Run("create both-set returns ErrOwnerXorTeamRequired", func(t *testing.T) {
		_, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			TeamID:      &teamID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 1,
			Currency:    "EUR",
		})
		if !errors.Is(err, records.ErrOwnerXorTeamRequired) {
			t.Errorf("Create both-set: err = %v, want ErrOwnerXorTeamRequired", err)
		}
	})

	t.Run("create neither-set returns ErrOwnerXorTeamRequired", func(t *testing.T) {
		_, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 1,
			Currency:    "EUR",
		})
		if !errors.Is(err, records.ErrOwnerXorTeamRequired) {
			t.Errorf("Create neither-set: err = %v, want ErrOwnerXorTeamRequired", err)
		}
	})

	t.Run("Get round-trips every field", func(t *testing.T) {
		created, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 20000000,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create for Get test: %v", err)
		}
		got, err := store.Get(ctx, created.ID, ws)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("id mismatch: got %s, want %s", got.ID, created.ID)
		}
		if got.WorkspaceID != ws {
			t.Errorf("workspace_id mismatch")
		}
		if got.OwnerID == nil || *got.OwnerID != userID {
			t.Errorf("owner_id mismatch: got %v", got.OwnerID)
		}
		if got.TargetMinor != 20000000 {
			t.Errorf("target_minor mismatch")
		}
		if got.Version != 1 {
			t.Errorf("version = %d, want 1", got.Version)
		}
	})

	t.Run("Get nonexistent returns ErrNotFound", func(t *testing.T) {
		_, err := store.Get(ctx, "00000000-0000-0000-0000-000000000000", ws)
		if !errors.Is(err, errs.ErrNotFound) {
			t.Errorf("Get nonexistent: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("Get archived returns ErrNotFound", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 1,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create for archived Get: %v", err)
		}
		if _, err := store.Archive(ctx, q.ID, ws); err != nil {
			t.Fatalf("Archive: %v", err)
		}
		_, err = store.Get(ctx, q.ID, ws)
		if !errors.Is(err, errs.ErrNotFound) {
			t.Errorf("Get archived: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("List paginates and filters", func(t *testing.T) {
		// Fresh workspace for isolation.
		ws2 := pgtest.NewWorkspaceSQL(t, db)
		pgtest.SetRLS(t, db, ws2)
		ctx2 := pgtest.AppCtx(ws2)
		u2 := seedUser(t, db, ws2)
		tm2 := seedTeam(t, db, ws2, "listteam")

		for i := 0; i < 3; i++ {
			if _, err := store.Create(ctx2, records.Quota{
				WorkspaceID: ws2,
				OwnerID:     &u2,
				PeriodStart: periodStart,
				PeriodEnd:   periodEnd,
				TargetMinor: int64(i + 1),
				Currency:    "EUR",
			}); err != nil {
				t.Fatalf("seed quota %d: %v", i, err)
			}
		}
		// One team-scoped quota.
		if _, err := store.Create(ctx2, records.Quota{
			WorkspaceID: ws2,
			TeamID:      &tm2,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 999,
			Currency:    "EUR",
		}); err != nil {
			t.Fatalf("seed team quota: %v", err)
		}

		// Default list (all 4).
		all, next, err := store.List(ctx2, ws2, "", 10, false, records.QuotaListFilter{})
		if err != nil {
			t.Fatalf("List all: %v", err)
		}
		if len(all) != 4 {
			t.Errorf("List len = %d, want 4", len(all))
		}
		if next != "" {
			t.Errorf("next = %q, want empty (all fit in page)", next)
		}

		// Filter by owner.
		byOwner, _, err := store.List(ctx2, ws2, "", 10, false, records.QuotaListFilter{OwnerID: u2})
		if err != nil {
			t.Fatalf("List by owner: %v", err)
		}
		if len(byOwner) != 3 {
			t.Errorf("List by owner len = %d, want 3", len(byOwner))
		}

		// Filter by team.
		byTeam, _, err := store.List(ctx2, ws2, "", 10, false, records.QuotaListFilter{TeamID: tm2})
		if err != nil {
			t.Fatalf("List by team: %v", err)
		}
		if len(byTeam) != 1 {
			t.Errorf("List by team len = %d, want 1", len(byTeam))
		}

		// Pagination: limit=2 on 4 items → next cursor set.
		page1, cursor2, err := store.List(ctx2, ws2, "", 2, false, records.QuotaListFilter{})
		if err != nil {
			t.Fatalf("List page1: %v", err)
		}
		if len(page1) != 2 {
			t.Errorf("page1 len = %d, want 2", len(page1))
		}
		if cursor2 == "" {
			t.Error("cursor2 is empty, expected a next-page cursor")
		}
		page2, cursor3, err := store.List(ctx2, ws2, cursor2, 2, false, records.QuotaListFilter{})
		if err != nil {
			t.Fatalf("List page2: %v", err)
		}
		if len(page2) != 2 {
			t.Errorf("page2 len = %d, want 2", len(page2))
		}
		if cursor3 != "" {
			t.Errorf("cursor3 = %q, want empty (last page)", cursor3)
		}
	})
}

func TestQuotaStore_Update(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	userID := seedUser(t, db, ws)
	teamID := seedTeam(t, db, ws, "updateteam")
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	t.Run("valid If-Match succeeds", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 10000000,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		updated, err := store.Update(ctx, q.ID, ws, map[string]any{
			"target_minor": int64(20000000),
		}, q.Version)
		if err != nil {
			t.Fatalf("Update valid If-Match: %v", err)
		}
		if updated.TargetMinor != 20000000 {
			t.Errorf("target_minor = %d, want 20000000", updated.TargetMinor)
		}
		if updated.Version <= q.Version {
			t.Errorf("version did not increment: old=%d new=%d", q.Version, updated.Version)
		}
		if auditCount(t, db, q.ID, "update") != 1 {
			t.Error("expected exactly 1 update audit_log row")
		}
	})

	t.Run("stale If-Match returns ErrVersionSkew", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 1,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		_, err = store.Update(ctx, q.ID, ws, map[string]any{"target_minor": int64(2)}, q.Version+1)
		if !errors.Is(err, errs.ErrVersionSkew) {
			t.Errorf("stale If-Match: err = %v, want ErrVersionSkew", err)
		}
	})

	t.Run("patching owner-scoped into both-set returns ErrOwnerXorTeamRequired and leaves row unchanged", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 1,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		// Patch team_id without clearing owner_id → both set → rejected.
		_, err = store.Update(ctx, q.ID, ws, map[string]any{"team_id": teamID}, 0)
		if !errors.Is(err, records.ErrOwnerXorTeamRequired) {
			t.Errorf("both-set patch: err = %v, want ErrOwnerXorTeamRequired", err)
		}
		// Row must be unchanged.
		after, err := store.Get(context.Background(), q.ID, ws)
		if err != nil {
			t.Fatalf("Get after rejected update: %v", err)
		}
		if after.OwnerID == nil || *after.OwnerID != userID {
			t.Error("owner_id was modified by rejected update")
		}
		if after.TeamID != nil {
			t.Error("team_id was set by rejected update")
		}
	})

	t.Run("switching scope (owner→team) by clearing owner_id + setting team_id", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 1,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		updated, err := store.Update(ctx, q.ID, ws, map[string]any{
			"owner_id": nil,
			"team_id":  teamID,
		}, 0)
		if err != nil {
			t.Fatalf("scope switch: %v", err)
		}
		if updated.OwnerID != nil {
			t.Errorf("owner_id after switch = %v, want nil", updated.OwnerID)
		}
		if updated.TeamID == nil || *updated.TeamID != teamID {
			t.Errorf("team_id after switch = %v, want %s", updated.TeamID, teamID)
		}
	})

	t.Run("update nonexistent returns ErrNotFound", func(t *testing.T) {
		_, err := store.Update(ctx, "00000000-0000-0000-0000-000000000000", ws,
			map[string]any{"target_minor": int64(1)}, 0)
		if !errors.Is(err, errs.ErrNotFound) {
			t.Errorf("update nonexistent: err = %v, want ErrNotFound", err)
		}
	})
}

func TestQuotaStore_Archive(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	userID := seedUser(t, db, ws)
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	t.Run("Archive sets archived_at and returns entity", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 1,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		archived, err := store.Archive(ctx, q.ID, ws)
		if err != nil {
			t.Fatalf("Archive: %v", err)
		}
		if archived.ArchivedAt == nil {
			t.Error("archived_at is nil after Archive")
		}
		if auditCount(t, db, q.ID, "archive") != 1 {
			t.Error("expected exactly 1 archive audit_log row")
		}
	})

	t.Run("Archive is idempotent", func(t *testing.T) {
		q, err := store.Create(ctx, records.Quota{
			WorkspaceID: ws,
			OwnerID:     &userID,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 1,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := store.Archive(ctx, q.ID, ws); err != nil {
			t.Fatalf("first Archive: %v", err)
		}
		// Second archive should not error.
		if _, err := store.Archive(ctx, q.ID, ws); err != nil {
			t.Fatalf("second Archive (idempotent): %v", err)
		}
		// Only one audit row (first archive only).
		if auditCount(t, db, q.ID, "archive") != 1 {
			t.Errorf("archive audit count = %d, want 1 (idempotent)", auditCount(t, db, q.ID, "archive"))
		}
	})

	t.Run("archived row excluded from default List, visible with include_archived=true", func(t *testing.T) {
		ws2 := pgtest.NewWorkspaceSQL(t, db)
		pgtest.SetRLS(t, db, ws2)
		ctx2 := pgtest.AppCtx(ws2)
		u2 := seedUser(t, db, ws2)

		live, err := store.Create(ctx2, records.Quota{
			WorkspaceID: ws2,
			OwnerID:     &u2,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 100,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create live: %v", err)
		}
		toArchive, err := store.Create(ctx2, records.Quota{
			WorkspaceID: ws2,
			OwnerID:     &u2,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			TargetMinor: 200,
			Currency:    "EUR",
		})
		if err != nil {
			t.Fatalf("Create to archive: %v", err)
		}
		if _, err := store.Archive(ctx2, toArchive.ID, ws2); err != nil {
			t.Fatalf("Archive: %v", err)
		}

		// Default list excludes archived.
		def, _, err := store.List(ctx2, ws2, "", 10, false, records.QuotaListFilter{})
		if err != nil {
			t.Fatalf("List default: %v", err)
		}
		if len(def) != 1 || def[0].ID != live.ID {
			t.Errorf("default List: got %d items (want 1 live), ids=%v", len(def), ids(def))
		}

		// include_archived includes both.
		all, _, err := store.List(ctx2, ws2, "", 10, true, records.QuotaListFilter{})
		if err != nil {
			t.Fatalf("List include_archived: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("include_archived List: got %d items (want 2)", len(all))
		}
	})
}

func ids(qs []records.Quota) []string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = q.ID
	}
	return out
}
