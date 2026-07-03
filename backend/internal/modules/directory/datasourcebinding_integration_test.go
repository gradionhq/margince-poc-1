//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

func mustSQLDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newWorkspaceSQL(t *testing.T, db *sql.DB) string {
	t.Helper()
	nonce := fmt.Sprintf("datasource-%d", time.Now().UnixNano())
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO workspace(name, slug, base_currency) VALUES ($1, $2, 'EUR') RETURNING id`,
		"ws-"+nonce, "slug-"+nonce).Scan(&id)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return id
}

func TestDatasourceIntegrationRoundTrip(t *testing.T) {
	db := mustSQLDB(t)
	wsID := newWorkspaceSQL(t, db)
	ctx := context.Background()

	persons := crmcore.NewPersonStore(db)
	orgs := crmcore.NewOrgStore(db)
	deals := crmcore.NewDealStore(db)
	activities := crmcore.NewActivityStore(db)
	leads := crmcore.NewLeadStore(db)

	p := crmcore.NewDatasourceProvider(wsID, persons, orgs, deals, activities, leads)

	// Create
	ref, err := p.Create(ctx, datasource.CreateInput{
		Type:       datasource.EntityPerson,
		Fields:     map[string]any{"full_name": "Alice Integration"},
		Source:     "api",
		CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Read back
	rec, err := p.Read(ctx, datasource.EntityRef{Type: datasource.EntityPerson, ID: ref.ID})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	person, ok := rec.(crmcore.Person)
	if !ok {
		t.Fatalf("expected Person, got %T", rec)
	}
	if person.FullName != "Alice Integration" {
		t.Errorf("FullName: got %q want %q", person.FullName, "Alice Integration")
	}
	if person.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %q want %q", person.WorkspaceID, wsID)
	}

	// Update
	_, err = p.Update(ctx, datasource.UpdateInput{
		Type:       datasource.EntityPerson,
		ID:         ref.ID,
		Patch:      map[string]any{"full_name": "Bob Integration"},
		Source:     "api",
		CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Read again
	rec2, err := p.Read(ctx, datasource.EntityRef{Type: datasource.EntityPerson, ID: ref.ID})
	if err != nil {
		t.Fatalf("Read2: %v", err)
	}
	person2, _ := rec2.(crmcore.Person)
	if person2.FullName != "Bob Integration" {
		t.Errorf("after update FullName: got %q want %q", person2.FullName, "Bob Integration")
	}
}

func TestDatasourceIntegrationAdvanceDeal(t *testing.T) {
	db := mustSQLDB(t)
	wsID := newWorkspaceSQL(t, db)
	ctx := context.Background()

	// Create a pipeline and a stage so deal FKs are valid.
	pipelines := crmcore.NewPipelineStore(db)
	stages := crmcore.NewStageStore(db)

	pl, err := pipelines.Create(ctx, crmcore.Pipeline{
		WorkspaceID: wsID,
		Name:        "test-pipeline",
		IsDefault:   false,
		Position:    1,
	})
	if err != nil {
		t.Fatalf("create pipeline: %v", err)
	}

	st, err := stages.Create(ctx, crmcore.Stage{
		WorkspaceID:    wsID,
		PipelineID:     pl.ID,
		Name:           "Prospecting",
		Position:       1,
		Semantic:       "open",
		WinProbability: 10,
	})
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	p := crmcore.NewDatasourceProvider(
		wsID,
		crmcore.NewPersonStore(db),
		crmcore.NewOrgStore(db),
		crmcore.NewDealStore(db),
		crmcore.NewActivityStore(db),
		crmcore.NewLeadStore(db),
	)

	// 1. Create a deal via the Datasource seam.
	ref, err := p.Create(ctx, datasource.CreateInput{
		Type: datasource.EntityDeal,
		Fields: map[string]any{
			"name":        "Integration Deal",
			"pipeline_id": pl.ID,
			"stage_id":    st.ID,
		},
		Source:     "api",
		CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("Create deal: %v", err)
	}

	// 2. Read back and assert initial status is "open".
	rec, err := p.Read(ctx, datasource.EntityRef{Type: datasource.EntityDeal, ID: ref.ID})
	if err != nil {
		t.Fatalf("Read deal (initial): %v", err)
	}
	deal, ok := rec.(crmcore.Deal)
	if !ok {
		t.Fatalf("expected Deal, got %T", rec)
	}
	if deal.Status != "open" {
		t.Errorf("initial Status: got %q want %q", deal.Status, "open")
	}

	// 3. Advance the deal status to "won" via the Datasource seam.
	_, err = p.AdvanceDeal(ctx, datasource.AdvanceDealInput{DealID: ref.ID, ToStatus: "won"})
	if err != nil {
		t.Fatalf("AdvanceDeal: %v", err)
	}

	// 4. Read back and assert the persisted status is "won" — proves the real DB write.
	rec2, err := p.Read(ctx, datasource.EntityRef{Type: datasource.EntityDeal, ID: ref.ID})
	if err != nil {
		t.Fatalf("Read deal (after advance): %v", err)
	}
	deal2, ok := rec2.(crmcore.Deal)
	if !ok {
		t.Fatalf("expected Deal after advance, got %T", rec2)
	}
	if deal2.Status != "won" {
		t.Errorf("after AdvanceDeal Status: got %q want %q", deal2.Status, "won")
	}
}

func TestDatasourceIntegrationNullProvenance(t *testing.T) {
	db := mustSQLDB(t)
	wsID := newWorkspaceSQL(t, db)
	ctx := context.Background()

	p := crmcore.NewDatasourceProvider(
		wsID,
		crmcore.NewPersonStore(db),
		crmcore.NewOrgStore(db),
		crmcore.NewDealStore(db),
		crmcore.NewActivityStore(db),
		crmcore.NewLeadStore(db),
	)

	_, err := p.Create(ctx, datasource.CreateInput{
		Type:   datasource.EntityPerson,
		Fields: map[string]any{"full_name": "NoProvenance"},
		Source: "", // missing
	})
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Errorf("expected ErrNullProvenance, got %v", err)
	}
}
