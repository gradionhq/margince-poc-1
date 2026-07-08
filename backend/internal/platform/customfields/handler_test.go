package customfields

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

type fakeVerifier struct {
	err error
}

func (f fakeVerifier) VerifyAndConsume(ctx context.Context, token string, want approvalsport.Binding) error {
	return f.err
}

func withCFPrincipal(r *http.Request, isAgent bool, userID, wsID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{UserID: userID, TenantID: wsID, IsAgent: isAgent})
	return r.WithContext(ctx)
}

func newCreateReq(body map[string]any, isAgent bool, token string) *http.Request {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/custom-fields", bytes.NewReader(b))
	req = withCFPrincipal(req, isAgent, "00000000-0000-0000-0000-0000000c0001", "00000000-0000-0000-0000-0000000c0099")
	if token != "" {
		req.Header.Set("X-Approval-Token", token)
	}
	return req
}

// assertAgentCreateForbidden drives the shared shape behind both
// "agent tried to create a custom field but the approval gate rejected it"
// tests below: db stays nil, so the assertion also proves Create() is never
// reached once the verifier says no.
func assertAgentCreateForbidden(t *testing.T, verifierErr error, wantCode string) {
	t.Helper()
	h := NewHandler(nil, fakeVerifier{err: verifierErr})
	token := ""
	if verifierErr != nil {
		token = "bad-token"
	}
	req := newCreateReq(map[string]any{"object": "deal", "label": "Renewal date", "type": "date", "source": "ui", "captured_by": "human:u1"}, true, token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != wantCode {
		t.Fatalf("expected code=%s, got %v", wantCode, body["code"])
	}
}

func TestCreateCustomField_AgentWithoutToken_403ApprovalRequired_NeverTouchesDB(t *testing.T) {
	assertAgentCreateForbidden(t, nil, "approval_required")
}

func TestCreateCustomField_AgentWithInvalidToken_403ApprovalTokenInvalid_NeverTouchesDB(t *testing.T) {
	assertAgentCreateForbidden(t, errTokenInvalidStub, "approval_token_invalid")
}

func TestCreateCustomField_MalformedBody_400_NeverTouchesDB(t *testing.T) {
	h := NewHandler(nil, fakeVerifier{})
	req := httptest.NewRequest(http.MethodPost, "/custom-fields", bytes.NewReader([]byte("{not json")))
	req = withCFPrincipal(req, false, "00000000-0000-0000-0000-0000000c0001", "00000000-0000-0000-0000-0000000c0099")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCustomField_GET_501(t *testing.T) {
	h := NewHandler(nil, fakeVerifier{})
	req := httptest.NewRequest(http.MethodGet, "/custom-fields?object=deal", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 (listCustomFields out of scope), got %d", w.Code)
	}
}

var errTokenInvalidStub = &tokenInvalidStubErr{}

type tokenInvalidStubErr struct{}

func (*tokenInvalidStubErr) Error() string { return "stub: token invalid" }
