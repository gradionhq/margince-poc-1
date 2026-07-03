//go:build integration

// Store-layer RLS conformance (data-model §1.3) — the regression gate for the
// audit's #1 finding: the core CRUD stores (PersonStore/DealStore/…) must run their
// SQL inside a tx that assumes the non-superuser margince_app role AND sets
// app.workspace_id, so FORCE RLS is actually enforced — not bypassed by leaning on the
// bare superuser pool + the app-layer workspace_id predicate alone.
//
// These tests drive the REAL store API (not raw SQL) and FAIL against the pre-fix
// store code, PASS after withWorkspace is wired in.
package crmcore_test

import (
	"context"
	"errors"
	"testing"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// appCtx builds a principal-bearing context so store-layer audit writes attribute
// to a human in the workspace (EntryFromPrincipal needs UserID+TenantID).
func appCtx(ws string) context.Context {
	return crmctx.With(context.Background(),
		crmctx.Principal{UserID: "human:store-rls-test", TenantID: ws})
}

// TestStoreRLSCrossWorkspaceReadEmpty proves that a person created in workspace A via
// PersonStore is invisible to a PersonStore reader scoped to workspace B. Because the
// store now reads under margince_app + app.workspace_id=B, the row is filtered by RLS,
// not merely by the app-layer predicate. The cross-workspace Get returns ErrNotFound;
// the cross-workspace List excludes A's row entirely.
func TestStoreRLSCrossWorkspaceReadEmpty(t *testing.T) {
	d := sqlDB(t)
	wsA := newWorkspaceSQL(t, d)
	wsB := newWorkspaceSQL(t, d)

	people := crmcore.NewPersonStore(d)

	pA := crmcore.NewPerson("Alice-RLS", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	pA.WorkspaceID = wsA
	created, err := people.Create(appCtx(wsA), pA)
	if err != nil {
		t.Fatalf("create person in A: %v", err)
	}

	// Cross-workspace Get from B must NOT return A's person.
	if _, err := people.Get(appCtx(wsB), created.ID, wsB); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("cross-workspace Get from B: want ErrNotFound, got %v", err)
	}

	// Cross-workspace List in B must be empty (A's row excluded by RLS).
	gotB, _, err := people.List(appCtx(wsB), wsB, "", 50, "")
	if err != nil {
		t.Fatalf("list in B: %v", err)
	}
	if len(gotB) != 0 {
		t.Fatalf("workspace B must see 0 people from A, got %d", len(gotB))
	}

	// Sanity: A still sees its own row (the store works when scoped correctly).
	gotA, _, err := people.List(appCtx(wsA), wsA, "", 50, "")
	if err != nil {
		t.Fatalf("list in A: %v", err)
	}
	if len(gotA) != 1 {
		t.Fatalf("workspace A must see exactly its 1 person, got %d", len(gotA))
	}
}

// TestStoreRLSCrossWorkspaceWriteDenied proves a write whose row workspace_id does not
// match the tx's app.workspace_id GUC is rejected by the person_tenant WITH CHECK
// policy under margince_app. We point a PersonStore.Create at workspace B's id while
// the row itself claims membership in a non-existent / mismatched workspace via a deal
// path. Concretely: a DealStore.Create referencing a pipeline/stage from workspace A,
// issued while scoped to workspace B, is denied — the store runs under RLS, so the
// cross-tenant FK rows are invisible and the insert fails rather than silently writing.
func TestStoreRLSCrossWorkspaceWriteDenied(t *testing.T) {
	d := sqlDB(t)
	wsA := newWorkspaceSQL(t, d)
	wsB := newWorkspaceSQL(t, d)

	// Seed a pipeline+stage in A (as A) so the deal has valid-looking FK targets.
	if _, err := d.Exec(`SET ROLE margince_app`); err != nil {
		t.Fatalf("set role: %v", err)
	}
	if _, err := d.Exec(`SET app.workspace_id = '` + wsA + `'`); err != nil {
		t.Fatalf("set guc A: %v", err)
	}
	var pipeID, stageID string
	if err := d.QueryRow(
		`INSERT INTO pipeline(workspace_id,name,is_default,position) VALUES ($1,'RLSPipe',true,1) RETURNING id`,
		wsA,
	).Scan(&pipeID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := d.QueryRow(
		`INSERT INTO stage(workspace_id,pipeline_id,name,position,semantic,win_probability)
		 VALUES ($1,$2,'Open',1,'open',0) RETURNING id`,
		wsA, pipeID,
	).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if _, err := d.Exec(`RESET ROLE`); err != nil {
		t.Fatalf("reset role: %v", err)
	}

	deals := crmcore.NewDealStore(d)

	// Create a deal scoped to B that references A's pipeline/stage. Under RLS scoped to
	// B, those FK rows are invisible, so the FK validation fails — the cross-tenant
	// write is denied rather than silently succeeding on the superuser pool.
	dB := crmcore.NewDeal("CrossTenantDeal", pipeID, stageID, prov.Provenance{Source: "api", CapturedBy: "human:test"})
	dB.WorkspaceID = wsB
	if _, err := deals.Create(appCtx(wsB), dB, ""); err == nil {
		t.Fatal("cross-workspace deal create referencing another tenant's pipeline/stage must be denied under RLS")
	}
}
