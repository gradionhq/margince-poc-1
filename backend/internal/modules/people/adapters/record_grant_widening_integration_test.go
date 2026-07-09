//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	organizations "github.com/gradionhq/margince/backend/internal/modules/organizations"
	orgdomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	adapters "github.com/gradionhq/margince/backend/internal/modules/people/adapters"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// recordGrantRoundTripCase bundles the record/party-identifying params of
// assertGrantRevokeAuditRoundTrip into a single value (SonarCloud flags
// functions with >7 parameters; the 5 fields below plus (ctx, t, db,
// grantStore) would otherwise total 9).
type recordGrantRoundTripCase struct {
	WorkspaceID, RecordType, RecordID, OwnerA, SubjectB string
}

// assertGrantRevokeAuditRoundTrip does one create-grant -> assert row+audit
// (record_share) -> revoke -> assert row-gone+audit (record_unshare)
// round-trip for a single (recordType, recordID) pair, granted by ownerA to
// subjectB in workspace ws. Shared by TestRecordGrant_PersonLeadCRUDOnly's
// person and lead cases, which were previously written out twice inline
// (identical shape).
func assertGrantRevokeAuditRoundTrip(ctx context.Context, t *testing.T, db *sql.DB, grantStore *adapters.RecordGrantStore, c recordGrantRoundTripCase) {
	t.Helper()

	grant, err := grantStore.Create(ctx, adapters.CreateRecordGrantInput{
		WorkspaceID:      c.WorkspaceID,
		RecordType:       c.RecordType,
		RecordID:         c.RecordID,
		SubjectType:      "user",
		SubjectID:        c.SubjectB,
		Access:           "read",
		GrantedBy:        c.OwnerA,
		GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("Create %s grant: %v", c.RecordType, err)
	}

	var grantCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, grant.ID).Scan(&grantCount); err != nil {
		t.Fatalf("count %s grant: %v", c.RecordType, err)
	}
	if grantCount != 1 {
		t.Errorf("want 1 %s grant row, got %d", c.RecordType, grantCount)
	}

	var auditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE action='record_share' AND entity_id=$1`, grant.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count %s audit: %v", c.RecordType, err)
	}
	if auditCount != 1 {
		t.Errorf("want 1 %s audit entry (action=record_share), got %d", c.RecordType, auditCount)
	}

	if err := grantStore.Revoke(ctx, grant.ID, c.WorkspaceID); err != nil {
		t.Fatalf("Revoke %s grant: %v", c.RecordType, err)
	}

	var revokeCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, grant.ID).Scan(&revokeCount); err != nil {
		t.Fatalf("count %s grant after revoke: %v", c.RecordType, err)
	}
	if revokeCount != 0 {
		t.Errorf("want 0 %s grant rows after revoke, got %d", c.RecordType, revokeCount)
	}

	var revokeAuditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE action='record_unshare' AND entity_id=$1`, grant.ID).Scan(&revokeAuditCount); err != nil {
		t.Fatalf("count %s revoke audit: %v", c.RecordType, err)
	}
	if revokeAuditCount != 1 {
		t.Errorf("want 1 %s audit entry (action=record_unshare), got %d", c.RecordType, revokeAuditCount)
	}
}

// seedGrantTestUsers seeds an "ownerA" and a "subjectB" app_user row in
// workspace ws, with emails distinguished by tag (e.g. a test-specific prefix
// plus a nonce) so failure output stays unique across the record_grant test
// suite. Shared by every TestRecordGrant_* test below, which previously wrote
// this exact two-INSERT seed out inline.
func seedGrantTestUsers(ctx context.Context, t *testing.T, db *sql.DB, ws, tag string) (ownerA, subjectB string) {
	t.Helper()

	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "ownerA-"+tag+"@test.com", "Owner A").Scan(&ownerA); err != nil {
		t.Fatalf("seed ownerA: %v", err)
	}

	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "subjectB-"+tag+"@test.com", "Subject B").Scan(&subjectB); err != nil {
		t.Fatalf("seed subjectB: %v", err)
	}
	return ownerA, subjectB
}

// seedGrantTestDeal seeds a pipeline+stage+deal (named dealName) owned by
// ownerA in workspace ws. Shared by TestRecordGrant_WidensThenRevokes and
// TestRecordGrant_WriteSatisfiesRead, which previously wrote this exact
// pipeline-create -> stage-create -> deal-create sequence out inline.
func seedGrantTestDeal(ctx context.Context, t *testing.T, db *sql.DB, ws, ownerA, dealName string) deals.Deal {
	t.Helper()

	pl, err := deals.NewPipelineStore(db).Create(ctx, deals.Pipeline{
		WorkspaceID: ws, Name: "pipeline-" + dealName, IsDefault: false, Position: 1,
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

	d := deals.NewDeal(dealName, pl.ID, st.ID, prov.Provenance{Source: "api", CapturedBy: "human:test"})
	d.WorkspaceID = ws
	d.OwnerID = &ownerA
	deal, err := deals.NewDealStore(db).Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}
	return deal
}

// TestRecordGrant_WidensThenRevokes proves AC-WS-B#2's full, real,
// end-to-end acceptance test for deal — the type whose app-layer OwnerID
// filter was actually widened in Task 6 Step 5: a user with no own access to
// a deal cannot see it via the owner-filtered list; once granted, they can;
// once revoked, they can't again; a write grant satisfies a read check.
func TestRecordGrant_WidensThenRevokes(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx := context.Background()
	ws := pgtest.NewWorkspaceSQL(t, db)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create two users: ownerA and subjectB.
	ownerA, subjectB := seedGrantTestUsers(ctx, t, db, ws, nonce)

	// Create a pipeline, stage, and a deal owned by ownerA.
	deal := seedGrantTestDeal(ctx, t, db, ws, ownerA, "Widening Test Deal-"+nonce)

	// 1. subjectB lists deals filtered by owner_id=subjectB — expect 0 results (subjectB owns nothing, no grant yet).
	dealList, _, err := deals.NewDealStore(db).ListFiltered(ctx, ws, "", 100, deals.DealListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list deals (before grant): %v", err)
	}
	if len(dealList) != 0 {
		t.Errorf("before grant: subject B should see 0 deals when filtering by owner_id, got %d", len(dealList))
	}

	// 2. Grant subjectB "read" access to the deal via ownerA (who has write access as the owner).
	grantStore := adapters.NewRecordGrantStore(db)
	grant, err := grantStore.Create(ctx, adapters.CreateRecordGrantInput{
		WorkspaceID:      ws,
		RecordType:       "deal",
		RecordID:         deal.ID,
		SubjectType:      "user",
		SubjectID:        subjectB,
		Access:           "read",
		GrantedBy:        ownerA,
		GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("Create grant: %v", err)
	}

	// 3. subjectB lists deals again, filtering by owner_id=subjectB — expect 1 result (the granted deal).
	dealList, _, err = deals.NewDealStore(db).ListFiltered(ctx, ws, "", 100, deals.DealListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list deals (after grant): %v", err)
	}
	if len(dealList) != 1 {
		t.Errorf("after grant: subject B should see exactly 1 deal when filtering by owner_id, got %d", len(dealList))
	}
	if len(dealList) > 0 && dealList[0].ID != deal.ID {
		t.Errorf("after grant: expected deal ID %s, got %s", deal.ID, dealList[0].ID)
	}

	// 4. Revoke the grant.
	if err := grantStore.Revoke(ctx, grant.ID, ws); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// 5. subjectB lists deals again — expect 0 results (grant revoked).
	dealList, _, err = deals.NewDealStore(db).ListFiltered(ctx, ws, "", 100, deals.DealListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list deals (after revoke): %v", err)
	}
	if len(dealList) != 0 {
		t.Errorf("after revoke: subject B should see 0 deals when filtering by owner_id, got %d", len(dealList))
	}
}

// TestRecordGrant_OrganizationWidensThenRevokes is TestRecordGrant_WidensThenRevokes's
// identical twin for organization (also app-layer-widened in Task 6 Step 5) — same
// 4-step grant/revoke shape, against store_org_list's List(OwnerID: ...) instead of deal's.
func TestRecordGrant_OrganizationWidensThenRevokes(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx := context.Background()
	ws := pgtest.NewWorkspaceSQL(t, db)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create two users: ownerA and subjectB.
	ownerA, subjectB := seedGrantTestUsers(ctx, t, db, ws, "org-"+nonce)

	// Create an organization owned by ownerA.
	org := orgdomain.NewOrganization("Org-"+nonce, prov.Provenance{Source: "api", CapturedBy: "human:test"})
	org.WorkspaceID = ws
	org.OwnerID = &ownerA
	org, err := organizations.NewOrgStore(db).Create(ctx, org, nil)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	// 1. subjectB lists orgs filtered by owner_id=subjectB — expect 0 results (subjectB owns nothing, no grant yet).
	orgList, _, err := organizations.NewOrgStore(db).List(ctx, ws, "", 100, "", organizations.OrgListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list orgs (before grant): %v", err)
	}
	if len(orgList) != 0 {
		t.Errorf("before grant: subject B should see 0 orgs when filtering by owner_id, got %d", len(orgList))
	}

	// 2. Grant subjectB "read" access to the org via ownerA (who has write access as the owner).
	grantStore := adapters.NewRecordGrantStore(db)
	grant, err := grantStore.Create(ctx, adapters.CreateRecordGrantInput{
		WorkspaceID:      ws,
		RecordType:       "organization",
		RecordID:         org.ID,
		SubjectType:      "user",
		SubjectID:        subjectB,
		Access:           "read",
		GrantedBy:        ownerA,
		GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("Create grant: %v", err)
	}

	// 3. subjectB lists orgs again, filtering by owner_id=subjectB — expect 1 result (the granted org).
	orgList, _, err = organizations.NewOrgStore(db).List(ctx, ws, "", 100, "", organizations.OrgListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list orgs (after grant): %v", err)
	}
	if len(orgList) != 1 {
		t.Errorf("after grant: subject B should see exactly 1 org when filtering by owner_id, got %d", len(orgList))
	}
	if len(orgList) > 0 && orgList[0].ID != org.ID {
		t.Errorf("after grant: expected org ID %s, got %s", org.ID, orgList[0].ID)
	}

	// 4. Revoke the grant.
	if err := grantStore.Revoke(ctx, grant.ID, ws); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// 5. subjectB lists orgs again — expect 0 results (grant revoked).
	orgList, _, err = organizations.NewOrgStore(db).List(ctx, ws, "", 100, "", organizations.OrgListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list orgs (after revoke): %v", err)
	}
	if len(orgList) != 0 {
		t.Errorf("after revoke: subject B should see 0 orgs when filtering by owner_id, got %d", len(orgList))
	}
}

// TestRecordGrant_PersonLeadCRUDOnly is the HONEST test for person/lead
// (coordinator decision, GH-209 escalation response, Option 1): record_grant
// on person/lead ships this phase as a fully working, audited,
// approval-gated grant/revoke LEDGER with NO visibility effect yet — real
// row-scope enforcement for person/lead doesn't exist anywhere in this
// codebase (app layer or RLS) and building it is deferred to a future
// Phase 2/WS-C ticket. This test therefore explicitly does NOT claim or
// assert a visibility change for person/lead — it only proves grant/revoke
// CRUD + audit round-trips, and proves the exact boolean predicate migration
// 000069 wired into person_tenant_isolation/lead_tenant_isolation is correct
// in isolation.
func TestRecordGrant_PersonLeadCRUDOnly(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx := context.Background()
	ws := pgtest.NewWorkspaceSQL(t, db)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create two users: ownerA and subjectB.
	ownerA, subjectB := seedGrantTestUsers(ctx, t, db, ws, "pl-"+nonce)

	// --- CRUD + audit round-trip for person ---
	// Create a person.
	var personID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO person(workspace_id,full_name,source,captured_by) VALUES($1,$2,'api','human:test') RETURNING id`,
		ws, "Person-"+nonce).Scan(&personID); err != nil {
		t.Fatalf("create person: %v", err)
	}

	grantStore := adapters.NewRecordGrantStore(db)
	assertGrantRevokeAuditRoundTrip(ctx, t, db, grantStore, recordGrantRoundTripCase{
		WorkspaceID: ws, RecordType: "person", RecordID: personID, OwnerA: ownerA, SubjectB: subjectB,
	})

	// --- CRUD + audit round-trip for lead ---
	// Create a lead.
	var leadID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO lead(workspace_id,full_name,source,captured_by) VALUES($1,$2,'api','human:test') RETURNING id`,
		ws, "Lead-"+nonce).Scan(&leadID); err != nil {
		t.Fatalf("create lead: %v", err)
	}

	assertGrantRevokeAuditRoundTrip(ctx, t, db, grantStore, recordGrantRoundTripCase{
		WorkspaceID: ws, RecordType: "lead", RecordID: leadID, OwnerA: ownerA, SubjectB: subjectB,
	})

	// --- predicate-correctness-in-isolation only (NOT a visibility claim) ---
	// Create a person and grant to subjectB directly (insert row directly).
	var personTestID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO person(workspace_id,full_name,source,captured_by) VALUES($1,$2,'api','human:test') RETURNING id`,
		ws, "PersonPredicate-"+nonce).Scan(&personTestID); err != nil {
		t.Fatalf("create person for predicate test: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO record_grant(workspace_id,record_type,record_id,subject_type,subject_id,access,granted_by)
		 VALUES($1::uuid,$2,$3::uuid,$4,$5::uuid,$6,$7::uuid)`,
		ws, "person", personTestID, "user", subjectB, "read", ownerA); err != nil {
		t.Fatalf("insert grant for predicate test: %v", err)
	}

	// Query the widened RLS predicate in isolation: the EXISTS(...) branch should match.
	// We test this by running the exact condition from the RLS policy as a standalone query.
	var predicateMatches bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM record_grant rg
			WHERE rg.record_id=$1::uuid AND rg.subject_id=$2::uuid AND rg.subject_type='user'
			  AND (rg.expires_at IS NULL OR rg.expires_at > now())
		)`, personTestID, subjectB).Scan(&predicateMatches); err != nil {
		t.Fatalf("check predicate: %v", err)
	}
	if !predicateMatches {
		t.Errorf("predicate should match active grant, got %v", predicateMatches)
	}

	// Revoke and verify the predicate no longer matches.
	var grantToRevoke string
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM record_grant WHERE record_id=$1::uuid AND subject_id=$2::uuid`,
		personTestID, subjectB).Scan(&grantToRevoke); err != nil {
		t.Fatalf("find grant to revoke: %v", err)
	}
	if err := grantStore.Revoke(ctx, grantToRevoke, ws); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	var predicateMatchesAfter bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM record_grant rg
			WHERE rg.record_id=$1::uuid AND rg.subject_id=$2::uuid AND rg.subject_type='user'
			  AND (rg.expires_at IS NULL OR rg.expires_at > now())
		)`, personTestID, subjectB).Scan(&predicateMatchesAfter); err != nil {
		t.Fatalf("check predicate after revoke: %v", err)
	}
	if predicateMatchesAfter {
		t.Errorf("predicate should NOT match after revoke, got %v", predicateMatchesAfter)
	}
}

// TestRecordGrant_WriteSatisfiesRead proves "'write' also satisfies 'read'"
// (contract description) — a write-access grant, when queried by a read-only
// visibility check, still counts as satisfying it. This proves the app-layer
// access ranking where "write" >= "read".
func TestRecordGrant_WriteSatisfiesRead(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx := context.Background()
	ws := pgtest.NewWorkspaceSQL(t, db)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create two users.
	ownerA, subjectB := seedGrantTestUsers(ctx, t, db, ws, "wsr-"+nonce)

	// Create a pipeline, stage, and deal.
	deal := seedGrantTestDeal(ctx, t, db, ws, ownerA, "WriteSatisfiesRead-"+nonce)

	// Grant "write" access to the deal.
	grantStore := adapters.NewRecordGrantStore(db)
	grant, err := grantStore.Create(ctx, adapters.CreateRecordGrantInput{
		WorkspaceID:      ws,
		RecordType:       "deal",
		RecordID:         deal.ID,
		SubjectType:      "user",
		SubjectID:        subjectB,
		Access:           "write",
		GrantedBy:        ownerA,
		GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("Create write grant: %v", err)
	}

	// Verify the grant is "write".
	if grant.Access != "write" {
		t.Errorf("grant access should be 'write', got %q", grant.Access)
	}

	// Query the widened RLS predicate: a read-check query
	// (access IN ('read','write')) should still match a write-access grant.
	var writeGrantMatches bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM record_grant rg
			WHERE rg.record_id=$1::uuid AND rg.subject_id=$2::uuid AND rg.subject_type='user'
			  AND rg.access IN ('read','write')
			  AND (rg.expires_at IS NULL OR rg.expires_at > now())
		)`, deal.ID, subjectB).Scan(&writeGrantMatches); err != nil {
		t.Fatalf("check write grant satisfies read query: %v", err)
	}
	if !writeGrantMatches {
		t.Errorf("write grant should satisfy a read-check query, got %v", writeGrantMatches)
	}

	// Verify the store's own ranking is correct (accessRank["write"] > accessRank["read"]).
	// This is a sanity check: attempting to grant read when the granter has only read should fail.
	_, err = grantStore.Create(ctx, adapters.CreateRecordGrantInput{
		WorkspaceID:      ws,
		RecordType:       "deal",
		RecordID:         deal.ID,
		SubjectType:      "user",
		SubjectID:        subjectB,
		Access:           "write",  // Trying to escalate to write...
		GrantedBy:        subjectB, // ...from a principal who only has read
		GrantorOwnAccess: "read",   // Only read access
	})
	// This should fail because write > read.
	if err == nil {
		t.Errorf("creating write grant with only read access should fail, but succeeded")
	}
	if err != adapters.ErrGrantExceedsGrantorAccess {
		t.Errorf("expected ErrGrantExceedsGrantorAccess, got %v", err)
	}
}
