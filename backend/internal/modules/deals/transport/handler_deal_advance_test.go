package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

type fakeStageSemanticReader struct {
	deal          domain.Deal
	semanticByID  map[string]string
	advanceCalled bool
	archiveCalled bool
}

func (f *fakeStageSemanticReader) Get(_ context.Context, _, _ string) (domain.Deal, error) {
	return f.deal, nil
}

func (f *fakeStageSemanticReader) GetAny(_ context.Context, _, _ string) (domain.Deal, error) {
	return f.deal, nil
}

func (f *fakeStageSemanticReader) StageSemantic(_ context.Context, stageID, _ string) (string, error) {
	return f.semanticByID[stageID], nil
}

func (f *fakeStageSemanticReader) Advance(_ context.Context, _, _ string, _ domain.AdvanceInput, _ int64, _ string) (domain.Deal, error) {
	f.advanceCalled = true
	return domain.Deal{}, nil
}

func (f *fakeStageSemanticReader) FindByIdempotencyKey(_ context.Context, _, _ string) (domain.Deal, bool, error) {
	return domain.Deal{}, false, nil
}

func (f *fakeStageSemanticReader) Create(_ context.Context, d domain.Deal, _ string) (domain.Deal, error) {
	return d, nil
}

func (f *fakeStageSemanticReader) Update(_ context.Context, _, _ string, _ map[string]any, _ int64) (domain.Deal, error) {
	return domain.Deal{}, nil
}

func (f *fakeStageSemanticReader) ListFiltered(_ context.Context, _, _ string, _ int, _ domain.DealListFilter) ([]domain.Deal, string, error) {
	return nil, "", nil
}

func (f *fakeStageSemanticReader) Restore(_ context.Context, _, _ string) (domain.Deal, error) {
	return domain.Deal{}, nil
}

func (f *fakeStageSemanticReader) Archive(_ context.Context, _, _ string) (domain.Deal, error) {
	f.archiveCalled = true
	return f.deal, nil
}

func TestDealHandler_Advance_AgentWithoutTokenOnYellowTransition_403(t *testing.T) {
	fake := &fakeStageSemanticReader{
		deal:         domain.Deal{ID: "deal-1", StageID: "open-stage"},
		semanticByID: map[string]string{"open-stage": "open", "won-stage": "won"},
	}
	h := &DealHandler{store: fake}

	req := httptest.NewRequest(http.MethodPost, "/deals/deal-1/advance",
		strings.NewReader(`{"to_stage_id":"won-stage","status":"won"}`))
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{UserID: "agent-1", TenantID: "ws-1", IsAgent: true}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if fake.advanceCalled {
		t.Fatal("store.Advance must not be called when the token gate rejects")
	}
}

func TestDealHandler_Advance_HumanOnYellowTransition_NoTokenNeeded(t *testing.T) {
	fake := &fakeStageSemanticReader{
		deal:         domain.Deal{ID: "deal-2", StageID: "open-stage"},
		semanticByID: map[string]string{"open-stage": "open", "won-stage": "won"},
	}
	h := &DealHandler{store: fake}

	req := httptest.NewRequest(http.MethodPost, "/deals/deal-2/advance",
		strings.NewReader(`{"to_stage_id":"won-stage","status":"won"}`))
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{UserID: "human-1", TenantID: "ws-1", IsAgent: false}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("human on 🟡 transition needs no token: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !fake.advanceCalled {
		t.Fatal("store.Advance must be called for a human caller")
	}
}
