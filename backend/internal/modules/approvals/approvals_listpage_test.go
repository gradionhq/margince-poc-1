package crmapprovals_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

// ─── recording fake SQL driver: captures the query + args and returns canned
// rows so the unit test can assert ListPage's query SHAPE and cursor arithmetic
// without a live Postgres (data-semantics are proven by the integration test). ──

type lpRecorder struct {
	lastQuery string
	lastArgs  []driver.Value
	rows      [][]driver.Value
}

var (
	lpOnce sync.Once
	lpRec  = &lpRecorder{}
)

func listPageFakeDB(t *testing.T) *sql.DB {
	t.Helper()
	lpOnce.Do(func() { sql.Register("crmapprovalslistpagefake", lpDriver{}) })
	db, err := sql.Open("crmapprovalslistpagefake", "")
	if err != nil {
		t.Fatalf("open fake list-page db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type lpDriver struct{}

func (lpDriver) Open(string) (driver.Conn, error) { return lpConn{}, nil }

type lpConn struct{}

func (lpConn) Prepare(string) (driver.Stmt, error) { return nil, io.EOF }
func (lpConn) Close() error                        { return nil }
func (lpConn) Begin() (driver.Tx, error)           { return nil, io.EOF }

// QueryContext records the final SQL + args, then returns the configured rows.
func (lpConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	lpRec.lastQuery = q
	lpRec.lastArgs = make([]driver.Value, len(args))
	for i, a := range args {
		lpRec.lastArgs[i] = a.Value
	}
	return &lpRows{rows: lpRec.rows}, nil
}

type lpRows struct {
	rows [][]driver.Value
	i    int
}

func (*lpRows) Columns() []string {
	return []string{
		"id", "workspace_id", "action_type", "payload", "status",
		"requested_by", "decided_by", "decided_at", "expires_at", "created_at",
	}
}
func (*lpRows) Close() error { return nil }
func (r *lpRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// lpRow builds one canned approval_item row (decided_by/at + expires_at NULL).
func lpRow(id string) []driver.Value {
	return []driver.Value{
		id, "ws1", "update_record", []byte(`{"id":"r1"}`),
		"pending", "agent1", nil, nil, nil, time.Now(),
	}
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestListPage_EmptyReturnsNonNilSlice(t *testing.T) {
	db := listPageFakeDB(t)
	lpRec.rows = nil // no rows
	repo := crmapprovals.NewPageRepository()

	items, next, err := repo.ListPage(context.Background(), db, "ws1", crmapprovals.StatusPending, "", "", 50)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if items == nil {
		t.Fatal("ListPage must return a non-nil empty slice, got nil")
	}
	if len(items) != 0 {
		t.Fatalf("want 0 items, got %d", len(items))
	}
	if next != "" {
		t.Fatalf("exhausted page must have empty next cursor, got %q", next)
	}
}

func TestListPage_ComputesNextCursorFromLimitPlusOne(t *testing.T) {
	db := listPageFakeDB(t)
	// Fetch limit+1 rows: ListPage must return exactly `limit` items and set the
	// next cursor to the last id of the returned page (id-keyed).
	lpRec.rows = [][]driver.Value{lpRow("a"), lpRow("b"), lpRow("c")}
	repo := crmapprovals.NewPageRepository()

	items, next, err := repo.ListPage(context.Background(), db, "ws1", crmapprovals.StatusPending, "", "", 2)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items (limit), got %d", len(items))
	}
	if next != "b" {
		t.Fatalf("next cursor must be last id of the page (b), got %q", next)
	}
	// LIMIT must be limit+1 to detect the extra row.
	if !strings.Contains(lpRec.lastQuery, "LIMIT") {
		t.Fatalf("query missing LIMIT: %s", lpRec.lastQuery)
	}
	if got := lpRec.lastArgs[len(lpRec.lastArgs)-1]; got != int64(3) {
		t.Fatalf("LIMIT arg must be limit+1 (3), got %v", got)
	}
}

func TestListPage_LastPageHasEmptyCursor(t *testing.T) {
	db := listPageFakeDB(t)
	lpRec.rows = [][]driver.Value{lpRow("a")} // fewer than limit+1
	repo := crmapprovals.NewPageRepository()

	items, next, err := repo.ListPage(context.Background(), db, "ws1", crmapprovals.StatusPending, "", "", 2)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if next != "" {
		t.Fatalf("non-full page must have empty next cursor, got %q", next)
	}
}

func TestListPage_CursorAndKindNarrowTheQuery(t *testing.T) {
	db := listPageFakeDB(t)
	lpRec.rows = nil
	repo := crmapprovals.NewPageRepository()

	if _, _, err := repo.ListPage(context.Background(), db, "ws1",
		crmapprovals.StatusPending, "send_email", "cursor-id", 10); err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	q := lpRec.lastQuery
	if !strings.Contains(q, "workspace_id") || !strings.Contains(q, "status") {
		t.Fatalf("query must always scope by workspace_id + status: %s", q)
	}
	if !strings.Contains(q, "id >") {
		t.Fatalf("non-empty afterID must add an id-keyed cursor clause: %s", q)
	}
	if !strings.Contains(q, "action_type") {
		t.Fatalf("non-empty kind must add an action_type filter: %s", q)
	}
	if !strings.Contains(q, "ORDER BY id") {
		t.Fatalf("page must be id-ordered for a stable cursor: %s", q)
	}
	// args: workspaceID, status, afterID, kind, limit+1
	if len(lpRec.lastArgs) != 5 {
		t.Fatalf("want 5 args (ws,status,after,kind,limit+1), got %d: %v", len(lpRec.lastArgs), lpRec.lastArgs)
	}
}
