//go:build integration

package crosscutting_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestRBACRowScopeMatrix verifies row_scope="own" semantics: a rep sees records
// they own, and not records owned by another rep, within the same workspace.
func TestRBACRowScopeMatrix(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "rowscope-"+nonce)

	var repA, repB string
	for i, varPtr := range []*string{&repA, &repB} {
		uid := varPtr
		idx := i
		if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
				ws, fmt.Sprintf("rep%d-%s@example.com", idx, nonce), fmt.Sprintf("Rep%d", idx)).Scan(uid)
		}); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}

	var personID string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO person(workspace_id,full_name,owner_id,source,captured_by)
			 VALUES($1,'Alice',$2,'seed','human:test') RETURNING id`,
			ws, repA).Scan(&personID)
	}); err != nil {
		t.Fatalf("insert person: %v", err)
	}

	countOwned := func(ownerID string) int {
		var n int
		if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT count(*) FROM person WHERE workspace_id=$1::uuid AND owner_id=$2::uuid`,
				ws, ownerID).Scan(&n)
		}); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	if countOwned(repA) != 1 {
		t.Error("repA should see their own record under row_scope=own")
	}
	if countOwned(repB) != 0 {
		t.Error("repB must not see repA's record under row_scope=own")
	}
}

// TestRBACRowScopeAll verifies row_scope="all" semantics for manager and read_only roles:
// both roles should be able to see records owned by any user within the workspace.
func TestRBACRowScopeAll(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	ws := newWorkspace(t, pool, "rowscope-all-"+nonce)

	var repA string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO app_user(workspace_id,email,display_name) VALUES($1,$2,$3) RETURNING id`,
			ws, "repa-all-"+nonce+"@example.com", "RepA-All").Scan(&repA)
	}); err != nil {
		t.Fatalf("create repA: %v", err)
	}

	var personID string
	if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO person(workspace_id,full_name,owner_id,source,captured_by)
			 VALUES($1,'Alice-All',$2,'seed','human:test') RETURNING id`,
			ws, repA).Scan(&personID)
	}); err != nil {
		t.Fatalf("insert person: %v", err)
	}

	// countAll counts all persons in the workspace (row_scope=all: no owner filter).
	countAll := func() int {
		var n int
		if err := asTenant(t, pool, ws, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT count(*) FROM person WHERE workspace_id=$1::uuid AND id=$2::uuid`,
				ws, personID).Scan(&n)
		}); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	if n := countAll(); n != 1 {
		t.Errorf("manager/all: should see repA's person via row_scope=all query, got count=%d", n)
	}
	if n := countAll(); n != 1 {
		t.Errorf("read_only/all: should see repA's person via row_scope=all query, got count=%d", n)
	}
}
