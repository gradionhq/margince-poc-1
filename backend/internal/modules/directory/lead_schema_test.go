//go:build integration

package crmcore_test

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
)

// TestLeadHasNoOrgFK asserts that lead has no foreign key into organization.
func TestLeadHasNoOrgFK(t *testing.T) {
	d := sqlDB(t)

	var count int
	err := d.QueryRow(`
		SELECT count(*)
		FROM information_schema.table_constraints tc
		JOIN information_schema.constraint_column_usage ccu
		  ON tc.constraint_name = ccu.constraint_name
		  AND tc.constraint_schema = ccu.constraint_schema
		WHERE tc.table_schema = 'public'
		  AND tc.table_name = 'lead'
		  AND tc.constraint_type = 'FOREIGN KEY'
		  AND ccu.table_name = 'organization'
	`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count > 0 {
		t.Errorf("lead table must have no FK into organization; found %d", count)
	}
}

// TestLeadPersonSegregation pins the lead/person FK directionality after 000030.
func TestLeadPersonSegregation(t *testing.T) {
	d := sqlDB(t)

	var leadToPersonCount int
	err := d.QueryRow(`
		SELECT count(*)
		FROM information_schema.table_constraints tc
		JOIN information_schema.constraint_column_usage ccu
		  ON tc.constraint_name = ccu.constraint_name
		  AND tc.constraint_schema = ccu.constraint_schema
		WHERE tc.table_schema = 'public'
		  AND tc.table_name = 'lead'
		  AND tc.constraint_type = 'FOREIGN KEY'
		  AND ccu.table_name = 'person'
	`).Scan(&leadToPersonCount)
	if err != nil {
		t.Fatal(err)
	}
	if leadToPersonCount != 1 {
		t.Errorf("lead→person FKs: want exactly 1 (promoted_person_id); got %d", leadToPersonCount)
	}

	var personToLeadCount int
	err = d.QueryRow(`
		SELECT count(*)
		FROM information_schema.table_constraints tc
		JOIN information_schema.constraint_column_usage ccu
		  ON tc.constraint_name = ccu.constraint_name
		  AND tc.constraint_schema = ccu.constraint_schema
		WHERE tc.table_schema = 'public'
		  AND tc.table_name = 'person'
		  AND tc.constraint_type = 'FOREIGN KEY'
		  AND ccu.table_name = 'lead'
	`).Scan(&personToLeadCount)
	if err != nil {
		t.Fatal(err)
	}
	if personToLeadCount != 1 {
		t.Errorf("person→lead FKs: want exactly 1 (converted_from_lead_id); got %d", personToLeadCount)
	}
}

// TestLeadSourceExtUniqueConstraint proves the external-import uniqueness and
// promoted lead round-trip on the live schema.
func TestLeadSourceExtUniqueConstraint(t *testing.T) {
	d := sqlDB(t)

	nonce := fmt.Sprintf("lead-schema-%d", os.Getpid())
	var wsID, personID string

	if err := d.QueryRow(
		`INSERT INTO workspace(name,slug,base_currency) VALUES ($1,$2,'USD') RETURNING id`,
		"ws-lead-"+nonce, "slug-lead-"+nonce,
	).Scan(&wsID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if _, err := d.Exec(`SET app.workspace_id = '` + wsID + `'`); err != nil {
		t.Fatalf("set workspace id: %v", err)
	}

	if err := d.QueryRow(
		`INSERT INTO person(workspace_id,full_name,source,captured_by) VALUES ($1,'Promo Person','test','test') RETURNING id`,
		wsID,
	).Scan(&personID); err != nil {
		t.Fatalf("create person: %v", err)
	}

	var leadID string
	if err := d.QueryRow(
		`
		INSERT INTO lead(workspace_id, full_name, status, source, captured_by, source_system, source_id)
		VALUES ($1, 'Apollo Lead', 'new', 'apollo', 'test', 'apollo', 'lead-1')
		RETURNING id`,
		wsID,
	).Scan(&leadID); err != nil {
		t.Fatalf("first lead insert failed: %v", err)
	}

	_, err := d.Exec(
		`
		INSERT INTO lead(workspace_id, full_name, status, source, captured_by, source_system, source_id)
		VALUES ($1, 'Apollo Lead Dup', 'new', 'apollo', 'test', 'apollo', 'lead-1')`,
		wsID,
	)
	if err == nil {
		t.Error("expected unique-constraint violation for duplicate (workspace_id, source_system, source_id), got nil")
	}

	if _, err := d.Exec(
		`
		INSERT INTO lead(workspace_id, full_name, status, source, captured_by, source_system, source_id)
		VALUES ($1, 'Apollo Lead 2', 'new', 'apollo', 'test', 'apollo', 'lead-2')`,
		wsID,
	); err != nil {
		t.Fatalf("second lead (different source_id) insert failed: %v", err)
	}

	if _, err := d.Exec(
		`
		UPDATE lead SET promoted_person_id = $1, promoted_at = now(), status = 'promoted'
		WHERE id = $2`,
		personID, leadID,
	); err != nil {
		t.Fatalf("update promoted_person_id: %v", err)
	}

	var gotPersonID string
	var gotPromotedAt sql.NullString
	if err := d.QueryRow(
		`
		SELECT promoted_person_id::text, promoted_at::text
		FROM lead WHERE id = $1`,
		leadID,
	).Scan(&gotPersonID, &gotPromotedAt); err != nil {
		t.Fatalf("read promoted lead: %v", err)
	}
	if gotPersonID != personID {
		t.Errorf("promoted_person_id: want %s, got %s", personID, gotPersonID)
	}
	if !gotPromotedAt.Valid {
		t.Error("promoted_at: want non-NULL, got NULL")
	}

	var srcCount int
	if err := d.QueryRow(
		`
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'lead'
		  AND column_name IN ('source', 'captured_by')
		  AND is_nullable = 'NO'`,
	).Scan(&srcCount); err != nil {
		t.Fatal(err)
	}
	if srcCount != 2 {
		t.Errorf("source and captured_by must both be NOT NULL; found %d NOT NULL of 2", srcCount)
	}
}
