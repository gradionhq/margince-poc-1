//go:build integration

package crmgdpr_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

// checkGranted exercises the canonical consent read path (ConsentRepository.
// FindForPurpose) and collapses it to the granted/not-granted bool the assertions
// below need. The tests set app.workspace_id session-wide before calling, which
// satisfies the RLS GUC that FindForPurpose leaves to its caller.
func checkGranted(t *testing.T, db *sql.DB, wsID, personID, purpose string) bool {
	t.Helper()
	repo := crmgdpr.NewConsentRepository(db)
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)
	state, err := repo.FindForPurpose(context.Background(), wsID, personID, purpose)
	if err != nil {
		t.Fatalf("FindForPurpose %q: %v", purpose, err)
	}
	return state == crmgdpr.Granted
}

// seedWorkspaceAndPerson creates a workspace + person and returns (wsID, personID).
func seedWorkspaceAndPerson(t *testing.T, db *sql.DB) (string, string) {
	t.Helper()
	wsID := ids.New()
	userID := ids.New()
	personID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`,
		wsID, "gdpr-test-"+wsID, "gdpr-test-"+wsID)
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`,
		userID, wsID, "u"+userID+"@t.test", "Tester")
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)
	mustExec(t, db, `INSERT INTO person (id,workspace_id,full_name,source,captured_by,version)
		VALUES ($1::uuid,$2::uuid,'Test Person','test','test',1)`, personID, wsID)
	return wsID, personID
}

// TestConsentPurposeSeeds verifies the four seed rows are present.
func TestConsentPurposeSeeds(t *testing.T) {
	db := testDB(t)
	expected := []string{"marketing_email", "marketing_phone", "profiling", "product_updates"}
	for _, name := range expected {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM consent_purpose WHERE name=$1`, name).Scan(&n); err != nil {
			t.Fatalf("query consent_purpose: %v", err)
		}
		if n != 1 {
			t.Errorf("consent_purpose seed %q: count=%d want 1", name, n)
		}
	}
}

// TestRecord_GrantThenWithdraw verifies that:
//   - grant produces state=granted + one consent_event row
//   - withdraw produces state=withdrawn + a second consent_event row
//   - both event rows have non-null policy_wording + policy_version
//   - each Record call writes exactly one audit_log row
func TestRecord_GrantThenWithdraw(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsID})

	grantReq := crmgdpr.ConsentRequest{
		WorkspaceID:   wsID,
		PersonID:      personID,
		PurposeName:   "marketing_email",
		NewState:      crmgdpr.Granted,
		Channel:       "web_form",
		LawfulBasis:   "consent",
		PolicyWording: "I agree to receive marketing emails",
		PolicyVersion: "v1",
		Source:        "test",
	}
	if err := crmgdpr.Record(ctx, db, grantReq); err != nil {
		t.Fatalf("Record grant: %v", err)
	}

	// Check state is granted.
	if !checkGranted(t, db, wsID, personID, "marketing_email") {
		t.Fatal("consent after grant: want granted")
	}

	// Verify consent_event row.
	var eventCount int
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)
	if err := db.QueryRow(
		`SELECT count(*) FROM consent_event WHERE person_id=$1::uuid AND event_state='granted'
		 AND policy_wording IS NOT NULL AND policy_version IS NOT NULL`, personID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("query consent_event: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected 1 granted consent_event, got %d", eventCount)
	}

	// Verify audit_log row for the grant.
	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='person_consent' AND action='update'
		 AND workspace_id=$1::uuid`, wsID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if auditCount < 1 {
		t.Fatalf("expected at least 1 audit_log row after grant, got %d", auditCount)
	}
	prevAuditCount := auditCount

	// Now withdraw.
	withdrawReq := crmgdpr.ConsentRequest{
		WorkspaceID:   wsID,
		PersonID:      personID,
		PurposeName:   "marketing_email",
		NewState:      crmgdpr.Withdrawn,
		Channel:       "email_link",
		LawfulBasis:   "consent",
		PolicyWording: "Withdrawal acknowledged",
		PolicyVersion: "v1",
		Source:        "test",
	}
	if err := crmgdpr.Record(ctx, db, withdrawReq); err != nil {
		t.Fatalf("Record withdraw: %v", err)
	}

	// State should now be withdrawn.
	if checkGranted(t, db, wsID, personID, "marketing_email") {
		t.Fatal("consent after withdraw: want not granted")
	}

	// Two consent_event rows total.
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)
	if err := db.QueryRow(
		`SELECT count(*) FROM consent_event WHERE person_id=$1::uuid`, personID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("query consent_event count: %v", err)
	}
	if eventCount != 2 {
		t.Fatalf("expected 2 consent_event rows, got %d", eventCount)
	}

	// Another audit row for the withdraw.
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='person_consent' AND action='update'
		 AND workspace_id=$1::uuid`, wsID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if auditCount != prevAuditCount+1 {
		t.Fatalf("expected %d audit rows after withdraw, got %d", prevAuditCount+1, auditCount)
	}
}

// TestConsentEvent_AppendOnly verifies UPDATE and DELETE on consent_event both raise an error.
func TestConsentEvent_AppendOnly(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsID})

	req := crmgdpr.ConsentRequest{
		WorkspaceID:   wsID,
		PersonID:      personID,
		PurposeName:   "product_updates",
		NewState:      crmgdpr.Granted,
		PolicyWording: "I agree",
		PolicyVersion: "v1",
		Source:        "test",
	}
	if err := crmgdpr.Record(ctx, db, req); err != nil {
		t.Fatalf("Record: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// UPDATE must fail.
	_, updateErr := db.Exec(
		`UPDATE consent_event SET source='mutated' WHERE person_id=$1::uuid`, personID,
	)
	if updateErr == nil {
		t.Fatal("UPDATE consent_event should raise an error (append-only)")
	}

	// DELETE must fail.
	_, deleteErr := db.Exec(
		`DELETE FROM consent_event WHERE person_id=$1::uuid`, personID,
	)
	if deleteErr == nil {
		t.Fatal("DELETE consent_event should raise an error (append-only)")
	}
}

// TestCheck_DefaultDeny verifies the default-deny contract:
//   - unknown state ⇒ false
//   - withdrawn state ⇒ false
//   - no row ⇒ false
//   - grant for a different purpose ⇒ false (cross-purpose isolation)
func TestCheck_DefaultDeny(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsID})

	// No row at all → false.
	if checkGranted(t, db, wsID, personID, "marketing_email") {
		t.Fatal("no row: want not granted (default-deny)")
	}

	// Grant product_updates only.
	if err := crmgdpr.Record(ctx, db, crmgdpr.ConsentRequest{
		WorkspaceID:   wsID,
		PersonID:      personID,
		PurposeName:   "product_updates",
		NewState:      crmgdpr.Granted,
		PolicyWording: "agree",
		PolicyVersion: "v1",
		Source:        "test",
	}); err != nil {
		t.Fatalf("Record product_updates grant: %v", err)
	}

	// Cross-purpose: marketing_email must still be false.
	if checkGranted(t, db, wsID, personID, "marketing_email") {
		t.Fatal("cross-purpose: want not granted (grant for product_updates must not satisfy marketing_email)")
	}

	// Withdraw product_updates → false.
	if err := crmgdpr.Record(ctx, db, crmgdpr.ConsentRequest{
		WorkspaceID:   wsID,
		PersonID:      personID,
		PurposeName:   "product_updates",
		NewState:      crmgdpr.Withdrawn,
		PolicyWording: "withdrawn",
		PolicyVersion: "v1",
		Source:        "test",
	}); err != nil {
		t.Fatalf("Record withdraw: %v", err)
	}
	if checkGranted(t, db, wsID, personID, "product_updates") {
		t.Fatal("withdrawn: want not granted")
	}
}
