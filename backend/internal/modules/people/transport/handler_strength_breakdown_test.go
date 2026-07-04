//go:build integration

package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func TestPersonHandler_StrengthBreakdown(t *testing.T) {
	db := openTestDB(t)
	store := directory.NewPersonStore(db)
	h := NewPersonHandler(store, db)

	const wsID = "00000000-0000-0000-0000-000000000001"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})

	p := directory.NewPerson("Breakdown Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = wsID
	created, err := store.Create(ctx, p, nil)
	if err != nil {
		t.Fatal("seed:", err)
	}
	seedPersonActivity(t, db, wsID, created.ID, "email", "inbound", time.Now().UTC().AddDate(0, 0, -3))
	seedPersonActivity(t, db, wsID, created.ID, "call", "outbound", time.Now().UTC().AddDate(0, 0, -10))

	req := withWorkspace(httptest.NewRequest(http.MethodGet, "/people/"+created.ID+"/strength-breakdown", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET .../strength-breakdown: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		PersonID               string  `json:"person_id"`
		Score                  int     `json:"score"`
		Bucket                 string  `json:"bucket"`
		Recency                float64 `json:"recency"`
		Frequency              float64 `json:"frequency"`
		Reciprocity            float64 `json:"reciprocity"`
		ContributingActivities []struct {
			ID string `json:"id"`
		} `json:"contributing_activities"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.PersonID != created.ID {
		t.Errorf("person_id = %q, want %q", resp.PersonID, created.ID)
	}
	if len(resp.ContributingActivities) != 2 {
		t.Fatalf("contributing_activities len = %d, want 2 (the evidence behind the score)", len(resp.ContributingActivities))
	}

	// 404 for an unknown person.
	req2 := withWorkspace(httptest.NewRequest(http.MethodGet, "/people/00000000-0000-0000-0000-0000000000ff/strength-breakdown", nil))
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("unknown person: want 404, got %d", w2.Code)
	}
}
