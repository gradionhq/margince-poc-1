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

	_ "github.com/lib/pq" // registers the "postgres" database/sql driver

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const rgHandlerTestWorkspaceID = "00000000-0000-0000-0000-000000000e01"

// rgHandlerTestAgentID is a fixed, valid-UUID agent caller ID for the 🟡
// approval-gating tests below — a real app_user row is seeded for it (via
// seedRGHandlerAgentUser) since record_grant.granted_by and
// audit_log.on_behalf_of both FK to app_user.id (mirrors
// handler_deal_advance_integration_test.go's seedAdvHandlerFixtures agent
// app_user pattern).
const rgHandlerTestAgentID = "00000000-0000-0000-0000-0000000e0002"

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

// withRGWorkspaceAgent mirrors withRGWorkspace but marks the principal as an
// agent (IsAgent: true, UserID: rgHandlerTestAgentID) so the handler's 🟡
// approval gate ("if p.IsAgent") engages.
func withRGWorkspaceAgent(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: rgHandlerTestWorkspaceID, UserID: rgHandlerTestAgentID, IsAgent: true})
	return r.WithContext(ctx)
}

// seedRGHandlerAgentUser seeds an is_agent=true app_user row for the fixed
// rgHandlerTestAgentID — required for an agent principal to be a valid
// record_grant.granted_by / audit_log.on_behalf_of FK target. Must be called
// after seedRGHandlerFixtures (which creates the workspace row this FKs to).
func seedRGHandlerAgentUser(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO app_user (id, workspace_id, email, display_name, is_agent)
		VALUES ($1, $2, 'rg-agent@test.example', 'RG Agent', true)
		ON CONFLICT (id) DO NOTHING`,
		rgHandlerTestAgentID, rgHandlerTestWorkspaceID); err != nil {
		t.Fatalf("seed agent app_user: %v", err)
	}
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

// rgGrantBody builds the standard "grant subjectID read access to
// person personID" request body shared by every create-path test below
// (a plain read grant on a person record is the common fixture; tests
// that need something different build their own body inline).
func rgGrantBody(personID, subjectID string) map[string]any {
	return map[string]any{
		"record_type": "person", "record_id": personID,
		"subject_type": "user", "subject_id": subjectID,
		"access": "read",
	}
}

// createRGGrant POSTs rgGrantBody(personID, subjectID) as granterID through h,
// fails the test unless the response is 201, and returns the decoded response
// body. Used by tests that need a pre-existing grant row as setup (e.g. to
// revoke or list) and don't care about the create response's own shape.
func createRGGrant(t *testing.T, h http.Handler, granterID, subjectID, personID string) map[string]any {
	t.Helper()
	b, _ := json.Marshal(rgGrantBody(personID, subjectID))
	req := withRGWorkspace(httptest.NewRequest(http.MethodPost, "/record-grants", bytes.NewReader(b)), granterID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("seed create status = %d, body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode seed create response: %v", err)
	}
	return created
}

// assertRGApprovalRequired asserts w is the 🟡 approval-gate's standard
// 403 {"code":"approval_required"} rejection — shared by the agent-without-
// token tests on both the create and revoke paths.
func assertRGApprovalRequired(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", w.Code, w.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if raw["code"] != "approval_required" {
		t.Fatalf("code = %v, want %q", raw["code"], "approval_required")
	}
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

	b, _ := json.Marshal(rgGrantBody(personID, subjectID))
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

	// crm.yaml's RecordGrant schema requires 8 fields
	// (id, record_type, record_id, subject_type, subject_id, access,
	// granted_by, created_at) — assert every one is present and non-null, not
	// just the 3 checked above.
	if got := raw["record_id"]; got != personID {
		t.Fatalf("record_id = %v, want %q", got, personID)
	}
	if got := raw["subject_type"]; got != "user" {
		t.Fatalf("subject_type = %v, want %q", got, "user")
	}
	if got := raw["subject_id"]; got != subjectID {
		t.Fatalf("subject_id = %v, want %q", got, subjectID)
	}
	if got := raw["access"]; got != "read" {
		t.Fatalf("access = %v, want %q", got, "read")
	}
	if got, ok := raw["created_at"]; !ok || got == nil {
		t.Fatalf("expected non-null %q key, got %+v", "created_at", raw)
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

	createRGGrant(t, h, granterID, subjectID, personID)

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

// TestRecordGrantHandler_Create_AgentWithoutToken_403ApprovalRequired proves
// the 🟡 (x-mcp-tool share_record) gate on POST /record-grants actually
// engages for an agent principal: FINAL-UAT found zero test drove the agent
// path at all, so a regression silently deleting the `if p.IsAgent {...}`
// check in create() would ship green without this.
func TestRecordGrantHandler_Create_AgentWithoutToken_403ApprovalRequired(t *testing.T) {
	db := openRGHandlerTestDB(t)
	_, subjectID, personID := seedRGHandlerFixtures(t, db, "create-agent-no-token")
	seedRGHandlerAgentUser(t, db)
	h := NewRecordGrantHandler(crmcore.NewRecordGrantStore(db), db)

	b, _ := json.Marshal(rgGrantBody(personID, subjectID))
	req := httptest.NewRequest(http.MethodPost, "/record-grants", bytes.NewReader(b))
	req = withRGWorkspaceAgent(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assertRGApprovalRequired(t, w)

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM record_grant WHERE record_type='person' AND record_id=$1::uuid AND subject_id=$2::uuid`,
		personID, subjectID).Scan(&count); err != nil {
		t.Fatalf("count record_grant: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no record_grant row inserted without an approval token, got count=%d", count)
	}
}

// TestRecordGrantHandler_Create_AgentWithValidToken_Succeeds proves the
// counterpart of the 403 above: an agent principal presenting a token whose
// diff_hash binds the exact create fields the handler itself hashes
// (record_type/record_id/subject_type/subject_id/access) is let through.
func TestRecordGrantHandler_Create_AgentWithValidToken_Succeeds(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "rg-handler-it-secret")
	db := openRGHandlerTestDB(t)
	_, subjectID, personID := seedRGHandlerFixtures(t, db, "create-agent-token")
	seedRGHandlerAgentUser(t, db)
	h := NewRecordGrantHandler(crmcore.NewRecordGrantStore(db), db)

	body := rgGrantBody(personID, subjectID)
	diffHash := crmapprovals.HashDiff(rgGrantBody(personID, subjectID))
	tok, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: "rg-create-jti-" + personID, ApprovalID: "appr-rg-create", WorkspaceID: rgHandlerTestWorkspaceID,
		Tool: "share_record", DiffHash: diffHash,
		Exp: time.Now().Add(5 * time.Minute), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/record-grants", bytes.NewReader(b))
	req = withRGWorkspaceAgent(req)
	req.Header.Set("X-Approval-Token", tok)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created grant: %v", err)
	}
	grantID, _ := created["id"].(string)
	if grantID == "" {
		t.Fatalf("expected non-empty id in response: %+v", created)
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, grantID).Scan(&count); err != nil {
		t.Fatalf("count record_grant: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the record_grant row to exist after a valid-token create, got count=%d", count)
	}
}

// TestRecordGrantHandler_Revoke_AgentWithoutToken_403ApprovalRequired proves
// the same 🟡 gate on DELETE /record-grants/{id} engages for an agent
// principal with no X-Approval-Token, and that the underlying row survives.
func TestRecordGrantHandler_Revoke_AgentWithoutToken_403ApprovalRequired(t *testing.T) {
	db := openRGHandlerTestDB(t)
	granterID, subjectID, personID := seedRGHandlerFixtures(t, db, "revoke-agent-no-token")
	seedRGHandlerAgentUser(t, db)
	h := NewRecordGrantHandler(crmcore.NewRecordGrantStore(db), db)

	created := createRGGrant(t, h, granterID, subjectID, personID)
	grantID, _ := created["id"].(string)

	delReq := httptest.NewRequest(http.MethodDelete, "/record-grants/"+grantID, nil)
	delReq.SetPathValue("id", grantID)
	delReq = withRGWorkspaceAgent(delReq)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, delReq)

	assertRGApprovalRequired(t, w)

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, grantID).Scan(&count); err != nil {
		t.Fatalf("count record_grant: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the record_grant row NOT to be deleted without an approval token, got count=%d", count)
	}
}

// TestRecordGrantHandler_Revoke_AgentWithValidToken_Succeeds proves the
// counterpart: an agent principal presenting a token whose diff_hash binds
// {"id": <grant id>} (the exact map revoke() itself hashes) is let through
// and the row is actually deleted.
func TestRecordGrantHandler_Revoke_AgentWithValidToken_Succeeds(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "rg-handler-it-secret")
	db := openRGHandlerTestDB(t)
	granterID, subjectID, personID := seedRGHandlerFixtures(t, db, "revoke-agent-token")
	seedRGHandlerAgentUser(t, db)
	h := NewRecordGrantHandler(crmcore.NewRecordGrantStore(db), db)

	created := createRGGrant(t, h, granterID, subjectID, personID)
	grantID, _ := created["id"].(string)

	diffHash := crmapprovals.HashDiff(map[string]any{"id": grantID})
	tok, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: "rg-revoke-jti-" + grantID, ApprovalID: "appr-rg-revoke", WorkspaceID: rgHandlerTestWorkspaceID,
		Tool: "share_record", DiffHash: diffHash,
		Exp: time.Now().Add(5 * time.Minute), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/record-grants/"+grantID, nil)
	delReq.SetPathValue("id", grantID)
	delReq = withRGWorkspaceAgent(delReq)
	delReq.Header.Set("X-Approval-Token", tok)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, delReq)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", w.Code, w.Body.String())
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, grantID).Scan(&count); err != nil {
		t.Fatalf("count record_grant: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected the record_grant row to be deleted after a valid-token revoke, got count=%d", count)
	}
}

// TestRecordGrantHandler_Revoke_ViaRealMux_DeletesRow is the real
// route-registration regression test for the bug where revoke() read the
// grant id via r.PathValue("id"): cmd/api/routes.go's registerCoreCRUD
// mounts /record-grants as a plain trailing-slash subtree
// (mux.Handle("/record-grants", h); mux.Handle("/record-grants/", h)) with
// NO {id} wildcard segment, so on a real net/http.ServeMux dispatch
// r.PathValue("id") is always "" — unlike the other Revoke_* tests above,
// which call delReq.SetPathValue("id", grantID) directly and so never
// exercise real routing at all. This test builds a mux with that exact
// registration shape and dispatches a genuine DELETE through it (no
// SetPathValue), proving the id is captured from the URL and the row is
// actually deleted end to end.
func TestRecordGrantHandler_Revoke_ViaRealMux_DeletesRow(t *testing.T) {
	db := openRGHandlerTestDB(t)
	granterID, subjectID, personID := seedRGHandlerFixtures(t, db, "revoke-real-mux")
	h := NewRecordGrantHandler(crmcore.NewRecordGrantStore(db), db)

	created := createRGGrant(t, h, granterID, subjectID, personID)
	grantID, _ := created["id"].(string)

	// Mirror routes.go's crud() helper verbatim: a bare path and its
	// trailing-slash subtree, both routed to h — no method prefix, no {id}.
	mux := http.NewServeMux()
	mux.Handle("/record-grants", h)
	mux.Handle("/record-grants/", h)

	delReq := withRGWorkspace(httptest.NewRequest(http.MethodDelete, "/record-grants/"+grantID, nil), granterID)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, delReq)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", w.Code, w.Body.String())
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM record_grant WHERE id=$1::uuid`, grantID).Scan(&count); err != nil {
		t.Fatalf("count record_grant: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected the record_grant row to be deleted via a real mux-dispatched DELETE, got count=%d", count)
	}
}
