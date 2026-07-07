//go:build integration

package crmgdpr_test

import (
	"context"
	"database/sql"
	"testing"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// TestGDPR_RLSBackstop proves the fix: person rows are visible only to their
// own tenant through the seam, even with a query carrying NO workspace_id
// predicate at all — the historical bug (WS-A) was that gdpr functions ran on
// the superuser pool, where FORCE RLS never engages, so only the hand-written
// predicate (present in this codebase's queries, but NOT a guarantee in
// general) stood between tenants.
func TestGDPR_RLSBackstop(t *testing.T) {
	db := testDB(t)
	wsA, personA := seedWorkspaceAndPerson(t, db)
	wsB, personB := seedWorkspaceAndPerson(t, db)
	_ = personB

	var countAsA int
	err := database.WithWorkspaceTx(context.Background(), db, wsA, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM person`).Scan(&countAsA)
	})
	if err != nil {
		t.Fatalf("query as tenant A: %v", err)
	}
	if countAsA != 1 {
		t.Errorf("tenant A should see exactly its own 1 person row with NO workspace_id predicate, got %d", countAsA)
	}

	var countAsB int
	err = database.WithWorkspaceTx(context.Background(), db, wsB, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM person WHERE id = $1::uuid`, personA).Scan(&countAsB)
	})
	if err != nil {
		t.Fatalf("query as tenant B: %v", err)
	}
	if countAsB != 0 {
		t.Errorf("tenant B must see 0 rows for tenant A's specific person id even with no workspace_id predicate, got %d", countAsB)
	}
}

// TestGDPR_ConsentPurposeCrossTenantIsolation proves AC-C6: consent_purpose's
// per-workspace widening (migration 000070_ws_c_conformance, UNIQUE(workspace_id,
// key)) yields a genuinely disjoint row per workspace for the same key — not a
// shared global lookup row — and that a consent grant recorded in one workspace
// is completely invisible from another, even via a query carrying NO explicit
// workspace_id predicate (mirrors TestGDPR_RLSBackstop's proof style).
func TestGDPR_ConsentPurposeCrossTenantIsolation(t *testing.T) {
	db := testDB(t)
	wsA, personA := seedWorkspaceAndPerson(t, db)
	wsB, personB := seedWorkspaceAndPerson(t, db)
	_ = personB

	// 1. Distinct IDs, same keys: consent_purpose is per-workspace, not a
	// shared global row, even though both workspaces seed identical keys.
	purposesA := make(map[string]string)
	err := database.WithWorkspaceTx(context.Background(), db, wsA, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT key, id FROM consent_purpose`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var key, id string
			if err := rows.Scan(&key, &id); err != nil {
				return err
			}
			purposesA[key] = id
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("query consent_purpose as tenant A: %v", err)
	}

	purposesB := make(map[string]string)
	err = database.WithWorkspaceTx(context.Background(), db, wsB, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT key, id FROM consent_purpose`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var key, id string
			if err := rows.Scan(&key, &id); err != nil {
				return err
			}
			purposesB[key] = id
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("query consent_purpose as tenant B: %v", err)
	}

	if len(purposesA) == 0 || len(purposesB) == 0 {
		t.Fatalf("expected both tenants to have seeded consent_purpose rows, got A=%d B=%d", len(purposesA), len(purposesB))
	}
	for key, idA := range purposesA {
		idB, ok := purposesB[key]
		if !ok {
			t.Errorf("key %q present in tenant A's consent_purpose but missing in tenant B's", key)
			continue
		}
		if idA == idB {
			t.Errorf("key %q: tenant A and tenant B share the same consent_purpose id %q — expected distinct per-workspace rows", key, idA)
		}
	}

	// 2. Record a consent grant in wsA for personA.
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsA})
	grantReq := crmgdpr.ConsentRequest{
		WorkspaceID:   wsA,
		PersonID:      personA,
		PurposeName:   "marketing_email",
		NewState:      crmgdpr.Granted,
		Channel:       "web_form",
		LawfulBasis:   "consent",
		PolicyWording: "I agree to receive marketing emails",
		PolicyVersion: "v1",
		Source:        "test",
	}
	if err := crmgdpr.Record(ctx, db, grantReq); err != nil {
		t.Fatalf("Record grant in wsA: %v", err)
	}

	// 3. No leak into wsB: scoped as tenant B, a query for personA's grant with
	// NO explicit workspace_id predicate must return 0 rows — RLS alone must
	// hide wsA's data from a wsB-scoped session.
	var leakedCount int
	err = database.WithWorkspaceTx(context.Background(), db, wsB, func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT count(*) FROM person_consent pc
			 JOIN consent_purpose cp ON cp.id = pc.purpose_id
			 WHERE pc.person_id = $1::uuid`, personA,
		).Scan(&leakedCount)
	})
	if err != nil {
		t.Fatalf("query person_consent as tenant B: %v", err)
	}
	if leakedCount != 0 {
		t.Errorf("tenant B must see 0 person_consent rows for tenant A's person, even with no workspace_id predicate, got %d", leakedCount)
	}

	// 4. wsB's own consent_purpose count for the same key is exactly 1 — its
	// own per-workspace row, unaffected by wsA's grant.
	var ownCount int
	err = database.WithWorkspaceTx(context.Background(), db, wsB, func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT count(*) FROM consent_purpose WHERE key = $1`, "marketing_email",
		).Scan(&ownCount)
	})
	if err != nil {
		t.Fatalf("query consent_purpose count as tenant B: %v", err)
	}
	if ownCount != 1 {
		t.Errorf("tenant B should see exactly 1 consent_purpose row for key %q, got %d", "marketing_email", ownCount)
	}
}
