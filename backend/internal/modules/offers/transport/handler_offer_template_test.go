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
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

type fakeOfferTemplateStore struct {
	templates map[string]domain.OfferTemplate
	nextErr   error
}

func newFakeOfferTemplateStore() *fakeOfferTemplateStore {
	return &fakeOfferTemplateStore{templates: make(map[string]domain.OfferTemplate)}
}

func (f *fakeOfferTemplateStore) Create(ctx context.Context, t domain.OfferTemplate) (domain.OfferTemplate, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.OfferTemplate{}, err
	}
	t.ID = "tpl-1"
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	t.Version = 1
	f.templates[t.ID] = t
	return t, nil
}

func (f *fakeOfferTemplateStore) Get(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error) {
	if t, ok := f.templates[id]; ok {
		return t, nil
	}
	return domain.OfferTemplate{}, errs.ErrNotFound
}

func (f *fakeOfferTemplateStore) List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]domain.OfferTemplate, string, error) {
	var out []domain.OfferTemplate
	for _, t := range f.templates {
		if !includeArchived && t.ArchivedAt != nil {
			continue
		}
		out = append(out, t)
	}
	return out, "", nil
}

func (f *fakeOfferTemplateStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.OfferTemplate, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.OfferTemplate{}, err
	}
	if t, ok := f.templates[id]; ok {
		if ifMatch != 0 && t.Version != ifMatch {
			return domain.OfferTemplate{}, errs.ErrVersionSkew
		}
		if name, ok := updates["name"].(string); ok {
			t.Name = name
		}
		t.Version++
		t.UpdatedAt = time.Now()
		f.templates[id] = t
		return t, nil
	}
	return domain.OfferTemplate{}, errs.ErrNotFound
}

func (f *fakeOfferTemplateStore) Archive(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error) {
	if t, ok := f.templates[id]; ok {
		now := time.Now()
		t.ArchivedAt = &now
		f.templates[id] = t
		return t, nil
	}
	return domain.OfferTemplate{}, errs.ErrNotFound
}

func TestOfferTemplateHandler_Create_Valid_Returns201(t *testing.T) {
	store := newFakeOfferTemplateStore()
	h := NewOfferTemplateHandler(store)

	body := map[string]any{
		"name":        "Standard DE",
		"layout":      map[string]any{"logo_ref": nil},
		"source":      "test",
		"captured_by": "human:test",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/offer-templates", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assertCreated201(t, w)
}

func TestOfferTemplateHandler_Create_DuplicateName_Returns409(t *testing.T) {
	store := newFakeOfferTemplateStore()
	store.nextErr = &adapters.ErrDuplicateTemplateName{ExistingID: "existing-id"}
	h := NewOfferTemplateHandler(store)

	body := map[string]any{
		"name":        "Dup Name",
		"source":      "test",
		"captured_by": "human:test",
	}
	postExpectConflict(t, h, "/offer-templates", body, "offer_template_name_duplicate")
}

func TestOfferTemplateHandler_Create_DefaultConflict_Returns409(t *testing.T) {
	store := newFakeOfferTemplateStore()
	store.nextErr = &adapters.ErrDefaultConflict{ExistingID: "existing-id", Locale: "de-DE"}
	h := NewOfferTemplateHandler(store)

	body := map[string]any{
		"name":        "Second Default",
		"is_default":  true,
		"source":      "test",
		"captured_by": "human:test",
	}
	postExpectConflict(t, h, "/offer-templates", body, "offer_template_default_conflict")
}

func TestOfferTemplateHandler_List_Empty_Returns200(t *testing.T) {
	store := newFakeOfferTemplateStore()
	h := NewOfferTemplateHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/offer-templates", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assertEmptyListOK(t, w)
}

func TestOfferTemplateHandler_Archive_Returns200(t *testing.T) {
	store := newFakeOfferTemplateStore()
	tpl := domain.NewOfferTemplate("Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	tpl.WorkspaceID = testWorkspaceID
	store.templates[tpl.ID] = tpl
	h := NewOfferTemplateHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/offer-templates/"+tpl.ID, nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	respBody := decodeJSONBody(t, w)
	if archivedAt, ok := respBody["archived_at"]; !ok || archivedAt == nil {
		t.Fatalf("expected archived_at set in response, got %v", respBody)
	}
}
