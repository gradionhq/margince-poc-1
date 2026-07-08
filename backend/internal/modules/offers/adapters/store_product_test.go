//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/lib/pq" // registers the postgres driver for database/sql

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func seedProductWorkspace(t *testing.T, db *sql.DB) string {
	t.Helper()
	wsID := ids.New()
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t-op-t04-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, wsID, "t-op-t04-ws-"+ids.New()); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, wsID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	return wsID
}

func strPtrOP(s string) *string { return &s }

func TestProductStore_CreateGetUpdateArchive_RoundTrip(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewProductStore(db)

	p := domain.NewProduct("Consulting Day", provTest())
	p.WorkspaceID = wsID
	p.UnitPriceMinor = 150000
	p.Currency = "EUR"

	created, err := s.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.UnitPriceMinor != 150000 {
		t.Fatalf("expected unit_price_minor=150000, got %d", created.UnitPriceMinor)
	}

	got, err := s.Get(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UnitPriceMinor != 150000 {
		t.Fatalf("get: expected unit_price_minor=150000 (int64 round-trip, OFFER-AC-9a), got %d", got.UnitPriceMinor)
	}

	updated, err := s.Update(context.Background(), created.ID, wsID, map[string]any{"name": "Consulting Day (Updated)"}, created.Version)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Consulting Day (Updated)" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}
	if updated.Version != created.Version+1 {
		t.Fatalf("expected version bump to %d, got %d", created.Version+1, updated.Version)
	}

	archived, err := s.Archive(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("expected archived_at set")
	}

	if _, err := s.Get(context.Background(), created.ID, wsID); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for archived row's default Get, got %v", err)
	}
}

func TestProductStore_Create_DuplicateSKU_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewProductStore(db)

	base := domain.NewProduct("A", provTest())
	base.WorkspaceID = wsID
	base.UnitPriceMinor = 100
	base.Currency = "EUR"
	base.SKU = strPtrOP("SKU-1")
	if _, err := s.Create(context.Background(), base); err != nil {
		t.Fatalf("first create: %v", err)
	}

	dup := domain.NewProduct("B", provTest())
	dup.WorkspaceID = wsID
	dup.UnitPriceMinor = 200
	dup.Currency = "EUR"
	dup.SKU = strPtrOP("SKU-1")
	_, err := s.Create(context.Background(), dup)
	var dupErr *adapters.ErrDuplicateSKU
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected ErrDuplicateSKU, got %v", err)
	}
}

func TestProductStore_Create_NullSKU_NeverCollides(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewProductStore(db)

	for i := 0; i < 2; i++ {
		p := domain.NewProduct("No SKU", provTest())
		p.WorkspaceID = wsID
		p.UnitPriceMinor = 100
		p.Currency = "EUR"
		if _, err := s.Create(context.Background(), p); err != nil {
			t.Fatalf("create %d: expected no collision for sku=nil, got %v", i, err)
		}
	}
}

func TestProductStore_Create_MissingProvenance_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewProductStore(db)

	p := domain.Product{WorkspaceID: wsID, Name: "X", UnitPriceMinor: 1, Currency: "EUR"}
	_, err := s.Create(context.Background(), p)
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Fatalf("expected ErrNullProvenance, got %v", err)
	}
}

func TestProductStore_List_EmptyCatalogue_ReturnsEmptyPage(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewProductStore(db)

	items, next, err := s.List(context.Background(), wsID, "", 20, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("expected empty (non-nil) slice, got %+v", items)
	}
	if next != "" {
		t.Fatalf("expected no next cursor, got %q", next)
	}
}

func TestProductStore_Update_VersionSkew_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewProductStore(db)

	p := domain.NewProduct("A", provTest())
	p.WorkspaceID = wsID
	p.UnitPriceMinor = 1
	p.Currency = "EUR"
	created, err := s.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = s.Update(context.Background(), created.ID, wsID, map[string]any{"name": "B"}, created.Version+99)
	if !errors.Is(err, errs.ErrVersionSkew) {
		t.Fatalf("expected ErrVersionSkew, got %v", err)
	}
}

func provTest() prov.Provenance { return prov.Provenance{Source: "test", CapturedBy: "human:test"} }
