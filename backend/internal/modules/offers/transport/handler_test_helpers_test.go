package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

// decodeJSONBody unmarshals w's recorded body into a map, failing the test on
// any decode error — the shared tail of every offers handler test that
// inspects a JSON response body.
func decodeJSONBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return body
}

// assertCreated201 asserts a 201 response with a non-empty Location header —
// the shared tail of every offers handler "create succeeds" test.
func assertCreated201(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Location") == "" {
		t.Fatal("expected Location header")
	}
}

// postExpectConflict POSTs body to path via h, asserts a 409 with the given
// problem-detail code, and returns the decoded response body for any
// caller that needs to assert further fields (e.g. details.existing_id).
func postExpectConflict(t *testing.T, h http.Handler, path string, body map[string]any, wantCode string) map[string]any {
	t.Helper()
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(bodyBytes))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", w.Code, w.Body.String())
	}
	respBody := decodeJSONBody(t, w)
	if code, ok := respBody["code"].(string); !ok || code != wantCode {
		t.Fatalf("expected code=%s, got %v", wantCode, respBody["code"])
	}
	return respBody
}

// assertEmptyListOK asserts a 200 response whose "data" field, if present,
// is an empty array — the shared tail of every offers handler "list on
// empty store" test.
func assertEmptyListOK(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	respBody := decodeJSONBody(t, w)
	if data, ok := respBody["data"]; ok && data != nil {
		if items, ok := data.([]any); !ok || len(items) != 0 {
			t.Fatalf("expected empty data array, got %v", respBody["data"])
		}
	}
}

type fakeVerifier struct{}

func (fakeVerifier) VerifyAndConsume(_ context.Context, _ string, _ approvalsport.Binding) error {
	return errors.New("unexpected approval verification in unit test")
}

func newTestOfferHandler() *OfferHandler {
	return NewOfferHandler(newFakeOfferStore(), newFakeOfferLineItemStore(), fakeVerifier{}, blobstore.NewMemoryStore(), NewNoOpRetriever())
}

var _ approvalsport.Verifier = fakeVerifier{}
