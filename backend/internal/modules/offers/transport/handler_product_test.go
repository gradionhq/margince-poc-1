package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const testWorkspaceID = "00000000-0000-0000-0000-000000000001"

func withWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: testWorkspaceID, UserID: "human:test"})
	return r.WithContext(ctx)
}

type fakeProductStore struct {
	products map[string]domain.Product
	nextErr  error
}

func newFakeProductStore() *fakeProductStore {
	return &fakeProductStore{products: make(map[string]domain.Product)}
}

func (f *fakeProductStore) Create(ctx context.Context, p domain.Product) (domain.Product, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Product{}, err
	}
	p.ID = "prod-1"
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	p.Version = 1
	f.products[p.ID] = p
	return p, nil
}

func (f *fakeProductStore) Get(ctx context.Context, id, workspaceID string) (domain.Product, error) {
	if p, ok := f.products[id]; ok {
		return p, nil
	}
	return domain.Product{}, errs.ErrNotFound
}

func (f *fakeProductStore) List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]domain.Product, string, error) {
	var out []domain.Product
	for _, p := range f.products {
		if !includeArchived && p.ArchivedAt != nil {
			continue
		}
		out = append(out, p)
	}
	return out, "", nil
}

func (f *fakeProductStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Product, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Product{}, err
	}
	if p, ok := f.products[id]; ok {
		if ifMatch != 0 && p.Version != ifMatch {
			return domain.Product{}, errs.ErrVersionSkew
		}
		if name, ok := updates["name"].(string); ok {
			p.Name = name
		}
		p.Version++
		p.UpdatedAt = time.Now()
		f.products[id] = p
		return p, nil
	}
	return domain.Product{}, errs.ErrNotFound
}

func (f *fakeProductStore) Archive(ctx context.Context, id, workspaceID string) (domain.Product, error) {
	if p, ok := f.products[id]; ok {
		now := time.Now()
		p.ArchivedAt = &now
		f.products[id] = p
		return p, nil
	}
	return domain.Product{}, errs.ErrNotFound
}

func TestProductHandler_Create_Valid_Returns201(t *testing.T) {
	store := newFakeProductStore()
	h := NewProductHandler(store)

	body := map[string]any{
		"name":               "Consulting Day",
		"unit_price_minor":   150000,
		"currency":           "EUR",
		"source":             "test",
		"captured_by":        "human:test",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc == "" {
		t.Fatal("expected Location header")
	}
	var respBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if unitPrice, ok := respBody["unit_price_minor"].(float64); !ok || int64(unitPrice) != 150000 {
		t.Fatalf("expected unit_price_minor=150000 as exact int64, got %v", respBody["unit_price_minor"])
	}
}

func TestProductHandler_Create_MissingProvenance_Returns422(t *testing.T) {
	store := newFakeProductStore()
	h := NewProductHandler(store)

	body := map[string]any{
		"name":             "Consulting Day",
		"unit_price_minor": 150000,
		"currency":         "EUR",
		// source and captured_by missing
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
}

func TestProductHandler_Create_DuplicateSKU_Returns409(t *testing.T) {
	store := newFakeProductStore()
	store.nextErr = &adapters.ErrDuplicateSKU{ExistingID: "existing-id", Field: "sku"}
	h := NewProductHandler(store)

	body := map[string]any{
		"name":             "Consulting Day",
		"sku":              "SKU-1",
		"unit_price_minor": 150000,
		"currency":         "EUR",
		"source":           "test",
		"captured_by":      "human:test",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", w.Code, w.Body.String())
	}
	var respBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if code, ok := respBody["code"].(string); !ok || code != "product_sku_duplicate" {
		t.Fatalf("expected code=product_sku_duplicate, got %v", respBody["code"])
	}
	if details, ok := respBody["details"].(map[string]any); !ok || details["existing_id"] != "existing-id" {
		t.Fatalf("expected details.existing_id=existing-id, got %v", respBody["details"])
	}
}

func TestProductHandler_List_Empty_Returns200(t *testing.T) {
	store := newFakeProductStore()
	h := NewProductHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var respBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if data, ok := respBody["data"]; ok && data != nil {
		if items, ok := data.([]any); !ok || len(items) != 0 {
			t.Fatalf("expected empty data array, got %v", respBody["data"])
		}
	}
}

func TestProductHandler_Update_VersionSkew_Returns409(t *testing.T) {
	store := newFakeProductStore()
	p := domain.NewProduct("Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = testWorkspaceID
	p.Currency = "EUR"
	p.UnitPriceMinor = 100
	store.products[p.ID] = p
	h := NewProductHandler(store)

	req := httptest.NewRequest(http.MethodPut, "/products/"+p.ID, bytes.NewReader([]byte(`{"name":"Updated"}`)))
	req.Header.Set("If-Match", "999")
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", w.Code, w.Body.String())
	}
	var respBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if code, ok := respBody["code"].(string); !ok || code != "version_skew" {
		t.Fatalf("expected code=version_skew, got %v", respBody["code"])
	}
}

func TestProductHandler_Archive_Returns200(t *testing.T) {
	store := newFakeProductStore()
	p := domain.NewProduct("Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = testWorkspaceID
	p.Currency = "EUR"
	p.UnitPriceMinor = 100
	store.products[p.ID] = p
	h := NewProductHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/products/"+p.ID, nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var respBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if archivedAt, ok := respBody["archived_at"]; !ok || archivedAt == nil {
		t.Fatalf("expected archived_at set in response, got %v", respBody)
	}
}
