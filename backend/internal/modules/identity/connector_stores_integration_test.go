//go:build integration

package crmauth_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"

	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	"github.com/gradionhq/margince/backend/internal/platform/keyvault"
)

func testProvider(t *testing.T) *keyvault.LocalProvider {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	p, err := keyvault.NewLocalProvider(key)
	if err != nil {
		t.Fatalf("NewLocalProvider: %v", err)
	}
	return p
}

// newWorkspaceAuth mirrors crm-core's newWorkspaceSQL test helper: inserts a
// fresh workspace row and returns its id, so each test is workspace-isolated.
func newWorkspaceAuth(t *testing.T, d *sql.DB) string {
	t.Helper()
	var id string
	if err := d.QueryRow(
		`INSERT INTO workspace(name, slug, base_currency) VALUES ('test-ws', gen_random_uuid()::text, 'EUR') RETURNING id`,
	).Scan(&id); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	return id
}

func sqlDBAuth(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("postgres", mustDSN(t))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// TestConnectorSecretIntegration_PutLookupRoundTrip proves AC2: a token
// persists as a connector_secret row (ciphertext + kms_key_id + rotated_at)
// and Lookup round-trips the plaintext through the provider.
func TestConnectorSecretIntegration_PutLookupRoundTrip(t *testing.T) {
	d := sqlDBAuth(t)
	ctx := context.Background()
	ws := newWorkspaceAuth(t, d)
	setWorkspaceGUC(t, d, ws)

	conns := crmauth.NewIncumbentConnectionStore(d)
	conn, err := conns.Create(ctx, ws, "hubspot", []string{"contacts.read", "contacts.write"})
	if err != nil {
		t.Fatalf("Create connection: %v", err)
	}

	secrets := crmauth.NewConnectorSecretStore(d, testProvider(t))
	rec, err := secrets.Put(ctx, ws, conn.ID, []byte("refresh-token-xyz"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if len(rec.Ciphertext) == 0 || rec.KMSKeyID == "" {
		t.Fatalf("Put returned empty ciphertext/kms_key_id")
	}

	got, err := secrets.Lookup(ctx, ws, conn.ID)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if string(got) != "refresh-token-xyz" {
		t.Fatalf("Lookup = %q, want %q", got, "refresh-token-xyz")
	}
}

// TestConnectorSecretIntegration_RotateAppendsAndLatestWins proves Rotate
// appends a new row (not an in-place overwrite) and Lookup always resolves the
// latest by rotated_at.
func TestConnectorSecretIntegration_RotateAppendsAndLatestWins(t *testing.T) {
	d := sqlDBAuth(t)
	ctx := context.Background()
	ws := newWorkspaceAuth(t, d)
	setWorkspaceGUC(t, d, ws)

	conns := crmauth.NewIncumbentConnectionStore(d)
	conn, err := conns.Create(ctx, ws, "hubspot", []string{"contacts.read"})
	if err != nil {
		t.Fatalf("Create connection: %v", err)
	}
	secrets := crmauth.NewConnectorSecretStore(d, testProvider(t))
	if _, err := secrets.Put(ctx, ws, conn.ID, []byte("token-v1")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := secrets.Rotate(ctx, ws, conn.ID, []byte("token-v2")); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	var count int
	if err := d.QueryRow(
		`SELECT count(*) FROM connector_secret WHERE workspace_id=$1::uuid AND connection_id=$2::uuid`,
		ws, conn.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("want 2 rows after Put+Rotate (append, not overwrite), got %d", count)
	}

	got, err := secrets.Lookup(ctx, ws, conn.ID)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if string(got) != "token-v2" {
		t.Fatalf("Lookup after rotate = %q, want latest %q", got, "token-v2")
	}
}

// TestConnectorSecretIntegration_RevokeFailsClosed proves AC3: after Revoke,
// the next Lookup fails rather than silently returning the stale token.
func TestConnectorSecretIntegration_RevokeFailsClosed(t *testing.T) {
	d := sqlDBAuth(t)
	ctx := context.Background()
	ws := newWorkspaceAuth(t, d)
	setWorkspaceGUC(t, d, ws)

	conns := crmauth.NewIncumbentConnectionStore(d)
	conn, err := conns.Create(ctx, ws, "hubspot", []string{"contacts.read"})
	if err != nil {
		t.Fatalf("Create connection: %v", err)
	}
	secrets := crmauth.NewConnectorSecretStore(d, testProvider(t))
	if _, err := secrets.Put(ctx, ws, conn.ID, []byte("token-v1")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := conns.Revoke(ctx, conn.ID, ws); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// Second revoke must fail loudly, not silently no-op past ErrNotFound.
	if err := conns.Revoke(ctx, conn.ID, ws); err == nil {
		t.Fatalf("second Revoke on an already-revoked connection must fail")
	}

	if _, err := secrets.Lookup(ctx, ws, conn.ID); err == nil {
		t.Fatalf("Lookup after revoke must fail closed, got nil error")
	}
}

// TestConnectorSecretIntegration_RotateFailsClosedOnRevokedConnection proves
// Rotate is guarded the same way Lookup is: rotating a revoked connection's
// token must fail rather than silently appending a dead secret row nobody can
// ever read back out.
func TestConnectorSecretIntegration_RotateFailsClosedOnRevokedConnection(t *testing.T) {
	d := sqlDBAuth(t)
	ctx := context.Background()
	ws := newWorkspaceAuth(t, d)
	setWorkspaceGUC(t, d, ws)

	conns := crmauth.NewIncumbentConnectionStore(d)
	conn, err := conns.Create(ctx, ws, "hubspot", []string{"contacts.read"})
	if err != nil {
		t.Fatalf("Create connection: %v", err)
	}
	secrets := crmauth.NewConnectorSecretStore(d, testProvider(t))
	if _, err := secrets.Put(ctx, ws, conn.ID, []byte("token-v1")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := conns.Revoke(ctx, conn.ID, ws); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if _, err := secrets.Rotate(ctx, ws, conn.ID, []byte("token-v2")); err == nil {
		t.Fatalf("Rotate against a revoked connection must fail closed, got nil error")
	}

	var count int
	if err := d.QueryRow(
		`SELECT count(*) FROM connector_secret WHERE workspace_id=$1::uuid AND connection_id=$2::uuid`,
		ws, conn.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("Rotate against a revoked connection must not append a row, got %d rows (want 1, the original Put)", count)
	}
}

// TestIncumbentConnectionIntegration_RLSDenyOnUnset proves the tenant-isolation
// policy denies reads with no app.workspace_id GUC set (fail-closed default).
// Uses SET LOCAL ROLE margince_app — the non-superuser role RLS is actually
// enforced against (the margince owner/superuser bypasses RLS per Postgres rules).
func TestIncumbentConnectionIntegration_RLSDenyOnUnset(t *testing.T) {
	d := sqlDBAuth(t)
	ctx := context.Background()
	ws := newWorkspaceAuth(t, d)
	setWorkspaceGUC(t, d, ws)

	conns := crmauth.NewIncumbentConnectionStore(d)
	if _, err := conns.Create(ctx, ws, "hubspot", []string{"contacts.read"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Use a fresh connection pool (never had the workspace GUC set) and switch
	// to the non-superuser app role inside a transaction — then confirm 0 rows
	// are visible (policy USING clause: nullif('', '')::uuid = NULL → deny).
	fresh := sqlDBAuth(t)
	tx, err := fresh.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		t.Fatalf("set role margince_app: %v", err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM incumbent_connection`).Scan(&count); err != nil {
		t.Fatalf("count with no GUC as margince_app: %v", err)
	}
	if count != 0 {
		t.Fatalf("RLS must deny all rows with app.workspace_id unset, got count=%d", count)
	}
}
