//go:build integration

package customfields_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// getCF drives GET /custom-fields with the given query string as a human
// principal (a read is never gated).
func getCF(h *customfields.Handler, wsID, userID, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/custom-fields?"+query, nil)
	ctx := crmctx.With(req.Context(), crmctx.Principal{UserID: userID, TenantID: wsID, IsAgent: false})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// cfListColumns decodes a listCustomFields response body into the set of
// returned column_name values.
func cfListColumns(t *testing.T, w *httptest.ResponseRecorder) map[string]bool {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data []map[string]any `json:"data"`
		Page map[string]any   `json:"page"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	cols := make(map[string]bool, len(body.Data))
	for _, f := range body.Data {
		cols[f["column_name"].(string)] = true
	}
	return cols
}

// TestListCustomFields_ReturnsObjectScopedFields proves CUSTOM-FIELDS-WIRE-1
// end to end: fields created through the real engine come back on the admin
// list read, scoped to the requested object, and a retired field is included
// by default but excluded when status=active narrows the view.
func TestListCustomFields_ReturnsObjectScopedFields(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers(t, db)
	tag := time.Now().Format("150405.000000000")

	// Two deal fields + one person field, all created through the real path.
	dealA := postCF(h, wsID, humanID, false, "", map[string]any{"object": "deal", "label": "Renewal date " + tag, "type": "date", "source": "ui", "captured_by": "human:" + humanID})
	if dealA.Code != http.StatusCreated {
		t.Fatalf("create dealA: %d: %s", dealA.Code, dealA.Body.String())
	}
	dealB := postCF(h, wsID, humanID, false, "", map[string]any{"object": "deal", "label": "Budget ceiling " + tag, "type": "currency", "currency": "EUR", "source": "ui", "captured_by": "human:" + humanID})
	if dealB.Code != http.StatusCreated {
		t.Fatalf("create dealB: %d: %s", dealB.Code, dealB.Body.String())
	}
	person := postCF(h, wsID, humanID, false, "", map[string]any{"object": "person", "label": "Shoe size " + tag, "type": "number", "source": "ui", "captured_by": "human:" + humanID})
	if person.Code != http.StatusCreated {
		t.Fatalf("create person: %d: %s", person.Code, person.Body.String())
	}
	var dealBCol string
	{
		var created map[string]any
		_ = json.Unmarshal(dealB.Body.Bytes(), &created)
		dealBCol = created["column_name"].(string)
	}

	// GET ?object=deal returns exactly the two deal fields, not the person one.
	dealCols := cfListColumns(t, getCF(h, wsID, humanID, "object=deal"))
	if len(dealCols) != 2 {
		t.Fatalf("object=deal: expected 2 fields, got %d (%v)", len(dealCols), dealCols)
	}
	personCols := cfListColumns(t, getCF(h, wsID, humanID, "object=person"))
	for c := range dealCols {
		if personCols[c] {
			t.Fatalf("deal column %q leaked into the person list", c)
		}
	}

	// Retire dealB, then confirm it is still listed by default (admin view
	// includes retired) but excluded by status=active.
	retire := newRetireHTTP(h, wsID, humanID, dealB)
	if retire.Code != http.StatusOK {
		t.Fatalf("retire dealB: %d: %s", retire.Code, retire.Body.String())
	}
	allCols := cfListColumns(t, getCF(h, wsID, humanID, "object=deal"))
	if !allCols[dealBCol] {
		t.Fatalf("default admin list must include the retired field %q, got %v", dealBCol, allCols)
	}
	activeCols := cfListColumns(t, getCF(h, wsID, humanID, "object=deal&status=active"))
	if activeCols[dealBCol] {
		t.Fatalf("status=active must exclude the retired field %q, got %v", dealBCol, activeCols)
	}
}

// newRetireHTTP retires a just-created field by driving POST
// /custom-fields/{id}/retire as a human (no token needed).
func newRetireHTTP(h *customfields.Handler, wsID, userID string, created *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	var body map[string]any
	_ = json.Unmarshal(created.Body.Bytes(), &body)
	id := body["id"].(string)
	req := httptest.NewRequest(http.MethodPost, "/custom-fields/"+id+"/retire", nil)
	ctx := crmctx.With(req.Context(), crmctx.Principal{UserID: userID, TenantID: wsID, IsAgent: false})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}
