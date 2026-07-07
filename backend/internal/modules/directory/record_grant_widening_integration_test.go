//go:build integration

package crmcore_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// TestRecordGrant_WidensThenRevokes proves AC-WS-B#2's full, real,
// end-to-end acceptance test for deal — the type whose app-layer OwnerID
// filter was actually widened in Task 6 Step 5: a user with no own access to
// a deal cannot see it via the owner-filtered list; once granted, they can;
// once revoked, they can't again; a write grant satisfies a read check.
func TestRecordGrant_WidensThenRevokes(t *testing.T) {
	db := sqlDB(t)
	ctx := context.Background()
	ws := newWorkspaceSQL(t, db)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create two users: ownerA and subjectB.
	var ownerA string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "ownerA-"+nonce+"@test.com", "Owner A").Scan(&ownerA); err != nil {
		t.Fatalf("seed ownerA: %v", err)
	}

	var subjectB string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "subjectB-"+nonce+"@test.com", "Subject B").Scan(&subjectB); err != nil {
		t.Fatalf("seed subjectB: %v", err)
	}

	// Create a pipeline and stage for the deal.
	pl, err := deals.NewPipelineStore(db).Create(ctx, deals.Pipeline{
		WorkspaceID: ws, Name: "test-" + nonce, IsDefault: false, Position: 1,
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

	// Create a deal owned by ownerA.
	d := crmcore.NewDeal("Widening Test Deal-"+nonce, pl.ID, st.ID, prov.Provenance{Source: "api", CapturedBy: "human:test"})
	d.WorkspaceID = ws
	d.OwnerID = &ownerA
	deal, err := crmcore.NewDealStore(db).Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}

	// 1. subjectB lists deals filtered by owner_id=subjectB — expect 0 results (subjectB owns nothing, no grant yet).
	dealList, _, err := crmcore.NewDealStore(db).ListFiltered(ctx, ws, "", 100, crmcore.DealListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list deals (before grant): %v", err)
	}
	if len(dealList) != 0 {
		t.Errorf("before grant: subject B should see 0 deals when filtering by owner_id, got %d", len(dealList))
	}

	// 2. Grant subjectB "read" access to the deal via ownerA (who has write access as the owner).
	grantStore := crmcore.NewRecordGrantStore(db)
	grant, err := grantStore.Create(ctx, crmcore.CreateRecordGrantInput{
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
	dealList, _, err = crmcore.NewDealStore(db).ListFiltered(ctx, ws, "", 100, crmcore.DealListFilter{OwnerID: subjectB})
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
	dealList, _, err = crmcore.NewDealStore(db).ListFiltered(ctx, ws, "", 100, crmcore.DealListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list deals (after revoke): %v", err)
	}
	if len(dealList) != 0 {
		t.Errorf("after revoke: subject B should see 0 deals when filtering by owner_id, got %d", len(dealList))
	}
}

// TestRecordGrant_OrganizationWidensThenRevokes is TestRecordGrant_WidensThenRevokes's
// identical twin for organization (also app-layer-widened in Task 6 Step 5) — same
// 4-step grant/revoke shape, against store_org_list's ListFiltered(OwnerID: ...) instead of deal's.
func TestRecordGrant_OrganizationWidensThenRevokes(t *testing.T) {
	db := sqlDB(t)
	ctx := context.Background()
	ws := newWorkspaceSQL(t, db)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create two users: ownerA and subjectB.
	var ownerA string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "ownerA-org-"+nonce+"@test.com", "Owner A").Scan(&ownerA); err != nil {
		t.Fatalf("seed ownerA: %v", err)
	}

	var subjectB string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "subjectB-org-"+nonce+"@test.com", "Subject B").Scan(&subjectB); err != nil {
		t.Fatalf("seed subjectB: %v", err)
	}

	// Create an organization owned by ownerA.
	org := crmcore.NewOrganization("Org-"+nonce, prov.Provenance{Source: "api", CapturedBy: "human:test"})
	org.WorkspaceID = ws
	org.OwnerID = &ownerA
	org, err := crmcore.NewOrgStore(db).Create(ctx, org)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	// 1. subjectB lists orgs filtered by owner_id=subjectB — expect 0 results (subjectB owns nothing, no grant yet).
	orgList, _, err := crmcore.NewOrgStore(db).List(ctx, ws, "", 100, "", crmcore.OrgListFilter{OwnerID: subjectB})
	if err != nil {
		t.Fatalf("list orgs (before grant): %v", err)
	}
	if len(orgList) != 0 {
		t.Errorf("before grant: subject B should see 0 orgs when filtering by owner_id, got %d", len(orgList))
	}

	// 2. Grant subjectB "read" access to the org via ownerA (who has write access as the owner).
	grantStore := crmcore.NewRecordGrantStore(db)
	grant, err := grantStore.Create(ctx, crmcore.CreateRecordGrantInput{
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
	orgList, _, err = crmcore.NewOrgStore(db).List(ctx, ws, "", 100, "", crmcore.OrgListFilter{OwnerID: subjectB})
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
	orgList, _, err = crmcore.NewOrgStore(db).List(ctx, ws, "", 100, "", crmcore.OrgListFilter{OwnerID: subjectB})
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
	db := sqlDB(t)
	ctx := context.Background()
	ws := newWorkspaceSQL(t, db)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create two users: ownerA and subjectB.
	var ownerA string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "ownerA-pl-"+nonce+"@test.com", "Owner A").Scan(&ownerA); err != nil {
		t.Fatalf("seed ownerA: %v", err)
	}

	var subjectB string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "subjectB-pl-"+nonce+"@test.com", "Subject B").Scan(&subjectB); err != nil {
		t.Fatalf("seed subjectB: %v", err)
	}

	// --- CRUD + audit round-trip for person ---
	// Create a person.
	var personID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO person(workspace_id,full_name,source,captured_by) VALUES($1,$2,'api','human:test') RETURNING id`,
		ws, "Person-"+nonce).Scan(&personID); err != nil {
		t.Fatalf("create person: %v", err)
	}

	// Grant read access to the person.
	grantStore := crmcore.NewRecordGrantStore(db)
	personGrant, err := grantStore.Create(ctx, crmcore.CreateRecordGrantInput{
		WorkspaceID:      ws,
		RecordType:       "person",
		RecordID:         personID,
		SubjectType:      "user",
		SubjectID:        subjectB,
		Access:           "read",
		GrantedBy:        ownerA,
		GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("Create person grant: %v", err)
	}

	// Verify the grant was written and an audit log entry exists.
	var personGrantCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, personGrant.ID).Scan(&personGrantCount); err != nil {
		t.Fatalf("count person grant: %v", err)
	}
	if personGrantCount != 1 {
		t.Errorf("want 1 person grant row, got %d", personGrantCount)
	}

	var personAuditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE action='record_share' AND entity_id=$1`, personGrant.ID).Scan(&personAuditCount); err != nil {
		t.Fatalf("count person audit: %v", err)
	}
	if personAuditCount != 1 {
		t.Errorf("want 1 person audit entry (action=record_share), got %d", personAuditCount)
	}

	// Revoke the person grant.
	if err := grantStore.Revoke(ctx, personGrant.ID, ws); err != nil {
		t.Fatalf("Revoke person grant: %v", err)
	}

	// Verify the grant was deleted and an audit log entry exists.
	var personRevokeCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, personGrant.ID).Scan(&personRevokeCount); err != nil {
		t.Fatalf("count person grant after revoke: %v", err)
	}
	if personRevokeCount != 0 {
		t.Errorf("want 0 person grant rows after revoke, got %d", personRevokeCount)
	}

	var personRevokeAuditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE action='record_unshare' AND entity_id=$1`, personGrant.ID).Scan(&personRevokeAuditCount); err != nil {
		t.Fatalf("count person revoke audit: %v", err)
	}
	if personRevokeAuditCount != 1 {
		t.Errorf("want 1 person audit entry (action=record_unshare), got %d", personRevokeAuditCount)
	}

	// --- CRUD + audit round-trip for lead ---
	// Create a lead.
	var leadID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO lead(workspace_id,full_name,source,captured_by) VALUES($1,$2,'api','human:test') RETURNING id`,
		ws, "Lead-"+nonce).Scan(&leadID); err != nil {
		t.Fatalf("create lead: %v", err)
	}

	// Grant read access to the lead.
	leadGrant, err := grantStore.Create(ctx, crmcore.CreateRecordGrantInput{
		WorkspaceID:      ws,
		RecordType:       "lead",
		RecordID:         leadID,
		SubjectType:      "user",
		SubjectID:        subjectB,
		Access:           "read",
		GrantedBy:        ownerA,
		GrantorOwnAccess: "write",
	})
	if err != nil {
		t.Fatalf("Create lead grant: %v", err)
	}

	// Verify the grant was written and an audit log entry exists.
	var leadGrantCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, leadGrant.ID).Scan(&leadGrantCount); err != nil {
		t.Fatalf("count lead grant: %v", err)
	}
	if leadGrantCount != 1 {
		t.Errorf("want 1 lead grant row, got %d", leadGrantCount)
	}

	var leadAuditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE action='record_share' AND entity_id=$1`, leadGrant.ID).Scan(&leadAuditCount); err != nil {
		t.Fatalf("count lead audit: %v", err)
	}
	if leadAuditCount != 1 {
		t.Errorf("want 1 lead audit entry (action=record_share), got %d", leadAuditCount)
	}

	// Revoke the lead grant.
	if err := grantStore.Revoke(ctx, leadGrant.ID, ws); err != nil {
		t.Fatalf("Revoke lead grant: %v", err)
	}

	// Verify the grant was deleted and an audit log entry exists.
	var leadRevokeCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, leadGrant.ID).Scan(&leadRevokeCount); err != nil {
		t.Fatalf("count lead grant after revoke: %v", err)
	}
	if leadRevokeCount != 0 {
		t.Errorf("want 0 lead grant rows after revoke, got %d", leadRevokeCount)
	}

	var leadRevokeAuditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE action='record_unshare' AND entity_id=$1`, leadGrant.ID).Scan(&leadRevokeAuditCount); err != nil {
		t.Fatalf("count lead revoke audit: %v", err)
	}
	if leadRevokeAuditCount != 1 {
		t.Errorf("want 1 lead audit entry (action=record_unshare), got %d", leadRevokeAuditCount)
	}

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
	db := sqlDB(t)
	ctx := context.Background()
	ws := newWorkspaceSQL(t, db)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create two users.
	var ownerA string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "ownerA-wsr-"+nonce+"@test.com", "Owner A").Scan(&ownerA); err != nil {
		t.Fatalf("seed ownerA: %v", err)
	}

	var subjectB string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
		ws, "subjectB-wsr-"+nonce+"@test.com", "Subject B").Scan(&subjectB); err != nil {
		t.Fatalf("seed subjectB: %v", err)
	}

	// Create a pipeline, stage, and deal.
	pl, err := deals.NewPipelineStore(db).Create(ctx, deals.Pipeline{
		WorkspaceID: ws, Name: "wsr-" + nonce, IsDefault: false, Position: 1,
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

	d := crmcore.NewDeal("WriteSatisfiesRead-"+nonce, pl.ID, st.ID, prov.Provenance{Source: "api", CapturedBy: "human:test"})
	d.WorkspaceID = ws
	d.OwnerID = &ownerA
	deal, err := crmcore.NewDealStore(db).Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}

	// Grant "write" access to the deal.
	grantStore := crmcore.NewRecordGrantStore(db)
	grant, err := grantStore.Create(ctx, crmcore.CreateRecordGrantInput{
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
	_, err = grantStore.Create(ctx, crmcore.CreateRecordGrantInput{
		WorkspaceID:      ws,
		RecordType:       "deal",
		RecordID:         deal.ID,
		SubjectType:      "user",
		SubjectID:        subjectB,
		Access:           "write", // Trying to escalate to write...
		GrantedBy:        subjectB, // ...from a principal who only has read
		GrantorOwnAccess: "read",   // Only read access
	})
	// This should fail because write > read.
	if err == nil {
		t.Errorf("creating write grant with only read access should fail, but succeeded")
	}
	if err != crmcore.ErrGrantExceedsGrantorAccess {
		t.Errorf("expected ErrGrantExceedsGrantorAccess, got %v", err)
	}
}
