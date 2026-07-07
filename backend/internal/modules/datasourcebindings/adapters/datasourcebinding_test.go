package adapters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// ---------------------------------------------------------------------------
// Compile assertion
// ---------------------------------------------------------------------------

var _ datasource.Provider = (*adapters.DatasourceProvider)(nil)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakePersonStore struct {
	persons map[string]domain.Person
	lastWS  string
}

func newFakePersonStore() *fakePersonStore {
	return &fakePersonStore{persons: map[string]domain.Person{}}
}

func (f *fakePersonStore) Create(ctx context.Context, p domain.Person, emails []domain.PersonEmailInput) (domain.Person, error) {
	f.lastWS = p.WorkspaceID
	p.ID = "person-1"
	f.persons[p.ID] = p
	return p, nil
}

func (f *fakePersonStore) Get(ctx context.Context, id, workspaceID string) (domain.Person, error) {
	f.lastWS = workspaceID
	p, ok := f.persons[id]
	if !ok {
		return domain.Person{}, errs.ErrNotFound
	}
	return p, nil
}

func (f *fakePersonStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Person, error) {
	f.lastWS = workspaceID
	p, ok := f.persons[id]
	if !ok {
		return domain.Person{}, errs.ErrNotFound
	}
	if fn, ok := updates["full_name"].(string); ok {
		p.FullName = fn
	}
	f.persons[id] = p
	return p, nil
}

func (f *fakePersonStore) List(ctx context.Context, workspaceID, cursor string, limit int, sort string) ([]domain.Person, string, error) {
	f.lastWS = workspaceID
	var out []domain.Person
	for _, p := range f.persons {
		out = append(out, p)
	}
	return out, "", nil
}

type fakePersonStoreVersionSkew struct {
	fakePersonStore
}

func (f *fakePersonStoreVersionSkew) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Person, error) {
	return domain.Person{}, errs.ErrVersionSkew
}

type fakeOrgStore struct{}

func (f *fakeOrgStore) Create(ctx context.Context, o domain.Organization) (domain.Organization, error) {
	o.ID = "org-1"
	return o, nil
}

func (f *fakeOrgStore) Get(ctx context.Context, id, workspaceID string) (domain.Organization, error) {
	return domain.Organization{}, errs.ErrNotFound
}

func (f *fakeOrgStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Organization, error) {
	return domain.Organization{}, errs.ErrNotFound
}

func (f *fakeOrgStore) List(ctx context.Context, workspaceID, cursor string, limit int, sort string, filter domain.OrgListFilter) ([]domain.Organization, string, error) {
	return nil, "", nil
}

type fakeDealStore struct{}

func (f *fakeDealStore) Create(ctx context.Context, d domain.Deal, idempotencyKey string) (domain.Deal, error) {
	d.ID = "deal-1"
	return d, nil
}

func (f *fakeDealStore) Get(ctx context.Context, id, workspaceID string) (domain.Deal, error) {
	return domain.Deal{}, errs.ErrNotFound
}

func (f *fakeDealStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Deal, error) {
	return domain.Deal{ID: id, WorkspaceID: workspaceID}, nil
}

func (f *fakeDealStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]domain.Deal, string, error) {
	return nil, "", nil
}

type fakeActivityStore struct{}

func (f *fakeActivityStore) Create(ctx context.Context, a domain.Activity) (domain.Activity, error) {
	a.ID = "activity-1"
	return a, nil
}

func (f *fakeActivityStore) Get(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	return domain.Activity{}, errs.ErrNotFound
}

func (f *fakeActivityStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Activity, error) {
	return domain.Activity{}, errs.ErrNotFound
}

func (f *fakeActivityStore) List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]domain.Activity, string, error) {
	return nil, "", nil
}

type fakeLeadStore struct{}

func (f *fakeLeadStore) Create(ctx context.Context, l domain.Lead) (domain.Lead, error) {
	l.ID = "lead-1"
	return l, nil
}

func (f *fakeLeadStore) Get(ctx context.Context, id, workspaceID string) (domain.Lead, error) {
	return domain.Lead{}, errs.ErrNotFound
}

func (f *fakeLeadStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Lead, error) {
	return domain.Lead{}, errs.ErrNotFound
}

func (f *fakeLeadStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]domain.Lead, string, error) {
	return nil, "", nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestProvider(persons *fakePersonStore) *adapters.DatasourceProvider {
	return adapters.NewDatasourceProvider("ws-test", persons, &fakeOrgStore{}, &fakeDealStore{}, &fakeActivityStore{}, &fakeLeadStore{})
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestDatasourceRoundTrip: Create → Read → Update → Read.
func TestDatasourceRoundTrip(t *testing.T) {
	ps := newFakePersonStore()
	p := newTestProvider(ps)
	ctx := context.Background()

	// Create
	ref, err := p.Create(ctx, datasource.CreateInput{
		Type:       datasource.EntityPerson,
		Fields:     map[string]any{"full_name": "Alice"},
		Source:     "api",
		CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ref.Type != datasource.EntityPerson {
		t.Errorf("got type %q want %q", ref.Type, datasource.EntityPerson)
	}

	// workspaceID threaded
	if ps.lastWS != "ws-test" {
		t.Errorf("workspaceID not threaded: got %q", ps.lastWS)
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
	if person.FullName != "Alice" {
		t.Errorf("FullName: got %q want Alice", person.FullName)
	}

	// Update
	ref2, err := p.Update(ctx, datasource.UpdateInput{
		Type:       datasource.EntityPerson,
		ID:         ref.ID,
		Patch:      map[string]any{"full_name": "Bob"},
		Source:     "api",
		CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if ref2.ID != ref.ID {
		t.Errorf("ref mismatch: %q vs %q", ref2.ID, ref.ID)
	}

	// Read again
	rec2, err := p.Read(ctx, datasource.EntityRef{Type: datasource.EntityPerson, ID: ref.ID})
	if err != nil {
		t.Fatalf("Read2: %v", err)
	}
	person2, _ := rec2.(domain.Person)
	if person2.FullName != "Bob" {
		t.Errorf("after update FullName: got %q want Bob", person2.FullName)
	}
}

// TestDatasourceNullProvenance: Create/Update with missing provenance → ErrNullProvenance.
func TestDatasourceNullProvenance(t *testing.T) {
	p := newTestProvider(newFakePersonStore())
	ctx := context.Background()

	_, err := p.Create(ctx, datasource.CreateInput{
		Type:   datasource.EntityPerson,
		Fields: map[string]any{"full_name": "X"},
		Source: "", // missing
	})
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Errorf("Create empty Source: got %v, want ErrNullProvenance", err)
	}

	_, err = p.Create(ctx, datasource.CreateInput{
		Type:       datasource.EntityPerson,
		Fields:     map[string]any{"full_name": "X"},
		Source:     "api",
		CapturedBy: "", // missing
	})
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Errorf("Create empty CapturedBy: got %v, want ErrNullProvenance", err)
	}

	_, err = p.Update(ctx, datasource.UpdateInput{
		Type:       datasource.EntityPerson,
		ID:         "x",
		Source:     "", // missing
		CapturedBy: "human:test",
	})
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Errorf("Update empty Source: got %v, want ErrNullProvenance", err)
	}
}

// TestDatasourceVersionSkew: store returning ErrVersionSkew is propagated.
func TestDatasourceVersionSkew(t *testing.T) {
	skewStore := &fakePersonStoreVersionSkew{fakePersonStore: *newFakePersonStore()}
	// pre-populate so Get works
	skewStore.persons["p1"] = domain.Person{ID: "p1", WorkspaceID: "ws-test", FullName: "Test"}
	p := adapters.NewDatasourceProvider("ws-test", skewStore, &fakeOrgStore{}, &fakeDealStore{}, &fakeActivityStore{}, &fakeLeadStore{})

	v := "1"
	_, err := p.Update(context.Background(), datasource.UpdateInput{
		Type:       datasource.EntityPerson,
		ID:         "p1",
		Patch:      map[string]any{"full_name": "Y"},
		Source:     "api",
		CapturedBy: "human:test",
		IfVersion:  &v,
	})
	if !errors.Is(err, datasource.ErrVersionSkew) {
		t.Errorf("expected ErrVersionSkew, got %v", err)
	}
}

// TestDatasourceFreshness: Freshness returns Authoritative=true.
func TestDatasourceFreshness(t *testing.T) {
	p := newTestProvider(newFakePersonStore())
	fi, err := p.Freshness(context.Background(), datasource.EntityRef{Type: datasource.EntityPerson, ID: "any"})
	if err != nil {
		t.Fatalf("Freshness: %v", err)
	}
	if !fi.Authoritative {
		t.Error("expected Authoritative=true")
	}
}

// TestDatasourceAllMethodsCallable: all 9 methods callable without panic.
func TestDatasourceAllMethodsCallable(t *testing.T) {
	ps := newFakePersonStore()
	p := newTestProvider(ps)
	ctx := context.Background()

	_, _ = p.Read(ctx, datasource.EntityRef{Type: datasource.EntityPerson, ID: "none"})
	_, _ = p.Search(ctx, datasource.SearchQuery{Type: datasource.EntityPerson, Limit: 5})
	objs, err := p.ListObjects(ctx)
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(objs) == 0 {
		t.Error("ListObjects returned empty")
	}
	fields, err := p.ListFields(ctx, datasource.EntityPerson)
	if err != nil {
		t.Fatalf("ListFields: %v", err)
	}
	if len(fields) == 0 {
		t.Error("ListFields returned empty for person")
	}
	_, _ = p.RunReport(ctx, datasource.ReportPlan{Name: "test"})
	_, _ = p.AdvanceDeal(ctx, datasource.AdvanceDealInput{DealID: "d1", ToStatus: "won"})
	_, _ = p.Freshness(ctx, datasource.EntityRef{Type: datasource.EntityPerson, ID: "any"})
}
