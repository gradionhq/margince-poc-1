//go:build integration

package transport

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const rgHandlerTestWorkspaceID = "00000000-0000-0000-0000-000000000e01"

func openRGHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func withRGWorkspace(r *http.Request, userID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: rgHandlerTestWorkspaceID, UserID: userID})
	return r.WithContext(ctx)
}

// seedRGHandlerFixtures seeds a workspace, an owner app_user (the granter — it
// owns the seeded person, giving it "write" own-access per
// resolveGrantorOwnAccess), a subject app_user (the grantee), and a person
// record owned by the granter.
func seedRGHandlerFixtures(t *testing.T, db *sql.DB, tag string) (granterID, subjectID, personID string) {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t-rg-handler-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, rgHandlerTestWorkspaceID, "t-rg-handler-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, rgHandlerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO app_user (id, workspace_id, email, display_name)
		VALUES (uuidv7(), $1, $2, 'Granter') RETURNING id`,
		rgHandlerTestWorkspaceID, "granter-"+tag+"@test.example").Scan(&granterID); err != nil {
		t.Fatalf("seed granter app_user: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO app_user (id, workspace_id, email, display_name)
		VALUES (uuidv7(), $1, $2, 'Subject') RETURNING id`,
		rgHandlerTestWorkspaceID, "subject-"+tag+"@test.example").Scan(&subjectID); err != nil {
		t.Fatalf("seed subject app_user: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, owner_id, source, captured_by)
		VALUES (uuidv7(), $1, $2, $3, 'test', 'human:test') RETURNING id`,
		rgHandlerTestWorkspaceID, "Person-"+tag, granterID).Scan(&personID); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return granterID, subjectID, personID
}

// TestRecordGrantHandler_Create_ReturnsSnakeCaseJSON proves POST
// /record-grants' response body is serialized with the contract's snake_case
// field names (e.g. "id", not "id" cast from an untagged Go field which would
// have rendered as "ID") — driving the real handler over HTTP (not calling
// the store directly) is required to observe the exact wire bug that was
// fixed on RecordGrant's json tags.
func TestRecordGrantHandler_Create_ReturnsSnakeCaseJSON(t *testing.T) {
	db := openRGHandlerTestDB(t)
	granterID, subjectID, personID := seedRGHandlerFixtures(t, db, "create")
	h := NewRecordGrantHandler(crmcore.NewRecordGrantStore(db), db)

	body := map[string]any{
		"record_type": "person", "record_id": personID,
		"subject_type": "user", "subject_id": subjectID,
		"access": "read",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/record-grants", bytes.NewReader(b))
	req = withRGWorkspace(req, granterID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw JSON: %v", err)
	}

	if _, ok := raw["id"]; !ok {
		t.Fatalf("expected top-level snake_case %q key, got %+v", "id", raw)
	}
	if got := raw["record_type"]; got != "person" {
		t.Fatalf("record_type = %v, want %q", got, "person")
	}
	if got := raw["granted_by"]; got != granterID {
		t.Fatalf("granted_by = %v, want %q", got, granterID)
	}

	for _, pascal := range []string{"ID", "WorkspaceID", "RecordType", "GrantedBy"} {
		if _, ok := raw[pascal]; ok {
			t.Fatalf("response must not contain PascalCase key %q (untagged struct leak): %+v", pascal, raw)
		}
	}
}

// TestRecordGrantHandler_List_ReturnsDataAndPageWithHasMore proves GET
// /record-grants' raw JSON envelope has both "data" and "page" keys, and that
// "page" carries a "has_more" boolean — the field the list handler's
// pageResponse(...) fix added (it previously only returned "next_cursor").
func TestRecordGrantHandler_List_ReturnsDataAndPageWithHasMore(t *testing.T) {
	db := openRGHandlerTestDB(t)
	granterID, subjectID, personID := seedRGHandlerFixtures(t, db, "list")
	h := NewRecordGrantHandler(crmcore.NewRecordGrantStore(db), db)

	createBody, _ := json.Marshal(map[string]any{
		"record_type": "person", "record_id": personID,
		"subject_type": "user", "subject_id": subjectID,
		"access": "read",
	})
	createReq := withRGWorkspace(httptest.NewRequest(http.MethodPost, "/record-grants", bytes.NewReader(createBody)), granterID)
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("seed create status = %d, body=%s", createW.Code, createW.Body.String())
	}

	listReq := withRGWorkspace(httptest.NewRequest(http.MethodGet, "/record-grants?record_type=person&record_id="+personID, nil), granterID)
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200, body=%s", listW.Code, listW.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(listW.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw JSON: %v", err)
	}

	if _, ok := raw["data"]; !ok {
		t.Fatalf(`expected top-level "data" key, got %+v`, raw)
	}
	page, ok := raw["page"].(map[string]any)
	if !ok {
		t.Fatalf(`expected top-level "page" object key, got %+v`, raw)
	}
	hasMore, ok := page["has_more"].(bool)
	if !ok {
		t.Fatalf(`expected "page.has_more" boolean key, got %+v`, page)
	}
	if hasMore {
		t.Fatalf("has_more = true, want false for a single-row result under the default limit")
	}
}
