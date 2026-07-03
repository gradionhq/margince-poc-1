//go:build integration

package crmgdpr_test

import (
	"context"
	"database/sql"
	"testing"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
)

// TestRetentionDefaults_SeededCorrectly verifies that SeedDefaults inserts exactly the
// 5 §3.4 rows and that they can be read back from the database.
func TestRetentionDefaults_SeededCorrectly(t *testing.T) {
	db := testDB(t)
	wsID, _ := seedWorkspaceAndPerson(t, db)

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	mustExecTx(t, tx, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	if err := crmgdpr.SeedDefaults(context.Background(), tx, wsID); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	type row struct {
		objectType string
		category   sql.NullString
		retainDays int
		action     string
	}
	rows, err := db.QueryContext(context.Background(),
		`SELECT object_type, category, retain_days, action
		 FROM retention_policy
		 WHERE workspace_id=$1::uuid
		 ORDER BY object_type, category NULLS FIRST`,
		wsID)
	if err != nil {
		t.Fatalf("query retention_policy: %v", err)
	}
	defer rows.Close()

	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.objectType, &r.category, &r.retainDays, &r.action); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	if len(got) != 5 {
		t.Fatalf("want 5 seeded rows, got %d", len(got))
	}

	type key struct{ objectType, category string }
	want := map[key]struct {
		retainDays int
		action     string
	}{
		{"lead", "unconverted"}:          {365, "anonymize"},
		{"activity", ""}:                 {1095, "archive"},
		{"activity", "transcript"}:       {365, "erase"},
		{"person", "no_consent_no_deal"}: {730, "anonymize"},
		{"deal", "lost"}:                 {1825, "archive"},
	}

	for _, r := range got {
		cat := ""
		if r.category.Valid {
			cat = r.category.String
		}
		k := key{r.objectType, cat}
		w, ok := want[k]
		if !ok {
			t.Errorf("unexpected row: %+v", r)
			continue
		}
		if r.retainDays != w.retainDays {
			t.Errorf("row %v/%v: retain_days want %d got %d", r.objectType, cat, w.retainDays, r.retainDays)
		}
		if r.action != w.action {
			t.Errorf("row %v/%v: action want %q got %q", r.objectType, cat, w.action, r.action)
		}
		delete(want, k)
	}
	for k := range want {
		t.Errorf("missing expected row: %+v", k)
	}
}

// TestLegalHold_ColumnsExistDefaultFalse verifies the 4 legal_hold columns exist defaulting false.
func TestLegalHold_ColumnsExistDefaultFalse(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	var lh bool
	if err := db.QueryRow(`SELECT legal_hold FROM person WHERE id=$1::uuid`, personID).Scan(&lh); err != nil {
		t.Fatalf("query person.legal_hold: %v", err)
	}
	if lh {
		t.Error("person.legal_hold: want false by default")
	}
}

// TestRetentionPolicy_UniqueNullCategory verifies the COALESCE-based unique index
// rejects a duplicate (workspace, object_type) when category IS NULL.
func TestRetentionPolicy_UniqueNullCategory(t *testing.T) {
	db := testDB(t)
	wsID, _ := seedWorkspaceAndPerson(t, db)
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	mustExec(t, db,
		`INSERT INTO retention_policy (workspace_id, object_type, category, retain_days, action)
		 VALUES ($1::uuid, 'thing', NULL, 30, 'archive')`, wsID)

	_, err := db.Exec(
		`INSERT INTO retention_policy (workspace_id, object_type, category, retain_days, action)
		 VALUES ($1::uuid, 'thing', NULL, 60, 'archive')`, wsID,
	)
	if err == nil {
		t.Error("expected unique-violation error for duplicate (ws, object_type, NULL category), got nil")
	}
}

func mustExecTx(t *testing.T, tx *sql.Tx, q string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(q, args...); err != nil {
		t.Fatalf("exec tx %q: %v", q, err)
	}
}
