package crmcore_test

import (
	"context"
	"errors"
	"testing"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// ---------------------------------------------------------------------------
// Compile assertion
// ---------------------------------------------------------------------------

var _ datasource.Provider = (*crmcore.DatasourceProvider)(nil)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakePersonStore struct {
	persons map[string]crmcore.Person
	lastWS  string
}

func newFakePersonStore() *fakePersonStore {
	return &fakePersonStore{persons: map[string]crmcore.Person{}}
}

func (f *fakePersonStore) Create(ctx context.Context, p crmcore.Person) (crmcore.Person, error) {
	f.lastWS = p.WorkspaceID
	p.ID = "person-1"
	f.persons[p.ID] = p
	return p, nil
}

func (f *fakePersonStore) Get(ctx context.Context, id, workspaceID string) (crmcore.Person, error) {
	f.lastWS = workspaceID
	p, ok := f.persons[id]
	if !ok {
		return crmcore.Person{}, errs.ErrNotFound
	}
	return p, nil
}

func (f *fakePersonStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (crmcore.Person, error) {
	f.lastWS = workspaceID
	p, ok := f.persons[id]
	if !ok {
		return crmcore.Person{}, errs.ErrNotFound
	}
	if fn, ok := updates["full_name"].(string); ok {
		p.FullName = fn
	}
	f.persons[id] = p
	return p, nil
}

func (f *fakePersonStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]crmcore.Person, string, error) {
	f.lastWS = workspaceID
	var out []crmcore.Person
	for _, p := range f.persons {
		out = append(out, p)
	}
	return out, "", nil
}

type fakePersonStoreVersionSkew struct {
	fakePersonStore
}

func (f *fakePersonStoreVersionSkew) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (crmcore.Person, error) {
	return crmcore.Person{}, errs.ErrVersionSkew
}

type fakeOrgStore struct{}

func (f *fakeOrgStore) Create(ctx context.Context, o crmcore.Organization) (crmcore.Organization, error) {
	o.ID = "org-1"
	return o, nil
}

func (f *fakeOrgStore) Get(ctx context.Context, id, workspaceID string) (crmcore.Organization, error) {
	return crmcore.Organization{}, errs.ErrNotFound
}

func (f *fakeOrgStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (crmcore.Organization, error) {
	return crmcore.Organization{}, errs.ErrNotFound
}

func (f *fakeOrgStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]crmcore.Organization, string, error) {
	return nil, "", nil
}

type fakeDealStore struct{}

func (f *fakeDealStore) Create(ctx context.Context, d crmcore.Deal) (crmcore.Deal, error) {
	d.ID = "deal-1"
	return d, nil
}

func (f *fakeDealStore) Get(ctx context.Context, id, workspaceID string) (crmcore.Deal, error) {
	return crmcore.Deal{}, errs.ErrNotFound
}

func (f *fakeDealStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (crmcore.Deal, error) {
	return crmcore.Deal{ID: id, WorkspaceID: workspaceID}, nil
}

func (f *fakeDealStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]crmcore.Deal, string, error) {
	return nil, "", nil
}

type fakeActivityStore struct{}

func (f *fakeActivityStore) Create(ctx context.Context, a crmcore.Activity) (crmcore.Activity, error) {
	a.ID = "activity-1"
	return a, nil
}

func (f *fakeActivityStore) Get(ctx context.Context, id, workspaceID string) (crmcore.Activity, error) {
	return crmcore.Activity{}, errs.ErrNotFound
}

func (f *fakeActivityStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (crmcore.Activity, error) {
	return crmcore.Activity{}, errs.ErrNotFound
}

func (f *fakeActivityStore) List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]crmcore.Activity, string, error) {
	return nil, "", nil
}

type fakeLeadStore struct{}

func (f *fakeLeadStore) Create(ctx context.Context, l crmcore.Lead) (crmcore.Lead, error) {
	l.ID = "lead-1"
	return l, nil
}

func (f *fakeLeadStore) Get(ctx context.Context, id, workspaceID string) (crmcore.Lead, error) {
	return crmcore.Lead{}, errs.ErrNotFound
}

func (f *fakeLeadStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (crmcore.Lead, error) {
	return crmcore.Lead{}, errs.ErrNotFound
}

func (f *fakeLeadStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]crmcore.Lead, string, error) {
	return nil, "", nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// personStoreI mirrors the unexported personStore interface in datasourcebinding.go so
// test fakes can be passed to newTestProvider without referencing the exported name.
type personStoreI interface {
	Create(ctx context.Context, p crmcore.Person) (crmcore.Person, error)
	Get(ctx context.Context, id, workspaceID string) (crmcore.Person, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (crmcore.Person, error)
	List(ctx context.Context, workspaceID, cursor string, limit int) ([]crmcore.Person, string, error)
}

func newTestProvider(persons personStoreI) *crmcore.DatasourceProvider {
	return crmcore.NewDatasourceProvider("ws-test", persons, &fakeOrgStore{}, &fakeDealStore{}, &fakeActivityStore{}, &fakeLeadStore{})
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
	person, ok := rec.(crmcore.Person)
	if !ok {
		t.Fatalf("expected Person, got %T", rec)
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
	person2, _ := rec2.(crmcore.Person)
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
	skewStore.persons["p1"] = crmcore.Person{ID: "p1", WorkspaceID: "ws-test", FullName: "Test"}
	p := newTestProvider(skewStore)

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

	// populate for AdvanceDeal
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
