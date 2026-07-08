package fieldhistory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// fakeStore satisfies fieldHistoryStore for handler unit tests.
type fakeStore struct {
	entries    []Entry
	nextCursor string
	err        error
	called     bool
}

func (f *fakeStore) List(
	_ context.Context, _, _, _ string, _, _ *string, _ string, _ int,
) ([]Entry, string, error) {
	f.called = true
	return f.entries, f.nextCursor, f.err
}

// testAllowAuthz is an Authorizer that always grants.
func testAllowAuthz(_ context.Context, _, _ string) error { return nil }

// testDenyAuthz is an Authorizer that always denies.
func testDenyAuthz(_ context.Context, _, _ string) error { return errors.New("forbidden") }

// withFieldHistoryWorkspace returns r with a workspace principal attached to its context.
func withFieldHistoryWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: "ws-test", UserID: "human:test"})
	return r.WithContext(ctx)
}

// doFHRequest issues the request through h.ServeHTTP and decodes the JSON body.
func doFHRequest(t *testing.T, h *Handler, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = withFieldHistoryWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (status=%d body=%s)", err, w.Code, w.Body.String())
	}
	return w.Code, body
}

const validTestUUID = "11111111-2222-3333-4444-555555555555"

// TestHandler_Validation covers all handler-level 400 cases.
func TestHandler_Validation(t *testing.T) {
	h := NewHandler(&fakeStore{}, testAllowAuthz)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing_both",
			path:       "/field-history",
			wantStatus: 400,
			wantCode:   "missing_params",
		},
		{
			name:       "missing_entity_type",
			path:       "/field-history?entity_id=" + validTestUUID,
			wantStatus: 400,
			wantCode:   "missing_params",
		},
		{
			name:       "missing_entity_id",
			path:       "/field-history?entity_type=person",
			wantStatus: 400,
			wantCode:   "missing_params",
		},
		{
			name:       "invalid_id_not_uuid",
			path:       "/field-history?entity_type=person&entity_id=not-a-uuid",
			wantStatus: 400,
			wantCode:   "invalid_id",
		},
		{
			name:       "invalid_entity_type",
			path:       "/field-history?entity_type=bogus&entity_id=" + validTestUUID,
			wantStatus: 400,
			wantCode:   "invalid_entity_type",
		},
		{
			name:       "invalid_actor_type",
			path:       "/field-history?entity_type=person&entity_id=" + validTestUUID + "&actor_type=robot",
			wantStatus: 400,
			wantCode:   "invalid_actor_type",
		},
		{
			name:       "invalid_cursor",
			path:       "/field-history?entity_type=person&entity_id=" + validTestUUID + "&cursor=!!!notbase64!!!",
			wantStatus: 400,
			wantCode:   "invalid_cursor",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, body := doFHRequest(t, h, tc.path)
			if status != tc.wantStatus {
				t.Errorf("status = %d, want %d", status, tc.wantStatus)
			}
			if got := body["code"]; got != tc.wantCode {
				t.Errorf("code = %v, want %q", got, tc.wantCode)
			}
		})
	}
}

// TestHandler_AllEntityTypesValid confirms all five contract enum values are accepted.
func TestHandler_AllEntityTypesValid(t *testing.T) {
	for _, et := range []string{"person", "organization", "deal", "lead", "activity"} {
		fake := &fakeStore{entries: []Entry{}}
		h := NewHandler(fake, testAllowAuthz)
		status, _ := doFHRequest(t, h, "/field-history?entity_type="+et+"&entity_id="+validTestUUID)
		if status != http.StatusOK {
			t.Errorf("entity_type=%q: status = %d, want 200", et, status)
		}
	}
}

// TestHandler_AllActorTypesValid confirms all four contract enum values are accepted.
func TestHandler_AllActorTypesValid(t *testing.T) {
	for _, at := range []string{"human", "agent", "system", "connector"} {
		fake := &fakeStore{entries: []Entry{}}
		h := NewHandler(fake, testAllowAuthz)
		status, _ := doFHRequest(t, h, "/field-history?entity_type=person&entity_id="+validTestUUID+"&actor_type="+at)
		if status != http.StatusOK {
			t.Errorf("actor_type=%q: status = %d, want 200", at, status)
		}
	}
}

// TestHandler_RBACDeny confirms 403 and that the store is never called.
func TestHandler_RBACDeny(t *testing.T) {
	fake := &fakeStore{}
	h := NewHandler(fake, testDenyAuthz)

	status, body := doFHRequest(t, h, "/field-history?entity_type=person&entity_id="+validTestUUID)
	if status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", status)
	}
	if body["code"] != "forbidden" {
		t.Errorf("code = %v, want forbidden", body["code"])
	}
	if fake.called {
		t.Error("store must not be called when RBAC denies")
	}
}

// TestHandler_HappyPath verifies the 200 response envelope and entry pass-through.
func TestHandler_HappyPath(t *testing.T) {
	oldVal := "Alice"
	newVal := "Bob"
	entry := Entry{
		ID:         "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		EntityType: "person",
		EntityID:   validTestUUID,
		Field:      "name",
		OldValue:   &oldVal,
		NewValue:   &newVal,
		ChangedAt:  time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		ActorType:  "human",
		ActorID:    "user-1",
	}
	fake := &fakeStore{
		entries:    []Entry{entry},
		nextCursor: "cursor-abc",
	}
	h := NewHandler(fake, testAllowAuthz)

	status, body := doFHRequest(t, h, "/field-history?entity_type=person&entity_id="+validTestUUID)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !fake.called {
		t.Error("store.List must be called on happy path")
	}

	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %v", body["data"])
	}
	if len(data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(data))
	}
	e0, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("data[0] is not an object: %v", data[0])
	}
	if e0["field"] != "name" {
		t.Errorf("data[0].field = %v, want name", e0["field"])
	}
	if e0["old_value"] != "Alice" {
		t.Errorf("data[0].old_value = %v, want Alice", e0["old_value"])
	}
	if e0["new_value"] != "Bob" {
		t.Errorf("data[0].new_value = %v, want Bob", e0["new_value"])
	}

	page, ok := body["page"].(map[string]any)
	if !ok {
		t.Fatalf("page is not an object: %v", body["page"])
	}
	if page["has_more"] != true {
		t.Errorf("has_more = %v, want true", page["has_more"])
	}
	if page["next_cursor"] != "cursor-abc" {
		t.Errorf("next_cursor = %v, want cursor-abc", page["next_cursor"])
	}
}

// TestHandler_HonestEmpty confirms 200 with empty data and has_more=false/next_cursor=null.
func TestHandler_HonestEmpty(t *testing.T) {
	fake := &fakeStore{entries: []Entry{}, nextCursor: ""}
	h := NewHandler(fake, testAllowAuthz)

	status, body := doFHRequest(t, h, "/field-history?entity_type=organization&entity_id="+validTestUUID)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (honest empty)", status)
	}

	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %v", body["data"])
	}
	if len(data) != 0 {
		t.Errorf("len(data) = %d, want 0 (empty)", len(data))
	}

	page, ok := body["page"].(map[string]any)
	if !ok {
		t.Fatalf("page is not an object: %v", body["page"])
	}
	if page["has_more"] != false {
		t.Errorf("has_more = %v, want false", page["has_more"])
	}
	if page["next_cursor"] != nil {
		t.Errorf("next_cursor = %v, want null", page["next_cursor"])
	}
}
