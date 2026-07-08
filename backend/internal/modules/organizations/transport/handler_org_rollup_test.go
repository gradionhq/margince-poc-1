package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/records"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// fakeRollupStore satisfies rollupStoreSeam for handler unit tests.
type fakeRollupStore struct {
	result records.RollupResult
	err    error
}

func (f *fakeRollupStore) Compute(_ context.Context, _, _, _, _ string) (records.RollupResult, error) {
	return f.result, f.err
}

func rollupHandlerForTest(store rollupStoreSeam) *OrganizationHandler {
	return &OrganizationHandler{rollupStore: store}
}

func withRollupWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: "ws-test", UserID: "human:test"})
	return r.WithContext(ctx)
}

func TestHierarchyRollup_200_TreeScope(t *testing.T) {
	currency := "EUR"
	weighted := int64(5000)
	closedWon := int64(3000)
	result := records.RollupResult{
		RootID:                 "root-id",
		Scope:                  "tree",
		WeightedPipelineMinor:  weighted,
		ClosedWonMinor:         closedWon,
		BaseCurrency:           currency,
		ActivityCount30d:       7,
		AggregatedAccountCount: 3,
		RestrictedExcluded:     []records.RestrictedNode{},
		ComputedAt:             time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC),
	}
	h := rollupHandlerForTest(&fakeRollupStore{result: result})

	req := httptest.NewRequest(http.MethodGet, "/organizations/root-id/hierarchy-rollup", nil)
	req = withRollupWorkspace(req)
	w := httptest.NewRecorder()
	h.hierarchyRollup(w, req, "root-id")

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["root_id"] != "root-id" {
		t.Errorf("root_id=%v want root-id", body["root_id"])
	}
	if body["scope"] != "tree" {
		t.Errorf("scope=%v want tree", body["scope"])
	}
	if body["activity_count_30d"] != float64(7) {
		t.Errorf("activity_count_30d=%v want 7", body["activity_count_30d"])
	}
	if body["aggregated_account_count"] != float64(3) {
		t.Errorf("aggregated_account_count=%v want 3", body["aggregated_account_count"])
	}
	wp, ok := body["weighted_pipeline"].(map[string]any)
	if !ok {
		t.Fatalf("weighted_pipeline not an object: %v", body["weighted_pipeline"])
	}
	if wp["amount_minor"] != float64(5000) {
		t.Errorf("weighted_pipeline.amount_minor=%v want 5000", wp["amount_minor"])
	}
	if wp["currency"] != "EUR" {
		t.Errorf("weighted_pipeline.currency=%v want EUR", wp["currency"])
	}
}

func TestHierarchyRollup_200_SelfScope(t *testing.T) {
	currency := "EUR"
	minor := int64(1000)
	result := records.RollupResult{
		RootID:                 "root-id",
		Scope:                  "self",
		WeightedPipelineMinor:  minor,
		ClosedWonMinor:         0,
		BaseCurrency:           currency,
		ActivityCount30d:       2,
		AggregatedAccountCount: 1,
		RestrictedExcluded:     []records.RestrictedNode{},
		ComputedAt:             time.Now().UTC(),
	}
	h := rollupHandlerForTest(&fakeRollupStore{result: result})

	req := httptest.NewRequest(http.MethodGet, "/organizations/root-id/hierarchy-rollup?scope=self", nil)
	req = withRollupWorkspace(req)
	w := httptest.NewRecorder()
	h.hierarchyRollup(w, req, "root-id")

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["scope"] != "self" {
		t.Errorf("scope=%v want self", body["scope"])
	}
	if body["aggregated_account_count"] != float64(1) {
		t.Errorf("aggregated_account_count=%v want 1", body["aggregated_account_count"])
	}
}

func TestHierarchyRollup_404_NotFound(t *testing.T) {
	h := rollupHandlerForTest(&fakeRollupStore{err: errs.ErrNotFound})

	req := httptest.NewRequest(http.MethodGet, "/organizations/missing/hierarchy-rollup", nil)
	req = withRollupWorkspace(req)
	w := httptest.NewRecorder()
	h.hierarchyRollup(w, req, "missing")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "not_found" {
		t.Errorf("code=%v want not_found", body["code"])
	}
}

func TestHierarchyRollup_422_FXRateUnavailable(t *testing.T) {
	fxErr := &deals.FXRateUnavailableError{
		Currency: "USD",
		AsOf:     time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC),
	}
	h := rollupHandlerForTest(&fakeRollupStore{err: fxErr})

	req := httptest.NewRequest(http.MethodGet, "/organizations/id/hierarchy-rollup", nil)
	req = withRollupWorkspace(req)
	w := httptest.NewRecorder()
	h.hierarchyRollup(w, req, "id")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "fx_rate_unavailable" {
		t.Errorf("code=%v want fx_rate_unavailable", body["code"])
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("details not an object: %v", body["details"])
	}
	if details["currency"] != "USD" {
		t.Errorf("details.currency=%v want USD", details["currency"])
	}
	if details["as_of"] != "2026-07-08" {
		t.Errorf("details.as_of=%v want 2026-07-08", details["as_of"])
	}
}

func TestHierarchyRollup_400_InvalidScope(t *testing.T) {
	h := rollupHandlerForTest(&fakeRollupStore{})

	req := httptest.NewRequest(http.MethodGet, "/organizations/id/hierarchy-rollup?scope=bogus", nil)
	req = withRollupWorkspace(req)
	w := httptest.NewRecorder()
	h.hierarchyRollup(w, req, "id")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "validation_error" {
		t.Errorf("code=%v want validation_error", body["code"])
	}
}
