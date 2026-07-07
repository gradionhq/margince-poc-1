//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/domain"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// ---------------------------------------------------------------------------
// Minimal SQL-backed stores implementing datasourcebindings/ports interfaces.
// These use datasourcebindings/domain types directly (not the entity modules'
// PersonStore etc., which use different domain types).
// ---------------------------------------------------------------------------

// intPersonStore is a minimal SQL-backed person store for integration tests.
type intPersonStore struct{ db *sql.DB }

func (s *intPersonStore) Create(ctx context.Context, p domain.Person, emails []domain.PersonEmailInput) (domain.Person, error) {
	if p.ID == "" {
		p.ID = ids.New()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO person(id, workspace_id, full_name, source, captured_by, version)
		 VALUES($1::uuid, $2::uuid, $3, $4, $5, 1)`,
		p.ID, p.WorkspaceID, p.FullName, p.Source, p.CapturedBy)
	return p, err
}

func (s *intPersonStore) Get(ctx context.Context, id, workspaceID string) (domain.Person, error) {
	var p domain.Person
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, workspace_id::text, full_name, source, captured_by
		 FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		id, workspaceID).Scan(&p.ID, &p.WorkspaceID, &p.FullName, &p.Source, &p.CapturedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Person{}, errs.ErrNotFound
	}
	return p, err
}

func (s *intPersonStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, _ int64) (domain.Person, error) {
	if fn, ok := updates["full_name"].(string); ok {
		_, err := s.db.ExecContext(ctx,
			`UPDATE person SET full_name=$1 WHERE id=$2::uuid AND workspace_id=$3::uuid`,
			fn, id, workspaceID)
		if err != nil {
			return domain.Person{}, err
		}
	}
	return s.Get(ctx, id, workspaceID)
}

func (s *intPersonStore) List(ctx context.Context, workspaceID, _ string, limit int, _ string) ([]domain.Person, string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id::text, workspace_id::text, full_name, source, captured_by
		 FROM person WHERE workspace_id=$1::uuid LIMIT $2`,
		workspaceID, limit)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []domain.Person
	for rows.Next() {
		var p domain.Person
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.FullName, &p.Source, &p.CapturedBy); err != nil {
			return nil, "", err
		}
		out = append(out, p)
	}
	return out, "", rows.Err()
}

// intOrgStore is a minimal SQL-backed org store for integration tests.
type intOrgStore struct{ db *sql.DB }

func (s *intOrgStore) Create(ctx context.Context, o domain.Organization) (domain.Organization, error) {
	if o.ID == "" {
		o.ID = ids.New()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO organization(id, workspace_id, name, source, captured_by)
		 VALUES($1::uuid, $2::uuid, $3, $4, $5)`,
		o.ID, o.WorkspaceID, o.DisplayName, o.Source, o.CapturedBy)
	return o, err
}

func (s *intOrgStore) Get(ctx context.Context, id, workspaceID string) (domain.Organization, error) {
	var o domain.Organization
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, workspace_id::text, name, source, captured_by
		 FROM organization WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		id, workspaceID).Scan(&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Source, &o.CapturedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Organization{}, errs.ErrNotFound
	}
	return o, err
}

func (s *intOrgStore) Update(ctx context.Context, id, workspaceID string, _ map[string]any, _ int64) (domain.Organization, error) {
	return s.Get(ctx, id, workspaceID)
}

func (s *intOrgStore) List(ctx context.Context, workspaceID, _ string, _ int, _ string, _ domain.OrgListFilter) ([]domain.Organization, string, error) {
	return nil, "", nil
}

// intDealStore is a minimal SQL-backed deal store for integration tests.
type intDealStore struct{ db *sql.DB }

func (s *intDealStore) Create(ctx context.Context, d domain.Deal, _ string) (domain.Deal, error) {
	if d.ID == "" {
		d.ID = ids.New()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO deal(id, workspace_id, name, pipeline_id, stage_id, status, source, captured_by, version)
		 VALUES($1::uuid, $2::uuid, $3, $4::uuid, $5::uuid, $6, $7, $8, 1)`,
		d.ID, d.WorkspaceID, d.Name, d.PipelineID, d.StageID, d.Status, d.Source, d.CapturedBy)
	return d, err
}

func (s *intDealStore) Get(ctx context.Context, id, workspaceID string) (domain.Deal, error) {
	var d domain.Deal
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, workspace_id::text, name, pipeline_id::text, stage_id::text, status, source, captured_by
		 FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		id, workspaceID).Scan(&d.ID, &d.WorkspaceID, &d.Name, &d.PipelineID, &d.StageID, &d.Status, &d.Source, &d.CapturedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Deal{}, errs.ErrNotFound
	}
	return d, err
}

func (s *intDealStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, _ int64) (domain.Deal, error) {
	if status, ok := updates["status"].(string); ok {
		_, err := s.db.ExecContext(ctx,
			`UPDATE deal SET status=$1 WHERE id=$2::uuid AND workspace_id=$3::uuid`,
			status, id, workspaceID)
		if err != nil {
			return domain.Deal{}, err
		}
	}
	return s.Get(ctx, id, workspaceID)
}

func (s *intDealStore) List(ctx context.Context, workspaceID, _ string, _ int) ([]domain.Deal, string, error) {
	return nil, "", nil
}

// intActivityStore is a minimal SQL-backed activity store for integration tests.
type intActivityStore struct{ db *sql.DB }

func (s *intActivityStore) Create(ctx context.Context, a domain.Activity) (domain.Activity, error) {
	if a.ID == "" {
		a.ID = ids.New()
	}
	if a.OccurredAt.IsZero() {
		a.OccurredAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO activity(id, workspace_id, kind, occurred_at, source, captured_by)
		 VALUES($1::uuid, $2::uuid, $3, $4, $5, $6)`,
		a.ID, a.WorkspaceID, a.Kind, a.OccurredAt, a.Source, a.CapturedBy)
	return a, err
}

func (s *intActivityStore) Get(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	var a domain.Activity
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, workspace_id::text, kind, occurred_at, source, captured_by
		 FROM activity WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		id, workspaceID).Scan(&a.ID, &a.WorkspaceID, &a.Kind, &a.OccurredAt, &a.Source, &a.CapturedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Activity{}, errs.ErrNotFound
	}
	return a, err
}

func (s *intActivityStore) Update(ctx context.Context, id, workspaceID string, _ map[string]any, _ int64) (domain.Activity, error) {
	return s.Get(ctx, id, workspaceID)
}

func (s *intActivityStore) List(ctx context.Context, workspaceID, _, _, _ string, _ int) ([]domain.Activity, string, error) {
	return nil, "", nil
}

// intLeadStore is a minimal SQL-backed lead store for integration tests.
type intLeadStore struct{ db *sql.DB }

func (s *intLeadStore) Create(ctx context.Context, l domain.Lead) (domain.Lead, error) {
	if l.ID == "" {
		l.ID = ids.New()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO lead(workspace_id, full_name, status, source, captured_by, version)
		 VALUES($1::uuid, 'test-lead', $2, $3, $4, 1) RETURNING id::text`,
		l.WorkspaceID, l.Status, l.Source, l.CapturedBy)
	return l, err
}

func (s *intLeadStore) Get(ctx context.Context, id, workspaceID string) (domain.Lead, error) {
	var l domain.Lead
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, workspace_id::text, status, source, captured_by FROM lead WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		id, workspaceID).Scan(&l.ID, &l.WorkspaceID, &l.Status, &l.Source, &l.CapturedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Lead{}, errs.ErrNotFound
	}
	return l, err
}

func (s *intLeadStore) Update(ctx context.Context, id, workspaceID string, _ map[string]any, _ int64) (domain.Lead, error) {
	return s.Get(ctx, id, workspaceID)
}

func (s *intLeadStore) List(ctx context.Context, workspaceID, _ string, _ int) ([]domain.Lead, string, error) {
	return nil, "", nil
}

// ---------------------------------------------------------------------------
// Helper: build a DatasourceProvider backed by the integration test stores.
// ---------------------------------------------------------------------------

func newIntProvider(t *testing.T, db *sql.DB, wsID string) *adapters.DatasourceProvider {
	t.Helper()
	return adapters.NewDatasourceProvider(
		wsID,
		&intPersonStore{db: db},
		&intOrgStore{db: db},
		&intDealStore{db: db},
		&intActivityStore{db: db},
		&intLeadStore{db: db},
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDatasourceIntegrationRoundTrip(t *testing.T) {
	db := mustSQLDB(t)
	wsID := newWorkspaceSQL(t, db)
	ctx := context.Background()

	// Set RLS GUC so RLS-enabled tables are writable.
	if _, err := db.ExecContext(ctx, "SET app.workspace_id = '"+wsID+"'"); err != nil {
		t.Fatalf("set rls guc: %v", err)
	}

	p := newIntProvider(t, db, wsID)

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
	person, ok := rec.(domain.Person)
	if !ok {
		t.Fatalf("expected domain.Person, got %T", rec)
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
	person2, _ := rec2.(domain.Person)
	if person2.FullName != "Bob Integration" {
		t.Errorf("after update FullName: got %q want %q", person2.FullName, "Bob Integration")
	}
}

func TestDatasourceIntegrationAdvanceDeal(t *testing.T) {
	db := mustSQLDB(t)
	wsID := newWorkspaceSQL(t, db)
	ctx := context.Background()

	// Set RLS GUC.
	if _, err := db.ExecContext(ctx, "SET app.workspace_id = '"+wsID+"'"); err != nil {
		t.Fatalf("set rls guc: %v", err)
	}

	// Create a pipeline and a stage so deal FKs are valid.
	pipelines := deals.NewPipelineStore(db)
	stages := deals.NewStageStore(db)

	pl, err := pipelines.Create(ctx, deals.Pipeline{
		WorkspaceID: wsID,
		Name:        "test-pipeline",
		IsDefault:   false,
		Position:    1,
	})
	if err != nil {
		t.Fatalf("create pipeline: %v", err)
	}

	st, err := stages.Create(ctx, deals.Stage{
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

	p := newIntProvider(t, db, wsID)

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
	deal, ok := rec.(domain.Deal)
	if !ok {
		t.Fatalf("expected domain.Deal, got %T", rec)
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
	deal2, ok := rec2.(domain.Deal)
	if !ok {
		t.Fatalf("expected domain.Deal after advance, got %T", rec2)
	}
	if deal2.Status != "won" {
		t.Errorf("after AdvanceDeal Status: got %q want %q", deal2.Status, "won")
	}
}

func TestDatasourceIntegrationNullProvenance(t *testing.T) {
	db := mustSQLDB(t)
	wsID := newWorkspaceSQL(t, db)
	ctx := context.Background()

	p := newIntProvider(t, db, wsID)

	_, err := p.Create(ctx, datasource.CreateInput{
		Type:   datasource.EntityPerson,
		Fields: map[string]any{"full_name": "NoProvenance"},
		Source: "", // missing
	})
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Errorf("expected ErrNullProvenance, got %v", err)
	}
}
