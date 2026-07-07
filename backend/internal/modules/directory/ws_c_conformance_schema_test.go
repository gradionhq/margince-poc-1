//go:build integration

// ws_c_conformance_schema_test.go proves migration 000070_ws_c_conformance's
// schema-level assertions (AC-C1..AC-C6) against the live, already-migrated
// TEST_DATABASE_URL database. Behavioral proofs that need a pre-migration
// fixture (the consent_purpose remap under the append-only trigger) live in
// consent_purpose_migration_test.go instead, on their own scratch database.
package crmcore_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"

	identitytransport "github.com/gradionhq/margince/backend/internal/modules/identity/transport"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// TestUUIDv7DefaultsRestored proves AC-C1: the four gen_random_uuid() survivors
// now default to uuidv7(), and the two natural-key text PKs are untouched.
func TestUUIDv7DefaultsRestored(t *testing.T) {
	d := sqlDB(t)

	for _, tbl := range []string{"event_outbox", "ai_metering", "embedding", "approval_item"} {
		var def sql.NullString
		if err := d.QueryRow(`
			SELECT column_default FROM information_schema.columns
			WHERE table_schema='public' AND table_name=$1 AND column_name='id'`, tbl).Scan(&def); err != nil {
			t.Fatalf("%s.id column_default: %v", tbl, err)
		}
		if !def.Valid || !strings.Contains(def.String, "uuidv7") {
			t.Errorf("%s.id default = %q, want to contain uuidv7()", tbl, def.String)
		}
	}

	// Regression guard: these two natural-key text PKs must remain untouched.
	naturalKeys := []struct{ table, column string }{
		{"oauth_auth_code", "code_hash"},
		{"consumed_approval_token", "jti"},
	}
	for _, nk := range naturalKeys {
		var dataType string
		var def sql.NullString
		if err := d.QueryRow(`
			SELECT data_type, column_default FROM information_schema.columns
			WHERE table_schema='public' AND table_name=$1 AND column_name=$2`,
			nk.table, nk.column).Scan(&dataType, &def); err != nil {
			t.Fatalf("%s.%s: %v", nk.table, nk.column, err)
		}
		if dataType != "text" {
			t.Errorf("%s.%s data_type = %q, want text", nk.table, nk.column, dataType)
		}
		if def.Valid {
			t.Errorf("%s.%s must have no default (natural key), got %q", nk.table, nk.column, def.String)
		}
	}
}

// assertUpdatedAtAdvances runs updateQ against id, then asserts updated_at
// strictly advanced.
func assertUpdatedAtAdvances(ctx context.Context, t *testing.T, d *sql.DB, table, id, updateQ string) {
	t.Helper()
	var before time.Time
	if err := d.QueryRowContext(ctx, "SELECT updated_at FROM "+table+" WHERE id=$1", id).Scan(&before); err != nil {
		t.Fatalf("%s: read updated_at before: %v", table, err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := d.ExecContext(ctx, updateQ, id); err != nil {
		t.Fatalf("%s: update: %v", table, err)
	}
	var after time.Time
	if err := d.QueryRowContext(ctx, "SELECT updated_at FROM "+table+" WHERE id=$1", id).Scan(&after); err != nil {
		t.Fatalf("%s: read updated_at after: %v", table, err)
	}
	if !after.After(before) {
		t.Errorf("%s: updated_at did not advance on UPDATE (before=%v after=%v)", table, before, after)
	}
}

// TestUpdatedAtTriggersRestored proves AC-C2: organization_domain, person_phone
// and embedding each bump updated_at on UPDATE.
func TestUpdatedAtTriggersRestored(t *testing.T) {
	d := sqlDB(t)
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	var wsID string
	if err := d.QueryRowContext(ctx,
		`INSERT INTO workspace(name,slug,base_currency) VALUES($1,$2,'EUR') RETURNING id`,
		"updtrig-"+nonce, "updtrig-"+nonce).Scan(&wsID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := d.ExecContext(ctx, `SELECT set_config('app.workspace_id',$1,false)`, wsID); err != nil {
		t.Fatalf("set guc: %v", err)
	}

	t.Run("organization_domain", func(t *testing.T) {
		var orgID string
		if err := d.QueryRowContext(ctx,
			`INSERT INTO organization(workspace_id,name,source,captured_by) VALUES($1,'Org','test','test') RETURNING id`,
			wsID).Scan(&orgID); err != nil {
			t.Fatalf("create organization: %v", err)
		}
		var domainID string
		if err := d.QueryRowContext(ctx,
			`INSERT INTO organization_domain(workspace_id,organization_id,domain) VALUES($1,$2,'example.com') RETURNING id`,
			wsID, orgID).Scan(&domainID); err != nil {
			t.Fatalf("create organization_domain: %v", err)
		}
		assertUpdatedAtAdvances(ctx, t, d, "organization_domain", domainID,
			`UPDATE organization_domain SET is_primary=true WHERE id=$1`)
	})

	t.Run("person_phone", func(t *testing.T) {
		var personID string
		if err := d.QueryRowContext(ctx,
			`INSERT INTO person(workspace_id,full_name,source,captured_by,version) VALUES($1,'Person','test','test',1) RETURNING id`,
			wsID).Scan(&personID); err != nil {
			t.Fatalf("create person: %v", err)
		}
		var phoneID string
		if err := d.QueryRowContext(ctx,
			`INSERT INTO person_phone(workspace_id,person_id,phone,source,captured_by) VALUES($1,$2,'+15550000000','test','test') RETURNING id`,
			wsID, personID).Scan(&phoneID); err != nil {
			t.Fatalf("create person_phone: %v", err)
		}
		assertUpdatedAtAdvances(ctx, t, d, "person_phone", phoneID,
			`UPDATE person_phone SET is_primary=true WHERE id=$1`)
	})

	t.Run("embedding", func(t *testing.T) {
		vec := "[" + strings.TrimSuffix(strings.Repeat("0,", 1024), ",") + "]"
		var embID string
		if err := d.QueryRowContext(ctx, `
			INSERT INTO embedding(workspace_id,source_type,source_id,content_hash,embedding,dims,source,captured_by)
			VALUES($1,'person',$2,'hash1',$3,1024,'test','test') RETURNING id`,
			wsID, ids.New(), vec).Scan(&embID); err != nil {
			t.Fatalf("create embedding: %v", err)
		}
		assertUpdatedAtAdvances(ctx, t, d, "embedding", embID,
			`UPDATE embedding SET content_hash='hash2' WHERE id=$1`)
	})
}

// fkDeleteRule returns the ON DELETE action for the named FK column.
func fkDeleteRule(t *testing.T, d *sql.DB, table, column string) string {
	t.Helper()
	var rule string
	err := d.QueryRow(`
		SELECT rc.delete_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.referential_constraints rc
		  ON tc.constraint_name = rc.constraint_name AND tc.table_schema = rc.constraint_schema
		WHERE tc.table_schema='public' AND tc.constraint_type='FOREIGN KEY'
		  AND tc.table_name=$1 AND kcu.column_name=$2`, table, column).Scan(&rule)
	if err != nil {
		t.Fatalf("fk delete_rule %s.%s: %v", table, column, err)
	}
	return rule
}

// TestSessionPassportAppUserFKActions proves AC-C3 (+D3): session.workspace_id,
// passport.workspace_id and passport.granted_by are now RESTRICT; the two
// untouched oauth_* FKs remain CASCADE (regression guard).
func TestSessionPassportAppUserFKActions(t *testing.T) {
	d := sqlDB(t)
	cases := []struct{ table, column, want string }{
		{"session", "workspace_id", "RESTRICT"},
		{"passport", "workspace_id", "RESTRICT"},
		{"passport", "granted_by", "RESTRICT"},
		{"oauth_client", "workspace_id", "CASCADE"},
		{"oauth_auth_code", "workspace_id", "CASCADE"},
	}
	for _, c := range cases {
		if got := fkDeleteRule(t, d, c.table, c.column); got != c.want {
			t.Errorf("%s.%s delete_rule = %q, want %q", c.table, c.column, got, c.want)
		}
	}
}

// TestEventOutboxCanonicalRLS proves the canonical deny-on-unset RLS policy: a
// connection with no app.workspace_id GUC sees zero rows and cannot insert.
func TestEventOutboxCanonicalRLS(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()

	var n int
	if err := asAppRole(t, pool, "", func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox`).Scan(&n)
	}); err != nil {
		t.Fatalf("count without guc: %v", err)
	}
	if n != 0 {
		t.Fatalf("event_outbox with no app.workspace_id must return 0 rows, got %d", n)
	}

	err := asAppRole(t, pool, "", func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO event_outbox(workspace_id,topic,entity_id) VALUES(gen_random_uuid(),'t',gen_random_uuid())`)
		return e
	})
	if err == nil {
		t.Fatal("event_outbox insert with no app.workspace_id must be rejected by WITH CHECK")
	}
}

// TestAppUserSeatTypeCheck proves AC-C4: app_user_agent_is_full rejects an
// agent seat with seat_type='read'; a human seat with seat_type='read' is
// fine; an insert omitting seat_type defaults to 'full'.
func TestAppUserSeatTypeCheck(t *testing.T) {
	d := sqlDB(t)
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	var wsID string
	if err := d.QueryRowContext(ctx,
		`INSERT INTO workspace(name,slug,base_currency) VALUES($1,$2,'EUR') RETURNING id`,
		"seattype-"+nonce, "seattype-"+nonce).Scan(&wsID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	_, err := d.ExecContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name,is_agent,seat_type) VALUES($1,$2,'Agent',true,'read')`,
		wsID, "agent-read-"+nonce+"@t.test")
	if err == nil {
		t.Fatal("expected CHECK violation for is_agent=true, seat_type='read'")
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		if pqErr.Code != "23514" {
			t.Errorf("error code = %s, want 23514 (check_violation)", pqErr.Code)
		}
	} else {
		t.Errorf("expected *pq.Error, got %T: %v", err, err)
	}

	if _, err := d.ExecContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name,is_agent,seat_type) VALUES($1,$2,'Human',false,'read')`,
		wsID, "human-read-"+nonce+"@t.test"); err != nil {
		t.Fatalf("is_agent=false, seat_type='read' should succeed: %v", err)
	}

	var seatType string
	if err := d.QueryRowContext(ctx,
		`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,'Defaulter') RETURNING seat_type`,
		wsID, "defaulter-"+nonce+"@t.test").Scan(&seatType); err != nil {
		t.Fatalf("insert omitting seat_type: %v", err)
	}
	if seatType != "full" {
		t.Errorf("default seat_type = %q, want %q", seatType, "full")
	}
}

// TestConsentPurposeWorkspaceScoped proves AC-C6 (D2): consent_purpose is now
// per-workspace (NOT NULL workspace_id, archived_at column, UNIQUE(workspace_id,
// key) rather than a bare UNIQUE(key)), and a brand-new workspace created
// through the real signup handler gets its own 4 default purposes (D2 step 7 —
// the backfill-of-pre-existing-workspaces path is proven separately in
// consent_purpose_migration_test.go, which is the only place a pre-migration
// fixture can exist).
func TestConsentPurposeWorkspaceScoped(t *testing.T) {
	d := sqlDB(t)

	var isNullable string
	if err := d.QueryRow(`
		SELECT is_nullable FROM information_schema.columns
		WHERE table_schema='public' AND table_name='consent_purpose' AND column_name='workspace_id'`).
		Scan(&isNullable); err != nil {
		t.Fatalf("query workspace_id nullability: %v", err)
	}
	if isNullable != "NO" {
		t.Errorf("consent_purpose.workspace_id is_nullable = %q, want NO", isNullable)
	}

	var archivedCount int
	if err := d.QueryRow(`
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema='public' AND table_name='consent_purpose' AND column_name='archived_at'`).
		Scan(&archivedCount); err != nil {
		t.Fatalf("query archived_at: %v", err)
	}
	if archivedCount != 1 {
		t.Error("consent_purpose.archived_at column missing (DM-DDL-10)")
	}

	rows, err := d.Query(`
		SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid='consent_purpose'::regclass AND contype='u'`)
	if err != nil {
		t.Fatalf("query unique constraints: %v", err)
	}
	defer rows.Close()
	var defs []string
	for rows.Next() {
		var def string
		if err := rows.Scan(&def); err != nil {
			t.Fatalf("scan constraint def: %v", err)
		}
		defs = append(defs, def)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate constraint defs: %v", err)
	}
	foundComposite := false
	for _, def := range defs {
		if strings.Contains(def, "workspace_id") && strings.Contains(def, "key") {
			foundComposite = true
		}
		if def == "UNIQUE (key)" {
			t.Errorf("consent_purpose still has a bare UNIQUE(key) constraint: %s", def)
		}
	}
	if !foundComposite {
		t.Errorf("consent_purpose missing UNIQUE(workspace_id, key); constraints found: %v", defs)
	}

	// A brand-new workspace created through the real signup handler gets its
	// own 4 consent_purpose rows (D2 step 7).
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	body := fmt.Sprintf(
		`{"name":"Signup Test","slug":"signup-test-%s","base_currency":"USD","admin_email":"admin-%s@t.test","admin_password":"pw","admin_display_name":"Admin"}`,
		nonce, nonce,
	)
	req := httptest.NewRequest(http.MethodPost, "/workspaces", strings.NewReader(body))
	w := httptest.NewRecorder()
	identitytransport.HandleCreateWorkspace(d).ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("HandleCreateWorkspace: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var n int
	if err := d.QueryRow(`SELECT count(*) FROM consent_purpose WHERE workspace_id=$1`, resp.ID).Scan(&n); err != nil {
		t.Fatalf("count new workspace purposes: %v", err)
	}
	if n != 4 {
		t.Errorf("new workspace %s: consent_purpose count = %d, want 4", resp.ID, n)
	}
}
