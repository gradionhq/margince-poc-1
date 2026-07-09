package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/ports/retrieval"
)

// fakeOfferStore is an in-memory OfferStore for handler tests — mirrors
// fakeProductStore's map-backed shape.
type fakeOfferStore struct {
	offers            map[string]domain.Offer
	nextErr           error
	regenerateErr     error
	regenerateReturn  domain.Offer
	regenerateID      string
	regenerateWSID    string
	regenerateSignals []domain.OfferLineSignal
	regenerateCalled  bool
}

func newFakeOfferStore() *fakeOfferStore {
	return &fakeOfferStore{offers: make(map[string]domain.Offer)}
}

func (f *fakeOfferStore) Create(ctx context.Context, o domain.Offer) (domain.Offer, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Offer{}, err
	}
	o.ID = "offer-1"
	o.Status = domain.OfferStatusDraft
	o.Revision = 1
	o.Version = 1
	o.NetMinor = 0
	o.TaxMinor = 0
	o.GrossMinor = 0
	o.CreatedAt = time.Now()
	o.UpdatedAt = time.Now()
	f.offers[o.ID] = o
	return o, nil
}

func (f *fakeOfferStore) Get(ctx context.Context, id, workspaceID string) (domain.Offer, error) {
	if o, ok := f.offers[id]; ok {
		return o, nil
	}
	return domain.Offer{}, errs.ErrNotFound
}

func (f *fakeOfferStore) List(ctx context.Context, workspaceID, dealID, cursor string, limit int, includeArchived bool) ([]domain.Offer, string, error) {
	var out []domain.Offer
	for _, o := range f.offers {
		if !includeArchived && o.ArchivedAt != nil {
			continue
		}
		out = append(out, o)
	}
	return out, "", nil
}

func (f *fakeOfferStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Offer, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Offer{}, err
	}
	o, ok := f.offers[id]
	if !ok {
		return domain.Offer{}, errs.ErrNotFound
	}
	if ifMatch != 0 && o.Version != ifMatch {
		return domain.Offer{}, errs.ErrVersionSkew
	}
	if v, ok := updates["intro_text"].(string); ok {
		o.IntroText = &v
	}
	o.Version++
	o.UpdatedAt = time.Now()
	f.offers[id] = o
	return o, nil
}

func (f *fakeOfferStore) Send(ctx context.Context, id, workspaceID string) (domain.Offer, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Offer{}, err
	}
	o, ok := f.offers[id]
	if !ok {
		return domain.Offer{}, errs.ErrNotFound
	}
	o.Status = domain.OfferStatusSent
	rate := "1.0000000000"
	now := time.Now().UTC()
	o.FxRateToBase = &rate
	o.FxRateDate = &now
	o.BuyerSnapshot = map[string]any{}
	o.IssuerSnapshot = map[string]any{"workspace_id": workspaceID, "name": "Test Workspace"}
	f.offers[id] = o
	return o, nil
}

func (f *fakeOfferStore) Accept(ctx context.Context, id, workspaceID string) (domain.Offer, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Offer{}, err
	}
	o, ok := f.offers[id]
	if !ok {
		return domain.Offer{}, errs.ErrNotFound
	}
	if o.Status != domain.OfferStatusSent {
		return domain.Offer{}, adapters.ErrOfferNotAcceptable
	}
	now := time.Now().UTC()
	o.Status = domain.OfferStatusAccepted
	o.AcceptedAt = &now
	f.offers[id] = o
	return o, nil
}

// Regenerate mirrors OfferStore.Regenerate's public contract closely enough
// for handler-level tests: it records every call's args (regenerateCalled/
// regenerateID/regenerateWSID/regenerateSignals) so a test can assert what
// the handler decoded and passed through, honors an explicit regenerateErr/
// regenerateReturn override when a test sets one (the OP-T07-style AI-signal
// tests), and otherwise falls back to a plain clone-and-supersede simulation
// (the OP-T06-style plain-regenerate tests) — offer-1 -> superseded,
// offer-2 -> the new draft.
func (f *fakeOfferStore) Regenerate(ctx context.Context, id, workspaceID string, signals []domain.OfferLineSignal) (domain.Offer, error) {
	f.regenerateCalled = true
	f.regenerateID = id
	f.regenerateWSID = workspaceID
	f.regenerateSignals = append([]domain.OfferLineSignal(nil), signals...)
	if f.regenerateErr != nil {
		err := f.regenerateErr
		f.regenerateErr = nil
		return domain.Offer{}, err
	}
	if f.regenerateReturn.ID != "" {
		return f.regenerateReturn, nil
	}
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Offer{}, err
	}
	o, ok := f.offers[id]
	if !ok {
		return domain.Offer{}, errs.ErrNotFound
	}
	next := o
	next.ID = "offer-2"
	next.Revision = o.Revision + 1
	next.Status = domain.OfferStatusDraft
	next.Version = 1
	o.Status = domain.OfferStatusSuperseded
	o.Version++
	f.offers[id] = o
	f.offers[next.ID] = next
	return next, nil
}

func (f *fakeOfferStore) PrepareRender(ctx context.Context, id, workspaceID string) (adapters.RenderIngredients, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return adapters.RenderIngredients{}, err
	}
	o, ok := f.offers[id]
	if !ok {
		return adapters.RenderIngredients{}, errs.ErrNotFound
	}
	return adapters.RenderIngredients{Offer: o, IssuerName: "Test Workspace", Locale: "de-DE"}, nil
}

func (f *fakeOfferStore) SetPdfAssetRef(ctx context.Context, id, workspaceID, ref string) (domain.Offer, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Offer{}, err
	}
	o, ok := f.offers[id]
	if !ok {
		return domain.Offer{}, errs.ErrNotFound
	}
	o.PdfAssetRef = &ref
	f.offers[id] = o
	return o, nil
}

// fakeOfferLineItemStore is an in-memory OfferLineItemStore for handler tests.
type fakeOfferLineItemStore struct {
	items   map[string]domain.OfferLineItem
	nextErr error
}

type fakeRetriever struct {
	ctx      retrieval.Context
	nextErr  error
	entityID string
	called   bool
}

func (f *fakeRetriever) Search(ctx context.Context, query string, limit int) ([]retrieval.Result, error) {
	return nil, nil
}

func (f *fakeRetriever) HybridSearch(ctx context.Context, q retrieval.HybridQuery) ([]retrieval.Result, error) {
	return nil, nil
}

func (f *fakeRetriever) AssembleContext(ctx context.Context, entityID string) (retrieval.Context, error) {
	f.called = true
	f.entityID = entityID
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return retrieval.Context{}, err
	}
	return f.ctx, nil
}

func newFakeOfferLineItemStore() *fakeOfferLineItemStore {
	return &fakeOfferLineItemStore{items: make(map[string]domain.OfferLineItem)}
}

func (f *fakeOfferLineItemStore) Create(ctx context.Context, li domain.OfferLineItem, explicitTaxRate *float64) (domain.OfferLineItem, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.OfferLineItem{}, err
	}
	li.ID = "li-1"
	li.CreatedAt = time.Now()
	li.UpdatedAt = time.Now()
	f.items[li.ID] = li
	return li, nil
}

func (f *fakeOfferLineItemStore) List(ctx context.Context, offerID, workspaceID string) ([]domain.OfferLineItem, error) {
	var out []domain.OfferLineItem
	for _, li := range f.items {
		if li.OfferID == offerID {
			out = append(out, li)
		}
	}
	return out, nil
}

func (f *fakeOfferLineItemStore) Update(ctx context.Context, id, offerID, workspaceID string, updates map[string]any) (domain.OfferLineItem, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.OfferLineItem{}, err
	}
	li, ok := f.items[id]
	if !ok {
		return domain.OfferLineItem{}, errs.ErrNotFound
	}
	if v, ok := updates["description"].(string); ok {
		li.Description = v
	}
	li.UpdatedAt = time.Now()
	f.items[id] = li
	return li, nil
}

func (f *fakeOfferLineItemStore) Delete(ctx context.Context, id, offerID, workspaceID string) error {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return err
	}
	if _, ok := f.items[id]; !ok {
		return errs.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

// newOfferFixture builds the "offer-1" fixture shared by the render/send/
// regenerate handler tests below — same id/workspace/deal/offer-number every
// time, varying only status and currency.
func newOfferFixture(status, currency string) domain.Offer {
	return domain.Offer{
		ID: "offer-1", WorkspaceID: testWorkspaceID, DealID: "deal-1", OfferNumber: "ANG-001",
		Status: status, Revision: 1, Version: 1, Currency: currency,
	}
}

func validCreateOfferBody() map[string]any {
	return map[string]any{
		"offer_number": "ANG-001",
		"currency":     "EUR",
		"source":       "test",
		"captured_by":  "human:test",
	}
}

func validCreateLineItemBody() map[string]any {
	return map[string]any{
		"position":         1,
		"description":      "Consulting Day",
		"quantity":         2.0,
		"unit_price_minor": 150000,
		"source":           "test",
		"captured_by":      "human:test",
	}
}

// doOfferAction builds an OfferHandler backed by offerStore, POSTs a
// bodyless request to path — as the default human principal (withWorkspace)
// when principal is nil, or as principal otherwise — and returns the
// recorded response alongside its decoded JSON body. This is the shared
// "seed a store, fire a Send/Accept/Regenerate action request" shape common
// to the handler's action-endpoint unit tests below.
func doOfferAction(t *testing.T, offerStore *fakeOfferStore, path string, principal *crmctx.Principal) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	h := NewOfferHandler(offerStore, newFakeOfferLineItemStore(), fakeVerifier{}, blobstore.NewMemoryStore(), NewNoOpRetriever())
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if principal != nil {
		req = req.WithContext(crmctx.With(req.Context(), *principal))
	} else {
		req = withWorkspace(req)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w, decodeJSONBody(t, w)
}

// ---- tests ----

func TestOfferHandler_CreateDealOffer_Created(t *testing.T) {
	h := newTestOfferHandler()
	bodyBytes, _ := json.Marshal(validCreateOfferBody())
	req := httptest.NewRequest(http.MethodPost, "/deals/deal-1/offers", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assertCreated201(t, w)
	respBody := decodeJSONBody(t, w)
	if status, ok := respBody["status"].(string); !ok || status != "draft" {
		t.Fatalf("expected status=draft, got %v", respBody["status"])
	}
	if rev, ok := respBody["revision"].(float64); !ok || int(rev) != 1 {
		t.Fatalf("expected revision=1, got %v", respBody["revision"])
	}
	if net, ok := respBody["net_minor"].(float64); !ok || int(net) != 0 {
		t.Fatalf("expected net_minor=0, got %v", respBody["net_minor"])
	}
}

func TestOfferHandler_CreateDealOffer_MissingProvenance_422(t *testing.T) {
	h := newTestOfferHandler()
	body := map[string]any{
		"offer_number": "ANG-001",
		"currency":     "EUR",
		// source and captured_by missing
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/deals/deal-1/offers", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	respBody := decodeJSONBody(t, w)
	details, _ := respBody["details"].(map[string]any)
	errsSlice, _ := details["errors"].([]any)
	if len(errsSlice) == 0 {
		t.Fatalf("expected field errors under details.errors, got %v", respBody)
	}
}

func TestOfferHandler_CreateDealOffer_DuplicateOfferNumber_409(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.nextErr = adapters.ErrDuplicateOfferNumber
	h := NewOfferHandler(offerStore, newFakeOfferLineItemStore(), fakeVerifier{}, blobstore.NewMemoryStore(), NewNoOpRetriever())

	postExpectConflict(t, h, "/deals/deal-1/offers", validCreateOfferBody(), "offer_number_duplicate")
}

func TestOfferHandler_ListDealOffers_Empty_OK(t *testing.T) {
	h := newTestOfferHandler()
	req := httptest.NewRequest(http.MethodGet, "/deals/deal-1/offers", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assertEmptyListOK(t, w)
}

func TestOfferHandler_GetOffer_NotFound_404(t *testing.T) {
	h := newTestOfferHandler()
	req := httptest.NewRequest(http.MethodGet, "/offers/missing-id", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", w.Code, w.Body.String())
	}
}

func TestOfferHandler_UpdateOffer_NotDraft_409(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.nextErr = adapters.ErrOfferNotDraft
	h := NewOfferHandler(offerStore, newFakeOfferLineItemStore(), fakeVerifier{}, blobstore.NewMemoryStore(), NewNoOpRetriever())

	body := map[string]any{"intro_text": "hello"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/offers/offer-1", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", w.Code, w.Body.String())
	}
	respBody := decodeJSONBody(t, w)
	if code, ok := respBody["code"].(string); !ok || code != "offer_not_draft" {
		t.Fatalf("expected code=offer_not_draft, got %v", respBody["code"])
	}
}

func TestOfferHandler_CreateOfferLineItem_Created(t *testing.T) {
	offerStore := newFakeOfferStore()
	o := domain.Offer{ID: "offer-1", Status: domain.OfferStatusDraft, Revision: 1, Version: 1}
	offerStore.offers["offer-1"] = o
	h := NewOfferHandler(offerStore, newFakeOfferLineItemStore(), fakeVerifier{}, blobstore.NewMemoryStore(), NewNoOpRetriever())

	bodyBytes, _ := json.Marshal(validCreateLineItemBody())
	req := httptest.NewRequest(http.MethodPost, "/offers/offer-1/line-items", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assertCreated201(t, w)
}

func TestOfferHandler_CreateOfferLineItem_PositionConflict_409(t *testing.T) {
	lineStore := newFakeOfferLineItemStore()
	lineStore.nextErr = &adapters.ErrDuplicatePosition{ExistingID: "li-existing", Position: 1}
	h := NewOfferHandler(newFakeOfferStore(), lineStore, fakeVerifier{}, blobstore.NewMemoryStore(), NewNoOpRetriever())

	respBody := postExpectConflict(t, h, "/offers/offer-1/line-items", validCreateLineItemBody(), "offer_line_item_position_duplicate")
	if details, ok := respBody["details"].(map[string]any); !ok || details["existing_id"] != "li-existing" {
		t.Fatalf("expected details.existing_id=li-existing, got %v", respBody["details"])
	}
}

func TestOfferHandler_UpdateOfferLineItem_Updated(t *testing.T) {
	lineStore := newFakeOfferLineItemStore()
	lineStore.items["li-1"] = domain.OfferLineItem{
		ID: "li-1", OfferID: "offer-1", Position: 1,
		Description: "Original", Quantity: 1, UnitPriceMinor: 100,
		Source: "test", CapturedBy: "human:test",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := NewOfferHandler(newFakeOfferStore(), lineStore, fakeVerifier{}, blobstore.NewMemoryStore(), NewNoOpRetriever())

	body := map[string]any{"description": "Updated"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/offers/offer-1/line-items/li-1", bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
}

func TestOfferHandler_DeleteOfferLineItem_NoContent(t *testing.T) {
	lineStore := newFakeOfferLineItemStore()
	lineStore.items["li-1"] = domain.OfferLineItem{
		ID: "li-1", OfferID: "offer-1",
		Source: "test", CapturedBy: "human:test",
	}
	h := NewOfferHandler(newFakeOfferStore(), lineStore, fakeVerifier{}, blobstore.NewMemoryStore(), NewNoOpRetriever())

	req := httptest.NewRequest(http.MethodDelete, "/offers/offer-1/line-items/li-1", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", w.Code, w.Body.String())
	}
}

func TestOfferHandler_RoutingDispatch_UnknownSuffix_404(t *testing.T) {
	h := newTestOfferHandler()
	req := httptest.NewRequest(http.MethodPost, "/offers/offer-1/unknown-verb", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", w.Code, w.Body.String())
	}
}

func TestOfferHandler_Render_SetsResult(t *testing.T) {
	offerStore := newFakeOfferStore()
	o := newOfferFixture(domain.OfferStatusDraft, "EUR")
	o.NetMinor, o.TaxMinor, o.GrossMinor = 125000, 23750, 148750
	offerStore.offers["offer-1"] = o
	blob := blobstore.NewMemoryStore()
	h := NewOfferHandler(offerStore, newFakeOfferLineItemStore(), fakeVerifier{}, blob, NewNoOpRetriever())

	req := httptest.NewRequest(http.MethodPost, "/offers/offer-1/render", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	respBody := decodeJSONBody(t, w)
	ref, _ := respBody["pdf_asset_ref"].(string)
	if ref == "" {
		t.Fatalf("expected pdf_asset_ref set, got %v", respBody["pdf_asset_ref"])
	}
	rc, err := blob.Get(context.Background(), ref)
	if err != nil {
		t.Fatalf("get pdf blob: %v", err)
	}
	defer rc.Close()
	pdfBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read pdf blob: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("expected a PDF header, got %q", pdfBytes[:min(20, len(pdfBytes))])
	}
}

func TestOfferHandler_Send_HumanPrincipal_NoTokenNeeded(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = newOfferFixture(domain.OfferStatusDraft, "EUR")
	w, respBody := doOfferAction(t, offerStore, "/offers/offer-1/send", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if status, ok := respBody["status"].(string); !ok || status != "sent" {
		t.Fatalf("expected status=sent, got %v", respBody["status"])
	}
	if rate, ok := respBody["fx_rate_to_base"].(string); !ok || rate != "1.0000000000" {
		t.Fatalf("expected fx_rate_to_base=1.0000000000, got %v", respBody["fx_rate_to_base"])
	}
	if respBody["buyer_snapshot"] == nil || respBody["issuer_snapshot"] == nil {
		t.Fatalf("expected snapshots populated, got buyer=%v issuer=%v", respBody["buyer_snapshot"], respBody["issuer_snapshot"])
	}
}

func TestOfferHandler_Accept_HumanPrincipal_NoTokenNeeded(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = newOfferFixture(domain.OfferStatusSent, "EUR")
	w, respBody := doOfferAction(t, offerStore, "/offers/offer-1/accept", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if status, ok := respBody["status"].(string); !ok || status != "accepted" {
		t.Fatalf("expected status=accepted, got %v", respBody["status"])
	}
	if respBody["accepted_at"] == nil {
		t.Fatal("expected accepted_at populated")
	}
}

func TestOfferHandler_Accept_AgentPrincipal_NoToken_403ApprovalRequired(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = newOfferFixture(domain.OfferStatusSent, "EUR")
	agent := crmctx.Principal{TenantID: testWorkspaceID, UserID: "agent:1", IsAgent: true}
	w, respBody := doOfferAction(t, offerStore, "/offers/offer-1/accept", &agent)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", w.Code, w.Body.String())
	}
	if code, ok := respBody["code"].(string); !ok || code != "approval_required" {
		t.Fatalf("expected code=approval_required, got %v", respBody["code"])
	}
}

func TestOfferHandler_Accept_NotSent_409(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = newOfferFixture(domain.OfferStatusDraft, "EUR")
	w, respBody := doOfferAction(t, offerStore, "/offers/offer-1/accept", nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", w.Code, w.Body.String())
	}
	if code, ok := respBody["code"].(string); !ok || code != "offer_not_acceptable" {
		t.Fatalf("expected code=offer_not_acceptable, got %v", respBody["code"])
	}
}

func TestOfferHandler_Send_AgentPrincipal_NoToken_403ApprovalRequired(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = newOfferFixture(domain.OfferStatusDraft, "EUR")
	agent := crmctx.Principal{TenantID: testWorkspaceID, UserID: "agent:1", IsAgent: true}
	w, respBody := doOfferAction(t, offerStore, "/offers/offer-1/send", &agent)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", w.Code, w.Body.String())
	}
	if code, ok := respBody["code"].(string); !ok || code != "approval_required" {
		t.Fatalf("expected code=approval_required, got %v", respBody["code"])
	}
}

func TestOfferHandler_Send_FXRateUnavailable_422(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = newOfferFixture(domain.OfferStatusDraft, "USD")
	offerStore.nextErr = &deals.FXRateUnavailableError{Currency: "USD", AsOf: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)}
	w, respBody := doOfferAction(t, offerStore, "/offers/offer-1/send", nil)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	if code, ok := respBody["code"].(string); !ok || code != "fx_rate_unavailable" {
		t.Fatalf("expected code=fx_rate_unavailable, got %v", respBody["code"])
	}
	details, _ := respBody["details"].(map[string]any)
	if details["currency"] != "USD" || details["as_of"] == nil {
		t.Fatalf("expected fx details, got %v", details)
	}
}

func TestOfferHandler_Regenerate_Success(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = newOfferFixture(domain.OfferStatusSent, "EUR")
	w, respBody := doOfferAction(t, offerStore, "/offers/offer-1/regenerate", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if status, ok := respBody["status"].(string); !ok || status != "draft" {
		t.Fatalf("expected the new revision's status=draft, got %v", respBody["status"])
	}
	if rev, ok := respBody["revision"].(float64); !ok || int(rev) != 2 {
		t.Fatalf("expected revision=2, got %v", respBody["revision"])
	}
	if prior := offerStore.offers["offer-1"]; prior.Status != domain.OfferStatusSuperseded {
		t.Fatalf("expected prior status=superseded, got %s", prior.Status)
	}
}

// TestOfferHandler_Regenerate_DecodesSignalsFromRetriever covers OP-T07's
// wiring: the handler resolves the offer, assembles the deal's retrieval
// context, decodes it into offer-line signals, and threads both the route's
// id/workspace and the decoded signals through to OfferStore.Regenerate.
func TestOfferHandler_Regenerate_DecodesSignalsFromRetriever(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = domain.Offer{ID: "offer-1", DealID: "deal-1", Status: domain.OfferStatusSent, Revision: 1, Version: 1}
	offerStore.regenerateReturn = domain.Offer{ID: "offer-2", DealID: "deal-1", Status: domain.OfferStatusDraft, Revision: 2, Version: 1}
	retriever := &fakeRetriever{
		ctx: retrieval.Context{
			Raw: map[string]any{
				"offer_line_signals": []domain.OfferLineSignal{
					{Description: "Consulting", Quantity: 1, Snippet: "consulting scope", SourceID: "activity-1"},
				},
			},
		},
	}
	h := NewOfferHandler(offerStore, newFakeOfferLineItemStore(), fakeVerifier{}, blobstore.NewMemoryStore(), retriever)

	req := httptest.NewRequest(http.MethodPost, "/offers/offer-1/regenerate", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if !offerStore.regenerateCalled {
		t.Fatalf("expected Regenerate to be called")
	}
	if offerStore.regenerateID != "offer-1" || offerStore.regenerateWSID != testWorkspaceID {
		t.Fatalf("expected Regenerate to receive the route offer id and workspace id, got id=%q workspace=%q", offerStore.regenerateID, offerStore.regenerateWSID)
	}
	if !retriever.called || retriever.entityID != "deal-1" {
		t.Fatalf("expected AssembleContext to be called with the offer's deal id, got called=%t entityID=%q", retriever.called, retriever.entityID)
	}
	if len(offerStore.regenerateSignals) != 1 || offerStore.regenerateSignals[0].Description != "Consulting" {
		t.Fatalf("expected the decoded signal slice to reach the store, got %+v", offerStore.regenerateSignals)
	}
	respBody := decodeJSONBody(t, w)
	if id, ok := respBody["id"].(string); !ok || id != "offer-2" {
		t.Fatalf("expected the regenerated offer body, got %+v", respBody)
	}
}

func TestOfferHandler_Regenerate_NotSent_409(t *testing.T) {
	offerStore := newFakeOfferStore()
	offerStore.offers["offer-1"] = domain.Offer{ID: "offer-1", DealID: "deal-1", Status: domain.OfferStatusDraft, Revision: 1, Version: 1}
	offerStore.regenerateErr = adapters.ErrOfferNotSent
	w, respBody := doOfferAction(t, offerStore, "/offers/offer-1/regenerate", nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", w.Code, w.Body.String())
	}
	if code, ok := respBody["code"].(string); !ok || code != "offer_not_sent" {
		t.Fatalf("expected code=offer_not_sent, got %v", respBody["code"])
	}
}

// Compile-time assertions that fakeOfferStore and fakeOfferLineItemStore
// implement the seam interfaces the handler uses.
var (
	_ offerStoreSeam         = (*fakeOfferStore)(nil)
	_ offerLineItemStoreSeam = (*fakeOfferLineItemStore)(nil)
	_ retrieval.Retriever    = (*fakeRetriever)(nil)
)

// Compile-time check that errors.Is works as expected for the non-pointer sentinel.
var (
	_ error = adapters.ErrOfferNotDraft
	_ error = adapters.ErrDuplicateOfferNumber
	_       = errors.Is(adapters.ErrOfferNotDraft, adapters.ErrOfferNotDraft)
)
