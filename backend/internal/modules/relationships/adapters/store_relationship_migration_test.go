//go:build integration

package adapters_test

import (
	"context"
	"testing"
)

// TestRelationshipVersionColumn_UpMigration confirms 000067 landed: version
// defaults to 1 on insert and increments on UPDATE via trg_relationship_touch,
// exactly like partner.version (store_partner_test.go's pattern).
func TestRelationshipVersionColumn_UpMigration(t *testing.T) {
	db := openTestDB(t)
	wsID := "00000000-0000-0000-0000-0000000000a1"
	setRLS(t, db, wsID)
	seedWorkspace(t, db, wsID)

	var version int64
	var relID string
	if err := db.QueryRowContext(context.Background(), `
		WITH p AS (
			INSERT INTO person (workspace_id, full_name, source, captured_by)
			VALUES ($1::uuid, 'P1', 'test', 'human:test')
			RETURNING id
		), o AS (
			INSERT INTO organization (workspace_id, name, classification, source, captured_by)
			VALUES ($1::uuid, 'O1', 'prospect', 'test', 'human:test')
			RETURNING id
		)
		INSERT INTO relationship (workspace_id, kind, person_id, organization_id, source, captured_by)
		SELECT $1::uuid, 'employment', p.id, o.id, 'test', 'human:test'
		FROM p, o
		RETURNING id, version`, wsID).Scan(&relID, &version); err != nil {
		t.Fatalf("insert relationship: %v", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}

	if _, err := db.ExecContext(context.Background(),
		`UPDATE relationship SET role='cto' WHERE id=$1::uuid`, relID); err != nil {
		t.Fatalf("update relationship: %v", err)
	}
	if err := db.QueryRowContext(context.Background(),
		`SELECT version FROM relationship WHERE id=$1::uuid`, relID).Scan(&version); err != nil {
		t.Fatalf("reselect version: %v", err)
	}
	if version != 2 {
		t.Fatalf("version after update = %d, want 2 (trg_relationship_touch)", version)
	}
}
