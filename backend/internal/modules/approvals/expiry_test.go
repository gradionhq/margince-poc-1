package crmapprovals_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/riverqueue/river"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

// ─── query-aware fake SQL driver for expiry worker tests ─────────────────────
//
// The sweep query contains "expires_at"; expireOne's other queries do not.
// This lets one fake driver handle both the sweep SELECT and per-item ops.

var expiryFakeOnce sync.Once

func fakeExpiryDB(t *testing.T) *sql.DB {
	t.Helper()
	expiryFakeOnce.Do(func() { sql.Register("crmapprovalsexpiryfake", expiryFakeDriver{}) })
	db, err := sql.Open("crmapprovalsexpiryfake", "")
	if err != nil {
		t.Fatalf("open fake expiry db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type expiryFakeDriver struct{}

func (expiryFakeDriver) Open(string) (driver.Conn, error) { return &expiryFakeConn{}, nil }

type expiryFakeConn struct{}

func (*expiryFakeConn) Prepare(q string) (driver.Stmt, error) {
	return &expiryFakeStmt{query: q}, nil
}
func (*expiryFakeConn) Close() error              { return nil }
func (*expiryFakeConn) Begin() (driver.Tx, error) { return expiryFakeTxConn{}, nil }

type expiryFakeTxConn struct{}

func (expiryFakeTxConn) Commit() error   { return nil }
func (expiryFakeTxConn) Rollback() error { return nil }

type expiryFakeStmt struct{ query string }

func (*expiryFakeStmt) Close() error  { return nil }
func (*expiryFakeStmt) NumInput() int { return -1 }
func (s *expiryFakeStmt) Exec([]driver.Value) (driver.Result, error) {
	return expiryFakeResult{rows: 1}, nil
}

func (s *expiryFakeStmt) Query([]driver.Value) (driver.Rows, error) {
	if strings.Contains(s.query, "expires_at") {
		return &expirySweepRows{}, nil
	}
	return &expiryIDRows{}, nil
}

type expiryFakeResult struct{ rows int64 }

func (r expiryFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r expiryFakeResult) RowsAffected() (int64, error) { return r.rows, nil }

type expirySweepRows struct{ done bool }

func (*expirySweepRows) Columns() []string { return []string{"id", "workspace_id"} }
func (*expirySweepRows) Close() error      { return nil }
func (r *expirySweepRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = "test-item-expire-1"
	dest[1] = "ws-expire-1"
	return nil
}

type expiryIDRows struct{ done bool }

func (*expiryIDRows) Columns() []string { return []string{"id"} }
func (*expiryIDRows) Close() error      { return nil }
func (r *expiryIDRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = "00000000-0000-0000-0000-000000000001"
	return nil
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestExpiryWorker_EmitsDecidedExpired(t *testing.T) {
	db := fakeExpiryDB(t)
	spy := &spyEmitter{} // defined in decision_test.go (same package)
	worker := crmapprovals.NewExpiryWorker(db)
	worker.Emitter = spy

	if err := worker.Work(context.Background(), &river.Job[crmapprovals.ExpiryArgs]{}); err != nil {
		t.Fatalf("Work: %v", err)
	}

	if len(spy.topics) != 1 {
		t.Fatalf("want 1 expired event, got %d: %v", len(spy.topics), spy.topics)
	}
	if spy.topics[0] != crmapprovals.TopicApprovalDecided {
		t.Fatalf("topic = %q, want %q", spy.topics[0], crmapprovals.TopicApprovalDecided)
	}
	if !strings.Contains(spy.payloads[0], `"expired"`) {
		t.Fatalf("payload = %s, want expired decision", spy.payloads[0])
	}
}
