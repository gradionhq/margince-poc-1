package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/shared/ports/extraction"
)

type fakeDealWriter struct {
	calls       int
	workspaceID string
	dealID      string
	updates     map[string]any
}

const testDealID = "00000000-0000-0000-0000-000000000001"

func (f *fakeDealWriter) UpdateFields(_ context.Context, workspaceID, dealID string, updates map[string]any) error {
	f.calls++
	f.workspaceID = workspaceID
	f.dealID = dealID
	f.updates = updates
	return nil
}

func TestAttachmentHandler_GetExtraction_NoOpExtractorReturnsEmpty(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	h := newTestHandlerWithSeams(store, &fakeAudit{}, extraction.NoOpExtractor{}, nil)

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id+"/extraction", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp types.AttachmentExtraction
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Fields) != 0 || len(resp.Omitted) != 0 {
		t.Fatalf("want empty extraction, got %+v", resp)
	}
}

func TestAttachmentHandler_GetExtraction_PartitionsGroundedAndOmitted(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	fixture := extraction.FixtureExtractor{Fields: map[string][]extraction.ExtractedField{
		id: {
			{Field: "name", Value: "Acme Corp", SourceQuote: "Acme Corp", PageOrSection: "p. 1", Confidence: "high"},
			{Field: "amount_minor", Value: "4200", SourceQuote: "$42.00", PageOrSection: "p. 2", Confidence: "medium"},
			{Field: "missing_field", Omitted: true, OmittedReason: "not_stated_in_file"},
		},
	}}
	h := newTestHandlerWithSeams(store, &fakeAudit{}, fixture, nil)

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id+"/extraction", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp types.AttachmentExtraction
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Fields) != 2 {
		t.Fatalf("want 2 grounded fields, got %d", len(resp.Fields))
	}
	if len(resp.Omitted) != 1 {
		t.Fatalf("want 1 omitted field, got %d", len(resp.Omitted))
	}
	if resp.Fields[0].Field != "name" || resp.Fields[0].Confidence != types.High {
		t.Fatalf("unexpected first grounded field: %+v", resp.Fields[0])
	}
	if resp.Omitted[0].Field != "missing_field" || resp.Omitted[0].Reason != types.NotStatedInFile {
		t.Fatalf("unexpected omitted field: %+v", resp.Omitted[0])
	}
}

func TestAttachmentHandler_AcceptExtraction_PersistsSelectedFieldsAndAudits(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	a := store.items[id]
	a.EntityID = testDealID
	store.items[id] = a
	audit := &fakeAudit{}
	writer := &fakeDealWriter{}
	fixture := extraction.FixtureExtractor{Fields: map[string][]extraction.ExtractedField{
		id: {
			{Field: "name", Value: "Acme Corp", SourceQuote: "Acme Corp", Confidence: "high"},
			{Field: "amount_minor", Value: "4200", SourceQuote: "$42.00", Confidence: "medium"},
			{Field: "currency", Value: "USD", SourceQuote: "USD", Confidence: "high"},
		},
	}}
	h := newTestHandlerWithSeams(store, audit, fixture, writer)

	body := types.AcceptExtractionRequest{FieldKeys: []string{"name", "amount_minor"}}
	raw, _ := json.Marshal(body)
	req := withAttachWorkspace(httptest.NewRequest(http.MethodPost, "/attachments/"+id+"/extraction:accept", bytes.NewReader(raw)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp types.AttachmentExtractionAcceptResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if writer.calls != 1 {
		t.Fatalf("want 1 deal write, got %d", writer.calls)
	}
	if writer.workspaceID != testWS || writer.dealID != testDealID {
		t.Fatalf("unexpected deal write target: %+v", writer)
	}
	if got := writer.updates["name"]; got != "Acme Corp" {
		t.Fatalf("want name update to use extracted value, got %#v", got)
	}
	if got := writer.updates["amount_minor"]; got != int64(4200) {
		t.Fatalf("want amount_minor update to be int64(4200), got %#v", got)
	}
	if len(audit.extractionCalls) != 2 {
		t.Fatalf("want 2 extraction audit rows, got %d", len(audit.extractionCalls))
	}
	for _, call := range audit.extractionCalls {
		if call.capturedBy != "agent:attachment-extractor" {
			t.Fatalf("want ai provenance for unedited fields, got %+v", call)
		}
	}
	if len(resp.Accepted) != 2 {
		t.Fatalf("want 2 accepted rows, got %d", len(resp.Accepted))
	}
	if resp.Accepted[0].Provenance != types.AcceptedExtractionFieldProvenanceAiExtracted {
		t.Fatalf("unexpected provenance for first accepted field: %+v", resp.Accepted[0])
	}
}

func TestAttachmentHandler_AcceptExtraction_EditedFieldFlipsProvenanceAndCapturedBy(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	a := store.items[id]
	a.EntityID = testDealID
	store.items[id] = a
	audit := &fakeAudit{}
	writer := &fakeDealWriter{}
	fixture := extraction.FixtureExtractor{Fields: map[string][]extraction.ExtractedField{
		id: {
			{Field: "name", Value: "Acme Corp", SourceQuote: "Acme Corp", Confidence: "high"},
			{Field: "currency", Value: "USD", SourceQuote: "USD", Confidence: "high"},
		},
	}}
	h := newTestHandlerWithSeams(store, audit, fixture, writer)

	edits := map[string]any{"name": "Acme Revised"}
	body := types.AcceptExtractionRequest{FieldKeys: []string{"name", "currency"}, Edits: &edits}
	raw, _ := json.Marshal(body)
	req := withAttachWorkspace(httptest.NewRequest(http.MethodPost, "/attachments/"+id+"/extraction:accept", bytes.NewReader(raw)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if got := writer.updates["name"]; got != "Acme Revised" {
		t.Fatalf("want edited name persisted, got %#v", got)
	}
	if got := writer.updates["currency"]; got != "USD" {
		t.Fatalf("want unedited currency persisted, got %#v", got)
	}
	if len(audit.extractionCalls) != 2 {
		t.Fatalf("want 2 extraction audit rows, got %d", len(audit.extractionCalls))
	}
	if audit.extractionCalls[0].field != "name" || audit.extractionCalls[0].capturedBy != "human:test" {
		t.Fatalf("want edited field audit to be human-authored, got %+v", audit.extractionCalls[0])
	}
	if audit.extractionCalls[1].field != "currency" || audit.extractionCalls[1].capturedBy != "agent:attachment-extractor" {
		t.Fatalf("want unedited field audit to remain ai-authored, got %+v", audit.extractionCalls[1])
	}
}

func TestAttachmentHandler_AcceptExtraction_NonDealReturns422(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	a := store.items[id]
	a.EntityType = domain.EntityTypeLead
	store.items[id] = a
	audit := &fakeAudit{}
	writer := &fakeDealWriter{}
	h := newTestHandlerWithSeams(store, audit, extraction.NoOpExtractor{}, writer)

	body := types.AcceptExtractionRequest{FieldKeys: []string{"name"}}
	raw, _ := json.Marshal(body)
	req := withAttachWorkspace(httptest.NewRequest(http.MethodPost, "/attachments/"+id+"/extraction:accept", bytes.NewReader(raw)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	if writer.calls != 0 || len(audit.extractionCalls) != 0 {
		t.Fatalf("want no side effects for non-deal attachment, got writer=%d audit=%d", writer.calls, len(audit.extractionCalls))
	}
}

func TestAttachmentHandler_AcceptExtraction_UnknownFieldRejectsWholeRequest(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	audit := &fakeAudit{}
	writer := &fakeDealWriter{}
	fixture := extraction.FixtureExtractor{Fields: map[string][]extraction.ExtractedField{
		id: {
			{Field: "name", Value: "Acme Corp", SourceQuote: "Acme Corp", Confidence: "high"},
		},
	}}
	h := newTestHandlerWithSeams(store, audit, fixture, writer)

	body := types.AcceptExtractionRequest{FieldKeys: []string{"name", "unknown"}}
	raw, _ := json.Marshal(body)
	req := withAttachWorkspace(httptest.NewRequest(http.MethodPost, "/attachments/"+id+"/extraction:accept", bytes.NewReader(raw)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	if writer.calls != 0 || len(audit.extractionCalls) != 0 {
		t.Fatalf("want no side effects for invalid field_keys, got writer=%d audit=%d", writer.calls, len(audit.extractionCalls))
	}
}

func TestAttachmentHandler_RequestAccess_Returns200AndAuditsPrincipal(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	audit := &fakeAudit{}
	h := newTestHandlerWithSeams(store, audit, extraction.NoOpExtractor{}, nil)

	req := withAttachWorkspace(httptest.NewRequest(http.MethodPost, "/attachments/"+id+"/request-access", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp types.RequestAccessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Requested {
		t.Fatal("want requested=true")
	}
	if audit.requestCalled != 1 {
		t.Fatalf("want 1 request-access audit row, got %d", audit.requestCalled)
	}
	if audit.requestCapturedBy != "human:test" {
		t.Fatalf("want request-access audit to carry the request principal, got %q", audit.requestCapturedBy)
	}
}

func TestAttachmentHandler_RoutingRegression_ListAndGetStillWork(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	h := newTestHandlerWithSeams(store, &fakeAudit{}, extraction.NoOpExtractor{}, nil)

	getReq := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id, nil))
	getW := httptest.NewRecorder()
	h.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET /attachments/{id} status = %d, want 200", getW.Code)
	}

	listReq := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments", nil))
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("GET /attachments status = %d, want 200", listW.Code)
	}
}
