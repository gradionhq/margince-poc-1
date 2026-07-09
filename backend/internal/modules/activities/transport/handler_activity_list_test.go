package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const activityHandlerTestWorkspaceID = "00000000-0000-0000-0000-000000000d01"

type activityHandlerListCaptureStore struct {
	t            *testing.T
	listCalled   bool
	filtered     actdomain.ActivityListFilter
	workspaceID  string
	cursor       string
	limit        int
	returnItems  []actdomain.Activity
	returnCursor string
}

func (f *activityHandlerListCaptureStore) Create(context.Context, actdomain.Activity) (actdomain.Activity, bool, error) {
	f.t.Fatal("Create should not be called")
	return actdomain.Activity{}, false, nil
}

func (f *activityHandlerListCaptureStore) Get(context.Context, string, string) (actdomain.Activity, error) {
	f.t.Fatal("Get should not be called")
	return actdomain.Activity{}, nil
}

func (f *activityHandlerListCaptureStore) List(_ context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]actdomain.Activity, string, error) {
	f.listCalled = true
	f.t.Fatalf("List should not be called for list handler requests; got workspace=%s entity_type=%s entity_id=%s cursor=%s limit=%d", workspaceID, entityType, entityID, cursor, limit)
	return nil, "", nil
}

func (f *activityHandlerListCaptureStore) ListFiltered(_ context.Context, workspaceID, cursor string, limit int, filter actdomain.ActivityListFilter) ([]actdomain.Activity, string, error) {
	f.filtered = filter
	f.workspaceID = workspaceID
	f.cursor = cursor
	f.limit = limit
	return f.returnItems, f.returnCursor, nil
}

func (f *activityHandlerListCaptureStore) Update(context.Context, string, string, map[string]any, int64) (actdomain.Activity, error) {
	f.t.Fatal("Update should not be called")
	return actdomain.Activity{}, nil
}

func (f *activityHandlerListCaptureStore) Archive(context.Context, string, string) (actdomain.Activity, error) {
	f.t.Fatal("Archive should not be called")
	return actdomain.Activity{}, nil
}

func (f *activityHandlerListCaptureStore) Relink(context.Context, string, string, string, string) (actdomain.Activity, error) {
	f.t.Fatal("Relink should not be called")
	return actdomain.Activity{}, nil
}

func withActivityWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: activityHandlerTestWorkspaceID, UserID: "human:test"})
	return r.WithContext(ctx)
}

func TestActivityHandler_List_RejectsUnknownSortField(t *testing.T) {
	store := &activityHandlerListCaptureStore{t: t}
	h := NewActivityHandler(store)

	req := withActivityWorkspace(httptest.NewRequest(http.MethodGet, "/activities?sort=-occurred_at,subject", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code != "sort_field_not_allowed" {
		t.Fatalf("expected sort_field_not_allowed, got %q (body=%s)", problem.Code, w.Body.String())
	}
	if store.listCalled {
		t.Fatal("list store method should not be called after sort validation fails")
	}
}

func TestActivityHandler_List_UsesFullQuerySurface(t *testing.T) {
	store := &activityHandlerListCaptureStore{
		t:            t,
		returnItems:  []actdomain.Activity{{ID: "a1"}},
		returnCursor: "next-cursor",
	}
	h := NewActivityHandler(store)

	req := withActivityWorkspace(httptest.NewRequest(http.MethodGet, "/activities?cursor=c123&limit=7&sort=-due_at&q=quarterly%20review&kind=task&entity_type=deal&entity_id=deal-1&assignee_id=user-1&include_archived=true", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if store.listCalled {
		t.Fatal("List should not be called by the activity list handler")
	}
	if store.workspaceID != activityHandlerTestWorkspaceID {
		t.Fatalf("workspaceID = %q, want %q", store.workspaceID, activityHandlerTestWorkspaceID)
	}
	if store.cursor != "c123" {
		t.Fatalf("cursor = %q, want %q", store.cursor, "c123")
	}
	if store.limit != 7 {
		t.Fatalf("limit = %d, want 7", store.limit)
	}
	if store.filtered.Sort != "-due_at" || store.filtered.Kind != "task" || store.filtered.EntityType != "deal" || store.filtered.EntityID != "deal-1" || store.filtered.AssigneeID != "user-1" || store.filtered.Q != "quarterly review" || !store.filtered.IncludeArchived {
		t.Fatalf("unexpected filter forwarded to store: %+v", store.filtered)
	}
	var page struct {
		Data []actdomain.Activity `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != "a1" {
		t.Fatalf("unexpected response payload: %+v", page.Data)
	}
}
