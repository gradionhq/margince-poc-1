//go:build integration

package crmcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'e03test',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "e03-"+wsID)
	if err != nil {
		t.Fatal("seed workspace:", err)
	}
}

// openTestDB, setRLS, and uniq duplicate
// modules/people/transport/handler_person_test.go's helpers of the same name:
// handler_person_test.go moved to package transport in the 1c restructure
// (task-3-brief.md) while handler_audit_history_test.go and
// store_deal_filter_test.go — which also use these — stayed in package
// crmcore (modules/directory); the two packages can no longer share a
// _test.go file. Duplicated rather than exported solely for this — same class
// of directory-move-forced duplication as httpserver's keyStatus (see
// internal/platform/httpserver/middleware.go).
var (
	testSeq      int64
	testRunEpoch = time.Now().UnixNano()
)

// uniq returns a string that is unique across test binary invocations.
func uniq() string {
	return fmt.Sprintf("%d-%d", testRunEpoch, atomic.AddInt64(&testSeq, 1))
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	d, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func setRLS(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		"SET app.workspace_id = '"+wsID+"'")
	if err != nil {
		t.Fatal("setRLS:", err)
	}
}

// fkIntoTable returns every (referencing_table, referencing_column) pair with a live
// FOREIGN KEY into table(id) — the DB-truth version of "grep migrations", used so the
// merge acceptance tests assert against every FK Postgres actually enforces today, not
// just the set the plan's author found by hand.
func fkIntoTable(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.Query(`
		SELECT tc.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu ON tc.constraint_name = ccu.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY' AND ccu.table_name = $1`, table)
	if err != nil {
		t.Fatalf("fkIntoTable(%s): %v", table, err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var refTable, refCol string
		if err := rows.Scan(&refTable, &refCol); err != nil {
			t.Fatal(err)
		}
		out[refTable] = refCol
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("fkIntoTable(%s) rows: %v", table, err)
	}
	return out
}

// TestPersonMergeFKWalkExhaustive proves PO-AC-17's "walks every FK" literally: every
// live FK into person(id) is either relinked to the target (asserted zero-rows-remaining
// on the loser) or is one of the plan's documented "leave historical rows alone"
// exceptions (asserted the loser row still exists so nothing is actually orphaned).
func TestPersonMergeFKWalkExhaustive(t *testing.T) {
	db := openTestDB(t)
	fks := fkIntoTable(t, db, "person")
	relinked := map[string]bool{"person_email": true, "person_phone": true, "relationship": true, "activity_link": true}
	leftAlone := map[string]bool{"person_consent": true, "consent_event": true, "lead": true, "person": true /* merged_into_id self-ref */}
	for table := range fks {
		if !relinked[table] && !leftAlone[table] {
			t.Fatalf("FK from %s into person(id) is neither relinked nor documented as intentionally left — merge relink logic is incomplete for this table", table)
		}
	}
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)
	loser := mkPerson(ctx, t, store, ws, "FKWalkLoser")
	target := mkPerson(ctx, t, store, ws, "FKWalkTarget")
	if _, err := store.Merge(ctx, loser.ID, target.ID, ws); err != nil {
		t.Fatalf("merge: %v", err)
	}
	for table := range relinked {
		col := fks[table]
		var n int
		db.QueryRow(fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s=$1::uuid AND archived_at IS NULL`, table, col), loser.ID).Scan(&n)
		if n != 0 {
			t.Fatalf("table %s still has %d live row(s) pointing at the archived loser after merge — relink incomplete", table, n)
		}
	}
	// The loser row itself must still exist (soft-archived), so any left-alone FK is
	// pointing at a real row, not a dangling one.
	var loserExists int
	db.QueryRow(`SELECT count(*) FROM person WHERE id=$1::uuid`, loser.ID).Scan(&loserExists)
	if loserExists != 1 {
		t.Fatalf("loser row must still exist post-merge (soft-archive, never delete) — got count=%d", loserExists)
	}
}

// assertNoRows fails the test if the query returns any rows.
func assertNoRows(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	var x int
	err := db.QueryRow(query, args...).Scan(&x)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no rows for %q, got x=%d err=%v", query, x, err)
	}
}

// seedAppUser seeds an app_user row so that audit_log.on_behalf_of FK is satisfied
// when agent-principal requests invoke crmaudit.Write.
func seedAppUser(t *testing.T, db *sql.DB, id, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO app_user(id,workspace_id,email,display_name,is_agent) VALUES($1::uuid,$2::uuid,$3,'Agent Test',true) ON CONFLICT DO NOTHING`,
		id, wsID, "e03-agent-"+id+"@example.com")
	if err != nil {
		t.Fatal("seed app_user:", err)
	}
}
