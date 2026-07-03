//go:build integration

package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func seedPersonActivity(t *testing.T, db *sql.DB, wsID, personID, kind, direction string, occurredAt time.Time) {
	t.Helper()

	var activityID string
	err := db.QueryRowContext(
		context.Background(), `
		INSERT INTO activity (workspace_id, kind, occurred_at, direction, source, captured_by)
		VALUES ($1,$2,$3,$4,'test','human:test')
		RETURNING id`,
		wsID, kind, occurredAt, direction,
	).Scan(&activityID)
	if err != nil {
		t.Fatal("seed activity:", err)
	}

	if _, err := db.ExecContext(
		context.Background(), `
		INSERT INTO activity_link (workspace_id, activity_id, entity_type, person_id)
		VALUES ($1,$2,'person',$3)`,
		wsID, activityID, personID,
	); err != nil {
		t.Fatal("seed activity_link:", err)
	}
}

func TestPersonHandler_Get_StrengthNoSignalYet(t *testing.T) {
	db := openTestDB(t)
	store := directory.NewPersonStore(db)
	h := NewPersonHandler(store)

	const wsID = testWorkspaceID
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})
	p := directory.NewPerson("NoSignal Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = wsID
	created, err := store.Create(ctx, p)
	if err != nil {
		t.Fatal("seed:", err)
	}

	req := withWorkspace(httptest.NewRequest(http.MethodGet, "/people/"+created.ID, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /people/{id}: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if v, ok := resp["strength"]; !ok || v != nil {
		t.Errorf("strength = %v, want present-and-null for zero interactions ever", v)
	}
}

func TestPersonHandler_Get_StrengthNoRecentActivity(t *testing.T) {
	db := openTestDB(t)
	store := directory.NewPersonStore(db)
	h := NewPersonHandler(store)

	const wsID = testWorkspaceID
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})
	p := directory.NewPerson("Stale Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = wsID
	created, err := store.Create(ctx, p)
	if err != nil {
		t.Fatal("seed:", err)
	}
	seedPersonActivity(t, db, wsID, created.ID, "email", "inbound", time.Now().UTC().AddDate(0, 0, -120))

	req := withWorkspace(httptest.NewRequest(http.MethodGet, "/people/"+created.ID, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /people/{id}: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	strength, ok := resp["strength"].(map[string]any)
	if !ok {
		t.Fatalf("strength = %v, want an object (score=0/weak), not null", resp["strength"])
	}
	if strength["score"] != float64(0) || strength["bucket"] != "weak" {
		t.Errorf("strength = %+v, want score=0 bucket=weak", strength)
	}
	if strength["no_recent_activity"] != true {
		t.Errorf("strength.no_recent_activity = %v, want true", strength["no_recent_activity"])
	}
}

func TestPersonHandler_List_SortStrength(t *testing.T) {
	db := openTestDB(t)
	store := directory.NewPersonStore(db)
	h := NewPersonHandler(store)

	const wsID = testWorkspaceID
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})

	mkPerson := func(name string) directory.Person {
		p := directory.NewPerson(name, prov.Provenance{Source: "test", CapturedBy: "human:test"})
		p.WorkspaceID = wsID
		created, err := store.Create(ctx, p)
		if err != nil {
			t.Fatal("seed:", err)
		}
		return created
	}

	weak := mkPerson("Weak Signal")
	strong := mkPerson("Strong Signal")
	for i := 0; i < 8; i++ {
		seedPersonActivity(t, db, wsID, strong.ID, "email", "inbound", time.Now().UTC().AddDate(0, 0, -i))
	}
	for i := 0; i < 7; i++ {
		seedPersonActivity(t, db, wsID, strong.ID, "call", "outbound", time.Now().UTC().AddDate(0, 0, -i-1))
	}
	seedPersonActivity(t, db, wsID, weak.ID, "email", "inbound", time.Now().UTC().AddDate(0, 0, -85))

	req := withWorkspace(httptest.NewRequest(http.MethodGet, "/people?sort=strength", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /people?sort=strength: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []directory.Person `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	idx := map[string]int{}
	for i, p := range resp.Data {
		idx[p.ID] = i
	}
	if idx[strong.ID] >= idx[weak.ID] {
		t.Errorf("sort=strength: strong (idx %d) must sort before weak (idx %d)", idx[strong.ID], idx[weak.ID])
	}

	req3 := withWorkspace(httptest.NewRequest(http.MethodGet, "/people?sort=-strength", nil))
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("GET /people?sort=-strength: want 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var resp3 struct {
		Data []directory.Person `json:"data"`
	}
	if err := json.NewDecoder(w3.Body).Decode(&resp3); err != nil {
		t.Fatal(err)
	}
	idx3 := map[string]int{}
	for i, p := range resp3.Data {
		idx3[p.ID] = i
	}
	if idx3[weak.ID] >= idx3[strong.ID] {
		t.Errorf("sort=-strength: weak (idx %d) must sort before strong (idx %d) — exact inverse of sort=strength", idx3[weak.ID], idx3[strong.ID])
	}

	req2 := withWorkspace(httptest.NewRequest(http.MethodGet, "/people?sort=bogus_field", nil))
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnprocessableEntity {
		t.Errorf("sort=bogus_field: want 422, got %d", w2.Code)
	}
}
