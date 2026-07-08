package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
)

func TestDealHandler_ServeHTTP_DeleteDispatchesToArchive(t *testing.T) {
	fake := &fakeStageSemanticReader{deal: domain.Deal{ID: "deal-1"}}
	h := &DealHandler{store: fake}
	req := httptest.NewRequest(http.MethodDelete, "/deals/deal-1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !fake.archiveCalled {
		t.Fatal("store.Archive must be called for DELETE /deals/{id}")
	}
}
