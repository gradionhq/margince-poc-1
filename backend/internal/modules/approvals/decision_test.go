package crmapprovals_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// ─── fake SQL driver for non-integration decision tests ───────────────────────

var decisionFakeOnce sync.Once

func fakeDecisionDB(t *testing.T) *sql.DB {
	t.Helper()
	decisionFakeOnce.Do(func() { sql.Register("crmapprovalsdecisionfake", decisionFakeDriver{}) })
	db, err := sql.Open("crmapprovalsdecisionfake", "")
	if err != nil {
		t.Fatalf("open fake decision db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func fakeDecisionTx(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin fake decision tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

type decisionFakeDriver struct{}

func (decisionFakeDriver) Open(string) (driver.Conn, error) { return &decisionFakeConn{}, nil }

type decisionFakeConn struct{}

func (*decisionFakeConn) Prepare(string) (driver.Stmt, error) { return &decisionFakeStmt{}, nil }
func (*decisionFakeConn) Close() error                        { return nil }
func (*decisionFakeConn) Begin() (driver.Tx, error)           { return decisionFakeTxConn{}, nil }

type decisionFakeTxConn struct{}

func (decisionFakeTxConn) Commit() error   { return nil }
func (decisionFakeTxConn) Rollback() error { return nil }

type decisionFakeStmt struct{}

func (*decisionFakeStmt) Close() error  { return nil }
func (*decisionFakeStmt) NumInput() int { return -1 }
func (*decisionFakeStmt) Exec([]driver.Value) (driver.Result, error) {
	return decisionFakeResult{}, nil
}

func (*decisionFakeStmt) Query([]driver.Value) (driver.Rows, error) { return &decisionFakeRows{}, nil }

type decisionFakeResult struct{}

func (decisionFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (decisionFakeResult) RowsAffected() (int64, error) { return 1, nil }

type decisionFakeRows struct{ done bool }

func (*decisionFakeRows) Columns() []string { return []string{"id"} }
func (*decisionFakeRows) Close() error      { return nil }
func (r *decisionFakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = "00000000-0000-0000-0000-000000000001"
	return nil
}

// ─── spy emitter (shared with expiry_test.go — same package) ─────────────────

type spyEmitter struct {
	topics   []string
	payloads []string
}

func (s *spyEmitter) Emit(_ context.Context, _ crmapprovals.DBExec, topic, _, _ string, p json.RawMessage) error {
	s.topics = append(s.topics, topic)
	s.payloads = append(s.payloads, string(p))
	return nil
}

// ─── local fakeRepo satisfying crmapprovals.Repository ───────────────────────

type fakeApprovalRepo struct {
	mu     sync.Mutex
	items  map[string]crmapprovals.Item
	nextID int
}

func newFakeApprovalRepo() *fakeApprovalRepo {
	return &fakeApprovalRepo{items: make(map[string]crmapprovals.Item)}
}

func (r *fakeApprovalRepo) Create(_ context.Context, _ crmapprovals.DBExec, it crmapprovals.Item) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	id := fmt.Sprintf("item-%d", r.nextID)
	it.ID = id
	r.items[id] = it
	return id, nil
}

func (r *fakeApprovalRepo) Get(_ context.Context, _ crmapprovals.DBExec, id string) (crmapprovals.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.items[id]
	if !ok {
		return crmapprovals.Item{}, fmt.Errorf("fakeApprovalRepo: not found %s", id)
	}
	return it, nil
}

func (r *fakeApprovalRepo) ListByStatus(_ context.Context, _ crmapprovals.DBExec, _ string, _ crmapprovals.Status) ([]crmapprovals.Item, error) {
	return nil, nil
}

func (r *fakeApprovalRepo) Transition(_ context.Context, _ crmapprovals.DBExec, id string, to crmapprovals.Status, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.items[id]
	if !ok {
		return fmt.Errorf("fakeApprovalRepo: not found %s", id)
	}
	// Mirror pgRepository.Transition: the UPDATE only affects a row WHERE status=
	// 'pending'. A second concurrent transition finds the row already non-pending and
	// fails (RowsAffected==0) — this is the row-lock that gates exactly-once exec.
	if it.Status != crmapprovals.StatusPending {
		return fmt.Errorf("fakeApprovalRepo transition: no pending row for id=%s", id)
	}
	it.Status = to
	r.items[id] = it
	return nil
}

func (r *fakeApprovalRepo) SetResumeWindow(_ context.Context, _ crmapprovals.DBExec, _ string, _ json.RawMessage) error {
	return nil
}

// ─── local fakeSor satisfying datasource.Provider ─────────────────────

type fakeSorProvider struct{}

func (fakeSorProvider) Read(_ context.Context, ref datasource.EntityRef) (any, error) {
	return map[string]any{"id": ref.ID}, nil
}

func (fakeSorProvider) Search(_ context.Context, _ datasource.SearchQuery) (datasource.SearchResult, error) {
	return datasource.SearchResult{}, nil
}

func (fakeSorProvider) ListObjects(_ context.Context) ([]datasource.ObjectDef, error) {
	return nil, nil
}

func (fakeSorProvider) ListFields(_ context.Context, _ datasource.EntityType) ([]datasource.FieldDef, error) {
	return nil, nil
}

func (fakeSorProvider) Create(_ context.Context, in datasource.CreateInput) (datasource.EntityRef, error) {
	return datasource.EntityRef{Type: in.Type, ID: "new-id"}, nil
}

func (fakeSorProvider) Update(_ context.Context, in datasource.UpdateInput) (datasource.EntityRef, error) {
	return datasource.EntityRef{Type: in.Type, ID: in.ID}, nil
}

func (fakeSorProvider) AdvanceDeal(_ context.Context, in datasource.AdvanceDealInput) (datasource.EntityRef, error) {
	return datasource.EntityRef{Type: datasource.EntityDeal, ID: in.DealID}, nil
}

func (fakeSorProvider) RunReport(_ context.Context, _ datasource.ReportPlan) (datasource.ReportResult, error) {
	//nolint:nilnil // fake provider: a nil report with no error is a valid empty result
	return nil, nil
}

func (fakeSorProvider) Freshness(_ context.Context, _ datasource.EntityRef) (datasource.FreshnessInfo, error) {
	return datasource.FreshnessInfo{Authoritative: true}, nil
}

func (fakeSorProvider) LinkConversation(_ context.Context, _ datasource.LinkConversationInput) (datasource.EntityRef, error) {
	return datasource.EntityRef{}, nil
}

func (fakeSorProvider) UnlinkConversation(_ context.Context, _ datasource.UnlinkConversationInput) error {
	return nil
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestDecider_Approve_EmitsDecided(t *testing.T) {
	db := fakeDecisionDB(t)
	tx := fakeDecisionTx(t, db)
	repo := newFakeApprovalRepo()
	spy := &spyEmitter{}
	d := crmapprovals.Decider{Repo: repo, Datasource: fakeSorProvider{}, Emitter: spy}

	id, err := repo.Create(context.Background(), tx, crmapprovals.Item{
		WorkspaceID: "ws1",
		ActionType:  "update_record",
		Payload:     json.RawMessage(`{"id":"r1"}`),
		Status:      crmapprovals.StatusPending,
		RequestedBy: "agent1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := d.Approve(context.Background(), tx, id, "human1"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if len(spy.topics) != 1 || spy.topics[0] != crmapprovals.TopicApprovalDecided {
		t.Fatalf("want 1 approval.decided, got %v", spy.topics)
	}
	if !strings.Contains(spy.payloads[0], `"approved"`) || !strings.Contains(spy.payloads[0], id) {
		t.Fatalf("payload = %s", spy.payloads[0])
	}
}

// TestDecider_Modify_ReStagesWhenEditLandsYellow proves the tier-on-edit guard:
// when the edited payload re-resolves to 🟡 (e.g. advance_deal to_status=won), Modify
// must NOT execute the edited action under the original approval — it re-stages a
// fresh pending item for its own approval cycle, and the datasource write never fires.
func TestDecider_Modify_ReStagesWhenEditLandsYellow(t *testing.T) {
	db := fakeDecisionDB(t)
	tx := fakeDecisionTx(t, db)
	repo := newFakeApprovalRepo()
	datasource := &spyExecDatasource{}
	d := crmapprovals.Decider{
		Repo: repo, Datasource: datasource,
		// Edited payloads that advance a deal to a terminal status re-resolve 🟡.
		ResolveYellow: func(_ string, payload json.RawMessage) bool {
			var pl struct {
				Fields map[string]any `json:"fields"`
			}
			_ = json.Unmarshal(payload, &pl)
			return pl.Fields["stage"] == "won"
		},
	}

	id, err := repo.Create(context.Background(), tx, crmapprovals.Item{
		WorkspaceID: "ws1",
		ActionType:  "update_record",
		Payload:     json.RawMessage(`{"kind":"deal","id":"d1","fields":{"stage":"qualified"}}`),
		Status:      crmapprovals.StatusPending,
		RequestedBy: "agent1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	edited := json.RawMessage(`{"kind":"deal","id":"d1","fields":{"stage":"won"}}`)
	passGate := func(context.Context, string, string, json.RawMessage) error { return nil }

	if err := d.Modify(context.Background(), tx, id, "human1", edited, passGate); err != nil {
		t.Fatalf("modify: %v", err)
	}

	// The edited (now-🟡) effect must NOT have executed.
	if datasource.updates != 0 {
		t.Fatalf("edit into 🟡 must re-stage, not execute: datasource.Update called %d times", datasource.updates)
	}
	// A fresh pending item must exist carrying the edited payload.
	foundPending := false
	for _, it := range repo.items {
		if it.Status == crmapprovals.StatusPending && string(it.Payload) == string(edited) {
			foundPending = true
		}
	}
	if !foundPending {
		t.Fatal("a re-staged pending item with the edited payload must exist")
	}
}

// spyExecDatasource counts datasource execution calls (Update/Create). Thread-safe so the
// concurrent-approve exactly-once test can assert the count under -race.
type spyExecDatasource struct {
	fakeSorProvider
	mu      sync.Mutex
	updates int
	creates int
}

func (s *spyExecDatasource) Update(ctx context.Context, in datasource.UpdateInput) (datasource.EntityRef, error) {
	s.mu.Lock()
	s.updates++
	s.mu.Unlock()
	return datasource.EntityRef{Type: in.Type, ID: in.ID}, nil
}

func (s *spyExecDatasource) Create(ctx context.Context, in datasource.CreateInput) (datasource.EntityRef, error) {
	s.mu.Lock()
	s.creates++
	s.mu.Unlock()
	return datasource.EntityRef{Type: in.Type, ID: "new-id"}, nil
}

func (s *spyExecDatasource) updateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updates
}

// TestDecider_Approve_ExactlyOnce_Sequential proves a re-approve of an already-
// approved item refuses (the item is no longer pending) and does NOT re-fire the
// irreversible datasource action: at-most-once execution across repeated approvals.
func TestDecider_Approve_ExactlyOnce_Sequential(t *testing.T) {
	db := fakeDecisionDB(t)
	tx := fakeDecisionTx(t, db)
	repo := newFakeApprovalRepo()
	spy := &spyExecDatasource{}
	d := crmapprovals.Decider{Repo: repo, Datasource: spy}

	id, err := repo.Create(context.Background(), tx, crmapprovals.Item{
		WorkspaceID: "ws1", ActionType: "update_record",
		Payload: json.RawMessage(`{"kind":"person","id":"r1","fields":{"email":"a@b.com"}}`),
		Status:  crmapprovals.StatusPending, RequestedBy: "agent1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := d.Approve(context.Background(), tx, id, "human1"); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	// Second approve must refuse — the item is no longer pending.
	if err := d.Approve(context.Background(), tx, id, "human2"); err == nil {
		t.Fatal("re-approve of an approved item must error, not silently re-fire")
	}
	if got := spy.updateCount(); got != 1 {
		t.Fatalf("datasource.Update fired %d times across two approves, want exactly 1", got)
	}
}

// TestDecider_Approve_ExactlyOnce_Concurrent proves two approvers racing the same
// pending item execute the action exactly once: the conditional transition claims
// the row (only one wins RowsAffected>0), so only the winner reaches exec.
func TestDecider_Approve_ExactlyOnce_Concurrent(t *testing.T) {
	db := fakeDecisionDB(t)
	repo := newFakeApprovalRepo()
	spy := &spyExecDatasource{}
	d := crmapprovals.Decider{Repo: repo, Datasource: spy}

	// Seed via a throwaway tx (Create doesn't touch the fake's tx).
	seedTx := fakeDecisionTx(t, db)
	id, err := repo.Create(context.Background(), seedTx, crmapprovals.Item{
		WorkspaceID: "ws1", ActionType: "update_record",
		Payload: json.RawMessage(`{"kind":"person","id":"r1","fields":{"email":"a@b.com"}}`),
		Status:  crmapprovals.StatusPending, RequestedBy: "agent1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tx := fakeDecisionTx(t, db)
			errs[idx] = d.Approve(context.Background(), tx, id, fmt.Sprintf("human%d", idx))
		}(i)
	}
	wg.Wait()

	wins, fails := 0, 0
	for _, e := range errs {
		if e == nil {
			wins++
		} else {
			fails++
		}
	}
	if wins != 1 || fails != 1 {
		t.Fatalf("concurrent approve: wins=%d fails=%d, want exactly 1 each", wins, fails)
	}
	if got := spy.updateCount(); got != 1 {
		t.Fatalf("concurrent approve fired datasource.Update %d times, want exactly 1", got)
	}
}

func TestDecider_Reject_EmitsDecided(t *testing.T) {
	db := fakeDecisionDB(t)
	tx := fakeDecisionTx(t, db)
	repo := newFakeApprovalRepo()
	spy := &spyEmitter{}
	d := crmapprovals.Decider{Repo: repo, Datasource: fakeSorProvider{}, Emitter: spy}

	id, err := repo.Create(context.Background(), tx, crmapprovals.Item{
		WorkspaceID: "ws1",
		ActionType:  "update_record",
		Payload:     json.RawMessage(`{"id":"r1"}`),
		Status:      crmapprovals.StatusPending,
		RequestedBy: "agent1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := d.Reject(context.Background(), tx, id, "human1", "not appropriate"); err != nil {
		t.Fatalf("reject: %v", err)
	}

	if len(spy.topics) != 1 || spy.topics[0] != crmapprovals.TopicApprovalDecided {
		t.Fatalf("want 1 approval.decided, got %v", spy.topics)
	}
	if !strings.Contains(spy.payloads[0], `"rejected"`) || !strings.Contains(spy.payloads[0], id) {
		t.Fatalf("payload = %s", spy.payloads[0])
	}
}
