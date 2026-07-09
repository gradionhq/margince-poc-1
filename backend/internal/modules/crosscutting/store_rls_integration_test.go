//go:build integration

// Store-layer RLS conformance (data-model §1.3) — the regression gate for the
// audit's #1 finding: the core CRUD stores (PersonStore/DealStore/…) must run their
// SQL inside a tx that assumes the non-superuser margince_app role AND sets
// app.workspace_id, so FORCE RLS is actually enforced — not bypassed by leaning on the
// bare superuser pool + the app-layer workspace_id predicate alone.
//
// These tests drive the REAL store API (not raw SQL) and FAIL against the pre-fix
// store code, PASS after withWorkspace is wired in.
package crosscutting_test

import (
	"context"
	"errors"
	"testing"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	people "github.com/gradionhq/margince/backend/internal/modules/people"
	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// appCtx builds a principal-bearing context so store-layer audit writes attribute
// to a human in the workspace (EntryFromPrincipal needs UserID+TenantID).
func appCtx(ws string) context.Context {
	return crmctx.With(context.Background(),
		crmctx.Principal{UserID: "human:store-rls-test", TenantID: ws})
}

func TestStoreRLSCrossWorkspaceReadEmpty(t *testing.T) {
	d := sqlDB(t)
	wsA := newWorkspaceSQL(t, d)
	wsB := newWorkspaceSQL(t, d)

	personStore := people.NewPersonStore(d)

	pA := people.NewPerson("Alice-RLS", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	pA.WorkspaceID = wsA
	created, err := personStore.Create(appCtx(wsA), pA, nil)
	if err != nil {
		t.Fatalf("create person in A: %v", err)
	}

	if _, err := personStore.Get(appCtx(wsB), created.ID, wsB); !errors.Is(err, apperrors.ErrNotFound) {
		t.Fatalf("cross-workspace Get from B: want ErrNotFound, got %v", err)
	}

	gotB, _, err := personStore.List(appCtx(wsB), wsB, "", 50, "")
	if err != nil {
		t.Fatalf("list in B: %v", err)
	}
	if len(gotB) != 0 {
		t.Fatalf("workspace B must see 0 people from A, got %d", len(gotB))
	}

	gotA, _, err := personStore.List(appCtx(wsA), wsA, "", 50, "")
	if err != nil {
		t.Fatalf("list in A: %v", err)
	}
	if len(gotA) != 1 {
		t.Fatalf("workspace A must see exactly its 1 person, got %d", len(gotA))
	}
}

func TestStoreRLSCrossWorkspaceWriteDenied(t *testing.T) {
	d := sqlDB(t)
	wsA := newWorkspaceSQL(t, d)
	wsB := newWorkspaceSQL(t, d)

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

	dealStore := deals.NewDealStore(d)

	dB := deals.NewDeal("CrossTenantDeal", pipeID, stageID, prov.Provenance{Source: "api", CapturedBy: "human:test"})
	dB.WorkspaceID = wsB
	if _, err := dealStore.Create(appCtx(wsB), dB, "", nil); err == nil {
		t.Fatal("cross-workspace deal create referencing another tenant's pipeline/stage must be denied under RLS")
	}
}
