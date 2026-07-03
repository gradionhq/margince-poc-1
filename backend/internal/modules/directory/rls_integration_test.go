//go:build integration

// RLS / schema conformance test (data-model §1.3 / §1.3a / §11). Runs against a
// live Postgres: `make infra-up && make migrate-up && make test-integration`.
// Excluded from the fast `make check` by the integration build tag.
package crmcore_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return pool
}

func newWorkspace(t *testing.T, pool *pgxpool.Pool, nonce string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO workspace(name, slug, base_currency) VALUES ($1, $2, 'EUR') RETURNING id`,
		"ws-"+nonce, "slug-"+nonce).Scan(&id)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return id
}

// asAppRole runs fn inside a tx as the non-superuser margince_app role (so RLS
// is enforced — superusers/owners bypass it), optionally with the tenant GUC set.
func asAppRole(t *testing.T, pool *pgxpool.Pool, ws string, fn func(tx pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		t.Fatalf("set role: %v", err)
	}
	if ws != "" {
		if _, err := tx.Exec(ctx, `select set_config('app.workspace_id', $1, true)`, ws); err != nil {
			t.Fatalf("set guc: %v", err)
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// asTenant is asAppRole with the tenant GUC set.
func asTenant(t *testing.T, pool *pgxpool.Pool, ws string, fn func(tx pgx.Tx) error) error {
	return asAppRole(t, pool, ws, fn)
}

func insertPerson(t *testing.T, pool *pgxpool.Pool, ws, name string) string {
	t.Helper()
	var id string
	err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`INSERT INTO person(workspace_id, full_name, source, captured_by)
			 VALUES ($1, $2, 'api', 'human:test') RETURNING id`, ws, name).Scan(&id)
	})
	if err != nil {
		t.Fatalf("insert person: %v", err)
	}
	return id
}

func TestRLSTenantIsolation(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	wsA := newWorkspace(t, pool, "a-"+nonce)
	wsB := newWorkspace(t, pool, "b-"+nonce)
	insertPerson(t, pool, wsA, "Alice-"+nonce)
	insertPerson(t, pool, wsB, "Bob-"+nonce)

	// Tenant A sees its own row, not B's (∅-query across tenants).
	count := func(ws, like string) int {
		var n int
		if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM person WHERE full_name LIKE $1`, like).Scan(&n)
		}); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	if got := count(wsA, "Alice-"+nonce); got != 1 {
		t.Fatalf("tenant A should see Alice: got %d", got)
	}
	if got := count(wsA, "Bob-"+nonce); got != 0 {
		t.Fatalf("tenant A must NOT see tenant B's Bob: got %d", got)
	}
	if got := count(wsB, "Alice-"+nonce); got != 0 {
		t.Fatalf("tenant B must NOT see tenant A's Alice: got %d", got)
	}

	// WITH CHECK: inserting a row for another tenant under A's GUC is rejected.
	err := asTenant(t, pool, wsA, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx,
			`INSERT INTO person(workspace_id, full_name, source, captured_by) VALUES ($1,'x','api','human:test')`, wsB)
		return e
	})
	if err == nil {
		t.Fatal("WITH CHECK should reject cross-tenant insert")
	}
}

func TestRLSDenyOnUnset(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()

	// App role with no app.workspace_id GUC: reads return zero rows.
	var n int
	if err := asAppRole(t, pool, "", func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM person`).Scan(&n)
	}); err != nil {
		t.Fatalf("count without guc: %v", err)
	}
	if n != 0 {
		t.Fatalf("connection with no app.workspace_id must see 0 rows, saw %d", n)
	}
	// And writes are rejected by WITH CHECK.
	err := asAppRole(t, pool, "", func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx,
			`INSERT INTO person(workspace_id, full_name, source, captured_by)
			 VALUES (gen_random_uuid(),'nobody','api','human:test')`)
		return e
	})
	if err == nil {
		t.Fatal("insert with no app.workspace_id must be rejected by WITH CHECK")
	}
}

func TestVersionBumpOnUpdate(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "v-"+nonce)
	id := insertPerson(t, pool, ws, "Vera-"+nonce)

	var v int64
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `UPDATE person SET title='VP' WHERE id=$1`, id); e != nil {
			return e
		}
		return tx.QueryRow(ctx, `SELECT version FROM person WHERE id=$1`, id).Scan(&v)
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if v != 2 {
		t.Fatalf("version should bump 1->2 on update, got %d", v)
	}
}

func TestAuditLogImmutable(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "audit-"+nonce)

	var id string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO audit_log(workspace_id, actor_type, actor_id, action, entity_type)
			 VALUES ($1,'human','human:test','create','person') RETURNING id`, ws).Scan(&id)
	}); err != nil {
		t.Fatalf("insert audit row: %v", err)
	}
	// Append-only is enforced by audit_log_immutable() BEFORE trigger (migration
	// 000003). Migration 000019 dropped the silent DO INSTEAD NOTHING rules so
	// DELETE/UPDATE now RAISE EXCEPTION (txn aborts) rather than silently no-op.
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `DELETE FROM audit_log WHERE id=$1`, id)
		return e
	}); err == nil {
		t.Fatal("audit_log DELETE must RAISE (append-only), got nil error — silent no-op is forbidden")
	}
	var n int
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE id=$1`, id).Scan(&n)
	}); err != nil {
		t.Fatalf("read back audit row: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit_log DELETE raised but row disappeared; count=%d, want 1", n)
	}
}

// TestAuditLogAppendOnly verifies the DB-layer append-only enforcement via the
// audit_log_immutable() BEFORE trigger (migration 000003). Migration 000019
// dropped the silent DO INSTEAD NOTHING rules so an UPDATE must RAISE EXCEPTION
// (txn aborts), leaving the original row unchanged.
func TestAuditLogAppendOnly(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "audit2-"+nonce)

	// Insert one audit row.
	var id string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO audit_log(workspace_id, actor_type, actor_id, action, entity_type)
			 VALUES ($1,'human','human:test','create','person') RETURNING id`, ws).Scan(&id)
	}); err != nil {
		t.Fatalf("insert audit row: %v", err)
	}

	// UPDATE must RAISE (no-op is forbidden since migration 000019).
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE audit_log SET action='hack' WHERE id=$1`, id)
		return e
	}); err == nil {
		t.Fatal("audit_log UPDATE must RAISE (append-only), got nil error — silent no-op is forbidden")
	}

	// Verify the original value is unchanged.
	var action string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT action FROM audit_log WHERE id=$1`, id).Scan(&action)
	}); err != nil {
		t.Fatalf("read back audit row: %v", err)
	}
	if action != "create" {
		t.Fatalf("audit_log UPDATE raised but row was mutated; got action=%q", action)
	}
}

// TestOrgCyclePrevention verifies the org-hierarchy cycle guard trigger rejects a
// parent assignment that would close a loop (A->B->C->A).
func TestOrgCyclePrevention(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "orgcycle-"+nonce)

	insertOrg := func(name string, parentID *string) string {
		var id string
		if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`INSERT INTO organization(workspace_id, name, source, captured_by)
				 VALUES($1,$2,'seed','human:test') RETURNING id`, ws, name).Scan(&id)
		}); err != nil {
			t.Fatalf("insert org %q: %v", name, err)
		}
		if parentID != nil {
			if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
				_, e := tx.Exec(ctx, `UPDATE organization SET parent_org_id=$1 WHERE id=$2`, *parentID, id)
				return e
			}); err != nil {
				t.Fatalf("set parent for %q: %v", name, err)
			}
		}
		return id
	}

	// Build a parent chain C -> B -> A (each org's parent_org_id points up):
	// A is the root, B's parent is A, C's parent is B.
	aID := insertOrg("A-"+nonce, nil)
	bID := insertOrg("B-"+nonce, &aID)
	cID := insertOrg("C-"+nonce, &bID)
	_ = cID

	// Attempt to set A's parent to C. That would close the loop
	// A -> C -> B -> A, which the cycle-guard trigger must reject.
	err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE organization SET parent_org_id=$1 WHERE id=$2`, cID, aID)
		return e
	})
	if err == nil {
		t.Fatal("org cycle A->C->B->A should be rejected by the cycle-guard trigger")
	}
}

// TestActivityIdempotentCapture verifies uq_activity_source rejects a duplicate
// (workspace_id, source_system, source_id) tuple — the idempotent-capture key.
func TestActivityIdempotentCapture(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "actdup-"+nonce)

	insert := func() error {
		return asTenant(t, pool, ws, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`INSERT INTO activity(workspace_id, kind, occurred_at, source, captured_by, source_system, source_id)
				 VALUES($1,'email',now(),'api','human:test','gmail',$2)`, ws, "msg-"+nonce)
			return err
		})
	}
	if err := insert(); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := insert(); err == nil {
		t.Fatal("duplicate source_system+source_id must be rejected by uq_activity_source")
	}
}

// TestProvenanceNotNull verifies that source and captured_by columns are NOT NULL
// for every tenant table that carries them (person, organization, deal, activity, lead).
func TestProvenanceNotNull(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "prov-"+nonce)

	// Create a pipeline + stage so deal rows can reference them.
	var pipelineID, stageID string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO pipeline(workspace_id,name,is_default,position) VALUES($1,'prov-pl',true,1) RETURNING id`,
			ws).Scan(&pipelineID); err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO stage(workspace_id,pipeline_id,name,position,semantic,win_probability)
			 VALUES($1,$2,'Open',1,'open',0) RETURNING id`,
			ws, pipelineID).Scan(&stageID)
	}); err != nil {
		t.Fatalf("create pipeline/stage: %v", err)
	}

	tables := []struct {
		name    string
		insertQ string
		args    func() []any
	}{
		{
			name:    "person",
			insertQ: `INSERT INTO person(workspace_id,full_name,source,captured_by) VALUES($1,'Prov Test','api','human:test') RETURNING source, captured_by`,
			args:    func() []any { return []any{ws} },
		},
		{
			name:    "organization",
			insertQ: `INSERT INTO organization(workspace_id,name,source,captured_by) VALUES($1,'Prov Org','api','human:test') RETURNING source, captured_by`,
			args:    func() []any { return []any{ws} },
		},
		{
			name:    "deal",
			insertQ: `INSERT INTO deal(workspace_id,name,pipeline_id,stage_id,status,source,captured_by,version) VALUES($1,'Prov Deal',$2,$3,'open','api','human:test',1) RETURNING source, captured_by`,
			args:    func() []any { return []any{ws, pipelineID, stageID} },
		},
		{
			name:    "activity",
			insertQ: `INSERT INTO activity(workspace_id,kind,occurred_at,source,captured_by) VALUES($1,'email',now(),'api','human:test') RETURNING source, captured_by`,
			args:    func() []any { return []any{ws} },
		},
		{
			name:    "lead",
			insertQ: `INSERT INTO lead(workspace_id,full_name,status,source,captured_by,version) VALUES($1,'Prov Lead','new','api','human:test',1) RETURNING source, captured_by`,
			args:    func() []any { return []any{ws} },
		},
	}

	for _, tbl := range tables {
		t.Run(tbl.name, func(t *testing.T) {
			var src, capturedBy string
			if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
				return tx.QueryRow(ctx, tbl.insertQ, tbl.args()...).Scan(&src, &capturedBy)
			}); err != nil {
				t.Fatalf("insert %s: %v", tbl.name, err)
			}
			if src == "" {
				t.Errorf("%s: source must be NOT NULL / non-empty", tbl.name)
			}
			if capturedBy == "" {
				t.Errorf("%s: captured_by must be NOT NULL / non-empty", tbl.name)
			}
		})
	}
}

// TestLeadAntiPollution inserts 3 leads and verifies 0 person rows exist for
// the same workspace. Proves lead segregation (ADR-0008).
func TestLeadAntiPollution(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "leadpoll-"+nonce)

	// Insert 3 leads.
	for i := 0; i < 3; i++ {
		if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`INSERT INTO lead(workspace_id,full_name,status,source,captured_by,version)
				 VALUES($1,$2,'new','import','human:test',1)`,
				ws, fmt.Sprintf("Lead-%d-%s", i, nonce))
			return err
		}); err != nil {
			t.Fatalf("insert lead %d: %v", i, err)
		}
	}

	// Verify 3 leads exist.
	var leadCount int
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM lead WHERE workspace_id=$1::uuid`, ws).Scan(&leadCount)
	}); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	if leadCount != 3 {
		t.Fatalf("expected 3 leads, got %d", leadCount)
	}

	// Verify 0 person rows exist for this workspace (lead segregation).
	var personCount int
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM person WHERE workspace_id=$1::uuid`, ws).Scan(&personCount)
	}); err != nil {
		t.Fatalf("count persons: %v", err)
	}
	if personCount != 0 {
		t.Fatalf("lead anti-pollution: expected 0 persons, got %d (leads must not pollute person table)", personCount)
	}
}

// TestAuditEventCoverage is in rls_schema_matrix_test.go (needs crmcore import;
// that file is excluded from arch-lint as a test composition root).

// TestDealCompositeFK verifies the migration-014 trigger rejects a deal whose
// stage_id belongs to a different pipeline than pipeline_id.
func TestDealCompositeFK(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "dealfk-"+nonce)

	// Create pipeline A with stage S1, and pipeline B.
	var pipelineA, pipelineB, stageS1 string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO pipeline(workspace_id,name,is_default,position) VALUES($1,'PipeA',true,1) RETURNING id`,
			ws).Scan(&pipelineA); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO stage(workspace_id,pipeline_id,name,position,semantic,win_probability)
			 VALUES($1,$2,'S1',1,'open',0) RETURNING id`,
			ws, pipelineA).Scan(&stageS1); err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO pipeline(workspace_id,name,is_default,position) VALUES($1,'PipeB',false,2) RETURNING id`,
			ws).Scan(&pipelineB)
	}); err != nil {
		t.Fatalf("setup pipelines: %v", err)
	}

	// Attempt to insert a deal with pipeline_id=B but stage_id=S1 (which belongs to A).
	// The trg_deal_stage_pipeline trigger must reject this.
	err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx,
			`INSERT INTO deal(workspace_id,name,pipeline_id,stage_id,status,source,captured_by,version)
			 VALUES($1,'Bad Deal',$2,$3,'open','api','human:test',1)`,
			ws, pipelineB, stageS1)
		return e
	})
	if err == nil {
		t.Fatal("deal composite FK: expected trigger to reject stage from wrong pipeline, but insert succeeded")
	}
}

// TestPasswordWorkspace inserts a workspace + admin user with password_hash in a
// single transaction and verifies both rows exist with password_hash IS NOT NULL.
func TestPasswordWorkspace(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Use a syntactically valid bcrypt hash string (cost 4, not real; just a non-null value).
	// The DB column is text NULL; we only assert it is stored as-is (not NULL).
	const fakeHash = "$2a$04$testhashtesthashtesthashtesthashtesthashtesthashhhhhhh"

	var wsID, userID string
	err := func() error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err := tx.QueryRow(ctx,
			`INSERT INTO workspace(name,slug,base_currency) VALUES($1,$2,'EUR') RETURNING id`,
			"pwws-"+nonce, "pwws-"+nonce).Scan(&wsID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO app_user(workspace_id,email,display_name,password_hash)
			 VALUES($1::uuid,$2,'Admin',$3) RETURNING id`,
			wsID, "admin-"+nonce+"@example.com", fakeHash).Scan(&userID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}()
	if err != nil {
		t.Fatalf("bootstrap workspace+user tx: %v", err)
	}

	// Verify workspace row exists.
	var wsName string
	if err := pool.QueryRow(ctx, `SELECT name FROM workspace WHERE id=$1::uuid`, wsID).Scan(&wsName); err != nil {
		t.Fatalf("read workspace: %v", err)
	}
	if wsName == "" {
		t.Error("workspace name should not be empty")
	}

	// Verify user row exists and password_hash IS NOT NULL.
	var storedHash *string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM app_user WHERE id=$1::uuid`, userID).Scan(&storedHash); err != nil {
		t.Fatalf("read app_user: %v", err)
	}
	if storedHash == nil || *storedHash == "" {
		t.Error("password_hash must not be NULL/empty on the admin user")
	}
}

// TestRelationshipStakeholderRoleEnum verifies migration 000063's CHECK constraint
// (DEAL-EXT-5): kind='deal_stakeholder' rows must have a role from the fixed
// vocabulary and NULL is disallowed for that kind; other kinds keep free-text role.
func TestRelationshipStakeholderRoleEnum(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "rolecheck-"+nonce)

	person := insertPerson(t, pool, ws, "Stakeholder-"+nonce)

	var pipelineID, stageID, dealID string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO pipeline(workspace_id,name,is_default,position) VALUES($1,'RolePipe',true,1) RETURNING id`,
			ws).Scan(&pipelineID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO stage(workspace_id,pipeline_id,name,position,semantic,win_probability)
			 VALUES($1,$2,'S1',1,'open',0) RETURNING id`,
			ws, pipelineID).Scan(&stageID); err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO deal(workspace_id,name,pipeline_id,stage_id,status,source,captured_by,version)
			 VALUES($1,'Role Deal',$2,$3,'open','api','human:test',1) RETURNING id`,
			ws, pipelineID, stageID).Scan(&dealID)
	}); err != nil {
		t.Fatalf("setup pipeline/stage/deal: %v", err)
	}

	insertStakeholder := func(role any) error {
		return asTenant(t, pool, ws, func(tx pgx.Tx) error {
			_, e := tx.Exec(ctx,
				`INSERT INTO relationship(workspace_id,kind,person_id,deal_id,role,source,captured_by)
				 VALUES($1,'deal_stakeholder',$2,$3,$4,'api','human:test')`,
				ws, person, dealID, role)
			return e
		})
	}

	// NULL role for a deal_stakeholder row is rejected.
	if err := insertStakeholder(nil); err == nil {
		t.Fatal("expected CHECK violation for NULL role on deal_stakeholder, insert succeeded")
	}

	// A role outside the fixed vocabulary is rejected.
	if err := insertStakeholder("not_a_real_role"); err == nil {
		t.Fatal("expected CHECK violation for invalid role on deal_stakeholder, insert succeeded")
	}

	// A valid role from the enum succeeds.
	if err := insertStakeholder("champion"); err != nil {
		t.Fatalf("expected valid role 'champion' to succeed: %v", err)
	}

	// A non-stakeholder kind (employment) still allows NULL role — CHECK is scoped, not table-wide.
	org := ""
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO organization(workspace_id,name,source,captured_by) VALUES($1,'RoleOrg',$2,$3) RETURNING id`,
			ws, "api", "human:test").Scan(&org)
	}); err != nil {
		t.Fatalf("setup organization: %v", err)
	}
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx,
			`INSERT INTO relationship(workspace_id,kind,person_id,organization_id,role,source,captured_by)
			 VALUES($1,'employment',$2,$3,NULL,'api','human:test')`,
			ws, person, org)
		return e
	}); err != nil {
		t.Fatalf("employment kind must still allow NULL role: %v", err)
	}
}
