package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpoint_ExposesCoreSet_BoundedLabels(t *testing.T) {
	// exercise the duration histogram once
	rec := httptest.NewRecorder()
	instrument("/people", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/people", nil))

	srv := httptest.NewServer(metricsHandler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	for _, name := range []string{"http_request_duration_seconds", "event_outbox_depth", "consumer_lag_seconds"} {
		if !strings.Contains(text, name) {
			t.Fatalf("missing metric %q in /metrics output", name)
		}
	}
	if strings.Contains(text, "workspace_id") || strings.Contains(text, "entity_id") {
		t.Fatalf("metrics must carry no high-cardinality labels (found workspace_id/entity_id)")
	}
}
