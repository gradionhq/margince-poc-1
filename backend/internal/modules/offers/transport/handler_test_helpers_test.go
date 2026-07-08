package transport

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
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
