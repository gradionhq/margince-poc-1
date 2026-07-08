package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/records"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

type fakeQuotaStore struct {
	createFn     func(ctx context.Context, q records.Quota) (records.Quota, error)
	getFn        func(ctx context.Context, id, workspaceID string) (records.Quota, error)
	listFn       func(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool, filter records.QuotaListFilter) ([]records.Quota, string, error)
	updateFn     func(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (records.Quota, error)
	archiveFn    func(ctx context.Context, id, workspaceID string) (records.Quota, error)
	attainmentFn func(ctx context.Context, id, workspaceID string) (records.Attainment, error)
}

func (f *fakeQuotaStore) Create(ctx context.Context, q records.Quota) (records.Quota, error) {
	return f.createFn(ctx, q)
}

func (f *fakeQuotaStore) Get(ctx context.Context, id, workspaceID string) (records.Quota, error) {
	return f.getFn(ctx, id, workspaceID)
}

func (f *fakeQuotaStore) List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool, filter records.QuotaListFilter) ([]records.Quota, string, error) {
	return f.listFn(ctx, workspaceID, cursor, limit, includeArchived, filter)
}

func (f *fakeQuotaStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (records.Quota, error) {
	return f.updateFn(ctx, id, workspaceID, updates, ifMatch)
}

func (f *fakeQuotaStore) Archive(ctx context.Context, id, workspaceID string) (records.Quota, error) {
	return f.archiveFn(ctx, id, workspaceID)
}

func (f *fakeQuotaStore) Attainment(ctx context.Context, id, workspaceID string) (records.Attainment, error) {
	return f.attainmentFn(ctx, id, workspaceID)
}

func withWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: "ws-1", UserID: "u-1"})
	return r.WithContext(ctx)
}

// doQuotaRequest builds a workspace-scoped request against store's handler and returns the
// recorded response. Shared by every TestQuotaHandler_* test below (SonarCloud new-code dedupe).
func doQuotaRequest(store *fakeQuotaStore, method, path, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r = withWorkspace(r)
	w := httptest.NewRecorder()
	NewQuotaHandler(store).ServeHTTP(w, r)
	return w
}

// decodeProblemCode decodes w's JSON body and returns its top-level "code" field.
func decodeProblemCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	code, _ := resp["code"].(string)
	return code
}

func TestQuotaHandler_Create_201(t *testing.T) {
	store := &fakeQuotaStore{
		createFn: func(_ context.Context, q records.Quota) (records.Quota, error) {
			q.ID = "q-1"
			return q, nil
		},
	}
	w := doQuotaRequest(store, http.MethodPost, "/quotas",
		`{"owner_id":"owner-1","period_start":"2025-01-01","period_end":"2025-12-31","target_minor":1000000,"currency":"EUR"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201 got %d: %s", w.Code, w.Body.String())
	}
}

func TestQuotaHandler_Create_422_OwnerXorTeam(t *testing.T) {
	store := &fakeQuotaStore{
		createFn: func(_ context.Context, _ records.Quota) (records.Quota, error) {
			return records.Quota{}, records.ErrOwnerXorTeamRequired
		},
	}
	w := doQuotaRequest(store, http.MethodPost, "/quotas",
		`{"owner_id":"o","team_id":"t","period_start":"2025-01-01","period_end":"2025-12-31","target_minor":1,"currency":"EUR"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	details, _ := resp["details"].(map[string]any)
	ferrs, _ := details["errors"].([]any)
	if len(ferrs) == 0 {
		t.Fatal("want field errors in details.errors")
	}
	fe, _ := ferrs[0].(map[string]any)
	if fe["field"] != "owner_id" || fe["code"] != "owner_xor_team_required" {
		t.Fatalf("wrong field error: %v", fe)
	}
}

func TestQuotaHandler_Create_400_MalformedDate(t *testing.T) {
	store := &fakeQuotaStore{}
	w := doQuotaRequest(store, http.MethodPost, "/quotas",
		`{"owner_id":"o","period_start":"not-a-date","period_end":"2025-12-31","target_minor":1,"currency":"EUR"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", w.Code)
	}
}

func TestQuotaHandler_Get_200(t *testing.T) {
	store := &fakeQuotaStore{
		getFn: func(_ context.Context, id, _ string) (records.Quota, error) {
			return records.Quota{ID: id}, nil
		},
	}
	w := doQuotaRequest(store, http.MethodGet, "/quotas/q-1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestQuotaHandler_Get_404(t *testing.T) {
	store := &fakeQuotaStore{
		getFn: func(_ context.Context, _, _ string) (records.Quota, error) {
			return records.Quota{}, errs.ErrNotFound
		},
	}
	w := doQuotaRequest(store, http.MethodGet, "/quotas/missing", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", w.Code)
	}
}

func TestQuotaHandler_List_200(t *testing.T) {
	store := &fakeQuotaStore{
		listFn: func(_ context.Context, _, _ string, _ int, _ bool, _ records.QuotaListFilter) ([]records.Quota, string, error) {
			return []records.Quota{{ID: "q-1"}}, "", nil
		},
	}
	w := doQuotaRequest(store, http.MethodGet, "/quotas", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestQuotaHandler_Update_200(t *testing.T) {
	store := &fakeQuotaStore{
		updateFn: func(_ context.Context, id, _ string, _ map[string]any, _ int64) (records.Quota, error) {
			return records.Quota{ID: id, Currency: "EUR"}, nil
		},
	}
	w := doQuotaRequest(store, http.MethodPatch, "/quotas/q-1", `{"currency":"EUR"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestQuotaHandler_Update_409_VersionSkew(t *testing.T) {
	store := &fakeQuotaStore{
		updateFn: func(_ context.Context, _, _ string, _ map[string]any, _ int64) (records.Quota, error) {
			return records.Quota{}, errs.ErrVersionSkew
		},
	}
	w := doQuotaRequest(store, http.MethodPatch, "/quotas/q-1", `{"currency":"EUR"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 got %d", w.Code)
	}
	if code := decodeProblemCode(t, w); code != "version_skew" {
		t.Fatalf("want version_skew code, got %v", code)
	}
}

func TestQuotaHandler_Update_422_OwnerXorTeam(t *testing.T) {
	store := &fakeQuotaStore{
		updateFn: func(_ context.Context, _, _ string, _ map[string]any, _ int64) (records.Quota, error) {
			return records.Quota{}, records.ErrOwnerXorTeamRequired
		},
	}
	w := doQuotaRequest(store, http.MethodPatch, "/quotas/q-1", `{"team_id":"t"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 got %d", w.Code)
	}
}

func TestQuotaHandler_Update_400_MalformedDate(t *testing.T) {
	store := &fakeQuotaStore{}
	w := doQuotaRequest(store, http.MethodPatch, "/quotas/q-1", `{"period_start":"bad-date"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", w.Code)
	}
}

func TestQuotaHandler_Archive_200(t *testing.T) {
	store := &fakeQuotaStore{
		archiveFn: func(_ context.Context, id, _ string) (records.Quota, error) {
			return records.Quota{ID: id}, nil
		},
	}
	w := doQuotaRequest(store, http.MethodDelete, "/quotas/q-1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestQuotaHandler_Attainment_200(t *testing.T) {
	store := &fakeQuotaStore{
		attainmentFn: func(_ context.Context, id, _ string) (records.Attainment, error) {
			return records.Attainment{QuotaID: id, Band: "met"}, nil
		},
	}
	w := doQuotaRequest(store, http.MethodGet, "/quotas/q-1/attainment", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestQuotaHandler_Attainment_404(t *testing.T) {
	store := &fakeQuotaStore{
		attainmentFn: func(_ context.Context, _, _ string) (records.Attainment, error) {
			return records.Attainment{}, errs.ErrNotFound
		},
	}
	w := doQuotaRequest(store, http.MethodGet, "/quotas/missing/attainment", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", w.Code)
	}
}

func TestQuotaHandler_Attainment_422_TargetZero(t *testing.T) {
	store := &fakeQuotaStore{
		attainmentFn: func(_ context.Context, _, _ string) (records.Attainment, error) {
			return records.Attainment{}, records.ErrAttainmentTargetZero
		},
	}
	w := doQuotaRequest(store, http.MethodGet, "/quotas/q-1/attainment", "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 got %d", w.Code)
	}
	if code := decodeProblemCode(t, w); code != "attainment_target_zero" {
		t.Fatalf("want attainment_target_zero got %v", code)
	}
}

func TestQuotaHandler_Attainment_422_ComputationFailed(t *testing.T) {
	store := &fakeQuotaStore{
		attainmentFn: func(_ context.Context, _, _ string) (records.Attainment, error) {
			return records.Attainment{}, errors.New("fx rate unavailable for USD")
		},
	}
	w := doQuotaRequest(store, http.MethodGet, "/quotas/q-1/attainment", "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 got %d", w.Code)
	}
	if code := decodeProblemCode(t, w); code != "attainment_computation_failed" {
		t.Fatalf("want attainment_computation_failed got %v", code)
	}
}
